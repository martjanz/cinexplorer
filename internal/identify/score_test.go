package identify

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		want float64
	}{
		{"Amarcord", "AMARCORD", 1},
		{"El Ángel Exterminador", "angel exterminador", 1},
		{"Solyaris", "Solaris", 1 - 1.0/8},
		{"", "Solaris", 0},
		{"abc", "xyz", 0},
	}
	for _, c := range cases {
		if got := similarity(c.a, c.b); !near(got, c.want) {
			t.Errorf("similarity(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestYearScore(t *testing.T) {
	cases := []struct {
		parsed, cand int
		want         float64
	}{
		{1973, 1973, 0.25}, {1973, 1974, 0.15}, {1973, 1972, 0.15}, {1973, 1975, 0},
		{1973, 0, 0}, {0, 1973, 0.10}, {0, 0, 0.10},
	}
	for _, c := range cases {
		if got := yearScore(c.parsed, c.cand); !near(got, c.want) {
			t.Errorf("yearScore(%d, %d) = %v, want %v", c.parsed, c.cand, got, c.want)
		}
	}
}

func TestDirectorMatches(t *testing.T) {
	cases := []struct {
		parsed string
		dirs   []string
		want   bool
	}{
		{"Polanski", []string{"Roman Polański"}, true},
		{"Fellini", []string{"Federico Fellini"}, true},
		{"Federico Fellini", []string{"Federico Fellini"}, true},
		{"Wong Kar-wai", []string{"Wong Kar-wai"}, true},
		{"Lynch", []string{"David Lynch", "Mark Frost"}, true},
		{"Donner", []string{"Richard Lester"}, false},
		{"", []string{"Anyone"}, false},
	}
	for _, c := range cases {
		if got := directorMatches(c.parsed, c.dirs); got != c.want {
			t.Errorf("directorMatches(%q, %q) = %v", c.parsed, c.dirs, got)
		}
	}
}
