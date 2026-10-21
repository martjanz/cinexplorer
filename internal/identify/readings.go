package identify

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"cinexplorer/internal/quality"
)

// cutset is trimmed from both ends of titles and directors.
const cutset = " -–,."

// readings returns the other interpretations of a parsed name, such as
// "Azor - Andreas Fontana" read as director first. Each reading keeps the
// year and runtime of q. The result is pure: it never looks at TMDB.
func readings(q Query) []Query {
	t := trimSide(q.Title)
	base := quality.NormTitle(q.Title)

	var out []Query
	type key struct{ title, director string }
	seen := map[key]bool{}
	add := func(title, director string) {
		title, director = trimSide(title), trimSide(director)
		if title == "" || isDigits(title) || isDigits(director) {
			return
		}
		if quality.NormTitle(title) == base {
			return
		}
		k := key{quality.NormTitle(title), quality.NormTitle(director)}
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, Query{Title: title, Director: director, Year: q.Year, Runtime: q.Runtime})
	}

	// A dash split point: the spaced one, or else the unspaced one between
	// letters when its sides have a fitting number of words.
	left, right, dash := splitSpaced(t)
	if !dash {
		left, right, dash = splitUnspaced(t)
	}
	if dash {
		add(left, right)
		add(right, left)
	}

	// The last comma, when the right side is 1 to 4 words long.
	if i := strings.LastIndex(t, ","); i >= 0 {
		if n := len(strings.Fields(t[i+1:])); n >= 1 && n <= 4 {
			add(t[:i], t[i+1:])
		}
	}

	// Trailing words as the director. Blocked by any dash split point or comma.
	if q.Director == "" && !dash && !strings.Contains(t, ",") {
		words := strings.Fields(t)
		n := len(words)
		if n >= 3 || (n == 2 && startsUpper(words[1])) {
			add(strings.Join(words[:n-1], " "), words[n-1])
			if n >= 3 {
				add(strings.Join(words[:n-2], " "), strings.Join(words[n-2:], " "))
			}
		}
	}

	// Title and director crossed, when a director is known.
	if q.Director != "" {
		add(q.Director, q.Title)
	}
	return out
}

// splitSpaced splits at the first " - " or " – ".
func splitSpaced(t string) (left, right string, ok bool) {
	best, sepLen := -1, 0
	for _, sep := range []string{" - ", " – "} {
		if i := strings.Index(t, sep); i >= 0 && (best < 0 || i < best) {
			best, sepLen = i, len(sep)
		}
	}
	if best < 0 {
		return "", "", false
	}
	return t[:best], t[best+sepLen:], true
}

// splitUnspaced splits at the first dash between two letters, when the left
// side has 1 to 3 words and the right side 2 or more.
func splitUnspaced(t string) (left, right string, ok bool) {
	runes := []rune(t)
	for i := 1; i < len(runes)-1; i++ {
		if runes[i] != '-' || !unicode.IsLetter(runes[i-1]) || !unicode.IsLetter(runes[i+1]) {
			continue
		}
		left, right = trimSide(string(runes[:i])), trimSide(string(runes[i+1:]))
		nl, nr := len(strings.Fields(left)), len(strings.Fields(right))
		return left, right, nl >= 1 && nl <= 3 && nr >= 2
	}
	return "", "", false
}

// trimSide trims the cutset from both ends.
func trimSide(s string) string {
	return strings.Trim(s, cutset)
}

// isDigits reports whether s is non-empty and made only of digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// startsUpper reports whether w begins with an uppercase letter.
func startsUpper(w string) bool {
	r, _ := utf8.DecodeRuneInString(w)
	return unicode.IsUpper(r)
}
