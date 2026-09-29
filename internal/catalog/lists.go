package catalog

import (
	"cmp"
	"maps"
	"slices"
	"strconv"

	"cinexplorer/internal/images"
	"cinexplorer/internal/quality"
	"cinexplorer/internal/store"
)

// ListRef names a list.
type ListRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// inList is an item's place in one list.
type inList struct {
	id    int64
	name  string
	added int64 // unix ms: the earliest entry that leads to the item
}

// targets maps the entries a list can hold to the items built from a
// snapshot: RefMovie of every movie item, and RefFingerprint of every
// present content, which leads to its movie's item once identified and to
// its own otherwise. Contents that are extras, not movies or missing lead
// nowhere, since build leaves them out.
func targets(es []*entry) map[string]*entry {
	out := map[string]*entry{}
	for _, e := range es {
		if e.item.Kind == KindMovie {
			out[store.RefMovie(e.item.TMDBID)] = e
		}
		for _, v := range e.versions {
			if v.Fingerprint != "" {
				out[store.RefFingerprint(v.Fingerprint)] = e
			}
		}
	}
	return out
}

// attachLists records on each item the lists that lead to it.
func attachLists(es []*entry, lists []store.List) {
	to := targets(es)
	for _, l := range lists {
		for _, en := range l.Entries {
			if e := to[en.Ref]; e != nil {
				e.item.addList(l.ID, l.Name, en.AddedAt)
			}
		}
	}
}

func (it *Item) addList(id int64, name string, added int64) {
	for i := range it.lists {
		if it.lists[i].id == id {
			it.lists[i].added = min(it.lists[i].added, added)
			return
		}
	}
	it.lists = append(it.lists, inList{id: id, name: name, added: added})
}

// addedTo tells when the item was added to the list id (as the lista facet
// has it), and whether it is in it at all.
func (it *Item) addedTo(id string) (int64, bool) {
	for _, l := range it.lists {
		if strconv.FormatInt(l.id, 10) == id {
			return l.added, true
		}
	}
	return 0, false
}

// listsOf names the lists that lead to the item itemID (see Item.id), by
// name.
func listsOf(snap store.Snapshot, itemID string) []ListRef {
	out := []ListRef{}
	for _, e := range build(snap, nil) {
		if e.item.id() != itemID {
			continue
		}
		for _, l := range e.item.lists {
			out = append(out, ListRef{ID: l.id, Name: l.name})
		}
	}
	slices.SortFunc(out, func(a, b ListRef) int {
		return cmp.Or(cmp.Compare(quality.NormTitle(a.Name), quality.NormTitle(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// ItemRef is the entry that adds an item to a list: a stored movie by its
// TMDB id, or a present content by its fingerprint (the item's key). ok is
// false for anything else.
func ItemRef(snap store.Snapshot, tmdbID int, key string) (string, bool) {
	if tmdbID > 0 {
		if _, ok := snap.Movies[tmdbID]; ok {
			return store.RefMovie(tmdbID), true
		}
		return "", false
	}
	for i := range snap.Versions {
		if v := &snap.Versions[i]; key != "" && v.Fingerprint == key && hasPresentMain(v) {
			return store.RefFingerprint(key), true
		}
	}
	return "", false
}

// RefsTo lists the entries of the list listID that lead to an item — the
// movie tmdbID, or the content key — plus the item's own entry, so that
// removing the item removes them all.
func RefsTo(snap store.Snapshot, listID int64, tmdbID int, key string) []string {
	direct := store.RefFingerprint(key)
	if tmdbID > 0 {
		direct = store.RefMovie(tmdbID)
	}
	to := targets(build(snap, nil))
	target := to[direct]
	var out []string
	for _, l := range snap.Lists {
		if l.ID != listID {
			continue
		}
		for _, en := range l.Entries {
			if en.Ref == direct || (target != nil && to[en.Ref] == target) {
				out = append(out, en.Ref)
			}
		}
	}
	return out
}

// ListCard is a list as the Listas page shows it.
type ListCard struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Count     int    `json:"count"`     // items present
	UpdatedAt int64  `json:"updatedAt"` // unix ms
	Cover     *Cover `json:"cover"`     // nil when none of its movies has a still
}

// Cover is the still of a list's card.
type Cover struct {
	TMDBID   int    `json:"tmdbId"`
	Backdrop string `json:"backdrop"` // image version
}

// ListCards returns every list, the most recently changed first. A card's
// cover is the still of the movie added last to the list that has one.
func ListCards(snap store.Snapshot) []ListCard {
	es := build(snap, nil)
	out := []ListCard{}
	for _, l := range snap.Lists {
		c := ListCard{ID: l.ID, Name: l.Name, UpdatedAt: l.UpdatedAt}
		id := strconv.FormatInt(l.ID, 10)
		var coverAdded int64
		for _, e := range es {
			added, ok := e.item.addedTo(id)
			if !ok {
				continue
			}
			c.Count++
			if e.item.Kind != KindMovie {
				continue
			}
			m := snap.Movies[e.item.TMDBID]
			if m.BackdropPath != "" && (c.Cover == nil || added > coverAdded) {
				c.Cover, coverAdded = &Cover{TMDBID: m.TMDBID, Backdrop: images.Version(m.BackdropPath)}, added
			}
		}
		out = append(out, c)
	}
	slices.SortStableFunc(out, func(a, b ListCard) int {
		return cmp.Or(-cmp.Compare(a.UpdatedAt, b.UpdatedAt), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// ApplyLists checks q's lista facet against the lists of snap. A list that
// no longer exists is dropped like a malformed value, and so is its order.
// It returns the list applied, nil when none.
func ApplyLists(q Query, snap store.Snapshot) (Query, *ListRef) {
	id := q.Facets[FacetList]
	if id == "" {
		return q, nil
	}
	for _, l := range snap.Lists {
		if strconv.FormatInt(l.ID, 10) == id {
			return q, &ListRef{ID: l.ID, Name: l.Name}
		}
	}
	q.Facets = maps.Clone(q.Facets)
	delete(q.Facets, FacetList)
	if q.Order == OrderListAdded {
		q.Order, q.Dir = OrderYear, Desc
	}
	return q, nil
}
