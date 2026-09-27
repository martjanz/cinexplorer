package server

import (
	"net/http"
	"strconv"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/store"
)

// snapshot reads the catalog for one request, answering 500 on failure.
func (s *Server) snapshot(w http.ResponseWriter) (store.Snapshot, bool) {
	snap, err := s.Store.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return snap, false
	}
	return snap, true
}

// explore answers Explorar: the items that match the facets in the query,
// sorted, plus every facet's values counted over the other facets.
func (s *Server) explore(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	q := catalog.ParseQuery(r.URL.Query())
	all := catalog.Items(snap, s.rt().Config.Roots)
	items := catalog.Filter(all, q.Facets)
	catalog.Sort(items, q.Order, q.Dir)
	writeJSON(w, map[string]any{"total": len(items), "query": q, "items": items, "facets": catalog.Counts(all, q.Facets)})
}

// home answers the home page: rows of movies, drawn with the page's seed.
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	seed, _ := strconv.ParseUint(r.URL.Query().Get("seed"), 10, 64)
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.HomePage(snap, seed))
}

func (s *Server) movie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	d, found := catalog.Movie(snap, id)
	if !found {
		http.Error(w, "la película no está en el catálogo", http.StatusNotFound)
		return
	}
	writeJSON(w, d)
}

// movies suggests catalog movies for "es un extra de…".
func (s *Server) movies(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.SuggestMovies(snap, r.URL.Query().Get("q"), r.URL.Query().Get("near")))
}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	d, found := catalog.Version(snap, r.PathValue("key"))
	if !found {
		http.Error(w, "la versión no está en el catálogo", http.StatusNotFound)
		return
	}
	writeJSON(w, d)
}

func (s *Server) duplicates(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.Duplicates(snap, s.rt().Config.Roots))
}

func (s *Server) unidentified(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.Unidentified(snap))
}
