package main

import (
	"fmt"
	"math"
	"net/netip"
	"sort"
	"strings"
	"time"
)

type officialSelectionPolicy struct {
	Datacenters  []string
	MaxLatencyMS int
	MinSpeedMB   float64
	Limit        int
}

type officialSelectionCandidate struct {
	result TestResult
	addr   netip.Addr
}

func normalizeDatacenterCode(value string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(value))
	if len(code) != 3 {
		return "", false
	}
	for i := range code {
		if code[i] < 'A' || code[i] > 'Z' {
			return "", false
		}
	}
	return code, true
}

func selectOfficialResults(results []TestResult, policy officialSelectionPolicy) ([]TestResult, error) {
	datacenters, maxLatency, err := validateOfficialSelectionPolicy(policy)
	if err != nil {
		return nil, err
	}

	candidates := make([]officialSelectionCandidate, 0, len(results))
	for _, result := range results {
		dc, ok := normalizeDatacenterCode(result.DataCenter)
		if !ok {
			continue
		}
		if _, ok := datacenters[dc]; !ok {
			continue
		}
		addr, err := netip.ParseAddr(strings.TrimSpace(result.IP))
		if err != nil || addr.Zone() != "" {
			continue
		}
		addr = addr.Unmap()
		if result.AvgLatency <= 0 || result.AvgLatency > maxLatency || !result.SpeedMeasured || !finiteNonnegative(result.MeasuredSpeedMB) || result.MeasuredSpeedMB < policy.MinSpeedMB {
			continue
		}

		result.IP = addr.String()
		result.DataCenter = dc
		candidates = append(candidates, officialSelectionCandidate{result: result, addr: addr})
	}

	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.result.MeasuredSpeedMB != right.result.MeasuredSpeedMB {
			return left.result.MeasuredSpeedMB > right.result.MeasuredSpeedMB
		}
		if left.result.AvgLatency != right.result.AvgLatency {
			return left.result.AvgLatency < right.result.AvgLatency
		}
		if left.result.IP != right.result.IP {
			return left.result.IP < right.result.IP
		}
		return lessOfficialSelectionMetadata(left.result, right.result)
	})

	selected := make([]TestResult, 0, min(policy.Limit, len(candidates)))
	seen := make(map[netip.Addr]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate.addr]; ok {
			continue
		}
		seen[candidate.addr] = struct{}{}
		selected = append(selected, candidate.result)
		if len(selected) == policy.Limit {
			break
		}
	}
	return selected, nil
}

func validateOfficialSelectionPolicy(policy officialSelectionPolicy) (map[string]struct{}, time.Duration, error) {
	if len(policy.Datacenters) == 0 {
		return nil, 0, fmt.Errorf("at least one datacenter is required")
	}
	datacenters := make(map[string]struct{}, len(policy.Datacenters))
	for _, value := range policy.Datacenters {
		code, ok := normalizeDatacenterCode(value)
		if !ok {
			return nil, 0, fmt.Errorf("invalid datacenter code %q", value)
		}
		datacenters[code] = struct{}{}
	}
	if policy.MaxLatencyMS <= 0 || int64(policy.MaxLatencyMS) > int64(math.MaxInt64)/int64(time.Millisecond) {
		return nil, 0, fmt.Errorf("maximum latency must be a positive millisecond value")
	}
	if !finiteNonnegative(policy.MinSpeedMB) {
		return nil, 0, fmt.Errorf("minimum speed must be finite and nonnegative")
	}
	if policy.Limit <= 0 {
		return nil, 0, fmt.Errorf("result limit must be positive")
	}
	return datacenters, time.Duration(policy.MaxLatencyMS) * time.Millisecond, nil
}

func finiteNonnegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func lessOfficialSelectionMetadata(left, right TestResult) bool {
	if left.DataCenter != right.DataCenter {
		return left.DataCenter < right.DataCenter
	}
	if left.Port != right.Port {
		return left.Port < right.Port
	}
	if left.DCCountry != right.DCCountry {
		return left.DCCountry < right.DCCountry
	}
	if left.Region != right.Region {
		return left.Region < right.Region
	}
	if left.City != right.City {
		return left.City < right.City
	}
	return left.Speed < right.Speed
}
