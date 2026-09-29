package catalog

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"cinexplorer/internal/images"
	"cinexplorer/internal/quality"
	"cinexplorer/internal/store"
)

// VersionCard is a version as shown in a movie's page.
type VersionCard struct {
	store.VersionView
	Copies int `json:"copies"` // present versions with this content, itself included
}

// ExtraFile is a bonus file (making-of, trailer…) found with a version.
type ExtraFile struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Missing bool   `json:"missing"`
}

// MovieDetail is a movie's page.
type MovieDetail struct {
	Movie         store.Movie   `json:"movie"`
	Poster        string        `json:"poster"`   // image versions (see images.Version)
	Backdrop      string        `json:"backdrop"` //
	Versions      []VersionCard `json:"versions"`
	Extras        []ExtraFile   `json:"extras"`
	ExtraVersions []VersionCard `json:"extraVersions"` // versions corrected as extras of this movie
	Lists         []ListRef     `json:"lists"`         // the lists that hold it
}

// Movie returns the page of a stored movie: its versions (the best first,
// missing ones last), the extra files found with them, and the versions the
// user marked as its extras.
func Movie(snap store.Snapshot, id int) (MovieDetail, bool) {
	m, ok := snap.Movies[id]
	if !ok {
		return MovieDetail{}, false
	}
	d := MovieDetail{Movie: m, Poster: images.Version(m.PosterPath), Backdrop: images.Version(m.BackdropPath),
		Versions: []VersionCard{}, Extras: []ExtraFile{}, ExtraVersions: []VersionCard{}}
	copies := copyCounts(snap)
	for i := range snap.Versions {
		v := &snap.Versions[i]
		ident := v.Identification
		if ident == nil || ident.TMDBID != id {
			continue
		}
		switch ident.Status {
		case store.StatusAuto, store.StatusManual:
			d.Versions = append(d.Versions, VersionCard{VersionView: *v, Copies: copies[v.Fingerprint]})
			for _, f := range v.Files {
				if f.Role == "extra" {
					d.Extras = append(d.Extras, ExtraFile{Path: f.Path, Size: f.Size, Missing: f.Missing})
				}
			}
		case store.StatusExtra:
			d.ExtraVersions = append(d.ExtraVersions, VersionCard{VersionView: *v, Copies: copies[v.Fingerprint]})
		}
	}
	d.Lists = listsOf(snap, store.RefMovie(id)) // a movie item's id is its RefMovie
	sortCards(d.Versions)
	return d, true
}

// copyCounts counts the present versions of each content.
func copyCounts(snap store.Snapshot) map[string]int {
	out := map[string]int{}
	for i := range snap.Versions {
		if v := &snap.Versions[i]; v.Fingerprint != "" && hasPresentMain(v) {
			out[v.Fingerprint]++
		}
	}
	return out
}

// sortCards puts the best version first, then present before missing, then
// higher resolutions and larger sizes.
func sortCards(cs []VersionCard) {
	slices.SortStableFunc(cs, func(a, b VersionCard) int {
		if a.Best != b.Best {
			if a.Best {
				return -1
			}
			return 1
		}
		pa, pb := hasPresentMain(&a.VersionView), hasPresentMain(&b.VersionView)
		if pa != pb {
			if pa {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(resRank[Resolution(b.Resolution)], resRank[Resolution(a.Resolution)]); c != 0 {
			return c
		}
		return cmp.Compare(b.Size, a.Size)
	})
}

// IdentityView is what is known about a version's identity.
type IdentityView struct {
	Status     string            `json:"status"`
	TMDBID     int               `json:"tmdbId"`
	Confidence float64           `json:"confidence"`
	Candidates []store.Candidate `json:"candidates"`
}

// VersionDetail is the page of content that is not (or not yet) a movie of
// the catalog: the versions with one fingerprint, or one version without
// fingerprint.
type VersionDetail struct {
	Key            string          `json:"key"`
	Title          string          `json:"title"`
	Year           int             `json:"year"`
	Versions       []VersionCard   `json:"versions"`
	Identification *IdentityView   `json:"identification"` // nil when never identified
	Movie          *store.MovieRef `json:"movie"`          // set when identified as a stored movie
	Lists          []ListRef       `json:"lists"`          // the lists that hold it
}

// Version returns the page of a version key (see VersionKey).
func Version(snap store.Snapshot, key string) (VersionDetail, bool) {
	d := VersionDetail{Key: key, Versions: []VersionCard{}}
	copies := copyCounts(snap)
	for i := range snap.Versions {
		v := &snap.Versions[i]
		if VersionKey(v) != key {
			continue
		}
		if len(d.Versions) == 0 {
			d.Title, d.Year, d.Movie = v.Title, v.Year, v.Movie
		}
		d.Versions = append(d.Versions, VersionCard{VersionView: *v, Copies: copies[v.Fingerprint]})
	}
	if len(d.Versions) == 0 {
		return d, false
	}
	d.Lists = listsOf(snap, key)
	sortCards(d.Versions)
	if id := snap.Identifications[d.Versions[0].Fingerprint]; id != nil && d.Versions[0].Fingerprint != "" {
		cands := id.Candidates
		if cands == nil {
			cands = []store.Candidate{}
		}
		d.Identification = &IdentityView{Status: id.Status, TMDBID: id.TMDBID, Confidence: id.Confidence, Candidates: cands}
	}
	return d, true
}

// suggestLimit is how many movies SuggestMovies returns.
const suggestLimit = 20

// SuggestMovies lists catalog movies for "es un extra de…": those in the
// folder of the version key near or in its parent folder first, then the
// rest, by title. q, when set, keeps the movies whose title or original
// title contains it (ignoring case, accents and punctuation).
func SuggestMovies(snap store.Snapshot, q, near string) []store.MovieRef {
	dirs := map[string]bool{}
	for i := range snap.Versions {
		if v := &snap.Versions[i]; near != "" && VersionKey(v) == near {
			dirs[v.Dir], dirs[path.Dir(v.Dir)] = true, true
		}
	}
	nq := quality.NormTitle(q)
	type cand struct {
		ref   store.MovieRef
		near  bool
		title string
	}
	var cs []cand
	for _, e := range build(snap, nil) {
		if e.item.Kind != KindMovie {
			continue
		}
		m := snap.Movies[e.item.TMDBID]
		if nq != "" && !strings.Contains(quality.NormTitle(m.Title), nq) && !strings.Contains(quality.NormTitle(m.OriginalTitle), nq) {
			continue
		}
		c := cand{ref: store.MovieRef{TMDBID: m.TMDBID, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: m.Year,
			Directors: m.Directors}, title: e.item.norm}
		if c.ref.Directors == nil {
			c.ref.Directors = []store.Person{}
		}
		for _, v := range e.versions {
			if dirs[v.Dir] {
				c.near = true
			}
		}
		cs = append(cs, c)
	}
	slices.SortStableFunc(cs, func(a, b cand) int {
		if a.near != b.near {
			if a.near {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(a.title, b.title); c != 0 {
			return c
		}
		return cmp.Compare(a.ref.TMDBID, b.ref.TMDBID)
	})
	out := []store.MovieRef{}
	for _, c := range cs[:min(len(cs), suggestLimit)] {
		out = append(out, c.ref)
	}
	return out
}

// UnidentifiedItem is a version in the Sin identificar queue.
type UnidentifiedItem struct {
	store.VersionView
	Candidates []store.Candidate `json:"candidates"`
}

// UnidentifiedReport is the Sin identificar queue: the contents the matcher
// could not decide on (one per fingerprint, in title order), and how many
// are still waiting to be identified at all.
type UnidentifiedReport struct {
	Pending int                `json:"pending"`
	Items   []UnidentifiedItem `json:"items"`
}

// Unidentified returns the Sin identificar queue.
func Unidentified(snap store.Snapshot) UnidentifiedReport {
	rep := UnidentifiedReport{Items: []UnidentifiedItem{}}
	seen := map[string]bool{}
	for i := range snap.Versions {
		v := &snap.Versions[i]
		if v.Fingerprint == "" || seen[v.Fingerprint] || !hasPresentMain(v) {
			continue
		}
		seen[v.Fingerprint] = true
		switch id := snap.Identifications[v.Fingerprint]; {
		case id == nil:
			rep.Pending++
		case id.Status == store.StatusUnmatched:
			cands := id.Candidates
			if cands == nil {
				cands = []store.Candidate{}
			}
			rep.Items = append(rep.Items, UnidentifiedItem{VersionView: *v, Candidates: cands})
		}
	}
	return rep
}
