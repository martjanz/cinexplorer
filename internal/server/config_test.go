package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cinexplorer/internal/config"
	"cinexplorer/internal/engine"
	"cinexplorer/internal/httpx"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// configServer is a first start: <tmp>/cinexplorer next to cine/ and
// ordenar/ (both empty), no config.json, with a token set in memory.
func configServer(t *testing.T) *Server {
	t.Helper()
	disk := t.TempDir()
	appDir := filepath.Join(disk, "cinexplorer")
	for _, d := range []string{appDir, filepath.Join(disk, "cine"), filepath.Join(disk, "ordenar")} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := &Server{AppDir: appDir, Store: st, Engine: &engine.Engine{AppDir: appDir, Store: st}}
	s.Engine.Start(config.Config{Roots: []string{"../cine"}, TMDBToken: "eyJsecret1234", Language: "es-AR"}, true)
	t.Cleanup(func() { s.Engine.Current().Stop() })
	return s
}

func send(t *testing.T, s *Server, method, url, body string, v any) int {
	t.Helper()
	rec := request(s.Handler(), method, url, body, "application/json", "127.0.0.1")
	if rec.Code == http.StatusOK && v != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("%s %s: %s (%v)", method, url, rec.Body, err)
		}
	}
	return rec.Code
}

func TestGetConfig(t *testing.T) {
	s := configServer(t)
	var v configView
	if code := getJSON(t, s, "/api/config", &v); code != 200 {
		t.Fatalf("status %d", code)
	}
	want := configView{SetupPending: true, Roots: []rootView{{"../cine", true}}, Suggested: []string{"../ordenar"},
		HasToken: true, TokenHint: "…1234", Language: "es-AR", Languages: config.Languages, ImagePrefetch: "none"}
	if !reflect.DeepEqual(v, want) {
		t.Fatalf("got %+v", v)
	}
	rec := request(s.Handler(), "GET", "/api/config", "", "", "127.0.0.1")
	if b := rec.Body.String(); b == "" || strings.Contains(b, "eyJsecret") {
		t.Fatalf("token sent back: %s", b)
	}
	var st map[string]any
	getJSON(t, s, "/api/status", &st)
	if st["setupPending"] != true {
		t.Fatalf("status %v", st)
	}
}

func TestCheckRootEndpoint(t *testing.T) {
	s := configServer(t)
	var v rootView
	abs := filepath.Join(filepath.Dir(s.AppDir), "ordenar")
	if code := send(t, s, "POST", "/api/config/root", fmt.Sprintf(`{"path":%q}`, abs), &v); code != 200 || v != (rootView{"../ordenar", true}) {
		t.Fatalf("%d %+v", code, v)
	}
	for _, p := range []string{"../nada", ".", ""} {
		if code := send(t, s, "POST", "/api/config/root", fmt.Sprintf(`{"path":%q}`, p), nil); code != http.StatusBadRequest {
			t.Errorf("%q: %d", p, code)
		}
	}
}

func TestCheckTokenEndpoint(t *testing.T) {
	s := configServer(t)
	for _, tc := range []struct {
		err  error
		want string
	}{
		{nil, `{"valid":true}`},
		{tmdb.ErrNotFound, `{"valid":true}`},
		{tmdb.ErrUnauthorized, `{"valid":false}`},
		{fmt.Errorf("%w: dial", httpx.ErrOffline), `{"valid":null}`},
	} {
		var got string
		s.VerifyToken = func(ctx context.Context, token string) error {
			got = token
			return tc.err
		}
		rec := request(s.Handler(), "POST", "/api/config/token", `{"token":" eyJnew "}`, "application/json", "127.0.0.1")
		if rec.Code != 200 || got != "eyJnew" || rec.Body.String() != tc.want+"\n" {
			t.Errorf("%v: %d %s (token %q)", tc.err, rec.Code, rec.Body, got)
		}
	}
	if code := send(t, s, "POST", "/api/config/token", `{"token":""}`, nil); code != http.StatusBadRequest {
		t.Fatalf("empty token: %d", code)
	}
}

func TestPutConfig(t *testing.T) {
	s := configServer(t)
	old := s.Engine.Current()
	var v configView
	// null keeps the token (no movies to identify: no network).
	body := `{"roots":["../cine","../ordenar"],"token":null,"language":"pt-BR","imagePrefetch":"posters"}`
	if code := send(t, s, "PUT", "/api/config", body, &v); code != 200 {
		t.Fatalf("status %d", code)
	}
	if v.SetupPending || !v.HasToken || v.Language != "pt-BR" || len(v.Roots) != 2 || len(v.Suggested) != 0 || s.Engine.Current() == old {
		t.Fatalf("got %+v", v)
	}
	saved, _, _ := config.Load(s.AppDir)
	want := config.Config{Roots: []string{"../cine", "../ordenar"}, TMDBToken: "eyJsecret1234", Language: "pt-BR", ImagePrefetch: "posters"}
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("saved %+v", saved)
	}
	// "" removes it.
	if code := send(t, s, "PUT", "/api/config", `{"roots":["../cine"],"token":"","language":"es-AR"}`, &v); code != 200 || v.HasToken {
		t.Fatalf("%d %+v", code, v)
	}
	if s.Engine.Current().TMDB != nil {
		t.Fatal("TMDB client without a token")
	}
}

func TestPutConfigRejects(t *testing.T) {
	s := configServer(t)
	for _, body := range []string{
		`{"roots":[],"language":"es-AR"}`,
		`{"roots":["../nada"],"language":"es-AR"}`,
		`{"roots":["../cine"],"language":"xx-XX"}`,
		`{"roots":["../cine"],"language":"es-AR","imagePrefetch":"some"}`,
		`{"roots":`,
	} {
		if code := send(t, s, "PUT", "/api/config", body, nil); code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, code)
		}
	}
	if !s.Engine.SetupPending() {
		t.Fatal("a rejected PUT applied the settings")
	}
	s.ReadOnly = true
	if code := send(t, s, "PUT", "/api/config", `{"roots":["../cine"],"language":"es-AR"}`, nil); code != http.StatusConflict {
		t.Fatalf("read-only: %d", code)
	}
}

func TestPutConfigKeepsSavedLanguage(t *testing.T) {
	s := configServer(t)
	s.Engine.Current().Config.Language = "es-ES" // an older config.json
	var v configView
	if code := send(t, s, "PUT", "/api/config", `{"roots":["../cine"],"language":"es-ES"}`, &v); code != 200 || v.Language != "es-ES" {
		t.Fatalf("%d %+v", code, v)
	}
	if v.Languages[len(v.Languages)-1] != "es-ES" {
		t.Fatalf("languages %v", v.Languages)
	}
}
