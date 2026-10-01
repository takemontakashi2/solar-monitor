package main

import (
	"context"
	"time"
)

type Sample struct {
	Time      time.Time `json:"time"`
	PVWatts   float64   `json:"pv_watts"`
	LoadWatts float64   `json:"load_watts"`
	GridWatts float64   `json:"grid_watts"`
	TodayKWh  float64   `json:"today_kwh"`
	Source    string    `json:"source"`
}

type Source interface {
	Read(ctx context.Context) (Sample, error)
}
