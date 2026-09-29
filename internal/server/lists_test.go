package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"cinexplorer/internal/catalog"
)

func TestListsEndpoints(t *testing.T) {
	s, _ := identifyServer(t)
	rec := request(s.Handler(), "POST", "/api/lists", `{"name":" Por ver "}`, "application/json", "127.0.0.1")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d: %s", rec.Code, rec.Body)
	}
	var l catalog.ListRef
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil || l.ID == 0 || l.Name != "Por ver" {
		t.Fatalf("created %s (%v)", rec.Body, err)
	}
	for body, want := range map[string]int{`{"name":"por ver"}`: http.StatusConflict, `{"name":"  "}`: http.StatusBadRequest, `{`: http.StatusBadRequest} {
		if code := request(s.Handler(), "POST", "/api/lists", body, "application/json", "127.0.0.1").Code; code != want {
			t.Errorf("create %s: %d, want %d", body, code, want)
		}
	}

	path := fmt.Sprintf("/api/lists/%d", l.ID)
	// The unidentified Amarcord, by its key (its fingerprint).
	if code := request(s.Handler(), "POST", path+"/entries", `{"key":"f1"}`, "application/json", "127.0.0.1").Code; code != http.StatusNoContent {
		t.Fatalf("add %d", code)
	}
	for _, c := range []struct {
		url, body string
		want      int
	}{
		{path + "/entries", `{"key":"nada"}`, http.StatusNotFound},
		{path + "/entries", `{"tmdbId":7857}`, http.StatusNotFound}, // not stored yet
		{path + "/entries", `{}`, http.StatusBadRequest},
		{path + "/entries", `{"tmdbId":7857,"key":"f1"}`, http.StatusBadRequest},
		{"/api/lists/999/entries", `{"key":"f1"}`, http.StatusNotFound},
		{"/api/lists/x/entries", `{"key":"f1"}`, http.StatusNotFound},
	} {
		if code := request(s.Handler(), "POST", c.url, c.body, "application/json", "127.0.0.1").Code; code != c.want {
			t.Errorf("add %s %s: %d, want %d", c.url, c.body, code, c.want)
		}
	}
	var cards []catalog.ListCard
	if code := getJSON(t, s, "/api/lists", &cards); code != 200 || len(cards) != 1 || cards[0].Count != 1 || cards[0].Name != "Por ver" {
		t.Fatalf("lists %d %+v", code, cards)
	}

	// Once identified, the entry leads to the movie, whose page names the list.
	post(t, s, `{"fingerprint":"f1","action":"movie","tmdbId":7857}`)
	var d catalog.MovieDetail
	getJSON(t, s, "/api/movies/7857", &d)
	if len(d.Lists) != 1 || d.Lists[0] != l {
		t.Fatalf("movie lists %+v", d.Lists)
	}
	var b exploreBody
	getJSON(t, s, fmt.Sprintf("/api/explore?lista=%d", l.ID), &b)
	if b.Total != 1 || b.Items[0].TMDBID != 7857 || b.Query.Order != catalog.OrderListAdded || b.List == nil || *b.List != l {
		t.Fatalf("explore %+v", b)
	}

	// Removing the movie removes the entry that led to it.
	if code := request(s.Handler(), "DELETE", path+"/entries", `{"tmdbId":7857}`, "application/json", "127.0.0.1").Code; code != http.StatusNoContent {
		t.Fatalf("remove %d", code)
	}
	getJSON(t, s, "/api/lists", &cards)
	if cards[0].Count != 0 {
		t.Fatalf("after remove %+v", cards)
	}

	if code := request(s.Handler(), "PATCH", path, `{"name":"Vistas"}`, "application/json", "127.0.0.1").Code; code != http.StatusNoContent {
		t.Fatalf("rename %d", code)
	}
	if code := request(s.Handler(), "PATCH", "/api/lists/999", `{"name":"Otra"}`, "application/json", "127.0.0.1").Code; code != http.StatusNotFound {
		t.Fatalf("rename unknown %d", code)
	}
	if code := request(s.Handler(), "DELETE", path, `{}`, "application/json", "127.0.0.1").Code; code != http.StatusNoContent {
		t.Fatalf("delete %d", code)
	}
	if code := request(s.Handler(), "DELETE", path, `{}`, "application/json", "127.0.0.1").Code; code != http.StatusNotFound {
		t.Fatalf("delete twice %d", code)
	}
	// A deleted list is dropped from Explorar's query, and so is its order.
	b = exploreBody{}
	getJSON(t, s, fmt.Sprintf("/api/explore?lista=%d", l.ID), &b)
	if b.List != nil || b.Query.Facets["lista"] != "" || b.Query.Order != catalog.OrderYear || b.Total != 1 {
		t.Fatalf("explore after delete %+v", b)
	}
}

func TestListsReadOnly(t *testing.T) {
	s, _ := newServer(t)
	s.ReadOnly = true
	for _, c := range []struct{ method, url string }{
		{"POST", "/api/lists"}, {"PATCH", "/api/lists/1"}, {"DELETE", "/api/lists/1"},
		{"POST", "/api/lists/1/entries"}, {"DELETE", "/api/lists/1/entries"},
	} {
		if code := request(s.Handler(), c.method, c.url, `{"name":"x","key":"f1"}`, "application/json", "127.0.0.1").Code; code != http.StatusConflict {
			t.Errorf("%s %s: %d", c.method, c.url, code)
		}
	}
	if code := request(s.Handler(), "POST", "/api/lists", `{"name":"x"}`, "text/plain", "127.0.0.1").Code; code != http.StatusUnsupportedMediaType {
		t.Fatalf("without JSON: %d", code)
	}
	var cards []catalog.ListCard
	if code := getJSON(t, s, "/api/lists", &cards); code != 200 || cards == nil {
		t.Fatalf("read %d %+v", code, cards)
	}
}
