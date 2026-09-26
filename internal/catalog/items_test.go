package catalog

import (
	"reflect"
	"testing"
)

func byID(items []Item) map[string]Item {
	out := map[string]Item{}
	for _, it := range items {
		out[it.id()] = it
	}
	return out
}

func TestItems(t *testing.T) {
	items := Items(snapshot(), roots)
	got := byID(items)
	want := []string{"movie:7857", "movie:7858", "n1", "s1", "id:9", "p1"}
	if len(items) != len(want) {
		t.Fatalf("items %v", got)
	}
	for _, id := range want {
		if _, ok := got[id]; !ok {
			t.Errorf("missing %s", id)
		}
	}

	a := got["movie:7857"]
	if a.Kind != KindMovie || a.Title != "Amarcord" || a.Year != 1973 || a.Versions != 3 || a.Size != 12600 ||
		a.Added != 100 || a.Resolution != "1080p" || a.Poster != "pa" ||
		!reflect.DeepEqual(a.Directors, []string{"Federico Fellini"}) || !reflect.DeepEqual(a.Countries, []string{"IT", "FR"}) {
		t.Errorf("amarcord %+v", a)
	}
	if !a.multiVersion || !a.identical || a.unidentified {
		t.Errorf("amarcord states: versions %v identical %v unidentified %v", a.multiVersion, a.identical, a.unidentified)
	}
	if !reflect.DeepEqual(a.resolutions, []string{"1080p", "SD"}) || !reflect.DeepEqual(a.subs, []string{"en", "es"}) {
		t.Errorf("amarcord resolutions %v subs %v", a.resolutions, a.subs)
	}
	if want := []string{"cine", "cine/1970s", "cine-ordenar", "cine/Collections"}; !reflect.DeepEqual(a.locations, want) {
		t.Errorf("amarcord locations %v", a.locations)
	}

	n := got["n1"]
	if n.Kind != KindVersion || n.Key != "n1" || n.Title != "Novecento" || n.Year != 1976 || n.Resolution != "720p" ||
		!n.unidentified || n.multiVersion || n.identical || !reflect.DeepEqual(n.Directors, []string{"Bertolucci"}) ||
		n.Poster != "" || len(n.Countries) != 0 {
		t.Errorf("novecento %+v", n)
	}
	// Identified as a movie that is not stored yet: shown as it is on disk.
	if p := got["p1"]; p.Kind != KindVersion || p.Title != "Solaris" || !p.unidentified {
		t.Errorf("solaris %+v", p)
	}
	if u := got["id:9"]; u.Key != "id:9" || u.Title != "Vacío" {
		t.Errorf("unhashed %+v", u)
	}
	if s := got["s1"]; s.Resolution != "SD" || !reflect.DeepEqual(s.locations, []string{"cine-ordenar", "cine-ordenar/Tarkovsky"}) {
		t.Errorf("stalker %+v %v", s, s.locations)
	}
}

func TestResolution(t *testing.T) {
	for in, want := range map[string]string{"2160p": "4K", "1080p": "1080p", "1080i": "1080p", "720p": "720p",
		"576p": "SD", "480p": "SD", "SD": "SD", "": ""} {
		if got := Resolution(in); got != want {
			t.Errorf("Resolution(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocations(t *testing.T) {
	cases := []struct {
		dir  string
		want []string
	}{
		{"../cine/1970s/Amarcord", []string{"cine", "cine/1970s"}},
		{"../cine", []string{"cine"}},
		{"../cine-ordenar/x", []string{"cine-ordenar", "cine-ordenar/x"}},
		{"../otro/x", nil},
	}
	for _, c := range cases {
		if got := locations(c.dir, roots); !reflect.DeepEqual(got, c.want) {
			t.Errorf("locations(%q) = %v, want %v", c.dir, got, c.want)
		}
	}
	if got := locations("../../media/cine/x", []string{"../../media/cine/"}); !reflect.DeepEqual(got, []string{"media/cine", "media/cine/x"}) {
		t.Errorf("deeper root: %v", got)
	}
}
