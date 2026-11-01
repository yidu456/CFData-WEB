# Official Measurement Implementation Plan

> TDD, independent review, full verification/build and local commit. No push; automatic scan/test/speed orchestration is the following slice.

**Goal:** Preserve official single/retest/batch/continue speed controls and truthful source, qualification, failure and rate-limited states.

**Architecture:** Add a small measurement model for source/request/status, extend indexed official result state and use monitor.startTask. Backend stays canonical for source precheck, sequential measurement, candidate selection and rate-limit stop. No parallel official worker setting.

**Spec:** `../modules/measurement.md` S01-S10, `../modules/official.md` O11, and data contracts. Native modules/Go1.26.2, local-only commits/protected files unchanged.

## Steps

- [x] Write `web/tests/measurement.test.mjs` RED for source auto/presets/custom scheme/domain validation, positive speedMin/default0.1, integer speedLimit/default5 and0 disabled, raw TestResult payload fields, four speed status categories (text failure not0), threshold equality, single endpoint membership and all stored batch rows independent of visible100.
- [x] Implement `web/features/measurement/model.js` exports `buildSpeedConfig(values)`, `speedStatus(text,min)`, `buildSpeedRequest(config,rows,ip)` and `buildBatchRequest(config,rows,continued)`; native request uses original nano/loss values, not display conversions.
- [x] Extend official model for `speed_requested`, `speed_test_result`, `official_speed_complete`, tested/continue state and active single endpoint. New non-continued batch clears display speeds after successful send, continue preserves and sends skipTested:true. Stop/failed/limit keeps acquired speeds.
- [x] Extend measurement controls and official table per-row speed/retest action, source options/status, one-key/continue actions; all controls obey global mutex. Source precheck only displays backend status/query, not a fake recheck. Add explicit attempt versus qualified progress counters in monitor. Update native embed/resource tests.
- [x] Add browser RED/GREEN single result, failure/qualification, batch/continue skip flag and preservation, source unavailable, speedLimit0, limits+generic completion, navigation/mobile, no arbitrary IP input. No public scans or real download targets.
- [x] Ensure single speed results persist into backend testResults for continue skipping; regression test helper without network. Add port to result payload compatibly so endpoint identification survives retest port changes, keeping old ip/speed keys for legacy. Inspect all callers before touching shared speed code.
- [x] Run full Node/Go tests/vet, existing browser suites and measurement mocks; review no blockers; Windows/LinuxCGO0 test build. Commit `Add official single and batch measurement controls` locally; next automatic workflow step separate.

## Limits

Existing backend batch chooses its cached testResults before fallback rows; source headers/auth not new features. Browser custom URL accepts http/https/domain form and does not store credentials. Result filtering/export/full table navigation remains stage5. Full rows retained even though only100 rendered; batch scope is all stored detailed candidates. No durable history or new concurrency/queue/schedule.

## Verification Record (2026-11-02)

- 39 focused Node tests and Go uncached tests/vet pass. New config/status/request/state and single-cache tests observed RED before implementation.
- Mock browser checks single/retest, qualified/failure status, batch/continue flags and speed preservation, rate-limited outcome surviving generic completion, global-port change with original row identity, and speedLimit0 disabling batch. Existing official/detail/task/real-init browser suites and 320/375/768/1366px remain green; screenshots inspected.
- Source presets/auto/custom and backend availability labels are wired. There is no fake recheck action. Official attempt and qualification counts display separately; no invented concurrency setting.
- Independent review found measurement-port versus row-port mismatch, stale cached tail speeds in fresh batches, and background replay not unlocking continue. Optional `resultPort` is backward compatible; legacy defaults to its selected port. Batch result envelopes now identify original row port. Fresh backend cached/fallback speeds clear before core; pre-cancelled Go test confirms no stale speed survives. Replayed final rows unlock continue from measured state. All regressions pass and re-review reports no blockers.
- Windows build and CGO-disabled stripped Linux amd64 release_assets/cfdata-test pass; generated artifacts ignored. No public speed downloads, uploads, APK hardware or Linux scheduling tested.
- Automatic scan/test/speed sequencing and publishing are not complete in this slice. Full output/filtering and all result navigation remain later migrations; legacy complete UI is unchanged.
