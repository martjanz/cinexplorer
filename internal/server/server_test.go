package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
)

const moviePath = "../cine/Amarcord.mkv"

func newServer(t *testing.T) (*Server, *[]string) {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.SyncFiles([]store.FileRow{{Path: moviePath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"}}, []string{"../cine"})
	st.ReplaceVersions([]grouping.Version{{
		Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
		Members: []grouping.Member{{Path: moviePath, Role: grouping.RoleMain}},
	}})
	var opened []string
	s := &Server{
		AppDir:   t.TempDir(),
		Store:    st,
		Opener:   func(p string) error { opened = append(opened, p); return nil },
		Revealer: func(p string) error { opened = append(opened, "reveal:"+p); return nil },
	}
	return s, &opened
}

func request(h http.Handler, method, target, body, ctype, host string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = host
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestVersionsEndpoint(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/api/versions", "", "", "127.0.0.1:8080")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var vs []store.VersionView
	if err := json.Unmarshal(rec.Body.Bytes(), &vs); err != nil || len(vs) != 1 || vs[0].Title != "Amarcord" {
		t.Fatalf("got %s (%v)", rec.Body, err)
	}
}

func TestOpenKnownFile(t *testing.T) {
	s, opened := newServer(t)
	rec := request(s.Handler(), "POST", "/api/open", `{"path":"`+moviePath+`"}`, "application/json", "localhost:8080")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if want := appdir.Abs(s.AppDir, moviePath); len(*opened) != 1 || (*opened)[0] != want {
		t.Fatalf("opened %v, want %s", *opened, want)
	}
}

func TestOpenRejectsUnknownFile(t *testing.T) {
	s, opened := newServer(t)
	rec := request(s.Handler(), "POST", "/api/open", `{"path":"../../Windows/notepad.exe"}`, "application/json", "127.0.0.1")
	if rec.Code != http.StatusNotFound || len(*opened) != 0 {
		t.Fatalf("status %d opened %v", rec.Code, *opened)
	}
}

func TestPostRequiresJSON(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "POST", "/api/open", `{"path":"`+moviePath+`"}`, "text/plain", "127.0.0.1")
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRejectsForeignHost(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/api/versions", "", "", "evil.example.com")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestScanInReadOnlyMode(t *testing.T) {
	s, _ := newServer(t)
	s.ReadOnly = true
	rec := request(s.Handler(), "POST", "/api/scan", "{}", "application/json", "127.0.0.1")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestServesIndex(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/", "", "", "127.0.0.1")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "CINEXPLORER") {
		t.Fatalf("status %d", rec.Code)
	}
}
