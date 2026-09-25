package quality

import "testing"

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b Candidate
		want int // sign
	}{
		{"resolution wins over codec and size", Candidate{"1080p", "h264", 1}, Candidate{"720p", "hevc", 9}, 1},
		{"1080i counts as 1080p", Candidate{"1080i", "h264", 5}, Candidate{"1080p", "h264", 5}, 0},
		{"codec breaks resolution ties", Candidate{"1080p", "hevc", 1}, Candidate{"1080p", "h264", 9}, 1},
		{"xvid beats mpeg2", Candidate{"576p", "mpeg4", 1}, Candidate{"576p", "mpeg2", 9}, 1},
		{"unranked codec beats unknown", Candidate{"SD", "cinepak", 1}, Candidate{"SD", "", 9}, 1},
		{"size breaks full ties", Candidate{"720p", "h264", 1}, Candidate{"720p", "h264", 2}, -1},
		{"known resolution beats unknown", Candidate{"SD", "", 1}, Candidate{"", "av1", 9}, 1},
	} {
		got := Compare(c.a, c.b)
		if sign(got) != c.want {
			t.Errorf("%s: Compare = %d, want sign %d", c.name, got, c.want)
		}
		if sign(Compare(c.b, c.a)) != -c.want {
			t.Errorf("%s: Compare is not antisymmetric", c.name)
		}
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

func TestGroupKey(t *testing.T) {
	same := [][2]any{
		{"El Ángel Exterminador", "ángel exterminador"},
		{"The Good, the Bad and the Ugly", "good the bad and the ugly"},
		{"Amarcord", "AMARCORD"},
		{"8½", "8 ½"},
	}
	for _, p := range same {
		if a, b := GroupKey(p[0].(string), 1973), GroupKey(p[1].(string), 1973); a != b {
			t.Errorf("GroupKey(%q) = %q, GroupKey(%q) = %q; want equal", p[0], a, p[1], b)
		}
	}
	if GroupKey("Amarcord", 1973) == GroupKey("Amarcord", 1974) {
		t.Error("different years must not group")
	}
	if GroupKey("The", 2000) != "the|2000" {
		t.Errorf("a lone article is the title: %q", GroupKey("The", 2000))
	}
	if GroupKey("", 1973) != "" || GroupKey("…", 1973) != "" {
		t.Error("empty titles must not group")
	}
	if got := GroupKey("La Strada", 1954); got != "strada|1954" {
		t.Errorf("GroupKey = %q", got)
	}
}
