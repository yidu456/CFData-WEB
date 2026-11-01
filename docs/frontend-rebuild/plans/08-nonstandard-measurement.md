# Nonstandard Relink And Measurement Implementation Plan

> TDD/review/verify/build/local commit only, then shared output. Never push.

**Goal:** NSB automatic relink/speed plus manual relink and batch/continue speed, preserving endpoint identity and final/live result state.

**Architecture:** Extend NSB model/view with request configuration and CSV-to-live field conversion. Existing backend commands own probes/concurrency and generate results; no new algorithm. Global monitor confirms destructive relink via NSB-owned native dialog with task/source identity guard.

**Spec:** N09-N11/N15, S01-S10, complete25 fields. Native modules/Go1.26.2; protected code untouched.

## Steps

- [x] Node RED: autoSpeed off speedTest0; on restores positive default5 and relink off follows public probe; custom probe accepts HTTP(S)/domain only; settings reuse validated speed source/min/limit. CSV25 header mapping reconstructs every nsbScanMessage including originalInput/IPport; batch sends all retained rows not visible100; continue preserves speed; fresh clears; live updates to final CSV cells keep identity/headers.
- [x] Implement exports `buildNSBExecution(values)`, `getNSBResultRows(state)`, `buildNSBFollowup(values,state,action,continued)` in existing NSB model; do not generalize official request. Form extra speed settings/probe select/input, automatic toggle; manual speed enables execution, relink confirmation explicitly notes candidates removed.
- [x] Extend model `nsb_followup_requested` to preserve full rows/metadata, replace final display from actual future CSV or update speed field by endpoint when batch returns live events only; stop retains acquired rows. Final outcomes not overwritten by generic completion. No stale final data masking live speed updates.
- [x] Native relink dialog validates same retained-result identity on confirmation; no control request after source/task changed. Actions disabled under global running/pending/connection lock. Auto URL availability labels reuse existing init fields; source/probe validation blocks secrets in logs, actual raw source retained for backend.
- [x] Mock-browser auto config/probe link, manual confirmation cancel/accept, exact followup payload and full field mapping, batch/continue skip flags, final25-row updates, partial stop/error/mode/mobile. No real probes/downloads.
- [x] Node/Go/vet/browser full regression, independent review/no blockers, Windows/LinuxCGO0 test build; local commit `Add nonstandard relink and measurement flows`.

## Limits

Publishing/exports/filters/full-table navigation remain future steps. Source URLs/credentials not stored; URL data visibly redacted. No task history/queue or multi-subscriber backend broadcast promised. Automatic publishing still stage6. Native probe errors preserved as failure text, never numeric0.

## Verification Record (2026-11-02)

- 47 focused Node tests and Go uncached tests/vet pass. New execution/mapping functions and browser controls observed RED; custom URL probe and endpoint-index regressions failed before fixes and now pass.
- NSB browser mock passes manual speed/relink payload, OriginalInput/25-field mapping, CSV speed updates, relink confirmation cancel/accept, disconnect invalidating confirmation and authoritative empty final disabling actions. Prior file/source race, safe data, partial/mode/layout tests remain green.
- Probe URL normalized to hostname (TLS/SNI and HTTP Host contract). CSV endpoint updates indexed; ready checks inspect pending fields directly, not rebuilding all full rows. All retained rows still serialize at actual followup dispatch, as required for full-data scope.
- Independent read-only review reran47 tests and confirmation/empty-final fake-DOM checks; no blockers. Windows and LinuxCGO0 stripped release_assets/cfdata-test pass, all real-init/official/task/mock browser suites green.
- No live public probe/download/upload, APK or service lifecycle tests. Full output/filter/export/publishing/settings/platform migration remains future work; existing complete UI still available. Automatic upload not claimed.
