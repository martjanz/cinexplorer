// Package wikidata looks movies up in Wikidata by their TMDB id (P4947) to
// fill gaps TMDB leaves: IMDb id, countries, directors and year.
package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"cinexplorer/internal/httpx"
)

// BatchSize is the most TMDB ids Lookup accepts per call.
const BatchSize = 50

type Client struct {
	Endpoint string // "https://query.wikidata.org/sparql"
	HTTP     *httpx.Client
}

// New returns a client that identifies itself as Wikimedia asks, limited to
// one request per second.
func New(version string) *Client {
	return &Client{
		Endpoint: "https://query.wikidata.org/sparql",
		HTTP: &httpx.Client{
			Limiter: &httpx.Limiter{Rate: 1, Burst: 1},
			Header: http.Header{
				"Accept":     {"application/sparql-results+json"},
				"User-Agent": {"cinexplorer/" + version + " (https://github.com/martjanz/cinexplorer)"},
			},
		},
	}
}

// Entity is what Wikidata knows about one movie.
type Entity struct {
	QID       string   // "Q18428"
	IMDbID    string   // "tt0071129"
	Countries []string // ISO 3166-1 alpha-2, sorted
	Directors []string // labels in the requested language (fallback English), sorted
	Year      int      // earliest publication year
}

// Lookup returns the entities for up to BatchSize TMDB ids, keyed by TMDB id.
// Ids Wikidata does not know are absent. lang is a TMDB-style language
// ("es-ES"); labels use its first part with English as fallback.
func (c *Client) Lookup(ctx context.Context, ids []int, lang string) (map[int]Entity, error) {
	if len(ids) == 0 {
		return map[int]Entity{}, nil
	}
	if len(ids) > BatchSize {
		return nil, fmt.Errorf("wikidata: %d ids, max %d", len(ids), BatchSize)
	}
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = strconv.Quote(strconv.Itoa(id))
	}
	labelLang := strings.ToLower(strings.SplitN(lang, "-", 2)[0])
	if labelLang == "" {
		labelLang = "en"
	}
	q := `SELECT ?tmdb ?item ?imdb ?countryIso ?directorLabel ?date WHERE {
  VALUES ?tmdb { ` + strings.Join(values, " ") + ` }
  ?item wdt:P4947 ?tmdb .
  OPTIONAL { ?item wdt:P345 ?imdb . }
  OPTIONAL { ?item wdt:P495 ?country . ?country wdt:P297 ?countryIso . }
  OPTIONAL { ?item wdt:P57 ?director . }
  OPTIONAL { ?item wdt:P577 ?date . }
  SERVICE wikibase:label { bd:serviceParam wikibase:language "` + labelLang + `,en". }
}`
	body, err := c.HTTP.Get(ctx, c.Endpoint+"?"+url.Values{"query": {q}}.Encode())
	if err != nil {
		return nil, err
	}
	return parse(body)
}

type binding map[string]struct {
	Value string `json:"value"`
}

func parse(body []byte) (map[int]Entity, error) {
	var res struct {
		Results struct {
			Bindings []binding `json:"bindings"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("wikidata: %w", err)
	}
	type acc struct {
		e         Entity
		countries map[string]bool
		directors map[string]bool
	}
	byID := map[int]*acc{}
	for _, b := range res.Results.Bindings {
		id, err := strconv.Atoi(b["tmdb"].Value)
		if err != nil {
			continue
		}
		a := byID[id]
		if a == nil {
			a = &acc{countries: map[string]bool{}, directors: map[string]bool{}}
			a.e.QID = strings.TrimPrefix(b["item"].Value, "http://www.wikidata.org/entity/")
			byID[id] = a
		}
		if v := b["imdb"].Value; strings.HasPrefix(v, "tt") && a.e.IMDbID == "" {
			a.e.IMDbID = v
		}
		if v := b["countryIso"].Value; len(v) == 2 {
			a.countries[strings.ToUpper(v)] = true
		}
		// An unlabelled director comes back as its QID: useless as a name.
		if v := b["directorLabel"].Value; v != "" && !isQID(v) {
			a.directors[v] = true
		}
		if v := b["date"].Value; len(v) >= 4 {
			if y, err := strconv.Atoi(v[:4]); err == nil && y > 1800 && (a.e.Year == 0 || y < a.e.Year) {
				a.e.Year = y
			}
		}
	}
	out := make(map[int]Entity, len(byID))
	for id, a := range byID {
		a.e.Countries = keys(a.countries)
		a.e.Directors = keys(a.directors)
		out[id] = a.e
	}
	return out, nil
}

func isQID(s string) bool {
	if len(s) < 2 || s[0] != 'Q' {
		return false
	}
	_, err := strconv.Atoi(s[1:])
	return err == nil
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
