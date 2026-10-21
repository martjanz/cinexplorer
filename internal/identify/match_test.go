package identify

import (
	"context"
	"errors"
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

func TestReadingsFindDashTitle(t *testing.T) {
	api := &fakeAPI{
		search: map[string][]tmdb.Result{
			"Azor|2021|es-ES": {{ID: 659954, Title: "Azor", ReleaseDate: "2021-09-10"}},
		},
		movies: map[string]tmdb.Details{
			"659954|es-ES": details(659954, "Azor", "2021-09-10", "", "Andreas Fontana"),
		},
	}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Azor - Andreas Fontana", Year: 2021})
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusAuto || id.TMDBID != 659954 {
		t.Fatalf("got %+v", id)
	}
	if n := api.called("search Andreas Fontana|"); n != 0 {
		t.Fatalf("second reading tried %d times", n)
	}
}

func TestReadingWithoutDirectorStaysForReview(t *testing.T) {
	api := &fakeAPI{
		search: map[string][]tmdb.Result{
			"Azor|2021|es-ES": {{ID: 659954, Title: "Azor", ReleaseDate: "2021-09-10"}},
		},
		movies: map[string]tmdb.Details{
			"659954|es-ES": details(659954, "Azor", "2021-09-10", "", "Otro Nombre"),
		},
	}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Azor - Andreas Fontana", Year: 2021})
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusUnmatched || len(id.Candidates) == 0 || id.Candidates[0].TMDBID != 659954 || id.Confidence < 0.80 {
		t.Fatalf("got %+v", id)
	}
}

func TestWeakMainLiftedByReadingIsGated(t *testing.T) {
	newAPI := func(dir string) *fakeAPI {
		r := []tmdb.Result{{ID: 38251, Title: "Mediterráneo", ReleaseDate: "1991-02-02"}}
		api := &fakeAPI{search: map[string][]tmdb.Result{
			"Mediterraneo G Salvatore|1991|es-ES": r,
			"Mediterraneo|1991|es-ES":             r,
		}}
		if dir != "" {
			api.movies = map[string]tmdb.Details{"38251|es-ES": details(38251, "Mediterráneo", "1991-02-02", "", dir)}
		}
		return api
	}
	q := Query{Title: "Mediterraneo G Salvatore", Year: 1991}
	id, _, err := Identify(context.Background(), newAPI(""), "es-ES", q)
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusUnmatched || id.Candidates[0].TMDBID != 38251 {
		t.Fatalf("without director: %+v", id)
	}
	id, _, err = Identify(context.Background(), newAPI("Gabriele Salvatore"), "es-ES", q)
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusAuto || id.TMDBID != 38251 {
		t.Fatalf("with director: %+v", id)
	}
}

func TestReadingsNotTriedWhenConclusive(t *testing.T) {
	api := &fakeAPI{search: map[string][]tmdb.Result{
		"Ju-On - The Grudge|2003|es-ES": {{ID: 1, Title: "Ju-On - The Grudge", ReleaseDate: "2003-01-01"}},
	}}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Ju-On - The Grudge", Year: 2003})
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusAuto || api.called("search Ju-On|") != 0 || api.called("search The Grudge|") != 0 {
		t.Fatalf("got %+v calls %q", id, api.calls)
	}
}

func TestSearchDoesNotTryReadings(t *testing.T) {
	api := &fakeAPI{}
	if _, _, err := Search(context.Background(), api, "es-ES", Query{Title: "Azor - Andreas Fontana"}); err != nil {
		t.Fatal(err)
	}
	if api.called("search Azor|") != 0 || api.called("search Azor - Andreas Fontana|") == 0 {
		t.Fatalf("calls %q", api.calls)
	}
}

func TestReadingErrorSkipsReading(t *testing.T) {
	api := &fakeAPI{
		fail: map[string]error{"Azor|2021|es-ES": errors.New("500")},
		search: map[string][]tmdb.Result{
			"Andreas Fontana|2021|es-ES": {{ID: 1, Title: "Otra", ReleaseDate: "2021-01-01"}},
		},
	}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Azor - Andreas Fontana", Year: 2021})
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusUnmatched || len(id.Candidates) == 0 || id.Candidates[0].TMDBID != 1 {
		t.Fatalf("got %+v", id)
	}
}

func phoenixAPI() *fakeAPI {
	d1, d2 := details(1525682, "Phoenix", "2014-10-01", ""), details(254578, "Phoenix", "2014-09-25", "")
	d1.Runtime, d2.Runtime = 15, 98
	return &fakeAPI{
		search: map[string][]tmdb.Result{
			"Phoenix|2014|es-ES": {
				{ID: 1525682, Title: "Phoenix", ReleaseDate: "2014-10-01"},
				{ID: 254578, Title: "Phoenix", ReleaseDate: "2014-09-25"},
			},
		},
		movies: map[string]tmdb.Details{"1525682|es-ES": d1, "254578|es-ES": d2},
	}
}

func TestRuntimeOrdersTies(t *testing.T) {
	api := phoenixAPI()
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Phoenix", Year: 2014, Runtime: 97})
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusUnmatched || len(id.Candidates) != 2 || id.Candidates[0].TMDBID != 254578 {
		t.Fatalf("got %+v", id)
	}
	if id.Confidence != 1 {
		t.Fatalf("confidence %v", id.Confidence)
	}

	api = phoenixAPI()
	id, _, err = Identify(context.Background(), api, "es-ES", Query{Title: "Phoenix", Year: 2014})
	if err != nil {
		t.Fatal(err)
	}
	if id.Candidates[0].TMDBID != 1525682 || api.called("movie ") != 0 {
		t.Fatalf("without runtime: %+v, %d movie calls", id.Candidates, api.called("movie "))
	}
}

func TestRuntimeMatches(t *testing.T) {
	for _, c := range []struct {
		runtime, minutes int
		want             bool
	}{{98, 97, true}, {103, 97, true}, {150, 140, true}, {15, 97, false}, {0, 97, false}} {
		if got := runtimeMatches(c.runtime, c.minutes); got != c.want {
			t.Errorf("runtimeMatches(%d, %d) = %v", c.runtime, c.minutes, got)
		}
	}
}

func TestRuntimeStepErrors(t *testing.T) {
	// A removed movie just does not match; a fatal error is returned.
	api := phoenixAPI()
	delete(api.movies, "254578|es-ES")
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Phoenix", Year: 2014, Runtime: 97})
	if err != nil || id.Candidates[0].TMDBID != 1525682 {
		t.Fatalf("got %+v, %v", id, err)
	}
	if _, _, err := Identify(context.Background(), movieFails{phoenixAPI()}, "es-ES", Query{Title: "Phoenix", Year: 2014, Runtime: 97}); err == nil {
		t.Fatal("fatal error swallowed")
	}
}

// movieFails is a fakeAPI whose Movie fails with a fatal error.
type movieFails struct{ *fakeAPI }

func (movieFails) Movie(context.Context, int, string) (tmdb.Details, error) {
	return tmdb.Details{}, tmdb.ErrUnauthorized
}
