package identify

import (
	"context"
	"slices"
	"testing"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

func TestChain(t *testing.T) {
	for lang, want := range map[string][]string{
		"es-AR": {"es-AR", "es-MX", "es-ES"},
		"es-ES": {"es-ES", "es-MX"},
		"pt-BR": {"pt-BR", "pt-PT"},
		"en-US": {"en-US", "en-GB"},
		"fr-FR": {"fr-FR"},
	} {
		if got := Chain(lang); !slices.Equal(got, want) {
			t.Errorf("Chain(%s) = %v, want %v", lang, got, want)
		}
	}
}

func TestTranslate(t *testing.T) {
	ts := []tmdb.Translation{
		tr("en-US", "The Night", "A day and a night."),
		tr("es-ES", "La noche", "Un día y una noche."),
		tr("es-MX", "", "Un día y una noche en Milán."),
		tr("es-AR", "", ""),
		tr("it-IT", "La notte", "Un giorno."),
	}
	for _, tc := range []struct {
		lang, title, overview   string // what TMDB returned in lang
		wantTitle, wantOverview string
	}{
		// es-AR has nothing: the title from es-ES (es-MX has none), the overview from es-MX.
		{"es-AR", "The Night", "", "La noche", "Un día y una noche en Milán."},
		{"es-ES", "La noche", "Un día y una noche.", "La noche", "Un día y una noche."},
		// No Portuguese at all: the title stays as TMDB gave it, the overview in English.
		{"pt-BR", "La notte", "", "La notte", "A day and a night."},
		// An empty title falls back to English too.
		{"pt-BR", "", "", "The Night", "A day and a night."},
	} {
		m := store.Movie{Title: tc.title, Overview: tc.overview}
		translate(&m, ts, tc.lang)
		if m.Title != tc.wantTitle || m.Overview != tc.wantOverview {
			t.Errorf("%s: got %q / %q", tc.lang, m.Title, m.Overview)
		}
	}
	// Any variant of the language comes after the chain.
	m := store.Movie{}
	translate(&m, []tmdb.Translation{tr("es-CO", "La noche (CO)", "")}, "es-AR")
	if m.Title != "La noche (CO)" {
		t.Errorf("other variant: %q", m.Title)
	}
}

func TestFetchMovieTranslations(t *testing.T) {
	r, api, _ := fixture(t)
	api.movies["7857|es-AR"] = details(7857, "Amarcord", "1973-12-18", "", "Federico Fellini")
	api.trans = map[int][]tmdb.Translation{7857: {tr("es-MX", "Amarcord: Mis recuerdos", "Rimini en los años 30.")}}
	r.Language = "es-AR"
	m, err := r.fetchMovie(context.Background(), 7857, nil)
	if err != nil || m.Title != "Amarcord: Mis recuerdos" || m.Overview != "Rimini en los años 30." || m.Language != "es-AR" {
		t.Fatalf("got %+v, %v", m, err)
	}
	// English needs no translations.
	api.movies["7857|en-US"] = details(7857, "Amarcord", "1973-12-18", "", "Federico Fellini")
	r.Language = "en-US"
	before := api.called("translations ")
	if _, err := r.fetchMovie(context.Background(), 7857, nil); err != nil || api.called("translations ") != before {
		t.Fatalf("en-US asked for translations (%v)", err)
	}
}
