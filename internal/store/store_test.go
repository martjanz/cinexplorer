package store

import (
	"path/filepath"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "cinexplorer.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

var roots = []string{"../cine", "../cine-ordenar"}

func TestSyncFilesUpsertsAndMarksMissing(t *testing.T) {
	s := open(t)
	rows := []FileRow{
		{Path: "../cine/a.mkv", Size: 10, MTime: 1, Fingerprint: "fa", Kind: "video"},
		{Path: "../cine/b.mkv", Size: 20, MTime: 2, Fingerprint: "fb", Kind: "video"},
	}
	if err := s.SyncFiles(rows, roots); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncFiles(rows[:1], roots); err != nil {
		t.Fatal(err)
	}
	idx, err := s.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	if idx["../cine/a.mkv"].Missing || !idx["../cine/b.mkv"].Missing || idx["../cine/b.mkv"].Fingerprint != "fb" {
		t.Fatalf("index %+v", idx)
	}
	// A root that was not scanned (e.g. not mounted) must not lose its files.
	if err := s.SyncFiles(nil, []string{"../cine-ordenar"}); err != nil {
		t.Fatal(err)
	}
	if idx, _ := s.FileIndex(); idx["../cine/a.mkv"].Missing {
		t.Fatal("files under unscanned roots must keep their state")
	}
}

func TestReplaceVersionsAndRead(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{
		{Path: "../cine/x/1900 - Part 1.mkv", Size: 10, MTime: 1, Fingerprint: "p1", Kind: "video"},
		{Path: "../cine/x/1900 - Part 2.mkv", Size: 12, MTime: 1, Fingerprint: "p2", Kind: "video"},
		{Path: "../cine/x/1900.spa.srt", Size: 1, MTime: 1, Kind: "subtitle"},
	}, roots)
	err := s.ReplaceVersions([]grouping.Version{{
		Dir:    "../cine/x",
		Parsed: nameparse.Parsed{Title: "1900", Year: 1976, Director: "Bernardo Bertolucci", Resolution: "1080p"},
		Size:   22,
		Parts:  2,
		Members: []grouping.Member{
			{Path: "../cine/x/1900 - Part 1.mkv", Role: grouping.RoleMain, Part: 1},
			{Path: "../cine/x/1900 - Part 2.mkv", Role: grouping.RoleMain, Part: 2},
			{Path: "../cine/x/1900.spa.srt", Role: grouping.RoleSubtitle, Lang: "es"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d versions", len(vs))
	}
	v := vs[0]
	if v.Title != "1900" || v.Year != 1976 || v.Parts != 2 || v.Size != 22 || v.SubLangs != "es" || len(v.Files) != 3 {
		t.Fatalf("got %+v", v)
	}
	if v.Files[0].Role != "main" || v.Files[0].Part != 1 || v.Files[2].Role != "subtitle" {
		t.Fatalf("files %+v", v.Files)
	}
	// Replacing again must not duplicate.
	s.ReplaceVersions(nil)
	if vs, _ := s.Versions(); len(vs) != 0 {
		t.Fatalf("expected empty, got %d", len(vs))
	}
}

func TestReplaceVersionsRejectsUnknownMemberPath(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{
		{Path: "../cine/x/1900 - Part 1.mkv", Size: 10, MTime: 1, Fingerprint: "p1", Kind: "video"},
	}, roots)
	err := s.ReplaceVersions([]grouping.Version{{
		Dir:    "../cine/x",
		Parsed: nameparse.Parsed{Title: "1900", Year: 1976},
		Size:   10,
		Parts:  1,
		Members: []grouping.Member{
			{Path: "../cine/x/1900 - Part 1.mkv", Role: grouping.RoleMain, Part: 1},
			// Never passed to SyncFiles: simulates grouping/scan drift.
			{Path: "../cine/x/1900 - Part 2.mkv", Role: grouping.RoleMain, Part: 2},
		},
	}})
	if err == nil {
		t.Fatal("expected error for version member referencing unknown file")
	}
}

func TestDuplicatesAndHasFile(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{
		{Path: "../cine/a.mkv", Size: 100, MTime: 1, Fingerprint: "same", Kind: "video"},
		{Path: "../cine-ordenar/a-copy.mkv", Size: 100, MTime: 1, Fingerprint: "same", Kind: "video"},
		{Path: "../cine/b.mkv", Size: 50, MTime: 1, Fingerprint: "other", Kind: "video"},
	}, roots)
	d, err := s.Duplicates()
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != 1 || d[0].Size != 100 || len(d[0].Paths) != 2 {
		t.Fatalf("got %+v", d)
	}
	if ok, _ := s.HasFile("../cine/a.mkv"); !ok {
		t.Fatal("known file")
	}
	if ok, _ := s.HasFile("../../etc/passwd"); ok {
		t.Fatal("unknown file")
	}
}

func TestOpenMemory(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if vs, err := s.Versions(); err != nil || len(vs) != 0 {
		t.Fatalf("got %v, %v", vs, err)
	}
}
