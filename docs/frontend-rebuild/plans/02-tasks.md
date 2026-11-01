# Backend Connection And Global Tasks Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans and TDD. Complete verification and independent review before the local milestone commit; do not push.

**Goal:** Connect the new shell to the existing backend and make global/background tasks observable and controllable without migrating task creation or result tables prematurely.

**Architecture:** A DOM-free WebSocket client owns reconnect/authentication/lifecycle. A pure task reducer owns snapshots, progress and specific terminal outcomes. A task view/controller renders safe text, polls discovery, confirms stop/background actions and keeps monitoring outside navigation panels. Existing task starts remain in legacy until stages 3/4; following does not promise historical rows.

**Tech Stack:** Go 1.26.2, native browser modules and WebSocket, existing Node runner/Playwright only.

**Spec:** `../design.md`, `../modules/tasks.md`, `../protocol.md`, `../modules/access-platform.md`.

## Global Constraints

- Local commits only on frontend/rebuild; no push, remote changes, main, .github, Android source or reference Go changes.
- Existing /ws request/event names and algorithm behavior remain compatible; no task replay, queue, new schedule or credential storage.
- WS/WSS matches page protocol; 3-second retry, never queue/replay commands, stop reconnecting when authentication has expired.
- Controls disabled until initialization and whenever an action is pending; stop keeps results and waits for the server rather than claiming immediate cancellation.
- Preserve failed/partial/stopped/rate-limited terminal outcomes against generic task_complete and stale same-task snapshots.
- Background discovery is distinct from following: poll once per second only when a running snapshot is not followed; completed follow recovers only what the backend already provides.
- Logs use textContent, are bounded to 200 entries, and do not log raw messages/config/credentials. No external runtime dependency.
- All generated binaries, runtime files and screenshots remain ignored.

## Files And Interfaces

| File | Responsibility |
|---|---|
| `combined_refactor/web/api/socket.js` | `createSocketClient({url,onState,onMessage,onError,WebSocketImpl,fetchImpl,setTimer,clearTimer}) -> {start,send,close}`; envelope validation and reconnect/auth |
| `combined_refactor/web/features/tasks/state.js` | `createTaskState()`, `reduceTask(state,{type,data})`; immutable snapshots/progress/outcome transitions |
| `combined_refactor/web/features/tasks/view.js` | `mountTasks({root,client}) -> {onState,onMessage,destroy}`; global strip, details, actions, bounded log and poll |
| `combined_refactor/web/app.js` | Wire socket/controller and page lifecycle alongside existing theme/navigation |
| `combined_refactor/web/index.html`, `web/styles/layout.css`, `web/styles/controls.css` | Real status and task monitor, responsive controls and confirmation dialog |
| `combined_refactor/types.go`, `frontend_test.go` | Explicit nested-module embedding and resource tests; no broad directory embedding |
| `combined_refactor/web/tests/socket.test.mjs`, `tasks.test.mjs` | Built-in Node tests for boundary/timer/state logic |
| `combined_refactor/web/tests/browser-smoke.mjs`, `tasks-browser.mjs` | Real init smoke plus mock WS lifecycle/control/layout/browser scenarios |
| `docs/frontend-rebuild/{README.md,design.md,plans/*}` | Latest local-commit rule and truthful milestone progress |

## Task 1: Connection Boundary

- [x] Write tests before module: valid/invalid envelopes, WSS URL, open/init, closed send false, 3-second reconnect, no replay, malformed frame recovery, authentication redirect stops retry, failed auth probe still retries, stale callback/close cleanup.

```js
const client = createSocketClient({ url: 'wss://example.test/ws', ...fakeEnvironment });
client.start();
assert.equal(client.send('stop_task'), false);
socket.open();
assert.equal(client.send('get_background_task_status'), true);
socket.close();
await flush();
assert.equal(timer.delay, 3000);
```

- [x] Run `node --test web/tests/socket.test.mjs`; confirm missing module failure.
- [x] Implement the closure with one current socket and retry timer; validate `{type: nonempty string, data}` objects, reject malformed JSON/arrays. Probe authenticated `/workbench` after failed close using same-origin fetch; a login redirect becomes auth-required. Unexpected probe errors are network failures, not authentication claims. Start/close invalidate callbacks and cancel timers.
- [x] Rerun Node tests; extend only the explicit embed patterns for new JS directories and add Go asset tests. Run Go tests/vet.

## Task 2: Global Task Monitor

- [x] Write reducer tests for official scan/detail/speed and NSB scan/reallink/speed, snapshots, missing/follow/enabled, pending stop, terminal precedence, a new task resetting stale outcomes, numeric validation and unrelated events leaving state unchanged.

```js
let state = reduceTask(createTaskState(), { type: 'nsb_csv_complete', data: { status: 'partial', message: 'partial result', rows: [] } });
state = reduceTask(state, { type: 'task_complete', data: null });
assert.equal(state.outcome, 'partial');
```

- [x] Run `node --test web/tests/tasks.test.mjs`; confirm missing module failure.
- [x] Implement reducer and view: init version/speed source/geo warning; known snapshot fields only; progress clamps 0..100 but shows counts separately; specific completion statuses survive generic completion, stop acknowledgements and same-task summaries. New running task or startedAt resets old terminal state. No raw params/config dumps.
- [x] Render global strip and monitor outside result flows. Follow retrieves future events/final available payload and logs their availability without inventing result tables. Confirm stop/background with native dialog; no requests on cancel. Disable action until server response/timeout, preserve stopping while running snapshots arrive; allow status refresh rather than blind retry. Keep polling while stopping without following.
- [x] Add lifecycle wiring: one socket per page, close on pagehide and reconnect on persisted pageshow; theme remains compatible. Update stage label/pending text, but retain all legacy result/start links.

## Task 3: Verification And Local Delivery

- [x] Update browser smoke from pending backend to real init; use actual loopback executable and validate all sections at 320/375/768/1366px with existing theme checks.
- [x] Add mock-browser checks for discovery/poll, follow/no auto-follow, confirmed/cancelled stop/background, injected partial+generic completion, safe malicious text, auth redirect and no retry, reconnect and log bounds. Mock scans; no real credentials or DNS/upload changes.
- [x] Run Go tests/vet, all focused Node tests, headless browser scenarios; visually inspect monitor desktop/mobile screenshots. Independent read-only review must report no remaining blockers.
- [x] Rebuild Windows smoke executable and Linux amd64 test artifact with CGO disabled to release_assets/cfdata-test. Inspect staged diff; docs reflect only local commit delivery and no default switch.
- [x] Commit `Connect workbench backend and global task monitoring` locally, then plan the next official-workflow step. Do not run git push.

## Explicit Limits

No official/NSB start forms or result-table migration yet. A new connection sees only background snapshots, not other connections' foreground tasks; the existing backend start-task mutex remains authoritative. Completed background status after reconnection can be less specific than live completion payloads because current backend snapshots do not retain all failure/partial metadata. Logs are per-page, not durable history. Authentication redirects use the existing login page, which returns to legacy on successful login.

## Verification Record (2026-11-02)

- RED confirmed for absent socket/state modules, then for stale closed callbacks, duplicate background counters, task error precedence, unbounded auth probe and URL credential redaction. Final 22 focused Node tests pass.
- Go tests (`-count=1`) and vet pass. Explicit new asset paths are embedded; test sources remain excluded. Existing backend task handlers/algorithms/auth/CLI unchanged.
- Browser smoke passed against the real Windows executable with backend init and all four sections at 320/375/768/1366px; themes, keyboard/focus, Android bridge mock and blocked storage regressions remain green.
- Mock WebSocket browser scenarios passed discovery/poll/follow, confirmed/cancelled stop/background, task replacement invalidating confirmation, stopping snapshots, partial/error terminal precedence, authoritative background counts, text-only bounded logs, URL credential redaction, reconnect and authentication expiry.
- Desktop/mobile task-monitor screenshots visually inspected. Windows smoke and CGO-disabled Linux amd64 builds passed; `release_assets/cfdata-test` updated, no binaries staged.
- Independent review reproduced reported issues; after fixes it reran 22 tests plus fake-DOM modal and logging checks and reported no remaining blockers.
- Authentication probes time out after 5 seconds and normal retries remain 3 seconds. Probe cancellation and stale callbacks do not reactivate a closed client.
- Generic backend `error` frames carry no request correlation. While a control command is pending, errors are treated as request errors; otherwise a running task becomes failed. This existing protocol ambiguity needs explicit backend event correlation if future concurrent commands require finer classification.
- No real scanning/uploads, APK/hardware, backend restart recovery or live Linux scheduling tested. Synthetic persisted-page events test cleanup/reconnect; actual browser BFCache eligibility remains unverified.
