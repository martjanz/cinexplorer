// Package catalog turns a snapshot of the store into what the pages show:
// the items of Explorar with their facets, movie and version details, and
// duplicates. Everything here is pure: no SQL, no network.
package catalog

import (
	"path"
	"strconv"
	"strings"

	"cinexplorer/internal/images"
	"cinexplorer/internal/quality"
	"cinexplorer/internal/store"
)

// Item kinds.
const (
	KindMovie   = "movie"   // an identified movie, with all its versions
	KindVersion = "version" // content not identified (yet): one per fingerprint
)

// Item is one card of Explorar.
type Item struct {
	Kind          string   `json:"kind"`
	TMDBID        int      `json:"tmdbId,omitempty"` // KindMovie
	Key           string   `json:"key,omitempty"`    // KindVersion: fingerprint, or "id:<version id>" without one
	Title         string   `json:"title"`
	OriginalTitle string   `json:"originalTitle"`
	Year          int      `json:"year"`
	Directors     []string `json:"directors"`
	Countries     []string `json:"countries"`
	Poster        string   `json:"poster"`     // image version for /img/poster/<id>.jpg?v=, "" without poster
	Resolution    string   `json:"resolution"` // the best one: "4K", "1080p", "720p", "SD" or ""
	Size          int64    `json:"size"`       // all its present versions
	Added         int64    `json:"added"`      // unix ms
	Versions      int      `json:"versions"`

	// What the facets look at.
	directorIDs  []int
	genres       []string
	lang         string
	collectionID int
	collection   string
	resolutions  []string
	subs         []string
	locations    []string
	unidentified bool
	multiVersion bool   // two or more different contents
	identical    bool   // the same content in two or more places
	norm         string // title as compared when sorting
	lists        []inList
	listAdded    int64 // when it was added to the list being sorted by (SortQuery)
}

// entry is an item with the versions it stands for.
type entry struct {
	item     Item
	versions []*store.VersionView
}

// build groups the present versions into items: identified versions of a
// stored movie into that movie; not identified ones (never identified,
// unmatched, or pointing to a movie that is not stored) by fingerprint.
// Versions that are not movies or are extras are left out, as are versions
// whose main files are all missing.
func build(snap store.Snapshot, roots []string) []*entry {
	var out []*entry
	byKey := map[string]*entry{}
	for i := range snap.Versions {
		v := &snap.Versions[i]
		if !hasPresentMain(v) {
			continue
		}
		var key string
		var movie *store.Movie
		switch id := v.Identification; {
		case id != nil && (id.Status == store.StatusIgnored || id.Status == store.StatusExtra):
			continue
		case id != nil && (id.Status == store.StatusAuto || id.Status == store.StatusManual):
			if m, ok := snap.Movies[id.TMDBID]; ok {
				movie, key = &m, "movie:"+strconv.Itoa(m.TMDBID)
			}
		}
		if key == "" {
			key = VersionKey(v)
		}
		e := byKey[key]
		if e == nil {
			e = &entry{item: newItem(v, movie)}
			byKey[key] = e
			out = append(out, e)
		}
		e.versions = append(e.versions, v)
	}
	for _, e := range out {
		e.finish(roots)
	}
	attachLists(out, snap.Lists)
	return out
}

// VersionKey is how a not identified version is addressed: its fingerprint,
// or its id while it has none (not hashed yet, or an empty file).
func VersionKey(v *store.VersionView) string {
	if v.Fingerprint != "" {
		return v.Fingerprint
	}
	return "id:" + strconv.FormatInt(v.ID, 10)
}

func newItem(v *store.VersionView, m *store.Movie) Item {
	if m == nil {
		it := Item{Kind: KindVersion, Key: VersionKey(v), Title: v.Title, Year: v.Year,
			Directors: []string{}, Countries: []string{}, unidentified: true}
		if v.Director != "" {
			it.Directors = []string{v.Director}
		}
		return it
	}
	it := Item{Kind: KindMovie, TMDBID: m.TMDBID, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: m.Year,
		Directors: []string{}, Countries: m.Countries, Poster: images.Version(m.PosterPath),
		genres: m.Genres, lang: m.OriginalLang, collectionID: m.CollectionID, collection: m.Collection}
	for _, d := range m.Directors {
		it.Directors = append(it.Directors, d.Name)
		it.directorIDs = append(it.directorIDs, d.ID)
	}
	if it.Countries == nil {
		it.Countries = []string{}
	}
	return it
}

// finish computes what depends on all the versions of the item.
func (e *entry) finish(roots []string) {
	it := &e.item
	it.norm = quality.NormTitle(it.Title)
	copies := map[string]int{}
	contents := 0
	for _, v := range e.versions {
		it.Size += v.Size
		if v.Added > 0 && (it.Added == 0 || v.Added < it.Added) {
			it.Added = v.Added
		}
		if r := Resolution(v.Resolution); r != "" {
			it.resolutions = appendNew(it.resolutions, r)
			if resRank[r] > resRank[it.Resolution] {
				it.Resolution = r
			}
		}
		for _, l := range subLangs(v) {
			it.subs = appendNew(it.subs, l)
		}
		for _, l := range locations(v.Dir, roots) {
			it.locations = appendNew(it.locations, l)
		}
		if v.Fingerprint == "" {
			contents++
		} else {
			if copies[v.Fingerprint] == 0 {
				contents++
			}
			copies[v.Fingerprint]++
		}
	}
	it.Versions = len(e.versions)
	it.multiVersion = contents > 1
	for _, n := range copies {
		if n > 1 {
			it.identical = true
		}
	}
}

// resRank orders the resolution labels of Explorar.
var resRank = map[string]int{"4K": 4, "1080p": 3, "720p": 2, "SD": 1}

// Resolution maps a version's resolution ("2160p", "1080i", "576p"…) to the
// labels of Explorar.
func Resolution(r string) string {
	switch r {
	case "":
		return ""
	case "2160p":
		return "4K"
	case "1080p", "1080i":
		return "1080p"
	case "720p":
		return "720p"
	}
	return "SD"
}

// subLangs lists the subtitle languages of a version: embedded tracks and
// external files, known languages only.
func subLangs(v *store.VersionView) []string {
	var out []string
	for _, t := range v.Subs {
		if t.Lang != "" {
			out = appendNew(out, t.Lang)
		}
	}
	for _, l := range strings.Split(v.SubLangs, ",") {
		if l != "" && l != "?" {
			out = appendNew(out, l)
		}
	}
	return out
}

// locations gives the Ubicación values of a folder: the root it is under
// and, below the root, its first-level folder ("cine" and "cine/1970s" for
// "../cine/1970s/Amarcord"). Roots are shown without their leading "../".
func locations(dir string, roots []string) []string {
	for _, r := range roots {
		r = strings.TrimSuffix(r, "/")
		if dir != r && !strings.HasPrefix(dir, r+"/") {
			continue
		}
		label := rootLabel(r)
		out := []string{label}
		if rest := strings.TrimPrefix(dir, r+"/"); dir != r {
			first, _, _ := strings.Cut(rest, "/")
			out = append(out, label+"/"+first)
		}
		return out
	}
	return nil
}

func rootLabel(root string) string {
	for strings.HasPrefix(root, "../") {
		root = strings.TrimPrefix(root, "../")
	}
	return path.Clean(root)
}

func hasPresentMain(v *store.VersionView) bool {
	for _, f := range v.Files {
		if f.Role == "main" && !f.Missing {
			return true
		}
	}
	return false
}

func appendNew(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// Items returns every item of the catalog, sorted by year, newest first.
func Items(snap store.Snapshot, roots []string) []Item {
	es := build(snap, roots)
	out := make([]Item, len(es))
	for i, e := range es {
		out[i] = e.item
	}
	Sort(out, OrderYear, "")
	return out
}
