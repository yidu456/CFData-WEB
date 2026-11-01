package main

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestCLIDatacenters(t *testing.T) {
	got, err := parseCLIDatacenters(" hkg,NRT,HKG ")
	if err != nil || !reflect.DeepEqual(got, []string{"HKG", "NRT"}) {
		t.Fatalf("centers = %v, err = %v", got, err)
	}
	for _, invalid := range []string{"HKG,", ",NRT", "HKG,,NRT", "HK", "HK1", "HKG/NRT"} {
		if _, err := parseCLIDatacenters(invalid); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
	if got, err := parseCLIDatacenters(""); err != nil || len(got) != 0 {
		t.Fatalf("blank selection = %v, %v", got, err)
	}
}

func TestCLIOfficialCombinedMeasurements(t *testing.T) {
	cfg := &cliConfig{dc: "hkg,NRT,HKG,SIN", port: 443, delay: 500, speedLimit: 20, speedMin: 0.1}
	s := &appSession{emit: func(string, interface{}) {}}
	scans, details, speeds := 0, []string{}, 0
	deps := cliOfficialDependencies{
		Scan: func(context.Context, *appSession, int, int, int, int, string) {
			scans++
			s.scanResults = []ScanResult{
				{IP: "192.0.2.1", Port: 443, DataCenter: "HKG"},
				{IP: "192.0.2.2", Port: 443, DataCenter: "NRT"},
				{IP: "192.0.2.3", Port: 443, DataCenter: "LAX"},
			}
		},
		Detail: func(_ context.Context, _ *appSession, dc string, port, delay int, mode string, quiet ...bool) {
			details = append(details, dc)
			if len(quiet) == 0 || !quiet[0] {
				t.Fatal("combined detail passes must preserve earlier centers")
			}
			ip, latency := "192.0.2.1", 30*time.Millisecond
			if dc == "NRT" {
				ip, latency = "192.0.2.2", 10*time.Millisecond
			}
			s.testResults = append(s.testResults, TestResult{IP: ip, Port: port, DataCenter: dc, AvgLatency: latency})
		},
		Speed: func(_ context.Context, _ *appSession, rows []TestResult, port, limit int, min float64) []TestResult {
			speeds++
			if len(details) != 2 || len(rows) != 2 || rows[0].DataCenter != "NRT" || rows[1].DataCenter != "HKG" || limit != 20 {
				t.Fatalf("speed before complete union or wrong global limit: details=%v rows=%v limit=%d", details, rows, limit)
			}
			for i := range rows {
				applyOfficialSpeedMeasurement(&rows[i], float64(i+1), "")
			}
			return rows
		},
	}
	scanRows, rows, err := runOfficialCLIMeasurements(cfg, s, deps)
	if err != nil || scans != 1 || speeds != 1 || len(scanRows) != 3 || len(rows) != 2 || !reflect.DeepEqual(details, []string{"HKG", "NRT"}) {
		t.Fatalf("scans=%d details=%v speeds=%d results=%v err=%v", scans, details, speeds, rows, err)
	}
	if rows[0].MeasuredSpeedMB != 1 || rows[1].MeasuredSpeedMB != 2 {
		t.Fatal("measured results lost")
	}
}

func TestCLIOfficialValidationCancellationAndAutoCenter(t *testing.T) {
	for _, scenario := range []string{"invalid", "missing", "cancel", "automatic", "speed-disabled"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := &cliConfig{dc: "HKG", port: 443, delay: 500, speedLimit: 5}
			s := &appSession{emit: func(string, interface{}) {}}
			if scenario == "invalid" {
				cfg.dc = "HK!"
			}
			if scenario == "missing" {
				cfg.dc = "SIN"
			}
			if scenario == "automatic" {
				cfg.dc = ""
			}
			if scenario == "speed-disabled" {
				cfg.speedLimit = 0
			}
			scans, details, speeds := 0, []string{}, 0
			deps := cliOfficialDependencies{
				Scan: func(ctx context.Context, _ *appSession, a, b, c, d int, mode string) {
					scans++
					s.scanResults = []ScanResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", TCPDuration: 30 * time.Millisecond}, {IP: "192.0.2.2", Port: 443, DataCenter: "NRT", TCPDuration: 10 * time.Millisecond}}
					if scenario == "cancel" {
						s.taskCancel()
					}
				},
				Detail: func(_ context.Context, _ *appSession, dc string, port, delay int, mode string, quiet ...bool) {
					details = append(details, dc)
					s.testResults = append(s.testResults, TestResult{IP: "192.0.2.1", Port: port, DataCenter: dc})
				},
				Speed: func(_ context.Context, _ *appSession, rows []TestResult, port, limit int, min float64) []TestResult {
					speeds++
					return rows
				},
			}
			_, _, err := runOfficialCLIMeasurements(cfg, s, deps)
			switch scenario {
			case "invalid":
				if err == nil || scans != 0 {
					t.Fatalf("invalid input scanned: %v", err)
				}
			case "missing", "cancel":
				if err == nil || len(details) != 0 || speeds != 0 {
					t.Fatalf("unsafe continuation: details=%v speed=%d err=%v", details, speeds, err)
				}
			case "automatic":
				if err != nil || !reflect.DeepEqual(details, []string{"NRT"}) {
					t.Fatalf("auto=%v err=%v", details, err)
				}
			case "speed-disabled":
				if err != nil || speeds != 0 || len(details) != 1 {
					t.Fatalf("disabled speed ran: %v", err)
				}
			}
		})
	}
}
