package identify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/httpx"
	"cinexplorer/internal/images"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// fixture builds a catalog with an Amarcord version and a Stalker DVD whose
// folder has a .nfo with the IMDb id, and a runner on a fake TMDB.
func fixture(t *testing.T) (*Runner, *fakeAPI, *fakeWikidata) {
	t.Helper()
	disk := t.TempDir()
	appDir := filepath.Join(disk, "cinexplorer")
	nfo := filepath.Join(disk, "cine", "d", "movie.nfo")
	if err := os.MkdirAll(filepath.Dir(nfo), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nfo, []byte("Stalker\nhttps://www.imdb.com/title/tt0079944/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(appDir, "cinexplorer.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.SyncFiles([]store.FileRow{
		{Path: "../cine/a/Amarcord.avi", Size: 700, MTime: 1, Fingerprint: "a1", Kind: "video"},
		{Path: "../cine/d/VIDEO_TS/VTS_01_1.VOB", Size: 900, MTime: 1, Fingerprint: "vob", Kind: "dvd"},
		{Path: "../cine/d/movie.nfo", Size: 50, MTime: 1, Kind: "info"},
	}, []string{"../cine"})
	setVersions(t, st, "Amarcord")

	api := &fakeAPI{
		search: map[string][]tmdb.Result{
			"Amarcord|1973|es-ES": {{ID: 7857, Title: "Amarcord", OriginalTitle: "Amarcord", ReleaseDate: "1973-12-18"}},
		},
		find: map[string][]tmdb.Result{"tt0079944": {{ID: 1398, Title: "Stalker", ReleaseDate: "1979-05-25"}}},
		movies: map[string]tmdb.Details{
			"7857|es-ES": details(7857, "Amarcord", "1973-12-18", "Rimini.", "Federico Fellini"),
			"1398|es-ES": details(1398, "Stalker", "1979-05-25", "", "Andrei Tarkovsky"),
		},
		trans: map[int][]tmdb.Translation{1398: {tr("en-US", "Stalker", "The Zone.")}},
	}
	wd := &fakeWikidata{ents: map[int]wikidata.Entity{
		1398: {QID: "Q498906", IMDbID: "tt0079944", Countries: []string{"SU"}, Year: 1979},
	}}
	r := &Runner{AppDir: appDir, Store: st, TMDB: api, Wikidata: wd, Language: "es-ES",
		After: func(time.Duration, func()) func() { return func() {} }}
	return r, api, wd
}

// setVersions (re)builds the two versions, the first with the given title.
func setVersions(t *testing.T, st *store.Store, title string) {
	t.Helper()
	if err := st.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/a", Parsed: nameparse.Parsed{Title: title, Year: 1973}, Size: 700, Parts: 1,
			Members: []grouping.Member{{Path: "../cine/a/Amarcord.avi", Role: grouping.RoleMain}}},
		{Dir: "../cine/d", Parsed: nameparse.Parsed{Title: "d"}, Size: 900, Parts: 1,
			Members: []grouping.Member{{Path: "../cine/d/VIDEO_TS/VTS_01_1.VOB", Role: grouping.RoleMain}}},
	}); err != nil {
		t.Fatal(err)
	}
}

func current(t *testing.T, st *store.Store, fp string) *store.Identification {
	t.Helper()
	ts, err := st.IdentifyTargets()
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range ts {
		if x.Fingerprint == fp {
			return x.Current
		}
	}
	t.Fatalf("no target %s", fp)
	return nil
}

func TestRunIdentifiesAndEnriches(t *testing.T) {
	r, api, wd := fixture(t)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	a, d := current(t, r.Store, "a1"), current(t, r.Store, "vob")
	if a.Status != store.StatusAuto || a.TMDBID != 7857 || a.Query != "Amarcord|1973|||es-ES" || a.MatcherVersion != MatcherVersion {
		t.Fatalf("amarcord %+v", a)
	}
	if d.Status != store.StatusAuto || d.TMDBID != 1398 || d.Confidence != 1 || d.Query != "d|0||tt0079944|es-ES" {
		t.Fatalf("stalker %+v", d)
	}
	m, ok, _ := r.Store.Movie(7857)
	if !ok || m.Overview != "Rimini." || m.Directors[0].Name != "Federico Fellini" || !m.WikidataDone || m.WikidataID != "" {
		t.Fatalf("amarcord movie %+v", m)
	}
	s, _, _ := r.Store.Movie(1398)
	if s.Overview != "The Zone." || s.Language != "es-ES" || s.WikidataID != "Q498906" || !slices.Equal(s.Countries, []string{"SU"}) {
		t.Fatalf("stalker movie %+v", s)
	}
	// Amarcord's details came with the match; Stalker's were fetched in
	// es-ES. Each needed its translations (the overview came in English).
	if api.called("movie 1398|") != 1 || api.called("movie 7857|") != 1 || api.called("translations ") != 2 {
		t.Fatalf("calls %v", api.calls)
	}
	if len(wd.calls) != 1 || len(wd.calls[0]) != 2 {
		t.Fatalf("wikidata calls %v", wd.calls)
	}
	st := r.Status()
	if st.State != StateIdle || st.ToIdentify != 2 || st.Identified != 2 || st.ToEnrich != 2 || st.Enriched != 2 {
		t.Fatalf("status %+v", st)
	}

	// Nothing changed: a second run makes no TMDB calls.
	before := len(api.calls)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(api.calls) != before {
		t.Fatalf("second run called %v", api.calls[before:])
	}
}

func TestRunReusesMatchDetails(t *testing.T) {
	r, api, _ := fixture(t)
	// With a parsed director the matcher fetches credits; enrichment reuses them.
	api.search["Amarcord|1973|es-ES"] = []tmdb.Result{{ID: 7857, Title: "Amarcord", ReleaseDate: "1973-12-18"}}
	r.Store.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/a", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973, Director: "Fellini"}, Size: 700, Parts: 1,
			Members: []grouping.Member{{Path: "../cine/a/Amarcord.avi", Role: grouping.RoleMain}}},
	})
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a := current(t, r.Store, "a1"); a.Status != store.StatusAuto || a.Confidence != 1 {
		t.Fatalf("amarcord %+v", a)
	}
	if n := api.called("movie 7857|es-ES"); n != 1 {
		t.Fatalf("fetched %d times", n)
	}
}

func TestRunKeepsCorrectionsAndRematchesStale(t *testing.T) {
	r, api, _ := fixture(t)
	ctx := context.Background()
	r.Run(ctx)
	if err := r.Store.SetCorrection("a1", store.StatusIgnored, 0); err != nil {
		t.Fatal(err)
	}
	// Even a changed name does not redo a correction.
	setVersions(t, r.Store, "Amarcord!")
	api.calls = nil
	r.Run(ctx)
	if api.called("search") != 0 || current(t, r.Store, "a1").Status != store.StatusIgnored {
		t.Fatalf("correction redone: %v", api.calls)
	}

	// A changed name redoes an automatic match.
	r.Store.ResetIdentification("a1")
	r.Run(ctx)
	setVersions(t, r.Store, "Amarcord")
	api.calls = nil
	r.Run(ctx)
	if api.called("search Amarcord|1973") != 1 || current(t, r.Store, "a1").Query != "Amarcord|1973|||es-ES" {
		t.Fatalf("stale query not redone: %v", api.calls)
	}

	// So does a result from an older matcher.
	old := *current(t, r.Store, "a1")
	old.MatcherVersion = MatcherVersion - 1
	r.Store.SaveIdentifications([]store.Identification{old})
	api.calls = nil
	r.Run(ctx)
	if api.called("search") != 1 {
		t.Fatalf("old matcher result not redone: %v", api.calls)
	}
}

func TestRunRematchesOnLanguageChange(t *testing.T) {
	r, api, _ := fixture(t)
	ctx := context.Background()
	r.Store.SaveIdentifications([]store.Identification{{Fingerprint: "a1", Status: store.StatusUnmatched,
		Query: "Amarcord|1973|||es-ES", MatcherVersion: MatcherVersion}})
	r.Run(ctx)
	if api.called("search Amarcord") != 0 {
		t.Fatalf("up to date result redone: %v", api.calls)
	}
	// Candidates carry titles in the configured language: another language
	// means another search.
	r.Language = "en-US"
	api.search["Amarcord|1973|en-US"] = api.search["Amarcord|1973|es-ES"]
	api.movies["7857|en-US"] = details(7857, "Amarcord", "1973-12-18", "Rimini.", "Federico Fellini")
	r.Run(ctx)
	if a := current(t, r.Store, "a1"); api.called("search Amarcord|1973|en-US") != 1 || a.Status != store.StatusAuto || a.Query != "Amarcord|1973|||en-US" {
		t.Fatalf("language change: %+v calls %v", a, api.calls)
	}
}

func TestRunSavesFailedItemAsUnmatched(t *testing.T) {
	r, api, _ := fixture(t)
	ctx := context.Background()
	api.fail = map[string]error{"Amarcord|1973|es-ES": errors.New("respuesta inesperada")}
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	a := current(t, r.Store, "a1")
	if a == nil || a.Status != store.StatusUnmatched || len(a.Candidates) != 0 || a.Query != "Amarcord|1973|||es-ES" || a.MatcherVersion != MatcherVersion {
		t.Fatalf("failed item %+v", a)
	}
	// It shows up for review instead of being retried on every run.
	api.calls = nil
	r.Run(ctx)
	if api.called("search") != 0 {
		t.Fatalf("failed item retried: %v", api.calls)
	}
}

func TestRunOfflineSchedulesRetries(t *testing.T) {
	r, api, _ := fixture(t)
	var delays []time.Duration
	cancels := 0
	r.After = func(d time.Duration, f func()) func() {
		delays = append(delays, d)
		return func() { cancels++ }
	}
	api.err = fmt.Errorf("%w: dial tcp", httpx.ErrOffline)
	ctx := context.Background()
	for range 6 {
		if err := r.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	want := []time.Duration{5 * time.Minute, 10 * time.Minute, 20 * time.Minute, 40 * time.Minute, time.Hour, time.Hour}
	if !slices.Equal(delays, want) || r.Status().State != StateOffline {
		t.Fatalf("delays %v state %s", delays, r.Status().State)
	}
	if cancels != 5 {
		t.Fatalf("each run should cancel the pending retry: %d", cancels)
	}
	if current(t, r.Store, "a1") != nil {
		t.Fatal("offline run stored a result")
	}

	api.err = nil
	r.Run(ctx)
	if r.Status().State != StateIdle || current(t, r.Store, "a1") == nil {
		t.Fatalf("after reconnect: %+v", r.Status())
	}
	api.err = fmt.Errorf("%w: again", httpx.ErrOffline)
	r.Store.ResetIdentification("a1")
	r.Run(ctx)
	if delays[len(delays)-1] != 5*time.Minute {
		t.Fatalf("backoff not reset: %v", delays)
	}
}

func TestRunBadTokenAndNoToken(t *testing.T) {
	r, api, _ := fixture(t)
	scheduled := false
	r.After = func(time.Duration, func()) func() { scheduled = true; return func() {} }
	api.err = tmdb.ErrUnauthorized
	if err := r.Run(context.Background()); !errors.Is(err, tmdb.ErrUnauthorized) {
		t.Fatalf("err %v", err)
	}
	if r.Status().State != StateBadToken || scheduled {
		t.Fatalf("state %s scheduled %v", r.Status().State, scheduled)
	}
	r.TMDB = nil
	if st := r.Status().State; st != StateNoToken {
		t.Fatalf("no token before running: %s", st)
	}
	if err := r.Run(context.Background()); err != nil || r.Status().State != StateNoToken {
		t.Fatalf("no token: %v %s", err, r.Status().State)
	}
	if err := r.Adopt(context.Background(), 7857); !errors.Is(err, ErrNoToken) {
		t.Fatalf("adopt without token: %v", err)
	}
}

func TestEnrichInvalidatesDeletedMovie(t *testing.T) {
	r, api, _ := fixture(t)
	api.search["Amarcord|1973|es-ES"] = []tmdb.Result{{ID: 999, Title: "Amarcord", ReleaseDate: "1973-01-01"}}
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	// 999 matched, but TMDB says it does not exist: the match is forgotten.
	if a := current(t, r.Store, "a1"); a != nil {
		t.Fatalf("amarcord %+v", a)
	}
}

func TestWikidataErrorSkipsPhase(t *testing.T) {
	r, _, wd := fixture(t)
	wd.err = errors.New("wikidata: bad JSON")
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m, _, _ := r.Store.Movie(7857); m.WikidataDone || len(wd.calls) != 1 {
		t.Fatalf("movie %+v calls %v", m, wd.calls)
	}
	wd.err = fmt.Errorf("%w: timeout", httpx.ErrOffline)
	r.Run(context.Background())
	if r.Status().State != StateOffline {
		t.Fatalf("state %s", r.Status().State)
	}
}

type fetcher struct{ paths []string }

func (f *fetcher) Image(ctx context.Context, path, size string) ([]byte, error) {
	f.paths = append(f.paths, size+path)
	return []byte("x"), nil
}

func TestPrefetch(t *testing.T) {
	r, api, _ := fixture(t)
	d := api.movies["7857|es-ES"]
	d.PosterPath, d.BackdropPath = "/p.jpg", "/b.jpg"
	api.movies["7857|es-ES"] = d
	f := &fetcher{}
	r.Images = &images.Cache{Dir: t.TempDir(), Fetch: f}
	r.Prefetch = PrefetchPosters
	r.Run(context.Background())
	if !slices.Equal(f.paths, []string{"w342/p.jpg"}) {
		t.Fatalf("posters: %v", f.paths)
	}
	r.Prefetch = PrefetchAll
	r.Run(context.Background())
	if !slices.Equal(f.paths, []string{"w342/p.jpg", "w1280/b.jpg"}) {
		t.Fatalf("all: %v", f.paths)
	}
}

func TestAdopt(t *testing.T) {
	r, _, _ := fixture(t)
	ctx := context.Background()
	if err := r.Adopt(ctx, 7857); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.Store.Movie(7857); !ok {
		t.Fatal("not stored")
	}
	if err := r.Adopt(ctx, 5); !errors.Is(err, tmdb.ErrNotFound) {
		t.Fatalf("unknown id: %v", err)
	}
}

func TestTriggerRunsInBackground(t *testing.T) {
	r, _, _ := fixture(t)
	r.Trigger()
	r.Trigger() // coalesced into one follow-up run
	deadline := time.Now().Add(5 * time.Second)
	for current(t, r.Store, "a1") == nil || r.busy() {
		if time.Now().After(deadline) {
			t.Fatal("background run did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("runner still busy: %v", err)
	}
}

func (r *Runner) busy() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func TestRunFallsBackToFolderName(t *testing.T) {
	r, api, _ := fixture(t)
	dir := "../cine/Los Gauchos Judíos (Juan José Jusid, Argentina, 1974)"
	r.Store.SyncFiles([]store.FileRow{{Path: dir + "/cd 01.avi", Size: 700, MTime: 1, Fingerprint: "g1", Kind: "video"}}, []string{"../cine"})
	r.Store.ReplaceVersions([]grouping.Version{{Dir: dir, Parsed: nameparse.Parsed{Title: "Los Gauchos Judios Rip mentecato"},
		Size: 700, Parts: 1, Members: []grouping.Member{{Path: dir + "/cd 01.avi", Role: grouping.RoleMain}}}})
	api.search["Los Gauchos Judios|1974|es-ES"] = []tmdb.Result{{ID: 537898, Title: "Los gauchos judíos", ReleaseDate: "1975-05-01"}}
	api.movies["537898|es-ES"] = details(537898, "Los gauchos judíos", "1975-05-01", "Entre Ríos.", "Juan José Jusid")
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	g := current(t, r.Store, "g1")
	if g.Status != store.StatusAuto || g.TMDBID != 537898 ||
		g.Query != "Los Gauchos Judios Rip mentecato|0||"+"|Los Gauchos Judíos|1974|Juan José Jusid||es-ES" {
		t.Fatalf("gauchos %+v", g)
	}
}

// blockingAPI waits in every search until its context ends.
type blockingAPI struct {
	*fakeAPI
	started chan struct{}
}

func (b *blockingAPI) SearchMovie(ctx context.Context, q string, year int, lang string) ([]tmdb.Result, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestCloseStopsBackgroundRuns(t *testing.T) {
	r, api, _ := fixture(t)
	b := &blockingAPI{fakeAPI: api, started: make(chan struct{}, 1)}
	r.TMDB = b
	r.Trigger()
	select {
	case <-b.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the run did not start")
	}
	r.Trigger() // a follow-up that Close must drop
	r.Close()
	if r.busy() {
		t.Fatal("still running after Close")
	}
	if st := r.Status().State; st != StateIdle {
		t.Fatalf("state %s", st)
	}
	r.TMDB = api
	r.Trigger()
	time.Sleep(50 * time.Millisecond)
	if r.busy() || current(t, r.Store, "a1") != nil {
		t.Fatal("Trigger ran after Close")
	}
}
