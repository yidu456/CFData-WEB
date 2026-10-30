package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCompletedWebBatchRetestBackgroundReplayRetainsRowProvenance(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.2", DataCenter: "NRT"}}, emit: func(string, interface{}) {}}
	if err := runWebBatch(context.Background(), s, validWebBatchRequest(), batchTestDependencies()); err != nil {
		t.Fatal(err)
	}
	batch, err := s.completedWebBatchResults(0)
	if err != nil {
		t.Fatal(err)
	}
	if !s.beginTask(func() {}, "individual retest", "official", nil) {
		t.Fatal("retest rejected")
	}
	if !s.enableBackgroundTask() {
		t.Fatal("background mode rejected")
	}
	defer func() { s.endTask(); clearCompletedBackgroundTaskSession() }()
	// Invalid URL exercises the real retest's result mutation without any network request.
	runSpeedTest(context.Background(), s, "192.0.2.2", 443, "http://[invalid")
	s.endTask()
	if _, err := s.completedWebBatchResults(batch.ID); err == nil {
		t.Fatal("retest retained publication authority")
	}
	if s.replayCompletedWebBatch() {
		t.Fatal("invalidated batch replayed as completed")
	}
	if currentBackgroundTaskSession() != s {
		t.Fatal("fresh follower cannot find background session")
	}
	events := 0
	s.emit = func(kind string, data interface{}) {
		if kind != "test_result" && kind != "test_complete" {
			t.Fatal("unexpected fallback event: " + kind)
		}
		raw, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		var rows []map[string]interface{}
		if kind == "test_result" {
			var row map[string]interface{}
			_ = json.Unmarshal(raw, &row)
			rows = []map[string]interface{}{row}
		} else {
			_ = json.Unmarshal(raw, &rows)
		}
		if len(rows) != 1 || rows[0]["WebBatchDerived"] != true || rows[0]["IP"] != "192.0.2.2" {
			t.Fatalf("fallback lost batch row provenance: %s", raw)
		}
		events++
	}
	s.replayOfficialTestResults()
	if events != 2 {
		t.Fatalf("inspection replay events: %d", events)
	}
}

func TestGenuineLegacyDetailedTestReplacementClearsReplayProvenance(t *testing.T) {
	s := &appSession{emit: func(string, interface{}) {}}
	if err := runWebBatch(context.Background(), s, validWebBatchRequest(), batchTestDependencies()); err == nil {
		t.Fatal("empty batch fixture should fail")
	}
	// With no candidate IPs this genuine legacy replacement fails safely without network,
	// but cannot retain an old Web-batch collection masquerading as a new legacy test.
	s.testResults = []TestResult{{IP: "192.0.2.2", WebBatchDerived: true}}
	runDetailedTest(context.Background(), s, "NRT", 443, 500, scanModeTCPing)
	if len(s.testResults) != 0 {
		t.Fatal("legacy replacement kept old Web-batch collection")
	}
}
