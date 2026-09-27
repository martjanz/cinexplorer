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
	if !created || !reflect.DeepEqual(cfg.Roots, want) || cfg.Language != "es-AR" {
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

// disk makes <tmp>/cinexplorer (the app dir) next to cine/1970s, and a file.
func disk(t *testing.T) (appDir, root string) {
	t.Helper()
	root = t.TempDir()
	appDir = filepath.Join(root, "cinexplorer")
	for _, d := range []string{appDir, filepath.Join(root, "cine", "1970s"), filepath.Join(appDir, "cache")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return appDir, root
}

func TestRoot(t *testing.T) {
	appDir, root := disk(t)
	for _, tc := range []struct{ in, want string }{
		{"../cine", "../cine"},
		{"../cine/", "../cine"},
		{"./../cine/1970s", "../cine/1970s"},
		{filepath.Join(root, "cine"), "../cine"},
		{"  ../cine  ", "../cine"},
	} {
		got, err := Root(appDir, tc.in)
		if err != nil || got != tc.want {
			t.Errorf("Root(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"", "../nada", "../notes.txt", ".", appDir, "..", root, "cache", filepath.Join(appDir, "cache")} {
		if got, err := Root(appDir, in); err == nil {
			t.Errorf("Root(%q) = %q, want an error", in, got)
		}
	}
}

// TestRootCanonicalizesCase checks that a case-variant spelling of an
// existing folder resolves to the on-disk name, so a case-insensitive
// filesystem (Windows, macOS) never admits the same folder twice under two
// different spellings. On a case-sensitive filesystem (Linux) the lowercase
// spelling of "Cine" doesn't exist, so there's nothing to canonicalize.
func TestRootCanonicalizesCase(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "cinexplorer")
	if err := os.MkdirAll(filepath.Join(root, "Cine", "1970s"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}

	lower := filepath.Join(root, "cine")
	if _, err := os.Stat(lower); err != nil {
		t.Skip("case-sensitive filesystem: no case-insensitive match to canonicalize")
	}

	if got, err := Root(appDir, lower); err != nil || got != "../Cine" {
		t.Fatalf("Root(%q) = %q, %v; want ../Cine", lower, got, err)
	}
	nested := filepath.Join(root, "cine", "1970S")
	if got, err := Root(appDir, nested); err != nil || got != "../Cine/1970s" {
		t.Fatalf("Root(%q) = %q, %v; want ../Cine/1970s", nested, got, err)
	}
}

func TestCheckRoots(t *testing.T) {
	appDir, _ := disk(t)
	ok := [][]string{
		{"../cine"},
		{"../cine", "../gone"}, // saved, unplugged
	}
	for _, roots := range ok {
		if err := CheckRoots(appDir, roots, []string{"../gone"}); err != nil {
			t.Errorf("%v: %v", roots, err)
		}
	}
	bad := [][]string{
		nil,
		{"../gone"},                  // new and missing
		{"../cine/"},                 // not in catalog form
		{"../cine", "../cine"},       // repeated
		{"../cine", "../cine/1970s"}, // nested
		{"../cine/1970s", "../cine"}, // nested, the other way
	}
	for _, roots := range bad {
		if err := CheckRoots(appDir, roots, nil); err == nil {
			t.Errorf("%v: want an error", roots)
		}
	}
}

func TestAvailable(t *testing.T) {
	appDir, _ := disk(t)
	if !Available(appDir, "../cine") || Available(appDir, "../nada") || Available(appDir, "../notes.txt") {
		t.Fatal("Available")
	}
}
