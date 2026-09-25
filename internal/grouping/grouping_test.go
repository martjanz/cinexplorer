package grouping

import (
	"reflect"
	"testing"

	"cinexplorer/internal/mediafile"
)

var roots = []string{"../cine", "../cine-ordenar"}

func only(t *testing.T, vs []Version) Version {
	t.Helper()
	if len(vs) != 1 {
		t.Fatalf("want 1 version, got %d: %+v", len(vs), vs)
	}
	return vs[0]
}

func TestPartsAndExtras(t *testing.T) {
	dir := "../cine/1970s/1900 (Novecento) (1976) [mkvonly]"
	v := only(t, Build([]Entry{
		{dir + "/1900 (Novecento) (1976) - Part 2.mkv", 2_177_896_754, mediafile.Video},
		{dir + "/1900 (Novecento) (1976) - Part 1.mkv", 2_295_706_615, mediafile.Video},
		{dir + "/Bonus 1900 The Story, The Cast, Creating An Epic Eng + Ita + Rus subs.mkv", 296_513_285, mediafile.Video},
	}, roots))

	if v.Dir != dir || v.Parts != 2 || v.Size != 4_473_603_369 {
		t.Fatalf("got dir=%q parts=%d size=%d", v.Dir, v.Parts, v.Size)
	}
	if v.Parsed.Title != "1900 (Novecento)" || v.Parsed.Year != 1976 {
		t.Fatalf("parsed %+v", v.Parsed)
	}
	want := []Member{
		{Path: dir + "/1900 (Novecento) (1976) - Part 1.mkv", Role: RoleMain, Part: 1},
		{Path: dir + "/1900 (Novecento) (1976) - Part 2.mkv", Role: RoleMain, Part: 2},
		{Path: dir + "/Bonus 1900 The Story, The Cast, Creating An Epic Eng + Ita + Rus subs.mkv", Role: RoleExtra},
	}
	if !reflect.DeepEqual(v.Members, want) {
		t.Fatalf("members %+v", v.Members)
	}
}

func TestContainerFolderKeepsFileNames(t *testing.T) {
	vs := Build([]Entry{
		{"../cine/1970s/Arabian.Nights.1974.CC.1080p.BluRay.FLAC1.0.x264-ADE.mkv", 9_000_000_000, mediafile.Video},
		{"../cine/1970s/Deep.Throat.1972.1080p.BluRay.x264.DTS-FGT.mkv", 7_000_000_000, mediafile.Video},
	}, roots)
	if len(vs) != 2 || vs[0].Parsed.Title != "Arabian Nights" || vs[1].Parsed.Title != "Deep Throat" {
		t.Fatalf("got %+v", vs)
	}
}

func TestLooseFileInRoot(t *testing.T) {
	v := only(t, Build([]Entry{{"../cine-ordenar/Attenberg.avi", 700_000_000, mediafile.Video}}, roots))
	if v.Parsed.Title != "Attenberg" || v.Dir != "../cine-ordenar" {
		t.Fatalf("got %+v", v)
	}
}

func TestVideoTS(t *testing.T) {
	v := only(t, Build([]Entry{
		{"../cine-ordenar/Aurora/VIDEO_TS/VTS_01_1.VOB", 1_000_000_000, mediafile.DVD},
		{"../cine-ordenar/Aurora/VIDEO_TS/VIDEO_TS.IFO", 20_000, mediafile.DVD},
	}, roots))
	if v.Dir != "../cine-ordenar/Aurora" || v.Parsed.Title != "Aurora" || v.Parts != 1 || v.Size != 1_000_020_000 {
		t.Fatalf("got %+v", v)
	}
}

func TestSubtitlesInFolderAndSubsDir(t *testing.T) {
	dir := "../cine/2000s/In the Mood for Love (2000)"
	v := only(t, Build([]Entry{
		{dir + "/In.the.Mood.for.Love.2000.720p.BluRay.x264.mkv", 5_000_000_000, mediafile.Video},
		{dir + "/In.the.Mood.for.Love.2000.720p.BluRay.x264.spa.srt", 90_000, mediafile.Subtitle},
		{dir + "/Subs/English.srt", 90_000, mediafile.Subtitle},
	}, roots))
	if v.Parsed.Title != "In the Mood for Love" || v.Parsed.Year != 2000 {
		t.Fatalf("parsed %+v", v.Parsed)
	}
	if got := v.SubLangs(); !reflect.DeepEqual(got, []string{"en", "es"}) {
		t.Fatalf("sub langs %v", got)
	}
}

func TestSameNameDifferentContainerAreTwoVersions(t *testing.T) {
	vs := Build([]Entry{
		{"../cine-ordenar/Carandiru/Carandiru.avi", 700_000_000, mediafile.Video},
		{"../cine-ordenar/Carandiru/Carandiru.mkv", 1_400_000_000, mediafile.Video},
	}, roots)
	if len(vs) != 2 || vs[0].Parts != 1 || vs[1].Parts != 1 {
		t.Fatalf("got %+v", vs)
	}
}

func TestSubtitlesMatchByPrefix(t *testing.T) {
	vs := Build([]Entry{
		{"../cine-ordenar/A.avi", 100, mediafile.Video},
		{"../cine-ordenar/A.srt", 1, mediafile.Subtitle},
		{"../cine-ordenar/B.avi", 100, mediafile.Video},
		{"../cine-ordenar/B.en.srt", 1, mediafile.Subtitle},
	}, roots)
	if len(vs) != 2 {
		t.Fatalf("got %d versions", len(vs))
	}
	if got := vs[0].SubLangs(); !reflect.DeepEqual(got, []string{"?"}) {
		t.Fatalf("A subs %v", got)
	}
	if got := vs[1].SubLangs(); !reflect.DeepEqual(got, []string{"en"}) {
		t.Fatalf("B subs %v", got)
	}
}

func TestOrphanedExtraAttachesToLargestVersion(t *testing.T) {
	vs := Build([]Entry{
		{"../cine-ordenar/Carandiru/Carandiru.avi", 700_000_000, mediafile.Video},
		{"../cine-ordenar/Carandiru/Carandiru.mkv", 1_400_000_000, mediafile.Video},
		{"../cine-ordenar/Carandiru/Carandiru-trailer.mp4", 50_000_000, mediafile.Video},
	}, roots)
	if len(vs) != 2 {
		t.Fatalf("got %d versions: %+v", len(vs), vs)
	}
	var total int
	for _, v := range vs {
		for _, m := range v.Members {
			if m.Role == RoleExtra {
				total++
				if v.Size != 1_400_000_000 {
					t.Fatalf("extra attached to version with size %d, want largest (1_400_000_000)", v.Size)
				}
			}
		}
	}
	if total != 1 {
		t.Fatalf("want exactly 1 extra member across all versions, got %d", total)
	}
}

func TestOrphanedSubtitleAttachesToLargestVersion(t *testing.T) {
	dir := "../cine-ordenar/Aurora"
	vs := Build([]Entry{
		{dir + "/VIDEO_TS/VTS_01_1.VOB", 1_000_000_000, mediafile.DVD},
		{dir + "/VIDEO_TS/VIDEO_TS.IFO", 20_000, mediafile.DVD},
		{dir + "/Loose.mkv", 500_000, mediafile.Video},
		{dir + "/NoPrefixMatch.srt", 1, mediafile.Subtitle},
	}, roots)
	if len(vs) != 2 {
		t.Fatalf("got %d versions: %+v", len(vs), vs)
	}
	var total int
	for _, v := range vs {
		for _, m := range v.Members {
			if m.Role == RoleSubtitle {
				total++
				if v.Size != 1_000_020_000 {
					t.Fatalf("subtitle attached to version with size %d, want largest (1_000_020_000, the DVD)", v.Size)
				}
			}
		}
	}
	if total != 1 {
		t.Fatalf("want exactly 1 subtitle member across all versions, got %d", total)
	}
}

func TestSplitPart(t *testing.T) {
	cases := map[string]struct {
		base string
		part int
	}{
		"Movie CD1":       {"Movie", 1},
		"Movie.cd2":       {"Movie", 2},
		"Movie (Part 2)":  {"Movie", 2},
		"Movie - Disc 1":  {"Movie", 1},
		"The Apartment 2": {"The Apartment 2", 0},
		"Egypt 2":         {"Egypt 2", 0},
	}
	for in, want := range cases {
		base, part := splitPart(in)
		if base != want.base || part != want.part {
			t.Errorf("splitPart(%q) = %q, %d", in, base, part)
		}
	}
}
