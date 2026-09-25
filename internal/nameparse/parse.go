// Package nameparse extracts title, year, director and release details from
// the free-form file and folder names found in a personal movie library.
package nameparse

import (
	"regexp"
	"strconv"
	"strings"
)

type Parsed struct {
	Title      string
	Year       int
	Director   string
	Countries  []string
	Resolution string
	Source     string
	Codec      string
	Language   string
	Group      string
	IMDbID     string
}

var (
	noiseRe        = regexp.MustCompile(`(?i)\(?(?:found\.via\.|emule\.via\.)?clan-sudamerica\.net\)?|\(?www\.[^\s()\[\]]+\)?|\s-\s*youtube\b|\[(?:yts|rarbg|eztv)[^\]]*\]|\[mkvonly\]|\[vostf\]|\[vost\s*fr\]|\[[^\[\]]*\.(?:com|net|org)\]`)
	imdbRe         = regexp.MustCompile(`\btt\d{7,8}\b`)
	yearDigitsRe   = regexp.MustCompile(`(?:18|19|20)\d{2}`)
	bracketRe      = regexp.MustCompile(`[(\[]([^()\[\]]*)[)\]]`)
	resRe          = regexp.MustCompile(`(?i)\b(?:2160p|1080p|1080i|720p|576p|480p|4k)\b`)
	sourceRe       = regexp.MustCompile(`(?i)\b(?:blu-?ray|brrip|bdrip|web-?dl|webrip|dvdrip|dvdscr|hdtv|hdrip|vhsrip|tvrip)\b`)
	codecRe        = regexp.MustCompile(`(?i)\b(?:x26[45]|h[ .]?26[45]|hevc|xvid|divx|av1)\b`)
	langRe         = regexp.MustCompile(`(?i)\b(?:spanish|castellano|latino|english|german|french|italian|portuguese)\b`)
	releaseGroupRe = regexp.MustCompile(`-([A-Za-z0-9]+)$`)
	yearPrefixRe   = regexp.MustCompile(`^\s*((?:18|19|20)\d{2})\s+-\s+(.+)$`)
	lastFirstRe    = regexp.MustCompile(`^\s*([^,\-]+),\s*([^,\-]+?)\s+-\s+(.+)$`)
	// listPrefixRe strips a numbered-list index ("01 - Title") from curated
	// collection folders (e.g. "100 mejores pelis argentinas"). Capped at 3
	// digits so it never overlaps a 4-digit year, which yearPrefixRe already
	// handles.
	listPrefixRe = regexp.MustCompile(`^\s*\d{1,3}\s*-\s+(.+)$`)
	// leadingYearBracketRe matches a "[YYYY]" or "(YYYY)" tag at the very
	// start of a name. Some collections repeat the year here in addition to
	// a later "(Year, ...)" group; lastYearGroup always consumes the LAST
	// bracket group with a year, so without stripping this redundant leading
	// tag first it is left dangling in the title with its closing bracket
	// unbalanced (tidy only trims from the very edges).
	leadingYearBracketRe = regexp.MustCompile(`^\s*[\[(](?:18|19|20)\d{2}[\])]\s*`)

	sceneSeparators = strings.NewReplacer(".", " ", "_", " ")
)

var countryNames = map[string]bool{
	"usa": true, "uk": true, "eeuu": true, "urss": true, "ussr": true,
	"argentina": true, "brasil": true, "brazil": true, "chile": true, "uruguay": true, "méxico": true, "mexico": true,
	"italia": true, "italy": true, "france": true, "francia": true, "germany": true, "alemania": true,
	"españa": true, "spain": true, "portugal": true, "polonia": true, "poland": true, "suecia": true, "sweden": true,
	"dinamarca": true, "denmark": true, "hungría": true, "hungary": true, "rusia": true, "russia": true,
	"japan": true, "japón": true, "china": true, "india": true, "iran": true, "irán": true, "korea": true, "corea": true,
	"canada": true, "canadá": true,
}

// Parse extracts what it can from a single name (file stem or folder name).
func Parse(name string) Parsed {
	var p Parsed
	s := noiseRe.ReplaceAllString(name, " ")
	leadYear := 0
	if m := leadingYearBracketRe.FindString(s); m != "" {
		leadYear, _ = strconv.Atoi(yearDigitsRe.FindString(m))
		s = leadingYearBracketRe.ReplaceAllString(s, " ")
	}
	if id := imdbRe.FindString(s); id != "" {
		p.IMDbID = id
		s = imdbRe.ReplaceAllString(s, " ")
	}
	p.Resolution = normRes(resRe.FindString(s))
	p.Source = normSource(sourceRe.FindString(s))
	p.Codec = normCodec(codecRe.FindString(s))
	p.Language = normLang(langRe.FindString(s))
	if t := strings.TrimSpace(s); !strings.Contains(t, " ") && hasTech(t) {
		if m := releaseGroupRe.FindStringSubmatch(t); m != nil {
			p.Group = m[1]
		}
	}

	// Scene-style names use dots or underscores as word separators.
	if n := strings.Count(s, ".") + strings.Count(s, "_"); n >= 3 || (n > 0 && !strings.Contains(strings.TrimSpace(s), " ")) {
		s = sceneSeparators.Replace(s)
	}

	if loc, inner := lastYearGroup(s); loc != nil {
		p.Year, p.Director, p.Countries = parseGroup(inner)
		if before := s[:loc[0]]; strings.TrimSpace(before) != "" {
			s = before
		} else {
			s = s[loc[1]:]
		}
		s = s[:techIndex(s, false)]
	} else {
		cut := techIndex(s, false)
		if y, off := lastYearBefore(s, cut); y != 0 {
			p.Year = y
			s = s[:off]
		} else {
			s = s[:techIndex(s, true)]
		}
	}

	s = applyTitlePatterns(s, &p)
	p.Title = tidy(s)
	if p.Title == "" {
		p.Title = tidy(sceneSeparators.Replace(name))
	}
	if p.Year == 0 {
		p.Year = leadYear
	}
	return p
}

func hasTech(s string) bool {
	return resRe.MatchString(s) || sourceRe.MatchString(s) || codecRe.MatchString(s)
}

// techIndex returns the offset of the first release token (resolution, source,
// codec and, optionally, language) or len(s) if there is none.
func techIndex(s string, withLang bool) int {
	cut := len(s)
	res := []*regexp.Regexp{resRe, sourceRe, codecRe}
	if withLang {
		res = append(res, langRe)
	}
	for _, re := range res {
		if loc := re.FindStringIndex(s); loc != nil && loc[0] < cut {
			cut = loc[0]
		}
	}
	return cut
}

// yearBoundaryOK reports whether the 4-digit run s[start:end] is not itself
// part of a longer digit run and is not immediately followed by 'p' (e.g. the
// "1080p" resolution tag). It replaces boundary bytes that used to be baked
// into the match itself (and thus consumed, breaking FindAll on two nearby
// years) with a peek at the surrounding bytes.
func yearBoundaryOK(s string, start, end int) bool {
	if start > 0 {
		c := s[start-1]
		if c >= '0' && c <= '9' {
			return false
		}
	}
	if end < len(s) {
		c := s[end]
		if (c >= '0' && c <= '9') || c == 'p' {
			return false
		}
	}
	return true
}

// yearAt returns the first plausible year in s and its offset, or 0, -1.
func yearAt(s string) (int, int) {
	for _, m := range yearDigitsRe.FindAllStringIndex(s, -1) {
		if !yearBoundaryOK(s, m[0], m[1]) {
			continue
		}
		y, _ := strconv.Atoi(s[m[0]:m[1]])
		if y >= 1880 && y <= 2099 {
			return y, m[0]
		}
	}
	return 0, -1
}

// lastYearBefore returns the last plausible year before limit that is not at
// the very start of s (a leading number is part of the title: "2001 A Space…").
func lastYearBefore(s string, limit int) (int, int) {
	year, at := 0, -1
	for _, m := range yearDigitsRe.FindAllStringIndex(s, -1) {
		off := m[0]
		if off >= limit || strings.TrimSpace(s[:off]) == "" {
			continue
		}
		if !yearBoundaryOK(s, m[0], m[1]) {
			continue
		}
		if y, _ := strconv.Atoi(s[m[0]:m[1]]); y >= 1880 && y <= 2099 {
			year, at = y, off
		}
	}
	return year, at
}

// lastYearGroup finds the last (…) or […] group containing a year.
func lastYearGroup(s string) ([]int, string) {
	all := bracketRe.FindAllStringSubmatchIndex(s, -1)
	for i := len(all) - 1; i >= 0; i-- {
		m := all[i]
		inner := s[m[2]:m[3]]
		if y, _ := yearAt(inner); y != 0 {
			return m[:2], inner
		}
	}
	return nil, ""
}

// parseGroup reads "Director, Country, Year" style groups. Parts before the
// year are director or country; parts after the year are countries.
func parseGroup(inner string) (int, string, []string) {
	year, off := yearAt(inner)
	trim := func(x string) string { return strings.Trim(x, " -–.") }
	var director string
	var countries []string
	for _, part := range strings.Split(inner[:off], ",") {
		part = trim(part)
		switch {
		case part == "":
		case isCountry(part):
			countries = append(countries, part)
		case director == "":
			director = part
		}
	}
	for _, part := range strings.Split(inner[off+4:], ",") {
		if part = trim(part); part != "" {
			countries = append(countries, part)
		}
	}
	return year, director, countries
}

func isCountry(s string) bool {
	if countryNames[strings.ToLower(s)] {
		return true
	}
	// Only a bare 2-letter all-caps token (e.g. "UK", "US") is treated as a
	// country by heuristic. 3-letter all-caps tokens are common non-country
	// release tags ("CC", "OST") and must instead be listed explicitly in
	// countryNames (which already covers "usa", "uk", "urss", etc).
	return len(s) == 2 && s == strings.ToUpper(s) && strings.ToLower(s) != s
}

// applyTitlePatterns handles "1976 - Title", "Last, First - Title" and
// "Title_Director" conventions.
func applyTitlePatterns(s string, p *Parsed) string {
	if m := yearPrefixRe.FindStringSubmatch(s); m != nil && p.Year == 0 {
		p.Year, _ = strconv.Atoi(m[1])
		return m[2]
	}
	if m := listPrefixRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	if m := lastFirstRe.FindStringSubmatch(s); m != nil && p.Director == "" {
		p.Director = strings.TrimSpace(m[2]) + " " + strings.TrimSpace(m[1])
		return m[3]
	}
	if strings.Count(s, "_") == 1 && strings.Contains(s, " ") && p.Director == "" {
		i := strings.Index(s, "_")
		p.Director = tidy(s[i+1:])
		return s[:i]
	}
	return s
}

func tidy(s string) string {
	return strings.Trim(strings.Join(strings.Fields(s), " "), " -–_.,([")
}

func normRes(s string) string {
	s = strings.ToLower(s)
	if s == "4k" {
		return "2160p"
	}
	return s
}

func normSource(s string) string {
	switch strings.ToLower(strings.ReplaceAll(s, "-", "")) {
	case "bluray", "brrip", "bdrip":
		return "BluRay"
	case "webdl":
		return "WEB-DL"
	case "webrip":
		return "WEBRip"
	case "dvdrip", "dvdscr":
		return "DVDRip"
	case "hdtv":
		return "HDTV"
	case "hdrip":
		return "HDRip"
	case "vhsrip":
		return "VHSRip"
	case "tvrip":
		return "TVRip"
	}
	return s
}

var codecSeparators = strings.NewReplacer(".", "", " ", "")

func normCodec(s string) string {
	switch codecSeparators.Replace(strings.ToLower(s)) {
	case "x264", "h264":
		return "H.264"
	case "x265", "h265", "hevc":
		return "H.265"
	case "xvid":
		return "XviD"
	case "divx":
		return "DivX"
	case "av1":
		return "AV1"
	}
	return s
}

var langNames = map[string]string{
	"spanish": "es", "castellano": "es", "latino": "es", "english": "en", "german": "de",
	"french": "fr", "italian": "it", "portuguese": "pt",
}

func normLang(s string) string { return langNames[strings.ToLower(s)] }
