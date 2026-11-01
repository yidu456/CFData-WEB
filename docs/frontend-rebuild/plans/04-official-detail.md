# Official Detailed Test Implementation Plan

> Execute with TDD, independent review, verification/build and a local commit; do not push. Continue measurement after this working slice.

**Goal:** Turn selected DC into the existing start_test workflow and preserve full/partial detailed results with correct duration/loss display.

**Architecture:** Extend the existing official model/view, reuse indexed collections and global monitor.startTask. The Go testing algorithm and protocol stay unchanged.

**Spec:** `../modules/official.md` O08/O09 and `../data-contracts.md` 4.2. Native modules, Go1.26.2, no dependency additions; protect main/.github/reference/Android.

## Files And Steps

- [x] Extend `web/tests/official.test.mjs` first: buildTestRequest validates selected DC exists and reuses current port/delay/scanMode with 0 delay; detailed rows dedupe IP+port, preserve metadata/Speed, convert nanos to ms and LossRate*100, keep unavailable values null. Final test_complete replaces streamed set; stopped/error retain partial. New scan clears old test data; retry test resets only detailed data. Run RED.
- [x] Extend `web/features/official/model.js`: `buildTestRequest(values,selectedDC,centers)`, `test_requested`/`test_result`/`test_complete`/task terminal transitions, indexed testRows/testIndex, `visibleTestRows(state)`; selectedDC remains stable, testingDC/testedDCs label current/successful centers. Read final arrays as authoritative; late replay metadata remains intact. Run GREEN.
- [x] Add browser regression in `web/tests/official-browser.mjs` before UI: choose DC, submit start_test exact payload, start disabled while task running, receive live row with12ms/15ms/20ms/5% loss, final array, retry clears tests not scan, stopped retains partial and error becomes failed. Confirm existing scan, mode/navigation, background follow isolation, responsive and safe text tests remain green.
- [x] Extend `web/features/official/view.js` and `web/index.html`: selected DC test/retest button, visible testing/completed labels, partial test table and bounded100 display label; native action accessible on mobile. Global task controls own stop/background; no second socket. All duration/loss fields use explicit model conversions, speed text remains unparsed. Modest CSS reuse only.
- [x] Run all focused Node tests, Go tests/vet, real-init and official/task browser mocks. Inspect mobile/desktop screenshot. Independent read-only review; fix blockers using regression tests. Build Windows smoke and CGO-disabled Linuxamd64 `release_assets/cfdata-test`.
- [x] Update coverage and local plan; commit `Add official data-center detailed testing` locally. Next slice is single/batch/continue/automatic measurement, not yet implemented here.

## Limits

TCPing10 / HTTPing3 rounds and candidate generation remain backend-owned. Partial stopped rows come from already-streamed test_result events because backend does not emit test_complete on cancellation. No full-history replay, export/filtering, measurement start or automatic pipeline in this slice; complete legacy UI stays available. Latest100 visible detailed rows do not discard full stored data.

## Verification Record (2026-11-02)

- 33 focused Node tests and Go uncached tests/vet pass. Request/model/browser tests failed first on missing detailed functions, rows/action and connection candidate state.
- Official mock browser validates selected-DC exact start_test, zero delay, correct 12/15/20ms and 5% loss display, retest/partial retention, global controls, early error and reconnect invalidating server candidates without deleting displayed results.
- Independent review found stale backend retest cache and stopped-replay status masking. Focused pre-cancelled Go test failed with prior NRT cache before the fix. `runDetailedTest` now clears prior results and caches current partial rows before cancelled/no-result returns; no probe algorithms/round counts changed. This minimal backend lifecycle adjustment is part of the slice, not excluded as a frontend-only change.
- Stopped/background replay now keeps its specific terminal phase/message while updating available counts. Read-only re-review reran official/task tests and reported no blockers.
- Windows and CGO-disabled stripped Linux amd64 test builds pass; release_assets/cfdata-test updated. Desktop/mobile detailed-table screenshots inspected; all four responsive widths and existing theme/tasks regression suites pass.
- No public scan/measurement, live upload, APK hardware or CLI scheduling exercised. Actual automatic measurement pipeline and complete output/filtering remain later steps. Foreground reconnect creates a new server session: detailed tests disabled until a new scan or successful background follow restores candidates.
