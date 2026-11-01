package main

import (
	"encoding/json"
	"errors"
	"math"
	"net/netip"
	"strconv"
	"strings"
)

func validRetainedEndpoint(ip string, port int) bool {
	addr, err := netip.ParseAddr(ip)
	return err == nil && addr.Zone() == "" && port > 0 && port <= 65535
}

func validRetainedNumber(value, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= maximum
}

func validRetainedDisplayNumber(value, suffix string, maximum float64) bool {
	if value == "" || value == "-" {
		return true
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, suffix)), 64)
	return err == nil && validRetainedNumber(number, maximum)
}

func validRetainedSpeed(value string) bool {
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		return validRetainedNumber(number, math.MaxFloat64)
	}
	return !strings.Contains(value, "MB/s") || validRetainedDisplayNumber(value, "MB/s", math.MaxFloat64)
}

func validateRetainedResultsDisk(value retainedResultsSnapshot, raw []byte) error {
	fail := errors.New("invalid persisted result rows; file left unchanged")
	if value.Generation > 1<<53-1 || len(value.Centers) > 1000 || len(value.Measurements) > retainedResultsMaxRows {
		return fail
	}
	for _, row := range value.Scans {
		if row.RecordedScanMode != "" && row.RecordedScanMode != scanModeTCPing && row.RecordedScanMode != scanModeHTTPing {
			return fail
		}
		if !validRetainedEndpoint(row.IP, row.Port) || row.TCPDuration < 0 || (row.DataCenter != "" && !validRetainedDC(row.DataCenter)) || !validRetainedDisplayNumber(row.LatencyStr, "ms", math.MaxFloat64) {
			return fail
		}
	}
	for _, row := range value.Tests {
		if row.RecordedScanMode != "" && row.RecordedScanMode != scanModeTCPing && row.RecordedScanMode != scanModeHTTPing {
			return fail
		}
		if !validRetainedEndpoint(row.IP, row.Port) || row.MinLatency < 0 || row.MaxLatency < 0 || row.AvgLatency < 0 || !validRetainedNumber(row.LossRate, 1) || !validRetainedSpeed(row.Speed) || (row.DataCenter != "" && !validRetainedDC(row.DataCenter)) {
			return fail
		}
	}
	for _, row := range value.NSBLive {
		port, err := strconv.Atoi(row.Port)
		if err != nil || !validRetainedEndpoint(row.IP, port) || !validRetainedDisplayNumber(row.LossRate, "%", 100) || !validRetainedDisplayNumber(row.Latency, "ms", math.MaxFloat64) || !validRetainedSpeed(row.Speed) || (row.DC != "" && !validRetainedDC(row.DC)) {
			return fail
		}
	}
	for _, row := range value.Centers {
		if !validRetainedDC(row.DataCenter) || row.IPCount < 0 || row.IPCount > retainedResultsMaxRows || row.MinLatency < 0 {
			return fail
		}
	}
	for key, measurement := range value.Measurements {
		parts := strings.Split(key, "\x00")
		if len(parts) != 2 || !validRetainedNumber(measurement.MB, math.MaxFloat64) {
			return fail
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil || !validRetainedEndpoint(parts[0], port) {
			return fail
		}
	}
	if csv := value.NSBCSV; csv != nil {
		if len(csv.Headers) > 64 || csv.QualifiedCount < 0 || csv.QualifiedCount > retainedResultsMaxRows {
			return fail
		}
		keys := make([]string, len(csv.Headers))
		ipIndex, portIndex := -1, -1
		for i, header := range csv.Headers {
			keys[i] = retainedCSVFieldKey(header)
			if keys[i] == "ip" {
				ipIndex = i
			}
			if keys[i] == "port" {
				portIndex = i
			}
		}
		if len(csv.Rows) > 0 && (ipIndex < 0 || portIndex < 0) {
			return fail
		}
		for _, row := range csv.Rows {
			if len(row) != len(keys) {
				return fail
			}
			port, err := strconv.Atoi(row[portIndex])
			if err != nil || !validRetainedEndpoint(row[ipIndex], port) {
				return fail
			}
			for i, key := range keys {
				if key == "lossRate" && !validRetainedDisplayNumber(row[i], "%", 100) {
					return fail
				}
				if key == "latency" && !validRetainedDisplayNumber(row[i], "ms", math.MaxFloat64) {
					return fail
				}
				if key == "speed" && !validRetainedSpeed(row[i]) {
					return fail
				}
			}
		}
	}
	// Original official renderers interpolate display metadata as HTML text.
	var fields interface{}
	if json.Unmarshal(raw, &fields) != nil || !validRetainedDiskText(fields) {
		return fail
	}
	return nil
}

func validRetainedDiskText(value interface{}) bool {
	switch value := value.(type) {
	case string:
		return len(value) <= 4096 && !strings.ContainsAny(value, "<>")
	case []interface{}:
		for _, item := range value {
			if !validRetainedDiskText(item) {
				return false
			}
		}
	case map[string]interface{}:
		for key, item := range value {
			if len(key) > 4096 || strings.ContainsAny(key, "<>") || !validRetainedDiskText(item) {
				return false
			}
		}
	}
	return true
}
