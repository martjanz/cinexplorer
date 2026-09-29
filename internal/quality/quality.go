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
	"ą", "a", "ę", "e", "ı", "i", "ý", "y", "ÿ", "y",
	"ć", "c", "č", "c", "ď", "d", "đ", "d", "ğ", "g", "ł", "l", "ń", "n", "ň", "n",
	"ř", "r", "ś", "s", "š", "s", "ş", "s", "ť", "t", "ź", "z", "ż", "z", "ž", "z",
)

// StripAccents removes accents from s, keeping case, spaces and punctuation.
func StripAccents(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsUpper(r) {
			if f := []rune(folds.Replace(string(unicode.ToLower(r)))); len(f) == 1 {
				return unicode.ToUpper(f[0])
			}
			return r
		}
		if f := []rune(folds.Replace(string(r))); len(f) == 1 {
			return f[0]
		}
		return r
	}, s)
}

// Words folds s to lowercase ASCII-ish words: accents removed, split on
// anything that is not a letter or digit.
func Words(s string) []string {
	s = folds.Replace(strings.ToLower(s))
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// NormTitle is a title reduced for comparison: folded words without a
// leading article, joined by single spaces. A lone article is kept.
func NormTitle(title string) string {
	words := Words(title)
	if len(words) > 1 && articles[words[0]] {
		words = words[1:]
	}
	return strings.Join(words, " ")
}

// GroupKey returns the provisional identity of a movie: its normalized title
// plus the year. Versions with the same key are treated as the same movie
// when they have no TMDB id. An empty title yields "", which never groups.
func GroupKey(title string, year int) string {
	t := NormTitle(title)
	if t == "" {
		return ""
	}
	return t + "|" + strconv.Itoa(year)
}
