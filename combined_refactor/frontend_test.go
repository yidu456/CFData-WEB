package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func workbenchTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	user, password := webUser, webPassword
	webUser, webPassword = "", ""
	t.Cleanup(func() { webUser, webPassword = user, password })
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("legacy entry"))
	})
	registerWorkbenchRoutes(mux)
	return mux
}

func TestWorkbenchRoutes(t *testing.T) {
	mux := workbenchTestMux(t)
	for _, tc := range []struct {
		path, mime, content string
	}{
		{"/workbench", "text/html; charset=utf-8", "<title>CFData"},
		{"/assets/app.js", "text/javascript; charset=utf-8", "./navigation.js"},
		{"/assets/navigation.js", "text/javascript; charset=utf-8", "normalizeSection"},
		{"/assets/api/socket.js", "text/javascript; charset=utf-8", "createSocketClient"},
		{"/assets/features/tasks/state.js", "text/javascript; charset=utf-8", "reduceTask"},
		{"/assets/features/official/model.js", "text/javascript; charset=utf-8", "buildScanRequest"},
		{"/assets/features/official/view.js", "text/javascript; charset=utf-8", "mountOfficial"},
		{"/assets/features/measurement/model.js", "text/javascript; charset=utf-8", "buildSpeedConfig"},
		{"/assets/features/nonstandard/model.js", "text/javascript; charset=utf-8", "buildNSBRequest"},
		{"/assets/features/nonstandard/view.js", "text/javascript; charset=utf-8", "mountNSB"},
		{"/assets/features/results/browser.js", "text/javascript; charset=utf-8", "browseResultRows"},
		{"/assets/features/results/controls.js", "text/javascript; charset=utf-8", "mountResultBrowser"},
		{"/assets/platform/theme.js", "text/javascript; charset=utf-8", "cfdata-theme"},
		{"/assets/styles/tokens.css", "text/css; charset=utf-8", "--cf-brand"},
		{"/assets/styles/layout.css", "text/css; charset=utf-8", "@media"},
		{"/assets/styles/controls.css", "text/css; charset=utf-8", ":focus-visible"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				resp := httptest.NewRecorder()
				mux.ServeHTTP(resp, httptest.NewRequest(method, tc.path, nil))
				if resp.Code != http.StatusOK || resp.Header().Get("Content-Type") != tc.mime {
					t.Fatalf("%s: status=%d headers=%v", method, resp.Code, resp.Header())
				}
				if resp.Header().Get("Cache-Control") != "no-cache" || resp.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Fatalf("missing cache or MIME protection: %v", resp.Header())
				}
				if method == http.MethodHead && resp.Body.Len() != 0 {
					t.Fatal("HEAD returned a body")
				}
				if method == http.MethodGet && !strings.Contains(resp.Body.String(), tc.content) {
					t.Fatalf("missing embedded content %q", tc.content)
				}
			}
		})
	}
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))
	if resp.Body.String() != "legacy entry" {
		t.Fatal("new routes replaced the legacy default")
	}
}

func TestWorkbenchRejectsUnexpectedResources(t *testing.T) {
	mux := workbenchTestMux(t)
	for _, path := range []string{
		"/workbench/", "/workbench/extra", "/assets", "/assets/", "/assets/index.html",
		"/assets/tests/shell.test.mjs", "/assets/main.go", "/assets/.env", "/assets/.hidden/file.js",
		"/assets/styles/", "/assets/missing.js", "/assets/platform/missing.css",
	} {
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, resp.Code)
		}
	}
	// ServeMux canonicalizes traversal paths; verify the handler also rejects them directly.
	for _, path := range []string{"/assets/../app.js", "/assets/%2e%2e/main.go", "/assets//app.js", "/assets/styles/../app.js"} {
		resp := httptest.NewRecorder()
		handleWorkbenchAsset(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, resp.Code)
		}
	}
	if _, err := staticFiles.ReadFile("web/tests/shell.test.mjs"); err == nil {
		t.Fatal("test sources must not be embedded in the binary")
	}
}

func TestWorkbenchMethods(t *testing.T) {
	mux := workbenchTestMux(t)
	for _, path := range []string{"/workbench", "/assets/app.js"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions} {
			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, httptest.NewRequest(method, path, nil))
			if resp.Code != http.StatusMethodNotAllowed || resp.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("%s %s: status=%d headers=%v", method, path, resp.Code, resp.Header())
			}
		}
	}
}

func TestWorkbenchAuthentication(t *testing.T) {
	mux := workbenchTestMux(t)
	webUser, webPassword = "test-user", "test-password"
	token, _, err := createAuthSession(webUser)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deleteAuthSession(token) })
	for _, path := range []string{"/workbench", "/workbench/extra", "/assets", "/assets/app.js", "/assets/styles/tokens.css", "/assets/platform/theme.js"} {
		for _, cookie := range []string{"", "invalid", token} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if cookie != "" {
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie})
			}
			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, req)
			if cookie != token {
				if resp.Code != http.StatusFound || resp.Header().Get("Location") != "/auth/login" {
					t.Errorf("unauthenticated %s: status=%d headers=%v", path, resp.Code, resp.Header())
				}
			} else if strings.HasSuffix(path, "/extra") || path == "/assets" {
				if resp.Code != http.StatusNotFound {
					t.Errorf("authenticated invalid %s: status=%d", path, resp.Code)
				}
			} else if resp.Code != http.StatusOK {
				t.Errorf("authenticated %s: status=%d", path, resp.Code)
			}
		}
	}
}
