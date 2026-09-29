package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestImportAndDismissCollections(t *testing.T) {
	s := listStore(t)
	const kubrick, otra = "../cine/Collections/Kubrick", "../cine/Collections/Otra"
	id, err := s.ImportCollection(kubrick, " Kubrick ", 0, []string{"k1", "k2"})
	if err != nil || id == 0 {
		t.Fatalf("import %d, %v", id, err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	d := snap.CollectionFolders[kubrick]
	if d.Status != CollectionImported || d.ListID != id || len(d.Seen) != 2 || !d.Seen["k1"] || !d.Seen["k2"] {
		t.Fatalf("decision %+v", d)
	}
	if l := snap.Lists[0]; l.Name != "Kubrick" || len(l.Entries) != 2 || l.Entries[0].Ref != "fp:k1" || l.Entries[1].Ref != "fp:k2" {
		t.Fatalf("list %+v", l)
	}

	// New contents go to the same list; declining others keeps the import.
	if _, err := s.ImportCollection(kubrick, "", id, []string{"k3"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DismissCollection(kubrick, []string{"k4"}); err != nil {
		t.Fatal(err)
	}
	snap, _ = s.Snapshot()
	if d := snap.CollectionFolders[kubrick]; d.Status != CollectionImported || d.ListID != id || len(d.Seen) != 4 {
		t.Fatalf("decision %+v", d)
	}
	if n := len(snap.Lists[0].Entries); n != 3 {
		t.Fatalf("entries %d", n)
	}

	// A taken name or an unknown list fail and record nothing.
	if _, err := s.ImportCollection(otra, "kubrick", 0, []string{"o1"}); !errors.Is(err, ErrListExists) {
		t.Fatalf("taken name: %v", err)
	}
	if _, err := s.ImportCollection(otra, "", 999, []string{"o1"}); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("unknown list: %v", err)
	}
	snap, _ = s.Snapshot()
	if _, ok := snap.CollectionFolders[otra]; ok || len(snap.Lists) != 1 {
		t.Fatalf("recorded a failed import: %+v", snap.CollectionFolders)
	}

	// Dismissing a folder never imported; deleting a list leaves its import
	// without a list.
	if err := s.DismissCollection(otra, []string{"o1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteList(id); err != nil {
		t.Fatal(err)
	}
	snap, _ = s.Snapshot()
	if d := snap.CollectionFolders[kubrick]; d.Status != CollectionImported || d.ListID != 0 {
		t.Fatalf("kubrick %+v", d)
	}
	if d := snap.CollectionFolders[otra]; d.Status != CollectionDismissed || d.ListID != 0 || !d.Seen["o1"] {
		t.Fatalf("otra %+v", d)
	}
}

func TestSnapshotOfCatalogWithoutLists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cinexplorer.db")
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"collection_seen", "collection_folders", "list_entries", "lists"} {
		if _, err := s.db.Exec(`DROP TABLE ` + table); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	snap, err := ro.Snapshot()
	if err != nil || snap.Lists == nil || len(snap.Lists) != 0 || snap.CollectionFolders == nil || len(snap.CollectionFolders) != 0 {
		t.Fatalf("snapshot %+v, %v", snap, err)
	}
}
