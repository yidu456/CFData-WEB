package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHuaweiExplicitEncryptedSaveNeverCallsCloudOrReturnsSecrets(t *testing.T) {
	calls := 0
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return huaweiResponse(r, huaweiRecordFixture)
	})
	var kind string
	var payload interface{}
	s := &appSession{emit: func(k string, p interface{}) { kind, payload = k, p }}
	for _, persist := range []bool{false, true} {
		request, _ := json.Marshal(huaweiConfigRequest{Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef", Persist: persist})
		if !handleHuaweiWebAction(context.Background(), s, store, "huawei_config_set", request, func() bool { return true }) {
			t.Fatal("configuration action unhandled")
		}
		if calls != 0 {
			t.Fatal("saving config contacted cloud")
		}
		raw, _ := json.Marshal(payload)
		if strings.Contains(string(raw), "AK_PRIVATE_123456789") || strings.Contains(string(raw), "SK_PRIVATE") {
			t.Fatal("configuration response exposed credentials")
		}
		if kind != "huawei_response" {
			t.Fatal("configuration failed")
		}
	}
}

func TestHuaweiAllActionsRequireStrictOriginAndLocalOrExistingAuth(t *testing.T) {
	oldUser, oldPass := webUser, webPassword
	defer func() { webUser, webPassword = oldUser, oldPass }()
	webUser, webPassword = "", ""
	for _, tc := range []struct {
		host, origin, remote string
		want                 bool
	}{
		{"127.0.0.1:13335", "http://127.0.0.1:13335", "127.0.0.1:50000", true},
		{"localhost:13335", "http://localhost:13335", "[::1]:50000", true},
		{"evil.test:13335", "http://evil.test:13335", "127.0.0.1:50000", false},
		{"127.0.0.1:13335", "http://evil.test", "127.0.0.1:50000", false},
		{"127.0.0.1:13335", "", "127.0.0.1:50000", false},
		{"127.0.0.1:13335", "http://127.0.0.1:13335", "192.0.2.3:50000", false},
	} {
		r := httptest.NewRequest("GET", "http://"+tc.host+"/ws", nil)
		r.Host = tc.host
		r.RemoteAddr = tc.remote
		r.Header.Set("Origin", tc.origin)
		if got := huaweiWebAuthorized(r); got != tc.want {
			t.Fatalf("%+v got %v", tc, got)
		}
	}
	webUser, webPassword = "user", "password"
	r := httptest.NewRequest("GET", "https://example.com/ws", nil)
	r.Header.Set("Origin", "https://example.com")
	r.RemoteAddr = "192.0.2.3:50000"
	if huaweiWebAuthorized(r) {
		t.Fatal("remote without login authorized")
	}
	token, _, _ := createAuthSession("user")
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	if !huaweiWebAuthorized(r) {
		t.Fatal("existing authenticated remote rejected")
	}
	deleteAuthSession(token)
	if huaweiWebAuthorized(r) {
		t.Fatal("revoked login retains DNS permission")
	}
}

func TestHuaweiWebDeniesAllActionsBeforeCloudOrSecrets(t *testing.T) {
	called := 0
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) { called++; return huaweiResponse(r, huaweiRecordFixture) })
	s := &appSession{emit: func(string, interface{}) {}}
	for _, action := range []string{"huawei_config_get", "huawei_config_set", "huawei_test", "huawei_zones", "huawei_records", "huawei_select", "huawei_preview", "huawei_update", "web_publish_options", "web_github_publish"} {
		if !handleHuaweiWebAction(context.Background(), s, store, action, json.RawMessage(`{}`), func() bool { return false }) {
			t.Fatal(action)
		}
	}
	if called != 0 {
		t.Fatal("unauthorized cloud call")
	}
}

func TestWebPublishingUsesSnapshotAndIndependentOnceOnlyAttempts(t *testing.T) {
	s := &appSession{webBatch: &webBatchSnapshot{ID: 7, CompletedAt: time.Now(), Results: []TestResult{{IP: "192.0.2.2", Port: 443, DataCenter: "NRT", Speed: "2MB/s"}}}}
	if _, err := s.claimWebPublication(7, "dns", true, func() bool { return true }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.claimWebPublication(7, "dns", true, func() bool { return true }); err == nil {
		t.Fatal("replayed auto DNS publication accepted")
	}
	if _, err := s.claimWebPublication(7, "github", true, func() bool { return true }); err != nil {
		t.Fatal("DNS attempt blocked independent GitHub")
	}
	if _, err := s.claimWebPublication(8, "github", false, func() bool { return true }); err == nil {
		t.Fatal("foreign/stale batch accepted")
	}
	s.isTaskRunning = true
	if _, err := s.claimWebPublication(7, "github", false, func() bool { return true }); err == nil {
		t.Fatal("incomplete task accepted")
	}
	s.isTaskRunning = false
	if _, err := s.completedWebBatchResults(7); err != nil {
		t.Fatal("publisher invalidated batch")
	}
	content, err := webGitHubContent(*s.webBatch, "txt")
	if err != nil || content != "192.0.2.2:443#NRT\n" {
		t.Fatalf("backend content %q %v", content, err)
	}
}

func TestHuaweiNewTaskAndStopCancelPendingMutation(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			reads, writes := 0, 0
			s := &appSession{webBatch: &webBatchSnapshot{ID: 7, CompletedAt: time.Now(), Results: []TestResult{{IP: "192.0.2.2"}}}}
			s.dnsTarget = &huaweiRecord{ID: "record1", ZoneID: "zone1", Name: "edge.example.com.", Type: "A", Line: "default_view", revision: 1}
			store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPut {
					writes++
				}
				reads++
				if reads == 2 {
					if stop {
						s.stopTask()
					} else {
						if !s.beginTask(func() {}, "new task", "official", nil) {
							t.Fatal("new task rejected")
						}
					}
				}
				return huaweiResponse(r, huaweiRecordFixture)
			})
			p, err := store.preview(context.Background(), *s.dnsTarget, s.webBatch.Results)
			if err != nil {
				t.Fatal(err)
			}
			s.emit = func(string, interface{}) {}
			s.publishWebDNS(context.Background(), store, 7, &p, false, func() bool { return true })
			if !stop {
				s.endTask()
			}
			if writes != 0 {
				t.Fatal("invalidated batch mutated DNS")
			}
		})
	}
}

func TestHuaweiReconnectRebindsPermissionAndRefreshesCancelledOperation(t *testing.T) {
	s := &appSession{}
	r := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
	r.RemoteAddr = "127.0.0.1:50000"
	r.Header.Set("Origin", "http://127.0.0.1")
	s.refreshWebPublicationAfterReconnect()
	ctx, cancel := s.publicationContext(context.Background())
	defer cancel()
	s.cancelWebPublication()
	if ctx.Err() == nil {
		t.Fatal("disconnect did not cancel publication")
	}
	s.refreshWebPublicationAfterReconnect()
	ctx2, cancel2 := s.publicationContext(context.Background())
	defer cancel2()
	if ctx2.Err() != nil {
		t.Fatal("reconnected session cannot preview/update")
	}
}

func TestSelectedGitHubUsesBackendContentAfterDNSFailure(t *testing.T) {
	managementHTTPClientCache.Lock()
	old := managementHTTPClientCache.client
	managementHTTPClientCache.Unlock()
	defer func() {
		managementHTTPClientCache.Lock()
		managementHTTPClientCache.client = old
		managementHTTPClientCache.Unlock()
	}()
	writes := 0
	managementHTTPClientCache.Lock()
	managementHTTPClientCache.client = &http.Client{Transport: huaweiTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			return huaweiResponse(r, `{"sha":"oldsha"}`)
		}
		writes++
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		raw, _ := base64.StdEncoding.DecodeString(body["content"])
		if string(raw) != "192.0.2.2:443#NRT\n" || strings.Contains(string(raw), "attacker") {
			t.Fatalf("browser content published: %q", raw)
		}
		return huaweiResponse(r, `{}`)
	})}
	managementHTTPClientCache.Unlock()
	statuses := map[string]string{}
	s := &appSession{webBatch: &webBatchSnapshot{ID: 7, CompletedAt: time.Now(), Results: []TestResult{{IP: "192.0.2.2", Port: 443, DataCenter: "NRT"}}}, emit: func(kind string, data interface{}) {
		if kind == "web_publish_status" {
			p := data.(map[string]interface{})
			statuses[p["destination"].(string)] = p["status"].(string)
		}
	}}
	s.publishWebDNS(context.Background(), huaweiTestStore(t, nil), 7, nil, true, func() bool { return true })
	opts := webPublishOptions{Format: "txt", GitHubConfig: githubUploadRequest{Token: "fake-token", Owner: "owner", Repo: "repo", Path: "ip.txt", Content: "attacker"}}
	s.publishWebGitHub(context.Background(), 7, opts, true, func() bool { return true })
	s.publishWebGitHub(context.Background(), 7, opts, true, func() bool { return true })
	if writes != 1 || statuses["dns"] != "failed" || statuses["github"] != "denied" {
		t.Fatalf("independence/replay: %d %v", writes, statuses)
	}
}

func TestSelectedGitHubNewMeasurementBetweenReadAndPutPreventsUpload(t *testing.T) {
	managementHTTPClientCache.Lock()
	old := managementHTTPClientCache.client
	managementHTTPClientCache.Unlock()
	defer func() {
		managementHTTPClientCache.Lock()
		managementHTTPClientCache.client = old
		managementHTTPClientCache.Unlock()
	}()
	writes := 0
	s := &appSession{emit: func(string, interface{}) {}, webBatch: &webBatchSnapshot{ID: 7, CompletedAt: time.Now(), Results: []TestResult{{IP: "192.0.2.2", Port: 443}}}}
	managementHTTPClientCache.Lock()
	managementHTTPClientCache.client = &http.Client{Transport: huaweiTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut {
			writes++
			return huaweiResponse(r, `{}`)
		}
		if !s.beginTask(func() {}, "new task", "official", nil) {
			t.Fatal("new task rejected")
		}
		return huaweiResponse(r, `{"sha":"oldsha"}`)
	})}
	managementHTTPClientCache.Unlock()
	s.publishWebGitHub(context.Background(), 7, webPublishOptions{Format: "txt", GitHubConfig: githubUploadRequest{Token: "fake-token", Owner: "owner", Repo: "repo", Path: "ip.txt"}}, false, func() bool { return true })
	s.endTask()
	if writes != 0 {
		t.Fatal("new measurement invalidation bypassed")
	}
}

func TestBackgroundBatchAutoPublicationSurvivesDisconnectedPreview(t *testing.T) {
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) { return huaweiResponse(r, huaweiRecordFixture) })
	old := webHuaweiDNS
	webHuaweiDNS = store
	defer func() { webHuaweiDNS = old }()
	done := make(chan string, 1)
	s := &appSession{backgroundTask: true, dnsTarget: &huaweiRecord{ID: "record1", ZoneID: "zone1", Name: "edge.example.com.", Type: "A", Line: "default_view", revision: 1}, webPublishOptions: webPublishOptions{DNS: true, authorized: func() bool { return true }}, webBatch: &webBatchSnapshot{ID: 7, CompletedAt: time.Now(), Results: []TestResult{{IP: "192.0.2.2"}}}, emit: func(kind string, data interface{}) {
		if kind == "web_publish_status" {
			p := data.(map[string]interface{})
			if p["status"] != "running" {
				done <- p["status"].(string)
			}
		}
	}}
	ctx, cancel := s.publicationContext(context.Background())
	defer cancel()
	s.cancelWebPublication()
	if ctx.Err() == nil {
		t.Fatal("in-flight preview was not cancelled on disconnect")
	}
	s.autoPublishCompletedWebBatch()
	select {
	case status := <-done:
		if status != "updated" {
			t.Fatalf("opted-in background completion failed after disconnect: %s", status)
		}
	case <-time.After(time.Second):
		t.Fatal("background publication did not finish")
	}
}

func huaweiFollowerRequests() (*http.Request, *http.Request) {
	valid := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
	valid.RemoteAddr = "127.0.0.1:50000"
	valid.Header.Set("Origin", "http://127.0.0.1")
	foreign := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
	foreign.RemoteAddr = "127.0.0.1:50001"
	foreign.Header.Set("Origin", "https://foreign.test")
	return valid, foreign
}

func TestInterleavedBackgroundFollowerCannotBorrowConfigurationAuthority(t *testing.T) {
	valid, foreign := huaweiFollowerRequests()
	store := huaweiTestStore(t, nil)
	s := &appSession{backgroundTask: true, emit: func(string, interface{}) {}}
	// Two live handlers share this session: a valid action interleaves the foreign handler's entry.
	s.refreshWebPublicationAfterReconnect()
	handleHuaweiWebAction(valid.Context(), s, store, "huawei_config_get", json.RawMessage(`{}`), func() bool { return huaweiWebAuthorized(valid) })
	handleHuaweiWebAction(foreign.Context(), s, store, "huawei_config_set", json.RawMessage(`{"region":"cn-east-3","projectId":"0123456789abcdef0123456789abcdef"}`), func() bool { return huaweiWebAuthorized(foreign) })
	if store.status().Region != "cn-north-4" {
		t.Fatal("foreign follower borrowed legitimate handshake and changed configuration")
	}
}

func TestBackgroundOptInOwnerIsNotReplacedByForeignFollower(t *testing.T) {
	valid, foreign := huaweiFollowerRequests()
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) { return huaweiResponse(r, huaweiRecordFixture) })
	old := webHuaweiDNS
	webHuaweiDNS = store
	defer func() { webHuaweiDNS = old }()
	done := make(chan string, 1)
	s := &appSession{backgroundTask: true, dnsTarget: &huaweiRecord{ID: "record1", ZoneID: "zone1", Name: "edge.example.com.", Type: "A", Line: "default_view", revision: 1}, webBatch: &webBatchSnapshot{ID: 7, CompletedAt: time.Now(), Results: []TestResult{{IP: "192.0.2.2"}}}, emit: func(kind string, data interface{}) {
		if kind == "web_publish_status" {
			p := data.(map[string]interface{})
			if p["status"] != "running" {
				done <- p["status"].(string)
			}
		}
	}}
	s.refreshWebPublicationAfterReconnect()
	handleHuaweiWebAction(valid.Context(), s, store, "web_publish_options", json.RawMessage(`{"dns":true,"github":false}`), func() bool { return huaweiWebAuthorized(valid) })
	s.refreshWebPublicationAfterReconnect()
	handleHuaweiWebAction(foreign.Context(), s, store, "web_publish_options", json.RawMessage(`{"dns":false,"github":false}`), func() bool { return huaweiWebAuthorized(foreign) })
	s.autoPublishCompletedWebBatch()
	select {
	case status := <-done:
		if status != "updated" {
			t.Fatalf("legitimate opt-in authority overwritten: %s", status)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("foreign follower revoked authorized opt-in completion")
	}
}

func TestQueuedHuaweiReadRetainsInitiatingConnectionAuthority(t *testing.T) {
	valid, foreign := huaweiFollowerRequests()
	called := 0
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) { called++; return huaweiResponse(r, `{"zones":[]}`) })
	done := make(chan struct{}, 1)
	s := &appSession{backgroundTask: true, emit: func(kind string, data interface{}) {
		if kind == "huawei_response" && data.(map[string]interface{})["action"] == "huawei_zones" {
			done <- struct{}{}
		}
	}}
	s.dnsMutex.Lock()
	handleHuaweiWebAction(valid.Context(), s, store, "huawei_zones", json.RawMessage(`{}`), func() bool { return huaweiWebAuthorized(valid) })
	// The valid operation is queued; concurrent foreign follower actions must not revoke or lend authority.
	handleHuaweiWebAction(foreign.Context(), s, store, "huawei_zones", json.RawMessage(`{}`), func() bool { return huaweiWebAuthorized(foreign) })
	handleHuaweiWebAction(valid.Context(), s, store, "huawei_config_get", json.RawMessage(`{}`), func() bool { return huaweiWebAuthorized(valid) })
	s.dnsMutex.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queued authorized read borrowed foreign follower authority")
	}
	if called != 1 {
		t.Fatalf("foreign/valid read interleaving made %d cloud calls", called)
	}
}

func TestDNSMutationCannotBorrowOtherFollowersValidLoginAfterOwnerRevocation(t *testing.T) {
	oldUser, oldPassword := webUser, webPassword
	webUser, webPassword = "user", "password"
	defer func() { webUser, webPassword = oldUser, oldPassword }()
	owner, other := huaweiFollowerRequests()
	other.Header.Set("Origin", "http://127.0.0.1")
	ownerToken, _, _ := createAuthSession("user")
	defer deleteAuthSession(ownerToken)
	otherToken, _, _ := createAuthSession("user")
	defer deleteAuthSession(otherToken)
	owner.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})
	other.AddCookie(&http.Cookie{Name: sessionCookieName, Value: otherToken})
	reads, writes := 0, 0
	s := &appSession{backgroundTask: true, emit: func(string, interface{}) {}, webBatch: &webBatchSnapshot{ID: 7, CompletedAt: time.Now(), Results: []TestResult{{IP: "192.0.2.2"}}}, dnsTarget: &huaweiRecord{ID: "record1", ZoneID: "zone1", Name: "edge.example.com.", Type: "A", Line: "default_view", revision: 1}}
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut {
			writes++
		}
		reads++
		if reads == 2 {
			deleteAuthSession(ownerToken)
			handleHuaweiWebAction(other.Context(), s, webHuaweiDNS, "huawei_config_get", json.RawMessage(`{}`), func() bool { return huaweiWebAuthorized(other) })
		}
		return huaweiResponse(r, huaweiRecordFixture)
	})
	p, err := store.preview(owner.Context(), *s.dnsTarget, s.webBatch.Results)
	if err != nil {
		t.Fatal(err)
	}
	s.publishWebDNS(owner.Context(), store, 7, &p, false, func() bool { return huaweiWebAuthorized(owner) })
	if writes != 0 {
		t.Fatal("revoked initiating connection borrowed other follower login at DNS mutation")
	}
}

func TestGitHubMutationCannotBorrowOtherFollowersValidLoginAfterOwnerRevocation(t *testing.T) {
	oldUser, oldPassword := webUser, webPassword
	webUser, webPassword = "user", "password"
	defer func() { webUser, webPassword = oldUser, oldPassword }()
	owner, other := huaweiFollowerRequests()
	other.Header.Set("Origin", "http://127.0.0.1")
	ownerToken, _, _ := createAuthSession("user")
	defer deleteAuthSession(ownerToken)
	otherToken, _, _ := createAuthSession("user")
	defer deleteAuthSession(otherToken)
	owner.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})
	other.AddCookie(&http.Cookie{Name: sessionCookieName, Value: otherToken})
	s := &appSession{emit: func(string, interface{}) {}, webBatch: &webBatchSnapshot{ID: 7, CompletedAt: time.Now(), Results: []TestResult{{IP: "192.0.2.2", Port: 443}}}}
	managementHTTPClientCache.Lock()
	old := managementHTTPClientCache.client
	writes := 0
	managementHTTPClientCache.client = &http.Client{Transport: huaweiTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut {
			writes++
			return huaweiResponse(r, `{}`)
		}
		deleteAuthSession(ownerToken)
		handleHuaweiWebAction(other.Context(), s, webHuaweiDNS, "huawei_config_get", json.RawMessage(`{}`), func() bool { return huaweiWebAuthorized(other) })
		return huaweiResponse(r, `{"sha":"oldsha"}`)
	})}
	managementHTTPClientCache.Unlock()
	defer func() {
		managementHTTPClientCache.Lock()
		managementHTTPClientCache.client = old
		managementHTTPClientCache.Unlock()
	}()
	s.publishWebGitHub(owner.Context(), 7, webPublishOptions{Format: "txt", GitHubConfig: githubUploadRequest{Token: "fake-token", Owner: "owner", Repo: "repo", Path: "ip.txt"}}, false, func() bool { return huaweiWebAuthorized(owner) })
	if writes != 0 {
		t.Fatal("revoked initiating connection borrowed other follower login at GitHub mutation")
	}
}

func TestQueuedDNSPublisherRechecksInitiatingAuthorityBeforeCloudPreview(t *testing.T) {
	var allowed atomic.Bool
	allowed.Store(true)
	entered := make(chan struct{})
	var once sync.Once
	authority := func() bool { value := allowed.Load(); once.Do(func() { close(entered) }); return value }
	cloudCalls := 0
	store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
		cloudCalls++
		return huaweiResponse(r, huaweiRecordFixture)
	})
	s := &appSession{emit: func(string, interface{}) {}, webBatch: &webBatchSnapshot{ID: 7, CompletedAt: time.Now(), Results: []TestResult{{IP: "192.0.2.2"}}}, dnsTarget: &huaweiRecord{ID: "record1", ZoneID: "zone1", Name: "edge.example.com.", Type: "A", Line: "default_view", revision: 1}}
	s.dnsMutex.Lock()
	done := make(chan struct{})
	go func() { s.publishWebDNS(context.Background(), store, 7, nil, true, authority); close(done) }()
	<-entered
	allowed.Store(false)
	s.dnsMutex.Unlock()
	<-done
	if cloudCalls != 0 {
		t.Fatal("queued publisher read cloud after initiating authority was revoked")
	}
}
