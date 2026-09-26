# Cinexplorer — Etapa 4a: Catálogo navegable — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reemplazar la página mínima de las etapas 1–3 por una interfaz Svelte embebida en el binario, con Explorar (grilla de afiches con facetas en la URL), Ficha de película con sus versiones en disco y Revisar (Sin identificar y Duplicados), y resolver los pendientes de la Etapa 3 que afectan lo que la interfaz muestra.

**Architecture:** Un paquete Go nuevo, `internal/catalog`, arma todo lo que ven las páginas (ítems de Explorar, facetas y conteos, fichas, duplicados, cola de sin identificar) con funciones puras sobre una instantánea del catálogo que `store` lee en una transacción. El servidor expone esos datos y sirve la app compilada, que vive en `web/` (Svelte 5 + Vite) y se compila a `internal/server/dist/`, commiteado y embebido con `go:embed`: `go build` y `go test` no necesitan Node.

**Tech Stack:** Go (sin dependencias nuevas), `modernc.org/sqlite`, Svelte 5 (runes, JavaScript), Vite 8, Vitest 5, `@fontsource-variable/inter`.

Spec: `docs/superpowers/specs/2026-09-26-cinexplorer-m4a-catalogo-design.md` (detalle de esta etapa) y `docs/superpowers/specs/2026-09-25-cinexplorer-design.md` §5.

**Nota sobre el código de este plan:** todo el código se prototipó y se probó antes de escribir el plan: suite de Go y de Vitest en verde, build de Vite sin advertencias y una prueba real sobre `D:\cine\1970s` con token (Explorar con facetas, Ficha, cola de Revisar con atajos, Duplicados, ancho de teléfono). Los bloques de código salen tal cual del prototipo, y el plan se validó aplicándolo sobre un árbol limpio y comparando el resultado con el prototipo. Copiá el código tal cual; si algo no compila o un test no da lo esperado, es un error del plan: reportalo en lugar de improvisar.

---

## Hoja de ruta (etapas)

| Etapa | Contenido | Estado |
|---|---|---|
| 1 — Núcleo local | escaneo, parser de nombres, versiones, SQLite, API mínima, build, CI | ✅ en `main` |
| 2 — Datos técnicos | lectores nativos MKV/MP4/AVI/IFO, fallback `ffprobe`, tabla `media`, mejor versión | ✅ en `main` |
| 3 — Identificación | TMDB + Wikidata, puntaje de confianza, películas, correcciones por huella, imágenes | ✅ en `main` |
| **4a — Catálogo navegable (este plan)** | Svelte embebido, Explorar con facetas, Ficha, Revisar | |
| 4b — Descubrimiento y primer uso | Inicio estilo MUBI, búsqueda FTS5, asistente de primer uso | |
| 5 — Curaduría | listas, etiquetas, importación de `Collections/` | |

---

## Estructura de archivos (Etapa 4a)

```
internal/images/images.go          caché nombrada por la ruta de TMDB; Version(ruta) para las URLs
internal/store/snapshot.go         read (transacción de lectura), Snapshot
internal/store/store.go            versions(tx), first_seen (migración, SyncFiles), FileView.Size, VersionView.Added
internal/store/schema.sql          files.first_seen
internal/store/identity.go         attachIdentity(tx); sin Unidentified (pasa a catalog)
internal/store/technical.go        attachMedia(tx)
internal/store/movies.go           queryMovies(tx, …)
internal/identify/match.go         clave con idioma, créditos que faltan no cortan la búsqueda, MatcherVersion 2
internal/identify/runner.go        needsMatch con idioma, error por ítem → unmatched, prefetch con Has(ruta)
internal/catalog/items.go          Item, build, Items, Resolution, locations, VersionKey
internal/catalog/facets.go         facetas, Query, ParseQuery, Filter, Counts
internal/catalog/sort.go           Sort
internal/catalog/duplicates.go     Duplicates
internal/catalog/detail.go         Movie, Version, SuggestMovies, Unidentified
internal/server/catalog.go         /api/explore, /api/movies, /api/movies/{id}, /api/versions/{key}, /api/duplicates, /api/unidentified
internal/server/static.go          la app embebida y sus rutas del lado del cliente
internal/server/dist/              build de web/ (commiteado)
cmd/cinexplorer/main.go            setup (armado de la app) separado de run; Roots al servidor
cmd/cinexplorer/main_test.go       smoke test de un primer arranque
web/                               Svelte 5 + Vite
  src/lib/                         router, facets, format, keys, status (+ tests), api, nav.svelte, app.svelte
  src/components/                  Nav, Estado, Afiche, BarraFacetas, Candidatos, SelectorExtra, Identificar,
                                   TarjetaVersion, SinIdentificar, Duplicados
  src/pages/                       Explorar, Pelicula, Version, Revisar
.github/workflows/ci.yml           job web: tests, build y build commiteado al día
scripts/build.sh                   compila la interfaz antes que los binarios
```

Dependencias nuevas entre paquetes: `catalog → {store, images, quality, probe (tests)}`, `server → catalog`. `store` sigue sin depender de nada nuevo. Sin ciclos.

Convenciones que ya usa el proyecto y hay que mantener: comentarios en inglés, mensajes de log y de UI en español, tests de Go en el mismo paquete (`package x`, no `x_test`), commits en inglés con prefijo convencional. En el frontend: JavaScript sin TypeScript, comentarios en inglés, textos en español.

**Entorno.** En Windows con Git Bash, `go` debería estar en el PATH; si no: `export PATH="$PATH:/c/Program Files/Go/bin"`. Vite 8 y Vitest 5 piden **Node 22.12 o posterior**: comprobalo con `node --version`. En la máquina de desarrollo el Node por defecto es 22.6 y hay un Node 24 instalado con nvm-windows; antes de cualquier comando `npm`/`npx`: `export PATH="/c/Users/martin/AppData/Roaming/nvm/v24.21.0:$PATH"`. Algunos archivos existentes están con CRLF en el checkout: los reemplazos de texto de este plan son sobre el contenido, no sobre los finales de línea; `gofmt` normaliza. El build de `web/` se commitea: cada tarea del frontend termina con `npm run build` y suma `internal/server/dist/` al commit.

**Formato de los cambios.** "Crear `x`:" y "Reemplazá `x` completo por:" dan el archivo entero. "En `x`, reemplazá: … por: …" da un fragmento que aparece exactamente una vez en el archivo y su reemplazo; si hay varios en la misma tarea, aplicalos en orden.

---

### Task 1: Imágenes nombradas por su ruta en TMDB

**Files:**
- Modify: `internal/images/images.go` (completo), `internal/identify/runner.go`, `internal/server/identify.go`
- Test: `internal/images/images_test.go` (completo), `internal/server/identify_test.go`

Pendiente 1 de la Etapa 3 (spec §7): la caché guardaba `cache/posters/{id}.jpg`, así que un cambio de `poster_path` (otro idioma, un afiche nuevo en TMDB) seguía sirviendo el archivo viejo. Ahora el archivo se llama `{id}-{nombre de la ruta}.jpg`; al guardar uno se borran los anteriores de esa película (incluido el `{id}.jpg` de la Etapa 3, y no los temporales de descargas en curso). `Has` recibe la ruta. `images.Version(ruta)` da el nombre sin extensión, que las páginas agregan como `?v=` a la URL: con el `v` vigente la respuesta se puede guardar para siempre en el navegador; sin él o con uno viejo, `no-cache`. Las vistas previas de candidatos (`?p=`) mantienen `max-age=86400`.

- [ ] **Step 1: Escribir los tests que fallan**

Reemplazá `internal/images/images_test.go` completo por:

```go
package images

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	if len(f.calls) != 1 || !c.Has(Poster, 7857, "/p.jpg") {
		t.Fatalf("calls %v", f.calls)
	}
	if _, err := os.Stat(filepath.Join(c.Dir, "posters", "7857-p.jpg")); err != nil {
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
	if offline.Has(Poster, 1, "/p.jpg") {
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
	if c.Has(Poster, 1, "/p.jpg") {
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
	if len(entries) != 1 || entries[0].Name() != "1-p.jpg" {
		t.Fatalf("cache dir: %v", entries)
	}
	if b, _ := os.ReadFile(filepath.Join(c.Dir, "posters", "1-p.jpg")); len(b) != 1<<16 {
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
	if c.Has(Poster, 1, "/candidate.jpg") {
		t.Fatal("preview stored the image")
	}
	// A preview of the cached path is served from the cache.
	c.Get(ctx, Poster, 1, "/real.jpg")
	if b, _ := c.Preview(ctx, Poster, 1, "/real.jpg"); string(b) != "img:w342/real.jpg" || len(f.calls) != 2 {
		t.Fatalf("preview after caching %q, calls %v", b, f.calls)
	}
}

func TestNewPathReplacesCachedImage(t *testing.T) {
	f := &fakeFetcher{}
	c := &Cache{Dir: t.TempDir(), Fetch: f}
	ctx := context.Background()
	dir := filepath.Join(c.Dir, "posters")
	os.MkdirAll(dir, 0o755)
	// A stage 3 cache file, another movie's image and a download in progress.
	for _, name := range []string{"12.jpg", "123-x.jpg", "12-new.jpg.42.tmp"} {
		os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644)
	}
	c.Get(ctx, Poster, 12, "/old.jpg")
	if b, _ := c.Get(ctx, Poster, 12, "/new.jpg"); string(b) != "img:w342/new.jpg" {
		t.Fatalf("new path served %q", b)
	}
	var names []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{"12-new.jpg", "12-new.jpg.42.tmp", "123-x.jpg"}; !slices.Equal(names, want) {
		t.Fatalf("cache dir %v, want %v", names, want)
	}
	if c.Has(Poster, 12, "/old.jpg") || !c.Has(Poster, 12, "/new.jpg") {
		t.Fatal("Has does not follow the path")
	}
}

func TestVersion(t *testing.T) {
	for p, want := range map[string]string{"/kqjL17yufvn9OVLyXYpvtyrFfak.jpg": "kqjL17yufvn9OVLyXYpvtyrFfak", "/a-b_c.png": "a-b_c", "": "", "../x.jpg": ""} {
		if got := Version(p); got != want {
			t.Errorf("Version(%q) = %q, want %q", p, got, want)
		}
	}
}
```

En `internal/server/identify_test.go`, reemplazá:

```go
	}
	if s.Images.Has(images.Poster, 7857) {
		t.Fatal("candidate preview was cached")
```

por:

```go
	}
	if s.Images.Has(images.Poster, 7857, "/other.jpg") {
		t.Fatal("candidate preview was cached")
```

En `internal/server/identify_test.go`, reemplazá:

```go
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
```

por:

```go
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
	// With the current version the URL names one image for good; without it
	// (or with an old one) the browser has to ask again.
	for url, want := range map[string]string{
		"/img/poster/7857.jpg?v=p":      "public, max-age=31536000, immutable",
		"/img/poster/7857.jpg?v=old":    "no-cache",
		"/img/poster/7857.jpg":          "no-cache",
		"/img/poster/7857.jpg?p=/x.jpg": "no-cache",
	} {
		if got := request(s.Handler(), "GET", url, "", "", "127.0.0.1").Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control %q, want %q", url, got, want)
		}
	}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/images ./internal/server`
Expected: FAIL de compilación en `internal/images` (`too many arguments in call to c.Has`) y en `internal/server`.

- [ ] **Step 3: Implementar**

Reemplazá `internal/images/images.go` completo por:

```go
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
```

En `internal/identify/runner.go`, reemplazá:

```go
		for _, w := range wanted {
			if w.path == "" || r.Images.Has(w.kind, m.TMDBID) {
				continue
```

por:

```go
		for _, w := range wanted {
			if w.path == "" || r.Images.Has(w.kind, m.TMDBID, w.path) {
				continue
```

En `internal/server/identify.go`, reemplazá:

```go
// downloading it once when missing. A candidate that is not a stored movie
// yet passes its TMDB path as ?p=.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
```

por:

```go
// downloading it once when missing. A candidate that is not a stored movie
// yet passes its TMDB path as ?p=. Pages add the image's version as ?v= so
// that browsers can keep it for good.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
```

En `internal/server/identify.go`, reemplazá:

```go
	var b []byte
	if found {
```

por:

```go
	var b []byte
	cache := "no-cache"
	if found {
```

En `internal/server/identify.go`, reemplazá:

```go
		b, err = s.Images.Get(r.Context(), kind, id, path)
	} else {
```

por:

```go
		b, err = s.Images.Get(r.Context(), kind, id, path)
		// ?v= names the image at one path: the URL changes with the path.
		if v := r.URL.Query().Get("v"); v != "" && v == images.Version(path) {
			cache = "public, max-age=31536000, immutable"
		}
	} else {
```

En `internal/server/identify.go`, reemplazá:

```go
		b, err = s.Images.Preview(r.Context(), kind, id, r.URL.Query().Get("p"))
	}
```

por:

```go
		b, err = s.Images.Preview(r.Context(), kind, id, r.URL.Query().Get("p"))
		cache = "max-age=86400"
	}
```

En `internal/server/identify.go`, reemplazá:

```go
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(b)
```

por:

```go
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Header().Set("Cache-Control", cache)
	w.Write(b)
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l internal/images internal/server internal/identify; go vet ./... && go test ./...`
Expected: `gofmt` puede listar archivos que no tocaste (CRLF del checkout); los que tocaste no deben aparecer. `go vet` sin salida; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/images internal/identify/runner.go internal/server/identify.go internal/server/identify_test.go
git commit -m "fix(images): cache images by TMDB path; versioned image URLs"
```

---

### Task 2: Lecturas en una transacción e instantánea del catálogo

**Files:**
- Create: `internal/store/snapshot.go`
- Modify: `internal/store/store.go`, `internal/store/technical.go`, `internal/store/identity.go`, `internal/store/movies.go`
- Test: `internal/store/snapshot_test.go`

Pendiente de la Etapa 2 y spec §5.1: con el escáner y el Runner escribiendo en segundo plano, una respuesta armada con varias consultas sueltas podía mezclar estados. `read(f)` corre `f` dentro de una transacción; `versions`, `attachMedia`, `attachIdentity`, `representatives`, `identifications` y `queryMovies` reciben el `querier` de la transacción (el catálogo tiene una sola conexión, que la transacción ocupa: dentro de `f` no se puede usar `s.db`). `Snapshot()` lee versiones, identificaciones y películas en una sola transacción; es lo que va a usar `internal/catalog`. `Unidentified` también pasa a leer en una transacción (en la Task 8 se reemplaza por `catalog.Unidentified`).

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/store/snapshot_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"
)

func TestSnapshot(t *testing.T) {
	s := identityCatalog(t)
	s.SaveMovie(Movie{TMDBID: 7857, Title: "Amarcord", Year: 1973, Genres: []string{"Comedia"}, Language: "es-ES"})
	s.SaveIdentifications([]Identification{
		{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857, Confidence: 0.93},
		{Fingerprint: "vob2", Status: StatusUnmatched, Candidates: []Candidate{{TMDBID: 1398, Title: "Stalker", Score: 0.7}}},
	})
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	vs, _ := s.Versions()
	if len(snap.Versions) != len(vs) || len(vs) != 3 {
		t.Fatalf("versions %d, want %d", len(snap.Versions), len(vs))
	}
	for i := range vs {
		if snap.Versions[i].ID != vs[i].ID || snap.Versions[i].Best != vs[i].Best || snap.Versions[i].Fingerprint != vs[i].Fingerprint {
			t.Errorf("version %d: %+v, want %+v", i, snap.Versions[i], vs[i])
		}
	}
	if m := snap.Movies[7857]; m.Title != "Amarcord" || m.Genres[0] != "Comedia" || len(snap.Movies) != 1 {
		t.Fatalf("movies %+v", snap.Movies)
	}
	if id := snap.Identifications["vob2"]; id == nil || id.Candidates[0].TMDBID != 1398 || len(snap.Identifications) != 2 {
		t.Fatalf("identifications %+v", snap.Identifications)
	}
}

func TestSnapshotOfOlderReadOnlyCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cinexplorer.db")
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	s.db.Exec(`DROP TABLE identifications`)
	s.db.Exec(`DROP TABLE movies`)
	s.Close()
	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	snap, err := ro.Snapshot()
	if err != nil || len(snap.Versions) != 0 || len(snap.Movies) != 0 || len(snap.Identifications) != 0 {
		t.Fatalf("snapshot %+v, %v", snap, err)
	}
}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/store`
Expected: FAIL de compilación: `s.Snapshot undefined`.

- [ ] **Step 3: Implementar**

Crear `internal/store/snapshot.go`:

```go
package store

// Snapshot is the whole catalog as the pages see it, read at one point in
// time.
type Snapshot struct {
	Versions        []VersionView              // as Versions returns them
	Identifications map[string]*Identification // by fingerprint
	Movies          map[int]Movie              // by TMDB id
}

// read runs f inside a transaction, so that everything f reads comes from
// one state of the catalog even while the scanner or the identification
// runner write. f must read through tx only: the catalog has a single
// connection, which the transaction holds.
func (s *Store) read(f func(tx querier) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return f(tx)
}

// Snapshot reads versions, identifications and movies in one transaction.
func (s *Store) Snapshot() (Snapshot, error) {
	snap := Snapshot{Identifications: map[string]*Identification{}, Movies: map[int]Movie{}}
	err := s.read(func(tx querier) error {
		vs, err := s.versions(tx)
		if err != nil {
			return err
		}
		snap.Versions = vs
		if !s.hasIdentity {
			return nil
		}
		if snap.Identifications, err = s.identifications(tx); err != nil {
			return err
		}
		ms, err := s.queryMovies(tx, `SELECT `+movieColumns+` FROM movies ORDER BY tmdb_id`)
		for _, m := range ms {
			snap.Movies[m.TMDBID] = m
		}
		return err
	})
	return snap, err
}
```

En `internal/store/store.go`, reemplazá:

```go

// Versions returns every version with its files and technical data, ordered
// by title and year.
func (s *Store) Versions() ([]VersionView, error) {
	rows, err := s.db.Query(`SELECT id, dir, title, year, director, countries, resolution, source, codec, language,
		size, parts, sub_langs FROM versions ORDER BY title COLLATE NOCASE, year, id`)
```

por:

```go

// Versions returns every version with its files, technical data and
// identity, ordered by title and year.
func (s *Store) Versions() ([]VersionView, error) {
	var vs []VersionView
	err := s.read(func(tx querier) (err error) {
		vs, err = s.versions(tx)
		return err
	})
	return vs, err
}

func (s *Store) versions(tx querier) ([]VersionView, error) {
	rows, err := tx.Query(`SELECT id, dir, title, year, director, countries, resolution, source, codec, language,
		size, parts, sub_langs FROM versions ORDER BY title COLLATE NOCASE, year, id`)
```

En `internal/store/store.go`, reemplazá:

```go

	frows, err := s.db.Query(`SELECT version_id, path, role, part, lang, missing FROM files
		WHERE version_id IS NOT NULL
```

por:

```go

	frows, err := tx.Query(`SELECT version_id, path, role, part, lang, missing FROM files
		WHERE version_id IS NOT NULL
```

En `internal/store/store.go`, reemplazá:

```go
	}
	if err := s.attachMedia(out, pos); err != nil {
		return nil, err
	}
	if err := s.attachIdentity(out, pos); err != nil {
		return nil, err
```

por:

```go
	}
	if err := s.attachMedia(tx, out, pos); err != nil {
		return nil, err
	}
	if err := s.attachIdentity(tx, out, pos); err != nil {
		return nil, err
```

En `internal/store/technical.go`, reemplazá:

```go
// and failed ones are ignored, so the name-parsed values remain.
func (s *Store) attachMedia(out []VersionView, pos map[int64]int) error {
	if !s.hasMedia {
```

por:

```go
// and failed ones are ignored, so the name-parsed values remain.
func (s *Store) attachMedia(tx querier, out []VersionView, pos map[int64]int) error {
	if !s.hasMedia {
```

En `internal/store/technical.go`, reemplazá:

```go
	}
	rows, err := s.db.Query(`SELECT f.version_id, f.kind, m.duration_ms, m.width, m.height, m.video_codec, m.audio, m.subs
		FROM files f JOIN media m ON m.file_id = f.id
```

por:

```go
	}
	rows, err := tx.Query(`SELECT f.version_id, f.kind, m.duration_ms, m.width, m.height, m.video_codec, m.audio, m.subs
		FROM files f JOIN media m ON m.file_id = f.id
```

En `internal/store/identity.go`, reemplazá:

```go
func (s *Store) Unidentified() ([]UnidentifiedView, error) {
	vs, err := s.Versions()
	if err != nil {
		return nil, err
	}
	ids, err := s.identifications(s.db)
	if err != nil {
```

por:

```go
func (s *Store) Unidentified() ([]UnidentifiedView, error) {
	var vs []VersionView
	var ids map[string]*Identification
	err := s.read(func(tx querier) (err error) {
		if vs, err = s.versions(tx); err != nil {
			return err
		}
		ids, err = s.identifications(tx)
		return err
	})
	if err != nil {
```

En `internal/store/identity.go`, reemplazá:

```go
// attachIdentity fills Fingerprint, Identification and Movie of each version.
func (s *Store) attachIdentity(out []VersionView, pos map[int64]int) error {
	reps, err := s.representatives(s.db)
	if err != nil {
```

por:

```go
// attachIdentity fills Fingerprint, Identification and Movie of each version.
func (s *Store) attachIdentity(tx querier, out []VersionView, pos map[int64]int) error {
	reps, err := s.representatives(tx)
	if err != nil {
```

En `internal/store/identity.go`, reemplazá:

```go
		return nil
	}
	ids, err := s.identifications(s.db)
	if err != nil {
		return err
```

por:

```go
		return nil
	}
	ids, err := s.identifications(tx)
	if err != nil {
		return err
```

En `internal/store/identity.go`, reemplazá:

```go
	refs := map[int]*MovieRef{}
	rows, err := s.db.Query(`SELECT tmdb_id, title, original_title, year, directors FROM movies`)
	if err != nil {
```

por:

```go
	refs := map[int]*MovieRef{}
	rows, err := tx.Query(`SELECT tmdb_id, title, original_title, year, directors FROM movies`)
	if err != nil {
```

En `internal/store/movies.go`, reemplazá:

```go
	}
	return s.queryMovies(`SELECT ` + movieColumns + ` FROM movies ORDER BY tmdb_id`)
}
```

por:

```go
	}
	return s.queryMovies(s.db, `SELECT `+movieColumns+` FROM movies ORDER BY tmdb_id`)
}
```

En `internal/store/movies.go`, reemplazá:

```go
func (s *Store) PendingWikidata(limit int) ([]Movie, error) {
	return s.queryMovies(`SELECT `+movieColumns+` FROM movies WHERE wikidata_state = 0 ORDER BY tmdb_id LIMIT ?`, limit)
}

func (s *Store) queryMovies(q string, args ...any) ([]Movie, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
```

por:

```go
func (s *Store) PendingWikidata(limit int) ([]Movie, error) {
	return s.queryMovies(s.db, `SELECT `+movieColumns+` FROM movies WHERE wikidata_state = 0 ORDER BY tmdb_id LIMIT ?`, limit)
}

func (s *Store) queryMovies(tx querier, q string, args ...any) ([]Movie, error) {
	rows, err := tx.Query(q, args...)
	if err != nil {
```

- [ ] **Step 4: Correr los tests**

Run: `go vet ./... && go test ./...`
Expected: todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): read transactions and catalog snapshot"
```

---

### Task 3: Fecha de alta y tamaño de cada archivo

**Files:**
- Modify: `internal/store/schema.sql`, `internal/store/store.go`
- Test: `internal/store/firstseen_test.go`

Spec §5.2. `files.first_seen` (unix ms) es la primera columna que se agrega a una tabla existente, así que `openDB` (solo en aperturas escribibles) la agrega con `ALTER TABLE` si falta y la llena con `mtime`. `SyncFiles` la completa al insertar una ruta nueva: hereda la fecha más antigua de su huella (un archivo movido no es "nuevo") o, sin huella previa, la hora actual; en un reescaneo no se toca. Un catálogo viejo abierto en modo consulta (sin la columna) lee `0` (`hasFirstSeen`). `VersionView.Added` es la menor `first_seen` de sus archivos principales presentes, y `FileView.Size` el tamaño de cada archivo (la Ficha lo muestra para los extras).

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/store/firstseen_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"
	"time"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
)

func firstSeen(t *testing.T, s *Store, path string) int64 {
	t.Helper()
	var v int64
	if err := s.db.QueryRow(`SELECT first_seen FROM files WHERE path = ?`, path).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFirstSeen(t *testing.T) {
	s := open(t)
	before := time.Now().UnixMilli()
	s.SyncFiles([]FileRow{{Path: "../cine/a.mkv", Size: 10, MTime: 1, Fingerprint: "fa", Kind: "video"}}, roots)
	first := firstSeen(t, s, "../cine/a.mkv")
	if first < before || first > time.Now().UnixMilli() {
		t.Fatalf("first_seen %d, want the time of the scan", first)
	}
	// Rescanning a changed file keeps the date.
	time.Sleep(5 * time.Millisecond)
	s.SyncFiles([]FileRow{{Path: "../cine/a.mkv", Size: 11, MTime: 2, Fingerprint: "fa2", Kind: "video"}}, roots)
	if got := firstSeen(t, s, "../cine/a.mkv"); got != first {
		t.Fatalf("rescan changed first_seen: %d → %d", first, got)
	}
	// A moved file (new path, same content) keeps the date of its content.
	s.SyncFiles([]FileRow{{Path: "../cine/b/a.mkv", Size: 11, MTime: 2, Fingerprint: "fa2", Kind: "video"}}, roots)
	if got := firstSeen(t, s, "../cine/b/a.mkv"); got != first {
		t.Fatalf("moved file first_seen %d, want %d", got, first)
	}
	// Without a fingerprint there is nothing to follow.
	s.SyncFiles([]FileRow{{Path: "../cine/c.mkv", Size: 0, MTime: 2, Kind: "video"}}, roots)
	if got := firstSeen(t, s, "../cine/c.mkv"); got <= first {
		t.Fatalf("new empty file first_seen %d", got)
	}
}

func TestVersionAdded(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{
		{Path: "../cine/x/CD1.avi", Size: 10, MTime: 1, Fingerprint: "p1", Kind: "video"},
		{Path: "../cine/x/CD2.avi", Size: 10, MTime: 1, Fingerprint: "p2", Kind: "video"},
	}, roots)
	s.db.Exec(`UPDATE files SET first_seen = 500 WHERE path = '../cine/x/CD1.avi'`)
	s.db.Exec(`UPDATE files SET first_seen = 300 WHERE path = '../cine/x/CD2.avi'`)
	s.ReplaceVersions([]grouping.Version{{Dir: "../cine/x", Parsed: nameparse.Parsed{Title: "X"}, Size: 20, Parts: 2,
		Members: []grouping.Member{{Path: "../cine/x/CD1.avi", Role: grouping.RoleMain, Part: 1},
			{Path: "../cine/x/CD2.avi", Role: grouping.RoleMain, Part: 2}}}})
	vs, err := s.Versions()
	if err != nil || len(vs) != 1 || vs[0].Added != 300 || vs[0].Files[0].Size != 10 {
		t.Fatalf("versions %+v, %v", vs, err)
	}
}

// olderCatalog is a catalog from before first_seen, with one file.
func olderCatalog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cinexplorer.db")
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`ALTER TABLE files DROP COLUMN first_seen`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO files (path, size, mtime, fingerprint, kind) VALUES ('../cine/a.mkv', 10, 1234, 'fa', 'video')`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	return path
}

func TestFirstSeenMigration(t *testing.T) {
	path := olderCatalog(t)
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := firstSeen(t, s, "../cine/a.mkv"); got != 1234 {
		t.Fatalf("migrated first_seen %d, want the mtime", got)
	}
}

func TestOlderReadOnlyCatalogWithoutFirstSeen(t *testing.T) {
	path := olderCatalog(t)
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Versions(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/store`
Expected: FAIL de compilación: `vs[0].Added undefined` y `vs[0].Files[0].Size undefined`.

- [ ] **Step 3: Implementar**

En `internal/store/schema.sql`, reemplazá:

```sql
  part        INTEGER NOT NULL DEFAULT 0,
  lang        TEXT    NOT NULL DEFAULT ''
);
```

por:

```sql
  part        INTEGER NOT NULL DEFAULT 0,
  lang        TEXT    NOT NULL DEFAULT '',
  first_seen  INTEGER NOT NULL DEFAULT 0    -- unix milliseconds: when the content was first catalogued
);
```

En `internal/store/store.go`, reemplazá:

```go
	"strings"
```

por:

```go
	"strings"
	"time"
```

En `internal/store/store.go`, reemplazá:

```go
	hasIdentity bool
}
```

por:

```go
	hasIdentity bool
	// hasFirstSeen is false for a catalog from before files.first_seen
	// opened read-only.
	hasFirstSeen bool
}
```

En `internal/store/store.go`, reemplazá:

```go
	Path    string `json:"path"`
	Role    string `json:"role"`
```

por:

```go
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Role    string `json:"role"`
```

En `internal/store/store.go`, reemplazá:

```go
	Files      []FileView `json:"files"`
```

por:

```go
	Files      []FileView `json:"files"`
	Added      int64      `json:"added"` // unix ms: first_seen of its earliest present main file
```

En `internal/store/store.go`, reemplazá:

```go
			return nil, err
		}
	}
	s := &Store{db: db}
```

por:

```go
			return nil, err
		}
		if err := addFirstSeen(db); err != nil {
			db.Close()
			return nil, err
		}
	}
	s := &Store{db: db}
```

En `internal/store/store.go`, reemplazá:

```go
	}
	return s, nil
}
```

por:

```go
	}
	if err := db.QueryRow(`SELECT COUNT(*) > 0 FROM pragma_table_info('files') WHERE name = 'first_seen'`).Scan(&s.hasFirstSeen); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// addFirstSeen adds files.first_seen to a catalog from before it existed
// (CREATE TABLE IF NOT EXISTS does not add columns). The files already
// catalogued get their modification time.
func addFirstSeen(db *sql.DB) error {
	var has bool
	if err := db.QueryRow(`SELECT COUNT(*) > 0 FROM pragma_table_info('files') WHERE name = 'first_seen'`).Scan(&has); err != nil || has {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`ALTER TABLE files ADD COLUMN first_seen INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE files SET first_seen = mtime`); err != nil {
		return err
	}
	return tx.Commit()
}
```

En `internal/store/store.go`, reemplazá:

```go

	up, err := tx.Prepare(`INSERT INTO files (path, size, mtime, fingerprint, kind, missing) VALUES (?, ?, ?, ?, ?, 0)
		ON CONFLICT(path) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
```

por:

```go

	// A new path gets the date its content was first seen under any path
	// (a moved file is not new), or now. Known paths keep theirs.
	up, err := tx.Prepare(`INSERT INTO files (path, size, mtime, fingerprint, kind, missing, first_seen)
		VALUES (?1, ?2, ?3, ?4, ?5, 0, COALESCE((SELECT MIN(first_seen) FROM files WHERE fingerprint = ?4 AND ?4 != ''), ?6))
		ON CONFLICT(path) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
```

En `internal/store/store.go`, reemplazá:

```go
	defer up.Close()
	present := make(map[string]bool, len(seen))
	for _, f := range seen {
		if _, err := up.Exec(f.Path, f.Size, f.MTime, f.Fingerprint, f.Kind); err != nil {
			return err
```

por:

```go
	defer up.Close()
	now := time.Now().UnixMilli()
	present := make(map[string]bool, len(seen))
	for _, f := range seen {
		if _, err := up.Exec(f.Path, f.Size, f.MTime, f.Fingerprint, f.Kind, now); err != nil {
			return err
```

En `internal/store/store.go`, reemplazá:

```go

	frows, err := tx.Query(`SELECT version_id, path, role, part, lang, missing FROM files
		WHERE version_id IS NOT NULL
```

por:

```go

	firstSeen := "0"
	if s.hasFirstSeen {
		firstSeen = "first_seen"
	}
	frows, err := tx.Query(`SELECT version_id, path, size, role, part, lang, missing, ` + firstSeen + ` FROM files
		WHERE version_id IS NOT NULL
```

En `internal/store/store.go`, reemplazá:

```go
	for frows.Next() {
		var id int64
		var f FileView
		if err := frows.Scan(&id, &f.Path, &f.Role, &f.Part, &f.Lang, &f.Missing); err != nil {
			return nil, err
```

por:

```go
	for frows.Next() {
		var id, seen int64
		var f FileView
		if err := frows.Scan(&id, &f.Path, &f.Size, &f.Role, &f.Part, &f.Lang, &f.Missing, &seen); err != nil {
			return nil, err
```

En `internal/store/store.go`, reemplazá:

```go
		if i, ok := pos[id]; ok {
			out[i].Files = append(out[i].Files, f)
		}
```

por:

```go
		if i, ok := pos[id]; ok {
			v := &out[i]
			v.Files = append(v.Files, f)
			if f.Role == "main" && !f.Missing && (v.Added == 0 || seen < v.Added) {
				v.Added = seen
			}
		}
```

- [ ] **Step 4: Correr los tests**

Run: `go vet ./... && go test ./...`
Expected: todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): first_seen date and size of files, added date of versions"
```

---

### Task 4: Idioma en la clave de identificación y errores por ítem a revisión

**Files:**
- Modify: `internal/identify/match.go`, `internal/identify/runner.go`
- Test: `internal/identify/fake_test.go`, `internal/identify/runner_test.go`, `internal/identify/match_test.go`

Pendientes 3 y 4 de la Etapa 3 (spec §8). La clave guardada (`storedKey`) es la de la consulta más el idioma: al cambiar `language`, las `auto`/`unmatched` se recalculan y los candidatos quedan en el idioma nuevo. Al revisar directores, un error que no es de red ni de token en `Movie()` (una película que TMDB ya no tiene) solo le quita a ese candidato la parte del director. Cualquier otro error que no es de red ni de token al identificar un ítem lo guarda como `unmatched` sin candidatos, en lugar de reintentarlo en cada corrida: así aparece en Revisar. `MatcherVersion` pasa a 2. El corpus de calibración no se regraba (no cambia ni el puntaje ni los umbrales).

- [ ] **Step 1: Escribir los tests que fallan**

En `internal/identify/fake_test.go`, reemplazá:

```go
	err    error                   // returned by every call when set
	calls  []string
```

por:

```go
	err    error                   // returned by every call when set
	fail   map[string]error        // returned by the search with that key
	calls  []string
```

En `internal/identify/fake_test.go`, reemplazá:

```go
	if err := f.record("search " + key); err != nil {
		return nil, err
```

por:

```go
	if err := f.record("search " + key); err != nil {
		return nil, err
	}
	if err := f.fail[key]; err != nil {
		return nil, err
```

En `internal/identify/runner_test.go`, reemplazá:

```go
	a, d := current(t, r.Store, "a1"), current(t, r.Store, "vob")
	if a.Status != store.StatusAuto || a.TMDBID != 7857 || a.Query != "Amarcord|1973||" || a.MatcherVersion != MatcherVersion {
		t.Fatalf("amarcord %+v", a)
	}
	if d.Status != store.StatusAuto || d.TMDBID != 1398 || d.Confidence != 1 || d.Query != "d|0||tt0079944" {
		t.Fatalf("stalker %+v", d)
```

por:

```go
	a, d := current(t, r.Store, "a1"), current(t, r.Store, "vob")
	if a.Status != store.StatusAuto || a.TMDBID != 7857 || a.Query != "Amarcord|1973|||es-ES" || a.MatcherVersion != MatcherVersion {
		t.Fatalf("amarcord %+v", a)
	}
	if d.Status != store.StatusAuto || d.TMDBID != 1398 || d.Confidence != 1 || d.Query != "d|0||tt0079944|es-ES" {
		t.Fatalf("stalker %+v", d)
```

En `internal/identify/runner_test.go`, reemplazá:

```go
	r.Run(ctx)
	if api.called("search Amarcord|1973") != 1 || current(t, r.Store, "a1").Query != "Amarcord|1973||" {
		t.Fatalf("stale query not redone: %v", api.calls)
```

por:

```go
	r.Run(ctx)
	if api.called("search Amarcord|1973") != 1 || current(t, r.Store, "a1").Query != "Amarcord|1973|||es-ES" {
		t.Fatalf("stale query not redone: %v", api.calls)
```

En `internal/identify/runner_test.go`, reemplazá:

```go
		t.Fatalf("old matcher result not redone: %v", api.calls)
	}
```

por:

```go
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
```

En `internal/identify/runner_test.go`, reemplazá:

```go
	if g.Status != store.StatusAuto || g.TMDBID != 537898 ||
		g.Query != "Los Gauchos Judios Rip mentecato|0||"+"|Los Gauchos Judíos|1974|Juan José Jusid|" {
		t.Fatalf("gauchos %+v", g)
```

por:

```go
	if g.Status != store.StatusAuto || g.TMDBID != 537898 ||
		g.Query != "Los Gauchos Judios Rip mentecato|0||"+"|Los Gauchos Judíos|1974|Juan José Jusid||es-ES" {
		t.Fatalf("gauchos %+v", g)
```

En `internal/identify/match_test.go`, reemplazá:

```go

func TestIdentifyWithoutTitle(t *testing.T) {
```

por:

```go

func TestSearchSkipsMissingCredits(t *testing.T) {
	// A candidate whose details TMDB no longer has gets no director share,
	// instead of failing the whole search.
	api := &fakeAPI{
		search: map[string][]tmdb.Result{"Ordet|0|es-ES": {
			{ID: 262879, Title: "Ordet", ReleaseDate: "1943-01-01"},
			{ID: 48035, Title: "La palabra", OriginalTitle: "Ordet", ReleaseDate: "1955-01-10"},
		}},
		movies: map[string]tmdb.Details{"48035|es-ES": details(48035, "La palabra", "1955-01-10", "", "Carl Theodor Dreyer")},
	}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Ordet", Director: "Dreyer"})
	if err != nil || id.Status != store.StatusAuto || id.TMDBID != 48035 {
		t.Fatalf("got %+v, %v", id, err)
	}
}

func TestIdentifyWithoutTitle(t *testing.T) {
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/identify`
Expected: FAIL en `TestSearchSkipsMissingCredits` (`no existe en TMDB`), `TestRunIdentifiesAndEnriches` (la clave no termina en `|es-ES`), `TestRunKeepsCorrectionsAndRematchesStale`, `TestRunRematchesOnLanguageChange`, `TestRunSavesFailedItemAsUnmatched` y `TestRunFallsBackToFolderName`.

- [ ] **Step 3: Implementar**

En `internal/identify/match.go`, reemplazá:

```go
// MatcherVersion changes whenever the matching algorithm or its thresholds
// change, so earlier automatic results are recomputed.
const MatcherVersion = 1
```

por:

```go
// MatcherVersion changes whenever the matching algorithm or its thresholds
// change, so earlier automatic results are recomputed. 2: the language is
// part of the stored query; unavailable credits no longer fail a search.
const MatcherVersion = 2
```

En `internal/identify/match.go`, reemplazá:

```go

// Key identifies the query; a stored result made for another key is stale.
func (q Query) Key() string {
```

por:

```go

// Key identifies the query.
func (q Query) Key() string {
```

En `internal/identify/match.go`, reemplazá:

```go
	return k
}
```

por:

```go
	return k
}

// storedKey identifies a query made in a language: a stored result made for
// another key is stale. The language counts because the candidates' titles
// are in it.
func (q Query) storedKey(lang string) string {
	return q.Key() + "|" + lang
}
```

En `internal/identify/match.go`, reemplazá:

```go
			d, err := api.Movie(ctx, cands[i].TMDBID, lang)
			if err != nil {
				return nil, nil, err
			}
```

por:

```go
			d, err := api.Movie(ctx, cands[i].TMDBID, lang)
			if fatal(err) {
				return nil, nil, err
			}
			if err != nil {
				// Details TMDB cannot give now (a removed movie) only cost
				// the candidate its director share.
				continue
			}
```

En `internal/identify/match.go`, reemplazá:

```go
	}
	id.Query = q.Key()
	return id, details, nil
```

por:

```go
	}
	id.Query = q.storedKey(lang)
	return id, details, nil
```

En `internal/identify/runner.go`, reemplazá:

```go
// of it. Corrections are never redone.
func needsMatch(cur *store.Identification, q Query) bool {
	if cur == nil {
```

por:

```go
// of it. Corrections are never redone.
func needsMatch(cur *store.Identification, q Query, lang string) bool {
	if cur == nil {
```

En `internal/identify/runner.go`, reemplazá:

```go
	}
	return cur.Query != q.Key() || cur.MatcherVersion < MatcherVersion
}
```

por:

```go
	}
	return cur.Query != q.storedKey(lang) || cur.MatcherVersion < MatcherVersion
}
```

En `internal/identify/runner.go`, reemplazá:

```go
		}
		if needsMatch(t.Current, q) {
			jobs = append(jobs, job{t.Fingerprint, q})
```

por:

```go
		}
		if needsMatch(t.Current, q, r.Language) {
			jobs = append(jobs, job{t.Fingerprint, q})
```

En `internal/identify/runner.go`, reemplazá:

```go
		if err != nil {
			log.Printf("no se pudo identificar %q: %v", j.query.Title, err)
			continue
		}
```

por:

```go
		if err != nil {
			// Left for review rather than retried on every run.
			log.Printf("no se pudo identificar %q: %v", j.query.Title, err)
			id = store.Identification{Status: store.StatusUnmatched, Query: j.query.storedKey(r.Language),
				MatcherVersion: MatcherVersion}
		}
```

- [ ] **Step 4: Correr los tests**

Run: `go vet ./... && go test ./...`
Expected: todos los paquetes `ok` (el corpus de `internal/identify` también).

- [ ] **Step 5: Commit**

```bash
git add internal/identify
git commit -m "fix(identify): language in the query key; failed items go to review"
```

---

### Task 5: Catálogo: ítems, facetas y orden de Explorar

**Files:**
- Create: `internal/catalog/items.go`, `internal/catalog/facets.go`, `internal/catalog/sort.go`
- Test: `internal/catalog/fixture_test.go`, `internal/catalog/items_test.go`, `internal/catalog/facets_test.go`

Spec §5.3. `internal/catalog` no hace SQL: trabaja sobre un `store.Snapshot`, así que los tests arman el catálogo a mano (`fixture_test.go`). `build` agrupa las versiones presentes en ítems: las identificadas (`auto`/`manual`) de una película guardada, en esa película; las demás (pendientes, `unmatched`, o apuntando a una película que todavía no está guardada), por huella —`VersionKey` es la huella, o `id:<n>` sin ella—; `ignored` y `extra` quedan afuera, igual que las versiones sin ningún archivo principal presente. Cada ítem guarda, sin serializar, lo que miran las facetas. `ParseQuery` descarta parámetros desconocidos y valores mal formados (un valor bien formado que no coincide con nada se aplica igual y no encuentra nada). `Counts` cuenta cada faceta con **las demás** aplicadas. `Sort` deja los ítems sin año al final y desempata por título normalizado y luego por id, para que el orden sea estable entre pedidos.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/catalog/fixture_test.go`:

```go
package catalog

import (
	"cinexplorer/internal/probe"
	"cinexplorer/internal/store"
)

var roots = []string{"../cine", "../cine-ordenar"}

// version builds a present one-file version.
func version(id int64, dir, file, fp, title string, year int, res string, size, added int64) store.VersionView {
	return store.VersionView{ID: id, Dir: dir, Title: title, Year: year, Resolution: res, Size: size, Parts: 1,
		Added: added, Fingerprint: fp, Audio: []probe.Track{}, Subs: []probe.Track{},
		Files: []store.FileView{{Path: dir + "/" + file, Size: size, Role: "main"}}}
}

func identified(v store.VersionView, status string, tmdbID int) store.VersionView {
	v.Identification = &store.IdentView{Status: status, TMDBID: tmdbID, Confidence: 0.9}
	return v
}

// snapshot is a small catalog:
//   - Amarcord (7857): a 1080p version (best) and an SD version present twice
//     (identical copies), plus a making-of marked as its extra;
//   - Roma (7858): one version, in a TMDB collection;
//   - Novecento: unmatched; Stalker: never identified; a sample marked as not
//     a movie; a file not hashed yet; a version identified as a movie that is
//     not stored (yet); a version whose file is missing.
func snapshot() store.Snapshot {
	a1 := identified(version(1, "../cine/1970s/Amarcord", "Amarcord.1973.1080p.mkv", "a1", "Amarcord", 1973, "1080p", 9800, 100), store.StatusAuto, 7857)
	a1.Best = true
	a1.Subs = []probe.Track{{Codec: "srt", Lang: "en"}}
	a2 := identified(version(2, "../cine-ordenar", "Amarcord CD1.avi", "a2", "Amarcord", 0, "SD", 1400, 200), store.StatusManual, 7857)
	a2.SubLangs = "es,?"
	a3 := identified(version(3, "../cine/Collections/Fellini", "Amarcord CD1.avi", "a2", "Amarcord", 0, "SD", 1400, 300), store.StatusManual, 7857)
	a3.SubLangs = "es"
	extra := identified(version(4, "../cine/1970s/Amarcord/Extras", "Making of.avi", "e1", "Making of", 0, "", 350, 100), store.StatusExtra, 7857)
	roma := identified(version(5, "../cine/1970s/Roma", "Roma.mkv", "r1", "Roma", 1972, "1080p", 5000, 50), store.StatusAuto, 7858)
	nove := identified(version(6, "../cine/1970s", "Novecento.avi", "n1", "Novecento", 1976, "720p", 700, 400), store.StatusUnmatched, 0)
	nove.Director = "Bertolucci"
	stalker := version(7, "../cine-ordenar/Tarkovsky", "Stalker.avi", "s1", "Stalker", 1979, "576p", 800, 500)
	sample := identified(version(8, "../cine-ordenar", "sample.mkv", "x1", "sample", 0, "", 10, 500), store.StatusIgnored, 0)
	unhashed := version(9, "../cine-ordenar", "Vacío.avi", "", "Vacío", 0, "", 0, 600)
	solaris := identified(version(10, "../cine-ordenar", "Solaris.1972.avi", "p1", "Solaris", 1972, "", 900, 700), store.StatusAuto, 555)
	gone := version(11, "../cine/1980s", "Gone.avi", "g1", "Gone", 1985, "", 100, 10)
	gone.Files[0].Missing = true

	snap := store.Snapshot{
		Versions: []store.VersionView{a1, a2, a3, extra, roma, nove, stalker, sample, unhashed, solaris, gone},
		Identifications: map[string]*store.Identification{
			"n1": {Fingerprint: "n1", Status: store.StatusUnmatched, Candidates: []store.Candidate{{TMDBID: 1, Title: "Novecento", Year: 1976, Score: 0.7}}},
		},
		Movies: map[int]store.Movie{
			7857: {TMDBID: 7857, Title: "Amarcord", OriginalTitle: "Amarcord", Year: 1973, OriginalLang: "it",
				Directors: []store.Person{{ID: 4415, Name: "Federico Fellini"}}, Genres: []string{"Comedia", "Drama"},
				Countries: []string{"IT", "FR"}, PosterPath: "/pa.jpg"},
			7858: {TMDBID: 7858, Title: "Roma", OriginalTitle: "Roma", Year: 1972, OriginalLang: "it",
				Directors: []store.Person{{ID: 4415, Name: "Federico Fellini"}}, Genres: []string{"Drama"},
				Countries: []string{"IT"}, CollectionID: 99, Collection: "Fellini"},
		},
	}
	// The identification rows behind the versions' identity.
	for _, v := range snap.Versions {
		if id := v.Identification; id != nil && snap.Identifications[v.Fingerprint] == nil {
			snap.Identifications[v.Fingerprint] = &store.Identification{Fingerprint: v.Fingerprint, Status: id.Status, TMDBID: id.TMDBID}
		}
	}
	return snap
}
```

Crear `internal/catalog/items_test.go`:

```go
package catalog

import (
	"reflect"
	"testing"
)

func byID(items []Item) map[string]Item {
	out := map[string]Item{}
	for _, it := range items {
		out[it.id()] = it
	}
	return out
}

func TestItems(t *testing.T) {
	items := Items(snapshot(), roots)
	got := byID(items)
	want := []string{"movie:7857", "movie:7858", "n1", "s1", "id:9", "p1"}
	if len(items) != len(want) {
		t.Fatalf("items %v", got)
	}
	for _, id := range want {
		if _, ok := got[id]; !ok {
			t.Errorf("missing %s", id)
		}
	}

	a := got["movie:7857"]
	if a.Kind != KindMovie || a.Title != "Amarcord" || a.Year != 1973 || a.Versions != 3 || a.Size != 12600 ||
		a.Added != 100 || a.Resolution != "1080p" || a.Poster != "pa" ||
		!reflect.DeepEqual(a.Directors, []string{"Federico Fellini"}) || !reflect.DeepEqual(a.Countries, []string{"IT", "FR"}) {
		t.Errorf("amarcord %+v", a)
	}
	if !a.multiVersion || !a.identical || a.unidentified {
		t.Errorf("amarcord states: versions %v identical %v unidentified %v", a.multiVersion, a.identical, a.unidentified)
	}
	if !reflect.DeepEqual(a.resolutions, []string{"1080p", "SD"}) || !reflect.DeepEqual(a.subs, []string{"en", "es"}) {
		t.Errorf("amarcord resolutions %v subs %v", a.resolutions, a.subs)
	}
	if want := []string{"cine", "cine/1970s", "cine-ordenar", "cine/Collections"}; !reflect.DeepEqual(a.locations, want) {
		t.Errorf("amarcord locations %v", a.locations)
	}

	n := got["n1"]
	if n.Kind != KindVersion || n.Key != "n1" || n.Title != "Novecento" || n.Year != 1976 || n.Resolution != "720p" ||
		!n.unidentified || n.multiVersion || n.identical || !reflect.DeepEqual(n.Directors, []string{"Bertolucci"}) ||
		n.Poster != "" || len(n.Countries) != 0 {
		t.Errorf("novecento %+v", n)
	}
	// Identified as a movie that is not stored yet: shown as it is on disk.
	if p := got["p1"]; p.Kind != KindVersion || p.Title != "Solaris" || !p.unidentified {
		t.Errorf("solaris %+v", p)
	}
	if u := got["id:9"]; u.Key != "id:9" || u.Title != "Vacío" {
		t.Errorf("unhashed %+v", u)
	}
	if s := got["s1"]; s.Resolution != "SD" || !reflect.DeepEqual(s.locations, []string{"cine-ordenar", "cine-ordenar/Tarkovsky"}) {
		t.Errorf("stalker %+v %v", s, s.locations)
	}
}

func TestResolution(t *testing.T) {
	for in, want := range map[string]string{"2160p": "4K", "1080p": "1080p", "1080i": "1080p", "720p": "720p",
		"576p": "SD", "480p": "SD", "SD": "SD", "": ""} {
		if got := Resolution(in); got != want {
			t.Errorf("Resolution(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocations(t *testing.T) {
	cases := []struct {
		dir  string
		want []string
	}{
		{"../cine/1970s/Amarcord", []string{"cine", "cine/1970s"}},
		{"../cine", []string{"cine"}},
		{"../cine-ordenar/x", []string{"cine-ordenar", "cine-ordenar/x"}},
		{"../otro/x", nil},
	}
	for _, c := range cases {
		if got := locations(c.dir, roots); !reflect.DeepEqual(got, c.want) {
			t.Errorf("locations(%q) = %v, want %v", c.dir, got, c.want)
		}
	}
	if got := locations("../../media/cine/x", []string{"../../media/cine/"}); !reflect.DeepEqual(got, []string{"media/cine", "media/cine/x"}) {
		t.Errorf("deeper root: %v", got)
	}
}
```

Crear `internal/catalog/facets_test.go`:

```go
package catalog

import (
	"net/url"
	"reflect"
	"sort"
	"testing"
)

func ids(items []Item) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.id())
	}
	sort.Strings(out)
	return out
}

func TestParseQuery(t *testing.T) {
	cases := []struct {
		query string
		want  Query
	}{
		{"", Query{Facets: map[string]string{}, Order: OrderYear, Dir: Desc}},
		{"decada=1970&pais=IT&orden=titulo", Query{Facets: map[string]string{"decada": "1970", "pais": "IT"}, Order: OrderTitle, Dir: Asc}},
		{"orden=tamano&dir=asc&estado=copia-identica", Query{Facets: map[string]string{"estado": "copia-identica"}, Order: OrderSize, Dir: Asc}},
		// Malformed values and unknown parameters are dropped.
		{"decada=1975&anio=x&director=0&pais=it&idioma=Italian&resolucion=8K&estado=roto&genero=%20&foo=1&orden=nada&dir=up",
			Query{Facets: map[string]string{}, Order: OrderYear, Dir: Desc}},
		{"director=4415&coleccion=99&idioma=it&subs=spa&ubicacion=cine/1970s&resolucion=4K&genero=Drama&anio=1973",
			Query{Facets: map[string]string{"director": "4415", "coleccion": "99", "idioma": "it", "subs": "spa",
				"ubicacion": "cine/1970s", "resolucion": "4K", "genero": "Drama", "anio": "1973"}, Order: OrderYear, Dir: Desc}},
	}
	for _, c := range cases {
		v, _ := url.ParseQuery(c.query)
		if got := ParseQuery(v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %+v, want %+v", c.query, got, c.want)
		}
	}
}

func TestFilter(t *testing.T) {
	items := Items(snapshot(), roots)
	cases := []struct {
		facets map[string]string
		want   []string
	}{
		{map[string]string{}, []string{"id:9", "movie:7857", "movie:7858", "n1", "p1", "s1"}},
		{map[string]string{"decada": "1970"}, []string{"movie:7857", "movie:7858", "n1", "p1", "s1"}},
		{map[string]string{"anio": "1972"}, []string{"movie:7858", "p1"}},
		// TMDB facets only see identified movies.
		{map[string]string{"director": "4415"}, []string{"movie:7857", "movie:7858"}},
		{map[string]string{"genero": "Comedia"}, []string{"movie:7857"}},
		{map[string]string{"pais": "FR"}, []string{"movie:7857"}},
		{map[string]string{"idioma": "it"}, []string{"movie:7857", "movie:7858"}},
		{map[string]string{"coleccion": "99"}, []string{"movie:7858"}},
		// Any version counts: Amarcord has an SD version too.
		{map[string]string{"resolucion": "SD"}, []string{"movie:7857", "s1"}},
		{map[string]string{"subs": "es"}, []string{"movie:7857"}},
		{map[string]string{"ubicacion": "cine-ordenar"}, []string{"id:9", "movie:7857", "p1", "s1"}},
		{map[string]string{"ubicacion": "cine/1970s"}, []string{"movie:7857", "movie:7858", "n1"}},
		{map[string]string{"estado": "sin-identificar"}, []string{"id:9", "n1", "p1", "s1"}},
		{map[string]string{"estado": "varias-versiones"}, []string{"movie:7857"}},
		{map[string]string{"estado": "copia-identica"}, []string{"movie:7857"}},
		{map[string]string{"decada": "1970", "director": "4415", "resolucion": "1080p"}, []string{"movie:7857", "movie:7858"}},
		{map[string]string{"director": "1"}, []string{}},
	}
	for _, c := range cases {
		if got := ids(Filter(items, c.facets)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%v: got %v, want %v", c.facets, got, c.want)
		}
	}
}

func TestCounts(t *testing.T) {
	items := Items(snapshot(), roots)
	counts := Counts(items, map[string]string{"pais": "IT", "resolucion": "1080p"})
	// Each menu counts the items that match the other facets.
	if got, want := counts["resolucion"], []FacetValue{{"1080p", "1080p", 2}, {"SD", "SD", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("resolucion %v, want %v", got, want)
	}
	if got, want := counts["pais"], []FacetValue{{"IT", "IT", 2}, {"FR", "FR", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("pais %v, want %v", got, want)
	}
	if got, want := counts["director"], []FacetValue{{"4415", "Federico Fellini", 2}}; !reflect.DeepEqual(got, want) {
		t.Errorf("director %v, want %v", got, want)
	}
	if got, want := counts["coleccion"], []FacetValue{{"99", "Fellini", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("coleccion %v, want %v", got, want)
	}
	if got, want := counts["genero"], []FacetValue{{"Drama", "Drama", 2}, {"Comedia", "Comedia", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("genero %v, want %v", got, want)
	}

	all := Counts(items, map[string]string{})
	if got, want := all["decada"], []FacetValue{{"1970", "1970s", 5}}; !reflect.DeepEqual(got, want) {
		t.Errorf("decada %v, want %v", got, want)
	}
	if got, want := all["anio"], []FacetValue{{"1979", "1979", 1}, {"1976", "1976", 1}, {"1973", "1973", 1}, {"1972", "1972", 2}}; !reflect.DeepEqual(got, want) {
		t.Errorf("anio %v, want %v", got, want)
	}
	if got, want := all["ubicacion"], []FacetValue{{"cine", "cine", 3}, {"cine-ordenar", "cine-ordenar", 4},
		{"cine-ordenar/Tarkovsky", "cine-ordenar/Tarkovsky", 1}, {"cine/1970s", "cine/1970s", 3}, {"cine/Collections", "cine/Collections", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("ubicacion %v, want %v", got, want)
	}
	if got, want := all["estado"], []FacetValue{{"sin-identificar", "sin-identificar", 4}, {"copia-identica", "copia-identica", 1},
		{"varias-versiones", "varias-versiones", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("estado %v, want %v", got, want)
	}
	if len(all) != len(FacetNames) {
		t.Errorf("facets %d, want %d", len(all), len(FacetNames))
	}
	for _, f := range FacetNames {
		if all[f] == nil {
			t.Errorf("%s: nil instead of an empty list", f)
		}
	}
}

func TestSort(t *testing.T) {
	items := Items(snapshot(), roots)
	order := func(o, dir string) []string {
		Sort(items, o, dir)
		out := []string{}
		for _, it := range items {
			out = append(out, it.id())
		}
		return out
	}
	cases := []struct {
		order, dir string
		want       []string
	}{
		// Without a year last; Roma and Solaris tie on 1972 and go by title.
		{OrderYear, "", []string{"s1", "n1", "movie:7857", "movie:7858", "p1", "id:9"}},
		{OrderYear, Asc, []string{"movie:7858", "p1", "movie:7857", "n1", "s1", "id:9"}},
		{OrderTitle, "", []string{"movie:7857", "n1", "movie:7858", "p1", "s1", "id:9"}},
		{OrderTitle, Desc, []string{"id:9", "s1", "p1", "movie:7858", "n1", "movie:7857"}},
		{OrderAdded, "", []string{"p1", "id:9", "s1", "n1", "movie:7857", "movie:7858"}},
		{OrderSize, "", []string{"movie:7857", "movie:7858", "p1", "s1", "n1", "id:9"}},
	}
	for _, c := range cases {
		if got := order(c.order, c.dir); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %s: got %v, want %v", c.order, c.dir, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/catalog`
Expected: FAIL de compilación: `undefined: Items`, `undefined: Item`, etc.

- [ ] **Step 3: Implementar**

Crear `internal/catalog/items.go`:

```go
// Package catalog turns a snapshot of the store into what the pages show:
// the items of Explorar with their facets, movie and version details, and
// duplicates. Everything here is pure: no SQL, no network.
package catalog

import (
	"path"
	"strconv"
	"strings"

	"cinexplorer/internal/images"
	"cinexplorer/internal/quality"
	"cinexplorer/internal/store"
)

// Item kinds.
const (
	KindMovie   = "movie"   // an identified movie, with all its versions
	KindVersion = "version" // content not identified (yet): one per fingerprint
)

// Item is one card of Explorar.
type Item struct {
	Kind          string   `json:"kind"`
	TMDBID        int      `json:"tmdbId,omitempty"` // KindMovie
	Key           string   `json:"key,omitempty"`    // KindVersion: fingerprint, or "id:<version id>" without one
	Title         string   `json:"title"`
	OriginalTitle string   `json:"originalTitle"`
	Year          int      `json:"year"`
	Directors     []string `json:"directors"`
	Countries     []string `json:"countries"`
	Poster        string   `json:"poster"`     // image version for /img/poster/<id>.jpg?v=, "" without poster
	Resolution    string   `json:"resolution"` // the best one: "4K", "1080p", "720p", "SD" or ""
	Size          int64    `json:"size"`       // all its present versions
	Added         int64    `json:"added"`      // unix ms
	Versions      int      `json:"versions"`

	// What the facets look at.
	directorIDs  []int
	genres       []string
	lang         string
	collectionID int
	collection   string
	resolutions  []string
	subs         []string
	locations    []string
	unidentified bool
	multiVersion bool   // two or more different contents
	identical    bool   // the same content in two or more places
	norm         string // title as compared when sorting
}

// entry is an item with the versions it stands for.
type entry struct {
	item     Item
	versions []*store.VersionView
}

// build groups the present versions into items: identified versions of a
// stored movie into that movie; not identified ones (never identified,
// unmatched, or pointing to a movie that is not stored) by fingerprint.
// Versions that are not movies or are extras are left out, as are versions
// whose main files are all missing.
func build(snap store.Snapshot, roots []string) []*entry {
	var out []*entry
	byKey := map[string]*entry{}
	for i := range snap.Versions {
		v := &snap.Versions[i]
		if !hasPresentMain(v) {
			continue
		}
		var key string
		var movie *store.Movie
		switch id := v.Identification; {
		case id != nil && (id.Status == store.StatusIgnored || id.Status == store.StatusExtra):
			continue
		case id != nil && (id.Status == store.StatusAuto || id.Status == store.StatusManual):
			if m, ok := snap.Movies[id.TMDBID]; ok {
				movie, key = &m, "movie:"+strconv.Itoa(m.TMDBID)
			}
		}
		if key == "" {
			key = VersionKey(v)
		}
		e := byKey[key]
		if e == nil {
			e = &entry{item: newItem(v, movie)}
			byKey[key] = e
			out = append(out, e)
		}
		e.versions = append(e.versions, v)
	}
	for _, e := range out {
		e.finish(roots)
	}
	return out
}

// VersionKey is how a not identified version is addressed: its fingerprint,
// or its id while it has none (not hashed yet, or an empty file).
func VersionKey(v *store.VersionView) string {
	if v.Fingerprint != "" {
		return v.Fingerprint
	}
	return "id:" + strconv.FormatInt(v.ID, 10)
}

func newItem(v *store.VersionView, m *store.Movie) Item {
	if m == nil {
		it := Item{Kind: KindVersion, Key: VersionKey(v), Title: v.Title, Year: v.Year,
			Directors: []string{}, Countries: []string{}, unidentified: true}
		if v.Director != "" {
			it.Directors = []string{v.Director}
		}
		return it
	}
	it := Item{Kind: KindMovie, TMDBID: m.TMDBID, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: m.Year,
		Directors: []string{}, Countries: m.Countries, Poster: images.Version(m.PosterPath),
		genres: m.Genres, lang: m.OriginalLang, collectionID: m.CollectionID, collection: m.Collection}
	for _, d := range m.Directors {
		it.Directors = append(it.Directors, d.Name)
		it.directorIDs = append(it.directorIDs, d.ID)
	}
	if it.Countries == nil {
		it.Countries = []string{}
	}
	return it
}

// finish computes what depends on all the versions of the item.
func (e *entry) finish(roots []string) {
	it := &e.item
	it.norm = quality.NormTitle(it.Title)
	copies := map[string]int{}
	contents := 0
	for _, v := range e.versions {
		it.Size += v.Size
		if v.Added > 0 && (it.Added == 0 || v.Added < it.Added) {
			it.Added = v.Added
		}
		if r := Resolution(v.Resolution); r != "" {
			it.resolutions = appendNew(it.resolutions, r)
			if resRank[r] > resRank[it.Resolution] {
				it.Resolution = r
			}
		}
		for _, l := range subLangs(v) {
			it.subs = appendNew(it.subs, l)
		}
		for _, l := range locations(v.Dir, roots) {
			it.locations = appendNew(it.locations, l)
		}
		if v.Fingerprint == "" {
			contents++
		} else {
			if copies[v.Fingerprint] == 0 {
				contents++
			}
			copies[v.Fingerprint]++
		}
	}
	it.Versions = len(e.versions)
	it.multiVersion = contents > 1
	for _, n := range copies {
		if n > 1 {
			it.identical = true
		}
	}
}

// resRank orders the resolution labels of Explorar.
var resRank = map[string]int{"4K": 4, "1080p": 3, "720p": 2, "SD": 1}

// Resolution maps a version's resolution ("2160p", "1080i", "576p"…) to the
// labels of Explorar.
func Resolution(r string) string {
	switch r {
	case "":
		return ""
	case "2160p":
		return "4K"
	case "1080p", "1080i":
		return "1080p"
	case "720p":
		return "720p"
	}
	return "SD"
}

// subLangs lists the subtitle languages of a version: embedded tracks and
// external files, known languages only.
func subLangs(v *store.VersionView) []string {
	var out []string
	for _, t := range v.Subs {
		if t.Lang != "" {
			out = appendNew(out, t.Lang)
		}
	}
	for _, l := range strings.Split(v.SubLangs, ",") {
		if l != "" && l != "?" {
			out = appendNew(out, l)
		}
	}
	return out
}

// locations gives the Ubicación values of a folder: the root it is under
// and, below the root, its first-level folder ("cine" and "cine/1970s" for
// "../cine/1970s/Amarcord"). Roots are shown without their leading "../".
func locations(dir string, roots []string) []string {
	for _, r := range roots {
		r = strings.TrimSuffix(r, "/")
		if dir != r && !strings.HasPrefix(dir, r+"/") {
			continue
		}
		label := rootLabel(r)
		out := []string{label}
		if rest := strings.TrimPrefix(dir, r+"/"); dir != r {
			first, _, _ := strings.Cut(rest, "/")
			out = append(out, label+"/"+first)
		}
		return out
	}
	return nil
}

func rootLabel(root string) string {
	for strings.HasPrefix(root, "../") {
		root = strings.TrimPrefix(root, "../")
	}
	return path.Clean(root)
}

func hasPresentMain(v *store.VersionView) bool {
	for _, f := range v.Files {
		if f.Role == "main" && !f.Missing {
			return true
		}
	}
	return false
}

func appendNew(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// Items returns every item of the catalog, sorted by year, newest first.
func Items(snap store.Snapshot, roots []string) []Item {
	es := build(snap, roots)
	out := make([]Item, len(es))
	for i, e := range es {
		out[i] = e.item
	}
	Sort(out, OrderYear, "")
	return out
}
```

Crear `internal/catalog/facets.go`:

```go
package catalog

import (
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Facet names, as they appear in Explorar's URL.
const (
	FacetDecade     = "decada"
	FacetYear       = "anio"
	FacetDirector   = "director"   // TMDB person id
	FacetGenre      = "genero"     // localized name
	FacetCountry    = "pais"       // ISO 3166-1 alpha-2
	FacetLanguage   = "idioma"     // original language, ISO 639-1
	FacetCollection = "coleccion"  // TMDB collection id
	FacetResolution = "resolucion" // "4K", "1080p", "720p", "SD"
	FacetSubs       = "subs"       // subtitle language
	FacetLocation   = "ubicacion"  // "cine" or "cine/1970s"
	FacetState      = "estado"     // StateUnidentified, StateVersions or StateIdentical
)

// FacetNames lists the facets in the order of Explorar's bar.
var FacetNames = []string{FacetDecade, FacetYear, FacetDirector, FacetGenre, FacetCountry, FacetLanguage,
	FacetCollection, FacetResolution, FacetSubs, FacetLocation, FacetState}

// Values of FacetState.
const (
	StateUnidentified = "sin-identificar"
	StateVersions     = "varias-versiones"
	StateIdentical    = "copia-identica"
)

// Sort orders and directions.
const (
	OrderYear  = "anio"
	OrderTitle = "titulo"
	OrderAdded = "agregado"
	OrderSize  = "tamano"
	Asc        = "asc"
	Desc       = "desc"
)

// Query is what Explorar asks for: one value per facet, and an order.
type Query struct {
	Facets map[string]string `json:"facets"`
	Order  string            `json:"order"`
	Dir    string            `json:"dir"`
}

var (
	numberRe  = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
	langRe    = regexp.MustCompile(`^[a-z]{2,3}$`)
)

// valid reports whether value is well formed for a facet. A well formed value
// that matches nothing is still applied (and finds nothing).
func valid(facet, value string) bool {
	switch facet {
	case FacetDecade:
		n, err := strconv.Atoi(value)
		return err == nil && numberRe.MatchString(value) && n%10 == 0
	case FacetYear, FacetDirector, FacetCollection:
		return numberRe.MatchString(value)
	case FacetCountry:
		return countryRe.MatchString(value)
	case FacetLanguage, FacetSubs:
		return langRe.MatchString(value)
	case FacetResolution:
		_, ok := resRank[value]
		return ok
	case FacetState:
		return value == StateUnidentified || value == StateVersions || value == StateIdentical
	case FacetGenre, FacetLocation:
		return strings.TrimSpace(value) != ""
	}
	return false
}

// ParseQuery reads Explorar's parameters. Unknown parameters and malformed
// values are dropped; the order defaults to year, newest first, and each
// order has its natural direction (A to Z for titles, largest or newest
// first otherwise).
func ParseQuery(v url.Values) Query {
	q := Query{Facets: map[string]string{}, Order: OrderYear}
	for _, f := range FacetNames {
		if val := v.Get(f); valid(f, val) {
			q.Facets[f] = val
		}
	}
	switch o := v.Get("orden"); o {
	case OrderTitle, OrderAdded, OrderSize:
		q.Order = o
	}
	q.Dir = Desc
	if q.Order == OrderTitle {
		q.Dir = Asc
	}
	if d := v.Get("dir"); d == Asc || d == Desc {
		q.Dir = d
	}
	return q
}

// values lists an item's values for a facet.
func (it *Item) values(facet string) []string {
	switch facet {
	case FacetDecade:
		if it.Year > 0 {
			return []string{strconv.Itoa(it.Year / 10 * 10)}
		}
	case FacetYear:
		if it.Year > 0 {
			return []string{strconv.Itoa(it.Year)}
		}
	case FacetDirector:
		out := make([]string, len(it.directorIDs))
		for i, id := range it.directorIDs {
			out[i] = strconv.Itoa(id)
		}
		return out
	case FacetGenre:
		return it.genres
	case FacetCountry:
		if it.Kind == KindMovie {
			return it.Countries
		}
	case FacetLanguage:
		if it.lang != "" {
			return []string{it.lang}
		}
	case FacetCollection:
		if it.collectionID > 0 {
			return []string{strconv.Itoa(it.collectionID)}
		}
	case FacetResolution:
		return it.resolutions
	case FacetSubs:
		return it.subs
	case FacetLocation:
		return it.locations
	case FacetState:
		var out []string
		if it.unidentified {
			out = append(out, StateUnidentified)
		}
		if it.multiVersion {
			out = append(out, StateVersions)
		}
		if it.identical {
			out = append(out, StateIdentical)
		}
		return out
	}
	return nil
}

func (it *Item) matches(facets map[string]string, except string) bool {
	for f, want := range facets {
		if f == except {
			continue
		}
		found := false
		for _, v := range it.values(f) {
			if v == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Filter keeps the items that match every facet.
func Filter(items []Item, facets map[string]string) []Item {
	out := []Item{}
	for i := range items {
		if items[i].matches(facets, "") {
			out = append(out, items[i])
		}
	}
	return out
}

// FacetValue is one option of a facet's menu.
type FacetValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// Counts lists, for every facet, the values found in the items that match
// the other facets, with how many items have each. So a menu shows what
// choosing a value in it would give. Years, decades and locations come in
// their natural order; the rest, most frequent first.
func Counts(items []Item, facets map[string]string) map[string][]FacetValue {
	out := map[string][]FacetValue{}
	for _, f := range FacetNames {
		counts := map[string]int{}
		labels := map[string]string{}
		for i := range items {
			it := &items[i]
			if !it.matches(facets, f) {
				continue
			}
			for _, v := range it.values(f) {
				counts[v]++
				if _, ok := labels[v]; !ok {
					labels[v] = it.label(f, v)
				}
			}
		}
		vs := make([]FacetValue, 0, len(counts))
		for v, n := range counts {
			vs = append(vs, FacetValue{Value: v, Label: labels[v], Count: n})
		}
		sort.Slice(vs, func(i, j int) bool {
			switch f {
			case FacetDecade, FacetYear:
				return vs[i].Value > vs[j].Value
			case FacetLocation:
				return vs[i].Value < vs[j].Value
			}
			if vs[i].Count != vs[j].Count {
				return vs[i].Count > vs[j].Count
			}
			return vs[i].Label < vs[j].Label
		})
		out[f] = vs
	}
	return out
}

// label is how a facet value reads: names for ids, "1970s" for a decade,
// the value itself otherwise (countries and languages are named by the page).
func (it *Item) label(facet, value string) string {
	switch facet {
	case FacetDecade:
		return value + "s"
	case FacetDirector:
		for i, id := range it.directorIDs {
			if strconv.Itoa(id) == value {
				return it.Directors[i]
			}
		}
	case FacetCollection:
		return it.collection
	}
	return value
}
```

Crear `internal/catalog/sort.go`:

```go
package catalog

import (
	"cmp"
	"slices"
	"strconv"
)

// Sort orders items by year, title, date added or size, in dir (Asc or
// Desc; "" is the order's natural direction). Items without a year go last
// either way. Ties are broken by title and then by id, so the order is
// stable between requests.
func Sort(items []Item, order, dir string) {
	if dir == "" {
		dir = Desc
		if order == OrderTitle {
			dir = Asc
		}
	}
	slices.SortStableFunc(items, func(a, b Item) int {
		if order == OrderYear && (a.Year == 0) != (b.Year == 0) {
			if a.Year == 0 {
				return 1
			}
			return -1
		}
		var c int
		switch order {
		case OrderTitle:
			c = cmp.Compare(a.norm, b.norm)
		case OrderAdded:
			c = cmp.Compare(a.Added, b.Added)
		case OrderSize:
			c = cmp.Compare(a.Size, b.Size)
		default:
			c = cmp.Compare(a.Year, b.Year)
		}
		if dir == Desc {
			c = -c
		}
		if c != 0 {
			return c
		}
		if c = cmp.Compare(a.norm, b.norm); c != 0 {
			return c
		}
		return cmp.Compare(a.id(), b.id())
	})
}

// id tells items apart: a movie's TMDB id or a version's key.
func (it *Item) id() string {
	if it.Kind == KindMovie {
		return "movie:" + strconv.Itoa(it.TMDBID)
	}
	return it.Key
}
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l internal/catalog; go vet ./internal/catalog && go test ./internal/catalog`
Expected: `gofmt` sin salida; `ok  cinexplorer/internal/catalog`.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog
git commit -m "feat(catalog): Explorar items, facets and sorting"
```

---

### Task 6: Catálogo: duplicados

**Files:**
- Create: `internal/catalog/duplicates.go`
- Test: `internal/catalog/duplicates_test.go`

Spec §5.3 (Duplicados). Sobre los mismos ítems de Explorar: los que tienen copias idénticas (misma huella en ≥ 2 versiones presentes) o varias versiones (≥ 2 contenidos distintos de una película). Recuperable: cada copia de más de un contenido, más —en varias versiones— los contenidos que no son el de la mejor versión (la marcada por `markBest`; si no hay, la más grande), cada huella una sola vez. Grupos de mayor a menor recuperable; dentro de cada uno, la mejor versión primero.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/catalog/duplicates_test.go`:

```go
package catalog

import (
	"reflect"
	"testing"

	"cinexplorer/internal/store"
)

func TestDuplicates(t *testing.T) {
	snap := snapshot()
	// Stalker, not identified, is also in another folder.
	snap.Versions = append(snap.Versions, version(12, "../cine/1970s/Stalker", "Stalker.avi", "s1", "Stalker", 1979, "576p", 800, 500))
	rep := Duplicates(snap, roots)
	if len(rep.Groups) != 2 {
		t.Fatalf("groups %+v", rep.Groups)
	}
	a, s := rep.Groups[0], rep.Groups[1]
	// Amarcord: one more copy of the SD content (1400) plus the SD content,
	// which is not the best version (1400).
	if a.TMDBID != 7857 || a.Kind != KindMovie || a.Recoverable != 2800 ||
		!reflect.DeepEqual(a.Types, []string{DupIdentical, DupVersions}) {
		t.Errorf("amarcord %+v", a)
	}
	if len(a.Versions) != 3 || !a.Versions[0].Best || a.Versions[0].Path != "../cine/1970s/Amarcord/Amarcord.1973.1080p.mkv" ||
		a.Versions[1].Fingerprint != "a2" || a.Versions[2].Fingerprint != "a2" {
		t.Errorf("amarcord versions %+v", a.Versions)
	}
	if s.Key != "s1" || s.Kind != KindVersion || s.Recoverable != 800 || !reflect.DeepEqual(s.Types, []string{DupIdentical}) || len(s.Versions) != 2 {
		t.Errorf("stalker %+v", s)
	}
	if rep.Recoverable != 3600 {
		t.Errorf("total %d", rep.Recoverable)
	}
}

func TestDuplicatesNone(t *testing.T) {
	rep := Duplicates(store.Snapshot{}, roots)
	if rep.Groups == nil || len(rep.Groups) != 0 || rep.Recoverable != 0 {
		t.Fatalf("report %+v", rep)
	}
}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/catalog`
Expected: FAIL de compilación: `undefined: Duplicates`.

- [ ] **Step 3: Implementar**

Crear `internal/catalog/duplicates.go`:

```go
package catalog

import (
	"cmp"
	"slices"

	"cinexplorer/internal/store"
)

// Duplicate types.
const (
	DupIdentical = "identical" // the same content (fingerprint) in two or more places
	DupVersions  = "versions"  // different contents of the same movie
)

// DupVersion is a version as listed in Duplicados.
type DupVersion struct {
	ID          int64  `json:"id"`
	Fingerprint string `json:"fingerprint"`
	Resolution  string `json:"resolution"`
	Size        int64  `json:"size"`
	Path        string `json:"path"` // its first present main file
	Best        bool   `json:"best"`
}

// DupGroup is a movie (or unidentified content) with duplicates.
type DupGroup struct {
	Types       []string     `json:"types"`
	Kind        string       `json:"kind"`
	TMDBID      int          `json:"tmdbId,omitempty"`
	Key         string       `json:"key,omitempty"`
	Title       string       `json:"title"`
	Year        int          `json:"year"`
	Recoverable int64        `json:"recoverable"`
	Versions    []DupVersion `json:"versions"`
}

type DupReport struct {
	Recoverable int64      `json:"recoverable"`
	Groups      []DupGroup `json:"groups"`
}

// Duplicates lists the items with identical copies or several versions, the
// most space recoverable first. Recoverable space is what deleting all but
// one copy of each content would free, plus, for several versions of a
// movie, the contents other than the best one.
func Duplicates(snap store.Snapshot, roots []string) DupReport {
	rep := DupReport{Groups: []DupGroup{}}
	for _, e := range build(snap, roots) {
		it := &e.item
		if !it.identical && !it.multiVersion {
			continue
		}
		g := DupGroup{Types: []string{}, Kind: it.Kind, TMDBID: it.TMDBID, Key: it.Key, Title: it.Title, Year: it.Year}
		var keep *store.VersionView // the version worth keeping
		for _, v := range e.versions {
			if keep == nil || v.Best || (!keep.Best && v.Size > keep.Size) {
				keep = v
			}
		}
		if it.identical {
			g.Types = append(g.Types, DupIdentical)
		}
		if it.multiVersion {
			g.Types = append(g.Types, DupVersions)
		}
		counted := map[string]bool{}
		for _, v := range e.versions {
			g.Versions = append(g.Versions, DupVersion{ID: v.ID, Fingerprint: v.Fingerprint, Resolution: v.Resolution,
				Size: v.Size, Path: firstMain(v), Best: v.Best})
			if v.Fingerprint != "" && counted[v.Fingerprint] {
				g.Recoverable += v.Size // one more copy of a content already counted
				continue
			}
			counted[v.Fingerprint] = v.Fingerprint != ""
			if it.multiVersion && !sameContent(v, keep) {
				g.Recoverable += v.Size
			}
		}
		slices.SortStableFunc(g.Versions, func(a, b DupVersion) int {
			if a.Best != b.Best {
				if a.Best {
					return -1
				}
				return 1
			}
			return cmp.Compare(b.Size, a.Size)
		})
		rep.Recoverable += g.Recoverable
		rep.Groups = append(rep.Groups, g)
	}
	slices.SortStableFunc(rep.Groups, func(a, b DupGroup) int {
		if c := cmp.Compare(b.Recoverable, a.Recoverable); c != 0 {
			return c
		}
		return cmp.Compare(a.Title, b.Title)
	})
	return rep
}

// sameContent reports whether two versions hold the same content.
func sameContent(a, b *store.VersionView) bool {
	if a.Fingerprint == "" || b.Fingerprint == "" {
		return a == b
	}
	return a.Fingerprint == b.Fingerprint
}

func firstMain(v *store.VersionView) string {
	for _, f := range v.Files {
		if f.Role == "main" && !f.Missing {
			return f.Path
		}
	}
	return ""
}
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l internal/catalog; go vet ./internal/catalog && go test ./internal/catalog`
Expected: sin salida de `gofmt`; `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog
git commit -m "feat(catalog): duplicates with recoverable space"
```

---

### Task 7: Catálogo: fichas, sugerencias y cola de sin identificar

**Files:**
- Create: `internal/catalog/detail.go`
- Test: `internal/catalog/detail_test.go`

Spec §6. `Movie` arma la ficha de una película guardada: sus versiones con `copies` (versiones presentes con esa huella), la mejor primero y las que faltan al final; los archivos extra de sus versiones y las versiones corregidas como "extra de" ella. `Version` arma la ficha provisoria de una clave (`VersionKey`), con la identificación y sus candidatos. `SuggestMovies` sirve al selector de "es un extra de…": primero las películas con alguna versión en la carpeta de la clave `near` o en su carpeta padre, después el resto, por título; `q` filtra por título u original normalizados. `Unidentified` es la cola de Revisar (una entrada por huella `unmatched`, con sus candidatos) y cuántas huellas presentes todavía no tienen fila de identificación (`pending`).

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/catalog/detail_test.go`:

```go
package catalog

import (
	"testing"

	"cinexplorer/internal/store"
)

func TestMovie(t *testing.T) {
	snap := snapshot()
	// A version of Amarcord whose file is gone, and a trailer found with the
	// best version.
	gone := identified(version(13, "../cine/old", "Amarcord.avi", "a9", "Amarcord", 1973, "2160p", 20000, 10), store.StatusAuto, 7857)
	gone.Files[0].Missing = true
	snap.Versions = append(snap.Versions, gone)
	snap.Versions[0].Files = append(snap.Versions[0].Files, store.FileView{Path: "../cine/1970s/Amarcord/trailer.mkv", Size: 30, Role: "extra"})

	d, ok := Movie(snap, 7857)
	if !ok || d.Movie.Title != "Amarcord" || d.Poster != "pa" || d.Backdrop != "" {
		t.Fatalf("movie %+v %v", d, ok)
	}
	var got []int64
	for _, v := range d.Versions {
		got = append(got, v.ID)
	}
	// Best first; the missing 4K last; the two SD copies in between.
	if len(got) != 4 || got[0] != 1 || got[3] != 13 || d.Versions[1].Copies != 2 || d.Versions[0].Copies != 1 || d.Versions[3].Copies != 0 {
		t.Fatalf("versions %v %+v", got, d.Versions)
	}
	if len(d.Extras) != 1 || d.Extras[0].Path != "../cine/1970s/Amarcord/trailer.mkv" || d.Extras[0].Size != 30 {
		t.Errorf("extras %+v", d.Extras)
	}
	if len(d.ExtraVersions) != 1 || d.ExtraVersions[0].ID != 4 {
		t.Errorf("extra versions %+v", d.ExtraVersions)
	}
	if _, ok := Movie(snap, 555); ok {
		t.Error("movie not stored found")
	}
}

func TestVersion(t *testing.T) {
	snap := snapshot()
	d, ok := Version(snap, "n1")
	if !ok || d.Title != "Novecento" || d.Year != 1976 || len(d.Versions) != 1 || d.Movie != nil ||
		d.Identification == nil || d.Identification.Status != store.StatusUnmatched || len(d.Identification.Candidates) != 1 {
		t.Fatalf("novecento %+v %v", d, ok)
	}
	if d, ok := Version(snap, "s1"); !ok || d.Identification != nil {
		t.Errorf("never identified %+v", d)
	}
	if d, ok := Version(snap, "a2"); !ok || len(d.Versions) != 2 || d.Versions[0].Copies != 2 {
		t.Errorf("copies %+v", d)
	}
	if d, ok := Version(snap, "id:9"); !ok || d.Title != "Vacío" || d.Identification != nil {
		t.Errorf("unhashed %+v", d)
	}
	for _, k := range []string{"zz", "id:99", ""} {
		if _, ok := Version(snap, k); ok {
			t.Errorf("%q found", k)
		}
	}
}

func TestSuggestMovies(t *testing.T) {
	snap := snapshot()
	titles := func(refs []store.MovieRef) []string {
		out := []string{}
		for _, r := range refs {
			out = append(out, r.Title)
		}
		return out
	}
	// The making-of lives in Amarcord/Extras: Amarcord comes first.
	snap.Versions[3].Identification = nil
	if got := titles(SuggestMovies(snap, "", "e1")); len(got) != 2 || got[0] != "Amarcord" || got[1] != "Roma" {
		t.Errorf("near e1: %v", got)
	}
	// A trailer in Roma/Extras puts Roma first.
	snap.Versions = append(snap.Versions, version(14, "../cine/1970s/Roma/Extras", "Trailer.avi", "t1", "Trailer", 0, "", 5, 1))
	if got := titles(SuggestMovies(snap, "", "t1")); len(got) != 2 || got[0] != "Roma" {
		t.Errorf("near t1: %v", got)
	}
	if got := titles(SuggestMovies(snap, "ROMÁ", "")); len(got) != 1 || got[0] != "Roma" {
		t.Errorf("q: %v", got)
	}
	if got := SuggestMovies(snap, "nada", ""); got == nil || len(got) != 0 {
		t.Errorf("no match: %v", got)
	}
}

func TestUnidentified(t *testing.T) {
	snap := snapshot()
	// A copy of Novecento elsewhere: still one item.
	snap.Versions = append(snap.Versions, identified(version(15, "../cine-ordenar", "Novecento.avi", "n1", "Novecento", 1976, "720p", 700, 400), store.StatusUnmatched, 0))
	rep := Unidentified(snap)
	// Waiting: only Stalker; missing and unhashed files do not count.
	if rep.Pending != 1 {
		t.Errorf("pending %d", rep.Pending)
	}
	if len(rep.Items) != 1 || rep.Items[0].Fingerprint != "n1" || len(rep.Items[0].Candidates) != 1 || rep.Items[0].ID != 6 {
		t.Fatalf("items %+v", rep.Items)
	}
}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/catalog`
Expected: FAIL de compilación: `undefined: Movie`, `undefined: Version`, `undefined: SuggestMovies`, `undefined: Unidentified`.

- [ ] **Step 3: Implementar**

Crear `internal/catalog/detail.go`:

```go
package catalog

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"cinexplorer/internal/images"
	"cinexplorer/internal/quality"
	"cinexplorer/internal/store"
)

// VersionCard is a version as shown in a movie's page.
type VersionCard struct {
	store.VersionView
	Copies int `json:"copies"` // present versions with this content, itself included
}

// ExtraFile is a bonus file (making-of, trailer…) found with a version.
type ExtraFile struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Missing bool   `json:"missing"`
}

// MovieDetail is a movie's page.
type MovieDetail struct {
	Movie         store.Movie   `json:"movie"`
	Poster        string        `json:"poster"`   // image versions (see images.Version)
	Backdrop      string        `json:"backdrop"` //
	Versions      []VersionCard `json:"versions"`
	Extras        []ExtraFile   `json:"extras"`
	ExtraVersions []VersionCard `json:"extraVersions"` // versions corrected as extras of this movie
}

// Movie returns the page of a stored movie: its versions (the best first,
// missing ones last), the extra files found with them, and the versions the
// user marked as its extras.
func Movie(snap store.Snapshot, id int) (MovieDetail, bool) {
	m, ok := snap.Movies[id]
	if !ok {
		return MovieDetail{}, false
	}
	d := MovieDetail{Movie: m, Poster: images.Version(m.PosterPath), Backdrop: images.Version(m.BackdropPath),
		Versions: []VersionCard{}, Extras: []ExtraFile{}, ExtraVersions: []VersionCard{}}
	copies := copyCounts(snap)
	for i := range snap.Versions {
		v := &snap.Versions[i]
		ident := v.Identification
		if ident == nil || ident.TMDBID != id {
			continue
		}
		switch ident.Status {
		case store.StatusAuto, store.StatusManual:
			d.Versions = append(d.Versions, VersionCard{VersionView: *v, Copies: copies[v.Fingerprint]})
			for _, f := range v.Files {
				if f.Role == "extra" {
					d.Extras = append(d.Extras, ExtraFile{Path: f.Path, Size: f.Size, Missing: f.Missing})
				}
			}
		case store.StatusExtra:
			d.ExtraVersions = append(d.ExtraVersions, VersionCard{VersionView: *v, Copies: copies[v.Fingerprint]})
		}
	}
	sortCards(d.Versions)
	return d, true
}

// copyCounts counts the present versions of each content.
func copyCounts(snap store.Snapshot) map[string]int {
	out := map[string]int{}
	for i := range snap.Versions {
		if v := &snap.Versions[i]; v.Fingerprint != "" && hasPresentMain(v) {
			out[v.Fingerprint]++
		}
	}
	return out
}

// sortCards puts the best version first, then present before missing, then
// higher resolutions and larger sizes.
func sortCards(cs []VersionCard) {
	slices.SortStableFunc(cs, func(a, b VersionCard) int {
		if a.Best != b.Best {
			if a.Best {
				return -1
			}
			return 1
		}
		pa, pb := hasPresentMain(&a.VersionView), hasPresentMain(&b.VersionView)
		if pa != pb {
			if pa {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(resRank[Resolution(b.Resolution)], resRank[Resolution(a.Resolution)]); c != 0 {
			return c
		}
		return cmp.Compare(b.Size, a.Size)
	})
}

// IdentityView is what is known about a version's identity.
type IdentityView struct {
	Status     string            `json:"status"`
	TMDBID     int               `json:"tmdbId"`
	Confidence float64           `json:"confidence"`
	Candidates []store.Candidate `json:"candidates"`
}

// VersionDetail is the page of content that is not (or not yet) a movie of
// the catalog: the versions with one fingerprint, or one version without
// fingerprint.
type VersionDetail struct {
	Key            string          `json:"key"`
	Title          string          `json:"title"`
	Year           int             `json:"year"`
	Versions       []VersionCard   `json:"versions"`
	Identification *IdentityView   `json:"identification"` // nil when never identified
	Movie          *store.MovieRef `json:"movie"`          // set when identified as a stored movie
}

// Version returns the page of a version key (see VersionKey).
func Version(snap store.Snapshot, key string) (VersionDetail, bool) {
	d := VersionDetail{Key: key, Versions: []VersionCard{}}
	copies := copyCounts(snap)
	for i := range snap.Versions {
		v := &snap.Versions[i]
		if VersionKey(v) != key {
			continue
		}
		if len(d.Versions) == 0 {
			d.Title, d.Year, d.Movie = v.Title, v.Year, v.Movie
		}
		d.Versions = append(d.Versions, VersionCard{VersionView: *v, Copies: copies[v.Fingerprint]})
	}
	if len(d.Versions) == 0 {
		return d, false
	}
	sortCards(d.Versions)
	if id := snap.Identifications[d.Versions[0].Fingerprint]; id != nil && d.Versions[0].Fingerprint != "" {
		cands := id.Candidates
		if cands == nil {
			cands = []store.Candidate{}
		}
		d.Identification = &IdentityView{Status: id.Status, TMDBID: id.TMDBID, Confidence: id.Confidence, Candidates: cands}
	}
	return d, true
}

// suggestLimit is how many movies SuggestMovies returns.
const suggestLimit = 20

// SuggestMovies lists catalog movies for "es un extra de…": those in the
// folder of the version key near or in its parent folder first, then the
// rest, by title. q, when set, keeps the movies whose title or original
// title contains it (ignoring case, accents and punctuation).
func SuggestMovies(snap store.Snapshot, q, near string) []store.MovieRef {
	dirs := map[string]bool{}
	for i := range snap.Versions {
		if v := &snap.Versions[i]; near != "" && VersionKey(v) == near {
			dirs[v.Dir], dirs[path.Dir(v.Dir)] = true, true
		}
	}
	nq := quality.NormTitle(q)
	type cand struct {
		ref   store.MovieRef
		near  bool
		title string
	}
	var cs []cand
	for _, e := range build(snap, nil) {
		if e.item.Kind != KindMovie {
			continue
		}
		m := snap.Movies[e.item.TMDBID]
		if nq != "" && !strings.Contains(quality.NormTitle(m.Title), nq) && !strings.Contains(quality.NormTitle(m.OriginalTitle), nq) {
			continue
		}
		c := cand{ref: store.MovieRef{TMDBID: m.TMDBID, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: m.Year,
			Directors: m.Directors}, title: e.item.norm}
		if c.ref.Directors == nil {
			c.ref.Directors = []store.Person{}
		}
		for _, v := range e.versions {
			if dirs[v.Dir] {
				c.near = true
			}
		}
		cs = append(cs, c)
	}
	slices.SortStableFunc(cs, func(a, b cand) int {
		if a.near != b.near {
			if a.near {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(a.title, b.title); c != 0 {
			return c
		}
		return cmp.Compare(a.ref.TMDBID, b.ref.TMDBID)
	})
	out := []store.MovieRef{}
	for _, c := range cs[:min(len(cs), suggestLimit)] {
		out = append(out, c.ref)
	}
	return out
}

// UnidentifiedItem is a version in the Sin identificar queue.
type UnidentifiedItem struct {
	store.VersionView
	Candidates []store.Candidate `json:"candidates"`
}

// UnidentifiedReport is the Sin identificar queue: the contents the matcher
// could not decide on (one per fingerprint, in title order), and how many
// are still waiting to be identified at all.
type UnidentifiedReport struct {
	Pending int                `json:"pending"`
	Items   []UnidentifiedItem `json:"items"`
}

// Unidentified returns the Sin identificar queue.
func Unidentified(snap store.Snapshot) UnidentifiedReport {
	rep := UnidentifiedReport{Items: []UnidentifiedItem{}}
	seen := map[string]bool{}
	for i := range snap.Versions {
		v := &snap.Versions[i]
		if v.Fingerprint == "" || seen[v.Fingerprint] || !hasPresentMain(v) {
			continue
		}
		seen[v.Fingerprint] = true
		switch id := snap.Identifications[v.Fingerprint]; {
		case id == nil:
			rep.Pending++
		case id.Status == store.StatusUnmatched:
			cands := id.Candidates
			if cands == nil {
				cands = []store.Candidate{}
			}
			rep.Items = append(rep.Items, UnidentifiedItem{VersionView: *v, Candidates: cands})
		}
	}
	return rep
}
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l internal/catalog; go vet ./internal/catalog && go test ./internal/catalog`
Expected: sin salida de `gofmt`; `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog
git commit -m "feat(catalog): movie and version pages, movie suggestions, review queue"
```

---

### Task 8: API de Explorar, fichas, duplicados y revisión

**Files:**
- Create: `internal/server/catalog.go`
- Modify: `internal/server/server.go`, `internal/server/identify.go`, `internal/store/identity.go`, `cmd/cinexplorer/main.go`
- Test: `internal/server/catalog_test.go`, `internal/server/server_test.go`, `internal/server/identify_test.go`, `internal/store/versions_identity_test.go`

Spec §6. Cada endpoint lee una instantánea (`Store.Snapshot`) y responde lo que arma `internal/catalog`. `Server.Roots` recibe las raíces de `config.json` (la faceta Ubicación las necesita). `/api/duplicates` pasa de copias a nivel de archivo a `catalog.Duplicates`, y `/api/unidentified` a `catalog.Unidentified` (`{pending, items}`), así que `store.Unidentified` desaparece. `store.Duplicates` queda: lo usan los tests del escáner.

La página mínima de las etapas 1–3 todavía está embebida y espera las formas viejas de `/api/unidentified` y `/api/duplicates`; la reemplaza la app nueva a partir de la Task 11.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/server/catalog_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
)

// addCopy puts an identical copy of Amarcord (fingerprint f1) in
// ../cine-ordenar, as a second version.
func addCopy(t *testing.T, s *Server) {
	t.Helper()
	s.Roots = []string{"../cine", "../cine-ordenar"}
	const copyPath = "../cine-ordenar/Amarcord.mkv"
	if err := s.Store.SyncFiles([]store.FileRow{
		{Path: moviePath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"},
		{Path: copyPath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"},
	}, s.Roots); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ReplaceVersions([]grouping.Version{
		{Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
			Members: []grouping.Member{{Path: moviePath, Role: grouping.RoleMain}}},
		{Dir: "../cine-ordenar", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
			Members: []grouping.Member{{Path: copyPath, Role: grouping.RoleMain}}},
	}); err != nil {
		t.Fatal(err)
	}
}

func getJSON(t *testing.T, s *Server, url string, v any) int {
	t.Helper()
	rec := request(s.Handler(), "GET", url, "", "", "127.0.0.1")
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("%s: %s (%v)", url, rec.Body, err)
		}
	}
	return rec.Code
}

type exploreBody struct {
	Total  int                             `json:"total"`
	Query  catalog.Query                   `json:"query"`
	Items  []catalog.Item                  `json:"items"`
	Facets map[string][]catalog.FacetValue `json:"facets"`
}

func TestExploreEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	addCopy(t, s)
	var b exploreBody
	if code := getJSON(t, s, "/api/explore", &b); code != 200 {
		t.Fatalf("status %d", code)
	}
	if b.Total != 1 || len(b.Items) != 1 || b.Items[0].Kind != catalog.KindVersion || b.Items[0].Key != "f1" ||
		b.Items[0].Versions != 2 || b.Query.Order != catalog.OrderYear || len(b.Facets) != len(catalog.FacetNames) {
		t.Fatalf("explore %+v", b)
	}
	// Once identified it is a movie; facets filter and are counted.
	if code := post(t, s, `{"fingerprint":"f1","action":"movie","tmdbId":7857}`); code != http.StatusNoContent {
		t.Fatalf("identify %d", code)
	}
	b = exploreBody{}
	getJSON(t, s, "/api/explore?decada=1970&estado=copia-identica&orden=titulo&foo=1&pais=zz", &b)
	if b.Total != 1 || b.Items[0].Kind != catalog.KindMovie || b.Items[0].TMDBID != 7857 || b.Items[0].Poster != "p" {
		t.Fatalf("filtered %+v", b)
	}
	// Malformed facets are dropped, which the page sees in query.
	if len(b.Query.Facets) != 2 || b.Query.Order != catalog.OrderTitle || b.Query.Dir != catalog.Asc {
		t.Fatalf("query %+v", b.Query)
	}
	if loc := b.Facets["ubicacion"]; len(loc) != 2 || loc[0].Value != "cine" || loc[0].Count != 1 {
		t.Fatalf("ubicacion %+v", loc)
	}
	b = exploreBody{}
	getJSON(t, s, "/api/explore?decada=1980", &b)
	if b.Total != 0 || b.Items == nil {
		t.Fatalf("no match %+v", b)
	}
}

func TestMovieEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	addCopy(t, s)
	var d catalog.MovieDetail
	if code := getJSON(t, s, "/api/movies/7857", &d); code != http.StatusNotFound {
		t.Fatalf("not stored yet: %d", code)
	}
	post(t, s, `{"fingerprint":"f1","action":"movie","tmdbId":7857}`)
	if code := getJSON(t, s, "/api/movies/7857", &d); code != 200 {
		t.Fatalf("status %d", code)
	}
	if d.Movie.Title != "Amarcord" || d.Movie.Overview != "Rimini." || d.Poster != "p" || d.Backdrop != "b" ||
		len(d.Versions) != 2 || d.Versions[0].Copies != 2 {
		t.Fatalf("movie %+v", d)
	}
	for _, bad := range []string{"/api/movies/abc", "/api/movies/0", "/api/movies/5"} {
		if code := getJSON(t, s, bad, &d); code != http.StatusNotFound {
			t.Errorf("%s: %d", bad, code)
		}
	}
}

func TestVersionEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	s.Store.SaveIdentifications([]store.Identification{{Fingerprint: "f1", Status: store.StatusUnmatched,
		Candidates: []store.Candidate{{TMDBID: 7857, Title: "Amarcord", Score: 0.7}}}})
	var d catalog.VersionDetail
	if code := getJSON(t, s, "/api/versions/f1", &d); code != 200 {
		t.Fatalf("status %d", code)
	}
	if d.Title != "Amarcord" || len(d.Versions) != 1 || d.Identification == nil || len(d.Identification.Candidates) != 1 {
		t.Fatalf("version %+v", d)
	}
	for _, bad := range []string{"/api/versions/zz", "/api/versions/id:99"} {
		if code := getJSON(t, s, bad, &d); code != http.StatusNotFound {
			t.Errorf("%s: %d", bad, code)
		}
	}
}

func TestMoviesEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	addCopy(t, s)
	var refs []store.MovieRef
	if getJSON(t, s, "/api/movies?q=amarcord", &refs); len(refs) != 0 {
		t.Fatalf("before identifying: %+v", refs)
	}
	s.Identifier.Adopt(context.Background(), 7857)
	s.Store.SetCorrection("f1", store.StatusManual, 7857)
	if getJSON(t, s, "/api/movies?q=AMARCORD&near=f1", &refs); len(refs) != 1 || refs[0].TMDBID != 7857 {
		t.Fatalf("suggestions %+v", refs)
	}
}
```

En `internal/server/server_test.go`, reemplazá:

```go
	"cinexplorer/internal/appdir"
	"cinexplorer/internal/grouping"
```

por:

```go
	"cinexplorer/internal/appdir"
	"cinexplorer/internal/catalog"
	"cinexplorer/internal/grouping"
```

En `internal/server/server_test.go`, reemplazá:

```go
	s, _ := newServer(t)
	if err := s.Store.SyncFiles([]store.FileRow{
		{Path: "../cine/a.mkv", Size: 100, MTime: 1, Fingerprint: "same", Kind: "video"},
		{Path: "../cine-ordenar/a-copy.mkv", Size: 100, MTime: 1, Fingerprint: "same", Kind: "video"},
	}, []string{"../cine", "../cine-ordenar"}); err != nil {
		t.Fatal(err)
	}
	rec := request(s.Handler(), "GET", "/api/duplicates", "", "", "127.0.0.1")
```

por:

```go
	s, _ := newServer(t)
	addCopy(t, s)
	rec := request(s.Handler(), "GET", "/api/duplicates", "", "", "127.0.0.1")
```

En `internal/server/server_test.go`, reemplazá:

```go
	}
	var d []store.Duplicate
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
```

por:

```go
	}
	var d catalog.DupReport
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
```

En `internal/server/server_test.go`, reemplazá:

```go
	}
	if len(d) != 1 || d[0].Size != 100 || len(d[0].Paths) != 2 {
		t.Fatalf("got %+v", d)
```

por:

```go
	}
	if d.Recoverable != 10 || len(d.Groups) != 1 || d.Groups[0].Key != "f1" || len(d.Groups[0].Versions) != 2 {
		t.Fatalf("got %+v", d)
```

En `internal/server/identify_test.go`, reemplazá:

```go
	rec := request(s.Handler(), "GET", "/api/unidentified", "", "", "127.0.0.1")
	var u []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil || len(u) != 1 || u[0]["dir"] != "../cine" {
		t.Fatalf("body %s", rec.Body)
	}
	if c, ok := u[0]["candidates"].([]any); !ok || len(c) != 1 {
		t.Fatalf("candidates %v", u[0]["candidates"])
	}
```

por:

```go
	rec := request(s.Handler(), "GET", "/api/unidentified", "", "", "127.0.0.1")
	var u struct {
		Pending int              `json:"pending"`
		Items   []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil || u.Pending != 0 || len(u.Items) != 1 || u.Items[0]["dir"] != "../cine" {
		t.Fatalf("body %s", rec.Body)
	}
	if c, ok := u.Items[0]["candidates"].([]any); !ok || len(c) != 1 {
		t.Fatalf("candidates %v", u.Items[0]["candidates"])
	}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/server`
Expected: FAIL de compilación: `s.Roots undefined` (y otros).

- [ ] **Step 3: Implementar**

Crear `internal/server/catalog.go`:

```go
package server

import (
	"net/http"
	"strconv"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/store"
)

// snapshot reads the catalog for one request, answering 500 on failure.
func (s *Server) snapshot(w http.ResponseWriter) (store.Snapshot, bool) {
	snap, err := s.Store.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return snap, false
	}
	return snap, true
}

// explore answers Explorar: the items that match the facets in the query,
// sorted, plus every facet's values counted over the other facets.
func (s *Server) explore(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	q := catalog.ParseQuery(r.URL.Query())
	all := catalog.Items(snap, s.Roots)
	items := catalog.Filter(all, q.Facets)
	catalog.Sort(items, q.Order, q.Dir)
	writeJSON(w, map[string]any{"total": len(items), "query": q, "items": items, "facets": catalog.Counts(all, q.Facets)})
}

func (s *Server) movie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	d, found := catalog.Movie(snap, id)
	if !found {
		http.Error(w, "la película no está en el catálogo", http.StatusNotFound)
		return
	}
	writeJSON(w, d)
}

// movies suggests catalog movies for "es un extra de…".
func (s *Server) movies(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.SuggestMovies(snap, r.URL.Query().Get("q"), r.URL.Query().Get("near")))
}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	d, found := catalog.Version(snap, r.PathValue("key"))
	if !found {
		http.Error(w, "la versión no está en el catálogo", http.StatusNotFound)
		return
	}
	writeJSON(w, d)
}

func (s *Server) duplicates(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.Duplicates(snap, s.Roots))
}

func (s *Server) unidentified(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.Unidentified(snap))
}
```

En `internal/server/server.go`, reemplazá:

```go
	AppDir   string
	Store    *store.Store
```

por:

```go
	AppDir   string
	Roots    []string // catalog-form paths, as in config.json
	Store    *store.Store
```

En `internal/server/server.go`, reemplazá:

```go
	mux.HandleFunc("GET /api/versions", s.versions)
	mux.HandleFunc("GET /api/duplicates", s.duplicates)
```

por:

```go
	mux.HandleFunc("GET /api/versions", s.versions)
	mux.HandleFunc("GET /api/versions/{key}", s.version)
	mux.HandleFunc("GET /api/explore", s.explore)
	mux.HandleFunc("GET /api/movies", s.movies)
	mux.HandleFunc("GET /api/movies/{id}", s.movie)
	mux.HandleFunc("GET /api/duplicates", s.duplicates)
```

En `internal/server/server.go`, reemplazá:

```go

func (s *Server) duplicates(w http.ResponseWriter, r *http.Request) {
	d, err := s.Store.Duplicates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, d)
}

func (s *Server) rescan(w http.ResponseWriter, r *http.Request) {
```

por:

```go

func (s *Server) rescan(w http.ResponseWriter, r *http.Request) {
```

En `internal/server/identify.go`, reemplazá:

```go
var imdbIDRe = regexp.MustCompile(`^tt\d{7,8}$`)

func (s *Server) unidentified(w http.ResponseWriter, r *http.Request) {
	u, err := s.Store.Unidentified()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, u)
}
```

por:

```go
var imdbIDRe = regexp.MustCompile(`^tt\d{7,8}$`)
```

En `internal/store/identity.go`, reemplazá:

```go

// UnidentifiedView is a version the matcher could not decide on.
type UnidentifiedView struct {
	VersionView
	Candidates []Candidate `json:"candidates"`
}

// Unidentified returns the unmatched versions with their candidates, one per
// fingerprint.
func (s *Store) Unidentified() ([]UnidentifiedView, error) {
	var vs []VersionView
	var ids map[string]*Identification
	err := s.read(func(tx querier) (err error) {
		if vs, err = s.versions(tx); err != nil {
			return err
		}
		ids, err = s.identifications(tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := []UnidentifiedView{}
	seen := map[string]bool{}
	for _, v := range vs {
		i := ids[v.Fingerprint]
		if i == nil || i.Status != StatusUnmatched || seen[v.Fingerprint] {
			continue
		}
		seen[v.Fingerprint] = true
		cands := i.Candidates
		if cands == nil {
			cands = []Candidate{}
		}
		out = append(out, UnidentifiedView{VersionView: v, Candidates: cands})
	}
	return out, nil
}

// attachIdentity fills Fingerprint, Identification and Movie of each version.
```

por:

```go

// attachIdentity fills Fingerprint, Identification and Movie of each version.
```

En `internal/store/versions_identity_test.go`, reemplazá:

```go
	}

	un, err := s.Unidentified()
	if err != nil || len(un) != 1 || un[0].Dir != "../cine/d" || len(un[0].Candidates) != 1 || un[0].Candidates[0].TMDBID != 1398 {
		t.Fatalf("unidentified %+v %v", un, err)
	}
}
```

por:

```go
	}
}
```

En `internal/store/versions_identity_test.go`, reemplazá:

```go
	}
	if un, err := ro.Unidentified(); err != nil || len(un) != 0 {
		t.Fatalf("unidentified %v %v", un, err)
	}
	if ts, err := ro.IdentifyTargets(); err != nil || ts != nil {
```

por:

```go
	}
	if ts, err := ro.IdentifyTargets(); err != nil || ts != nil {
```

En `cmd/cinexplorer/main.go`, reemplazá:

```go

	srv := &server.Server{AppDir: appDir, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal,
		Language: cfg.Language, Images: &images.Cache{Dir: filepath.Join(appDir, "cache"), ReadOnly: readOnly}}
```

por:

```go

	srv := &server.Server{AppDir: appDir, Roots: cfg.Roots, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal,
		Language: cfg.Language, Images: &images.Cache{Dir: filepath.Join(appDir, "cache"), ReadOnly: readOnly}}
```

- [ ] **Step 4: Correr los tests**

Run: `go vet ./... && go test ./...`
Expected: todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/server internal/store cmd/cinexplorer
git commit -m "feat(server): Explorar, movie, version, suggestion, duplicates and review endpoints"
```

---

### Task 9: Servir la app embebida y sus rutas

**Files:**
- Create: `internal/server/static.go`
- Move: `internal/server/web/` → `internal/server/dist/`
- Modify: `internal/server/server.go`
- Test: `internal/server/static_test.go`, `internal/server/server_test.go`

Spec §4. El servidor embebe `internal/server/dist/` (la salida de Vite, desde la Task 11). Un archivo del build se sirve tal cual, y los de `assets/` (con hash en el nombre) con caché de un año; `/api/…` e `/img/…` que no existen, y cualquier ruta con extensión que no es un archivo (`/assets/falta.js`), dan 404; el resto de las rutas (`/explorar`, `/pelicula/7857`…) recibe `index.html` con `no-cache`, y el router de la app las resuelve. `Server.Static` permite a los tests servir una app falsa. Mientras no haya build, la página mínima se mueve a `dist/` para que `go:embed` tenga qué embeber.

- [ ] **Step 1: Mover la página mínima**

```bash
git mv internal/server/web internal/server/dist
```

- [ ] **Step 2: Escribir los tests que fallan**

Crear `internal/server/static_test.go`:

```go
package server

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticApp(t *testing.T) {
	s, _ := newServer(t)
	s.Static = fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>Cinexplorer</title>")},
		"assets/index-abc1.js": {Data: []byte("console.log(1)")},
		"favicon.svg":          {Data: []byte("<svg/>")},
	}
	h := s.Handler()
	cases := []struct {
		path, body, cache string
		code              int
	}{
		{"/", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/explorar?decada=1970", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/pelicula/7857", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/version/id:9", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/revisar/duplicados", "<title>Cinexplorer</title>", "no-cache", 200},
		{"/assets/index-abc1.js", "console.log(1)", "public, max-age=31536000, immutable", 200},
		{"/favicon.svg", "<svg/>", "", 200},
		{"/assets/missing.js", "", "", http.StatusNotFound},
		{"/api/nada", "", "", http.StatusNotFound},
		{"/img/x", "", "", http.StatusNotFound},
	}
	for _, c := range cases {
		rec := request(h, "GET", c.path, "", "", "127.0.0.1")
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.body) || rec.Header().Get("Cache-Control") != c.cache {
			t.Errorf("%s: %d %q (Cache-Control %q), want %d %q (%q)", c.path, rec.Code, rec.Body, rec.Header().Get("Cache-Control"),
				c.code, c.body, c.cache)
		}
	}
}

func TestEmbeddedAppHasIndex(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/", "", "", "127.0.0.1")
	if rec.Code != 200 || !strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
		t.Fatalf("status %d: %.200s", rec.Code, rec.Body)
	}
}
```

En `internal/server/server_test.go`, reemplazá:

```go

func TestServesIndex(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/", "", "", "127.0.0.1")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "CINEXPLORER") {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestDuplicatesEndpoint(t *testing.T) {
```

por:

```go

func TestDuplicatesEndpoint(t *testing.T) {
```

- [ ] **Step 3: Correr los tests para ver que fallan**

Run: `go test ./internal/server`
Expected: FAIL de compilación (`s.Static undefined`; `pattern web: no matching files found` por el `go:embed` de la carpeta movida).

- [ ] **Step 4: Implementar**

Crear `internal/server/static.go`:

```go
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist is the web app's build (web/ compiled by Vite).
//
//go:embed dist
var dist embed.FS

// webApp returns the files of the web app: s.Static, or the embedded build.
func (s *Server) webApp() fs.FS {
	if s.Static != nil {
		return s.Static
	}
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// static serves the web app. Files of the build are served as they are
// (those under assets/ carry a content hash in their name, so browsers may
// keep them for good); any other path without an extension is one of the
// app's routes (/explorar, /pelicula/123…) and gets index.html, which the
// app's router resolves.
func (s *Server) static() http.Handler {
	app := s.webApp()
	files := http.FileServerFS(app)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(p, "api/") || strings.HasPrefix(p, "img/") {
			http.NotFound(w, r)
			return
		}
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(app, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if path.Ext(p) != "" {
				http.NotFound(w, r) // a missing file, not a route
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, app, "index.html")
	})
}
```

En `internal/server/server.go`, reemplazá:

```go
	"context"
	"embed"
	"encoding/json"
```

por:

```go
	"context"
	"encoding/json"
```

En `internal/server/server.go`, reemplazá:

```go

//go:embed web
var webFS embed.FS

type Server struct {
```

por:

```go

type Server struct {
```

En `internal/server/server.go`, reemplazá:

```go
	Language   string
}
```

por:

```go
	Language   string

	Static fs.FS // the web app; nil: the embedded build
}
```

En `internal/server/server.go`, reemplazá:

```go
func (s *Server) Handler() http.Handler {
	static, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /api/status", s.status)
```

por:

```go
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", s.static())
	mux.HandleFunc("GET /api/status", s.status)
```

- [ ] **Step 5: Correr los tests**

Run: `go vet ./... && go test ./...`
Expected: todos los paquetes `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/server
git commit -m "feat(server): serve the embedded web app with client-side routes"
```

---

### Task 10: Smoke test de un primer arranque

**Files:**
- Modify: `cmd/cinexplorer/main.go`
- Test: `cmd/cinexplorer/main_test.go`

Spec §10. `run` se separa en `setup` (config, catálogo, servidor, escaneo en segundo plano) y la escucha HTTP, para probar la app entera sin abrir puertos ni navegador. El test arma un disco sintético (`cinexplorer/` al lado de `cine/1970s/…/Amarcord.1973.720p.mkv`), sin `config.json` y sin token, espera el fin del escaneo y comprueba, con `httptest.NewServer`, la app embebida en `/` y en una ruta del cliente, `/api/explore` y el estado `noToken`. Corre en la CI de las tres plataformas.

- [ ] **Step 1: Escribir el test que falla**

Crear `cmd/cinexplorer/main_test.go`:

```go
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSmoke runs the app as a first start would: an app directory next to a
// movie folder, no config.json and no TMDB token.
func TestSmoke(t *testing.T) {
	disk := t.TempDir()
	appDir := filepath.Join(disk, "cinexplorer")
	movie := filepath.Join(disk, "cine", "1970s", "Amarcord (Federico Fellini, 1973)", "Amarcord.1973.720p.mkv")
	for _, d := range []string{appDir, filepath.Dir(movie)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(movie, []byte(strings.Repeat("x", 4096)), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, closeStore, err := setup(appDir)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	deadline := time.Now().Add(10 * time.Second)
	for st := srv.Scanner.Status(); st.Running || st.Finished.IsZero(); st = srv.Scanner.Status() {
		if time.Now().After(deadline) {
			t.Fatalf("scan did not finish: %+v", st)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(appDir, "config.json")); err != nil {
		t.Fatalf("config.json: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	get := func(path string) (int, string) {
		t.Helper()
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	for _, p := range []string{"/", "/explorar?decada=1970"} {
		if code, body := get(p); code != 200 || !strings.Contains(strings.ToLower(body), "<!doctype html>") {
			t.Errorf("%s: %d %.200s", p, code, body)
		}
	}
	code, body := get("/api/explore?decada=1970")
	var explore struct {
		Total int `json:"total"`
		Items []struct {
			Title string `json:"title"`
			Year  int    `json:"year"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &explore); err != nil || code != 200 || explore.Total != 1 ||
		explore.Items[0].Title != "Amarcord" || explore.Items[0].Year != 1973 {
		t.Fatalf("explore %d %s (%v)", code, body, err)
	}
	if code, body := get("/api/status"); code != 200 || !strings.Contains(body, `"state":"noToken"`) {
		t.Errorf("status %d %s", code, body)
	}
}
```

- [ ] **Step 2: Correr el test para ver que falla**

Run: `go test ./cmd/cinexplorer`
Expected: FAIL de compilación: `undefined: setup`.

- [ ] **Step 3: Implementar**

En `cmd/cinexplorer/main.go`, reemplazá:

```go
func run(dirOverride string, port int, browser bool) error {
	appDir, err := appdir.Resolve(dirOverride)
	if err != nil {
		return err
	}
	readOnly := !appdir.Writable(appDir)
	cfg, created, err := config.Load(appDir)
	if err != nil {
		return err
	}
	if created && !readOnly {
		if err := config.Save(appDir, cfg); err != nil {
			return err
		}
```

por:

```go
func run(dirOverride string, port int, browser bool) error {
	srv, closeStore, err := setup(dirOverride)
	if err != nil {
		return err
	}
	defer closeStore()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"
	log.Printf("Cinexplorer en %s (Ctrl+C para salir)", url)
	if browser {
		if err := platform.Open(url); err != nil {
			log.Printf("no se pudo abrir el navegador: %v", err)
		}
	}
	return http.Serve(ln, srv.Handler())
}

// setup opens the catalog in the app directory and wires the server: TMDB
// when there is a token, and, when the directory is writable, the scanner
// (started right away) and the identification runner. closeStore closes the
// catalog.
func setup(dirOverride string) (srv *server.Server, closeStore func() error, err error) {
	appDir, err := appdir.Resolve(dirOverride)
	if err != nil {
		return nil, nil, err
	}
	readOnly := !appdir.Writable(appDir)
	cfg, created, err := config.Load(appDir)
	if err != nil {
		return nil, nil, err
	}
	if created && !readOnly {
		if err := config.Save(appDir, cfg); err != nil {
			return nil, nil, err
		}
```

En `cmd/cinexplorer/main.go`, reemplazá:

```go
	st, err := openStore(appDir, readOnly)
	if err != nil {
		return err
	}
	defer st.Close()

	srv := &server.Server{AppDir: appDir, Roots: cfg.Roots, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal,
		Language: cfg.Language, Images: &images.Cache{Dir: filepath.Join(appDir, "cache"), ReadOnly: readOnly}}
```

por:

```go
	st, err := openStore(appDir, readOnly)
	if err != nil {
		return nil, nil, err
	}

	srv = &server.Server{AppDir: appDir, Roots: cfg.Roots, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal,
		Language: cfg.Language, Images: &images.Cache{Dir: filepath.Join(appDir, "cache"), ReadOnly: readOnly}}
```

En `cmd/cinexplorer/main.go`, reemplazá:

```go
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"
	log.Printf("Cinexplorer en %s (Ctrl+C para salir)", url)
	if browser {
		if err := platform.Open(url); err != nil {
			log.Printf("no se pudo abrir el navegador: %v", err)
		}
	}
	return http.Serve(ln, srv.Handler())
}
```

por:

```go
	}
	return srv, st.Close, nil
}
```

- [ ] **Step 4: Correr los tests**

Run: `go vet ./... && go test -count=3 ./cmd/cinexplorer && go test ./...`
Expected: `ok` tres veces seguidas (no es inestable) y la suite completa en verde.

- [ ] **Step 5: Commit**

```bash
git add cmd/cinexplorer
git commit -m "test: end-to-end smoke test of a first start"
```

---

### Task 11: Proyecto web (Svelte + Vite), build embebido y CI

**Files:**
- Create: `web/package.json`, `web/vite.config.js`, `web/svelte.config.js`, `web/index.html`, `web/public/favicon.svg`, `web/src/main.js`, `web/src/app.css`, `web/src/App.svelte` (provisorio), `web/src/lib/router.js`, `.gitattributes`
- Modify: `.gitignore`, `.github/workflows/ci.yml`, `scripts/build.sh`
- Test: `web/src/lib/router.test.js`
- Generated: `web/package-lock.json`, `internal/server/dist/` (build)

Spec §3. Vite compila `web/` a `internal/server/dist/` (reemplaza la página mínima). El `.gitignore` tenía `dist/`, que también ignoraba `internal/server/dist/`: pasa a `/dist/` (la salida de `scripts/build.sh`) y suma `node_modules/`. `.gitattributes` evita que Git convierta los finales de línea del build y fija LF en las fuentes de `web/` (el repo se usa con `core.autocrlf`): así un build en Windows coincide byte a byte con el de la CI en Linux. La CI suma un job `web` que corre los tests, compila y falla si el build commiteado no coincide con `web/`. Arranca también la primera pieza de lógica pura: el router (`resolve` ruta → pantalla, y `appLink`, que decide si un clic en un enlace navega dentro de la app).

`App.svelte` es provisorio en esta tarea: la versión completa llega en la Task 14.

- [ ] **Step 1: Configuración del repositorio**

En `.gitignore`, reemplazá:

```text
.worktrees/
dist/
.run/
*.db
```

por:

```text
.worktrees/
/dist/
.run/
*.db
node_modules/
```

Crear `.gitattributes`:

```text
# The web build is committed and embedded: keep it byte for byte on every OS.
internal/server/dist/** -text
# Its sources too (LF everywhere), so that a build on Windows matches the one in CI.
web/** text eol=lf
```

En `.github/workflows/ci.yml`, reemplazá:

```yaml
      - run: go test ./...
```

por:

```yaml
      - run: go test ./...
  web:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: npm ci
      - run: npm test
      - run: npm run build
      # The build is committed (Go embeds it): it must match the sources.
      - name: committed build is up to date
        run: |
          changes="$(git status --porcelain ../internal/server/dist)"
          if [ -n "$changes" ]; then
            echo "internal/server/dist no coincide con web/: corré npm run build en web/ y commiteá el resultado."
            echo "$changes"
            exit 1
          fi
```

En `scripts/build.sh`, reemplazá:

```bash
#!/usr/bin/env bash
# Cross-compiles Cinexplorer for Windows, macOS (universal) and Linux into dist/cinexplorer/.
set -euo pipefail
cd "$(dirname "$0")/.."
out=dist/cinexplorer
```

por:

```bash
#!/usr/bin/env bash
# Builds the web app and cross-compiles Cinexplorer for Windows, macOS
# (universal) and Linux into dist/cinexplorer/.
set -euo pipefail
cd "$(dirname "$0")/.."
(cd web && npm ci && npm test && npm run build)
out=dist/cinexplorer
```

- [ ] **Step 2: Proyecto web**

Crear `web/package.json`:

```json
{
  "name": "cinexplorer-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "test": "vitest run"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "^7.3.1",
    "svelte": "^5.57.1",
    "vite": "^8.3.1",
    "vitest": "^5.0.2"
  },
  "dependencies": {
    "@fontsource-variable/inter": "^5.3.0"
  }
}
```

Crear `web/vite.config.js`:

```js
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vite'

// The build goes where the Go server embeds it. In development, `npm run
// dev` serves the app and sends /api and /img to the Go server, started
// with: go run ./cmd/cinexplorer -port 8080 -no-browser
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../internal/server/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/img': 'http://127.0.0.1:8080',
    },
  },
  test: {
    include: ['src/**/*.test.js'],
  },
})
```

Crear `web/svelte.config.js`:

```js
// Svelte 5 with the defaults; the file only exists so that the Vite plugin
// does not warn about its absence.
export default {}
```

Crear `web/index.html`:

```html
<!doctype html>
<html lang="es">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Cinexplorer</title>
    <link rel="icon" href="/favicon.svg" />
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.js"></script>
  </body>
</html>
```

Crear `web/public/favicon.svg`:

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#14171c"/><rect x="7" y="5" width="18" height="22" rx="2" fill="none" stroke="#40bcf4" stroke-width="2.5"/><path d="M13 12v8l7-4z" fill="#40bcf4"/></svg>
```

Crear `web/src/main.js`:

```js
import '@fontsource-variable/inter'
import './app.css'
import { mount } from 'svelte'
import App from './App.svelte'

mount(App, { target: document.getElementById('app') })
```

Crear `web/src/app.css`:

```css
:root {
  --bg: #14171c;
  --surface: #1b1f25;
  --surface-2: #232830;
  --line: #2a3038;
  --line-strong: #3a424c;
  --text: #d8dde3;
  --strong: #ffffff;
  --muted: #9aa4ae;
  --faint: #6c7682;
  --accent: #40bcf4;
  --on-accent: #08141c;
  --best: #7ee29a;
  --best-bg: #1f4d2e;
  --same: #f0c060;
  --same-bg: #4d3a14;
  --warn: #e0a040;
  --radius: 4px;
  --gutter: 16px;
  color-scheme: dark;
  font-family: 'Inter Variable', system-ui, sans-serif;
  font-size: 15px;
  line-height: 1.45;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
}

a {
  color: inherit;
  text-decoration: none;
}

a:hover {
  color: var(--strong);
}

button,
input,
select {
  font: inherit;
  color: inherit;
}

button {
  cursor: pointer;
  background: transparent;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius);
  padding: 4px 12px;
}

button:hover:not(:disabled) {
  border-color: var(--muted);
}

button:disabled {
  opacity: 0.5;
  cursor: default;
}

button.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--on-accent);
  font-weight: 600;
}

input[type='search'],
input[type='text'],
select {
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: var(--radius);
  padding: 4px 8px;
}

input::placeholder {
  color: var(--faint);
}

:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

main {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 var(--gutter) 48px;
}

.label {
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--faint);
}

.path {
  font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
  font-size: 12px;
  color: var(--faint);
  word-break: break-all;
}

.empty {
  color: var(--muted);
  padding: 48px 0;
  text-align: center;
}
```

Un `App.svelte` provisorio, que la Task 14 reemplaza.

Crear `web/src/App.svelte`:


```svelte
<main>
  <p class="empty">Cinexplorer</p>
</main>
```

Run (con Node ≥ 22.12 en el PATH, ver **Entorno**): `cd web && npm install`
Expected: crea `web/node_modules/` y `web/package-lock.json`, `found 0 vulnerabilities`.

- [ ] **Step 3: Escribir el test que falla**

Crear `web/src/lib/router.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { appLink, itemHref, resolve } from './router.js'

describe('resolve', () => {
  it.each([
    ['/', { page: 'redirect', to: '/explorar' }],
    ['/explorar', { page: 'explorar' }],
    ['/revisar', { page: 'revisar', tab: 'sin-identificar' }],
    ['/revisar/duplicados', { page: 'revisar', tab: 'duplicados' }],
    ['/pelicula/7857', { page: 'pelicula', id: 7857 }],
    ['/version/a1b2', { page: 'version', key: 'a1b2' }],
    ['/version/id%3A9', { page: 'version', key: 'id:9' }],
    ['/pelicula/0', { page: 'notfound' }],
    ['/pelicula/abc', { page: 'notfound' }],
    ['/version/%E0%A4%A', { page: 'notfound' }],
    ['/otra', { page: 'notfound' }],
  ])('%s', (path, want) => {
    expect(resolve(path)).toEqual(want)
  })
})

function anchor(href, attrs = {}) {
  return {
    href,
    target: attrs.target ?? '',
    hasAttribute: (name) => name in attrs,
  }
}

const click = { button: 0, defaultPrevented: false }
const origin = 'http://127.0.0.1:8080'

describe('appLink', () => {
  it('keeps links to the app inside the app', () => {
    expect(appLink(click, anchor('http://127.0.0.1:8080/explorar?pais=IT'), origin)).toBe('/explorar?pais=IT')
    expect(appLink(click, anchor('/pelicula/1'), origin)).toBe('/pelicula/1')
  })
  it('leaves the rest to the browser', () => {
    expect(appLink(click, null, origin)).toBeNull()
    expect(appLink(click, anchor('https://www.themoviedb.org/'), origin)).toBeNull()
    expect(appLink(click, anchor('/img/poster/1.jpg'), origin)).toBeNull()
    expect(appLink(click, anchor('/api/explore'), origin)).toBeNull()
    expect(appLink(click, anchor('/explorar', { target: '_blank' }), origin)).toBeNull()
    expect(appLink(click, anchor('/explorar', { download: '' }), origin)).toBeNull()
    expect(appLink({ ...click, ctrlKey: true }, anchor('/explorar'), origin)).toBeNull()
    expect(appLink({ ...click, button: 1 }, anchor('/explorar'), origin)).toBeNull()
    expect(appLink({ ...click, defaultPrevented: true }, anchor('/explorar'), origin)).toBeNull()
  })
})

describe('itemHref', () => {
  it('links movies and versions', () => {
    expect(itemHref({ kind: 'movie', tmdbId: 7857 })).toBe('/pelicula/7857')
    expect(itemHref({ kind: 'version', key: 'id:9' })).toBe('/version/id%3A9')
  })
})
```

- [ ] **Step 4: Correr el test para ver que falla**

Run: `cd web && npm test`
Expected: FAIL: `Failed to resolve import "./router.js"`.

- [ ] **Step 5: Implementar**

Crear `web/src/lib/router.js`:

```js
// Routes of the app. Pure: nav.svelte.js keeps the current route.

// resolve maps a pathname to the page to show and its parameters.
export function resolve(pathname) {
  if (pathname === '/') return { page: 'redirect', to: '/explorar' }
  if (pathname === '/explorar') return { page: 'explorar' }
  if (pathname === '/revisar') return { page: 'revisar', tab: 'sin-identificar' }
  if (pathname === '/revisar/duplicados') return { page: 'revisar', tab: 'duplicados' }
  let m = pathname.match(/^\/pelicula\/([1-9][0-9]*)$/)
  if (m) return { page: 'pelicula', id: Number(m[1]) }
  m = pathname.match(/^\/version\/([^/]+)$/)
  if (m) {
    try {
      return { page: 'version', key: decodeURIComponent(m[1]) }
    } catch {
      // A malformed escape: not a route.
    }
  }
  return { page: 'notfound' }
}

// appLink returns the path + query a click on a link should navigate to
// inside the app, or null when the browser should handle it (another site,
// the API or images, a new tab, a download, modifier keys).
export function appLink(event, anchor, origin) {
  if (!anchor || event.defaultPrevented || event.button !== 0) return null
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return null
  if (anchor.target || anchor.hasAttribute('download')) return null
  const url = new URL(anchor.href, origin)
  if (url.origin !== origin) return null
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/img/')) return null
  return url.pathname + url.search
}

// movieHref and versionHref are the pages of Explorar's items.
export function movieHref(tmdbId) {
  return `/pelicula/${tmdbId}`
}

export function versionHref(key) {
  return `/version/${encodeURIComponent(key)}`
}

export function itemHref(item) {
  return item.kind === 'movie' ? movieHref(item.tmdbId) : versionHref(item.key)
}
```

- [ ] **Step 6: Tests, build y Go**

Run: `cd web && npm test && npm run build`
Expected: `Tests  … passed`; el build termina con `✓ built in …` y sin advertencias `[vite-plugin-svelte]`; `internal/server/dist/` queda con `index.html`, `favicon.svg` y `assets/` (JS, CSS y las fuentes Inter).

Run: `go vet ./... && go test ./...`
Expected: todos los paquetes `ok` (`TestEmbeddedAppHasIndex` y el smoke test ya ven el `index.html` de Vite).

Run: `git status --short internal/server/dist`
Expected: `index.html` modificado y `favicon.svg` y `assets/…` nuevos (si no aparecen, revisá el `.gitignore` del Step 1).

- [ ] **Step 7: Commit**

```bash
git add .gitignore .gitattributes .github/workflows/ci.yml scripts/build.sh web internal/server/dist
git status --short   # no debe haber nada de web/node_modules
git commit -m "feat(web): Svelte + Vite project embedded in the binary"
```

---

### Task 12: Lógica pura: atajos de teclado y estado del servidor

**Files:**
- Create: `web/src/lib/keys.js`, `web/src/lib/status.js`
- Test: `web/src/lib/keys.test.js`, `web/src/lib/status.test.js`

Spec §9.1, §9.5 y §9.8. `keyAction` traduce una tecla en una acción de la cola de Sin identificar (↑/↓, 1–5, `/`, `N`, `E`), salvo que se esté escribiendo en un campo o haya modificadores. `status.js` lee `/api/status`: si hay algo corriendo (`busy`), el texto y el tono del indicador (`summary`), el progreso de una corrida (`progress`, para refrescar las páginas mientras avanza) y por qué no se puede buscar en TMDB (`tmdbProblem`).

- [ ] **Step 1: Escribir los tests que fallan**

Crear `web/src/lib/keys.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { keyAction } from './keys.js'

const key = (k, extra = {}) => ({ key: k, target: { tagName: 'BODY' }, ...extra })

describe('keyAction', () => {
  it.each([
    ['ArrowDown', { type: 'next' }],
    ['ArrowUp', { type: 'prev' }],
    ['1', { type: 'pick', index: 0 }],
    ['5', { type: 'pick', index: 4 }],
    ['/', { type: 'search' }],
    ['n', { type: 'ignore' }],
    ['N', { type: 'ignore' }],
    ['e', { type: 'extra' }],
    ['6', null],
    ['x', null],
  ])('%s', (k, want) => {
    expect(keyAction(key(k))).toEqual(want)
  })
  it('ignores typing and modifiers', () => {
    expect(keyAction(key('n', { target: { tagName: 'INPUT' } }))).toBeNull()
    expect(keyAction(key('1', { target: { tagName: 'DIV', isContentEditable: true } }))).toBeNull()
    expect(keyAction(key('1', { ctrlKey: true }))).toBeNull()
  })
})
```

Crear `web/src/lib/status.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { busy, progress, summary, tmdbProblem } from './status.js'

const idle = { readOnly: false, scan: { running: false }, identify: { state: 'idle' } }

describe('busy', () => {
  it('is set while scanning or identifying', () => {
    expect(busy(null)).toBe(false)
    expect(busy(idle)).toBe(false)
    expect(busy({ ...idle, scan: { running: true } })).toBe(true)
    expect(busy({ ...idle, identify: { state: 'running' } })).toBe(true)
    expect(busy({ ...idle, identify: null })).toBe(false)
  })
})

describe('summary', () => {
  it.each([
    [idle, 'Al día', ''],
    [{ ...idle, scan: { running: true, files: 120 } }, 'Escaneando · 120 archivos', 'busy'],
    [{ ...idle, scan: { running: true, files: 120, toProbe: 40, probed: 12 } }, 'Escaneando · 12/40 analizados', 'busy'],
    [{ ...idle, identify: { state: 'running', toIdentify: 300, identified: 120 } }, 'Identificando · 120/300', 'busy'],
    [
      { ...idle, identify: { state: 'running', toIdentify: 2, identified: 2, toEnrich: 10, enriched: 3 } },
      'Completando datos · 3/10',
      'busy',
    ],
    [{ ...idle, identify: { state: 'offline' } }, 'Sin conexión con TMDB', 'warn'],
    [{ ...idle, identify: { state: 'noToken' } }, 'Falta el token de TMDB', 'warn'],
    [{ ...idle, identify: { state: 'badToken' } }, 'Token de TMDB inválido', 'warn'],
    [{ readOnly: true, scan: {}, identify: null }, 'Modo consulta', 'warn'],
  ])('%j', (st, text, tone) => {
    expect(summary(st)).toEqual({ text, tone })
  })
})

describe('tmdbProblem', () => {
  it('explains why TMDB cannot be searched', () => {
    expect(tmdbProblem(idle)).toBe('')
    expect(tmdbProblem(null)).toBe('')
    expect(tmdbProblem({ ...idle, identify: { state: 'noToken' } })).toMatch(/token/)
    expect(tmdbProblem({ ...idle, identify: { state: 'offline' } })).toMatch(/conexión/)
    expect(tmdbProblem({ readOnly: true, identify: null })).toMatch(/consulta/)
  })
})

describe('progress', () => {
  it('moves with versions, identifications and enrichment', () => {
    expect(progress(null)).toBe('')
    expect(progress(idle)).toBe('0|0|0')
    expect(progress({ scan: { versions: 40 }, identify: { identified: 3, enriched: 1 } })).toBe('40|3|1')
    expect(progress({ scan: { versions: 40 }, identify: null })).toBe('40|0|0')
  })
})
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `cd web && npm test`
Expected: FAIL: no se resuelven `./keys.js` ni `./status.js`.

- [ ] **Step 3: Implementar**

Crear `web/src/lib/keys.js`:

```js
// Keyboard shortcuts of the Sin identificar queue.

// keyAction maps a keydown event to an action of the queue, or null. Keys
// typed into a field, or with Ctrl/Alt/Meta, are not shortcuts.
export function keyAction(event) {
  if (event.ctrlKey || event.altKey || event.metaKey) return null
  const tag = event.target?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || event.target?.isContentEditable) return null
  switch (event.key) {
    case 'ArrowDown':
      return { type: 'next' }
    case 'ArrowUp':
      return { type: 'prev' }
    case '/':
      return { type: 'search' }
    case 'n':
    case 'N':
      return { type: 'ignore' }
    case 'e':
    case 'E':
      return { type: 'extra' }
  }
  if (/^[1-5]$/.test(event.key)) return { type: 'pick', index: Number(event.key) - 1 }
  return null
}
```

Crear `web/src/lib/status.js`:

```js
// Reading /api/status: {readOnly, scan, identify}.

// busy reports whether a scan or an identification run is going on.
export function busy(st) {
  return !!st && (!!st.scan?.running || st.identify?.state === 'running')
}

// progress sums up what a run has changed so far: when it moves, the pages
// have new data to show.
export function progress(st) {
  if (!st) return ''
  return [st.scan?.versions ?? 0, st.identify?.identified ?? 0, st.identify?.enriched ?? 0].join('|')
}

// tmdbProblem explains why TMDB cannot be searched now ("" when it can).
export function tmdbProblem(st) {
  if (!st) return ''
  if (st.readOnly) return 'Modo consulta: el catálogo no se puede modificar.'
  switch (st.identify?.state) {
    case 'noToken':
      return 'Falta el token de TMDB en config.json: no se pueden buscar películas.'
    case 'badToken':
      return 'TMDB rechazó el token de config.json.'
    case 'offline':
      return 'Sin conexión con TMDB: se reintenta solo.'
  }
  return ''
}

// summary is the short text of the status indicator, and its tone:
// "busy", "warn" or "" (all quiet).
export function summary(st) {
  if (!st) return { text: '', tone: '' }
  if (st.scan?.running) {
    const { files = 0, probed = 0, toProbe = 0 } = st.scan
    const detail = toProbe > 0 ? `${probed}/${toProbe} analizados` : `${files} archivos`
    return { text: `Escaneando · ${detail}`, tone: 'busy' }
  }
  const id = st.identify
  if (id?.state === 'running') {
    if (id.toEnrich > 0 && id.identified >= id.toIdentify) {
      return { text: `Completando datos · ${id.enriched}/${id.toEnrich}`, tone: 'busy' }
    }
    return { text: `Identificando · ${id.identified}/${id.toIdentify}`, tone: 'busy' }
  }
  if (st.readOnly) return { text: 'Modo consulta', tone: 'warn' }
  switch (id?.state) {
    case 'offline':
      return { text: 'Sin conexión con TMDB', tone: 'warn' }
    case 'noToken':
      return { text: 'Falta el token de TMDB', tone: 'warn' }
    case 'badToken':
      return { text: 'Token de TMDB inválido', tone: 'warn' }
  }
  return { text: 'Al día', tone: '' }
}
```

- [ ] **Step 4: Correr los tests**

Run: `cd web && npm test`
Expected: todos los tests pasan.

- [ ] **Step 5: Commit**

Estos módulos todavía no los importa la app, así que el build no cambia.

```bash
git add web/src/lib
git commit -m "feat(web): keyboard shortcuts and server status logic"
```

---

### Task 13: Lógica pura: formatos y facetas en la URL

**Files:**
- Create: `web/src/lib/format.js`, `web/src/lib/facets.js`
- Test: `web/src/lib/format.test.js`, `web/src/lib/facets.test.js`

Spec §9.8. `format.js`: tamaños como los muestra el explorador de archivos (`9,8 GB`, base 1024), duraciones (`2 h 3 min`), países e idiomas en castellano con `Intl.DisplayNames`, codec, pistas y la línea técnica de una versión. `facets.js`: la consulta de Explorar ↔ la URL. `parse` descarta parámetros desconocidos (los valores los valida el servidor); `toSearch` escribe en un orden fijo y sin los valores por defecto, así dos consultas iguales dan la misma URL y la app puede comparar lo que pidió con lo que el servidor aplicó. `withFacet` mantiene el año dentro de la década.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `web/src/lib/format.test.js`:

```js
import { describe, expect, it } from 'vitest'
import {
  country,
  duration,
  fileName,
  language,
  languages,
  mainFile,
  percent,
  resolution,
  runtime,
  sameTitle,
  size,
  subtitles,
  versionLine,
} from './format.js'

describe('size', () => {
  it.each([
    [0, '0 B'],
    [512, '512 B'],
    [1536, '1,5 KB'],
    [10522669875, '9,8 GB'],
    [150 * 1024 ** 3, '150 GB'],
    [undefined, '0 B'],
  ])('%s → %s', (bytes, want) => {
    expect(size(bytes)).toBe(want)
  })
})

describe('duration', () => {
  it.each([
    [0, ''],
    [45 * 60000, '45 min'],
    [123 * 60000, '2 h 3 min'],
    [120 * 60000, '2 h'],
    [undefined, ''],
  ])('%s → %s', (ms, want) => {
    expect(duration(ms)).toBe(want)
  })
  it('formats TMDB runtimes', () => {
    expect(runtime(123)).toBe('2 h 3 min')
    expect(runtime(0)).toBe('')
  })
})

describe('names', () => {
  it('names countries and languages in Spanish', () => {
    expect(country('IT')).toBe('Italia')
    expect(language('en')).toBe('inglés')
    expect(country('')).toBe('')
    expect(language('zz')).toBe('zz')
  })
})

describe('versions', () => {
  const v = {
    source: 'BluRay',
    codec: 'h264',
    parts: 2,
    size: 10522669875,
    durationMs: 123 * 60000,
    subs: [{ lang: 'en' }, { lang: '' }],
    files: [
      { path: '../cine/a/CD1.mkv', role: 'main', missing: true },
      { path: '../cine/a/CD2.mkv', role: 'main', missing: false },
      { path: '../cine/a/a.es.srt', role: 'subtitle', lang: 'es' },
      { path: '../cine/a/a.srt', role: 'subtitle', lang: '' },
    ],
  }
  it('describes a version', () => {
    expect(versionLine(v)).toBe('BluRay · H.264 · 2 partes · 9,8 GB · 2 h 3 min')
    expect(versionLine({ size: 0, parts: 1 })).toBe('0 B')
    expect(resolution('2160p')).toBe('4K')
    expect(resolution('576p')).toBe('576p')
    expect(resolution('')).toBe('—')
  })
  it('lists languages', () => {
    expect(languages([{ lang: 'it' }, { lang: 'en' }, { lang: 'it' }, { lang: '' }])).toBe('italiano, inglés')
    expect(subtitles(v)).toBe('inglés, español, ? (ext.)')
    expect(subtitles({ subs: [], files: [] })).toBe('')
  })
  it('finds the file to open', () => {
    expect(mainFile(v).path).toBe('../cine/a/CD2.mkv')
    expect(mainFile({ files: [] })).toBeNull()
    expect(fileName('../cine/a/CD2.mkv')).toBe('CD2.mkv')
  })
})

describe('misc', () => {
  it('formats scores', () => {
    expect(percent(0.924)).toBe('92%')
  })
  it('compares titles loosely', () => {
    expect(sameTitle('Amarcord', 'AMARCORD')).toBe(true)
    expect(sameTitle('Pájaros', 'Pajaros')).toBe(true)
    expect(sameTitle('Gritos y susurros', 'Viskningar och rop')).toBe(false)
  })
})
```

Crear `web/src/lib/facets.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { chips, parse, toSearch, valueLabel, withFacet, withOrder } from './facets.js'

describe('parse and toSearch', () => {
  it('reads facets and order', () => {
    expect(parse('?pais=IT&decada=1970&orden=titulo&foo=1')).toEqual({
      facets: { pais: 'IT', decada: '1970' },
      order: 'titulo',
      dir: 'asc',
    })
    expect(parse('')).toEqual({ facets: {}, order: 'anio', dir: 'desc' })
    expect(parse('?orden=nada&dir=up')).toEqual({ facets: {}, order: 'anio', dir: 'desc' })
    expect(parse('?orden=tamano&dir=asc')).toEqual({ facets: {}, order: 'tamano', dir: 'asc' })
  })
  it('writes a stable URL without defaults', () => {
    expect(toSearch(parse('?pais=IT&decada=1970&orden=titulo&dir=asc'))).toBe('?decada=1970&pais=IT&orden=titulo')
    expect(toSearch(parse(''))).toBe('')
    expect(toSearch({ facets: {}, order: 'anio', dir: 'asc' })).toBe('?dir=asc')
    expect(toSearch({ facets: { ubicacion: 'cine/1970s' }, order: 'titulo', dir: 'desc' })).toBe(
      '?ubicacion=cine%2F1970s&orden=titulo&dir=desc',
    )
  })
  it('round-trips the server query', () => {
    const server = { facets: { anio: '1973', decada: '1970', director: '4415' }, order: 'agregado', dir: 'desc' }
    expect(parse(toSearch(server))).toEqual(server)
  })
})

describe('withFacet', () => {
  const q = { facets: { decada: '1970', anio: '1973' }, order: 'anio', dir: 'desc' }
  it('sets and clears', () => {
    expect(withFacet(q, 'pais', 'IT').facets).toEqual({ decada: '1970', anio: '1973', pais: 'IT' })
    expect(withFacet(q, 'anio', null).facets).toEqual({ decada: '1970' })
  })
  it('keeps the year inside the decade', () => {
    expect(withFacet(q, 'decada', '1980').facets).toEqual({ decada: '1980' })
    expect(withFacet(q, 'decada', null).facets).toEqual({})
    expect(withFacet({ facets: {} }, 'anio', 1985).facets).toEqual({ anio: '1985', decada: '1980' })
  })
  it('does not change the query it gets', () => {
    withFacet(q, 'pais', 'IT')
    expect(q.facets).toEqual({ decada: '1970', anio: '1973' })
  })
})

describe('withOrder', () => {
  it('resets the direction', () => {
    expect(withOrder({ facets: {}, order: 'anio', dir: 'asc' }, 'titulo')).toEqual({ facets: {}, order: 'titulo', dir: 'asc' })
    expect(withOrder({ facets: {}, order: 'titulo', dir: 'asc' }, 'tamano').dir).toBe('desc')
  })
})

describe('labels', () => {
  it('names values', () => {
    expect(valueLabel('decada', '1970')).toBe('1970s')
    expect(valueLabel('pais', 'IT')).toBe('Italia')
    expect(valueLabel('idioma', 'it')).toBe('italiano')
    expect(valueLabel('estado', 'copia-identica')).toBe('Copia idéntica')
    expect(valueLabel('director', '4415', { value: '4415', label: 'Federico Fellini' })).toBe('Federico Fellini')
    expect(valueLabel('director', '4415')).toBe('4415')
  })
  it('lists the applied facets as chips', () => {
    const counts = { director: [{ value: '4415', label: 'Federico Fellini', count: 2 }] }
    expect(chips({ facets: { director: '4415', decada: '1970' } }, counts)).toEqual([
      { name: 'decada', value: '1970', label: '1970s' },
      { name: 'director', value: '4415', label: 'Federico Fellini' },
    ])
  })
})
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `cd web && npm test`
Expected: FAIL: no se resuelven `./format.js` ni `./facets.js`.

- [ ] **Step 3: Implementar**

Crear `web/src/lib/format.js`:

```js
// Formatting of sizes, durations, languages, countries and versions.

const units = ['B', 'KB', 'MB', 'GB', 'TB']

// size formats bytes the way file managers do (1 KB = 1024 B): "9,8 GB".
export function size(bytes) {
  let n = bytes || 0
  let u = 0
  while (n >= 1024 && u < units.length - 1) {
    n /= 1024
    u++
  }
  const digits = u === 0 || n >= 100 ? 0 : 1
  return `${n.toLocaleString('es', { maximumFractionDigits: digits })} ${units[u]}`
}

// duration formats milliseconds as "2 h 3 min" ("" when unknown).
export function duration(ms) {
  const min = Math.round((ms || 0) / 60000)
  if (min <= 0) return ''
  const h = Math.floor(min / 60)
  const m = min % 60
  if (h === 0) return `${m} min`
  return m === 0 ? `${h} h` : `${h} h ${m} min`
}

// runtime formats minutes (TMDB's runtime) the same way.
export function runtime(minutes) {
  return duration((minutes || 0) * 60000)
}

const names = {}

function displayName(type, code) {
  if (!code) return ''
  try {
    names[type] ??= new Intl.DisplayNames(['es'], { type })
    return names[type].of(code) || code
  } catch {
    return code
  }
}

// country names an ISO 3166-1 code in Spanish ("IT" → "Italia").
export function country(code) {
  return displayName('region', code)
}

// language names an ISO 639 code in Spanish ("it" → "italiano").
export function language(code) {
  return displayName('language', code)
}

// resolution is how a version's resolution reads on its card.
export function resolution(r) {
  if (r === '2160p') return '4K'
  return r || '—'
}

const codecs = {
  h264: 'H.264',
  hevc: 'H.265',
  av1: 'AV1',
  vp9: 'VP9',
  vp8: 'VP8',
  mpeg4: 'XviD/DivX',
  mpeg2: 'MPEG-2',
  mpeg1: 'MPEG-1',
  wmv: 'WMV',
  rv: 'RealVideo',
}

export function codec(c) {
  return codecs[c] ?? c ?? ''
}

// languages lists the languages of tracks, known ones only, without
// repeating: "italiano, inglés".
export function languages(tracks) {
  const seen = []
  for (const t of tracks ?? []) {
    const name = language(t.lang)
    if (name && !seen.includes(name)) seen.push(name)
  }
  return seen.join(', ')
}

// subtitles lists a version's subtitle languages: embedded tracks, then
// external files marked "(ext.)".
export function subtitles(version) {
  const out = []
  const embedded = languages(version.subs)
  if (embedded) out.push(embedded)
  const external = []
  for (const f of version.files ?? []) {
    if (f.role !== 'subtitle' || f.missing) continue
    const name = f.lang ? language(f.lang) : '?'
    if (!external.includes(name)) external.push(name)
  }
  if (external.length) out.push(`${external.join(', ')} (ext.)`)
  return out.join(', ')
}

// versionLine is "BluRay · H.264 · 2 partes · 9,8 GB · 2 h 3 min".
export function versionLine(v) {
  return [v.source, codec(v.codec), v.parts > 1 ? `${v.parts} partes` : '', size(v.size), duration(v.durationMs)]
    .filter(Boolean)
    .join(' · ')
}

// mainFile is the first present main file of a version (the one ▶ opens).
export function mainFile(v) {
  return (v.files ?? []).find((f) => f.role === 'main' && !f.missing) ?? null
}

// fileName is the last part of a catalog path.
export function fileName(path) {
  return (path ?? '').split('/').pop()
}

export function percent(score) {
  return `${Math.round((score || 0) * 100)}%`
}

// sameTitle compares titles ignoring case and accents, to show the original
// title only when it says something new.
export function sameTitle(a, b) {
  const norm = (s) =>
    (s ?? '')
      .normalize('NFD')
      .replace(/\p{Diacritic}/gu, '')
      .toLowerCase()
      .trim()
  return norm(a) === norm(b)
}
```

Crear `web/src/lib/facets.js`:

```js
// Explorar's query: facets and order, as they appear in the URL.
import { country, language } from './format.js'

// FACETS lists the facets of the bar, in order. The year lives inside the
// decade menu; `more` facets go in the "Más" menu.
export const FACETS = [
  { name: 'decada', label: 'Década' },
  { name: 'director', label: 'Director' },
  { name: 'genero', label: 'Género' },
  { name: 'pais', label: 'País' },
  { name: 'resolucion', label: 'Resolución' },
  { name: 'subs', label: 'Subtítulos' },
  { name: 'idioma', label: 'Idioma original', more: true },
  { name: 'coleccion', label: 'Colección', more: true },
  { name: 'ubicacion', label: 'Ubicación', more: true },
  { name: 'estado', label: 'Estado', more: true },
]

const NAMES = ['decada', 'anio', ...FACETS.slice(1).map((f) => f.name)]

export const ORDERS = [
  { value: 'anio', label: 'Año' },
  { value: 'titulo', label: 'Título' },
  { value: 'agregado', label: 'Agregado' },
  { value: 'tamano', label: 'Tamaño' },
]

export const STATES = {
  'sin-identificar': 'Sin identificar',
  'varias-versiones': 'Varias versiones',
  'copia-identica': 'Copia idéntica',
}

// defaultDir is the natural direction of an order: A to Z for titles,
// newest or largest first otherwise.
export function defaultDir(order) {
  return order === 'titulo' ? 'asc' : 'desc'
}

// parse reads a query string ("?decada=1970&orden=titulo"). Unknown
// parameters are dropped; the server validates the values.
export function parse(search) {
  const params = new URLSearchParams(search)
  const facets = {}
  for (const name of NAMES) {
    const v = params.get(name)
    if (v) facets[name] = v
  }
  const order = ORDERS.some((o) => o.value === params.get('orden')) ? params.get('orden') : 'anio'
  const dir = ['asc', 'desc'].includes(params.get('dir')) ? params.get('dir') : defaultDir(order)
  return { facets, order, dir }
}

// toSearch writes a query back, in a stable order and leaving defaults out,
// so that equal queries give equal URLs.
export function toSearch({ facets, order, dir }) {
  const params = new URLSearchParams()
  for (const name of NAMES) {
    if (facets[name]) params.set(name, facets[name])
  }
  if (order && order !== 'anio') params.set('orden', order)
  if (dir && dir !== defaultDir(order || 'anio')) params.set('dir', dir)
  const s = params.toString()
  return s ? `?${s}` : ''
}

// withFacet sets (or, with a null value, clears) one facet. A year outside
// the chosen decade is dropped, and clearing the decade clears the year.
export function withFacet(query, name, value) {
  const facets = { ...query.facets }
  if (value == null || value === '') delete facets[name]
  else facets[name] = String(value)
  if (name === 'decada' && facets.anio && (!facets.decada || !facets.anio.startsWith(facets.decada.slice(0, 3)))) {
    delete facets.anio
  }
  if (name === 'anio' && value && !facets.decada) {
    facets.decada = String(Math.floor(Number(value) / 10) * 10)
  }
  return { ...query, facets }
}

// withOrder changes the order, resetting the direction to its natural one.
export function withOrder(query, order) {
  return { ...query, order, dir: defaultDir(order) }
}

// valueLabel is how a facet value reads in menus and chips. counted is the
// server's {value, label, count} when known.
export function valueLabel(name, value, counted) {
  switch (name) {
    case 'decada':
      return `${value}s`
    case 'pais':
      return country(value)
    case 'idioma':
    case 'subs':
      return language(value)
    case 'estado':
      return STATES[value] ?? value
  }
  return counted?.label || value
}

// chips lists the applied facets, labelled with the server's counts.
export function chips(query, counts = {}) {
  const out = []
  for (const name of NAMES) {
    const value = query.facets[name]
    if (!value) continue
    const counted = (counts[name] ?? []).find((v) => v.value === value)
    out.push({ name, value, label: valueLabel(name, value, counted) })
  }
  return out
}
```

- [ ] **Step 4: Correr los tests**

Run: `cd web && npm test`
Expected: todos los tests pasan (los nombres en castellano necesitan el ICU completo de Node, que viene por defecto).

- [ ] **Step 5: Commit**

```bash
git add web/src/lib
git commit -m "feat(web): formatting and Explorar facets in the URL"
```

---

### Task 14: Esqueleto de la app: rutas, API, estado y barra superior

**Files:**
- Create: `web/src/lib/api.js`, `web/src/lib/nav.svelte.js`, `web/src/lib/app.svelte.js`, `web/src/components/Nav.svelte`, `web/src/components/Estado.svelte`, `web/src/pages/Explorar.svelte`, `web/src/pages/Pelicula.svelte`, `web/src/pages/Version.svelte`, `web/src/pages/Revisar.svelte` (las cuatro páginas, provisorias)
- Modify: `web/src/App.svelte` (completo)

Spec §9.1 y §9.7. `api.js` envuelve la API (los errores llevan el código HTTP y el texto del servidor) y arma las URLs de imágenes (`?v=` para películas guardadas, `?p=` para candidatos). `nav.svelte.js` guarda la ruta actual en un `$state` sincronizado con la barra de direcciones (`navigate`, `back`, `patchState` para recordar el scroll y cuánto se mostró de una lista). `app.svelte.js` consulta `/api/status` cada 3 s mientras algo corre y cada 30 s si no, sube `generation` al terminar una corrida (y cada 20 s mientras avanza una larga) para que las páginas recarguen, y muestra avisos por 6 s. `App.svelte` elige la página con `resolve`, redirige `/` a `/explorar` y convierte los clics en enlaces internos en navegación sin recargar. El indicador de estado (`Estado.svelte`) abre un panel con el detalle y "Volver a escanear".

Las páginas son provisorias en esta tarea (una línea cada una); las reemplazan las Tasks 15–17.

- [ ] **Step 1: Módulos con estado**

Crear `web/src/lib/api.js`:

```js
// Calls to the Go server. Errors carry the HTTP status and the server's text.

export class ApiError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

async function call(method, path, body) {
  const init = { method, headers: {} }
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  let res
  try {
    res = await fetch(path, init)
  } catch {
    throw new ApiError(0, 'No se pudo contactar a Cinexplorer. ¿Sigue abierto?')
  }
  if (!res.ok) {
    const text = (await res.text()).trim()
    throw new ApiError(res.status, text || `Error ${res.status}`)
  }
  if (res.status === 204 || res.status === 202) return null
  return res.json()
}

const get = (path) => call('GET', path)
const post = (path, body) => call('POST', path, body ?? {})

export const api = {
  status: () => get('/api/status'),
  explore: (search) => get(`/api/explore${search}`),
  movie: (id) => get(`/api/movies/${id}`),
  version: (key) => get(`/api/versions/${encodeURIComponent(key)}`),
  suggest: (q, near) => get(`/api/movies?${new URLSearchParams({ q, near })}`),
  unidentified: () => get('/api/unidentified'),
  duplicates: () => get('/api/duplicates'),
  search: (q) => get(`/api/tmdb/search?${new URLSearchParams({ q })}`),
  identify: (fingerprint, action, tmdbId = 0) => post('/api/identify', { fingerprint, action, tmdbId }),
  open: (path) => post('/api/open', { path }),
  reveal: (path) => post('/api/reveal', { path }),
  scan: () => post('/api/scan'),
}

// Image URLs. A stored movie's image carries its version (the name of the
// TMDB path) so browsers keep it until it changes; a candidate that is not
// stored yet passes its TMDB path.
export function posterURL(tmdbId, version) {
  return `/img/poster/${tmdbId}.jpg?v=${encodeURIComponent(version)}`
}

export function backdropURL(tmdbId, version) {
  return `/img/backdrop/${tmdbId}.jpg?v=${encodeURIComponent(version)}`
}

export function candidatePosterURL(c) {
  return `/img/poster/${c.tmdbId}.jpg?p=${encodeURIComponent(c.posterPath)}`
}
```

Crear `web/src/lib/nav.svelte.js`:

```js
// The current route, kept in sync with the address bar.

export const route = $state({ path: location.pathname, search: location.search })

// patchState merges data into the current history entry (the scroll
// position, how much of a list is shown…), so going back restores it.
export function patchState(data) {
  history.replaceState({ ...history.state, ...data }, '')
}

// navigate goes to a path inside the app. replace rewrites the current
// entry instead of adding one (used to correct the URL).
export function navigate(url, { replace = false } = {}) {
  if (replace) {
    history.replaceState(history.state, '', url)
  } else {
    patchState({ scroll: window.scrollY })
    history.pushState({}, '', url)
    window.scrollTo(0, 0)
  }
  route.path = location.pathname
  route.search = location.search
}

// back returns to the previous page, or to Explorar when there is none.
export function back() {
  if (history.length > 1) history.back()
  else navigate('/explorar')
}

// savedScroll is the scroll position to restore on this entry (0 for a new
// one).
export function savedScroll() {
  return history.state?.scroll ?? 0
}

history.scrollRestoration = 'manual'
window.addEventListener('popstate', () => {
  route.path = location.pathname
  route.search = location.search
})
```

Crear `web/src/lib/app.svelte.js`:

```js
// State shared by the whole app: the server's status and the notices.
import { api } from './api.js'
import { busy, progress } from './status.js'

export const app = $state({
  status: null,
  // generation grows each time a scan or an identification run ends, and
  // every 20 s while a long one makes progress: pages read it to load their
  // data again.
  generation: 0,
  notices: [],
})

let timer
let wasBusy = false
let lastProgress = ''
let lastRefresh = 0

async function poll() {
  clearTimeout(timer)
  try {
    const st = await api.status()
    const now = busy(st)
    const moved = progress(st) !== lastProgress
    if ((wasBusy && !now) || (now && moved && Date.now() - lastRefresh > 20000)) {
      app.generation++
      lastRefresh = Date.now()
    }
    wasBusy = now
    lastProgress = progress(st)
    app.status = st
  } catch {
    // The next poll tries again.
  }
  timer = setTimeout(poll, wasBusy ? 3000 : 30000)
}

// watchStatus starts polling /api/status: every 3 s while something runs,
// every 30 s otherwise.
export function watchStatus() {
  poll()
}

// refreshStatus asks for the status now, after an action that may have
// started a scan or an identification run.
export function refreshStatus() {
  poll()
}

let seq = 0

// notify shows a notice for 6 seconds.
export function notify(text) {
  const id = ++seq
  app.notices.push({ id, text })
  setTimeout(() => {
    app.notices = app.notices.filter((n) => n.id !== id)
  }, 6000)
}
```

- [ ] **Step 2: Barra superior e indicador de estado**

Crear `web/src/components/Nav.svelte`:

```svelte
<script>
  import Estado from './Estado.svelte'

  let { page } = $props()
</script>

<header>
  <nav>
    <a class="brand" href="/explorar">Cinexplorer</a>
    <a href="/explorar" class:active={page === 'explorar'}>Explorar</a>
    <a href="/revisar" class:active={page === 'revisar'}>Revisar</a>
    <span class="spacer"></span>
    <Estado />
  </nav>
</header>

<style>
  header {
    position: sticky;
    top: 0;
    z-index: 20;
    background: rgba(20, 23, 28, 0.94);
    backdrop-filter: blur(6px);
    border-bottom: 1px solid var(--line);
    margin-bottom: 16px;
  }
  nav {
    max-width: 1400px;
    margin: 0 auto;
    padding: 12px var(--gutter);
    display: flex;
    align-items: center;
    gap: 20px;
    font-size: 13px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .brand {
    color: var(--strong);
    font-weight: 700;
    letter-spacing: 0.14em;
  }
  .active {
    color: var(--strong);
  }
  .spacer {
    flex: 1;
  }
  @media (max-width: 640px) {
    nav {
      gap: 14px;
      letter-spacing: 0.04em;
    }
  }
</style>
```

Crear `web/src/components/Estado.svelte`:

```svelte
<script>
  import { api } from '../lib/api.js'
  import { app, notify, refreshStatus } from '../lib/app.svelte.js'
  import { summary } from '../lib/status.js'

  let open = $state(false)
  let box = $state()

  const st = $derived(app.status)
  const sum = $derived(summary(st))

  async function rescan() {
    try {
      await api.scan()
      refreshStatus()
    } catch (e) {
      notify(e.message)
    }
  }

  function onwindowclick(event) {
    if (open && box && !box.contains(event.target)) open = false
  }
</script>

<svelte:window onclick={onwindowclick} onkeydown={(e) => e.key === 'Escape' && (open = false)} />

{#if st}
  <div class="estado" bind:this={box}>
    <button class="summary {sum.tone}" onclick={() => (open = !open)} aria-expanded={open}>
      <span class="dot"></span><span class="text">{sum.text}</span>
    </button>
    {#if open}
      <div class="panel">
        {#if st.readOnly}
          <p>El directorio de la app no se puede escribir: se muestra el catálogo tal como está, sin escanear ni identificar.</p>
        {/if}
        {#if st.scan}
          <p>
            <span class="label">Escaneo</span><br />
            {#if st.scan.running}
              En curso: {st.scan.files} archivos, {st.scan.hashed} con huella nueva{st.scan.toProbe
                ? `, ${st.scan.probed}/${st.scan.toProbe} analizados`
                : ''}.
            {:else if st.scan.finished && !st.scan.finished.startsWith('0001')}
              Último: {new Date(st.scan.finished).toLocaleString('es')} · {st.scan.versions} versiones.
            {:else}
              Todavía no se escaneó.
            {/if}
            {#if st.scan.lastError}<br /><span class="error">{st.scan.lastError}</span>{/if}
          </p>
        {/if}
        {#if st.identify}
          <p>
            <span class="label">Identificación</span><br />
            {sum.tone === 'busy' || st.identify.state === 'idle'
              ? `${st.identify.identified}/${st.identify.toIdentify} identificadas, ${st.identify.enriched}/${st.identify.toEnrich} completadas.`
              : sum.text}
          </p>
        {/if}
        {#if !st.readOnly}
          <button onclick={rescan} disabled={st.scan?.running}>Volver a escanear</button>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .estado {
    position: relative;
    text-transform: none;
    letter-spacing: 0;
  }
  .summary {
    border: none;
    padding: 4px 8px;
    color: var(--muted);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--best);
  }
  .busy .dot {
    background: var(--accent);
    animation: pulse 1.2s ease-in-out infinite;
  }
  .warn {
    color: var(--warn);
  }
  .warn .dot {
    background: var(--warn);
  }
  @keyframes pulse {
    50% {
      opacity: 0.3;
    }
  }
  .panel {
    position: absolute;
    right: 0;
    top: calc(100% + 8px);
    width: min(340px, calc(100vw - 32px));
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 12px 14px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.55);
    color: var(--text);
    font-size: 14px;
  }
  @media (max-width: 640px) {
    .text {
      display: none;
    }
  }
  .panel p {
    margin: 0 0 12px;
  }
  .error {
    color: var(--warn);
  }
</style>
```

- [ ] **Step 3: Páginas provisorias y App**

Crear `web/src/pages/Explorar.svelte`:

```svelte
<p class="empty">Explorar</p>
```

Crear `web/src/pages/Pelicula.svelte`:

```svelte
<script>
  let { id } = $props()
</script>

<p class="empty">Película {id}</p>
```

Crear `web/src/pages/Version.svelte`:

```svelte
<script>
  let { key } = $props()
</script>

<p class="empty">Versión {key}</p>
```

Crear `web/src/pages/Revisar.svelte`:

```svelte
<script>
  let { tab } = $props()
</script>

<p class="empty">Revisar: {tab}</p>
```

Reemplazá `web/src/App.svelte` completo por:

```svelte
<script>
  import Nav from './components/Nav.svelte'
  import { app, watchStatus } from './lib/app.svelte.js'
  import { navigate, route } from './lib/nav.svelte.js'
  import { appLink, resolve } from './lib/router.js'
  import Explorar from './pages/Explorar.svelte'
  import Pelicula from './pages/Pelicula.svelte'
  import Revisar from './pages/Revisar.svelte'
  import Version from './pages/Version.svelte'

  const current = $derived(resolve(route.path))

  $effect(() => {
    if (current.page === 'redirect') navigate(current.to, { replace: true })
  })

  watchStatus()

  // Links inside the app change the route instead of reloading the page.
  function onclick(event) {
    const to = appLink(event, event.target.closest?.('a'), location.origin)
    if (to !== null) {
      event.preventDefault()
      navigate(to)
    }
  }
</script>

<svelte:window {onclick} />

<Nav page={current.page} />

<main>
  {#if current.page === 'explorar'}
    <Explorar />
  {:else if current.page === 'pelicula'}
    {#key current.id}
      <Pelicula id={current.id} />
    {/key}
  {:else if current.page === 'version'}
    {#key current.key}
      <Version key={current.key} />
    {/key}
  {:else if current.page === 'revisar'}
    <Revisar tab={current.tab} />
  {:else if current.page === 'notfound'}
    <p class="empty">No existe esta página. <a href="/explorar">Ir a Explorar</a></p>
  {/if}
</main>

<div class="notices" aria-live="polite">
  {#each app.notices as n (n.id)}
    <div class="notice">{n.text}</div>
  {/each}
</div>

<style>
  .notices {
    position: fixed;
    top: 64px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 30;
    width: min(560px, calc(100% - 32px));
  }
  .notice {
    background: var(--surface-2);
    border: 1px solid var(--warn);
    border-radius: var(--radius);
    padding: 8px 12px;
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
  }
</style>
```

- [ ] **Step 4: Tests y build**

Run: `cd web && npm test && npm run build`
Expected: tests en verde; `✓ built` sin advertencias `[vite-plugin-svelte]`.

Run: `go test ./internal/server ./cmd/cinexplorer`
Expected: `ok` en los dos.

- [ ] **Step 5: Probar en el navegador**

Run (en otra terminal, desde la raíz del repo): `go run ./cmd/cinexplorer -dir .run -port 8080 -no-browser`
(`.run/` está en `.gitignore`; sin `config.json` la app crea uno con las carpetas hermanas de `.run` como raíces. Para una prueba útil, editá `.run/config.json` y dejá en `roots` una carpeta chica con películas, relativa a `.run`, y reiniciá.)

Abrí `http://127.0.0.1:8080/`. Expected: redirige a `/explorar` y muestra "Explorar"; la barra dice CINEXPLORER · EXPLORAR · REVISAR y a la derecha el indicador ("Escaneando…", "Falta el token de TMDB" o "Al día"); "Revisar" navega sin recargar la página; atrás vuelve a Explorar; `/cualquiera` muestra "No existe esta página". Cortá el servidor con Ctrl+C.

- [ ] **Step 6: Commit**

```bash
git add web internal/server/dist
git commit -m "feat(web): app shell with routes, API client, status and navigation"
```

---

### Task 15: Explorar

**Files:**
- Create: `web/src/components/Afiche.svelte`, `web/src/components/BarraFacetas.svelte`
- Modify: `web/src/pages/Explorar.svelte` (completo)

Spec §9.2. `Explorar` pide `/api/explore` con la búsqueda de la URL cada vez que esta cambia (y cuando sube `generation`); si el servidor descartó facetas mal formadas, corrige la URL con `replaceState`. Dibuja la grilla en tandas de 120 con un centinela (`IntersectionObserver`) y, al volver atrás, restaura cuántas tarjetas había y el scroll. `BarraFacetas`: un desplegable por faceta con valores y conteos (con filtro de texto si hay más de 12; los años de la década elegida dentro del menú de Década), "Más ▾" con las cuatro facetas restantes, chips de las facetas activas con ✕, contador, orden y dirección. `Afiche`: afiche 2:3 con título, título original (si difiere) y año; lo no identificado y lo que no tiene afiche usan uno genérico (rayado y con borde punteado si no está identificado).

- [ ] **Step 1: Componentes**

Crear `web/src/components/Afiche.svelte`:

```svelte
<script>
  import { posterURL } from '../lib/api.js'
  import { sameTitle } from '../lib/format.js'
  import { itemHref } from '../lib/router.js'

  let { item } = $props()
  let failed = $state(false)

  const unidentified = $derived(item.kind !== 'movie')
  const showPoster = $derived(!unidentified && item.poster && !failed)
</script>

<a class="card" href={itemHref(item)}>
  {#if showPoster}
    <img
      class="poster"
      src={posterURL(item.tmdbId, item.poster)}
      alt={`${item.title} (${item.year || 's/f'})`}
      loading="lazy"
      onerror={() => (failed = true)}
    />
  {:else}
    <div class="poster generic" class:unidentified>
      <span class="gtitle">{item.title || 'Sin título'}</span>
      {#if unidentified}<small>sin identificar</small>{/if}
    </div>
  {/if}
  <div class="title">{item.title}</div>
  {#if item.originalTitle && !sameTitle(item.title, item.originalTitle)}
    <div class="original">{item.originalTitle}</div>
  {/if}
  <div class="year">{item.year || ''}{unidentified && item.year ? ' ?' : ''}</div>
</a>

<style>
  .card {
    display: block;
    min-width: 0;
  }
  .poster {
    display: block;
    width: 100%;
    aspect-ratio: 2 / 3;
    object-fit: cover;
    border-radius: 3px;
    background: var(--surface);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
  }
  .card:hover .poster {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .generic {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    text-align: center;
    padding: 10px;
    background: linear-gradient(160deg, #262c35, #15181d);
  }
  .generic.unidentified {
    background: repeating-linear-gradient(45deg, #1b1f25, #1b1f25 8px, #1f242b 8px, #1f242b 16px);
    border: 1px dashed #475060;
  }
  .gtitle {
    font-size: 13px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
    overflow-wrap: anywhere;
  }
  small {
    margin-top: 6px;
    color: var(--warn);
    font-size: 12px;
  }
  .title {
    margin-top: 6px;
    font-size: 13px;
    color: var(--strong);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .original {
    font-size: 12px;
    color: var(--faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .year {
    font-size: 12px;
    color: var(--faint);
  }
</style>
```

Crear `web/src/components/BarraFacetas.svelte`:

```svelte
<script>
  import { chips, FACETS, ORDERS, valueLabel, withFacet, withOrder } from '../lib/facets.js'

  // query: the applied query; facets: the server's counts per facet;
  // onchange(query): a new query was chosen.
  let { query, facets, total, onchange } = $props()

  let open = $state(null) // name of the open menu, "more", or null
  let filter = $state('')
  let bar = $state()

  const main = FACETS.filter((f) => !f.more)
  const more = FACETS.filter((f) => f.more)
  const applied = $derived(chips(query, facets))

  function toggle(name) {
    open = open === name ? null : name
    filter = ''
  }

  function choose(name, value) {
    open = null
    onchange(withFacet(query, name, query.facets[name] === value ? null : value))
  }

  function values(name) {
    const all = facets?.[name] ?? []
    const f = filter.trim().toLowerCase()
    if (!f) return all
    return all.filter((v) => valueLabel(name, v.value, v).toLowerCase().includes(f))
  }

  // The years of the chosen decade, inside the decade menu.
  const years = $derived((facets?.anio ?? []).filter((v) => query.facets.decada && v.value.startsWith(query.facets.decada.slice(0, 3))))

  function onwindowclick(event) {
    if (open && bar && !bar.contains(event.target)) open = null
  }
</script>

<svelte:window onclick={onwindowclick} onkeydown={(e) => e.key === 'Escape' && (open = null)} />

{#snippet menu(name)}
  {@const list = values(name)}
  {#if (facets?.[name] ?? []).length > 12}
    <!-- svelte-ignore a11y_autofocus -->
    <input type="search" placeholder="Filtrar…" bind:value={filter} autofocus />
  {/if}
  <ul>
    {#each list as v (v.value)}
      <li>
        <button class="option" class:chosen={query.facets[name] === v.value} onclick={() => choose(name, v.value)}>
          <span>{valueLabel(name, v.value, v)}</span><span class="count">{v.count}</span>
        </button>
      </li>
    {:else}
      <li class="none">Sin valores</li>
    {/each}
  </ul>
{/snippet}

<div class="bar" bind:this={bar}>
  {#each main as f (f.name)}
    <div class="facet">
      <button class="dd" class:on={query.facets[f.name]} onclick={() => toggle(f.name)} aria-expanded={open === f.name}>
        {f.label} ▾
      </button>
      {#if open === f.name}
        <div class="menu">
          {@render menu(f.name)}
          {#if f.name === 'decada' && years.length}
            <div class="label sub">Años</div>
            <ul>
              {#each years as y (y.value)}
                <li>
                  <button class="option" class:chosen={query.facets.anio === y.value} onclick={() => choose('anio', y.value)}>
                    <span>{y.value}</span><span class="count">{y.count}</span>
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}
    </div>
  {/each}
  <div class="facet">
    <button class="dd" class:on={more.some((f) => query.facets[f.name])} onclick={() => toggle('more')} aria-expanded={open === 'more'}>
      Más ▾
    </button>
    {#if open === 'more'}
      <div class="menu wide">
        {#each more as f (f.name)}
          <div class="label sub">{f.label}</div>
          {@render menu(f.name)}
        {/each}
      </div>
    {/if}
  </div>

  <div class="right">
    <span class="total">{total} {total === 1 ? 'película' : 'películas'}</span>
    <select value={query.order} onchange={(e) => onchange(withOrder(query, e.currentTarget.value))} aria-label="Orden">
      {#each ORDERS as o (o.value)}
        <option value={o.value}>{o.label}</option>
      {/each}
    </select>
    <button
      class="dir"
      onclick={() => onchange({ ...query, dir: query.dir === 'asc' ? 'desc' : 'asc' })}
      aria-label={query.dir === 'asc' ? 'Ascendente' : 'Descendente'}
      title={query.dir === 'asc' ? 'Ascendente' : 'Descendente'}>{query.dir === 'asc' ? '↑' : '↓'}</button
    >
  </div>
</div>

{#if applied.length}
  <div class="chips">
    {#each applied as c (c.name)}
      <button class="chip" onclick={() => onchange(withFacet(query, c.name, null))} aria-label={`Quitar ${c.label}`}>
        {c.label} ✕
      </button>
    {/each}
    {#if applied.length > 1}
      <button class="chip clear" onclick={() => onchange({ ...query, facets: {} })}>Quitar todo</button>
    {/if}
  </div>
{/if}

<style>
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--line);
    font-size: 13px;
  }
  .facet {
    position: relative;
  }
  .dd {
    color: var(--muted);
    border-color: var(--line);
    padding: 3px 10px;
  }
  .dd.on {
    color: var(--accent);
    border-color: var(--accent);
  }
  .menu {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    z-index: 15;
    min-width: 220px;
    max-height: 60vh;
    overflow-y: auto;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 6px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.55);
  }
  .menu.wide {
    min-width: 260px;
  }
  .menu input {
    width: 100%;
    margin-bottom: 6px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .option {
    width: 100%;
    display: flex;
    justify-content: space-between;
    gap: 12px;
    border: none;
    padding: 4px 8px;
    text-align: left;
  }
  .option:hover {
    background: var(--surface-2);
  }
  .option.chosen {
    color: var(--accent);
  }
  .count {
    color: var(--faint);
  }
  .none {
    color: var(--faint);
    padding: 4px 8px;
  }
  .sub {
    padding: 8px 8px 2px;
  }
  .right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .total {
    color: var(--faint);
  }
  .dir {
    padding: 3px 9px;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 10px;
  }
  .chip {
    border-color: var(--accent);
    color: var(--accent);
    font-size: 13px;
    padding: 2px 10px;
    border-radius: 12px;
  }
  .chip.clear {
    border-color: var(--line-strong);
    color: var(--muted);
  }
  @media (max-width: 640px) {
    .right {
      margin-left: 0;
      width: 100%;
    }
    .menu {
      position: fixed;
      left: 16px;
      right: 16px;
      top: 120px;
    }
  }
</style>
```

- [ ] **Step 2: Página**

Reemplazá `web/src/pages/Explorar.svelte` completo por:

```svelte
<script>
  import { tick } from 'svelte'
  import Afiche from '../components/Afiche.svelte'
  import BarraFacetas from '../components/BarraFacetas.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { parse, toSearch } from '../lib/facets.js'
  import { navigate, patchState, route, savedScroll } from '../lib/nav.svelte.js'

  const BATCH = 120

  let data = $state(null)
  let shown = $state(BATCH)
  let sentinel = $state()
  let loadedSearch = null

  const query = $derived(parse(route.search))

  // Load on every change of the URL, and again when a scan or an
  // identification run ends (same URL: the list refreshes in place).
  $effect(() => {
    const search = route.search
    app.generation
    load(search)
  })

  async function load(search) {
    let d
    try {
      d = await api.explore(search)
    } catch (e) {
      notify(e.message)
      return
    }
    if (search !== route.search || route.path !== '/explorar') return // a newer request is on its way
    const fresh = search !== loadedSearch
    data = d
    loadedSearch = search
    // The server drops malformed facets: show the URL of what it applied.
    const applied = toSearch(d.query)
    if (applied !== toSearch(parse(search))) {
      navigate('/explorar' + applied, { replace: true })
      return
    }
    if (fresh) {
      shown = Math.max(BATCH, history.state?.shown ?? BATCH)
      await tick()
      window.scrollTo(0, savedScroll())
    }
  }

  function change(q) {
    navigate('/explorar' + toSearch(q))
  }

  // Draw the next batch when the end of the grid comes into view.
  $effect(() => {
    if (!sentinel) return
    const io = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting && data && shown < data.items.length) {
        shown += BATCH
        patchState({ shown })
      }
    }, { rootMargin: '800px' })
    io.observe(sentinel)
    return () => io.disconnect()
  })

  const scanning = $derived(!!app.status?.scan?.running)
  const filtered = $derived(Object.keys(query.facets).length > 0)
</script>

<svelte:head><title>Explorar · Cinexplorer</title></svelte:head>

{#if data}
  <BarraFacetas {query} facets={data.facets} total={data.total} onchange={change} />
  {#if data.items.length}
    <div class="grid">
      {#each data.items.slice(0, shown) as item (item.kind === 'movie' ? `m${item.tmdbId}` : `v${item.key}`)}
        <Afiche {item} />
      {/each}
    </div>
    <div bind:this={sentinel}></div>
  {:else if filtered}
    <p class="empty">Nada con estos filtros. <a href="/explorar">Quitar filtros</a></p>
  {:else if scanning}
    <p class="empty">Escaneando… Las películas van a aparecer a medida que se encuentren.</p>
  {:else}
    <p class="empty">El catálogo está vacío. Revisá las raíces en config.json y volvé a escanear.</p>
  {/if}
{/if}

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 20px 14px;
    margin-top: 16px;
  }
  @media (max-width: 640px) {
    .grid {
      grid-template-columns: repeat(auto-fill, minmax(104px, 1fr));
      gap: 14px 10px;
    }
  }
</style>
```

- [ ] **Step 3: Tests y build**

Run: `cd web && npm test && npm run build`
Expected: tests en verde; `✓ built` sin advertencias.

Run: `go test ./internal/server ./cmd/cinexplorer`
Expected: `ok`.

- [ ] **Step 4: Probar en el navegador**

Con la app corriendo como en la Task 14 (reiniciá `go run` para que embeba el build nuevo), en `/explorar`:
- La grilla muestra la colección; lo no identificado, con afiche rayado y "sin identificar".
- Elegí un director: la URL pasa a `?director=<id>`, aparece el chip con su nombre y el contador baja. El ✕ del chip lo quita. Atrás/adelante recorren los filtros.
- `/explorar?decada=1975&pais=IT` se corrige solo a `?pais=IT` (1975 no es una década).
- Cambiá el orden a Título y la dirección con ↓/↑.
- A 375 px de ancho (herramientas del navegador), sin scroll horizontal.

- [ ] **Step 5: Commit**

```bash
git add web internal/server/dist
git commit -m "feat(web): Explorar with facets in the URL"
```

---

### Task 16: Ficha de película e identificación

**Files:**
- Create: `web/src/components/Candidatos.svelte`, `web/src/components/SelectorExtra.svelte`, `web/src/components/Identificar.svelte`, `web/src/components/TarjetaVersion.svelte`
- Modify: `web/src/pages/Pelicula.svelte` (completo)

Spec §9.3. `Identificar` es el bloque que decide qué es una versión: candidatos (un clic confirma), búsqueda manual en TMDB (título o `tt…`), "No es una película" y "Es un extra de…" (con `SelectorExtra`, que sugiere películas del catálogo empezando por las de la misma carpeta). Con `active`, responde a los atajos de la cola; sin TMDB (sin token, sin red, modo consulta) deshabilita la búsqueda y los candidatos y dice por qué. `TarjetaVersion`: resolución grande, MEJOR y COPIA IDÉNTICA ×n, línea técnica, audio y subtítulos, ruta, ▶ Ver, Carpeta y el menú ⋯ (Cambiar película, Es un extra de, No es una película y, si hay una corrección, Volver a automática); una versión cuyos archivos faltan se ve atenuada y sin botones. `Pelicula`: cabecera con la escena de fondo, afiche, título en mayúsculas, original · año, y director, países, géneros y colección como enlaces a Explorar; sinopsis, reparto, versiones y extras.

- [ ] **Step 1: Componentes de identificación**

Crear `web/src/components/Candidatos.svelte`:

```svelte
<script>
  import { candidatePosterURL } from '../lib/api.js'
  import { percent, sameTitle } from '../lib/format.js'

  // list: TMDB candidates {tmdbId, title, originalTitle, year, posterPath,
  // score}; onpick(candidate); numbered: show 1–5 for the keyboard.
  let { list, onpick, numbered = false, disabled = false } = $props()
</script>

<div class="cands">
  {#each list as c, i (c.tmdbId)}
    <button class="cand" onclick={() => onpick(c)} {disabled} title={`Es ${c.title} (${c.year || 's/f'})`}>
      {#if c.posterPath}
        <img src={candidatePosterURL(c)} alt="" loading="lazy" />
      {:else}
        <div class="noimg"></div>
      {/if}
      <div class="ct">
        {#if numbered && i < 5}<span class="n">{i + 1}</span>{/if}
        {c.title}
      </div>
      <div class="cm">
        {c.year || 's/f'}{c.originalTitle && !sameTitle(c.title, c.originalTitle) ? ` · ${c.originalTitle}` : ''}
      </div>
      <div class="pct">{percent(c.score)}</div>
    </button>
  {/each}
</div>

<style>
  .cands {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(118px, 1fr));
    gap: 10px;
  }
  .cand {
    display: block;
    text-align: left;
    padding: 6px;
    border-color: var(--line);
    min-width: 0;
  }
  .cand:hover:not(:disabled) {
    border-color: var(--accent);
  }
  img,
  .noimg {
    display: block;
    width: 100%;
    aspect-ratio: 2 / 3;
    object-fit: cover;
    border-radius: 2px;
    background: var(--surface-2);
  }
  .ct {
    margin-top: 5px;
    font-size: 13px;
    color: var(--strong);
    line-height: 1.25;
  }
  .n {
    display: inline-block;
    min-width: 16px;
    margin-right: 4px;
    font-size: 11px;
    text-align: center;
    border: 1px solid var(--line-strong);
    border-radius: 3px;
    color: var(--muted);
  }
  .cm {
    font-size: 12px;
    color: var(--faint);
  }
  .pct {
    font-size: 12px;
    color: var(--accent);
    font-weight: 600;
  }
</style>
```

Crear `web/src/components/SelectorExtra.svelte`:

```svelte
<script>
  import { api } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'

  // near: the version key whose folder suggests movies first;
  // onpick(movie): a movie of the catalog was chosen.
  let { near, onpick, disabled = false } = $props()

  let q = $state('')
  let list = $state([])
  let timer

  async function load() {
    try {
      list = await api.suggest(q, near)
    } catch (e) {
      notify(e.message)
    }
  }

  $effect(() => {
    q
    clearTimeout(timer)
    timer = setTimeout(load, 200)
    return () => clearTimeout(timer)
  })
</script>

<div class="selector">
  <!-- svelte-ignore a11y_autofocus -->
  <input type="search" placeholder="¿De qué película es? (título)" bind:value={q} autofocus />
  <ul>
    {#each list as m (m.tmdbId)}
      <li>
        <button onclick={() => onpick(m)} {disabled}>
          {m.title}
          <span class="meta">{m.year || 's/f'}{m.directors.length ? ` · ${m.directors.map((d) => d.name).join(', ')}` : ''}</span>
        </button>
      </li>
    {:else}
      <li class="none">Ninguna película del catálogo coincide.</li>
    {/each}
  </ul>
</div>

<style>
  .selector {
    margin-top: 10px;
    max-width: 520px;
  }
  input {
    width: 100%;
  }
  ul {
    list-style: none;
    margin: 6px 0 0;
    padding: 0;
    max-height: 260px;
    overflow-y: auto;
  }
  li button {
    width: 100%;
    text-align: left;
    border: none;
    padding: 5px 8px;
  }
  li button:hover:not(:disabled) {
    background: var(--surface-2);
  }
  .meta {
    color: var(--faint);
    margin-left: 6px;
    font-size: 13px;
  }
  .none {
    color: var(--faint);
    padding: 5px 8px;
  }
</style>
```

Crear `web/src/components/Identificar.svelte`:

```svelte
<script>
  import { api } from '../lib/api.js'
  import { app, notify, refreshStatus } from '../lib/app.svelte.js'
  import { keyAction } from '../lib/keys.js'
  import { tmdbProblem } from '../lib/status.js'
  import Candidatos from './Candidatos.svelte'
  import SelectorExtra from './SelectorExtra.svelte'

  // What a version is: one of its candidates, a movie found by searching
  // TMDB, not a movie, or an extra of a movie of the catalog.
  //   fingerprint: the version's; candidates: the matcher's (may be empty);
  //   active: answers the queue's keyboard shortcuts;
  //   ondone({action, tmdbId}): the decision was saved (action "gone" when
  //   the version no longer exists);
  //   startWith: "extra" opens the movie selector, "search" focuses the
  //   search field.
  let { fingerprint, candidates = [], active = false, startWith = '', ondone } = $props()

  let q = $state('')
  let results = $state(null)
  let searching = $state(false)
  let busy = $state(false)
  // svelte-ignore state_referenced_locally (only the initial value counts)
  let extra = $state(startWith === 'extra')
  let input = $state()

  $effect(() => {
    if (startWith === 'search') input?.focus()
  })

  const problem = $derived(tmdbProblem(app.status))

  async function search(event) {
    event?.preventDefault()
    if (!q.trim()) return
    searching = true
    try {
      results = await api.search(q.trim())
    } catch (e) {
      notify(e.message)
    } finally {
      searching = false
    }
  }

  async function act(action, tmdbId = 0) {
    if (busy) return
    busy = true
    try {
      await api.identify(fingerprint, action, tmdbId)
      refreshStatus()
      ondone({ action, tmdbId })
    } catch (e) {
      // The server's text says what failed: an unknown fingerprint (the
      // file changed since the page loaded), a movie TMDB does not have, no
      // network, no token, read-only mode.
      notify(e.message)
      if (e.status === 404 && e.message === 'huella desconocida') ondone({ action: 'gone' })
    } finally {
      busy = false
    }
  }

  function onkeydown(event) {
    if (!active) return
    const a = keyAction(event)
    if (!a) return
    if (a.type === 'pick' && candidates[a.index] && !problem) {
      event.preventDefault()
      act('movie', candidates[a.index].tmdbId)
    } else if (a.type === 'search' && !problem) {
      event.preventDefault()
      input?.focus()
    } else if (a.type === 'ignore') {
      event.preventDefault()
      act('ignore')
    } else if (a.type === 'extra') {
      event.preventDefault()
      extra = !extra
    }
  }
</script>

<svelte:window {onkeydown} />

<div class="identificar">
  {#if problem}
    <p class="problem">{problem}</p>
  {/if}
  {#if candidates.length}
    <Candidatos list={candidates} numbered={active} disabled={busy || !!problem} onpick={(c) => act('movie', c.tmdbId)} />
  {/if}
  <form class="search" onsubmit={search}>
    <input
      type="search"
      placeholder="Buscar en TMDB (título o tt…)"
      bind:value={q}
      bind:this={input}
      disabled={!!problem}
    />
    <button type="submit" disabled={!!problem || searching || !q.trim()}>{searching ? 'Buscando…' : 'Buscar'}</button>
    <span class="gap"></span>
    <button type="button" onclick={() => act('ignore')} disabled={busy}>No es una película</button>
    <button type="button" onclick={() => (extra = !extra)} aria-expanded={extra} disabled={busy}>Es un extra de…</button>
  </form>
  {#if results}
    {#if results.length}
      <Candidatos list={results} disabled={busy} onpick={(c) => act('movie', c.tmdbId)} />
    {:else}
      <p class="none">TMDB no encontró nada con «{q}».</p>
    {/if}
  {/if}
  {#if extra}
    <SelectorExtra near={fingerprint} disabled={busy} onpick={(m) => act('extra', m.tmdbId)} />
  {/if}
</div>

<style>
  .identificar {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .search {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    align-items: center;
  }
  .search input {
    flex: 1 1 220px;
    min-width: 0;
  }
  .gap {
    flex: 0 0 8px;
  }
  .problem {
    margin: 0;
    color: var(--warn);
  }
  .none {
    margin: 0;
    color: var(--muted);
  }
</style>
```

- [ ] **Step 2: Tarjeta de versión y página**

Crear `web/src/components/TarjetaVersion.svelte`:

```svelte
<script>
  import { api } from '../lib/api.js'
  import { app, notify, refreshStatus } from '../lib/app.svelte.js'
  import { languages, mainFile, resolution, subtitles, versionLine } from '../lib/format.js'
  import Identificar from './Identificar.svelte'

  // v: a version card (a version plus `copies`); corrections: show the ⋯
  // menu; onchanged(result): its identification changed.
  let { v, corrections = true, onchanged } = $props()

  let menu = $state(false)
  let panel = $state('') // "search" or "extra": the identification block
  let box = $state()

  const file = $derived(mainFile(v))
  const status = $derived(v.identification?.status ?? '')
  const correctable = $derived(corrections && !app.status?.readOnly && v.fingerprint !== '')
  const corrected = $derived(['manual', 'ignored', 'extra'].includes(status))
  const audio = $derived(languages(v.audio))
  const subs = $derived(subtitles(v))

  async function run(action) {
    try {
      await (action === 'open' ? api.open(file.path) : api.reveal(file.path))
    } catch (e) {
      notify(e.message)
    }
  }

  async function correct(action) {
    menu = false
    try {
      await api.identify(v.fingerprint, action)
      refreshStatus()
      onchanged({ action })
    } catch (e) {
      notify(e.message)
    }
  }

  function done(result) {
    panel = ''
    onchanged(result)
  }

  function onwindowclick(event) {
    if (menu && box && !box.contains(event.target)) menu = false
  }
</script>

<svelte:window onclick={onwindowclick} />

<article class="card" class:missing={!file}>
  <div class="res">
    <span class="big">{resolution(v.resolution)}</span>
    {#if v.best}<span class="tag best">Mejor</span>{/if}
    {#if v.copies > 1}<span class="tag same">Copia idéntica ×{v.copies}</span>{/if}
    {#if !file}<span class="tag gone">No encontrado</span>{/if}
  </div>
  <div class="info">
    <div>{versionLine(v)}</div>
    {#if audio || subs}
      <div class="tracks">
        {#if audio}Audio: {audio}{/if}{#if audio && subs}&nbsp;·&nbsp;{/if}{#if subs}Subs: {subs}{/if}
      </div>
    {/if}
    <div class="path">{(file ?? v.files[0])?.path ?? v.dir}</div>
  </div>
  <div class="actions" bind:this={box}>
    {#if file}
      <button class="primary" onclick={() => run('open')}>▶ Ver</button>
      <button onclick={() => run('reveal')}>Carpeta</button>
    {/if}
    {#if correctable}
      <button onclick={() => (menu = !menu)} aria-expanded={menu} aria-label="Corregir">⋯</button>
      {#if menu}
        <div class="menu">
          <button onclick={() => ((panel = 'search'), (menu = false))}>Cambiar película…</button>
          <button onclick={() => ((panel = 'extra'), (menu = false))}>Es un extra de…</button>
          <button onclick={() => correct('ignore')}>No es una película</button>
          {#if corrected}
            <button onclick={() => correct('reset')}>Volver a automática</button>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
  {#if panel}
    <div class="panel">
      {#key panel}
        <Identificar fingerprint={v.fingerprint} startWith={panel} ondone={done} />
      {/key}
      <button class="close" onclick={() => (panel = '')}>Cancelar</button>
    </div>
  {/if}
</article>

<style>
  .card {
    display: grid;
    grid-template-columns: 110px 1fr auto;
    gap: 14px;
    align-items: center;
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 12px 14px;
  }
  .missing {
    opacity: 0.55;
  }
  .res {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
  .big {
    font-size: 24px;
    font-weight: 700;
    color: var(--strong);
  }
  .tag {
    font-size: 10px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 1px 6px;
    border-radius: 2px;
  }
  .best {
    background: var(--best-bg);
    color: var(--best);
  }
  .same {
    background: var(--same-bg);
    color: var(--same);
  }
  .gone {
    background: var(--surface-2);
    color: var(--muted);
  }
  .info {
    min-width: 0;
    color: var(--muted);
    font-size: 14px;
  }
  .tracks {
    margin-top: 2px;
  }
  .info .path {
    margin-top: 6px;
  }
  .actions {
    position: relative;
    display: flex;
    gap: 6px;
  }
  .menu {
    position: absolute;
    right: 0;
    top: calc(100% + 6px);
    z-index: 10;
    display: flex;
    flex-direction: column;
    min-width: 210px;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 4px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.55);
  }
  .menu button {
    border: none;
    text-align: left;
    padding: 6px 10px;
  }
  .menu button:hover {
    background: var(--surface);
  }
  .panel {
    grid-column: 1 / -1;
    border-top: 1px solid var(--line);
    padding-top: 12px;
  }
  .close {
    margin-top: 10px;
  }
  @media (max-width: 640px) {
    .card {
      grid-template-columns: 1fr;
    }
  }
</style>
```

Reemplazá `web/src/pages/Pelicula.svelte` completo por:

```svelte
<script>
  import TarjetaVersion from '../components/TarjetaVersion.svelte'
  import { api, backdropURL, posterURL } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { toSearch } from '../lib/facets.js'
  import { country, fileName, runtime, sameTitle, size } from '../lib/format.js'

  let { id } = $props()

  let d = $state(null)
  let missing = $state(false)
  let noPoster = $state(false)

  async function load() {
    try {
      d = await api.movie(id)
      missing = false
    } catch (e) {
      if (e.status === 404) missing = true
      else notify(e.message)
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  const m = $derived(d?.movie)
  const explore = (facets) => '/explorar' + toSearch({ facets, order: 'anio', dir: 'desc' })

  async function open(path) {
    try {
      await api.open(path)
    } catch (e) {
      notify(e.message)
    }
  }
</script>

<svelte:head><title>{m ? `${m.title} · Cinexplorer` : 'Cinexplorer'}</title></svelte:head>

{#if missing}
  <p class="empty">Esta película ya no está en el catálogo. <a href="/explorar">Ir a Explorar</a></p>
{:else if m}
  <section class="hero" style:background-image={d.backdrop ? `url(${backdropURL(m.tmdbId, d.backdrop)})` : null}>
    <div class="shade"></div>
    <div class="head">
      {#if d.poster && !noPoster}
        <img class="poster" src={posterURL(m.tmdbId, d.poster)} alt="" onerror={() => (noPoster = true)} />
      {:else}
        <div class="poster generic"></div>
      {/if}
      <div class="titles">
        <h1>{m.title}</h1>
        <div class="sub">
          {#if m.originalTitle && !sameTitle(m.title, m.originalTitle)}{m.originalTitle} · {/if}{m.year || 's/f'}
        </div>
        <div class="meta">
          {#each m.directors as p, i (i)}
            {#if i > 0},&nbsp;{/if}{#if p.id}<a href={explore({ director: String(p.id) })}>{p.name}</a>{:else}{p.name}{/if}
          {/each}
          {#each m.countries as c (c)}
            <span class="sep">·</span><a href={explore({ pais: c })}>{country(c)}</a>
          {/each}
          {#if m.runtime}<span class="sep">·</span>{runtime(m.runtime)}{/if}
          {#each m.genres as g (g)}
            <span class="sep">·</span><a href={explore({ genero: g })}>{g}</a>
          {/each}
          {#if m.collectionId}
            <span class="sep">·</span><a href={explore({ coleccion: String(m.collectionId) })}>{m.collection}</a>
          {/if}
        </div>
      </div>
    </div>
  </section>

  <div class="body">
    {#if m.overview}<p class="overview">{m.overview}</p>{/if}
    {#if m.cast.length}
      <p class="cast"><span class="label">Reparto</span> {m.cast.map((c) => c.name).join(', ')}</p>
    {/if}

    <h2 class="label">En disco · {d.versions.length} {d.versions.length === 1 ? 'versión' : 'versiones'}</h2>
    <div class="versions">
      {#each d.versions as v (v.id)}
        <TarjetaVersion {v} onchanged={load} />
      {/each}
    </div>

    {#if d.extras.length || d.extraVersions.length}
      <h2 class="label">Extras</h2>
      <ul class="extras">
        {#each d.extras as x (x.path)}
          <li>
            {#if !x.missing}<button onclick={() => open(x.path)} aria-label="Ver">▶</button>{/if}
            <span>{fileName(x.path)}</span><span class="size">{size(x.size)}</span>
          </li>
        {/each}
        {#each d.extraVersions as v (v.id)}
          {@const f = v.files.find((x) => x.role === 'main' && !x.missing)}
          <li>
            {#if f}<button onclick={() => open(f.path)} aria-label="Ver">▶</button>{/if}
            <span>{fileName((f ?? v.files[0])?.path)}</span><span class="size">{size(v.size)}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}

<style>
  .hero {
    position: relative;
    margin: -16px calc(-1 * var(--gutter)) 0;
    min-height: 300px;
    background: linear-gradient(120deg, #2a3038, #14171c 70%) center / cover no-repeat;
    display: flex;
    align-items: flex-end;
  }
  .shade {
    position: absolute;
    inset: 0;
    background: linear-gradient(90deg, rgba(20, 23, 28, 0.95) 25%, rgba(20, 23, 28, 0.35)),
      linear-gradient(0deg, var(--bg), transparent 45%);
  }
  .head {
    position: relative;
    display: flex;
    gap: 24px;
    align-items: flex-end;
    padding: 48px var(--gutter) 0;
    width: 100%;
  }
  .poster {
    width: 170px;
    aspect-ratio: 2 / 3;
    object-fit: cover;
    border-radius: 3px;
    box-shadow: 0 8px 28px rgba(0, 0, 0, 0.6);
    flex: none;
    margin-bottom: -40px;
  }
  .generic {
    background: linear-gradient(160deg, #262c35, #15181d);
  }
  h1 {
    margin: 0;
    font-size: clamp(26px, 4vw, 40px);
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--strong);
    line-height: 1.1;
  }
  .sub {
    color: var(--muted);
    margin-top: 4px;
  }
  .meta {
    color: var(--muted);
    margin-top: 6px;
    font-size: 14px;
  }
  .meta a {
    color: var(--text);
  }
  .sep {
    margin: 0 6px;
    color: var(--faint);
  }
  .body {
    padding-left: calc(170px + 24px);
    margin-top: 16px;
  }
  .overview {
    max-width: 720px;
    color: var(--text);
  }
  .cast {
    max-width: 720px;
    color: var(--muted);
    font-size: 14px;
  }
  h2 {
    margin: 32px 0 10px;
    font-weight: 500;
  }
  .versions {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .extras {
    list-style: none;
    padding: 0;
    margin: 0;
    color: var(--muted);
  }
  .extras li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 0;
  }
  .extras button {
    padding: 0 8px;
  }
  .size {
    color: var(--faint);
    font-size: 13px;
  }
  @media (max-width: 760px) {
    .body {
      padding-left: 0;
      margin-top: 56px;
    }
    .poster {
      width: 110px;
    }
  }
</style>
```

- [ ] **Step 3: Tests y build**

Run: `cd web && npm test && npm run build`
Expected: tests en verde; `✓ built` sin advertencias.

Run: `go test ./internal/server ./cmd/cinexplorer`
Expected: `ok`.

- [ ] **Step 4: Probar en el navegador**

Con un catálogo identificado (token de TMDB en `.run/config.json`), abrí una película desde Explorar:
- Cabecera con escena, afiche y datos; los enlaces de director y país abren Explorar filtrado.
- Una película con varias versiones muestra MEJOR en la primera; una copia idéntica, COPIA IDÉNTICA ×2.
- ▶ Ver abre el archivo con el reproductor del sistema; Carpeta lo muestra en el explorador de archivos.
- ⋯ → Cambiar película… abre la búsqueda; ⋯ → No es una película saca la versión de la ficha (y de Explorar).

- [ ] **Step 5: Commit**

```bash
git add web internal/server/dist
git commit -m "feat(web): movie page with version cards and identification"
```

---

### Task 17: Ficha provisoria y Revisar

**Files:**
- Create: `web/src/components/SinIdentificar.svelte`, `web/src/components/Duplicados.svelte`
- Modify: `web/src/pages/Version.svelte`, `web/src/pages/Revisar.svelte` (completos)

Spec §9.4 y §9.5. `Version`: título y año del nombre, afiche genérico, estado, tarjetas (sin menú ⋯: el bloque `Identificar` de abajo ya tiene las acciones) y, si la versión tiene huella y no es modo consulta, `Identificar`; al confirmar una película navega a su ficha, al marcar "no es película" o "extra de" vuelve atrás. `Revisar`: pestañas con conteos ("Sin identificar (N) · M en espera", "Duplicados (N · X GB)"). `SinIdentificar`: el ítem actual expandido (nombre, ruta, tokens del nombre, `Identificar` con atajos, ▶) y los demás en una línea con el mejor puntaje; al decidir, el ítem sale de la lista y se abre el siguiente. `Duplicados`: total recuperable y un grupo por película o contenido, con tipos, recuperable y una línea por versión.

- [ ] **Step 1: Componentes de Revisar**

Crear `web/src/components/SinIdentificar.svelte`:

```svelte
<script>
  import { tick } from 'svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { fileName, mainFile, percent } from '../lib/format.js'
  import { keyAction } from '../lib/keys.js'
  import Identificar from './Identificar.svelte'

  // items: the queue ({...version, candidates}); onresolved(fingerprint):
  // an item was decided on and leaves the queue.
  let { items, onresolved } = $props()

  let current = $state(0)
  let rows = $state([])

  const readOnly = $derived(!!app.status?.readOnly)

  $effect(() => {
    if (current >= items.length) current = Math.max(0, items.length - 1)
  })

  async function select(i) {
    current = i
    await tick()
    rows[i]?.scrollIntoView({ block: 'nearest' })
  }

  function onkeydown(event) {
    const a = keyAction(event)
    if (a?.type === 'next' && current < items.length - 1) {
      event.preventDefault()
      select(current + 1)
    } else if (a?.type === 'prev' && current > 0) {
      event.preventDefault()
      select(current - 1)
    }
  }

  async function play(it) {
    const f = mainFile(it)
    if (!f) return
    try {
      await api.open(f.path)
    } catch (e) {
      notify(e.message)
    }
  }

  function tokens(it) {
    return [
      ['título', it.title],
      ['año', it.year || ''],
      ['director', it.director],
      ['resolución', it.resolution],
      ['origen', it.source],
      ['codec', it.codec],
      ['idioma', it.language],
    ].filter(([, v]) => v)
  }
</script>

<svelte:window {onkeydown} />

{#if items.length === 0}
  <p class="empty">No hay nada para revisar.</p>
{:else}
  {#if !readOnly}
    <p class="keys">↑↓ cambiar de ítem · 1–5 confirmar candidato · / buscar · N no es película · E extra de…</p>
  {/if}
  <ol>
    {#each items as it, i (it.fingerprint)}
      {@const f = mainFile(it)}
      <li bind:this={rows[i]} class:open={i === current}>
        {#if i === current}
          <div class="item">
            <div class="top">
              <div>
                <div class="name">{fileName(f?.path)}</div>
                <div class="path">{f?.path}</div>
              </div>
              {#if f}<button onclick={() => play(it)} aria-label="Ver el archivo">▶</button>{/if}
            </div>
            <div class="tokens">
              {#each tokens(it) as [k, v] (k)}<span>{k}: {v}</span>{/each}
            </div>
            {#if readOnly}
              <p class="note">Modo consulta: no se puede identificar.</p>
            {:else}
              {#key it.fingerprint}
                <Identificar
                  fingerprint={it.fingerprint}
                  candidates={it.candidates}
                  active
                  ondone={() => onresolved(it.fingerprint)}
                />
              {/key}
            {/if}
          </div>
        {:else}
          <button class="row" onclick={() => select(i)}>
            <span class="rname">{fileName(f?.path)}<span class="path">{f?.path}</span></span>
            <span class="best">
              {it.candidates.length
                ? `${it.candidates.length} ${it.candidates.length === 1 ? 'candidato' : 'candidatos'} · mejor ${percent(it.candidates[0].score)}`
                : 'sin candidatos'}
            </span>
          </button>
        {/if}
      </li>
    {/each}
  </ol>
{/if}

<style>
  .keys {
    color: var(--faint);
    font-size: 13px;
    margin: 0 0 10px;
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .item {
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .top {
    display: flex;
    justify-content: space-between;
    gap: 12px;
  }
  .name {
    color: var(--strong);
    font-weight: 600;
  }
  .tokens {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .tokens span {
    background: var(--surface-2);
    border-radius: 3px;
    padding: 1px 7px;
    font-size: 12px;
    color: var(--muted);
  }
  .row {
    width: 100%;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    text-align: left;
    background: var(--surface);
    border-color: var(--line);
    padding: 8px 14px;
  }
  .rname {
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .best {
    flex: none;
    color: var(--accent);
    font-size: 13px;
  }
  .note {
    margin: 0;
    color: var(--muted);
  }
</style>
```

Crear `web/src/components/Duplicados.svelte`:

```svelte
<script>
  import { resolution, size } from '../lib/format.js'
  import { itemHref } from '../lib/router.js'

  // report: {recoverable, groups} from /api/duplicates.
  let { report } = $props()

  const TYPES = { identical: 'Copia idéntica', versions: 'Varias versiones' }
</script>

{#if report.groups.length === 0}
  <p class="empty">No hay duplicados.</p>
{:else}
  <p class="total">
    Espacio recuperable: <strong>{size(report.recoverable)}</strong>
    <span class="hint">(dejando una copia de cada contenido y, si hay varias versiones, solo la mejor)</span>
  </p>
  <ul>
    {#each report.groups as g (g.kind + (g.tmdbId || g.key))}
      <li>
        <a class="group" href={itemHref(g)}>
          <div class="head">
            <span class="title">{g.title}</span>
            <span class="year">{g.year || ''}</span>
            {#each g.types as t (t)}<span class="tag {t}">{TYPES[t]}</span>{/each}
            <span class="rec">{size(g.recoverable)}</span>
          </div>
          {#each g.versions as v (v.id)}
            <div class="v" class:best={v.best}>
              <span class="res">{resolution(v.resolution)}</span>
              <span class="size">{size(v.size)}</span>
              <span class="path">{v.path}</span>
            </div>
          {/each}
        </a>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .total {
    margin: 0 0 14px;
  }
  .hint {
    color: var(--faint);
    font-size: 13px;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .group {
    display: block;
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 10px 14px;
  }
  .group:hover {
    border-color: var(--line-strong);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 8px;
    margin-bottom: 6px;
  }
  .title {
    color: var(--strong);
    font-weight: 600;
  }
  .year {
    color: var(--faint);
  }
  .tag {
    font-size: 10px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 1px 6px;
    border-radius: 2px;
    background: var(--surface-2);
    color: var(--muted);
  }
  .tag.identical {
    background: var(--same-bg);
    color: var(--same);
  }
  .rec {
    margin-left: auto;
    color: var(--accent);
    font-weight: 600;
  }
  .v {
    display: grid;
    grid-template-columns: 60px 80px 1fr;
    gap: 10px;
    font-size: 13px;
    color: var(--muted);
    padding: 2px 0;
  }
  .v.best .res {
    color: var(--best);
  }
  .size {
    color: var(--faint);
  }
</style>
```

- [ ] **Step 2: Páginas**

Reemplazá `web/src/pages/Version.svelte` completo por:

```svelte
<script>
  import Identificar from '../components/Identificar.svelte'
  import TarjetaVersion from '../components/TarjetaVersion.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { back, navigate } from '../lib/nav.svelte.js'
  import { movieHref } from '../lib/router.js'

  // The page of content that is not a movie of the catalog (yet).
  let { key } = $props()

  let d = $state(null)
  let missing = $state(false)

  async function load() {
    try {
      d = await api.version(key)
      missing = false
    } catch (e) {
      if (e.status === 404) missing = true
      else notify(e.message)
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  const STATUS = {
    unmatched: 'Sin identificar',
    ignored: 'No es una película',
    extra: 'Extra',
    auto: 'Identificada',
    manual: 'Identificada a mano',
  }

  const fingerprint = $derived(d?.versions[0]?.fingerprint ?? '')

  function done({ action, tmdbId }) {
    if (action === 'movie') navigate(movieHref(tmdbId))
    else if (action === 'gone') missing = true
    else back()
  }
</script>

<svelte:head><title>{d ? `${d.title} · Cinexplorer` : 'Cinexplorer'}</title></svelte:head>

{#if missing}
  <p class="empty">Esta versión ya no está en el catálogo. <a href="/explorar">Ir a Explorar</a></p>
{:else if d}
  <section class="head">
    <div class="poster"><span>{d.title || 'Sin título'}</span></div>
    <div>
      <h1>{d.title || 'Sin título'}</h1>
      <div class="sub">
        {d.year || 's/f'} · {STATUS[d.identification?.status] ?? 'Pendiente de identificar'}
      </div>
      {#if d.movie}
        <p>Identificada como <a href={movieHref(d.movie.tmdbId)}>{d.movie.title} ({d.movie.year})</a>.</p>
      {/if}
    </div>
  </section>

  <div class="versions">
    {#each d.versions as v (v.id)}
      <TarjetaVersion {v} corrections={false} onchanged={load} />
    {/each}
  </div>

  {#if fingerprint && !app.status?.readOnly}
    <h2 class="label">¿Qué es?</h2>
    <Identificar {fingerprint} candidates={d.identification?.candidates ?? []} ondone={done} />
  {:else if !fingerprint}
    <p class="note">Todavía no tiene huella (el archivo no se terminó de leer, o está vacío): no se puede identificar.</p>
  {/if}
{/if}

<style>
  .head {
    display: flex;
    gap: 24px;
    align-items: flex-end;
    margin: 16px 0 24px;
  }
  .poster {
    width: 120px;
    aspect-ratio: 2 / 3;
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: 8px;
    border-radius: 3px;
    border: 1px dashed #475060;
    background: repeating-linear-gradient(45deg, #1b1f25, #1b1f25 8px, #1f242b 8px, #1f242b 16px);
    color: var(--muted);
    font-size: 12px;
    text-transform: uppercase;
    overflow-wrap: anywhere;
  }
  h1 {
    margin: 0;
    font-size: clamp(22px, 3.5vw, 34px);
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--strong);
  }
  .sub {
    color: var(--muted);
  }
  .versions {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  h2 {
    margin: 32px 0 10px;
    font-weight: 500;
  }
  .note {
    color: var(--muted);
  }
</style>
```

Reemplazá `web/src/pages/Revisar.svelte` completo por:

```svelte
<script>
  import Duplicados from '../components/Duplicados.svelte'
  import SinIdentificar from '../components/SinIdentificar.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { size } from '../lib/format.js'

  let { tab } = $props()

  let queue = $state(null)
  let dups = $state(null)

  async function load() {
    try {
      ;[queue, dups] = await Promise.all([api.unidentified(), api.duplicates()])
    } catch (e) {
      notify(e.message)
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  // An item left the queue: count it out without reloading the list.
  function resolved(fingerprint) {
    queue.items = queue.items.filter((it) => it.fingerprint !== fingerprint)
  }
</script>

<svelte:head><title>Revisar · Cinexplorer</title></svelte:head>

<nav class="tabs">
  <a href="/revisar" class:on={tab === 'sin-identificar'}>
    Sin identificar {#if queue}({queue.items.length}){/if}
    {#if queue?.pending}<span class="waiting">· {queue.pending} en espera</span>{/if}
  </a>
  <a href="/revisar/duplicados" class:on={tab === 'duplicados'}>
    Duplicados {#if dups}({dups.groups.length} · {size(dups.recoverable)}){/if}
  </a>
</nav>

{#if tab === 'sin-identificar' && queue}
  <SinIdentificar items={queue.items} onresolved={resolved} />
{:else if tab === 'duplicados' && dups}
  <Duplicados report={dups} />
{/if}

<style>
  .tabs {
    display: flex;
    gap: 22px;
    border-bottom: 1px solid var(--line);
    margin-bottom: 16px;
    font-size: 13px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .tabs a {
    padding-bottom: 8px;
  }
  .tabs .on {
    color: var(--strong);
    border-bottom: 2px solid var(--accent);
  }
  .waiting {
    color: var(--faint);
    text-transform: none;
    letter-spacing: 0;
  }
</style>
```

- [ ] **Step 3: Tests y build**

Run: `cd web && npm test && npm run build`
Expected: tests en verde; `✓ built` sin advertencias.

Run: `go vet ./... && go test ./...`
Expected: todos los paquetes `ok`.

- [ ] **Step 4: Probar en el navegador**

- `/revisar`: la cola muestra el primer ítem expandido. ↓ y ↑ cambian de ítem; `1` confirma el primer candidato y el ítem sale de la lista; `E` abre el selector de "extra de…" con el foco en su campo; `/` enfoca la búsqueda.
- `/revisar/duplicados`: total recuperable y grupos; un clic abre la ficha.
- Un ítem no identificado de Explorar abre su ficha provisoria; un archivo vacío explica que no se puede identificar.

- [ ] **Step 5: Commit**

```bash
git add web internal/server/dist
git commit -m "feat(web): provisional version page and Revisar"
```

---

### Task 18: README, verificación final y prueba real

**Files:**
- Modify: `README.md`

- [ ] **Step 1: README**

En `README.md`, reemplazá:

```markdown
| 3. Identificación | TMDB + Wikidata, puntaje de confianza, correcciones, afiches | ✅ |
| 4. Interfaz | interfaz Svelte (Inicio, Explorar, Ficha, Revisar, búsqueda) | pendiente |
| 5. Curaduría | listas, etiquetas, importación de `Collections/` | pendiente |

Por ahora la interfaz es una página mínima, pensada para revisar el catálogo.
Tiene tres pestañas:

- **Versiones:** cada versión con la película identificada, su calidad, audio,
  subtítulos, la marca **MEJOR** cuando tenés varias, y los botones ▶ Ver y
  Carpeta.
- **Sin identificar:** los casos dudosos, con candidatos para confirmar y
  búsqueda manual.
- **Copias idénticas:** archivos repetidos byte a byte.
```

por:

```markdown
| 3. Identificación | TMDB + Wikidata, puntaje de confianza, correcciones, afiches | ✅ |
| 4a. Catálogo navegable | interfaz Svelte: Explorar con facetas, Ficha, Revisar | ✅ |
| 4b. Descubrimiento | Inicio, búsqueda instantánea, asistente de primer uso | pendiente |
| 5. Curaduría | listas, etiquetas, importación de `Collections/` | pendiente |

La interfaz tiene tres partes:

- **Explorar:** la colección como grilla de afiches, con facetas combinables
  (década y año, director, género, país, resolución, subtítulos, idioma
  original, colección, ubicación en disco y estado) y orden por año, título,
  fecha de alta o tamaño. Los filtros quedan en la dirección de la página, así
  que se pueden guardar como favoritos y el botón "atrás" funciona. Lo que
  todavía no está identificado aparece igual, con un afiche genérico.
- **Ficha de cada película:** datos de TMDB y, debajo, cada versión en disco
  con su calidad, audio, subtítulos, la marca **MEJOR** y **COPIA IDÉNTICA**,
  y los botones ▶ Ver y Carpeta. El menú ⋯ de cada versión corrige la
  identificación.
- **Revisar:** la cola de **Sin identificar** (candidatos, búsqueda manual, "no
  es una película", "es un extra de…", con atajos de teclado) y los
  **Duplicados** (copias idénticas y varias versiones de una película, con el
  espacio que se podría recuperar).

Un indicador arriba a la derecha muestra el escaneo y la identificación en
curso; las páginas se actualizan solas a medida que avanzan.
```

En `README.md`, reemplazá:

```markdown

Para compilarlo: Go 1.27 o posterior (ver [Desarrollo](#desarrollo)).
```

por:

```markdown

Para compilarlo: Go 1.27 o posterior; Node 22.12 o posterior solo para
modificar la interfaz (ver [Desarrollo](#desarrollo)).
```

En `README.md`, reemplazá:

```markdown

La página usa una API JSON local; también sirve para scripts. Los `POST`
requieren `Content-Type: application/json`.
```

por:

```markdown

La interfaz usa una API JSON local; también sirve para scripts. Los `POST`
requieren `Content-Type: application/json`.
```

En `README.md`, reemplazá:

```markdown
| `GET /api/status` | Estado del escaneo y de la identificación. |
| `GET /api/versions` | Todas las versiones con archivos, datos técnicos, identificación y película. |
| `GET /api/unidentified` | Versiones sin identificar, con candidatos. |
| `GET /api/duplicates` | Grupos de copias idénticas. |
| `GET /api/tmdb/search?q=&year=` | Búsqueda manual en TMDB (`q` puede ser un IMDb id `tt…`). |
```

por:

```markdown
| `GET /api/status` | Estado del escaneo y de la identificación. |
| `GET /api/explore?decada=&anio=&director=&genero=&pais=&idioma=&coleccion=&resolucion=&subs=&ubicacion=&estado=&orden=&dir=` | Ítems de Explorar que cumplen las facetas, y los valores de cada faceta con su conteo. |
| `GET /api/movies/{tmdbId}` | Ficha de una película: datos, versiones y extras. |
| `GET /api/movies?q=&near=` | Películas del catálogo para "es un extra de…". |
| `GET /api/versions/{huella}` | Ficha de una versión que no es (todavía) una película del catálogo. |
| `GET /api/versions` | Todas las versiones con archivos, datos técnicos, identificación y película. |
| `GET /api/unidentified` | Cola de sin identificar, con candidatos, y cuántas esperan identificación. |
| `GET /api/duplicates` | Copias idénticas y varias versiones, con el espacio recuperable. |
| `GET /api/tmdb/search?q=&year=` | Búsqueda manual en TMDB (`q` puede ser un IMDb id `tt…`). |
```

En `README.md`, reemplazá:

```markdown
| `POST /api/open`, `POST /api/reveal` | Abre un archivo del catálogo con la aplicación del sistema, o muestra su carpeta. |
| `GET /img/{poster\|backdrop}/{tmdbId}.jpg` | Imágenes desde la caché. |
```

por:

```markdown
| `POST /api/open`, `POST /api/reveal` | Abre un archivo del catálogo con la aplicación del sistema, o muestra su carpeta. |
| `GET /img/{poster\|backdrop}/{tmdbId}.jpg` | Imágenes desde la caché (`?v=` identifica la versión de la imagen). |
```

En `README.md`, reemplazá:

```markdown
[`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite), escrito en Go
puro, así que la compilación cruzada es directa. No hay otras dependencias.
```

por:

```markdown
[`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite), escrito en Go
puro, así que la compilación cruzada es directa.

La interfaz (Svelte 5 + Vite, en `web/`) se compila a `internal/server/dist/`,
que está commiteado y queda embebido en el binario: para compilar o testear el
Go no hace falta Node. Para modificar la interfaz hace falta **Node 22.12+**.
```

En `README.md`, reemplazá:

````markdown
go run ./cmd/cinexplorer -dir .run     # corre la app con config y catálogo en .run/
bash scripts/build.sh                  # compila los tres binarios en dist/cinexplorer/
```
````

por:

````markdown
go run ./cmd/cinexplorer -dir .run     # corre la app con config y catálogo en .run/
bash scripts/build.sh                  # compila la interfaz y los tres binarios en dist/cinexplorer/
```

Para trabajar en la interfaz con recarga en caliente:

```bash
go run ./cmd/cinexplorer -dir .run -port 8080 -no-browser   # la API, en una terminal
cd web && npm install && npm run dev                        # la interfaz en http://localhost:5173
npm test                                                    # tests de la lógica de la interfaz (Vitest)
npm run build                                               # actualiza internal/server/dist/ (commitealo)
```
````

En `README.md`, reemplazá:

```markdown
`go run` y necesita red la primera vez. La CI (GitHub Actions) corre `go vet` y
`go test` en Windows, macOS y Linux.
```

por:

```markdown
`go run` y necesita red la primera vez. La CI (GitHub Actions) corre `go vet` y
`go test` en Windows, macOS y Linux, y en otro job los tests de la interfaz y
su build, que tiene que coincidir con el commiteado.
```

En `README.md`, reemplazá:

````markdown
internal/identify     puntaje, búsqueda, enriquecimiento y el proceso en segundo plano
internal/server       API JSON y página embebida (internal/server/web)
internal/platform     abrir archivos y carpetas con el sistema
```
````

por:

````markdown
internal/identify     puntaje, búsqueda, enriquecimiento y el proceso en segundo plano
internal/catalog      ítems, facetas, fichas y duplicados de la interfaz
internal/server       API JSON y la interfaz embebida (internal/server/dist)
internal/platform     abrir archivos y carpetas con el sistema
web/                  la interfaz (Svelte 5 + Vite)
```
````

En `README.md`, reemplazá:

```markdown
  La app reintenta sola; mientras tanto, todo lo demás funciona.
- **Una película mal identificada o sin identificar:** confirmá el candidato
  correcto, o buscala a mano por título o por IMDb id (`tt0071129`), en la
  pestaña "Sin identificar".
- **Archivos que no aparecen:** revisá `roots` en `config.json`; las rutas son
```

por:

```markdown
  La app reintenta sola; mientras tanto, todo lo demás funciona.
- **Una película mal identificada o sin identificar:** en Revisar → Sin
  identificar, confirmá el candidato correcto o buscala a mano por título o por
  IMDb id (`tt0071129`). Una identificación equivocada se corrige desde el menú
  ⋯ de la versión, en la ficha de la película.
- **Archivos que no aparecen:** revisá `roots` en `config.json`; las rutas son
```

- [ ] **Step 2: Verificación completa**

Run: `gofmt -l internal cmd; go vet ./... && go test ./...`
Expected: `gofmt` sin archivos de esta etapa; todo `ok`.

Run: `cd web && npm ci && npm test && npm run build && cd .. && git status --porcelain internal/server/dist`
Expected: tests en verde, build sin advertencias y **ninguna** línea de `git status`: el build commiteado coincide con las fuentes (lo mismo que va a comprobar la CI).

Run: `GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/cinexplorer && GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/cinexplorer`
Expected: sin errores.

- [ ] **Step 3: Prueba real (manual, fuera de CI)**

Binario con `config.json` apuntando a una parte de la colección (por ejemplo `D:\cine\1970s`) y el token de TMDB. Recorrer: Explorar con dos o tres facetas y los órdenes, atrás/adelante; una Ficha con varias versiones (▶ Ver y Carpeta); la cola de Revisar con los atajos; Duplicados; el indicador de estado durante el escaneo y la identificación (la grilla se actualiza sola); a ancho de teléfono.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: README for the stage 4a interface"
```
