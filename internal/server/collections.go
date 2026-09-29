package server

import (
	"net/http"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/store"
)

func (s *Server) collections(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.Collections(snap))
}

// pendingFolder finds the folder of Collections/ at path among those with
// something to offer, answering 404 when it is not one.
func pendingFolder(w http.ResponseWriter, snap store.Snapshot, path string) (catalog.CollectionFolder, bool) {
	for _, f := range catalog.Collections(snap) {
		if f.Path == path {
			return f, true
		}
	}
	http.Error(w, "la carpeta no tiene nada para importar", http.StatusNotFound)
	return catalog.CollectionFolder{}, false
}

// importCollection adds a folder's new contents to a new list (name) or to
// an existing one (listId).
func (s *Server) importCollection(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	var req struct {
		Path   string `json:"path"`
		Name   string `json:"name"`
		ListID int64  `json:"listId"`
	}
	if !decode(w, r, &req) {
		return
	}
	if (req.Name != "") == (req.ListID > 0) {
		http.Error(w, "indicá el nombre de una lista nueva o una lista existente", http.StatusBadRequest)
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	f, ok := pendingFolder(w, snap, req.Path)
	if !ok {
		return
	}
	id, err := s.Store.ImportCollection(req.Path, req.Name, req.ListID, f.Fingerprints)
	if err != nil {
		listError(w, err)
		return
	}
	writeJSON(w, map[string]int64{"listId": id})
}

// dismissCollection declines a folder's new contents.
func (s *Server) dismissCollection(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if !decode(w, r, &req) {
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	f, ok := pendingFolder(w, snap, req.Path)
	if !ok {
		return
	}
	if err := s.Store.DismissCollection(req.Path, f.Fingerprints); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
