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
	`)
	return err
}

func (s *Store) Append(sample Sample) error {
	_, err := s.db.Exec(`
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
	return err
}

func (s *Store) Latest(limit int) ([]Sample, error) {
	rows, err := s.db.Query(`
		SELECT time, pv_watts, load_watts, grid_watts, today_kwh, source
		FROM samples
		ORDER BY time DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Sample, 0, limit)
	for rows.Next() {
		var sample Sample
		var rawTime string
		err := rows.Scan(&rawTime, &sample.PVWatts, &sample.LoadWatts, &sample.GridWatts, &sample.TodayKWh, &sample.Source)
		if err != nil {
			return nil, err
		}
		sample.Time, err = time.Parse(time.RFC3339Nano, rawTime)
		if err != nil {
			return nil, err
		}
		out = append(out, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
