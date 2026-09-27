// Package tmdb is a small client for The Movie Database API v3.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"cinexplorer/internal/httpx"
)

var (
	ErrOffline      = httpx.ErrOffline
	ErrUnauthorized = errors.New("token de TMDB inválido")
	ErrNotFound     = errors.New("no existe en TMDB")
)

type Client struct {
	BaseURL  string // "https://api.themoviedb.org/3"
	ImageURL string // "https://image.tmdb.org/t/p"
	HTTP     *httpx.Client
}

// New returns a client authenticated with a v4 read access token, limited to
// 20 requests per second (TMDB allows about 50).
func New(token string) *Client {
	return &Client{
		BaseURL:  "https://api.themoviedb.org/3",
		ImageURL: "https://image.tmdb.org/t/p",
		HTTP: &httpx.Client{
			Limiter: &httpx.Limiter{Rate: 20, Burst: 10},
			Header:  http.Header{"Authorization": {"Bearer " + token}, "Accept": {"application/json"}},
		},
	}
}

// Result is a movie as listed by search and find.
type Result struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	ReleaseDate   string `json:"release_date"`
	PosterPath    string `json:"poster_path"`
}

func (r Result) Year() int { return year(r.ReleaseDate) }

type Person struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Job       string `json:"job"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

// Details is a movie with its credits (append_to_response=credits).
type Details struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	OriginalTitle    string `json:"original_title"`
	ReleaseDate      string `json:"release_date"`
	Runtime          int    `json:"runtime"`
	OriginalLanguage string `json:"original_language"`
	Overview         string `json:"overview"`
	IMDbID           string `json:"imdb_id"`
	PosterPath       string `json:"poster_path"`
	BackdropPath     string `json:"backdrop_path"`
	Genres           []struct {
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCountries []struct {
		ISO string `json:"iso_3166_1"`
	} `json:"production_countries"`
	Collection *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"belongs_to_collection"`
	Credits struct {
		Cast []Person `json:"cast"`
		Crew []Person `json:"crew"`
	} `json:"credits"`
}

func (d Details) Year() int { return year(d.ReleaseDate) }

// Directors returns the crew members with job "Director", without repeats.
func (d Details) Directors() []Person {
	var out []Person
	seen := map[int]bool{}
	for _, p := range d.Credits.Crew {
		if p.Job == "Director" && !seen[p.ID] {
			seen[p.ID] = true
			out = append(out, p)
		}
	}
	return out
}

func year(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

// Translation is a movie's texts in one language and country
// (GET /movie/{id}/translations). Empty fields were not translated.
type Translation struct {
	Language string `json:"iso_639_1"`  // "es"
	Country  string `json:"iso_3166_1"` // "AR"
	Data     struct {
		Title    string `json:"title"`
		Overview string `json:"overview"`
	} `json:"data"`
}

// Tag is the translation's language as TMDB takes it ("es-AR").
func (t Translation) Tag() string { return t.Language + "-" + t.Country }

// Translations returns every translation of a movie's title and overview.
func (c *Client) Translations(ctx context.Context, id int) ([]Translation, error) {
	var out struct {
		Translations []Translation `json:"translations"`
	}
	err := c.get(ctx, "/movie/"+strconv.Itoa(id)+"/translations", &out)
	return out.Translations, err
}

// SearchMovie searches by title; year 0 means any year. Adult titles are
// included: the collection being catalogued is the user's own, not a public
// listing, so TMDB's default filter (which would hide a real owned title
// like "Deep Throat") does not apply here.
func (c *Client) SearchMovie(ctx context.Context, query string, year int, lang string) ([]Result, error) {
	v := url.Values{"query": {query}, "language": {lang}, "include_adult": {"true"}}
	if year > 0 {
		v.Set("year", strconv.Itoa(year))
	}
	var out struct {
		Results []Result `json:"results"`
	}
	err := c.get(ctx, "/search/movie?"+v.Encode(), &out)
	return out.Results, err
}

// FindIMDb looks a movie up by its IMDb id ("tt0071129").
func (c *Client) FindIMDb(ctx context.Context, imdbID, lang string) ([]Result, error) {
	v := url.Values{"external_source": {"imdb_id"}, "language": {lang}}
	var out struct {
		Results []Result `json:"movie_results"`
	}
	err := c.get(ctx, "/find/"+url.PathEscape(imdbID)+"?"+v.Encode(), &out)
	return out.Results, err
}

// Movie returns a movie's details and credits.
func (c *Client) Movie(ctx context.Context, id int, lang string) (Details, error) {
	v := url.Values{"language": {lang}, "append_to_response": {"credits"}}
	var d Details
	err := c.get(ctx, "/movie/"+strconv.Itoa(id)+"?"+v.Encode(), &d)
	return d, err
}

// Image downloads an image by its TMDB path ("/abc.jpg") at a size ("w342").
func (c *Client) Image(ctx context.Context, path, size string) ([]byte, error) {
	b, err := c.HTTP.Get(ctx, c.ImageURL+"/"+size+path)
	return b, mapErr(err)
}

func (c *Client) get(ctx context.Context, pathQuery string, out any) error {
	b, err := c.HTTP.Get(ctx, c.BaseURL+pathQuery)
	if err != nil {
		return mapErr(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("tmdb %s: %w", pathQuery, err)
	}
	return nil
}

func mapErr(err error) error {
	var se *httpx.StatusError
	if errors.As(err, &se) {
		switch se.Code {
		case http.StatusUnauthorized:
			return ErrUnauthorized
		case http.StatusNotFound:
			return ErrNotFound
		}
	}
	return err
}
