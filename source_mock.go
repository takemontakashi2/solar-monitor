package main

import (
	"context"
	"math"
	"math/rand"
	"time"
)

type MockSource struct {
	rand *rand.Rand
}

func NewMockSource() *MockSource {
	return &MockSource{rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (m *MockSource) Read(_ context.Context) (Sample, error) {
	now := time.Now()
	hour := float64(now.Hour()) + float64(now.Minute())/60
	daylight := math.Sin((hour - 6) / 12 * math.Pi)
	if daylight < 0 {
		daylight = 0
	}
	pv := daylight*4200 + m.rand.Float64()*180
	load := 450 + m.rand.Float64()*900
	grid := load - pv
	today := daylight * 24
	return Sample{Time: now, PVWatts: floatPtr(pv), LoadWatts: floatPtr(load), GridWatts: floatPtr(grid), TodayKWh: floatPtr(today), Source: "mock"}, nil
}
