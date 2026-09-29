package catalog

import (
	"path"
	"slices"
	"strings"

	"cinexplorer/internal/store"
)

// previewSize is how many items a folder of Collections/ shows.
const previewSize = 8

// CollectionFolder is a folder of Collections/ with contents to offer as a
// list.
type CollectionFolder struct {
	Path    string   `json:"path"`
	Name    string   `json:"name"`
	Total   int      `json:"total"`   // contents present in the folder
	New     int      `json:"new"`     // contents not offered before
	Preview []Item   `json:"preview"` // the items of some of the new contents
	List    *ListRef `json:"list"`    // the list an earlier import went to
	// Fingerprints are the new contents: what importing adds.
	Fingerprints []string `json:"-"`
}

// collectionPath gives the folder of Collections/ that dir is in: the path
// down to the subfolder of the first folder named Collections (any case).
func collectionPath(dir string) (string, bool) {
	segs := strings.Split(dir, "/")
	for i, s := range segs {
		if strings.EqualFold(s, "Collections") {
			if i+1 < len(segs) {
				return strings.Join(segs[:i+2], "/"), true
			}
			return "", false
		}
	}
	return "", false
}

// Collections lists the folders of Collections/ with something to offer:
// never decided on, or imported with new contents since. Dismissed folders,
// and imported ones whose list was deleted, are not offered again. A
// folder's contents are the present versions under it, at any depth, that
// are neither extras nor marked as not movies, by fingerprint.
func Collections(snap store.Snapshot) []CollectionFolder {
	type folder struct {
		fps   []string
		items map[string]*entry // by fingerprint
	}
	folders := map[string]*folder{}
	for _, e := range build(snap, nil) {
		for _, v := range e.versions {
			p, ok := collectionPath(v.Dir)
			if !ok || v.Fingerprint == "" {
				continue
			}
			f := folders[p]
			if f == nil {
				f = &folder{items: map[string]*entry{}}
				folders[p] = f
			}
			if f.items[v.Fingerprint] == nil {
				f.items[v.Fingerprint] = e
				f.fps = append(f.fps, v.Fingerprint)
			}
		}
	}
	names := map[int64]string{}
	for _, l := range snap.Lists {
		names[l.ID] = l.Name
	}
	out := []CollectionFolder{}
	for p, f := range folders {
		c := CollectionFolder{Path: p, Name: path.Base(p), Total: len(f.fps), Preview: []Item{}, Fingerprints: []string{}}
		d, decided := snap.CollectionFolders[p]
		if decided {
			name, ok := names[d.ListID]
			if d.Status != store.CollectionImported || !ok {
				continue
			}
			c.List = &ListRef{ID: d.ListID, Name: name}
		}
		shown := map[*entry]bool{}
		for _, fp := range f.fps {
			if d.Seen[fp] {
				continue
			}
			c.Fingerprints = append(c.Fingerprints, fp)
			if e := f.items[fp]; !shown[e] && len(c.Preview) < previewSize {
				shown[e] = true
				c.Preview = append(c.Preview, e.item)
			}
		}
		if c.New = len(c.Fingerprints); c.New > 0 {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b CollectionFolder) int { return strings.Compare(a.Path, b.Path) })
	return out
}
