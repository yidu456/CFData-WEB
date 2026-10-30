package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func validWebBatchRequest() startWebBatchRequest {
	return startWebBatchRequest{Datacenters: []string{"fra", " nrt "}, MaxLatencyMS: 500, MinSpeedMB: 1, Limit: 10, Port: 443, URL: "speed.cloudflare.com/__down", ScanMode: scanModeTCPing}
}

func TestWebBatchRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*startWebBatchRequest)
	}{
		{"no DC", func(r *startWebBatchRequest) { r.Datacenters = nil }},
		{"invalid DC", func(r *startWebBatchRequest) { r.Datacenters = []string{"FRA<script>"} }},
		{"delay", func(r *startWebBatchRequest) { r.MaxLatencyMS = 0 }},
		{"speed", func(r *startWebBatchRequest) { r.MinSpeedMB = math.NaN() }},
		{"limit", func(r *startWebBatchRequest) { r.Limit = 0 }},
		{"port", func(r *startWebBatchRequest) { r.Port = 65536 }},
		{"negative port", func(r *startWebBatchRequest) { r.Port = -1 }},
		{"mode", func(r *startWebBatchRequest) { r.ScanMode = "invalid" }},
		{"URL", func(r *startWebBatchRequest) { r.URL = "https://" }},
		{"URL credentials", func(r *startWebBatchRequest) { r.URL = "https://key:secret@example.com/" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := validWebBatchRequest()
			tc.change(&r)
			if _, err := normalizeWebBatchRequest(r); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
	r, err := normalizeWebBatchRequest(validWebBatchRequest())
	if err != nil || strings.Join(r.Datacenters, ",") != "FRA,NRT" {
		t.Fatalf("normalization: %+v %v", r, err)
	}
}

func batchTestDependencies() webBatchDependencies {
	return webBatchDependencies{
		latency: func(ctx context.Context, ip string, port, delay int, mode string) (*TestResult, string, string) {
			return &TestResult{IP: ip, Port: port, AvgLatency: 100 * time.Millisecond, MinLatency: 90 * time.Millisecond, MaxLatency: 110 * time.Millisecond}, "", ""
		},
		speed: func(context.Context, string, int, string) (float64, string) { return 5, "" },
		pause: func(context.Context) bool { return true },
	}
}

func TestWebBatchMeasuresAllCandidatesBeforeFinalLimit(t *testing.T) {
	s := &appSession{}
	for i := 1; i <= 14; i++ {
		dc := "FRA"
		if i%2 == 0 {
			dc = "NRT"
		}
		s.scanResults = append(s.scanResults, ScanResult{IP: fmt.Sprintf("192.0.2.%d", i), DataCenter: dc, DCCountry: "XX", Region: "region", City: "city"})
	}
	// Duplicate addresses must not cost another measurement, or inflate qualified totals.
	s.scanResults = append(s.scanResults, s.scanResults[0], ScanResult{IP: "198.51.100.1", DataCenter: "LHR"})
	deps := batchTestDependencies()
	deps.speed = func(_ context.Context, ip string, _ int, _ string) (float64, string) {
		var i int
		fmt.Sscanf(ip, "192.0.2.%d", &i)
		return float64(i) + 0.004, ""
	}
	var events []string
	var mu sync.Mutex
	s.emit = func(kind string, _ interface{}) { mu.Lock(); events = append(events, kind); mu.Unlock() }
	if err := runWebBatch(context.Background(), s, validWebBatchRequest(), deps); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.completedWebBatchResults(0)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.CandidateCount != 14 || snapshot.MeasuredCount != 14 || snapshot.QualifiedCount != 14 || len(snapshot.Results) != 10 {
		t.Fatalf("totals: %+v", snapshot)
	}
	if snapshot.Results[0].IP != "192.0.2.14" || snapshot.Results[9].IP != "192.0.2.5" {
		t.Fatalf("final ranking: %+v", snapshot.Results)
	}
	for _, r := range snapshot.Results {
		if r.DCCountry != "XX" || r.City != "city" || r.Region != "region" || r.Port != 443 || !r.SpeedMeasured {
			t.Fatalf("metadata lost: %+v", r)
		}
	}
	if snapshot.Results[0].MeasuredSpeedMB != 14.004 {
		t.Fatal("precise measurement replaced by rounded display")
	}
	if events[len(events)-1] != "web_batch_complete" {
		t.Fatalf("missing final event: %v", events)
	}
	snapshot.Results[0].IP = "203.0.113.1"
	snapshot.Policy.Datacenters[0] = "LHR"
	again, _ := s.completedWebBatchResults(snapshot.ID)
	if again.Results[0].IP != "192.0.2.14" || again.Policy.Datacenters[0] != "FRA" {
		t.Fatal("accessor leaked mutable authority")
	}
	if _, err := s.completedWebBatchResults(snapshot.ID + 1); err == nil {
		t.Fatal("stale batch ID accepted")
	}
}

func TestWebBatchThresholdsMissingDatacenterAndFailures(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{
		{IP: "192.0.2.1", DataCenter: "NRT"}, {IP: "192.0.2.2", DataCenter: "NRT"},
		{IP: "192.0.2.3", DataCenter: "NRT"}, {IP: "192.0.2.4", DataCenter: "NRT"},
	}}
	deps := batchTestDependencies()
	latency := deps.latency
	deps.latency = func(ctx context.Context, ip string, port, delay int, mode string) (*TestResult, string, string) {
		r, _, _ := latency(ctx, ip, port, delay, mode)
		if ip == "192.0.2.2" {
			r.AvgLatency = 500*time.Millisecond + time.Nanosecond
		}
		return r, "", ""
	}
	deps.speed = func(_ context.Context, ip string, _ int, _ string) (float64, string) {
		if ip == "192.0.2.3" {
			return 0.999, ""
		}
		if ip == "192.0.2.4" {
			return 999, "measurement failed"
		}
		return 1, ""
	}
	if err := runWebBatch(context.Background(), s, validWebBatchRequest(), deps); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.completedWebBatchResults(0)
	if err != nil || len(snapshot.Results) != 1 || snapshot.MeasuredCount != 3 || snapshot.QualifiedCount != 1 {
		t.Fatalf("thresholds: %+v %v", snapshot, err)
	}
	if strings.Join(snapshot.MissingDatacenters, ",") != "FRA" {
		t.Fatalf("missing DCs: %v", snapshot.MissingDatacenters)
	}
}

func TestWebBatchIncompleteTasksNeverPublish(t *testing.T) {
	for _, mode := range []string{"cancel", "panic", "no candidates", "no qualified", "speed failures", "incomplete pause"} {
		t.Run(mode, func(t *testing.T) {
			s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.1", DataCenter: "FRA"}}}
			deps := batchTestDependencies()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "cancel":
				deps.speed = func(context.Context, string, int, string) (float64, string) { cancel(); return 5, "" }
			case "panic":
				deps.speed = func(context.Context, string, int, string) (float64, string) { panic("failed stage") }
			case "no candidates":
				s.scanResults[0].DataCenter = "LHR"
			case "no qualified":
				deps.speed = func(context.Context, string, int, string) (float64, string) { return 0.1, "" }
			case "speed failures":
				deps.speed = func(context.Context, string, int, string) (float64, string) { return 50, "failed" }
			case "incomplete pause":
				s.scanResults = append(s.scanResults, ScanResult{IP: "192.0.2.2", DataCenter: "FRA"})
				deps.pause = func(context.Context) bool { return false }
			}
			_ = runWebBatch(ctx, s, validWebBatchRequest(), deps)
			if _, err := s.completedWebBatchResults(0); err == nil {
				t.Fatal("incomplete/empty task publishable")
			}
		})
	}
}

func TestWebBatchSnapshotInvalidatedOnlyByAcceptedTasksAndCancellation(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.1", DataCenter: "FRA"}}}
	if err := runWebBatch(context.Background(), s, validWebBatchRequest(), batchTestDependencies()); err != nil {
		t.Fatal(err)
	}
	// Rendering/status events must not erase a verified snapshot.
	s.sendWSMessage("log", "rendering")
	if _, err := s.completedWebBatchResults(0); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !s.beginTask(cancel, "new task", "official", nil) {
		t.Fatal("could not begin task")
	}
	defer s.endTask()
	if _, err := s.completedWebBatchResults(0); err == nil {
		t.Fatal("new accepted task retained publication authority")
	}
	s.stopTask()
	if ctx.Err() == nil {
		t.Fatal("cancel not delivered")
	}
}

func TestOfficialNumericSpeedMeasurementIsNeverInferredFromDisplay(t *testing.T) {
	r := TestResult{Speed: "100MB/s", MeasuredSpeedMB: 100, SpeedMeasured: true}
	applyOfficialSpeedMeasurement(&r, 999, "failed")
	if r.SpeedMeasured || r.MeasuredSpeedMB != 0 || r.Speed != "failed" {
		t.Fatalf("stale numeric result retained: %+v", r)
	}
	applyOfficialSpeedMeasurement(&r, 1.23456, "")
	if !r.SpeedMeasured || r.MeasuredSpeedMB != 1.23456 || r.Speed != "1.23MB/s" {
		t.Fatalf("precise numeric result: %+v", r)
	}
}

func TestSingleIPSpeedRetainsOnlySuccessfulNumericMeasurement(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusTooManyRequests} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(strings.Repeat("x", 32768)))
			}))
			defer server.Close()
			host, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
			port, _ := strconv.Atoi(portText)
			s := &appSession{testResults: []TestResult{{IP: host, SpeedMeasured: true, MeasuredSpeedMB: 999}}}
			runSpeedTest(context.Background(), s, host, port, server.URL)
			r := s.testResults[0]
			if status == http.StatusOK {
				if !r.SpeedMeasured || r.MeasuredSpeedMB <= 0 || r.MeasuredSpeedMB == 999 {
					t.Fatalf("successful numeric measurement not retained: %+v", r)
				}
			} else if r.SpeedMeasured || r.MeasuredSpeedMB != 0 {
				t.Fatalf("failed measurement retained stale number: %+v", r)
			}
		})
	}
}

func TestWebBatchRejectsZeroLatencyBeforeSpeedMeasurement(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.1", DataCenter: "FRA"}}}
	deps := batchTestDependencies()
	deps.latency = func(context.Context, string, int, int, string) (*TestResult, string, string) {
		return &TestResult{AvgLatency: 0}, "", ""
	}
	if err := runWebBatch(context.Background(), s, validWebBatchRequest(), deps); err != nil {
		t.Fatal(err)
	}
	if s.webBatch.MeasuredCount != 0 {
		t.Fatal("invalid zero latency was speed tested")
	}
}

func TestBrokenSpeedResponseCannotProduceSuccessfulNumericMeasurement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "65536")
		_, _ = w.Write([]byte(strings.Repeat("x", 32768)))
	}))
	defer server.Close()
	host, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	if speed, failure := runWindowedSpeedTest(context.Background(), host, port, server.URL); failure == "" || speed != 0 {
		t.Fatalf("broken transfer accepted: speed=%v failure=%q", speed, failure)
	}
}

func TestWebBatchFailureClearsPreviousDetailedResults(t *testing.T) {
	s := &appSession{testResults: []TestResult{{IP: "192.0.2.99", SpeedMeasured: true, MeasuredSpeedMB: 10}}}
	if err := runWebBatch(context.Background(), s, validWebBatchRequest(), batchTestDependencies()); err == nil {
		t.Fatal("no candidates accepted")
	}
	if len(s.testResults) != 0 {
		t.Fatal("old results survive failed batch and could be replayed")
	}
}

func TestCompletedWebBatchReplayPreservesBackendSnapshot(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.1", DataCenter: "FRA"}}}
	if err := runWebBatch(context.Background(), s, validWebBatchRequest(), batchTestDependencies()); err != nil {
		t.Fatal(err)
	}
	var events []string
	s.emit = func(kind string, _ interface{}) { events = append(events, kind) }
	if !s.replayCompletedWebBatch() {
		t.Fatal("completed batch cannot be replayed")
	}
	if strings.Join(events, ",") != "web_batch_started,web_batch_complete,task_complete" {
		t.Fatalf("legacy callbacks or missing replay: %v", events)
	}
	if _, err := s.completedWebBatchResults(0); err != nil {
		t.Fatal("replay erased publication authority")
	}
	s.webBatch = nil
	if s.replayCompletedWebBatch() {
		t.Fatal("failed batch replayed")
	}
}

func TestWebBatchAccessorWaitsForTaskLifecycleCompletion(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.1", DataCenter: "FRA"}}}
	s.emit = func(kind string, _ interface{}) {
		if kind == "web_batch_complete" {
			if _, err := s.completedWebBatchResults(0); err == nil {
				t.Error("publishable before task teardown")
			}
		}
	}
	if err := s.runTaskSync(func(ctx context.Context, s *appSession) {
		if err := runWebBatch(ctx, s, validWebBatchRequest(), batchTestDependencies()); err != nil {
			t.Error(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.completedWebBatchResults(0); err != nil {
		t.Fatal(err)
	}
}

func TestWebBatchDrainsFinalLatencyProgressBeforeTeardown(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancelled=%t", cancelled), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				entered := make(chan struct{})
				release := make(chan struct{})
				done := make(chan error, 1)
				s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.1", DataCenter: "NRT"}}}
				deps := batchTestDependencies()
				deps.latency = func(context.Context, string, int, int, string) (*TestResult, string, string) {
					if cancelled {
						cancel()
					}
					return nil, "latency failed", ""
				}
				s.emit = func(kind string, data interface{}) {
					if kind != "web_batch_progress" {
						return
					}
					p := data.(map[string]interface{})
					if p["phase"] == "latency" && p["current"] == p["total"] {
						close(entered)
						<-release
					}
				}
				go func() { done <- runWebBatch(ctx, s, validWebBatchRequest(), deps) }()
				<-entered
				synctest.Wait()
				select {
				case <-done:
					close(release)
					t.Fatal("batch completed/cancelled before final latency progress drained")
				default:
				}
				close(release)
				err := <-done
				if cancelled != (err != nil) {
					t.Fatalf("unexpected task outcome: %v", err)
				}
			})
		})
	}
}

func TestWebBatchRealBackgroundReducerTracksLifecycle(t *testing.T) {
	s := &appSession{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !s.beginTask(cancel, "multi DC", "official", nil) {
		t.Fatal("task rejected")
	}
	defer s.endTask()
	if !s.enableBackgroundTask() {
		t.Fatal("background mode rejected")
	}
	defer func() { s.endTask(); clearCompletedBackgroundTaskSession() }()
	summary := webBatchSnapshot{ID: 1, Policy: officialSelectionPolicy{Datacenters: []string{"NRT", "LHR"}, Limit: 10}, CandidateCount: 3, MissingDatacenters: []string{"LHR"}}
	s.sendWSMessage("web_batch_started", summary)
	status := s.backgroundSummary()
	if status.ScanTotal != 3 || status.DCCount != 2 || !strings.Contains(status.Message, "LHR") {
		t.Fatalf("started state missing: %+v", status)
	}
	s.sendWSMessage("web_batch_result", map[string]interface{}{"result": TestResult{IP: "192.0.2.1"}, "qualified": false})
	s.sendWSMessage("web_batch_result", map[string]interface{}{"result": TestResult{IP: "192.0.2.2"}, "qualified": false})
	s.sendWSMessage("web_batch_progress", map[string]interface{}{"phase": "latency", "current": 3, "total": 3})
	status = s.backgroundSummary()
	if status.ScanSuccess != 2 || status.ScanFailed != 1 || status.TestCount != 2 || status.Percent != 100 {
		t.Fatalf("latency state missing: %+v", status)
	}
	s.sendWSMessage("web_batch_progress", map[string]interface{}{"phase": "speed", "current": 0, "total": 2, "qualified": 0})
	s.sendWSMessage("web_batch_result", map[string]interface{}{"result": TestResult{IP: "192.0.2.1", Speed: "1.00MB/s", SpeedMeasured: true, MeasuredSpeedMB: 0.999}, "qualified": false})
	s.sendWSMessage("web_batch_progress", map[string]interface{}{"phase": "speed", "current": 1, "total": 2, "qualified": 0})
	status = s.backgroundSummary()
	if status.Phase != "测速中" || status.Percent != 50 || status.SpeedCount != 1 || status.SpeedSuccess != 1 || status.SpeedQualified != 0 {
		t.Fatalf("speed state missing or rounded qualification used: %+v", status)
	}
	s.sendWSMessage("web_batch_result", map[string]interface{}{"result": TestResult{IP: "192.0.2.2", Speed: "failed"}, "qualified": false})
	s.sendWSMessage("web_batch_progress", map[string]interface{}{"phase": "speed", "current": 2, "total": 2, "qualified": 0})
	status = s.backgroundSummary()
	if status.SpeedFailed != 1 || status.SpeedCount != 2 {
		t.Fatalf("failure counts missing: %+v", status)
	}
	summary.MeasuredCount = 2
	summary.QualifiedCount = 0
	summary.Results = []TestResult{}
	summary.CompletedAt = time.Now()
	s.sendWSMessage("web_batch_complete", summary)
	s.endTask()
	s.sendWSMessage("task_complete", nil)
	status = s.backgroundSummary()
	if status.Running || status.ResultCount != 0 || status.SpeedCount != 2 || status.SpeedSuccess != 1 || status.SpeedFailed != 1 || status.SpeedQualified != 0 || !strings.Contains(status.Message, "不可发布") {
		t.Fatalf("completed empty/failure totals lost: %+v", status)
	}
	if ctx.Err() != nil {
		t.Fatal("task unexpectedly cancelled")
	}
}

func TestWebBatchBackgroundFailureSurvivesGenericCompletion(t *testing.T) {
	for _, state := range []string{"failed", "cancelled"} {
		t.Run(state, func(t *testing.T) {
			s := &appSession{backgroundTask: true, isTaskRunning: true}
			s.sendWSMessage("web_batch_started", webBatchSnapshot{ID: 1, CandidateCount: 2})
			s.sendWSMessage("web_batch_failed", map[string]interface{}{"status": state, "message": "stage did not finish"})
			s.isTaskRunning = false
			s.sendWSMessage("task_complete", nil)
			status := s.backgroundSummary()
			if status.Phase == "完成" || !strings.Contains(status.Message, "stage did not finish") {
				t.Fatalf("failure overwritten: %+v", status)
			}
		})
	}
}

func TestCompletedEmptyWebBatchReplaysDisplayButCannotPublish(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.1", DataCenter: "NRT"}}}
	deps := batchTestDependencies()
	deps.speed = func(context.Context, string, int, string) (float64, string) { return 0.1, "" }
	if err := runWebBatch(context.Background(), s, validWebBatchRequest(), deps); err != nil {
		t.Fatal(err)
	}
	var events []string
	var replay webBatchSnapshot
	s.emit = func(kind string, data interface{}) {
		events = append(events, kind)
		if kind == "web_batch_complete" {
			replay = data.(webBatchSnapshot)
		}
	}
	if !s.replayCompletedWebBatch() {
		t.Fatal("completed empty display snapshot rejected")
	}
	if strings.Join(events, ",") != "web_batch_started,web_batch_complete,task_complete" || replay.MeasuredCount != 1 || replay.QualifiedCount != 0 || len(replay.Results) != 0 || strings.Join(replay.MissingDatacenters, ",") != "FRA" {
		t.Fatalf("empty replay lost summary or used legacy callbacks: %v %+v", events, replay)
	}
	if _, err := s.completedWebBatchResults(replay.ID); err == nil {
		t.Fatal("empty replay granted publishing authority")
	}
}

func TestWebBatchRealBackgroundCompletionRetainsQualifiedAndFinalTotals(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{
		{IP: "192.0.2.1", DataCenter: "NRT"}, {IP: "192.0.2.2", DataCenter: "NRT"}, {IP: "192.0.2.3", DataCenter: "NRT"},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !s.beginTask(cancel, "multi DC", "official", nil) {
		t.Fatal("task rejected")
	}
	defer func() { s.endTask(); clearCompletedBackgroundTaskSession() }()
	if !s.enableBackgroundTask() {
		t.Fatal("background mode rejected")
	}
	r := validWebBatchRequest()
	r.Limit = 1
	deps := batchTestDependencies()
	deps.speed = func(_ context.Context, ip string, _ int, _ string) (float64, string) {
		if ip == "192.0.2.3" {
			return 0.1, ""
		}
		return 5, ""
	}
	if err := runWebBatch(ctx, s, r, deps); err != nil {
		t.Fatal(err)
	}
	s.endTask()
	s.sendWSMessage("task_complete", nil)
	for i := 0; i < 2; i++ {
		status := s.backgroundSummary()
		if status.WebBatchStatus != "completed" || status.Running || status.ScanTotal != 3 || status.SpeedCount != 3 || status.SpeedSuccess != 3 || status.SpeedQualified != 2 || status.ResultCount != 1 || status.Percent != 100 {
			t.Fatalf("completion/replay totals lost: %+v", status)
		}
		if !s.replayCompletedWebBatch() {
			t.Fatal("completed background batch replay rejected")
		}
	}
}

func TestWebBatchLateCancellationOverridesCompletedBackgroundOutcome(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.1", DataCenter: "NRT"}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !s.beginTask(cancel, "multi DC", "official", nil) {
		t.Fatal("task rejected")
	}
	defer func() { s.endTask(); clearCompletedBackgroundTaskSession() }()
	if !s.enableBackgroundTask() {
		t.Fatal("background mode rejected")
	}
	if err := runWebBatch(ctx, s, validWebBatchRequest(), batchTestDependencies()); err != nil {
		t.Fatal(err)
	}
	if status := s.backgroundSummary(); status.WebBatchStatus != "completed" {
		t.Fatalf("batch completion not emitted: %+v", status)
	}
	s.cancelTaskSilently()
	s.endTask()
	s.sendWSMessage("task_stopped", nil)
	status := s.backgroundSummary()
	if status.WebBatchStatus != "cancelled" || status.Phase != "已停止" || status.Running {
		t.Fatalf("late cancellation retained successful outcome: %+v", status)
	}
	if ctx.Err() == nil {
		t.Fatal("context not cancelled")
	}
	if _, err := s.completedWebBatchResults(0); err == nil {
		t.Fatal("late-cancelled batch remains publishable")
	}
}
