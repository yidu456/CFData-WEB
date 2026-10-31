//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHuaweiLegacyDPAPILoadAndExplicitSaveConversion(t *testing.T) {
	config := huaweiConfigRequest{AK: "LEGACY_AK", SK: "LEGACY_SK", Region: "cn-north-4", ProjectID: "0123456789abcdef0123456789abcdef"}
	plain, _ := json.Marshal(config)
	legacy, err := huaweiDPAPI(plain, true)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "huawei-dns.private")
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	store := &huaweiDNSStore{path: path}
	if err := store.load(); err != nil || store.config != config || store.privateFileStatus != "legacy-dpapi" {
		t.Fatal("legacy DPAPI Huawei config did not load truthfully", err, store.privateFileStatus)
	}
	if !store.status().Persisted {
		t.Fatal("loaded legacy DPAPI config not reported as persisted")
	}
	if err := store.configure(huaweiConfigRequest{Region: "cn-east-3", ProjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}); err != nil {
		t.Fatal(err)
	}
	if store.status().Persisted {
		t.Fatal("memory-only change still reported as persisted")
	}
	if err := store.load(); err != nil || !store.status().Persisted {
		t.Fatal("legacy DPAPI reload did not restore persisted status", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, legacy) {
		t.Fatal("load rewrote legacy DPAPI Huawei config")
	}
	locked, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	request := huaweiConfigRequest{Region: config.Region, ProjectID: config.ProjectID, Persist: true}
	if err := store.configure(request); err == nil {
		locked.Close()
		t.Fatal("locked legacy DPAPI Huawei config was replaced")
	}
	locked.Close()
	if after, _ := os.ReadFile(path); !bytes.Equal(after, legacy) {
		t.Fatal("failed save changed legacy DPAPI Huawei config")
	}
	if err := store.configure(request); err != nil {
		t.Fatal(err)
	}
	portable, _ := os.ReadFile(path)
	if !bytes.HasPrefix(portable, []byte(privateEnvelopeMagic)) || bytes.Equal(portable, legacy) {
		t.Fatal("explicit Huawei save did not convert legacy DPAPI")
	}
	reloaded := &huaweiDNSStore{path: path}
	if err := reloaded.load(); err != nil || reloaded.config.AK != config.AK || reloaded.config.SK != config.SK || reloaded.config.Region != config.Region || reloaded.config.ProjectID != config.ProjectID || reloaded.privateFileStatus != "encrypted" {
		t.Fatal("converted Huawei config did not reload", err)
	}
}

func TestWebStateLegacyDPAPILoadAndExplicitSaveConversion(t *testing.T) {
	plain := []byte(`{"version":1,"revision":1,"sections":{"theme":"dark"}}`)
	legacy, err := huaweiDPAPI(plain, true)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "web-state.private")
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	store := &webStateStore{path: path}
	if err := store.load(); err != nil || store.fileStatus != "legacy-dpapi" || store.document.Revision != 1 {
		t.Fatal("legacy DPAPI Web state did not load truthfully", err, store.fileStatus)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, legacy) {
		t.Fatal("load rewrote legacy DPAPI Web state")
	}
	locked, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := statePatch(t, store, 1, `{"theme":"light"}`); err == nil {
		locked.Close()
		t.Fatal("locked legacy DPAPI Web state was replaced")
	}
	locked.Close()
	if after, _ := os.ReadFile(path); !bytes.Equal(after, legacy) {
		t.Fatal("failed save changed legacy DPAPI Web state")
	}
	if err := statePatch(t, store, 1, `{"theme":"light"}`); err != nil {
		t.Fatal(err)
	}
	portable, _ := os.ReadFile(path)
	if !bytes.HasPrefix(portable, []byte(privateEnvelopeMagic)) || bytes.Equal(portable, legacy) {
		t.Fatal("explicit Web-state save did not convert legacy DPAPI")
	}
	reloaded := &webStateStore{path: path}
	if err := reloaded.load(); err != nil || reloaded.fileStatus != "encrypted" || reloaded.document.Revision != 2 {
		t.Fatal("converted Web state did not reload", err)
	}
}
