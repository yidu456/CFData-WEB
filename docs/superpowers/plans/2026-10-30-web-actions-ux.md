# Web Actions UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development sequentially; controller reviews/commits/pushes each task.

**Goal:** Separate result destinations, improve discovered-DC selection/search, and clearly provide encrypted Huawei basic-config persistence.
**Architecture:** Reuse embedded HTML forms/functions and backend DPAPI store. Small view navigation plus local selection filtering; no new libraries or publishing APIs.
**Tech Stack:** Go 1.26.2, embedded HTML/JS/CSS, Node assertions, installed Chrome/Playwright.
**Spec:** docs/superpowers/specs/2026-10-30-web-actions-ux.md

## Global Constraints

- Worktree D:/Program_code/cloudflare_worker/cloudflare-speedtest/.worktrees/cfdata-web-dev, branch dev, base 988e4ab7066e070ab3e6908e23353a5061feca11. Remote yidu456 uses git@yidu456:yidu456/CFData-WEB.git.
- Only controller commits/pushes; no main/beta/.github/reference root code/CLI/scheduler changes. No dependency additions or broad formatting.
- Go $env:TEMP/cfdata-go1.26.2/go/bin/go.exe; scoped gofmt, go test ./..., go vet ./..., all Node suites and installed Chrome smoke. Build CGO=0 Windows amd64 ../release_assets/cfdata-web-preview.exe and Linux ../release_assets/cfdata-test. Never overwrite running executable; controller builds pending name if preview running.
- Test-first red/green for nontrivial behavior, mock all cloud/scan actions. Each step is independently reviewed then committed/pushed.
- Preserve batch authority/provenance, strict per-operation origin/auth, independent statuses, existing account/formatting behavior. UI navigation never publishes.

## Task 1: Destination Chooser and Dedicated Views

**Files:** combined_refactor/index.html, focused Node tests and tests/web_preview_smoke.cjs; README.md only relevant instructions.
**Consumes:** openResultModal(mode), existing result/account/upload forms, openHuaweiPanel/closeHuaweiPanel and current gated publisher functions.
**Produces:** chooser with four actions; dedicated views with stable original IDs and working back/focus paths; no content/secret duplication.

- [x] Trace all result modal open/close/format/preview/quick-file and Huawei entry/focus callers; add failing tests for chooser-first, one visible destination, preserved config, back/Escape, empty-batch DNS config and forbidden mutation.
- [x] Implement minimal in-modal destination navigation; show only selected destination fields/actions/shared output controls; connect DNS view to reusable Huawei panel with return path. Preserve legacy entry and account functions.
- [x] Verify real mocked Chrome desktop/mobile including existing suites updated for intentional navigation; no actual upload/DNS update.
- [x] Controller review, verify test/vet/JS/builds; commit/push feat: separate Web result operation destinations.

## Task 2: Datacenter Search and Accessible Selection

**Files:** combined_refactor/index.html, web_batch_ui_test.js or focused test, tests/web_preview_smoke.cjs; README.md relevant text.
**Consumes:** selectedWebDCs, dcList, toggleWebDatacenter, updateWebBatchSelection and Task1 modal navigation unchanged.
**Produces:** query filter local to discovered options, synchronized 24px table checkboxes with 44px targets and row state.

- [x] Trace selection/re-render/scan reset/background restore callers; write failing code/city/country/case search, hidden selection retention, no-match/reset/running state tests and click/keyboard/mobile regressions.
- [x] Add searchable dropdown and improved scoped checkbox styles/row highlight; no native-control accessibility loss, new selection semantics or scanning logic.
- [x] Verify existing single-center action doesn't toggle checkbox and table/dropdown sync; actual Chrome query/click/focus/viewport assertions.
- [x] Controller review, verify all suites/vet/scopedfmt/builds; commit/push feat: improve datacenter search and selection controls.

## Task 3: Explicit Encrypted Huawei Save and Final Delivery

**Files:** huawei_dns.go/tests, huawei_private_windows.go/other.go if needed, index.html/huawei_ui_test.js, portable smoke and README.md.
**Consumes:** Windows DPAPI basic-config store and Task1 dedicated Huawei view.
**Produces:** explicit encrypted-save action, truthful encryption capability/status/path, no plaintext persistence fallback on unsupported hosts.

- [x] Trace configure/load/status/save/error/masking callers; failing tests for explicit encryption request without cloud writes, full basic-config ciphertext/reload/overwrite/tamper, failure preserves state/file and unsupported persistence fails closed.
- [x] Reuse DPAPI/file; add only necessary capability metadata and clear Save Encrypted versus memory Apply UI. Keep blank preservation, existing Windows ciphertext and sanitized responses. No main-config/export integration or encryption key manager.
- [x] Verify unit/frontend/browser flows plus whole-branch review; no live credentials/provider mutations. Document bound-user portability and non-Windows memory-only limitation.
- [x] Controller review/verify/build and commit/push feat: expose encrypted Huawei configuration saving; preserve original process/EXE.
- [ ] Launch separate loopback preview and verify live HTTP/UI. Both process-launch attempts were blocked by the execution tool policy; no process was started or stopped. Built EXE is ready for manual launch with `-host 127.0.0.1 -port 13336`.
