package identify

import (
	"slices"

	"cinexplorer/internal/quality"
)

// Weights of the confidence score (they add up to 1).
const (
	titleWeight    = 0.60
	yearWeight     = 0.25
	directorWeight = 0.15
)

// similarity compares two titles after normalization: 1 − edit distance /
// longer length, over runes. Empty titles are not similar to anything.
func similarity(a, b string) float64 {
	ra, rb := []rune(quality.NormTitle(a)), []rune(quality.NormTitle(b))
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	return 1 - float64(levenshtein(ra, rb))/float64(max(len(ra), len(rb)))
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// titleScore is the best similarity of the parsed title to either title of
// the candidate, weighted.
func titleScore(parsed, title, original string) float64 {
	return titleWeight * max(similarity(parsed, title), similarity(parsed, original))
}

// yearScore rewards an exact year and tolerates ±1 (festival vs release
// dates). Without a parsed year it gives a neutral share.
func yearScore(parsed, candidate int) float64 {
	switch {
	case parsed == 0:
		return 0.10
	case candidate == parsed:
		return yearWeight
	case candidate != 0 && (candidate == parsed-1 || candidate == parsed+1):
		return 0.15
	}
	return 0
}

// directorMatches reports whether the parsed director (often just a
// surname: "Polanski", "Fellini") names one of the candidate's directors:
// its last word must be one of a director's words.
func directorMatches(parsed string, directors []string) bool {
	words := quality.Words(parsed)
	if len(words) == 0 {
		return false
	}
	last := words[len(words)-1]
	for _, d := range directors {
		if slices.Contains(quality.Words(d), last) {
			return true
		}
	}
	return false
}
