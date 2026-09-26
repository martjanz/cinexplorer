package catalog

import (
	"testing"

	"cinexplorer/internal/store"
)

func TestMovie(t *testing.T) {
	snap := snapshot()
	// A version of Amarcord whose file is gone, and a trailer found with the
	// best version.
	gone := identified(version(13, "../cine/old", "Amarcord.avi", "a9", "Amarcord", 1973, "2160p", 20000, 10), store.StatusAuto, 7857)
	gone.Files[0].Missing = true
	snap.Versions = append(snap.Versions, gone)
	snap.Versions[0].Files = append(snap.Versions[0].Files, store.FileView{Path: "../cine/1970s/Amarcord/trailer.mkv", Size: 30, Role: "extra"})

	d, ok := Movie(snap, 7857)
	if !ok || d.Movie.Title != "Amarcord" || d.Poster != "pa" || d.Backdrop != "" {
		t.Fatalf("movie %+v %v", d, ok)
	}
	var got []int64
	for _, v := range d.Versions {
		got = append(got, v.ID)
	}
	// Best first; the missing 4K last; the two SD copies in between.
	if len(got) != 4 || got[0] != 1 || got[3] != 13 || d.Versions[1].Copies != 2 || d.Versions[0].Copies != 1 || d.Versions[3].Copies != 0 {
		t.Fatalf("versions %v %+v", got, d.Versions)
	}
	if len(d.Extras) != 1 || d.Extras[0].Path != "../cine/1970s/Amarcord/trailer.mkv" || d.Extras[0].Size != 30 {
		t.Errorf("extras %+v", d.Extras)
	}
	if len(d.ExtraVersions) != 1 || d.ExtraVersions[0].ID != 4 {
		t.Errorf("extra versions %+v", d.ExtraVersions)
	}
	if _, ok := Movie(snap, 555); ok {
		t.Error("movie not stored found")
	}
}

func TestVersion(t *testing.T) {
	snap := snapshot()
	d, ok := Version(snap, "n1")
	if !ok || d.Title != "Novecento" || d.Year != 1976 || len(d.Versions) != 1 || d.Movie != nil ||
		d.Identification == nil || d.Identification.Status != store.StatusUnmatched || len(d.Identification.Candidates) != 1 {
		t.Fatalf("novecento %+v %v", d, ok)
	}
	if d, ok := Version(snap, "s1"); !ok || d.Identification != nil {
		t.Errorf("never identified %+v", d)
	}
	if d, ok := Version(snap, "a2"); !ok || len(d.Versions) != 2 || d.Versions[0].Copies != 2 {
		t.Errorf("copies %+v", d)
	}
	if d, ok := Version(snap, "id:9"); !ok || d.Title != "Vacío" || d.Identification != nil {
		t.Errorf("unhashed %+v", d)
	}
	for _, k := range []string{"zz", "id:99", ""} {
		if _, ok := Version(snap, k); ok {
			t.Errorf("%q found", k)
		}
	}
}

func TestSuggestMovies(t *testing.T) {
	snap := snapshot()
	titles := func(refs []store.MovieRef) []string {
		out := []string{}
		for _, r := range refs {
			out = append(out, r.Title)
		}
		return out
	}
	// The making-of lives in Amarcord/Extras: Amarcord comes first.
	snap.Versions[3].Identification = nil
	if got := titles(SuggestMovies(snap, "", "e1")); len(got) != 2 || got[0] != "Amarcord" || got[1] != "Roma" {
		t.Errorf("near e1: %v", got)
	}
	// A trailer in Roma/Extras puts Roma first.
	snap.Versions = append(snap.Versions, version(14, "../cine/1970s/Roma/Extras", "Trailer.avi", "t1", "Trailer", 0, "", 5, 1))
	if got := titles(SuggestMovies(snap, "", "t1")); len(got) != 2 || got[0] != "Roma" {
		t.Errorf("near t1: %v", got)
	}
	if got := titles(SuggestMovies(snap, "ROMÁ", "")); len(got) != 1 || got[0] != "Roma" {
		t.Errorf("q: %v", got)
	}
	if got := SuggestMovies(snap, "nada", ""); got == nil || len(got) != 0 {
		t.Errorf("no match: %v", got)
	}
}

func TestUnidentified(t *testing.T) {
	snap := snapshot()
	// A copy of Novecento elsewhere: still one item.
	snap.Versions = append(snap.Versions, identified(version(15, "../cine-ordenar", "Novecento.avi", "n1", "Novecento", 1976, "720p", 700, 400), store.StatusUnmatched, 0))
	rep := Unidentified(snap)
	// Waiting: only Stalker; missing and unhashed files do not count.
	if rep.Pending != 1 {
		t.Errorf("pending %d", rep.Pending)
	}
	if len(rep.Items) != 1 || rep.Items[0].Fingerprint != "n1" || len(rep.Items[0].Candidates) != 1 || rep.Items[0].ID != 6 {
		t.Fatalf("items %+v", rep.Items)
	}
}
