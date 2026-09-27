package store

import "testing"

func TestMarkOutsideRoots(t *testing.T) {
	s := open(t)
	rows := []FileRow{
		{Path: "../cine/a.mkv", Size: 10, MTime: 1, Fingerprint: "fa", Kind: "video"},
		{Path: "../cine-ordenar/b.mkv", Size: 20, MTime: 2, Fingerprint: "fb", Kind: "video"},
		{Path: "../otro/c.mkv", Size: 30, MTime: 3, Fingerprint: "fc", Kind: "video"},
	}
	if err := s.SyncFiles(rows, []string{"../cine", "../cine-ordenar", "../otro"}); err != nil {
		t.Fatal(err)
	}
	// ../otro was removed; ../cine-ordenar is configured (maybe unplugged);
	// "./../cine/" is ../cine written another way.
	if err := s.MarkOutsideRoots([]string{"./../cine/", "../cine-ordenar"}); err != nil {
		t.Fatal(err)
	}
	idx, err := s.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	if idx["../cine/a.mkv"].Missing || idx["../cine-ordenar/b.mkv"].Missing || !idx["../otro/c.mkv"].Missing {
		t.Fatalf("index %+v", idx)
	}
	// Adding the root again brings its files back.
	if err := s.SyncFiles(rows[2:], []string{"../otro"}); err != nil {
		t.Fatal(err)
	}
	if idx, _ := s.FileIndex(); idx["../otro/c.mkv"].Missing {
		t.Fatal("c.mkv still missing")
	}
}

func TestSnapshotChanges(t *testing.T) {
	s := open(t)
	a, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Snapshot()
	if a.Changes != b.Changes {
		t.Fatalf("no writes, changes %d → %d", a.Changes, b.Changes)
	}
	if err := s.SyncFiles([]FileRow{{Path: "../cine/a.mkv", Size: 1, MTime: 1, Kind: "video"}}, roots); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Snapshot()
	if c.Changes == b.Changes {
		t.Fatalf("a write left changes at %d", c.Changes)
	}
	if n, err := s.Changes(); err != nil || n != c.Changes {
		t.Fatalf("Changes() = %d, %v; snapshot %d", n, err, c.Changes)
	}
}
