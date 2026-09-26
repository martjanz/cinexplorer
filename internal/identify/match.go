// Package identify matches versions to TMDB movies, enriches them with TMDB
// and Wikidata data, and keeps doing so in the background while offline.
package identify

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// MatcherVersion changes whenever the matching algorithm or its thresholds
// change, so earlier automatic results are recomputed.
const MatcherVersion = 1

// A version is assigned automatically when its best candidate scores at
// least AutoThreshold and beats the runner-up by at least AutoMargin.
const (
	AutoThreshold = 0.80
	AutoMargin    = 0.10
)

const (
	searchLimit    = 10 // search results scored
	directorChecks = 3  // best candidates whose credits are fetched
	keptCandidates = 5  // candidates stored for review
)

// API is the part of the TMDB client the matcher uses.
type API interface {
	SearchMovie(ctx context.Context, query string, year int, lang string) ([]tmdb.Result, error)
	FindIMDb(ctx context.Context, imdbID, lang string) ([]tmdb.Result, error)
	Movie(ctx context.Context, id int, lang string) (tmdb.Details, error)
}

// Query is what is known about a version from its names and .nfo.
type Query struct {
	Title    string
	Year     int
	Director string
	IMDbID   string
	// Folder is the parse of the version's folder name, tried when the
	// query itself is not conclusive (nil when the folder does not look like
	// a movie name).
	Folder *Query
}

// Key identifies the query; a stored result made for another key is stale.
func (q Query) Key() string {
	k := fmt.Sprintf("%s|%d|%s|%s", q.Title, q.Year, q.Director, q.IMDbID)
	if q.Folder != nil {
		k += "|" + q.Folder.Key()
	}
	return k
}

// Search returns scored candidates, best first, and the details fetched on
// the way (in lang), which enrichment can reuse. An IMDb id that TMDB knows
// yields that movie alone with score 1.
func Search(ctx context.Context, api API, lang string, q Query) ([]store.Candidate, map[int]tmdb.Details, error) {
	details := map[int]tmdb.Details{}
	if q.IMDbID != "" {
		rs, err := api.FindIMDb(ctx, q.IMDbID, lang)
		if err != nil {
			return nil, nil, err
		}
		if len(rs) > 0 {
			c := candidate(rs[0])
			c.Score = 1
			return []store.Candidate{c}, details, nil
		}
	}
	variants := titleVariants(q.Title)
	if len(variants) == 0 {
		return []store.Candidate{}, details, nil
	}
	hits, err := search(ctx, api, variants[0], q.Year, lang)
	if err != nil {
		return nil, nil, err
	}
	cands := score(q, variants, hits)
	// An alternative title in brackets ("Solyaris [Solaris]") is searched
	// too when the main one is not conclusive.
	for _, v := range variants[1:] {
		if conclusive(cands) {
			break
		}
		more, err := search(ctx, api, v, q.Year, lang)
		if err != nil {
			return nil, nil, err
		}
		hits = merge(hits, more, false)
		cands = score(q, variants, hits)
	}
	// Files are often named with the English title, which a localized
	// search shows neither as title nor as original title: when the result is
	// not conclusive, the English titles are looked at too.
	if lang != fallbackLang && !conclusive(cands) {
		en, err := search(ctx, api, variants[0], q.Year, fallbackLang)
		if err != nil {
			return nil, nil, err
		}
		hits = merge(hits, en, true)
		cands = score(q, variants, hits)
	}
	if q.Director != "" {
		for i := range min(directorChecks, len(cands)) {
			d, err := api.Movie(ctx, cands[i].TMDBID, lang)
			if err != nil {
				return nil, nil, err
			}
			details[d.ID] = d
			var names []string
			for _, p := range d.Directors() {
				names = append(names, p.Name)
			}
			if directorMatches(q.Director, names) {
				cands[i].Score += directorWeight
			}
		}
		sortCandidates(cands)
	}
	return cands, details, nil
}

// Identify decides what a query is: StatusAuto with the movie, or
// StatusUnmatched with the best candidates for review. When the query is not
// conclusive its folder query is tried, and the better outcome is kept.
func Identify(ctx context.Context, api API, lang string, q Query) (store.Identification, map[int]tmdb.Details, error) {
	id, details, err := identify(ctx, api, lang, q)
	if err != nil {
		return id, nil, err
	}
	if q.Folder != nil && id.Status != store.StatusAuto {
		alt, more, err := identify(ctx, api, lang, *q.Folder)
		if err != nil {
			return id, nil, err
		}
		for k, d := range more {
			details[k] = d
		}
		if alt.Status == store.StatusAuto || alt.Confidence > id.Confidence {
			id = alt
		}
	}
	id.Query = q.Key()
	return id, details, nil
}

func identify(ctx context.Context, api API, lang string, q Query) (store.Identification, map[int]tmdb.Details, error) {
	cands, details, err := Search(ctx, api, lang, q)
	if err != nil {
		return store.Identification{}, nil, err
	}
	id := store.Identification{Status: store.StatusUnmatched, MatcherVersion: MatcherVersion}
	if len(cands) > 0 {
		if conclusive(cands) {
			id.Status, id.TMDBID = store.StatusAuto, cands[0].TMDBID
		}
		id.Confidence = cands[0].Score
	}
	id.Candidates = cands[:min(keptCandidates, len(cands))]
	return id, details, nil
}

// fallbackLang is the language of the second search and of missing texts.
const fallbackLang = "en-US"

// hit is a search result plus its English title when known.
type hit struct {
	tmdb.Result
	english string
}

// titleVariants splits "La piel dura (L'argent de poche)" into the title
// outside brackets followed by each bracketed part; a title without
// brackets is its own only variant.
func titleVariants(title string) []string {
	var out []string
	outside := strings.TrimSpace(strings.Join(strings.Fields(bracketRe.ReplaceAllString(title, " ")), " "))
	if outside != "" {
		out = append(out, outside)
	}
	for _, m := range bracketRe.FindAllStringSubmatch(title, -1) {
		if v := strings.TrimSpace(m[1]); v != "" {
			out = append(out, v)
		}
	}
	return out
}

var bracketRe = regexp.MustCompile(`[(\[]([^()\[\]]*)[)\]]`)

// search looks a title up with its year, then without it if nothing comes
// back, keeping at most searchLimit results.
func search(ctx context.Context, api API, title string, year int, lang string) ([]hit, error) {
	rs, err := api.SearchMovie(ctx, title, year, lang)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 && year > 0 {
		if rs, err = api.SearchMovie(ctx, title, 0, lang); err != nil {
			return nil, err
		}
	}
	hits := make([]hit, 0, min(len(rs), searchLimit))
	for _, r := range rs[:min(len(rs), searchLimit)] {
		hits = append(hits, hit{Result: r})
	}
	return hits, nil
}

// merge adds the movies of more that hits lacks, up to searchLimit in total.
// With english set, more comes from an English search and its titles are
// recorded as the English titles of the movies.
func merge(hits, more []hit, english bool) []hit {
	pos := map[int]int{}
	for i, h := range hits {
		pos[h.ID] = i
	}
	for _, h := range more {
		if english {
			h.english = h.Title
		}
		if i, ok := pos[h.ID]; ok {
			if english {
				hits[i].english = h.Title
			}
		} else if len(hits) < searchLimit {
			pos[h.ID] = len(hits)
			hits = append(hits, h)
		}
	}
	return hits
}

// score rates hits on title and year (rescaled to 0..1 when no director is
// known, since the director share cannot be earned), best first. The title
// share is the best similarity between any variant of the parsed title and
// the movie's localized, original or English title.
func score(q Query, variants []string, hits []hit) []store.Candidate {
	cands := make([]store.Candidate, len(hits))
	for i, h := range hits {
		cands[i] = candidate(h.Result)
		t := 0.0
		for _, v := range variants {
			t = max(t, titleScore(v, h.Title, h.OriginalTitle), titleWeight*similarity(v, h.english))
		}
		cands[i].Score = t + yearScore(q.Year, h.Year())
		if q.Director == "" {
			cands[i].Score /= titleWeight + yearWeight
		}
	}
	sortCandidates(cands)
	return cands
}

// conclusive reports whether the best candidate would be assigned.
func conclusive(cands []store.Candidate) bool {
	return len(cands) > 0 && cands[0].Score >= AutoThreshold &&
		(len(cands) == 1 || cands[0].Score-cands[1].Score >= AutoMargin)
}

func candidate(r tmdb.Result) store.Candidate {
	return store.Candidate{TMDBID: r.ID, Title: r.Title, OriginalTitle: r.OriginalTitle, Year: r.Year(), PosterPath: r.PosterPath}
}

// sortCandidates orders by score, keeping TMDB's order (popularity) on ties.
func sortCandidates(cs []store.Candidate) {
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].Score > cs[j].Score })
}
