package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

type huaweiManualEvent struct {
	kind string
	data interface{}
}

func huaweiManualSession() (*appSession, chan huaweiManualEvent) {
	events := make(chan huaweiManualEvent, 16)
	return &appSession{dnsTarget: &huaweiRecord{ID: "record1", ZoneID: "zone1", Name: "edge.example.com.", Type: "A", Line: "default_view", revision: 1}, emit: func(kind string, data interface{}) {
		events <- huaweiManualEvent{kind, data}
	}}, events
}

func huaweiManualAction(t *testing.T, s *appSession, store *huaweiDNSStore, action string, request interface{}) {
	t.Helper()
	raw, err := json.Marshal(request)
	if err != nil || !handleHuaweiWebAction(context.Background(), s, store, action, raw, func() bool { return true }) {
		t.Fatalf("action %s: %v", action, err)
	}
}

func huaweiManualWait(t *testing.T, events <-chan huaweiManualEvent) huaweiManualEvent {
	t.Helper()
	for {
		select {
		case event := <-events:
			if event.kind == "web_publish_status" && event.data.(map[string]interface{})["status"] == "running" {
				continue
			}
			return event
		case <-time.After(2 * time.Second):
			t.Fatal("Huawei action did not finish")
		}
	}
}

func TestHuaweiManualPreviewPublishesExactOrderedSelectionWithoutBatch(t *testing.T) {
	for _, typ := range []string{"A", "AAAA"} {
		t.Run(typ, func(t *testing.T) {
			s, events := huaweiManualSession()
			s.dnsTarget.Type = typ
			fixture := huaweiRecordFixture
			ips := []string{"2001:db8::8", "192.0.2.8", "192.0.2.2", "2001:0db8::2", "192.0.2.8", "2001:db8::2"}
			want := []string{"192.0.2.8", "192.0.2.2"}
			if typ == "AAAA" {
				fixture = strings.ReplaceAll(strings.ReplaceAll(fixture, `"type":"A"`, `"type":"AAAA"`), "192.0.2.1", "2001:db8::1")
				want = []string{"2001:db8::8", "2001:db8::2"}
			}
			var published []string
			store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPut {
					var body struct {
						Records []string `json:"records"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						return nil, err
					}
					published = body.Records
				}
				return huaweiResponse(r, fixture)
			})
			huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": ips, "batchId": 0})
			event := huaweiManualWait(t, events)
			if event.kind != "huawei_response" {
				t.Fatalf("ordinary exact selection rejected: %+v", event)
			}
			p := event.data.(map[string]interface{})["data"].(huaweiPreview)
			if p.Token == "" || p.BatchID != 0 || !reflect.DeepEqual(p.New, want) {
				t.Fatalf("exact ordered preview: %#v", p)
			}
			huaweiManualAction(t, s, store, "huawei_update", map[string]interface{}{"token": p.Token, "confirmed": true, "batchId": 0, "ips": []string{"192.0.2.99"}})
			event = huaweiManualWait(t, events)
			if event.kind != "web_publish_status" || event.data.(map[string]interface{})["status"] != "updated" || event.data.(map[string]interface{})["batchId"] != uint64(0) || !reflect.DeepEqual(published, want) {
				t.Fatalf("preview-bound publication: %+v, records=%v", event, published)
			}
			huaweiManualAction(t, s, store, "huawei_update", map[string]interface{}{"token": p.Token, "confirmed": true, "batchId": 0})
			if event := huaweiManualWait(t, events); event.kind != "huawei_error" {
				t.Fatalf("one-time token replay accepted: %+v", event)
			}
		})
	}
}

func TestHuaweiManualSelectionSupportsMoreThanTenThousandIPs(t *testing.T) {
	s, events := huaweiManualSession()
	s.dnsTarget.Type = "AAAA"
	fixture := strings.ReplaceAll(strings.ReplaceAll(huaweiRecordFixture, `"type":"A"`, `"type":"AAAA"`), "192.0.2.1", "2001:db8::1")
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) { return huaweiResponse(r, fixture) })
	ips := make([]string, 12001)
	for i := range ips {
		ips[i] = fmt.Sprintf("2001:db8:1::%x", i+1)
	}
	huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": ips, "batchId": 0})
	event := huaweiManualWait(t, events)
	if event.kind != "huawei_response" || !reflect.DeepEqual(event.data.(map[string]interface{})["data"].(huaweiPreview).New, ips) {
		t.Fatalf("large ordinary selection rejected or capped: %+v", event)
	}
}

func TestHuaweiManualPreviewRejectsInvalidBareIPsBeforeCloud(t *testing.T) {
	for _, ips := range [][]string{nil, {}, {"edge.example.com"}, {"192.0.2.2:443"}, {"[2001:db8::2]"}, {"::ffff:192.0.2.2"}, {"fe80::1%eth0"}, {"0.0.0.0"}, {"ff02::1"}, {"192.0.2.2", "invalid"}, {" 192.0.2.2"}} {
		t.Run(fmt.Sprint(ips), func(t *testing.T) {
			s, events := huaweiManualSession()
			calls := 0
			store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
				calls++
				return huaweiResponse(r, huaweiRecordFixture)
			})
			huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": ips, "batchId": 0})
			if event := huaweiManualWait(t, events); event.kind != "huawei_error" || calls != 0 {
				t.Fatalf("invalid input reached provider: %+v calls=%d", event, calls)
			}
		})
	}
}

func TestHuaweiManualNoMatchingFamilyNeverClearsRecord(t *testing.T) {
	s, events := huaweiManualSession()
	writes := 0
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			writes++
		}
		return huaweiResponse(r, huaweiRecordFixture)
	})
	huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": []string{"2001:db8::2"}, "batchId": 0})
	event := huaweiManualWait(t, events)
	if event.kind != "huawei_response" {
		t.Fatalf("family-only mismatch rejected: %+v", event)
	}
	p := event.data.(map[string]interface{})["data"].(huaweiPreview)
	if p.Changed || len(p.New) != 0 {
		t.Fatalf("family mismatch preview: %+v", p)
	}
	huaweiManualAction(t, s, store, "huawei_update", map[string]interface{}{"token": p.Token, "confirmed": true, "batchId": 0})
	event = huaweiManualWait(t, events)
	if event.kind != "web_publish_status" || event.data.(map[string]interface{})["status"] != "skipped" || writes != 0 {
		t.Fatalf("family mismatch mutated record: %+v writes=%d", event, writes)
	}
}

func TestHuaweiManualPreviewRequiresIdleTaskAndCannotBorrowBatch(t *testing.T) {
	s, events := huaweiManualSession()
	s.isTaskRunning = true
	calls := 0
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return huaweiResponse(r, huaweiRecordFixture)
	})
	huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": []string{"192.0.2.2"}, "batchId": 0})
	if event := huaweiManualWait(t, events); event.kind != "huawei_error" || calls != 0 {
		t.Fatalf("running task accepted manual preview: %+v calls=%d", event, calls)
	}
	s.isTaskRunning = false
	huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": []string{"192.0.2.2"}, "batchId": 7})
	if event := huaweiManualWait(t, events); event.kind != "huawei_error" || calls != 0 {
		t.Fatalf("explicit IPs bypassed legacy batch ID: %+v calls=%d", event, calls)
	}
}

func TestHuaweiManualConsumedPreviewCannotSurviveTaskOrDisconnectInvalidation(t *testing.T) {
	for _, invalidation := range []string{"new task", "stop", "disconnect", "reselect"} {
		t.Run(invalidation, func(t *testing.T) {
			s, events := huaweiManualSession()
			writes := 0
			store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet {
					writes++
				}
				return huaweiResponse(r, huaweiRecordFixture)
			})
			huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": []string{"192.0.2.2"}, "batchId": 0})
			event := huaweiManualWait(t, events)
			if event.kind != "huawei_response" {
				t.Fatalf("preview failed: %+v", event)
			}
			p := event.data.(map[string]interface{})["data"].(huaweiPreview)
			// Model an update consumed just before its publication goroutine starts.
			s.taskMutex.Lock()
			s.dnsPreview = nil
			s.taskMutex.Unlock()
			switch invalidation {
			case "new task":
				if !s.beginTask(func() {}, "new task", "official", nil) {
					t.Fatal("new task rejected")
				}
				s.endTask()
			case "stop":
				s.stopTask()
			case "disconnect":
				s.cancelWebPublication()
				s.refreshWebPublicationAfterReconnect()
			case "reselect":
				s.dnsRecords = []huaweiRecord{*s.dnsTarget}
				huaweiManualAction(t, s, store, "huawei_select", map[string]string{"zoneId": "zone1", "recordId": "record1"})
				if event := huaweiManualWait(t, events); event.kind != "huawei_response" {
					t.Fatalf("reselection failed: %+v", event)
				}
			}
			s.publishWebDNS(context.Background(), store, 0, &p, false, func() bool { return true })
			event = huaweiManualWait(t, events)
			if event.kind != "web_publish_status" || event.data.(map[string]interface{})["status"] != "denied" || writes != 0 {
				t.Fatalf("consumed token survived invalidation: %+v writes=%d", event, writes)
			}
		})
	}
}

func TestHuaweiManualMutationRechecksCloudTargetConfigurationAndPermission(t *testing.T) {
	for _, change := range []string{"cloud record", "target", "configuration", "permission", "disconnect"} {
		t.Run(change, func(t *testing.T) {
			s, events := huaweiManualSession()
			reads, writes := 0, 0
			allowed := true
			var store *huaweiDNSStore
			store = huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet {
					writes++
					return huaweiResponse(r, huaweiRecordFixture)
				}
				reads++
				if reads == 2 {
					switch change {
					case "cloud record":
						return huaweiResponse(r, strings.Replace(huaweiRecordFixture, `"ttl":300`, `"ttl":600`, 1))
					case "target":
						s.taskMutex.Lock()
						copy := *s.dnsTarget
						copy.Line = "other_line"
						s.dnsTarget = &copy
						s.taskMutex.Unlock()
					case "configuration":
						if err := store.configure(huaweiConfigRequest{Region: "cn-east-3", ProjectID: "0123456789abcdef0123456789abcdef"}); err != nil {
							return nil, err
						}
					case "permission":
						allowed = false
					case "disconnect":
						s.cancelWebPublication()
					}
				}
				return huaweiResponse(r, huaweiRecordFixture)
			})
			huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": []string{"192.0.2.2"}, "batchId": 0})
			event := huaweiManualWait(t, events)
			if event.kind != "huawei_response" {
				t.Fatalf("preview failed: %+v", event)
			}
			p := event.data.(map[string]interface{})["data"].(huaweiPreview)
			s.publishWebDNS(context.Background(), store, 0, &p, false, func() bool { return allowed })
			event = huaweiManualWait(t, events)
			if event.kind != "web_publish_status" || event.data.(map[string]interface{})["status"] != "failed" || writes != 0 {
				t.Fatalf("changed publication gate bypassed: %+v writes=%d", event, writes)
			}
		})
	}
}

func TestHuaweiManualFailedPreviewSendLeavesNoToken(t *testing.T) {
	s, _ := huaweiManualSession()
	s.emit = nil
	read := make(chan struct{})
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		close(read)
		return huaweiResponse(r, huaweiRecordFixture)
	})
	huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": []string{"192.0.2.2"}, "batchId": 0})
	select {
	case <-read:
	case <-time.After(time.Second):
		t.Fatal("provider preview never entered")
	}
	s.dnsMutex.Lock()
	defer s.dnsMutex.Unlock()
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	if s.dnsPreview != nil {
		t.Fatal("undelivered preview retained a usable token")
	}
}

func TestHuaweiManualPreviewEchoesRequestID(t *testing.T) {
	s, events := huaweiManualSession()
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) { return huaweiResponse(r, huaweiRecordFixture) })
	huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": []string{"192.0.2.2"}, "batchId": 0, "requestId": "dns-42"})
	event := huaweiManualWait(t, events)
	if event.kind != "huawei_response" {
		t.Fatalf("preview failed: %+v", event)
	}
	raw, err := json.Marshal(event.data.(map[string]interface{})["data"])
	var reply map[string]interface{}
	if err != nil || json.Unmarshal(raw, &reply) != nil || reply["requestId"] != "dns-42" {
		t.Fatalf("preview request correlation missing: %s, %v", raw, err)
	}
}

func TestHuaweiManualPreviewRejectsInvalidRequestIDBeforeCloud(t *testing.T) {
	for _, id := range []string{strings.Repeat("a", 129), "dns\n1", "dns 1", "dns.1", "dns-\u4e00"} {
		t.Run(id, func(t *testing.T) {
			s, events := huaweiManualSession()
			calls := 0
			store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
				calls++
				return huaweiResponse(r, huaweiRecordFixture)
			})
			huaweiManualAction(t, s, store, "huawei_preview", map[string]interface{}{"ips": []string{"192.0.2.2"}, "batchId": 0, "requestId": id})
			if event := huaweiManualWait(t, events); event.kind != "huawei_error" || calls != 0 {
				t.Fatalf("invalid correlation ID reached cloud: %+v calls=%d", event, calls)
			}
		})
	}
}
