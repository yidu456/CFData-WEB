package main

import (
	"bytes"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

func registerWorkbenchRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/workbench", requireAuth(handleWorkbench))
	mux.HandleFunc("/workbench/", requireAuth(handleWorkbench))
	mux.HandleFunc("/assets", requireAuth(handleWorkbenchAsset))
	mux.HandleFunc("/assets/", requireAuth(handleWorkbenchAsset))
}

func handleWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/workbench" {
		http.NotFound(w, r)
		return
	}
	serveWorkbenchFile(w, r, "web/index.html", "text/html; charset=utf-8")
}

func handleWorkbenchAsset(w http.ResponseWriter, r *http.Request) {
	name, found := strings.CutPrefix(r.URL.Path, "/assets/")
	if !found || !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") || part == "tests" {
			http.NotFound(w, r)
			return
		}
	}
	mime := ""
	switch {
	case strings.HasSuffix(name, ".js"):
		mime = "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		mime = "text/css; charset=utf-8"
	default:
		http.NotFound(w, r)
		return
	}
	serveWorkbenchFile(w, r, "web/"+name, mime)
}

func serveWorkbenchFile(w http.ResponseWriter, r *http.Request, name, mime string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := staticFiles.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
