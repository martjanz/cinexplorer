package images

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
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
