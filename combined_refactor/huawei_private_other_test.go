//go:build !windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestHuaweiPortableProtectRejectsGenericPlaintext(t *testing.T) {
	plain := []byte(`{"ak":"legacy-ak","sk":"legacy-sk","region":"cn-north-4","projectId":"0123456789abcdef0123456789abcdef"}`)
	protected, err := protectHuaweiPrivate(plain)
	if err != nil || bytes.Contains(protected, plain) {
		t.Fatal("portable encryption failed or returned plaintext", err)
	}
	if legacy, err := unprotectHuaweiPrivate(plain); err == nil || len(legacy) != 0 {
		t.Fatal("generic decrypt accepted plaintext")
	}
}

func TestHuaweiValidatedLegacyPlaintextReadRemainsSeparate(t *testing.T) {
	plain := []byte(`{"ak":"legacy-ak","sk":"legacy-sk","region":"cn-north-4","projectId":"0123456789abcdef0123456789abcdef"}`)
	path := filepath.Join(t.TempDir(), "huawei-dns.private")
	if err := os.WriteFile(path, plain, 0600); err != nil {
		t.Fatal(err)
	}
	config, raw, _, format, err := readHuaweiPrivateConfig(path)
	if err != nil || format != "legacy-plaintext" || config.SK != "legacy-sk" || !bytes.Equal(raw, plain) {
		t.Fatal("validated Huawei plaintext compatibility lost", err, format)
	}
}
