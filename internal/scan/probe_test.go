package scan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cinexplorer/internal/probe"
	"cinexplorer/internal/probe/probetest"
)

func TestScanProbesMainVideosOnce(t *testing.T) {
	disk, app, st := setup(t)
	dir := filepath.Join(disk, "cine", "Stalker (1979)")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Stalker.mkv"), probetest.MKV(1920, 1040, 9_720_000, "rus"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "Stalker.spa.srt"), 100, 's')
	writeFile(t, filepath.Join(dir, "Extras", "Trailer.mkv"), 10, 't')

	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st, Probe: probe.Prober{}.Probe}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := sc.Status(); got.ToProbe != 1 || got.Probed != 1 {
		t.Fatalf("status %+v", got)
	}
	vs, err := st.Versions()
	if err != nil || len(vs) != 1 {
		t.Fatalf("versions %+v, %v", vs, err)
	}
	if v := vs[0]; v.Resolution != "1080p" || v.DurationMs != 9_720_000 || v.Codec != "h264" || len(v.Audio) != 1 || v.Audio[0].Lang != "ru" {
		t.Fatalf("version %+v", v)
	}

	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := sc.Status(); got.ToProbe != 0 {
		t.Fatalf("second scan probed again: %+v", got)
	}
}

func TestScanRetriesOnlyReadErrors(t *testing.T) {
	disk, app, st := setup(t)
	writeFile(t, filepath.Join(disk, "cine", "a", "Locked.mkv"), 4096, 'l')
	writeFile(t, filepath.Join(disk, "cine", "b", "Broken.mkv"), 4096, 'b')
	calls := map[string]int{}
	fake := func(ctx context.Context, p string) (probe.Info, error) {
		calls[filepath.Base(p)]++
		if filepath.Base(p) == "Locked.mkv" {
			return probe.Info{}, fmt.Errorf("%w: sharing violation", probe.ErrIO)
		}
		return probe.Info{}, fmt.Errorf("%w: truncated file", probe.ErrInvalid)
	}
	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st, Probe: fake}
	for range 2 {
		if err := sc.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if calls["Locked.mkv"] != 2 || calls["Broken.mkv"] != 1 {
		t.Fatalf("calls %v", calls)
	}
}

func TestScanCancelKeepsProbedResults(t *testing.T) {
	disk, app, st := setup(t)
	for _, n := range []string{"a", "b", "c"} {
		writeFile(t, filepath.Join(disk, "cine", n, n+".mkv"), 4096, n[0])
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake := func(context.Context, string) (probe.Info, error) {
		cancel() // the drive goes away after the first file
		return probe.Info{Width: 640, Height: 480}, nil
	}
	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st, Probe: fake}
	if err := sc.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	pending, err := st.PendingProbes()
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending %+v, %v", pending, err)
	}
}
