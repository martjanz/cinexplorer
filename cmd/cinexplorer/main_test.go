package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSmoke runs the app as a first start would: an app directory next to a
// movie folder, no config.json and no TMDB token. The first-use settings are
// saved as proposed, and the movie shows up.
func TestSmoke(t *testing.T) {
	disk := t.TempDir()
	appDir := filepath.Join(disk, "cinexplorer")
	movie := filepath.Join(disk, "cine", "1970s", "Amarcord (Federico Fellini, 1973)", "Amarcord.1973.720p.mkv")
	for _, d := range []string{appDir, filepath.Dir(movie)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(movie, []byte(strings.Repeat("x", 4096)), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, closeStore, err := setup(appDir)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	call := func(method, path, body string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	get := func(path string) (int, string) { t.Helper(); return call("GET", path, "") }

	// First start: nothing saved or scanned until the settings are.
	if code, body := get("/api/status"); code != 200 || !strings.Contains(body, `"setupPending":true`) {
		t.Fatalf("status %d %s", code, body)
	}
	code, body := get("/api/config")
	var cfg struct {
		Roots []struct {
			Path string `json:"path"`
		} `json:"roots"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal([]byte(body), &cfg); err != nil || code != 200 || len(cfg.Roots) != 1 ||
		cfg.Roots[0].Path != "../cine" || cfg.Language != "es-AR" {
		t.Fatalf("config %d %s (%v)", code, body, err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "config.json")); err == nil {
		t.Fatal("config.json written before the first-use settings")
	}
	if code, body := call("PUT", "/api/config", `{"roots":["../cine"],"language":"es-AR"}`); code != 200 {
		t.Fatalf("put %d %s", code, body)
	}
	if _, err := os.Stat(filepath.Join(appDir, "config.json")); err != nil {
		t.Fatalf("config.json: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for st := srv.Engine.Current().Scanner.Status(); st.Running || st.Finished.IsZero(); st = srv.Engine.Current().Scanner.Status() {
		if time.Now().After(deadline) {
			t.Fatalf("scan did not finish: %+v", st)
		}
		time.Sleep(20 * time.Millisecond)
	}

	for _, p := range []string{"/", "/explorar?decada=1970", "/buscar?q=amarcord", "/bienvenida", "/ajustes"} {
		if code, body := get(p); code != 200 || !strings.Contains(strings.ToLower(body), "<!doctype html>") {
			t.Errorf("%s: %d %.200s", p, code, body)
		}
	}
	code, body = get("/api/explore?decada=1970")
	var explore struct {
		Total int `json:"total"`
		Items []struct {
			Title string `json:"title"`
			Year  int    `json:"year"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &explore); err != nil || code != 200 || explore.Total != 1 ||
		explore.Items[0].Title != "Amarcord" || explore.Items[0].Year != 1973 {
		t.Fatalf("explore %d %s (%v)", code, body, err)
	}
	if code, body := get("/api/search?q=amarc"); code != 200 || !strings.Contains(body, `"total":1`) {
		t.Errorf("search %d %s", code, body)
	}
	if code, body := get("/api/status"); code != 200 || !strings.Contains(body, `"state":"noToken"`) ||
		!strings.Contains(body, `"setupPending":false`) {
		t.Errorf("status %d %s", code, body)
	}
}
