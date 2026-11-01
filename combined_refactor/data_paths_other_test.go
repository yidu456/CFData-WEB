//go:build !android

package main

import (
	"path/filepath"
	"testing"
)

func TestDesktopIgnoresAndroidDataDirectoryOverride(t *testing.T) {
	dir := testProgramDirectory(t)
	t.Setenv("CFDATA_DATA_DIR", t.TempDir())
	got, err := defaultDataPath("huawei-dns.private")
	if err != nil || got != filepath.Join(dir, "huawei-dns.private") {
		t.Fatalf("Android setting changed desktop storage: %q, %v", got, err)
	}
}
