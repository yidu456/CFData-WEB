//go:build android

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAndroidHuaweiEncryptedSaveAppliesAndReloadsWithoutExecutableAccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CFDATA_DATA_DIR", dir)
	old := programExecutable
	programExecutable = func() (string, error) { return "", errors.New("read-only Android library") }
	t.Cleanup(func() { programExecutable = old })
	path, err := defaultDataPath("huawei-dns.private")
	if err != nil {
		t.Fatal(err)
	}
	s := &huaweiDNSStore{path: path}
	c := huaweiConfigRequest{AK: "ANDROID_FIXTURE_AK", SK: "ANDROID_FIXTURE_SK", Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef", Persist: true}
	if err := s.configure(c); err != nil || !s.status().Persisted || !s.status().HasSecret {
		t.Fatalf("valid Android configuration did not save and apply: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "huawei-dns.private"))
	if err != nil || bytes.Contains(raw, []byte(c.AK)) || bytes.Contains(raw, []byte(c.SK)) {
		t.Fatal("Android credentials missing or stored as plaintext")
	}
	reloaded := &huaweiDNSStore{path: path}
	if err := reloaded.load(); err != nil || reloaded.config != c || !reloaded.status().Persisted {
		t.Fatal("saved Android credentials did not survive restart", err)
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	s.path = filepath.Join(blocker, "huawei-dns.private")
	changed := c
	changed.SK = "REJECTED_FIXTURE_SK"
	if err := s.configure(changed); err == nil || s.config != c {
		t.Fatal("failed Android save applied a new secret")
	}
	if now, err := os.ReadFile(path); err != nil || !bytes.Equal(now, raw) {
		t.Fatal("failed Android save destroyed the previous encrypted file")
	}
}

func TestAndroidPreferencesAndScanResultsReloadFromSameDataDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CFDATA_DATA_DIR", dir)
	path, err := defaultDataPath("web-state.private")
	if err != nil {
		t.Fatal(err)
	}
	state := &webStateStore{path: path}
	if err := state.load(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.patch(0, map[string]json.RawMessage{"theme": json.RawMessage(`"dark"`)}, false, func() bool { return true }); err != nil {
		t.Fatal(err)
	}
	reloadedState := &webStateStore{path: path}
	if err := reloadedState.load(); err != nil || string(reloadedState.snapshot().Sections["theme"]) != `"dark"` {
		t.Fatal("Android preferences did not reload", err)
	}
	resultPath, err := defaultDataPath("latest-results.json")
	if err != nil {
		t.Fatal(err)
	}
	store := &retainedResultsStore{path: resultPath}
	s := &appSession{retained: store, emit: func(string, interface{}) {}}
	if err := store.begin(s, true, "official", map[string]interface{}{"port": 443}); err != nil {
		t.Fatal(err)
	}
	s.sendWSMessage("scan_result", ScanResult{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", TCPDuration: time.Millisecond})
	s.sendWSMessage("test_result", TestResult{IP: "192.0.2.1", Port: 443, DataCenter: "HKG", Speed: "2.50MB/s", MeasuredSpeedMB: 2.5, SpeedMeasured: true})
	if err := store.finish(s); err != nil {
		t.Fatal(err)
	}
	reloaded := &retainedResultsStore{path: resultPath}
	reader := &appSession{retained: reloaded}
	got, err := reloaded.get(reader)
	if err != nil || !got.Saved || len(reader.scanResults) != 1 || len(reader.testResults) != 1 || reader.testResults[0].MeasuredSpeedMB != 2.5 {
		t.Fatal("Android scan/measurement data did not survive restart", err)
	}
}
