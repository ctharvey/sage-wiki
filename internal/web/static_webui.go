//go:build webui

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var distFS embed.FS

func init() {
	// Override the static handler when webui is built
	staticHandler = func(projectDir string) http.HandlerFunc {
		// Serve from embedded dist/ directory
		distRoot, _ := fs.Sub(distFS, "dist")
		fileServer := http.FileServer(http.FS(distRoot))

		return func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// SPA fallback: /wiki/* routes serve index.html
			if strings.HasPrefix(path, "/wiki/") || path == "/" {
				data, err := distFS.ReadFile("dist/index.html")
				if err != nil {
					http.Error(w, "index.html not found", 500)
					return
				}
				w.Header().Set("Content-Type", "text/html")
				w.Write(data)
				return
			}

			// Serve static assets
			fileServer.ServeHTTP(w, r)
		}
	}
}
