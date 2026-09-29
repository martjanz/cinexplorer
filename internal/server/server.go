// Package server exposes the catalog as a JSON API plus the embedded web UI.
// It only answers requests addressed to localhost.
package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/engine"
	"cinexplorer/internal/identify"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/store"
)

type Server struct {
	AppDir   string
	Store    *store.Store
	ReadOnly bool
	Opener   func(target string) error
	Revealer func(target string) error
	// Engine holds what config.json decides (roots, TMDB, scanner,
	// identification, images, language); handlers read it through rt.
	Engine *engine.Engine
	// VerifyToken asks TMDB whether it takes a token; nil means asking
	// for a well-known movie.
	VerifyToken func(ctx context.Context, token string) error

	Static fs.FS // the web app; nil: the embedded build

	finder searchState
}

// rt is the runtime in use. A handler reads it once: settings saved during
// the request replace it, they do not change it.
func (s *Server) rt() *engine.Runtime { return s.Engine.Current() }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", s.static())
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/versions", s.versions)
	mux.HandleFunc("GET /api/versions/{key}", s.version)
	mux.HandleFunc("GET /api/explore", s.explore)
	mux.HandleFunc("GET /api/movies", s.movies)
	mux.HandleFunc("GET /api/movies/{id}", s.movie)
	mux.HandleFunc("GET /api/duplicates", s.duplicates)
	mux.HandleFunc("POST /api/scan", jsonOnly(s.rescan))
	mux.HandleFunc("POST /api/open", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Opener) }))
	mux.HandleFunc("POST /api/reveal", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Revealer) }))
	mux.HandleFunc("GET /api/unidentified", s.unidentified)
	mux.HandleFunc("GET /api/tmdb/search", s.search)
	mux.HandleFunc("POST /api/identify", jsonOnly(s.identify))
	mux.HandleFunc("GET /img/{kind}/{file}", s.image)
	mux.HandleFunc("GET /api/search", s.find)
	mux.HandleFunc("GET /api/home", s.home)
	mux.HandleFunc("GET /api/lists", s.lists)
	mux.HandleFunc("POST /api/lists", jsonOnly(s.createList))
	mux.HandleFunc("PATCH /api/lists/{id}", jsonOnly(s.renameList))
	mux.HandleFunc("DELETE /api/lists/{id}", jsonOnly(s.deleteList))
	mux.HandleFunc("POST /api/lists/{id}/entries", jsonOnly(s.addEntry))
	mux.HandleFunc("DELETE /api/lists/{id}/entries", jsonOnly(s.removeEntry))
	mux.HandleFunc("GET /api/collections", s.collections)
	mux.HandleFunc("POST /api/collections/import", jsonOnly(s.importCollection))
	mux.HandleFunc("POST /api/collections/dismiss", jsonOnly(s.dismissCollection))
	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("PUT /api/config", jsonOnly(s.putConfig))
	mux.HandleFunc("POST /api/config/root", jsonOnly(s.checkRoot))
	mux.HandleFunc("POST /api/config/token", jsonOnly(s.checkToken))
	return localOnly(mux)
}

// localOnly rejects requests whose Host is not localhost (DNS rebinding) and
// requests made by other sites' pages: an <img> or <script> pointing at the
// app would otherwise reveal which movies the catalog has, or spend the TMDB
// quota. Navigating to the app from a link still works.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		site := r.Header.Get("Sec-Fetch-Site")
		navigation := r.Method == http.MethodGet && r.Header.Get("Sec-Fetch-Mode") == "navigate"
		if (site == "cross-site" || site == "same-site") && !navigation {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		// Browsers without Sec-Fetch-* still refuse to show the responses
		// to other origins.
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// jsonOnly requires a JSON content type, which forces a CORS preflight on
// cross-site requests and so blocks other web pages from calling the API.
func jsonOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "se requiere application/json", http.StatusUnsupportedMediaType)
			return
		}
		h(w, r)
	}
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	rt := s.rt()
	var st scan.Status
	if rt.Scanner != nil {
		st = rt.Scanner.Status()
	}
	var id *identify.Status
	if rt.Identifier != nil {
		st := rt.Identifier.Status()
		id = &st
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "setupPending": s.Engine.SetupPending(), "scan": st, "identify": id})
}

func (s *Server) versions(w http.ResponseWriter, r *http.Request) {
	vs, err := s.Store.Versions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, vs)
}

func (s *Server) rescan(w http.ResponseWriter, r *http.Request) {
	if s.Engine.SetupPending() {
		http.Error(w, "falta completar el asistente de primer uso", http.StatusConflict)
		return
	}
	rt := s.rt()
	if s.ReadOnly || rt.Scanner == nil {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return
	}
	rt.Scan()
	w.WriteHeader(http.StatusAccepted)
}

// withFile runs action on a catalogued file; arbitrary paths are refused.
func (s *Server) withFile(w http.ResponseWriter, r *http.Request, action func(string) error) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	ok, err := s.Store.HasFile(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "archivo desconocido", http.StatusNotFound)
		return
	}
	if err := action(appdir.Abs(s.AppDir, req.Path)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json: %v", err)
	}
}
