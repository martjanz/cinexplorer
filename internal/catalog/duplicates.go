package catalog

import (
	"cmp"
	"slices"

	"cinexplorer/internal/store"
)

// Duplicate types.
const (
	DupIdentical = "identical" // the same content (fingerprint) in two or more places
	DupVersions  = "versions"  // different contents of the same movie
)

// DupVersion is a version as listed in Duplicados.
type DupVersion struct {
	ID          int64  `json:"id"`
	Fingerprint string `json:"fingerprint"`
	Resolution  string `json:"resolution"`
	Size        int64  `json:"size"`
	Path        string `json:"path"` // its first present main file
	Best        bool   `json:"best"`
}

// DupGroup is a movie (or unidentified content) with duplicates.
type DupGroup struct {
	Types       []string     `json:"types"`
	Kind        string       `json:"kind"`
	TMDBID      int          `json:"tmdbId,omitempty"`
	Key         string       `json:"key,omitempty"`
	Title       string       `json:"title"`
	Year        int          `json:"year"`
	Recoverable int64        `json:"recoverable"`
	Versions    []DupVersion `json:"versions"`
}

type DupReport struct {
	Recoverable int64      `json:"recoverable"`
	Groups      []DupGroup `json:"groups"`
}

// Duplicates lists the items with identical copies or several versions, the
// most space recoverable first. Recoverable space is what deleting all but
// one copy of each content would free, plus, for several versions of a
// movie, the contents other than the best one.
func Duplicates(snap store.Snapshot, roots []string) DupReport {
	rep := DupReport{Groups: []DupGroup{}}
	for _, e := range build(snap, roots) {
		it := &e.item
		if !it.identical && !it.multiVersion {
			continue
		}
		g := DupGroup{Types: []string{}, Kind: it.Kind, TMDBID: it.TMDBID, Key: it.Key, Title: it.Title, Year: it.Year}
		var keep *store.VersionView // the version worth keeping
		for _, v := range e.versions {
			if keep == nil || v.Best || (!keep.Best && v.Size > keep.Size) {
				keep = v
			}
		}
		if it.identical {
			g.Types = append(g.Types, DupIdentical)
		}
		if it.multiVersion {
			g.Types = append(g.Types, DupVersions)
		}
		counted := map[string]bool{}
		for _, v := range e.versions {
			g.Versions = append(g.Versions, DupVersion{ID: v.ID, Fingerprint: v.Fingerprint, Resolution: v.Resolution,
				Size: v.Size, Path: firstMain(v), Best: v.Best})
			if v.Fingerprint != "" && counted[v.Fingerprint] {
				g.Recoverable += v.Size // one more copy of a content already counted
				continue
			}
			counted[v.Fingerprint] = v.Fingerprint != ""
			if it.multiVersion && !sameContent(v, keep) {
				g.Recoverable += v.Size
			}
		}
		slices.SortStableFunc(g.Versions, func(a, b DupVersion) int {
			if a.Best != b.Best {
				if a.Best {
					return -1
				}
				return 1
			}
			return cmp.Compare(b.Size, a.Size)
		})
		rep.Recoverable += g.Recoverable
		rep.Groups = append(rep.Groups, g)
	}
	slices.SortStableFunc(rep.Groups, func(a, b DupGroup) int {
		if c := cmp.Compare(b.Recoverable, a.Recoverable); c != 0 {
			return c
		}
		return cmp.Compare(a.Title, b.Title)
	})
	return rep
}

// sameContent reports whether two versions hold the same content.
func sameContent(a, b *store.VersionView) bool {
	if a.Fingerprint == "" || b.Fingerprint == "" {
		return a == b
	}
	return a.Fingerprint == b.Fingerprint
}

func firstMain(v *store.VersionView) string {
	for _, f := range v.Files {
		if f.Role == "main" && !f.Missing {
			return f.Path
		}
	}
	return ""
}
