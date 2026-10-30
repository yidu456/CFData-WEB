package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testProgramDirectory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cfdata.exe"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	old := programExecutable
	programExecutable = func() (string, error) { return filepath.Join(dir, "cfdata.exe"), nil }
	t.Cleanup(func() { programExecutable = old })
	t.Chdir(t.TempDir())
	return dir
}

func TestDefaultDataPathsUseExecutableNotLaunchDirectory(t *testing.T) {
	dir := testProgramDirectory(t)
	for _, name := range []string{"locations.json", "ips-v4.txt", "ips-v6.txt", asnDBFile, "cfdata-config.json", debugLogFileName, "huawei-dns.private"} {
		got, err := defaultDataPath(name)
		if err != nil || got != filepath.Join(dir, name) {
			t.Fatalf("%s: unexpected path %q: %v", name, got, err)
		}
	}
	for _, name := range []string{"", ".", "..", "../escape", filepath.Join(dir, "absolute")} {
		if _, err := defaultDataPath(name); err == nil {
			t.Fatalf("accepted non-basename %q", name)
		}
	}
	got, err := defaultCLIConfigPath()
	if err != nil || got != filepath.Join(dir, "cfdata-config.json") {
		t.Fatal(got, err)
	}
	if _, _, err := loadOrCreateCLIConfig(got); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatal(err)
	}
	oldDebug := debugMode
	debugMode = true
	t.Cleanup(func() { debugMode = oldDebug })
	appendDebugLogLine("offline path fixture")
	if data, err := os.ReadFile(filepath.Join(dir, debugLogFileName)); err != nil || !strings.Contains(string(data), "offline path fixture") {
		t.Fatal(string(data), err)
	}
	if _, err := os.Stat(debugLogFileName); !os.IsNotExist(err) {
		t.Fatal("debug log used launch directory")
	}
}

func TestCacheReadsAndCleanupUseExecutableDirectory(t *testing.T) {
	dir := testProgramDirectory(t)
	oldLocations := locationMap
	t.Cleanup(func() { locationMap = oldLocations; resetASNReader() })
	for _, name := range []string{"ips-v4.txt", "ips-v6.txt", "locations.json", asnDBFile} {
		if err := os.WriteFile(name, []byte("launch directory must survive"), 0600); err != nil {
			t.Fatal(err)
		}
		content := "192.0.2.1\n"
		if name == "locations.json" {
			content = `[{"iata":"TEST","city":"offline"}]`
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	content, err := getIPListContent("ips-v4.txt", "https://invalid.invalid/no-network")
	if err != nil || content != "192.0.2.1\n" {
		t.Fatal(content, err)
	}
	initLocations()
	if _, ok := locationMap["TEST"]; !ok {
		t.Fatal("locations read launch directory")
	}
	resetASNReader()
	if err := os.Remove(asnDBFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(asnDBFile, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := getASNReader(); err == nil || !strings.Contains(err.Error(), "invalid MaxMind DB") {
		t.Fatal("ASN did not open executable cache", err)
	}
	if err := os.Remove(asnDBFile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(asnDBFile, []byte("launch directory must survive"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &appSession{emit: func(string, interface{}) {}}
	resetAllConfigFiles(s)
	for _, name := range []string{"ips-v4.txt", "ips-v6.txt", "locations.json", asnDBFile} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatal("exe cache survived", name, err)
		}
		if data, err := os.ReadFile(name); err != nil || string(data) != "launch directory must survive" {
			t.Fatal("launch cache removed", name, err)
		}
	}
}

func TestExecutableResolutionErrorsDoNotFallBack(t *testing.T) {
	testProgramDirectory(t)
	programExecutable = func() (string, error) { return "", errors.New("resolution unavailable") }
	for _, resolve := range []func() (string, error){programDataDir, defaultCLIConfigPath, defaultDebugLogPath} {
		if path, err := resolve(); err == nil || path != "" {
			t.Fatal("silent fallback", path, err)
		}
	}
	if _, err := getIPListContent("ips-v4.txt", "https://invalid.invalid"); err == nil {
		t.Fatal("cache ignored resolution failure")
	}
	programExecutable = func() (string, error) { return "relative.exe", nil }
	if _, err := programDataDir(); err == nil {
		t.Fatal("relative executable accepted")
	}
}

func TestDefaultResultsBesideExecutableExplicitResultsPreserved(t *testing.T) {
	dir := testProgramDirectory(t)
	for _, name := range []string{"", "ip.csv"} {
		if err := writeNSBCSV(name, nil, 0, true, scanModeTCPing); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "ip.csv")); err != nil {
		t.Fatal(err)
	}
	if err := writeNSBCSV("subdir/custom.csv", nil, 0, true, scanModeTCPing); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("custom.csv"); err != nil {
		t.Fatal("explicit sanitized NSB output changed", err)
	}
	for _, name := range []string{"", "cfdata-results"} {
		cfg := &cliConfig{outFile: name, export: defaultCLIExportConfig()}
		if err := writeCLIExportAndMaybeUpload(cfg, []cliResultRow{{"ip": "192.0.2.1", "port": "443"}}, "offline"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "cfdata-results.txt")); err != nil {
		t.Fatal(err)
	}
	cfg := &cliConfig{outFile: "subdir/custom", export: defaultCLIExportConfig()}
	if err := writeCLIExportAndMaybeUpload(cfg, []cliResultRow{{"ip": "192.0.2.1", "port": "443"}}, "offline"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("custom.txt"); err != nil {
		t.Fatal("explicit sanitized CLI output changed", err)
	}
	if _, err := os.Stat("cfdata-results.txt"); !os.IsNotExist(err) {
		t.Fatal("default CLI output used launch directory")
	}
	if _, err := os.Stat("ip.csv"); !os.IsNotExist(err) {
		t.Fatal("default NSB output used launch directory")
	}
}

func TestDownloadedCachesAndFailedWritesStayBesideExecutable(t *testing.T) {
	dir := testProgramDirectory(t)
	managementHTTPClientCache.Lock()
	old := managementHTTPClientCache.client
	calls := 0
	managementHTTPClientCache.client = &http.Client{Transport: huaweiTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("192.0.2.2\n")), Header: make(http.Header)}, nil
	})}
	managementHTTPClientCache.Unlock()
	t.Cleanup(func() {
		managementHTTPClientCache.Lock()
		managementHTTPClientCache.client = old
		managementHTTPClientCache.Unlock()
	})
	if _, err := getIPListContent("ips-v6.txt", "https://mock.invalid/ips"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "ips-v6.txt")); err != nil || string(got) != "192.0.2.2\n" {
		t.Fatal("download cache misplaced", err)
	}
	if _, err := os.Stat("ips-v6.txt"); !os.IsNotExist(err) {
		t.Fatal("download wrote launch directory")
	}
	if calls != 1 {
		t.Fatal("unexpected transport calls", calls)
	}
	if err := os.Mkdir(filepath.Join(dir, "ips-v4.txt"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := getIPListContent("ips-v4.txt", "https://mock.invalid/ips"); err == nil || calls != 1 {
		t.Fatal("invalid cache retried or fell back")
	}
	if err := atomicWriteFile(filepath.Join(dir, "ips-v4.txt"), []byte("replacement"), 0600); err == nil {
		t.Fatal("write to directory succeeded")
	}
	if temps, _ := filepath.Glob(filepath.Join(dir, "*.tmp-*")); len(temps) != 0 {
		t.Fatal("failed atomic write left temporary files")
	}
}

func TestNSBTemporaryInputUsesExecutableDirectoryAndIsRemoved(t *testing.T) {
	dir := testProgramDirectory(t)
	observed := false
	session := &appSession{emit: func(kind string, _ interface{}) {
		if kind != "error" {
			return
		}
		files, err := filepath.Glob(filepath.Join(dir, "cfdata-nsb-*.txt"))
		if err != nil || len(files) != 1 {
			t.Fatal("temporary NSB input not in executable directory", files, err)
		}
		if got, err := os.ReadFile(files[0]); err != nil || string(got) != "offline temporary input" {
			t.Fatal("temporary content missing", err)
		}
		observed = true
	}}
	runNSBTask(context.Background(), session, "fixture", "offline temporary input", "ip.csv", 1, 0, 0, "", false, 0, 1, "", 0, 0, true, scanModeTCPing, "")
	if !observed {
		t.Fatal("expected validation failure not reached")
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "cfdata-nsb-*.txt")); len(files) != 0 {
		t.Fatal("NSB temporary input not removed")
	}
	if files, _ := filepath.Glob("cfdata-nsb-*.txt"); len(files) != 0 {
		t.Fatal("NSB temporary input used launch directory")
	}
}
