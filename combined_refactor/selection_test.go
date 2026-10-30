package main

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func measuredResult(ip, dc string, latency time.Duration, speed float64) TestResult {
	return TestResult{
		IP:              ip,
		DataCenter:      dc,
		AvgLatency:      latency,
		Speed:           "legacy display",
		MeasuredSpeedMB: speed,
		SpeedMeasured:   true,
	}
}

func resultIPs(results []TestResult) []string {
	ips := make([]string, len(results))
	for i := range results {
		ips[i] = results[i].IP
	}
	return ips
}

func TestSelectOfficialResultsFiltersSelectedDatacenterUnion(t *testing.T) {
	results := []TestResult{
		measuredResult("192.0.2.1", "FRA", 30*time.Millisecond, 8),
		measuredResult("192.0.2.2", "sjc", 20*time.Millisecond, 10),
		measuredResult("192.0.2.3", "HKG", 10*time.Millisecond, 20),
	}
	policy := officialSelectionPolicy{
		Datacenters:  []string{" fra ", "SJC", "FRA"},
		MaxLatencyMS: 50,
		MinSpeedMB:   5,
		Limit:        10,
	}

	got, err := selectOfficialResults(results, policy)
	if err != nil {
		t.Fatalf("selectOfficialResults() error = %v", err)
	}
	if want := []string{"192.0.2.2", "192.0.2.1"}; !reflect.DeepEqual(resultIPs(got), want) {
		t.Fatalf("selected IPs = %v, want %v", resultIPs(got), want)
	}
	if got[1].DataCenter != "FRA" || got[0].DataCenter != "SJC" {
		t.Fatalf("normalized datacenters = [%s %s], want [SJC FRA]", got[0].DataCenter, got[1].DataCenter)
	}
}

func TestSelectOfficialResultsIncludesLatencyAndSpeedBoundaries(t *testing.T) {
	results := []TestResult{
		measuredResult("192.0.2.1", "FRA", 50*time.Millisecond, 5),
		measuredResult("192.0.2.2", "FRA", 50*time.Millisecond+time.Nanosecond, 6),
		measuredResult("192.0.2.3", "FRA", 49*time.Millisecond, math.Nextafter(5, 0)),
	}

	got, err := selectOfficialResults(results, officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 50, MinSpeedMB: 5, Limit: 10})
	if err != nil {
		t.Fatalf("selectOfficialResults() error = %v", err)
	}
	if want := []string{"192.0.2.1"}; !reflect.DeepEqual(resultIPs(got), want) {
		t.Fatalf("selected IPs = %v, want %v", resultIPs(got), want)
	}
}

func TestSelectOfficialResultsRejectsInvalidPolicy(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	tests := []struct {
		name   string
		policy officialSelectionPolicy
	}{
		{name: "missing datacenter", policy: officialSelectionPolicy{MaxLatencyMS: 50, MinSpeedMB: 1, Limit: 10}},
		{name: "empty datacenter", policy: officialSelectionPolicy{Datacenters: []string{"FRA", " "}, MaxLatencyMS: 50, MinSpeedMB: 1, Limit: 10}},
		{name: "invalid datacenter", policy: officialSelectionPolicy{Datacenters: []string{"FR4"}, MaxLatencyMS: 50, MinSpeedMB: 1, Limit: 10}},
		{name: "zero latency", policy: officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 0, MinSpeedMB: 1, Limit: 10}},
		{name: "negative latency", policy: officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: -1, MinSpeedMB: 1, Limit: 10}},
		{name: "overflowing latency", policy: officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: maxInt, MinSpeedMB: 1, Limit: 10}},
		{name: "negative speed", policy: officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 50, MinSpeedMB: -1, Limit: 10}},
		{name: "nan speed", policy: officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 50, MinSpeedMB: math.NaN(), Limit: 10}},
		{name: "infinite speed", policy: officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 50, MinSpeedMB: math.Inf(1), Limit: 10}},
		{name: "zero limit", policy: officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 50, MinSpeedMB: 1, Limit: 0}},
		{name: "negative limit", policy: officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 50, MinSpeedMB: 1, Limit: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := selectOfficialResults(nil, tt.policy); err == nil {
				t.Fatal("selectOfficialResults() error = nil, want validation error")
			}
		})
	}
}

func TestSelectOfficialResultsExcludesInvalidCandidates(t *testing.T) {
	valid := measuredResult("192.0.2.1", "FRA", 20*time.Millisecond, 5)
	unmeasured := measuredResult("192.0.2.2", "FRA", 20*time.Millisecond, 100)
	unmeasured.SpeedMeasured = false
	unmeasured.Speed = "100.00MB/s"

	results := []TestResult{
		valid,
		measuredResult("not-an-ip", "FRA", 20*time.Millisecond, 100),
		measuredResult("192.0.2.3", "FR4", 20*time.Millisecond, 100),
		measuredResult("192.0.2.4", "HKG", 20*time.Millisecond, 100),
		measuredResult("192.0.2.5", "FRA", 0, 100),
		measuredResult("192.0.2.6", "FRA", -time.Millisecond, 100),
		unmeasured,
		measuredResult("192.0.2.7", "FRA", 20*time.Millisecond, -1),
		measuredResult("192.0.2.8", "FRA", 20*time.Millisecond, math.NaN()),
		measuredResult("192.0.2.9", "FRA", 20*time.Millisecond, math.Inf(1)),
	}

	got, err := selectOfficialResults(results, officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 50, MinSpeedMB: 0, Limit: 10})
	if err != nil {
		t.Fatalf("selectOfficialResults() error = %v", err)
	}
	if want := []string{"192.0.2.1"}; !reflect.DeepEqual(resultIPs(got), want) {
		t.Fatalf("selected IPs = %v, want %v", resultIPs(got), want)
	}
}

func TestSelectOfficialResultsNormalizesAndDeduplicatesIP(t *testing.T) {
	results := []TestResult{
		measuredResult("2001:0db8:0:0:0:0:0:1", "FRA", 25*time.Millisecond, 8),
		measuredResult("2001:db8::1", "SJC", 30*time.Millisecond, 9),
		measuredResult("::ffff:192.0.2.10", "FRA", 20*time.Millisecond, 7),
		measuredResult("192.0.2.10", "SJC", 10*time.Millisecond, 7),
	}

	got, err := selectOfficialResults(results, officialSelectionPolicy{Datacenters: []string{"FRA", "SJC"}, MaxLatencyMS: 50, MinSpeedMB: 1, Limit: 10})
	if err != nil {
		t.Fatalf("selectOfficialResults() error = %v", err)
	}
	if want := []string{"2001:db8::1", "192.0.2.10"}; !reflect.DeepEqual(resultIPs(got), want) {
		t.Fatalf("selected IPs = %v, want %v", resultIPs(got), want)
	}
	if got[0].DataCenter != "SJC" || got[1].DataCenter != "SJC" {
		t.Fatalf("duplicate winners have datacenters [%s %s], want [SJC SJC]", got[0].DataCenter, got[1].DataCenter)
	}
}

func TestSelectOfficialResultsAppliesTotalLimitAfterGlobalRanking(t *testing.T) {
	results := make([]TestResult, 0, 12)
	for i := 1; i <= 12; i++ {
		dc := "FRA"
		if i%2 == 0 {
			dc = "SJC"
		}
		results = append(results, measuredResult("192.0.2."+strconv.Itoa(i), dc, 20*time.Millisecond, float64(i)))
	}

	got, err := selectOfficialResults(results, officialSelectionPolicy{Datacenters: []string{"FRA", "SJC"}, MaxLatencyMS: 50, MinSpeedMB: 1, Limit: 10})
	if err != nil {
		t.Fatalf("selectOfficialResults() error = %v", err)
	}
	want := []string{"192.0.2.12", "192.0.2.11", "192.0.2.10", "192.0.2.9", "192.0.2.8", "192.0.2.7", "192.0.2.6", "192.0.2.5", "192.0.2.4", "192.0.2.3"}
	if !reflect.DeepEqual(resultIPs(got), want) {
		t.Fatalf("selected IPs = %v, want %v", resultIPs(got), want)
	}
}

func TestSelectOfficialResultsUsesDeterministicTieOrder(t *testing.T) {
	results := []TestResult{
		measuredResult("192.0.2.20", "FRA", 20*time.Millisecond, 10),
		measuredResult("192.0.2.3", "FRA", 20*time.Millisecond, 10),
		measuredResult("192.0.2.2", "FRA", 10*time.Millisecond, 10),
		measuredResult("192.0.2.1", "FRA", 30*time.Millisecond, 11),
	}

	got, err := selectOfficialResults(results, officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 50, MinSpeedMB: 1, Limit: 10})
	if err != nil {
		t.Fatalf("selectOfficialResults() error = %v", err)
	}
	want := []string{"192.0.2.1", "192.0.2.2", "192.0.2.20", "192.0.2.3"}
	if !reflect.DeepEqual(resultIPs(got), want) {
		t.Fatalf("selected IPs = %v, want %v", resultIPs(got), want)
	}
}

func TestSelectOfficialResultsAllowsFewerAndEmptyResults(t *testing.T) {
	policy := officialSelectionPolicy{Datacenters: []string{"FRA"}, MaxLatencyMS: 50, MinSpeedMB: 1, Limit: 10}

	got, err := selectOfficialResults([]TestResult{measuredResult("192.0.2.1", "FRA", 20*time.Millisecond, 5)}, policy)
	if err != nil {
		t.Fatalf("selectOfficialResults(one) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("selectOfficialResults(one) length = %d, want 1", len(got))
	}

	got, err = selectOfficialResults(nil, policy)
	if err != nil {
		t.Fatalf("selectOfficialResults(nil) error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("selectOfficialResults(nil) = %v, want empty", got)
	}
}

func TestSelectOfficialResultsDoesNotMutateInput(t *testing.T) {
	results := []TestResult{
		measuredResult("2001:0db8:0:0:0:0:0:1", "fra", 20*time.Millisecond, 5),
		measuredResult("192.0.2.2", "SJC", 10*time.Millisecond, 6),
	}
	want := append([]TestResult(nil), results...)

	if _, err := selectOfficialResults(results, officialSelectionPolicy{Datacenters: []string{"FRA", "SJC"}, MaxLatencyMS: 50, MinSpeedMB: 1, Limit: 10}); err != nil {
		t.Fatalf("selectOfficialResults() error = %v", err)
	}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("input mutated: got %#v, want %#v", results, want)
	}
}

func TestTestResultNumericSpeedStateIsBackendOnly(t *testing.T) {
	payload, err := json.Marshal(measuredResult("192.0.2.1", "FRA", 20*time.Millisecond, 5.123456))
	if err != nil {
		t.Fatalf("json.Marshal(TestResult) error = %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("json.Unmarshal(TestResult) error = %v", err)
	}
	for _, field := range []string{"MeasuredSpeedMB", "SpeedMeasured"} {
		if _, ok := fields[field]; ok {
			t.Fatalf("backend-only field %q leaked in TestResult JSON: %s", field, payload)
		}
	}
	if fields["Speed"] != "legacy display" {
		t.Fatalf("legacy Speed JSON value = %v, want legacy display", fields["Speed"])
	}
}
