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
// movie folder, no config.json and no TMDB token; then the settings are
// saved as proposed.
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
	// First start: nothing saved or scanned until the settings are.
	time.Sleep(50 * time.Millisecond)
	if !srv.Engine.SetupPending() || !srv.Engine.Current().Scanner.Status().Finished.IsZero() {
		t.Fatal("scanned before the first-use settings")
	}
	if _, err := os.Stat(filepath.Join(appDir, "config.json")); err == nil {
		t.Fatal("config.json written before the first-use settings")
	}
	cfg := srv.Engine.Current().Config
	if len(cfg.Roots) != 1 || cfg.Roots[0] != "../cine" || cfg.Language != "es-AR" {
		t.Fatalf("proposed %+v", cfg)
	}
	if err := srv.Engine.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for st := srv.Engine.Current().Scanner.Status(); st.Running || st.Finished.IsZero(); st = srv.Engine.Current().Scanner.Status() {
		if time.Now().After(deadline) {
			t.Fatalf("scan did not finish: %+v", st)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(appDir, "config.json")); err != nil {
		t.Fatalf("config.json: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	get := func(path string) (int, string) {
		t.Helper()
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	for _, p := range []string{"/", "/explorar?decada=1970"} {
		if code, body := get(p); code != 200 || !strings.Contains(strings.ToLower(body), "<!doctype html>") {
			t.Errorf("%s: %d %.200s", p, code, body)
		}
	}
	code, body := get("/api/explore?decada=1970")
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
	if code, body := get("/api/status"); code != 200 || !strings.Contains(body, `"state":"noToken"`) {
		t.Errorf("status %d %s", code, body)
	}
}
