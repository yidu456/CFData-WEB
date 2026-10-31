package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func statePatch(t *testing.T, s *webStateStore, rev uint64, raw string) error {
	t.Helper()
	var patch map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &patch); err != nil {
		t.Fatal(err)
	}
	_, err := s.patch(rev, patch, false, func() bool { return true })
	return err
}

func TestWebStateSectionsEncryptedReloadAndStaleWrite(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	if err := s.load(); err != nil {
		t.Fatal(err)
	}
	if err := statePatch(t, s, 0, `{"theme":"dark"}`); err != nil {
		t.Fatal(err)
	}
	if err := statePatch(t, s, 1, `{"githubAccounts":[{"name":"fixture","repo":"owner/repo","token":"PRIVATE_FIXTURE_TOKEN"}]}`); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(s.path)
	if strings.Contains(string(raw), "PRIVATE_FIXTURE_TOKEN") || strings.Contains(string(raw), "dark") {
		t.Fatal("state written in plaintext")
	}
	reloaded := &webStateStore{path: s.path}
	if err := reloaded.load(); err != nil {
		t.Fatal(err)
	}
	got := reloaded.snapshot()
	if got.Revision != 2 || string(got.Sections["theme"]) != `"dark"` || !strings.Contains(string(got.Sections["githubAccounts"]), "PRIVATE_FIXTURE_TOKEN") {
		t.Fatal("lost unrelated section")
	}
	if err := statePatch(t, s, 1, `{"theme":"light"}`); err == nil {
		t.Fatal("stale competing write accepted")
	}
	if string(s.snapshot().Sections["theme"]) != `"dark"` {
		t.Fatal("stale write mutated memory")
	}
	if err := os.WriteFile(s.path, []byte(`{"theme":"light"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.load(); err == nil {
		t.Fatal("plaintext/tamper accepted")
	}
	if string(s.snapshot().Sections["theme"]) != `"dark"` {
		t.Fatal("failed read mutated good memory")
	}
}

func TestWebStateBoundsSchemaAndAbsentOnlyMigration(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	for _, raw := range []string{
		`{"other":true}`, `{"theme":"purple"}`, `{"autoload":"anything"}`, `{"config":{"official":{"autoPublish":true}}}`,
		`{"config":{"official":{"port":"70000"}}}`, `{"config":{"selectedDatacenters":["invalid"]}}`,
		`{"config":{"nsb":{"sourceUrl":"https://user:secret@example.com/a"}}}`, `{"githubAccounts":{}}`,
		`{"githubAccounts":[{"repo":"owner"}]}`, `{"githubAccounts":[{"repo":"owner/repo","tokenHash":"not hex"}]}`,
		`{"edgetunnelAccounts":[{"name":"edge","host":"javascript:secret","mode":"overwrite"}]}`,
		`{"edgetunnelAccounts":[{"name":"edge","host":"example.com","mode":"invalid"}]}`,
		`{"githubAccounts":[{"repo":"owner/repo","name":"` + strings.Repeat("a", 4097) + `"}]}`,
	} {
		if err := statePatch(t, s, 0, raw); err == nil {
			t.Fatal("invalid section accepted")
		}
		if s.snapshot().Revision != 0 {
			t.Fatal("validation failure mutated state")
		}
	}
	if err := statePatch(t, s, 0, `{"theme":"light"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.patch(1, map[string]json.RawMessage{"theme": json.RawMessage(`"dark"`)}, true, func() bool { return true }); err == nil {
		t.Fatal("migration overwrote existing data")
	}
	if string(s.snapshot().Sections["theme"]) != `"light"` {
		t.Fatal("migration changed existing state")
	}
}

func TestWebStateConcurrentCASPreservesSections(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, theme := range []string{`"dark"`, `"light"`} {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			_, e := s.patch(0, map[string]json.RawMessage{"theme": json.RawMessage(v)}, false, func() bool { return true })
			results <- e
		}(theme)
	}
	wg.Wait()
	close(results)
	ok := 0
	for e := range results {
		if e == nil {
			ok++
		}
	}
	if ok != 1 || s.snapshot().Revision != 1 {
		t.Fatal("competing writes were not serialized with CAS")
	}
}

func TestWebStateAuthorizedCorrelatedContentAndSafeMetadata(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	if e := statePatch(t, s, 0, `{"githubAccounts":[{"repo":"owner/repo","token":"PRIVATE_FIXTURE_TOKEN"}]}`); e != nil {
		t.Fatal(e)
	}
	var kind string
	var payload interface{}
	emit := func(k string, p interface{}) { kind, payload = k, p }
	for _, action := range []string{"web_state_get", "web_state_patch", "web_state_migrate", "web_state_legacy_cleanup_ready"} {
		if !handleWebStateAction(s, action, json.RawMessage(`{"requestId":"fixture-1","revision":0,"sections":{"theme":"dark"}}`), func() bool { return false }, emit) {
			t.Fatal(action)
		}
		raw, _ := json.Marshal(payload)
		if kind != "web_state_response" || strings.Contains(string(raw), "PRIVATE_FIXTURE") || !strings.Contains(string(raw), `"requestId":"fixture-1"`) || !strings.Contains(string(raw), `"code":"unauthorized"`) {
			t.Fatal("unsafe or uncorrelated denial")
		}
	}
	if !handleWebStateAction(s, "web_state_get", json.RawMessage(`{"requestId":"get","content":false}`), func() bool { return true }, emit) {
		t.Fatal("metadata unhandled")
	}
	raw, _ := json.Marshal(payload)
	if strings.Contains(string(raw), "PRIVATE_FIXTURE") || strings.Contains(string(raw), "sections") {
		t.Fatal("metadata leaked content")
	}
	handleWebStateAction(s, "web_state_get", json.RawMessage(`{"requestId":"get","content":true}`), func() bool { return true }, emit)
	raw, _ = json.Marshal(payload)
	if !strings.Contains(string(raw), "PRIVATE_FIXTURE_TOKEN") {
		t.Fatal("explicit authorized content absent")
	}
	oldUser, oldPass := webUser, webPassword
	defer func() { webUser, webPassword = oldUser, oldPass }()
	webUser, webPassword = "", ""
	for _, origin := range []string{"", "http://foreign.test", "http://127.0.0.1:13335"} {
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:13335/ws", nil)
		r.RemoteAddr = "127.0.0.1:50000"
		r.Header.Set("Origin", origin)
		handleWebStateAction(s, "web_state_get", json.RawMessage(`{"requestId":"auth","content":true}`), func() bool { return huaweiWebAuthorized(r) }, emit)
		raw, _ = json.Marshal(payload)
		if (origin == "http://127.0.0.1:13335") == strings.Contains(string(raw), `"code":"unauthorized"`) {
			t.Fatal("origin gate mismatch")
		}
	}
}

func TestWebStateRejectsUnsafeRequestID(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	for _, requestID := range []string{"line\nbreak", "private token"} {
		var response map[string]interface{}
		raw, _ := json.Marshal(map[string]interface{}{"requestId": requestID, "content": true})
		handleWebStateAction(s, "web_state_get", raw, func() bool { return true }, func(_ string, payload interface{}) {
			response = payload.(map[string]interface{})
		})
		if response["code"] != "invalid" || response["requestId"] != "" {
			t.Fatal("unsafe request ID echoed", response)
		}
	}
}

func TestWebStateUnavailablePathNoFallback(t *testing.T) {
	s := &webStateStore{}
	if e := s.load(); e == nil {
		t.Fatal("empty path silently loaded")
	}
	if e := statePatch(t, s, 0, `{"theme":"dark"}`); e == nil {
		t.Fatal("empty path saved")
	}
	if s.snapshot().Revision != 0 {
		t.Fatal("failure mutated memory")
	}
}

func TestWebStatePatchRejectsUnexpectedRequestFields(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	var response map[string]interface{}
	handleWebStateAction(s, "web_state_patch", json.RawMessage(`{"requestId":"strict","revision":0,"sections":{"theme":"dark"},"unexpected":true}`), func() bool { return true }, func(_ string, payload interface{}) {
		response = payload.(map[string]interface{})
	})
	if response["code"] != "invalid" || s.snapshot().Revision != 0 {
		t.Fatal("request accepted unknown fields", response)
	}
}

func TestWebStateGetRejectsMutationFields(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	var response map[string]interface{}
	handleWebStateAction(s, "web_state_get", json.RawMessage(`{"requestId":"strict-get","content":true,"sections":{"theme":"dark"}}`), func() bool { return true }, func(_ string, payload interface{}) {
		response = payload.(map[string]interface{})
	})
	if response["code"] != "invalid" {
		t.Fatal("read accepted mutation fields", response)
	}
}

func TestWebStateActionsRejectIrrelevantFields(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	tests := []struct {
		action string
		raw    string
	}{
		{"web_state_patch", `{"requestId":"patch","revision":0,"content":true,"sections":{"theme":"dark"}}`},
		{"web_state_migrate", `{"requestId":"migrate","revision":0,"content":true,"sections":{"theme":"dark"},"sourceDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`},
		{"web_state_legacy_cleanup_ready", `{"requestId":"cleanup","revision":0,"content":true,"sourceDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","migrationToken":"token"}`},
	}
	for _, test := range tests {
		var response map[string]interface{}
		handleWebStateAction(s, test.action, json.RawMessage(test.raw), func() bool { return true }, func(_ string, payload interface{}) {
			response = payload.(map[string]interface{})
		})
		if response["code"] != "invalid" || s.snapshot().Revision != 0 {
			t.Fatal(test.action, "accepted irrelevant fields", response)
		}
	}
}

func TestWebStateGetDetectsChangedCiphertext(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	if err := statePatch(t, s, 0, `{"theme":"dark"}`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path, []byte("changed outside process"), 0600); err != nil {
		t.Fatal(err)
	}
	var response map[string]interface{}
	handleWebStateAction(s, "web_state_get", json.RawMessage(`{"requestId":"changed","content":true}`), func() bool { return true }, func(_ string, payload interface{}) {
		response = payload.(map[string]interface{})
	})
	if response["code"] != "unavailable" || string(s.snapshot().Sections["theme"]) != `"dark"` {
		t.Fatal("changed ciphertext reported stale metadata or destroyed last good memory", response)
	}
}

func TestWebStateSaveFailurePreservesLastGoodMemory(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	if err := statePatch(t, s, 0, `{"theme":"dark"}`); err != nil {
		t.Fatal(err)
	}
	oldPath := s.path
	if err := os.Mkdir(oldPath+".block", 0700); err != nil {
		t.Fatal(err)
	}
	s.path = oldPath + ".block"
	if err := statePatch(t, s, 1, `{"theme":"light"}`); err == nil {
		t.Fatal("save failure accepted")
	}
	if s.snapshot().Revision != 1 || string(s.snapshot().Sections["theme"]) != `"dark"` {
		t.Fatal("save failure destroyed last good in-memory state")
	}
}

func TestWebStateQueuedActionRechecksInitiatingPermission(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	for _, action := range []string{"web_state_get", "web_state_patch", "web_state_legacy_cleanup_ready"} {
		raw := `{"requestId":"queued","content":true}`
		if action == "web_state_patch" {
			raw = `{"requestId":"queued","sections":{"theme":"dark"}}`
		} else if action == "web_state_legacy_cleanup_ready" {
			raw = `{"requestId":"queued","sourceDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","migrationToken":"token"}`
		}
		var allowed atomic.Bool
		allowed.Store(true)
		entered := make(chan struct{}, 1)
		done := make(chan map[string]interface{}, 1)
		s.mu.Lock()
		go handleWebStateAction(s, action, json.RawMessage(raw), func() bool {
			select {
			case entered <- struct{}{}:
			default:
			}
			return allowed.Load()
		}, func(_ string, p interface{}) { done <- p.(map[string]interface{}) })
		<-entered
		allowed.Store(false)
		s.mu.Unlock()
		response := <-done
		if response["code"] != "unauthorized" {
			t.Fatal("queued action kept expired permission", action, response)
		}
		if s.snapshot().Revision != 0 {
			t.Fatal("revoked writer mutated state")
		}
	}
}

func TestWebStateMigrationReceiptRequiresUnchangedDestination(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	var response map[string]interface{}
	emit := func(_ string, p interface{}) { response = p.(map[string]interface{}) }
	digest := strings.Repeat("a", 64)
	handleWebStateAction(s, "web_state_migrate", json.RawMessage(`{"requestId":"import","revision":0,"sourceDigest":"`+digest+`","sections":{"theme":"dark","autoload":"backend","config":{"selectedDatacenters":["NRT"]}}}`), func() bool { return true }, emit)
	if response["ok"] != true {
		t.Fatal(response)
	}
	token := response["migrationToken"].(string)
	check := json.RawMessage(`{"requestId":"cleanup","revision":1,"sourceDigest":"` + digest + `","migrationToken":"` + token + `"}`)
	handleWebStateAction(s, "web_state_legacy_cleanup_ready", check, func() bool { return true }, emit)
	if response["cleanupReady"] != true {
		t.Fatal("verified cleanup unavailable")
	}
	if e := statePatch(t, s, 1, `{"theme":"light"}`); e != nil {
		t.Fatal(e)
	}
	handleWebStateAction(s, "web_state_legacy_cleanup_ready", check, func() bool { return true }, emit)
	if response["ok"] != false {
		t.Fatal("changed destination allowed browser cleanup")
	}
	before, _ := os.ReadFile(s.path)
	if e := os.WriteFile(s.path, append(before, 1), 0600); e != nil {
		t.Fatal(e)
	}
	if e := statePatch(t, s, 2, `{"theme":"dark"}`); e == nil {
		t.Fatal("externally changed ciphertext overwritten")
	}
}

func TestWebStateSocketReplyStaysOnInitiatingConnectionWhileFollowing(t *testing.T) {
	oldStore, oldBackground := webState, backgroundTaskSession
	oldSkip, oldURL, oldUser, oldPass := skipGeoCheck, speedTestURL, webUser, webPassword
	defer func() {
		webState, backgroundTaskSession = oldStore, oldBackground
		skipGeoCheck, speedTestURL, webUser, webPassword = oldSkip, oldURL, oldUser, oldPass
	}()
	webState = &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	if e := statePatch(t, webState, 0, `{"githubAccounts":[{"repo":"owner/repo","token":"SOCKET_PRIVATE_FIXTURE"}]}`); e != nil {
		t.Fatal(e)
	}
	skipGeoCheck, speedTestURL, webUser, webPassword = true, "https://example.invalid/speed", "", ""
	managementHTTPClientCache.Lock()
	oldClient := managementHTTPClientCache.client
	managementHTTPClientCache.client = &http.Client{Transport: huaweiTestTransport(func(r *http.Request) (*http.Response, error) {
		return huaweiResponse(r, `{"tag_name":"fixture","html_url":"https://example.invalid"}`)
	})}
	managementHTTPClientCache.Unlock()
	defer func() {
		managementHTTPClientCache.Lock()
		managementHTTPClientCache.client = oldClient
		managementHTTPClientCache.Unlock()
	}()
	followed := make(chan struct{}, 1)
	leaked := make(chan struct{}, 1)
	backgroundTaskSession = &appSession{backgroundTask: true, emit: func(kind string, p interface{}) {
		if kind == "background_task_following" {
			followed <- struct{}{}
		}
		if kind == "web_state_response" {
			leaked <- struct{}{}
		}
	}}
	server := httptest.NewServer(http.HandlerFunc(handleWebSocket))
	defer server.Close()
	ws, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", http.Header{"Origin": []string{server.URL}})
	if e != nil {
		t.Fatal(e)
	}
	defer ws.Close()
	if e := ws.WriteJSON(map[string]interface{}{"type": "follow_background_task"}); e != nil {
		t.Fatal(e)
	}
	select {
	case <-followed:
	case <-time.After(time.Second):
		t.Fatal("background follow failed")
	}
	if e := ws.WriteJSON(map[string]interface{}{"type": "web_state_get", "data": map[string]interface{}{"requestId": "socket-get", "content": true}}); e != nil {
		t.Fatal(e)
	}
	ws.SetReadDeadline(time.Now().Add(time.Second))
	for {
		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if e := ws.ReadJSON(&msg); e != nil {
			t.Fatal("private reply did not reach initiating socket", e)
		}
		if msg.Type != "web_state_response" {
			continue
		}
		if !strings.Contains(string(msg.Data), "SOCKET_PRIVATE_FIXTURE") {
			t.Fatal("authorized socket content absent")
		}
		break
	}
	select {
	case <-leaked:
		t.Fatal("private content emitted through followed task")
	default:
	}
}
