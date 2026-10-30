package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

type huaweiTestTransport func(*http.Request) (*http.Response, error)

func (f huaweiTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const huaweiRecordFixture = `{"id":"record1","zone_id":"zone1","zone_name":"example.com.","name":"edge.example.com.","type":"A","ttl":300,"line":"default_view","weight":5,"description":"keep me","status":"ACTIVE","records":["192.0.2.1"]}`

func huaweiTestStore(t *testing.T, handler huaweiTestTransport) *huaweiDNSStore {
	t.Helper()
	s := &huaweiDNSStore{path: filepath.Join(t.TempDir(), "private", "huawei-dns.private"), transport: handler}
	if err := s.configure(huaweiConfigRequest{AK: "AK_PRIVATE_123456789", SK: "SK_PRIVATE_123456789", Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef"}); err != nil {
		t.Fatal(err)
	}
	return s
}

func huaweiResponse(r *http.Request, body string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

func TestHuaweiCredentialsPreservedRedactedAndPrivateReload(t *testing.T) {
	s := huaweiTestStore(t, nil)
	if err := s.configure(huaweiConfigRequest{Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef", Persist: true}); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(s.status())
	if strings.Contains(string(b), "AK_PRIVATE_123456789") || strings.Contains(string(b), "SK_PRIVATE") {
		t.Fatalf("credentials exposed: %s", b)
	}
	loaded := &huaweiDNSStore{path: s.path}
	if err := loaded.load(); err != nil {
		t.Fatal(err)
	}
	if loaded.config.AK != "AK_PRIVATE_123456789" || loaded.config.SK != "SK_PRIVATE_123456789" {
		t.Fatal("blank inputs lost saved credentials")
	}
	if _, err := os.Stat(s.path); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		raw, err := os.ReadFile(s.path)
		if err != nil || strings.Contains(string(raw), "SK_PRIVATE_123456789") {
			t.Fatal("Windows private file is not encrypted")
		}
	} else {
		file, _ := os.Stat(s.path)
		dir, _ := os.Stat(filepath.Dir(s.path))
		if file.Mode().Perm() != 0600 || dir.Mode().Perm() != 0700 {
			t.Fatal("private configuration permissions are too broad")
		}
	}
	if err := s.configure(huaweiConfigRequest{Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef", Persist: true}); err != nil {
		t.Fatalf("explicit overwrite of saved private config failed: %v", err)
	}
	if err := s.configure(huaweiConfigRequest{Region: "https://attacker.test", ProjectID: "x"}); err == nil {
		t.Fatal("arbitrary signed endpoint accepted")
	}
}

func TestHuaweiShortAKNeverReturnedInFull(t *testing.T) {
	s := huaweiTestStore(t, nil)
	if err := s.configure(huaweiConfigRequest{AK: "key!", Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(s.status())
	if strings.Contains(string(raw), "key!") {
		t.Fatalf("full short AK exposed: %s", raw)
	}
}

func TestHuaweiSDKDiscoveryAndRecordsOnlyUpdate(t *testing.T) {
	var writes int
	s := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "dns.cn-north-4.myhuaweicloud.com" || r.Header.Get("X-Project-Id") != "0123456789abcdef0123456789abcdef" || r.Header.Get("Authorization") == "" {
			t.Fatalf("incorrect DNS authentication: %s %v", r.URL, r.Header)
		}
		if _, ok := r.Context().Deadline(); !ok {
			t.Fatal("SDK call lacks deadline")
		}
		if r.Method == http.MethodPut {
			writes++
			if r.URL.Path != "/v2.1/zones/zone1/recordsets/record1" {
				t.Fatal(r.URL.Path)
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			want := map[string]interface{}{"name": "edge.example.com.", "type": "A", "records": []interface{}{"192.0.2.2"}}
			if !reflect.DeepEqual(body, want) {
				t.Fatalf("properties changed or ports leaked: %#v", body)
			}
			return huaweiResponse(r, huaweiRecordFixture)
		}
		switch r.URL.Path {
		case "/v2/zones":
			return huaweiResponse(r, `{"zones":[{"id":"zone1","name":"example.com.","zone_type":"public"}]}`)
		case "/v2.1/zones/zone1/recordsets":
			return huaweiResponse(r, `{"recordsets":[`+huaweiRecordFixture+`]}`)
		default:
			return huaweiResponse(r, huaweiRecordFixture)
		}
	})
	ctx := context.Background()
	zones, err := s.zones(ctx)
	if err != nil || len(zones) != 1 || zones[0].ID != "zone1" {
		t.Fatalf("zones: %v %v", zones, err)
	}
	records, err := s.records(ctx, "zone1")
	if err != nil || len(records) != 1 || records[0].Line != "default_view" {
		t.Fatalf("line discovery: %v %v", records, err)
	}
	preview, err := s.preview(ctx, records[0], []TestResult{{IP: "192.0.2.2", Port: 443}, {IP: "2001:db8::2", Port: 443}})
	if err != nil || !reflect.DeepEqual(preview.New, []string{"192.0.2.2"}) {
		t.Fatalf("preview: %#v %v", preview, err)
	}
	if _, err := s.update(ctx, preview, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatalf("writes=%d", writes)
	}
}

func TestHuaweiUnchangedEmptyIdentityAndCancellationPreserveOldDNS(t *testing.T) {
	var current = huaweiRecordFixture
	var writes int
	s := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			writes++
		}
		return huaweiResponse(r, current)
	})
	target := huaweiRecord{ID: "record1", ZoneID: "zone1", Name: "edge.example.com.", Type: "A", Line: "default_view", revision: 1}
	for _, results := range [][]TestResult{{{IP: "192.0.2.1"}}, {{IP: "2001:db8::1"}}, nil} {
		p, err := s.preview(context.Background(), target, results)
		if err != nil {
			t.Fatal(err)
		}
		status, err := s.update(context.Background(), p, func() error { return nil })
		if err != nil || status != "skipped" {
			t.Fatalf("%s %v", status, err)
		}
	}
	p, err := s.preview(context.Background(), target, []TestResult{{IP: "192.0.2.2"}})
	if err != nil {
		t.Fatal(err)
	}
	current = strings.Replace(huaweiRecordFixture, `"ttl":300`, `"ttl":600`, 1)
	if _, err := s.update(context.Background(), p, func() error { return nil }); err == nil {
		t.Fatal("stale preview accepted")
	}
	current = strings.Replace(huaweiRecordFixture, `"line":"default_view"`, `"line":"other"`, 1)
	if _, err := s.preview(context.Background(), target, []TestResult{{IP: "192.0.2.2"}}); err == nil {
		t.Fatal("changed line accepted")
	}
	current = huaweiRecordFixture
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.update(ctx, p, func() error { return nil }); err == nil {
		t.Fatal("cancelled update accepted")
	}
	if _, err := s.update(context.Background(), p, func() error { return context.Canceled }); err == nil {
		t.Fatal("invalidated snapshot accepted")
	}
	if writes != 0 {
		t.Fatalf("unexpected mutations %d", writes)
	}
}

func TestHuaweiChangingServiceConfigRequiresRediscovery(t *testing.T) {
	s := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/v2.1/zones/zone1/recordsets" {
			return huaweiResponse(r, `{"recordsets":[`+huaweiRecordFixture+`]}`)
		}
		return huaweiResponse(r, huaweiRecordFixture)
	})
	rows, err := s.records(context.Background(), "zone1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.configure(huaweiConfigRequest{Region: "cn-east-3", ProjectID: "0123456789abcdef0123456789abcdef"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.preview(context.Background(), rows[0], []TestResult{{IP: "192.0.2.2"}}); err == nil {
		t.Fatal("old service target reused after configuration changed")
	}
}

func TestHuaweiPureIPFamilies(t *testing.T) {
	rows := []TestResult{{IP: "192.0.2.2"}, {IP: "192.0.2.2"}, {IP: "192.0.2.2:443"}, {IP: "::ffff:192.0.2.3"}, {IP: "2001:db8::2"}, {IP: "fe80::1%eth0"}}
	if got := huaweiIPs(rows, "A"); !reflect.DeepEqual(got, []string{"192.0.2.2"}) {
		t.Fatal(got)
	}
	if got := huaweiIPs(rows, "AAAA"); !reflect.DeepEqual(got, []string{"2001:db8::2"}) {
		t.Fatal(got)
	}
}

func TestHuaweiBoundedSanitizedFailures(t *testing.T) {
	s := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) { return nil, io.ErrUnexpectedEOF })
	start := time.Now()
	_, err := s.zones(context.Background())
	if err == nil || strings.Contains(err.Error(), "AK_PRIVATE") || time.Since(start) > time.Second {
		t.Fatalf("unsafe/unbounded error: %v", err)
	}
}

func TestHuaweiInFlightRequestHonorsCancellationWithoutDetachedMutation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	s := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	done := make(chan error, 1)
	go func() { _, err := s.zones(ctx); done <- err }()
	<-entered
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("SDK transport detached from operation cancellation")
	}
}

func TestHuaweiDiscoveryPaginatesPublicZonesAndLineRecords(t *testing.T) {
	zoneReads, recordReads := 0, 0
	s := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("limit") != "500" {
			t.Fatal("discovery page size missing")
		}
		if r.URL.Path == "/v2/zones" {
			zoneReads++
			if r.URL.Query().Get("type") != "public" {
				t.Fatal("non-public discovery")
			}
			if zoneReads == 1 {
				return huaweiResponse(r, `{"zones":[`+strings.TrimSuffix(strings.Repeat(`{"id":"zone1","name":"example.com.","zone_type":"public"},`, 500), ",")+`]}`)
			}
			if r.URL.Query().Get("marker") != "zone1" {
				t.Fatal("zone pagination marker missing")
			}
			return huaweiResponse(r, `{"zones":[{"id":"zone2","name":"other.example.","zone_type":"public"}]}`)
		}
		recordReads++
		if recordReads == 1 {
			return huaweiResponse(r, `{"recordsets":[`+strings.TrimSuffix(strings.Repeat(huaweiRecordFixture+",", 500), ",")+`]}`)
		}
		if r.URL.Query().Get("marker") != "record1" {
			t.Fatal("record pagination marker missing")
		}
		return huaweiResponse(r, `{"recordsets":[`+strings.Replace(huaweiRecordFixture, `"id":"record1"`, `"id":"record2"`, 1)+`]}`)
	})
	zones, err := s.zones(context.Background())
	if err != nil || len(zones) != 501 || zones[500].ID != "zone2" {
		t.Fatalf("pagination zones: %d %v", len(zones), err)
	}
	records, err := s.records(context.Background(), "zone1")
	if err != nil || len(records) != 501 || records[500].ID != "record2" {
		t.Fatalf("pagination records: %d %v", len(records), err)
	}
}
