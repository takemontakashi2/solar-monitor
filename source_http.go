package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPJSONSource struct {
	url    string
	client *http.Client
}

func NewHTTPJSONSource(url string) *HTTPJSONSource {
	return &HTTPJSONSource{url: url, client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *HTTPJSONSource) Read(ctx context.Context) (Sample, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return Sample{}, err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return Sample{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Sample{}, fmt.Errorf("source returned %s", res.Status)
	}

	var sample Sample
	if err := json.NewDecoder(res.Body).Decode(&sample); err != nil {
		return Sample{}, err
	}
	if sample.Time.IsZero() {
		sample.Time = time.Now()
	}
	if sample.Source == "" {
		sample.Source = "http-json"
	}
	return sample, nil
}
