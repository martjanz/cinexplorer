// Package quality ranks versions of the same movie to pick the best one.
package quality

import (
	"cmp"
	"strconv"
	"strings"
	"unicode"
)

type Candidate struct {
	Resolution string // "2160p", "1080p", … as produced by probe.ResolutionLabel or nameparse
	Codec      string // normalized video codec ("hevc", "h264", …)
	Size       int64
}

var resRank = map[string]int{"2160p": 7, "1080p": 6, "1080i": 6, "720p": 5, "576p": 4, "480p": 3, "SD": 2}

// codecOrder lists video codecs from most to least efficient.
var codecOrder = []string{"av1", "hevc", "vp9", "h264", "vp8", "mpeg4", "wmv", "rv", "mpeg2", "mpeg1"}

func codecRank(c string) int {
	for i, name := range codecOrder {
		if c == name {
			return len(codecOrder) + 1 - i
		}
	}
	if c != "" {
		return 1 // known to exist, but not ranked
	}
	return 0
}

// Compare orders candidates by resolution, then codec, then size. It returns
// a positive number when a is better than b.
func Compare(a, b Candidate) int {
	if c := cmp.Compare(resRank[a.Resolution], resRank[b.Resolution]); c != 0 {
		return c
	}
	if c := cmp.Compare(codecRank(a.Codec), codecRank(b.Codec)); c != 0 {
		return c
	}
	return cmp.Compare(a.Size, b.Size)
}

var articles = map[string]bool{"the": true, "el": true, "la": true, "los": true, "las": true, "le": true, "les": true, "il": true, "lo": true}

var folds = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a", "å", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o", "ø", "o",
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c", "æ", "ae", "œ", "oe", "ß", "ss",
)

// GroupKey returns the provisional identity of a movie: its title folded to
// lowercase ASCII words without a leading article, plus the year. Versions
// with the same key are treated as the same movie until TMDB ids exist. An
// empty title yields "", which never groups.
func GroupKey(title string, year int) string {
	s := folds.Replace(strings.ToLower(title))
	words := strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if len(words) > 1 && articles[words[0]] {
		words = words[1:]
	}
	if len(words) == 0 {
		return ""
	}
	return strings.Join(words, " ") + "|" + strconv.Itoa(year)
}
