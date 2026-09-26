package identify

import (
	"context"
	"errors"
	"log"
	"path"
	"sync"
	"time"

	"cinexplorer/internal/httpx"
	"cinexplorer/internal/images"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// Runner states, as shown by the UI.
const (
	StateIdle     = "idle"
	StateRunning  = "running"
	StateOffline  = "offline"  // waiting to retry
	StateNoToken  = "noToken"  // no TMDB token configured
	StateBadToken = "badToken" // TMDB rejected the token
)

// Image prefetch modes (config.json "imagePrefetch").
const (
	PrefetchNone    = "none"
	PrefetchPosters = "posters"
	PrefetchAll     = "all"
)

var (
	ErrBusy    = errors.New("ya hay una identificación en curso")
	ErrNoToken = errors.New("sin token de TMDB")
)

// Retry delays while offline: the first, doubled up to the last.
const (
	firstRetry = 5 * time.Minute
	maxRetry   = time.Hour
)

// saveBatch is how many identifications are committed at once.
const saveBatch = 20

type Status struct {
	State      string `json:"state"`
	ToIdentify int    `json:"toIdentify"`
	Identified int    `json:"identified"`
	ToEnrich   int    `json:"toEnrich"`
	Enriched   int    `json:"enriched"`
}

// Wikidata is the part of the Wikidata client the runner uses.
type Wikidata interface {
	Lookup(ctx context.Context, ids []int, lang string) (map[int]wikidata.Entity, error)
}

type Runner struct {
	AppDir   string
	Store    *store.Store
	TMDB     API      // nil: no token, nothing to do
	Wikidata Wikidata // nil: skip the Wikidata phase
	Images   *images.Cache
	Language string
	Prefetch string // PrefetchNone, PrefetchPosters or PrefetchAll
	// After schedules f to run after d and returns a function that cancels
	// it; nil means time.AfterFunc. Tests replace it.
	After func(d time.Duration, f func()) (cancel func())

	mu      sync.Mutex
	status  Status
	running bool
	again   bool // a Trigger arrived during a run
	backoff time.Duration
	cancel  func() // pending retry
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.status
	switch {
	case r.TMDB == nil:
		s.State = StateNoToken // known before any run
	case s.State == "":
		s.State = StateIdle
	}
	return s
}

// Trigger starts a run in the background. A trigger during a run makes one
// more run follow it.
func (r *Runner) Trigger() {
	r.mu.Lock()
	if r.running {
		r.again = true
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()
	go func() {
		for {
			if err := r.run(context.Background()); err != nil {
				log.Printf("identificación: %v", err)
			}
			r.mu.Lock()
			if !r.again {
				r.running = false
				r.mu.Unlock()
				return
			}
			r.again = false
			r.mu.Unlock()
		}
	}()
}

// Run performs one run synchronously; ErrBusy if one is in progress.
func (r *Runner) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrBusy
	}
	r.running = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()
	return r.run(ctx)
}

func (r *Runner) run(ctx context.Context) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if r.TMDB == nil {
		r.status = Status{State: StateNoToken}
		r.mu.Unlock()
		return nil
	}
	r.status = Status{State: StateRunning}
	r.mu.Unlock()

	// Details fetched while matching, reused by enrichment in this run.
	fetched := map[int]tmdb.Details{}
	err := r.identifyAll(ctx, fetched)
	if err == nil {
		err = r.enrichAll(ctx, fetched)
	}
	if err == nil {
		err = r.wikidataAll(ctx)
	}
	if err == nil {
		err = r.prefetchAll(ctx)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case errors.Is(err, httpx.ErrOffline):
		r.status.State = StateOffline
		if r.backoff == 0 {
			r.backoff = firstRetry
		} else {
			r.backoff = min(2*r.backoff, maxRetry)
		}
		after := r.After
		if after == nil {
			after = func(d time.Duration, f func()) func() { t := time.AfterFunc(d, f); return func() { t.Stop() } }
		}
		r.cancel = after(r.backoff, r.Trigger)
		log.Printf("identificación: sin conexión, se reintenta en %v", r.backoff)
		return nil
	case errors.Is(err, tmdb.ErrUnauthorized):
		r.status.State = StateBadToken
		return err
	}
	r.status.State = StateIdle
	r.backoff = 0
	return err
}

// fatal errors stop a run: no network, bad token, or cancellation. Other
// errors only skip the item at hand.
func fatal(err error) bool {
	return errors.Is(err, httpx.ErrOffline) || errors.Is(err, tmdb.ErrUnauthorized) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

type job struct {
	fingerprint string
	query       Query
}

// needsMatch reports whether a fingerprint has to be (re)matched: never
// matched, or matched by the matcher for another query or an older version
// of it. Corrections are never redone.
func needsMatch(cur *store.Identification, q Query, lang string) bool {
	if cur == nil {
		return true
	}
	if cur.Status != store.StatusAuto && cur.Status != store.StatusUnmatched {
		return false
	}
	return cur.Query != q.storedKey(lang) || cur.MatcherVersion < MatcherVersion
}

func (r *Runner) identifyAll(ctx context.Context, fetched map[int]tmdb.Details) error {
	targets, err := r.Store.IdentifyTargets()
	if err != nil {
		return err
	}
	var jobs []job
	for _, t := range targets {
		q := Query{Title: t.Title, Year: t.Year, Director: t.Director, IMDbID: t.IMDbID}
		if id := imdbFromNFOs(r.AppDir, t.NFOs); id != "" {
			q.IMDbID = id
		}
		// A folder named like a movie ("Title (Director, 1974)") helps when
		// the file names do not: several versions in one folder keep their
		// own, often poorer, names.
		if f := nameparse.Parse(path.Base(t.Dir)); f.Year > 0 && (f.Title != q.Title || f.Year != q.Year) {
			q.Folder = &Query{Title: f.Title, Year: f.Year, Director: f.Director}
		}
		if needsMatch(t.Current, q, r.Language) {
			jobs = append(jobs, job{t.Fingerprint, q})
		}
	}
	r.mu.Lock()
	r.status.ToIdentify = len(jobs)
	r.mu.Unlock()

	var batch []store.Identification
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := r.Store.SaveIdentifications(batch)
		batch = batch[:0]
		return err
	}
	for _, j := range jobs {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, flush())
		}
		id, details, err := Identify(ctx, r.TMDB, r.Language, j.query)
		if fatal(err) {
			return errors.Join(err, flush())
		}
		if err != nil {
			// Left for review rather than retried on every run.
			log.Printf("no se pudo identificar %q: %v", j.query.Title, err)
			id = store.Identification{Status: store.StatusUnmatched, Query: j.query.storedKey(r.Language),
				MatcherVersion: MatcherVersion}
		}
		for k, d := range details {
			fetched[k] = d
		}
		id.Fingerprint = j.fingerprint
		batch = append(batch, id)
		r.mu.Lock()
		r.status.Identified++
		r.mu.Unlock()
		if len(batch) >= saveBatch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

// Adopt fetches a movie from TMDB and stores it; it is how a manual
// identification is validated. tmdb.ErrNotFound means the id does not exist.
func (r *Runner) Adopt(ctx context.Context, id int) error {
	if r.TMDB == nil {
		return ErrNoToken
	}
	m, err := r.fetchMovie(ctx, id, nil)
	if err != nil {
		return err
	}
	return r.Store.SaveMovie(m)
}

// fetchMovie gets a movie in the configured language, completing an empty
// title or overview in English.
func (r *Runner) fetchMovie(ctx context.Context, id int, fetched map[int]tmdb.Details) (store.Movie, error) {
	d, ok := fetched[id]
	if !ok {
		var err error
		if d, err = r.TMDB.Movie(ctx, id, r.Language); err != nil {
			return store.Movie{}, err
		}
	}
	m := movieFrom(d, r.Language)
	if r.Language != fallbackLang && (m.Title == "" || m.Overview == "") {
		en, err := r.TMDB.Movie(ctx, id, fallbackLang)
		if err != nil {
			return store.Movie{}, err
		}
		if m.Title == "" {
			m.Title = en.Title
		}
		if m.Overview == "" {
			m.Overview = en.Overview
		}
	}
	return m, nil
}

func (r *Runner) enrichAll(ctx context.Context, fetched map[int]tmdb.Details) error {
	ids, err := r.Store.MoviesToEnrich(r.Language)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.status.ToEnrich = len(ids)
	r.mu.Unlock()
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		m, err := r.fetchMovie(ctx, id, fetched)
		switch {
		case errors.Is(err, tmdb.ErrNotFound):
			log.Printf("la película %d ya no existe en TMDB", id)
			if err := r.Store.InvalidateMovie(id); err != nil {
				return err
			}
			continue
		case fatal(err):
			return err
		case err != nil:
			log.Printf("no se pudo obtener la película %d: %v", id, err)
			continue
		}
		if err := r.Store.SaveMovie(m); err != nil {
			return err
		}
		r.mu.Lock()
		r.status.Enriched++
		r.mu.Unlock()
	}
	return nil
}

func (r *Runner) wikidataAll(ctx context.Context) error {
	if r.Wikidata == nil {
		return nil
	}
	for {
		ms, err := r.Store.PendingWikidata(wikidata.BatchSize)
		if err != nil || len(ms) == 0 {
			return err
		}
		ids := make([]int, len(ms))
		for i, m := range ms {
			ids[i] = m.TMDBID
		}
		ents, err := r.Wikidata.Lookup(ctx, ids, r.Language)
		if fatal(err) {
			return err
		}
		if err != nil {
			// Not worth retrying in a loop: the next run tries again.
			log.Printf("wikidata: %v", err)
			return nil
		}
		for _, m := range ms {
			fillFromWikidata(&m, ents[m.TMDBID])
			if err := r.Store.SaveMovie(m); err != nil {
				return err
			}
		}
	}
}

type imageRef struct {
	kind images.Kind
	path string
}

func (r *Runner) prefetchAll(ctx context.Context) error {
	if r.Images == nil || (r.Prefetch != PrefetchPosters && r.Prefetch != PrefetchAll) {
		return nil
	}
	ms, err := r.Store.Movies()
	if err != nil {
		return err
	}
	for _, m := range ms {
		wanted := []imageRef{{images.Poster, m.PosterPath}}
		if r.Prefetch == PrefetchAll {
			wanted = append(wanted, imageRef{images.Backdrop, m.BackdropPath})
		}
		for _, w := range wanted {
			if w.path == "" || r.Images.Has(w.kind, m.TMDBID, w.path) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if _, err := r.Images.Get(ctx, w.kind, m.TMDBID, w.path); fatal(err) {
				return err
			} else if err != nil {
				log.Printf("imagen %s de %d: %v", w.kind, m.TMDBID, err)
			}
		}
	}
	return nil
}
