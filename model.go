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

type PropertyStat struct {
	EOJ               string    `json:"eoj"`
	EPC               string    `json:"epc"`
	Name              string    `json:"name,omitempty"`
	Count             int       `json:"count"`
	FirstTime         time.Time `json:"first_time"`
	LatestTime        time.Time `json:"latest_time"`
	First             *float64  `json:"first,omitempty"`
	Latest            *float64  `json:"latest,omitempty"`
	Min               *float64  `json:"min,omitempty"`
	Max               *float64  `json:"max,omitempty"`
	Delta             *float64  `json:"delta,omitempty"`
	Range             *float64  `json:"range,omitempty"`
	LatestRaw         string    `json:"latest_raw"`
	LatestDescription string    `json:"latest_description,omitempty"`
}

type Source interface {
	Read(ctx context.Context) (Sample, error)
}

func floatPtr(v float64) *float64 {
	return &v
}
