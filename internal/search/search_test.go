package search

import (
	"slices"
	"testing"
)

var docs = []Doc{
	{Key: "movie:1", Title: "El ángel exterminador", Original: "El ángel exterminador", Directors: "Luis Buñuel", Cast: "Silvia Pinal"},
	{Key: "movie:2", Title: "Mujeres al borde de un ataque de nervios", Directors: "Pedro Almodóvar", Cast: "Carmen Maura, Antonio Banderas"},
	{Key: "movie:3", Title: "La piel que habito", Directors: "Pedro Almodóvar", Cast: "Antonio Banderas"},
	{Key: "movie:4", Title: "Banderas rojas", Directors: "Otro"},
	{Key: "f123", Title: "Rip mentecato", Files: "Rip mentecato"},
	{Key: "movie:5", Title: "8½", Original: "Otto e mezzo", Directors: "Federico Fellini", Files: "Fellini 8 y medio"},
}

func index(t *testing.T) *Index {
	t.Helper()
	x, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { x.Close() })
	if err := x.Refresh(1, func() []Doc { return docs }); err != nil {
		t.Fatal(err)
	}
	return x
}

func TestQuery(t *testing.T) {
	x := index(t)
	for q, want := range map[string][]string{
		"angel":              {"movie:1"},
		"ÁNGEL EXTER":        {"movie:1"},
		"bunu":               {"movie:1"},
		"almodóvar piel":     {"movie:3"},
		"mentecato":          {"f123"},
		"otto mezzo":         {"movie:5"},
		"8":                  nil, // too short
		"a":                  nil,
		`"; DROP -x:y* OR (`: nil, // FTS5 syntax is only words here
		"nada que ver":       nil,
		"felli":              {"movie:5"},
	} {
		got, err := x.Query(q)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("Query(%q) = %v, %v; want %v", q, got, err, want)
		}
	}
}

func TestQueryRanksTitleFirst(t *testing.T) {
	x := index(t)
	got, err := x.Query("banderas")
	if err != nil || len(got) != 3 || got[0] != "movie:4" || !slices.Contains(got, "movie:2") || !slices.Contains(got, "movie:3") {
		t.Fatalf("got %v, %v", got, err)
	}
	got, _ = x.Query("almodovar")
	slices.Sort(got)
	if !slices.Equal(got, []string{"movie:2", "movie:3"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRefreshOnlyWhenChanged(t *testing.T) {
	x := index(t)
	calls := 0
	more := func() []Doc { calls++; return append(slices.Clone(docs), Doc{Key: "movie:9", Title: "Stalker"}) }
	if err := x.Refresh(1, more); err != nil || calls != 0 {
		t.Fatalf("rebuilt with the same counter (%v)", err)
	}
	if got, _ := x.Query("stalker"); got != nil {
		t.Fatalf("got %v", got)
	}
	if err := x.Refresh(2, more); err != nil || calls != 1 {
		t.Fatalf("not rebuilt (%v)", err)
	}
	if got, _ := x.Query("stalker"); !slices.Equal(got, []string{"movie:9"}) {
		t.Fatalf("got %v", got)
	}
	if got, _ := x.Query("almodovar"); len(got) != 2 {
		t.Fatalf("rebuilt twice the same docs? %v", got)
	}
}
