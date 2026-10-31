# Executable-Local Data and Huawei Page Plan

> Execute sequentially with superpowers:subagent-driven-development; controller owns commits/pushes.

**Goal:** Store program-owned default data beside the executable, move saved Web state out of localStorage, and simplify Huawei configuration. Browser downloads stay unchanged.
**Architecture:** Explicit executable-local paths, existing Windows DPAPI/atomic writes, bounded authenticated Web-state operations, and existing Huawei forms grouped into three sections. No global Chdir, database or dependencies.
**Spec:** docs/superpowers/specs/2026-10-30-exe-data-huawei-design.md (approved 2026-10-30).
**Tech Stack:** Go1.26.2, embedded HTML/CSS/JavaScript, Windows DPAPI, Node assertions, installed Chrome/Playwright.

## Global Constraints

- Existing worktree D:/Program_code/cloudflare_worker/cloudflare-speedtest/.worktrees/cfdata-web-dev; dev base5f6a86a44ec3640e12f0135d3e24810951660704. Remote yidu456 is git@yidu456:yidu456/CFData-WEB.git. Controller reviews, commits and pushes each task.
- No main, .github, root reference code, original production EXE changes, installed schedulers, cloud writes/scans or new dependencies. CLI may receive shared default-path wiring only; preserve explicit paths and scheduler semantics.
- Web downloads retain Blob/download behavior; no result-save endpoint, duplicate export or automatic history. Transient task/File objects remain transient.
- No os.Chdir; no persistence fallback to AppData/plaintext/localStorage. DPAPI remains Windows-current-user bound; unsupported encrypted writes fail closed; memory remains available.
- New persistent operations require existing strict same-origin/local or active-login checks, bounds/schema validation, atomic updates, safe errors and no secret logging/export. Storage loads grant no automatic publish authority.
- Preserve original account formats/unlock/upload behavior, completed-batch provenance, immutable authorization, confirmation and independent statuses. Old source cleanup is explicit and only allowed after verified migration; no failed-write data loss.
- Go executable $env:TEMP/cfdata-go1.26.2/go/bin/go.exe. Node C:/Users/Administrator/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node.exe, NODE_PATH sibling node_modules, CHROME_PATH C:/Program Files/Google/Chrome/Application/chrome.exe.
- Test-first focused RED/GREEN. Controller runs go test -count=1 ./..., go vet ./..., scoped gofmt and all Node/actual mocked Chrome tests. Build CGO0 Windows ../release_assets/cfdata-web-preview.exe (pending name if running) and Linux ../release_assets/cfdata-test after each step. No suite repetition by reviewers without a specific unanswered doubt.
- Preview launches previously blocked by tool policy; do not bypass or claim a running preview. Preserve originals; use allowed separate loopback launch only if permitted.

## Task 1: Shared Executable Paths and Huawei Compatibility

**Files:** create combined_refactor/data_paths.go, data_paths_test.go; modify storage.go, asn.go, debug.go, tasks_compact.go, tasks_nsb.go, cli.go, huawei_dns.go/huawei_web.go and focused tests. main.go only startup wiring if needed. README relevant text.
**Consumes:** defaultCLIConfigPath(), defaultDebugLogPath(), getIPListContent(), initLocations(), cache cleanup, writeNSBCSV(), CLI export, saveHuaweiPrivate(), store.load/status/configure and authorized Huawei actions.
**Produces:** programDataDir() (string,error), defaultDataPath(name string) (string,error); default program-owned files resolved consistently without modifying process CWD. Huawei defaults to exe/huawei-dns.private; legacy AppData availability/migration status and explicit authorized migration/cleanup actions for Task3.

- [x] Trace every default path read/write/remove/temp caller, separating explicit inputs/outputs. Focused tests inject a temp executable location and different CWD; require locations/IP/ASN/debug/config/results to resolve beside executable and cleanup to leave same-named launch-directory files untouched. Preserve safeFilename and explicit-input semantics; errors cannot silently target CWD.
- [x] Example executable resolution check: `got, err := defaultDataPath("locations.json"); if err != nil || got != filepath.Join(exeDir,"locations.json") { t.Fatalf("unexpected path: %q",got) }`. Add real temp read/write and failure tests, not assertions only on constructed constants.
- [x] Wire shared paths into cached-file reads/writes/cleanup and default output/temp paths. Keep explicit path behavior; propagate path errors. Do not Chmod the executable parent directory merely to save a private file; private files retain atomic replacement and restrictive mode.
- [x] Keep old DPAPI ciphertext readable. Legacy AppData file never auto-overwrites an exe file. Implement explicit migration only when destination absent, decrypt/validate legacy full config and atomically save/reload before claiming success. Explicit cleanup requires a successful migration and unchanged verified source/destination; preserve sources on failure/tamper/unsupported platform. Safe capability/path/legacy metadata only.
- [x] Test old/new precedence, absent/invalid/locked paths, failed migration/reload, cleanup before migration and source changes, no cloud calls, authorization and full DPAPI compatibility. Migration actions preserve current authority and disable stale DNS preview/opt-ins when config changes.
- [x] Report stable interfaces/evidence to .superpowers/sdd/2026-10-30-exe-data-huawei/task-1-report.md. Review, controller verification/builds; commit/push `feat: store default program data beside executable`.

## Task 2: Encrypted Web State and Browser Migration

**Files:** create combined_refactor/web_state.go, web_state_test.go, web_state_ui_test.js; modify server.go/main.go for strictly authorized dispatch/startup, index.html and direct existing UI fixtures/smoke, README/.gitignore only relevant persistence changes.
**Consumes:** Task1 programDataDir/defaultDataPath, existing DPAPI/private atomic writer and huaweiWebAuthorized(r); existing account objects/tokenCipher/passwordCipher and UI save/load/theme/autoload calls.
**Produces:** executable-local web-state.private and explicit web_state_get/web_state_patch/web_state_migrate/web_state_legacy_cleanup_ready acknowledgement (cleanup browser-side only). Strict validated sections: form/config, theme, autoload, githubAccounts, edgetunnelAccounts. Safe storage metadata separated from explicit authorized content response; patch serialized/CAS or equivalent stale-write protection. Exact API documented in report for Task3/4.

- [x] Write failed-save/load/tamper/unavailable/path tests and authorized action tests. Reject unknown/oversized sections, malformed account arrays/URLs/field values, unauthorized/foreign-origin reads+writes and stale competing writes. Account/theme patch must preserve other sections. Use temp DPAPI files/mocked transports, not actual providers.
- [x] Example preservation check: apply theme patch then account patch and compare saved+reloaded theme/account sections. Locked destination save must leave bytes/revision/memory unchanged. Unsupported platform must produce an error before any file write.
- [x] Add minimal bounded backend store and startup load; reuse DPAPI and atomic writer. Never restore cloud-write opt-ins/task snapshots from saved state or import. Browser get/patch returns request-correlated results; failures not optimistic persisted success.
- [x] Replace localStorage writes/normal reads with backend-backed memory state and async operations. Trace account save/delete and token migration callers so they await persistence/retain existing values on failure. Existing account ciphertext format/unlock remains compatible, including origin-bound ciphertext migration limits explained to user.
- [x] Theme/autoload/config follow backend storage, no hidden plaintext/browser fallback. During unavailable backend allow memory use with truthful unsaved state. Reconnect loads safely without overwriting dirty unsaved fields or reenabling automatic publication. Save/load should preserve selected DC membership but only restore discovered selections when discovery is available.
- [x] Explicit legacy-browser import reads old keys only on migration request; no stale overwrite of existing backend state. Keep source keys until explicit cleanup after acknowledged successful unchanged import; exclude auto-publish opt-ins and invalid data. Browser cleanup cannot remove unrelated keys; failures/changed source preserve old data.
- [x] Real mocked Chrome checks reload/reconnect/preferences/account saves/deletes/migration/failure/unsupported, no new persistent localStorage writes, no secret logs/errors, no cloud actions, and unchanged browser download event/content. Existing4Node suites stay green; add actual function tests for new branches.
- [x] Report .superpowers/sdd/2026-10-30-exe-data-huawei/task-2-report.md; review/controller verify/builds; commit/push `feat: persist Web preferences and accounts in encrypted local state`.

## Task 3: Clear Huawei Sections, Help and Single Entry

**Files:** index.html, huawei_ui_test.js/result_actions_ui_test.js, tests/web_preview_smoke.cjs and README. Tests may add focused huawei_page_ui_test.js.
**Consumes:** Task1 Huawei migration/status/actions, Task2 storage metadata/state APIs; existing Huawei field IDs, open/back/focus/notice, discovery/preview/confirmation operations.
**Produces:** only resultHuaweiChoice entry, grouped connection/target/preview interface, official helper links and explicit legacy migration/cleanup controls. Existing field IDs and publishing contracts unchanged.

- [x] Verify official Huawei documentation/console URLs without credentials; record sources in report. AK/SK create/manage, regional Project ID, regions and public zone/record console links use target=_blank rel=noopener noreferrer. Manual key creation only; no credential-retrieval API or external link auto-navigation.
- [x] Failing tests require old settings-menu Huawei shortcut/heading absent while empty-result chooser DNS remains reachable. Section headings/actions/status ownership clear; one instance per credential field; no navigation cloud mutation.
- [x] Reuse existing visual tokens/font; three semantic sections: Connection and credentials, DNS target, Preview and update. Compact safe status summary; paths/encryption/endpoint and migration explanations expandable. Separate apply/save/read-only discovery/update feedback; action-disabled conditions correct. Native keyboard and 390px mobile scrolling/focus/back/notice traps retained.
- [x] Add available legacy-file migration and separately confirmed cleanup using Task1 actions; saving or following help never publishes. Show exact effective file location and unsupported/unwritable errors. Preserve automatic-publication explicit opt-in behavior.
- [x] Mocked actual Chrome desktop/mobile screenshots and keyboard help/back/Escape/empty entry/migration/discovery/preview tests, all existing suites; report task-3-report.md. Review/controller verify/builds; commit/push `feat: clarify Huawei DNS setup and remove duplicate entry`.

## Task 4: Integrated QA and Final Delivery

**Files:** tests/web_preview_smoke.cjs and README only gaps discovered in documented QA; source fixes assigned to original implementer with regression tests, not broad refactor.
**Consumes:** Task1 paths/migration, Task2 Web state and Task3 UI.
**Produces:** reproducible offline integrated test coverage, truthful run/storage/migration docs, verified Windows/Linux artifacts and final whole-branch review.

- [ ] Check path resolution from different launch CWD, legacy/new precedence, encrypted full saved state, per-section updates/reconnect/dirty edits, unchanged download and single Huawei entry. Run all Node suites, actual Chrome smoke, uncached Go tests/vet/scopedfmt/whitespace. Windows DPAPI runs natively; Linux build/test crosscompile without claiming Linux runtime/race/Android verification.
- [ ] Inspect desktop/mobile screenshots. Document storage writable-directory requirement, explicit migration/cleanup, current-user DPAPI binding and unsupported nonWindows writes; CLI/scheduling not newly implemented. No logs/screenshots with live secrets.
- [ ] Broad complete-delta review from5f6a86a through staged final changes, fix regression findings via implementer and scoped re-review. Commit/push `test: verify executable-local storage and Huawei setup flows` only if actual Task4 tracked changes exist; otherwise record verification with no empty commit.
- [ ] Build both artifacts; if an allowed separate hidden loopback preview launch is possible verify exact PID/EXE/13336 bind/HTTP/new UI. If policy blocks it, document manual command and leave live preview verification explicitly pending, no alternative policy bypass. Never replace/stop original production process.
