package server

import (
	"net/http"
	"testing"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
)

type searchResult struct {
	Q         string                `json:"q"`
	Total     int                   `json:"total"`
	Items     []catalog.Item        `json:"items"`
	Directors []catalog.DirectorHit `json:"directors"`
}

func TestSearchCatalog(t *testing.T) {
	s, _ := identifyServer(t)
	var res searchResult
	// Not identified yet: found by the title of its file name.
	if code := getJSON(t, s, "/api/search?q=amarc", &res); code != http.StatusOK || res.Total != 1 ||
		res.Items[0].Kind != catalog.KindVersion || res.Items[0].Key != "f1" {
		t.Fatalf("%d %+v", code, res)
	}
	// Identified: the index follows the catalog.
	s.rt().Identifier.Adopt(t.Context(), 7857)
	if err := s.Store.SetCorrection("f1", store.StatusManual, 7857); err != nil {
		t.Fatal(err)
	}
	m, _, _ := s.Store.Movie(7857)
	m.Directors = []store.Person{{ID: 4415, Name: "Federico Fellini"}}
	if err := s.Store.SaveMovie(m); err != nil {
		t.Fatal(err)
	}
	if code := getJSON(t, s, "/api/search?q=FELLÍNI", &res); code != http.StatusOK || res.Total != 1 ||
		res.Items[0].TMDBID != 7857 || len(res.Directors) != 1 || res.Directors[0].Count != 1 {
		t.Fatalf("%d %+v", code, res)
	}
	if code := getJSON(t, s, "/api/search?q=a", &res); code != http.StatusOK || res.Total != 0 || res.Items == nil || res.Directors == nil {
		t.Fatalf("short query: %d %+v", code, res)
	}
}

func TestSearchLimit(t *testing.T) {
	s, _ := newServer(t)
	// Two contents titled Amarcord: two items.
	other := "../cine/Amarcord.avi"
	if err := s.Store.SyncFiles([]store.FileRow{
		{Path: moviePath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"},
		{Path: other, Size: 20, MTime: 1, Fingerprint: "f2", Kind: "video"},
	}, []string{"../cine"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ReplaceVersions([]grouping.Version{
		{Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
			Members: []grouping.Member{{Path: moviePath, Role: grouping.RoleMain}}},
		{Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 20, Parts: 1,
			Members: []grouping.Member{{Path: other, Role: grouping.RoleMain}}},
	}); err != nil {
		t.Fatal(err)
	}
	var res searchResult
	getJSON(t, s, "/api/search?q=amarcord&limit=1", &res)
	if res.Total != 2 || len(res.Items) != 1 {
		t.Fatalf("got %+v", res)
	}
	getJSON(t, s, "/api/search?q=amarcord&limit=0", &res) // out of range: the default
	if len(res.Items) != 2 {
		t.Fatalf("got %+v", res)
	}
}
