package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"cinexplorer/internal/httpx"
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

var imdbIDRe = regexp.MustCompile(`^tt\d{7,8}$`)

func (s *Server) unidentified(w http.ResponseWriter, r *http.Request) {
	u, err := s.Store.Unidentified()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, u)
}

// search is the manual TMDB search: by title (and optional year), or by
// IMDb id when q is one.
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	if s.TMDB == nil {
		http.Error(w, "sin token de TMDB", http.StatusServiceUnavailable)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	query := identify.Query{Title: q, Year: year}
	if imdbIDRe.MatchString(q) {
		query = identify.Query{IMDbID: q}
	}
	cands, _, err := identify.Search(r.Context(), s.TMDB, s.Language, query)
	if err != nil {
		tmdbError(w, err)
		return
	}
	writeJSON(w, cands)
}

// identify records the user's decision about a version's fingerprint.
func (s *Server) identify(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly || s.Identifier == nil {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return
	}
	var req struct {
		Fingerprint string `json:"fingerprint"`
		Action      string `json:"action"` // movie | extra | ignore | reset
		TMDBID      int    `json:"tmdbId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	var err error
	switch req.Action {
	case "movie", "extra":
		if req.TMDBID <= 0 {
			http.Error(w, "falta tmdbId", http.StatusBadRequest)
			return
		}
		if err := s.Identifier.Adopt(r.Context(), req.TMDBID); err != nil {
			tmdbError(w, err)
			return
		}
		status := store.StatusManual
		if req.Action == "extra" {
			status = store.StatusExtra
		}
		if err = s.Store.SetCorrection(req.Fingerprint, status, req.TMDBID); err == nil {
			s.Identifier.Trigger() // Wikidata and images for the new movie
		}
	case "ignore":
		err = s.Store.SetCorrection(req.Fingerprint, store.StatusIgnored, 0)
	case "reset":
		if err = s.Store.ResetIdentification(req.Fingerprint); err == nil {
			s.Identifier.Trigger()
		}
	default:
		http.Error(w, "acción desconocida", http.StatusBadRequest)
		return
	}
	switch {
	case errors.Is(err, store.ErrUnknownFingerprint):
		http.Error(w, err.Error(), http.StatusNotFound)
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func tmdbError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tmdb.ErrNotFound):
		http.Error(w, "no existe en TMDB", http.StatusNotFound)
	case errors.Is(err, httpx.ErrOffline):
		http.Error(w, "sin conexión con TMDB", http.StatusServiceUnavailable)
	case errors.Is(err, tmdb.ErrUnauthorized):
		http.Error(w, err.Error(), http.StatusBadGateway)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// image serves /img/{poster|backdrop}/{tmdbId}.jpg from the cache,
// downloading it once when missing. A candidate that is not a stored movie
// yet passes its TMDB path as ?p=.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	kind := images.Kind(r.PathValue("kind"))
	idText, ok := strings.CutSuffix(r.PathValue("file"), ".jpg")
	id, err := strconv.Atoi(idText)
	if s.Images == nil || !images.ValidKind(kind) || !ok || err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	m, found, err := s.Store.Movie(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var b []byte
	if found {
		path := m.PosterPath
		if kind == images.Backdrop {
			path = m.BackdropPath
		}
		b, err = s.Images.Get(r.Context(), kind, id, path)
	} else {
		// ?p= comes from the page (any page can send it): shown, never cached.
		b, err = s.Images.Preview(r.Context(), kind, id, r.URL.Query().Get("p"))
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(b)
}
