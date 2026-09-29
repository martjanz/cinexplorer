package catalog

import (
	"cmp"
	"math/rand/v2"
	"net/url"
	"slices"
	"strconv"

	"cinexplorer/internal/images"
	"cinexplorer/internal/store"
)

// Row kinds of the home page.
const (
	RowRecent     = "recent"
	RowRandom     = "random"
	RowDecade     = "decade"
	RowDirector   = "director"
	RowCountry    = "country"
	RowGenre      = "genre"
	RowCollection = "collection"
	RowList       = "list"
)

// rowSize is how many movies a row shows at most.
const rowSize = 20

// maxListRows is how many lists Inicio shows at most; minListRow, how
// many movies a list needs for a row.
const (
	maxListRows = 2
	minListRow  = 3
)

// HomeItem is a movie as a home row shows it.
type HomeItem struct {
	TMDBID        int      `json:"tmdbId"`
	Title         string   `json:"title"`
	OriginalTitle string   `json:"originalTitle"`
	Year          int      `json:"year"`
	Directors     []string `json:"directors"`
	Countries     []string `json:"countries"`
	Backdrop      string   `json:"backdrop"` // image version, "" without one
	Poster        string   `json:"poster"`
}

// HomeRow is one row of the home page. Value is the facet value it shows
// (a decade, a director id…), Label its name, Href Explorar with that facet.
type HomeRow struct {
	Kind  string     `json:"kind"`
	Value string     `json:"value"`
	Label string     `json:"label"`
	Href  string     `json:"href"`
	Items []HomeItem `json:"items"`
}

type Home struct {
	Total int       `json:"total"` // identified movies present
	Rows  []HomeRow `json:"rows"`
}

// group is a candidate for a random row: the movies that share a value.
type group struct {
	value, label string
	movies       []*entry
}

// HomePage builds the home rows from the identified movies: the ones added
// last, some drawn at random, and a random decade, director, country, genre
// and TMDB collection with enough movies, and the lists changed last, in a random order. The same seed
// gives the same rows.
func HomePage(snap store.Snapshot, seed uint64) Home {
	var movies []*entry
	for _, e := range build(snap, nil) {
		if e.item.Kind == KindMovie {
			movies = append(movies, e)
		}
	}
	home := Home{Total: len(movies), Rows: []HomeRow{}}
	if len(movies) == 0 {
		return home
	}
	rng := rand.New(rand.NewPCG(seed, 0x636978706c6f7265)) // "cixplore"

	recent := slices.Clone(movies)
	slices.SortStableFunc(recent, func(a, b *entry) int {
		return cmp.Or(-cmp.Compare(a.item.Added, b.item.Added), cmp.Compare(a.item.norm, b.item.norm))
	})
	home.Rows = append(home.Rows, row(snap, RowRecent, group{movies: recent}, "/explorar?orden=agregado"))

	if len(movies) >= 6 {
		random := slices.Clone(movies)
		rng.Shuffle(len(random), func(i, j int) { random[i], random[j] = random[j], random[i] })
		home.Rows = append(home.Rows, row(snap, RowRandom, group{movies: random}, "/explorar"))
	}

	add := func(kind, facet string, min int, byYear bool, values func(*entry) (vals, labels []string)) {
		groups := map[string]*group{}
		for _, e := range movies {
			vals, labels := values(e)
			for i, v := range vals {
				g := groups[v]
				if g == nil {
					g = &group{value: v, label: labels[i]}
					groups[v] = g
				}
				g.movies = append(g.movies, e)
			}
		}
		var keys []string
		for v, g := range groups {
			if len(g.movies) >= min {
				keys = append(keys, v)
			}
		}
		if len(keys) == 0 {
			return
		}
		slices.Sort(keys) // maps have no order: the seed alone decides
		g := groups[keys[rng.IntN(len(keys))]]
		if byYear {
			slices.SortStableFunc(g.movies, func(a, b *entry) int {
				return cmp.Or(cmp.Compare(a.item.Year, b.item.Year), cmp.Compare(a.item.norm, b.item.norm))
			})
		} else {
			rng.Shuffle(len(g.movies), func(i, j int) { g.movies[i], g.movies[j] = g.movies[j], g.movies[i] })
		}
		home.Rows = append(home.Rows, row(snap, kind, *g, "/explorar?"+url.Values{facet: {g.value}}.Encode()))
	}
	add(RowDecade, FacetDecade, 6, false, func(e *entry) ([]string, []string) {
		if e.item.Year == 0 {
			return nil, nil
		}
		d := strconv.Itoa(e.item.Year / 10 * 10)
		return []string{d}, []string{d + "s"}
	})
	add(RowDirector, FacetDirector, 3, true, func(e *entry) (vals, labels []string) {
		for i, id := range e.item.directorIDs {
			if id > 0 {
				vals, labels = append(vals, strconv.Itoa(id)), append(labels, e.item.Directors[i])
			}
		}
		return
	})
	add(RowCountry, FacetCountry, 6, false, func(e *entry) ([]string, []string) {
		return e.item.Countries, e.item.Countries
	})
	add(RowGenre, FacetGenre, 6, false, func(e *entry) ([]string, []string) {
		return e.item.genres, e.item.genres
	})
	add(RowCollection, FacetCollection, 2, true, func(e *entry) ([]string, []string) {
		if e.item.collectionID == 0 {
			return nil, nil
		}
		return []string{strconv.Itoa(e.item.collectionID)}, []string{e.item.collection}
	})

	// The lists changed last, with their movies added last first.
	lists := slices.Clone(snap.Lists)
	slices.SortStableFunc(lists, func(a, b store.List) int {
		return cmp.Or(-cmp.Compare(a.UpdatedAt, b.UpdatedAt), cmp.Compare(a.ID, b.ID))
	})
	shown := 0
	for _, l := range lists {
		if shown == maxListRows {
			break
		}
		id := strconv.FormatInt(l.ID, 10)
		var in []*entry
		for _, e := range movies {
			if _, ok := e.item.addedTo(id); ok {
				in = append(in, e)
			}
		}
		if len(in) < minListRow {
			continue
		}
		slices.SortStableFunc(in, func(a, b *entry) int {
			ta, _ := a.item.addedTo(id)
			tb, _ := b.item.addedTo(id)
			return cmp.Or(-cmp.Compare(ta, tb), cmp.Compare(a.item.norm, b.item.norm))
		})
		home.Rows = append(home.Rows, row(snap, RowList, group{value: id, label: l.Name, movies: in},
			"/explorar?"+url.Values{FacetList: {id}}.Encode()))
		shown++
	}
	rng.Shuffle(len(home.Rows), func(i, j int) { home.Rows[i], home.Rows[j] = home.Rows[j], home.Rows[i] })
	return home
}

func row(snap store.Snapshot, kind string, g group, href string) HomeRow {
	r := HomeRow{Kind: kind, Value: g.value, Label: g.label, Href: href, Items: []HomeItem{}}
	for _, e := range g.movies[:min(rowSize, len(g.movies))] {
		it := e.item
		m := snap.Movies[it.TMDBID]
		r.Items = append(r.Items, HomeItem{TMDBID: it.TMDBID, Title: it.Title, OriginalTitle: it.OriginalTitle,
			Year: it.Year, Directors: it.Directors, Countries: it.Countries,
			Backdrop: images.Version(m.BackdropPath), Poster: it.Poster})
	}
	return r
}
