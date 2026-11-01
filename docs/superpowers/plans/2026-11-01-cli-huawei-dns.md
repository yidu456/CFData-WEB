# CLI Huawei DNS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task in the existing isolated worktree. Steps use checkbox tracking. The user requests sequential execution and a commit after each completed task.

**Goal:** Scan once, measure selected centers, globally rank qualified results, and explicitly update a chosen Huawei A/AAAA record.

**Architecture:** Keep CLI orchestration separate from DNS publication. Reuse `runDetailedTest`, `runOfficialSpeedTests`, `selectOfficialResults`, and `huaweiDNSStore` discovery/preview/update. Network-bound scan/detail/speed functions are supplied to the CLI orchestration helper for offline behavioral tests.

**Tech Stack:** Go 1.26.2, standard library, existing Huawei SDK v0.1.221, existing Node/Chrome regression suites.

**Spec:** `docs/superpowers/specs/2026-11-01-cli-huawei-dns.md`

## Global Constraints

- Work only in `.worktrees/cfdata-web-dev`, branch `dev`; do not touch main, `.github/`, reference Go files, live tasks or installed schedulers.
- Preserve existing capabilities and defaults. No new dependencies.
- DNS uses qualified measured results, global speed-descending/latency-ascending ordering, no per-center quotas, configurable inclusive rank range default 1..10.
- Existing speed limit 5 and export end row 20 remain unchanged. Examples request a larger speed pool explicitly.
- Credentials use `huawei-dns.private` or memory-only environment inputs; confirmation authority is invocation-only.
- Tests and builds must pass before each commit. Commit each completed task; no new push authorized.
- Use Go at `C:/Users/Administrator/AppData/Local/Temp/cfdata-handover-go1.26.2/go/bin/go.exe`, `CGO_ENABLED=0`.

## Task 0: Record The Approved Design

- [x] Read CLI, menu, selection, Huawei store, protocol/task callers and repository constraints.
- [x] Write this plan and the accompanying spec; confirm every requirement maps to a task.
- [x] Commit only the spec and plan as `docs: plan CLI multi-center Huawei DNS workflow`.

## Task 1: One Scan And Combined Multi-Center CLI Testing

**Files:** Create `combined_refactor/cli_official.go`, `combined_refactor/cli_official_test.go`; modify `combined_refactor/cli.go` and `combined_refactor/menu.go`.

**Interfaces:**
- `parseCLIDatacenters(string) ([]string, error)` normalizes/deduplicates comma-separated codes using `normalizeDatacenterCode`.
- `cliOfficialDependencies` owns network scan/detail/speed function signatures matching the existing implementations.
- `runOfficialCLIMeasurements(*cliConfig, *appSession, cliOfficialDependencies) ([]ScanResult, []TestResult, error)` scans once, resolves automatic/selected centers, completes detail passes, then performs one combined speed pass.
- `runOfficialCLI` keeps output behavior and calls this helper with real dependencies.

- [x] Write failing table tests for malformed/normalized center lists and a behavioral test whose fake scan populates HKG/NRT/SIN, detail results populate measured candidates, and combined speed input contains only HKG/NRT.

```go
centers, err := parseCLIDatacenters(" hkg,NRT,HKG ")
if err != nil || !reflect.DeepEqual(centers, []string{"HKG", "NRT"}) {
    t.Fatalf("centers = %v, err = %v", centers, err)
}
```

- [x] Run `go test -run 'TestCLI(Datacenters|Official)' -count=1`; observe missing feature failure.
- [x] Move existing official orchestration to the helper, validate before scanning, keep blank automatic selection, process found centers sequentially with cache-preserving quiet detail testing, report missing centers, and sort the combined pool before one global speed pass. Cancellation returns an error and never proceeds to output publication.
- [x] Update `offdc` help/menu hints to accept `HKG,NRT`; preserve existing single/blank behavior.
- [x] Run focused/full tests and vet; build Windows preview check and Linux `release_assets/cfdata-test`; mark task complete and commit as `feat: support combined multi-center CLI measurements`.

## Task 2: Safe Ranked Huawei DNS Publication And CLI Config

**Files:** Create `combined_refactor/cli_huawei.go`, `combined_refactor/cli_huawei_test.go`; modify `combined_refactor/cli.go` and `combined_refactor/main.go`.

**Interfaces:**
- `cliHuaweiConfig` contains Enabled, Zone, Name, Type, Line, Mode, StartRow, EndRow plus `Confirmed bool` with `json:"-"`.
- `selectCLIHuaweiResults(*cliConfig, []TestResult) ([]TestResult, error)` filters matching family/endpoint and reuses exact qualified global selection before the rank slice.
- `updateCLIHuaweiDNS(context.Context, *huaweiDNSStore, *cliConfig, []TestResult, func(huaweiPreview) bool) error` discovers, selects, previews, appends/overwrites and confirms a records-only update.
- `loadCLIHuaweiStore() (*huaweiDNSStore, error)` loads the private file and applies validated memory-only environment overrides.
- `applyCLIHuaweiConfig` implements explicit command-line > nested file > defaults without persisting Confirmed.

- [x] Write failing selection tests with HKG/NRT interleaved by speed, below-threshold rounded measurements, IPv6/wrong-port/duplicate IPs, rank ranges, and empty selection; expected first ranks are derived by hand.

```go
cfg.huawei.StartRow, cfg.huawei.EndRow = 1, 2
got, err := selectCLIHuaweiResults(cfg, results)
if err != nil || got[0].IP != "192.0.2.2" || got[1].IP != "192.0.2.3" {
    t.Fatalf("selection = %v, err = %v", got, err)
}
```

- [x] Write transport-backed tests for exact public-zone/record discovery, overwrite/append bodies, denied confirmation, empty selection, ambiguous targets, wrong family, stale cloud/config revision, cancellation, and unchanged no-op. Assert final records and absence of unintended mutation, not just mock call counts.
- [x] Run `go test -run 'TestCLIHuawei' -count=1` and observe RED.
- [x] Implement nested file config/defaults/flags (`huawei`, `hwzone`, `hwname`, `hwtype`, `hwline`, `hwmode`, `hwstartrow`, `hwendrow`, invocation-only `hwconfirm`) and early validation. Reject official-incompatible modes and disabled speed, invalid family/mode/ranges, partial credential overrides.
- [x] Publish from canonical final TestResults after export succeeds, independent of export row/qualification filters. Show old/new target/IP preview before asking; no confirmation or an empty qualified pool leaves DNS unchanged. Reuse store preview/update safety gates without Web-origin requirements in local CLI.
- [x] Run full tests/vet/builds; mark task complete and commit as `feat: publish ranked CLI results to Huawei DNS safely`.

## Task 3: Interactive Setup, Usage And End-To-End Regression

**Files:** Create `combined_refactor/menu_huawei.go`, `combined_refactor/menu_huawei_test.go`; modify `combined_refactor/menu.go`, `combined_refactor/cli.go`, `README.md`; extend CLI tests.

**Interfaces:**
- `runCLIHuaweiWizard(*cliHuaweiConfig) int` edits only non-secret nested DNS settings.
- `configureCLIHuaweiCredentials(*huaweiDNSStore) int` applies or explicitly encrypts AK/SK/region/project; never puts them in the CLI template or prints saved secret values.
- `confirmCLIHuaweiPreview(huaweiPreview) bool` accepts only explicit yes; EOF/back/no refuses.
- Main menu retains items 1..6 and adds a dedicated item 7 for Huawei credentials.

- [ ] Write input-driven failing tests for wizard cancel/valid ranges, credential encryption/reload without plaintext CLI secrets, confirmation yes/no/EOF, explicit false overrides across menu runs, old config defaults/round-trip, and `rewriteBoolFlagArgs` for new boolean flags.

```go
menuInputReader = bufio.NewReader(strings.NewReader("n\n"))
if confirmCLIHuaweiPreview(preview) { t.Fatal("declined update authorized") }
```

- [ ] Run focused menu/config tests and observe RED.
- [ ] Add non-secret DNS settings to the existing wizard; add dedicated credential menu and redacted config summary/help. Saved templates omit Confirmed and all AK/SK. Preserve existing provider and scheduling menus.
- [ ] Document explicit ten-rank workflow, larger measured pool, A/AAAA choice, line/zone/name, overwrite/append, interactive vs invocation confirmation, private-file portability limits, DNS answer ordering, and flags before positional `qs`.
- [ ] Run full Go/vet, all eight frontend VM and four Chrome suites, Windows/Linux builds. Inspect the diff, runtime/protected-file exclusions and secrets before committing.
- [ ] Mark all tasks complete and commit as `feat: add CLI Huawei setup and document ranked DNS workflow`.

## Acceptance

- [ ] One initial scan, no unselected detail centers, one combined global speed pass.
- [ ] Global qualified speed ranking and inclusive 1..10 selection work with arbitrary center mixes; fewer results remain valid and empty results never clear DNS.
- [ ] Existing DNS target/line/TTL remain intact; updates require current preview and explicit authority.
- [ ] Old CLI config, nonstandard CLI, Web regressions, exports and provider pages remain intact.
- [ ] Each task has its own passing tests/build and commit; no live scans/cloud writes or new push.
