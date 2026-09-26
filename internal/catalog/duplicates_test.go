package catalog

import (
	"reflect"
	"testing"

	"cinexplorer/internal/store"
)

func TestDuplicates(t *testing.T) {
	snap := snapshot()
	// Stalker, not identified, is also in another folder.
	snap.Versions = append(snap.Versions, version(12, "../cine/1970s/Stalker", "Stalker.avi", "s1", "Stalker", 1979, "576p", 800, 500))
	rep := Duplicates(snap, roots)
	if len(rep.Groups) != 2 {
		t.Fatalf("groups %+v", rep.Groups)
	}
	a, s := rep.Groups[0], rep.Groups[1]
	// Amarcord: one more copy of the SD content (1400) plus the SD content,
	// which is not the best version (1400).
	if a.TMDBID != 7857 || a.Kind != KindMovie || a.Recoverable != 2800 ||
		!reflect.DeepEqual(a.Types, []string{DupIdentical, DupVersions}) {
		t.Errorf("amarcord %+v", a)
	}
	if len(a.Versions) != 3 || !a.Versions[0].Best || a.Versions[0].Path != "../cine/1970s/Amarcord/Amarcord.1973.1080p.mkv" ||
		a.Versions[1].Fingerprint != "a2" || a.Versions[2].Fingerprint != "a2" {
		t.Errorf("amarcord versions %+v", a.Versions)
	}
	if s.Key != "s1" || s.Kind != KindVersion || s.Recoverable != 800 || !reflect.DeepEqual(s.Types, []string{DupIdentical}) || len(s.Versions) != 2 {
		t.Errorf("stalker %+v", s)
	}
	if rep.Recoverable != 3600 {
		t.Errorf("total %d", rep.Recoverable)
	}
}

func TestDuplicatesNone(t *testing.T) {
	rep := Duplicates(store.Snapshot{}, roots)
	if rep.Groups == nil || len(rep.Groups) != 0 || rep.Recoverable != 0 {
		t.Fatalf("report %+v", rep)
	}
}
