package scan

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"cinexplorer/internal/store"
)

func writeFile(t *testing.T, path string, size int, fill byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{fill}, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) (string, string, *store.Store) {
	t.Helper()
	disk := t.TempDir()
	app := filepath.Join(disk, "cinexplorer")
	if err := os.Mkdir(app, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(app, "cinexplorer.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return disk, app, st
}

func TestScanBuildsCatalogIncrementally(t *testing.T) {
	disk, app, st := setup(t)
	amarcord := filepath.Join(disk, "cine", "1970s", "Amarcord [Federico Fellini, 1973]")
	writeFile(t, filepath.Join(amarcord, "amarcord.mkv"), 4096, 'a')
	writeFile(t, filepath.Join(amarcord, "amarcord.spa.srt"), 100, 's')
	writeFile(t, filepath.Join(amarcord, "info.nfo"), 10, 'n')
	writeFile(t, filepath.Join(disk, "cine", "Thumbs.db"), 10, 'x')
	writeFile(t, filepath.Join(disk, "cine-ordenar", "Attenberg.avi"), 4096, 'b')

	sc := &Scanner{AppDir: app, Roots: []string{"../cine", "../cine-ordenar"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	vs, err := st.Versions()
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("want 2 versions, got %+v", vs)
	}
	if v := vs[0]; v.Title != "Amarcord" || v.Year != 1973 || v.Director != "Federico Fellini" || v.SubLangs != "es" {
		t.Fatalf("got %+v", v)
	}
	if got := sc.Status(); got.Files != 4 || got.Hashed != 2 || got.Versions != 2 || got.Running {
		t.Fatalf("status %+v", got)
	}

	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := sc.Status(); got.Hashed != 0 {
		t.Fatalf("second scan re-hashed %d files", got.Hashed)
	}
}

func TestScanMarksMissingAndFindsCopies(t *testing.T) {
	disk, app, st := setup(t)
	original := filepath.Join(disk, "cine", "a", "Movie.mkv")
	writeFile(t, original, 4096, 'm')
	writeFile(t, filepath.Join(disk, "cine-ordenar", "movie-copy.mkv"), 4096, 'm')

	sc := &Scanner{AppDir: app, Roots: []string{"../cine", "../cine-ordenar"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d, _ := st.Duplicates(); len(d) != 1 || len(d[0].Paths) != 2 {
		t.Fatalf("duplicates %+v", d)
	}

	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d, _ := st.Duplicates(); len(d) != 0 {
		t.Fatalf("duplicates after delete %+v", d)
	}
	idx, _ := st.FileIndex()
	if !idx["../cine/a/Movie.mkv"].Missing {
		t.Fatal("deleted file should be marked missing")
	}
}

func TestScanSkipsUnavailableRoot(t *testing.T) {
	disk, app, st := setup(t)
	writeFile(t, filepath.Join(disk, "cine", "Stalker (1979).mkv"), 4096, 's')
	sc := &Scanner{AppDir: app, Roots: []string{"../cine", "../no-existe"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if vs, _ := st.Versions(); len(vs) != 1 || vs[0].Title != "Stalker" {
		t.Fatalf("got %+v", vs)
	}
}
