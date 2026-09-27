package scan

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"cinexplorer/internal/appdir"
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

// TestScanPreservesUnreadableFiles is a regression test: a file that fails to
// fingerprint (locked, flaky drive, half-written, corrupt) must never be
// silently dropped nor wrongly marked missing. We simulate "unreadable"
// portably with a dangling symlink, since permission bits (chmod) don't
// block reads on Windows. If this environment cannot create symlinks
// (e.g. Windows without Developer Mode/admin), the test skips itself.
func TestScanPreservesUnreadableFiles(t *testing.T) {
	disk, app, st := setup(t)

	// A healthy, unrelated file: it must be unaffected by the broken ones.
	good := filepath.Join(disk, "cine", "good", "Movie.mkv")
	writeFile(t, good, 4096, 'g')

	// A file that is healthy on the first scan (gets a real fingerprint),
	// then becomes unreadable without being deleted: simulates a locked
	// file or a flaky external drive going away and coming back corrupt.
	flakyDir := filepath.Join(disk, "cine", "flaky")
	flaky := filepath.Join(flakyDir, "Ok.mkv")
	writeFile(t, flaky, 4096, 'o')

	// A brand-new file whose target is already gone on its very first scan:
	// simulates a corrupt/half-written file that fails to open at all.
	brokenTarget := filepath.Join(disk, "cine", "bad", "target.mkv")
	writeFile(t, brokenTarget, 4096, 'z')
	broken := filepath.Join(disk, "cine", "bad", "broken.mkv")
	if err := os.Symlink(brokenTarget, broken); err != nil {
		t.Skipf("symlinks not supported in this environment, skipping: %v", err)
	}
	if err := os.Remove(brokenTarget); err != nil {
		t.Fatal(err)
	}

	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st}

	// First scan: "flaky" is healthy and gets a real fingerprint; "broken" is
	// already dangling the first time it is ever seen.
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx, err := st.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	relFlaky, _ := appdir.Rel(app, flaky)
	relBroken, _ := appdir.Rel(app, broken)
	relGood, _ := appdir.Rel(app, good)

	flakyRow, ok := idx[relFlaky]
	if !ok || flakyRow.Missing || flakyRow.Fingerprint == "" {
		t.Fatalf("flaky row after first scan: ok=%v %+v", ok, flakyRow)
	}
	originalFingerprint := flakyRow.Fingerprint

	brokenRow, ok := idx[relBroken]
	if !ok || brokenRow.Missing {
		t.Fatalf("broken row after first scan: ok=%v %+v", ok, brokenRow)
	}
	if brokenRow.Fingerprint != "" {
		t.Fatalf("brand-new unreadable file should not have a real fingerprint yet: %+v", brokenRow)
	}

	// Now "flaky" becomes unreadable without being deleted: replace it with a
	// dangling symlink of the same name, so its size/mtime differ from what
	// the store knows and a re-fingerprint is attempted (and fails).
	if err := os.Remove(flaky); err != nil {
		t.Fatal(err)
	}
	flakyTarget := filepath.Join(flakyDir, "gone.mkv")
	writeFile(t, flakyTarget, 8192, 'q')
	if err := os.Symlink(flakyTarget, flaky); err != nil {
		t.Skipf("symlinks not supported in this environment, skipping: %v", err)
	}
	if err := os.Remove(flakyTarget); err != nil {
		t.Fatal(err)
	}

	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx, err = st.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	flakyRow, ok = idx[relFlaky]
	if !ok {
		t.Fatal("transiently unreadable file disappeared from the catalog")
	}
	if flakyRow.Missing {
		t.Fatal("transiently unreadable file was wrongly marked missing")
	}
	if flakyRow.Fingerprint != originalFingerprint {
		t.Fatalf("fingerprint of unreadable file should be preserved, got %q want %q", flakyRow.Fingerprint, originalFingerprint)
	}
	if row, ok := idx[relGood]; !ok || row.Missing {
		t.Fatalf("unrelated healthy file affected: ok=%v %+v", ok, row)
	}
}

// TestScanIsolatesPartiallyUnreadableRoot is a regression test: when a
// subdirectory of an otherwise-mounted root produces an I/O error partway
// through the walk, files under the rest of that root must still be
// cataloged, but the root must NOT be treated as fully, cleanly scanned —
// otherwise known files under the unreadable part get wrongly marked
// missing. We simulate the I/O error with a Windows deny ACL (chmod does not
// block reads on Windows); the test skips on non-Windows platforms and skips
// itself if the ACL trick doesn't actually block access in this environment
// (e.g. running as an administrator, where deny ACLs can be bypassed).
func TestScanIsolatesPartiallyUnreadableRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("uses a Windows-specific deny ACL to simulate a mid-walk I/O error")
	}
	disk, app, st := setup(t)

	okFile := filepath.Join(disk, "cine", "ok", "Fine.mkv")
	writeFile(t, okFile, 4096, 'f')
	flakyDir := filepath.Join(disk, "cine", "flaky")
	flakyFile := filepath.Join(flakyDir, "Locked.mkv")
	writeFile(t, flakyFile, 4096, 'l')

	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx, err := st.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	relFlaky, _ := appdir.Rel(app, flakyFile)
	relOk, _ := appdir.Rel(app, okFile)
	if row, ok := idx[relFlaky]; !ok || row.Missing {
		t.Fatalf("flaky file not cataloged after first scan: ok=%v %+v", ok, row)
	}

	user := os.Getenv("USERNAME")
	if user == "" {
		t.Skip("USERNAME not set, cannot build an icacls deny rule")
	}
	if out, err := exec.Command("icacls", flakyDir, "/deny", user+":(RX)").CombinedOutput(); err != nil {
		t.Skipf("could not set up a deny ACL, skipping: %v: %s", err, out)
	}
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		_ = exec.Command("icacls", flakyDir, "/remove:d", user).Run()
	}
	t.Cleanup(restore)

	// Confirm the deny actually blocks reads before asserting anything on it.
	if _, err := os.ReadDir(flakyDir); err == nil {
		restore()
		t.Skip("deny ACL did not block directory listing in this environment, skipping")
	}

	if err := sc.Run(context.Background()); err != nil {
		restore()
		t.Fatal(err)
	}
	restore()

	idx, err = st.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	if row, ok := idx[relFlaky]; !ok || row.Missing {
		t.Fatalf("file under a partially unreadable root was wrongly marked missing: ok=%v %+v", ok, row)
	}
	if row, ok := idx[relOk]; !ok || row.Missing {
		t.Fatalf("unaffected file under the same root was impacted: ok=%v %+v", ok, row)
	}
}

func TestScanForgetsFingerprintOfEmptyFiles(t *testing.T) {
	disk, app, st := setup(t)
	writeFile(t, filepath.Join(disk, "cine", "A", "a.avi"), 0, 0)
	writeFile(t, filepath.Join(disk, "cine", "B", "b.avi"), 0, 0)
	// An older catalog gave both the same fingerprint.
	info, _ := os.Stat(filepath.Join(disk, "cine", "A", "a.avi"))
	infoB, _ := os.Stat(filepath.Join(disk, "cine", "B", "b.avi"))
	st.SyncFiles([]store.FileRow{
		{Path: "../cine/A/a.avi", Size: 0, MTime: info.ModTime().UnixMilli(), Fingerprint: "e3b0", Kind: "video"},
		{Path: "../cine/B/b.avi", Size: 0, MTime: infoB.ModTime().UnixMilli(), Fingerprint: "e3b0", Kind: "video"},
	}, []string{"../cine"})
	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx, _ := st.FileIndex()
	if idx["../cine/A/a.avi"].Fingerprint != "" || idx["../cine/B/b.avi"].Fingerprint != "" {
		t.Fatalf("index %+v", idx)
	}
	if d, _ := st.Duplicates(); len(d) != 0 {
		t.Fatalf("empty files reported as copies: %+v", d)
	}
}

func TestScanCallsOnDone(t *testing.T) {
	_, app, st := setup(t)
	calls := 0
	sc := &Scanner{AppDir: app, Roots: []string{"../missing"}, Store: st, OnDone: func() { calls++ }}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("OnDone called %d times", calls)
	}
}

func TestScanMarksRemovedRootMissing(t *testing.T) {
	disk, app, st := setup(t)
	writeFile(t, filepath.Join(disk, "cine", "Amarcord.1973.mkv"), 4096, 'a')
	writeFile(t, filepath.Join(disk, "cine-ordenar", "Attenberg.2010.avi"), 4096, 'b')
	sc := &Scanner{AppDir: app, Roots: []string{"../cine", "../cine-ordenar"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	sc.Roots = []string{"../cine"}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx, err := st.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	if idx["../cine/Amarcord.1973.mkv"].Missing || !idx["../cine-ordenar/Attenberg.2010.avi"].Missing {
		t.Fatalf("index %+v", idx)
	}
}
