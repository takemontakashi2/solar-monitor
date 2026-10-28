package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

var version = "0.9.13"

func versionInfo() string {
	info := "solar-monitor " + version
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			if setting.Key == "vcs.revision" {
				info += " (" + setting.Value + ")"
			}
			if setting.Key == "vcs.modified" && setting.Value == "true" {
				info += " dirty"
			}
		}
	}
	return info
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "scan" {
		runScan(os.Args[2:])
		return
	}
	if err := runServer(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func runServer(args []string) error {
	fs := flag.NewFlagSet("solar-monitor", flag.ExitOnError)
	debugLogging := fs.Bool("debug", false, "show detailed collection diagnostics")
	var showVersion bool
	fs.BoolVar(&showVersion, "v", false, "show version and exit")
	fs.BoolVar(&showVersion, "version", false, "show version and exit")
	addr := fs.String("addr", ":8080", "HTTP listen address")
	data := fs.String("data", "data/solar.db", "SQLite data file")
	configPath := fs.String("config", "data/config.json", "config file")
	flushInterval := fs.Duration("flush-interval", 0, "batch database writes at this interval (0 saves each collection)")
	interval := fs.Duration("interval", 60*time.Second, "pause between collections")
	sourceKind := fs.String("source", "echonet", "source type: echonet, mock, or http-json")
	sourceURL := fs.String("source-url", "", "HTTP JSON source URL")
	echonetAddr := fs.String("echonet-addr", "", "ECHONET Lite target IPv4 address")
	echonetFields := fs.String("echonet-fields", "pv=027901:e0,today=027901:e1:0.001", "ECHONET fields: name=EOJ:EPC[:scale], comma-separated")
	fullScan := fs.Bool("full-scan", false, "run a full ECHONET property scan on every collection")
	_ = fs.Parse(args)
	if showVersion {
		fmt.Println(versionInfo())
		return nil
	}

	if *flushInterval < 0 || *interval <= 0 {
		return errors.New("flush-interval must be non-negative and interval must be positive")
	}
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		return err
	}

	store, err := NewStore(*data)
	if err != nil {
		return err
	}
	defer store.Close()

	source, err := buildSource(*sourceKind, *sourceURL, *echonetAddr, *echonetFields, *fullScan, cfg, func(updated Config) error {
		cfg = updated
		return SaveConfig(*configPath, updated)
	})
	if err != nil {
		return err
	}

	if echonet, ok := source.(*ECHONETSource); ok {
		echonet.debug = *debugLogging
		echonet.rejectedFixed, err = store.RejectedFixed(echonet.addr)
		if err != nil {
			return err
		}
		echonet.fixedRead, err = store.FixedReads(echonet.addr)
		if err != nil {
			return err
		}
	}
	if *flushInterval > 0 {
		store.EnableBuffering()
	}
	app := NewApp(store, source)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	collectorDone := make(chan struct{})
	go func() { defer close(collectorDone); app.RunCollector(ctx, *interval) }()
	flusherDone := make(chan struct{})
	go func() {
		defer close(flusherDone)
		if *flushInterval == 0 {
			return
		}
		ticker := time.NewTicker(*flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := store.Flush(); err != nil {
					log.Printf("flush failed; retaining unsaved data: %v", err)
				}
			}
		}
	}()
	server := &http.Server{Addr: *addr, Handler: app.Routes(), ReadHeaderTimeout: 10 * time.Second}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.ListenAndServe() }()
	listenAddress := *addr
	if strings.HasPrefix(listenAddress, ":") {
		listenAddress = "localhost" + listenAddress
	}
	log.Printf("listening on http://%s", listenAddress)
	var serverErr error
	select {
	case <-ctx.Done():
	case err := <-serverDone:
		if !errors.Is(err, http.ErrServerClosed) {
			serverErr = fmt.Errorf("HTTP server failed: %w", err)
		}
		stop()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown failed: %v", err)
	}
	<-collectorDone
	<-flusherDone
	if err := store.Flush(); err != nil {
		return fmt.Errorf("final save failed; unsaved data will be lost: %w", err)
	} else if *flushInterval > 0 {
		log.Print("remaining data saved")
	}
	return serverErr
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

func buildSource(kind, sourceURL, echonetAddr, echonetFields string, fullScan bool, cfg Config, saveConfig func(Config) error) (Source, error) {
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
			echonetAddr = cfg.ECHONETAddr
		}
		if echonetAddr == "" {
			discovered, err := discoverECHONETMonitorAddr(context.Background(), 5*time.Second)
			if err != nil {
				return nil, err
			}
			echonetAddr = discovered
			log.Printf("discovered ECHONET Lite node: %s", echonetAddr)
		}
		if cfg.ECHONETAddr != echonetAddr && saveConfig != nil {
			cfg.ECHONETAddr = echonetAddr
			if err := saveConfig(cfg); err != nil {
				return nil, err
			}
			log.Printf("saved ECHONET Lite node to config: %s", echonetAddr)
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

func discoverECHONETMonitorAddr(parent context.Context, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	nodes, err := DiscoverECHONETNodes(ctx)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", errors.New("no ECHONET Lite nodes found; use -echonet-addr to specify one")
	}
	for _, node := range nodes {
		objects, err := NewECHONETScanner(node.Addr).readInstanceList(ctx)
		if err != nil {
			continue
		}
		if hasMonitorObject(objects) {
			return node.Addr, nil
		}
	}
	return nodes[0].Addr, nil
}

func hasMonitorObject(objects []ECHONETObject) bool {
	for _, object := range objects {
		class := uint16(object.EOJ[0])<<8 | uint16(object.EOJ[1])
		switch class {
		case 0x0279, 0x027d:
			return true
		}
	}
	return false
}
