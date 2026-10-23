package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"cinexplorer/internal/httpx"
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

var imdbIDRe = regexp.MustCompile(`^tt\d{7,8}$`)

// search is the manual TMDB search: by title (and optional year), or by
// IMDb id when q is one.
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	rt := s.rt()
	if rt.TMDB == nil {
		http.Error(w, "sin token de TMDB", http.StatusServiceUnavailable)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	query := identify.Query{Title: q, Year: year}
	if imdbIDRe.MatchString(q) {
		query = identify.Query{IMDbID: q}
	}
	cands, _, err := identify.Search(r.Context(), rt.TMDB, rt.Config.Language, query)
	if err != nil {
		tmdbError(w, err)
		return
	}
	writeJSON(w, cands)
}

// candidateDetails is what the review shows of a candidate before deciding.
type candidateDetails struct {
	TMDBID        int      `json:"tmdbId"`
	Title         string   `json:"title"`
	OriginalTitle string   `json:"originalTitle"`
	Year          int      `json:"year"`
	Runtime       int      `json:"runtime"`
	Overview      string   `json:"overview"`
	PosterPath    string   `json:"posterPath"`
	Genres        []string `json:"genres"`
	Directors     []string `json:"directors"`
	Cast          []string `json:"cast"` // the first billed
}

const detailsCast = 6

// candidate returns a TMDB movie's details, stored or not, to tell
// candidates apart.
func (s *Server) candidate(w http.ResponseWriter, r *http.Request) {
	rt := s.rt()
	if rt.TMDB == nil {
		http.Error(w, "sin token de TMDB", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	m, err := rt.TMDB.Movie(r.Context(), id, rt.Config.Language)
	if err != nil {
		tmdbError(w, err)
		return
	}
	d := candidateDetails{TMDBID: m.ID, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: m.Year(),
		Runtime: m.Runtime, Overview: m.Overview, PosterPath: m.PosterPath,
		Genres: []string{}, Directors: []string{}, Cast: []string{}}
	for _, g := range m.Genres {
		d.Genres = append(d.Genres, g.Name)
	}
	for _, p := range m.Directors() {
		d.Directors = append(d.Directors, p.Name)
	}
	cast := slices.Clone(m.Credits.Cast)
	slices.SortStableFunc(cast, func(a, b tmdb.Person) int { return a.Order - b.Order })
	for _, p := range cast[:min(len(cast), detailsCast)] {
		d.Cast = append(d.Cast, p.Name)
	}
	writeJSON(w, d)
}

// identify records the user's decision about a version's fingerprint.
func (s *Server) identify(w http.ResponseWriter, r *http.Request) {
	runner := s.rt().Identifier
	if s.ReadOnly || runner == nil {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return
	}
	var req struct {
		Fingerprint string `json:"fingerprint"`
		Action      string `json:"action"` // movie | extra | ignore | reset | part-of | unlink
		TMDBID      int    `json:"tmdbId"`
		Leader      string `json:"leader"` // part-of: fingerprint of the version this one is a part of
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	// Checked first so an unknown fingerprint costs no TMDB request.
	known, err := s.Store.IsRepresentative(req.Fingerprint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !known {
		http.Error(w, store.ErrUnknownFingerprint.Error(), http.StatusNotFound)
		return
	}
	switch req.Action {
	case "movie", "extra":
		if req.TMDBID <= 0 {
			http.Error(w, "falta tmdbId", http.StatusBadRequest)
			return
		}
		if err := runner.Adopt(r.Context(), req.TMDBID); err != nil {
			tmdbError(w, err)
			return
		}
		status := store.StatusManual
		if req.Action == "extra" {
			status = store.StatusExtra
		}
		if err = s.Store.SetCorrection(req.Fingerprint, status, req.TMDBID); err == nil {
			runner.Trigger() // Wikidata and images for the new movie
		}
	case "ignore":
		err = s.Store.SetCorrection(req.Fingerprint, store.StatusIgnored, 0)
	case "reset":
		if err = s.Store.ResetIdentification(req.Fingerprint); err == nil {
			runner.Trigger()
		}
	case "part-of":
		if req.Leader == "" {
			http.Error(w, store.ErrUnknownFingerprint.Error(), http.StatusNotFound)
			return
		}
		err = s.Store.SetPartLink(req.Fingerprint, req.Leader)
	case "unlink":
		var n int
		if n, err = s.Store.Unlink(req.Fingerprint); err == nil && n > 0 {
			s.rt().Scan() // the next scan rebuilds the versions apart
		}
	default:
		http.Error(w, "acción desconocida", http.StatusBadRequest)
		return
	}
	switch {
	case errors.Is(err, store.ErrPartLink):
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	case errors.Is(err, identify.ErrNoToken):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, tmdb.ErrUnauthorized):
		http.Error(w, err.Error(), http.StatusBadGateway)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// image serves /img/{poster|backdrop}/{tmdbId}.jpg from the cache,
// downloading it once when missing. A candidate that is not a stored movie
// yet passes its TMDB path as ?p=. Pages add the image's version as ?v= so
// that browsers can keep it for good.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	kind := images.Kind(r.PathValue("kind"))
	idText, ok := strings.CutSuffix(r.PathValue("file"), ".jpg")
	id, err := strconv.Atoi(idText)
	cache := s.rt().Images
	if cache == nil || !images.ValidKind(kind) || !ok || err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	m, found, err := s.Store.Movie(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var b []byte
	control := "no-cache"
	if found {
		path := m.PosterPath
		if kind == images.Backdrop {
			path = m.BackdropPath
		}
		b, err = cache.Get(r.Context(), kind, id, path)
		// ?v= names the image at one path: the URL changes with the path.
		if v := r.URL.Query().Get("v"); v != "" && v == images.Version(path) {
			control = "public, max-age=31536000, immutable"
		}
	} else {
		// ?p= comes from the page (any page can send it): shown, never cached.
		b, err = cache.Preview(r.Context(), kind, id, r.URL.Query().Get("p"))
		control = "max-age=86400"
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Header().Set("Cache-Control", control)
	w.Write(b)
}
