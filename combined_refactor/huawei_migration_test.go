package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func migrationTestStore(t *testing.T) (*huaweiDNSStore, huaweiConfigRequest) {
	t.Helper()
	s := huaweiTestStore(t, nil)
	s.legacyPath = filepath.Join(t.TempDir(), "huawei-dns.private")
	c := huaweiConfigRequest{AK: "OLD_AK_123456789", SK: "OLD_SK_123456789", Region: "cn-east-3", ProjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Persist: true}
	raw, _ := json.Marshal(c)
	if runtime.GOOS == "windows" {
		var err error
		raw, err = protectHuaweiPrivate(raw)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(s.legacyPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return s, c
}

func TestHuaweiDefaultPathAndLegacyNeverAutoload(t *testing.T) {
	dir := testProgramDirectory(t)
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	legacyDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(legacyDir, "cfdata-web", "huawei-dns.private")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("legacy must survive"), 0600); err != nil {
		t.Fatal(err)
	}
	old := webHuaweiDNS
	webHuaweiDNS = &huaweiDNSStore{}
	t.Cleanup(func() { webHuaweiDNS = old })
	initHuaweiPrivateConfig()
	if webHuaweiDNS.path != filepath.Join(dir, "huawei-dns.private") || webHuaweiDNS.legacyPath != legacyPath {
		t.Fatal("wrong default private paths")
	}
	if webHuaweiDNS.config.AK != "" || webHuaweiDNS.status().PrivateFileStatus != "absent" {
		t.Fatal("legacy was automatically loaded")
	}
	if got, _ := os.ReadFile(legacyPath); string(got) != "legacy must survive" {
		t.Fatal("legacy was changed")
	}
}

func TestHuaweiLegacyMigrationVerifiedFullConfigAndExplicitCleanup(t *testing.T) {
	s, c := migrationTestStore(t)
	if err := s.cleanupLegacy(); err == nil {
		t.Fatal("cleanup before migration succeeded")
	}
	source, _ := os.ReadFile(s.legacyPath)
	if err := s.migrateLegacy(); err != nil {
		t.Fatal(err)
	}
	if s.config != c || !s.status().LegacyCleanupReady || !s.status().Persisted {
		t.Fatal("migration did not verify/apply full config")
	}
	loaded := &huaweiDNSStore{path: s.path}
	if err := loaded.load(); err != nil || loaded.config != c {
		t.Fatal("migrated full config reload failed", err)
	}
	if got, _ := os.ReadFile(s.legacyPath); string(got) != string(source) {
		t.Fatal("migration removed/changed source")
	}
	if err := s.migrateLegacy(); err == nil {
		t.Fatal("migration overwrote destination")
	}
	if err := s.cleanupLegacy(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.legacyPath); !os.IsNotExist(err) {
		t.Fatal("explicit cleanup failed")
	}
	if err := loaded.load(); err != nil || loaded.config != c {
		t.Fatal("cleanup destroyed destination")
	}
}

func TestHuaweiMigrationFailuresAndChangedCleanupPreserveFiles(t *testing.T) {
	for _, which := range []string{"invalid", "existing", "blocked", "source-changed", "destination-changed"} {
		t.Run(which, func(t *testing.T) {
			s, _ := migrationTestStore(t)
			before, rev := s.config, s.revision
			source, _ := os.ReadFile(s.legacyPath)
			switch which {
			case "invalid":
				os.WriteFile(s.legacyPath, []byte("not valid ciphertext"), 0600)
				source = []byte("not valid ciphertext")
			case "existing":
				os.MkdirAll(filepath.Dir(s.path), 0700)
				os.WriteFile(s.path, []byte("existing must survive"), 0600)
			case "blocked":
				os.WriteFile(filepath.Dir(s.path), []byte("blocking parent"), 0600)
			}
			if strings.HasSuffix(which, "-changed") {
				if err := s.migrateLegacy(); err != nil {
					t.Fatal(err)
				}
				path := s.legacyPath
				if which == "destination-changed" {
					path = s.path
				}
				if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := s.cleanupLegacy(); err == nil || s.status().LegacyCleanupReady {
					t.Fatal("changed file cleanup authorized")
				}
				if _, err := os.Stat(s.legacyPath); err != nil {
					t.Fatal("cleanup failure destroyed source")
				}
				return
			}
			if err := s.migrateLegacy(); err == nil || s.config != before || s.revision != rev {
				t.Fatal("failed migration changed good memory")
			}
			if got, _ := os.ReadFile(s.legacyPath); string(got) != string(source) {
				t.Fatal("failed migration changed source")
			}
			if which == "existing" {
				if got, _ := os.ReadFile(s.path); string(got) != "existing must survive" {
					t.Fatal("existing destination overwritten")
				}
			}
		})
	}
}

func TestHuaweiMigrationActionsRequireAuthorizationAndConfirmation(t *testing.T) {
	for _, action := range []string{"huawei_legacy_migrate", "huawei_legacy_cleanup"} {
		for _, authorized := range []bool{false, true} {
			s, _ := migrationTestStore(t)
			var kind string
			session := &appSession{emit: func(k string, _ interface{}) { kind = k }, dnsPreview: &huaweiPreview{}, webPublishOptions: webPublishOptions{DNS: true}}
			handleHuaweiWebAction(context.Background(), session, s, action, json.RawMessage(`{}`), func() bool { return authorized })
			if kind != "huawei_error" {
				t.Fatal("unconfirmed action accepted")
			}
			if _, err := os.Stat(s.legacyPath); err != nil {
				t.Fatal("unconfirmed action changed source")
			}
			if !session.webPublishOptions.DNS || session.dnsPreview == nil {
				t.Fatal("failed action changed authority")
			}
		}
	}
	s, _ := migrationTestStore(t)
	var kind string
	session := &appSession{emit: func(k string, _ interface{}) { kind = k }, dnsPreview: &huaweiPreview{}, webPublishOptions: webPublishOptions{DNS: true, GitHub: true}}
	handleHuaweiWebAction(context.Background(), session, s, "huawei_legacy_migrate", json.RawMessage(`{"confirmed":true}`), func() bool { return true })
	if kind != "huawei_response" || session.webPublishOptions.DNS || session.dnsPreview != nil || !session.webPublishOptions.GitHub {
		t.Fatal("migration changed unrelated authority or retained stale DNS opt-in")
	}
}

func TestHuaweiPrivateSaveDoesNotChangeParentMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0750); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0700) })
	before, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := huaweiConfigRequest{AK: "FIXTURE_AK", SK: "FIXTURE_SK", Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef"}
	raw, _ := json.Marshal(c)
	if err := saveHuaweiPrivate(filepath.Join(dir, "huawei-dns.private"), raw); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(dir)
	if err != nil || before.Mode() != after.Mode() {
		t.Fatal("private save mutated executable parent mode", err)
	}
}

func TestHuaweiLegacyCleanupLockedSourcePreservesVerifiedDestination(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows DPAPI and file sharing")
	}
	s, _ := migrationTestStore(t)
	if err := s.migrateLegacy(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path)
	locked, err := os.Open(s.legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.cleanupLegacy(); err == nil {
		t.Fatal("locked legacy deletion succeeded")
	}
	if got, err := os.ReadFile(s.path); err != nil || string(got) != string(before) {
		t.Fatal("failed cleanup changed destination")
	}
	locked.Close()
	if err := s.cleanupLegacy(); err != nil {
		t.Fatal(err)
	}
}
