# Cinexplorer — Etapa 4b: Descubrimiento y primer uso — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sumar a la interfaz el Inicio estilo MUBI, la búsqueda instantánea con FTS5, el asistente de primer uso y una página de Ajustes que cambia carpetas, token, idioma y descarga de imágenes sin reiniciar; y el idioma `es-AR` por defecto, con respaldo en las traducciones más cercanas de TMDB.

**Architecture:** Un paquete nuevo, `internal/engine`, arma desde `config.json` todo lo que depende de él (cliente de TMDB, caché de imágenes, escáner, identificación) en un `Runtime` que se reemplaza entero al guardar los ajustes; el servidor lo lee una vez por pedido. La búsqueda es un índice FTS5 en una base SQLite en memoria (`internal/search`), derivado de los ítems de Explorar y reconstruido cuando cambia el contador de cambios del catálogo. Las filas del Inicio son funciones puras de `internal/catalog` sobre la instantánea. En `web/`, páginas nuevas (Inicio, Buscar, Bienvenida, Ajustes) y su lógica pura con tests.

**Tech Stack:** Go (sin dependencias nuevas), `modernc.org/sqlite` (FTS5 incluido), Svelte 5 (runes, JavaScript), Vite 8, Vitest 5.

Spec: `docs/superpowers/specs/2026-09-27-cinexplorer-m4b-descubrimiento-design.md` (detalle de esta etapa) y `docs/superpowers/specs/2026-09-25-cinexplorer-design.md` §5.1, §5.6 y §5.7.

**Nota sobre el código de este plan:** todo el código se prototipó y se probó antes de escribir el plan: suite de Go en verde (también con `-race`), `go vet` para Windows, macOS y Linux, Vitest en verde, build de Vite sin advertencias y reproducible, y una revisión en el navegador sobre un catálogo de demostración (Inicio, búsqueda, asistente completo, Ajustes guardando y quitando una carpeta, 1280 y 375 px de ancho). Los bloques de código salen tal cual del prototipo, commit por commit, y el plan se validó aplicándolo sobre un árbol limpio y comparando el resultado de cada tarea con el prototipo. Copiá el código tal cual; si algo no compila o un test no da lo esperado, es un error del plan: reportalo en lugar de improvisar.

**Lo que no se pudo probar al prototipar:** el entorno no tenía salida a TMDB, así que la verificación del token contra el servicio real, la forma de `GET /movie/{id}/translations` y el comportamiento de `language=es-AR` sin traducción argentina quedan para la prueba real de la Task 16. Los tests de Go no se corrieron en Windows ni en macOS (solo `go vet` y compilación cruzada); los corre la CI.

---

## Hoja de ruta (etapas)

| Etapa | Contenido | Estado |
|---|---|---|
| 1 — Núcleo local | escaneo, parser de nombres, versiones, SQLite, API mínima, build, CI | ✅ en `main` |
| 2 — Datos técnicos | lectores nativos MKV/MP4/AVI/IFO, fallback `ffprobe`, tabla `media`, mejor versión | ✅ en `main` |
| 3 — Identificación | TMDB + Wikidata, puntaje de confianza, películas, correcciones por huella, imágenes | ✅ en `main` |
| 4a — Catálogo navegable | Svelte embebido, Explorar con facetas, Ficha, Revisar | ✅ en `main` |
| **4b — Descubrimiento y primer uso (este plan)** | Inicio, búsqueda FTS5, asistente de primer uso, Ajustes, es-AR | |
| 5 — Curaduría | listas, etiquetas, importación de `Collections/` | |

---

## Estructura de archivos (Etapa 4b)

```
internal/config/config.go          es-AR por defecto, Languages, PrefetchModes, Root, CheckRoots, Available
internal/store/store.go            MarkOutsideRoots
internal/store/snapshot.go         Snapshot.Changes, Changes
internal/scan/scan.go              MarkOutsideRoots después de SyncFiles
internal/tmdb/tmdb.go              Translations
internal/identify/translate.go     Chain y la cadena de traducciones
internal/identify/runner.go        fetchMovie con traducciones; contexto propio y Close
internal/engine/engine.go          Engine (Start, Apply, Current, Use), Runtime (Scan, Stop)
internal/search/search.go          índice FTS5 en memoria: Refresh, Query, Terms
internal/catalog/search.go         ItemKey, SearchDocs, Directors
internal/catalog/home.go           HomePage (filas del Inicio)
internal/server/server.go          Engine y rt(); rutas nuevas; setupPending en /api/status
internal/server/config.go          /api/config (GET, PUT), /api/config/root, /api/config/token
internal/server/search.go          /api/search con ítems e índice reutilizados
internal/server/catalog.go         /api/home
cmd/cinexplorer/main.go            setup con el engine; el primer arranque no guarda ni escanea
web/src/lib/                       home.js, search.js, settings.js (+ tests); router, format, status, api, app.svelte
web/src/components/                FilaInicio, TarjetaEscena, Busqueda, Raices, Token, Idioma; Nav, Estado, Identificar
web/src/pages/                     Inicio, Buscar, Bienvenida, Ajustes; Explorar
README.md
```

Dependencias nuevas entre paquetes: `engine → {config, identify, images, scan, store, tmdb}`, `server → {engine, search}`, `catalog → search`, `search → quality`, `config → appdir`. Sin ciclos.

Convenciones que ya usa el proyecto y hay que mantener: comentarios en inglés, mensajes de log y de UI en español, tests de Go en el mismo paquete (`package x`, no `x_test`), commits en inglés con prefijo convencional. En el frontend: JavaScript sin TypeScript, comentarios en inglés, textos en español.

**Entorno.** En Windows con Git Bash, `go` debería estar en el PATH; si no: `export PATH="$PATH:/c/Program Files/Go/bin"`. Vite 8 y Vitest 5 piden **Node 22.12 o posterior**: comprobalo con `node --version`. En la máquina de desarrollo el Node por defecto es 22.6 y hay un Node 24 instalado con nvm-windows; antes de cualquier comando `npm`/`npx`: `export PATH="/c/Users/martin/AppData/Roaming/nvm/v24.21.0:$PATH"`. Algunos archivos existentes están con CRLF en el checkout: los reemplazos de texto de este plan son sobre el contenido, no sobre los finales de línea; `gofmt` normaliza. El build de `web/` se commitea: cada tarea del frontend termina con `npm run build` y suma `internal/server/dist/` al commit.

**Formato de los cambios.** "Crear `x`:" y "Reemplazá `x` completo por:" dan el archivo entero. "En `x`, reemplazá: … por: …" da un fragmento que aparece exactamente una vez en el archivo y su reemplazo; si hay varios en la misma tarea, aplicalos en orden.

---

### Task 1: Configuración: es-AR, idiomas ofrecidos y validación de carpetas

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

Spec §4.1 y §4.3. `config.Load` completa `language` con `es-AR` (antes `es-ES`), y la configuración propuesta del primer uso también. `config.Languages` y `config.PrefetchModes` son las listas que ofrecen el asistente y Ajustes. `config.Root` convierte una carpeta escrita por la persona (absoluta o relativa a la app) a la forma del catálogo, y rechaza la que no existe, la que no es carpeta, la de la app, la que la contiene y la que está adentro. `config.CheckRoots` valida la lista entera: no vacía, sin repetidas ni anidadas; las raíces que ya estaban guardadas se aceptan aunque no estén disponibles (un disco desenchufado). `config.Available` dice si una raíz está hoy.

- [ ] **Step 1: Escribir los tests que fallan**

Reemplazá `internal/config/config_test.go` completo por:

```go
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadDefaultsToSiblingDirs(t *testing.T) {
	disk := t.TempDir()
	for _, d := range []string{"cinexplorer", "cine", "cine-ordenar", "$RECYCLE.BIN", ".Trashes", "System Volume Information"} {
		if err := os.Mkdir(filepath.Join(disk, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(disk, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, created, err := Load(filepath.Join(disk, "cinexplorer"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"../cine", "../cine-ordenar"}
	if !created || !reflect.DeepEqual(cfg.Roots, want) || cfg.Language != "es-AR" {
		t.Fatalf("got created=%v cfg=%+v", created, cfg)
	}
}

func TestSaveThenLoad(t *testing.T) {
	dir := t.TempDir()
	in := Config{Roots: []string{"../x"}, TMDBToken: "tok", Language: "es-ES", ImagePrefetch: "posters"}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, created, err := Load(dir)
	if err != nil || created || !reflect.DeepEqual(in, out) {
		t.Fatalf("got %+v created=%v err=%v", out, created, err)
	}
}

func TestLoadRejectsBrokenJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, FileName), []byte("{roots:"), 0o644)
	if _, _, err := Load(dir); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// disk makes <tmp>/cinexplorer (the app dir) next to cine/1970s, and a file.
func disk(t *testing.T) (appDir, root string) {
	t.Helper()
	root = t.TempDir()
	appDir = filepath.Join(root, "cinexplorer")
	for _, d := range []string{appDir, filepath.Join(root, "cine", "1970s"), filepath.Join(appDir, "cache")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return appDir, root
}

func TestRoot(t *testing.T) {
	appDir, root := disk(t)
	for _, tc := range []struct{ in, want string }{
		{"../cine", "../cine"},
		{"../cine/", "../cine"},
		{"./../cine/1970s", "../cine/1970s"},
		{filepath.Join(root, "cine"), "../cine"},
		{"  ../cine  ", "../cine"},
	} {
		got, err := Root(appDir, tc.in)
		if err != nil || got != tc.want {
			t.Errorf("Root(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"", "../nada", "../notes.txt", ".", appDir, "..", root, "cache", filepath.Join(appDir, "cache")} {
		if got, err := Root(appDir, in); err == nil {
			t.Errorf("Root(%q) = %q, want an error", in, got)
		}
	}
}

func TestCheckRoots(t *testing.T) {
	appDir, _ := disk(t)
	ok := [][]string{
		{"../cine"},
		{"../cine", "../gone"}, // saved, unplugged
	}
	for _, roots := range ok {
		if err := CheckRoots(appDir, roots, []string{"../gone"}); err != nil {
			t.Errorf("%v: %v", roots, err)
		}
	}
	bad := [][]string{
		nil,
		{"../gone"},                  // new and missing
		{"../cine/"},                 // not in catalog form
		{"../cine", "../cine"},       // repeated
		{"../cine", "../cine/1970s"}, // nested
		{"../cine/1970s", "../cine"}, // nested, the other way
	}
	for _, roots := range bad {
		if err := CheckRoots(appDir, roots, nil); err == nil {
			t.Errorf("%v: want an error", roots)
		}
	}
}

func TestAvailable(t *testing.T) {
	appDir, _ := disk(t)
	if !Available(appDir, "../cine") || Available(appDir, "../nada") || Available(appDir, "../notes.txt") {
		t.Fatal("Available")
	}
}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/config`

Expected: FAIL, por ejemplo:

```
internal/config/config_test.go:76:15: undefined: Root
internal/config/config_test.go:82:18: undefined: Root
internal/config/config_test.go:95:13: undefined: CheckRoots
internal/config/config_test.go:108:13: undefined: CheckRoots
FAIL	cinexplorer/internal/config [build failed]
```

- [ ] **Step 3: Implementar**

Reemplazá `internal/config/config.go` completo por:

```go
// Package config reads and writes config.json next to the executable.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"cinexplorer/internal/appdir"
)

const FileName = "config.json"

// DefaultLanguage is the metadata language of a new configuration.
const DefaultLanguage = "es-AR"

// Languages are the metadata languages offered in the settings.
var Languages = []string{"es-AR", "en-US", "pt-BR"}

// PrefetchModes are the values of imagePrefetch; the first is the default.
var PrefetchModes = []string{"none", "posters", "all"}

type Config struct {
	Roots     []string `json:"roots"`     // catalog-form paths, relative to the app dir
	TMDBToken string   `json:"tmdbToken"` // TMDB v4 read access token
	Language  string   `json:"language"`
	// ImagePrefetch downloads images ahead: "posters", "all", or "none"
	// (the default: on demand only).
	ImagePrefetch string `json:"imagePrefetch,omitempty"`
}

// Load reads config.json from appDir. When the file does not exist it returns
// defaults (the sibling directories of appDir as roots) and created=true.
func Load(appDir string) (Config, bool, error) {
	data, err := os.ReadFile(filepath.Join(appDir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		roots, err := DefaultRoots(appDir)
		if err != nil {
			return Config{}, false, err
		}
		return Config{Roots: roots, Language: DefaultLanguage}, true, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("%s: %w", FileName, err)
	}
	if cfg.Language == "" {
		cfg.Language = DefaultLanguage
	}
	return cfg, false, nil
}

func Save(appDir string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(appDir, FileName), append(data, '\n'), 0o644)
}

// DefaultRoots lists the sibling directories of appDir as "../<name>",
// skipping hidden and system folders.
func DefaultRoots(appDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(appDir))
	if err != nil {
		return nil, err
	}
	self := filepath.Base(appDir)
	roots := []string{}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name == self || skipRoot(name) {
			continue
		}
		roots = append(roots, "../"+name)
	}
	return roots, nil
}

func skipRoot(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
		return true
	}
	switch strings.ToLower(name) {
	case "system volume information", "recycler", "lost+found":
		return true
	}
	return false
}

// Root turns a folder the user typed (absolute, or relative to the app
// directory) into catalog form: relative to appDir, '/'-separated. The folder
// must exist, and neither be, contain nor sit inside the app directory.
func Root(appDir, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("falta la carpeta")
	}
	abs := input
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(appDir, filepath.FromSlash(abs))
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("no existe la carpeta %s", input)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("%s no es una carpeta", input)
	}
	rel, err := appdir.Rel(appDir, abs)
	if err != nil {
		// Another drive on Windows: the catalog only keeps relative paths.
		return "", fmt.Errorf("%s tiene que estar en el mismo disco que la app", input)
	}
	rel = path.Clean(rel)
	switch {
	case rel == ".":
		return "", errors.New("esa es la carpeta de la app")
	case onlyParents(rel):
		return "", errors.New("esa carpeta contiene a la de la app")
	case rel != ".." && !strings.HasPrefix(rel, "../"):
		return "", errors.New("esa carpeta está dentro de la de la app")
	}
	return rel, nil
}

// onlyParents reports whether rel is "..", "../.." and so on.
func onlyParents(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if part != ".." {
			return false
		}
	}
	return true
}

// CheckRoots validates the roots of a configuration. Roots in saved (the
// current configuration) are accepted as they are, even when unavailable
// (an unplugged drive); new ones must pass Root and already be in catalog
// form. No root may repeat or contain another.
func CheckRoots(appDir string, roots, saved []string) error {
	if len(roots) == 0 {
		return errors.New("elegí al menos una carpeta")
	}
	for i, r := range roots {
		if !slices.Contains(saved, r) {
			rel, err := Root(appDir, r)
			if err != nil {
				return err
			}
			if rel != r {
				return fmt.Errorf("carpeta mal escrita: %s", r)
			}
		}
		for _, o := range roots[:i] {
			switch {
			case o == r:
				return fmt.Errorf("carpeta repetida: %s", r)
			case strings.HasPrefix(r, o+"/"), strings.HasPrefix(o, r+"/"):
				return fmt.Errorf("%s y %s están una dentro de la otra", o, r)
			}
		}
	}
	return nil
}

// Available reports whether a root in catalog form is a folder right now.
func Available(appDir, root string) bool {
	st, err := os.Stat(appdir.Abs(appDir, root))
	return err == nil && st.IsDir()
}
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): es-AR by default, offered languages, root validation"
```

---

### Task 2: Carpetas quitadas y contador de cambios del catálogo

**Files:**
- Modify: `internal/scan/scan.go`, `internal/store/identity.go`, `internal/store/snapshot.go`, `internal/store/store.go`
- Test: `internal/scan/scan_test.go`, `internal/store/roots_test.go` (nuevo)

Spec §3.4 y §6.1. `Store.MarkOutsideRoots` marca como faltantes los archivos presentes que no están bajo ninguna raíz configurada: lo que deja una raíz quitada en Ajustes. Las raíces se limpian con `path.Clean` (`./../cine/` vale como `../cine`), y las configuradas pero no disponibles no se tocan. El escáner lo llama justo después de `SyncFiles`, con todas las raíces configuradas.

`Snapshot.Changes` es `SELECT total_changes()` leído en la misma transacción que la instantánea: cambia cuando el catálogo cambió. La búsqueda lo usa para saber cuándo reconstruir su índice (Task 8). `querier` suma `QueryRow` (`*sql.Tx` y `*sql.DB` ya lo tienen).

- [ ] **Step 1: Escribir los tests que fallan**

En `internal/scan/scan_test.go`, reemplazá:

```go
		t.Fatalf("OnDone called %d times", calls)
	}
}

```

por:

```go
		t.Fatalf("OnDone called %d times", calls)
	}
}

func TestScanMarksRemovedRootMissing(t *testing.T) {
	disk, app, st := setup(t)
	writeFile(t, filepath.Join(disk, "cine", "Amarcord.1973.mkv"), 4096, 'a')
	writeFile(t, filepath.Join(disk, "cine-ordenar", "Attenberg.2010.avi"), 4096, 'b')
	sc := &Scanner{AppDir: app, Roots: []string{"../cine", "../cine-ordenar"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	sc.Roots = []string{"../cine"}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx, err := st.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	if idx["../cine/Amarcord.1973.mkv"].Missing || !idx["../cine-ordenar/Attenberg.2010.avi"].Missing {
		t.Fatalf("index %+v", idx)
	}
}

```

Crear `internal/store/roots_test.go`:

```go
package store

import "testing"

func TestMarkOutsideRoots(t *testing.T) {
	s := open(t)
	rows := []FileRow{
		{Path: "../cine/a.mkv", Size: 10, MTime: 1, Fingerprint: "fa", Kind: "video"},
		{Path: "../cine-ordenar/b.mkv", Size: 20, MTime: 2, Fingerprint: "fb", Kind: "video"},
		{Path: "../otro/c.mkv", Size: 30, MTime: 3, Fingerprint: "fc", Kind: "video"},
	}
	if err := s.SyncFiles(rows, []string{"../cine", "../cine-ordenar", "../otro"}); err != nil {
		t.Fatal(err)
	}
	// ../otro was removed; ../cine-ordenar is configured (maybe unplugged);
	// "./../cine/" is ../cine written another way.
	if err := s.MarkOutsideRoots([]string{"./../cine/", "../cine-ordenar"}); err != nil {
		t.Fatal(err)
	}
	idx, err := s.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	if idx["../cine/a.mkv"].Missing || idx["../cine-ordenar/b.mkv"].Missing || !idx["../otro/c.mkv"].Missing {
		t.Fatalf("index %+v", idx)
	}
	// Adding the root again brings its files back.
	if err := s.SyncFiles(rows[2:], []string{"../otro"}); err != nil {
		t.Fatal(err)
	}
	if idx, _ := s.FileIndex(); idx["../otro/c.mkv"].Missing {
		t.Fatal("c.mkv still missing")
	}
}

func TestSnapshotChanges(t *testing.T) {
	s := open(t)
	a, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Snapshot()
	if a.Changes != b.Changes {
		t.Fatalf("no writes, changes %d → %d", a.Changes, b.Changes)
	}
	if err := s.SyncFiles([]FileRow{{Path: "../cine/a.mkv", Size: 1, MTime: 1, Kind: "video"}}, roots); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Snapshot()
	if c.Changes == b.Changes {
		t.Fatalf("a write left changes at %d", c.Changes)
	}
}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/store ./internal/scan`

Expected: FAIL, por ejemplo:

```
internal/store/roots_test.go:17:14: s.MarkOutsideRoots undefined (type *Store has no field or method MarkOutsideRoots)
internal/store/roots_test.go:43:7: a.Changes undefined (type Snapshot has no field or method Changes)
internal/store/roots_test.go:43:20: b.Changes undefined (type Snapshot has no field or method Changes)
internal/store/roots_test.go:44:46: a.Changes undefined (type Snapshot has no field or method Changes)
FAIL	cinexplorer/internal/store [build failed]
--- FAIL: TestScanMarksRemovedRootMissing (0.02s)
```

- [ ] **Step 3: Implementar**

En `internal/scan/scan.go`, reemplazá:

```go
	}
	versions := grouping.Build(entries, attempted)
```

por:

```go
	}
	if err := s.Store.MarkOutsideRoots(s.Roots); err != nil {
		return err
	}
	versions := grouping.Build(entries, attempted)
```

En `internal/store/identity.go`, reemplazá:

```go
	Query(query string, args ...any) (*sql.Rows, error)
}
```

por:

```go
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}
```

En `internal/store/snapshot.go`, reemplazá:

```go
	Movies          map[int]Movie              // by TMDB id
}
```

por:

```go
	Movies          map[int]Movie              // by TMDB id
	// Changes counts the rows written through the catalog's connection
	// since it was opened: it differs between two snapshots when the
	// catalog changed in between (it restarts if the connection does).
	Changes int64
}
```

En `internal/store/snapshot.go`, reemplazá:

```go
	err := s.read(func(tx querier) error {
		vs, err := s.versions(tx)
```

por:

```go
	err := s.read(func(tx querier) error {
		if err := tx.QueryRow(`SELECT total_changes()`).Scan(&snap.Changes); err != nil {
			return err
		}
		vs, err := s.versions(tx)
```

En `internal/store/store.go`, reemplazá:

```go
	"fmt"
	"strings"
```

por:

```go
	"fmt"
	"path"
	"strings"
```

En `internal/store/store.go`, reemplazá:

```go

func underAny(p string, roots []string) bool {
```

por:

```go

// MarkOutsideRoots marks as missing the present files that are not under any
// of roots (the configured ones, available or not): what a removed root
// leaves behind. They come back if the root is added again.
func (s *Store) MarkOutsideRoots(roots []string) error {
	clean := make([]string, len(roots))
	for i, r := range roots {
		clean[i] = path.Clean(r)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT path FROM files WHERE missing = 0`)
	if err != nil {
		return err
	}
	var gone []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		if !underAny(p, clean) {
			gone = append(gone, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range gone {
		if _, err := tx.Exec(`UPDATE files SET missing = 1 WHERE path = ?`, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func underAny(p string, roots []string) bool {
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/scan.go internal/scan/scan_test.go internal/store/identity.go internal/store/roots_test.go internal/store/snapshot.go internal/store/store.go
git commit -m "feat(store): files outside the configured roots are missing; snapshot change counter"
```

---

### Task 3: Títulos y sinopsis desde la traducción más cercana

**Files:**
- Create: `internal/identify/translate.go`
- Modify: `internal/identify/match.go`, `internal/identify/runner.go`, `internal/tmdb/tmdb.go`
- Test: `internal/identify/fake_test.go`, `internal/identify/runner_test.go`, `internal/identify/translate_test.go` (nuevo), `internal/server/identify_test.go`, `internal/tmdb/tmdb_test.go`

Spec §5. Con `language=es-AR`, TMDB no toma la traducción de otra variante del español cuando no hay una argentina: `fetchMovie` pide además `GET /movie/{id}/translations` (salvo en `en-US`) y completa título y sinopsis con la cadena de `Chain` (`es-AR → es-MX → es-ES`, después cualquier otra variante del idioma). Un título que no está en ningún español queda como lo dio TMDB (normalmente el original); la sinopsis, y un título vacío, caen al inglés. Esto reemplaza al pedido extra en `en-US` de la Etapa 3.

La identificación no usa `Translations`: el corpus de calibración está grabado por URI y no cambia. `identify.API` suma el método, así que los fakes de `identify` y `server` también.

**A verificar en la prueba real (Task 16):** desde el entorno donde se prototipó no había salida a TMDB. La forma de `/translations` sale de la documentación de TMDB, y el comportamiento de `language=es-AR` sin traducción argentina, de lo que se sabe de la API; ninguna de las dos se pudo comprobar contra el servicio.

- [ ] **Step 1: Escribir los tests que fallan**

En `internal/identify/fake_test.go`, reemplazá:

```go
	"strconv"
	"sync"
```

por:

```go
	"strconv"
	"strings"
	"sync"
```

En `internal/identify/fake_test.go`, reemplazá:

```go
	err    error                   // returned by every call when set
	fail   map[string]error        // returned by the search with that key
```

por:

```go
	trans  map[int][]tmdb.Translation
	err    error            // returned by every call when set
	fail   map[string]error // returned by the search with that key
```

En `internal/identify/fake_test.go`, reemplazá:

```go

func (f *fakeAPI) called(prefix string) int {
```

por:

```go

func (f *fakeAPI) Translations(ctx context.Context, id int) ([]tmdb.Translation, error) {
	if err := f.record("translations " + strconv.Itoa(id)); err != nil {
		return nil, err
	}
	return f.trans[id], nil
}

// tr is a translation for fakeAPI.trans.
func tr(tag, title, overview string) tmdb.Translation {
	var t tmdb.Translation
	t.Language, t.Country, _ = strings.Cut(tag, "-")
	t.Data.Title, t.Data.Overview = title, overview
	return t
}

func (f *fakeAPI) called(prefix string) int {
```

En `internal/identify/runner_test.go`, reemplazá:

```go
			"1398|en-US": details(1398, "Stalker", "1979-05-25", "The Zone.", "Andrei Tarkovsky"),
		},
```

por:

```go
		},
		trans: map[int][]tmdb.Translation{1398: {tr("en-US", "Stalker", "The Zone.")}},
```

En `internal/identify/runner_test.go`, reemplazá:

```go
	// Amarcord's details came with the match (no director parsed, so no),
	// Stalker needed es-ES plus en-US for the empty overview.
	if n := api.called("movie 1398|"); n != 2 {
		t.Fatalf("stalker fetched %d times: %v", n, api.calls)
```

por:

```go
	// Amarcord's details came with the match; Stalker's were fetched in
	// es-ES. Each needed its translations (the overview came in English).
	if api.called("movie 1398|") != 1 || api.called("movie 7857|") != 1 || api.called("translations ") != 2 {
		t.Fatalf("calls %v", api.calls)
```

Crear `internal/identify/translate_test.go`:

```go
package identify

import (
	"context"
	"slices"
	"testing"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

func TestChain(t *testing.T) {
	for lang, want := range map[string][]string{
		"es-AR": {"es-AR", "es-MX", "es-ES"},
		"es-ES": {"es-ES", "es-MX"},
		"pt-BR": {"pt-BR", "pt-PT"},
		"en-US": {"en-US", "en-GB"},
		"fr-FR": {"fr-FR"},
	} {
		if got := Chain(lang); !slices.Equal(got, want) {
			t.Errorf("Chain(%s) = %v, want %v", lang, got, want)
		}
	}
}

func TestTranslate(t *testing.T) {
	ts := []tmdb.Translation{
		tr("en-US", "The Night", "A day and a night."),
		tr("es-ES", "La noche", "Un día y una noche."),
		tr("es-MX", "", "Un día y una noche en Milán."),
		tr("es-AR", "", ""),
		tr("it-IT", "La notte", "Un giorno."),
	}
	for _, tc := range []struct {
		lang, title, overview   string // what TMDB returned in lang
		wantTitle, wantOverview string
	}{
		// es-AR has nothing: the title from es-ES (es-MX has none), the overview from es-MX.
		{"es-AR", "The Night", "", "La noche", "Un día y una noche en Milán."},
		{"es-ES", "La noche", "Un día y una noche.", "La noche", "Un día y una noche."},
		// No Portuguese at all: the title stays as TMDB gave it, the overview in English.
		{"pt-BR", "La notte", "", "La notte", "A day and a night."},
		// An empty title falls back to English too.
		{"pt-BR", "", "", "The Night", "A day and a night."},
	} {
		m := store.Movie{Title: tc.title, Overview: tc.overview}
		translate(&m, ts, tc.lang)
		if m.Title != tc.wantTitle || m.Overview != tc.wantOverview {
			t.Errorf("%s: got %q / %q", tc.lang, m.Title, m.Overview)
		}
	}
	// Any variant of the language comes after the chain.
	m := store.Movie{}
	translate(&m, []tmdb.Translation{tr("es-CO", "La noche (CO)", "")}, "es-AR")
	if m.Title != "La noche (CO)" {
		t.Errorf("other variant: %q", m.Title)
	}
}

func TestFetchMovieTranslations(t *testing.T) {
	r, api, _ := fixture(t)
	api.movies["7857|es-AR"] = details(7857, "Amarcord", "1973-12-18", "", "Federico Fellini")
	api.trans = map[int][]tmdb.Translation{7857: {tr("es-MX", "Amarcord: Mis recuerdos", "Rimini en los años 30.")}}
	r.Language = "es-AR"
	m, err := r.fetchMovie(context.Background(), 7857, nil)
	if err != nil || m.Title != "Amarcord: Mis recuerdos" || m.Overview != "Rimini en los años 30." || m.Language != "es-AR" {
		t.Fatalf("got %+v, %v", m, err)
	}
	// English needs no translations.
	api.movies["7857|en-US"] = details(7857, "Amarcord", "1973-12-18", "", "Federico Fellini")
	r.Language = "en-US"
	before := api.called("translations ")
	if _, err := r.fetchMovie(context.Background(), 7857, nil); err != nil || api.called("translations ") != before {
		t.Fatalf("en-US asked for translations (%v)", err)
	}
}
```

En `internal/server/identify_test.go`, reemplazá:

```go
		PosterPath: "/p.jpg", BackdropPath: "/b.jpg"}, nil
}
```

por:

```go
		PosterPath: "/p.jpg", BackdropPath: "/b.jpg"}, nil
}

func (f *fakeTMDB) Translations(ctx context.Context, id int) ([]tmdb.Translation, error) {
	return nil, nil
}
```

En `internal/tmdb/tmdb_test.go`, reemplazá:

```go
		t.Fatalf("got %q, %v", b, err)
	}
}

```

por:

```go
		t.Fatalf("got %q, %v", b, err)
	}
}

func TestTranslations(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/movie/7857/translations" {
			t.Errorf("request %s", r.URL)
		}
		w.Write([]byte(`{"id":7857,"translations":[{"iso_3166_1":"MX","iso_639_1":"es","name":"Español","english_name":"Spanish","data":{"homepage":"","overview":"Rimini.","runtime":123,"tagline":"","title":"Amarcord"}}]}`))
	})
	ts, err := c.Translations(context.Background(), 7857)
	if err != nil || len(ts) != 1 || ts[0].Tag() != "es-MX" || ts[0].Data.Title != "Amarcord" || ts[0].Data.Overview != "Rimini." {
		t.Fatalf("got %+v, %v", ts, err)
	}
}

```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/identify ./internal/tmdb ./internal/server`

Expected: FAIL, por ejemplo:

```
internal/tmdb/tmdb_test.go:144:15: c.Translations undefined (type *Client has no field or method Translations)
internal/server/identify_test.go:42:70: undefined: tmdb.Translation
internal/identify/fake_test.go:20:24: undefined: tmdb.Translation
internal/identify/fake_test.go:63:69: undefined: tmdb.Translation
FAIL	cinexplorer/internal/identify [build failed]
FAIL	cinexplorer/internal/tmdb [build failed]
```

- [ ] **Step 3: Implementar**

En `internal/identify/match.go`, reemplazá:

```go
	Movie(ctx context.Context, id int, lang string) (tmdb.Details, error)
}
```

por:

```go
	Movie(ctx context.Context, id int, lang string) (tmdb.Details, error)
	Translations(ctx context.Context, id int) ([]tmdb.Translation, error)
}
```

En `internal/identify/runner.go`, reemplazá:

```go
// fetchMovie gets a movie in the configured language, completing an empty
// title or overview in English.
```

por:

```go
// fetchMovie gets a movie in the configured language. Titles and overviews
// TMDB has not translated to it are taken from the closest translation
// (Chain), the overview finally in English.
```

En `internal/identify/runner.go`, reemplazá:

```go
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
```

por:

```go
	if r.Language != fallbackLang {
		ts, err := r.TMDB.Translations(ctx, id)
		if err != nil {
			return store.Movie{}, err
		}
		translate(&m, ts, r.Language)
```

Crear `internal/identify/translate.go`:

```go
package identify

import (
	"strings"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// preferred lists, for a language, the regional variants to fall back on,
// closest first.
var preferred = map[string][]string{
	"es": {"es-MX", "es-ES"},
	"pt": {"pt-BR", "pt-PT"},
	"en": {"en-US", "en-GB"},
}

// Chain is the order in which translations are tried for lang: lang itself,
// then the preferred variants of its language ("es-AR" → es-AR, es-MX,
// es-ES). Any other variant of the language comes after the chain.
func Chain(lang string) []string {
	out := []string{lang}
	base, _, _ := strings.Cut(lang, "-")
	for _, v := range preferred[base] {
		if v != lang {
			out = append(out, v)
		}
	}
	return out
}

// pick returns the first non-empty text of the translations in the chain of
// lang, then of any variant of its language.
func pick(ts []tmdb.Translation, lang string, text func(tmdb.Translation) string) string {
	for _, tag := range Chain(lang) {
		for _, t := range ts {
			if t.Tag() == tag && text(t) != "" {
				return text(t)
			}
		}
	}
	base, _, _ := strings.Cut(lang, "-")
	for _, t := range ts {
		if t.Language == base && text(t) != "" {
			return text(t)
		}
	}
	return ""
}

func title(t tmdb.Translation) string    { return t.Data.Title }
func overview(t tmdb.Translation) string { return t.Data.Overview }

// translate completes a movie fetched in lang with its translations: title
// and overview from the chain of lang. A title found nowhere in the language
// stays as TMDB gave it (usually the original); an overview, and a title
// still empty, fall back to English.
func translate(m *store.Movie, ts []tmdb.Translation, lang string) {
	if t := pick(ts, lang, title); t != "" {
		m.Title = t
	}
	if o := pick(ts, lang, overview); o != "" {
		m.Overview = o
	}
	if m.Title == "" {
		m.Title = pick(ts, fallbackLang, title)
	}
	if m.Overview == "" {
		m.Overview = pick(ts, fallbackLang, overview)
	}
}
```

En `internal/tmdb/tmdb.go`, reemplazá:

```go

// Image downloads an image by its TMDB path ("/abc.jpg") at a size ("w342").
```

por:

```go

// Translation is a movie's texts in one language and country
// (GET /movie/{id}/translations). Empty fields were not translated.
type Translation struct {
	Language string `json:"iso_639_1"`  // "es"
	Country  string `json:"iso_3166_1"` // "AR"
	Data     struct {
		Title    string `json:"title"`
		Overview string `json:"overview"`
	} `json:"data"`
}

// Tag is the translation's language as TMDB takes it ("es-AR").
func (t Translation) Tag() string { return t.Language + "-" + t.Country }

// Translations returns every translation of a movie's title and overview.
func (c *Client) Translations(ctx context.Context, id int) ([]Translation, error) {
	var out struct {
		Translations []Translation `json:"translations"`
	}
	err := c.get(ctx, "/movie/"+strconv.Itoa(id)+"/translations", &out)
	return out.Translations, err
}

// Image downloads an image by its TMDB path ("/abc.jpg") at a size ("w342").
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/identify/fake_test.go internal/identify/match.go internal/identify/runner.go internal/identify/runner_test.go internal/identify/translate.go internal/identify/translate_test.go internal/server/identify_test.go internal/tmdb/tmdb.go internal/tmdb/tmdb_test.go
git commit -m "feat(identify): titles and overviews from the closest translation (es-AR → es-MX → es-ES → en)"
```

---

### Task 4: Runner.Close: cortar y esperar la identificación en segundo plano

**Files:**
- Modify: `internal/identify/runner.go`
- Test: `internal/identify/runner_test.go`

Spec §3.1. Para reemplazar la configuración en caliente, la identificación en curso tiene que poder detenerse. Las corridas de `Trigger` usan ahora un contexto propio del `Runner` (antes `context.Background()`); `Close` lo cancela, cancela el reintento pendiente y espera a que la corrida termine. Después de `Close`, `Trigger` no hace nada y una corrida cancelada no programa reintentos ni deja el estado en `offline`. Lo guardado hasta el corte queda.

- [ ] **Step 1: Escribir los tests que fallan**

En `internal/identify/runner_test.go`, reemplazá:

```go
		t.Fatalf("gauchos %+v", g)
	}
}

```

por:

```go
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

```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/identify`

Expected: FAIL, por ejemplo:

```
internal/identify/runner_test.go:430:4: r.Close undefined (type *Runner has no field or method Close)
FAIL	cinexplorer/internal/identify [build failed]
```

- [ ] **Step 3: Implementar**

En `internal/identify/runner.go`, reemplazá:

```go
	cancel  func() // pending retry
}
```

por:

```go
	cancel  func() // pending retry
	// base is the context of the background runs; Close cancels it and
	// waits for them (wg).
	base   context.Context
	stop   context.CancelFunc
	closed bool
	wg     sync.WaitGroup
}
```

En `internal/identify/runner.go`, reemplazá:

```go
// more run follow it.
func (r *Runner) Trigger() {
	r.mu.Lock()
```

por:

```go
// more run follow it. After Close it does nothing.
func (r *Runner) Trigger() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
```

En `internal/identify/runner.go`, reemplazá:

```go
	r.mu.Unlock()
	go func() {
		for {
			if err := r.run(context.Background()); err != nil {
				log.Printf("identificación: %v", err)
			}
			r.mu.Lock()
			if !r.again {
```

por:

```go
	if r.base == nil {
		r.base, r.stop = context.WithCancel(context.Background())
	}
	ctx := r.base
	r.wg.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		for {
			if err := r.run(ctx); err != nil && ctx.Err() == nil {
				log.Printf("identificación: %v", err)
			}
			r.mu.Lock()
			if !r.again || r.closed {
```

En `internal/identify/runner.go`, reemplazá:

```go
	}()
}
```

por:

```go
	}()
}

// Close stops the background runs for good: it cancels the one in progress,
// and the pending retry, and waits for it to end. What it saved stays.
func (r *Runner) Close() {
	r.mu.Lock()
	r.closed = true
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if r.stop != nil {
		r.stop()
	}
	r.mu.Unlock()
	r.wg.Wait()
}
```

En `internal/identify/runner.go`, reemplazá:

```go
	switch {
	case errors.Is(err, httpx.ErrOffline):
```

por:

```go
	switch {
	case r.closed:
		r.status.State = StateIdle
		return nil
	case errors.Is(err, httpx.ErrOffline):
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/identify/runner.go internal/identify/runner_test.go
git commit -m "feat(identify): Runner.Close cancels and waits for background runs"
```

---

### Task 5: Paquete engine: lo que depende de config.json, reemplazable entero

**Files:**
- Create: `internal/engine/engine.go`
- Test: `internal/engine/engine_test.go` (nuevo)

Spec §3.1. `engine.Engine` arma a partir de un `config.Config` un `Runtime` con el cliente de TMDB, la caché de imágenes (una por `Runtime`: su `Fetch` no cambia después), el escáner y el `Runner`, y lo guarda en un `atomic.Pointer`. `Start` arma el primero y escanea, salvo en el primer uso (`setupPending`). `Apply` guarda `config.json`, detiene el `Runtime` actual (`Stop`: cancela el escaneo y la identificación y espera), arma otro y escanea; en modo consulta devuelve `ErrReadOnly`. `Runtime.Scan` lanza un escaneo con el contexto del `Runtime`; después de `Stop` no hace nada. `Use` pone un `Runtime` tal cual (para los tests del servidor).

Los tests no usan red: el `Apply` con token apunta a una carpeta vacía, así que no hay nada que identificar.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/engine/engine_test.go`:

```go
package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"cinexplorer/internal/config"
	"cinexplorer/internal/probe"
	"cinexplorer/internal/store"
)

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
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/engine`

Expected: FAIL, por ejemplo:

```
internal/engine/engine_test.go:18:46: undefined: Engine
internal/engine/engine_test.go:41:8: undefined: Engine
internal/engine/engine_test.go:51:32: undefined: Engine
internal/engine/engine_test.go:62:30: undefined: Engine
FAIL	cinexplorer/internal/engine [build failed]
```

- [ ] **Step 3: Implementar**

Crear `internal/engine/engine.go`:

```go
// Package engine builds, from config.json, the parts of the app that depend
// on it (TMDB client, image cache, scanner, identification runner) and
// replaces them all at once when the configuration changes.
package engine

import (
	"context"
	"errors"
	"log"
	"path/filepath"
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
// per request; it does not change after it is built.
type Runtime struct {
	Config     config.Config
	TMDB       identify.API     // nil without a token
	Images     *images.Cache    // its Fetch is the TMDB client of this runtime
	Scanner    *scan.Scanner    // nil in read-only mode
	Identifier *identify.Runner // nil in read-only mode

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	stopped bool
	scans   sync.WaitGroup
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
func (e *Engine) Use(rt *Runtime) { e.current.Store(rt) }

// SetupPending reports whether the first-use assistant has to run.
func (e *Engine) SetupPending() bool { return e.pending.Load() }

// Apply saves cfg as config.json and replaces the runtime: it stops the
// current one (cancelling its scan and identification), starts one built
// from cfg and scans. cfg must be valid.
func (e *Engine) Apply(cfg config.Config) error {
	if e.ReadOnly {
		return ErrReadOnly
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := config.Save(e.AppDir, cfg); err != nil {
		return err
	}
	if old := e.current.Load(); old != nil {
		old.Stop()
	}
	rt := e.build(cfg)
	e.current.Store(rt)
	e.pending.Store(false)
	rt.Scan()
	return nil
}

func (e *Engine) build(cfg config.Config) *Runtime {
	rt := &Runtime{Config: cfg, Images: &images.Cache{Dir: filepath.Join(e.AppDir, "cache"), ReadOnly: e.ReadOnly}}
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

// Scan starts a scan in the background; nothing in read-only mode, after
// Stop, or while one is running.
func (rt *Runtime) Scan() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.Scanner == nil || rt.stopped {
		return
	}
	if rt.ctx == nil {
		rt.ctx, rt.cancel = context.WithCancel(context.Background())
	}
	ctx := rt.ctx
	rt.scans.Add(1)
	go func() {
		defer rt.scans.Done()
		if err := rt.Scanner.Run(ctx); err != nil && !errors.Is(err, scan.ErrBusy) && ctx.Err() == nil {
			log.Printf("escaneo: %v", err)
		}
	}()
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
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/engine.go internal/engine/engine_test.go
git commit -m "feat(engine): runtime built from config.json, replaced as a whole by Apply"
```

---

### Task 6: El servidor lee el runtime del engine; el primer arranque espera los ajustes

**Files:**
- Modify: `cmd/cinexplorer/main.go`, `internal/server/catalog.go`, `internal/server/identify.go`, `internal/server/server.go`
- Test: `cmd/cinexplorer/main_test.go`, `internal/server/catalog_test.go`, `internal/server/identify_test.go`, `internal/server/server_test.go`

Spec §3.2 y §3.3. `Server` pierde `Roots`, `Scanner`, `TMDB`, `Identifier`, `Images` y `Language`, y suma `Engine`: cada handler lee `s.rt()` una vez por pedido. `/api/status` suma `setupPending`; `POST /api/scan` usa `Runtime.Scan`.

`setup` ya no guarda `config.json` en el primer arranque: con la carpeta escribible hace `Engine.Start(propuesta, true)` y no escanea hasta que se guarden los ajustes. La función que cierra el catálogo detiene antes el `Runtime`. El smoke test pasa a guardar la propuesta con `Engine.Apply` (la Task 11 lo cambia por `PUT /api/config`).

- [ ] **Step 1: Escribir los tests que fallan**

En `cmd/cinexplorer/main_test.go`, reemplazá:

```go
// movie folder, no config.json and no TMDB token.
```

por:

```go
// movie folder, no config.json and no TMDB token; then the settings are
// saved as proposed.
```

En `cmd/cinexplorer/main_test.go`, reemplazá:

```go
	deadline := time.Now().Add(10 * time.Second)
	for st := srv.Scanner.Status(); st.Running || st.Finished.IsZero(); st = srv.Scanner.Status() {
```

por:

```go
	// First start: nothing saved or scanned until the settings are.
	time.Sleep(50 * time.Millisecond)
	if !srv.Engine.SetupPending() || !srv.Engine.Current().Scanner.Status().Finished.IsZero() {
		t.Fatal("scanned before the first-use settings")
	}
	if _, err := os.Stat(filepath.Join(appDir, "config.json")); err == nil {
		t.Fatal("config.json written before the first-use settings")
	}
	cfg := srv.Engine.Current().Config
	if len(cfg.Roots) != 1 || cfg.Roots[0] != "../cine" || cfg.Language != "es-AR" {
		t.Fatalf("proposed %+v", cfg)
	}
	if err := srv.Engine.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for st := srv.Engine.Current().Scanner.Status(); st.Running || st.Finished.IsZero(); st = srv.Engine.Current().Scanner.Status() {
```

En `internal/server/catalog_test.go`, reemplazá:

```go
	s.Roots = []string{"../cine", "../cine-ordenar"}
```

por:

```go
	s.rt().Config.Roots = []string{"../cine", "../cine-ordenar"}
```

En `internal/server/catalog_test.go`, reemplazá:

```go
	}, s.Roots); err != nil {
```

por:

```go
	}, s.rt().Config.Roots); err != nil {
```

En `internal/server/catalog_test.go`, reemplazá:

```go
	s.Identifier.Adopt(context.Background(), 7857)
```

por:

```go
	s.rt().Identifier.Adopt(context.Background(), 7857)
```

En `internal/server/identify_test.go`, reemplazá:

```go
	s.TMDB, s.Language = f, "es-ES"
	s.Identifier = &identify.Runner{Store: s.Store, TMDB: f, Language: "es-ES"}
	s.Images = &images.Cache{Dir: t.TempDir(), Fetch: f}
```

por:

```go
	rt := s.rt()
	rt.TMDB = f
	rt.Identifier = &identify.Runner{Store: s.Store, TMDB: f, Language: "es-ES"}
	rt.Images = &images.Cache{Dir: t.TempDir(), Fetch: f}
```

En `internal/server/identify_test.go`, reemplazá:

```go
	s.TMDB = nil
```

por:

```go
	s.rt().TMDB = nil
```

En `internal/server/identify_test.go`, reemplazá:

```go
	if s.Images.Has(images.Poster, 7857, "/other.jpg") {
		t.Fatal("candidate preview was cached")
	}
	s.Identifier.Adopt(context.Background(), 7857)
```

por:

```go
	if s.rt().Images.Has(images.Poster, 7857, "/other.jpg") {
		t.Fatal("candidate preview was cached")
	}
	s.rt().Identifier.Adopt(context.Background(), 7857)
```

En `internal/server/identify_test.go`, reemplazá:

```go
	s.Identifier.TMDB = nil
```

por:

```go
	s.rt().Identifier.TMDB = nil
```

En `internal/server/server_test.go`, reemplazá:

```go
	"cinexplorer/internal/catalog"
	"cinexplorer/internal/grouping"
```

por:

```go
	"cinexplorer/internal/catalog"
	"cinexplorer/internal/config"
	"cinexplorer/internal/engine"
	"cinexplorer/internal/grouping"
```

En `internal/server/server_test.go`, reemplazá:

```go
	}
	return s, &opened
```

por:

```go
	}
	s.Engine = &engine.Engine{AppDir: s.AppDir, Store: st}
	s.Engine.Use(&engine.Runtime{Config: config.Config{Roots: []string{"../cine"}, Language: "es-ES"}})
	return s, &opened
```

En `internal/server/server_test.go`, reemplazá:

```go
	s.Scanner = &scan.Scanner{AppDir: s.AppDir, Store: s.Store}
```

por:

```go
	s.rt().Scanner = &scan.Scanner{AppDir: s.AppDir, Store: s.Store}
```

En `internal/server/server_test.go`, reemplazá:

```go
		st := s.Scanner.Status()
```

por:

```go
		st := s.rt().Scanner.Status()
```

En `internal/server/server_test.go`, reemplazá:

```go
	s.Scanner = &scan.Scanner{}
```

por:

```go
	s.rt().Scanner = &scan.Scanner{}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/server ./cmd/cinexplorer`

Expected: FAIL, por ejemplo:

```
cmd/cinexplorer/main_test.go:38:10: srv.Engine undefined (type *server.Server has no field or method Engine)
cmd/cinexplorer/main_test.go:44:13: srv.Engine undefined (type *server.Server has no field or method Engine)
cmd/cinexplorer/main_test.go:48:16: srv.Engine undefined (type *server.Server has no field or method Engine)
cmd/cinexplorer/main_test.go:52:16: srv.Engine undefined (type *server.Server has no field or method Engine)
FAIL	cinexplorer/internal/server [build failed]
FAIL	cinexplorer/cmd/cinexplorer [build failed]
```

- [ ] **Step 3: Implementar**

En `cmd/cinexplorer/main.go`, reemplazá:

```go
import (
	"context"
	"errors"
```

por:

```go
import (
	"errors"
```

En `cmd/cinexplorer/main.go`, reemplazá:

```go
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/platform"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/server"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
```

por:

```go
	"cinexplorer/internal/engine"
	"cinexplorer/internal/platform"
	"cinexplorer/internal/server"
	"cinexplorer/internal/store"
```

En `cmd/cinexplorer/main.go`, reemplazá:

```go
// setup opens the catalog in the app directory and wires the server: TMDB
// when there is a token, and, when the directory is writable, the scanner
// (started right away) and the identification runner. closeStore closes the
// catalog.
```

por:

```go
// setup opens the catalog in the app directory and wires the server around
// an engine built from config.json. Without config.json (a first start) and
// with a writable directory, nothing is saved or scanned until the first-use
// assistant saves the settings; otherwise the scan starts right away.
// closeStore stops the engine and closes the catalog.
```

En `cmd/cinexplorer/main.go`, reemplazá:

```go
	}
	if created && !readOnly {
		if err := config.Save(appDir, cfg); err != nil {
			return nil, nil, err
		}
		log.Printf("config.json creado con raíces %v", cfg.Roots)
	}

```

por:

```go
	}

```

En `cmd/cinexplorer/main.go`, reemplazá:

```go
	srv = &server.Server{AppDir: appDir, Roots: cfg.Roots, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal,
		Language: cfg.Language, Images: &images.Cache{Dir: filepath.Join(appDir, "cache"), ReadOnly: readOnly}}
	var api *tmdb.Client
	if cfg.TMDBToken != "" {
		api = tmdb.New(cfg.TMDBToken)
		srv.TMDB, srv.Images.Fetch = api, api
	} else {
		log.Print("sin tmdbToken en config.json: no se identifican películas")
	}
	if readOnly {
		log.Print("el directorio de la app no es escribible: modo consulta")
	} else {
		runner := &identify.Runner{AppDir: appDir, Store: st, Wikidata: wikidata.New(version), Images: srv.Images,
			Language: cfg.Language, Prefetch: cfg.ImagePrefetch}
		if api != nil {
			runner.TMDB = api // only when set: a nil *tmdb.Client in the interface would not read as "no token"
		}
		srv.Identifier = runner
		srv.Scanner = &scan.Scanner{AppDir: appDir, Roots: cfg.Roots, Store: st, OnDone: runner.Trigger}
		go func() {
			if err := srv.Scanner.Run(context.Background()); err != nil {
				log.Printf("escaneo: %v", err)
			}
		}()
	}
	return srv, st.Close, nil
```

por:

```go
	eng := &engine.Engine{AppDir: appDir, Store: st, ReadOnly: readOnly, Wikidata: wikidata.New(version)}
	pending := created && !readOnly
	eng.Start(cfg, pending)
	if readOnly {
		log.Print("el directorio de la app no es escribible: modo consulta")
	}
	if pending {
		log.Print("primer uso: elegí las carpetas y el token en el navegador")
	}
	srv = &server.Server{AppDir: appDir, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal,
		Engine: eng}
	return srv, func() error {
		eng.Current().Stop()
		return st.Close()
	}, nil
```

En `internal/server/catalog.go`, reemplazá:

```go
	all := catalog.Items(snap, s.Roots)
```

por:

```go
	all := catalog.Items(snap, s.rt().Config.Roots)
```

En `internal/server/catalog.go`, reemplazá:

```go
	writeJSON(w, catalog.Duplicates(snap, s.Roots))
```

por:

```go
	writeJSON(w, catalog.Duplicates(snap, s.rt().Config.Roots))
```

Reemplazá `internal/server/identify.go` completo por:

```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"cinexplorer/internal/httpx"
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

var imdbIDRe = regexp.MustCompile(`^tt\d{7,8}$`)

// search is the manual TMDB search: by title (and optional year), or by
// IMDb id when q is one.
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	rt := s.rt()
	if rt.TMDB == nil {
		http.Error(w, "sin token de TMDB", http.StatusServiceUnavailable)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	query := identify.Query{Title: q, Year: year}
	if imdbIDRe.MatchString(q) {
		query = identify.Query{IMDbID: q}
	}
	cands, _, err := identify.Search(r.Context(), rt.TMDB, rt.Config.Language, query)
	if err != nil {
		tmdbError(w, err)
		return
	}
	writeJSON(w, cands)
}

// identify records the user's decision about a version's fingerprint.
func (s *Server) identify(w http.ResponseWriter, r *http.Request) {
	runner := s.rt().Identifier
	if s.ReadOnly || runner == nil {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return
	}
	var req struct {
		Fingerprint string `json:"fingerprint"`
		Action      string `json:"action"` // movie | extra | ignore | reset
		TMDBID      int    `json:"tmdbId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	// Checked first so an unknown fingerprint costs no TMDB request.
	known, err := s.Store.IsRepresentative(req.Fingerprint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !known {
		http.Error(w, store.ErrUnknownFingerprint.Error(), http.StatusNotFound)
		return
	}
	switch req.Action {
	case "movie", "extra":
		if req.TMDBID <= 0 {
			http.Error(w, "falta tmdbId", http.StatusBadRequest)
			return
		}
		if err := runner.Adopt(r.Context(), req.TMDBID); err != nil {
			tmdbError(w, err)
			return
		}
		status := store.StatusManual
		if req.Action == "extra" {
			status = store.StatusExtra
		}
		if err = s.Store.SetCorrection(req.Fingerprint, status, req.TMDBID); err == nil {
			runner.Trigger() // Wikidata and images for the new movie
		}
	case "ignore":
		err = s.Store.SetCorrection(req.Fingerprint, store.StatusIgnored, 0)
	case "reset":
		if err = s.Store.ResetIdentification(req.Fingerprint); err == nil {
			runner.Trigger()
		}
	default:
		http.Error(w, "acción desconocida", http.StatusBadRequest)
		return
	}
	switch {
	case errors.Is(err, store.ErrUnknownFingerprint):
		http.Error(w, err.Error(), http.StatusNotFound)
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func tmdbError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tmdb.ErrNotFound):
		http.Error(w, "no existe en TMDB", http.StatusNotFound)
	case errors.Is(err, httpx.ErrOffline):
		http.Error(w, "sin conexión con TMDB", http.StatusServiceUnavailable)
	case errors.Is(err, identify.ErrNoToken):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, tmdb.ErrUnauthorized):
		http.Error(w, err.Error(), http.StatusBadGateway)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// image serves /img/{poster|backdrop}/{tmdbId}.jpg from the cache,
// downloading it once when missing. A candidate that is not a stored movie
// yet passes its TMDB path as ?p=. Pages add the image's version as ?v= so
// that browsers can keep it for good.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	kind := images.Kind(r.PathValue("kind"))
	idText, ok := strings.CutSuffix(r.PathValue("file"), ".jpg")
	id, err := strconv.Atoi(idText)
	cache := s.rt().Images
	if cache == nil || !images.ValidKind(kind) || !ok || err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	m, found, err := s.Store.Movie(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var b []byte
	control := "no-cache"
	if found {
		path := m.PosterPath
		if kind == images.Backdrop {
			path = m.BackdropPath
		}
		b, err = cache.Get(r.Context(), kind, id, path)
		// ?v= names the image at one path: the URL changes with the path.
		if v := r.URL.Query().Get("v"); v != "" && v == images.Version(path) {
			control = "public, max-age=31536000, immutable"
		}
	} else {
		// ?p= comes from the page (any page can send it): shown, never cached.
		b, err = cache.Preview(r.Context(), kind, id, r.URL.Query().Get("p"))
		control = "max-age=86400"
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Header().Set("Cache-Control", control)
	w.Write(b)
}
```

En `internal/server/server.go`, reemplazá:

```go
	"context"
	"encoding/json"
	"errors"
```

por:

```go
	"encoding/json"
```

En `internal/server/server.go`, reemplazá:

```go
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
```

por:

```go
	"cinexplorer/internal/engine"
	"cinexplorer/internal/identify"
```

En `internal/server/server.go`, reemplazá:

```go
	Roots    []string // catalog-form paths, as in config.json
	Store    *store.Store
	Scanner  *scan.Scanner // nil in read-only mode
	ReadOnly bool
	Opener   func(target string) error
	Revealer func(target string) error

	TMDB       identify.API     // nil without a TMDB token
	Identifier *identify.Runner // nil in read-only mode
	Images     *images.Cache
	Language   string

	Static fs.FS // the web app; nil: the embedded build
}
```

por:

```go
	Store    *store.Store
	ReadOnly bool
	Opener   func(target string) error
	Revealer func(target string) error
	// Engine holds what config.json decides (roots, TMDB, scanner,
	// identification, images, language); handlers read it through rt.
	Engine *engine.Engine

	Static fs.FS // the web app; nil: the embedded build
}

// rt is the runtime in use. A handler reads it once: settings saved during
// the request replace it, they do not change it.
func (s *Server) rt() *engine.Runtime { return s.Engine.Current() }
```

En `internal/server/server.go`, reemplazá:

```go
	var st scan.Status
	if s.Scanner != nil {
		st = s.Scanner.Status()
	}
	var id *identify.Status
	if s.Identifier != nil {
		st := s.Identifier.Status()
		id = &st
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "scan": st, "identify": id})
```

por:

```go
	rt := s.rt()
	var st scan.Status
	if rt.Scanner != nil {
		st = rt.Scanner.Status()
	}
	var id *identify.Status
	if rt.Identifier != nil {
		st := rt.Identifier.Status()
		id = &st
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "setupPending": s.Engine.SetupPending(), "scan": st, "identify": id})
```

En `internal/server/server.go`, reemplazá:

```go
	if s.ReadOnly || s.Scanner == nil {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return
	}
	go func() {
		if err := s.Scanner.Run(context.Background()); err != nil && !errors.Is(err, scan.ErrBusy) {
			log.Printf("escaneo: %v", err)
		}
	}()
```

por:

```go
	rt := s.rt()
	if s.ReadOnly || rt.Scanner == nil {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return
	}
	rt.Scan()
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add cmd/cinexplorer/main.go cmd/cinexplorer/main_test.go internal/server/catalog.go internal/server/catalog_test.go internal/server/identify.go internal/server/identify_test.go internal/server/server.go internal/server/server_test.go
git commit -m "feat(server): read the engine's runtime per request; first start waits for the settings"
```

---

### Task 7: API de ajustes

**Files:**
- Create: `internal/server/config.go`
- Modify: `internal/server/server.go`
- Test: `internal/server/config_test.go` (nuevo)

Spec §4.2 y §4.3. `GET /api/config` devuelve la configuración como la muestran el asistente y Ajustes: raíces con disponibilidad, carpetas hermanas sugeridas, si hay token y sus últimos 4 caracteres (el token nunca vuelve), idioma e idiomas ofrecidos (más el guardado si no está en la lista), descarga de imágenes. `POST /api/config/root` valida una carpeta con `config.Root`. `POST /api/config/token` verifica un token pidiendo *Fight Club* (id 550) **en un solo intento** y con 15 s de límite (con los reintentos del cliente, sin red la respuesta tardaba 15 s): `{"valid": true|false|null}`, `null` cuando TMDB no respondió. `PUT /api/config` valida (raíces, idioma, descarga de imágenes), conserva el token si viene ausente o `null`, lo quita con `""`, y aplica con `Engine.Apply`: 400 si algo no valida, 409 en modo consulta. `Server.VerifyToken` deja a los tests reemplazar la verificación.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/server/config_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cinexplorer/internal/config"
	"cinexplorer/internal/engine"
	"cinexplorer/internal/httpx"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// configServer is a first start: <tmp>/cinexplorer next to cine/ and
// ordenar/ (both empty), no config.json, with a token set in memory.
func configServer(t *testing.T) *Server {
	t.Helper()
	disk := t.TempDir()
	appDir := filepath.Join(disk, "cinexplorer")
	for _, d := range []string{appDir, filepath.Join(disk, "cine"), filepath.Join(disk, "ordenar")} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := &Server{AppDir: appDir, Store: st, Engine: &engine.Engine{AppDir: appDir, Store: st}}
	s.Engine.Start(config.Config{Roots: []string{"../cine"}, TMDBToken: "eyJsecret1234", Language: "es-AR"}, true)
	t.Cleanup(func() { s.Engine.Current().Stop() })
	return s
}

func send(t *testing.T, s *Server, method, url, body string, v any) int {
	t.Helper()
	rec := request(s.Handler(), method, url, body, "application/json", "127.0.0.1")
	if rec.Code == http.StatusOK && v != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("%s %s: %s (%v)", method, url, rec.Body, err)
		}
	}
	return rec.Code
}

func TestGetConfig(t *testing.T) {
	s := configServer(t)
	var v configView
	if code := getJSON(t, s, "/api/config", &v); code != 200 {
		t.Fatalf("status %d", code)
	}
	want := configView{SetupPending: true, Roots: []rootView{{"../cine", true}}, Suggested: []string{"../ordenar"},
		HasToken: true, TokenHint: "…1234", Language: "es-AR", Languages: config.Languages, ImagePrefetch: "none"}
	if !reflect.DeepEqual(v, want) {
		t.Fatalf("got %+v", v)
	}
	rec := request(s.Handler(), "GET", "/api/config", "", "", "127.0.0.1")
	if b := rec.Body.String(); b == "" || strings.Contains(b, "eyJsecret") {
		t.Fatalf("token sent back: %s", b)
	}
	var st map[string]any
	getJSON(t, s, "/api/status", &st)
	if st["setupPending"] != true {
		t.Fatalf("status %v", st)
	}
}

func TestCheckRootEndpoint(t *testing.T) {
	s := configServer(t)
	var v rootView
	abs := filepath.Join(filepath.Dir(s.AppDir), "ordenar")
	if code := send(t, s, "POST", "/api/config/root", fmt.Sprintf(`{"path":%q}`, abs), &v); code != 200 || v != (rootView{"../ordenar", true}) {
		t.Fatalf("%d %+v", code, v)
	}
	for _, p := range []string{"../nada", ".", ""} {
		if code := send(t, s, "POST", "/api/config/root", fmt.Sprintf(`{"path":%q}`, p), nil); code != http.StatusBadRequest {
			t.Errorf("%q: %d", p, code)
		}
	}
}

func TestCheckTokenEndpoint(t *testing.T) {
	s := configServer(t)
	for _, tc := range []struct {
		err  error
		want string
	}{
		{nil, `{"valid":true}`},
		{tmdb.ErrNotFound, `{"valid":true}`},
		{tmdb.ErrUnauthorized, `{"valid":false}`},
		{fmt.Errorf("%w: dial", httpx.ErrOffline), `{"valid":null}`},
	} {
		var got string
		s.VerifyToken = func(ctx context.Context, token string) error {
			got = token
			return tc.err
		}
		rec := request(s.Handler(), "POST", "/api/config/token", `{"token":" eyJnew "}`, "application/json", "127.0.0.1")
		if rec.Code != 200 || got != "eyJnew" || rec.Body.String() != tc.want+"\n" {
			t.Errorf("%v: %d %s (token %q)", tc.err, rec.Code, rec.Body, got)
		}
	}
	if code := send(t, s, "POST", "/api/config/token", `{"token":""}`, nil); code != http.StatusBadRequest {
		t.Fatalf("empty token: %d", code)
	}
}

func TestPutConfig(t *testing.T) {
	s := configServer(t)
	old := s.Engine.Current()
	var v configView
	// null keeps the token (no movies to identify: no network).
	body := `{"roots":["../cine","../ordenar"],"token":null,"language":"pt-BR","imagePrefetch":"posters"}`
	if code := send(t, s, "PUT", "/api/config", body, &v); code != 200 {
		t.Fatalf("status %d", code)
	}
	if v.SetupPending || !v.HasToken || v.Language != "pt-BR" || len(v.Roots) != 2 || len(v.Suggested) != 0 || s.Engine.Current() == old {
		t.Fatalf("got %+v", v)
	}
	saved, _, _ := config.Load(s.AppDir)
	want := config.Config{Roots: []string{"../cine", "../ordenar"}, TMDBToken: "eyJsecret1234", Language: "pt-BR", ImagePrefetch: "posters"}
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("saved %+v", saved)
	}
	// "" removes it.
	if code := send(t, s, "PUT", "/api/config", `{"roots":["../cine"],"token":"","language":"es-AR"}`, &v); code != 200 || v.HasToken {
		t.Fatalf("%d %+v", code, v)
	}
	if s.Engine.Current().TMDB != nil {
		t.Fatal("TMDB client without a token")
	}
}

func TestPutConfigRejects(t *testing.T) {
	s := configServer(t)
	for _, body := range []string{
		`{"roots":[],"language":"es-AR"}`,
		`{"roots":["../nada"],"language":"es-AR"}`,
		`{"roots":["../cine"],"language":"xx-XX"}`,
		`{"roots":["../cine"],"language":"es-AR","imagePrefetch":"some"}`,
		`{"roots":`,
	} {
		if code := send(t, s, "PUT", "/api/config", body, nil); code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, code)
		}
	}
	if !s.Engine.SetupPending() {
		t.Fatal("a rejected PUT applied the settings")
	}
	s.ReadOnly = true
	if code := send(t, s, "PUT", "/api/config", `{"roots":["../cine"],"language":"es-AR"}`, nil); code != http.StatusConflict {
		t.Fatalf("read-only: %d", code)
	}
}

func TestPutConfigKeepsSavedLanguage(t *testing.T) {
	s := configServer(t)
	s.Engine.Current().Config.Language = "es-ES" // an older config.json
	var v configView
	if code := send(t, s, "PUT", "/api/config", `{"roots":["../cine"],"language":"es-ES"}`, &v); code != 200 || v.Language != "es-ES" {
		t.Fatalf("%d %+v", code, v)
	}
	if v.Languages[len(v.Languages)-1] != "es-ES" {
		t.Fatalf("languages %v", v.Languages)
	}
}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/server`

Expected: FAIL, por ejemplo:

```
internal/server/config_test.go:56:8: undefined: configView
internal/server/config_test.go:60:10: undefined: configView
internal/server/config_test.go:60:50: undefined: rootView
internal/server/config_test.go:78:8: undefined: rootView
FAIL	cinexplorer/internal/server [build failed]
```

- [ ] **Step 3: Implementar**

Crear `internal/server/config.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"cinexplorer/internal/config"
	"cinexplorer/internal/httpx"
	"cinexplorer/internal/tmdb"
)

type rootView struct {
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

// configView is the configuration as the settings show it: the token is
// never sent back, only its last characters.
type configView struct {
	SetupPending  bool       `json:"setupPending"`
	ReadOnly      bool       `json:"readOnly"`
	Roots         []rootView `json:"roots"`
	Suggested     []string   `json:"suggested"` // sibling folders that are not roots
	HasToken      bool       `json:"hasToken"`
	TokenHint     string     `json:"tokenHint"`
	Language      string     `json:"language"`
	Languages     []string   `json:"languages"`
	ImagePrefetch string     `json:"imagePrefetch"`
}

func (s *Server) configView() configView {
	cfg := s.rt().Config
	v := configView{SetupPending: s.Engine.SetupPending(), ReadOnly: s.ReadOnly, Roots: []rootView{}, Suggested: []string{},
		HasToken: cfg.TMDBToken != "", Language: cfg.Language, Languages: config.Languages,
		ImagePrefetch: cfg.ImagePrefetch}
	for _, r := range cfg.Roots {
		v.Roots = append(v.Roots, rootView{r, config.Available(s.AppDir, r)})
	}
	siblings, _ := config.DefaultRoots(s.AppDir)
	for _, r := range siblings {
		if !slices.Contains(cfg.Roots, r) {
			v.Suggested = append(v.Suggested, r)
		}
	}
	if len(cfg.TMDBToken) >= 8 {
		v.TokenHint = "…" + cfg.TMDBToken[len(cfg.TMDBToken)-4:]
	}
	if v.ImagePrefetch == "" {
		v.ImagePrefetch = config.PrefetchModes[0]
	}
	if !slices.Contains(v.Languages, v.Language) {
		v.Languages = append(slices.Clone(v.Languages), v.Language)
	}
	return v
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.configView())
}

// checkRoot turns a folder the user typed into a root, or says why it
// cannot be one.
func (s *Server) checkRoot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	root, err := config.Root(s.AppDir, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, rootView{root, true})
}

// verifyToken asks TMDB for a well-known movie with the token, once: the
// person is waiting for the answer.
func verifyToken(ctx context.Context, token string) error {
	c := tmdb.New(token)
	c.HTTP.Attempts = 1
	_, err := c.Movie(ctx, 550, "en-US")
	return err
}

// checkToken tells whether TMDB takes a token: valid true or false, or null
// when TMDB could not be reached.
func (s *Server) checkToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		http.Error(w, "falta el token", http.StatusBadRequest)
		return
	}
	verify := s.VerifyToken
	if verify == nil {
		verify = verifyToken
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	var valid *bool
	switch err := verify(ctx, token); {
	case errors.Is(err, tmdb.ErrUnauthorized):
		valid = new(bool)
	case errors.Is(err, httpx.ErrOffline), errors.Is(err, context.DeadlineExceeded):
	default: // any other answer means TMDB took the token
		valid = new(bool)
		*valid = true
	}
	writeJSON(w, map[string]*bool{"valid": valid})
}

// putConfig validates and saves the settings, and restarts what depends on
// them. A missing or null token keeps the saved one; "" removes it.
func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly {
		http.Error(w, "modo consulta: la configuración no se puede modificar", http.StatusConflict)
		return
	}
	var req struct {
		Roots         []string `json:"roots"`
		Token         *string  `json:"token"`
		Language      string   `json:"language"`
		ImagePrefetch string   `json:"imagePrefetch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	cur := s.rt().Config
	if err := config.CheckRoots(s.AppDir, req.Roots, cur.Roots); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !slices.Contains(config.Languages, req.Language) && req.Language != cur.Language {
		http.Error(w, "idioma desconocido: "+req.Language, http.StatusBadRequest)
		return
	}
	if req.ImagePrefetch != "" && !slices.Contains(config.PrefetchModes, req.ImagePrefetch) {
		http.Error(w, "descarga de imágenes desconocida: "+req.ImagePrefetch, http.StatusBadRequest)
		return
	}
	cfg := config.Config{Roots: req.Roots, TMDBToken: cur.TMDBToken, Language: req.Language, ImagePrefetch: req.ImagePrefetch}
	if req.Token != nil {
		cfg.TMDBToken = strings.TrimSpace(*req.Token)
	}
	if err := s.Engine.Apply(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.configView())
}
```

En `internal/server/server.go`, reemplazá:

```go
import (
	"encoding/json"
```

por:

```go
import (
	"context"
	"encoding/json"
```

En `internal/server/server.go`, reemplazá:

```go
	Engine *engine.Engine

```

por:

```go
	Engine *engine.Engine
	// VerifyToken asks TMDB whether it takes a token; nil means asking
	// for a well-known movie.
	VerifyToken func(ctx context.Context, token string) error

```

En `internal/server/server.go`, reemplazá:

```go
	mux.HandleFunc("GET /img/{kind}/{file}", s.image)
	return localOnly(mux)
```

por:

```go
	mux.HandleFunc("GET /img/{kind}/{file}", s.image)
	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("PUT /api/config", jsonOnly(s.putConfig))
	mux.HandleFunc("POST /api/config/root", jsonOnly(s.checkRoot))
	mux.HandleFunc("POST /api/config/token", jsonOnly(s.checkToken))
	return localOnly(mux)
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/server/config.go internal/server/config_test.go internal/server/server.go
git commit -m "feat(server): settings API (read, check a folder, verify the token, save and apply)"
```

---

### Task 8: Índice de búsqueda FTS5 en memoria

**Files:**
- Create: `internal/catalog/search.go`, `internal/search/search.go`
- Test: `internal/catalog/search_test.go` (nuevo), `internal/search/search_test.go` (nuevo)

Spec §6.1. `internal/search` guarda un documento por ítem de Explorar en una tabla FTS5 de una base SQLite en memoria (una sola conexión: la base vive en ella), con el tokenizador `unicode61 remove_diacritics 2` (sin tildes ni mayúsculas). `Refresh` la reconstruye solo si cambió el contador del catálogo. `Query` parte el texto con `Terms` (`quality.Words`), pone cada palabra entre comillas (así nada de la sintaxis de FTS5 llega a la consulta) y la última como prefijo; menos de 2 letras o dígitos no busca. Orden: `bm25` con pesos título 10, original 8, directores 4, reparto 1, archivos 2; desempate por orden de inserción (título).

En `catalog`: `SearchDocs` arma los documentos (película: títulos, directores, reparto y los títulos parseados de sus archivos; sin identificar: título y director parseados), `ItemKey` la clave de cada ítem y `Directors` los directores de los ítems encontrados cuyo nombre coincide, con cuántas películas del catálogo dirigieron.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/catalog/search_test.go`:

```go
package catalog

import (
	"testing"

	"cinexplorer/internal/quality"
	"cinexplorer/internal/search"
	"cinexplorer/internal/store"
)

func TestSearchDocs(t *testing.T) {
	snap := snapshot()
	m := snap.Movies[7857]
	m.Cast = []store.CastMember{{ID: 1, Name: "Magali Noël"}, {ID: 2, Name: "Bruno Zanin"}}
	snap.Movies[7857] = m
	docs := SearchDocs(snap)
	byKey := map[string]search.Doc{}
	var order []string
	for _, d := range docs {
		byKey[d.Key] = d
		order = append(order, d.Title)
	}
	want := search.Doc{Key: "movie:7857", Title: "Amarcord", Original: "Amarcord", Directors: "Federico Fellini",
		Cast: "Magali Noël, Bruno Zanin", Files: "Amarcord"}
	if byKey["movie:7857"] != want {
		t.Errorf("amarcord %+v", byKey["movie:7857"])
	}
	if d := byKey["n1"]; d.Title != "Novecento" || d.Directors != "Bertolucci" || d.Original != "" {
		t.Errorf("novecento %+v", d)
	}
	if _, ok := byKey["x1"]; ok {
		t.Error("a version that is not a movie is searchable")
	}
	if _, ok := byKey["g1"]; ok {
		t.Error("a missing version is searchable")
	}
	for i := 1; i < len(docs); i++ {
		if quality.NormTitle(order[i-1]) > quality.NormTitle(order[i]) {
			t.Errorf("not in title order: %v", order)
			break
		}
	}
}

func TestDirectors(t *testing.T) {
	all := Items(snapshot(), roots)
	got := Directors(all, all, search.Terms("fell"), 5)
	if len(got) != 1 || got[0] != (DirectorHit{4415, "Federico Fellini", 2}) {
		t.Fatalf("got %+v", got)
	}
	for _, q := range []string{"federico x", "ellini", "bertolucci"} { // Bertolucci is only parsed
		if got := Directors(all, all, search.Terms(q), 5); len(got) != 0 {
			t.Errorf("%q: %+v", q, got)
		}
	}
	if got := Directors(all, all, nil, 5); got == nil || len(got) != 0 {
		t.Errorf("no terms: %#v", got)
	}
}
```

Crear `internal/search/search_test.go`:

```go
package search

import (
	"slices"
	"testing"
)

var docs = []Doc{
	{Key: "movie:1", Title: "El ángel exterminador", Original: "El ángel exterminador", Directors: "Luis Buñuel", Cast: "Silvia Pinal"},
	{Key: "movie:2", Title: "Mujeres al borde de un ataque de nervios", Directors: "Pedro Almodóvar", Cast: "Carmen Maura, Antonio Banderas"},
	{Key: "movie:3", Title: "La piel que habito", Directors: "Pedro Almodóvar", Cast: "Antonio Banderas"},
	{Key: "movie:4", Title: "Banderas rojas", Directors: "Otro"},
	{Key: "f123", Title: "Rip mentecato", Files: "Rip mentecato"},
	{Key: "movie:5", Title: "8½", Original: "Otto e mezzo", Directors: "Federico Fellini", Files: "Fellini 8 y medio"},
}

func index(t *testing.T) *Index {
	t.Helper()
	x, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { x.Close() })
	if err := x.Refresh(1, func() []Doc { return docs }); err != nil {
		t.Fatal(err)
	}
	return x
}

func TestQuery(t *testing.T) {
	x := index(t)
	for q, want := range map[string][]string{
		"angel":              {"movie:1"},
		"ÁNGEL EXTER":        {"movie:1"},
		"bunu":               {"movie:1"},
		"almodóvar piel":     {"movie:3"},
		"mentecato":          {"f123"},
		"otto mezzo":         {"movie:5"},
		"8":                  nil, // too short
		"a":                  nil,
		`"; DROP -x:y* OR (`: nil, // FTS5 syntax is only words here
		"nada que ver":       nil,
		"felli":              {"movie:5"},
	} {
		got, err := x.Query(q)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("Query(%q) = %v, %v; want %v", q, got, err, want)
		}
	}
}

func TestQueryRanksTitleFirst(t *testing.T) {
	x := index(t)
	got, err := x.Query("banderas")
	if err != nil || len(got) != 3 || got[0] != "movie:4" || !slices.Contains(got, "movie:2") || !slices.Contains(got, "movie:3") {
		t.Fatalf("got %v, %v", got, err)
	}
	got, _ = x.Query("almodovar")
	slices.Sort(got)
	if !slices.Equal(got, []string{"movie:2", "movie:3"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRefreshOnlyWhenChanged(t *testing.T) {
	x := index(t)
	calls := 0
	more := func() []Doc { calls++; return append(slices.Clone(docs), Doc{Key: "movie:9", Title: "Stalker"}) }
	if err := x.Refresh(1, more); err != nil || calls != 0 {
		t.Fatalf("rebuilt with the same counter (%v)", err)
	}
	if got, _ := x.Query("stalker"); got != nil {
		t.Fatalf("got %v", got)
	}
	if err := x.Refresh(2, more); err != nil || calls != 1 {
		t.Fatalf("not rebuilt (%v)", err)
	}
	if got, _ := x.Query("stalker"); !slices.Equal(got, []string{"movie:9"}) {
		t.Fatalf("got %v", got)
	}
	if got, _ := x.Query("almodovar"); len(got) != 2 {
		t.Fatalf("rebuilt twice the same docs? %v", got)
	}
}
```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/search ./internal/catalog`

Expected: FAIL, por ejemplo:

```
internal/search/search_test.go:8:14: undefined: Doc
internal/search/search_test.go:17:27: undefined: Index
internal/search/search_test.go:19:12: undefined: New
internal/search/search_test.go:24:34: undefined: Doc
FAIL	cinexplorer/internal/search [build failed]
FAIL	cinexplorer/internal/catalog [build failed]
```

- [ ] **Step 3: Implementar**

Crear `internal/catalog/search.go`:

```go
package catalog

import (
	"cmp"
	"slices"
	"strings"

	"cinexplorer/internal/quality"
	"cinexplorer/internal/search"
	"cinexplorer/internal/store"
)

// ItemKey identifies an item in the search index: "movie:<TMDB id>", or a
// version's key.
func ItemKey(it *Item) string { return it.id() }

// SearchDocs gives the search index one document per item of Explorar, in
// title order: a movie's titles, directors and cast plus the titles parsed
// from its files; a version not identified, its parsed title and director.
func SearchDocs(snap store.Snapshot) []search.Doc {
	es := build(snap, nil)
	slices.SortStableFunc(es, func(a, b *entry) int {
		return cmp.Or(cmp.Compare(a.item.norm, b.item.norm), cmp.Compare(a.item.id(), b.item.id()))
	})
	docs := make([]search.Doc, 0, len(es))
	for _, e := range es {
		it := &e.item
		d := search.Doc{Key: it.id(), Title: it.Title, Directors: strings.Join(it.Directors, ", ")}
		if it.Kind == KindMovie {
			m := snap.Movies[it.TMDBID]
			d.Original = m.OriginalTitle
			var cast, files []string
			for _, c := range m.Cast {
				cast = append(cast, c.Name)
			}
			for _, v := range e.versions {
				if v.Title != "" && !slices.Contains(files, v.Title) {
					files = append(files, v.Title)
				}
			}
			d.Cast, d.Files = strings.Join(cast, ", "), strings.Join(files, " · ")
		}
		docs = append(docs, d)
	}
	return docs
}

// DirectorHit is a director whose name matches a search.
type DirectorHit struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"` // movies of the catalog they directed
}

// Directors lists the directors of the found items whose name has every
// term (the last one as the start of a word), with how many of all the items
// they directed; the most prolific first, at most limit.
func Directors(all, found []Item, terms []string, limit int) []DirectorHit {
	if len(terms) == 0 {
		return []DirectorHit{}
	}
	names := map[int]string{}
	for _, it := range found {
		for i, id := range it.directorIDs {
			if id > 0 && nameMatches(it.Directors[i], terms) {
				names[id] = it.Directors[i]
			}
		}
	}
	out := []DirectorHit{}
	for id, name := range names {
		n := 0
		for _, it := range all {
			if slices.Contains(it.directorIDs, id) {
				n++
			}
		}
		out = append(out, DirectorHit{id, name, n})
	}
	slices.SortFunc(out, func(a, b DirectorHit) int {
		return cmp.Or(-cmp.Compare(a.Count, b.Count), cmp.Compare(quality.NormTitle(a.Name), quality.NormTitle(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out[:min(limit, len(out))]
}

func nameMatches(name string, terms []string) bool {
	words := quality.Words(name)
	for i, t := range terms {
		ok := slices.ContainsFunc(words, func(w string) bool {
			if i == len(terms)-1 {
				return strings.HasPrefix(w, t)
			}
			return w == t
		})
		if !ok {
			return false
		}
	}
	return true
}
```

Crear `internal/search/search.go`:

```go
// Package search is the instant search of the catalog: a full-text index
// (SQLite FTS5) kept in memory and rebuilt from the catalog's items when the
// catalog changes. It ignores case and accents and matches the last word as
// a prefix, so results come while typing.
package search

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"cinexplorer/internal/quality"
)

// Doc is what the index keeps of one item of the catalog.
type Doc struct {
	Key       string // the item's key (catalog.ItemKey)
	Title     string
	Original  string
	Directors string
	Cast      string
	Files     string // titles parsed from the item's file names
}

// MinLength is how many letters or digits a query needs.
const MinLength = 2

// rank weighs the columns: title, original title, directors, cast, files
// (the key is not indexed).
const rank = `bm25(docs, 0, 10, 8, 4, 1, 2)`

type Index struct {
	mu      sync.Mutex
	db      *sql.DB
	changes int64
	built   bool
}

// New returns an empty index.
func New() (*Index, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	// An in-memory database lives in its connection: keep exactly one.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	if _, err := db.Exec(`CREATE VIRTUAL TABLE docs USING fts5(key UNINDEXED, title, original, directors, cast, files,
		tokenize = "unicode61 remove_diacritics 2")`); err != nil {
		db.Close()
		return nil, err
	}
	return &Index{db: db}, nil
}

func (x *Index) Close() error { return x.db.Close() }

// Refresh rebuilds the index from docs() when the catalog changed since the
// last build: changes is the catalog's change counter (store.Snapshot).
// docs are indexed in order, which breaks ties between equal ranks.
func (x *Index) Refresh(changes int64, docs func() []Doc) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.built && x.changes == changes {
		return nil
	}
	tx, err := x.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM docs`); err != nil {
		return err
	}
	ins, err := tx.Prepare(`INSERT INTO docs (key, title, original, directors, cast, files) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, d := range docs() {
		if _, err := ins.Exec(d.Key, d.Title, d.Original, d.Directors, d.Cast, d.Files); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	x.changes, x.built = changes, true
	return nil
}

// Terms splits a query into folded words (no case, no accents).
func Terms(q string) []string { return quality.Words(q) }

// expression is the FTS5 query for terms: all of them, the last as a prefix.
// Terms are letters and digits only, so quoting them is enough.
func expression(terms []string) string {
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = `"` + t + `"`
	}
	parts[len(parts)-1] += "*"
	return strings.Join(parts, " ")
}

// Query returns the keys of the items that match q, best first. A query
// with fewer than MinLength letters or digits matches nothing.
func (x *Index) Query(q string) ([]string, error) {
	terms := Terms(q)
	if len([]rune(strings.Join(terms, ""))) < MinLength {
		return nil, nil
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	rows, err := x.db.Query(fmt.Sprintf(`SELECT key FROM docs WHERE docs MATCH ? ORDER BY %s, rowid`, rank), expression(terms))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/search.go internal/catalog/search_test.go internal/search/search.go internal/search/search_test.go
git commit -m "feat(search): in-memory FTS5 index of the catalog's items; matching directors"
```

---

### Task 9: API de búsqueda

**Files:**
- Create: `internal/server/search.go`
- Modify: `internal/server/server.go`, `internal/store/snapshot.go`
- Test: `internal/server/search_test.go` (nuevo), `internal/store/roots_test.go`

Spec §6.2. `GET /api/search?q=&limit=` (`limit` de 1 a 200, 8 por defecto) → `{q, total, items, directors}`, con `items` en la forma de Explorar. Leer la instantánea cuesta ~135 ms con 5.000 películas, así que el servidor guarda los ítems y el índice y los reutiliza mientras `Store.Changes()` (el contador leído solo, sin la instantánea) no cambie: con el catálogo quieto, una búsqueda tarda 1–25 ms. El campo del servidor se llama `finder` porque `search` ya es el handler de la búsqueda en TMDB.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/server/search_test.go`:

```go
package server

import (
	"net/http"
	"testing"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
)

type searchResult struct {
	Q         string                `json:"q"`
	Total     int                   `json:"total"`
	Items     []catalog.Item        `json:"items"`
	Directors []catalog.DirectorHit `json:"directors"`
}

func TestSearchCatalog(t *testing.T) {
	s, _ := identifyServer(t)
	var res searchResult
	// Not identified yet: found by the title of its file name.
	if code := getJSON(t, s, "/api/search?q=amarc", &res); code != http.StatusOK || res.Total != 1 ||
		res.Items[0].Kind != catalog.KindVersion || res.Items[0].Key != "f1" {
		t.Fatalf("%d %+v", code, res)
	}
	// Identified: the index follows the catalog.
	s.rt().Identifier.Adopt(t.Context(), 7857)
	if err := s.Store.SetCorrection("f1", store.StatusManual, 7857); err != nil {
		t.Fatal(err)
	}
	m, _, _ := s.Store.Movie(7857)
	m.Directors = []store.Person{{ID: 4415, Name: "Federico Fellini"}}
	if err := s.Store.SaveMovie(m); err != nil {
		t.Fatal(err)
	}
	if code := getJSON(t, s, "/api/search?q=FELLÍNI", &res); code != http.StatusOK || res.Total != 1 ||
		res.Items[0].TMDBID != 7857 || len(res.Directors) != 1 || res.Directors[0].Count != 1 {
		t.Fatalf("%d %+v", code, res)
	}
	if code := getJSON(t, s, "/api/search?q=a", &res); code != http.StatusOK || res.Total != 0 || res.Items == nil || res.Directors == nil {
		t.Fatalf("short query: %d %+v", code, res)
	}
}

func TestSearchLimit(t *testing.T) {
	s, _ := newServer(t)
	// Two contents titled Amarcord: two items.
	other := "../cine/Amarcord.avi"
	if err := s.Store.SyncFiles([]store.FileRow{
		{Path: moviePath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"},
		{Path: other, Size: 20, MTime: 1, Fingerprint: "f2", Kind: "video"},
	}, []string{"../cine"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ReplaceVersions([]grouping.Version{
		{Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
			Members: []grouping.Member{{Path: moviePath, Role: grouping.RoleMain}}},
		{Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 20, Parts: 1,
			Members: []grouping.Member{{Path: other, Role: grouping.RoleMain}}},
	}); err != nil {
		t.Fatal(err)
	}
	var res searchResult
	getJSON(t, s, "/api/search?q=amarcord&limit=1", &res)
	if res.Total != 2 || len(res.Items) != 1 {
		t.Fatalf("got %+v", res)
	}
	getJSON(t, s, "/api/search?q=amarcord&limit=0", &res) // out of range: the default
	if len(res.Items) != 2 {
		t.Fatalf("got %+v", res)
	}
}
```

En `internal/store/roots_test.go`, reemplazá:

```go
		t.Fatalf("a write left changes at %d", c.Changes)
	}
}

```

por:

```go
		t.Fatalf("a write left changes at %d", c.Changes)
	}
	if n, err := s.Changes(); err != nil || n != c.Changes {
		t.Fatalf("Changes() = %d, %v; snapshot %d", n, err, c.Changes)
	}
}

```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/server ./internal/store`

Expected: FAIL, por ejemplo:

```
internal/store/roots_test.go:53:17: s.Changes undefined (type *Store has no field or method Changes)
--- FAIL: TestSearchCatalog (0.00s)
--- FAIL: TestSearchLimit (0.00s)
FAIL	cinexplorer/internal/server	0.130s
FAIL	cinexplorer/internal/store [build failed]
```

- [ ] **Step 3: Implementar**

Crear `internal/server/search.go`:

```go
package server

import (
	"net/http"
	"strconv"
	"sync"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/search"
)

// searchState is what the search keeps between requests: the index and the
// items it was built from, current while the catalog's change counter stays.
type searchState struct {
	once  sync.Once
	index *search.Index
	err   error

	mu      sync.Mutex
	built   bool
	changes int64
	items   []catalog.Item
	byKey   map[string]catalog.Item
}

// searchItems returns the items of the catalog, with the index up to date.
// Typing makes a request per pause: while nothing changes they are reused
// instead of reading the whole catalog each time.
func (s *Server) searchItems() (*search.Index, []catalog.Item, map[string]catalog.Item, error) {
	st := &s.finder
	st.once.Do(func() { st.index, st.err = search.New() })
	if st.err != nil {
		return nil, nil, nil, st.err
	}
	changes, err := s.Store.Changes()
	if err != nil {
		return nil, nil, nil, err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.built && st.changes == changes {
		return st.index, st.items, st.byKey, nil
	}
	snap, err := s.Store.Snapshot()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := st.index.Refresh(snap.Changes, func() []search.Doc { return catalog.SearchDocs(snap) }); err != nil {
		return nil, nil, nil, err
	}
	st.items = catalog.Items(snap, nil)
	st.byKey = make(map[string]catalog.Item, len(st.items))
	for i := range st.items {
		st.byKey[catalog.ItemKey(&st.items[i])] = st.items[i]
	}
	st.built, st.changes = true, snap.Changes
	return st.index, st.items, st.byKey, nil
}

// find is the instant search: the items whose titles, directors, cast or
// file names match q, best first, and the directors whose name matches.
func (s *Server) find(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 200 {
		limit = 8
	}
	idx, all, byKey, err := s.searchItems()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	keys, err := idx.Query(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	found := []catalog.Item{}
	for _, k := range keys {
		if it, ok := byKey[k]; ok {
			found = append(found, it)
		}
	}
	writeJSON(w, map[string]any{"q": q, "total": len(found), "items": found[:min(limit, len(found))],
		"directors": catalog.Directors(all, found, search.Terms(q), 5)})
}
```

En `internal/server/server.go`, reemplazá:

```go
	Static fs.FS // the web app; nil: the embedded build
}
```

por:

```go
	Static fs.FS // the web app; nil: the embedded build

	finder searchState
}
```

En `internal/server/server.go`, reemplazá:

```go
	mux.HandleFunc("GET /img/{kind}/{file}", s.image)
	mux.HandleFunc("GET /api/config", s.getConfig)
```

por:

```go
	mux.HandleFunc("GET /img/{kind}/{file}", s.image)
	mux.HandleFunc("GET /api/search", s.find)
	mux.HandleFunc("GET /api/config", s.getConfig)
```

En `internal/store/snapshot.go`, reemplazá:

```go
	return f(tx)
}
```

por:

```go
	return f(tx)
}

// Changes is the change counter of Snapshot, read on its own: a cheap way to
// know whether what was derived from the last snapshot is still current.
func (s *Store) Changes() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT total_changes()`).Scan(&n)
	return n, err
}
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/server/search.go internal/server/search_test.go internal/server/server.go internal/store/roots_test.go internal/store/snapshot.go
git commit -m "feat(server): catalog search endpoint"
```

---

### Task 10: Filas del Inicio

**Files:**
- Create: `internal/catalog/home.go`
- Modify: `internal/server/catalog.go`, `internal/server/server.go`
- Test: `internal/catalog/home_test.go` (nuevo), `internal/server/catalog_test.go`

Spec §7.1 y §7.2. `catalog.HomePage(snap, seed)` arma las filas con las películas identificadas y presentes: agregadas recientemente (por fecha de alta) y, sorteadas con `math/rand/v2` (PCG sembrado con `seed`), una década (≥ 6 películas), un director con id de TMDB (≥ 3), un país (≥ 6), un género (≥ 6) y una colección (≥ 2). Década, país y género van mezclados; director y colección, por año. Hasta 20 ítems por fila; una fila sin candidatos no aparece. Los valores se ordenan antes de sortear, así que la misma semilla da las mismas filas. `GET /api/home?seed=` la devuelve.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/catalog/home_test.go`:

```go
package catalog

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"cinexplorer/internal/store"
)

// homeSnapshot has 14 movies: 1970–1976 and 1980–1986; the first 8 by
// director 1 (the 1970s ones and 1980), countries alternate IT and FR, all
// Drama, and two in collection 5. Plus an unidentified version.
func homeSnapshot() store.Snapshot {
	snap := store.Snapshot{Identifications: map[string]*store.Identification{}, Movies: map[int]store.Movie{}}
	years := []int{1970, 1971, 1972, 1973, 1974, 1975, 1976, 1980, 1981, 1982, 1983, 1984, 1985, 1986}
	for i, y := range years {
		id := 100 + i
		fp := fmt.Sprintf("f%d", i)
		v := identified(version(int64(i+1), "../cine", fp+".mkv", fp, "x", y, "1080p", 10, int64(1000+i)), store.StatusAuto, id)
		snap.Versions = append(snap.Versions, v)
		m := store.Movie{TMDBID: id, Title: fmt.Sprintf("Película %02d", i), Year: y, Genres: []string{"Drama"},
			Countries: []string{[]string{"IT", "FR"}[i%2]}, BackdropPath: fmt.Sprintf("/b%d.jpg", i), PosterPath: "/p.jpg"}
		if i < 8 {
			m.Directors = []store.Person{{ID: 1, Name: "Director Uno"}}
		} else {
			m.Directors = []store.Person{{ID: 0, Name: "Sin id"}} // from Wikidata: not a facet
		}
		if i == 3 || i == 9 {
			m.CollectionID, m.Collection = 5, "Saga"
		}
		snap.Movies[id] = m
	}
	snap.Versions = append(snap.Versions, version(99, "../cine", "Stalker.avi", "s1", "Stalker", 1979, "", 10, 5000))
	return snap
}

func TestHomePage(t *testing.T) {
	h := HomePage(homeSnapshot(), 1)
	if h.Total != 14 {
		t.Fatalf("total %d", h.Total)
	}
	var kinds []string
	rows := map[string]HomeRow{}
	for _, r := range h.Rows {
		kinds = append(kinds, r.Kind)
		rows[r.Kind] = r
	}
	if want := []string{RowRecent, RowDecade, RowDirector, RowCountry, RowGenre, RowCollection}; !slices.Equal(kinds, want) {
		t.Fatalf("rows %v", kinds)
	}
	recent := rows[RowRecent]
	if recent.Href != "/explorar?orden=agregado" || len(recent.Items) != 14 || recent.Items[0].TMDBID != 113 || recent.Items[13].TMDBID != 100 {
		t.Fatalf("recent %+v", recent)
	}
	if it := recent.Items[0]; it.Backdrop != "b13" || it.Poster != "p" || it.Year != 1986 || !slices.Equal(it.Countries, []string{"FR"}) {
		t.Fatalf("item %+v", it)
	}
	if d := rows[RowDecade]; (d.Value != "1970" && d.Value != "1980") || d.Label != d.Value+"s" || d.Href != "/explorar?decada="+d.Value || len(d.Items) != 7 {
		t.Fatalf("decade %+v", d)
	}
	dir := rows[RowDirector]
	if dir.Value != "1" || dir.Label != "Director Uno" || dir.Href != "/explorar?director=1" || len(dir.Items) != 8 || dir.Items[0].Year != 1970 || dir.Items[7].Year != 1980 {
		t.Fatalf("director %+v", dir)
	}
	if c := rows[RowCountry]; (c.Value != "IT" && c.Value != "FR") || c.Label != c.Value || len(c.Items) != 7 {
		t.Fatalf("country %+v", c)
	}
	if g := rows[RowGenre]; g.Value != "Drama" || g.Href != "/explorar?genero=Drama" || len(g.Items) != 14 {
		t.Fatalf("genre %+v", g)
	}
	if c := rows[RowCollection]; c.Value != "5" || c.Label != "Saga" || c.Href != "/explorar?coleccion=5" ||
		len(c.Items) != 2 || c.Items[0].Year != 1973 {
		t.Fatalf("collection %+v", c)
	}
}

func TestHomePageSeed(t *testing.T) {
	snap := homeSnapshot()
	if a, b := HomePage(snap, 7), HomePage(snap, 7); !reflect.DeepEqual(a, b) {
		t.Fatal("the same seed gave other rows")
	}
	// Some seed picks another decade or shuffles the genre row otherwise.
	first := HomePage(snap, 0)
	for seed := uint64(1); seed < 20; seed++ {
		if !reflect.DeepEqual(HomePage(snap, seed), first) {
			return
		}
	}
	t.Fatal("every seed gave the same rows")
}

func TestHomePageMinimums(t *testing.T) {
	snap := homeSnapshot()
	for id := 106; id < 114; id++ { // keep 6 movies: 1970–1975
		delete(snap.Movies, id)
	}
	h := HomePage(snap, 1)
	var kinds []string
	for _, r := range h.Rows {
		kinds = append(kinds, r.Kind)
	}
	// 6 by director 1 (≥3); 3 per country (<6); 1 in the collection (<2).
	if want := []string{RowRecent, RowDecade, RowDirector, RowGenre}; !slices.Equal(kinds, want) || h.Total != 6 {
		t.Fatalf("rows %v total %d", kinds, h.Total)
	}
	if h := HomePage(store.Snapshot{}, 1); h.Total != 0 || h.Rows == nil || len(h.Rows) != 0 {
		t.Fatalf("empty: %+v", h)
	}
}
```

En `internal/server/catalog_test.go`, reemplazá:

```go
		t.Fatalf("suggestions %+v", refs)
	}
}

```

por:

```go
		t.Fatalf("suggestions %+v", refs)
	}
}

func TestHomeEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	var h catalog.Home
	if code := getJSON(t, s, "/api/home?seed=3", &h); code != 200 || h.Total != 0 || h.Rows == nil {
		t.Fatalf("%d %+v", code, h)
	}
	s.rt().Identifier.Adopt(context.Background(), 7857)
	if err := s.Store.SetCorrection("f1", store.StatusManual, 7857); err != nil {
		t.Fatal(err)
	}
	if code := getJSON(t, s, "/api/home?seed=x", &h); code != 200 || h.Total != 1 || len(h.Rows) != 1 ||
		h.Rows[0].Kind != catalog.RowRecent || h.Rows[0].Items[0].Backdrop != "b" {
		t.Fatalf("%d %+v", code, h)
	}
}

```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `go test ./internal/catalog ./internal/server`

Expected: FAIL, por ejemplo:

```
internal/catalog/home_test.go:40:7: undefined: HomePage
internal/catalog/home_test.go:45:21: undefined: HomeRow
internal/catalog/home_test.go:50:22: undefined: RowRecent
internal/catalog/home_test.go:50:33: undefined: RowDecade
FAIL	cinexplorer/internal/catalog [build failed]
FAIL	cinexplorer/internal/server [build failed]
```

- [ ] **Step 3: Implementar**

Crear `internal/catalog/home.go`:

```go
package catalog

import (
	"cmp"
	"math/rand/v2"
	"net/url"
	"slices"
	"strconv"

	"cinexplorer/internal/images"
	"cinexplorer/internal/store"
)

// Row kinds of the home page.
const (
	RowRecent     = "recent"
	RowDecade     = "decade"
	RowDirector   = "director"
	RowCountry    = "country"
	RowGenre      = "genre"
	RowCollection = "collection"
)

// rowSize is how many movies a row shows at most.
const rowSize = 20

// HomeItem is a movie as a home row shows it.
type HomeItem struct {
	TMDBID        int      `json:"tmdbId"`
	Title         string   `json:"title"`
	OriginalTitle string   `json:"originalTitle"`
	Year          int      `json:"year"`
	Directors     []string `json:"directors"`
	Countries     []string `json:"countries"`
	Backdrop      string   `json:"backdrop"` // image version, "" without one
	Poster        string   `json:"poster"`
}

// HomeRow is one row of the home page. Value is the facet value it shows
// (a decade, a director id…), Label its name, Href Explorar with that facet.
type HomeRow struct {
	Kind  string     `json:"kind"`
	Value string     `json:"value"`
	Label string     `json:"label"`
	Href  string     `json:"href"`
	Items []HomeItem `json:"items"`
}

type Home struct {
	Total int       `json:"total"` // identified movies present
	Rows  []HomeRow `json:"rows"`
}

// group is a candidate for a random row: the movies that share a value.
type group struct {
	value, label string
	movies       []*entry
}

// HomePage builds the home rows from the identified movies: the ones added
// last, then a random decade, director, country, genre and TMDB collection
// with enough movies. The same seed gives the same rows.
func HomePage(snap store.Snapshot, seed uint64) Home {
	var movies []*entry
	for _, e := range build(snap, nil) {
		if e.item.Kind == KindMovie {
			movies = append(movies, e)
		}
	}
	home := Home{Total: len(movies), Rows: []HomeRow{}}
	if len(movies) == 0 {
		return home
	}
	rng := rand.New(rand.NewPCG(seed, 0x636978706c6f7265)) // "cixplore"

	recent := slices.Clone(movies)
	slices.SortStableFunc(recent, func(a, b *entry) int {
		return cmp.Or(-cmp.Compare(a.item.Added, b.item.Added), cmp.Compare(a.item.norm, b.item.norm))
	})
	home.Rows = append(home.Rows, row(snap, RowRecent, group{movies: recent}, "/explorar?orden=agregado"))

	add := func(kind, facet string, min int, byYear bool, values func(*entry) (vals, labels []string)) {
		groups := map[string]*group{}
		for _, e := range movies {
			vals, labels := values(e)
			for i, v := range vals {
				g := groups[v]
				if g == nil {
					g = &group{value: v, label: labels[i]}
					groups[v] = g
				}
				g.movies = append(g.movies, e)
			}
		}
		var keys []string
		for v, g := range groups {
			if len(g.movies) >= min {
				keys = append(keys, v)
			}
		}
		if len(keys) == 0 {
			return
		}
		slices.Sort(keys) // maps have no order: the seed alone decides
		g := groups[keys[rng.IntN(len(keys))]]
		if byYear {
			slices.SortStableFunc(g.movies, func(a, b *entry) int {
				return cmp.Or(cmp.Compare(a.item.Year, b.item.Year), cmp.Compare(a.item.norm, b.item.norm))
			})
		} else {
			rng.Shuffle(len(g.movies), func(i, j int) { g.movies[i], g.movies[j] = g.movies[j], g.movies[i] })
		}
		home.Rows = append(home.Rows, row(snap, kind, *g, "/explorar?"+url.Values{facet: {g.value}}.Encode()))
	}
	add(RowDecade, FacetDecade, 6, false, func(e *entry) ([]string, []string) {
		if e.item.Year == 0 {
			return nil, nil
		}
		d := strconv.Itoa(e.item.Year / 10 * 10)
		return []string{d}, []string{d + "s"}
	})
	add(RowDirector, FacetDirector, 3, true, func(e *entry) (vals, labels []string) {
		for i, id := range e.item.directorIDs {
			if id > 0 {
				vals, labels = append(vals, strconv.Itoa(id)), append(labels, e.item.Directors[i])
			}
		}
		return
	})
	add(RowCountry, FacetCountry, 6, false, func(e *entry) ([]string, []string) {
		return e.item.Countries, e.item.Countries
	})
	add(RowGenre, FacetGenre, 6, false, func(e *entry) ([]string, []string) {
		return e.item.genres, e.item.genres
	})
	add(RowCollection, FacetCollection, 2, true, func(e *entry) ([]string, []string) {
		if e.item.collectionID == 0 {
			return nil, nil
		}
		return []string{strconv.Itoa(e.item.collectionID)}, []string{e.item.collection}
	})
	return home
}

func row(snap store.Snapshot, kind string, g group, href string) HomeRow {
	r := HomeRow{Kind: kind, Value: g.value, Label: g.label, Href: href, Items: []HomeItem{}}
	for _, e := range g.movies[:min(rowSize, len(g.movies))] {
		it := e.item
		m := snap.Movies[it.TMDBID]
		r.Items = append(r.Items, HomeItem{TMDBID: it.TMDBID, Title: it.Title, OriginalTitle: it.OriginalTitle,
			Year: it.Year, Directors: it.Directors, Countries: it.Countries,
			Backdrop: images.Version(m.BackdropPath), Poster: it.Poster})
	}
	return r
}
```

En `internal/server/catalog.go`, reemplazá:

```go
	writeJSON(w, map[string]any{"total": len(items), "query": q, "items": items, "facets": catalog.Counts(all, q.Facets)})
}
```

por:

```go
	writeJSON(w, map[string]any{"total": len(items), "query": q, "items": items, "facets": catalog.Counts(all, q.Facets)})
}

// home answers the home page: rows of movies, drawn with the page's seed.
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	seed, _ := strconv.ParseUint(r.URL.Query().Get("seed"), 10, 64)
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.HomePage(snap, seed))
}
```

En `internal/server/server.go`, reemplazá:

```go
	mux.HandleFunc("GET /api/search", s.find)
	mux.HandleFunc("GET /api/config", s.getConfig)
```

por:

```go
	mux.HandleFunc("GET /api/search", s.find)
	mux.HandleFunc("GET /api/home", s.home)
	mux.HandleFunc("GET /api/config", s.getConfig)
```

- [ ] **Step 4: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/home.go internal/catalog/home_test.go internal/server/catalog.go internal/server/catalog_test.go internal/server/server.go
git commit -m "feat(catalog): home rows (recent, decade, director, country, genre, collection) drawn from a seed"
```

---

### Task 11: Smoke test del primer uso

**Files:**
- Test: `cmd/cinexplorer/main_test.go`

Spec §10. El smoke test recorre el primer uso por HTTP: sin `config.json`, `/api/status` informa `setupPending`, `/api/config` propone `../cine` y `es-AR`, y no se crea el archivo; `PUT /api/config` lo guarda y escanea; después la película aparece en Explorar y en la búsqueda, y las rutas nuevas de la app (`/buscar`, `/bienvenida`, `/ajustes`) devuelven el `index.html` embebido. Prueba comportamiento que ya existe: tiene que pasar de entrada.

- [ ] **Step 1: Escribir el test**

Reemplazá `cmd/cinexplorer/main_test.go` completo por:

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
// movie folder, no config.json and no TMDB token. The first-use settings are
// saved as proposed, and the movie shows up.
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
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	call := func(method, path, body string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	get := func(path string) (int, string) { t.Helper(); return call("GET", path, "") }

	// First start: nothing saved or scanned until the settings are.
	if code, body := get("/api/status"); code != 200 || !strings.Contains(body, `"setupPending":true`) {
		t.Fatalf("status %d %s", code, body)
	}
	code, body := get("/api/config")
	var cfg struct {
		Roots []struct {
			Path string `json:"path"`
		} `json:"roots"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal([]byte(body), &cfg); err != nil || code != 200 || len(cfg.Roots) != 1 ||
		cfg.Roots[0].Path != "../cine" || cfg.Language != "es-AR" {
		t.Fatalf("config %d %s (%v)", code, body, err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "config.json")); err == nil {
		t.Fatal("config.json written before the first-use settings")
	}
	if code, body := call("PUT", "/api/config", `{"roots":["../cine"],"language":"es-AR"}`); code != 200 {
		t.Fatalf("put %d %s", code, body)
	}
	if _, err := os.Stat(filepath.Join(appDir, "config.json")); err != nil {
		t.Fatalf("config.json: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for st := srv.Engine.Current().Scanner.Status(); st.Running || st.Finished.IsZero(); st = srv.Engine.Current().Scanner.Status() {
		if time.Now().After(deadline) {
			t.Fatalf("scan did not finish: %+v", st)
		}
		time.Sleep(20 * time.Millisecond)
	}

	for _, p := range []string{"/", "/explorar?decada=1970", "/buscar?q=amarcord", "/bienvenida", "/ajustes"} {
		if code, body := get(p); code != 200 || !strings.Contains(strings.ToLower(body), "<!doctype html>") {
			t.Errorf("%s: %d %.200s", p, code, body)
		}
	}
	code, body = get("/api/explore?decada=1970")
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
	if code, body := get("/api/search?q=amarc"); code != 200 || !strings.Contains(body, `"total":1`) {
		t.Errorf("search %d %s", code, body)
	}
	if code, body := get("/api/status"); code != 200 || !strings.Contains(body, `"state":"noToken"`) ||
		!strings.Contains(body, `"setupPending":false`) {
		t.Errorf("status %d %s", code, body)
	}
}
```

- [ ] **Step 2: Correr el test**

Run: `go test ./cmd/cinexplorer`

Expected: `ok` (el comportamiento ya está implementado en las tareas anteriores).

- [ ] **Step 3: Correr los tests**

Run: `gofmt -l . && go vet ./... && go test ./...`

Expected: `gofmt` no lista nada; todos los paquetes `ok`.

- [ ] **Step 4: Commit**

```bash
git add cmd/cinexplorer/main_test.go
git commit -m "test: smoke test of the first-use settings"
```

---

### Task 12: Lógica pura del frontend: Inicio, búsqueda, ajustes y rutas

**Files:**
- Create: `web/src/lib/home.js`, `web/src/lib/search.js`, `web/src/lib/settings.js`
- Modify: `web/src/lib/format.js`, `web/src/lib/router.js`, `web/src/lib/status.js`
- Test: `web/src/lib/format.test.js`, `web/src/lib/home.test.js` (nuevo), `web/src/lib/router.test.js`, `web/src/lib/search.test.js` (nuevo), `web/src/lib/settings.test.js` (nuevo), `web/src/lib/status.test.js`

Spec §6.3, §7.3 y §8. Módulos sin estado, con tests:

- `router.js`: `/` es el Inicio (antes redirigía a Explorar); `/buscar`, `/ajustes`, `/bienvenida`.
- `format.js`: `creditLine`, la línea "DIRECTOR · PAÍS · AÑO" de las tarjetas del Inicio.
- `home.js`: título de cada fila y la semilla de la visita en `sessionStorage` (si no se puede usar, se sortea cada vez).
- `search.js`: cuándo buscar, adónde lleva cada resultado, la selección con flechas, el destino de Enter y el atajo Ctrl+K / ⌘K.
- `settings.js`: el borrador de los ajustes a partir de `GET /api/config`, el cuerpo del `PUT` (`null` conserva el token) y si hay cambios.
- `status.js`: los textos que decían "config.json" nombran Ajustes; `tokenProblem`.

`App.svelte` todavía no tiene la página de Inicio (llega en la Task 13): entre esta tarea y la siguiente, `/` no muestra nada.

- [ ] **Step 1: Escribir los tests que fallan**

En `web/src/lib/format.test.js`, reemplazá:

```js
  country,
  duration,
```

por:

```js
  country,
  creditLine,
  duration,
```

En `web/src/lib/format.test.js`, reemplazá:

```js

describe('misc', () => {
```

por:

```js

describe('creditLine', () => {
  it('joins director, countries and year', () => {
    expect(creditLine({ directors: ['Federico Fellini', 'Otro'], countries: ['IT', 'FR', 'DE'], year: 1973 })).toBe(
      'Federico Fellini · Italia, Francia · 1973',
    )
    expect(creditLine({ directors: [], countries: [], year: 0 })).toBe('')
    expect(creditLine({ year: 1979 })).toBe('1979')
  })
})

describe('misc', () => {
```

Crear `web/src/lib/home.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { homeSeed, rowTitle, saveSeed } from './home.js'

describe('rowTitle', () => {
  it.each([
    [{ kind: 'recent' }, 'Agregadas recientemente'],
    [{ kind: 'decade', value: '1970', label: '1970s' }, 'Los 70'],
    [{ kind: 'decade', value: '1920', label: '1920s' }, 'Los 20'],
    [{ kind: 'decade', value: '2000', label: '2000s' }, 'Los 2000'],
    [{ kind: 'decade', value: '1910', label: '1910s' }, 'Los 1910'],
    [{ kind: 'director', value: '4415', label: 'Federico Fellini' }, 'Dirigidas por Federico Fellini'],
    [{ kind: 'country', value: 'IT', label: 'IT' }, 'Cine de Italia'],
    [{ kind: 'genre', value: 'Drama', label: 'Drama' }, 'Drama'],
    [{ kind: 'collection', value: '10', label: 'El Padrino - Colección' }, 'El Padrino - Colección'],
  ])('%o', (row, want) => {
    expect(rowTitle(row)).toBe(want)
  })
})

function memory() {
  const data = {}
  return { getItem: (k) => data[k] ?? null, setItem: (k, v) => (data[k] = v) }
}

describe('homeSeed', () => {
  it('keeps the seed for the visit', () => {
    const s = memory()
    const seed = homeSeed(s)
    expect(seed).toBeGreaterThan(0)
    expect(homeSeed(s)).toBe(seed)
    expect(saveSeed(s, 42)).toBe(42)
    expect(homeSeed(s)).toBe(42)
  })
  it('works without storage', () => {
    const broken = {
      getItem: () => {
        throw new Error('denied')
      },
      setItem: () => {
        throw new Error('denied')
      },
    }
    expect(homeSeed(broken)).toBeGreaterThanOrEqual(0)
    expect(homeSeed(undefined)).toBeGreaterThanOrEqual(0)
  })
})
```

En `web/src/lib/router.test.js`, reemplazá:

```js
    ['/', { page: 'redirect', to: '/explorar' }],
    ['/explorar', { page: 'explorar' }],
```

por:

```js
    ['/', { page: 'inicio' }],
    ['/explorar', { page: 'explorar' }],
    ['/buscar', { page: 'buscar' }],
    ['/ajustes', { page: 'ajustes' }],
    ['/bienvenida', { page: 'bienvenida' }],
```

Crear `web/src/lib/search.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { directorHref, enterHref, isShortcut, move, options, searchable, searchHref } from './search.js'

const result = {
  items: [
    { kind: 'movie', tmdbId: 7857, title: 'Amarcord' },
    { kind: 'version', key: 'f1', title: 'Rip mentecato' },
  ],
  directors: [{ id: 4415, name: 'Federico Fellini', count: 12 }],
}

describe('search', () => {
  it('asks from two letters or digits', () => {
    expect(searchable('a')).toBe(false)
    expect(searchable(' a. ')).toBe(false)
    expect(searchable('8½')).toBe(true)
    expect(searchable('ái')).toBe(true)
    expect(searchable(undefined)).toBe(false)
  })
  it('links results', () => {
    expect(searchHref(' la noche ')).toBe('/buscar?q=la%20noche')
    expect(directorHref(4415)).toBe('/explorar?director=4415')
    expect(options(result).map((o) => o.href)).toEqual(['/pelicula/7857', '/version/f1', '/explorar?director=4415'])
    expect(options(null)).toEqual([])
  })
  it('moves the selection with the arrows', () => {
    expect(move(-1, 'ArrowDown', 3)).toBe(0)
    expect(move(2, 'ArrowDown', 3)).toBe(-1)
    expect(move(-1, 'ArrowUp', 3)).toBe(2)
    expect(move(0, 'ArrowUp', 3)).toBe(-1)
    expect(move(1, 'Enter', 3)).toBe(1)
    expect(move(0, 'ArrowDown', 0)).toBe(-1)
  })
  it('goes where Enter says', () => {
    const opts = options(result)
    expect(enterHref(2, opts, 'fell')).toBe('/explorar?director=4415')
    expect(enterHref(-1, opts, 'fell')).toBe('/buscar?q=fell')
    expect(enterHref(-1, opts, 'f')).toBeNull()
  })
  it('knows its shortcut', () => {
    expect(isShortcut({ ctrlKey: true, key: 'k' })).toBe(true)
    expect(isShortcut({ metaKey: true, key: 'K' })).toBe(true)
    expect(isShortcut({ key: 'k' })).toBe(false)
    expect(isShortcut({ ctrlKey: true, shiftKey: true, key: 'k' })).toBe(false)
  })
})
```

Crear `web/src/lib/settings.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { addRoot, body, changed, draft, languageName } from './settings.js'

const config = {
  setupPending: false,
  readOnly: false,
  roots: [{ path: '../cine', available: true }],
  suggested: ['../cine-ordenar'],
  hasToken: true,
  tokenHint: '…a1b2',
  language: 'es-AR',
  languages: ['es-AR', 'en-US', 'pt-BR'],
  imagePrefetch: 'none',
}

describe('settings', () => {
  it('names languages', () => {
    expect(languageName('pt-BR')).toBe('Português (Brasil)')
    expect(languageName('fr-FR')).toBe('fr-FR')
  })
  it('checks the suggested folders on first use only', () => {
    expect(draft(config).roots.map((r) => r.checked)).toEqual([true, false])
    expect(draft({ ...config, setupPending: true }).roots.map((r) => r.checked)).toEqual([true, true])
  })
  it('keeps the token unless changed', () => {
    const d = draft(config)
    expect(body(d)).toEqual({ roots: ['../cine'], token: null, language: 'es-AR', imagePrefetch: 'none' })
    expect(changed(config, d)).toBe(false)
    expect(body({ ...d, token: ' eyJ ' }).token).toBe('eyJ')
    expect(body({ ...d, token: '' }).token).toBe('')
    expect(changed(config, { ...d, token: '' })).toBe(true)
  })
  it('adds folders', () => {
    let d = addRoot(draft(config), { path: '../otras', available: true })
    d = addRoot(d, { path: '../cine-ordenar', available: true })
    expect(body(d).roots).toEqual(['../cine', '../cine-ordenar', '../otras'])
    expect(changed(config, d)).toBe(true)
    expect(changed(config, { ...draft(config), language: 'en-US' })).toBe(true)
  })
})
```

En `web/src/lib/status.test.js`, reemplazá:

```js
import { busy, progress, summary, tmdbProblem } from './status.js'
```

por:

```js
import { busy, progress, summary, tmdbProblem, tokenProblem } from './status.js'
```

En `web/src/lib/status.test.js`, reemplazá:

```js
    expect(progress({ scan: { versions: 40 }, identify: null })).toBe('40|0|0')
  })
})

```

por:

```js
    expect(progress({ scan: { versions: 40 }, identify: null })).toBe('40|0|0')
  })
})

describe('tokenProblem', () => {
  it('is set when Ajustes can fix the token', () => {
    expect(tokenProblem(null)).toBe(false)
    expect(tokenProblem(idle)).toBe(false)
    expect(tokenProblem({ ...idle, identify: { state: 'noToken' } })).toBe(true)
    expect(tokenProblem({ ...idle, identify: { state: 'badToken' } })).toBe(true)
    expect(tokenProblem({ ...idle, identify: { state: 'offline' } })).toBe(false)
    expect(tokenProblem({ ...idle, readOnly: true, identify: { state: 'noToken' } })).toBe(false)
  })
})

```

- [ ] **Step 2: Correr los tests para ver que fallan**

Run: `cd web && npm test`

Expected: FAIL, por ejemplo:

```
Test Files  6 failed | 2 passed (8)
Tests  6 failed | 63 passed (69)
FAIL  src/lib/home.test.js [ src/lib/home.test.js ]
Error: Cannot find module './home.js' imported from web/src/lib/home.test.js
FAIL  src/lib/search.test.js [ src/lib/search.test.js ]
Error: Cannot find module './search.js' imported from web/src/lib/search.test.js
```

- [ ] **Step 3: Implementar**

En `web/src/lib/format.js`, reemplazá:

```js
  return norm(a) === norm(b)
}

```

por:

```js
  return norm(a) === norm(b)
}

// creditLine is the line under a title on the home page: first director,
// up to two countries and the year ("Federico Fellini · Italia, Francia ·
// 1973"); what is missing is left out. Pages show it in capitals.
export function creditLine(movie) {
  const countries = (movie.countries ?? []).slice(0, 2).map(country).join(', ')
  return [movie.directors?.[0], countries, movie.year || ''].filter(Boolean).join(' · ')
}

```

Crear `web/src/lib/home.js`:

```js
// The home page: its rows' titles and the seed that draws them.
import { country } from './format.js'

// rowTitle is the heading of a home row (pages show it in capitals).
export function rowTitle(row) {
  switch (row.kind) {
    case 'recent':
      return 'Agregadas recientemente'
    case 'decade': {
      const y = Number(row.value)
      return y >= 1920 && y <= 1990 ? `Los ${String(y).slice(2)}` : `Los ${y}`
    }
    case 'director':
      return `Dirigidas por ${row.label}`
    case 'country':
      return `Cine de ${country(row.value)}`
  }
  return row.label
}

const KEY = 'cx-home-seed'

// newSeed draws a seed for the rows.
export function newSeed() {
  return Math.floor(Math.random() * 2 ** 32)
}

// homeSeed is the seed of this visit: kept in sessionStorage so going back
// or reloading shows the same rows; drawn anew in a new tab or session.
// storage may be missing or throw (private mode): then each call draws.
export function homeSeed(storage) {
  try {
    const kept = Number(storage?.getItem(KEY))
    if (Number.isInteger(kept) && kept > 0) return kept
  } catch {
    // No storage: a seed for this time.
  }
  return saveSeed(storage, newSeed())
}

// saveSeed keeps a seed for the rest of the visit (↻ draws another).
export function saveSeed(storage, seed) {
  try {
    storage?.setItem(KEY, String(seed))
  } catch {
    // Not kept: the next visit draws again.
  }
  return seed
}
```

En `web/src/lib/router.js`, reemplazá:

```js
  if (pathname === '/') return { page: 'redirect', to: '/explorar' }
  if (pathname === '/explorar') return { page: 'explorar' }
```

por:

```js
  if (pathname === '/') return { page: 'inicio' }
  if (pathname === '/explorar') return { page: 'explorar' }
  if (pathname === '/buscar') return { page: 'buscar' }
  if (pathname === '/ajustes') return { page: 'ajustes' }
  if (pathname === '/bienvenida') return { page: 'bienvenida' }
```

Crear `web/src/lib/search.js`:

```js
// The instant search: when to ask, where results lead, keyboard selection.
import { itemHref } from './router.js'

// MIN is how many letters or digits a query needs (as in the server).
export const MIN = 2

// searchable reports whether q is worth asking for.
export function searchable(q) {
  return (q ?? '').replace(/[^\p{L}\p{N}]/gu, '').length >= MIN
}

export function searchHref(q) {
  return `/buscar?q=${encodeURIComponent(q.trim())}`
}

export function directorHref(id) {
  return `/explorar?director=${id}`
}

// options are the entries of the dropdown, in keyboard order: the items,
// then the directors.
export function options(result) {
  if (!result) return []
  return [
    ...result.items.map((item) => ({ kind: 'item', item, href: itemHref(item) })),
    ...result.directors.map((d) => ({ kind: 'director', director: d, href: directorHref(d.id) })),
  ]
}

// move returns the selected option after an arrow key: -1 is none (the
// text field), and the selection wraps around.
export function move(selected, key, count) {
  if (count === 0) return -1
  if (key === 'ArrowDown') return selected + 1 >= count ? -1 : selected + 1
  if (key === 'ArrowUp') return selected <= -1 ? count - 1 : selected - 1
  return selected
}

// enterHref is where Enter goes: the selected option, or the results page.
export function enterHref(selected, opts, q) {
  if (selected >= 0 && selected < opts.length) return opts[selected].href
  return searchable(q) ? searchHref(q) : null
}

// isShortcut tells the key that opens the search from anywhere: Ctrl+K, or
// ⌘K on a Mac.
export function isShortcut(event) {
  return !!(event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'k'
}
```

Crear `web/src/lib/settings.js`:

```js
// The settings as the pages edit them: a draft made from GET /api/config,
// and the body of PUT /api/config.

const names = {
  'es-AR': 'Español (Argentina)',
  'en-US': 'English (US)',
  'pt-BR': 'Português (Brasil)',
  'es-ES': 'Español (España)',
}

// languageName names a metadata language in its own language.
export function languageName(code) {
  return names[code] ?? code
}

export const prefetchModes = [
  { value: 'none', label: 'Al verlas', hint: 'Cada imagen se baja la primera vez que aparece.' },
  { value: 'posters', label: 'Afiches por adelantado', hint: 'Los afiches se bajan durante la identificación.' },
  {
    value: 'all',
    label: 'Todo por adelantado',
    hint: 'Afiches e imágenes de escena, para usar la app sin red. Son varios cientos de MB.',
  },
]

// draft is what the page edits. On first use the suggested folders come
// checked (the assistant proposes every sibling folder); later they are
// offered unchecked. token is null while the saved one is kept.
export function draft(config) {
  const roots = [
    ...config.roots.map((r) => ({ ...r, checked: true })),
    ...config.suggested.map((path) => ({ path, available: true, checked: config.setupPending })),
  ]
  return { roots, token: null, language: config.language, imagePrefetch: config.imagePrefetch }
}

// addRoot adds a checked folder, or checks it when it is already listed.
export function addRoot(d, root) {
  const known = d.roots.find((r) => r.path === root.path)
  const roots = known
    ? d.roots.map((r) => (r.path === root.path ? { ...r, checked: true } : r))
    : [...d.roots, { ...root, checked: true }]
  return { ...d, roots }
}

// body is the PUT /api/config request for a draft: null keeps the token,
// "" removes it.
export function body(d) {
  return {
    roots: d.roots.filter((r) => r.checked).map((r) => r.path),
    token: d.token === null ? null : d.token.trim(),
    language: d.language,
    imagePrefetch: d.imagePrefetch,
  }
}

// changed reports whether saving the draft would change the configuration.
export function changed(config, d) {
  const b = body(d)
  const saved = config.roots.map((r) => r.path)
  return (
    b.token !== null ||
    b.language !== config.language ||
    b.imagePrefetch !== config.imagePrefetch ||
    b.roots.length !== saved.length ||
    b.roots.some((r, i) => r !== saved[i])
  )
}
```

En `web/src/lib/status.js`, reemplazá:

```js
      return 'Falta el token de TMDB en config.json: no se pueden buscar películas.'
    case 'badToken':
      return 'TMDB rechazó el token de config.json.'
```

por:

```js
      return 'Falta el token de TMDB: cargalo en Ajustes para buscar películas.'
    case 'badToken':
      return 'TMDB rechazó el token: revisalo en Ajustes.'
```

En `web/src/lib/status.js`, reemplazá:

```js
  return { text: 'Al día', tone: '' }
}

```

por:

```js
  return { text: 'Al día', tone: '' }
}

// tokenProblem reports whether the TMDB token is missing or rejected: what
// Ajustes fixes.
export function tokenProblem(st) {
  return !!st && !st.readOnly && (st.identify?.state === 'noToken' || st.identify?.state === 'badToken')
}

```

- [ ] **Step 4: Correr los tests y compilar**

Run: `cd web && npm test && npm run build`

Expected: todos los tests en verde; el build sin advertencias, que actualiza `internal/server/dist/`.

- [ ] **Step 5: Commit**

```bash
git add web internal/server/dist
git commit -m "feat(web): pure logic for home, search, settings and the new routes"
```

---

### Task 13: Página de Inicio

**Files:**
- Create: `web/src/components/FilaInicio.svelte`, `web/src/components/TarjetaEscena.svelte`, `web/src/pages/Inicio.svelte`
- Modify: `web/src/App.svelte`, `web/src/components/Nav.svelte`, `web/src/lib/api.js`

Spec §7.3. `Inicio.svelte` pide `/api/home` con la semilla de la visita y la vuelve a pedir con `app.generation` (las filas se llenan mientras se identifica); ↻ sortea otra. `FilaInicio.svelte`: título, "Ver todas →", scroll horizontal con `scroll-snap` y flechas en pantallas con puntero. `TarjetaEscena.svelte`: imagen de escena 16:9 o, si no hay, el afiche recortado y desenfocado con el título encima; debajo, título en mayúsculas y `creditLine`. Sin películas identificadas: el progreso si algo corre, o enlaces a Ajustes (sin token), Explorar y Revisar. La barra suma "Inicio" y la marca lleva a `/`.

- [ ] **Step 1: Implementar**

En `web/src/App.svelte`, reemplazá:

```svelte
  import Explorar from './pages/Explorar.svelte'
  import Pelicula from './pages/Pelicula.svelte'
```

por:

```svelte
  import Explorar from './pages/Explorar.svelte'
  import Inicio from './pages/Inicio.svelte'
  import Pelicula from './pages/Pelicula.svelte'
```

En `web/src/App.svelte`, reemplazá:

```svelte
  const current = $derived(resolve(route.path))

  $effect(() => {
    if (current.page === 'redirect') navigate(current.to, { replace: true })
  })

```

por:

```svelte
  const current = $derived(resolve(route.path))

```

En `web/src/App.svelte`, reemplazá:

```svelte
  {#if current.page === 'explorar'}
```

por:

```svelte
  {#if current.page === 'inicio'}
    <Inicio />
  {:else if current.page === 'explorar'}
```

Crear `web/src/components/FilaInicio.svelte`:

```svelte
<script>
  import { rowTitle } from '../lib/home.js'
  import TarjetaEscena from './TarjetaEscena.svelte'

  let { row } = $props()
  let track = $state()
  let atStart = $state(true)
  let atEnd = $state(true)

  function update() {
    atStart = track.scrollLeft <= 4
    atEnd = track.scrollLeft + track.clientWidth >= track.scrollWidth - 4
  }

  $effect(() => {
    if (!track) return
    update()
    const ro = new ResizeObserver(update)
    ro.observe(track)
    return () => ro.disconnect()
  })

  function scroll(dir) {
    track.scrollBy({ left: dir * track.clientWidth * 0.9, behavior: 'smooth' })
  }
</script>

<section>
  <header>
    <h2>{rowTitle(row)}</h2>
    <a class="all" href={row.href}>Ver todas →</a>
  </header>
  <div class="wrap">
    <div class="track" bind:this={track} onscroll={update}>
      {#each row.items as movie (movie.tmdbId)}
        <TarjetaEscena {movie} />
      {/each}
    </div>
    {#if !atStart}
      <button class="arrow prev" onclick={() => scroll(-1)} aria-label="Anteriores">‹</button>
    {/if}
    {#if !atEnd}
      <button class="arrow next" onclick={() => scroll(1)} aria-label="Siguientes">›</button>
    {/if}
  </div>
</section>

<style>
  section {
    margin-bottom: 36px;
  }
  header {
    display: flex;
    align-items: baseline;
    gap: 16px;
    margin-bottom: 12px;
  }
  h2 {
    flex: 1;
    min-width: 0;
    margin: 0;
    font-size: 14px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--strong);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .all {
    flex: none;
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .wrap {
    position: relative;
  }
  .track {
    display: flex;
    gap: 14px;
    overflow-x: auto;
    scroll-snap-type: x mandatory;
    scrollbar-width: none;
    padding-bottom: 4px;
  }
  .track::-webkit-scrollbar {
    display: none;
  }
  .arrow {
    position: absolute;
    top: 0;
    height: calc(320px * 9 / 16); /* the card's image */
    width: 44px;
    border: none;
    border-radius: 0;
    font-size: 32px;
    color: var(--strong);
    background: linear-gradient(to right, rgba(20, 23, 28, 0.95), rgba(20, 23, 28, 0));
  }
  .prev {
    left: 0;
  }
  .next {
    right: 0;
    background: linear-gradient(to left, rgba(20, 23, 28, 0.95), rgba(20, 23, 28, 0));
  }
  @media (max-width: 640px) {
    .arrow {
      height: calc(70vw * 9 / 16);
    }
  }
  @media (hover: none) {
    .arrow {
      display: none;
    }
  }
</style>
```

En `web/src/components/Nav.svelte`, reemplazá:

```svelte
    <a class="brand" href="/explorar">Cinexplorer</a>
```

por:

```svelte
    <a class="brand" href="/">Cinexplorer</a>
    <a href="/" class:active={page === 'inicio'}>Inicio</a>
```

Crear `web/src/components/TarjetaEscena.svelte`:

```svelte
<script>
  import { backdropURL, posterURL } from '../lib/api.js'
  import { creditLine } from '../lib/format.js'
  import { movieHref } from '../lib/router.js'

  let { movie } = $props()
  let failed = $state(false)

  // The still when there is one; if not (or it does not load), the poster,
  // cropped and blurred, with the title over it.
  const still = $derived(!failed && movie.backdrop ? backdropURL(movie.tmdbId, movie.backdrop) : '')
  const poster = $derived(movie.poster ? posterURL(movie.tmdbId, movie.poster) : '')
</script>

<a class="card" href={movieHref(movie.tmdbId)}>
  <div class="frame">
    {#if still}
      <img src={still} alt="" loading="lazy" onerror={() => (failed = true)} />
    {:else}
      {#if poster}<img class="blur" src={poster} alt="" loading="lazy" />{/if}
      <span class="over">{movie.title}</span>
    {/if}
  </div>
  <div class="title">{movie.title}</div>
  <div class="credit">{creditLine(movie)}</div>
</a>

<style>
  .card {
    display: block;
    flex: 0 0 320px;
    min-width: 0;
    scroll-snap-align: start;
  }
  .frame {
    position: relative;
    aspect-ratio: 16 / 9;
    overflow: hidden;
    border-radius: 3px;
    background: linear-gradient(160deg, #262c35, #15181d);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
  }
  .card:hover .frame {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .blur {
    filter: blur(14px) brightness(0.6);
    transform: scale(1.2);
  }
  .over {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 12px;
    text-align: center;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--strong);
    overflow-wrap: anywhere;
  }
  .title {
    margin-top: 8px;
    font-size: 13px;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--strong);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .credit {
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  @media (max-width: 640px) {
    .card {
      flex-basis: 70vw;
    }
  }
</style>
```

En `web/src/lib/api.js`, reemplazá:

```js
  status: () => get('/api/status'),
  explore: (search) => get(`/api/explore${search}`),
```

por:

```js
  status: () => get('/api/status'),
  home: (seed) => get(`/api/home?seed=${seed}`),
  explore: (search) => get(`/api/explore${search}`),
```

Crear `web/src/pages/Inicio.svelte`:

```svelte
<script>
  import FilaInicio from '../components/FilaInicio.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { homeSeed, newSeed, saveSeed } from '../lib/home.js'
  import { busy, summary, tokenProblem } from '../lib/status.js'

  // sessionStorage may be unavailable (private mode, blocked site data).
  function storage() {
    try {
      return window.sessionStorage
    } catch {
      return undefined
    }
  }

  let seed = $state(homeSeed(storage()))
  let data = $state(null)
  let loads = 0

  // Load when the seed changes, and again as scans and identification runs
  // bring movies (the same seed keeps the same rows while they fill).
  $effect(() => {
    const s = seed
    app.generation
    load(s)
  })

  async function load(s) {
    const id = ++loads
    let d
    try {
      d = await api.home(s)
    } catch (e) {
      if (id === loads) notify(e.message)
      return
    }
    if (id === loads) data = d
  }

  function reshuffle() {
    seed = saveSeed(storage(), newSeed())
  }

  const st = $derived(app.status)
</script>

<svelte:head><title>Cinexplorer</title></svelte:head>

{#if data}
  {#if data.rows.length}
    <div class="bar">
      <span class="label">{data.total} películas</span>
      <button class="shuffle" onclick={reshuffle} title="Otras filas" aria-label="Otras filas">↻</button>
    </div>
    {#each data.rows as row (row.kind + ':' + row.value)}
      <FilaInicio {row} />
    {/each}
  {:else if busy(st)}
    <p class="empty">{summary(st).text}… Las películas van a aparecer a medida que se identifiquen.</p>
  {:else if tokenProblem(st)}
    <p class="empty">
      Sin un token de TMDB no se identifican películas. <a href="/ajustes">Cargalo en Ajustes</a> o mirá lo que hay en
      <a href="/explorar">Explorar</a>.
    </p>
  {:else}
    <p class="empty">
      Todavía no hay películas identificadas. Mirá lo que hay en <a href="/explorar">Explorar</a> o en
      <a href="/revisar">Revisar</a>.
    </p>
  {/if}
{/if}

<style>
  .bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 8px 0 20px;
  }
  .shuffle {
    border: none;
    font-size: 18px;
    padding: 2px 8px;
    color: var(--muted);
  }
  .shuffle:hover {
    color: var(--strong);
  }
  .empty a {
    color: var(--accent);
  }
</style>
```

- [ ] **Step 2: Correr los tests y compilar**

Run: `cd web && npm test && npm run build`

Expected: todos los tests en verde; el build sin advertencias (Svelte avisa de problemas de accesibilidad y de reactividad: no tiene que haber ninguno), que actualiza `internal/server/dist/`.

- [ ] **Step 3: Revisar en el navegador**

Run: `go run ./cmd/cinexplorer -dir .run` (o `npm run dev` con la API en el puerto 8080; ver README).

Con `go run ./cmd/cinexplorer -dir .run` sobre un catálogo con películas identificadas: el Inicio muestra las filas con sus títulos ("AGREGADAS RECIENTEMENTE", "LOS 70"…), las flechas desplazan la fila, "Ver todas →" abre Explorar con la faceta, ↻ cambia las filas y recargar la página conserva las mismas. Una película sin imagen de escena muestra el afiche desenfocado. En 375 px de ancho las tarjetas ocupan ~70 % y no hay scroll horizontal de la página.

- [ ] **Step 4: Commit**

```bash
git add web internal/server/dist
git commit -m "feat(web): home page with MUBI-style rows"
```

---

### Task 14: Búsqueda instantánea y página de resultados

**Files:**
- Create: `web/src/components/Busqueda.svelte`, `web/src/pages/Buscar.svelte`
- Modify: `web/src/App.svelte`, `web/src/components/Nav.svelte`, `web/src/lib/api.js`

Spec §6.3. `Busqueda.svelte`, en la barra: la lupa (o Ctrl+K / ⌘K) abre el campo; con 150 ms sin teclear pide `/api/search` y descarta respuestas viejas. El desplegable muestra hasta 8 ítems (miniatura, título, año · director, "sin identificar"), los directores y "Ver los N resultados →"; ↑/↓ mueven la selección, Enter abre la elegida o `/buscar`, Esc cierra. Cerrar al hacer clic afuera usa `composedPath()`: el botón que abre la búsqueda ya no está en la página cuando el clic llega a `window`. En pantallas chicas el campo y el desplegable ocupan el ancho.

`Buscar.svelte`: campo grande, cantidad, directores como chips y la grilla de afiches (hasta 200). La consulta vive en la URL (`replaceState` al escribir); si la URL cambia desde afuera (el desplegable), el campo se actualiza. En pantallas chicas se oculta "Inicio" de la barra y se achican espacios y letra: entra en 375 px.

- [ ] **Step 1: Implementar**

En `web/src/App.svelte`, reemplazá:

```svelte
  import { appLink, resolve } from './lib/router.js'
  import Explorar from './pages/Explorar.svelte'
```

por:

```svelte
  import { appLink, resolve } from './lib/router.js'
  import Buscar from './pages/Buscar.svelte'
  import Explorar from './pages/Explorar.svelte'
```

En `web/src/App.svelte`, reemplazá:

```svelte
    <Explorar />
  {:else if current.page === 'pelicula'}
```

por:

```svelte
    <Explorar />
  {:else if current.page === 'buscar'}
    <Buscar />
  {:else if current.page === 'pelicula'}
```

Crear `web/src/components/Busqueda.svelte`:

```svelte
<script>
  import { tick } from 'svelte'
  import { api, posterURL } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { navigate, route } from '../lib/nav.svelte.js'
  import { enterHref, isShortcut, move, options, searchable, searchHref } from '../lib/search.js'

  let open = $state(false)
  let q = $state('')
  let result = $state(null)
  let selected = $state(-1)
  let input = $state()
  let box = $state()
  let timer
  let asks = 0

  const opts = $derived(options(result))
  const itemCount = $derived(result?.items.length ?? 0)

  async function show() {
    open = true
    await tick()
    input?.focus()
    input?.select()
  }

  function close() {
    open = false
    selected = -1
  }

  // Ask after a pause in the typing; answers to older queries are dropped.
  function oninput() {
    clearTimeout(timer)
    selected = -1
    if (!searchable(q)) {
      asks++
      result = null
      return
    }
    timer = setTimeout(ask, 150)
  }

  async function ask() {
    const id = ++asks
    try {
      const r = await api.find(q)
      if (id === asks) result = r
    } catch (e) {
      if (id === asks) notify(e.message)
    }
  }

  function go(href) {
    if (!href) return
    close()
    navigate(href)
  }

  function onkeydown(event) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      selected = move(selected, event.key, opts.length)
    } else if (event.key === 'Enter') {
      event.preventDefault()
      go(enterHref(selected, opts, q))
    } else if (event.key === 'Escape') {
      close()
    }
  }

  function onwindowkeydown(event) {
    if (isShortcut(event)) {
      event.preventDefault()
      show()
    }
  }

  // The path is fixed when the click starts: the button that opened the
  // search is gone from the page by the time the click reaches the window.
  function onwindowclick(event) {
    if (open && box && !event.composedPath().includes(box)) close()
  }

  // Following a result (or any link) closes the search.
  $effect(() => {
    route.path
    route.search
    close()
  })
</script>

<svelte:window onkeydown={onwindowkeydown} onclick={onwindowclick} />

<div class="busqueda" bind:this={box}>
  {#if open}
    <input
      bind:this={input}
      bind:value={q}
      {oninput}
      {onkeydown}
      type="search"
      placeholder="Título, director, actor…"
      aria-label="Buscar en el catálogo"
      autocomplete="off"
      spellcheck="false"
    />
    {#if result && searchable(q)}
      <div class="panel" role="listbox">
        {#if result.total === 0}
          <p class="none">Nada con «{q}».</p>
        {/if}
        {#each result.items as item, i (item.kind === 'movie' ? `m${item.tmdbId}` : `v${item.key}`)}
          <a class="option" class:selected={selected === i} href={opts[i].href} onclick={close} role="option" aria-selected={selected === i}>
            {#if item.kind === 'movie' && item.poster}
              <img src={posterURL(item.tmdbId, item.poster)} alt="" />
            {:else}
              <span class="thumb" class:unidentified={item.kind !== 'movie'}></span>
            {/if}
            <span class="text">
              <span class="title">{item.title}</span>
              <span class="meta">
                {[item.year || '', item.directors?.[0] ?? '', item.kind === 'movie' ? '' : 'sin identificar']
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </span>
          </a>
        {/each}
        {#if result.directors.length}
          <div class="label group">Directores</div>
          {#each result.directors as d, j (d.id)}
            <a
              class="option director"
              class:selected={selected === itemCount + j}
              href={opts[itemCount + j].href}
              onclick={close}
              role="option"
              aria-selected={selected === itemCount + j}
            >
              <span class="title">{d.name}</span>
              <span class="meta">{d.count} {d.count === 1 ? 'película' : 'películas'}</span>
            </a>
          {/each}
        {/if}
        {#if result.total > 0}
          <a class="all" href={searchHref(q)} onclick={close}>
            Ver {result.total === 1 ? 'el resultado' : `los ${result.total} resultados`} →
          </a>
        {/if}
      </div>
    {/if}
  {:else}
    <button class="icon" onclick={show} title="Buscar (Ctrl+K)" aria-label="Buscar">
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
        <circle cx="10.5" cy="10.5" r="6.5" fill="none" stroke="currentColor" stroke-width="2" />
        <path d="M15.5 15.5 21 21" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
      </svg>
    </button>
  {/if}
</div>

<style>
  .busqueda {
    position: relative;
    text-transform: none;
    letter-spacing: 0;
  }
  .icon {
    display: flex;
    border: none;
    padding: 4px;
    color: var(--muted);
  }
  .icon:hover {
    color: var(--strong);
  }
  input {
    width: 260px;
  }
  .panel {
    position: absolute;
    right: 0;
    top: calc(100% + 8px);
    width: 360px;
    max-height: calc(100vh - 80px);
    overflow-y: auto;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.6);
    padding: 6px 0;
    z-index: 40;
    color: var(--text);
  }
  .option {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 6px 12px;
  }
  .option.selected,
  .option:hover {
    background: var(--line);
  }
  img,
  .thumb {
    flex: none;
    width: 32px;
    height: 48px;
    object-fit: cover;
    border-radius: 2px;
    background: linear-gradient(160deg, #262c35, #15181d);
  }
  .thumb.unidentified {
    background: repeating-linear-gradient(45deg, #1b1f25, #1b1f25 4px, #262c35 4px, #262c35 8px);
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .title {
    color: var(--strong);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .meta {
    font-size: 12px;
    color: var(--faint);
  }
  .director {
    justify-content: space-between;
  }
  .group {
    padding: 10px 12px 4px;
  }
  .none {
    margin: 0;
    padding: 8px 12px;
    color: var(--muted);
  }
  .all {
    display: block;
    padding: 8px 12px 4px;
    font-size: 13px;
    color: var(--accent);
    border-top: 1px solid var(--line);
    margin-top: 6px;
  }
  @media (max-width: 640px) {
    .busqueda:has(input) {
      position: static;
    }
    input {
      position: absolute;
      left: var(--gutter);
      right: var(--gutter);
      top: 8px;
      width: auto;
      z-index: 41;
    }
    .panel {
      left: var(--gutter);
      right: var(--gutter);
      width: auto;
      top: 52px;
    }
  }
</style>
```

En `web/src/components/Nav.svelte`, reemplazá:

```svelte
<script>
  import Estado from './Estado.svelte'
```

por:

```svelte
<script>
  import Busqueda from './Busqueda.svelte'
  import Estado from './Estado.svelte'
```

En `web/src/components/Nav.svelte`, reemplazá:

```svelte
    <a href="/" class:active={page === 'inicio'}>Inicio</a>
    <a href="/explorar" class:active={page === 'explorar'}>Explorar</a>
    <a href="/revisar" class:active={page === 'revisar'}>Revisar</a>
    <span class="spacer"></span>
```

por:

```svelte
    <a class="home" href="/" class:active={page === 'inicio'}>Inicio</a>
    <a href="/explorar" class:active={page === 'explorar'}>Explorar</a>
    <a href="/revisar" class:active={page === 'revisar'}>Revisar</a>
    <span class="spacer"></span>
    <Busqueda />
```

En `web/src/components/Nav.svelte`, reemplazá:

```svelte
      gap: 14px;
      letter-spacing: 0.04em;
```

por:

```svelte
      gap: 12px;
      letter-spacing: 0.04em;
    }
    .home {
      display: none; /* the brand leads home */
```

En `web/src/lib/api.js`, reemplazá:

```js
  home: (seed) => get(`/api/home?seed=${seed}`),
  explore: (search) => get(`/api/explore${search}`),
```

por:

```js
  home: (seed) => get(`/api/home?seed=${seed}`),
  find: (q, limit = 8) => get(`/api/search?${new URLSearchParams({ q, limit })}`),
  explore: (search) => get(`/api/explore${search}`),
```

Crear `web/src/pages/Buscar.svelte`:

```svelte
<script>
  import Afiche from '../components/Afiche.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { navigate, route } from '../lib/nav.svelte.js'
  import { directorHref, searchable, searchHref } from '../lib/search.js'

  const LIMIT = 200

  let q = $state('')
  let result = $state(null)
  let written = null // the last query this page put in the URL
  let timer
  let asks = 0

  // The query lives in the URL: typing rewrites it (no new history entry),
  // and it is asked again when a scan or an identification run ends. A URL
  // changed from elsewhere (the search in the bar) replaces the text.
  $effect(() => {
    const text = new URLSearchParams(route.search).get('q') ?? ''
    app.generation
    if (text !== written) {
      q = text
      written = text
    }
    ask(text)
  })

  async function ask(text) {
    const id = ++asks
    if (!searchable(text)) {
      result = null
      return
    }
    try {
      const r = await api.find(text, LIMIT)
      if (id === asks) result = r
    } catch (e) {
      if (id === asks) notify(e.message)
    }
  }

  function oninput() {
    clearTimeout(timer)
    timer = setTimeout(() => {
      written = q.trim()
      navigate(searchHref(q), { replace: true })
    }, 150)
  }
</script>

<svelte:head><title>{q ? `${q} · ` : ''}Buscar · Cinexplorer</title></svelte:head>

<div class="head">
  <input
    type="search"
    bind:value={q}
    {oninput}
    placeholder="Título, director, actor…"
    aria-label="Buscar en el catálogo"
    autocomplete="off"
    spellcheck="false"
  />
  {#if result}
    <span class="count">
      {result.total === 1 ? '1 resultado' : `${result.total} resultados`}{result.total > LIMIT
        ? ` · se muestran los primeros ${LIMIT}`
        : ''}
    </span>
  {/if}
</div>

{#if result?.directors.length}
  <div class="directors">
    <span class="label">Directores</span>
    {#each result.directors as d (d.id)}
      <a class="chip" href={directorHref(d.id)}>{d.name} <small>{d.count}</small></a>
    {/each}
  </div>
{/if}

{#if result?.items.length}
  <div class="grid">
    {#each result.items as item (item.kind === 'movie' ? `m${item.tmdbId}` : `v${item.key}`)}
      <Afiche {item} />
    {/each}
  </div>
{:else if result}
  <p class="empty">Nada con «{q}». La búsqueda mira títulos, títulos originales, directores, reparto y nombres de archivo.</p>
{:else}
  <p class="empty">Escribí al menos dos letras.</p>
{/if}

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
    margin: 8px 0 16px;
  }
  input {
    flex: 1 1 320px;
    max-width: 560px;
    font-size: 18px;
    padding: 8px 12px;
  }
  .count {
    color: var(--muted);
    font-size: 13px;
  }
  .directors {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
  }
  .chip {
    border: 1px solid var(--line-strong);
    border-radius: 999px;
    padding: 2px 10px;
    font-size: 13px;
  }
  .chip small {
    color: var(--faint);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 20px 14px;
  }
  @media (max-width: 640px) {
    .grid {
      grid-template-columns: repeat(auto-fill, minmax(104px, 1fr));
      gap: 14px 10px;
    }
  }
</style>
```

- [ ] **Step 2: Correr los tests y compilar**

Run: `cd web && npm test && npm run build`

Expected: todos los tests en verde; el build sin advertencias (Svelte avisa de problemas de accesibilidad y de reactividad: no tiene que haber ninguno), que actualiza `internal/server/dist/`.

- [ ] **Step 3: Revisar en el navegador**

Run: `go run ./cmd/cinexplorer -dir .run` (o `npm run dev` con la API en el puerto 8080; ver README).

"almodovar" (sin tilde) encuentra las de Almodóvar y lo muestra en Directores; un actor encuentra sus películas; un título sin identificar aparece con "sin identificar"; ↓ y Enter abren la elegida; Enter sin elegir abre `/buscar?q=…`; Esc cierra. En 375 px la lupa abre el campo a todo el ancho y la página no tiene scroll horizontal.

- [ ] **Step 4: Commit**

```bash
git add web internal/server/dist
git commit -m "feat(web): instant search in the bar and the results page"
```

---

### Task 15: Asistente de primer uso y Ajustes

**Files:**
- Create: `web/src/components/Idioma.svelte`, `web/src/components/Raices.svelte`, `web/src/components/Token.svelte`, `web/src/pages/Ajustes.svelte`, `web/src/pages/Bienvenida.svelte`
- Modify: `web/src/App.svelte`, `web/src/components/Estado.svelte`, `web/src/components/Identificar.svelte`, `web/src/components/Nav.svelte`, `web/src/lib/api.js`, `web/src/lib/app.svelte.js`, `web/src/pages/Explorar.svelte`

Spec §8. `App.svelte` lleva a `/bienvenida` cualquier ruta mientras el estado diga `setupPending`, sin barra. `Bienvenida.svelte`: tres pasos (carpetas, token, idioma) y **Empezar**, que guarda con `PUT /api/config`, apaga `setupPending` en el estado local (`settingsSaved`: el sondeo normal tarda hasta 30 s) y va al Inicio. `Ajustes.svelte`: las mismas secciones más la descarga de imágenes, **Guardar** (solo con cambios) y **Descartar**; aviso al cambiar el idioma; en modo consulta, todo deshabilitado.

Componentes compartidos: `Raices.svelte` (casillas y "Agregar" con `POST /api/config/root`), `Token.svelte` (conservar, cambiar o quitar; **Verificar** con `POST /api/config/token`) e `Idioma.svelte`. La barra suma el engranaje (ícono "settings" de Feather Icons, MIT), el panel de estado un enlace a Ajustes, `Identificar` "Ir a Ajustes" cuando falta el token o fue rechazado, y Explorar vacío nombra Ajustes.

- [ ] **Step 1: Implementar**

En `web/src/App.svelte`, reemplazá:

```svelte
  import { appLink, resolve } from './lib/router.js'
  import Buscar from './pages/Buscar.svelte'
```

por:

```svelte
  import { appLink, resolve } from './lib/router.js'
  import Ajustes from './pages/Ajustes.svelte'
  import Bienvenida from './pages/Bienvenida.svelte'
  import Buscar from './pages/Buscar.svelte'
```

En `web/src/App.svelte`, reemplazá:

```svelte
  const current = $derived(resolve(route.path))

```

por:

```svelte
  const current = $derived(resolve(route.path))

  // Until the first-use settings are saved, every page is the assistant.
  $effect(() => {
    if (app.status?.setupPending && current.page !== 'bienvenida') navigate('/bienvenida', { replace: true })
  })

```

En `web/src/App.svelte`, reemplazá:

```svelte
<Nav page={current.page} />
```

por:

```svelte
{#if current.page !== 'bienvenida'}
  <Nav page={current.page} />
{/if}
```

En `web/src/App.svelte`, reemplazá:

```svelte
    <Buscar />
  {:else if current.page === 'pelicula'}
```

por:

```svelte
    <Buscar />
  {:else if current.page === 'ajustes'}
    <Ajustes />
  {:else if current.page === 'bienvenida'}
    <Bienvenida />
  {:else if current.page === 'pelicula'}
```

En `web/src/components/Estado.svelte`, reemplazá:

```svelte
        {/if}
      </div>
```

por:

```svelte
        {/if}
        <a class="to-settings" href="/ajustes" onclick={() => (open = false)}>Ajustes →</a>
      </div>
```

En `web/src/components/Estado.svelte`, reemplazá:

```svelte
  }
  .panel {
```

por:

```svelte
  }
  .to-settings {
    margin-left: 12px;
    font-size: 13px;
    color: var(--accent);
  }
  .panel {
```

En `web/src/components/Identificar.svelte`, reemplazá:

```svelte
  import { tmdbProblem } from '../lib/status.js'
```

por:

```svelte
  import { tmdbProblem, tokenProblem } from '../lib/status.js'
```

En `web/src/components/Identificar.svelte`, reemplazá:

```svelte
    <p class="problem">{problem}</p>
```

por:

```svelte
    <p class="problem">{problem} {#if tokenProblem(app.status)}<a href="/ajustes">Ir a Ajustes</a>{/if}</p>
```

En `web/src/components/Identificar.svelte`, reemplazá:

```svelte
  }
  .none {
```

por:

```svelte
  }
  .problem a {
    color: var(--accent);
  }
  .none {
```

Crear `web/src/components/Idioma.svelte`:

```svelte
<script>
  import { languageName } from '../lib/settings.js'

  let { language = $bindable(), languages, disabled = false } = $props()
</script>

<div class="options" role="radiogroup" aria-label="Idioma de los datos">
  {#each languages as code (code)}
    <label class:on={language === code}>
      <input type="radio" name="idioma" value={code} bind:group={language} {disabled} />
      {languageName(code)}
    </label>
  {/each}
</div>

<style>
  .options {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  label {
    display: flex;
    align-items: center;
    gap: 6px;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 6px 12px;
    cursor: pointer;
  }
  label.on {
    border-color: var(--accent);
    color: var(--strong);
  }
</style>
```

En `web/src/components/Nav.svelte`, reemplazá:

```svelte
    <Estado />
  </nav>
```

por:

```svelte
    <Estado />
    <a class="settings" href="/ajustes" class:active={page === 'ajustes'} title="Ajustes" aria-label="Ajustes">
      <!-- Feather Icons "settings" (MIT) -->
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2"
        stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="3" />
        <path
          d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
        />
      </svg>
    </a>
  </nav>
```

En `web/src/components/Nav.svelte`, reemplazá:

```svelte
  @media (max-width: 640px) {
    nav {
      gap: 12px;
      letter-spacing: 0.04em;
```

por:

```svelte
  .settings {
    display: flex;
  }
  @media (max-width: 640px) {
    nav {
      gap: 8px;
      font-size: 12px;
      letter-spacing: 0.02em;
    }
    .brand {
      letter-spacing: 0.08em;
```

Crear `web/src/components/Raices.svelte`:

```svelte
<script>
  import { api } from '../lib/api.js'
  import { addRoot } from '../lib/settings.js'

  // d is the settings draft; the folders are d.roots.
  let { d = $bindable(), disabled = false } = $props()
  let path = $state('')
  let error = $state('')
  let checking = $state(false)

  // Folders read as they sit next to the app: "../cine" is "cine".
  function label(p) {
    return p.replace(/^(\.\.\/)+/, '')
  }

  async function add(event) {
    event.preventDefault()
    if (!path.trim()) return
    checking = true
    error = ''
    try {
      d = addRoot(d, await api.checkRoot(path))
      path = ''
    } catch (e) {
      error = e.message
    }
    checking = false
  }
</script>

<ul>
  {#each d.roots as root (root.path)}
    <li>
      <label>
        <input type="checkbox" bind:checked={root.checked} {disabled} />
        <span class="name">{label(root.path)}</span>
        {#if !root.available}<small>no disponible</small>{/if}
      </label>
      <span class="path">{root.path}</span>
    </li>
  {:else}
    <li class="none">No hay carpetas al lado de la de Cinexplorer: agregá una.</li>
  {/each}
</ul>
<form onsubmit={add}>
  <input type="text" bind:value={path} placeholder="Otra carpeta: D:\peliculas o ../videos" {disabled} aria-label="Otra carpeta" />
  <button type="submit" disabled={disabled || checking || !path.trim()}>Agregar</button>
</form>
{#if error}<p class="error">{error}</p>{/if}

<style>
  ul {
    list-style: none;
    margin: 0 0 12px;
    padding: 0;
  }
  li {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
  }
  label {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .name {
    color: var(--strong);
  }
  small {
    color: var(--warn);
  }
  .none {
    color: var(--muted);
  }
  form {
    display: flex;
    gap: 8px;
  }
  form input {
    flex: 1;
    min-width: 0;
  }
  .error {
    color: var(--warn);
    margin: 6px 0 0;
  }
</style>
```

Crear `web/src/components/Token.svelte`:

```svelte
<script>
  import { api } from '../lib/api.js'

  // token is the draft's: null keeps the saved one, "" removes it, text
  // replaces it. config tells whether one is saved.
  let { token = $bindable(null), config, disabled = false } = $props()
  let mode = $state(config.hasToken ? 'keep' : 'edit') // keep | edit | remove
  let text = $state('')
  let check = $state(null) // null, 'checking', true, false, 'unknown'

  $effect(() => {
    if (mode === 'edit') token = text.trim() ? text : null
    else token = mode === 'remove' ? '' : null
  })

  async function verify() {
    check = 'checking'
    try {
      const r = await api.checkToken(text)
      check = r.valid === null ? 'unknown' : r.valid
    } catch {
      check = 'unknown'
    }
  }
</script>

<p class="help">
  Cinexplorer identifica las películas con <a href="https://www.themoviedb.org/" target="_blank" rel="noreferrer">TMDB</a>.
  El token es gratuito: creá una cuenta, entrá a
  <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noreferrer">Settings → API</a> y copiá el
  <strong>API Read Access Token</strong> (el largo, que empieza con <code>eyJ</code>; la "API Key" corta no sirve).
</p>

{#if mode === 'keep'}
  <p class="saved">
    Token cargado <span class="path">{config.tokenHint}</span>
    <button onclick={() => (mode = 'edit')} {disabled}>Cambiar</button>
    <button onclick={() => (mode = 'remove')} {disabled}>Quitar</button>
  </p>
{:else if mode === 'remove'}
  <p class="saved">
    Se va a quitar el token: sin él no se identifican películas.
    <button onclick={() => (mode = 'keep')} {disabled}>Deshacer</button>
  </p>
{:else}
  <div class="row">
    <input
      type="text"
      bind:value={text}
      oninput={() => (check = null)}
      placeholder="eyJhbGciOiJIUzI1NiJ9…"
      aria-label="Token de TMDB"
      autocomplete="off"
      spellcheck="false"
      {disabled}
    />
    <button onclick={verify} disabled={disabled || !text.trim() || check === 'checking'}>Verificar</button>
    {#if config.hasToken}<button onclick={() => ((mode = 'keep'), (text = ''))} {disabled}>Cancelar</button>{/if}
  </div>
  {#if check === 'checking'}
    <p class="note">Verificando…</p>
  {:else if check === true}
    <p class="note ok">✓ TMDB aceptó el token.</p>
  {:else if check === false}
    <p class="note bad">✕ TMDB rechazó el token. Revisá que sea el API Read Access Token.</p>
  {:else if check === 'unknown'}
    <p class="note">No se pudo verificar (¿sin conexión?). Se guarda igual.</p>
  {/if}
{/if}

<style>
  .help {
    color: var(--muted);
    margin-top: 0;
  }
  .help a {
    color: var(--accent);
  }
  code {
    font-size: 13px;
  }
  .saved {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 10px;
  }
  .row {
    display: flex;
    gap: 8px;
  }
  .row input {
    flex: 1;
    min-width: 0;
    font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
    font-size: 13px;
  }
  .note {
    margin: 6px 0 0;
    color: var(--muted);
  }
  .ok {
    color: var(--best);
  }
  .bad {
    color: var(--warn);
  }
</style>
```

En `web/src/lib/api.js`, reemplazá:

```js
const post = (path, body) => call('POST', path, body ?? {})

```

por:

```js
const post = (path, body) => call('POST', path, body ?? {})
const put = (path, body) => call('PUT', path, body)

```

En `web/src/lib/api.js`, reemplazá:

```js
  scan: () => post('/api/scan'),
}
```

por:

```js
  scan: () => post('/api/scan'),
  config: () => get('/api/config'),
  saveConfig: (body) => put('/api/config', body),
  checkRoot: (path) => post('/api/config/root', { path }),
  checkToken: (token) => post('/api/config/token', { token }),
}
```

En `web/src/lib/app.svelte.js`, reemplazá:

```js

// refreshStatus asks for the status now, after an action that may have
```

por:

```js

// settingsSaved updates what the pages know right after saving the
// settings (the first-use assistant is done) and asks for the new status.
export function settingsSaved() {
  if (app.status) app.status = { ...app.status, setupPending: false }
  poll()
}

// refreshStatus asks for the status now, after an action that may have
```

Crear `web/src/pages/Ajustes.svelte`:

```svelte
<script>
  import Idioma from '../components/Idioma.svelte'
  import Raices from '../components/Raices.svelte'
  import Token from '../components/Token.svelte'
  import { api } from '../lib/api.js'
  import { notify, settingsSaved } from '../lib/app.svelte.js'
  import { body, changed, draft, prefetchModes } from '../lib/settings.js'

  let config = $state(null)
  let d = $state(null)
  let saving = $state(false)
  let version = $state(0) // remounts the token field after saving

  $effect(() => {
    api.config().then(
      (c) => {
        config = c
        d = draft(c)
      },
      (e) => notify(e.message),
    )
  })

  const dirty = $derived(!!config && !!d && changed(config, d))
  const languageChanged = $derived(!!config && !!d && d.language !== config.language)

  async function save() {
    saving = true
    try {
      config = await api.saveConfig(body(d))
      d = draft(config)
      version++
      settingsSaved()
      notify('Ajustes guardados. Se vuelve a escanear.')
    } catch (e) {
      notify(e.message)
    }
    saving = false
  }
</script>

<svelte:head><title>Ajustes · Cinexplorer</title></svelte:head>

{#if d}
  <div class="ajustes">
    <h1 class="label">Ajustes</h1>
    {#if config.readOnly}
      <p class="warn">Modo consulta: la carpeta de la app no se puede escribir, así que los ajustes no se pueden cambiar.</p>
    {/if}

    <section>
      <h2>Carpetas con películas</h2>
      <p class="help">
        Quitar una carpeta la saca del catálogo (las correcciones se conservan). Una carpeta que no está disponible,
        como un disco desenchufado, se mantiene tal cual.
      </p>
      <Raices bind:d disabled={config.readOnly} />
    </section>

    <section>
      <h2>Token de TMDB</h2>
      {#key version}
        <Token bind:token={d.token} {config} disabled={config.readOnly} />
      {/key}
    </section>

    <section>
      <h2>Idioma de los datos</h2>
      <Idioma bind:language={d.language} languages={config.languages} disabled={config.readOnly} />
      {#if languageChanged}
        <p class="warn">
          Al cambiar el idioma se vuelven a pedir a TMDB los datos de toda la colección, y se revisan las identificaciones
          automáticas. Con muchas películas puede tardar un buen rato.
        </p>
      {/if}
    </section>

    <section>
      <h2>Imágenes</h2>
      <div class="modes">
        {#each prefetchModes as m (m.value)}
          <label>
            <input type="radio" name="prefetch" value={m.value} bind:group={d.imagePrefetch} disabled={config.readOnly} />
            <span><strong>{m.label}</strong><br /><small>{m.hint}</small></span>
          </label>
        {/each}
      </div>
    </section>

    {#if !config.readOnly}
      <div class="actions">
        <button class="primary" onclick={save} disabled={!dirty || saving || !d.roots.some((r) => r.checked)}>
          {saving ? 'Guardando…' : 'Guardar'}
        </button>
        {#if dirty}<button onclick={() => ((d = draft(config)), version++)} disabled={saving}>Descartar</button>{/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .ajustes {
    max-width: 720px;
  }
  section {
    padding: 20px 0;
    border-bottom: 1px solid var(--line);
  }
  h2 {
    font-size: 16px;
    color: var(--strong);
    margin: 0 0 8px;
  }
  .help {
    color: var(--muted);
    margin-top: 0;
  }
  .warn {
    color: var(--warn);
  }
  .modes {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .modes label {
    display: flex;
    gap: 10px;
    align-items: baseline;
  }
  small {
    color: var(--faint);
  }
  .actions {
    display: flex;
    gap: 8px;
    margin-top: 24px;
  }
</style>
```

Crear `web/src/pages/Bienvenida.svelte`:

```svelte
<script>
  import Idioma from '../components/Idioma.svelte'
  import Raices from '../components/Raices.svelte'
  import Token from '../components/Token.svelte'
  import { api } from '../lib/api.js'
  import { notify, settingsSaved } from '../lib/app.svelte.js'
  import { navigate } from '../lib/nav.svelte.js'
  import { body, draft } from '../lib/settings.js'

  const steps = ['Carpetas', 'Token de TMDB', 'Idioma']

  let config = $state(null)
  let d = $state(null)
  let step = $state(0)
  let saving = $state(false)

  $effect(() => {
    api.config().then(
      (c) => {
        if (!c.setupPending) return navigate('/', { replace: true })
        config = c
        d = draft(c)
      },
      (e) => notify(e.message),
    )
  })

  const rootsChosen = $derived(!!d && d.roots.some((r) => r.checked))

  async function start() {
    saving = true
    try {
      await api.saveConfig(body(d))
      settingsSaved()
      navigate('/', { replace: true })
    } catch (e) {
      notify(e.message)
      saving = false
    }
  }
</script>

<svelte:head><title>Bienvenida · Cinexplorer</title></svelte:head>

{#if d}
  <div class="bienvenida">
    <h1>Cinexplorer</h1>
    <ol class="steps">
      {#each steps as name, i (name)}
        <li class:on={i === step} class:done={i < step}>{i + 1}. {name}</li>
      {/each}
    </ol>

    {#if step === 0}
      <h2>¿Dónde están tus películas?</h2>
      <p class="help">
        Estas son las carpetas que están al lado de la de Cinexplorer. Cinexplorer solo las lee: nunca mueve, renombra
        ni borra archivos.
      </p>
      <Raices bind:d />
    {:else if step === 1}
      <h2>Token de TMDB</h2>
      <Token bind:token={d.token} {config} />
    {:else}
      <h2>Idioma de los datos</h2>
      <p class="help">Títulos, sinopsis y géneros se piden en este idioma. Se puede cambiar después en Ajustes.</p>
      <Idioma bind:language={d.language} languages={config.languages} />
    {/if}

    <div class="actions">
      {#if step > 0}<button onclick={() => step--} disabled={saving}>Atrás</button>{/if}
      <span class="spacer"></span>
      {#if step === 1 && d.token === null}
        <button onclick={() => step++}>Seguir sin token</button>
      {:else if step < steps.length - 1}
        <button class="primary" onclick={() => step++} disabled={!rootsChosen}>Siguiente</button>
      {:else}
        <button class="primary" onclick={start} disabled={saving || !rootsChosen}>
          {saving ? 'Guardando…' : 'Empezar'}
        </button>
      {/if}
    </div>
  </div>
{/if}

<style>
  .bienvenida {
    max-width: 640px;
    margin: 32px auto 0;
  }
  h1 {
    margin: 0 0 20px;
    font-size: 20px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--strong);
  }
  .steps {
    display: flex;
    gap: 20px;
    list-style: none;
    padding: 0 0 12px;
    margin: 0 0 24px;
    border-bottom: 1px solid var(--line);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--faint);
  }
  .steps .on {
    color: var(--strong);
  }
  .steps .done {
    color: var(--muted);
  }
  h2 {
    font-size: 18px;
    color: var(--strong);
    margin: 0 0 8px;
  }
  .help {
    color: var(--muted);
  }
  .actions {
    display: flex;
    gap: 8px;
    margin-top: 28px;
  }
  .spacer {
    flex: 1;
  }
  @media (max-width: 640px) {
    .steps {
      gap: 12px;
      flex-wrap: wrap;
    }
  }
</style>
```

En `web/src/pages/Explorar.svelte`, reemplazá:

```svelte
    <p class="empty">El catálogo está vacío. Revisá las raíces en config.json y volvé a escanear.</p>
```

por:

```svelte
    <p class="empty">El catálogo está vacío. Revisá las carpetas en <a href="/ajustes">Ajustes</a>.</p>
```

- [ ] **Step 2: Correr los tests y compilar**

Run: `cd web && npm test && npm run build`

Expected: todos los tests en verde; el build sin advertencias (Svelte avisa de problemas de accesibilidad y de reactividad: no tiene que haber ninguno), que actualiza `internal/server/dist/`.

- [ ] **Step 3: Revisar en el navegador**

Run: `go run ./cmd/cinexplorer -dir .run` (o `npm run dev` con la API en el puerto 8080; ver README).

Con una carpeta de app nueva (sin `config.json`): cualquier ruta lleva al asistente, que propone las carpetas hermanas; una ruta inexistente en "Agregar" muestra el error; Verificar con un token falso dice "rechazó"; sin red, "no se pudo verificar"; **Empezar** crea `config.json`, lleva al Inicio y arranca el escaneo. En Ajustes: quitar una carpeta y guardar la saca de Explorar y la ofrece de nuevo sin marcar; cambiar el idioma muestra el aviso; con un token guardado se ve "…" y sus últimos 4 caracteres.

- [ ] **Step 4: Commit**

```bash
git add web internal/server/dist
git commit -m "feat(web): first-use assistant and settings page"
```

---

### Task 16: README, verificación final y prueba real

**Files:**
- Modify: `README.md`

El README describe el Inicio, la búsqueda, el asistente y Ajustes, el idioma `es-AR` con su cadena de respaldo y los endpoints nuevos.

- [ ] **Step 1: README**

En `README.md`, reemplazá:

```markdown
| 4b. Descubrimiento | Inicio, búsqueda instantánea, asistente de primer uso | pendiente |
| 5. Curaduría | listas, etiquetas, importación de `Collections/` | pendiente |

La interfaz tiene tres partes:

```

por:

```markdown
| 4b. Descubrimiento | Inicio, búsqueda instantánea, asistente de primer uso, Ajustes | ✅ |
| 5. Curaduría | listas, etiquetas, importación de `Collections/` | pendiente |

La interfaz tiene estas partes:

- **Inicio:** filas de imágenes de escena al estilo de MUBI: lo agregado
  recientemente y, sorteadas en cada visita, una década, un director, un país,
  un género y una colección. Cada fila lleva a Explorar con ese filtro. El
  botón ↻ sortea otras.
- **Búsqueda** (la lupa, o Ctrl+K / ⌘K): resultados mientras escribís, por
  título, título original, director, reparto o nombre de archivo, sin importar
  tildes ni mayúsculas. Enter abre la página con todos los resultados.
```

En `README.md`, reemplazá:

```markdown
  espacio que se podría recuperar).

```

por:

```markdown
  espacio que se podría recuperar).

- **Ajustes** (el engranaje): carpetas, token de TMDB, idioma de los datos y
  descarga de imágenes. Los cambios se aplican sin reiniciar la app.

```

En `README.md`, reemplazá:

```markdown
3. Se abre el navegador con la app. En el primer uso se crea `config.json` con
   las carpetas hermanas como raíces (ignora las ocultas y las del sistema, como
   `$RECYCLE.BIN` o `System Volume Information`).
4. Cerrá la app, agregá tu token de TMDB en `config.json` y volvé a abrirla. El
   primer escaneo analiza todo; los siguientes solo lo que cambió.
```

por:

```markdown
3. Se abre el navegador con el asistente de primer uso, en tres pasos:
   - **Carpetas:** propone las que están al lado de `cinexplorer/` (ignora las
     ocultas y las del sistema, como `$RECYCLE.BIN` o `System Volume
     Information`); se pueden desmarcar o agregar otras escribiendo la ruta.
   - **Token de TMDB:** pegalo y verificalo (ver [Requisitos](#requisitos)).
     Se puede seguir sin token y cargarlo después en Ajustes.
   - **Idioma** de títulos y sinopsis: español (Argentina), inglés (EE. UU.) o
     portugués (Brasil).
4. Al terminar se guarda `config.json` y empieza el escaneo; el Inicio se va
   llenando a medida que se identifican las películas. El primer escaneo
   analiza todo; los siguientes solo lo que cambió.
```

En `README.md`, reemplazá:

```markdown
`config.json`, junto al ejecutable:
```

por:

```markdown
Todo se cambia desde **Ajustes**, en la app. Queda guardado en `config.json`,
junto al ejecutable:
```

En `README.md`, reemplazá:

```markdown
  "language": "es-ES",
```

por:

```markdown
  "language": "es-AR",
```

En `README.md`, reemplazá:

```markdown
| `roots` | Carpetas a escanear, **relativas a la carpeta de la app** y con `/` como separador. Una raíz que no está disponible (por ejemplo, un disco desenchufado) se saltea sin tocar lo que ya había en el catálogo. |
| `tmdbToken` | Token de lectura de TMDB (v4). Vacío: no se identifica. |
| `language` | Idioma de títulos, sinopsis y géneros, en formato TMDB (`es-ES`, `es-MX`, `en-US`, `pt-BR`…). Si lo cambiás, los datos se vuelven a pedir en el nuevo idioma. |
```

por:

```markdown
| `roots` | Carpetas a escanear, **relativas a la carpeta de la app** y con `/` como separador (tienen que estar en el mismo disco que la app). Una raíz que no está disponible (por ejemplo, un disco desenchufado) se saltea sin tocar lo que ya había en el catálogo. Los archivos de una raíz que se quita pasan a "no encontrado". |
| `tmdbToken` | Token de lectura de TMDB (v4). Vacío: no se identifica. |
| `language` | Idioma de títulos, sinopsis y géneros: `es-AR` (por defecto), `en-US` o `pt-BR`. Lo que TMDB no tiene traducido a esa variante se toma de la más cercana (para `es-AR`: `es-MX`, después `es-ES`) y, la sinopsis, en último caso en inglés. Si lo cambiás, los datos se vuelven a pedir en el nuevo idioma. |
```

En `README.md`, reemplazá:

```markdown
| `-no-browser` | No abrir el navegador al arrancar. |
```

por:

```markdown
| `-no-browser` | No abrir el navegador al arrancar. Sin `config.json`, la app espera que se complete el asistente en la dirección que muestra. |
```

En `README.md`, reemplazá:

```markdown
  token.
```

por:

```markdown
  token. Verificar el token en el asistente o en Ajustes pide una película
  conocida (*Fight Club*).
```

En `README.md`, reemplazá:

```markdown
| `GET /api/status` | Estado del escaneo y de la identificación. |
```

por:

```markdown
| `GET /api/status` | Estado del escaneo y de la identificación, y si falta el primer uso (`setupPending`). |
| `GET /api/home?seed=` | Filas del Inicio (la misma semilla da las mismas filas). |
| `GET /api/search?q=&limit=` | Búsqueda en el catálogo: ítems y directores que coinciden. |
| `GET /api/config`, `PUT /api/config` | Ajustes (el token nunca se devuelve). En `PUT`, `token` ausente o `null` conserva el guardado y `""` lo quita. |
| `POST /api/config/root`, `POST /api/config/token` | Valida una carpeta, o verifica un token con TMDB. |
```

En `README.md`, reemplazá:

```markdown
internal/catalog      ítems, facetas, fichas y duplicados de la interfaz
```

por:

```markdown
internal/engine       lo que depende de config.json (TMDB, escaneo, identificación), reemplazable en caliente
internal/catalog      ítems, facetas, fichas, duplicados y filas del Inicio
internal/search       índice de búsqueda (FTS5 en memoria)
```

En `README.md`, reemplazá:

```markdown
- **"Falta tmdbToken en config.json" / "El token de TMDB no es válido":**
  revisá que sea el *API Read Access Token* (v4), no la API Key corta.
```

por:

```markdown
- **"Falta el token de TMDB" / "Token de TMDB inválido":** cargalo o
  cambialo en Ajustes; tiene que ser el *API Read Access Token* (v4), no la
  API Key corta.
```

En `README.md`, reemplazá:

```markdown
- **Archivos que no aparecen:** revisá `roots` en `config.json`; las rutas son
  relativas a la carpeta de la app.
```

por:

```markdown
- **Archivos que no aparecen:** revisá las carpetas en Ajustes.
```

- [ ] **Step 2: Verificación final**

Run:

```bash
gofmt -l .
go vet ./...
go test ./...
cd web && npm ci && npm test && npm run build && cd ..
git status --porcelain internal/server/dist
```

Expected: `gofmt` no lista nada; todos los paquetes `ok`; Vitest en verde; el build sin advertencias y sin cambios en `internal/server/dist` (el commiteado está al día).

- [ ] **Step 3: Prueba real (manual, fuera de CI)**

Con el binario (`bash scripts/build.sh`) en una carpeta nueva, sin `config.json`, al lado de una parte de `D:\cine`, y un token real:

1. El asistente propone las carpetas hermanas; **Verificar** con el token real dice "✓ TMDB aceptó el token"; con uno inventado, "rechazó". Elegir `Español (Argentina)` y **Empezar**.
2. El Inicio se va llenando mientras se identifica; al terminar, las filas tienen imágenes de escena; ↻ cambia las filas; "Ver todas →" abre Explorar con la faceta.
3. **Traducciones (spec §5):** abrir la ficha de películas no argentinas (una italiana, una rusa, una estadounidense) y comprobar título y sinopsis en español. Si alguna quedó en inglés o con el título original teniendo traducción en `es-MX`/`es-ES`, anotar el id: la cadena no está funcionando como se espera. Anotar también si `GET https://api.themoviedb.org/3/movie/{id}/translations` responde con la forma de `tmdb.Translation`.
4. Búsqueda: "almodovar", "bunuel" (sin tildes), un actor, un título sin identificar; Enter y la página de resultados.
5. Ajustes: quitar una carpeta y guardar (desaparece de Explorar y queda ofrecida sin marcar); volver a agregarla; cambiar a `en-US` y volver a `es-AR` (el aviso aparece; los títulos cambian al terminar la corrida); quitar y volver a cargar el token.
6. Todo lo anterior también en 375 px de ancho.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: README for home, search, first use and settings"
```
