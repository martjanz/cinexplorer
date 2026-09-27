package catalog

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"cinexplorer/internal/store"
)

// homeSnapshot has 14 movies: 1970–1976 and 1980–1986; the first 8 by
// director 1 (the 1970s ones and 1980), countries alternate IT and FR, all
// Drama, and two in collection 5. Plus an unidentified version.
func homeSnapshot() store.Snapshot {
	snap := store.Snapshot{Identifications: map[string]*store.Identification{}, Movies: map[int]store.Movie{}}
	years := []int{1970, 1971, 1972, 1973, 1974, 1975, 1976, 1980, 1981, 1982, 1983, 1984, 1985, 1986}
	for i, y := range years {
		id := 100 + i
		fp := fmt.Sprintf("f%d", i)
		v := identified(version(int64(i+1), "../cine", fp+".mkv", fp, "x", y, "1080p", 10, int64(1000+i)), store.StatusAuto, id)
		snap.Versions = append(snap.Versions, v)
		m := store.Movie{TMDBID: id, Title: fmt.Sprintf("Película %02d", i), Year: y, Genres: []string{"Drama"},
			Countries: []string{[]string{"IT", "FR"}[i%2]}, BackdropPath: fmt.Sprintf("/b%d.jpg", i), PosterPath: "/p.jpg"}
		if i < 8 {
			m.Directors = []store.Person{{ID: 1, Name: "Director Uno"}}
		} else {
			m.Directors = []store.Person{{ID: 0, Name: "Sin id"}} // from Wikidata: not a facet
		}
		if i == 3 || i == 9 {
			m.CollectionID, m.Collection = 5, "Saga"
		}
		snap.Movies[id] = m
	}
	snap.Versions = append(snap.Versions, version(99, "../cine", "Stalker.avi", "s1", "Stalker", 1979, "", 10, 5000))
	return snap
}

func TestHomePage(t *testing.T) {
	h := HomePage(homeSnapshot(), 1)
	if h.Total != 14 {
		t.Fatalf("total %d", h.Total)
	}
	var kinds []string
	rows := map[string]HomeRow{}
	for _, r := range h.Rows {
		kinds = append(kinds, r.Kind)
		rows[r.Kind] = r
	}
	if want := []string{RowRecent, RowDecade, RowDirector, RowCountry, RowGenre, RowCollection}; !slices.Equal(kinds, want) {
		t.Fatalf("rows %v", kinds)
	}
	recent := rows[RowRecent]
	if recent.Href != "/explorar?orden=agregado" || len(recent.Items) != 14 || recent.Items[0].TMDBID != 113 || recent.Items[13].TMDBID != 100 {
		t.Fatalf("recent %+v", recent)
	}
	if it := recent.Items[0]; it.Backdrop != "b13" || it.Poster != "p" || it.Year != 1986 || !slices.Equal(it.Countries, []string{"FR"}) {
		t.Fatalf("item %+v", it)
	}
	if d := rows[RowDecade]; (d.Value != "1970" && d.Value != "1980") || d.Label != d.Value+"s" || d.Href != "/explorar?decada="+d.Value || len(d.Items) != 7 {
		t.Fatalf("decade %+v", d)
	}
	dir := rows[RowDirector]
	if dir.Value != "1" || dir.Label != "Director Uno" || dir.Href != "/explorar?director=1" || len(dir.Items) != 8 || dir.Items[0].Year != 1970 || dir.Items[7].Year != 1980 {
		t.Fatalf("director %+v", dir)
	}
	if c := rows[RowCountry]; (c.Value != "IT" && c.Value != "FR") || c.Label != c.Value || len(c.Items) != 7 {
		t.Fatalf("country %+v", c)
	}
	if g := rows[RowGenre]; g.Value != "Drama" || g.Href != "/explorar?genero=Drama" || len(g.Items) != 14 {
		t.Fatalf("genre %+v", g)
	}
	if c := rows[RowCollection]; c.Value != "5" || c.Label != "Saga" || c.Href != "/explorar?coleccion=5" ||
		len(c.Items) != 2 || c.Items[0].Year != 1973 {
		t.Fatalf("collection %+v", c)
	}
}

func TestHomePageSeed(t *testing.T) {
	snap := homeSnapshot()
	if a, b := HomePage(snap, 7), HomePage(snap, 7); !reflect.DeepEqual(a, b) {
		t.Fatal("the same seed gave other rows")
	}
	// Some seed picks another decade or shuffles the genre row otherwise.
	first := HomePage(snap, 0)
	for seed := uint64(1); seed < 20; seed++ {
		if !reflect.DeepEqual(HomePage(snap, seed), first) {
			return
		}
	}
	t.Fatal("every seed gave the same rows")
}

func TestHomePageMinimums(t *testing.T) {
	snap := homeSnapshot()
	for id := 106; id < 114; id++ { // keep 6 movies: 1970–1975
		delete(snap.Movies, id)
	}
	h := HomePage(snap, 1)
	var kinds []string
	for _, r := range h.Rows {
		kinds = append(kinds, r.Kind)
	}
	// 6 by director 1 (≥3); 3 per country (<6); 1 in the collection (<2).
	if want := []string{RowRecent, RowDecade, RowDirector, RowGenre}; !slices.Equal(kinds, want) || h.Total != 6 {
		t.Fatalf("rows %v total %d", kinds, h.Total)
	}
	if h := HomePage(store.Snapshot{}, 1); h.Total != 0 || h.Rows == nil || len(h.Rows) != 0 {
		t.Fatalf("empty: %+v", h)
	}
}
