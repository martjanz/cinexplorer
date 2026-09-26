package wikidata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLookup(t *testing.T) {
	fixture, err := os.ReadFile("testdata/lookup.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		if !strings.Contains(q, `VALUES ?tmdb { "7857" "550" "999999999" }`) || !strings.Contains(q, `wikibase:language "es,en"`) {
			t.Errorf("query %s", q)
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "cinexplorer/test (") {
			t.Errorf("User-Agent %q", r.Header.Get("User-Agent"))
		}
		w.Write(fixture)
	}))
	defer srv.Close()
	c := New("test")
	c.Endpoint = srv.URL
	c.HTTP.Limiter = nil
	c.HTTP.Backoff = time.Millisecond

	got, err := c.Lookup(context.Background(), []int{7857, 550, 999999999}, "es-ES")
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]Entity{
		7857: {QID: "Q18428", IMDbID: "tt0071129", Countries: []string{"FR", "IT"}, Directors: []string{"Federico Fellini"}, Year: 1973},
		550:  {QID: "Q190050", IMDbID: "tt0137523", Countries: []string{"DE", "US"}, Directors: []string{"David Fincher"}, Year: 1999},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestLookupLimits(t *testing.T) {
	c := New("test")
	if got, err := c.Lookup(context.Background(), nil, "es-ES"); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
	if _, err := c.Lookup(context.Background(), make([]int, BatchSize+1), "es-ES"); err == nil {
		t.Fatal("oversized batch accepted")
	}
}

func TestParseSkipsUnlabelledDirectors(t *testing.T) {
	got, err := parse([]byte(`{"results":{"bindings":[{"tmdb":{"value":"1"},"item":{"value":"http://www.wikidata.org/entity/Q1"},"directorLabel":{"value":"Q999"}}]}}`))
	if err != nil || len(got[1].Directors) != 0 || got[1].QID != "Q1" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestLookupSanitizesLanguage(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("query")
		w.Write([]byte(`{"results":{"bindings":[]}}`))
	}))
	defer srv.Close()
	c := New("test")
	c.Endpoint = srv.URL
	c.HTTP.Limiter = nil
	for lang, want := range map[string]string{"pt-BR": `"pt,en"`, `x" } #`: `"en,en"`, "": `"en,en"`} {
		if _, err := c.Lookup(context.Background(), []int{1}, lang); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, `wikibase:language `+want) {
			t.Errorf("lang %q: query %s", lang, got)
		}
	}
}
