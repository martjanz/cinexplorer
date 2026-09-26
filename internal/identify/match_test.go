package identify

import (
	"context"
	"reflect"
	"testing"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

func TestSearchFallbacks(t *testing.T) {
	api := &fakeAPI{
		search: map[string][]tmdb.Result{
			// Nothing with the year: retried without it.
			"Cries and Whispers|0|es-ES": {{ID: 10238, Title: "Gritos y susurros", OriginalTitle: "Viskningar och rop", ReleaseDate: "1972-03-05"}},
			// The English search names it as the file does.
			"Cries and Whispers|1972|en-US": {{ID: 10238, Title: "Cries and Whispers", ReleaseDate: "1972-03-05"}},
		},
		find: map[string][]tmdb.Result{},
	}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Cries and Whispers", Year: 1972, IMDbID: "tt9999999"})
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusAuto || id.TMDBID != 10238 || id.Candidates[0].Title != "Gritos y susurros" {
		t.Fatalf("got %+v", id)
	}
	want := []string{"find tt9999999", "search Cries and Whispers|1972|es-ES", "search Cries and Whispers|0|es-ES",
		"search Cries and Whispers|1972|en-US"}
	if !reflect.DeepEqual(api.calls, want) {
		t.Fatalf("calls %q", api.calls)
	}
}

func TestSearchDirectorBreaksTies(t *testing.T) {
	api := &fakeAPI{
		search: map[string][]tmdb.Result{"Ordet|0|es-ES": {
			{ID: 262879, Title: "Ordet", ReleaseDate: "1943-01-01"},
			{ID: 48035, Title: "La palabra", OriginalTitle: "Ordet", ReleaseDate: "1955-01-10"},
		}},
		movies: map[string]tmdb.Details{
			"262879|es-ES": details(262879, "Ordet", "1943-01-01", "", "Gustaf Molander"),
			"48035|es-ES":  details(48035, "La palabra", "1955-01-10", "", "Carl Theodor Dreyer"),
		},
	}
	ctx := context.Background()
	id, fetched, err := Identify(ctx, api, "es-ES", Query{Title: "Ordet", Director: "Dreyer"})
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusAuto || id.TMDBID != 48035 || len(fetched) != 2 {
		t.Fatalf("got %+v fetched %d", id, len(fetched))
	}
	// Without the director it is a tie: left for review with both candidates.
	api.movies = nil
	id, _, _ = Identify(ctx, api, "es-ES", Query{Title: "Ordet"})
	if id.Status != store.StatusUnmatched || len(id.Candidates) != 2 || id.TMDBID != 0 {
		t.Fatalf("got %+v", id)
	}
}

func TestSearchSkipsMissingCredits(t *testing.T) {
	// A candidate whose details TMDB no longer has gets no director share,
	// instead of failing the whole search.
	api := &fakeAPI{
		search: map[string][]tmdb.Result{"Ordet|0|es-ES": {
			{ID: 262879, Title: "Ordet", ReleaseDate: "1943-01-01"},
			{ID: 48035, Title: "La palabra", OriginalTitle: "Ordet", ReleaseDate: "1955-01-10"},
		}},
		movies: map[string]tmdb.Details{"48035|es-ES": details(48035, "La palabra", "1955-01-10", "", "Carl Theodor Dreyer")},
	}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Ordet", Director: "Dreyer"})
	if err != nil || id.Status != store.StatusAuto || id.TMDBID != 48035 {
		t.Fatalf("got %+v, %v", id, err)
	}
}

func TestIdentifyWithoutTitle(t *testing.T) {
	api := &fakeAPI{}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{})
	if err != nil || id.Status != store.StatusUnmatched || len(id.Candidates) != 0 || len(api.calls) != 0 {
		t.Fatalf("got %+v %v calls %v", id, err, api.calls)
	}
}

func TestTitleVariants(t *testing.T) {
	cases := map[string][]string{
		"Amarcord":                         {"Amarcord"},
		"La piel dura (L'argent de poche)": {"La piel dura", "L'argent de poche"},
		"Solyaris [Solaris]":               {"Solyaris", "Solaris"},
		"(Novecento)":                      {"Novecento"},
		"":                                 nil,
	}
	for in, want := range cases {
		if got := titleVariants(in); !reflect.DeepEqual(got, want) {
			t.Errorf("titleVariants(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueryKey(t *testing.T) {
	q := Query{Title: "Amarcord", Year: 1973, Director: "Fellini", IMDbID: "tt0071129"}
	if q.Key() != "Amarcord|1973|Fellini|tt0071129" {
		t.Fatal(q.Key())
	}
}
