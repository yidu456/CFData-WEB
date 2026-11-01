# Nonstandard Input And Scan Implementation Plan

> Execute TDD/review/verify/build/local commit, then relink/measurement slice; no push.

**Goal:** File or HTTP(S) URL input, validated NSB scan settings and real live/final result display without losing original endpoints or CSV fields.

**Architecture:** New NSB model/view wired through existing monitor, existing start_nsb_task and nsb_* events. File read occurs in browser with input-mode cancellation guards; URLs fetched only by backend. Domain state retains all live rows by IP+port and all final headers/rows; latest100 display is explicit. Official/NSB result panels switch without clearing either task data.

**Spec:** N01-N08/N12-N14, G04/data contracts; Go1.26.2 native modules, no dependencies/protected file changes.

## Steps

- [x] Node RED tests `buildNSBRequest(values,input)` exactly one source, HTTP(S) URL, positive resultLimit/threads, port1..65535, delay0, TLS bool, scanMode, compact/full/outFile. Source parsing/DNS remain backend. `reduceNSB` live IPport dedupe, sorted list authority, final variable25-column headers/rows retention, partial/error/stop preserve and new start resets, task identity follow isolation.
- [x] Implement `web/features/nonstandard/model.js` and tests. Source length/file size safety at browser input (16MiB) explicit message; no sample test data shipped. Scan request speedTest0/relinkHost empty in this slice; subsequent slice supplies real values.
- [x] Add view file read/change/error and source mode reset guarding stale async read completions. Form defaults100/0/1000/TLStrue443; TLS toggle adjusts only default443/80, manual alternate port preserved. Dedicated input read status, validation and output filename. Global monitor startTask mode argument generalized only for actual NSB caller; no duplicated socket.
- [x] NSB result panel separately shown on mode switch, includes all current live field columns and exact final CSV headers/rows. Safe text nodes, horizontal scroll container, recent100 with full stored counts; compact/full semantics preserve 25 fields. No copying/filtering/export UI until output stage; output CSV backend writes at requested file remains explicit.
- [x] Browser RED/GREEN file/URL mutual exclusivity, stale read failure, TLS fallback, validated payload, progress/global mutex, live updates/no duplicate, final25 columns/partial completion, mode persistence and320/375/768/1366 containment. No public URLs or true network scans.
- [x] Go/node/vet/browser suites and independent review, Windows/LinuxCGO0 test binary, update docs and local commit `Add nonstandard input and scan workspace`. Then relink/auto/manual speed slice.

## Limits

No automatic upload yet; publishing migration handles it. Relink/measurement not exposed until next slice. Backend protocol stores background summary/final CSV, not full live history. Result full navigation/export/Android file save is later. Local file/URL content is not persisted by new browser code.

## Verification Record (2026-11-02)

- 44 Node tests and Go uncached tests/vet pass; new NSB module and browser file-input controls initially absent (RED).
- Mock NSB browser passes file/URL mode exclusivity, deferred fileA→oversized fileB race, stale content rejection, TLS fallback, exact start payload, live endpoint dedupe, safe HTML/URL text, complete25-column final CSV, partial terminal and preserved mode data. All4 responsive widths pass; NSB mobile/desktop screenshots inspected.
- Independent review caught reading flag stuck after oversized selection and full-table conversion before visible slicing. Browser regression observed old binary start stuck (RED), new fixed binary passes. Live rendering slices100 before converting fields; full stored rows remain unchanged. URL cell redaction does not mutate retained data.
- Existing official/manual/automatic, global task/auth and real-init theme/browser suites pass. Resource tests cover new explicit embedded modules; no test sources embedded. Windows/LinuxCGO0 stripped release_assets/cfdata-test rebuilt.
- Relink/NSB measurement and automatic upload remain upcoming slices; output/download/platform full回归 not done. No real external URL fetch/scan, uploads or Android hardware tested.
