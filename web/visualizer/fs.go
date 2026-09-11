package visualizer

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// FS is the static fleet-visualizer page (solo-track demo UI).
//
//go:embed index.html app.js
var FS embed.FS

// Handler serves the visualizer under any prefix; empty path is index.html.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" || strings.HasSuffix(name, "/") {
			name = "index.html"
		}
		b, err := fs.ReadFile(FS, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(name, ".js") {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		} else if strings.HasSuffix(name, ".html") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	})
}
