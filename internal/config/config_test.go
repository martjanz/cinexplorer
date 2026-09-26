package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadDefaultsToSiblingDirs(t *testing.T) {
	disk := t.TempDir()
	for _, d := range []string{"cinexplorer", "cine", "cine-ordenar", "$RECYCLE.BIN", ".Trashes", "System Volume Information"} {
		if err := os.Mkdir(filepath.Join(disk, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(disk, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, created, err := Load(filepath.Join(disk, "cinexplorer"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"../cine", "../cine-ordenar"}
	if !created || !reflect.DeepEqual(cfg.Roots, want) || cfg.Language != "es-ES" {
		t.Fatalf("got created=%v cfg=%+v", created, cfg)
	}
}

func TestSaveThenLoad(t *testing.T) {
	dir := t.TempDir()
	in := Config{Roots: []string{"../x"}, TMDBToken: "tok", Language: "es-ES", ImagePrefetch: "posters"}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, created, err := Load(dir)
	if err != nil || created || !reflect.DeepEqual(in, out) {
		t.Fatalf("got %+v created=%v err=%v", out, created, err)
	}
}

func TestLoadRejectsBrokenJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, FileName), []byte("{roots:"), 0o644)
	if _, _, err := Load(dir); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
