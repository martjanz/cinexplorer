package catalog

import (
	"testing"

	"cinexplorer/internal/quality"
	"cinexplorer/internal/search"
	"cinexplorer/internal/store"
)

func TestSearchDocs(t *testing.T) {
	snap := snapshot()
	m := snap.Movies[7857]
	m.Cast = []store.CastMember{{ID: 1, Name: "Magali Noël"}, {ID: 2, Name: "Bruno Zanin"}}
	snap.Movies[7857] = m
	docs := SearchDocs(snap)
	byKey := map[string]search.Doc{}
	var order []string
	for _, d := range docs {
		byKey[d.Key] = d
		order = append(order, d.Title)
	}
	want := search.Doc{Key: "movie:7857", Title: "Amarcord", Original: "Amarcord", Directors: "Federico Fellini",
		Cast: "Magali Noël, Bruno Zanin", Files: "Amarcord"}
	if byKey["movie:7857"] != want {
		t.Errorf("amarcord %+v", byKey["movie:7857"])
	}
	if d := byKey["n1"]; d.Title != "Novecento" || d.Directors != "Bertolucci" || d.Original != "" {
		t.Errorf("novecento %+v", d)
	}
	if _, ok := byKey["x1"]; ok {
		t.Error("a version that is not a movie is searchable")
	}
	if _, ok := byKey["g1"]; ok {
		t.Error("a missing version is searchable")
	}
	for i := 1; i < len(docs); i++ {
		if quality.NormTitle(order[i-1]) > quality.NormTitle(order[i]) {
			t.Errorf("not in title order: %v", order)
			break
		}
	}
}

func TestDirectors(t *testing.T) {
	all := Items(snapshot(), roots)
	got := Directors(all, all, search.Terms("fell"), 5)
	if len(got) != 1 || got[0] != (DirectorHit{4415, "Federico Fellini", 2}) {
		t.Fatalf("got %+v", got)
	}
	for _, q := range []string{"federico x", "ellini", "bertolucci"} { // Bertolucci is only parsed
		if got := Directors(all, all, search.Terms(q), 5); len(got) != 0 {
			t.Errorf("%q: %+v", q, got)
		}
	}
	if got := Directors(all, all, nil, 5); got == nil || len(got) != 0 {
		t.Errorf("no terms: %#v", got)
	}
}
