package store

import (
	"reflect"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/probe"
)

// saveProbe stores a probe result for the catalogued file at path.
func saveProbe(t *testing.T, s *Store, path string, info probe.Info, errText string) {
	t.Helper()
	var r ProbeResult
	if err := s.db.QueryRow(`SELECT id, size, mtime FROM files WHERE path = ?`, path).Scan(&r.FileID, &r.Size, &r.MTime); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	r.Info, r.Err = info, errText
	if err := s.SaveProbes([]ProbeResult{r}); err != nil {
		t.Fatal(err)
	}
}

func versionByDir(t *testing.T, s *Store, dir string) VersionView {
	t.Helper()
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if v.Dir == dir {
			return v
		}
	}
	t.Fatalf("no version in %s", dir)
	return VersionView{}
}

func TestVersionsTechnicalData(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{
		{Path: "../cine/e/1900 - Part 1.mkv", Size: 10, MTime: 1, Fingerprint: "p1", Kind: "video"},
		{Path: "../cine/e/1900 - Part 2.mkv", Size: 12, MTime: 1, Fingerprint: "p2", Kind: "video"},
		{Path: "../cine/c/VIDEO_TS/VTS_01_0.IFO", Size: 10, MTime: 1, Fingerprint: "v1", Kind: "dvd"},
		{Path: "../cine/c/VIDEO_TS/VTS_01_1.VOB", Size: 900, MTime: 1, Fingerprint: "v2", Kind: "dvd"},
		{Path: "../cine/c/VIDEO_TS/VTS_02_0.IFO", Size: 10, MTime: 1, Fingerprint: "v3", Kind: "dvd"},
		{Path: "../cine/b/Stalker.avi", Size: 90, MTime: 1, Fingerprint: "s", Kind: "video"},
	}, roots)
	main := func(p string, part int) grouping.Member {
		return grouping.Member{Path: p, Role: grouping.RoleMain, Part: part}
	}
	err := s.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/e", Parsed: nameparse.Parsed{Title: "1900", Resolution: "720p"}, Size: 22, Parts: 2,
			Members: []grouping.Member{main("../cine/e/1900 - Part 1.mkv", 1), main("../cine/e/1900 - Part 2.mkv", 2)}},
		{Dir: "../cine/c", Parsed: nameparse.Parsed{Title: "c"}, Size: 920, Parts: 1, Members: []grouping.Member{
			main("../cine/c/VIDEO_TS/VTS_01_0.IFO", 0), main("../cine/c/VIDEO_TS/VTS_01_1.VOB", 0), main("../cine/c/VIDEO_TS/VTS_02_0.IFO", 0)}},
		{Dir: "../cine/b", Parsed: nameparse.Parsed{Title: "Stalker", Resolution: "576p"}, Size: 90, Parts: 1,
			Members: []grouping.Member{main("../cine/b/Stalker.avi", 0)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	es6, en2 := probe.Track{Codec: "ac3", Lang: "es", Channels: 6}, probe.Track{Codec: "aac", Lang: "en", Channels: 2}
	saveProbe(t, s, "../cine/e/1900 - Part 1.mkv", probe.Info{DurationMs: 3_600_000, Width: 1920, Height: 800, VideoCodec: "h264",
		Audio: []probe.Track{es6}, Subs: []probe.Track{{Codec: "srt", Lang: "it"}}}, "")
	saveProbe(t, s, "../cine/e/1900 - Part 2.mkv", probe.Info{DurationMs: 3_000_000, Width: 1920, Height: 800, VideoCodec: "h264",
		Audio: []probe.Track{es6, en2}}, "")
	saveProbe(t, s, "../cine/c/VIDEO_TS/VTS_01_0.IFO", probe.Info{DurationMs: 600_000, Width: 720, Height: 576, VideoCodec: "mpeg2"}, "")
	saveProbe(t, s, "../cine/c/VIDEO_TS/VTS_02_0.IFO", probe.Info{DurationMs: 6_000_000, Width: 720, Height: 480, VideoCodec: "mpeg2",
		Audio: []probe.Track{{Codec: "ac3", Lang: "fr", Channels: 2}}}, "")
	saveProbe(t, s, "../cine/b/Stalker.avi", probe.Info{}, "probe: invalid header: truncated file")

	v := versionByDir(t, s, "../cine/e")
	if v.DurationMs != 6_600_000 || v.Width != 1920 || v.Resolution != "1080p" || v.Codec != "h264" || v.VideoCodec != "h264" {
		t.Fatalf("multi-part: %+v", v)
	}
	if !reflect.DeepEqual(v.Audio, []probe.Track{es6, en2}) || !reflect.DeepEqual(v.Subs, []probe.Track{{Codec: "srt", Lang: "it"}}) {
		t.Fatalf("tracks: %+v / %+v", v.Audio, v.Subs)
	}

	dvd := versionByDir(t, s, "../cine/c")
	if dvd.DurationMs != 6_000_000 || dvd.Height != 480 || dvd.Resolution != "480p" || len(dvd.Audio) != 1 {
		t.Fatalf("dvd: %+v", dvd)
	}

	// A failed probe keeps the name-parsed data.
	if st := versionByDir(t, s, "../cine/b"); st.Width != 0 || st.DurationMs != 0 || st.Resolution != "576p" || st.Audio == nil {
		t.Fatalf("failed probe: %+v", st)
	}

	// A probe result for an older state of the file is ignored.
	s.SyncFiles([]FileRow{{Path: "../cine/e/1900 - Part 1.mkv", Size: 11, MTime: 2, Fingerprint: "p1b", Kind: "video"}}, nil)
	if v := versionByDir(t, s, "../cine/e"); v.DurationMs != 3_000_000 {
		t.Fatalf("stale part still counted: %+v", v)
	}
}

func TestVersionsNormalizesNameCodec(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{{Path: "../cine/x.avi", Size: 1, MTime: 1, Fingerprint: "x", Kind: "video"}}, roots)
	s.ReplaceVersions([]grouping.Version{{Dir: "../cine", Parsed: nameparse.Parsed{Title: "X", Codec: "XviD"}, Size: 1, Parts: 1,
		Members: []grouping.Member{{Path: "../cine/x.avi", Role: grouping.RoleMain}}}})
	if v := versionByDir(t, s, "../cine"); v.Codec != "mpeg4" {
		t.Fatalf("codec = %q", v.Codec)
	}
}

func TestVersionsMarksBest(t *testing.T) {
	s := open(t)
	type vdef struct {
		dir, file, title, res, codec string
		size                         int64
	}
	defs := []vdef{
		{"../cine/1", "a.mkv", "Amarcord", "720p", "H.264", 4},
		{"../cine/2", "a.avi", "Amarcord", "1080p", "XviD", 9},
		{"../cine/3", "a.mkv", "Amarcord", "1080p", "H.265", 3},
		{"../cine/4", "a.mkv", "amarcord", "2160p", "H.265", 50}, // file missing: not a candidate
		{"../cine/5", "s.mkv", "Stalker", "1080p", "H.264", 5},   // only version: never "best"
	}
	var rows []FileRow
	var gv []grouping.Version
	for i, d := range defs {
		p := d.dir + "/" + d.file
		rows = append(rows, FileRow{Path: p, Size: d.size, MTime: 1, Fingerprint: string(rune('a' + i)), Kind: "video"})
		gv = append(gv, grouping.Version{Dir: d.dir, Size: d.size, Parts: 1,
			Parsed:  nameparse.Parsed{Title: d.title, Year: 1973, Resolution: d.res, Codec: d.codec},
			Members: []grouping.Member{{Path: p, Role: grouping.RoleMain}}})
	}
	s.SyncFiles(rows, roots)
	s.SyncFiles(append(rows[:3:3], rows[4]), roots) // ../cine/4 goes missing
	if err := s.ReplaceVersions(gv); err != nil {
		t.Fatal(err)
	}
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	var best []string
	for _, v := range vs {
		if v.Best {
			best = append(best, v.Dir)
		}
	}
	if !reflect.DeepEqual(best, []string{"../cine/3"}) {
		t.Fatalf("best = %v", best)
	}
}
