package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	data := flag.String("data", "data/solar.db", "SQLite data file")
	interval := flag.Duration("interval", 60*time.Second, "collection interval")
	sourceKind := flag.String("source", "mock", "source type: mock, http-json, or echonet")
	sourceURL := flag.String("source-url", "", "HTTP JSON source URL")
	echonetAddr := flag.String("echonet-addr", "", "ECHONET Lite target IPv4 address")
	echonetFields := flag.String("echonet-fields", "pv=027901:e0,load=028701:e7,grid=028801:e7,today=027901:e1:0.001", "ECHONET fields: name=EOJ:EPC[:scale], comma-separated")
	flag.Parse()

	store, err := NewStore(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	source, err := buildSource(*sourceKind, *sourceURL, *echonetAddr, *echonetFields)
	if err != nil {
		log.Fatal(err)
	}

	app := NewApp(store, source)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.RunCollector(ctx, *interval)

	log.Printf("listening on http://localhost%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, app.Routes()))
}

func buildSource(kind, sourceURL, echonetAddr, echonetFields string) (Source, error) {
	switch kind {
	case "mock":
		return NewMockSource(), nil
	case "http-json":
		if sourceURL == "" {
			return nil, errors.New("-source-url is required for http-json")
		}
		return NewHTTPJSONSource(sourceURL), nil
	case "echonet":
		if echonetAddr == "" {
			return nil, errors.New("-echonet-addr is required for echonet")
		}
		fields, err := parseECHONETFields(echonetFields)
		if err != nil {
			return nil, err
		}
		return NewECHONETSource(echonetAddr, fields), nil
	default:
		return nil, fmt.Errorf("unknown source %q", kind)
	}
}
