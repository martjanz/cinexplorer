package identify

import (
	"sort"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// castLimit is how many cast members are kept, in billing order.
const castLimit = 10

// movieFrom converts TMDB details fetched in lang into a stored movie.
func movieFrom(d tmdb.Details, lang string) store.Movie {
	m := store.Movie{
		TMDBID: d.ID, Title: d.Title, OriginalTitle: d.OriginalTitle, Year: d.Year(), Runtime: d.Runtime,
		OriginalLang: d.OriginalLanguage, Overview: d.Overview, PosterPath: d.PosterPath,
		BackdropPath: d.BackdropPath, IMDbID: d.IMDbID, Language: lang,
		Directors: []store.Person{}, Cast: []store.CastMember{}, Genres: []string{}, Countries: []string{},
	}
	for _, p := range d.Directors() {
		m.Directors = append(m.Directors, store.Person{ID: p.ID, Name: p.Name})
	}
	cast := append([]tmdb.Person(nil), d.Credits.Cast...)
	sort.SliceStable(cast, func(i, j int) bool { return cast[i].Order < cast[j].Order })
	for _, p := range cast[:min(castLimit, len(cast))] {
		m.Cast = append(m.Cast, store.CastMember{ID: p.ID, Name: p.Name, Character: p.Character})
	}
	for _, g := range d.Genres {
		m.Genres = append(m.Genres, g.Name)
	}
	for _, c := range d.ProductionCountries {
		m.Countries = append(m.Countries, c.ISO)
	}
	if d.Collection != nil {
		m.CollectionID, m.Collection = d.Collection.ID, d.Collection.Name
	}
	return m
}

// fillFromWikidata completes the fields TMDB left empty and marks the movie
// as looked up.
func fillFromWikidata(m *store.Movie, e wikidata.Entity) {
	m.WikidataDone = true
	if e.QID != "" {
		m.WikidataID = e.QID
	}
	if m.IMDbID == "" {
		m.IMDbID = e.IMDbID
	}
	if len(m.Countries) == 0 && len(e.Countries) > 0 {
		m.Countries = e.Countries
	}
	if len(m.Directors) == 0 {
		for _, name := range e.Directors {
			m.Directors = append(m.Directors, store.Person{Name: name})
		}
	}
	if m.Year == 0 {
		m.Year = e.Year
	}
}
