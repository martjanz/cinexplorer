package mediafile

import "testing"

func TestClassify(t *testing.T) {
	cases := map[string]Kind{
		"movie.MKV":                       Video,
		"Brasileirinho.avi":               Video,
		"VTS_01_1.VOB":                    DVD,
		"VIDEO_TS.IFO":                    DVD,
		"movie.spa.srt":                   Subtitle,
		"movie.idx":                       Subtitle,
		"seventhseal-KARiNA.nfo":          Info,
		"Thumbs.db":                       Junk,
		".DS_Store":                       Junk,
		"sync.ffs_db":                     Junk,
		"._movie.mkv":                     Junk,
		"Uploaded @ thepiratebay.org.txt": Junk,
		"cover.jpg":                       Other,
	}
	for name, want := range cases {
		if got := Classify(name); got != want {
			t.Errorf("Classify(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestStoredAndFingerprinted(t *testing.T) {
	for _, k := range []Kind{Video, DVD, Subtitle, Info} {
		if !k.Stored() {
			t.Errorf("%s should be stored", k)
		}
	}
	for _, k := range []Kind{Junk, Other} {
		if k.Stored() {
			t.Errorf("%s should not be stored", k)
		}
	}
	if !Video.Fingerprinted() || !DVD.Fingerprinted() || Subtitle.Fingerprinted() {
		t.Error("only video and DVD files are fingerprinted")
	}
}
