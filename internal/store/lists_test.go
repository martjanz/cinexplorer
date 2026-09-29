package store

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func listStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func snapLists(t *testing.T, s *Store) []List {
	t.Helper()
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return snap.Lists
}

func TestListName(t *testing.T) {
	long := strings.Repeat("á", 100)
	for in, want := range map[string]string{"  Noir ": "Noir", "Películas de Fellini": "Películas de Fellini", long: long} {
		if got, err := ListName(in); err != nil || got != want {
			t.Errorf("ListName(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "   ", strings.Repeat("a", 101)} {
		if _, err := ListName(in); !errors.Is(err, ErrListName) {
			t.Errorf("ListName(%q): %v", in, err)
		}
	}
}

func TestCreateRenameDeleteList(t *testing.T) {
	s := listStore(t)
	noir, err := s.CreateList(" Noir ")
	if err != nil || noir.ID == 0 || noir.Name != "Noir" || noir.CreatedAt == 0 || noir.UpdatedAt != noir.CreatedAt {
		t.Fatalf("create %+v, %v", noir, err)
	}
	if _, err := s.CreateList("NOIR"); !errors.Is(err, ErrListExists) {
		t.Fatalf("repeated name: %v", err)
	}
	if _, err := s.CreateList(""); !errors.Is(err, ErrListName) {
		t.Fatalf("empty name: %v", err)
	}
	// Case is ignored beyond ASCII too.
	if _, err := s.CreateList("Películas"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateList("PELÍCULAS"); !errors.Is(err, ErrListExists) {
		t.Fatalf("repeated accented name: %v", err)
	}
	if err := s.RenameList(noir.ID, "Cine negro"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameList(noir.ID, "cine NEGRO"); err != nil {
		t.Fatalf("its own name in other case: %v", err)
	}
	if err := s.RenameList(noir.ID, "películas"); !errors.Is(err, ErrListExists) {
		t.Fatalf("rename to a taken name: %v", err)
	}
	if err := s.RenameList(noir.ID, " "); !errors.Is(err, ErrListName) {
		t.Fatalf("rename to nothing: %v", err)
	}
	if err := s.RenameList(999, "Otra"); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("rename unknown: %v", err)
	}
	ls := snapLists(t, s)
	if len(ls) != 2 || ls[0].Name != "cine NEGRO" || ls[1].Name != "Películas" || len(ls[0].Entries) != 0 {
		t.Fatalf("lists %+v", ls)
	}
	if err := s.DeleteList(noir.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteList(noir.ID); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("delete twice: %v", err)
	}
	if ls := snapLists(t, s); len(ls) != 1 {
		t.Fatalf("lists %+v", ls)
	}
}

func TestListEntries(t *testing.T) {
	s := listStore(t)
	l, err := s.CreateList("Noir")
	if err != nil {
		t.Fatal(err)
	}
	if RefMovie(7857) != "movie:7857" || RefFingerprint("s1") != "fp:s1" {
		t.Fatalf("refs %q %q", RefMovie(7857), RefFingerprint("s1"))
	}
	if err := s.AddEntries(l.ID, []string{RefMovie(7857), RefFingerprint("s1")}); err != nil {
		t.Fatal(err)
	}
	first := snapLists(t, s)[0].Entries
	if len(first) != 2 || first[0].AddedAt == 0 {
		t.Fatalf("entries %+v", first)
	}
	// Adding again changes nothing: the entries keep their date.
	if err := s.AddEntries(l.ID, []string{RefMovie(7857)}); err != nil {
		t.Fatal(err)
	}
	if got := snapLists(t, s)[0].Entries; !reflect.DeepEqual(got, first) {
		t.Fatalf("entries %+v, want %+v", got, first)
	}
	if err := s.RemoveEntries(l.ID, []string{RefFingerprint("s1"), RefFingerprint("nada")}); err != nil {
		t.Fatal(err)
	}
	if got := snapLists(t, s)[0].Entries; len(got) != 1 || got[0].Ref != "movie:7857" {
		t.Fatalf("entries %+v", got)
	}
	if err := s.AddEntries(999, []string{RefMovie(1)}); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("add to unknown: %v", err)
	}
	if err := s.RemoveEntries(999, nil); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("remove from unknown: %v", err)
	}
	// Deleting the list deletes its entries.
	if err := s.DeleteList(l.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM list_entries`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("entries left %d, %v", n, err)
	}
}
