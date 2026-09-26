package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist is the web app's build (web/ compiled by Vite).
//
//go:embed dist
var dist embed.FS

// webApp returns the files of the web app: s.Static, or the embedded build.
func (s *Server) webApp() fs.FS {
	if s.Static != nil {
		return s.Static
	}
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// static serves the web app. Files of the build are served as they are
// (those under assets/ carry a content hash in their name, so browsers may
// keep them for good); any other path without an extension is one of the
// app's routes (/explorar, /pelicula/123…) and gets index.html, which the
// app's router resolves.
func (s *Server) static() http.Handler {
	app := s.webApp()
	files := http.FileServerFS(app)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(p, "api/") || strings.HasPrefix(p, "img/") {
			http.NotFound(w, r)
			return
		}
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(app, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if path.Ext(p) != "" {
				http.NotFound(w, r) // a missing file, not a route
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, app, "index.html")
	})
}
