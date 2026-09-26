package server

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticApp(t *testing.T) {
	s, _ := newServer(t)
	s.Static = fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>Cinexplorer</title>")},
		"assets/index-abc1.js": {Data: []byte("console.log(1)")},
		"favicon.svg":          {Data: []byte("<svg/>")},
	}
	h := s.Handler()
	cases := []struct {
		path, body, cache string
		code              int
	}{
		{"/", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/explorar?decada=1970", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/pelicula/7857", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/version/id:9", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/revisar/duplicados", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/assets/index-abc1.js", "console.log(1)", "public, max-age=31536000, immutable", 200},
		{"/favicon.svg", "<svg/>", "", 200},
		{"/assets/missing.js", "", "", http.StatusNotFound},
		{"/api/nada", "", "", http.StatusNotFound},
		{"/img/x", "", "", http.StatusNotFound},
	}
	for _, c := range cases {
		rec := request(h, "GET", c.path, "", "", "127.0.0.1")
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.body) || rec.Header().Get("Cache-Control") != c.cache {
			t.Errorf("%s: %d %q (Cache-Control %q), want %d %q (%q)", c.path, rec.Code, rec.Body, rec.Header().Get("Cache-Control"),
				c.code, c.body, c.cache)
		}
	}
}

func TestEmbeddedAppHasIndex(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/", "", "", "127.0.0.1")
	if rec.Code != 200 || !strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
		t.Fatalf("status %d: %.200s", rec.Code, rec.Body)
	}
}
