// Package server exposes the catalog as a JSON API plus the embedded web UI.
// It only answers requests addressed to localhost.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/store"
)

//go:embed web
var webFS embed.FS

type Server struct {
	AppDir   string
	Store    *store.Store
	Scanner  *scan.Scanner // nil in read-only mode
	ReadOnly bool
	Opener   func(target string) error
	Revealer func(target string) error

	TMDB       identify.API     // nil without a TMDB token
	Identifier *identify.Runner // nil in read-only mode
	Images     *images.Cache
	Language   string
}

func (s *Server) Handler() http.Handler {
	static, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/versions", s.versions)
	mux.HandleFunc("GET /api/duplicates", s.duplicates)
	mux.HandleFunc("POST /api/scan", jsonOnly(s.rescan))
	mux.HandleFunc("POST /api/open", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Opener) }))
	mux.HandleFunc("POST /api/reveal", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Revealer) }))
	mux.HandleFunc("GET /api/unidentified", s.unidentified)
	mux.HandleFunc("GET /api/tmdb/search", s.search)
	mux.HandleFunc("POST /api/identify", jsonOnly(s.identify))
	mux.HandleFunc("GET /img/{kind}/{file}", s.image)
	return localOnly(mux)
}

// localOnly rejects requests whose Host is not localhost (DNS rebinding).
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
	var st scan.Status
	if s.Scanner != nil {
		st = s.Scanner.Status()
	}
	var id *identify.Status
	if s.Identifier != nil {
		st := s.Identifier.Status()
		id = &st
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "scan": st, "identify": id})
}

func (s *Server) versions(w http.ResponseWriter, r *http.Request) {
	vs, err := s.Store.Versions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, vs)
}

func (s *Server) duplicates(w http.ResponseWriter, r *http.Request) {
	d, err := s.Store.Duplicates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, d)
}

func (s *Server) rescan(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly || s.Scanner == nil {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return
	}
	go func() {
		if err := s.Scanner.Run(context.Background()); err != nil && !errors.Is(err, scan.ErrBusy) {
			log.Printf("escaneo: %v", err)
		}
	}()
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
