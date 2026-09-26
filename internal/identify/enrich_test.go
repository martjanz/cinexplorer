package identify

import (
	"testing"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

func TestMovieFromAndWikidata(t *testing.T) {
	d := details(7857, "Amarcord", "1973-12-18", "Rimini.", "Federico Fellini", "Federico Fellini")
	d.Credits.Crew[1].ID = d.Credits.Crew[0].ID
	for i := range 12 {
		d.Credits.Cast = append(d.Credits.Cast, tmdb.Person{ID: i, Name: "Actor", Order: 11 - i})
	}
	d.Collection = &struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{9, "Col"}
	m := movieFrom(d, "es-ES")
	if len(m.Directors) != 1 || len(m.Cast) != castLimit || m.Cast[0].ID != 11 || m.CollectionID != 9 || m.Year != 1973 ||
		m.Genres == nil || m.Countries == nil {
		t.Fatalf("got %+v", m)
	}
	fillFromWikidata(&m, wikidata.Entity{QID: "Q1", IMDbID: "tt1", Countries: []string{"IT"}, Directors: []string{"X"}, Year: 1900})
	if !m.WikidataDone || m.WikidataID != "Q1" || m.IMDbID != "tt1" || m.Countries[0] != "IT" ||
		m.Directors[0].Name != "Federico Fellini" || m.Year != 1973 {
		t.Fatalf("filled %+v", m)
	}
	empty := store.Movie{}
	fillFromWikidata(&empty, wikidata.Entity{})
	if !empty.WikidataDone || empty.WikidataID != "" {
		t.Fatalf("no entity %+v", empty)
	}
}
