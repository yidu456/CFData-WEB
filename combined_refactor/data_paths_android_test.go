//go:build android

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestAndroidDataPathsDoNotUseInstalledExecutable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CFDATA_DATA_DIR", dir)
	old := programExecutable
	programExecutable = func() (string, error) { return "", errors.New("installed library is read-only") }
	t.Cleanup(func() { programExecutable = old })
	for _, name := range []string{"huawei-dns.private", "web-state.private", "latest-results.json", "cfdata-config.json", "locations.json", "ips-v4.txt", "ips-v6.txt", "GeoLite2-ASN.mmdb", "cfdata-debug.log"} {
		got, err := defaultDataPath(name)
		if err != nil || got != filepath.Join(dir, name) {
			t.Fatalf("%s must use writable Android data directory: %q, %v", name, got, err)
		}
	}
}

func TestAndroidInvalidDataDirectoryDoesNotFallBackToExecutable(t *testing.T) {
	for _, dir := range []string{"relative", "../outside"} {
		t.Setenv("CFDATA_DATA_DIR", dir)
		if _, err := programDataDir(); err == nil {
			t.Fatalf("accepted relative Android data directory %q", dir)
		}
	}
}

func TestAndroidDefaultDataDirectoryIsDownload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows host simulation cannot resolve absolute Android paths")
	}
	t.Setenv("CFDATA_DATA_DIR", "")
	got, err := programDataDir()
	if err != nil || got != "/storage/emulated/0/Download/cfdata-web" {
		t.Fatalf("Android default points outside Download: %q, %v", got, err)
	}
}

func TestAndroidDataPathStillRejectsTraversal(t *testing.T) {
	t.Setenv("CFDATA_DATA_DIR", t.TempDir())
	for _, name := range []string{"", ".", "..", "../private", "nested/file", `nested\file`, "C:private"} {
		if _, err := defaultDataPath(name); err == nil {
			t.Fatalf("accepted unsafe data filename %q", name)
		}
	}
}

func TestAndroidFixedModesDoNotHideRealPermissionOrOutsideDirectoryErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CFDATA_DATA_DIR", dir)
	inside := filepath.Join(dir, "huawei-dns.private")
	for _, modeErr := range []error{syscall.EPERM, syscall.EOPNOTSUPP, syscall.ENOSYS} {
		wrapped := &os.PathError{Op: "chmod", Path: inside, Err: modeErr}
		if err := androidModeError(inside, wrapped); err != nil {
			t.Fatalf("fixed shared-storage modes prevented saving: %v", err)
		}
		for _, outside := range []string{filepath.Join(dir, "..", "outside"), filepath.Join(dir+"-outside", "private")} {
			if err := androidModeError(outside, wrapped); err != wrapped {
				t.Fatal("ignored permission error outside the application data directory")
			}
		}
	}
	for _, denied := range []error{syscall.EACCES, syscall.ENOSPC, os.ErrNotExist} {
		if err := androidModeError(inside, denied); err != denied {
			t.Fatal("ignored a real access, space or missing-file error")
		}
	}
}
