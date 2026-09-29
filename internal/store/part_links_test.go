package store

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/mediafile"
	"cinexplorer/internal/nameparse"
)

const (
	pA  = "../cine/Shoah/Shoah - Part 1.mkv"
	pB  = "../cine/Shoah/Shoah - Part 2 - Treblinka.mkv"
	pBS = "../cine/Shoah/Shoah - Part 2 - Treblinka.es.srt"
	pC  = "../cine/Shoah/Shoah - Part 3 - Sobibor.mkv"
)

func partVersions(only ...string) []grouping.Version {
	main := func(p string) grouping.Member { return grouping.Member{Path: p, Role: grouping.RoleMain} }
	all := map[string]grouping.Version{
		"A": {Dir: "../cine/Shoah", Parsed: nameparse.Parsed{Title: "Shoah"}, Size: 100, Parts: 1, Members: []grouping.Member{main(pA)}},
		"B": {Dir: "../cine/Shoah", Parsed: nameparse.Parsed{Title: "Shoah 2"}, Size: 90, Parts: 1,
			Members: []grouping.Member{main(pB), {Path: pBS, Role: grouping.RoleSubtitle, Lang: "es"}}},
		"C": {Dir: "../cine/Shoah", Parsed: nameparse.Parsed{Title: "Shoah 3"}, Size: 80, Parts: 1, Members: []grouping.Member{main(pC)}},
	}
	var out []grouping.Version
	for _, k := range only {
		out = append(out, all[k])
	}
	return out
}

func partCatalog(t *testing.T) *Store {
	t.Helper()
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.SyncFiles([]FileRow{
		{Path: pA, Size: 100, MTime: 1, Fingerprint: "s1", Kind: "video"},
		{Path: pB, Size: 90, MTime: 1, Fingerprint: "s2", Kind: "video"},
		{Path: pBS, Size: 1, MTime: 1, Kind: string(mediafile.Subtitle)},
		{Path: pC, Size: 80, MTime: 1, Fingerprint: "s3", Kind: "video"},
	}, []string{"../cine"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceVersions(partVersions("A", "B", "C")); err != nil {
		t.Fatal(err)
	}
	return s
}

func mustVersions(t *testing.T, s *Store) []VersionView {
	t.Helper()
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	return vs
}

func TestSetPartLinkMergesVersions(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s2", "s1"); err != nil {
		t.Fatal(err)
	}
	vs := mustVersions(t, s)
	if len(vs) != 2 {
		t.Fatalf("want 2 versions, got %+v", vs)
	}
	var shoah VersionView
	for _, v := range vs {
		if v.Title == "Shoah" {
			shoah = v
		}
	}
	if shoah.Parts != 2 || shoah.Size != 190 || shoah.SubLangs != "es" || !shoah.PartLinked || shoah.Fingerprint != "s1" {
		t.Fatalf("merged version %+v", shoah)
	}
	var got []string
	for _, f := range shoah.Files {
		got = append(got, f.Role+":"+f.Path)
		if f.Role == "main" && f.Part == 0 {
			t.Fatalf("main file without part: %+v", f)
		}
	}
	want := []string{"main:" + pA, "main:" + pB, "subtitle:" + pBS}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files %v, want %v", got, want)
	}
}

func TestSetPartLinkValidates(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s1", "s1"); !errors.Is(err, ErrPartLink) {
		t.Errorf("self: %v", err)
	}
	if err := s.SetPartLink("nope", "s1"); !errors.Is(err, ErrUnknownFingerprint) {
		t.Errorf("unknown follower: %v", err)
	}
	if err := s.SetPartLink("s2", "nope"); !errors.Is(err, ErrUnknownFingerprint) {
		t.Errorf("unknown leader: %v", err)
	}
}

func TestPartLinkChainMergesAllInOrder(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s3", "s2"); err != nil { // C joins B
		t.Fatal(err)
	}
	if err := s.SetPartLink("s2", "s1"); err != nil { // B (with C) joins A
		t.Fatal(err)
	}
	vs := mustVersions(t, s)
	if len(vs) != 1 || vs[0].Parts != 3 || vs[0].Size != 270 || vs[0].Fingerprint != "s1" {
		t.Fatalf("got %+v", vs)
	}
	var paths []string
	for i, f := range vs[0].Files {
		if f.Role == "main" {
			paths = append(paths, f.Path)
			if f.Part != len(paths) {
				t.Fatalf("file %d has part %d", i, f.Part)
			}
		}
	}
	if !reflect.DeepEqual(paths, []string{pA, pB, pC}) {
		t.Fatalf("order %v", paths)
	}
}

func TestPartLinksSurviveRebuildAndWaitForMissingLeader(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s2", "s1"); err != nil {
		t.Fatal(err)
	}
	// A rescan rebuilds the versions apart...
	if err := s.ReplaceVersions(partVersions("B", "C")); err != nil { // ...and A is not there
		t.Fatal(err)
	}
	if n, err := s.ApplyPartLinks(); n != 0 || err != nil {
		t.Fatalf("leader absent: merged %d, err %v", n, err)
	}
	if len(mustVersions(t, s)) != 2 {
		t.Fatal("versions changed without the leader")
	}
	// ...and the link applies again when the leader is back.
	if err := s.ReplaceVersions(partVersions("A", "B", "C")); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ApplyPartLinks(); n != 1 || err != nil {
		t.Fatalf("merged %d, err %v", n, err)
	}
	if n, err := s.ApplyPartLinks(); n != 0 || err != nil { // idempotent
		t.Fatalf("second apply: merged %d, err %v", n, err)
	}
	if len(mustVersions(t, s)) != 2 {
		t.Fatal("want 2 versions after re-apply")
	}
}

func TestPartLinkCycleIsIgnored(t *testing.T) {
	s := partCatalog(t)
	for _, q := range [][2]string{{"s1", "s2"}, {"s2", "s1"}} {
		if _, err := s.db.Exec(`INSERT INTO part_links (fingerprint, leader, created_at) VALUES (?, ?, 0)`, q[0], q[1]); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := s.ApplyPartLinks(); n != 0 || err != nil {
		t.Fatalf("cycle: merged %d, err %v", n, err)
	}
	if len(mustVersions(t, s)) != 3 {
		t.Fatal("a cycle merged versions")
	}
}

func TestUnlink(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s3", "s2"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPartLink("s2", "s1"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Unlink("s1"); n != 2 || err != nil { // both, the chained one included
		t.Fatalf("unlinked %d, err %v", n, err)
	}
	if n, err := s.Unlink("s1"); n != 0 || err != nil {
		t.Fatalf("second unlink: %d, %v", n, err)
	}
	// The next scan rebuilds three separate versions and nothing merges them.
	if err := s.ReplaceVersions(partVersions("A", "B", "C")); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ApplyPartLinks(); n != 0 || err != nil {
		t.Fatalf("apply after unlink: %d, %v", n, err)
	}
	vs := mustVersions(t, s)
	if len(vs) != 3 {
		t.Fatalf("want 3 versions, got %d", len(vs))
	}
	for _, v := range vs {
		if v.PartLinked {
			t.Fatalf("%q still marked linked", v.Title)
		}
	}
}

func TestApplyPartLinksWithIdenticalCopiesNeverDuplicatesAPart(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	const (
		la = "../cine/L/L.mkv"
		fa = "../cine/F/F.mkv"
		lb = "../cine2/L/L.mkv"
		fb = "../cine2/F/F.mkv"
	)
	if err := s.SyncFiles([]FileRow{
		{Path: la, Size: 100, MTime: 1, Fingerprint: "l", Kind: "video"},
		{Path: fa, Size: 90, MTime: 1, Fingerprint: "f", Kind: "video"},
		{Path: lb, Size: 100, MTime: 1, Fingerprint: "l", Kind: "video"},
		{Path: fb, Size: 90, MTime: 1, Fingerprint: "f", Kind: "video"},
	}, []string{"../cine", "../cine2"}); err != nil {
		t.Fatal(err)
	}
	mk := func(dir, title, p string, size int64) grouping.Version {
		return grouping.Version{Dir: dir, Parsed: nameparse.Parsed{Title: title}, Size: size, Parts: 1,
			Members: []grouping.Member{{Path: p, Role: grouping.RoleMain}}}
	}
	if err := s.ReplaceVersions([]grouping.Version{
		mk("../cine/L", "L", la, 100), mk("../cine/F", "F", fa, 90),
		mk("../cine2/L", "L", lb, 100), mk("../cine2/F", "F", fb, 90),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPartLink("f", "l"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyPartLinks(); err != nil { // an unrelated, later apply
		t.Fatal(err)
	}
	rows, err := s.db.Query(`SELECT version_id, part, fingerprint FROM files WHERE version_id IS NOT NULL AND role = 'main'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	parts, fps := map[[2]int64]bool{}, map[string]bool{}
	for rows.Next() {
		var vid, part int64
		var fp string
		if err := rows.Scan(&vid, &part, &fp); err != nil {
			t.Fatal(err)
		}
		if k := [2]int64{vid, part}; parts[k] {
			t.Fatalf("version %d has part %d twice", vid, part)
		} else {
			parts[k] = true
		}
		if k := fmt.Sprintf("%d|%s", vid, fp); fps[k] {
			t.Fatalf("version %d has two main files with fingerprint %s", vid, fp)
		} else {
			fps[k] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestSetPartLinkRejectsCycleThroughFollower(t *testing.T) {
	s := partCatalog(t)
	for _, q := range [][2]string{{"s1", "s2"}, {"s2", "s3"}} {
		if _, err := s.db.Exec(`INSERT INTO part_links (fingerprint, leader, created_at) VALUES (?, ?, 0)`, q[0], q[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetPartLink("s2", "s1"); !errors.Is(err, ErrPartLink) {
		t.Fatalf("want ErrPartLink, got %v", err)
	}
	var leader string
	if err := s.db.QueryRow(`SELECT leader FROM part_links WHERE fingerprint = 's2'`).Scan(&leader); err != nil || leader != "s3" {
		t.Fatalf("s2 link changed: %q, %v", leader, err)
	}
}
