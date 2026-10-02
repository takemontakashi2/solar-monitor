package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "scan" {
		runScan(os.Args[2:])
		return
	}
	runServer(os.Args[1:])
}

func runServer(args []string) {
	fs := flag.NewFlagSet("solar-monitor", flag.ExitOnError)
	addr := fs.String("addr", ":8080", "HTTP listen address")
	data := fs.String("data", "data/solar.db", "SQLite data file")
	interval := fs.Duration("interval", 60*time.Second, "collection interval")
	sourceKind := fs.String("source", "mock", "source type: mock, http-json, or echonet")
	sourceURL := fs.String("source-url", "", "HTTP JSON source URL")
	echonetAddr := fs.String("echonet-addr", "", "ECHONET Lite target IPv4 address")
	echonetFields := fs.String("echonet-fields", "pv=027901:e0,today=027901:e1:0.001", "ECHONET fields: name=EOJ:EPC[:scale], comma-separated")
	fullScan := fs.Bool("full-scan", false, "run a full ECHONET property scan on every collection")
	_ = fs.Parse(args)

	store, err := NewStore(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	source, err := buildSource(*sourceKind, *sourceURL, *echonetAddr, *echonetFields, *fullScan)
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

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	echonetAddr := fs.String("echonet-addr", "", "ECHONET Lite target IPv4 address")
	timeout := fs.Duration("timeout", 5*time.Second, "scan timeout")
	raw := fs.Bool("raw", false, "dump raw UDP responses")
	_ = fs.Parse(args)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if *raw {
		if *echonetAddr == "" {
			if err := DumpRawECHONETDiscovery(ctx, os.Stdout); err != nil {
				log.Fatal(err)
			}
			return
		}
		if err := DumpRawECHONETTarget(ctx, os.Stdout, *echonetAddr); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *echonetAddr == "" {
		if err := PrintECHONETDiscovery(ctx, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := NewECHONETScanner(*echonetAddr).Print(ctx, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func buildSource(kind, sourceURL, echonetAddr, echonetFields string, fullScan bool) (Source, error) {
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
			discovered, err := discoverFirstECHONETAddr(context.Background(), 3*time.Second)
			if err != nil {
				return nil, err
			}
			echonetAddr = discovered
		}
		fields, err := parseECHONETFields(echonetFields)
		if err != nil {
			return nil, err
		}
		return NewECHONETSource(echonetAddr, fields, fullScan), nil
	default:
		return nil, fmt.Errorf("unknown source %q", kind)
	}
}

func discoverFirstECHONETAddr(parent context.Context, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	nodes, err := DiscoverECHONETNodes(ctx)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", errors.New("no ECHONET Lite nodes found; use -echonet-addr to specify one")
	}
	return nodes[0].Addr, nil
}
