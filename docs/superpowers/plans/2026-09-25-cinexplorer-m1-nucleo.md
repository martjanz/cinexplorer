# Cinexplorer — Etapa 1: Núcleo local — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Un ejecutable portable que escanea las raíces configuradas (sin tocar los archivos), arma el catálogo de versiones a partir de los nombres, detecta copias idénticas y lo muestra en una página web local mínima con "▶ Ver" y "Carpeta".

**Architecture:** Binario Go con servidor HTTP en `127.0.0.1` y frontend estático embebido. Todo el estado vive en `cinexplorer.db` (SQLite en Go puro) junto al ejecutable; las rutas se guardan relativas al directorio de la app con `/`. Pipeline: escaneo incremental → huella parcial → agrupación en versiones → parseo de nombres → persistencia.

**Tech Stack:** Go ≥ 1.24, `modernc.org/sqlite`, `golang.org/x/sys/windows`, HTML/JS sin framework (el frontend Svelte llega en la Etapa 4).

Spec: `docs/superpowers/specs/2026-09-25-cinexplorer-design.md`

---

## Hoja de ruta (etapas)

Cada etapa produce software usable y tiene su propio plan, que se escribe al terminar la anterior.

| Etapa | Contenido | Spec |
|---|---|---|
| **1 — Núcleo local (este plan)** | appdir/config portables, clasificación, huella, parser de nombres, versiones (partes, VIDEO_TS, subs, extras), SQLite, escaneo incremental, "no encontrado", copias idénticas, API + página mínima, abrir/mostrar en carpeta, build multiplataforma, CI | §2, §3.1–3.3, §4 pasos 1–3, §4.2, §6 |
| 2 — Datos técnicos | lectores nativos de headers MKV/MP4/AVI, fallback `ffprobe`, criterio de "mejor versión" | §4 paso 4, §5.3 |
| 3 — Identificación | cliente TMDB con rate limit, Wikidata, puntaje de confianza, tabla de películas, identificaciones por huella, IMDb desde `.nfo`, caché de imágenes, reintentos sin red | §3.1, §4 pasos 5–6 |
| 4 — Interfaz | Svelte + Vite embebido, Inter, Inicio (filas), Explorar (grilla + facetas en URL), Ficha con tarjetas de versiones, búsqueda FTS5, asistente de primer uso | §5.1–5.3, §5.6–5.7 |
| 5 — Curaduría | Revisar (sin identificar / duplicados / colecciones), listas, etiquetas, importación de `Collections/` | §4.1, §5.4–5.5 |

---

## Estructura de archivos (Etapa 1)

```
go.mod / go.sum
cmd/cinexplorer/main.go            arranque: appdir, config, store, scanner, servidor, navegador
internal/appdir/appdir.go          directorio de la app, rutas relativas ↔ absolutas, escribible
internal/config/config.go          config.json: carga, defaults (carpetas hermanas), guardado
internal/mediafile/classify.go     tipo de archivo por nombre/extensión
internal/fingerprint/fingerprint.go huella parcial (1 MiB inicial + 1 MiB final + tamaño)
internal/nameparse/parse.go        nombre → título, año, director, países, datos técnicos
internal/nameparse/merge.go        combinar parseo de archivo y de carpeta
internal/grouping/grouping.go      archivos → versiones (partes, VIDEO_TS, subs, extras)
internal/store/schema.sql          esquema SQLite
internal/store/store.go            apertura, archivos, versiones, duplicados
internal/scan/scan.go              escaneo incremental con estado de progreso
internal/platform/platform.go      helper común
internal/platform/open_windows.go  abrir / mostrar en Explorer
internal/platform/open_unix.go     abrir / mostrar en Finder / xdg-open
internal/server/server.go          API JSON + estáticos embebidos, solo localhost
internal/server/web/index.html     página mínima
scripts/build.sh                   compilación cruzada → dist/cinexplorer/
.github/workflows/ci.yml           tests en Windows, macOS y Linux
README.md
```

Dependencias entre paquetes: `scan → {store, grouping, fingerprint, mediafile, appdir}`, `store → grouping → {nameparse, mediafile}`, `server → {store, scan, appdir}`, `main → todo`. Sin ciclos.

---

### Task 0: Toolchain y módulo

**Files:**
- Create: `go.mod`
- Modify: `.gitignore`

- [ ] **Step 1: Instalar Go** (requiere confirmación del usuario: instala software y acepta la licencia)

Run (PowerShell): `winget install --id GoLang.Go -e`
Luego, en Git Bash, si `go` todavía no está en el PATH de la sesión: `export PATH="$PATH:/c/Program Files/Go/bin"`

- [ ] **Step 2: Verificar**

Run: `go version`
Expected: `go version go1.2x.y windows/amd64` (≥ 1.24)

- [ ] **Step 3: Inicializar el módulo**

Run: `cd /d/x-projects/cinexplorer && go mod init cinexplorer`
Expected: `go: creating new go.mod: module cinexplorer`

- [ ] **Step 4: Ampliar `.gitignore`**

Contenido completo de `.gitignore`:

```
.superpowers/
dist/
.run/
*.db
```

- [ ] **Step 5: Commit**

```bash
git add go.mod .gitignore
git commit -m "chore: init Go module"
```

---

### Task 1: appdir — rutas portables

**Files:**
- Create: `internal/appdir/appdir.go`
- Test: `internal/appdir/appdir_test.go`

- [ ] **Step 1: Test que falla**

```go
package appdir

import (
	"path/filepath"
	"testing"
)

func TestRelAbsRoundTrip(t *testing.T) {
	app := filepath.Join(t.TempDir(), "cinexplorer")
	movie := filepath.Join(filepath.Dir(app), "cine", "1970s", "Amarcord.mkv")

	rel, err := Rel(app, movie)
	if err != nil {
		t.Fatal(err)
	}
	if rel != "../cine/1970s/Amarcord.mkv" {
		t.Fatalf("Rel = %q", rel)
	}
	if got := Abs(app, rel); got != movie {
		t.Fatalf("Abs = %q, want %q", got, movie)
	}
}

func TestWritable(t *testing.T) {
	if !Writable(t.TempDir()) {
		t.Fatal("temp dir should be writable")
	}
	if Writable(filepath.Join(t.TempDir(), "does-not-exist")) {
		t.Fatal("missing dir must not be writable")
	}
}

func TestResolveOverride(t *testing.T) {
	dir := t.TempDir()
	got, err := Resolve(dir)
	if err != nil || got != dir {
		t.Fatalf("Resolve(%q) = %q, %v", dir, got, err)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/appdir/`
Expected: FAIL — `undefined: Rel`

- [ ] **Step 3: Implementación**

```go
// Package appdir resolves the application directory and converts between
// absolute paths and the portable, app-relative form stored in the catalog.
package appdir

import (
	"os"
	"path/filepath"
)

// Resolve returns override (made absolute) if set, otherwise the directory
// holding the running executable.
func Resolve(override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// Rel converts an absolute path into catalog form: relative to appDir and
// '/'-separated, so the same catalog works on every OS and mount point.
func Rel(appDir, abs string) (string, error) {
	r, err := filepath.Rel(appDir, abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(r), nil
}

// Abs converts a catalog path back into an absolute OS path.
func Abs(appDir, rel string) string {
	return filepath.Join(appDir, filepath.FromSlash(rel))
}

// Writable reports whether files can be created in dir.
func Writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".cx-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	return os.Remove(name) == nil
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/appdir/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/appdir
git commit -m "feat(appdir): portable app-relative paths"
```

---

### Task 2: config — config.json con defaults

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Test que falla**

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
	if !created || !reflect.DeepEqual(cfg.Roots, want) || cfg.Language != "es-ES" {
		t.Fatalf("got created=%v cfg=%+v", created, cfg)
	}
}

func TestSaveThenLoad(t *testing.T) {
	dir := t.TempDir()
	in := Config{Roots: []string{"../x"}, TMDBToken: "tok", Language: "es-ES"}
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
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/config/`
Expected: FAIL — `undefined: Load`

- [ ] **Step 3: Implementación**

```go
// Package config reads and writes config.json next to the executable.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const FileName = "config.json"

type Config struct {
	Roots     []string `json:"roots"`     // catalog-form paths, relative to the app dir
	TMDBToken string   `json:"tmdbToken"` // used from stage 3 on
	Language  string   `json:"language"`
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
		return Config{Roots: roots, Language: "es-ES"}, true, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("%s: %w", FileName, err)
	}
	if cfg.Language == "" {
		cfg.Language = "es-ES"
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
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/config/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): config.json with sibling-dir defaults"
```

---

### Task 3: mediafile — clasificación

**Files:**
- Create: `internal/mediafile/classify.go`
- Test: `internal/mediafile/classify_test.go`

- [ ] **Step 1: Test que falla**

```go
package mediafile

import "testing"

func TestClassify(t *testing.T) {
	cases := map[string]Kind{
		"movie.MKV":                        Video,
		"Brasileirinho.avi":                Video,
		"VTS_01_1.VOB":                     DVD,
		"VIDEO_TS.IFO":                     DVD,
		"movie.spa.srt":                    Subtitle,
		"movie.idx":                        Subtitle,
		"seventhseal-KARiNA.nfo":           Info,
		"Thumbs.db":                        Junk,
		".DS_Store":                        Junk,
		"sync.ffs_db":                      Junk,
		"._movie.mkv":                      Junk,
		"Uploaded @ thepiratebay.org.txt":  Junk,
		"cover.jpg":                        Other,
	}
	for name, want := range cases {
		if got := Classify(name); got != want {
			t.Errorf("Classify(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestStoredAndFingerprinted(t *testing.T) {
	for _, k := range []Kind{Video, DVD, Subtitle, Info} {
		if !k.Stored() {
			t.Errorf("%s should be stored", k)
		}
	}
	for _, k := range []Kind{Junk, Other} {
		if k.Stored() {
			t.Errorf("%s should not be stored", k)
		}
	}
	if !Video.Fingerprinted() || !DVD.Fingerprinted() || Subtitle.Fingerprinted() {
		t.Error("only video and DVD files are fingerprinted")
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/mediafile/`
Expected: FAIL — `undefined: Classify`

- [ ] **Step 3: Implementación**

```go
// Package mediafile classifies files found in the library by name.
package mediafile

import (
	"path/filepath"
	"strings"
)

type Kind string

const (
	Video    Kind = "video"
	DVD      Kind = "dvd" // VOB/IFO/BUP, normally inside VIDEO_TS
	Subtitle Kind = "subtitle"
	Info     Kind = "info" // .nfo; may carry an IMDb id (used from stage 3)
	Junk     Kind = "junk"
	Other    Kind = "other"
)

var (
	videoExt  = set(".mkv", ".mp4", ".m4v", ".avi", ".mov", ".wmv", ".mpg", ".mpeg", ".ts", ".m2ts", ".webm", ".ogm", ".rmvb", ".rm", ".divx", ".flv", ".3gp")
	dvdExt    = set(".vob", ".ifo", ".bup")
	subExt    = set(".srt", ".sub", ".idx", ".ass", ".ssa", ".vtt", ".smi")
	junkExt   = set(".txt", ".url", ".sfv", ".md5", ".db", ".ini", ".torrent", ".html", ".htm", ".lnk", ".log", ".par2")
	junkNames = set("thumbs.db", "desktop.ini", ".ds_store", "sync.ffs_db")
)

func Classify(name string) Kind {
	lower := strings.ToLower(name)
	if junkNames[lower] || strings.HasPrefix(lower, "._") {
		return Junk
	}
	ext := filepath.Ext(lower)
	switch {
	case videoExt[ext]:
		return Video
	case dvdExt[ext]:
		return DVD
	case subExt[ext]:
		return Subtitle
	case ext == ".nfo":
		return Info
	case junkExt[ext]:
		return Junk
	}
	return Other
}

// Stored reports whether files of this kind are kept in the catalog.
func (k Kind) Stored() bool {
	return k == Video || k == DVD || k == Subtitle || k == Info
}

// Fingerprinted reports whether the scanner computes a content fingerprint.
func (k Kind) Fingerprinted() bool {
	return k == Video || k == DVD
}

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/mediafile/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mediafile
git commit -m "feat(mediafile): classify library files by name"
```

---

### Task 4: fingerprint — huella parcial

**Files:**
- Create: `internal/fingerprint/fingerprint.go`
- Test: `internal/fingerprint/fingerprint_test.go`

- [ ] **Step 1: Test que falla**

```go
package fingerprint

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fp-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(data)
	f.Close()
	return f.Name()
}

func must(t *testing.T, path string) string {
	t.Helper()
	fp, err := Of(path)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

func TestIdenticalContentSameFingerprint(t *testing.T) {
	data := bytes.Repeat([]byte("cine"), 1000)
	if must(t, write(t, data)) != must(t, write(t, data)) {
		t.Fatal("identical files must match")
	}
}

func TestDifferentContentDifferentFingerprint(t *testing.T) {
	a := bytes.Repeat([]byte{'a'}, 5000)
	b := bytes.Repeat([]byte{'a'}, 5000)
	b[4999] = 'b'
	if must(t, write(t, a)) == must(t, write(t, b)) {
		t.Fatal("different files must not match")
	}
}

func TestLargeFileOnlyEdgesAndSizeCount(t *testing.T) {
	a := bytes.Repeat([]byte{'x'}, 3*chunk)
	b := bytes.Repeat([]byte{'x'}, 3*chunk)
	b[chunk+10] = 'y' // middle byte: deliberately not covered
	if must(t, write(t, a)) != must(t, write(t, b)) {
		t.Fatal("middle bytes are not part of the fingerprint")
	}
	c := append(bytes.Repeat([]byte{'x'}, 3*chunk), 'x') // one byte longer
	if must(t, write(t, a)) == must(t, write(t, c)) {
		t.Fatal("size is part of the fingerprint")
	}
}

func TestMissingFile(t *testing.T) {
	if _, err := Of(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/fingerprint/`
Expected: FAIL — `undefined: Of`

- [ ] **Step 3: Implementación**

```go
// Package fingerprint computes a cheap content fingerprint for large media files.
package fingerprint

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
)

const chunk = 1 << 20

// Of hashes the first and last MiB plus the file size. Identical files always
// match, and a moved or renamed file keeps its fingerprint.
func Of(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := st.Size()
	h := sha256.New()
	if size <= 2*chunk {
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
	} else {
		if _, err := io.CopyN(h, f, chunk); err != nil {
			return "", err
		}
		if _, err := f.Seek(-chunk, io.SeekEnd); err != nil {
			return "", err
		}
		if _, err := io.CopyN(h, f, chunk); err != nil {
			return "", err
		}
	}
	var sz [8]byte
	binary.LittleEndian.PutUint64(sz[:], uint64(size))
	h.Write(sz[:])
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/fingerprint/ -v`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/fingerprint
git commit -m "feat(fingerprint): partial content fingerprint"
```

---

### Task 5: nameparse — parser de nombres

El corpus usa nombres reales de la colección. Cada vez que un nombre real se parsee mal, se agrega aquí antes de corregir.

**Files:**
- Create: `internal/nameparse/parse.go`
- Test: `internal/nameparse/parse_test.go`

- [ ] **Step 1: Test que falla**

```go
package nameparse

import (
	"reflect"
	"testing"
)

func TestParseCorpus(t *testing.T) {
	cases := []struct {
		in   string
		want Parsed
	}{
		{"Amarcord [Federico Fellini, 1973]", Parsed{Title: "Amarcord", Year: 1973, Director: "Federico Fellini"}},
		{"Chinatown (Polanski, USA, 1974)", Parsed{Title: "Chinatown", Year: 1974, Director: "Polanski", Countries: []string{"USA"}}},
		{"Annie Hall [1977, USA]", Parsed{Title: "Annie Hall", Year: 1977, Countries: []string{"USA"}}},
		{"1900 (Novecento) (1976) [mkvonly]", Parsed{Title: "1900 (Novecento)", Year: 1976}},
		{"Arabian.Nights.1974.CC.1080p.BluRay.FLAC1.0.x264-ADE", Parsed{Title: "Arabian Nights", Year: 1974, Resolution: "1080p", Source: "BluRay", Codec: "H.264", Group: "ADE"}},
		{"Auf.der.anderen.Seite.German.AC3.DVDRiP.XviD-EMPiRE", Parsed{Title: "Auf der anderen Seite", Source: "DVDRip", Codec: "XviD", Language: "de", Group: "EMPiRE"}},
		{"El.Castillo.Ambulante.Spanish.XviD.AC3.DVDRip.By.FreAk.TEAm", Parsed{Title: "El Castillo Ambulante", Source: "DVDRip", Codec: "XviD", Language: "es"}},
		{"El viaje de Chihiro DVDRIP SPANISH DIVX (www.lamejorfrikiweb.cjb.net)", Parsed{Title: "El viaje de Chihiro", Source: "DVDRip", Codec: "DivX", Language: "es"}},
		{"El.extraño.caso.del.hombre.y.la.bestia.(1951).1080p.emule.via.clan-sudamerica.net", Parsed{Title: "El extraño caso del hombre y la bestia", Year: 1951, Resolution: "1080p"}},
		{"El señor Galíndez (Rodolfo Kuhn, 1983) - YouTube.(Found.via.clan-sudamerica.net)", Parsed{Title: "El señor Galíndez", Year: 1983, Director: "Rodolfo Kuhn"}},
		{"Antes de la Lluvia-Before the Rain (Milko Manchevski -1994- Uk-Fr-Macedonia)", Parsed{Title: "Antes de la Lluvia-Before the Rain", Year: 1994, Director: "Milko Manchevski", Countries: []string{"Uk-Fr-Macedonia"}}},
		{"Benning, James - The United States of America (2022)", Parsed{Title: "The United States of America", Year: 2022, Director: "James Benning"}},
		{"1976 - 9 Lives of a Wet Pussy", Parsed{Title: "9 Lives of a Wet Pussy", Year: 1976}},
		{"BuSan_Tsai Ming-lian", Parsed{Title: "BuSan", Director: "Tsai Ming-lian"}},
		{"Double Play James Benning And Richard Linklater (2013) [BluRay] [1080p] [YTS.AM]", Parsed{Title: "Double Play James Benning And Richard Linklater", Year: 2013, Resolution: "1080p", Source: "BluRay"}},
		{"1990", Parsed{Title: "1990"}},
		{"Eight and a Half (1963) 720p.BRRip.x264.AC3-WAF", Parsed{Title: "Eight and a Half", Year: 1963, Resolution: "720p", Source: "BluRay", Codec: "H.264"}},
		{"The.Movie.tt0111161.720p", Parsed{Title: "The Movie", Resolution: "720p", IMDbID: "tt0111161"}},
		{"2001.A.Space.Odyssey.1968.1080p.BluRay", Parsed{Title: "2001 A Space Odyssey", Year: 1968, Resolution: "1080p", Source: "BluRay"}},
		{"Cortázar. Instrucciones de montaje (II)", Parsed{Title: "Cortázar. Instrucciones de montaje (II)"}},
		{"AquelMartes-Monteaun", Parsed{Title: "AquelMartes-Monteaun"}},
	}
	for _, c := range cases {
		if got := Parse(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Parse(%q)\n got  %+v\n want %+v", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/nameparse/`
Expected: FAIL — `undefined: Parse`

- [ ] **Step 3: Implementación**

```go
// Package nameparse extracts title, year, director and release details from
// the free-form file and folder names found in a personal movie library.
package nameparse

import (
	"regexp"
	"strconv"
	"strings"
)

type Parsed struct {
	Title      string
	Year       int
	Director   string
	Countries  []string
	Resolution string
	Source     string
	Codec      string
	Language   string
	Group      string
	IMDbID     string
}

var (
	noiseRe        = regexp.MustCompile(`(?i)\(?(?:found\.via\.|emule\.via\.)?clan-sudamerica\.net\)?|\(?www\.[^\s()\[\]]+\)?|\s-\s*youtube\b|\[(?:yts|rarbg|eztv)[^\]]*\]|\[mkvonly\]`)
	imdbRe         = regexp.MustCompile(`\btt\d{7,8}\b`)
	yearRe         = regexp.MustCompile(`(?:^|[^0-9])((?:18|19|20)\d{2})(?:[^0-9p]|$)`)
	bracketRe      = regexp.MustCompile(`[(\[]([^()\[\]]*)[)\]]`)
	resRe          = regexp.MustCompile(`(?i)\b(?:2160p|1080p|1080i|720p|576p|480p|4k)\b`)
	sourceRe       = regexp.MustCompile(`(?i)\b(?:blu-?ray|brrip|bdrip|web-?dl|webrip|dvdrip|dvdscr|hdtv|hdrip|vhsrip|tvrip)\b`)
	codecRe        = regexp.MustCompile(`(?i)\b(?:x26[45]|h[ .]?26[45]|hevc|xvid|divx|av1)\b`)
	langRe         = regexp.MustCompile(`(?i)\b(?:spanish|castellano|latino|english|german|french|italian|portuguese)\b`)
	releaseGroupRe = regexp.MustCompile(`-([A-Za-z0-9]+)$`)
	yearPrefixRe   = regexp.MustCompile(`^\s*((?:18|19|20)\d{2})\s+-\s+(.+)$`)
	lastFirstRe    = regexp.MustCompile(`^\s*([^,\-]+),\s*([^,\-]+?)\s+-\s+(.+)$`)

	sceneSeparators = strings.NewReplacer(".", " ", "_", " ")
)

var countryNames = map[string]bool{
	"usa": true, "uk": true, "eeuu": true, "urss": true, "ussr": true,
	"argentina": true, "brasil": true, "brazil": true, "chile": true, "uruguay": true, "méxico": true, "mexico": true,
	"italia": true, "italy": true, "france": true, "francia": true, "germany": true, "alemania": true,
	"españa": true, "spain": true, "portugal": true, "polonia": true, "poland": true, "suecia": true, "sweden": true,
	"dinamarca": true, "denmark": true, "hungría": true, "hungary": true, "rusia": true, "russia": true,
	"japan": true, "japón": true, "china": true, "india": true, "iran": true, "irán": true, "korea": true, "corea": true,
	"canada": true, "canadá": true,
}

// Parse extracts what it can from a single name (file stem or folder name).
func Parse(name string) Parsed {
	var p Parsed
	s := noiseRe.ReplaceAllString(name, " ")
	if id := imdbRe.FindString(s); id != "" {
		p.IMDbID = id
		s = imdbRe.ReplaceAllString(s, " ")
	}
	p.Resolution = normRes(resRe.FindString(s))
	p.Source = normSource(sourceRe.FindString(s))
	p.Codec = normCodec(codecRe.FindString(s))
	p.Language = normLang(langRe.FindString(s))
	if t := strings.TrimSpace(s); !strings.Contains(t, " ") && hasTech(t) {
		if m := releaseGroupRe.FindStringSubmatch(t); m != nil {
			p.Group = m[1]
		}
	}

	// Scene-style names use dots or underscores as word separators.
	if n := strings.Count(s, ".") + strings.Count(s, "_"); n >= 3 || (n > 0 && !strings.Contains(strings.TrimSpace(s), " ")) {
		s = sceneSeparators.Replace(s)
	}

	if loc, inner := lastYearGroup(s); loc != nil {
		p.Year, p.Director, p.Countries = parseGroup(inner)
		if before := s[:loc[0]]; strings.TrimSpace(before) != "" {
			s = before
		} else {
			s = s[loc[1]:]
		}
		s = s[:techIndex(s, false)]
	} else {
		cut := techIndex(s, false)
		if y, off := lastYearBefore(s, cut); y != 0 {
			p.Year = y
			s = s[:off]
		} else {
			s = s[:techIndex(s, true)]
		}
	}

	s = applyTitlePatterns(s, &p)
	p.Title = tidy(s)
	if p.Title == "" {
		p.Title = tidy(sceneSeparators.Replace(name))
	}
	return p
}

func hasTech(s string) bool {
	return resRe.MatchString(s) || sourceRe.MatchString(s) || codecRe.MatchString(s)
}

// techIndex returns the offset of the first release token (resolution, source,
// codec and, optionally, language) or len(s) if there is none.
func techIndex(s string, withLang bool) int {
	cut := len(s)
	res := []*regexp.Regexp{resRe, sourceRe, codecRe}
	if withLang {
		res = append(res, langRe)
	}
	for _, re := range res {
		if loc := re.FindStringIndex(s); loc != nil && loc[0] < cut {
			cut = loc[0]
		}
	}
	return cut
}

// yearAt returns the first plausible year in s and its offset, or 0, -1.
func yearAt(s string) (int, int) {
	for _, m := range yearRe.FindAllStringSubmatchIndex(s, -1) {
		y, _ := strconv.Atoi(s[m[2]:m[3]])
		if y >= 1880 && y <= 2099 {
			return y, m[2]
		}
	}
	return 0, -1
}

// lastYearBefore returns the last plausible year before limit that is not at
// the very start of s (a leading number is part of the title: "2001 A Space…").
func lastYearBefore(s string, limit int) (int, int) {
	year, at := 0, -1
	for _, m := range yearRe.FindAllStringSubmatchIndex(s, -1) {
		off := m[2]
		if off >= limit || strings.TrimSpace(s[:off]) == "" {
			continue
		}
		if y, _ := strconv.Atoi(s[m[2]:m[3]]); y >= 1880 && y <= 2099 {
			year, at = y, off
		}
	}
	return year, at
}

// lastYearGroup finds the last (…) or […] group containing a year.
func lastYearGroup(s string) ([]int, string) {
	all := bracketRe.FindAllStringSubmatchIndex(s, -1)
	for i := len(all) - 1; i >= 0; i-- {
		m := all[i]
		inner := s[m[2]:m[3]]
		if y, _ := yearAt(inner); y != 0 {
			return m[:2], inner
		}
	}
	return nil, ""
}

// parseGroup reads "Director, Country, Year" style groups. Parts before the
// year are director or country; parts after the year are countries.
func parseGroup(inner string) (int, string, []string) {
	year, off := yearAt(inner)
	trim := func(x string) string { return strings.Trim(x, " -–.") }
	var director string
	var countries []string
	for _, part := range strings.Split(inner[:off], ",") {
		part = trim(part)
		switch {
		case part == "":
		case isCountry(part):
			countries = append(countries, part)
		case director == "":
			director = part
		}
	}
	for _, part := range strings.Split(inner[off+4:], ",") {
		if part = trim(part); part != "" {
			countries = append(countries, part)
		}
	}
	return year, director, countries
}

func isCountry(s string) bool {
	if countryNames[strings.ToLower(s)] {
		return true
	}
	return len(s) <= 3 && s == strings.ToUpper(s) && strings.ToLower(s) != s
}

// applyTitlePatterns handles "1976 - Title", "Last, First - Title" and
// "Title_Director" conventions.
func applyTitlePatterns(s string, p *Parsed) string {
	if m := yearPrefixRe.FindStringSubmatch(s); m != nil && p.Year == 0 {
		p.Year, _ = strconv.Atoi(m[1])
		return m[2]
	}
	if m := lastFirstRe.FindStringSubmatch(s); m != nil && p.Director == "" {
		p.Director = strings.TrimSpace(m[2]) + " " + strings.TrimSpace(m[1])
		return m[3]
	}
	if strings.Count(s, "_") == 1 && strings.Contains(s, " ") && p.Director == "" {
		i := strings.Index(s, "_")
		p.Director = tidy(s[i+1:])
		return s[:i]
	}
	return s
}

func tidy(s string) string {
	return strings.Trim(strings.Join(strings.Fields(s), " "), " -–_.,([")
}

func normRes(s string) string {
	s = strings.ToLower(s)
	if s == "4k" {
		return "2160p"
	}
	return s
}

func normSource(s string) string {
	switch strings.ToLower(strings.ReplaceAll(s, "-", "")) {
	case "bluray", "brrip", "bdrip":
		return "BluRay"
	case "webdl":
		return "WEB-DL"
	case "webrip":
		return "WEBRip"
	case "dvdrip", "dvdscr":
		return "DVDRip"
	case "hdtv":
		return "HDTV"
	case "hdrip":
		return "HDRip"
	case "vhsrip":
		return "VHSRip"
	case "tvrip":
		return "TVRip"
	}
	return s
}

var codecSeparators = strings.NewReplacer(".", "", " ", "")

func normCodec(s string) string {
	switch codecSeparators.Replace(strings.ToLower(s)) {
	case "x264", "h264":
		return "H.264"
	case "x265", "h265", "hevc":
		return "H.265"
	case "xvid":
		return "XviD"
	case "divx":
		return "DivX"
	case "av1":
		return "AV1"
	}
	return s
}

var langNames = map[string]string{
	"spanish": "es", "castellano": "es", "latino": "es", "english": "en", "german": "de",
	"french": "fr", "italian": "it", "portuguese": "pt",
}

func normLang(s string) string { return langNames[strings.ToLower(s)] }
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/nameparse/ -v`
Expected: PASS. Si algún caso falla, corregir el parser (no el caso esperado) salvo que el caso esperado sea claramente erróneo; en ese caso documentarlo en el commit.

- [ ] **Step 5: Commit**

```bash
git add internal/nameparse
git commit -m "feat(nameparse): parse titles, years, directors and release tokens"
```

---

### Task 6: nameparse — combinar archivo y carpeta

**Files:**
- Create: `internal/nameparse/merge.go`
- Test: `internal/nameparse/merge_test.go`

- [ ] **Step 1: Test que falla**

```go
package nameparse

import "testing"

func TestMergePrefersMoreInformativeAndFillsGaps(t *testing.T) {
	file := Parse("amarcord.720p.x264")
	dir := Parse("Amarcord [Federico Fellini, 1973]")
	got := Merge(file, dir)
	if got.Title != "Amarcord" || got.Year != 1973 || got.Director != "Federico Fellini" ||
		got.Resolution != "720p" || got.Codec != "H.264" {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeTieKeepsFirst(t *testing.T) {
	got := Merge(Parse("Stalker (1979)"), Parse("Solaris (1972)"))
	if got.Title != "Stalker" || got.Year != 1979 {
		t.Fatalf("got %+v", got)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/nameparse/ -run Merge`
Expected: FAIL — `undefined: Merge`

- [ ] **Step 3: Implementación**

```go
package nameparse

// Merge returns the more informative of two parses (e.g. file name vs folder
// name), filling its empty fields from the other. On a tie a wins.
func Merge(a, b Parsed) Parsed {
	if score(b) > score(a) {
		a, b = b, a
	}
	if a.Year == 0 {
		a.Year = b.Year
	}
	if a.Director == "" {
		a.Director = b.Director
	}
	if len(a.Countries) == 0 {
		a.Countries = b.Countries
	}
	fill := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}
	fill(&a.Resolution, b.Resolution)
	fill(&a.Source, b.Source)
	fill(&a.Codec, b.Codec)
	fill(&a.Language, b.Language)
	fill(&a.Group, b.Group)
	fill(&a.IMDbID, b.IMDbID)
	return a
}

func score(p Parsed) int {
	s := 0
	if p.Year != 0 {
		s += 2
	}
	if p.Director != "" {
		s++
	}
	return s
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/nameparse/ -v`
Expected: PASS (corpus + 2 tests de Merge)

- [ ] **Step 5: Commit**

```bash
git add internal/nameparse
git commit -m "feat(nameparse): merge file and folder parses"
```

---

### Task 7: grouping — archivos → versiones

Reglas (spec §4 paso 2):
- El **directorio dueño** de un archivo es su carpeta, salvo que esta se llame `VIDEO_TS`, `Subs`/`Subtitles`/`Subtitulos` o `Extras`/`Bonus`/`Featurettes`: en ese caso es la carpeta padre.
- Videos del mismo dueño con el mismo nombre base y un marcador de parte (`Part 1`, `CD2`, `Disc 1`) forman una versión. Sin marcador de parte, la extensión forma parte de la clave (`X.avi` y `X.mkv` son versiones distintas).
- Todos los VOB/IFO/BUP de un dueño forman una versión y el nombre sale de la carpeta.
- Extra = carpeta de extras, palabra clave en el nombre o tamaño < 15% del video más grande del dueño. Los extras se asocian solo si el dueño tiene exactamente una versión.
- Subtítulo → la versión cuyo nombre base es el prefijo más largo de su nombre; si no hay ninguna y hay una sola versión, a esa.
- Si el dueño no es una raíz y tiene una sola versión, se combina el parseo con el nombre de la carpeta (`nameparse.Merge`).

**Files:**
- Create: `internal/grouping/grouping.go`
- Test: `internal/grouping/grouping_test.go`

- [ ] **Step 1: Test que falla**

```go
package grouping

import (
	"reflect"
	"testing"

	"cinexplorer/internal/mediafile"
)

var roots = []string{"../cine", "../cine-ordenar"}

func only(t *testing.T, vs []Version) Version {
	t.Helper()
	if len(vs) != 1 {
		t.Fatalf("want 1 version, got %d: %+v", len(vs), vs)
	}
	return vs[0]
}

func TestPartsAndExtras(t *testing.T) {
	dir := "../cine/1970s/1900 (Novecento) (1976) [mkvonly]"
	v := only(t, Build([]Entry{
		{dir + "/1900 (Novecento) (1976) - Part 2.mkv", 2_177_896_754, mediafile.Video},
		{dir + "/1900 (Novecento) (1976) - Part 1.mkv", 2_295_706_615, mediafile.Video},
		{dir + "/Bonus 1900 The Story, The Cast, Creating An Epic Eng + Ita + Rus subs.mkv", 296_513_285, mediafile.Video},
	}, roots))

	if v.Dir != dir || v.Parts != 2 || v.Size != 4_473_603_369 {
		t.Fatalf("got dir=%q parts=%d size=%d", v.Dir, v.Parts, v.Size)
	}
	if v.Parsed.Title != "1900 (Novecento)" || v.Parsed.Year != 1976 {
		t.Fatalf("parsed %+v", v.Parsed)
	}
	want := []Member{
		{Path: dir + "/1900 (Novecento) (1976) - Part 1.mkv", Role: RoleMain, Part: 1},
		{Path: dir + "/1900 (Novecento) (1976) - Part 2.mkv", Role: RoleMain, Part: 2},
		{Path: dir + "/Bonus 1900 The Story, The Cast, Creating An Epic Eng + Ita + Rus subs.mkv", Role: RoleExtra},
	}
	if !reflect.DeepEqual(v.Members, want) {
		t.Fatalf("members %+v", v.Members)
	}
}

func TestContainerFolderKeepsFileNames(t *testing.T) {
	vs := Build([]Entry{
		{"../cine/1970s/Arabian.Nights.1974.CC.1080p.BluRay.FLAC1.0.x264-ADE.mkv", 9_000_000_000, mediafile.Video},
		{"../cine/1970s/Deep.Throat.1972.1080p.BluRay.x264.DTS-FGT.mkv", 7_000_000_000, mediafile.Video},
	}, roots)
	if len(vs) != 2 || vs[0].Parsed.Title != "Arabian Nights" || vs[1].Parsed.Title != "Deep Throat" {
		t.Fatalf("got %+v", vs)
	}
}

func TestLooseFileInRoot(t *testing.T) {
	v := only(t, Build([]Entry{{"../cine-ordenar/Attenberg.avi", 700_000_000, mediafile.Video}}, roots))
	if v.Parsed.Title != "Attenberg" || v.Dir != "../cine-ordenar" {
		t.Fatalf("got %+v", v)
	}
}

func TestVideoTS(t *testing.T) {
	v := only(t, Build([]Entry{
		{"../cine-ordenar/Aurora/VIDEO_TS/VTS_01_1.VOB", 1_000_000_000, mediafile.DVD},
		{"../cine-ordenar/Aurora/VIDEO_TS/VIDEO_TS.IFO", 20_000, mediafile.DVD},
	}, roots))
	if v.Dir != "../cine-ordenar/Aurora" || v.Parsed.Title != "Aurora" || v.Parts != 1 || v.Size != 1_000_020_000 {
		t.Fatalf("got %+v", v)
	}
}

func TestSubtitlesInFolderAndSubsDir(t *testing.T) {
	dir := "../cine/2000s/In the Mood for Love (2000)"
	v := only(t, Build([]Entry{
		{dir + "/In.the.Mood.for.Love.2000.720p.BluRay.x264.mkv", 5_000_000_000, mediafile.Video},
		{dir + "/In.the.Mood.for.Love.2000.720p.BluRay.x264.spa.srt", 90_000, mediafile.Subtitle},
		{dir + "/Subs/English.srt", 90_000, mediafile.Subtitle},
	}, roots))
	if v.Parsed.Title != "In the Mood for Love" || v.Parsed.Year != 2000 {
		t.Fatalf("parsed %+v", v.Parsed)
	}
	if got := v.SubLangs(); !reflect.DeepEqual(got, []string{"en", "es"}) {
		t.Fatalf("sub langs %v", got)
	}
}

func TestSameNameDifferentContainerAreTwoVersions(t *testing.T) {
	vs := Build([]Entry{
		{"../cine-ordenar/Carandiru/Carandiru.avi", 700_000_000, mediafile.Video},
		{"../cine-ordenar/Carandiru/Carandiru.mkv", 1_400_000_000, mediafile.Video},
	}, roots)
	if len(vs) != 2 || vs[0].Parts != 1 || vs[1].Parts != 1 {
		t.Fatalf("got %+v", vs)
	}
}

func TestSubtitlesMatchByPrefix(t *testing.T) {
	vs := Build([]Entry{
		{"../cine-ordenar/A.avi", 100, mediafile.Video},
		{"../cine-ordenar/A.srt", 1, mediafile.Subtitle},
		{"../cine-ordenar/B.avi", 100, mediafile.Video},
		{"../cine-ordenar/B.en.srt", 1, mediafile.Subtitle},
	}, roots)
	if len(vs) != 2 {
		t.Fatalf("got %d versions", len(vs))
	}
	if got := vs[0].SubLangs(); !reflect.DeepEqual(got, []string{"?"}) {
		t.Fatalf("A subs %v", got)
	}
	if got := vs[1].SubLangs(); !reflect.DeepEqual(got, []string{"en"}) {
		t.Fatalf("B subs %v", got)
	}
}

func TestSplitPart(t *testing.T) {
	cases := map[string]struct {
		base string
		part int
	}{
		"Movie CD1":                    {"Movie", 1},
		"Movie.cd2":                    {"Movie", 2},
		"Movie (Part 2)":               {"Movie", 2},
		"Movie - Disc 1":               {"Movie", 1},
		"The Apartment 2":              {"The Apartment 2", 0},
		"Egypt 2":                      {"Egypt 2", 0},
	}
	for in, want := range cases {
		base, part := splitPart(in)
		if base != want.base || part != want.part {
			t.Errorf("splitPart(%q) = %q, %d", in, base, part)
		}
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/grouping/`
Expected: FAIL — `undefined: Build`

- [ ] **Step 3: Implementación**

```go
// Package grouping turns the flat list of library files into versions: the
// set of files that play as one unit, plus their subtitles and extras.
package grouping

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"cinexplorer/internal/mediafile"
	"cinexplorer/internal/nameparse"
)

type Entry struct {
	Path string // catalog form, '/'-separated
	Size int64
	Kind mediafile.Kind
}

type Role string

const (
	RoleMain     Role = "main"
	RoleSubtitle Role = "subtitle"
	RoleExtra    Role = "extra"
)

type Member struct {
	Path string
	Role Role
	Part int    // 1-based for multi-part versions, 0 otherwise
	Lang string // subtitles only
}

type Version struct {
	Dir     string
	Members []Member
	Parsed  nameparse.Parsed
	Size    int64 // main files only
	Parts   int
}

// SubLangs returns the sorted, unique subtitle languages ("?" when unknown).
func (v Version) SubLangs() []string {
	seen := map[string]bool{}
	for _, m := range v.Members {
		if m.Role == RoleSubtitle {
			l := m.Lang
			if l == "" {
				l = "?"
			}
			seen[l] = true
		}
	}
	out := make([]string, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

var (
	partRe        = regexp.MustCompile(`(?i)[\s._\-(\[]*\b(?:part|pt|cd|disc|disk|parte)[\s._\-]*(\d{1,2})\b[)\]]?`)
	extraRe       = regexp.MustCompile(`(?i)\b(?:bonus|extras?|featurettes?|trailer|sample|making[\s._-]?of|behind[\s._-]the[\s._-]scenes|deleted[\s._-]scenes)\b`)
	extraDirs     = map[string]bool{"extras": true, "extra": true, "bonus": true, "featurettes": true, "special features": true}
	subDirs       = map[string]bool{"subs": true, "subtitles": true, "subtitulos": true, "subtítulos": true}
	subQualifiers = map[string]bool{"forced": true, "sdh": true, "hi": true, "cc": true}
	roleOrder     = map[Role]int{RoleMain: 0, RoleSubtitle: 1, RoleExtra: 2}
)

var langCodes = map[string]string{
	"es": "es", "spa": "es", "esp": "es", "spanish": "es", "español": "es", "espanol": "es", "castellano": "es", "latino": "es",
	"en": "en", "eng": "en", "english": "en", "ingles": "en", "inglés": "en",
	"it": "it", "ita": "it", "italian": "it", "italiano": "it",
	"fr": "fr", "fre": "fr", "fra": "fr", "french": "fr", "frances": "fr", "francés": "fr",
	"de": "de", "ger": "de", "deu": "de", "german": "de", "aleman": "de", "alemán": "de",
	"pt": "pt", "por": "pt", "portuguese": "pt", "portugues": "pt", "português": "pt",
	"ru": "ru", "rus": "ru", "russian": "ru",
	"ja": "ja", "jpn": "ja", "japanese": "ja",
}

// Build groups entries into versions. roots are the catalog-form library
// roots; a folder that is a root is never used as a movie name.
func Build(entries []Entry, roots []string) []Version {
	isRoot := map[string]bool{}
	for _, r := range roots {
		isRoot[path.Clean(r)] = true
	}
	byDir := map[string][]Entry{}
	for _, e := range entries {
		d := ownerDir(e.Path)
		byDir[d] = append(byDir[d], e)
	}
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	var out []Version
	for _, d := range dirs {
		out = append(out, buildDir(d, byDir[d], isRoot[d])...)
	}
	return out
}

func ownerDir(p string) string {
	dir := path.Dir(p)
	base := strings.ToLower(path.Base(dir))
	if base == "video_ts" || extraDirs[base] || subDirs[base] {
		return path.Dir(dir)
	}
	return dir
}

func buildDir(dir string, es []Entry, dirIsRoot bool) []Version {
	sort.Slice(es, func(i, j int) bool { return es[i].Path < es[j].Path })
	var largest int64
	for _, e := range es {
		if e.Kind == mediafile.Video && e.Size > largest {
			largest = e.Size
		}
	}

	var versions []*Version
	keys := map[*Version]string{} // lowercase base name, for subtitle matching
	byKey := map[string]*Version{}
	var extras []Member
	var dvd *Version

	for _, e := range es {
		switch e.Kind {
		case mediafile.Video:
			name := stem(e.Path)
			if isExtra(e, name, largest) {
				extras = append(extras, Member{Path: e.Path, Role: RoleExtra})
				continue
			}
			base, part := splitPart(name)
			key := strings.ToLower(base)
			if part == 0 {
				key += "|" + strings.ToLower(path.Ext(e.Path))
			}
			v := byKey[key]
			if v == nil {
				v = &Version{Dir: dir, Parsed: nameparse.Parse(base)}
				byKey[key] = v
				keys[v] = strings.ToLower(base)
				versions = append(versions, v)
			}
			v.Members = append(v.Members, Member{Path: e.Path, Role: RoleMain, Part: part})
			v.Size += e.Size
		case mediafile.DVD:
			if dvd == nil {
				dvd = &Version{Dir: dir, Parsed: nameparse.Parse(path.Base(dir))}
				versions = append(versions, dvd)
			}
			dvd.Members = append(dvd.Members, Member{Path: e.Path, Role: RoleMain})
			dvd.Size += e.Size
		}
	}
	if len(versions) == 0 {
		return nil
	}
	if len(versions) == 1 && !dirIsRoot && versions[0] != dvd {
		versions[0].Parsed = nameparse.Merge(versions[0].Parsed, nameparse.Parse(path.Base(dir)))
	}

	for _, e := range es {
		if e.Kind != mediafile.Subtitle {
			continue
		}
		s := stem(e.Path)
		if v := subtitleOwner(strings.ToLower(s), versions, keys); v != nil {
			v.Members = append(v.Members, Member{Path: e.Path, Role: RoleSubtitle, Lang: subLang(s)})
		}
	}
	if len(versions) == 1 {
		versions[0].Members = append(versions[0].Members, extras...)
	}

	out := make([]Version, 0, len(versions))
	for _, v := range versions {
		v.Parts = 0
		for _, m := range v.Members {
			if m.Role == RoleMain {
				v.Parts++
			}
		}
		if v == dvd {
			v.Parts = 1
		}
		sortMembers(v.Members)
		out = append(out, *v)
	}
	return out
}

func isExtra(e Entry, name string, largest int64) bool {
	if extraDirs[strings.ToLower(path.Base(path.Dir(e.Path)))] {
		return true
	}
	if extraRe.MatchString(name) {
		return true
	}
	return largest > 0 && e.Size*100 < largest*15
}

// splitPart removes a part marker ("CD1", "Part 2", "Disc 1") from name.
func splitPart(name string) (string, int) {
	loc := partRe.FindStringSubmatchIndex(name)
	if loc == nil {
		return name, 0
	}
	n, _ := strconv.Atoi(name[loc[2]:loc[3]])
	return strings.TrimSpace(name[:loc[0]] + name[loc[1]:]), n
}

func subtitleOwner(sub string, versions []*Version, keys map[*Version]string) *Version {
	var best *Version
	bestLen := 0
	for _, v := range versions {
		k := keys[v]
		if k != "" && strings.HasPrefix(sub, k) && len(k) > bestLen {
			best, bestLen = v, len(k)
		}
	}
	if best == nil && len(versions) == 1 {
		best = versions[0]
	}
	return best
}

// subLang reads the language from the last token of a subtitle name
// ("Movie.spa", "English", "Movie.en.forced").
func subLang(stem string) string {
	tokens := strings.FieldsFunc(strings.ToLower(stem), func(r rune) bool {
		return strings.ContainsRune(" ._-[]()", r)
	})
	for i := len(tokens) - 1; i >= 0; i-- {
		if subQualifiers[tokens[i]] {
			continue
		}
		return langCodes[tokens[i]]
	}
	return ""
}

func sortMembers(ms []Member) {
	sort.SliceStable(ms, func(i, j int) bool {
		if roleOrder[ms[i].Role] != roleOrder[ms[j].Role] {
			return roleOrder[ms[i].Role] < roleOrder[ms[j].Role]
		}
		if ms[i].Part != ms[j].Part {
			return ms[i].Part < ms[j].Part
		}
		return ms[i].Path < ms[j].Path
	})
}

func stem(p string) string {
	b := path.Base(p)
	return strings.TrimSuffix(b, path.Ext(b))
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/grouping/ -v`
Expected: PASS (8 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/grouping
git commit -m "feat(grouping): build versions from parts, DVDs, subtitles and extras"
```

---

### Task 8: store — SQLite

**Files:**
- Create: `internal/store/schema.sql`
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Agregar la dependencia**

Run: `go get modernc.org/sqlite@latest`
Expected: `go: added modernc.org/sqlite v1.x.y`

- [ ] **Step 2: Test que falla**

```go
package store

import (
	"path/filepath"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "cinexplorer.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

var roots = []string{"../cine", "../cine-ordenar"}

func TestSyncFilesUpsertsAndMarksMissing(t *testing.T) {
	s := open(t)
	rows := []FileRow{
		{Path: "../cine/a.mkv", Size: 10, MTime: 1, Fingerprint: "fa", Kind: "video"},
		{Path: "../cine/b.mkv", Size: 20, MTime: 2, Fingerprint: "fb", Kind: "video"},
	}
	if err := s.SyncFiles(rows, roots); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncFiles(rows[:1], roots); err != nil {
		t.Fatal(err)
	}
	idx, err := s.FileIndex()
	if err != nil {
		t.Fatal(err)
	}
	if idx["../cine/a.mkv"].Missing || !idx["../cine/b.mkv"].Missing || idx["../cine/b.mkv"].Fingerprint != "fb" {
		t.Fatalf("index %+v", idx)
	}
	// A root that was not scanned (e.g. not mounted) must not lose its files.
	if err := s.SyncFiles(nil, []string{"../cine-ordenar"}); err != nil {
		t.Fatal(err)
	}
	if idx, _ := s.FileIndex(); idx["../cine/a.mkv"].Missing {
		t.Fatal("files under unscanned roots must keep their state")
	}
}

func TestReplaceVersionsAndRead(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{
		{Path: "../cine/x/1900 - Part 1.mkv", Size: 10, MTime: 1, Fingerprint: "p1", Kind: "video"},
		{Path: "../cine/x/1900 - Part 2.mkv", Size: 12, MTime: 1, Fingerprint: "p2", Kind: "video"},
		{Path: "../cine/x/1900.spa.srt", Size: 1, MTime: 1, Kind: "subtitle"},
	}, roots)
	err := s.ReplaceVersions([]grouping.Version{{
		Dir:    "../cine/x",
		Parsed: nameparse.Parsed{Title: "1900", Year: 1976, Director: "Bernardo Bertolucci", Resolution: "1080p"},
		Size:   22,
		Parts:  2,
		Members: []grouping.Member{
			{Path: "../cine/x/1900 - Part 1.mkv", Role: grouping.RoleMain, Part: 1},
			{Path: "../cine/x/1900 - Part 2.mkv", Role: grouping.RoleMain, Part: 2},
			{Path: "../cine/x/1900.spa.srt", Role: grouping.RoleSubtitle, Lang: "es"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d versions", len(vs))
	}
	v := vs[0]
	if v.Title != "1900" || v.Year != 1976 || v.Parts != 2 || v.Size != 22 || v.SubLangs != "es" || len(v.Files) != 3 {
		t.Fatalf("got %+v", v)
	}
	if v.Files[0].Role != "main" || v.Files[0].Part != 1 || v.Files[2].Role != "subtitle" {
		t.Fatalf("files %+v", v.Files)
	}
	// Replacing again must not duplicate.
	s.ReplaceVersions(nil)
	if vs, _ := s.Versions(); len(vs) != 0 {
		t.Fatalf("expected empty, got %d", len(vs))
	}
}

func TestDuplicatesAndHasFile(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{
		{Path: "../cine/a.mkv", Size: 100, MTime: 1, Fingerprint: "same", Kind: "video"},
		{Path: "../cine-ordenar/a-copy.mkv", Size: 100, MTime: 1, Fingerprint: "same", Kind: "video"},
		{Path: "../cine/b.mkv", Size: 50, MTime: 1, Fingerprint: "other", Kind: "video"},
	}, roots)
	d, err := s.Duplicates()
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != 1 || d[0].Size != 100 || len(d[0].Paths) != 2 {
		t.Fatalf("got %+v", d)
	}
	if ok, _ := s.HasFile("../cine/a.mkv"); !ok {
		t.Fatal("known file")
	}
	if ok, _ := s.HasFile("../../etc/passwd"); ok {
		t.Fatal("unknown file")
	}
}

func TestOpenMemory(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if vs, err := s.Versions(); err != nil || len(vs) != 0 {
		t.Fatalf("got %v, %v", vs, err)
	}
}
```

- [ ] **Step 3: Verificar que falla**

Run: `go test ./internal/store/`
Expected: FAIL — `undefined: Open`

- [ ] **Step 4: Esquema**

`internal/store/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS versions (
  id            INTEGER PRIMARY KEY,
  dir           TEXT    NOT NULL,
  title         TEXT    NOT NULL,
  year          INTEGER NOT NULL DEFAULT 0,
  director      TEXT    NOT NULL DEFAULT '',
  countries     TEXT    NOT NULL DEFAULT '',
  resolution    TEXT    NOT NULL DEFAULT '',
  source        TEXT    NOT NULL DEFAULT '',
  codec         TEXT    NOT NULL DEFAULT '',
  language      TEXT    NOT NULL DEFAULT '',
  release_group TEXT    NOT NULL DEFAULT '',
  imdb_id       TEXT    NOT NULL DEFAULT '',
  size          INTEGER NOT NULL,
  parts         INTEGER NOT NULL,
  sub_langs     TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS files (
  id          INTEGER PRIMARY KEY,
  path        TEXT    NOT NULL UNIQUE,  -- relative to the app dir, '/'-separated
  size        INTEGER NOT NULL,
  mtime       INTEGER NOT NULL,         -- unix milliseconds
  fingerprint TEXT    NOT NULL DEFAULT '',
  kind        TEXT    NOT NULL,
  missing     INTEGER NOT NULL DEFAULT 0,
  version_id  INTEGER REFERENCES versions(id) ON DELETE SET NULL,
  role        TEXT    NOT NULL DEFAULT '',
  part        INTEGER NOT NULL DEFAULT 0,
  lang        TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS files_fingerprint ON files(fingerprint);
CREATE INDEX IF NOT EXISTS files_version ON files(version_id);
```

- [ ] **Step 5: Implementación**

`internal/store/store.go`:

```go
// Package store persists the catalog in a SQLite file next to the executable.
package store

import (
	"database/sql"
	_ "embed"
	"strings"

	_ "modernc.org/sqlite"

	"cinexplorer/internal/grouping"
)

//go:embed schema.sql
var schema string

type Store struct {
	db *sql.DB
}

type FileRow struct {
	Path        string
	Size        int64
	MTime       int64
	Fingerprint string
	Kind        string
	Missing     bool
}

type FileView struct {
	Path    string `json:"path"`
	Role    string `json:"role"`
	Part    int    `json:"part"`
	Lang    string `json:"lang"`
	Missing bool   `json:"missing"`
}

type VersionView struct {
	ID         int64      `json:"id"`
	Dir        string     `json:"dir"`
	Title      string     `json:"title"`
	Year       int        `json:"year"`
	Director   string     `json:"director"`
	Countries  string     `json:"countries"`
	Resolution string     `json:"resolution"`
	Source     string     `json:"source"`
	Codec      string     `json:"codec"`
	Language   string     `json:"language"`
	Size       int64      `json:"size"`
	Parts      int        `json:"parts"`
	SubLangs   string     `json:"subLangs"`
	Files      []FileView `json:"files"`
}

type Duplicate struct {
	Fingerprint string   `json:"fingerprint"`
	Size        int64    `json:"size"`
	Paths       []string `json:"paths"`
}

// Open opens (creating if needed) the catalog. Journal mode DELETE instead of
// WAL: safer on removable drives. readOnly opens with query_only.
func Open(path string, readOnly bool) (*Store, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if readOnly {
		dsn += "&_pragma=query_only(1)"
	} else {
		dsn += "&_pragma=journal_mode(DELETE)"
	}
	return open(dsn, !readOnly)
}

// OpenMemory returns an empty in-memory catalog (read-only mode without a db file).
func OpenMemory() (*Store, error) {
	return open(":memory:?_pragma=foreign_keys(1)", true)
}

func open(dsn string, migrate bool) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if migrate {
		if _, err := db.Exec(schema); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// FileIndex returns every known file by path, including missing ones.
func (s *Store) FileIndex() (map[string]FileRow, error) {
	rows, err := s.db.Query(`SELECT path, size, mtime, fingerprint, kind, missing FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	idx := map[string]FileRow{}
	for rows.Next() {
		var f FileRow
		if err := rows.Scan(&f.Path, &f.Size, &f.MTime, &f.Fingerprint, &f.Kind, &f.Missing); err != nil {
			return nil, err
		}
		idx[f.Path] = f
	}
	return idx, rows.Err()
}

// SyncFiles upserts the files seen in a scan and marks as missing the known
// files under scannedRoots that were not seen. Files under other roots keep
// their state, so an unmounted folder does not wipe its entries.
func (s *Store) SyncFiles(seen []FileRow, scannedRoots []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	up, err := tx.Prepare(`INSERT INTO files (path, size, mtime, fingerprint, kind, missing) VALUES (?, ?, ?, ?, ?, 0)
		ON CONFLICT(path) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
		fingerprint = excluded.fingerprint, kind = excluded.kind, missing = 0`)
	if err != nil {
		return err
	}
	defer up.Close()
	present := make(map[string]bool, len(seen))
	for _, f := range seen {
		if _, err := up.Exec(f.Path, f.Size, f.MTime, f.Fingerprint, f.Kind); err != nil {
			return err
		}
		present[f.Path] = true
	}

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
		if !present[p] && underAny(p, scannedRoots) {
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
	for _, r := range roots {
		if strings.HasPrefix(p, strings.TrimSuffix(r, "/")+"/") {
			return true
		}
	}
	return false
}

// ReplaceVersions rebuilds the versions table and the file→version links.
func (s *Store) ReplaceVersions(vs []grouping.Version) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE files SET version_id = NULL, role = '', part = 0, lang = ''`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM versions`); err != nil {
		return err
	}
	insV, err := tx.Prepare(`INSERT INTO versions (dir, title, year, director, countries, resolution, source, codec,
		language, release_group, imdb_id, size, parts, sub_langs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insV.Close()
	updF, err := tx.Prepare(`UPDATE files SET version_id = ?, role = ?, part = ?, lang = ? WHERE path = ?`)
	if err != nil {
		return err
	}
	defer updF.Close()

	for _, v := range vs {
		p := v.Parsed
		res, err := insV.Exec(v.Dir, p.Title, p.Year, p.Director, strings.Join(p.Countries, ", "), p.Resolution,
			p.Source, p.Codec, p.Language, p.Group, p.IMDbID, v.Size, v.Parts, strings.Join(v.SubLangs(), ","))
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for _, m := range v.Members {
			if _, err := updF.Exec(id, string(m.Role), m.Part, m.Lang, m.Path); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// Versions returns every version with its files, ordered by title and year.
func (s *Store) Versions() ([]VersionView, error) {
	rows, err := s.db.Query(`SELECT id, dir, title, year, director, countries, resolution, source, codec, language,
		size, parts, sub_langs FROM versions ORDER BY title COLLATE NOCASE, year, id`)
	if err != nil {
		return nil, err
	}
	out := []VersionView{}
	pos := map[int64]int{}
	for rows.Next() {
		v := VersionView{Files: []FileView{}}
		if err := rows.Scan(&v.ID, &v.Dir, &v.Title, &v.Year, &v.Director, &v.Countries, &v.Resolution,
			&v.Source, &v.Codec, &v.Language, &v.Size, &v.Parts, &v.SubLangs); err != nil {
			rows.Close()
			return nil, err
		}
		pos[v.ID] = len(out)
		out = append(out, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	frows, err := s.db.Query(`SELECT version_id, path, role, part, lang, missing FROM files
		WHERE version_id IS NOT NULL
		ORDER BY version_id, CASE role WHEN 'main' THEN 0 WHEN 'subtitle' THEN 1 ELSE 2 END, part, path`)
	if err != nil {
		return nil, err
	}
	defer frows.Close()
	for frows.Next() {
		var id int64
		var f FileView
		if err := frows.Scan(&id, &f.Path, &f.Role, &f.Part, &f.Lang, &f.Missing); err != nil {
			return nil, err
		}
		if i, ok := pos[id]; ok {
			out[i].Files = append(out[i].Files, f)
		}
	}
	return out, frows.Err()
}

// Duplicates returns groups of present files with the same fingerprint,
// largest first.
func (s *Store) Duplicates() ([]Duplicate, error) {
	rows, err := s.db.Query(`SELECT fingerprint, size, path FROM files
		WHERE missing = 0 AND fingerprint != '' AND fingerprint IN (
			SELECT fingerprint FROM files WHERE missing = 0 AND fingerprint != ''
			GROUP BY fingerprint HAVING COUNT(*) > 1)
		ORDER BY size DESC, fingerprint, path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Duplicate{}
	for rows.Next() {
		var fp, p string
		var size int64
		if err := rows.Scan(&fp, &size, &p); err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].Fingerprint == fp {
			out[n-1].Paths = append(out[n-1].Paths, p)
			continue
		}
		out = append(out, Duplicate{Fingerprint: fp, Size: size, Paths: []string{p}})
	}
	return out, rows.Err()
}

// HasFile reports whether path is a present file in the catalog. The API uses
// it so that only catalogued files can be opened.
func (s *Store) HasFile(path string) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM files WHERE path = ? AND missing = 0`, path).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
```

- [ ] **Step 6: Verificar que pasa**

Run: `go test ./internal/store/ -v`
Expected: PASS (4 tests)

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/store
git commit -m "feat(store): SQLite catalog for files, versions and duplicates"
```

---

### Task 9: scan — escaneo incremental

**Files:**
- Create: `internal/scan/scan.go`
- Test: `internal/scan/scan_test.go`

- [ ] **Step 1: Test que falla**

```go
package scan

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"cinexplorer/internal/store"
)

func writeFile(t *testing.T, path string, size int, fill byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{fill}, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) (string, string, *store.Store) {
	t.Helper()
	disk := t.TempDir()
	app := filepath.Join(disk, "cinexplorer")
	if err := os.Mkdir(app, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(app, "cinexplorer.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return disk, app, st
}

func TestScanBuildsCatalogIncrementally(t *testing.T) {
	disk, app, st := setup(t)
	amarcord := filepath.Join(disk, "cine", "1970s", "Amarcord [Federico Fellini, 1973]")
	writeFile(t, filepath.Join(amarcord, "amarcord.mkv"), 4096, 'a')
	writeFile(t, filepath.Join(amarcord, "amarcord.spa.srt"), 100, 's')
	writeFile(t, filepath.Join(amarcord, "info.nfo"), 10, 'n')
	writeFile(t, filepath.Join(disk, "cine", "Thumbs.db"), 10, 'x')
	writeFile(t, filepath.Join(disk, "cine-ordenar", "Attenberg.avi"), 4096, 'b')

	sc := &Scanner{AppDir: app, Roots: []string{"../cine", "../cine-ordenar"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	vs, err := st.Versions()
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("want 2 versions, got %+v", vs)
	}
	if v := vs[0]; v.Title != "Amarcord" || v.Year != 1973 || v.Director != "Federico Fellini" || v.SubLangs != "es" {
		t.Fatalf("got %+v", v)
	}
	if got := sc.Status(); got.Files != 4 || got.Hashed != 2 || got.Versions != 2 || got.Running {
		t.Fatalf("status %+v", got)
	}

	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := sc.Status(); got.Hashed != 0 {
		t.Fatalf("second scan re-hashed %d files", got.Hashed)
	}
}

func TestScanMarksMissingAndFindsCopies(t *testing.T) {
	disk, app, st := setup(t)
	original := filepath.Join(disk, "cine", "a", "Movie.mkv")
	writeFile(t, original, 4096, 'm')
	writeFile(t, filepath.Join(disk, "cine-ordenar", "movie-copy.mkv"), 4096, 'm')

	sc := &Scanner{AppDir: app, Roots: []string{"../cine", "../cine-ordenar"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d, _ := st.Duplicates(); len(d) != 1 || len(d[0].Paths) != 2 {
		t.Fatalf("duplicates %+v", d)
	}

	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d, _ := st.Duplicates(); len(d) != 0 {
		t.Fatalf("duplicates after delete %+v", d)
	}
	idx, _ := st.FileIndex()
	if !idx["../cine/a/Movie.mkv"].Missing {
		t.Fatal("deleted file should be marked missing")
	}
}

func TestScanSkipsUnavailableRoot(t *testing.T) {
	disk, app, st := setup(t)
	writeFile(t, filepath.Join(disk, "cine", "Stalker (1979).mkv"), 4096, 's')
	sc := &Scanner{AppDir: app, Roots: []string{"../cine", "../no-existe"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if vs, _ := st.Versions(); len(vs) != 1 || vs[0].Title != "Stalker" {
		t.Fatalf("got %+v", vs)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/scan/`
Expected: FAIL — `undefined: Scanner`

- [ ] **Step 3: Implementación**

```go
// Package scan walks the library roots and refreshes the catalog. It only
// reads the library; the only writes go to the catalog database.
package scan

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/fingerprint"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/mediafile"
	"cinexplorer/internal/store"
)

var ErrBusy = errors.New("ya hay un escaneo en curso")

type Status struct {
	Running   bool      `json:"running"`
	Files     int64     `json:"files"`
	Hashed    int64     `json:"hashed"`
	Versions  int       `json:"versions"`
	LastError string    `json:"lastError"`
	Finished  time.Time `json:"finished"`
}

type Scanner struct {
	AppDir string
	Roots  []string // catalog form
	Store  *store.Store

	mu     sync.Mutex
	status Status
}

func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Run performs one incremental scan. A call while another is running returns ErrBusy.
func (s *Scanner) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		return ErrBusy
	}
	s.status = Status{Running: true}
	s.mu.Unlock()

	err := s.run(ctx)

	s.mu.Lock()
	s.status.Running = false
	s.status.Finished = time.Now()
	if err != nil {
		s.status.LastError = err.Error()
	}
	s.mu.Unlock()
	return err
}

func (s *Scanner) run(ctx context.Context) error {
	known, err := s.Store.FileIndex()
	if err != nil {
		return err
	}
	var seen []store.FileRow
	var entries []grouping.Entry
	var scanned []string

	for _, root := range s.Roots {
		abs := appdir.Abs(s.AppDir, root)
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			log.Printf("raíz no disponible, se omite: %s", root)
			continue
		}
		scanned = append(scanned, root)
		err := filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("no se puede leer %s: %v", p, err)
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				if p != abs && strings.HasPrefix(d.Name(), ".") {
					return fs.SkipDir
				}
				return nil
			}
			kind := mediafile.Classify(d.Name())
			if !kind.Stored() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				log.Printf("no se puede leer %s: %v", p, err)
				return nil
			}
			rel, err := appdir.Rel(s.AppDir, p)
			if err != nil {
				return err
			}
			row := store.FileRow{Path: rel, Size: info.Size(), MTime: info.ModTime().UnixMilli(), Kind: string(kind)}
			hashed := int64(0)
			if k, ok := known[rel]; ok && k.Size == row.Size && k.MTime == row.MTime && (k.Fingerprint != "" || !kind.Fingerprinted()) {
				row.Fingerprint = k.Fingerprint
			} else if kind.Fingerprinted() {
				fp, err := fingerprint.Of(p)
				if err != nil {
					log.Printf("no se puede leer %s: %v", p, err)
					return nil
				}
				row.Fingerprint = fp
				hashed = 1
			}
			seen = append(seen, row)
			entries = append(entries, grouping.Entry{Path: rel, Size: row.Size, Kind: kind})
			s.add(1, hashed)
			return nil
		})
		if err != nil {
			return err
		}
		// A root that vanished mid-walk means the disk was unplugged: keep the
		// previous catalog instead of marking everything as missing.
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("la raíz %s desapareció durante el escaneo", root)
		}
	}

	if err := s.Store.SyncFiles(seen, scanned); err != nil {
		return err
	}
	versions := grouping.Build(entries, scanned)
	if err := s.Store.ReplaceVersions(versions); err != nil {
		return err
	}
	s.mu.Lock()
	s.status.Versions = len(versions)
	s.mu.Unlock()
	return nil
}

func (s *Scanner) add(files, hashed int64) {
	s.mu.Lock()
	s.status.Files += files
	s.status.Hashed += hashed
	s.mu.Unlock()
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/scan/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/scan
git commit -m "feat(scan): incremental read-only library scan"
```

---

### Task 10: platform — abrir y mostrar en carpeta

Sin tests automáticos (efectos sobre el escritorio); se verifica que compile en los tres sistemas y se prueba a mano en la Task 13.

**Files:**
- Create: `internal/platform/platform.go`
- Create: `internal/platform/open_windows.go`
- Create: `internal/platform/open_unix.go`

- [ ] **Step 1: Dependencia**

Run: `go get golang.org/x/sys@latest`

- [ ] **Step 2: Código común**

`internal/platform/platform.go`:

```go
// Package platform opens files and folders with the OS default applications.
package platform

import "os/exec"

// start launches cmd without blocking and reaps it in the background.
func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
```

- [ ] **Step 3: Windows**

`internal/platform/open_windows.go`:

```go
//go:build windows

package platform

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// Open opens target (file or URL) with its default application.
func Open(target string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

// Reveal opens Explorer with target selected. The command line is built by
// hand because Explorer does not accept Go's default argument quoting.
func Reveal(target string) error {
	cmd := exec.Command("explorer")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer /select,"` + target + `"`}
	return start(cmd)
}
```

- [ ] **Step 4: macOS / Linux**

`internal/platform/open_unix.go`:

```go
//go:build !windows

package platform

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// Open opens target (file or URL) with its default application.
func Open(target string) error {
	if runtime.GOOS == "darwin" {
		return start(exec.Command("open", target))
	}
	return start(exec.Command("xdg-open", target))
}

// Reveal shows target in the file manager (selected on macOS, its folder on Linux).
func Reveal(target string) error {
	if runtime.GOOS == "darwin" {
		return start(exec.Command("open", "-R", target))
	}
	return start(exec.Command("xdg-open", filepath.Dir(target)))
}
```

- [ ] **Step 5: Verificar compilación en los tres sistemas**

Run:
```bash
go vet ./internal/platform/ && GOOS=linux go vet ./internal/platform/ && GOOS=darwin go vet ./internal/platform/
```
Expected: sin salida (éxito)

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/platform
git commit -m "feat(platform): open files and reveal them in the file manager"
```

---

### Task 11: server — API y página mínima

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/web/index.html`
- Test: `internal/server/server_test.go`

- [ ] **Step 1: Test que falla**

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
)

const moviePath = "../cine/Amarcord.mkv"

func newServer(t *testing.T) (*Server, *[]string) {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.SyncFiles([]store.FileRow{{Path: moviePath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"}}, []string{"../cine"})
	st.ReplaceVersions([]grouping.Version{{
		Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
		Members: []grouping.Member{{Path: moviePath, Role: grouping.RoleMain}},
	}})
	var opened []string
	s := &Server{
		AppDir: t.TempDir(),
		Store:  st,
		Opener: func(p string) error { opened = append(opened, p); return nil },
		Revealer: func(p string) error { opened = append(opened, "reveal:"+p); return nil },
	}
	return s, &opened
}

func request(h http.Handler, method, target, body, ctype, host string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = host
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestVersionsEndpoint(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/api/versions", "", "", "127.0.0.1:8080")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var vs []store.VersionView
	if err := json.Unmarshal(rec.Body.Bytes(), &vs); err != nil || len(vs) != 1 || vs[0].Title != "Amarcord" {
		t.Fatalf("got %s (%v)", rec.Body, err)
	}
}

func TestOpenKnownFile(t *testing.T) {
	s, opened := newServer(t)
	rec := request(s.Handler(), "POST", "/api/open", `{"path":"`+moviePath+`"}`, "application/json", "localhost:8080")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if want := appdir.Abs(s.AppDir, moviePath); len(*opened) != 1 || (*opened)[0] != want {
		t.Fatalf("opened %v, want %s", *opened, want)
	}
}

func TestOpenRejectsUnknownFile(t *testing.T) {
	s, opened := newServer(t)
	rec := request(s.Handler(), "POST", "/api/open", `{"path":"../../Windows/notepad.exe"}`, "application/json", "127.0.0.1")
	if rec.Code != http.StatusNotFound || len(*opened) != 0 {
		t.Fatalf("status %d opened %v", rec.Code, *opened)
	}
}

func TestPostRequiresJSON(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "POST", "/api/open", `{"path":"`+moviePath+`"}`, "text/plain", "127.0.0.1")
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRejectsForeignHost(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/api/versions", "", "", "evil.example.com")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestScanInReadOnlyMode(t *testing.T) {
	s, _ := newServer(t)
	s.ReadOnly = true
	rec := request(s.Handler(), "POST", "/api/scan", "{}", "application/json", "127.0.0.1")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestServesIndex(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/", "", "", "127.0.0.1")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "CINEXPLORER") {
		t.Fatalf("status %d", rec.Code)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/server/`
Expected: FAIL — `undefined: Server`

- [ ] **Step 3: Servidor**

`internal/server/server.go`:

```go
// Package server exposes the catalog as a JSON API plus the embedded web UI.
// It only answers requests addressed to localhost.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/store"
)

//go:embed web
var webFS embed.FS

type Server struct {
	AppDir   string
	Store    *store.Store
	Scanner  *scan.Scanner // nil in read-only mode
	ReadOnly bool
	Opener   func(target string) error
	Revealer func(target string) error
}

func (s *Server) Handler() http.Handler {
	static, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/versions", s.versions)
	mux.HandleFunc("GET /api/duplicates", s.duplicates)
	mux.HandleFunc("POST /api/scan", jsonOnly(s.rescan))
	mux.HandleFunc("POST /api/open", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Opener) }))
	mux.HandleFunc("POST /api/reveal", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Revealer) }))
	return localOnly(mux)
}

// localOnly rejects requests whose Host is not localhost (DNS rebinding).
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// jsonOnly requires a JSON content type, which forces a CORS preflight on
// cross-site requests and so blocks other web pages from calling the API.
func jsonOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "se requiere application/json", http.StatusUnsupportedMediaType)
			return
		}
		h(w, r)
	}
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	var st scan.Status
	if s.Scanner != nil {
		st = s.Scanner.Status()
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "scan": st})
}

func (s *Server) versions(w http.ResponseWriter, r *http.Request) {
	vs, err := s.Store.Versions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, vs)
}

func (s *Server) duplicates(w http.ResponseWriter, r *http.Request) {
	d, err := s.Store.Duplicates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, d)
}

func (s *Server) rescan(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly || s.Scanner == nil {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return
	}
	go func() {
		if err := s.Scanner.Run(context.Background()); err != nil && !errors.Is(err, scan.ErrBusy) {
			log.Printf("escaneo: %v", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}

// withFile runs action on a catalogued file; arbitrary paths are refused.
func (s *Server) withFile(w http.ResponseWriter, r *http.Request, action func(string) error) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	ok, err := s.Store.HasFile(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "archivo desconocido", http.StatusNotFound)
		return
	}
	if err := action(appdir.Abs(s.AppDir, req.Path)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json: %v", err)
	}
}
```

- [ ] **Step 4: Página mínima**

`internal/server/web/index.html`:

```html
<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cinexplorer</title>
<style>
  :root { --bg: #0f1115; --fg: #e8e8e8; --muted: #8b929a; --line: #1d2026; }
  * { box-sizing: border-box; }
  body { margin: 0; background: var(--bg); color: var(--fg); font: 14px/1.5 Inter, system-ui, sans-serif; }
  header { display: flex; align-items: center; gap: 24px; padding: 18px 32px; }
  header b { font-weight: 800; letter-spacing: .04em; }
  nav button { background: none; border: 0; color: var(--muted); font: inherit; cursor: pointer; padding: 0; margin-right: 18px; }
  nav button.on { color: var(--fg); }
  #status { margin-left: auto; color: var(--muted); font-size: 12px; }
  main { padding: 0 32px 48px; }
  input { width: 100%; max-width: 420px; background: #171a20; border: 1px solid #262a31; color: var(--fg);
          padding: 8px 12px; border-radius: 4px; font: inherit; margin: 8px 0 16px; }
  .row { display: grid; grid-template-columns: 1fr auto; gap: 16px; padding: 12px 0; border-bottom: 1px solid var(--line); }
  .t { font-weight: 600; } .t span { color: var(--muted); font-weight: 400; }
  .meta, .path { color: var(--muted); font-size: 12px; } .path { word-break: break-all; }
  .acts { white-space: nowrap; }
  .acts button { background: none; border: 1px solid #30353d; color: var(--fg); border-radius: 3px;
                 padding: 3px 10px; cursor: pointer; margin-left: 6px; font: inherit; font-size: 12px; }
  .empty { color: var(--muted); padding: 40px 0; }
  @media (max-width: 600px) { header, main { padding-left: 16px; padding-right: 16px; } .row { grid-template-columns: 1fr; } }
</style>
</head>
<body>
<header>
  <b>CINEXPLORER</b>
  <nav><button data-view="versions" class="on">Versiones</button><button data-view="copies">Copias idénticas</button></nav>
  <div id="status"></div>
</header>
<main>
  <input id="q" placeholder="Filtrar por título, director o carpeta…" autocomplete="off">
  <div id="list"></div>
</main>
<script>
const $ = (s) => document.querySelector(s);
let view = 'versions', versions = [], copies = [], wasRunning = false;

const gb = (n) => (n / 1e9).toFixed(1) + ' GB';
const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

async function post(url, body) {
  const r = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!r.ok) alert(await r.text());
}

async function load() {
  [versions, copies] = await Promise.all([
    fetch('/api/versions').then((r) => r.json()),
    fetch('/api/duplicates').then((r) => r.json()),
  ]);
  render();
}

function versionRow(v) {
  const main = v.files.find((f) => f.role === 'main');
  const tech = [v.resolution, v.source, v.codec].filter(Boolean).map(esc);
  if (v.parts > 1) tech.push(v.parts + ' partes');
  tech.push(gb(v.size));
  if (v.subLangs) tech.push('subs ' + esc(v.subLangs));
  return `<div class="row"><div>
      <div class="t">${esc(v.title)} <span>${v.year || ''} ${v.director ? '· ' + esc(v.director) : ''}</span></div>
      <div class="meta">${tech.join(' · ')}</div>
      <div class="path">${esc(v.dir)}</div>
    </div><div class="acts">${main ? `<button data-open="${esc(main.path)}">▶ Ver</button><button data-reveal="${esc(main.path)}">Carpeta</button>` : ''}</div></div>`;
}

function copyRow(c) {
  return `<div class="row"><div>
      <div class="t">${gb(c.size)} <span>× ${c.paths.length}</span></div>
      ${c.paths.map((p) => `<div class="path">${esc(p)}</div>`).join('')}
    </div><div class="acts"><button data-reveal="${esc(c.paths[0])}">Carpeta</button></div></div>`;
}

function render() {
  const q = $('#q').value.trim().toLowerCase();
  const rows = view === 'versions'
    ? versions.filter((v) => !q || [v.title, v.director, v.dir].join(' ').toLowerCase().includes(q)).map(versionRow)
    : copies.filter((c) => !q || c.paths.join(' ').toLowerCase().includes(q)).map(copyRow);
  $('#list').innerHTML = rows.length ? rows.join('') : '<div class="empty">Sin resultados.</div>';
}

document.addEventListener('click', (e) => {
  const b = e.target.closest('button');
  if (!b) return;
  if (b.dataset.view) {
    view = b.dataset.view;
    document.querySelectorAll('nav button').forEach((n) => n.classList.toggle('on', n === b));
    render();
  }
  if (b.dataset.open) post('/api/open', { path: b.dataset.open });
  if (b.dataset.reveal) post('/api/reveal', { path: b.dataset.reveal });
});
$('#q').addEventListener('input', render);

async function poll() {
  const s = await fetch('/api/status').then((r) => r.json());
  const sc = s.scan;
  $('#status').textContent = s.readOnly ? 'Modo consulta (solo lectura)'
    : sc.running ? `Escaneando… ${sc.files} archivos`
    : sc.lastError ? 'Error: ' + sc.lastError
    : `${versions.length} versiones`;
  if (wasRunning && !sc.running) load();
  wasRunning = sc.running;
  setTimeout(poll, sc.running ? 1000 : 5000);
}
load().then(poll);
</script>
</body>
</html>
```

- [ ] **Step 5: Verificar que pasa**

Run: `go test ./internal/server/ -v`
Expected: PASS (7 tests)

- [ ] **Step 6: Commit**

```bash
git add internal/server
git commit -m "feat(server): local JSON API and minimal catalog page"
```

---

### Task 12: main — arranque

**Files:**
- Create: `cmd/cinexplorer/main.go`

- [ ] **Step 1: Implementación**

```go
// Command cinexplorer serves a local, read-only catalog of a movie library.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/config"
	"cinexplorer/internal/platform"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/server"
	"cinexplorer/internal/store"
)

func main() {
	dir := flag.String("dir", os.Getenv("CINEXPLORER_DIR"), "directorio de la app (por defecto, el del ejecutable)")
	port := flag.Int("port", 0, "puerto HTTP (0 = uno libre)")
	noBrowser := flag.Bool("no-browser", false, "no abrir el navegador")
	flag.Parse()
	if err := run(*dir, *port, !*noBrowser); err != nil {
		log.Fatal(err)
	}
}

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
		log.Printf("config.json creado con raíces %v", cfg.Roots)
	}

	st, err := openStore(appDir, readOnly)
	if err != nil {
		return err
	}
	defer st.Close()

	srv := &server.Server{AppDir: appDir, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal}
	if readOnly {
		log.Print("el directorio de la app no es escribible: modo consulta")
	} else {
		srv.Scanner = &scan.Scanner{AppDir: appDir, Roots: cfg.Roots, Store: st}
		go func() {
			if err := srv.Scanner.Run(context.Background()); err != nil {
				log.Printf("escaneo: %v", err)
			}
		}()
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

// openStore opens the catalog. In read-only mode with no catalog file yet it
// falls back to an empty in-memory one.
func openStore(appDir string, readOnly bool) (*store.Store, error) {
	path := filepath.Join(appDir, "cinexplorer.db")
	if readOnly {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			return store.OpenMemory()
		}
	}
	return store.Open(path, readOnly)
}
```

- [ ] **Step 2: Compilar y correr todos los tests**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: todos los paquetes `ok`

- [ ] **Step 3: Smoke test sobre un árbol sintético**

```bash
tmp=$(mktemp -d)
mkdir -p "$tmp/app" "$tmp/cine/Stalker (1979)"
head -c 5000 /dev/urandom > "$tmp/cine/Stalker (1979)/stalker.mkv"
go build -o "$tmp/cx" ./cmd/cinexplorer
"$tmp/cx" -dir "$tmp/app" -port 8765 -no-browser &
sleep 3 && curl -s http://127.0.0.1:8765/api/versions; kill %1
cat "$tmp/app/config.json"
```
Expected: JSON con una versión `"title":"Stalker","year":1979`, y `config.json` con `"roots": ["../cine"]`.

- [ ] **Step 4: Commit**

```bash
git add cmd
git commit -m "feat: cinexplorer entry point"
```

---

### Task 13: build multiplataforma, CI y README

**Files:**
- Create: `scripts/build.sh`
- Create: `.github/workflows/ci.yml`
- Create: `README.md`

- [ ] **Step 1: Script de build**

`scripts/build.sh`:

```bash
#!/usr/bin/env bash
# Cross-compiles Cinexplorer for Windows, macOS (universal) and Linux into dist/cinexplorer/.
set -euo pipefail
cd "$(dirname "$0")/.."
out=dist/cinexplorer
mkdir -p "$out"
export CGO_ENABLED=0
build() { GOOS=$1 GOARCH=$2 go build -trimpath -ldflags "-s -w" -o "$out/$3" ./cmd/cinexplorer; }
build windows amd64 cinexplorer-windows.exe
build linux   amd64 cinexplorer-linux
build darwin  amd64 cinexplorer-macos-amd64
build darwin  arm64 cinexplorer-macos-arm64
go run github.com/randall77/makefat@latest "$out/cinexplorer-macos" "$out/cinexplorer-macos-amd64" "$out/cinexplorer-macos-arm64"
rm "$out/cinexplorer-macos-amd64" "$out/cinexplorer-macos-arm64"
ls -la "$out"
```

- [ ] **Step 2: Ejecutarlo**

Run: `bash scripts/build.sh`
Expected: `dist/cinexplorer/` con `cinexplorer-windows.exe`, `cinexplorer-linux`, `cinexplorer-macos`

- [ ] **Step 3: CI**

`.github/workflows/ci.yml`:

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go test ./...
```

- [ ] **Step 4: README**

`README.md`:

````markdown
# Cinexplorer

Explorador local de una colección de películas. Lee las carpetas configuradas
sin modificarlas y guarda su catálogo junto al ejecutable.

## Uso

1. Copiá la carpeta `cinexplorer/` (con los tres ejecutables) a la raíz del disco,
   al lado de tus carpetas de películas.
2. Ejecutá el binario de tu sistema:
   - Windows: `cinexplorer-windows.exe`
   - macOS: `cinexplorer-macos` (la primera vez: clic derecho → Abrir, porque no está firmado)
   - Linux: `./cinexplorer-linux`
3. Se abre el navegador. En el primer uso se crea `config.json` con las carpetas
   hermanas como raíces; editalo para cambiarlas.

Si la carpeta no se puede escribir (por ejemplo, un disco NTFS en macOS), la app
arranca en modo consulta con el catálogo existente.

## Desarrollo

```bash
go test ./...
go run ./cmd/cinexplorer -dir .run
bash scripts/build.sh
```
````

- [ ] **Step 5: Commit**

```bash
git add scripts .github README.md
git commit -m "chore: cross-platform build, CI and README"
```

---

### Task 14: Verificación con la colección real

No escribe nada en `D:\cine` ni en `D:\cine-ordenar`: la app solo escribe en `.run/` dentro del proyecto (ignorado por git).

- [ ] **Step 1: Configurar un directorio de app de desarrollo**

```bash
mkdir -p .run
printf '{\n  "roots": ["../../../cine", "../../../cine-ordenar"],\n  "tmdbToken": "",\n  "language": "es-ES"\n}\n' > .run/config.json
```

- [ ] **Step 2: Ejecutar**

Run: `go run ./cmd/cinexplorer -dir .run`
Expected: se abre el navegador; el estado muestra "Escaneando… N archivos" y luego "N versiones".

- [ ] **Step 3: Verificar a mano**

- `1900 (Novecento)` aparece como una versión de 2 partes con año 1976.
- Las carpetas de década (`1970s`) no se usan como título.
- "Copias idénticas" lista copias reales, si las hay.
- "▶ Ver" abre el reproductor predeterminado y "Carpeta" abre el Explorador con el archivo seleccionado.
- Un segundo arranque termina el escaneo en segundos (sin volver a calcular huellas).

- [ ] **Step 4: Alimentar el corpus**

Revisar la lista buscando títulos mal parseados. Por cada patrón nuevo, agregar un caso a `TestParseCorpus` (Task 5) que falle, corregir `parse.go` y hacer commit:

```bash
git add internal/nameparse
git commit -m "fix(nameparse): handle <patrón>"
```
