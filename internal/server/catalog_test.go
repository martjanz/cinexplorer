package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
)

// addCopy puts an identical copy of Amarcord (fingerprint f1) in
// ../cine-ordenar, as a second version.
func addCopy(t *testing.T, s *Server) {
	t.Helper()
	s.rt().Config.Roots = []string{"../cine", "../cine-ordenar"}
	const copyPath = "../cine-ordenar/Amarcord.mkv"
	if err := s.Store.SyncFiles([]store.FileRow{
		{Path: moviePath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"},
		{Path: copyPath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"},
	}, s.rt().Config.Roots); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ReplaceVersions([]grouping.Version{
		{Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
			Members: []grouping.Member{{Path: moviePath, Role: grouping.RoleMain}}},
		{Dir: "../cine-ordenar", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
			Members: []grouping.Member{{Path: copyPath, Role: grouping.RoleMain}}},
	}); err != nil {
		t.Fatal(err)
	}
}

func getJSON(t *testing.T, s *Server, url string, v any) int {
	t.Helper()
	rec := request(s.Handler(), "GET", url, "", "", "127.0.0.1")
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("%s: %s (%v)", url, rec.Body, err)
		}
	}
	return rec.Code
}

type exploreBody struct {
	Total  int                             `json:"total"`
	Query  catalog.Query                   `json:"query"`
	Items  []catalog.Item                  `json:"items"`
	Facets map[string][]catalog.FacetValue `json:"facets"`
}

func TestExploreEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	addCopy(t, s)
	var b exploreBody
	if code := getJSON(t, s, "/api/explore", &b); code != 200 {
		t.Fatalf("status %d", code)
	}
	if b.Total != 1 || len(b.Items) != 1 || b.Items[0].Kind != catalog.KindVersion || b.Items[0].Key != "f1" ||
		b.Items[0].Versions != 2 || b.Query.Order != catalog.OrderYear || len(b.Facets) != len(catalog.FacetNames) {
		t.Fatalf("explore %+v", b)
	}
	// Once identified it is a movie; facets filter and are counted.
	if code := post(t, s, `{"fingerprint":"f1","action":"movie","tmdbId":7857}`); code != http.StatusNoContent {
		t.Fatalf("identify %d", code)
	}
	b = exploreBody{}
	getJSON(t, s, "/api/explore?decada=1970&estado=copia-identica&orden=titulo&foo=1&pais=zz", &b)
	if b.Total != 1 || b.Items[0].Kind != catalog.KindMovie || b.Items[0].TMDBID != 7857 || b.Items[0].Poster != "p" {
		t.Fatalf("filtered %+v", b)
	}
	// Malformed facets are dropped, which the page sees in query.
	if len(b.Query.Facets) != 2 || b.Query.Order != catalog.OrderTitle || b.Query.Dir != catalog.Asc {
		t.Fatalf("query %+v", b.Query)
	}
	if loc := b.Facets["ubicacion"]; len(loc) != 2 || loc[0].Value != "cine" || loc[0].Count != 1 {
		t.Fatalf("ubicacion %+v", loc)
	}
	b = exploreBody{}
	getJSON(t, s, "/api/explore?decada=1980", &b)
	if b.Total != 0 || b.Items == nil {
		t.Fatalf("no match %+v", b)
	}
}

func TestMovieEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	addCopy(t, s)
	var d catalog.MovieDetail
	if code := getJSON(t, s, "/api/movies/7857", &d); code != http.StatusNotFound {
		t.Fatalf("not stored yet: %d", code)
	}
	post(t, s, `{"fingerprint":"f1","action":"movie","tmdbId":7857}`)
	if code := getJSON(t, s, "/api/movies/7857", &d); code != 200 {
		t.Fatalf("status %d", code)
	}
	if d.Movie.Title != "Amarcord" || d.Movie.Overview != "Rimini." || d.Poster != "p" || d.Backdrop != "b" ||
		len(d.Versions) != 2 || d.Versions[0].Copies != 2 {
		t.Fatalf("movie %+v", d)
	}
	for _, bad := range []string{"/api/movies/abc", "/api/movies/0", "/api/movies/5"} {
		if code := getJSON(t, s, bad, &d); code != http.StatusNotFound {
			t.Errorf("%s: %d", bad, code)
		}
	}
}

func TestVersionEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	s.Store.SaveIdentifications([]store.Identification{{Fingerprint: "f1", Status: store.StatusUnmatched,
		Candidates: []store.Candidate{{TMDBID: 7857, Title: "Amarcord", Score: 0.7}}}})
	var d catalog.VersionDetail
	if code := getJSON(t, s, "/api/versions/f1", &d); code != 200 {
		t.Fatalf("status %d", code)
	}
	if d.Title != "Amarcord" || len(d.Versions) != 1 || d.Identification == nil || len(d.Identification.Candidates) != 1 {
		t.Fatalf("version %+v", d)
	}
	for _, bad := range []string{"/api/versions/zz", "/api/versions/id:99"} {
		if code := getJSON(t, s, bad, &d); code != http.StatusNotFound {
			t.Errorf("%s: %d", bad, code)
		}
	}
}

func TestMoviesEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	addCopy(t, s)
	var refs []store.MovieRef
	if getJSON(t, s, "/api/movies?q=amarcord", &refs); len(refs) != 0 {
		t.Fatalf("before identifying: %+v", refs)
	}
	s.rt().Identifier.Adopt(context.Background(), 7857)
	s.Store.SetCorrection("f1", store.StatusManual, 7857)
	if getJSON(t, s, "/api/movies?q=AMARCORD&near=f1", &refs); len(refs) != 1 || refs[0].TMDBID != 7857 {
		t.Fatalf("suggestions %+v", refs)
	}
}

func TestHomeEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	var h struct {
		catalog.Home
		TileSize string
	}
	if code := getJSON(t, s, "/api/home?seed=3", &h); code != 200 || h.Total != 0 || h.Rows == nil || h.TileSize != "medium" {
		t.Fatalf("%d %+v", code, h)
	}
	s.rt().Identifier.Adopt(context.Background(), 7857)
	if err := s.Store.SetCorrection("f1", store.StatusManual, 7857); err != nil {
		t.Fatal(err)
	}
	if code := getJSON(t, s, "/api/home?seed=x", &h); code != 200 || h.Total != 1 || len(h.Rows) != 1 ||
		h.Rows[0].Kind != catalog.RowRecent || h.Rows[0].Items[0].Backdrop != "b" {
		t.Fatalf("%d %+v", code, h)
	}
}
