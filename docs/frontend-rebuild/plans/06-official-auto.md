# Official Automatic Workflow Implementation Plan

> TDD, review, verification/build, local commit only; next nonstandard workflow. No push.

**Goal:** A single official action runs scan, selected/automatic DC detailed test and optional qualified-speed batch, including after background transfer.

**Architecture:** New workbench requests an optional `serverAutoFlow` flag on existing start_task; backend owns sequential followup while old clients remain unchanged. Reuse native forms, global monitor and existing result handlers—no frontend duplicate stage commands.

**Spec:** O10 and existing measurement/task contracts. No algorithm, UI framework or schedule rewrite. Protected branches/files unchanged.

## Steps

- [x] Add Node RED tests `buildAutoScanRequest(scan,speed,target)` explicit auto/serverAutoFlow and all current speed params; empty target auto chooses, invalid target rejected, speedLimit0 still requests detailed test without speed batch.
- [x] Add Go RED tests for followup eligibility (legacy foreground false, legacy background true, explicit server flow foreground true, cancellation false) and DC selector picks available requested center/lowest-latency default; unavailable target reports error rather than running arbitrary candidates.
- [x] Add small Go auto helper in tasks_official.go, optional protocol flag and server start_task conditions. Only explicit new flag removes background-only gate; legacy automatic behavior stays compatible. Explicit server flow clears stale detailed cache at start, stops after failed scan/detail, zero limit skips speed. Retain existing single global task lock for entire sequence.
- [x] Native automatic toggle/DC input in official form, compose request from validated scan+speed settings; no automatic frontend start_test/batch sends. Keep changing form settings from mutating running request. Target available in final summary selected for display; status clarifies pipeline ownership and zero-limit behavior.
- [x] Mock-browser auto request and phase sequence including background transfer: one start_task, no duplicate start_test/batch, global running during intermediate completion, stopped/failure prevents success overwrite. Existing manual regressions unchanged.
- [x] Full Node/Go/vet/browser and mobile screenshots; independent review with blockers fixed; Windows/LinuxCGO0 test build. Local commit `Add backend-owned official automatic workflow`.

## Boundaries

Automatic upload is not dropped: it moves to publishing migration with saved targets (stage6). New pipeline therefore does not promise upload now. Real public scan/download, Linux service/Android lifetime and actual network success remain unverified; backend process lifetime governs background continuation. Current protocol does not restore all historic rows after reconnect.

## Verification Record (2026-11-02)

- 41 focused Node tests and Go uncached tests/vet pass. Ownership/target/zero-limit tests observed missing-function/flag RED before implementation; auto DC-label regression failed before the model update and now passes.
- Optional serverAutoFlow is explicit on new workbench requests. Legacy foreground clients retain their own stage orchestration; legacy background auto still continues. One accepted task owns scan/detail/speed; cancellation ends followup. Explicit flow clears stale scan/detail data before starting, and target DC must exist in current rows.
- Official browser mock verifies one start_task carries current validated fields/target, intermediate detailed completion does not enable a second start, no duplicate start_test or speed-batch commands, and final actual DC selected/tested labels. Existing manual, reconnect, task/auth/stop and responsive suites remain green.
- Independent review found/fixed missing automatic DC labels; re-review reports no blockers. Windows and stripped CGO0 Linuxamd64 test binaries rebuilt; release_assets/cfdata-test updated.
- No public scans/downloads, uploads, APK or scheduling tests. Automatic publishing remains an explicit stage6 migration, not implemented or claimed complete. Server-owned pipeline and background continuation are contract/unit/mock verified, not a live network success guarantee.
