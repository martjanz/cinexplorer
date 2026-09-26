package images

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeFetcher struct {
	calls []string
	err   error
}

func (f *fakeFetcher) Image(ctx context.Context, path, size string) ([]byte, error) {
	f.calls = append(f.calls, size+path)
	if f.err != nil {
		return nil, f.err
	}
	return []byte("img:" + size + path), nil
}

func TestGetDownloadsOnceAndCaches(t *testing.T) {
	f := &fakeFetcher{}
	c := &Cache{Dir: t.TempDir(), Fetch: f}
	ctx := context.Background()
	for range 2 {
		b, err := c.Get(ctx, Poster, 7857, "/p.jpg")
		if err != nil || string(b) != "img:w342/p.jpg" {
			t.Fatalf("got %q, %v", b, err)
		}
	}
	if len(f.calls) != 1 || !c.Has(Poster, 7857) {
		t.Fatalf("calls %v", f.calls)
	}
	if _, err := os.Stat(filepath.Join(c.Dir, "posters", "7857.jpg")); err != nil {
		t.Fatal(err)
	}
	if b, _ := c.Get(ctx, Backdrop, 7857, "/b.jpg"); string(b) != "img:w1280/b.jpg" {
		t.Fatalf("backdrop %q", b)
	}
}

func TestGetUnavailable(t *testing.T) {
	ctx := context.Background()
	offline := &Cache{Dir: t.TempDir(), Fetch: &fakeFetcher{err: errors.New("sin red")}}
	if _, err := offline.Get(ctx, Poster, 1, "/p.jpg"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("offline: %v", err)
	}
	if offline.Has(Poster, 1) {
		t.Error("failed download cached")
	}
	noFetch := &Cache{Dir: t.TempDir()}
	if _, err := noFetch.Get(ctx, Poster, 1, "/p.jpg"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("no fetcher: %v", err)
	}
	f := &fakeFetcher{}
	bad := &Cache{Dir: t.TempDir(), Fetch: f}
	for _, p := range []string{"", "/../x.jpg", "p.jpg", "/a/b.jpg"} {
		if _, err := bad.Get(ctx, Poster, 1, p); !errors.Is(err, ErrUnavailable) {
			t.Errorf("path %q: %v", p, err)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("fetched invalid paths: %v", f.calls)
	}
	if _, err := bad.Get(ctx, "thumb", 1, "/p.jpg"); err == nil {
		t.Error("unknown kind accepted")
	}
}

func TestGetReadOnlyDoesNotStore(t *testing.T) {
	c := &Cache{Dir: t.TempDir(), Fetch: &fakeFetcher{}, ReadOnly: true}
	if b, err := c.Get(context.Background(), Poster, 1, "/p.jpg"); err != nil || len(b) == 0 {
		t.Fatalf("got %q, %v", b, err)
	}
	if c.Has(Poster, 1) {
		t.Fatal("read-only cache stored the image")
	}
}

func TestGetConcurrentDownloads(t *testing.T) {
	c := &Cache{Dir: t.TempDir(), Fetch: &slowFetcher{}}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b, err := c.Get(context.Background(), Poster, 1, "/p.jpg"); err != nil || string(b) != strings.Repeat("x", 1<<16) {
				t.Errorf("got %d bytes, %v", len(b), err)
			}
		}()
	}
	wg.Wait()
	entries, _ := os.ReadDir(filepath.Join(c.Dir, "posters"))
	if len(entries) != 1 || entries[0].Name() != "1.jpg" {
		t.Fatalf("cache dir: %v", entries)
	}
	if b, _ := os.ReadFile(filepath.Join(c.Dir, "posters", "1.jpg")); len(b) != 1<<16 {
		t.Fatalf("cached %d bytes", len(b))
	}
}

// slowFetcher returns a 64 KiB image after a pause, so downloads overlap.
type slowFetcher struct{}

func (slowFetcher) Image(ctx context.Context, path, size string) ([]byte, error) {
	time.Sleep(20 * time.Millisecond)
	return []byte(strings.Repeat("x", 1<<16)), nil
}

func TestGetUnwritableCacheStillServes(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "cache")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil { // a file where the directory should go
		t.Fatal(err)
	}
	c := &Cache{Dir: blocker, Fetch: &fakeFetcher{}}
	if b, err := c.Get(context.Background(), Poster, 1, "/p.jpg"); err != nil || len(b) == 0 {
		t.Fatalf("got %q, %v", b, err)
	}
}

func TestPreviewDoesNotStore(t *testing.T) {
	f := &fakeFetcher{}
	c := &Cache{Dir: t.TempDir(), Fetch: f}
	ctx := context.Background()
	if b, err := c.Preview(ctx, Poster, 1, "/candidate.jpg"); err != nil || string(b) != "img:w342/candidate.jpg" {
		t.Fatalf("got %q, %v", b, err)
	}
	if c.Has(Poster, 1) {
		t.Fatal("preview stored the image")
	}
	// Once the real image is cached, previews serve it too.
	c.Get(ctx, Poster, 1, "/real.jpg")
	if b, _ := c.Preview(ctx, Poster, 1, "/candidate.jpg"); string(b) != "img:w342/real.jpg" {
		t.Fatalf("preview after caching %q", b)
	}
}
