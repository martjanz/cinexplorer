package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/store"
)

// writable answers 409 when the catalog is open read-only.
func (s *Server) writable(w http.ResponseWriter) bool {
	if s.ReadOnly {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return false
	}
	return true
}

// listError answers an error of the lists' store calls.
func listError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrListName):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, store.ErrListExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, store.ErrUnknownList):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// decode reads a JSON body into v, answering 400 when it is not one.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return false
	}
	return true
}

// pathList reads the list id of the URL, answering 404 when malformed.
func pathList(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, store.ErrUnknownList.Error(), http.StatusNotFound)
		return 0, false
	}
	return id, true
}

// itemRequest names an item of Explorar: a movie or a content.
type itemRequest struct {
	TMDBID int    `json:"tmdbId"`
	Key    string `json:"key"`
}

// readItem reads the item of an entries request: exactly one of tmdbId and
// key.
func readItem(w http.ResponseWriter, r *http.Request) (itemRequest, bool) {
	var req itemRequest
	if !decode(w, r, &req) {
		return req, false
	}
	if (req.TMDBID > 0) == (req.Key != "") {
		http.Error(w, "falta tmdbId o key", http.StatusBadRequest)
		return req, false
	}
	return req, true
}

func (s *Server) lists(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.ListCards(snap))
}

func (s *Server) createList(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	l, err := s.Store.CreateList(req.Name)
	if err != nil {
		listError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(catalog.ListRef{ID: l.ID, Name: l.Name})
}

func (s *Server) renameList(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := pathList(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.Store.RenameList(id, req.Name); err != nil {
		listError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteList(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := pathList(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteList(id); err != nil {
		listError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addEntry(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := pathList(w, r)
	if !ok {
		return
	}
	req, ok := readItem(w, r)
	if !ok {
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	ref, found := catalog.ItemRef(snap, req.TMDBID, req.Key)
	if !found {
		http.Error(w, "no está en el catálogo", http.StatusNotFound)
		return
	}
	if err := s.Store.AddEntries(id, []string{ref}); err != nil {
		listError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeEntry removes an item from a list: every entry that leads to it.
func (s *Server) removeEntry(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := pathList(w, r)
	if !ok {
		return
	}
	req, ok := readItem(w, r)
	if !ok {
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	if err := s.Store.RemoveEntries(id, catalog.RefsTo(snap, id, req.TMDBID, req.Key)); err != nil {
		listError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
