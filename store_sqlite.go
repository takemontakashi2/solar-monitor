package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db              *sql.DB
	mu              sync.RWMutex
	buffered        bool
	pending         []Sample
	pendingBytes    int
	maxPendingBytes int
	fixedPending    map[string][]SampleProperty
	rejectedPending map[string]map[string]bool
}

func NewStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) init() error {
	_, err := s.db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA busy_timeout = 5000;
        CREATE TABLE IF NOT EXISTS rejected_fixed (target TEXT NOT NULL, property_key TEXT NOT NULL, PRIMARY KEY(target,property_key));
        CREATE TABLE IF NOT EXISTS fixed_reads (
          target TEXT NOT NULL, property_key TEXT NOT NULL,
          PRIMARY KEY(target, property_key)
        );
		CREATE TABLE IF NOT EXISTS samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			time TEXT NOT NULL,
			pv_watts REAL NOT NULL,
			load_watts REAL NOT NULL,
			grid_watts REAL NOT NULL,
			today_kwh REAL NOT NULL,
			source TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS samples_time_idx ON samples(time);
 CREATE INDEX IF NOT EXISTS samples_instant_idx ON samples(julianday(time));
		CREATE TABLE IF NOT EXISTS properties (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sample_id INTEGER NOT NULL,
			time TEXT NOT NULL,
			eoj TEXT NOT NULL,
			epc TEXT NOT NULL,
			name TEXT NOT NULL,
			raw TEXT NOT NULL,
			unsigned_value INTEGER,
			signed_value INTEGER,
			float_value REAL,
			description TEXT NOT NULL,
			FOREIGN KEY(sample_id) REFERENCES samples(id)
		);
		CREATE INDEX IF NOT EXISTS properties_time_idx ON properties(time);
		CREATE INDEX IF NOT EXISTS properties_key_idx ON properties(eoj, epc, time);
		CREATE INDEX IF NOT EXISTS properties_sample_idx ON properties(sample_id, eoj, epc);
	`)
	if err != nil {
		return err
	}
	// Preserve missing readings separately from genuine zeroes, including on old databases.
	rows, err := s.db.Query(`PRAGMA table_info(samples)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		found = found || name == "valid_fields"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !found {
		_, err = s.db.Exec(`ALTER TABLE samples ADD COLUMN valid_fields INTEGER NOT NULL DEFAULT -1`)
	}
	return err
}

func (s *Store) Append(sample Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.buffered {
		return s.appendBatch([]Sample{sample})
	}
	// Own the queued data so callers cannot mutate it after collection.
	raw, err := json.Marshal(sample)
	if err != nil {
		return err
	}
	if s.pendingBytes+len(raw) > s.maxPendingBytes {
		if err := s.flushLocked(); err != nil {
			return fmt.Errorf("buffer full; flush failed: %w", err)
		}
	}
	if len(raw) > s.maxPendingBytes {
		return fmt.Errorf("sample exceeds buffer capacity")
	}
	var owned Sample
	if err := json.Unmarshal(raw, &owned); err != nil {
		return err
	}
	s.pending = append(s.pending, owned)
	s.pendingBytes += len(raw)
	return nil
}

func (s *Store) appendBatch(samples []Sample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, sample := range samples {
		if err := insertSample(tx, sample); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertSample(tx *sql.Tx, sample Sample) error {
	res, err := tx.Exec(`
		INSERT INTO samples (time, pv_watts, load_watts, grid_watts, today_kwh, source, valid_fields)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		sample.Time.Format(time.RFC3339Nano),
		valueOrZero(sample.PVWatts),
		valueOrZero(sample.LoadWatts),
		valueOrZero(sample.GridWatts),
		valueOrZero(sample.TodayKWh),
		sample.Source,
		sampleValidity(sample),
	)
	if err != nil {
		return err
	}
	sampleID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO properties (sample_id, time, eoj, epc, name, raw, unsigned_value, signed_value, float_value, description)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, prop := range sample.Properties {
		if prop.Time.IsZero() {
			prop.Time = sample.Time
		}
		_, err := stmt.Exec(
			sampleID,
			prop.Time.Format(time.RFC3339Nano),
			prop.EOJ,
			prop.EPC,
			prop.Name,
			prop.Raw,
			nullUint64(prop.Unsigned),
			nullInt64(prop.Signed),
			nullFloat64(prop.Float),
			prop.Description,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// EnableBuffering must be called before starting collectors or HTTP handlers.
func (s *Store) EnableBuffering() {
	s.buffered = true
	s.maxPendingBytes = 16 * 1024 * 1024
	s.fixedPending = map[string][]SampleProperty{}
	s.rejectedPending = map[string]map[string]bool{}
}

func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

func (s *Store) flushLocked() error {
	if err := s.appendBatch(s.pending); err != nil {
		return err
	}
	s.pending = nil
	s.pendingBytes = 0
	for target, props := range s.fixedPending {
		if err := s.recordFixedReads(target, props); err != nil {
			return err
		}
		delete(s.fixedPending, target)
	}
	for target, rejected := range s.rejectedPending {
		if err := s.saveRejectedFixed(target, rejected); err != nil {
			return err
		}
		delete(s.rejectedPending, target)
	}
	return nil
}

func (s *Store) Latest(limit int) ([]Sample, error) {
	return s.latest(limit, true)
}

func (s *Store) LatestSummaries(limit int) ([]Sample, error) {
	return s.latest(limit, false)
}

func (s *Store) latest(limit int, includeProperties bool) ([]Sample, error) {
	return s.readSamples(limit, includeProperties, "", nil)
}

func (s *Store) SummariesBetween(start, end time.Time) ([]Sample, error) {
	return s.readSamples(-1, false, " WHERE julianday(time) >= julianday(?) AND julianday(time) < julianday(?) ", []any{start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)})
}

func (s *Store) readSamples(limit int, includeProperties bool, filter string, args []any) ([]Sample, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readCombined(limit, includeProperties, filter, args)
}

func (s *Store) readCombined(limit int, includeProperties bool, filter string, args []any) ([]Sample, error) {
	out, err := s.readDiskSamples(limit, includeProperties, filter, args)
	if err != nil {
		return nil, err
	}
	for _, sample := range s.pending {
		if filter != "" {
			start, _ := time.Parse(time.RFC3339Nano, args[0].(string))
			end, _ := time.Parse(time.RFC3339Nano, args[1].(string))
			if sample.Time.Before(start) || !sample.Time.Before(end) {
				continue
			}
		}
		if !includeProperties {
			sample.Properties = nil
		}
		out = append(out, sample)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	if limit >= 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func (s *Store) readDiskSamples(limit int, includeProperties bool, filter string, args []any) ([]Sample, error) {
	args = append(args, limit)
	rows, err := s.db.Query(`
		SELECT id, time, pv_watts, load_watts, grid_watts, today_kwh, source, valid_fields,
        (SELECT float_value FROM properties p WHERE p.sample_id=samples.id AND p.eoj='027d01' AND p.epc='e4' ORDER BY p.id DESC LIMIT 1)
		FROM samples`+filter+`
		ORDER BY julianday(time) DESC, id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	legacy := map[int64]bool{}
	out := []Sample{}
	for rows.Next() {
		var sample Sample
		var id int64
		var rawTime string
		var pv, load, grid, today float64
		var valid int
		var battery sql.NullFloat64
		err := rows.Scan(&id, &rawTime, &pv, &load, &grid, &today, &sample.Source, &valid, &battery)
		if err != nil {
			return nil, err
		}
		if battery.Valid && battery.Float64 >= 0 && battery.Float64 <= 100 {
			sample.BatteryPercent = floatPtr(battery.Float64)
		}
		sample.Time, err = time.Parse(time.RFC3339Nano, rawTime)
		if valid >= 0 {
			if valid&1 != 0 {
				sample.PVWatts = floatPtr(pv)
			}
			if valid&2 != 0 {
				sample.LoadWatts = floatPtr(load)
			}
			if valid&4 != 0 {
				sample.GridWatts = floatPtr(grid)
			}
			if valid&8 != 0 {
				sample.TodayKWh = floatPtr(today)
			}
		} else if sample.Source != "echonet" {
			sample.PVWatts = floatPtr(pv)
			sample.LoadWatts = floatPtr(load)
			sample.GridWatts = floatPtr(grid)
			sample.TodayKWh = floatPtr(today)
		} else {
			legacy[id] = true
		}
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
		out = append(out, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
		ids[i], ids[j] = ids[j], ids[i]
	}
	rows.Close()
	for i, id := range ids {
		if !includeProperties && !legacy[id] {
			continue
		}
		props, err := s.propertiesForSample(id)
		if err != nil {
			return nil, err
		}
		out[i].Properties = props
		if out[i].Source == "echonet" {
			deriveECHONETSummary(&out[i])
		}
		if !includeProperties {
			out[i].Properties = nil
		}
	}
	return out, nil
}

func sampleValidity(sample Sample) int {
	mask := 0
	for i, value := range []*float64{sample.PVWatts, sample.LoadWatts, sample.GridWatts, sample.TodayKWh} {
		if value != nil {
			mask |= 1 << i
		}
	}
	return mask
}

func (s *Store) propertiesForSample(sampleID int64) ([]SampleProperty, error) {
	rows, err := s.db.Query(`
		SELECT time, eoj, epc, name, raw, unsigned_value, signed_value, float_value, description
		FROM properties
		WHERE sample_id = ?
		ORDER BY eoj, epc
	`, sampleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanProperties(rows)
}

func (s *Store) LatestProperties(limit int, keys map[string]struct{}) ([]SampleProperty, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.pending) == 0 {
		return s.diskProperties(limit, keys)
	}
	props := []SampleProperty{}
	if limit == 1 {
		saved, err := s.latestPropertyValues(keys)
		if err != nil {
			return nil, err
		}
		byKey := map[string]SampleProperty{}
		for _, p := range saved {
			byKey[p.EOJ+":"+p.EPC] = p
		}
		for _, sample := range s.pending {
			for _, p := range sampleProperties(sample, keys) {
				key := p.EOJ + ":" + p.EPC
				if old, ok := byKey[key]; !ok || !p.Time.Before(old.Time) {
					byKey[key] = p
				}
			}
		}
		for _, p := range byKey {
			props = append(props, p)
		}
	} else {
		samples, err := s.readCombined(limit, true, "", nil)
		if err != nil {
			return nil, err
		}
		for _, sample := range samples {
			props = append(props, sampleProperties(sample, keys)...)
		}
	}
	sort.SliceStable(props, func(i, j int) bool {
		if limit != 1 && !props[i].Time.Equal(props[j].Time) {
			return props[i].Time.Before(props[j].Time)
		}
		return props[i].EOJ+props[i].EPC < props[j].EOJ+props[j].EPC
	})
	return props, nil
}

func sampleProperties(sample Sample, keys map[string]struct{}) []SampleProperty {
	out := []SampleProperty{}
	for _, p := range sample.Properties {
		if _, ok := keys[p.EOJ+":"+p.EPC]; len(keys) > 0 && !ok {
			continue
		}
		if p.Time.IsZero() {
			p.Time = sample.Time
		}
		p.Fixed = isFixedProperty(p.EOJ, p.EPC)
		out = append(out, p)
	}
	return out
}

func (s *Store) diskProperties(limit int, keys map[string]struct{}) ([]SampleProperty, error) {
	if limit == 1 {
		return s.latestPropertyValues(keys)
	}
	args := []any{limit}
	filter := ""
	if len(keys) > 0 {
		clauses := make([]string, 0, len(keys))
		for key := range keys {
			eoj, epc, ok := strings.Cut(key, ":")
			if !ok || eoj == "" || epc == "" {
				continue
			}
			clauses = append(clauses, "(eoj = ? AND epc = ?)")
			args = append(args, eoj, epc)
		}
		if len(clauses) > 0 {
			filter = " AND (" + strings.Join(clauses, " OR ") + ")"
		}
	}
	rows, err := s.db.Query(`
		WITH latest_samples AS (
			SELECT id
			FROM samples
			ORDER BY time DESC, id DESC
			LIMIT ?
		)
		SELECT time, eoj, epc, name, raw, unsigned_value, signed_value, float_value, description
		FROM properties
		WHERE sample_id IN (SELECT id FROM latest_samples)`+filter+`
		ORDER BY time, eoj, epc
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanProperties(rows)
}

// Return the last successful reading of each property, retaining its original timestamp.
func (s *Store) latestPropertyValues(keys map[string]struct{}) ([]SampleProperty, error) {
	filter := ""
	args := []any{}
	clauses := []string{}
	for key := range keys {
		eoj, epc, ok := strings.Cut(key, ":")
		if !ok {
			continue
		}
		clauses = append(clauses, "(eoj = ? AND epc = ?)")
		args = append(args, eoj, epc)
	}
	if len(clauses) > 0 {
		filter = " WHERE " + strings.Join(clauses, " OR ")
	}
	rows, err := s.db.Query(`
		SELECT time, eoj, epc, name, raw, unsigned_value, signed_value, float_value, description
		FROM properties WHERE id IN (
			SELECT (SELECT p.id FROM properties p WHERE p.eoj = k.eoj AND p.epc = k.epc ORDER BY p.time DESC, p.id DESC LIMIT 1)
			FROM (SELECT DISTINCT eoj, epc FROM properties`+filter+`) k
		) ORDER BY eoj, epc`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProperties(rows)
}

func scanProperties(rows *sql.Rows) ([]SampleProperty, error) {
	props := []SampleProperty{}
	for rows.Next() {
		var prop SampleProperty
		var rawTime string
		var unsigned sql.NullInt64
		var signed sql.NullInt64
		var float sql.NullFloat64
		err := rows.Scan(&rawTime, &prop.EOJ, &prop.EPC, &prop.Name, &prop.Raw, &unsigned, &signed, &float, &prop.Description)
		if err != nil {
			return nil, err
		}
		prop.Fixed = isFixedProperty(prop.EOJ, prop.EPC)
		prop.Time, err = time.Parse(time.RFC3339Nano, rawTime)
		if err != nil {
			return nil, err
		}
		if unsigned.Valid {
			v := uint64(unsigned.Int64)
			prop.Unsigned = &v
		}
		if signed.Valid {
			v := signed.Int64
			prop.Signed = &v
		}
		if float.Valid {
			v := float.Float64
			prop.Float = &v
		}
		props = append(props, prop)
	}
	return props, rows.Err()
}

func (s *Store) PropertyStats(limit int) ([]PropertyStat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.pending) > 0 {
		samples, err := s.readCombined(limit, true, "", nil)
		if err != nil {
			return nil, err
		}
		props := []SampleProperty{}
		for _, sample := range samples {
			props = append(props, sampleProperties(sample, nil)...)
		}
		return propertyStats(props), nil
	}

	rows, err := s.db.Query(`
		WITH latest_samples AS (
			SELECT id
			FROM samples
			ORDER BY time DESC, id DESC
			LIMIT ?
		)
		SELECT time, eoj, epc, name, raw, float_value, description
		FROM properties
		WHERE sample_id IN (SELECT id FROM latest_samples)
		ORDER BY time ASC, eoj ASC, epc ASC
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	props := []SampleProperty{}
	for rows.Next() {
		var rawTime string
		var p SampleProperty
		var value sql.NullFloat64
		if err := rows.Scan(&rawTime, &p.EOJ, &p.EPC, &p.Name, &p.Raw, &value, &p.Description); err != nil {
			return nil, err
		}
		p.Time, err = time.Parse(time.RFC3339Nano, rawTime)
		if err != nil {
			return nil, err
		}
		if value.Valid {
			p.Float = floatPtr(value.Float64)
		}
		props = append(props, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return propertyStats(props), nil
}

func propertyStats(props []SampleProperty) []PropertyStat {
	byKey := map[string]*PropertyStat{}
	for _, p := range props {
		eoj, epc, name, raw, description, t := p.EOJ, p.EPC, p.Name, p.Raw, p.Description, p.Time
		key := eoj + ":" + epc
		stat, ok := byKey[key]
		if !ok {
			stat = &PropertyStat{
				EOJ:               eoj,
				EPC:               epc,
				Name:              name,
				FirstTime:         t,
				LatestTime:        t,
				LatestRaw:         raw,
				LatestDescription: description,
			}
			byKey[key] = stat
		}
		stat.Count++
		stat.Name = name
		stat.LatestTime = t
		stat.LatestRaw = raw
		stat.LatestDescription = description
		if p.Float == nil {
			continue
		}
		v := *p.Float
		if stat.First == nil {
			stat.First = floatPtr(v)
		}
		stat.Latest = floatPtr(v)
		if stat.Min == nil || v < *stat.Min {
			stat.Min = floatPtr(v)
		}
		if stat.Max == nil || v > *stat.Max {
			stat.Max = floatPtr(v)
		}
		if stat.First != nil && stat.Latest != nil {
			stat.Delta = floatPtr(*stat.Latest - *stat.First)
		}
		if stat.Min != nil && stat.Max != nil {
			stat.Range = floatPtr(*stat.Max - *stat.Min)
		}
	}
	stats := make([]PropertyStat, 0, len(byKey))
	for _, stat := range byKey {
		stats = append(stats, *stat)
	}
	sort.Slice(stats, func(i, j int) bool {
		leftRange := valueOf(stats[i].Range)
		rightRange := valueOf(stats[j].Range)
		if leftRange != rightRange {
			return leftRange > rightRange
		}
		if stats[i].EOJ != stats[j].EOJ {
			return stats[i].EOJ < stats[j].EOJ
		}
		return stats[i].EPC < stats[j].EPC
	})
	return stats
}

func nullUint64(v *uint64) any {
	if v == nil {
		return nil
	}
	return int64(*v)
}

func nullInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullFloat64(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func valueOf(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func (s *Store) FixedReads(target string) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT property_key FROM fixed_reads WHERE target = ?`, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out[key] = true
	}
	return out, rows.Err()
}

func (s *Store) RecordFixedReads(target string, props []SampleProperty) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fixed := []SampleProperty{}
	for _, p := range props {
		if isFixedProperty(p.EOJ, p.EPC) && p.Raw != "" {
			fixed = append(fixed, p)
		}
	}
	if len(fixed) == 0 {
		return nil
	}
	if s.buffered {
		byKey := map[string]SampleProperty{}
		for _, p := range s.fixedPending[target] {
			byKey[p.EOJ+":"+p.EPC] = p
		}
		for _, p := range fixed {
			byKey[p.EOJ+":"+p.EPC] = p
		}
		merged := make([]SampleProperty, 0, len(byKey))
		for _, p := range byKey {
			merged = append(merged, p)
		}
		s.fixedPending[target] = merged
		return nil
	}
	return s.recordFixedReads(target, fixed)
}
func (s *Store) recordFixedReads(target string, props []SampleProperty) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range props {
		if !isFixedProperty(p.EOJ, p.EPC) || p.Raw == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO fixed_reads(target,property_key) VALUES (?,?)`, target, p.EOJ+":"+p.EPC); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RejectedFixed(target string) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT property_key FROM rejected_fixed WHERE target=?`, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out[key] = true
	}
	return out, rows.Err()
}
func (s *Store) SaveRejectedFixed(target string, rejected map[string]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.buffered {
		snapshot := map[string]bool{}
		for key, active := range rejected {
			if active {
				snapshot[key] = true
			}
		}
		s.rejectedPending[target] = snapshot
		return nil
	}
	return s.saveRejectedFixed(target, rejected)
}
func (s *Store) saveRejectedFixed(target string, rejected map[string]bool) error {
	existing, err := s.RejectedFixed(target)
	if err != nil {
		return err
	}
	same := true
	for key := range existing {
		if !rejected[key] {
			same = false
		}
	}
	for key, active := range rejected {
		if active && !existing[key] {
			same = false
		}
	}
	if same {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM rejected_fixed WHERE target=?`, target); err != nil {
		return err
	}
	for key, active := range rejected {
		if !active {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO rejected_fixed(target,property_key) VALUES (?,?)`, target, key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HasSamplesBetween includes unsaved readings and stops at the first stored match.
func (s *Store) HasSamplesBetween(start, end time.Time) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sample := range s.pending {
		if !sample.Time.Before(start) && sample.Time.Before(end) {
			return true, nil
		}
	}
	var found bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM samples WHERE julianday(time)>=julianday(?) AND julianday(time)<julianday(?))`, start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)).Scan(&found)
	return found, err
}
