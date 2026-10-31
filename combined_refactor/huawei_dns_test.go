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
	raw, err := os.ReadFile(s.path)
	if err != nil || strings.Contains(string(raw), "SK_PRIVATE_123456789") {
		t.Fatal("private file is not encrypted")
	}
	if err := s.configure(huaweiConfigRequest{Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef", Persist: true}); err != nil {
		t.Fatalf("explicit overwrite of saved private config failed: %v", err)
	}
	if err := s.configure(huaweiConfigRequest{Region: "https://attacker.test", ProjectID: "x"}); err == nil {
		t.Fatal("arbitrary signed endpoint accepted")
	}
}

func huaweiStatusMap(t *testing.T, s *huaweiDNSStore) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(s.status())
	if err != nil {
		t.Fatal(err)
	}
	var status map[string]interface{}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	return status
}

func TestHuaweiEncryptedSaveStatusTracksAppliedConfigAndFile(t *testing.T) {
	s := huaweiTestStore(t, nil)
	status := huaweiStatusMap(t, s)
	if status["encryptedSaveSupported"] != true || status["privatePath"] != s.path || status["privateFileStatus"] != "absent" || status["persisted"] != false {
		t.Fatalf("incorrect initial capability/status: %v", status)
	}
	if err := s.configure(huaweiConfigRequest{Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef", Persist: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.configure(huaweiConfigRequest{Region: "cn-east-3", ProjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}); err != nil {
		t.Fatal(err)
	}
	status = huaweiStatusMap(t, s)
	if status["persisted"] != false || status["privateFileStatus"] != "encrypted" {
		t.Fatalf("memory-only change misreported as saved: %v", status)
	}
	if err := s.load(); err != nil {
		t.Fatal(err)
	}
	status = huaweiStatusMap(t, s)
	if status["persisted"] != true || status["region"] != "cn-north-4" {
		t.Fatalf("reload did not restore saved config: %v", status)
	}
	if err := os.Remove(s.path); err != nil {
		t.Fatal(err)
	}
	status = huaweiStatusMap(t, s)
	if status["persisted"] != false || status["privateFileStatus"] != "absent" {
		t.Fatal("deleted private file still reported saved")
	}
}

func TestHuaweiEncryptedConfigCiphertextOverwriteAndTamperPreserveMemory(t *testing.T) {
	s := huaweiTestStore(t, nil)
	for _, c := range []huaweiConfigRequest{
		{Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef", Persist: true},
		{AK: "AK_REPLACED_987654321", SK: "SK_REPLACED_987654321", Region: "cn-east-3", ProjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Persist: true},
	} {
		if err := s.configure(c); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(s.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, plain := range []string{s.config.AK, s.config.SK, c.Region, c.ProjectID} {
			if strings.Contains(string(raw), plain) {
				t.Fatal("basic configuration escaped ciphertext")
			}
		}
		loaded := &huaweiDNSStore{path: s.path}
		if err := loaded.load(); err != nil || loaded.config != s.config {
			t.Fatal("ciphertext did not reload full configuration")
		}
	}
	before, revision := s.config, s.revision
	tampered := []byte("not portable ciphertext")
	if err := os.WriteFile(s.path, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.load(); err == nil || s.config != before || s.revision != revision {
		t.Fatal("failed decrypt replaced last good memory configuration")
	}
	raw, _ := os.ReadFile(s.path)
	if string(raw) != string(tampered) || huaweiStatusMap(t, s)["persisted"] != false {
		t.Fatal("failed reload rewrote file or reported applied config as saved")
	}
}

func TestHuaweiFailedSavePreservesConfigAndPrivateFile(t *testing.T) {
	s := huaweiTestStore(t, nil)
	if err := s.configure(huaweiConfigRequest{Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef", Persist: true}); err != nil {
		t.Fatal(err)
	}
	goodPath := s.path
	goodFile, _ := os.ReadFile(goodPath)
	before, revision := s.config, s.revision
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	s.path = filepath.Join(blocker, "huawei-dns.private")
	err := s.configure(huaweiConfigRequest{AK: "AK_FAILED_SECRET", SK: "SK_FAILED_SECRET", Region: "cn-east-3", ProjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Persist: true})
	if err == nil || s.config != before || s.revision != revision || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), blocker) {
		t.Fatal("failed save lost state or exposed request/path")
	}
	if got, _ := os.ReadFile(goodPath); string(got) != string(goodFile) {
		t.Fatal("failed save changed last good private file")
	}
	if got, _ := os.ReadFile(blocker); string(got) != "keep" {
		t.Fatal("failed save replaced existing file")
	}
}

func TestHuaweiFailedOverwritePreservesSameCiphertextFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows file sharing and DPAPI")
	}
	s := huaweiTestStore(t, nil)
	if err := s.configure(huaweiConfigRequest{Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef", Persist: true}); err != nil {
		t.Fatal(err)
	}
	before, revision := s.config, s.revision
	ciphertext, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	// Go's Windows file handle does not share deletion, so replacement must fail.
	locked, err := os.Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Close()
	if err := s.configure(huaweiConfigRequest{Region: "cn-east-3", ProjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Persist: true}); err == nil {
		t.Fatal("overwrite unexpectedly succeeded while destination was locked")
	}
	if s.config != before || s.revision != revision {
		t.Fatal("failed overwrite changed last good memory config")
	}
	if got, err := os.ReadFile(s.path); err != nil || string(got) != string(ciphertext) {
		t.Fatal("failed overwrite changed original ciphertext")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(s.path), ".huawei-private-*"))
	if err != nil || len(files) != 0 {
		t.Fatal("failed overwrite left temporary ciphertext behind")
	}
}

func TestHuaweiInvalidDecryptedConfigPreservesMemoryAndFile(t *testing.T) {
	s := huaweiTestStore(t, nil)
	before, revision := s.config, s.revision
	for _, raw := range []string{"not JSON", `{"ak":"new-ak","sk":"new-sk","region":"https://attacker.test","projectId":"0123456789abcdef0123456789abcdef"}`} {
		ciphertext, err := protectHuaweiPrivate([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(s.path, ciphertext, 0600); err != nil {
			t.Fatal(err)
		}
		if err := s.load(); err == nil || s.config != before || s.revision != revision {
			t.Fatal("invalid decrypted config replaced last good memory")
		}
		if got, err := os.ReadFile(s.path); err != nil || string(got) != string(ciphertext) {
			t.Fatal("invalid decrypted config rewrote private file")
		}
	}
}

func TestHuaweiLegacyPlaintextLoadAndExplicitEncryptedSave(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("legacy plaintext compatibility is non-Windows only")
	}
	s := huaweiTestStore(t, nil)
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		t.Fatal(err)
	}
	legacy, _ := json.Marshal(s.config)
	if err := os.WriteFile(s.path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.load(); err != nil {
		t.Fatal(err)
	}
	status := huaweiStatusMap(t, s)
	if status["encryptedSaveSupported"] != true || status["persisted"] != false || status["privateFileStatus"] != "legacy-plaintext" {
		t.Fatalf("legacy plaintext called encrypted: %v", status)
	}
	if got, _ := os.ReadFile(s.path); string(got) != string(legacy) {
		t.Fatal("legacy load rewrote plaintext")
	}
	if err := s.configure(huaweiConfigRequest{Region: "cn-east-3", ProjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Persist: true}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(s.path); string(got) == string(legacy) || !strings.HasPrefix(string(got), privateEnvelopeMagic) {
		t.Fatal("explicit save did not replace legacy plaintext with portable ciphertext")
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
