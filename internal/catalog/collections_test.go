package catalog

import (
	"reflect"
	"testing"

	"cinexplorer/internal/store"
)

// collectionsSnapshot adds folders of Collections/ to the fixture, which
// already has an SD Amarcord (fingerprint a2) in ../cine/Collections/Fellini.
func collectionsSnapshot() store.Snapshot {
	snap := snapshot()
	gone := version(24, "../cine/Collections/Nada/Gone", "Gone.avi", "g2", "Gone", 1990, "", 10, 900)
	gone.Files[0].Missing = true
	snap.Versions = append(snap.Versions,
		identified(version(20, "../cine/Collections/Fellini/Roma (1972)", "Roma.avi", "r2", "Roma", 1972, "SD", 600, 900), store.StatusAuto, 7858),
		version(21, "../cine/collections/Kubrick", "The Shining.avi", "k1", "The Shining", 1980, "", 700, 900),
		identified(version(22, "../cine/collections/Kubrick/Extras", "Trailer.avi", "k2", "Trailer", 0, "", 10, 900), store.StatusExtra, 7857),
		version(23, "../cine/Collections", "Suelta.avi", "c0", "Suelta", 0, "", 10, 900), // not in a subfolder
		gone,
		version(25, "../cine/Collections/Wong/Collections/Otra", "Chungking.avi", "w1", "Chungking", 1994, "", 10, 900),
	)
	return snap
}

func folderSummary(fs []CollectionFolder) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.Path)
	}
	return out
}

func TestCollectionsFound(t *testing.T) {
	fs := Collections(collectionsSnapshot())
	// The first Collections of a path counts, in any case; extras, missing
	// files and loose files do not.
	want := []string{"../cine/Collections/Fellini", "../cine/Collections/Wong", "../cine/collections/Kubrick"}
	if got := folderSummary(fs); !reflect.DeepEqual(got, want) {
		t.Fatalf("folders %v", got)
	}
	f := fs[0]
	if f.Name != "Fellini" || f.Total != 2 || f.New != 2 || f.List != nil || !reflect.DeepEqual(f.Fingerprints, []string{"a2", "r2"}) {
		t.Fatalf("fellini %+v", f)
	}
	if len(f.Preview) != 2 || f.Preview[0].TMDBID != 7857 || f.Preview[1].TMDBID != 7858 {
		t.Fatalf("preview %+v", f.Preview)
	}
	if k := fs[2]; k.Name != "Kubrick" || k.Total != 1 || !reflect.DeepEqual(k.Fingerprints, []string{"k1"}) || k.Preview[0].Key != "k1" {
		t.Fatalf("kubrick %+v", k)
	}
}

func TestCollectionsDecided(t *testing.T) {
	snap := collectionsSnapshot()
	snap.Lists = []store.List{{ID: 4, Name: "Fellini"}}
	snap.CollectionFolders = map[string]store.CollectionDecision{
		"../cine/Collections/Fellini": {Status: store.CollectionImported, ListID: 4, Seen: map[string]bool{"a2": true}},
		"../cine/Collections/Wong":    {Status: store.CollectionDismissed, Seen: map[string]bool{}},
		"../cine/collections/Kubrick": {Status: store.CollectionImported, ListID: 0, Seen: map[string]bool{}}, // its list was deleted
	}
	fs := Collections(snap)
	if len(fs) != 1 {
		t.Fatalf("folders %v", folderSummary(fs))
	}
	f := fs[0]
	if f.Total != 2 || f.New != 1 || f.List == nil || *f.List != (ListRef{4, "Fellini"}) ||
		!reflect.DeepEqual(f.Fingerprints, []string{"r2"}) || len(f.Preview) != 1 || f.Preview[0].TMDBID != 7858 {
		t.Fatalf("fellini %+v", f)
	}
	// Once everything was offered, nothing is pending.
	snap.CollectionFolders["../cine/Collections/Fellini"].Seen["r2"] = true
	if fs := Collections(snap); len(fs) != 0 {
		t.Fatalf("folders %v", folderSummary(fs))
	}
}

func TestCollectionsEmpty(t *testing.T) {
	if fs := Collections(store.Snapshot{}); fs == nil || len(fs) != 0 {
		t.Fatalf("folders %+v", fs)
	}
}
