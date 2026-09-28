// Package engine builds, from config.json, the parts of the app that depend
// on it (TMDB client, image cache, scanner, identification runner) and
// replaces them all at once when the configuration changes.
package engine

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"

	"cinexplorer/internal/config"
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// ErrReadOnly means the app directory cannot be written: the configuration
// cannot change.
var ErrReadOnly = errors.New("modo consulta: la configuración no se puede modificar")

// Engine holds the current Runtime. Its fields are what does not depend on
// config.json; set them before Start.
type Engine struct {
	AppDir   string
	Store    *store.Store
	ReadOnly bool
	Wikidata identify.Wikidata // nil: skip the Wikidata phase

	mu      sync.Mutex // serializes Apply
	current atomic.Pointer[Runtime]
	pending atomic.Bool
}

// Runtime is everything built from one configuration. Pages read it once
// per request; it does not change after it is built. A change of display
// settings only replaces it with a copy that keeps the same parts running.
type Runtime struct {
	Config     config.Config
	TMDB       identify.API     // nil without a token
	Images     *images.Cache    // its Fetch is the TMDB client of this runtime
	Scanner    *scan.Scanner    // nil in read-only mode
	Identifier *identify.Runner // nil in read-only mode

	*lifecycle // shared by those copies
}

type lifecycle struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	stopped bool
	scans   sync.WaitGroup
	// owed are the roots whose scan was asked for and has not finished
	// yet; the next runtime scans them again.
	owed []string
}

// Start builds the first runtime. setupPending means config.json does not
// exist yet: nothing is scanned until Apply saves one.
func (e *Engine) Start(cfg config.Config, setupPending bool) {
	rt := e.build(cfg)
	e.current.Store(rt)
	e.pending.Store(setupPending)
	if !setupPending {
		rt.Scan()
	}
}

// Current is the runtime in use.
func (e *Engine) Current() *Runtime { return e.current.Load() }

// Use replaces the runtime as it is, without stopping the previous one or
// scanning (tests build their own).
func (e *Engine) Use(rt *Runtime) {
	if rt.lifecycle == nil {
		rt.lifecycle = &lifecycle{}
	}
	e.current.Store(rt)
}

// SetupPending reports whether the first-use assistant has to run.
func (e *Engine) SetupPending() bool { return e.pending.Load() }

// Apply saves cfg as config.json and replaces the runtime: it stops the
// current one (cancelling its scan and identification), starts one built
// from cfg and scans what the change needs (see Update). cfg must be valid.
func (e *Engine) Apply(cfg config.Config) error {
	return e.Update(func(config.Config) (config.Config, error) { return cfg, nil })
}

// ValidationError marks a rejected configuration (not an I/O failure): Update
// callers use it to tell "the request was invalid" from "saving failed".
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// Update resolves the next configuration against the current one under the
// same lock Apply uses, so a "keep the saved token/language" decision can
// never race a concurrent PUT: f receives the configuration in effect right
// now and returns what to save, or a *ValidationError if it rejects it. It
// then saves and replaces the runtime. What runs next depends on what
// changed:
//   - only display settings: the running parts are kept as they are;
//   - the roots: only the added ones are scanned (and what the removed ones
//     leave is marked missing), plus any whose scan the stopped runtime
//     had not finished;
//   - anything else (token, language, images): no scan, the identification
//     runs again with the new settings and redoes what they make stale.
//
// On first use (the setup was pending) every root is scanned.
func (e *Engine) Update(f func(cur config.Config) (config.Config, error)) error {
	if e.ReadOnly {
		return ErrReadOnly
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	var cur config.Config
	if old := e.current.Load(); old != nil {
		cur = old.Config
	}
	cfg, err := f(cur)
	if err != nil {
		return err
	}
	if err := config.Save(e.AppDir, cfg); err != nil {
		return err
	}
	old := e.current.Load()
	if old != nil && !e.pending.Load() && displayOnly(old.Config, cfg) {
		rt := *old
		rt.Config = cfg
		e.current.Store(&rt)
		return nil
	}
	if old == nil || e.pending.Load() {
		if old != nil {
			old.Stop()
		}
		rt := e.build(cfg)
		e.current.Store(rt)
		e.pending.Store(false)
		rt.Scan()
		return nil
	}
	old.Stop()
	rt := e.build(cfg)
	e.current.Store(rt)
	owed := old.owedRoots() // read after Stop: its scan has ended
	var roots []string
	for _, r := range cfg.Roots {
		if !slices.Contains(old.Config.Roots, r) || slices.Contains(owed, r) {
			roots = append(roots, r)
		}
	}
	if len(roots) > 0 || !reflect.DeepEqual(old.Config.Roots, cfg.Roots) {
		rt.scan(roots)
	} else {
		rt.Identifier.Trigger()
	}
	return nil
}

// displayOnly reports whether a and b differ at most in what only the pages
// use (the card size), which no part of the runtime depends on.
func displayOnly(a, b config.Config) bool {
	a.TileSize, b.TileSize = "", ""
	return reflect.DeepEqual(a, b)
}

func (e *Engine) build(cfg config.Config) *Runtime {
	rt := &Runtime{Config: cfg, Images: &images.Cache{Dir: filepath.Join(e.AppDir, "cache"), ReadOnly: e.ReadOnly},
		lifecycle: &lifecycle{}}
	var api *tmdb.Client
	if cfg.TMDBToken != "" {
		api = tmdb.New(cfg.TMDBToken)
		rt.TMDB, rt.Images.Fetch = api, api
	} else {
		log.Print("sin token de TMDB: no se identifican películas")
	}
	if e.ReadOnly {
		return rt
	}
	runner := &identify.Runner{AppDir: e.AppDir, Store: e.Store, Wikidata: e.Wikidata, Images: rt.Images,
		Language: cfg.Language, Prefetch: cfg.ImagePrefetch}
	if api != nil {
		runner.TMDB = api // only when set: a nil *tmdb.Client in the interface would not read as "no token"
	}
	rt.Identifier = runner
	rt.Scanner = &scan.Scanner{AppDir: e.AppDir, Roots: cfg.Roots, Store: e.Store, OnDone: runner.Trigger}
	return rt
}

// Scan starts a scan of every root in the background; nothing in read-only
// mode, after Stop, or while one is running.
func (rt *Runtime) Scan() {
	rt.scan(rt.Config.Roots)
}

// scan starts a scan of some roots in the background, like Scan. The roots
// stay owed until a scan of them ends without an error.
func (rt *Runtime) scan(roots []string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.Scanner == nil || rt.stopped {
		return
	}
	if rt.ctx == nil {
		rt.ctx, rt.cancel = context.WithCancel(context.Background())
	}
	for _, r := range roots {
		if !slices.Contains(rt.owed, r) {
			rt.owed = append(rt.owed, r)
		}
	}
	ctx := rt.ctx
	rt.scans.Add(1)
	go func() {
		defer rt.scans.Done()
		err := rt.Scanner.RunRoots(ctx, roots)
		if err == nil {
			rt.mu.Lock()
			rt.owed = slices.DeleteFunc(rt.owed, func(r string) bool { return slices.Contains(roots, r) })
			rt.mu.Unlock()
		} else if !errors.Is(err, scan.ErrBusy) && ctx.Err() == nil {
			log.Printf("escaneo: %v", err)
		}
	}()
}

// owedRoots are the roots whose scan has not finished.
func (rt *Runtime) owedRoots() []string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return slices.Clone(rt.owed)
}

// Stop cancels the runtime's scan and identification and waits for them to
// end. What they saved stays.
func (rt *Runtime) Stop() {
	rt.mu.Lock()
	rt.stopped = true
	if rt.cancel != nil {
		rt.cancel()
	}
	rt.mu.Unlock()
	if rt.Identifier != nil {
		rt.Identifier.Close()
	}
	rt.scans.Wait()
}
