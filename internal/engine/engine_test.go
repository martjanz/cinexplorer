package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"cinexplorer/internal/config"
	"cinexplorer/internal/probe"
	"cinexplorer/internal/store"
)

var errTest = errors.New("prueba")

// newEngine makes <tmp>/cinexplorer next to cine/ and otro/, each with a
// movie, and vacio/; and an engine over a fresh catalog.
func newEngine(t *testing.T, readOnly bool) *Engine {
	t.Helper()
	disk := t.TempDir()
	appDir := filepath.Join(disk, "cinexplorer")
	for _, f := range []string{"cine/Amarcord.1973.mkv", "otro/Stalker.1979.mkv"} {
		p := filepath.Join(disk, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{appDir, filepath.Join(disk, "vacio")} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	e := &Engine{AppDir: appDir, Store: st, ReadOnly: readOnly}
	t.Cleanup(func() {
		if rt := e.Current(); rt != nil {
			rt.Stop()
		}
	})
	return e
}

// waitScan waits for the current runtime's scan to finish.
func waitScan(t *testing.T, e *Engine) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for st := e.Current().Scanner.Status(); st.Running || st.Finished.IsZero(); st = e.Current().Scanner.Status() {
		if time.Now().After(deadline) {
			t.Fatalf("scan did not finish: %+v", st)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func titles(t *testing.T, e *Engine) []string {
	t.Helper()
	vs, err := e.Store.Versions()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, v := range vs {
		if !v.Files[0].Missing {
			out = append(out, v.Title)
		}
	}
	return out
}

func TestStartScans(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}, Language: "es-AR"}, false)
	rt := e.Current()
	if e.SetupPending() || rt.TMDB != nil || rt.Images.Fetch != nil || rt.Identifier == nil || rt.Identifier.Language != "es-AR" {
		t.Fatalf("runtime %+v", rt)
	}
	waitScan(t, e)
	if got := titles(t, e); !reflect.DeepEqual(got, []string{"Amarcord"}) {
		t.Fatalf("titles %v", got)
	}
}

func TestStartPendingDoesNotScan(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}}, true)
	time.Sleep(50 * time.Millisecond)
	if !e.SetupPending() || !e.Current().Scanner.Status().Finished.IsZero() {
		t.Fatal("scanned while the setup was pending")
	}
	if _, err := os.Stat(filepath.Join(e.AppDir, config.FileName)); err == nil {
		t.Fatal("config.json written")
	}
}

func TestApplyReplacesRuntime(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}, Language: "es-AR"}, true)
	old := e.Current()
	if err := e.Apply(config.Config{Roots: []string{"../otro"}, Language: "es-AR"}); err != nil {
		t.Fatal(err)
	}
	if e.SetupPending() || e.Current() == old {
		t.Fatal("runtime not replaced")
	}
	waitScan(t, e)
	if got := titles(t, e); !reflect.DeepEqual(got, []string{"Stalker"}) {
		t.Fatalf("titles %v", got)
	}
	old.Scan()
	if old.Scanner.Status().Running {
		t.Fatal("a stopped runtime scanned")
	}

	// With a token, over an empty root: nothing to identify, so no network.
	cfg := config.Config{Roots: []string{"../vacio"}, TMDBToken: "tok", Language: "en-US", ImagePrefetch: "posters"}
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	rt := e.Current()
	if rt.TMDB == nil || rt.Images.Fetch == nil || rt.Identifier.TMDB == nil || rt.Identifier.Language != "en-US" ||
		rt.Identifier.Prefetch != "posters" || !reflect.DeepEqual(rt.Scanner.Roots, cfg.Roots) {
		t.Fatalf("runtime %+v", rt)
	}
	saved, created, err := config.Load(e.AppDir)
	if err != nil || created || !reflect.DeepEqual(saved, cfg) {
		t.Fatalf("saved %+v created=%v err=%v", saved, created, err)
	}
	waitScan(t, e)
	if got := titles(t, e); len(got) != 0 {
		t.Fatalf("titles %v", got)
	}
}

// TestUpdateDisplayOnlyKeepsRuntime checks that changing only the card size
// saves config.json but neither stops the runtime nor scans again.
func TestUpdateDisplayOnlyKeepsRuntime(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}, Language: "es-AR"}, false)
	waitScan(t, e)
	old := e.Current()
	finished := old.Scanner.Status().Finished
	cfg := config.Config{Roots: []string{"../cine"}, Language: "es-AR", TileSize: "large"}
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond) // time for a scan it should not start
	rt := e.Current()
	if st := rt.Scanner.Status(); rt.Config.TileSize != "large" || rt.Scanner != old.Scanner || rt.Identifier != old.Identifier ||
		st.Running || !st.Finished.Equal(finished) {
		t.Fatalf("runtime %+v", rt)
	}
	if saved, _, _ := config.Load(e.AppDir); !reflect.DeepEqual(saved, cfg) {
		t.Fatalf("saved %+v", saved)
	}
	// Still running: it scans when asked, and a real change stops it.
	rt.Scan()
	waitScan(t, e)
	if err := e.Apply(config.Config{Roots: []string{"../otro"}, Language: "es-AR", TileSize: "large"}); err != nil {
		t.Fatal(err)
	}
	if e.Current().Scanner == old.Scanner {
		t.Fatal("runtime not replaced")
	}
	old.Scan()
	if old.Scanner.Status().Running {
		t.Fatal("a stopped runtime scanned")
	}
}

// TestUpdateSeesCurrentConfig checks that Update hands f the configuration
// in effect right now, under the same lock Apply uses, so resolving "keep
// the saved token" against it can't race a concurrent Apply/Update.
func TestUpdateSeesCurrentConfig(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}, TMDBToken: "tok1", Language: "es-AR"}, true)

	var saw config.Config
	err := e.Update(func(cur config.Config) (config.Config, error) {
		saw = cur
		cur.Language = "en-US" // keeps the token, like putConfig's token:null
		return cur, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if saw.TMDBToken != "tok1" || saw.Language != "es-AR" {
		t.Fatalf("f saw %+v, want the config in effect before this Update", saw)
	}
	if got := e.Current().Config; got.TMDBToken != "tok1" || got.Language != "en-US" {
		t.Fatalf("current %+v", got)
	}
}

// TestUpdateValidationErrorLeavesRuntime checks that a rejecting f neither
// saves config.json nor replaces the runtime.
func TestUpdateValidationErrorLeavesRuntime(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}, Language: "es-AR"}, true)
	old := e.Current()

	wantErr := &ValidationError{Err: errTest}
	err := e.Update(func(config.Config) (config.Config, error) { return config.Config{}, wantErr })
	if err != wantErr {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if e.Current() != old {
		t.Fatal("runtime replaced by a rejected Update")
	}
	if _, err := os.Stat(filepath.Join(e.AppDir, config.FileName)); err == nil {
		t.Fatal("config.json written by a rejected Update")
	}
}

func TestApplyReadOnly(t *testing.T) {
	e := newEngine(t, true)
	e.Start(config.Config{Roots: []string{"../cine"}}, false)
	if e.Current().Scanner != nil || e.Current().Identifier != nil {
		t.Fatal("read-only runtime with a scanner")
	}
	if err := e.Apply(config.Config{Roots: []string{"../cine"}}); err != ErrReadOnly {
		t.Fatalf("err %v", err)
	}
}

func TestStopCancelsScan(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}}, true)
	rt := e.Current()
	started := make(chan struct{})
	rt.Scanner.Probe = func(ctx context.Context, path string) (probe.Info, error) {
		close(started)
		<-ctx.Done()
		return probe.Info{}, ctx.Err()
	}
	rt.Scan()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the scan did not reach the probe")
	}
	rt.Stop()
	if rt.Scanner.Status().Running {
		t.Fatal("still scanning after Stop")
	}
}

// TestUpdateWithoutRootChangesDoesNotScan checks that a change of language,
// token or images runs the identification again but scans nothing.
func TestUpdateWithoutRootChangesDoesNotScan(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}, Language: "es-AR"}, false)
	waitScan(t, e)
	for _, cfg := range []config.Config{
		{Roots: []string{"../cine"}, Language: "en-US"},
		{Roots: []string{"../cine"}, Language: "en-US", ImagePrefetch: "all"},
	} {
		if err := e.Apply(cfg); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond) // time for a scan it should not start
		rt := e.Current()
		if st := rt.Scanner.Status(); st.Running || !st.Finished.IsZero() || rt.Identifier.Language != cfg.Language {
			t.Fatalf("scan %+v, runtime %+v", st, rt)
		}
	}
}

// TestUpdateScansAddedRoots checks that adding a root scans only it, and
// removing one only marks what it leaves.
func TestUpdateScansAddedRoots(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}}, false)
	waitScan(t, e)
	// Not seen: ../cine is not scanned again.
	p := filepath.Join(filepath.Dir(e.AppDir), "cine", "Nostalghia (1983)", "Nostalghia.1983.mkv")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("nostalghia"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.Apply(config.Config{Roots: []string{"../cine", "../otro"}}); err != nil {
		t.Fatal(err)
	}
	waitScan(t, e)
	if got, st := titles(t, e), e.Current().Scanner.Status(); !reflect.DeepEqual(got, []string{"Amarcord", "Stalker"}) || st.Files != 1 {
		t.Fatalf("titles %v, scan %+v", got, st)
	}
	if err := e.Apply(config.Config{Roots: []string{"../otro"}}); err != nil {
		t.Fatal(err)
	}
	waitScan(t, e)
	if got := titles(t, e); !reflect.DeepEqual(got, []string{"Stalker"}) {
		t.Fatalf("titles %v", got)
	}
	if idx, _ := e.Store.FileIndex(); !idx["../cine/Amarcord.1973.mkv"].Missing {
		t.Fatalf("index %+v", idx)
	}
}

// TestUpdateFinishesInterruptedScan checks that a scan cut short by a
// change is done again by the next runtime, even if the roots are the same.
func TestUpdateFinishesInterruptedScan(t *testing.T) {
	e := newEngine(t, false)
	e.Start(config.Config{Roots: []string{"../cine"}}, true)
	rt := e.Current()
	started := make(chan struct{})
	rt.Scanner.Probe = func(ctx context.Context, path string) (probe.Info, error) {
		close(started)
		<-ctx.Done()
		return probe.Info{}, ctx.Err()
	}
	e.pending.Store(false) // as if started with a config.json, but with the probe above
	rt.Scan()
	<-started
	if err := e.Apply(config.Config{Roots: []string{"../cine"}, Language: "en-US"}); err != nil {
		t.Fatal(err)
	}
	waitScan(t, e)
	if got := e.Current().Scanner.Status(); got.Files != 1 {
		t.Fatalf("scan %+v", got)
	}
}
