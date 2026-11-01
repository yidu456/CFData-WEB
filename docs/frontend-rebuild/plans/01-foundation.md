# Workbench Foundation Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking. Review is independent before the milestone commit.

**Goal:** Serve the approved modular workbench shell from the Go binary without removing any existing UI capabilities.

**Architecture:** A new embedded `web/` tree serves `/workbench` and authenticated `/assets/` resources. Native modules own navigation and theme; placeholder areas identify migration state and link the still-complete legacy default instead of exposing fake controls.

**Tech Stack:** Go 1.26.2, standard net/http/embed, native browser ES modules, Node built-in test runner, existing Go dependencies only.

**Spec:** `docs/frontend-rebuild/design.md`; existing function tables in `docs/frontend-rebuild/modules/`.

## Global Constraints

- Only `frontend/rebuild`; no changes to `.github/`, root reference Go sources, independent Worker or main.
- Old `/`, `/ws`, `/auth/login`, `/auth/logout`, `/favicon.png` keep behavior. No hidden switch to incomplete new UI.
- No external runtime CSS/font/icon/API dependency for the new shell.
- JS/CSS/source files by responsibility; HTML contains no embedded business JS/styles.
- Preview data, fake connected states and real credential inputs are not production shell content.
- Theme migrates compatible `cfdata-theme` values; storage failure does not prevent rendering.
- Authentication wraps all new routes; resource requests cannot expose directories, tests, Go source or arbitrary files.

## Files And Responsibilities

| File | Responsibility |
|---|---|
| `combined_refactor/frontend.go` | Register and serve fixed new entry/assets; method/path/MIME/cache rules |
| `combined_refactor/frontend_test.go` | Embedded entry/assets, auth, HEAD, method and path tests |
| `combined_refactor/types.go` | Extend existing embed declaration to include web assets |
| `combined_refactor/main.go` | Register new routes on existing mux |
| `combined_refactor/web/index.html` | Semantic application shell and stylesheet/module entry |
| `combined_refactor/web/app.js` | Wire existing shell nodes, navigation and theme only |
| `combined_refactor/web/navigation.js` | Closed set of valid section names; selected view lookup |
| `combined_refactor/web/platform/theme.js` | Stored preference, media/Android theme and status bar adaptation |
| `combined_refactor/web/styles/tokens.css` | Approved light/dark product variables |
| `combined_refactor/web/styles/layout.css` | Responsive shell/navigation/surface geometry |
| `combined_refactor/web/styles/controls.css` | Accessible native controls/focus/theme selector |
| `combined_refactor/web/tests/shell.test.mjs` | Navigation/theme behavior and HTML module boundaries |
| `combined_refactor/web/tests/browser-smoke.mjs` | Real embedded browser smoke checks using an already-installed Playwright |

## Task 1: Embedded Entry And Asset Routing

**Consumes:** existing `staticFiles embed.FS`, `requireAuth(http.HandlerFunc) http.HandlerFunc`.

**Produces:** `registerWorkbenchRoutes(mux *http.ServeMux)`, `handleWorkbench(w http.ResponseWriter, r *http.Request)`, `handleWorkbenchAsset(w http.ResponseWriter, r *http.Request)`; `/workbench`, `/assets/app.js`, `/assets/styles/*.css`, `/assets/platform/theme.js`.

- [x] Write `frontend_test.go` first. Assert the root preserves legacy and new assets are missing before registration is implemented:

```go
func TestWorkbenchRoutes(t *testing.T) {
    mux := http.NewServeMux()
    registerWorkbenchRoutes(mux)
    for _, path := range []string{"/workbench", "/assets/app.js", "/assets/styles/tokens.css"} {
        req := httptest.NewRequest(http.MethodGet, path, nil)
        resp := httptest.NewRecorder()
        mux.ServeHTTP(resp, req)
        if resp.Code != http.StatusOK { t.Fatalf("%s: status %d", path, resp.Code) }
    }
}
```

- [x] Run `go test ./...`; initial failure must report missing `registerWorkbenchRoutes`, not network or unrelated failures.
- [x] Add fixed resource handlers: accept GET/HEAD, otherwise 405/Allow. Asset relative path passes fs.ValidPath, rejects tests/hidden paths, accepts only supported .js/.css files, returns 404 otherwise. Read embedded bytes, explicitly set MIME, no-cache and nosniff; `http.ServeContent` handles GET/HEAD byte bodies.
- [x] Extend embed and register on `http.DefaultServeMux` without touching existing route handlers.
- [x] Add table tests: entry title/module tags; JS/CSS correct MIME; empty HEAD body; 405; path traversal/test/go-source/directory/missing 404; auth-off 200 vs auth-on redirect; no disk dependency.
- [x] Run `go test ./...` and `go vet ./...`; read exit codes.

## Task 2: Approved Modular Shell

**Consumes:** authenticated resources from Task 1; compatible `cfdata-theme` key; optional existing Android bridge.

**Produces:** `normalizeSection(value) -> string`, `readThemePreference(storage) -> 'auto'|'light'|'dark'`, `isDarkTheme(preference, systemDark, androidDark) -> boolean`, `applyThemePreference(preference, environment)`. `app.js` attaches native navigation buttons and appearance selector, without WebSocket or task code.

- [x] Add built-in Node tests, adapting browser-source imports via data URLs so no package.json/global module mode is needed:

```js
const load = async file => import('data:text/javascript;base64,' + Buffer.from(await readFile(file)).toString('base64'));
test('unknown navigation cannot select an arbitrary area', async () => {
  const { normalizeSection } = await load(new URL('../navigation.js', import.meta.url));
  assert.equal(normalizeSection('targets'), 'targets');
  assert.equal(normalizeSection('../secrets'), 'workbench');
});
test('automatic theme accepts native dark appearance', async () => {
  const { isDarkTheme } = await load(new URL('../platform/theme.js', import.meta.url));
  assert.equal(isDarkTheme('auto', false, true), true);
  assert.equal(isDarkTheme('light', true, true), false);
});
```

- [x] Run `node --test web/tests/*.test.mjs` and confirm missing-file failures before production modules exist.
- [x] Implement semantic shell: brand, development-stage label, old complete-version link, four sections (workbench/task monitoring/publishing/settings), mode buttons for future workspaces, theme selector. New page says backend integration is pending, not connected.
- [x] Implement closed navigation names and hide/show panels without rebuilding unrelated DOM. Native buttons, aria-current and meaningful headings.
- [x] Implement preference validation, safe storage read/write, system change updates, guarded Android isDarkMode/setDarkModeStatusBar. No unrelated config migration.
- [x] Create local product CSS files with approved blue/slate palette, fine borders, quiet rounded surfaces; all controls visible on mobile, min 44px touch targets, visible focus and reduced motion support.
- [x] Run Node tests, including malformed/unavailable storage, explicit dark/light overriding automatic, valid/invalid sections and no inline/external resources in entry.

## Task 3: Milestone Verification And Local Commit

- [x] Go tests/vet and Node tests pass; `gofmt` changed Go only.
- [x] Browser serves the real `/workbench` resource tree; test navigation/theme, no runtime exceptions, 320px/375px/desktop and no horizontal overflow. Existing default still displays legacy interface.
- [x] Build Windows local smoke executable in ignored build directory and Linux amd64 test artifact in `release_assets/cfdata-test` with CGO disabled; do not stage binaries.
- [x] Independent review checks auth and path exposure, embed packaging, existing routes, storage failure, accessible controls and no simulated success states.
- [x] Record actual verification and known limits in milestone status; stage only docs, new files and modified Go embedding/registration.
Local delivery: verified foundation committed as `c9e5a57`. Earlier push attempts were denied; latest user instruction explicitly supersedes the publication requirement: local commits only, no push, proceed to the next step.

## Completion Boundary

Foundation is complete only when the new entry loads from the binary and all above checks pass. It does not claim migration of scanning/tasks/results/publishing; those remain fully available through `/` until their separate milestones are complete.

## Verification Record (2026-11-01)

- Baseline Go tests/vet passed before changes; there were no checked-in tests.
- RED observed: new Go routing functions were undefined and new browser modules were missing. GREEN: Go tests/vet and seven Node tests passed.
- Independent review found a shared-theme BFCache restoration issue. Browser and Node regressions failed before the fix and passed after it; repeat review found no remaining blockers.
- The Windows executable served embedded assets from an ignored runtime directory containing no `web/` files. Browser smoke passed navigation/mode state, keyboard/focus, system/explicit themes, blocked localStorage, mock Android status bar, synthetic persisted pageshow, local-only resources and unchanged legacy entry content.
- Layout checks passed at 320, 375, 768 and 1366 pixels for all four sections; desktop/mobile/dark screenshots were visually inspected. Screenshot output stays in ignored `build/workbench-smoke/`.
- Windows smoke build and stripped CGO-disabled Linux amd64 build passed; the latter is `release_assets/cfdata-test`. Binaries/toolchains are ignored and not committed.
- Documentation check passed: 19 Markdown files, 62 local links, 112 unique feature entries preserved. Source and test files remain separate; tests are not embedded.
- Not verified: Android hardware/APK, actual browser BFCache eligibility/navigation, live scanning, uploads, CLI scheduling. This milestone intentionally does not connect task/publishing code.

### Reproduce Locally

From `combined_refactor/`, use Go 1.26.2 (or the compatible toolchain required by `go.mod`):

```sh
go test ./...
go vet ./...
node --test web/tests/shell.test.mjs
go run . -host 127.0.0.1 -port 13336
```

In another terminal, point `PLAYWRIGHT_MODULE` to an already-installed Playwright module if it is not normally resolvable; `BROWSER_CHANNEL` defaults to `msedge`. No production dependency is added:

```sh
node web/tests/browser-smoke.mjs http://127.0.0.1:13336 ../build/workbench-smoke
```
