package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// serveWebApp serves one built Vite app under a URL prefix.
// It searches for the app's dist directory in several places:
// ./web/apps/<app>/dist (repo root), ../web/apps/<app>/dist
// (when running from api/), and <exe-dir>/web/apps/<app>/dist.
// If no dist is found, it serves a helpful page explaining
// how to build the app, instead of a bare 404.
func serveWebApp(mux *http.ServeMux, prefix, app string) {
	mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		dir := findWebDist(app)
		if dir == "" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("<h1>Frontend not built yet</h1><p>Run <code>npm install && npm run build</code> in <code>web/apps/" + app + "</code>, then restart the API server.</p>"))
			return
		}
		// Strip the prefix, serve index.html for SPA paths.
		rest := strings.TrimPrefix(r.URL.Path, prefix)
		if rest == "" || rest == "/" {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		path := filepath.Join(dir, filepath.Clean("/"+rest))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		http.ServeFile(w, r, path)
	})
}

func findWebDist(app string) string {
	candidates := []string{
		filepath.Join("web", "apps", app, "dist"),
		filepath.Join("..", "web", "apps", app, "dist"),
		filepath.Join("..", "..", "web", "apps", app, "dist"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "web", "apps", app, "dist"))
	}
	for _, c := range candidates {
		if info, err := os.Stat(filepath.Join(c, "index.html")); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}
