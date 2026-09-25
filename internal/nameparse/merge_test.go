package nameparse

import "testing"

func TestMergePrefersMoreInformativeAndFillsGaps(t *testing.T) {
	file := Parse("amarcord.720p.x264")
	dir := Parse("Amarcord [Federico Fellini, 1973]")
	got := Merge(file, dir)
	if got.Title != "Amarcord" || got.Year != 1973 || got.Director != "Federico Fellini" ||
		got.Resolution != "720p" || got.Codec != "H.264" {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeTieKeepsFirst(t *testing.T) {
	got := Merge(Parse("Stalker (1979)"), Parse("Solaris (1972)"))
	if got.Title != "Stalker" || got.Year != 1979 {
		t.Fatalf("got %+v", got)
	}
}
