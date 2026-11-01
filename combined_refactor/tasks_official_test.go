package main

import (
	"context"
	"testing"
)

func TestCancelledDetailedRetestDoesNotReplayPreviousResults(t *testing.T) {
	session := &appSession{
		emit:        func(string, interface{}) {},
		scanResults: []ScanResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG"}},
		testResults: []TestResult{{IP: "192.0.2.9", Port: 443, DataCenter: "NRT"}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runDetailedTest(ctx, session, "HKG", 443, 500, scanModeTCPing)
	if len(session.testResults) != 0 {
		t.Fatalf("cancelled retest retained stale previous results: %#v", session.testResults)
	}
}

func TestOfficialAutoFlowOwnership(t *testing.T) {
	ctx := context.Background()
	s := &appSession{}
	params := startTaskRequest{AutoSpeed: true, OfficialSpeedLimit: 5}
	if officialAutoFollowupAllowed(ctx, s, params) {
		t.Fatal("legacy foreground client still owns its followup")
	}
	s.backgroundTask = true
	if !officialAutoFollowupAllowed(ctx, s, params) {
		t.Fatal("legacy background auto flow must continue")
	}
	s.backgroundTask = false
	params.ServerAutoFlow = true
	params.OfficialSpeedLimit = 0
	if !officialAutoFollowupAllowed(ctx, s, params) {
		t.Fatal("new explicit flow with no speed must still detail-test")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if officialAutoFollowupAllowed(cancelled, s, params) {
		t.Fatal("cancelled flow continued")
	}
}

func TestOfficialAutoDataCenterSelection(t *testing.T) {
	rows := []ScanResult{{DataCenter: "HKG", TCPDuration: 20}, {DataCenter: "NRT", TCPDuration: 10}}
	if dc, err := officialAutoDataCenter(rows, ""); err != nil || dc != "NRT" {
		t.Fatalf("automatic choice=%q error=%v", dc, err)
	}
	if dc, err := officialAutoDataCenter(rows, "hkg"); err != nil || dc != "HKG" {
		t.Fatalf("specified choice=%q error=%v", dc, err)
	}
	if _, err := officialAutoDataCenter(rows, "XXX"); err == nil {
		t.Fatal("missing specified DC accepted")
	}
}
