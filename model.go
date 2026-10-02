package main

import (
	"context"
	"time"
)

type Sample struct {
	Time       time.Time        `json:"time"`
	PVWatts    *float64         `json:"pv_watts"`
	LoadWatts  *float64         `json:"load_watts"`
	GridWatts  *float64         `json:"grid_watts"`
	TodayKWh   *float64         `json:"today_kwh"`
	Source     string           `json:"source"`
	Properties []SampleProperty `json:"properties,omitempty"`
}

type SampleProperty struct {
	Time        time.Time `json:"time,omitempty"`
	EOJ         string    `json:"eoj"`
	EPC         string    `json:"epc"`
	Name        string    `json:"name,omitempty"`
	Raw         string    `json:"raw"`
	Unsigned    *uint64   `json:"unsigned,omitempty"`
	Signed      *int64    `json:"signed,omitempty"`
	Float       *float64  `json:"float,omitempty"`
	Description string    `json:"description,omitempty"`
}

type Source interface {
	Read(ctx context.Context) (Sample, error)
}

func floatPtr(v float64) *float64 {
	return &v
}
