package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
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
	`)
	return err
}

func (s *Store) Append(sample Sample) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO samples (time, pv_watts, load_watts, grid_watts, today_kwh, source)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		sample.Time.Format(time.RFC3339Nano),
		sample.PVWatts,
		sample.LoadWatts,
		sample.GridWatts,
		sample.TodayKWh,
		sample.Source,
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
	return tx.Commit()
}

func (s *Store) Latest(limit int) ([]Sample, error) {
	rows, err := s.db.Query(`
		SELECT id, time, pv_watts, load_watts, grid_watts, today_kwh, source
		FROM samples
		ORDER BY time DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	out := make([]Sample, 0, limit)
	for rows.Next() {
		var sample Sample
		var id int64
		var rawTime string
		err := rows.Scan(&id, &rawTime, &sample.PVWatts, &sample.LoadWatts, &sample.GridWatts, &sample.TodayKWh, &sample.Source)
		if err != nil {
			return nil, err
		}
		sample.Time, err = time.Parse(time.RFC3339Nano, rawTime)
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
	for i, id := range ids {
		props, err := s.propertiesForSample(id)
		if err != nil {
			return nil, err
		}
		out[i].Properties = props
	}
	return out, nil
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
