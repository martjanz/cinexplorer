package store

import (
	"path/filepath"
	"reflect"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/probe"
)

// mediaLibrary catalogs a movie with an extra, a DVD and a missing file.
func mediaLibrary(t *testing.T, s *Store) {
	t.Helper()
	files := []FileRow{
		{Path: "../cine/a/Amarcord.mkv", Size: 100, MTime: 1, Fingerprint: "a", Kind: "video"},
		{Path: "../cine/a/Trailer.mkv", Size: 5, MTime: 1, Fingerprint: "t", Kind: "video"},
		{Path: "../cine/a/Amarcord.spa.srt", Size: 1, MTime: 1, Kind: "subtitle"},
		{Path: "../cine/b/Stalker.avi", Size: 90, MTime: 1, Fingerprint: "s", Kind: "video"},
		{Path: "../cine/c/VIDEO_TS/VIDEO_TS.IFO", Size: 10, MTime: 1, Fingerprint: "v0", Kind: "dvd"},
		{Path: "../cine/c/VIDEO_TS/VTS_01_0.IFO", Size: 10, MTime: 1, Fingerprint: "v1", Kind: "dvd"},
		{Path: "../cine/c/VIDEO_TS/VTS_01_1.VOB", Size: 900, MTime: 1, Fingerprint: "v2", Kind: "dvd"},
		{Path: "../cine/d/Gone.mkv", Size: 70, MTime: 1, Fingerprint: "g", Kind: "video"},
	}
	if err := s.SyncFiles(files, roots); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncFiles(files[:7], roots); err != nil { // Gone.mkv disappears
		t.Fatal(err)
	}
	main := func(p string) grouping.Member { return grouping.Member{Path: p, Role: grouping.RoleMain} }
	err := s.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/a", Parsed: nameparse.Parsed{Title: "Amarcord"}, Size: 100, Parts: 1, Members: []grouping.Member{
			main("../cine/a/Amarcord.mkv"),
			{Path: "../cine/a/Amarcord.spa.srt", Role: grouping.RoleSubtitle, Lang: "es"},
			{Path: "../cine/a/Trailer.mkv", Role: grouping.RoleExtra},
		}},
		{Dir: "../cine/b", Parsed: nameparse.Parsed{Title: "Stalker"}, Size: 90, Parts: 1,
			Members: []grouping.Member{main("../cine/b/Stalker.avi")}},
		{Dir: "../cine/c", Parsed: nameparse.Parsed{Title: "c"}, Size: 920, Parts: 1, Members: []grouping.Member{
			main("../cine/c/VIDEO_TS/VIDEO_TS.IFO"), main("../cine/c/VIDEO_TS/VTS_01_0.IFO"), main("../cine/c/VIDEO_TS/VTS_01_1.VOB"),
		}},
		{Dir: "../cine/d", Parsed: nameparse.Parsed{Title: "Gone"}, Size: 70, Parts: 1,
			Members: []grouping.Member{main("../cine/d/Gone.mkv")}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func pendingPaths(t *testing.T, s *Store) []string {
	t.Helper()
	ts, err := s.PendingProbes(ProbeEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, x := range ts {
		out = append(out, x.Path)
	}
	return out
}

func TestPendingProbes(t *testing.T) {
	s := open(t)
	mediaLibrary(t, s)
	want := []string{"../cine/a/Amarcord.mkv", "../cine/b/Stalker.avi", "../cine/c/VIDEO_TS/VTS_01_0.IFO"}
	if got := pendingPaths(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("pending = %v, want %v", got, want)
	}

	ts, err := s.PendingProbes(ProbeEnv{})
	if err != nil {
		t.Fatal(err)
	}
	err = s.SaveProbes([]ProbeResult{
		{FileID: ts[0].FileID, Size: ts[0].Size, MTime: ts[0].MTime, Info: probe.Info{Height: 1080}},
		{FileID: ts[1].FileID, Size: ts[1].Size, MTime: ts[1].MTime, Err: "probe: invalid header: truncated file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := pendingPaths(t, s); !reflect.DeepEqual(got, want[2:]) {
		t.Fatalf("after saving, pending = %v", got)
	}

	// A changed file is probed again, even if the previous attempt failed.
	if err := s.SyncFiles([]FileRow{{Path: "../cine/b/Stalker.avi", Size: 91, MTime: 2, Fingerprint: "s2", Kind: "video"}}, nil); err != nil {
		t.Fatal(err)
	}
	if got := pendingPaths(t, s); !reflect.DeepEqual(got, []string{"../cine/b/Stalker.avi", "../cine/c/VIDEO_TS/VTS_01_0.IFO"}) {
		t.Fatalf("after change, pending = %v", got)
	}
}

func TestSaveProbesUpserts(t *testing.T) {
	s := open(t)
	mediaLibrary(t, s)
	ts, err := s.PendingProbes(ProbeEnv{})
	if err != nil {
		t.Fatal(err)
	}
	r := ProbeResult{FileID: ts[0].FileID, Size: 100, MTime: 1, Info: probe.Info{Height: 720}}
	if err := s.SaveProbes([]ProbeResult{r}); err != nil {
		t.Fatal(err)
	}
	r.Info.Height = 1080
	if err := s.SaveProbes([]ProbeResult{r}); err != nil {
		t.Fatal(err)
	}
	var n, h int
	if err := s.db.QueryRow(`SELECT COUNT(*), MAX(height) FROM media`).Scan(&n, &h); err != nil {
		t.Fatal(err)
	}
	if n != 1 || h != 1080 {
		t.Fatalf("rows=%d height=%d", n, h)
	}
}

func TestReadOnlyStage1Catalog(t *testing.T) {
	// A catalog written by stage 1 has no media table, and a read-only open
	// does not run the schema: listing versions must still work.
	path := filepath.Join(t.TempDir(), "cinexplorer.db")
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	mediaLibrary(t, s)
	if _, err := s.db.Exec(`DROP TABLE media`); err != nil {
		t.Fatal(err)
	}
	s.Close()

	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if ts, err := ro.PendingProbes(ProbeEnv{}); err != nil || len(ts) != 0 {
		t.Fatalf("pending %v, %v", ts, err)
	}
	if vs, err := ro.Versions(); err != nil || len(vs) != 4 {
		t.Fatalf("versions %d, %v", len(vs), err)
	}
}

func TestPendingProbesAfterReaderOrFFprobeChange(t *testing.T) {
	s := open(t)
	mediaLibrary(t, s)
	old := ProbeEnv{Version: 1}
	ts, err := s.PendingProbes(old)
	if err != nil {
		t.Fatal(err)
	}
	err = s.SaveProbes([]ProbeResult{
		{FileID: ts[0].FileID, Size: ts[0].Size, MTime: ts[0].MTime, Info: probe.Info{Height: 1080}, Env: old},
		{FileID: ts[1].FileID, Size: ts[1].Size, MTime: ts[1].MTime, Err: "probe: unsupported format", Env: old},
		{FileID: ts[2].FileID, Size: ts[2].Size, MTime: ts[2].MTime, Info: probe.Info{Height: 576}, Env: old},
	})
	if err != nil {
		t.Fatal(err)
	}
	pending := func(env ProbeEnv) []string {
		ts, err := s.PendingProbes(env)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, x := range ts {
			out = append(out, x.Path)
		}
		return out
	}
	if got := pending(old); len(got) != 0 {
		t.Fatalf("same env: pending %v", got)
	}
	// ffprobe installed since: only the failure is worth another try.
	if got := pending(ProbeEnv{Version: 1, FFprobe: true}); !reflect.DeepEqual(got, []string{"../cine/b/Stalker.avi"}) {
		t.Fatalf("with ffprobe: pending %v", got)
	}
	// Better readers: everything they produced before is read again.
	if got := pending(ProbeEnv{Version: 2}); len(got) != 3 {
		t.Fatalf("new version: pending %v", got)
	}
}
