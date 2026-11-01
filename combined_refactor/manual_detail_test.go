package main

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestManualDetailScopeResolvesOnlyCurrentCandidatesAndPreservesCache(t *testing.T) {
	s := &appSession{scanResults: []ScanResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG"}, {IP: "192.0.2.2", Port: 443, DataCenter: "NRT"}}, testResults: []TestResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", Speed: "4MB/s"}}}
	got, err := resolveDetailDatacenters(s, []string{"NRT", "HKG", "NRT"})
	if err != nil || !reflect.DeepEqual(got, []string{"HKG", "NRT"}) {
		t.Fatalf("scope: %v %v", got, err)
	}
	if _, err := resolveDetailDatacenters(s, []string{"SIN"}); err == nil {
		t.Fatal("unknown center admitted")
	}
	if s.testResults[0].Speed != "4MB/s" {
		t.Fatal("scope selection erased cache")
	}
}

func TestManualScopePersistsSeparatelyFromSelectedCheckboxes(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	if err := store.setView(1, map[string]json.RawMessage{"activeCenters": json.RawMessage(`["HKG"]`)}); err != nil {
		t.Fatal(err)
	}
	got, err := store.get(s)
	if err != nil || string(got.View["activeCenters"]) != `["HKG"]` {
		t.Fatalf("scope not restored: %v", err)
	}
}

func TestHuaweiAppendDraftAndEditedFinalListDoNotReappendDeletedOldIPs(t *testing.T) {
	s, events := huaweiManualSession()
	var sent []string
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut {
			var body struct {
				Records []string `json:"records"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				return nil, err
			}
			sent = body.Records
		}
		return huaweiResponse(r, huaweiRecordFixture)
	})
	huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"batchId": 0, "ips": []string{"192.0.2.2", "192.0.2.1"}, "mode": "append"})
	event := huaweiManualWait(t, events)
	if event.kind != "huawei_response" {
		t.Fatalf("append rejected: %+v", event)
	}
	p := event.data.(map[string]interface{})["data"].(huaweiPreview)
	if !reflect.DeepEqual(p.New, []string{"192.0.2.1", "192.0.2.2"}) {
		t.Fatalf("append draft: %v", p.New)
	}
	huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"batchId": 0, "ips": []string{"192.0.2.8"}, "mode": "append", "final": true})
	event = huaweiManualWait(t, events)
	p = event.data.(map[string]interface{})["data"].(huaweiPreview)
	if !reflect.DeepEqual(p.New, []string{"192.0.2.8"}) {
		t.Fatalf("edited deletion undone: %v", p.New)
	}
	huaweiManualAction(t, s, store, "huawei_update", map[string]interface{}{"batchId": 0, "token": p.Token, "confirmed": true})
	if event := huaweiManualWait(t, events); event.kind != "web_publish_status" {
		t.Fatalf("update: %+v", event)
	}
	if !reflect.DeepEqual(sent, []string{"192.0.2.8"}) {
		t.Fatalf("wrong mutation: %v", sent)
	}
}

func TestScopedOfficialSpeedDoesNotTouchUnselectedCenters(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &appSession{testResults: []TestResult{{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", AvgLatency: time.Millisecond, Speed: "1MB/s"}, {IP: "192.0.2.2", Port: 443, DataCenter: "NRT", AvgLatency: time.Millisecond, Speed: "2MB/s"}}}
	s.emit = func(kind string, data interface{}) {
		if kind == "speed_test_result" {
			cancel()
		}
	}
	runOfficialSpeedBatch(ctx, s, 443, "http://[invalid", 10, 0.1, nil, false, []string{"NRT"})
	if s.testResults[0].Speed != "1MB/s" || s.testResults[1].Speed == "2MB/s" {
		t.Fatalf("speed scope ignored: %+v", s.testResults)
	}
}

func TestScopedSpeedBeforeLatencyPersistsNewCandidate(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", TCPDuration: time.Millisecond})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	if err := store.begin(s, false, "official", nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.emit = func(kind string, _ interface{}) {
		if kind == "speed_test_result" {
			cancel()
		}
	}
	runOfficialSpeedBatch(ctx, s, 443, "http://[invalid", 10, 0.1, nil, false, []string{"HKG"})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	got, err := store.get(s)
	if err != nil || len(got.Tests) != 1 || got.Tests[0].Speed == "" {
		t.Fatalf("candidate measurement lost: %+v %v", got.Tests, err)
	}
}

func TestSingleSpeedBeforeLatencyPersistsCandidate(t *testing.T) {
	store, s := retainedFixture(t)
	if err := store.begin(s, true, "official", nil); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", TCPDuration: time.Millisecond})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	if err := store.begin(s, false, "official", nil); err != nil {
		t.Fatal(err)
	}
	runSpeedTest(context.Background(), s, "192.0.2.1", 443, "http://[invalid")
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	got, err := store.get(s)
	if err != nil || len(got.Tests) != 1 || got.Tests[0].Speed == "" {
		t.Fatalf("single candidate lost: %+v %v", got.Tests, err)
	}
}
