package server

import (
	"net/http"
	"strconv"
	"sync"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/search"
)

// searchState is what the search keeps between requests: the index and the
// items it was built from, current while the catalog's change counter stays.
type searchState struct {
	once  sync.Once
	index *search.Index
	err   error

	mu      sync.Mutex
	built   bool
	changes int64
	items   []catalog.Item
	byKey   map[string]catalog.Item
}

// searchItems returns the items of the catalog, with the index up to date.
// Typing makes a request per pause: while nothing changes they are reused
// instead of reading the whole catalog each time.
func (s *Server) searchItems() (*search.Index, []catalog.Item, map[string]catalog.Item, error) {
	st := &s.finder
	st.once.Do(func() { st.index, st.err = search.New() })
	if st.err != nil {
		return nil, nil, nil, st.err
	}
	changes, err := s.Store.Changes()
	if err != nil {
		return nil, nil, nil, err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.built && st.changes == changes {
		return st.index, st.items, st.byKey, nil
	}
	snap, err := s.Store.Snapshot()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := st.index.Refresh(snap.Changes, func() []search.Doc { return catalog.SearchDocs(snap) }); err != nil {
		return nil, nil, nil, err
	}
	st.items = catalog.Items(snap, nil)
	st.byKey = make(map[string]catalog.Item, len(st.items))
	for i := range st.items {
		st.byKey[catalog.ItemKey(&st.items[i])] = st.items[i]
	}
	st.built, st.changes = true, snap.Changes
	return st.index, st.items, st.byKey, nil
}

// find is the instant search: the items whose titles, directors, cast or
// file names match q, best first, and the directors whose name matches.
func (s *Server) find(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 200 {
		limit = 8
	}
	idx, all, byKey, err := s.searchItems()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	keys, err := idx.Query(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	found := []catalog.Item{}
	for _, k := range keys {
		if it, ok := byKey[k]; ok {
			found = append(found, it)
		}
	}
	writeJSON(w, map[string]any{"q": q, "total": len(found), "items": found[:min(limit, len(found))],
		"directors": catalog.Directors(all, found, search.Terms(q), 5)})
}
