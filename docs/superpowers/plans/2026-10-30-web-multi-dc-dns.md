# Web Multi-Datacenter DNS Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development sequentially. Complete each task's tests/review before its one commit and push to dev.

**Goal:** Deliver a complete Web preview with multi-DC testing, final total top-10 selection, GitHub publishing, and fully configurable Huawei DNS updates. Defer CLI and Windows scheduling until Web approval.

**Architecture:** One backend result selector; one backend-owned batch task; the official Huawei SDK with isolated private credential persistence; extend the existing embedded UI and upload mechanisms. Keep single-DC and unrelated flows intact.

**Tech Stack:** Go 1.26.2, existing HTML/JavaScript/WebSocket code, Go testing/httptest, official Huawei Go SDK, browser automation.

**Spec:** docs/superpowers/specs/2026-10-30-web-multi-dc-dns.md

## Global Constraints

- Workspace: D:/Program_code/cloudflare_worker/cloudflare-speedtest/.worktrees/cfdata-web-dev, branch dev, remote yidu456 (git@yidu456:yidu456/CFData-WEB.git).
- Only the controller commits/pushes after task review. One commit per completed task; no force-push, no main/beta modifications, no changes under .github/ or reference root Go sources.
- Go executable: $env:TEMP/cfdata-go1.26.2/go/bin/go.exe. Prefix PATH with its bin for commands. Run go test ./..., go vet ./..., gofmt checks and CGO_ENABLED=0 Windows/Linux builds from combined_refactor. Linux output ../release_assets/cfdata-test; Windows output ../release_assets/cfdata-web-preview.exe. Binaries are ignored.
- All new behavior requires failing tests before implementation. Use mocks; never scan targets or publish to real cloud services during QA.
- Keep implementation scoped to Web. Shared helpers may support later CLI work, but do not change CLI behavior or scheduling now.
- Follow the spec's secret handling, origin/auth checks, IP validation, completed-task ownership, cancellation, empty-results, and independent publishing requirements.

## Task 1: Shared Qualified Selection Policy

**Files:** Create combined_refactor/selection.go and selection_test.go. Modify types.go only if needed to preserve exact numeric measured speed alongside its display string.

**Interface:** officialSelectionPolicy{Datacenters []string, MaxLatencyMS int, MinSpeedMB float64, Limit int}; selectOfficialResults(results []TestResult, policy officialSelectionPolicy) ([]TestResult, error). Normalize/validate DC codes; expose a reusable normalization helper if needed. Do not infer task completion from speed strings.

- [x] Write failing table-driven tests: FRA/SJC union, HKG exclusion, latency/speed boundaries, invalid/NaN/infinite/negative values, invalid IPs/DCs/limits, failed/unmeasured speed, duplicates, total cap=10, deterministic ties, fewer/empty results, and no input mutation.
- [x] Implement a pure selector with numeric qualification, speed-desc/latency-asc/IP tie sorting, IP normalization/deduplication, and total cap after filtering. Do not touch scanning, server, frontend, or CLI.
- [x] Run tests/vet/builds and report exact API/data-field decisions for Task 2.
- [x] Controller reviews, records evidence, commits and pushes feat: add shared Web result selection.

## Task 2: Backend-Owned Web Batch and Multi-Selection UI

**Files:** Modify combined_refactor/index.html, protocol.go, server.go, session.go, tasks_official.go and tasks_speed.go as required; create web_batch.go/web_batch_test.go and focused JS self-checks if that keeps orchestration isolated.

**Consumes:** Task 1 officialSelectionPolicy/selectOfficialResults.
**Produces:** Completed backend-owned selected results and policy snapshot in appSession; one batch request accepts selected datacenters, MaxLatencyMS/MinSpeedMB/Limit, test port and speed URL. It measures every latency-qualified candidate before selection and exposes selected results/summary to the UI.

- [ ] Write failing tests for request/policy validation, multi-DC candidate filtering, no selected candidates, complete speed measurements beyond 10, metadata retention, cancellation/failure rejecting publishable state, and final totals.
- [ ] Add datacenter checkboxes and a batch action without removing existing row actions. Reuse existing threshold controls where possible, but label final limit separately from existing stop-after-qualified count. Default final total limit 10.
- [ ] Implement backend-owned detailed test -> complete speed test -> selector -> final-results events, preserving legacy single-DC modes. Do not rely on a browser callback for subsequent batch stages. UI displays selection state, progress, ranking and qualified count, supports keyboard/mobile controls, and saves only nonsecret settings.
- [ ] Add a reusable accessor for completed selected backend results for Task 3. Do not accept arbitrary browser rows as DNS publishing authority. Do not clear verified state incorrectly on simple UI rendering.
- [ ] Verify Go and JavaScript tests, vet, builds and legacy behavior. Controller reviews, commits and pushes feat: add Web multi-datacenter top-results testing.

## Task 3: Complete Huawei DNS Configuration and Publishing UI

**Files:** Create combined_refactor/huawei_dns.go, huawei_dns_test.go and huawei_web.go/huawei_web_test.go if needed; modify go.mod/go.sum, main.go/server.go/protocol.go/session.go and index.html for real Web configuration and publication only.

**Consumes:** Task 2 completed results accessor and policy snapshot, existing Web authentication and GitHub upload functions.

- [ ] Inspect official SDK DNS authentication, region/endpoint and record identity before coding. Add only the official dependency required for DNS; do not hand-roll request signing.
- [ ] Write failing mocked tests for credential preservation/redaction, private save/reload, same-origin/local-or-authenticated access, public zones/recordset discovery, pure A/AAAA values, identity and TTL preservation, unchanged/empty updates, bounded failures, cancellation/incomplete-state denial and independent GitHub/DNS status.
- [ ] Implement functional panel/API: AK/SK inputs, service configuration, optional private backend save, connection test (read-only), zone and record dropdowns, target details, planned old/new IP preview, confirmed update button, and opt-in update after a completed Web batch. Blank credentials preserve saved values; responses never return SK/full AK. No browser secret storage/export/logging.
- [ ] Privileged mutation/discovery endpoints require same-origin validation and loopback/local-host access unless existing Web auth is enabled; reuse auth wrappers and do not weaken them. Private saved credentials are ignored by Git and permission-restricted; saving is explicit.
- [ ] Update existing selected recordsets only; verify zone/name/type/line, preserve properties, family-filter, skip unchanged, and keep old records for empty/failed/cancelled tasks. No create/delete.
- [ ] Wire final selected results into existing GitHub UI/manual upload and opt-in completion publication. Attempts/statuses remain independent; no secrets or untrusted arbitrary IPs leak into DNS payloads.
- [ ] Verify with mocks only, all Go/JS checks/vet/builds. Controller reviews, commits and pushes feat: add complete Web Huawei DNS publishing.

## Task 4: Web Preview QA and Delivery

**Files:** Add focused browser self-checks/fixtures under tests/ if needed; update README.md and this plan/spec only as required. No CLI/scheduler implementation.

- [ ] Exercise mocked FRA/SJC selection, final top-10, fewer/zero results, DNS configuration/discovery/preview/errors, independent publishing statuses, cancel/reconnect, legacy row tests, and desktop/mobile layout. Inspect screenshots.
- [ ] Run all Go tests (race where available), vet, formatting/JS checks, Windows/Linux builds, and final connected-path code review. Fix defects with regression tests.
- [ ] Document credentials, DNS preview/confirmation, speed/latency/count semantics, local-only access, untouched production EXE and deferred CLI work.
- [ ] Controller commits/pushes test: verify and document Web DNS preview.
- [ ] Start the separate preview build only on loopback, show the URL/build path and commits. Do not configure actual cloud targets or run real updates without the user's input.

## Execution Status

- Step 0: SSH Host yidu456 authenticated successfully. Native managed worktree rejected nested sourceRoot; used an isolated Git worktree at the explicit path above, based on target beta. Original main checkout is untouched.
- Go 1.26.2 downloaded from go.dev and SHA256 verified against official metadata. Baseline passed: go test ./..., go vet ./..., Windows and Linux builds.
- Initial CLI-first plan is superseded by this Web-first plan following the user's latest instruction. Credentials and real DNS configuration must be fully editable/testable in the Web version, not mock buttons.
