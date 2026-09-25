package nameparse

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseCorpus(t *testing.T) {
	cases := []struct {
		in   string
		want Parsed
	}{
		{"Amarcord [Federico Fellini, 1973]", Parsed{Title: "Amarcord", Year: 1973, Director: "Federico Fellini"}},
		{"Chinatown (Polanski, USA, 1974)", Parsed{Title: "Chinatown", Year: 1974, Director: "Polanski", Countries: []string{"USA"}}},
		{"Annie Hall [1977, USA]", Parsed{Title: "Annie Hall", Year: 1977, Countries: []string{"USA"}}},
		{"1900 (Novecento) (1976) [mkvonly]", Parsed{Title: "1900 (Novecento)", Year: 1976}},
		{"Arabian.Nights.1974.CC.1080p.BluRay.FLAC1.0.x264-ADE", Parsed{Title: "Arabian Nights", Year: 1974, Resolution: "1080p", Source: "BluRay", Codec: "H.264", Group: "ADE"}},
		{"Auf.der.anderen.Seite.German.AC3.DVDRiP.XviD-EMPiRE", Parsed{Title: "Auf der anderen Seite", Source: "DVDRip", Codec: "XviD", Language: "de", Group: "EMPiRE"}},
		{"El.Castillo.Ambulante.Spanish.XviD.AC3.DVDRip.By.FreAk.TEAm", Parsed{Title: "El Castillo Ambulante", Source: "DVDRip", Codec: "XviD", Language: "es"}},
		{"El viaje de Chihiro DVDRIP SPANISH DIVX (www.lamejorfrikiweb.cjb.net)", Parsed{Title: "El viaje de Chihiro", Source: "DVDRip", Codec: "DivX", Language: "es"}},
		{"El.extraño.caso.del.hombre.y.la.bestia.(1951).1080p.emule.via.clan-sudamerica.net", Parsed{Title: "El extraño caso del hombre y la bestia", Year: 1951, Resolution: "1080p"}},
		{"El señor Galíndez (Rodolfo Kuhn, 1983) - YouTube.(Found.via.clan-sudamerica.net)", Parsed{Title: "El señor Galíndez", Year: 1983, Director: "Rodolfo Kuhn"}},
		{"Antes de la Lluvia-Before the Rain (Milko Manchevski -1994- Uk-Fr-Macedonia)", Parsed{Title: "Antes de la Lluvia-Before the Rain", Year: 1994, Director: "Milko Manchevski", Countries: []string{"Uk-Fr-Macedonia"}}},
		{"Benning, James - The United States of America (2022)", Parsed{Title: "The United States of America", Year: 2022, Director: "James Benning"}},
		{"1976 - 9 Lives of a Wet Pussy", Parsed{Title: "9 Lives of a Wet Pussy", Year: 1976}},
		{"BuSan_Tsai Ming-lian", Parsed{Title: "BuSan", Director: "Tsai Ming-lian"}},
		{"Double Play James Benning And Richard Linklater (2013) [BluRay] [1080p] [YTS.AM]", Parsed{Title: "Double Play James Benning And Richard Linklater", Year: 2013, Resolution: "1080p", Source: "BluRay"}},
		{"1990", Parsed{Title: "1990"}},
		{"Eight and a Half (1963) 720p.BRRip.x264.AC3-WAF", Parsed{Title: "Eight and a Half", Year: 1963, Resolution: "720p", Source: "BluRay", Codec: "H.264"}},
		{"The.Movie.tt0111161.720p", Parsed{Title: "The Movie", Resolution: "720p", IMDbID: "tt0111161"}},
		{"2001.A.Space.Odyssey.1968.1080p.BluRay", Parsed{Title: "2001 A Space Odyssey", Year: 1968, Resolution: "1080p", Source: "BluRay"}},
		{"Cortázar. Instrucciones de montaje (II)", Parsed{Title: "Cortázar. Instrucciones de montaje (II)"}},
		{"AquelMartes-Monteaun", Parsed{Title: "AquelMartes-Monteaun"}},
	}
	for _, c := range cases {
		if got := Parse(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Parse(%q)\n got  %+v\n want %+v", c.in, got, c.want)
		}
	}
}

// TestYearBoundaryOverlap guards against a regression where two nearby years
// separated by a single non-digit byte (e.g. a hyphen in a year range) were
// not both discoverable, because the old yearRe pattern baked its boundary
// check into the consumed match, and FindAll only returns non-overlapping
// matches. yearAt must still find the first (leftmost) plausible year, and
// lastYearBefore must still find the last (rightmost) plausible year before
// a limit — even when the two years sit right next to each other.
func TestYearBoundaryOverlap(t *testing.T) {
	s := "Film 1974 1976 Sequel"

	if y, off := yearAt(s); y != 1974 || off != strings.Index(s, "1974") {
		t.Errorf("yearAt(%q) = (%d, %d), want (1974, %d)", s, y, off, strings.Index(s, "1974"))
	}

	if y, off := lastYearBefore(s, len(s)); y != 1976 || off != strings.Index(s, "1976") {
		t.Errorf("lastYearBefore(%q, len) = (%d, %d), want (1976, %d)", s, y, off, strings.Index(s, "1976"))
	}

	// A hyphenated year range with no separating space is an even tighter
	// case: only a single '-' byte separates the two years.
	r := "(1967-1968)"
	if y, off := yearAt(r); y != 1967 || off != strings.Index(r, "1967") {
		t.Errorf("yearAt(%q) = (%d, %d), want (1967, %d)", r, y, off, strings.Index(r, "1967"))
	}
	if y, off := lastYearBefore(r, len(r)); y != 1968 || off != strings.Index(r, "1968") {
		t.Errorf("lastYearBefore(%q, len) = (%d, %d), want (1968, %d)", r, y, off, strings.Index(r, "1968"))
	}
}
