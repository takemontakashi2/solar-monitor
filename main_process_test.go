package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestServerProcessHelper(t *testing.T) {
	if os.Getenv("SOLAR_TEST_SERVER") != "1" {
		return
	}
	if err := runServer([]string{"-source", "mock", "-addr", os.Getenv("SOLAR_TEST_ADDR"), "-data", os.Getenv("SOLAR_TEST_DB"), "-config", os.Getenv("SOLAR_TEST_DB") + ".json", "-interval", "20ms", "-flush-interval", os.Getenv("SOLAR_TEST_FLUSH")}); err != nil {
		t.Fatal(err)
	}
}

func TestServerPeriodicAndShutdownFlush(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM integration test")
	}
	for _, interval := range []string{"1h", "100ms"} {
		t.Run(interval, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := listener.Addr().String()
			listener.Close()
			db := filepath.Join(t.TempDir(), "solar.db")
			cmd := exec.Command(os.Args[0], "-test.run=^TestServerProcessHelper$")
			cmd.Env = append(os.Environ(), "SOLAR_TEST_SERVER=1", "SOLAR_TEST_ADDR="+addr, "SOLAR_TEST_DB="+db, "SOLAR_TEST_FLUSH="+interval)
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					cmd.Process.Kill()
					cmd.Wait()
				}
			})
			client := &http.Client{Timeout: time.Second}
			deadline := time.Now().Add(10 * time.Second)
			var visible []Sample
			for time.Now().Before(deadline) {
				res, err := client.Get("http://" + addr + "/api/samples?limit=5000")
				if err == nil {
					json.NewDecoder(res.Body).Decode(&visible)
					res.Body.Close()
					if len(visible) >= 4 {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
			}
			if len(visible) < 4 {
				t.Fatal("no live buffered readings")
			}
			store, err := NewStore(db)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if interval == "1h" {
				if diskCount(t, store) != 0 {
					t.Fatal("flushed too early")
				}
			} else {
				for diskCount(t, store) == 0 && time.Now().Before(deadline) {
					time.Sleep(20 * time.Millisecond)
				}
				if diskCount(t, store) == 0 {
					t.Fatal("periodic flush did not run")
				}
			}
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("shutdown: %v %s", err, output.String())
				}
			case <-time.After(10 * time.Second):
				cmd.Process.Kill()
				<-done
				t.Fatal("shutdown hung")
			}
			if count := diskCount(t, store); count < len(visible) {
				t.Fatal(fmt.Sprintf("lost data: %d stored, %d visible; %s", count, len(visible), output.String()))
			}
			if !bytes.Contains(output.Bytes(), []byte("remaining data saved")) {
				t.Fatalf("final flush not reported: %s", output.String())
			}
		})
	}
}
