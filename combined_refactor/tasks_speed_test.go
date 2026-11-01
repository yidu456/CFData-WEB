package main

import (
	"context"
	"testing"
)

func TestSingleSpeedResultPersistsForContinue(t *testing.T) {
	s := &appSession{testResults: []TestResult{{IP: "192.0.2.1", Port: 443}, {IP: "192.0.2.1", Port: 8443}}}
	s.recordOfficialSpeedResult("192.0.2.1", 443, "1.00MB/s")
	if s.testResults[0].Speed != "1.00MB/s" || s.testResults[1].Speed != "" {
		t.Fatalf("speed not persisted to exact endpoint: %#v", s.testResults)
	}
}

func TestFreshCancelledBatchClearsPreviouslyTestedCache(t *testing.T) {
	s := &appSession{emit: func(string, interface{}) {}, testResults: []TestResult{{IP: "192.0.2.1", Port: 443, Speed: "1.00MB/s"}, {IP: "192.0.2.2", Port: 443, Speed: "2.00MB/s"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runOfficialSpeedBatch(ctx, s, 443, "auto", 1, .1, nil, false)
	for _, result := range s.testResults {
		if result.Speed != "" {
			t.Fatalf("fresh cancelled batch retained stale speed: %#v", s.testResults)
		}
	}
}
