package identify

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// fakeAPI answers from maps. Search keys are "title|year|lang".
type fakeAPI struct {
	mu     sync.Mutex
	search map[string][]tmdb.Result
	find   map[string][]tmdb.Result
	movies map[string]tmdb.Details // "id|lang"
	err    error                   // returned by every call when set
	calls  []string
}

func (f *fakeAPI) record(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	return f.err
}

func (f *fakeAPI) SearchMovie(ctx context.Context, q string, year int, lang string) ([]tmdb.Result, error) {
	key := fmt.Sprintf("%s|%d|%s", q, year, lang)
	if err := f.record("search " + key); err != nil {
		return nil, err
	}
	return f.search[key], nil
}

func (f *fakeAPI) FindIMDb(ctx context.Context, id, lang string) ([]tmdb.Result, error) {
	if err := f.record("find " + id); err != nil {
		return nil, err
	}
	return f.find[id], nil
}

func (f *fakeAPI) Movie(ctx context.Context, id int, lang string) (tmdb.Details, error) {
	key := strconv.Itoa(id) + "|" + lang
	if err := f.record("movie " + key); err != nil {
		return tmdb.Details{}, err
	}
	d, ok := f.movies[key]
	if !ok {
		return d, tmdb.ErrNotFound
	}
	return d, nil
}

func (f *fakeAPI) called(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if len(c) >= len(prefix) && c[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}

type fakeWikidata struct {
	ents  map[int]wikidata.Entity
	err   error
	calls [][]int
}

func (f *fakeWikidata) Lookup(ctx context.Context, ids []int, lang string) (map[int]wikidata.Entity, error) {
	f.calls = append(f.calls, ids)
	return f.ents, f.err
}

func details(id int, title, date, overview string, directors ...string) tmdb.Details {
	d := tmdb.Details{ID: id, Title: title, OriginalTitle: title, ReleaseDate: date, Overview: overview}
	for i, n := range directors {
		d.Credits.Crew = append(d.Credits.Crew, tmdb.Person{ID: 100 + i, Name: n, Job: "Director"})
	}
	return d
}
