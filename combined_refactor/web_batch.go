package main

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

type startWebBatchRequest struct {
	Datacenters  []string `json:"datacenters"`
	MaxLatencyMS int      `json:"maxLatencyMS"`
	MinSpeedMB   float64  `json:"minSpeedMB"`
	Limit        int      `json:"limit"`
	Port         int      `json:"port"`
	URL          string   `json:"url"`
	ScanMode     string   `json:"scanMode"`
}

type webBatchSnapshot struct {
	ID                 uint64                  `json:"id"`
	Policy             officialSelectionPolicy `json:"policy"`
	Results            []TestResult            `json:"results"`
	CandidateCount     int                     `json:"candidateCount"`
	MeasuredCount      int                     `json:"measuredCount"`
	QualifiedCount     int                     `json:"qualifiedCount"`
	MissingDatacenters []string                `json:"missingDatacenters"`
	CompletedAt        time.Time               `json:"completedAt"`
}

type webBatchDependencies struct {
	latency func(context.Context, string, int, int, string) (*TestResult, string, string)
	speed   func(context.Context, string, int, string) (float64, string)
	pause   func(context.Context) bool
}

func normalizeWebBatchRequest(r startWebBatchRequest) (startWebBatchRequest, error) {
	policy := officialSelectionPolicy{Datacenters: r.Datacenters, MaxLatencyMS: r.MaxLatencyMS, MinSpeedMB: r.MinSpeedMB, Limit: r.Limit}
	dcs, _, err := validateOfficialSelectionPolicy(policy)
	if err != nil {
		return r, err
	}
	if r.MaxLatencyMS > 60000 || r.Limit > 10000 || len(dcs) > 256 {
		return r, fmt.Errorf("batch settings exceed supported bounds")
	}
	if r.Port < 1 || r.Port > 65535 {
		return r, fmt.Errorf("port must be between 1 and 65535")
	}
	if r.ScanMode == "" {
		r.ScanMode = scanModeTCPing
	}
	if r.ScanMode != scanModeTCPing && r.ScanMode != scanModeHTTPing {
		return r, fmt.Errorf("invalid scan mode")
	}
	if strings.Contains(r.URL, "://") && !strings.HasPrefix(strings.ToLower(r.URL), "http://") && !strings.HasPrefix(strings.ToLower(r.URL), "https://") {
		return r, fmt.Errorf("invalid speed URL protocol")
	}
	scheme := "http"
	if isTLSPort(r.Port) {
		scheme = "https"
	}
	parsed, err := parseSpeedTestURL(r.URL, scheme)
	if err != nil || parsed.User != nil {
		return r, fmt.Errorf("invalid speed URL")
	}
	r.URL = parsed.String()
	r.Datacenters = make([]string, 0, len(dcs))
	for dc := range dcs {
		r.Datacenters = append(r.Datacenters, dc)
	}
	sort.Strings(r.Datacenters)
	return r, nil
}

func defaultWebBatchDependencies() webBatchDependencies {
	return webBatchDependencies{latency: testIPLatency, speed: runWindowedSpeedTest, pause: func(ctx context.Context) bool {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(1200 * time.Millisecond):
			return true
		}
	}}
}

func runWebBatch(ctx context.Context, s *appSession, r startWebBatchRequest, deps webBatchDependencies) (err error) {
	s.taskMutex.Lock()
	s.webBatch = nil
	s.webBatchSequence++
	id := s.webBatchSequence
	s.taskMutex.Unlock()
	s.testMutex.Lock()
	s.testResults = nil
	s.testMutex.Unlock()
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("batch stage failed")
		}
		if err != nil {
			s.taskMutex.Lock()
			s.webBatch = nil
			s.taskMutex.Unlock()
			status := "failed"
			if ctx.Err() != nil {
				status = "cancelled"
			}
			s.sendWSMessage("web_batch_failed", map[string]interface{}{"id": id, "status": status, "message": err.Error()})
		}
	}()
	r, err = normalizeWebBatchRequest(r)
	if err != nil {
		return err
	}
	policy := officialSelectionPolicy{Datacenters: r.Datacenters, MaxLatencyMS: r.MaxLatencyMS, MinSpeedMB: r.MinSpeedMB, Limit: r.Limit}
	dcs, maxLatency, _ := validateOfficialSelectionPolicy(policy)
	s.scanMutex.Lock()
	scanned := append([]ScanResult(nil), s.scanResults...)
	s.scanMutex.Unlock()
	// Sort metadata as well as IP, so duplicate scan rows select a stable DC.
	sort.Slice(scanned, func(i, j int) bool {
		if scanned[i].IP != scanned[j].IP {
			return scanned[i].IP < scanned[j].IP
		}
		return scanned[i].DataCenter < scanned[j].DataCenter
	})
	var candidates []ScanResult
	seen := make(map[netip.Addr]bool)
	present := make(map[string]bool)
	for _, scan := range scanned {
		dc, valid := normalizeDatacenterCode(scan.DataCenter)
		if !valid {
			continue
		}
		if _, selected := dcs[dc]; !selected {
			continue
		}
		addr, parseErr := netip.ParseAddr(strings.TrimSpace(scan.IP))
		if parseErr != nil || addr.Zone() != "" {
			continue
		}
		addr = addr.Unmap()
		present[dc] = true
		if seen[addr] {
			continue
		}
		seen[addr] = true
		scan.IP, scan.DataCenter = addr.String(), dc
		candidates = append(candidates, scan)
	}
	summary := webBatchSnapshot{ID: id, Policy: policy, CandidateCount: len(candidates), Results: []TestResult{}, MissingDatacenters: []string{}}
	for _, dc := range r.Datacenters {
		if !present[dc] {
			summary.MissingDatacenters = append(summary.MissingDatacenters, dc)
		}
	}
	s.sendWSMessage("web_batch_started", summary)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no candidates in the selected datacenters")
	}
	var results []TestResult
	var resultMutex sync.Mutex
	stageFailed := false
	s.sendWSMessage("web_batch_progress", map[string]interface{}{"id": id, "phase": "latency", "current": 0, "total": len(candidates), "qualified": 0})
	// Drain workers even on cancellation; no previous stage may outlive this task.
	runBoundedWorkers(context.Background(), len(candidates), 50, 5, func(current, total int) {
		s.sendWSMessage("web_batch_progress", map[string]interface{}{"id": id, "phase": "latency", "current": current, "total": total})
	}, func(idx int) {
		defer func() {
			if recover() != nil {
				resultMutex.Lock()
				stageFailed = true
				resultMutex.Unlock()
			}
		}()
		if ctx.Err() != nil {
			return
		}
		scan := candidates[idx]
		res, _, _ := deps.latency(ctx, scan.IP, r.Port, r.MaxLatencyMS, r.ScanMode)
		if ctx.Err() != nil || res == nil || res.AvgLatency <= 0 || res.AvgLatency > maxLatency {
			return
		}
		res.IP, res.Port = scan.IP, r.Port
		res.WebBatchDerived = true
		res.DataCenter, res.DCCountry, res.Region, res.City = scan.DataCenter, scan.DCCountry, scan.Region, scan.City
		res.Speed, res.MeasuredSpeedMB, res.SpeedMeasured = "", 0, false
		resultMutex.Lock()
		results = append(results, *res)
		resultMutex.Unlock()
		s.sendWSMessage("web_batch_result", map[string]interface{}{"result": *res, "qualified": false})
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if stageFailed {
		return fmt.Errorf("latency stage failed")
	}
	sortOfficialTestResults(results)
	s.sendWSMessage("web_batch_progress", map[string]interface{}{"id": id, "phase": "speed", "current": 0, "total": len(results), "qualified": 0})
	qualified := 0
	for i := range results {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if i > 0 && !deps.pause(ctx) {
			return fmt.Errorf("speed stage interrupted")
		}
		speed, failure := deps.speed(ctx, results[i].IP, r.Port, r.URL)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		applyOfficialSpeedMeasurement(&results[i], speed, failure)
		summary.MeasuredCount++
		if results[i].SpeedMeasured && results[i].MeasuredSpeedMB >= r.MinSpeedMB {
			qualified++
		}
		s.sendWSMessage("web_batch_result", map[string]interface{}{"result": results[i], "qualified": results[i].SpeedMeasured && results[i].MeasuredSpeedMB >= r.MinSpeedMB})
		s.sendWSMessage("web_batch_progress", map[string]interface{}{"id": id, "phase": "speed", "current": i + 1, "total": len(results), "qualified": qualified})
	}
	allPolicy := policy
	allPolicy.Limit = max(1, len(results))
	allQualified, err := selectOfficialResults(results, allPolicy)
	if err != nil {
		return err
	}
	summary.QualifiedCount = len(allQualified)
	summary.Results, err = selectOfficialResults(results, policy)
	if err != nil {
		return err
	}
	summary.CompletedAt = time.Now()
	s.taskMutex.Lock()
	if ctx.Err() != nil {
		s.taskMutex.Unlock()
		return ctx.Err()
	}
	s.webBatch = &summary
	s.taskMutex.Unlock()
	s.testMutex.Lock()
	s.testResults = append([]TestResult(nil), summary.Results...)
	s.testMutex.Unlock()
	s.sendWSMessage("web_batch_complete", summary)
	return nil
}

func (s *appSession) completedWebBatchResults(expectedID uint64) (webBatchSnapshot, error) {
	snapshot, err := s.completedWebBatchSnapshot(expectedID)
	if err != nil {
		return webBatchSnapshot{}, err
	}
	if len(snapshot.Results) == 0 {
		return webBatchSnapshot{}, fmt.Errorf("no completed selected batch results")
	}
	return snapshot, nil
}

// Display/replay may inspect completed empty batches, but publishing may not.
func (s *appSession) completedWebBatchSnapshot(expectedID uint64) (webBatchSnapshot, error) {
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	if s.isTaskRunning || s.webBatch == nil || s.webBatch.CompletedAt.IsZero() {
		return webBatchSnapshot{}, fmt.Errorf("no completed selected batch results")
	}
	if expectedID != 0 && expectedID != s.webBatch.ID {
		return webBatchSnapshot{}, fmt.Errorf("selected batch has changed")
	}
	copy := *s.webBatch
	copy.Policy.Datacenters = append([]string(nil), copy.Policy.Datacenters...)
	copy.Results = append([]TestResult{}, copy.Results...)
	copy.MissingDatacenters = append([]string(nil), copy.MissingDatacenters...)
	return copy, nil
}

func (s *appSession) replayCompletedWebBatch() bool {
	snapshot, err := s.completedWebBatchSnapshot(0)
	if err != nil {
		return false
	}
	s.sendWSMessage("web_batch_started", snapshot)
	s.sendWSMessage("web_batch_complete", snapshot)
	s.sendWSMessage("task_complete", nil)
	return true
}
