package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui/dist
var dashboardFS embed.FS

func getEmbeddedHandler() http.Handler {
	fsys, err := fs.Sub(dashboardFS, "ui/dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Dashboard UI not built. Run: cd ui && bun run build"))
		})
	}

	fileServer := http.FileServer(http.FS(fsys))

	// SPA fallback: serve index.html for non-file paths
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if file exists in embedded FS
		f, err := fsys.Open(path[1:]) // strip leading /
		if err != nil {
			// File not found — serve index.html for SPA routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}
