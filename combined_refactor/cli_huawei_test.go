package main

import (
	"context"
	"encoding/json"
	"flag"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func cliHuaweiTestConfig() *cliConfig {
	c := &cliConfig{mode: "official", ipType: 4, port: 443, delay: 500, speedLimit: 50, speedMin: 2.5, dc: "HKG,NRT"}
	c.huawei = defaultCLIHuaweiConfig()
	c.huawei.Enabled, c.huawei.Zone, c.huawei.Name = true, "example.com", "edge.example.com"
	return c
}

func cliHuaweiMeasured(ip, dc string, speed float64) TestResult {
	r := TestResult{IP: ip, Port: 443, DataCenter: dc, AvgLatency: 20 * time.Millisecond}
	applyOfficialSpeedMeasurement(&r, speed, "")
	return r
}

func TestCLIHuaweiGlobalQualifiedRanking(t *testing.T) {
	cfg := cliHuaweiTestConfig()
	cfg.huawei.StartRow, cfg.huawei.EndRow = 1, 2
	rows := []TestResult{
		cliHuaweiMeasured("192.0.2.1", "HKG", 2.499),
		cliHuaweiMeasured("192.0.2.2", "NRT", 9),
		cliHuaweiMeasured("192.0.2.3", "HKG", 7),
		cliHuaweiMeasured("192.0.2.4", "NRT", 5),
		cliHuaweiMeasured("192.0.2.2", "HKG", 6),
		cliHuaweiMeasured("2001:db8::1", "NRT", 100),
		cliHuaweiMeasured("192.0.2.5", "LAX", 90),
		cliHuaweiMeasured("192.0.2.6", "HKG", math.Inf(1)),
		cliHuaweiMeasured("0.0.0.0", "HKG", 100),
	}
	wrongPort := cliHuaweiMeasured("192.0.2.7", "HKG", 50)
	wrongPort.Port = 8443
	unmeasured := cliHuaweiMeasured("192.0.2.8", "HKG", 40)
	unmeasured.SpeedMeasured = false
	slow := cliHuaweiMeasured("192.0.2.9", "HKG", 60)
	slow.AvgLatency = time.Second
	rows = append(rows, wrongPort, unmeasured, slow)
	got, err := selectCLIHuaweiResults(cfg, rows)
	if err != nil || !reflect.DeepEqual(huaweiIPs(got, "A"), []string{"192.0.2.2", "192.0.2.3"}) {
		t.Fatalf("global ranked results: %v err=%v", got, err)
	}
	cfg.huawei.StartRow, cfg.huawei.EndRow = 2, 0
	got, err = selectCLIHuaweiResults(cfg, rows)
	if err != nil || !reflect.DeepEqual(huaweiIPs(got, "A"), []string{"192.0.2.3", "192.0.2.4"}) {
		t.Fatalf("range over qualified unique IPs: %v err=%v", got, err)
	}
	cfg.ipType, cfg.huawei.Type = 6, "AAAA"
	cfg.huawei.StartRow = 1
	got, err = selectCLIHuaweiResults(cfg, rows)
	if err != nil || !reflect.DeepEqual(huaweiIPs(got, "AAAA"), []string{"2001:db8::1"}) {
		t.Fatalf("AAAA: %v %v", got, err)
	}
	cfg.huawei.StartRow = 20
	got, err = selectCLIHuaweiResults(cfg, rows)
	if err != nil || len(got) != 0 {
		t.Fatalf("out-of-range: %v %v", got, err)
	}
}

func TestCLIHuaweiValidation(t *testing.T) {
	for _, change := range []func(*cliConfig){
		func(c *cliConfig) { c.mode = "nsb" },
		func(c *cliConfig) { c.speedLimit = 0 },
		func(c *cliConfig) { c.speedMin = math.NaN() },
		func(c *cliConfig) { c.huawei.Type = "CNAME" },
		func(c *cliConfig) { c.huawei.Type = "AAAA" },
		func(c *cliConfig) { c.huawei.Zone = "https://example.com" },
		func(c *cliConfig) { c.huawei.Name = "edge.other.com" },
		func(c *cliConfig) { c.huawei.Line = "" },
		func(c *cliConfig) { c.huawei.Mode = "clear" },
		func(c *cliConfig) { c.huawei.StartRow = 0 },
		func(c *cliConfig) { c.huawei.EndRow = -1 },
		func(c *cliConfig) { c.huawei.StartRow = 5; c.huawei.EndRow = 2 },
		func(c *cliConfig) { c.compactIPv4 = true },
		func(c *cliConfig) { c.export.GHUpload = "file.txt" },
	} {
		cfg := cliHuaweiTestConfig()
		change(cfg)
		if err := validateCLIHuaweiConfig(cfg); err == nil {
			t.Errorf("accepted invalid configuration: %+v", cfg.huawei)
		}
	}
	cfg := cliHuaweiTestConfig()
	cfg.huawei.Enabled = false
	cfg.mode = "nsb"
	if err := validateCLIHuaweiConfig(cfg); err != nil {
		t.Fatal("disabled DNS changed old CLI behavior", err)
	}
}

func TestCLIHuaweiPublication(t *testing.T) {
	for _, scenario := range []string{"overwrite", "append", "aaaa", "deny", "no-callback", "confirmed", "empty", "unchanged", "stale-cloud", "stale-config", "cancel", "ambiguous-zone", "ambiguous-record", "missing-record", "wrong-line"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := cliHuaweiTestConfig()
			cfg.huawei.EndRow = 2
			if scenario == "append" {
				cfg.huawei.Mode = "append"
			}
			cfg.huawei.Confirmed = scenario == "confirmed"
			current, writes := huaweiRecordFixture, 0
			oldIP := "192.0.2.1"
			if scenario == "aaaa" {
				cfg.ipType, cfg.huawei.Type, oldIP = 6, "AAAA", "2001:db8::1"
				current = strings.ReplaceAll(strings.Replace(current, `"type":"A"`, `"type":"AAAA"`, 1), "192.0.2.1", oldIP)
			}
			if scenario == "wrong-line" {
				cfg.huawei.Line = "other"
			}
			var final map[string]interface{}
			store := huaweiTestStore(t, func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPut {
					writes++
					if err := json.NewDecoder(r.Body).Decode(&final); err != nil {
						t.Fatal(err)
					}
					return huaweiResponse(r, current)
				}
				switch r.URL.Path {
				case "/v2/zones":
					zones := `{"id":"zone1","name":"example.com.","zone_type":"public"}`
					if scenario == "ambiguous-zone" {
						zones += `,{"id":"zone2","name":"example.com.","zone_type":"public"}`
					}
					return huaweiResponse(r, `{"zones":[`+zones+`]}`)
				case "/v2.1/zones/zone1/recordsets":
					records := current
					if scenario == "ambiguous-record" {
						records += "," + strings.Replace(current, `"record1"`, `"record2"`, 1)
					}
					if scenario == "missing-record" {
						records = ""
					}
					return huaweiResponse(r, `{"recordsets":[`+records+`]}`)
				default:
					return huaweiResponse(r, current)
				}
			})
			rows := []TestResult{cliHuaweiMeasured("192.0.2.2", "NRT", 9), cliHuaweiMeasured("192.0.2.3", "HKG", 7)}
			if scenario == "aaaa" {
				rows = []TestResult{cliHuaweiMeasured("2001:db8::2", "NRT", 9), cliHuaweiMeasured("2001:db8::3", "HKG", 7)}
			}
			if scenario == "empty" {
				rows = nil
			}
			if scenario == "unchanged" {
				rows = []TestResult{cliHuaweiMeasured("192.0.2.1", "HKG", 9)}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			confirm := func(p huaweiPreview) bool {
				if !reflect.DeepEqual(p.Old, []string{oldIP}) {
					t.Fatalf("old preview=%v", p.Old)
				}
				if scenario == "stale-cloud" {
					current = strings.Replace(current, `"ttl":300`, `"ttl":600`, 1)
				}
				if scenario == "stale-config" {
					store.mu.Lock()
					store.revision++
					store.mu.Unlock()
				}
				if scenario == "cancel" {
					cancel()
				}
				return scenario != "deny"
			}
			if scenario == "no-callback" {
				confirm = nil
			}
			err := updateCLIHuaweiDNS(ctx, store, cfg, rows, confirm)
			wantWrite := scenario == "overwrite" || scenario == "append" || scenario == "confirmed" || scenario == "aaaa"
			if wantWrite {
				if err != nil || writes != 1 {
					t.Fatalf("publication writes=%d err=%v", writes, err)
				}
				ips := []interface{}{"192.0.2.2", "192.0.2.3"}
				if scenario == "aaaa" {
					ips = []interface{}{"2001:db8::2", "2001:db8::3"}
				}
				if scenario == "append" {
					ips = append([]interface{}{"192.0.2.1"}, ips...)
				}
				want := map[string]interface{}{"name": "edge.example.com.", "type": cfg.huawei.Type, "records": ips}
				if !reflect.DeepEqual(final, want) {
					t.Fatalf("mutated properties or incorrect rank/append: %#v", final)
				}
			} else if writes != 0 {
				t.Fatalf("unsafe mutation in %s: %v", scenario, final)
			}
			if strings.HasPrefix(scenario, "stale-") || strings.HasPrefix(scenario, "ambiguous-") || scenario == "missing-record" || scenario == "wrong-line" || scenario == "cancel" {
				if err == nil {
					t.Fatalf("unsafe scenario not rejected: %s", scenario)
				}
			}
		})
	}
}

func TestCLIHuaweiPrivateEnvironmentInputs(t *testing.T) {
	dir := testProgramDirectory(t)
	for _, key := range []string{"AK", "SK", "REGION", "PROJECT_ID"} {
		t.Setenv("CFDATA_HUAWEI_"+key, "")
	}
	t.Setenv("CFDATA_HUAWEI_AK", "AK_TEST_MEMORY_ONLY")
	if _, err := loadCLIHuaweiStore(); err == nil {
		t.Fatal("partial replacement credentials accepted")
	}
	t.Setenv("CFDATA_HUAWEI_SK", "SK_TEST_MEMORY_ONLY")
	t.Setenv("CFDATA_HUAWEI_REGION", "cn-north-4")
	t.Setenv("CFDATA_HUAWEI_PROJECT_ID", "0123456789abcdef0123456789abcdef")
	store, err := loadCLIHuaweiStore()
	if err != nil || !store.status().HasSecret {
		t.Fatalf("environment config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "huawei-dns.private")); !os.IsNotExist(err) {
		t.Fatal("memory-only environment saved to disk")
	}
	if err := os.WriteFile(filepath.Join(dir, "huawei-dns.private"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCLIHuaweiStore(); err == nil {
		t.Fatal("invalid private file silently ignored")
	}
}

func TestCLIHuaweiConfigResolution(t *testing.T) {
	oldFlags := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("cli-test", flag.ContinueOnError)
	t.Cleanup(func() { flag.CommandLine = oldFlags })
	cfg := registerCLIFlags()
	if err := flag.CommandLine.Parse([]string{"-huawei=false", "-hwzone=override.example", "-hwendrow=3", "-hwconfirm=true"}); err != nil {
		t.Fatal(err)
	}
	file := defaultCLIFileConfig()
	file.Huawei.Enabled, file.Huawei.Zone, file.Huawei.Name = true, "example.com", "edge.example.com"
	file.Huawei.Confirmed = true
	provided := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	applyCLIHuaweiConfig(&cfg.huawei, file.Huawei, provided)
	if cfg.huawei.Enabled || cfg.huawei.Zone != "override.example" || cfg.huawei.Name != "edge.example.com" || cfg.huawei.EndRow != 3 || !cfg.huawei.Confirmed {
		t.Fatalf("flag/file precedence: %+v", cfg.huawei)
	}
	applyCLIHuaweiConfig(&cfg.huawei, defaultCLIHuaweiConfig(), nil)
	if cfg.huawei.Enabled || cfg.huawei.Confirmed || cfg.huawei.Zone != "" || cfg.huawei.EndRow != 10 {
		t.Fatalf("stale menu state: %+v", cfg.huawei)
	}
	path := filepath.Join(t.TempDir(), "cli.json")
	if err := writeCLIConfigTemplate(path, file); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(raw)), `"confirmed"`) {
		t.Fatal("confirmation authority persisted")
	}
	loaded, _, err := loadOrCreateCLIConfig(path)
	if err != nil || !loaded.Huawei.Enabled || loaded.Huawei.Confirmed || loaded.Huawei.EndRow != 10 || loaded.OutEndRow != 20 || loaded.SpeedLimit != 5 {
		t.Fatalf("nested roundtrip/default compatibility: %+v %v", loaded, err)
	}
	oldPath := filepath.Join(t.TempDir(), "old-cli.json")
	if err := os.WriteFile(oldPath, []byte(`{"mode":"official","offdc":"HKG","offspeedlimit":17}`), 0600); err != nil {
		t.Fatal(err)
	}
	old, _, err := loadOrCreateCLIConfig(oldPath)
	if err != nil || old.DC != "HKG" || old.SpeedLimit != 17 || old.Huawei.Enabled || old.Huawei.StartRow != 1 || old.Huawei.EndRow != 10 {
		t.Fatalf("legacy configuration changed: %+v %v", old, err)
	}
}
