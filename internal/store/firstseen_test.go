package store

import (
	"path/filepath"
	"testing"
	"time"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
)

func firstSeen(t *testing.T, s *Store, path string) int64 {
	t.Helper()
	var v int64
	if err := s.db.QueryRow(`SELECT first_seen FROM files WHERE path = ?`, path).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFirstSeen(t *testing.T) {
	s := open(t)
	before := time.Now().UnixMilli()
	s.SyncFiles([]FileRow{{Path: "../cine/a.mkv", Size: 10, MTime: 1, Fingerprint: "fa", Kind: "video"}}, roots)
	first := firstSeen(t, s, "../cine/a.mkv")
	if first < before || first > time.Now().UnixMilli() {
		t.Fatalf("first_seen %d, want the time of the scan", first)
	}
	// Rescanning a changed file keeps the date.
	time.Sleep(5 * time.Millisecond)
	s.SyncFiles([]FileRow{{Path: "../cine/a.mkv", Size: 11, MTime: 2, Fingerprint: "fa2", Kind: "video"}}, roots)
	if got := firstSeen(t, s, "../cine/a.mkv"); got != first {
		t.Fatalf("rescan changed first_seen: %d → %d", first, got)
	}
	// A moved file (new path, same content) keeps the date of its content.
	s.SyncFiles([]FileRow{{Path: "../cine/b/a.mkv", Size: 11, MTime: 2, Fingerprint: "fa2", Kind: "video"}}, roots)
	if got := firstSeen(t, s, "../cine/b/a.mkv"); got != first {
		t.Fatalf("moved file first_seen %d, want %d", got, first)
	}
	// Without a fingerprint there is nothing to follow.
	s.SyncFiles([]FileRow{{Path: "../cine/c.mkv", Size: 0, MTime: 2, Kind: "video"}}, roots)
	if got := firstSeen(t, s, "../cine/c.mkv"); got <= first {
		t.Fatalf("new empty file first_seen %d", got)
	}
}

func TestVersionAdded(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{
		{Path: "../cine/x/CD1.avi", Size: 10, MTime: 1, Fingerprint: "p1", Kind: "video"},
		{Path: "../cine/x/CD2.avi", Size: 10, MTime: 1, Fingerprint: "p2", Kind: "video"},
	}, roots)
	s.db.Exec(`UPDATE files SET first_seen = 500 WHERE path = '../cine/x/CD1.avi'`)
	s.db.Exec(`UPDATE files SET first_seen = 300 WHERE path = '../cine/x/CD2.avi'`)
	s.ReplaceVersions([]grouping.Version{{Dir: "../cine/x", Parsed: nameparse.Parsed{Title: "X"}, Size: 20, Parts: 2,
		Members: []grouping.Member{{Path: "../cine/x/CD1.avi", Role: grouping.RoleMain, Part: 1},
			{Path: "../cine/x/CD2.avi", Role: grouping.RoleMain, Part: 2}}}})
	vs, err := s.Versions()
	if err != nil || len(vs) != 1 || vs[0].Added != 300 || vs[0].Files[0].Size != 10 {
		t.Fatalf("versions %+v, %v", vs, err)
	}
}

// olderCatalog is a catalog from before first_seen, with one file.
func olderCatalog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cinexplorer.db")
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`ALTER TABLE files DROP COLUMN first_seen`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO files (path, size, mtime, fingerprint, kind) VALUES ('../cine/a.mkv', 10, 1234, 'fa', 'video')`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	return path
}

func TestFirstSeenMigration(t *testing.T) {
	path := olderCatalog(t)
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := firstSeen(t, s, "../cine/a.mkv"); got != 1234 {
		t.Fatalf("migrated first_seen %d, want the mtime", got)
	}
}

func TestOlderReadOnlyCatalogWithoutFirstSeen(t *testing.T) {
	path := olderCatalog(t)
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Versions(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(); err != nil {
		t.Fatal(err)
	}
}
