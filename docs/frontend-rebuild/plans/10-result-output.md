# Result Copy And Export Implementation Plan

> Execute inline with TDD, independent review, verification, test builds and a local commit. User already approved continued staged execution; no push.

**Goal:** Migrate R09 and E01-E12 for all three result types without coupling output to table filters or publishing credentials.

**Architecture:** Pure normalized output rows and formatting; separate in-memory copy/export preferences per result type. Native dialog uses the same generated content for preview and execution. Whole authoritative view getters feed output. Platform clipboard/download helpers retain the Android global injection contract.

**Tech Stack:** Embedded native ES modules/CSS, Node tests, installed Playwright, Go1.26.2; no dependencies.

**Spec:** `modules/results.md`, `design.md`, Android `window.downloadFile(content,nameBase,ext)`.

## Constraints

- Only frontend/rebuild and maintained module/docs. Legacy default, Worker, .github and Android Java remain unchanged.
- Copy/export independent of table preferences, each other and task scope; default range1-20 applied after independent filters. Copy has no latency/loss controls.
- TXT always IP:PORT then optional # and selected nonempty extras. Defaults official dc/dcCountry, NSB dc/loc. IPv6 brackets optional without doubling.
- CSV escapes headers and values; custom constants support label:value and key=label:value, comma-separated, selected and ordered like fields.
- File export timestamp/BOM/MIME on desktop; call current global Android override at execution, pass plain content because native adds BOM.
- No stored credentials, history, remote uploads or framework added.

## Task 1: Pure Output And Platform Contract

Files: new `web/features/results/output.js`, `web/platform/output.js`, tests `web/tests/output.test.mjs`.

- [x] RED tests normalization (official latency AVG and raw metadata preserved, NSB full protocol fields), independent filter/range defaults105rows, strict thresholds and invalid range, TXT extras/order/actual space/brackets, escaped CSV headers/values, constant collision/safe keys and defaults.
- [x] Implement `normalizeOutputRows(rows,type)`, `createOutputConfig(type,kind)`, `parseCustomFields(spec)`, `buildOutput(rows,config,speedMin)` returning `{content,total,count,fields}`. Config fields are ordered keys; presets defaults/all/ipport.
- [x] RED platform tests clipboard rejected API -> execCommand fallback, fallback failure cleanup/focus; download BOM/MIME/name/revoke; native bridge plain content.
- [x] Implement `copyText(text,{navigator,document})`, `downloadText(content,name,format,environment)`; global download installed only if absent. UI must call window.downloadFile, not an imported captured override.
- [x] Run Node tests GREEN before UI integration.

## Task 2: Integrated Result Actions

Files: new `web/features/results/output-view.js`; modify `web/app.js`, official/NSB views, `web/index.html`, `web/styles/controls.css`.

- [x] RED browser checks copy/export entry on each result, default20 preview despite1 table row, independent range/constants/order/filter, preview equals clipboard/download, CSV escaping, rejected clipboard fallback, safe IP detail/source domain hint, no overflow320/375/768/1366.
- [x] Add `getOutputRows(type)` and `getSpeedMin()` providers. Add shared native IP/port buttons and external ippure link using DOM text/encoded URL, safe source hint, feedback via role=status.
- [x] Mount output controls; dialog binds independent source/kind configs, fields/all/ipport/default/up/down, constants, delimiter preset/custom including actual space, brackets, range/IP/status and export-only latency/loss/filename/format. Recompute preview on data events while open; validate and recompute at action time to avoid stale content. Empty/error disables action and explains it.
- [x] GREEN all browser suites; visual inspect output dialog desktop/mobile/light/dark.

## Task 3: Verification And Local Commit

- [x] Independent read-only review; fix findings with regression tests, re-review.
- [x] `node --test web/tests/*.test.mjs`; Go `test -count=1 ./...`, `vet ./...`; Windows whole-package build, Linux amd64 CGO0 build `release_assets/cfdata-test`.
- [ ] Record actual verification/limits in this plan and docs index, stage only related files, local commit `Add independent result copy and export workspace`.
- [ ] Begin publishing plan immediately after commit; do not push or change remotes.

## Verification Record (2026-11-02)

-60 Node tests pass, including output/platform7 RED/GREEN and official source metadata/exact-endpoint speed join regressions. Output controls were absent in browser RED; independent copy/export config, full105row scope behind1visible table row, filtered-range/default20 and preview equality now pass.
- All six browser suites pass against latest Windows embedded build: foundation/tasks/official/NSB/results/output. Desktop file download bytes verified exact preview plus UTF8 BOM and timestamp/MIME; clipboard rejection fallback and native late override covered with mocks.
- Review found and fixed scan-qualified output speed missing, invisible NSB feedback and keyboard toggle/boundary focus. Browser tests also reproduced stale pending clipboard callback overwriting another dialog; generation guard verified. Independent re-review no blockers;60 Node tests independently rerun.
- Go uncached tests/vet, whole-package Windows build and Linux amd64 CGO0 test build pass. `release_assets/cfdata-test` rebuilt. Output dialog320/375/768/1366 and dark screenshots visually inspected; all controls remain within viewport, dialog scrolls vertically.
- Copy/export preferences only retained in-page independently per result type; refresh resets defaults as stated in UI. No publishing credentials or table/task input changes, remote push, default route switch, real public scans/upload or physical Android/APK verification. Huge result sets still live in memory; output while open recomputes full selection on new data.
