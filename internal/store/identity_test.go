package store

import (
	"errors"
	"reflect"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
)

// identityCatalog has: a two-part movie, a DVD, an identical copy of the
// two-part movie's first part in another folder, and a .nfo next to the DVD.
func identityCatalog(t *testing.T) *Store {
	t.Helper()
	s := open(t)
	if err := s.SyncFiles([]FileRow{
		{Path: "../cine/a/Amarcord CD1.avi", Size: 700, MTime: 1, Fingerprint: "a1", Kind: "video"},
		{Path: "../cine/a/Amarcord CD2.avi", Size: 710, MTime: 1, Fingerprint: "a2", Kind: "video"},
		{Path: "../cine/copy/Amarcord.avi", Size: 700, MTime: 1, Fingerprint: "a1", Kind: "video"},
		{Path: "../cine/d/VIDEO_TS/VTS_01_0.IFO", Size: 10, MTime: 1, Fingerprint: "ifo", Kind: "dvd"},
		{Path: "../cine/d/VIDEO_TS/VTS_01_1.VOB", Size: 900, MTime: 1, Fingerprint: "vob1", Kind: "dvd"},
		{Path: "../cine/d/VIDEO_TS/VTS_01_2.VOB", Size: 950, MTime: 1, Fingerprint: "vob2", Kind: "dvd"},
		{Path: "../cine/d/movie.nfo", Size: 1, MTime: 1, Kind: "info"},
		{Path: "../cine/d/b.nfo", Size: 1, MTime: 1, Kind: "info"},
	}, roots); err != nil {
		t.Fatal(err)
	}
	main := func(p string, part int) grouping.Member {
		return grouping.Member{Path: p, Role: grouping.RoleMain, Part: part}
	}
	if err := s.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/a", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973, Director: "Fellini"}, Size: 1410, Parts: 2,
			Members: []grouping.Member{main("../cine/a/Amarcord CD1.avi", 1), main("../cine/a/Amarcord CD2.avi", 2)}},
		{Dir: "../cine/copy", Parsed: nameparse.Parsed{Title: "Amarcord"}, Size: 700, Parts: 1,
			Members: []grouping.Member{main("../cine/copy/Amarcord.avi", 0)}},
		{Dir: "../cine/d", Parsed: nameparse.Parsed{Title: "Stalker", IMDbID: "tt0079944"}, Size: 1860, Parts: 1,
			Members: []grouping.Member{main("../cine/d/VIDEO_TS/VTS_01_0.IFO", 0), main("../cine/d/VIDEO_TS/VTS_01_1.VOB", 0),
				main("../cine/d/VIDEO_TS/VTS_01_2.VOB", 0)}},
	}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIdentifyTargets(t *testing.T) {
	s := identityCatalog(t)
	ts, err := s.IdentifyTargets()
	if err != nil {
		t.Fatal(err)
	}
	want := []IdentifyTarget{
		{Fingerprint: "a1", Dir: "../cine/a", Title: "Amarcord", Year: 1973, Director: "Fellini"},
		{Fingerprint: "vob2", Dir: "../cine/d", Title: "Stalker", IMDbID: "tt0079944", NFOs: []string{"../cine/d/b.nfo", "../cine/d/movie.nfo"}},
	}
	if !reflect.DeepEqual(ts, want) {
		t.Fatalf("got %+v\nwant %+v", ts, want)
	}
}

func TestSaveIdentificationsKeepsCorrections(t *testing.T) {
	s := identityCatalog(t)
	auto := Identification{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857, Confidence: 0.9, Query: "q", MatcherVersion: 1,
		Candidates: []Candidate{{TMDBID: 7857, Title: "Amarcord", Year: 1973, Score: 0.9}}}
	if err := s.SaveIdentifications([]Identification{auto}); err != nil {
		t.Fatal(err)
	}
	ts, _ := s.IdentifyTargets()
	if c := ts[0].Current; c == nil || !reflect.DeepEqual(*c, auto) {
		t.Fatalf("current %+v", ts[0].Current)
	}

	if err := s.SetCorrection("a1", StatusManual, 42); err != nil {
		t.Fatal(err)
	}
	// A matcher result computed before the correction must not undo it.
	if err := s.SaveIdentifications([]Identification{auto}); err != nil {
		t.Fatal(err)
	}
	ts, _ = s.IdentifyTargets()
	if c := ts[0].Current; c.Status != StatusManual || c.TMDBID != 42 || c.Query != "q" || len(c.Candidates) != 1 {
		t.Fatalf("after correction %+v", c)
	}

	if err := s.ResetIdentification("a1"); err != nil {
		t.Fatal(err)
	}
	if ts, _ = s.IdentifyTargets(); ts[0].Current != nil {
		t.Fatalf("after reset %+v", ts[0].Current)
	}
}

func TestSetCorrectionValidates(t *testing.T) {
	s := identityCatalog(t)
	if err := s.SetCorrection("nope", StatusIgnored, 0); !errors.Is(err, ErrUnknownFingerprint) {
		t.Errorf("unknown fingerprint: %v", err)
	}
	if err := s.SetCorrection("", StatusIgnored, 0); !errors.Is(err, ErrUnknownFingerprint) {
		t.Errorf("empty fingerprint: %v", err)
	}
	if err := s.SetCorrection("a1", StatusAuto, 1); err == nil {
		t.Error("auto accepted as a correction")
	}
}

func TestMoviesRoundTripAndEnrichQueue(t *testing.T) {
	s := identityCatalog(t)
	m := Movie{TMDBID: 7857, Title: "Amarcord", OriginalTitle: "Amarcord", Year: 1973, Runtime: 123, OriginalLang: "it",
		Overview: "Rimini.", Directors: []Person{{ID: 4415, Name: "Federico Fellini"}},
		Cast: []CastMember{{ID: 1, Name: "Magali Noël", Character: "Gradisca"}}, Genres: []string{"Comedia"},
		Countries: []string{"IT", "FR"}, PosterPath: "/p.jpg", BackdropPath: "/b.jpg", IMDbID: "tt0071129", Language: "es-ES"}
	if err := s.SaveMovie(m); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Movie(7857)
	if err != nil || !ok || !reflect.DeepEqual(got, m) {
		t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, _ := s.Movie(1); ok {
		t.Fatal("unknown movie found")
	}

	s.SaveIdentifications([]Identification{
		{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857},
		{Fingerprint: "vob2", Status: StatusAuto, TMDBID: 1398},
	})
	s.SetCorrection("a2", StatusExtra, 500)
	ids, err := s.MoviesToEnrich("es-ES")
	if err != nil || !reflect.DeepEqual(ids, []int{500, 1398}) {
		t.Fatalf("to enrich %v %v", ids, err)
	}
	if ids, _ := s.MoviesToEnrich("en-US"); !reflect.DeepEqual(ids, []int{500, 1398, 7857}) {
		t.Fatalf("other language %v", ids)
	}

	pending, err := s.PendingWikidata(10)
	if err != nil || len(pending) != 1 || pending[0].TMDBID != 7857 {
		t.Fatalf("wikidata %+v %v", pending, err)
	}
	m.WikidataDone, m.WikidataID = true, "Q18428"
	s.SaveMovie(m)
	if pending, _ := s.PendingWikidata(10); len(pending) != 0 {
		t.Fatalf("wikidata after %+v", pending)
	}
	if all, _ := s.Movies(); len(all) != 1 {
		t.Fatalf("movies %+v", all)
	}
}

func TestInvalidateMovie(t *testing.T) {
	s := identityCatalog(t)
	s.SaveMovie(Movie{TMDBID: 7857, Title: "Amarcord", Language: "es-ES"})
	s.SaveIdentifications([]Identification{{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857}})
	s.SetCorrection("vob2", StatusManual, 7857)
	if err := s.InvalidateMovie(7857); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Movie(7857); ok {
		t.Fatal("movie kept")
	}
	ts, _ := s.IdentifyTargets()
	if ts[0].Current != nil || ts[1].Current == nil || ts[1].Current.Status != StatusManual {
		t.Fatalf("targets %+v %+v", ts[0].Current, ts[1].Current)
	}
}
