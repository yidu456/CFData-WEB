# Publishing Target Management Implementation Plan

> Execute inline using TDD, independent review, full verification, test builds and local commit; then manual publishing. User approved staged development, no push.

**Goal:** Manage saved GitHub/edgetunnel targets with stable IDs and preserve legacy account credentials without collecting credentials inside output configuration.

**Architecture:** Browser-owned metadata in one versioned key; optional credentials use native WebCrypto AES-GCM, bound to stable ID rather than editable name. Session-only secrets are keyed by ID and never serialized. Non-destructive importer keeps original legacy keys, imports each existing account once and carries legacy cipher context until user saves/re-encrypts.

**Tech Stack:** Native embedded JS/CSS, WebCrypto, Node test runner, installed browser harness, Go1.26.2.

**Spec:** `modules/publishing.md`, `design.md`; P03/P04/P08 and approved centralized target management. P01/P02/P05/P06/P07/P09/P10 execution is next independently reviewed step.

## Constraints

- frontend/rebuild only; no push/remotes/Worker/.github/reference code changes. No real upload/API/credentials in tests.
- Existing GitHub cfdata-github-accounts and edgetunnel cfdata-edgetunnel-accounts remain intact even on malformed JSON, encryption unavailable, storage quota/blocked errors or partial import.
- IDs survive rename/reorder and deletion references fail closed. Legacy import identity is provider plus original account ordinal/name/destination; repeat same snapshot cannot duplicate targets.
- New targets only GitHub/edgetunnel, platform-specific validated form. Do not advertise unimplemented DNS targets. GitHub repo/branch/path/message/format and edgetunnel host/append-or-overwrite remain distinct.
- Optional remember secret; no-WebCrypto means session-only with clear warning, never plaintext persistence. Stored ciphers do not protect against same-origin/XSS. Blank secret on metadata edit preserves existing credential unless explicitly forgotten.
- Browser storage scope and legacy one-time migration must be visible. Session-only targets/credentials remain usable when browser storage blocked but no false saved-to-disk message.

## Task 1: Targets Model And Secrets

Files: `web/features/publishing/targets.js`, `secrets.js`; test `web/tests/targets.test.mjs`.

- [x] RED tests validate provider-specific destination and reject HTTP credentials/query/hash/invalid mode/unimplemented provider; create/update retains ID; same names allowed, delete precise ID.
- [x] Implement `validateTarget(values,id)` returning normalized metadata only; `loadTargets(storage,createID)` returns `{targets,warning}` with non-destructive once-only legacy import; `saveTargets(storage,targets)` returns persisted boolean and readable warning.
- [x] RED tests legacy arrays/plain/cipher metadata, malformed/non-array/blocked/quota handling, duplicate import avoidance, unrelated storage untouched, no raw secret in serialized versioned key.
- [x] Implement `createTargetStore({storage,crypto,origin,createID})` with `list()`, `save(values)`, `remove(id)`, `credential(id)` and session map. `save` validates before replacing state; remembers optional encrypted secret; rename stable-ID crypto binding, explicit forget; imported legacy plaintext remains session only while original key stays intact.
- [x] RED crypto tests real WebCrypto roundtrip, ciphertext/context tamper failure, rename, legacy Github dot-separated/edgetunnel colon-separated cipher decryption with original origin/account key/salt/PBKDF2120000; unsupported/rejected crypto never writes plaintext or discards old cipher.
- [x] GREEN focused tests and all Node tests.

## Task 2: Target Workspace

Files: `web/features/publishing/targets-view.js`, `web/app.js`, `web/index.html`, `web/styles/controls.css`, Go embed/frontend route tests; browser `web/tests/targets-browser.mjs`.

- [x] RED browser existing legacy2platform targets imported once, no credentials in list, create/edit/rename/delete stableID, remember/session/forget flows, native validation, blocked storage fallback, stale async decrypt/save cannot affect another selection, 320/375/768/1366 widths and keyboard focus.
- [x] Mount target list and separate platform forms; explicit new/select/save/cancel/delete confirmation, masked credential input, stored/session/unavailable credential status, optional remember/forget. Do not preload decrypted secret on selection; user enters replacement only. Persisted metadata excludes secret input.
- [x] Use async operation guards bound to ID/revision; rerenders restore meaningful focus; no credential logs/previews/export or upload call from management. Clearly keep upload execution in legacy until next slice.
- [x] Embed new modules; Go routes GET/HEAD/MIME/auth and tests not embedded. Full browser regressions and visual inspect.

## Task 3: Verification And Local Commit

- [x] Read-only independent review; reproduce/fix findings with tests and re-review.
- [x] All Node/browser suites, Go uncached tests/vet and Windows/LinuxCGO0 builds; record actual limits and docs progress.
- [ ] Stage related files only, commit `Add saved publishing target management`; immediately begin manual upload plan. Never push.

## Verification Record (2026-11-02)

-68 Node tests pass, including8 focused target tests with real WebCrypto roundtrip and legacy cipher fixtures. RED confirmed absent modules, then rehydrate-on-reload credential loss and unsupportedcrypto replacement dropping cipher before fixes. Legacy original keys remain intact, imports/tombstones stable, session secrets not copied into new persistent key.
- Seven browser suites pass including target CRUD/rename/stableID/remember/forget/reload, original old token unchanged, storage blocked and window.crypto absent fallback. No upload commands sent by management. Responsive320/375/768/1366; screenshot inspected.
- Review found/fixed old cipher deletion when replacement encryption fails and absent Crypto ID fallback. Legacy plain credential forget prevents later rehydrate; encryption pending cannot resurrect deleted targets. Independent re-review reports no blockers and all8 target tests pass.
- Go uncached tests/vet, Windows build and Linux amd64CGO0 release_assets/cfdata-test build pass; embedded platform/output and publishing resources covered by Go routes. Legacy secret salt copied byte-for-byte without logging values; test fixtures use fake credentials only.
- Still browser-local, not CLI/cross-device storage. Existing legacy keys intentionally preserved, including any legacy plaintext already present; forgetting/deleting a new target does not erase old key. No real service credential/upload, Android hardware or next manual/automatic publishing validation.
