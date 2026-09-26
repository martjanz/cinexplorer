package store

import (
	"path/filepath"
	"testing"
)

func TestVersionsIdentityAndBestByMovie(t *testing.T) {
	s := identityCatalog(t)
	s.SaveMovie(Movie{TMDBID: 7857, Title: "Amarcord", Year: 1973, Directors: []Person{{ID: 4415, Name: "Federico Fellini"}}, Language: "es-ES"})
	s.SaveIdentifications([]Identification{
		{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857, Confidence: 0.93},
		{Fingerprint: "vob2", Status: StatusUnmatched, Candidates: []Candidate{{TMDBID: 1398, Title: "Stalker", Score: 0.7}}},
	})
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	byDir := map[string]VersionView{}
	for _, v := range vs {
		byDir[v.Dir] = v
	}
	a, c, d := byDir["../cine/a"], byDir["../cine/copy"], byDir["../cine/d"]
	if a.Fingerprint != "a1" || a.Identification == nil || a.Identification.Confidence != 0.93 ||
		a.Movie == nil || a.Movie.Title != "Amarcord" || a.Movie.Directors[0].Name != "Federico Fellini" {
		t.Fatalf("a %+v %+v %+v", a, a.Identification, a.Movie)
	}
	// The copy shares a1: same movie, so the two versions compete for best
	// although the copy's parsed title lacks the year.
	if c.Movie == nil || a.Best == c.Best {
		t.Fatalf("best a=%v copy=%v", a.Best, c.Best)
	}
	if !a.Best {
		t.Fatal("the larger two-part version should be best")
	}
	if d.Movie != nil || d.Identification.Status != StatusUnmatched {
		t.Fatalf("d %+v", d.Identification)
	}

	un, err := s.Unidentified()
	if err != nil || len(un) != 1 || un[0].Dir != "../cine/d" || len(un[0].Candidates) != 1 || un[0].Candidates[0].TMDBID != 1398 {
		t.Fatalf("unidentified %+v %v", un, err)
	}
}

func TestOlderReadOnlyCatalogHasNoIdentity(t *testing.T) {
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
	if _, err := ro.Versions(); err != nil {
		t.Fatal(err)
	}
	if un, err := ro.Unidentified(); err != nil || len(un) != 0 {
		t.Fatalf("unidentified %v %v", un, err)
	}
	if ts, err := ro.IdentifyTargets(); err != nil || ts != nil {
		t.Fatalf("targets %v %v", ts, err)
	}
	if _, ok, err := ro.Movie(1); ok || err != nil {
		t.Fatalf("movie %v %v", ok, err)
	}
}
