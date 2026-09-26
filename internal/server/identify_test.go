package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// fakeTMDB knows Amarcord (7857) and nothing else.
type fakeTMDB struct{ images []string }

func (f *fakeTMDB) SearchMovie(ctx context.Context, q string, year int, lang string) ([]tmdb.Result, error) {
	if strings.EqualFold(q, "amarcord") {
		return []tmdb.Result{{ID: 7857, Title: "Amarcord", ReleaseDate: "1973-12-18", PosterPath: "/p.jpg"}}, nil
	}
	return nil, nil
}

func (f *fakeTMDB) FindIMDb(ctx context.Context, id, lang string) ([]tmdb.Result, error) {
	if id == "tt0071129" {
		return []tmdb.Result{{ID: 7857, Title: "Amarcord", ReleaseDate: "1973-12-18"}}, nil
	}
	return nil, nil
}

func (f *fakeTMDB) Movie(ctx context.Context, id int, lang string) (tmdb.Details, error) {
	if id != 7857 {
		return tmdb.Details{}, tmdb.ErrNotFound
	}
	return tmdb.Details{ID: 7857, Title: "Amarcord", ReleaseDate: "1973-12-18", Overview: "Rimini.",
		PosterPath: "/p.jpg", BackdropPath: "/b.jpg"}, nil
}

func (f *fakeTMDB) Image(ctx context.Context, path, size string) ([]byte, error) {
	f.images = append(f.images, size+path)
	return []byte("\xff\xd8\xff\xe0 jpeg " + path), nil
}

func identifyServer(t *testing.T) (*Server, *fakeTMDB) {
	t.Helper()
	s, _ := newServer(t)
	f := &fakeTMDB{}
	s.TMDB, s.Language = f, "es-ES"
	s.Identifier = &identify.Runner{Store: s.Store, TMDB: f, Language: "es-ES"}
	s.Images = &images.Cache{Dir: t.TempDir(), Fetch: f}
	return s, f
}

func post(t *testing.T, s *Server, body string) int {
	t.Helper()
	return request(s.Handler(), "POST", "/api/identify", body, "application/json", "127.0.0.1").Code
}

func TestIdentifyManualAndVersions(t *testing.T) {
	s, _ := identifyServer(t)
	if code := post(t, s, `{"fingerprint":"f1","action":"movie","tmdbId":7857}`); code != http.StatusNoContent {
		t.Fatalf("status %d", code)
	}
	rec := request(s.Handler(), "GET", "/api/versions", "", "", "127.0.0.1")
	var vs []store.VersionView
	json.Unmarshal(rec.Body.Bytes(), &vs)
	if len(vs) != 1 || vs[0].Fingerprint != "f1" || vs[0].Movie == nil || vs[0].Movie.TMDBID != 7857 ||
		vs[0].Identification.Status != store.StatusManual {
		t.Fatalf("versions %s", rec.Body)
	}
}

func TestIdentifyErrors(t *testing.T) {
	s, _ := identifyServer(t)
	cases := []struct {
		body string
		want int
	}{
		{`{"fingerprint":"f1","action":"movie","tmdbId":5}`, http.StatusNotFound}, // unknown to TMDB
		{`{"fingerprint":"zz","action":"ignore"}`, http.StatusNotFound},           // unknown fingerprint
		{`{"fingerprint":"f1","action":"movie"}`, http.StatusBadRequest},          // no id
		{`{"fingerprint":"f1","action":"delete"}`, http.StatusBadRequest},         // unknown action
		{`{"fingerprint":"f1","action":"ignore"}`, http.StatusNoContent},
		{`{"fingerprint":"f1","action":"extra","tmdbId":7857}`, http.StatusNoContent},
		{`{"fingerprint":"f1","action":"reset"}`, http.StatusNoContent},
		{`not json`, http.StatusBadRequest},
	}
	for _, c := range cases {
		if code := post(t, s, c.body); code != c.want {
			t.Errorf("%s: status %d, want %d", c.body, code, c.want)
		}
	}
	s.ReadOnly = true
	if code := post(t, s, `{"fingerprint":"f1","action":"ignore"}`); code != http.StatusConflict {
		t.Errorf("read-only: %d", code)
	}
}

func TestUnidentifiedEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	s.Store.SaveIdentifications([]store.Identification{{Fingerprint: "f1", Status: store.StatusUnmatched,
		Candidates: []store.Candidate{{TMDBID: 7857, Title: "Amarcord", Score: 0.7}}}})
	rec := request(s.Handler(), "GET", "/api/unidentified", "", "", "127.0.0.1")
	var u []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil || len(u) != 1 || u[0]["dir"] != "../cine" {
		t.Fatalf("body %s", rec.Body)
	}
	if c, ok := u[0]["candidates"].([]any); !ok || len(c) != 1 {
		t.Fatalf("candidates %v", u[0]["candidates"])
	}
}

func TestSearchEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	for _, q := range []string{"/api/tmdb/search?q=Amarcord&year=1973", "/api/tmdb/search?q=tt0071129"} {
		rec := request(s.Handler(), "GET", q, "", "", "127.0.0.1")
		var cs []store.Candidate
		if err := json.Unmarshal(rec.Body.Bytes(), &cs); err != nil || len(cs) != 1 || cs[0].TMDBID != 7857 || cs[0].Score < 0.99 {
			t.Fatalf("%s: %s", q, rec.Body)
		}
	}
	s.TMDB = nil
	if rec := request(s.Handler(), "GET", "/api/tmdb/search?q=x", "", "", "127.0.0.1"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("without token: %d", rec.Code)
	}
}

func TestImageEndpoint(t *testing.T) {
	s, f := identifyServer(t)
	get := func(url string) int { return request(s.Handler(), "GET", url, "", "", "127.0.0.1").Code }
	body := func(url string) string { return request(s.Handler(), "GET", url, "", "", "127.0.0.1").Body.String() }
	// A candidate not stored yet: its path comes in the query, and it is
	// shown but not cached, so it cannot decide the movie's cached poster.
	if b := body("/img/poster/7857.jpg?p=/other.jpg"); !strings.HasSuffix(b, "/other.jpg") {
		t.Fatalf("candidate poster %q", b)
	}
	if s.Images.Has(images.Poster, 7857) {
		t.Fatal("candidate preview was cached")
	}
	s.Identifier.Adopt(context.Background(), 7857)
	if code := get("/img/backdrop/7857.jpg"); code != 200 {
		t.Fatalf("backdrop %d", code)
	}
	if b := body("/img/poster/7857.jpg"); !strings.HasSuffix(b, "/p.jpg") {
		t.Fatalf("movie poster %q", b)
	}
	if code := get("/img/poster/7857.jpg?p=/other.jpg"); code != 200 || len(f.images) != 3 {
		t.Fatalf("cached poster %d, downloads %v", code, f.images)
	}
	rec := request(s.Handler(), "GET", "/img/poster/7857.jpg", "", "", "127.0.0.1")
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
	for _, bad := range []string{"/img/thumb/7857.jpg", "/img/poster/abc.jpg", "/img/poster/7857.png", "/img/poster/1.jpg",
		"/img/poster/2.jpg?p=../../etc/passwd"} {
		if code := get(bad); code != http.StatusNotFound {
			t.Errorf("%s: %d", bad, code)
		}
	}
}

func TestStatusIncludesIdentify(t *testing.T) {
	s, _ := identifyServer(t)
	rec := request(s.Handler(), "GET", "/api/status", "", "", "127.0.0.1")
	var body struct {
		Identify *identify.Status `json:"identify"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Identify == nil || body.Identify.State != identify.StateIdle {
		t.Fatalf("status %s", rec.Body)
	}
}
