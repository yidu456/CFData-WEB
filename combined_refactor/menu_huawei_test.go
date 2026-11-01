package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func withCLIMenuInput(t *testing.T, input string) {
	t.Helper()
	old := menuInputReader
	menuInputReader = bufio.NewReader(strings.NewReader(input))
	t.Cleanup(func() { menuInputReader = old })
}

func TestCLIHuaweiWizard(t *testing.T) {
	c := defaultCLIHuaweiConfig()
	withCLIMenuInput(t, "1\nexample.com\nedge.example.com\n1\n\n2\n2\n1\n5\n")
	if result := runCLIHuaweiWizard(&c); result != promptOK {
		t.Fatalf("wizard outcome=%d", result)
	}
	if !c.Enabled || c.Zone != "example.com." || c.Name != "edge.example.com." || c.Type != "A" || c.Mode != "append" || c.StartRow != 2 || c.EndRow != 5 {
		t.Fatalf("wizard configuration: %+v", c)
	}
	before := c
	menuInputReader = bufio.NewReader(strings.NewReader("1\nb\n"))
	if result := runCLIHuaweiWizard(&c); result != promptBack || !reflect.DeepEqual(c, before) {
		t.Fatalf("cancel modified configuration: %+v %d", c, result)
	}
	menuInputReader = bufio.NewReader(strings.NewReader("2\n"))
	if runCLIHuaweiWizard(&c) != promptOK || c.Enabled {
		t.Fatal("could not disable DNS")
	}
}

func TestCLIHuaweiConfirmation(t *testing.T) {
	for _, input := range []string{"n\n", "b\n", "", "yes\n", "y\n", "Y\n"} {
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			withCLIMenuInput(t, input)
			want := strings.EqualFold(strings.TrimSpace(input), "y")
			if got := confirmCLIHuaweiPreview(huaweiPreview{}); got != want {
				t.Fatalf("input=%q authorized=%v", input, got)
			}
		})
	}
}

func TestCLIHuaweiCredentialMenuEncryption(t *testing.T) {
	for _, mode := range []string{"2", "1", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "huawei-dns.private")
			store := &huaweiDNSStore{path: path}
			input := "AK_TEST_CLI_NO_PLAINTEXT\nSK_TEST_CLI_NO_PLAINTEXT\ncn-north-4\n0123456789abcdef0123456789abcdef\n" + mode + "\n"
			if mode == "cancel" {
				input = "AK_TEST_CLI_NO_PLAINTEXT\nb\n"
			}
			withCLIMenuInput(t, input)
			result := configureCLIHuaweiCredentials(store)
			if mode == "cancel" {
				if result != promptBack || store.status().HasSecret {
					t.Fatal("cancel applied partial credentials")
				}
				return
			}
			if result != promptOK || !store.status().HasSecret {
				t.Fatalf("configuration failed %d", result)
			}
			data, err := os.ReadFile(path)
			if mode == "1" {
				if !os.IsNotExist(err) {
					t.Fatal("memory-only credentials saved")
				}
				return
			}
			if err != nil || strings.Contains(string(data), "AK_TEST_CLI") || strings.Contains(string(data), "SK_TEST_CLI") {
				t.Fatalf("plaintext or missing private configuration: %v", err)
			}
			loaded := &huaweiDNSStore{path: path}
			if err := loaded.load(); err != nil || loaded.config.AK != "AK_TEST_CLI_NO_PLAINTEXT" {
				t.Fatalf("encrypted reload failed: %v", err)
			}
			file := defaultCLIFileConfig()
			if err := writeCLIConfigTemplate(filepath.Join(filepath.Dir(path), "cli.json"), file); err != nil {
				t.Fatal(err)
			}
			jsonBytes, err := os.ReadFile(filepath.Join(filepath.Dir(path), "cli.json"))
			if err != nil || strings.Contains(string(jsonBytes), "AK_TEST_CLI") || strings.Contains(string(jsonBytes), "SK_TEST_CLI") {
				t.Fatal("credentials in ordinary CLI JSON")
			}
		})
	}
}

func TestCLIHuaweiBooleanFlagArguments(t *testing.T) {
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"cfdata", "-cli", "-huawei", "false", "-hwconfirm", "true", "qs"}
	rewriteBoolFlagArgs()
	want := []string{"cfdata", "-cli", "-huawei=false", "-hwconfirm=true", "qs"}
	if !reflect.DeepEqual(os.Args, want) {
		t.Fatalf("arguments: %v", os.Args)
	}
	oldFlags := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	t.Cleanup(func() { flag.CommandLine = oldFlags })
	cfg := registerCLIFlags()
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil || cfg.huawei.Enabled || !cfg.huawei.Confirmed || !reflect.DeepEqual(flag.Args(), []string{"qs"}) {
		t.Fatalf("flags lost: %+v %v", cfg.huawei, err)
	}
}

func TestCLIHuaweiOutputPublicationBoundary(t *testing.T) {
	for _, scenario := range []string{"rank-independent", "export-failure", "upload-failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Chdir(t.TempDir())
			cfg := cliHuaweiTestConfig()
			cfg.export = defaultCLIExportConfig()
			cfg.outFile, cfg.outIPType, cfg.outQualified, cfg.startRow, cfg.endRow = "result.txt", "all", "all", 1, 1
			cfg.huawei.EndRow, cfg.huawei.Confirmed = 2, true
			if scenario == "export-failure" {
				if err := os.Mkdir("result.txt", 0700); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "upload-failure" {
				cfg.export.GitHub = true
				cfg.export.GHRepo = "invalid"
			}
			var written []string
			store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPut {
					var body struct {
						Records []string `json:"records"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					written = body.Records
				}
				switch r.URL.Path {
				case "/v2/zones":
					return huaweiResponse(r, `{"zones":[{"id":"zone1","name":"example.com.","zone_type":"public"}]}`)
				case "/v2.1/zones/zone1/recordsets":
					return huaweiResponse(r, `{"recordsets":[`+huaweiRecordFixture+`]}`)
				default:
					return huaweiResponse(r, huaweiRecordFixture)
				}
			})
			rows := []TestResult{cliHuaweiMeasured("192.0.2.2", "NRT", 9), cliHuaweiMeasured("192.0.2.3", "HKG", 7)}
			err := writeOfficialCLIResults(context.Background(), cfg, nil, rows, store)
			if scenario != "rank-independent" {
				if err == nil || len(written) != 0 {
					t.Fatalf("DNS published after output error: %v %v", written, err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(written, []string{"192.0.2.2", "192.0.2.3"}) {
				t.Fatalf("DNS incorrectly reused export range: %v %v", written, err)
			}
			content, err := os.ReadFile("result.txt")
			if err != nil || !strings.Contains(string(content), "192.0.2.2") || strings.Contains(string(content), "192.0.2.3") {
				t.Fatalf("export range changed: %s %v", content, err)
			}
		})
	}
}
