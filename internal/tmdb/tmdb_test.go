package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New("secret")
	c.BaseURL, c.ImageURL = srv.URL+"/3", srv.URL+"/img"
	c.HTTP.Backoff = time.Millisecond
	c.HTTP.Attempts = 2
	return c
}

func TestSearchMovie(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/search/movie" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("request %s %v", r.URL, r.Header)
		}
		q := r.URL.Query()
		if q.Get("query") != "Amarcord" || q.Get("year") != "1973" || q.Get("language") != "es-ES" {
			t.Errorf("query %v", q)
		}
		// The collection is the user's own, so adult titles (e.g. "Deep
		// Throat", TMDB id 5853) must not be filtered out of results.
		if q.Get("include_adult") != "true" {
			t.Errorf("include_adult %v", q)
		}
		w.Write([]byte(`{"page":1,"results":[{"id":7857,"title":"Amarcord","original_title":"Amarcord","release_date":"1973-12-18","poster_path":"/a.jpg"}]}`))
	})
	rs, err := c.SearchMovie(context.Background(), "Amarcord", 1973, "es-ES")
	if err != nil || len(rs) != 1 || rs[0].ID != 7857 || rs[0].Year() != 1973 || rs[0].PosterPath != "/a.jpg" {
		t.Fatalf("got %+v, %v", rs, err)
	}
}

func TestSearchWithoutYear(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("year") {
			t.Errorf("year sent: %v", r.URL.Query())
		}
		w.Write([]byte(`{"results":[]}`))
	})
	if rs, err := c.SearchMovie(context.Background(), "X", 0, "es-ES"); err != nil || len(rs) != 0 {
		t.Fatalf("got %+v, %v", rs, err)
	}
}

func TestFindIMDb(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/find/tt0071129" || r.URL.Query().Get("external_source") != "imdb_id" {
			t.Errorf("request %s", r.URL)
		}
		w.Write([]byte(`{"movie_results":[{"id":7857,"title":"Amarcord","release_date":"1973-12-18"}],"tv_results":[]}`))
	})
	rs, err := c.FindIMDb(context.Background(), "tt0071129", "es-ES")
	if err != nil || len(rs) != 1 || rs[0].ID != 7857 {
		t.Fatalf("got %+v, %v", rs, err)
	}
}

func TestMovie(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/movie/7857" || r.URL.Query().Get("append_to_response") != "credits" {
			t.Errorf("request %s", r.URL)
		}
		w.Write([]byte(`{"id":7857,"title":"Amarcord","original_title":"Amarcord","release_date":"1973-12-18",
			"runtime":123,"original_language":"it","overview":"Rimini.","imdb_id":"tt0071129",
			"poster_path":"/p.jpg","backdrop_path":"/b.jpg","genres":[{"id":35,"name":"Comedia"}],
			"production_countries":[{"iso_3166_1":"IT","name":"Italy"},{"iso_3166_1":"FR","name":"France"}],
			"belongs_to_collection":null,
			"credits":{"cast":[{"id":1,"name":"Magali Noël","character":"Gradisca","order":0}],
			"crew":[{"id":4415,"name":"Federico Fellini","job":"Director"},{"id":4415,"name":"Federico Fellini","job":"Director"},
			{"id":9,"name":"Tonino Guerra","job":"Screenplay"}]}}`))
	})
	d, err := c.Movie(context.Background(), 7857, "es-ES")
	if err != nil {
		t.Fatal(err)
	}
	if d.Year() != 1973 || d.Runtime != 123 || d.IMDbID != "tt0071129" || len(d.Genres) != 1 ||
		len(d.ProductionCountries) != 2 || d.Collection != nil || len(d.Credits.Cast) != 1 {
		t.Fatalf("got %+v", d)
	}
	if ds := d.Directors(); len(ds) != 1 || ds[0].Name != "Federico Fellini" {
		t.Fatalf("directors %+v", ds)
	}
}

func TestErrors(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/movie/1":
			w.WriteHeader(http.StatusNotFound)
		case "/3/movie/2":
			w.WriteHeader(http.StatusUnauthorized)
		case "/3/movie/3":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.Write([]byte(`not json`))
		}
	})
	ctx := context.Background()
	if _, err := c.Movie(ctx, 1, "es-ES"); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	if _, err := c.Movie(ctx, 2, "es-ES"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("401: %v", err)
	}
	if _, err := c.Movie(ctx, 3, "es-ES"); !errors.Is(err, ErrOffline) {
		t.Errorf("503: %v", err)
	}
	if _, err := c.Movie(ctx, 4, "es-ES"); err == nil || errors.Is(err, ErrOffline) {
		t.Errorf("bad JSON: %v", err)
	}
}

func TestImage(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/img/w342/p.jpg" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Write([]byte("JPEG"))
	})
	if b, err := c.Image(context.Background(), "/p.jpg", "w342"); err != nil || string(b) != "JPEG" {
		t.Fatalf("got %q, %v", b, err)
	}
}

func TestTranslations(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/movie/7857/translations" {
			t.Errorf("request %s", r.URL)
		}
		w.Write([]byte(`{"id":7857,"translations":[{"iso_3166_1":"MX","iso_639_1":"es","name":"Español","english_name":"Spanish","data":{"homepage":"","overview":"Rimini.","runtime":123,"tagline":"","title":"Amarcord"}}]}`))
	})
	ts, err := c.Translations(context.Background(), 7857)
	if err != nil || len(ts) != 1 || ts[0].Tag() != "es-MX" || ts[0].Data.Title != "Amarcord" || ts[0].Data.Overview != "Rimini." {
		t.Fatalf("got %+v, %v", ts, err)
	}
}
