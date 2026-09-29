package catalog

import (
	"reflect"
	"slices"
	"testing"

	"cinexplorer/internal/store"
)

// withLists adds three lists to the fixture snapshot (see fixture_test.go).
func withLists(snap store.Snapshot) store.Snapshot {
	snap.Lists = []store.List{
		{ID: 1, Name: "Fellini", UpdatedAt: 20, Entries: []store.ListEntry{
			{Ref: "fp:a2", AddedAt: 5},      // Amarcord, through one of its contents
			{Ref: "movie:7857", AddedAt: 3}, // Amarcord again: counted once, added at 3
			{Ref: "movie:7858", AddedAt: 7},
			{Ref: "fp:e1", AddedAt: 8},   // an extra: leads nowhere
			{Ref: "fp:g1", AddedAt: 9},   // missing from disk
			{Ref: "movie:1", AddedAt: 9}, // not stored
		}},
		{ID: 2, Name: "Por ver", UpdatedAt: 30, Entries: []store.ListEntry{
			{Ref: "fp:s1", AddedAt: 1}, // Stalker, never identified
			{Ref: "fp:p1", AddedAt: 2}, // Solaris, identified as a movie not stored
		}},
		{ID: 3, Name: "vacía", UpdatedAt: 10},
	}
	return snap
}

func TestListsOnItems(t *testing.T) {
	got := map[string][]inList{}
	for _, it := range Items(withLists(snapshot()), roots) {
		if len(it.lists) > 0 {
			got[it.id()] = it.lists
		}
	}
	want := map[string][]inList{
		"movie:7857": {{id: 1, name: "Fellini", added: 3}},
		"movie:7858": {{id: 1, name: "Fellini", added: 7}},
		"s1":         {{id: 2, name: "Por ver", added: 1}},
		"p1":         {{id: 2, name: "Por ver", added: 2}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lists %+v", got)
	}
}

func TestListEntryFollowsIdentification(t *testing.T) {
	snap := withLists(snapshot())
	for i := range snap.Versions {
		if snap.Versions[i].Fingerprint == "s1" {
			snap.Versions[i] = identified(snap.Versions[i], store.StatusManual, 7858)
		}
	}
	d, _ := Movie(snap, 7858)
	if want := []ListRef{{1, "Fellini"}, {2, "Por ver"}}; !reflect.DeepEqual(d.Lists, want) {
		t.Fatalf("lists %+v", d.Lists)
	}
}

func TestDetailLists(t *testing.T) {
	snap := withLists(snapshot())
	if d, _ := Movie(snap, 7857); !reflect.DeepEqual(d.Lists, []ListRef{{1, "Fellini"}}) {
		t.Fatalf("movie %+v", d.Lists)
	}
	if d, _ := Version(snap, "s1"); !reflect.DeepEqual(d.Lists, []ListRef{{2, "Por ver"}}) {
		t.Fatalf("version %+v", d.Lists)
	}
	if d, _ := Version(snap, "n1"); d.Lists == nil || len(d.Lists) != 0 {
		t.Fatalf("no lists %+v", d.Lists)
	}
}

func TestItemRef(t *testing.T) {
	snap := snapshot()
	cases := []struct {
		tmdbID int
		key    string
		want   string
		ok     bool
	}{
		{7857, "", "movie:7857", true},
		{1, "", "", false},       // not stored
		{0, "s1", "fp:s1", true}, // a present content
		{0, "g1", "", false},     // missing from disk
		{0, "id:9", "", false},   // no fingerprint
		{0, "", "", false},
	}
	for _, c := range cases {
		if got, ok := ItemRef(snap, c.tmdbID, c.key); got != c.want || ok != c.ok {
			t.Errorf("ItemRef(%d, %q) = %q, %v", c.tmdbID, c.key, got, ok)
		}
	}
}

func TestRefsTo(t *testing.T) {
	snap := withLists(snapshot())
	got := RefsTo(snap, 1, 7857, "")
	slices.Sort(got)
	if want := []string{"fp:a2", "movie:7857"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("amarcord %v", got)
	}
	if got := RefsTo(snap, 2, 0, "s1"); !reflect.DeepEqual(got, []string{"fp:s1"}) {
		t.Fatalf("stalker %v", got)
	}
	if got := RefsTo(snap, 1, 0, "s1"); len(got) != 0 {
		t.Fatalf("other list %v", got)
	}
	// An entry whose content is gone can still be removed by its own ref.
	if got := RefsTo(snap, 1, 0, "g1"); !reflect.DeepEqual(got, []string{"fp:g1"}) {
		t.Fatalf("gone %v", got)
	}
}

func TestListCards(t *testing.T) {
	snap := withLists(snapshot())
	roma := snap.Movies[7858]
	roma.BackdropPath = "/r.jpg"
	snap.Movies[7858] = roma
	want := []ListCard{
		{ID: 2, Name: "Por ver", Count: 2, UpdatedAt: 30},
		{ID: 1, Name: "Fellini", Count: 2, UpdatedAt: 20, Cover: &Cover{TMDBID: 7858, Backdrop: "r"}},
		{ID: 3, Name: "vacía", Count: 0, UpdatedAt: 10},
	}
	if got := ListCards(snap); !reflect.DeepEqual(got, want) {
		t.Fatalf("cards %+v", got)
	}
	if got := ListCards(snapshot()); got == nil || len(got) != 0 {
		t.Fatalf("no lists %+v", got)
	}
}
