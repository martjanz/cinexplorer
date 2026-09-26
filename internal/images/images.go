// Package images keeps TMDB posters and backdrops in cache/ next to the
// catalog, downloading each one at most once.
package images

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

// file is where the image of movie id at tmdbPath is kept: named after the
// path, so a new path (another language, an updated poster) is a new file.
func (c *Cache) file(kind Kind, id int, tmdbPath string) string {
	return filepath.Join(c.Dir, string(kind)+"s", strconv.Itoa(id)+"-"+strings.TrimPrefix(tmdbPath, "/"))
}

// Version is the part of a TMDB path that tells images apart
// ("/kqjL17….jpg" → "kqjL17…"); "" for an invalid path. Pages add it to image
// URLs so that browsers fetch a movie's image again when its path changes.
func Version(tmdbPath string) string {
	if !ValidPath(tmdbPath) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(tmdbPath, "/"), path.Ext(tmdbPath))
}

// Has reports whether the image of movie id at tmdbPath is cached.
func (c *Cache) Has(kind Kind, id int, tmdbPath string) bool {
	if !ValidKind(kind) || !ValidPath(tmdbPath) {
		return false
	}
	_, err := os.Stat(c.file(kind, id, tmdbPath))
	return err == nil
}

// Get returns the image of movie id at tmdbPath, downloading it when it is
// not cached yet; the images cached for the movie's earlier paths are then
// removed. Storing the download is best effort: the image is returned even
// when it cannot be written to the cache.
func (c *Cache) Get(ctx context.Context, kind Kind, id int, tmdbPath string) ([]byte, error) {
	return c.get(ctx, kind, id, tmdbPath, !c.ReadOnly)
}

// Preview is Get without storing the download. It serves candidates whose
// TMDB path comes from the browser: such a path must never decide what the
// cache holds for a movie id.
func (c *Cache) Preview(ctx context.Context, kind Kind, id int, tmdbPath string) ([]byte, error) {
	return c.get(ctx, kind, id, tmdbPath, false)
}

func (c *Cache) get(ctx context.Context, kind Kind, id int, tmdbPath string, keep bool) ([]byte, error) {
	if !ValidKind(kind) {
		return nil, fmt.Errorf("images: kind %q", kind)
	}
	if !ValidPath(tmdbPath) {
		return nil, ErrUnavailable
	}
	name := c.file(kind, id, tmdbPath)
	b, err := os.ReadFile(name)
	if err == nil {
		return b, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if c.Fetch == nil {
		return nil, ErrUnavailable
	}
	b, err = c.Fetch.Image(ctx, tmdbPath, size[kind])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if keep {
		if err := store(name, b); err != nil {
			log.Printf("no se pudo guardar %s: %v", name, err)
		} else {
			removeOthers(name, id)
		}
	}
	return b, nil
}

// removeOthers deletes the images cached for movie id other than keep: the
// ones of earlier paths and the "<id>.jpg" of catalogs from before images
// were named after their path. Temporary files of downloads in progress are
// left alone.
func removeOthers(keep string, id int) {
	dir := filepath.Dir(keep)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix, legacy := strconv.Itoa(id)+"-", strconv.Itoa(id)+".jpg"
	for _, e := range entries {
		n := e.Name()
		if n == filepath.Base(keep) || strings.HasSuffix(n, ".tmp") || (n != legacy && !strings.HasPrefix(n, prefix)) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, n)); err != nil {
			log.Printf("no se pudo borrar %s: %v", n, err)
		}
	}
}

// store writes b to name through a temporary file of its own, renamed at the
// end: a cut never leaves a truncated image, and concurrent downloads of the
// same image (a page request and the prefetch) never share a file.
func store(name string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+".*.tmp")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), name)
	}
	if err != nil {
		_ = os.Remove(f.Name())
	}
	return err
}
