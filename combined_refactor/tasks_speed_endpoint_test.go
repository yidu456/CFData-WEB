package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSingleSpeedUpdatesOnlyRequestedEndpointAndIncludesPort(t *testing.T) {
	s := &appSession{testResults: []TestResult{{IP: "192.0.2.1", Port: 443, Speed: "1MB/s", SpeedMeasured: true, MeasuredSpeedMB: 1}, {IP: "192.0.2.1", Port: 8443, Speed: "2MB/s", SpeedMeasured: true, MeasuredSpeedMB: 2}}}
	var event map[string]string
	s.emit = func(kind string, data interface{}) {
		if kind == "speed_test_result" {
			event = data.(map[string]string)
		}
	}
	// Invalid URLs fail before dialing, exercising real mutation without a live scan.
	runSpeedTest(context.Background(), s, "192.0.2.1", 8443, "http://[invalid")
	if s.testResults[0].Speed != "1MB/s" || !s.testResults[0].SpeedMeasured || s.testResults[0].MeasuredSpeedMB != 1 {
		t.Fatal("single speed overwrote another port")
	}
	if !strings.Contains(s.testResults[1].Speed, "URL") || s.testResults[1].SpeedMeasured || event["port"] != "8443" {
		t.Fatalf("selected endpoint/event incorrect: %+v %+v", s.testResults, event)
	}
}

func TestOfficialBatchOverrideCreatesActualEndpointAndPreservesSourcePorts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &appSession{testResults: []TestResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", AvgLatency: time.Millisecond, Speed: "1MB/s", SpeedMeasured: true, MeasuredSpeedMB: 1}, {IP: "192.0.2.1", Port: 8443, DataCenter: "NRT", AvgLatency: 2 * time.Millisecond, Speed: "2MB/s", SpeedMeasured: true, MeasuredSpeedMB: 2}}}
	var event map[string]string
	s.emit = func(kind string, data interface{}) {
		if kind == "speed_test_result" {
			event = data.(map[string]string)
			cancel()
		}
	}
	runOfficialSpeedBatch(ctx, s, 9443, "http://[invalid", 1, 0.1, nil, true)
	if len(s.testResults) != 3 || s.testResults[0].Speed != "1MB/s" || s.testResults[1].Speed != "2MB/s" || s.testResults[2].Port != 9443 || !strings.Contains(s.testResults[2].Speed, "URL") {
		t.Fatalf("override conflated endpoints or skipped new endpoint: %+v", s.testResults)
	}
	if event["port"] != "9443" {
		t.Fatalf("override event mislabeled: %+v", event)
	}
}

func TestOfficialSpeedCoreOverrideDeduplicatesAndDoesNotMutateSources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rows := []TestResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", Speed: "1MB/s"}, {IP: "192.0.2.1", Port: 8443, DataCenter: "NRT", Speed: "2MB/s"}}
	var total int
	updated, _ := runOfficialSpeedTestsCore(ctx, rows, 8443, 1, 0.1, "http://[invalid", func(_, n, _ int, result TestResult) { total = n; cancel() }, nil)
	if total != 1 || len(updated) != 1 || updated[0].Port != 8443 || updated[0].DataCenter != "NRT" {
		t.Fatalf("effective candidates not deduplicated/canonical: %+v total=%d", updated, total)
	}
	if rows[0].Port != 443 || rows[0].Speed != "1MB/s" || rows[1].Speed != "2MB/s" {
		t.Fatalf("core mutated source endpoints: %+v", rows)
	}
}
