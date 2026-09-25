// Package grouping turns the flat list of library files into versions: the
// set of files that play as one unit, plus their subtitles and extras.
package grouping

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"cinexplorer/internal/mediafile"
	"cinexplorer/internal/nameparse"
)

type Entry struct {
	Path string // catalog form, '/'-separated
	Size int64
	Kind mediafile.Kind
}

type Role string

const (
	RoleMain     Role = "main"
	RoleSubtitle Role = "subtitle"
	RoleExtra    Role = "extra"
)

type Member struct {
	Path string
	Role Role
	Part int    // 1-based for multi-part versions, 0 otherwise
	Lang string // subtitles only
}

type Version struct {
	Dir     string
	Members []Member
	Parsed  nameparse.Parsed
	Size    int64 // main files only
	Parts   int
}

// SubLangs returns the sorted, unique subtitle languages ("?" when unknown).
func (v Version) SubLangs() []string {
	seen := map[string]bool{}
	for _, m := range v.Members {
		if m.Role == RoleSubtitle {
			l := m.Lang
			if l == "" {
				l = "?"
			}
			seen[l] = true
		}
	}
	out := make([]string, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

var (
	partRe        = regexp.MustCompile(`(?i)[\s._\-(\[]*\b(?:part|pt|cd|disc|disk|parte)[\s._\-]*(\d{1,2})\b[)\]]?`)
	extraRe       = regexp.MustCompile(`(?i)\b(?:bonus|extras?|featurettes?|trailer|sample|making[\s._-]?of|behind[\s._-]the[\s._-]scenes|deleted[\s._-]scenes)\b`)
	extraDirs     = map[string]bool{"extras": true, "extra": true, "bonus": true, "featurettes": true, "special features": true}
	subDirs       = map[string]bool{"subs": true, "subtitles": true, "subtitulos": true, "subtítulos": true}
	subQualifiers = map[string]bool{"forced": true, "sdh": true, "hi": true, "cc": true}
	roleOrder     = map[Role]int{RoleMain: 0, RoleSubtitle: 1, RoleExtra: 2}
)

var langCodes = map[string]string{
	"es": "es", "spa": "es", "esp": "es", "spanish": "es", "español": "es", "espanol": "es", "castellano": "es", "latino": "es",
	"en": "en", "eng": "en", "english": "en", "ingles": "en", "inglés": "en",
	"it": "it", "ita": "it", "italian": "it", "italiano": "it",
	"fr": "fr", "fre": "fr", "fra": "fr", "french": "fr", "frances": "fr", "francés": "fr",
	"de": "de", "ger": "de", "deu": "de", "german": "de", "aleman": "de", "alemán": "de",
	"pt": "pt", "por": "pt", "portuguese": "pt", "portugues": "pt", "português": "pt",
	"ru": "ru", "rus": "ru", "russian": "ru",
	"ja": "ja", "jpn": "ja", "japanese": "ja",
}

// Build groups entries into versions. roots are the catalog-form library
// roots; a folder that is a root is never used as a movie name.
func Build(entries []Entry, roots []string) []Version {
	isRoot := map[string]bool{}
	for _, r := range roots {
		isRoot[path.Clean(r)] = true
	}
	byDir := map[string][]Entry{}
	for _, e := range entries {
		d := ownerDir(e.Path)
		byDir[d] = append(byDir[d], e)
	}
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	var out []Version
	for _, d := range dirs {
		out = append(out, buildDir(d, byDir[d], isRoot[d])...)
	}
	return out
}

func ownerDir(p string) string {
	dir := path.Dir(p)
	base := strings.ToLower(path.Base(dir))
	if base == "video_ts" || extraDirs[base] || subDirs[base] {
		return path.Dir(dir)
	}
	return dir
}

func buildDir(dir string, es []Entry, dirIsRoot bool) []Version {
	sort.Slice(es, func(i, j int) bool { return es[i].Path < es[j].Path })
	var largest int64
	for _, e := range es {
		if e.Kind == mediafile.Video && e.Size > largest {
			largest = e.Size
		}
	}

	var versions []*Version
	keys := map[*Version]string{} // lowercase base name, for subtitle matching
	byKey := map[string]*Version{}
	var extras []Member
	var dvd *Version

	for _, e := range es {
		switch e.Kind {
		case mediafile.Video:
			name := stem(e.Path)
			if isExtra(e, name, largest) {
				extras = append(extras, Member{Path: e.Path, Role: RoleExtra})
				continue
			}
			base, part := splitPart(name)
			key := strings.ToLower(base)
			if part == 0 {
				key += "|" + strings.ToLower(path.Ext(e.Path))
			}
			v := byKey[key]
			if v == nil {
				v = &Version{Dir: dir, Parsed: nameparse.Parse(base)}
				byKey[key] = v
				keys[v] = strings.ToLower(base)
				versions = append(versions, v)
			}
			v.Members = append(v.Members, Member{Path: e.Path, Role: RoleMain, Part: part})
			v.Size += e.Size
		case mediafile.DVD:
			if dvd == nil {
				dvd = &Version{Dir: dir, Parsed: nameparse.Parse(path.Base(dir))}
				versions = append(versions, dvd)
			}
			dvd.Members = append(dvd.Members, Member{Path: e.Path, Role: RoleMain})
			dvd.Size += e.Size
		}
	}
	if len(versions) == 0 {
		return nil
	}
	if len(versions) == 1 && !dirIsRoot && versions[0] != dvd {
		versions[0].Parsed = nameparse.Merge(versions[0].Parsed, nameparse.Parse(path.Base(dir)))
	}

	for _, e := range es {
		if e.Kind != mediafile.Subtitle {
			continue
		}
		s := stem(e.Path)
		if v := subtitleOwner(strings.ToLower(s), versions, keys); v != nil {
			v.Members = append(v.Members, Member{Path: e.Path, Role: RoleSubtitle, Lang: subLang(s)})
		}
	}
	if len(versions) == 1 {
		versions[0].Members = append(versions[0].Members, extras...)
	}

	out := make([]Version, 0, len(versions))
	for _, v := range versions {
		v.Parts = 0
		for _, m := range v.Members {
			if m.Role == RoleMain {
				v.Parts++
			}
		}
		if v == dvd {
			v.Parts = 1
		}
		sortMembers(v.Members)
		out = append(out, *v)
	}
	return out
}

func isExtra(e Entry, name string, largest int64) bool {
	if extraDirs[strings.ToLower(path.Base(path.Dir(e.Path)))] {
		return true
	}
	if extraRe.MatchString(name) {
		return true
	}
	return largest > 0 && e.Size*100 < largest*15
}

// splitPart removes a part marker ("CD1", "Part 2", "Disc 1") from name.
func splitPart(name string) (string, int) {
	loc := partRe.FindStringSubmatchIndex(name)
	if loc == nil {
		return name, 0
	}
	n, _ := strconv.Atoi(name[loc[2]:loc[3]])
	return strings.TrimSpace(name[:loc[0]] + name[loc[1]:]), n
}

func subtitleOwner(sub string, versions []*Version, keys map[*Version]string) *Version {
	var best *Version
	bestLen := 0
	for _, v := range versions {
		k := keys[v]
		if k != "" && strings.HasPrefix(sub, k) && len(k) > bestLen {
			best, bestLen = v, len(k)
		}
	}
	if best == nil && len(versions) == 1 {
		best = versions[0]
	}
	return best
}

// subLang reads the language from the last token of a subtitle name
// ("Movie.spa", "English", "Movie.en.forced").
func subLang(stem string) string {
	tokens := strings.FieldsFunc(strings.ToLower(stem), func(r rune) bool {
		return strings.ContainsRune(" ._-[]()", r)
	})
	for i := len(tokens) - 1; i >= 0; i-- {
		if subQualifiers[tokens[i]] {
			continue
		}
		return langCodes[tokens[i]]
	}
	return ""
}

func sortMembers(ms []Member) {
	sort.SliceStable(ms, func(i, j int) bool {
		if roleOrder[ms[i].Role] != roleOrder[ms[j].Role] {
			return roleOrder[ms[i].Role] < roleOrder[ms[j].Role]
		}
		if ms[i].Part != ms[j].Part {
			return ms[i].Part < ms[j].Part
		}
		return ms[i].Path < ms[j].Path
	})
}

func stem(p string) string {
	b := path.Base(p)
	return strings.TrimSuffix(b, path.Ext(b))
}
