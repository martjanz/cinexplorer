package catalog

import (
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Facet names, as they appear in Explorar's URL.
const (
	FacetDecade     = "decada"
	FacetYear       = "anio"
	FacetDirector   = "director"   // TMDB person id
	FacetGenre      = "genero"     // localized name
	FacetCountry    = "pais"       // ISO 3166-1 alpha-2
	FacetLanguage   = "idioma"     // original language, ISO 639-1
	FacetCollection = "coleccion"  // TMDB collection id
	FacetResolution = "resolucion" // "4K", "1080p", "720p", "SD"
	FacetSubs       = "subs"       // subtitle language
	FacetLocation   = "ubicacion"  // "cine" or "cine/1970s"
	FacetState      = "estado"     // StateUnidentified, StateVersions or StateIdentical
)

// FacetNames lists the facets in the order of Explorar's bar.
var FacetNames = []string{FacetDecade, FacetYear, FacetDirector, FacetGenre, FacetCountry, FacetLanguage,
	FacetCollection, FacetResolution, FacetSubs, FacetLocation, FacetState}

// Values of FacetState.
const (
	StateUnidentified = "sin-identificar"
	StateVersions     = "varias-versiones"
	StateIdentical    = "copia-identica"
)

// Sort orders and directions.
const (
	OrderYear  = "anio"
	OrderTitle = "titulo"
	OrderAdded = "agregado"
	OrderSize  = "tamano"
	Asc        = "asc"
	Desc       = "desc"
)

// Query is what Explorar asks for: one value per facet, and an order.
type Query struct {
	Facets map[string]string `json:"facets"`
	Order  string            `json:"order"`
	Dir    string            `json:"dir"`
}

var (
	numberRe  = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
	langRe    = regexp.MustCompile(`^[a-z]{2,3}$`)
)

// valid reports whether value is well formed for a facet. A well formed value
// that matches nothing is still applied (and finds nothing).
func valid(facet, value string) bool {
	switch facet {
	case FacetDecade:
		n, err := strconv.Atoi(value)
		return err == nil && numberRe.MatchString(value) && n%10 == 0
	case FacetYear, FacetDirector, FacetCollection:
		return numberRe.MatchString(value)
	case FacetCountry:
		return countryRe.MatchString(value)
	case FacetLanguage, FacetSubs:
		return langRe.MatchString(value)
	case FacetResolution:
		_, ok := resRank[value]
		return ok
	case FacetState:
		return value == StateUnidentified || value == StateVersions || value == StateIdentical
	case FacetGenre, FacetLocation:
		return strings.TrimSpace(value) != ""
	}
	return false
}

// ParseQuery reads Explorar's parameters. Unknown parameters and malformed
// values are dropped; the order defaults to year, newest first, and each
// order has its natural direction (A to Z for titles, largest or newest
// first otherwise).
func ParseQuery(v url.Values) Query {
	q := Query{Facets: map[string]string{}, Order: OrderYear}
	for _, f := range FacetNames {
		if val := v.Get(f); valid(f, val) {
			q.Facets[f] = val
		}
	}
	switch o := v.Get("orden"); o {
	case OrderTitle, OrderAdded, OrderSize:
		q.Order = o
	}
	q.Dir = Desc
	if q.Order == OrderTitle {
		q.Dir = Asc
	}
	if d := v.Get("dir"); d == Asc || d == Desc {
		q.Dir = d
	}
	return q
}

// values lists an item's values for a facet.
func (it *Item) values(facet string) []string {
	switch facet {
	case FacetDecade:
		if it.Year > 0 {
			return []string{strconv.Itoa(it.Year / 10 * 10)}
		}
	case FacetYear:
		if it.Year > 0 {
			return []string{strconv.Itoa(it.Year)}
		}
	case FacetDirector:
		out := make([]string, len(it.directorIDs))
		for i, id := range it.directorIDs {
			out[i] = strconv.Itoa(id)
		}
		return out
	case FacetGenre:
		return it.genres
	case FacetCountry:
		if it.Kind == KindMovie {
			return it.Countries
		}
	case FacetLanguage:
		if it.lang != "" {
			return []string{it.lang}
		}
	case FacetCollection:
		if it.collectionID > 0 {
			return []string{strconv.Itoa(it.collectionID)}
		}
	case FacetResolution:
		return it.resolutions
	case FacetSubs:
		return it.subs
	case FacetLocation:
		return it.locations
	case FacetState:
		var out []string
		if it.unidentified {
			out = append(out, StateUnidentified)
		}
		if it.multiVersion {
			out = append(out, StateVersions)
		}
		if it.identical {
			out = append(out, StateIdentical)
		}
		return out
	}
	return nil
}

func (it *Item) matches(facets map[string]string, except string) bool {
	for f, want := range facets {
		if f == except {
			continue
		}
		found := false
		for _, v := range it.values(f) {
			if v == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Filter keeps the items that match every facet.
func Filter(items []Item, facets map[string]string) []Item {
	out := []Item{}
	for i := range items {
		if items[i].matches(facets, "") {
			out = append(out, items[i])
		}
	}
	return out
}

// FacetValue is one option of a facet's menu.
type FacetValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// Counts lists, for every facet, the values found in the items that match
// the other facets, with how many items have each. So a menu shows what
// choosing a value in it would give. Years, decades and locations come in
// their natural order; the rest, most frequent first.
func Counts(items []Item, facets map[string]string) map[string][]FacetValue {
	out := map[string][]FacetValue{}
	for _, f := range FacetNames {
		counts := map[string]int{}
		labels := map[string]string{}
		for i := range items {
			it := &items[i]
			if !it.matches(facets, f) {
				continue
			}
			for _, v := range it.values(f) {
				counts[v]++
				if _, ok := labels[v]; !ok {
					labels[v] = it.label(f, v)
				}
			}
		}
		vs := make([]FacetValue, 0, len(counts))
		for v, n := range counts {
			vs = append(vs, FacetValue{Value: v, Label: labels[v], Count: n})
		}
		sort.Slice(vs, func(i, j int) bool {
			switch f {
			case FacetDecade, FacetYear:
				return vs[i].Value > vs[j].Value
			case FacetLocation:
				return vs[i].Value < vs[j].Value
			}
			if vs[i].Count != vs[j].Count {
				return vs[i].Count > vs[j].Count
			}
			return vs[i].Label < vs[j].Label
		})
		out[f] = vs
	}
	return out
}

// label is how a facet value reads: names for ids, "1970s" for a decade,
// the value itself otherwise (countries and languages are named by the page).
func (it *Item) label(facet, value string) string {
	switch facet {
	case FacetDecade:
		return value + "s"
	case FacetDirector:
		for i, id := range it.directorIDs {
			if strconv.Itoa(id) == value {
				return it.Directors[i]
			}
		}
	case FacetCollection:
		return it.collection
	}
	return value
}
