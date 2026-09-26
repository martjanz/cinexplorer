package store

import (
	"path/filepath"
	"testing"
)

func TestSnapshot(t *testing.T) {
	s := identityCatalog(t)
	s.SaveMovie(Movie{TMDBID: 7857, Title: "Amarcord", Year: 1973, Genres: []string{"Comedia"}, Language: "es-ES"})
	s.SaveIdentifications([]Identification{
		{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857, Confidence: 0.93},
		{Fingerprint: "vob2", Status: StatusUnmatched, Candidates: []Candidate{{TMDBID: 1398, Title: "Stalker", Score: 0.7}}},
	})
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	vs, _ := s.Versions()
	if len(snap.Versions) != len(vs) || len(vs) != 3 {
		t.Fatalf("versions %d, want %d", len(snap.Versions), len(vs))
	}
	for i := range vs {
		if snap.Versions[i].ID != vs[i].ID || snap.Versions[i].Best != vs[i].Best || snap.Versions[i].Fingerprint != vs[i].Fingerprint {
			t.Errorf("version %d: %+v, want %+v", i, snap.Versions[i], vs[i])
		}
	}
	if m := snap.Movies[7857]; m.Title != "Amarcord" || m.Genres[0] != "Comedia" || len(snap.Movies) != 1 {
		t.Fatalf("movies %+v", snap.Movies)
	}
	if id := snap.Identifications["vob2"]; id == nil || id.Candidates[0].TMDBID != 1398 || len(snap.Identifications) != 2 {
		t.Fatalf("identifications %+v", snap.Identifications)
	}
}

func TestSnapshotOfOlderReadOnlyCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cinexplorer.db")
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	s.db.Exec(`DROP TABLE identifications`)
	s.db.Exec(`DROP TABLE movies`)
	s.Close()
	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	snap, err := ro.Snapshot()
	if err != nil || len(snap.Versions) != 0 || len(snap.Movies) != 0 || len(snap.Identifications) != 0 {
		t.Fatalf("snapshot %+v, %v", snap, err)
	}
}
