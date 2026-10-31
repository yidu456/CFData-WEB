# Portable Private Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Controller owns verification and commits.

**Goal:** Save Huawei credentials and Web preferences/accounts as nonplaintext files that the application can decrypt on every supported platform.

**Architecture:** Use Go's standard AES-256-GCM with a fixed application key, random nonce per write, and a versioned authenticated envelope. Read existing Windows DPAPI files with the original current-user decoder; convert only after an explicit successful save. Keep atomic replacement, strict authorization, validation, state CAS and publication guards unchanged.

**Tech Stack:** Go 1.26.2 standard crypto libraries, existing platform DPAPI reader and embedded HTML/Node tests.

**Spec:** User approval on 2026-10-31: reversible application encryption for all private storage, including Huawei AK/SK. This amendment overrides Windows-only encryption requirements in the earlier executable-local storage design; all unrelated requirements remain binding.

## Global Constraints

- Work in the existing `dev` worktree at `3554c58`; preserve staged README verification additions and Web-state plaintext-rejection regression work. Do not operate on main, protected files, production EXE, real clouds or schedulers.
- A fixed key is extractable from the application; this prevents casual plaintext inspection, not reverse engineering. Explain this accurately. No new dependency, password prompt, key file, or configurable encryption policy.
- Common decrypt rejects plaintext, malformed/unknown envelopes and failed authentication. Never retry a recognized but damaged portable envelope as DPAPI or JSON.
- Windows DPAPI remains a legacy read path. Other platforms cannot decrypt old DPAPI bytes; show an error and preserve them. Old non-Windows Huawei JSON can retain its existing validated compatibility read, labelled legacy plaintext; Web state must never accept plaintext.
- Reads never rewrite files. Explicit save atomically replaces only after encryption succeeds; locked/failed writes preserve good memory, revision and files. Explicit old-location migration and separate verified cleanup remain unchanged.
- Preserve browser account inner ciphertext/unlock behavior, same-origin authorization, direct replies, dirty/load guards, browser downloads and no restored publishing authority.
- Runtime: `%TEMP%/cfdata-handover-go1.26.2/go/bin/go.exe`; Node from bundled runtime; installed Chrome only. Build Windows pending EXE and `release_assets/cfdata-test`; do not replace running preview PID 24580.

## Task 1: Portable Cipher, Legacy Reads and Storage Tests

**Files:** create `combined_refactor/private_cipher.go`, `private_cipher_test.go`, Windows compatibility tests; modify platform private helpers, `huawei_dns.go`, `huawei_migration.go`, `web_state.go` only where required, and focused existing storage/auth/migration tests.

**Interfaces:** Retain `protectHuaweiPrivate([]byte) ([]byte,error)`, `unprotectHuaweiPrivate([]byte) ([]byte,error)`, and `huaweiPrivateEncryptionSupported`. Common protection writes the portable envelope; platform legacy decryption is read-only.

- [x] Add tests first for nonplaintext round trip, randomized writes, empty payload, truncation/bitflip/unknown version rejection and plaintext Web-state rejection. Use an assertion such as `if bytes.Contains(ciphertext, secret) || bytes.Equal(first, second) { t.Fatal("plaintext or reused nonce") }`.
- [x] Implement a single versioned envelope using AES-GCM and `crypto/rand`. Authenticate the envelope prefix as additional data. Use a stable public application key; do not describe it as a protected secret.
- [x] Preserve old DPAPI reads on Windows. Test genuine `huaweiDPAPI(plain,true)` fixtures for both full Huawei configuration and Web state; load leaves bytes unchanged, failed save preserves them, successful explicit save writes the portable envelope and reloads.
- [x] Keep legacy Huawei plaintext reads separate from generic decrypt, with schema validation and truthful `legacy-plaintext` status. No plaintext Web-state fallback on any platform.
- [x] Enable encrypted saving/migration on all platforms and update only obsolete Windows-only tests; retain actual Windows file-lock guards. Run shared persistence/CAS/authorization/migration tests on Windows and cross-compile Linux tests without claiming native Linux execution.
- [x] Report exact RED/GREEN results and changed files to this plan's ignored workspace; no worker commit/push or formatter over unrelated files.

## Task 2: UI, Documentation and Integrated Verification

**Files:** embedded `index.html`, affected Node/Chrome fixtures, README, prior design/plan amendment notes only.

- [x] Replace Windows-only/DPAPI-only saving text with portable application encryption and its reverse-engineering limit; describe old DPAPI read compatibility and manual conversion through Save accurately.
- [x] Keep capability/permission/path failures truthful. Preserve input IDs, status isolation, manual migration/cleanup, reconnect/unknown-cloud-outcome warnings and browser account crypto behavior.
- [x] Add actual-function assertions for portable-save copy and legacy DPAPI caveats; retain negative capability mock scenarios as backend failures, not platform-wide assumptions.
- [x] Controller independently runs uncached Go tests, vet, scoped read-only formatting, six Node suites, both offline Chrome smokes, Windows/Linux builds and Linux test cross-compilation. Review the complete amendment and fix actionable findings before integration.
- [x] Finish the earlier storage/UI plan under this approved encryption amendment, recording any untested Linux/race/Android/cloud/preview runtime gaps. Leave queued DNS filtering/IP-only/combobox requests for their separate follow-up plan.
