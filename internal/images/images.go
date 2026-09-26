// Package images keeps TMDB posters and backdrops in cache/ next to the
// catalog, downloading each one at most once.
package images

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type Kind string

const (
	Poster   Kind = "poster"
	Backdrop Kind = "backdrop"
)

// size is the TMDB rendition stored for each kind.
var size = map[Kind]string{Poster: "w342", Backdrop: "w1280"}

// ErrUnavailable means the image is not cached and cannot be fetched now.
var ErrUnavailable = errors.New("imagen no disponible")

// Fetcher downloads a TMDB image (tmdb.Client implements it).
type Fetcher interface {
	Image(ctx context.Context, path, size string) ([]byte, error)
}

type Cache struct {
	Dir      string  // <app dir>/cache
	Fetch    Fetcher // nil: serve only what is cached
	ReadOnly bool    // serve downloads without storing them
}

// validPath is the shape of a TMDB image path ("/kqjL17yufvn9OVLyXYpvtyrFfak.jpg").
var validPath = regexp.MustCompile(`^/[A-Za-z0-9_-]+\.(?:jpg|png)$`)

// ValidKind reports whether k is a kind the cache stores.
func ValidKind(k Kind) bool { _, ok := size[k]; return ok }

// ValidPath reports whether p looks like a TMDB image path.
func ValidPath(p string) bool { return validPath.MatchString(p) }

func (c *Cache) file(kind Kind, id int) string {
	return filepath.Join(c.Dir, string(kind)+"s", strconv.Itoa(id)+".jpg")
}

// Has reports whether the image of movie id is cached.
func (c *Cache) Has(kind Kind, id int) bool {
	_, err := os.Stat(c.file(kind, id))
	return err == nil
}

// Get returns the image of movie id, downloading tmdbPath when it is not
// cached yet. The download is written to a temporary file and renamed, so a
// cut never leaves a truncated image behind.
func (c *Cache) Get(ctx context.Context, kind Kind, id int, tmdbPath string) ([]byte, error) {
	if !ValidKind(kind) {
		return nil, fmt.Errorf("images: kind %q", kind)
	}
	name := c.file(kind, id)
	b, err := os.ReadFile(name)
	if err == nil {
		return b, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if c.Fetch == nil || !ValidPath(tmdbPath) {
		return nil, ErrUnavailable
	}
	b, err = c.Fetch.Image(ctx, tmdbPath, size[kind])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if c.ReadOnly {
		return b, nil
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return b, err
	}
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return b, err
	}
	if err := os.Rename(tmp, name); err != nil {
		os.Remove(tmp)
		return b, err
	}
	return b, nil
}
