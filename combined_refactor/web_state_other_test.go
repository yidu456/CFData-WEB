//go:build !windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWebStateRejectsPlaintextFileOnPortablePlatform(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web-state.private")
	raw := []byte(`{"version":1,"revision":1,"sections":{"theme":"dark","githubAccounts":[{"repo":"owner/repo","token":"PRIVATE_FIXTURE_TOKEN"}]}}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	s := &webStateStore{
		path: path,
		document: webStateDocument{Version: 1, Revision: 7, Sections: map[string]json.RawMessage{
			"theme": json.RawMessage(`"light"`),
		}},
		fileStatus: "encrypted",
	}
	if err := s.load(); err == nil {
		t.Fatal("plaintext Web state accepted")
	}
	if s.fileStatus != "unavailable" || s.document.Revision != 7 || string(s.document.Sections["theme"]) != `"light"` {
		t.Fatal("rejected plaintext changed last-good memory")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(raw) {
		t.Fatal("rejected plaintext file was changed")
	}
}

func TestWebStateAbsentFileRemainsAvailable(t *testing.T) {
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	if err := s.load(); err != nil || s.fileStatus != "absent" || s.document.Revision != 0 {
		t.Fatal("absent Web state should remain available")
	}
}
