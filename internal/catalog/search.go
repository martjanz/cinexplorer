package catalog

import (
	"cmp"
	"slices"
	"strings"

	"cinexplorer/internal/quality"
	"cinexplorer/internal/search"
	"cinexplorer/internal/store"
)

// ItemKey identifies an item in the search index: "movie:<TMDB id>", or a
// version's key.
func ItemKey(it *Item) string { return it.id() }

// SearchDocs gives the search index one document per item of Explorar, in
// title order: a movie's titles, directors and cast plus the titles parsed
// from its files; a version not identified, its parsed title and director.
func SearchDocs(snap store.Snapshot) []search.Doc {
	es := build(snap, nil)
	slices.SortStableFunc(es, func(a, b *entry) int {
		return cmp.Or(cmp.Compare(a.item.norm, b.item.norm), cmp.Compare(a.item.id(), b.item.id()))
	})
	docs := make([]search.Doc, 0, len(es))
	for _, e := range es {
		it := &e.item
		d := search.Doc{Key: it.id(), Title: it.Title, Directors: strings.Join(it.Directors, ", ")}
		if it.Kind == KindMovie {
			m := snap.Movies[it.TMDBID]
			d.Original = m.OriginalTitle
			var cast, files []string
			for _, c := range m.Cast {
				cast = append(cast, c.Name)
			}
			for _, v := range e.versions {
				if v.Title != "" && !slices.Contains(files, v.Title) {
					files = append(files, v.Title)
				}
			}
			d.Cast, d.Files = strings.Join(cast, ", "), strings.Join(files, " · ")
		}
		docs = append(docs, d)
	}
	return docs
}

// DirectorHit is a director whose name matches a search.
type DirectorHit struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"` // movies of the catalog they directed
}

// Directors lists the directors of the found items whose name has every
// term (the last one as the start of a word), with how many of all the items
// they directed; the most prolific first, at most limit.
func Directors(all, found []Item, terms []string, limit int) []DirectorHit {
	if len(terms) == 0 {
		return []DirectorHit{}
	}
	names := map[int]string{}
	for _, it := range found {
		for i, id := range it.directorIDs {
			if id > 0 && nameMatches(it.Directors[i], terms) {
				names[id] = it.Directors[i]
			}
		}
	}
	out := []DirectorHit{}
	for id, name := range names {
		n := 0
		for _, it := range all {
			if slices.Contains(it.directorIDs, id) {
				n++
			}
		}
		out = append(out, DirectorHit{id, name, n})
	}
	slices.SortFunc(out, func(a, b DirectorHit) int {
		return cmp.Or(-cmp.Compare(a.Count, b.Count), cmp.Compare(quality.NormTitle(a.Name), quality.NormTitle(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out[:min(limit, len(out))]
}

func nameMatches(name string, terms []string) bool {
	words := quality.Words(name)
	for i, t := range terms {
		ok := slices.ContainsFunc(words, func(w string) bool {
			if i == len(terms)-1 {
				return strings.HasPrefix(w, t)
			}
			return w == t
		})
		if !ok {
			return false
		}
	}
	return true
}
