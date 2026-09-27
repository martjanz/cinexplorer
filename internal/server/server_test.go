package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/catalog"
	"cinexplorer/internal/config"
	"cinexplorer/internal/engine"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/scan"
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
	s.Engine = &engine.Engine{AppDir: s.AppDir, Store: st}
	s.Engine.Use(&engine.Runtime{Config: config.Config{Roots: []string{"../cine"}, Language: "es-ES"}})
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

func TestDuplicatesEndpoint(t *testing.T) {
	s, _ := newServer(t)
	addCopy(t, s)
	rec := request(s.Handler(), "GET", "/api/duplicates", "", "", "127.0.0.1")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var d catalog.DupReport
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("got %s (%v)", rec.Body, err)
	}
	if d.Recoverable != 10 || len(d.Groups) != 1 || d.Groups[0].Key != "f1" || len(d.Groups[0].Versions) != 2 {
		t.Fatalf("got %+v", d)
	}
}

func TestScanRunsWhenWritable(t *testing.T) {
	s, _ := newServer(t)
	s.rt().Scanner = &scan.Scanner{AppDir: s.AppDir, Store: s.Store}
	rec := request(s.Handler(), "POST", "/api/scan", "{}", "application/json", "127.0.0.1")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		st := s.rt().Scanner.Status()
		if !st.Running && !st.Finished.IsZero() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("scan did not complete in time: %+v", st)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestVersionsEndpointIncludesTechnicalFields(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/api/versions", "", "", "127.0.0.1:8080")
	var vs []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &vs); err != nil || len(vs) != 1 {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	for _, k := range []string{"durationMs", "width", "height", "videoCodec", "best"} {
		if _, ok := vs[0][k]; !ok {
			t.Errorf("missing %q in %v", k, vs[0])
		}
	}
	for _, k := range []string{"audio", "subs"} {
		if _, ok := vs[0][k].([]any); !ok {
			t.Errorf("%q must be a JSON array, got %v", k, vs[0][k])
		}
	}
}

func TestStatusReportsProbeProgress(t *testing.T) {
	s, _ := newServer(t)
	s.rt().Scanner = &scan.Scanner{}
	rec := request(s.Handler(), "GET", "/api/status", "", "", "127.0.0.1:8080")
	var body struct {
		Scan map[string]any `json:"scan"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body.Scan["toProbe"]; !ok {
		t.Fatalf("status %s", rec.Body)
	}
	if _, ok := body.Scan["probed"]; !ok {
		t.Fatalf("status %s", rec.Body)
	}
}
