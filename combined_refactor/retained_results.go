package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const retainedResultsMaxBytes = 64 << 20
const retainedResultsMaxRows = 100000

type retainedResultsSnapshot struct {
	Generation   uint64                              `json:"generation"`
	Mode         string                              `json:"mode"`
	Scans        []ScanResult                        `json:"scans"`
	Tests        []TestResult                        `json:"tests"`
	NSBLive      []nsbScanMessage                    `json:"nsbLive"`
	NSBCSV       *nsbCSVCompletePayload              `json:"nsbCSV,omitempty"`
	Centers      []DataCenterInfo                    `json:"centers"`
	Params       map[string]interface{}              `json:"params"`
	View         map[string]json.RawMessage          `json:"view"`
	Saved        bool                                `json:"saved"`
	Error        string                              `json:"error,omitempty"`
	Measurements map[string]retainedSpeedMeasurement `json:"testMeasurements,omitempty"`
}

type retainedSpeedMeasurement struct {
	MB       float64 `json:"mb"`
	Measured bool    `json:"measured"`
}

type retainedResultsStore struct {
	mu         sync.Mutex
	path       string
	loaded     bool
	value      retainedResultsSnapshot
	writer     *appSession
	timer      *time.Timer
	indices    [4]map[string]int
	csvKeys    []string
	incomplete bool
}

var latestResults = &retainedResultsStore{}
var errRetainedGeneration = errors.New("latest results changed")

func emptyRetainedResults(generation uint64, mode string) retainedResultsSnapshot {
	return retainedResultsSnapshot{Generation: generation, Mode: mode, Scans: []ScanResult{}, Tests: []TestResult{}, NSBLive: []nsbScanMessage{}, Centers: []DataCenterInfo{}, Params: map[string]interface{}{}, View: map[string]json.RawMessage{}, Measurements: map[string]retainedSpeedMeasurement{}}
}

func (r *retainedResultsStore) loadLocked() error {
	if r.loaded {
		return nil
	}
	if r.path == "" {
		path, err := defaultDataPath("latest-results.json")
		if err != nil {
			return err
		}
		r.path = path
	}
	value := emptyRetainedResults(0, "")
	f, err := os.Open(r.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		defer f.Close()
		raw, err := io.ReadAll(io.LimitReader(f, retainedResultsMaxBytes+1))
		if err != nil {
			return err
		}
		if len(raw) > retainedResultsMaxBytes {
			return errors.New("latest results exceed 64 MiB; file left unchanged")
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("read latest results: %w", err)
		}
		if value.Mode != "" && value.Mode != "official" && value.Mode != "nsb" {
			return errors.New("invalid latest results mode")
		}
		if len(value.Scans) > retainedResultsMaxRows || len(value.Tests) > retainedResultsMaxRows || len(value.NSBLive) > retainedResultsMaxRows || (value.NSBCSV != nil && len(value.NSBCSV.Rows) > retainedResultsMaxRows) {
			return errors.New("latest results exceed 100000 rows; file left unchanged")
		}
		if err := validateRetainedResultsDisk(value, raw); err != nil {
			return err
		}
		if err := validateRetainedView(value.View); err != nil {
			return err
		}
		value.Params = safeRetainedParams(value.Params)
		value.Saved, value.Error = true, ""
	}
	r.value, r.loaded = value, true
	if r.value.View == nil {
		r.value.View = map[string]json.RawMessage{}
	}
	if r.value.Measurements == nil {
		r.value.Measurements = map[string]retainedSpeedMeasurement{}
	}
	if r.value.Scans == nil {
		r.value.Scans = []ScanResult{}
	}
	if r.value.Tests == nil {
		r.value.Tests = []TestResult{}
	}
	if r.value.NSBLive == nil {
		r.value.NSBLive = []nsbScanMessage{}
	}
	if r.value.Centers == nil {
		r.value.Centers = []DataCenterInfo{}
	}
	r.reindexLocked()
	return nil
}

func (r *retainedResultsStore) reindexLocked() {
	for i := range r.indices {
		r.indices[i] = make(map[string]int)
	}
	for i, v := range r.value.Scans {
		r.indices[0][retainedKey(v.IP, strconv.Itoa(v.Port))] = i
	}
	for i, v := range r.value.Tests {
		r.indices[1][retainedKey(v.IP, strconv.Itoa(v.Port))] = i
	}
	for i, v := range r.value.NSBLive {
		r.indices[2][retainedKey(v.IP, v.Port)] = i
	}
	r.indexNSBCSVLocked()
}

func (r *retainedResultsStore) indexNSBCSVLocked() {
	r.indices[3] = make(map[string]int)
	r.csvKeys = nil
	if r.value.NSBCSV == nil {
		return
	}
	ipIndex, portIndex := -1, -1
	for i, header := range r.value.NSBCSV.Headers {
		key := retainedCSVFieldKey(header)
		r.csvKeys = append(r.csvKeys, key)
		if key == "ip" {
			ipIndex = i
		}
		if key == "port" {
			portIndex = i
		}
	}
	if ipIndex < 0 || portIndex < 0 {
		return
	}
	for i, row := range r.value.NSBCSV.Rows {
		if ipIndex < len(row) && portIndex < len(row) {
			r.indices[3][retainedKey(row[ipIndex], row[portIndex])] = i
		}
	}
}

func retainedCSVFieldKey(header string) string {
	if header == "Port" {
		return "port"
	}
	if header == "OriginalInput" {
		return "originalInput"
	}
	return cliFieldKeyFromHeader(header)
}

func retainedKey(ip, port string) string { return ip + "\x00" + port }

func (r *retainedResultsStore) writeLocked(value retainedResultsSnapshot) error {
	value.Saved, value.Error = true, ""
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > retainedResultsMaxBytes {
		return errors.New("latest results exceed 64 MiB; previous file left unchanged")
	}
	return atomicWriteFile(r.path, raw, 0600)
}

func (r *retainedResultsStore) get(s *appSession) (retainedResultsSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.loadLocked(); err != nil {
		return retainedResultsSnapshot{}, err
	}
	raw, err := json.Marshal(r.value)
	if err != nil {
		return retainedResultsSnapshot{}, err
	}
	var copy retainedResultsSnapshot
	if err := json.Unmarshal(raw, &copy); err != nil {
		return copy, err
	}
	if len(copy.Centers) == 0 && len(copy.Scans) > 0 {
		copy.Centers = buildDCList(copy.Scans)
	}
	if r.writer != s {
		s.restoreRetainedResults(copy)
	}
	return copy, nil
}

func (s *appSession) restoreRetainedResults(value retainedResultsSnapshot) {
	s.retainedGeneration = value.Generation
	s.scanMutex.Lock()
	s.scanResults = append([]ScanResult(nil), value.Scans...)
	s.scanMutex.Unlock()
	s.testMutex.Lock()
	s.testResults = append([]TestResult(nil), value.Tests...)
	for i := range s.testResults {
		if measured, ok := value.Measurements[retainedKey(s.testResults[i].IP, strconv.Itoa(s.testResults[i].Port))]; ok {
			s.testResults[i].MeasuredSpeedMB, s.testResults[i].SpeedMeasured = measured.MB, measured.Measured
		} else if speed, ok := parseSpeedMBForSort(s.testResults[i].Speed); ok {
			s.testResults[i].MeasuredSpeedMB, s.testResults[i].SpeedMeasured = speed, true
		}
	}
	s.testMutex.Unlock()
	s.nsbMutex.Lock()
	s.nsbCompletePayload = value.NSBCSV
	s.nsbHeaders, s.nsbRows = nil, nil
	if value.NSBCSV != nil {
		s.nsbHeaders, s.nsbRows = value.NSBCSV.Headers, value.NSBCSV.Rows
	}
	s.nsbMutex.Unlock()
}

func (r *retainedResultsStore) begin(s *appSession, initial bool, mode string, params map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.loadLocked(); err != nil {
		return err
	}
	if r.writer != nil {
		return errors.New("latest results have an active writer")
	}
	if s.retainedGeneration != r.value.Generation {
		return fmt.Errorf("%w; reload before starting the task", errRetainedGeneration)
	}
	if initial {
		next := emptyRetainedResults(r.value.Generation+1, mode)
		next.Params = safeRetainedParams(params)
		updateRetainedFilterProvenance(&next, next.Params)
		if err := r.writeLocked(next); err != nil {
			return err
		}
		next.Saved = true
		r.value = next
		r.incomplete = false
		r.reindexLocked()
		s.restoreRetainedResults(next)
	} else {
		if r.value.Mode != mode || mode == "" {
			return errors.New("no matching retained scan to continue")
		}
		for k, v := range safeRetainedParams(params) {
			r.value.Params[k] = v
		}
		updateRetainedFilterProvenance(&r.value, safeRetainedParams(params))
	}
	r.writer = s
	return nil
}

func (r *retainedResultsStore) clear(generation uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.loadLocked(); err != nil {
		return err
	}
	if generation != r.value.Generation {
		return fmt.Errorf("%w; reload before clearing", errRetainedGeneration)
	}
	if r.writer != nil || anyTaskRunning() {
		return errors.New("stop the active task before clearing results")
	}
	next := emptyRetainedResults(generation+1, "")
	if err := r.writeLocked(next); err != nil {
		return err
	}
	next.Saved = true
	r.value = next
	r.incomplete = false
	r.reindexLocked()
	return nil
}

func (r *retainedResultsStore) setView(generation uint64, view map[string]json.RawMessage) error {
	if err := validateRetainedView(view); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.loadLocked(); err != nil {
		return err
	}
	if generation != r.value.Generation {
		return fmt.Errorf("%w; reload before saving selection", errRetainedGeneration)
	}
	if r.incomplete {
		return errors.New(r.value.Error)
	}
	next := r.value
	next.View = make(map[string]json.RawMessage, len(r.value.View)+len(view))
	for k, v := range r.value.View {
		next.View[k] = v
	}
	for k, v := range view {
		next.View[k] = append(json.RawMessage(nil), v...)
	}
	if err := r.writeLocked(next); err != nil {
		return err
	}
	next.Saved, next.Error = true, ""
	r.value = next
	return nil
}

func (r *retainedResultsStore) annotate(s *appSession, kind string, data interface{}) interface{} {
	r.mu.Lock()
	mode, _ := r.value.Params["scanMode"].(string)
	writer := r.writer == s && s.retainedGeneration == r.value.Generation
	r.mu.Unlock()
	if !writer || (mode != scanModeTCPing && mode != scanModeHTTPing) {
		return data
	}
	test := func(row TestResult) TestResult {
		if row.RecordedScanMode == "" {
			row.RecordedScanMode = mode
		}
		return row
	}
	list := func(rows []TestResult) []TestResult {
		copy := append([]TestResult(nil), rows...)
		for i := range copy {
			copy[i] = test(copy[i])
		}
		return copy
	}
	switch kind {
	case "scan_result":
		if row, ok := data.(ScanResult); ok {
			if row.RecordedScanMode == "" {
				row.RecordedScanMode = mode
			}
			return row
		}
	case "test_result":
		if row, ok := data.(TestResult); ok {
			return test(row)
		}
	case "test_complete":
		if rows, ok := data.([]TestResult); ok {
			return list(rows)
		}
	case "web_batch_result":
		if payload, ok := data.(map[string]interface{}); ok {
			if row, ok := payload["result"].(TestResult); ok {
				copy := make(map[string]interface{}, len(payload))
				for key, value := range payload {
					copy[key] = value
				}
				copy["result"] = test(row)
				return copy
			}
		}
	case "web_batch_complete":
		if batch, ok := data.(webBatchSnapshot); ok {
			batch.Results = list(batch.Results)
			return batch
		}
	}
	return data
}

func (r *retainedResultsStore) record(s *appSession, kind string, data interface{}) {
	r.mu.Lock()
	if r.writer != s || s.retainedGeneration != r.value.Generation {
		r.mu.Unlock()
		return
	}
	changed := true
	var captureErr error
	switch kind {
	case "scan_result":
		if v, ok := data.(ScanResult); ok {
			key := retainedKey(v.IP, strconv.Itoa(v.Port))
			if i, exists := r.indices[0][key]; exists {
				r.value.Scans[i] = v
			} else if len(r.value.Scans) < retainedResultsMaxRows {
				r.indices[0][key] = len(r.value.Scans)
				r.value.Scans = append(r.value.Scans, v)
			} else {
				captureErr = errors.New("scan retention exceeds 100000 rows; latest snapshot is incomplete")
			}
		}
	case "test_result":
		if v, ok := data.(TestResult); ok {
			captureErr = r.mergeTestLocked(v)
		}
	case "web_batch_result":
		if payload, ok := data.(map[string]interface{}); ok {
			if v, ok := payload["result"].(TestResult); ok {
				captureErr = r.mergeTestLocked(v)
			}
		}
	case "web_batch_complete":
		if payload, ok := data.(webBatchSnapshot); ok {
			for _, v := range payload.Results {
				if err := r.mergeTestLocked(v); err != nil {
					captureErr = err
					break
				}
			}
		}
	case "test_complete":
		if list, ok := data.([]TestResult); ok {
			for _, v := range list {
				if err := r.mergeTestLocked(v); err != nil {
					captureErr = err
					break
				}
			}
		}
	case "speed_test_result":
		if v, ok := data.(map[string]string); ok {
			r.value.View["officialSpeedEnabled"] = json.RawMessage(`true`)
			index, exists := r.indices[1][retainedKey(v["ip"], v["port"])]
			if !exists {
				for i, row := range r.value.Tests {
					if row.IP != v["ip"] {
						continue
					}
					if v["port"] == "" {
						index, exists = i, true
						break
					}
					port, err := strconv.Atoi(v["port"])
					if err != nil || port < 1 || port > 65535 {
						break
					}
					row.Port, row.Speed, row.MeasuredSpeedMB, row.SpeedMeasured = port, "", 0, false
					captureErr = r.mergeTestLocked(row)
					index, exists = r.indices[1][retainedKey(row.IP, strconv.Itoa(row.Port))]
					break
				}
			}
			if exists {
				r.value.Tests[index].Speed = v["speed"]
				speed, measured := parseSpeedMBForSort(v["speed"])
				r.value.Measurements[retainedKey(r.value.Tests[index].IP, strconv.Itoa(r.value.Tests[index].Port))] = retainedSpeedMeasurement{MB: speed, Measured: measured}
			}
		}
	case "nsb_scan_result":
		if v, ok := data.(nsbScanMessage); ok {
			captureErr = r.mergeNSBLocked(v)
		}
	case "nsb_scan_sorted":
		if list, ok := data.([]nsbScanMessage); ok {
			for _, v := range list {
				if err := r.mergeNSBLocked(v); err != nil {
					captureErr = err
					break
				}
			}
		}
	case "nsb_csv_complete":
		if v, ok := data.(nsbCSVCompletePayload); ok {
			if len(v.Rows) > retainedResultsMaxRows {
				captureErr = errors.New("CSV retention exceeds 100000 rows; latest snapshot is incomplete")
			} else {
				r.value.NSBCSV = &v
				r.indexNSBCSVLocked()
			}
		}
	case "nsb_speed_complete":
		if payload, ok := data.(map[string]interface{}); ok && r.value.NSBCSV != nil {
			r.value.NSBCSV.QualifiedCount = asInt(payload["totalQualified"])
		}
	case "scan_complete_wait_dc":
		if list, ok := data.([]DataCenterInfo); ok {
			r.value.Centers = append([]DataCenterInfo(nil), list...)
		}
	default:
		changed = false
	}
	if changed {
		r.value.Saved = false
		if captureErr != nil {
			r.value.Error = captureErr.Error()
			r.incomplete = true
		}
		if r.timer == nil {
			r.timer = time.AfterFunc(2*time.Second, func() { r.checkpoint(s) })
		}
	}
	generation := r.value.Generation
	r.mu.Unlock()
	if captureErr != nil {
		s.sendWSMessageDirect("retained_results_error", map[string]interface{}{"generation": generation, "message": captureErr.Error(), "code": "bounds"})
	}
}

func (r *retainedResultsStore) mergeTestLocked(v TestResult) error {
	key := retainedKey(v.IP, strconv.Itoa(v.Port))
	if i, exists := r.indices[1][key]; exists {
		if v.Speed == "" {
			v.Speed = r.value.Tests[i].Speed
			v.MeasuredSpeedMB, v.SpeedMeasured = r.value.Tests[i].MeasuredSpeedMB, r.value.Tests[i].SpeedMeasured
		}
		r.value.Tests[i] = v
	} else {
		if len(r.value.Tests) >= retainedResultsMaxRows {
			return errors.New("test retention exceeds 100000 rows; latest snapshot is incomplete")
		}
		r.indices[1][key] = len(r.value.Tests)
		r.value.Tests = append(r.value.Tests, v)
	}
	if v.SpeedMeasured {
		r.value.Measurements[key] = retainedSpeedMeasurement{MB: v.MeasuredSpeedMB, Measured: true}
	}
	return nil
}

func (r *retainedResultsStore) mergeNSBLocked(v nsbScanMessage) error {
	key := retainedKey(v.IP, v.Port)
	if i, exists := r.indices[2][key]; exists {
		// Compact and speed updates must not erase earlier probe details.
		previous, _ := json.Marshal(r.value.NSBLive[i])
		update, _ := json.Marshal(v)
		var fields, patch map[string]json.RawMessage
		json.Unmarshal(previous, &fields)
		json.Unmarshal(update, &patch)
		for k, val := range patch {
			fields[k] = val
		}
		merged, _ := json.Marshal(fields)
		var value nsbScanMessage
		json.Unmarshal(merged, &value)
		if v.Speed != "" {
			value.SpeedQualified = v.SpeedQualified
		}
		r.value.NSBLive[i] = value
	} else {
		if len(r.value.NSBLive) >= retainedResultsMaxRows {
			return errors.New("NSB retention exceeds 100000 rows; latest snapshot is incomplete")
		}
		r.indices[2][key] = len(r.value.NSBLive)
		r.value.NSBLive = append(r.value.NSBLive, v)
	}
	if rowIndex, exists := r.indices[3][key]; exists && r.value.NSBCSV != nil {
		// Patch only supplied fields; compact updates lack richer CSV columns.
		raw, _ := json.Marshal(v)
		var patch map[string]json.RawMessage
		json.Unmarshal(raw, &patch)
		row := r.value.NSBCSV.Rows[rowIndex]
		for i, field := range r.csvKeys {
			var text string
			if i < len(row) && json.Unmarshal(patch[field], &text) == nil && text != "" {
				row[i] = text
			}
		}
	}
	return nil
}

func (r *retainedResultsStore) flushLocked(s *appSession) error {
	if r.writer != s {
		return nil
	}
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	if r.incomplete {
		return errors.New(r.value.Error)
	}
	if err := r.writeLocked(r.value); err != nil {
		r.value.Saved, r.value.Error = false, err.Error()
		return err
	}
	r.value.Saved, r.value.Error = true, ""
	return nil
}

func (r *retainedResultsStore) checkpoint(s *appSession) {
	r.mu.Lock()
	if r.writer != s {
		r.mu.Unlock()
		return
	}
	err := r.flushLocked(s)
	generation, saved := r.value.Generation, r.value.Saved
	r.mu.Unlock()
	if err != nil {
		s.sendWSMessageDirect("retained_results_error", map[string]interface{}{"generation": generation, "message": err.Error()})
	} else {
		s.sendWSMessageDirect("retained_results_saved", map[string]interface{}{"generation": generation, "saved": saved})
	}
}

func (r *retainedResultsStore) finish(s *appSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer != s {
		return nil
	}
	s.testMutex.Lock()
	for _, result := range s.testResults {
		if result.Speed != "" {
			r.value.Measurements[retainedKey(result.IP, strconv.Itoa(result.Port))] = retainedSpeedMeasurement{MB: result.MeasuredSpeedMB, Measured: result.SpeedMeasured}
		}
	}
	s.testMutex.Unlock()
	err := r.flushLocked(s)
	s.restoreRetainedResults(r.value)
	r.writer = nil
	return err
}

func updateRetainedFilterProvenance(value *retainedResultsSnapshot, params map[string]interface{}) {
	set := func(key string, data interface{}) { raw, _ := json.Marshal(data); value.View[key] = raw }
	setMinimum := func(key string, data interface{}, fallback float64) {
		minimum, _ := data.(float64)
		if minimum <= 0 {
			minimum = fallback
		}
		set(key, minimum)
	}
	set("mode", value.Mode)
	if value.Mode == "nsb" {
		if compact, ok := params["compact"]; ok {
			set("nsbCompact", compact)
		}
		if minimum, ok := params["speedMin"]; ok {
			setMinimum("nsbSpeedMin", minimum, 0)
		}
		if count, ok := params["speedTest"]; ok {
			set("nsbSpeedEnabled", asInt(count) > 0)
		}
	} else {
		if minimum, ok := params["officialSpeedMin"]; ok {
			setMinimum("officialSpeedMin", minimum, 0.1)
		}
		if minimum, ok := params["speedMin"]; ok {
			setMinimum("officialSpeedMin", minimum, 0.1)
		}
		if count, ok := params["officialSpeedLimit"]; ok {
			enabled, _ := params["autoSpeed"].(bool)
			set("officialSpeedEnabled", enabled && asInt(count) > 0)
		}
		if count, ok := params["speedLimit"]; ok {
			set("officialSpeedEnabled", asInt(count) > 0)
		}
	}
}

func safeRetainedParams(params map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for _, key := range strings.Fields("ipType threads port delay scanMode autoSpeed fileName sourceURL outFile maxThreads fallbackPort speedTest speedURL enableTLS resultLimit dc speedMin speedLimit compact skipTested url datacenters finalLimit officialTargetDC officialSpeedPort officialSpeedLimit officialSpeedMin officialSpeedURL") {
		value, ok := params[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string, bool, int, int64, uint64:
		case float64:
			if math.IsNaN(typed) || math.IsInf(typed, 0) {
				continue
			}
		case []string:
			if key != "datacenters" || len(typed) > 1000 {
				continue
			}
		case []interface{}:
			if key != "datacenters" || len(typed) > 1000 {
				continue
			}
			valid := true
			for _, item := range typed {
				if text, ok := item.(string); !ok || !validRetainedDC(text) {
					valid = false
				}
			}
			if !valid {
				continue
			}
		default:
			continue
		}
		if text, ok := value.(string); ok {
			if len(text) > 4096 {
				continue
			}
			if key == "url" || strings.HasSuffix(strings.ToLower(key), "url") {
				u, err := url.Parse(text)
				if err != nil || u.User != nil {
					continue
				}
				u.RawQuery, u.Fragment = "", ""
				value = u.String()
			}
		}
		out[key] = value
	}
	return out
}

func validateRetainedView(view map[string]json.RawMessage) error {
	for key, raw := range view {
		valid := false
		if strings.TrimSpace(string(raw)) == "null" {
			return fmt.Errorf("invalid retained view field %q", key)
		}
		switch key {
		case "selectedCenters", "testedCenters", "activeCenters":
			var values []string
			valid = json.Unmarshal(raw, &values) == nil && len(values) <= 1000
			for _, dc := range values {
				if !validRetainedDC(dc) {
					valid = false
				}
			}
		case "currentDC":
			var value string
			valid = json.Unmarshal(raw, &value) == nil && (value == "" || validRetainedDC(value))
		case "mode", "activeTab", "officialQualified", "nsbQualified", "nsbIPType":
			var value string
			if json.Unmarshal(raw, &value) == nil {
				switch key {
				case "mode":
					valid = value == "official" || value == "nsb"
				case "activeTab":
					valid = value == "scan" || value == "test"
				case "nsbIPType":
					valid = value == "all" || value == "ipv4" || value == "ipv6"
				default:
					valid = value == "all" || value == "qualified" || value == "unqualified"
				}
			}
		case "officialSpeedEnabled", "nsbSpeedEnabled", "nsbCompact":
			var value bool
			valid = string(raw) != "null" && json.Unmarshal(raw, &value) == nil
		case "officialLatency", "nsbLatency", "officialLoss", "nsbLoss", "officialSpeedMin", "nsbSpeedMin":
			var value float64
			valid = string(raw) != "null" && json.Unmarshal(raw, &value) == nil && !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1e9
			if strings.HasSuffix(key, "Loss") && value > 100 {
				valid = false
			}
		case "officialSort", "nsbSort":
			var values []string
			valid = json.Unmarshal(raw, &values) == nil && len(values) <= 8
			for _, value := range values {
				if !strings.Contains(" latency loss speed dc ip port ", " "+value+" ") {
					valid = false
				}
			}
		}
		if !valid {
			return fmt.Errorf("invalid retained view field %q", key)
		}
	}
	return nil
}

func validRetainedDC(dc string) bool {
	if len(dc) != 3 {
		return false
	}
	for _, c := range dc {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func handleRetainedResultsAction(s *appSession, kind string, data json.RawMessage, authorized func() bool, reply func(string, interface{})) bool {
	if kind != "retained_results_get" && kind != "retained_results_view" && kind != "retained_results_clear" {
		return false
	}
	fail := func(err error) {
		payload := map[string]interface{}{"message": err.Error(), "action": kind}
		if errors.Is(err, errRetainedGeneration) {
			payload["code"] = "stale"
		}
		reply("retained_results_error", payload)
	}
	if authorized == nil || !authorized() {
		fail(errors.New("result retention requires an authorized same-origin connection"))
		return true
	}
	if s.retained == nil {
		s.retained = latestResults
	}
	var request struct {
		Generation *uint64                    `json:"generation"`
		View       map[string]json.RawMessage `json:"view"`
	}
	if kind != "retained_results_get" {
		if err := json.Unmarshal(data, &request); err != nil {
			fail(err)
			return true
		}
		if request.Generation == nil {
			fail(errors.New("retained results generation is required"))
			return true
		}
		var err error
		if kind == "retained_results_clear" {
			err = s.retained.clear(*request.Generation)
		} else {
			err = s.retained.setView(*request.Generation, request.View)
		}
		if err != nil {
			fail(err)
			return true
		}
		if kind == "retained_results_view" {
			reply("retained_results_saved", map[string]interface{}{"generation": *request.Generation, "saved": true})
			return true
		}
	}
	value, err := s.retained.get(s)
	if err != nil {
		fail(err)
	} else {
		reply("retained_results", value)
	}
	return true
}

func validateRetainedTaskGeneration(s *appSession, kind string, data json.RawMessage, authorized func() bool) error {
	if s.retained == nil {
		return nil
	}
	switch kind {
	case "start_task", "start_nsb_task", "start_test", "start_speed_test", "start_official_speed_batch", "start_nsb_speed_batch", "start_nsb_reallink", "start_web_batch":
	default:
		return nil
	}
	if authorized == nil || !authorized() {
		return errors.New("result retention requires an authorized same-origin connection")
	}
	var request struct {
		Generation *uint64 `json:"generation"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return err
	}
	s.retained.mu.Lock()
	defer s.retained.mu.Unlock()
	if err := s.retained.loadLocked(); err != nil {
		return err
	}
	if request.Generation == nil || *request.Generation != s.retained.value.Generation {
		return fmt.Errorf("%w; reload before starting the task", errRetainedGeneration)
	}
	s.retainedGeneration = *request.Generation
	return nil
}

func validateNSBInitialParameters(params startNSBTaskRequest) error {
	if params.FallbackPort < 1 || params.FallbackPort > 65535 {
		return errors.New("fallback port must be between 1 and 65535")
	}
	if params.ResultLimit <= 0 {
		return errors.New("NSB latency result limit must be positive")
	}
	return nil
}
