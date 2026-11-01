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
