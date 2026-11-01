package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func retainedFixture(t *testing.T) (*retainedResultsStore, *appSession) {
	t.Helper()
	store := &retainedResultsStore{path: filepath.Join(t.TempDir(), "latest-results.json")}
	s := &appSession{retained: store, emit: func(string, interface{}) {}}
	return store, s
}

func TestRetainedResultsReloadRestoresCanonicalPoolsWithoutWriterAuthority(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", map[string]interface{}{"port": 443, "token": "DO_NOT_SAVE"}); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", TCPDuration: time.Millisecond})
	s.sendWSMessage("test_result", TestResult{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", Speed: "1.12MB/s", MeasuredSpeedMB: 1.1234, SpeedMeasured: true})
	s.sendWSMessage("scan_complete_wait_dc", []DataCenterInfo{{DataCenter: "HKG", IPCount: 1}})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(store.path)
	if err != nil || !strings.Contains(string(raw), "192.0.2.1") || strings.Contains(string(raw), "DO_NOT_SAVE") {
		t.Fatalf("plain snapshot/privacy: %s, %v", raw, err)
	}
	reloaded := &retainedResultsStore{path: store.path}
	reader := &appSession{retained: reloaded}
	got, err := reloaded.get(reader)
	if err != nil || got.Generation != 1 || !got.Saved || len(reader.scanResults) != 1 || len(reader.testResults) != 1 {
		t.Fatalf("restore failed: %+v %v", got, err)
	}
	if !reader.testResults[0].SpeedMeasured || reader.testResults[0].MeasuredSpeedMB != 1.1234 {
		t.Fatal("numeric speed provenance lost")
	}
	reader.sendWSMessage("test_result", TestResult{IP: "192.0.2.99"})
	got, _ = reloaded.get(reader)
	if len(got.Tests) != 1 {
		t.Fatal("restored reader acquired write authority")
	}
}

func TestRetainedResultsRunParametersRestoreSpeedFilterProvenance(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", map[string]interface{}{"autoSpeed": true, "officialSpeedLimit": 5, "officialSpeedMin": 2.5}); err != nil {
		t.Fatal(err)
	}
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	got, _ := store.get(s)
	if string(got.View["officialSpeedMin"]) != "2.5" || string(got.View["officialSpeedEnabled"]) != "true" {
		t.Fatalf("initial filter provenance lost: %+v", got.View)
	}
	if err := store.begin(s, false, "official", map[string]interface{}{"speedLimit": 1, "speedMin": 3.5}); err != nil {
		t.Fatal(err)
	}
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	got, _ = store.get(s)
	if string(got.View["officialSpeedMin"]) != "3.5" || string(got.View["officialSpeedEnabled"]) != "true" {
		t.Fatalf("continued filter provenance lost: %+v", got.View)
	}
}

func TestRetainedResultsPartialCheckpointRestoresCentersAndNormalizesDisabledSpeed(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", map[string]interface{}{"officialSpeedMin": -1.0, "officialSpeedLimit": 0}); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", TCPDuration: 12 * time.Millisecond})
	store.checkpoint(s)
	reloaded := &retainedResultsStore{path: store.path}
	got, err := reloaded.get(&appSession{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Centers) != 1 || got.Centers[0].DataCenter != "HKG" || got.Centers[0].MinLatency != 12 || string(got.View["officialSpeedMin"]) != "0.1" {
		t.Fatalf("partial restore cannot continue: %+v", got)
	}
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
}

func TestRetainedResultsSpeedFailureReplacesOldMeasurement(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("test_result", TestResult{IP: "192.0.2.1", Port: 443, Speed: "2MB/s", MeasuredSpeedMB: 2, SpeedMeasured: true})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	if err := store.begin(s, false, "official", nil); err != nil {
		t.Fatal(err)
	}
	s.testResults[0].Speed, s.testResults[0].MeasuredSpeedMB, s.testResults[0].SpeedMeasured = "failed", 0, false
	s.sendWSMessage("speed_test_result", map[string]string{"ip": "192.0.2.1", "speed": "failed"})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	reader := &appSession{}
	if _, err := store.get(reader); err != nil {
		t.Fatal(err)
	}
	if reader.testResults[0].SpeedMeasured || reader.testResults[0].MeasuredSpeedMB != 0 {
		t.Fatal("failed speed retained old success provenance")
	}
}

func TestRetainedResultsMergePreservesOtherCentersAndSpeed(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	for _, r := range []TestResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG"}, {IP: "192.0.2.2", Port: 443, DataCenter: "NRT"}} {
		s.sendWSMessage("test_result", r)
	}
	s.sendWSMessage("speed_test_result", map[string]string{"ip": "192.0.2.1", "speed": "2MB/s"})
	s.sendWSMessage("test_complete", []TestResult{{IP: "192.0.2.2", Port: 443, DataCenter: "NRT", AvgLatency: time.Millisecond}})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	got, _ := store.get(s)
	if len(got.Tests) != 2 || got.Tests[0].Speed != "2MB/s" || got.Tests[1].AvgLatency != time.Millisecond {
		t.Fatalf("merge discarded other center: %+v", got.Tests)
	}
}

func TestRetainedResultsRowsPreserveTheirActualScanModeAcrossContinuations(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", map[string]interface{}{"scanMode": scanModeHTTPing}); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1", Port: 443, DataCenter: "HKG"})
	s.sendWSMessage("test_result", TestResult{IP: "192.0.2.1", Port: 443, DataCenter: "HKG"})
	s.sendWSMessage("web_batch_result", map[string]interface{}{"result": TestResult{IP: "192.0.2.2", Port: 443, DataCenter: "NRT"}})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	if err := store.begin(s, false, "official", map[string]interface{}{"scanMode": scanModeTCPing}); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("test_result", TestResult{IP: "192.0.2.3", Port: 443, DataCenter: "SJC"})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	reloaded := &retainedResultsStore{path: store.path}
	got, err := reloaded.get(&appSession{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Scans[0].RecordedScanMode != scanModeHTTPing || got.Tests[0].RecordedScanMode != scanModeHTTPing || got.Tests[1].RecordedScanMode != scanModeHTTPing || got.Tests[2].RecordedScanMode != scanModeTCPing {
		t.Fatalf("form/latest task mode replaced row provenance: %+v %+v", got.Scans, got.Tests)
	}
}

func TestRetainedResultsEmitsActualRunModeWithoutMutatingSourceRows(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", map[string]interface{}{"scanMode": scanModeHTTPing}); err != nil {
		t.Fatal(err)
	}
	var events []interface{}
	s.emit = func(kind string, data interface{}) {
		if kind == "scan_result" || kind == "test_result" || kind == "test_complete" || kind == "web_batch_result" || kind == "web_batch_complete" {
			events = append(events, data)
		}
	}
	scan := ScanResult{IP: "192.0.2.1", Port: 443}
	test := TestResult{IP: "192.0.2.1", Port: 443}
	list := []TestResult{test, {IP: "192.0.2.2", Port: 443, RecordedScanMode: scanModeTCPing}}
	result := map[string]interface{}{"result": test, "qualified": false}
	batch := webBatchSnapshot{Results: []TestResult{test}}
	s.sendWSMessage("scan_result", scan)
	s.sendWSMessage("test_result", test)
	s.sendWSMessage("test_complete", list)
	s.sendWSMessage("web_batch_result", result)
	s.sendWSMessage("web_batch_complete", batch)
	if events[0].(ScanResult).RecordedScanMode != scanModeHTTPing || events[1].(TestResult).RecordedScanMode != scanModeHTTPing || events[2].([]TestResult)[0].RecordedScanMode != scanModeHTTPing || events[2].([]TestResult)[1].RecordedScanMode != scanModeTCPing || events[3].(map[string]interface{})["result"].(TestResult).RecordedScanMode != scanModeHTTPing || events[4].(webBatchSnapshot).Results[0].RecordedScanMode != scanModeHTTPing {
		t.Fatalf("emitted form-dependent/unmarked provenance: %+v", events)
	}
	if scan.RecordedScanMode != "" || test.RecordedScanMode != "" || list[0].RecordedScanMode != "" || result["result"].(TestResult).RecordedScanMode != "" || batch.Results[0].RecordedScanMode != "" {
		t.Fatal("annotation mutated caller-owned rows")
	}
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	reader := &appSession{retained: store, emit: func(string, interface{}) {}}
	if got := store.annotate(reader, "test_result", test).(TestResult); got.RecordedScanMode != "" {
		t.Fatal("read-only session acquired current writer provenance")
	}
}

func TestRetainedResultsSpeedEventsKeepPortsSeparateAndLegacyTouchesFirstOnly(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("test_result", TestResult{IP: "192.0.2.1", Port: 443, Speed: "1MB/s"})
	s.sendWSMessage("test_result", TestResult{IP: "192.0.2.1", Port: 8443, Speed: "2MB/s"})
	s.sendWSMessage("speed_test_result", map[string]string{"ip": "192.0.2.1", "port": "8443", "speed": "3MB/s"})
	s.sendWSMessage("speed_test_result", map[string]string{"ip": "192.0.2.1", "speed": "4MB/s"})
	s.sendWSMessage("speed_test_result", map[string]string{"ip": "192.0.2.1", "port": "9443", "speed": "5MB/s"})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	got, _ := store.get(s)
	if len(got.Tests) != 3 || got.Tests[0].Speed != "4MB/s" || got.Tests[1].Speed != "3MB/s" || got.Tests[2].Port != 9443 || got.Tests[2].Speed != "5MB/s" {
		t.Fatalf("retention conflated endpoint events: %+v", got.Tests)
	}
}

func TestRetainedResultsWebBatchKeepsMeasuredPoolNotFinalLimit(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	for _, r := range []TestResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG"}, {IP: "192.0.2.2", Port: 443, DataCenter: "NRT"}} {
		s.sendWSMessage("web_batch_result", map[string]interface{}{"result": r, "qualified": false})
	}
	s.sendWSMessage("web_batch_complete", webBatchSnapshot{Results: []TestResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", Speed: "2MB/s", MeasuredSpeedMB: 2, SpeedMeasured: true}}})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	got, _ := store.get(s)
	if len(got.Tests) != 2 || got.Tests[0].Speed != "2MB/s" {
		t.Fatalf("batch final limit discarded measured pool: %+v", got.Tests)
	}
}

func TestRetainedResultsStaleClearAndFailedNewScanPreservePrevious(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1"})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	if err := store.clear(0); err == nil {
		t.Fatal("stale clear accepted")
	}
	oldPath := store.path
	store.path = filepath.Join(t.TempDir(), "absent", "latest-results.json")
	if err := store.clear(1); err == nil {
		t.Fatal("failed clear reported success")
	}
	if err := store.begin(s, true, "nsb", nil); err == nil {
		t.Fatal("failed new scan admitted")
	}
	got, _ := store.get(s)
	if got.Generation != 1 || len(got.Scans) != 1 {
		t.Fatalf("failed save destroyed memory: %+v", got)
	}
	store.path = oldPath
	if err := store.clear(1); err != nil {
		t.Fatal(err)
	}
	got, _ = store.get(s)
	if got.Generation != 2 || len(got.Scans) != 0 || !got.Saved {
		t.Fatalf("clear not saved: %+v", got)
	}
	if err := store.begin(s, false, "official", nil); err == nil {
		t.Fatal("empty clear became continuation")
	}
}

func TestRetainedResultsReaderCannotDisplaceRunningWriter(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "nsb", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("nsb_scan_result", nsbScanMessage{IP: "192.0.2.1", Port: "443", DC: "HKG"})
	reader := &appSession{retained: store, emit: func(string, interface{}) {}}
	if _, err := store.get(reader); err != nil {
		t.Fatal(err)
	}
	if err := store.begin(reader, false, "nsb", nil); err == nil {
		t.Fatal("running writer displaced")
	}
	if err := store.clear(1); err == nil {
		t.Fatal("clear while running accepted")
	}
	reader.sendWSMessage("nsb_scan_result", nsbScanMessage{IP: "192.0.2.99", Port: "443"})
	s.sendWSMessage("nsb_scan_result", nsbScanMessage{IP: "192.0.2.1", Port: "443", Speed: "3MB/s"})
	s.sendWSMessage("nsb_csv_complete", nsbCSVCompletePayload{Headers: []string{"IP", "Port"}, Rows: [][]string{{"192.0.2.1", "443"}}, Status: "completed"})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	got, _ := store.get(reader)
	if len(got.NSBLive) != 1 || got.NSBLive[0].Speed != "3MB/s" || got.NSBLive[0].DC != "HKG" || got.NSBCSV == nil || len(reader.nsbRows) != 1 {
		t.Fatalf("NSB merge/pool lost: %+v", got)
	}
}

func TestRetainedResultsNSBSpeedUpdatesExistingCSVWithoutDiscardingColumnsOrLivePool(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "nsb", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("nsb_scan_result", nsbScanMessage{IP: "192.0.2.1", Port: "443", Speed: "1MB/s", SpeedQualified: false})
	s.sendWSMessage("nsb_scan_result", nsbScanMessage{IP: "192.0.2.1", Port: "8443", Speed: "2MB/s", SpeedQualified: true})
	s.sendWSMessage("nsb_scan_result", nsbScanMessage{IP: "192.0.2.99", Port: "443", Speed: "3MB/s", SpeedQualified: true})
	s.sendWSMessage("nsb_csv_complete", nsbCSVCompletePayload{Headers: []string{"下载速度", "端口号", "未知扩展", "IP地址", "TLS版本", "城市"}, Rows: [][]string{{"1MB/s", "443", "preserve-first", "192.0.2.1", "TLSv1.3", "Hong Kong"}, {"2MB/s", "8443", "preserve-second", "192.0.2.1", "TLSv1.2", "Tokyo"}}, QualifiedCount: 1})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	if err := store.begin(s, false, "nsb", map[string]interface{}{"speedMin": 2.0, "speedTest": 1}); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("nsb_scan_result", nsbScanMessage{IP: "192.0.2.1", Port: "443", Speed: "4MB/s", SpeedQualified: true, City: "Singapore"})
	s.sendWSMessage("nsb_speed_complete", map[string]interface{}{"qualified": 1, "totalQualified": 3, "limit": 1})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	reloaded := &retainedResultsStore{path: store.path}
	reader := &appSession{}
	got, err := reloaded.get(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.NSBLive) != 3 || !got.NSBLive[0].SpeedQualified || len(got.NSBCSV.Rows) != 2 || got.NSBCSV.QualifiedCount != 3 {
		t.Fatalf("CSV/live qualification lost: %+v", got)
	}
	if strings.Join(reader.nsbRows[0], "|") != "4MB/s|443|preserve-first|192.0.2.1|TLSv1.3|Singapore" || strings.Join(reader.nsbRows[1], "|") != "2MB/s|8443|preserve-second|192.0.2.1|TLSv1.2|Tokyo" {
		t.Fatalf("CSV stale or columns/order discarded: %+v", reader.nsbRows)
	}
}

func TestRetainedResultsDiskRowsRejectInvalidEndpointsNumbersAndMarkup(t *testing.T) {
	for _, body := range []string{
		`"scans":[{"IP":"<img src=x onerror=alert(1)>","Port":443}]`,
		`"scans":[{"IP":"192.0.2.1","Port":65536}]`,
		`"scans":[{"IP":"192.0.2.1","Port":443,"TCPDuration":-1}]`,
		`"scans":[{"IP":"192.0.2.1","Port":443,"DataCenter":"<x>"}]`,
		`"scans":[{"IP":"192.0.2.1","Port":443,"RecordedScanMode":"unknown"}]`,
		`"tests":[{"IP":"192.0.2.1","Port":443,"LossRate":1.1}]`,
		`"tests":[{"IP":"192.0.2.1","Port":443,"AvgLatency":-1}]`,
		`"tests":[{"IP":"192.0.2.1","Port":443,"Speed":"<svg onload=alert(1)>"}]`,
		`"nsbLive":[{"ip":"192.0.2.1","port":"0"}]`,
		`"nsbLive":[{"ip":"192.0.2.1","port":"443","lossRate":"NaN%"}]`,
		`"nsbLive":[{"ip":"192.0.2.1","port":"443","speed":"NaN"}]`,
		`"nsbCSV":{"headers":["IP地址","端口号"],"rows":[["bad-host","443"]]}`,
		`"nsbCSV":{"headers":["IP地址","端口号"],"rows":[["192.0.2.1"]]}`,
		`"nsbCSV":{"headers":["IP地址","端口号","下载速度"],"rows":[["192.0.2.1","443","NaNMB/s"]]}`,
		`"centers":[{"DataCenter":"<x>","IPCount":1,"MinLatency":10}]`,
		`"testMeasurements":{"192.0.2.1\u0000443":{"mb":-1,"measured":true}}`,
	} {
		store, _ := retainedFixture(t)
		raw := []byte(`{"generation":1,"mode":"official",` + body + `}`)
		if err := os.WriteFile(store.path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.get(&appSession{}); err == nil {
			t.Fatalf("unsafe persisted row accepted: %s", body)
		}
		after, _ := os.ReadFile(store.path)
		if string(after) != string(raw) {
			t.Fatal("rejected file modified")
		}
	}
}

func TestRetainedResultsViewRejectsSecretsAndMalformedFilters(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", map[string]interface{}{"sourceURL": "https://user:secret@example.com/data?token=secret", "port": 443}); err != nil {
		t.Fatal(err)
	}
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"token":"secret"}`, `{"officialLoss":101}`, `{"activeTab":"bogus"}`, `{"selectedCenters":["not-a-dc"]}`, `{"officialSort":["token"]}`} {
		var view map[string]json.RawMessage
		json.Unmarshal([]byte(raw), &view)
		if err := store.setView(1, view); err == nil {
			t.Fatalf("invalid view accepted: %s", raw)
		}
	}
	var view map[string]json.RawMessage
	json.Unmarshal([]byte(`{"selectedCenters":["HKG","NRT"],"testedCenters":["HKG"],"officialLoss":10,"activeTab":"test","nsbCompact":true}`), &view)
	if err := store.setView(1, view); err != nil {
		t.Fatal(err)
	}
	if err := store.setView(0, view); err == nil {
		t.Fatal("stale view accepted")
	}
	raw, _ := os.ReadFile(store.path)
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "user:") {
		t.Fatal("source credential persisted")
	}
}

func TestRetainedResultsCorruptAndOversizeLoadDoesNotOverwrite(t *testing.T) {
	store, _ := retainedFixture(t)
	for _, data := range [][]byte{[]byte(`{"generation":1,"mode":"wrong"}`), []byte(`not json`), make([]byte, retainedResultsMaxBytes+1)} {
		if err := os.WriteFile(store.path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.get(&appSession{}); err == nil {
			t.Fatal("invalid snapshot accepted")
		}
	}
}

func TestRetainedResultsFailedCheckpointRetriesWithoutDestroyingData(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1"})
	path := store.path
	store.path = filepath.Join(t.TempDir(), "missing", "latest-results.json")
	store.checkpoint(s)
	store.path = path
	if err := store.finish(s); err != nil {
		t.Fatalf("failed checkpoint permanently prevented retry: %v", err)
	}
	got, _ := store.get(s)
	if !got.Saved || got.Error != "" || len(got.Scans) != 1 {
		t.Fatalf("retry lost data: %+v", got)
	}
}

func TestRetainedResultsRejectedAdmissionDoesNotClearOrRun(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1"})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	blocking := &appSession{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !blocking.beginTask(cancel, "blocking", "", nil) {
		t.Fatal("could not hold admission")
	}
	defer blocking.endTask()
	var admission bool
	s.emit = func(kind string, data interface{}) {
		if kind == "retained_results_error" {
			admission, _ = data.(map[string]interface{})["admission"].(bool)
		}
	}
	s.startTaskNamed("官方优选扫描", "official", nil, func(context.Context, *appSession) { t.Error("rejected task ran") })
	got, _ := store.get(s)
	if got.Generation != 1 || len(got.Scans) != 1 || ctx.Err() != nil || !admission {
		t.Fatalf("rejected task cleared results: %+v", got)
	}
}

func TestRetainedResultsViewAcknowledgesWithoutReinstallingResults(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("test_result", TestResult{IP: "192.0.2.1", Port: 443})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	s.testResults = nil
	var kind string
	var payload interface{}
	reply := func(k string, data interface{}) { kind, payload = k, data }
	handleRetainedResultsAction(s, "retained_results_view", json.RawMessage(`{"generation":1,"view":{"activeTab":"test"}}`), func() bool { return true }, reply)
	if kind != "retained_results_saved" || len(s.testResults) != 0 {
		t.Fatal("view save reinstalled pools/returned destructive full snapshot")
	}
	handleRetainedResultsAction(s, "retained_results_view", json.RawMessage(`{"generation":0,"view":{}}`), func() bool { return true }, reply)
	data := payload.(map[string]interface{})
	if kind != "retained_results_error" || data["code"] != "stale" || data["action"] != "retained_results_view" {
		t.Fatalf("view conflict not labeled: %+v", data)
	}
}

func TestRetainedResultsNSBRejectsDeterministicInvalidParametersBeforeAdmission(t *testing.T) {
	for _, params := range []startNSBTaskRequest{{FallbackPort: 0, ResultLimit: 1}, {FallbackPort: 65536, ResultLimit: 1}, {FallbackPort: 443, ResultLimit: 0}} {
		if err := validateNSBInitialParameters(params); err == nil {
			t.Fatalf("invalid initial NSB parameters accepted: %+v", params)
		}
	}
	if err := validateNSBInitialParameters(startNSBTaskRequest{FallbackPort: 443, ResultLimit: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestRetainedResultsStartedOnlyAfterAdmissionAndSavedReset(t *testing.T) {
	store, s := retainedFixture(t)
	events := make(chan string, 20)
	s.emit = func(kind string, data interface{}) {
		if kind == "retained_results_started" {
			raw, err := os.ReadFile(store.path)
			if err != nil || !strings.Contains(string(raw), `"generation":1`) {
				t.Error("started before durable reset")
			}
		}
		events <- kind
	}
	s.startTaskNamed("官方优选扫描", "official", nil, func(_ context.Context, s *appSession) { s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1"}) })
	var order []string
	for {
		select {
		case kind := <-events:
			order = append(order, kind)
			if kind == "task_complete" {
				if order[0] != "retained_results_started" {
					t.Fatalf("wrong admission order: %v", order)
				}
				got, _ := store.get(s)
				if !got.Saved || len(got.Scans) != 1 {
					t.Fatalf("termination not flushed: %+v", got)
				}
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("admitted stub did not terminate")
		}
	}
}

func TestRetainedResultsActionsRequireAuthorizationAndCurrentGeneration(t *testing.T) {
	store, s := retainedFixture(t)
	if _, err := store.get(s); err != nil {
		t.Fatal(err)
	}
	var kind string
	reply := func(k string, _ interface{}) { kind = k }
	if !handleRetainedResultsAction(s, "retained_results_get", nil, func() bool { return false }, reply) || kind != "retained_results_error" {
		t.Fatal("unauthorized read accepted")
	}
	if err := validateRetainedTaskGeneration(s, "start_task", json.RawMessage(`{"generation":0}`), func() bool { return true }); err != nil {
		t.Fatal(err)
	}
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	if err := validateRetainedTaskGeneration(s, "start_task", json.RawMessage(`{"generation":0}`), func() bool { return true }); err == nil {
		t.Fatal("stale initial request accepted")
	}
	if err := validateRetainedTaskGeneration(s, "start_test", json.RawMessage(`{}`), func() bool { return true }); err == nil {
		t.Fatal("unversioned continuation accepted")
	}
	if err := validateRetainedTaskGeneration(s, "start_test", json.RawMessage(`{"generation":1}`), func() bool { return false }); err == nil {
		t.Fatal("unauthorized followed session inherited writer admission")
	}
	handleRetainedResultsAction(s, "retained_results_clear", json.RawMessage(`{"generation":1}`), func() bool { return false }, reply)
	got, _ := store.get(s)
	if got.Generation != 1 {
		t.Fatal("unauthorized clear mutated snapshot")
	}
}

func TestRetainedResultsBoundsReportIncompleteWithoutOverwritingPreviousFile(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	store.value.Scans = make([]ScanResult, retainedResultsMaxRows)
	var failure bool
	s.emit = func(kind string, _ interface{}) {
		if kind == "retained_results_error" {
			failure = true
		}
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1"})
	if err := store.finish(s); err == nil {
		t.Fatal("over-limit snapshot reported saved")
	}
	raw, _ := os.ReadFile(store.path)
	var disk retainedResultsSnapshot
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatal(err)
	}
	if !failure || len(disk.Scans) != 0 || store.value.Error == "" || store.value.Saved {
		t.Fatal("bounds silently truncated/overwrote previous file")
	}
}
