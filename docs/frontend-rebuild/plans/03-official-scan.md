# Official Scan Workbench Implementation Plan

> **For agentic workers:** Execute inline with TDD and independent read-only review. Verify/build, commit this working slice locally, then continue detailed-test/measurement slice. Never push.

**Goal:** Deliver the official IPv4/IPv6 scan controls, real scan rows and data-center summary/selection in the new workbench.

**Architecture:** Official model validates request values and stores scan/DC state independently from global task state. The view sends existing start_task through the global monitor, receives existing scan_result/scan_complete_wait_dc events and renders safe rows. DC selection is retained for the next detailed-test slice; it does not falsely claim to execute a test yet.

**Tech Stack:** Native JS/CSS/Go embed, Go 1.26.2, Node built-in tests and installed Playwright.

**Spec:** `../design.md`, `../modules/official.md` (O01-O07 and DC selection part of O08), `../data-contracts.md`.

## Constraints

- Local commits only on frontend/rebuild; no main/remote/.github/reference/Android changes.
- Preserve all legacy workflows until corresponding slices are complete. This step does not expose unsupported automatic-test/speed/upload controls.
- Native form with IPv4/IPv6, all 13 existing ports, concurrency default100, delay default500ms (0 allowed), TCPing/HTTPing with clear measurement text.
- Validate integer types and ranges at the browser boundary: family4/6, port1..65535, concurrency1..10000, delay0..60000; accepted port list matches existing UI. No silent fallback for malformed inputs. Upper concurrency/delay limits are a first-release browser safeguard; Go behavior is unchanged.
- Global task start sets pending/running synchronously only after send succeeds; disable start during running/stopping/uninitialized/disconnected state and keep one task state across navigation/mode.
- Keep all received rows, keyed by IP+port; display up to latest100 live rows and all DC summaries. Full result-table/output functionality is stage5; label this bound explicitly, no hidden data discard.
- Rows use exact fields IP/Port/DataCenter/DCCountry/Region/City/LatencyStr/TCPDuration. Nanoseconds convert to milliseconds for numeric sorting only; never display raw nanos as ms. Unknown latency remains unavailable, not zero.
- Stopping/errors retain partial rows/DC data; new successful start resets previous rows and selection. Followed-background scan summaries can be displayed without claiming historical scan rows exist.

## Files And Interfaces

- Create `web/features/official/model.js`: `buildScanRequest(values)`, `createOfficialState()`, `reduceOfficial(state,event)`, `visibleScanRows(state)`; retain normalized full rows and sorted DC values.
- Create `web/features/official/view.js`: `mountOfficial({root,monitor}) -> {onMessage,onState}`; native submit/DC selection and safe DOM rendering, no direct socket creation.
- Modify `web/features/tasks/{state.js,view.js}`: request_start transition and monitor `startTask(type,data,label)`, `getState()`, `isReady()` bridge; notify app views on message/state changes.
- Modify `web/app.js`, `web/index.html`, `web/styles/{layout.css,controls.css}`, `types.go`, `frontend_test.go`: wire/markup/resource packaging.
- Create `web/tests/official.test.mjs`, `official-browser.mjs`; adjust shared smoke if newly visible fields demand layout checks. Update docs status and local planning ledger.

## Task 1: Model And Request Validation

- [x] Write tests for family/13 ports/default values/zero delay/bad integers/invalid bounds; duplicate IP-port events update the same row; latency nanoseconds sorting and DC count/minimum sorting; unknown malformed events do not inject fields; selected DC cannot be arbitrary; no reset after partial completion.

```js
assert.equal(buildScanRequest({ipType:'6',port:'443',threads:'100',delay:'0',scanMode:'httping'}).delay, 0);
assert.throws(() => buildScanRequest({ipType:'4',port:'443',threads:'abc',delay:'500',scanMode:'tcping'}));
let state = reduceOfficial(createOfficialState(), {type:'scan_result', data:{IP:'192.0.2.1',Port:443,TCPDuration:12000000}});
assert.equal(state.rows[0].latencyMS, 12);
```

- [x] Run `node --test web/tests/official.test.mjs`, see missing module failure, implement model, rerun green.

## Task 2: Working Scan UI And Global Start Coordination

- [x] Add regression for global request_start clears previous outcome and owns task mode immediately; error before first progress becomes failed and permits retry; task control remains available. Run RED before code changes.
- [x] Implement native controls, scan submit and validation message; disable form submit under global lock; preserve selected form values while running. Monitor handles pending start distinct from stop/follow and releases pending upon progress/error/task completion. Start time identifies new foreground task for modal safety.
- [x] Render recent scan rows with whole stored-count label; DC summaries appear on completion (including stop-partial), selected DC has aria-pressed/clear selected label. Detailed-test action remains explicitly pending and links legacy; next slice will replace it.
- [x] Extend embed only for `web/features/official/*.js`; add Go asset tests. Wire message/connection notifications from app without introducing generic event bus.

## Task 3: Verification And Local Commit

- [x] Mock-browser scan: submit exact WS payload, double-submit disabled, navigate/switch mode preserves global task, scan rows/DC and selection safe, malicious text not HTML, partial stop/result and early error, retry resets only after successful send. Do not run public scans.
- [x] Go tests/vet and all Node tests; real backend smoke remains initialized. Check desktop/mobile320/375/768/1366px input/table containment and touch targets, inspect screenshots; independent review without blockers.
- [x] Build Windows and Linux CGO-disabled release_assets/cfdata-test; update truthful coverage/limits; local commit `Add official scan controls and data-center workspace` then proceed to detailed testing/measurement step.

## Verification Record (2026-11-02)

- RED: new official module absent, global local-start transition absent; independent review regressions confirmed different-task result mixing and quadratic retention at 65,536 rows.
- GREEN: 29 Node tests pass; large IPv6 retention is indexed and frame-batched, 65,536 rows retain all data with bounded 100-row display in approximately 50-66ms of model work in this environment (not a hardware guarantee).
- Go uncached tests and vet pass; Windows and stripped CGO-disabled Linux amd64 test binaries built. Test modules are not embedded.
- Official mock browser passed exact request/all ports/zero delay, running mutex, stop controls before first progress, safe live rows, DC selection, partial stop, early error/retry and different-background identity isolation.
- Existing task and real-init browser regressions pass, all sections at 320/375/768/1366px. Desktop/mobile scan screenshots visually inspected.
- Independent review reran tests and identity/index assertions; no remaining blockers. Same-task background handoff preserves partial rows; following a genuinely different background task clears prior scan/DC selection.
- No public scan, upload, APK or CLI scheduling tested. Detailed test/auto/batch/single/continue measurement remains for subsequent slices; complete legacy entry remains available.
