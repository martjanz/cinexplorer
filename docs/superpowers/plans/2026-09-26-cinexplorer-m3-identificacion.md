# Cinexplorer — Etapa 3: Identificación — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Asociar cada versión en disco a una película de TMDB (automáticamente cuando la confianza es alta, con cola de revisión cuando no), enriquecerla con TMDB y Wikidata, cachear imágenes y seguir funcionando sin red, con correcciones manuales atadas a la huella de contenido.

**Architecture:** Paquetes nuevos: `httpx` (límite + reintentos), `tmdb` y `wikidata` (clientes), `images` (caché en `cache/`) e `identify` (puntaje, búsqueda, `.nfo`, enriquecimiento y un `Runner` en segundo plano que el escáner dispara al terminar). `store` suma las tablas `movies` e `identifications` (clave: huella del archivo representativo de cada versión) y agrupa la mejor versión por id de TMDB. El servidor expone sin identificar, búsqueda manual, correcciones e imágenes; la página mínima muestra la identidad y permite confirmar candidatos.

**Tech Stack:** Go (sin dependencias nuevas), `modernc.org/sqlite`, TMDB API v3 (token v4), SPARQL de Wikidata.

Spec: `docs/superpowers/specs/2026-09-26-cinexplorer-m3-identificacion-design.md` (detalle de esta etapa) y `docs/superpowers/specs/2026-09-25-cinexplorer-design.md` §3.1, §4 pasos 5–6, §4.2.

**Nota sobre el código de este plan:** todo el código se prototipó y se probó antes de escribir el plan: suite completa en verde, compilación cruzada, corpus de 30 nombres reales contra TMDB y una corrida real sobre `D:\cine\1970s` con token (33 automáticas de 41, todas correctas). El fixture grande del corpus (`internal/identify/testdata/tmdb_corpus.json`, respuestas reales de TMDB recortadas) ya está commiteado en la rama junto con este plan. Copiá el código tal cual; si algo no compila o un test no da lo esperado, es un error del plan: reportalo en lugar de improvisar.

---

## Hoja de ruta (etapas)

| Etapa | Contenido | Estado |
|---|---|---|
| 1 — Núcleo local | escaneo, parser de nombres, versiones, SQLite, API mínima, build, CI | ✅ en `main` |
| 2 — Datos técnicos | lectores nativos MKV/MP4/AVI/IFO, fallback `ffprobe`, tabla `media`, mejor versión | ✅ en `main` |
| **3 — Identificación (este plan)** | TMDB + Wikidata, puntaje de confianza, películas, correcciones por huella, imágenes | |
| 4 — Interfaz | Svelte + Vite, Inicio, Explorar, Ficha, Revisar, búsqueda FTS5, primer uso | |
| 5 — Curaduría | listas, etiquetas, importación de `Collections/` | |

---

## Estructura de archivos (Etapa 3)

```
internal/httpx/httpx.go            Limiter (token bucket), Client.Get con reintentos, ErrOffline, StatusError
internal/tmdb/tmdb.go              SearchMovie, FindIMDb, Movie (con créditos), Image; ErrUnauthorized/ErrNotFound
internal/wikidata/wikidata.go      Lookup por lotes (P4947 → QID, P345, P495/P297, P57, P577)
internal/images/images.go          caché cache/posters y cache/backdrops, descarga única
internal/store/schema.sql          + tablas movies e identifications
internal/store/movies.go           Movie, SaveMovie, Movie(id), Movies, MoviesToEnrich, PendingWikidata, InvalidateMovie
internal/store/identity.go         representativos, IdentifyTargets, SaveIdentifications, correcciones, Unidentified, attachIdentity
internal/store/store.go            VersionView + fingerprint/identification/movie; hasIdentity
internal/store/technical.go        markBest agrupa por id de TMDB
internal/identify/score.go         similitud, año, director
internal/identify/match.go         Query, Search, Identify, umbrales, MatcherVersion
internal/identify/nfo.go           IMDb desde .nfo
internal/identify/enrich.go        TMDB → store.Movie, huecos desde Wikidata
internal/identify/runner.go        Runner: fases, estado, reintentos offline, Trigger, Adopt
internal/identify/testdata/tmdb_corpus.json  respuestas de TMDB grabadas (ya commiteado)
internal/fingerprint/fingerprint.go  archivos vacíos sin huella
internal/quality/quality.go        Words, NormTitle, más letras plegadas
internal/config/config.go          + imagePrefetch
internal/scan/scan.go              no reutiliza huellas de archivos vacíos; OnDone
internal/server/identify.go        /api/unidentified, /api/tmdb/search, /api/identify, /img/…
internal/server/server.go          campos nuevos, rutas, identify en /api/status
internal/server/web/index.html     identidad por versión, pestaña Sin identificar
cmd/cinexplorer/main.go            conexión de TMDB, Wikidata, imágenes y Runner
```

Dependencias nuevas entre paquetes: `tmdb → httpx`, `wikidata → httpx`, `identify → {httpx, tmdb, wikidata, images, store, quality, nameparse, appdir}`, `server → {identify, images, tmdb, httpx}`. `store` no depende de `tmdb` ni de `identify`. Sin ciclos.

Convenciones que ya usa el proyecto y hay que mantener: comentarios en inglés, mensajes de log y de UI en español, tests en el mismo paquete (`package x`, no `x_test`), commits en inglés con prefijo convencional.

En Windows con Git Bash, `go` debería estar en el PATH; si no: `export PATH="$PATH:/c/Program Files/Go/bin"`. Algunos archivos existentes están con CRLF en el checkout: los reemplazos de texto de este plan son sobre el contenido, no sobre los finales de línea; `gofmt` normaliza.

---
### Task 1: Normalización de títulos reutilizable (`quality`)

**Files:**
- Modify: `internal/quality/quality.go`
- Test: `internal/quality/quality_test.go`

`identify` compara títulos con la misma normalización que `GroupKey`. Se exponen `Words` y `NormTitle`, y el plegado de acentos suma letras de Europa del Este y del turco (`Polański` ↔ `Polanski`). `GroupKey` pasa a usar `NormTitle` sin cambiar su resultado.

- [ ] **Step 1: Escribir el test que falla**

Agregá al final de `internal/quality/quality_test.go`:

```go
func TestNormTitle(t *testing.T) {
	cases := map[string]string{
		"El Ángel Exterminador":          "angel exterminador",
		"The Good, the Bad and the Ugly": "good the bad and the ugly",
		"The":                            "the",
		"8½":                             "8",
		"":                               "",
	}
	for in, want := range cases {
		if got := NormTitle(in); got != want {
			t.Errorf("NormTitle(%q) = %q, want %q", in, got, want)
		}
	}
	if got := NormTitle("Łódź Żółć Ščř"); got != "lodz zolc scr" {
		t.Errorf("NormTitle = %q", got)
	}
	if got := Words("Wong Kar-wai"); len(got) != 3 || got[2] != "wai" {
		t.Errorf("Words = %q", got)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/quality/`
Expected: FAIL de compilación: `undefined: NormTitle` / `undefined: Words`.

- [ ] **Step 3: Implementar**

En `internal/quality/quality.go`, reemplazá:

```go
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c", "æ", "ae", "œ", "oe", "ß", "ss",
)

// GroupKey returns the provisional identity of a movie: its title folded to
// lowercase ASCII words without a leading article, plus the year. Versions
// with the same key are treated as the same movie until TMDB ids exist. An
// empty title yields "", which never groups.
func GroupKey(title string, year int) string {
	s := folds.Replace(strings.ToLower(title))
	words := strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if len(words) > 1 && articles[words[0]] {
		words = words[1:]
	}
	if len(words) == 0 {
		return ""
	}
	return strings.Join(words, " ") + "|" + strconv.Itoa(year)
}
```

por:

```go
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c", "æ", "ae", "œ", "oe", "ß", "ss",
	"ą", "a", "ę", "e", "ı", "i", "ý", "y", "ÿ", "y",
	"ć", "c", "č", "c", "ď", "d", "đ", "d", "ğ", "g", "ł", "l", "ń", "n", "ň", "n",
	"ř", "r", "ś", "s", "š", "s", "ş", "s", "ť", "t", "ź", "z", "ż", "z", "ž", "z",
)

// Words folds s to lowercase ASCII-ish words: accents removed, split on
// anything that is not a letter or digit.
func Words(s string) []string {
	s = folds.Replace(strings.ToLower(s))
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// NormTitle is a title reduced for comparison: folded words without a
// leading article, joined by single spaces. A lone article is kept.
func NormTitle(title string) string {
	words := Words(title)
	if len(words) > 1 && articles[words[0]] {
		words = words[1:]
	}
	return strings.Join(words, " ")
}

// GroupKey returns the provisional identity of a movie: its normalized title
// plus the year. Versions with the same key are treated as the same movie
// when they have no TMDB id. An empty title yields "", which never groups.
func GroupKey(title string, year int) string {
	t := NormTitle(title)
	if t == "" {
		return ""
	}
	return t + "|" + strconv.Itoa(year)
}
```


- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/quality/`
Expected: `ok  cinexplorer/internal/quality`

- [ ] **Step 5: Commit**

```bash
git add internal/quality
git commit -m "feat(quality): expose title normalization, fold more accents

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Archivos vacíos sin huella

**Files:**
- Modify: `internal/fingerprint/fingerprint.go`
- Modify: `internal/scan/scan.go`
- Test: `internal/fingerprint/fingerprint_test.go`
- Test: `internal/scan/scan_test.go`

Hay 32 videos de 0 bytes en la colección real: todos tenían la misma huella, así que figuraban como copias idénticas y una identificación se contagiaría entre ellos. `fingerprint.Of` devuelve `""` para un archivo vacío, y el escáner deja de reutilizar la huella guardada de un archivo vacío (para corregir catálogos existentes). Ojo: la condición va en la reutilización y no antes del cálculo, porque `TestScanPreservesUnreadableFiles` usa un symlink colgante que se ve con tamaño 0 y debe conservar su huella anterior.

- [ ] **Step 1: Escribir los tests que fallan**

Agregá al final de `internal/fingerprint/fingerprint_test.go`:

```go
func TestEmptyFileHasNoFingerprint(t *testing.T) {
	p := filepath.Join(t.TempDir(), "empty.avi")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if fp, err := Of(p); err != nil || fp != "" {
		t.Fatalf("got %q, %v", fp, err)
	}
}
```

Agregá al final de `internal/scan/scan_test.go`:

```go
func TestScanForgetsFingerprintOfEmptyFiles(t *testing.T) {
	disk, app, st := setup(t)
	writeFile(t, filepath.Join(disk, "cine", "A", "a.avi"), 0, 0)
	writeFile(t, filepath.Join(disk, "cine", "B", "b.avi"), 0, 0)
	// An older catalog gave both the same fingerprint.
	info, _ := os.Stat(filepath.Join(disk, "cine", "A", "a.avi"))
	infoB, _ := os.Stat(filepath.Join(disk, "cine", "B", "b.avi"))
	st.SyncFiles([]store.FileRow{
		{Path: "../cine/A/a.avi", Size: 0, MTime: info.ModTime().UnixMilli(), Fingerprint: "e3b0", Kind: "video"},
		{Path: "../cine/B/b.avi", Size: 0, MTime: infoB.ModTime().UnixMilli(), Fingerprint: "e3b0", Kind: "video"},
	}, []string{"../cine"})
	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx, _ := st.FileIndex()
	if idx["../cine/A/a.avi"].Fingerprint != "" || idx["../cine/B/b.avi"].Fingerprint != "" {
		t.Fatalf("index %+v", idx)
	}
	if d, _ := st.Duplicates(); len(d) != 0 {
		t.Fatalf("empty files reported as copies: %+v", d)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/fingerprint/ ./internal/scan/`
Expected: FAIL: `got "e3b0c442…"` en fingerprint y huellas no vacías / copias reportadas en scan.

- [ ] **Step 3: Implementar**

En `internal/fingerprint/fingerprint.go`, reemplazá:

```go

// Of hashes the first and last MiB plus the file size. Identical files always
// match, and a moved or renamed file keeps its fingerprint.
func Of(path string) (string, error) {
	f, err := os.Open(path)
```

por:

```go

// Of hashes the first and last MiB plus the file size. Identical files always
// match, and a moved or renamed file keeps its fingerprint. An empty file
// (a failed copy, a placeholder) has no content to recognize: its
// fingerprint is "".
func Of(path string) (string, error) {
	f, err := os.Open(path)
```

En `internal/fingerprint/fingerprint.go`, reemplazá:

```go
	}
	size := st.Size()
	h := sha256.New()
	if size <= 2*chunk {
```

por:

```go
	}
	size := st.Size()
	if size == 0 {
		return "", nil
	}
	h := sha256.New()
	if size <= 2*chunk {
```

En `internal/scan/scan.go`, reemplazá:

```go
			known_, wasKnown := known[rel]
			hashed := int64(0)
			if wasKnown && known_.Size == row.Size && known_.MTime == row.MTime && (known_.Fingerprint != "" || !kind.Fingerprinted()) {
				row.Fingerprint = known_.Fingerprint
			} else if kind.Fingerprinted() {
```

por:

```go
			known_, wasKnown := known[rel]
			hashed := int64(0)
			// Empty files are fingerprinted again: catalogs made before
			// fingerprint.Of skipped them hold a shared, meaningless one.
			if wasKnown && row.Size > 0 && known_.Size == row.Size && known_.MTime == row.MTime && (known_.Fingerprint != "" || !kind.Fingerprinted()) {
				row.Fingerprint = known_.Fingerprint
			} else if kind.Fingerprinted() {
```


- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/fingerprint/ ./internal/scan/`
Expected: ambos `ok` (incluido `TestScanPreservesUnreadableFiles`).

- [ ] **Step 5: Commit**

```bash
git add internal/fingerprint internal/scan
git commit -m "fix: empty files get no fingerprint

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Cliente HTTP con límite y reintentos (`internal/httpx`)

**Files:**
- Create: `internal/httpx/httpx.go`
- Test: `internal/httpx/httpx_test.go`

Plomería compartida por TMDB y Wikidata: `Limiter` (token bucket) y `Client.Get` (reintentos con backoff exponencial y jitter ante 429/5xx/red, respeta `Retry-After`, `*StatusError` para los demás códigos, `ErrOffline` al agotar intentos).

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/httpx/httpx_test.go`:

```go
package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fastClient() *Client { return &Client{Backoff: time.Millisecond} }

func TestGetRetriesThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(http.StatusBadGateway)
		default:
			w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	body, err := fastClient().Get(context.Background(), srv.URL)
	if err != nil || string(body) != "ok" || calls.Load() != 3 {
		t.Fatalf("body %q err %v calls %d", body, err, calls.Load())
	}
}

func TestGetGivesUpAsOffline(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := fastClient()
	c.Attempts = 3
	_, err := c.Get(context.Background(), srv.URL)
	if !errors.Is(err, ErrOffline) || calls.Load() != 3 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}

func TestGetNetworkErrorIsOffline(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens there any more
	c := fastClient()
	c.Attempts = 2
	if _, err := c.Get(context.Background(), url); !errors.Is(err, ErrOffline) {
		t.Fatalf("err %v", err)
	}
}

func TestGetFinalStatus(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := fastClient().Get(context.Background(), srv.URL)
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 401 || calls.Load() != 1 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}

func TestGetHonoursRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	start := time.Now()
	if _, err := fastClient().Get(context.Background(), srv.URL); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < time.Second {
		t.Fatalf("retried after %v, want ≥ 1 s", d)
	}
}

func TestGetSendsHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.Header.Get("Authorization")))
	}))
	defer srv.Close()
	c := fastClient()
	c.Header = http.Header{"Authorization": {"Bearer x"}}
	if body, _ := c.Get(context.Background(), srv.URL); string(body) != "Bearer x" {
		t.Fatalf("body %q", body)
	}
}

func TestLimiter(t *testing.T) {
	l := &Limiter{Rate: 100, Burst: 2}
	start := time.Now()
	for range 6 {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	// 2 immediate, then 4 at 10 ms each.
	if d := time.Since(start); d < 35*time.Millisecond {
		t.Fatalf("6 tokens in %v", d)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	slow := &Limiter{Rate: 0.001, Burst: 1}
	slow.Wait(context.Background())
	if err := slow.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/httpx/`
Expected: FAIL de compilación (paquete sin código).

- [ ] **Step 3: Implementar**

Crear `internal/httpx/httpx.go`:

```go
// Package httpx is the HTTP plumbing shared by the metadata clients: a
// token-bucket rate limiter and GET requests with retries and backoff.
package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ErrOffline means the service could not be reached (network down, DNS,
// timeouts, or repeated 429/5xx): the work should be retried later.
var ErrOffline = errors.New("sin conexión")

// StatusError is a final (non-retried) HTTP error status.
type StatusError struct {
	Code int
	URL  string
}

func (e *StatusError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Code, e.URL) }

// Limiter is a token bucket: Rate tokens per second, holding at most Burst.
type Limiter struct {
	Rate  float64
	Burst float64

	mu     sync.Mutex
	tokens float64
	last   time.Time
	init   bool
}

// Wait blocks until a token is available or ctx ends.
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		if !l.init {
			l.tokens, l.last, l.init = l.Burst, now, true
		}
		l.tokens = min(l.Burst, l.tokens+now.Sub(l.last).Seconds()*l.Rate)
		l.last = now
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		wait := time.Duration((1 - l.tokens) / l.Rate * float64(time.Second))
		l.mu.Unlock()
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// Client performs rate-limited GET requests with retries.
type Client struct {
	HTTP     *http.Client
	Limiter  *Limiter
	Attempts int           // total tries; 0 means 5
	Backoff  time.Duration // first retry delay, doubled each time; 0 means 1 s
	Header   http.Header   // added to every request
}

// maxBody caps a response so a misbehaving server cannot exhaust memory.
const maxBody = 32 << 20

// Get fetches url and returns the body of a 2xx response. 429, 5xx and
// network errors are retried with exponential backoff (honouring
// Retry-After); when the attempts run out the error wraps ErrOffline. Other
// statuses return a *StatusError right away.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	attempts, backoff := c.Attempts, c.Backoff
	if attempts == 0 {
		attempts = 5
	}
	if backoff == 0 {
		backoff = time.Second
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	var last error
	for i := range attempts {
		if i > 0 {
			d := backoff << (i - 1)
			var ra *retryAfter
			if errors.As(last, &ra) && ra.d > d {
				d = ra.d
			}
			d += time.Duration(rand.Int64N(int64(d)/4 + 1))
			t := time.NewTimer(d)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, ctx.Err()
			case <-t.C:
			}
		}
		if c.Limiter != nil {
			if err := c.Limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}
		body, err := c.once(ctx, hc, url)
		if err == nil {
			return body, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var se *StatusError
		if errors.As(err, &se) {
			return nil, err
		}
		last = err
	}
	return nil, fmt.Errorf("%w: %v", ErrOffline, last)
}

// retryAfter is a retryable status, with the delay the server asked for.
type retryAfter struct {
	code int
	d    time.Duration
}

func (e *retryAfter) Error() string { return "HTTP " + strconv.Itoa(e.code) }

func (c *Client) once(ctx context.Context, hc *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, vs := range c.Header {
		req.Header[k] = vs
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		secs, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		return nil, &retryAfter{code: resp.StatusCode, d: time.Duration(secs) * time.Second}
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, &StatusError{Code: resp.StatusCode, URL: url}
	}
	return body, err
}
```

- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/httpx/`
Expected: `ok` (tarda ~2 s por el test de `Retry-After`).

- [ ] **Step 5: Commit**

```bash
git add internal/httpx
git commit -m "feat(httpx): rate-limited GET with retries

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Cliente TMDB (`internal/tmdb`)

**Files:**
- Create: `internal/tmdb/tmdb.go`
- Test: `internal/tmdb/tmdb_test.go`

Búsqueda, find por IMDb, detalles con créditos (`append_to_response=credits`) e imágenes. 20 req/s con ráfaga 10. Errores: `ErrOffline` (alias de `httpx.ErrOffline`), `ErrUnauthorized` (401), `ErrNotFound` (404).

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/tmdb/tmdb_test.go`:

```go
package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New("secret")
	c.BaseURL, c.ImageURL = srv.URL+"/3", srv.URL+"/img"
	c.HTTP.Backoff = time.Millisecond
	c.HTTP.Attempts = 2
	return c
}

func TestSearchMovie(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/search/movie" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("request %s %v", r.URL, r.Header)
		}
		q := r.URL.Query()
		if q.Get("query") != "Amarcord" || q.Get("year") != "1973" || q.Get("language") != "es-ES" {
			t.Errorf("query %v", q)
		}
		w.Write([]byte(`{"page":1,"results":[{"id":7857,"title":"Amarcord","original_title":"Amarcord","release_date":"1973-12-18","poster_path":"/a.jpg"}]}`))
	})
	rs, err := c.SearchMovie(context.Background(), "Amarcord", 1973, "es-ES")
	if err != nil || len(rs) != 1 || rs[0].ID != 7857 || rs[0].Year() != 1973 || rs[0].PosterPath != "/a.jpg" {
		t.Fatalf("got %+v, %v", rs, err)
	}
}

func TestSearchWithoutYear(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("year") {
			t.Errorf("year sent: %v", r.URL.Query())
		}
		w.Write([]byte(`{"results":[]}`))
	})
	if rs, err := c.SearchMovie(context.Background(), "X", 0, "es-ES"); err != nil || len(rs) != 0 {
		t.Fatalf("got %+v, %v", rs, err)
	}
}

func TestFindIMDb(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/find/tt0071129" || r.URL.Query().Get("external_source") != "imdb_id" {
			t.Errorf("request %s", r.URL)
		}
		w.Write([]byte(`{"movie_results":[{"id":7857,"title":"Amarcord","release_date":"1973-12-18"}],"tv_results":[]}`))
	})
	rs, err := c.FindIMDb(context.Background(), "tt0071129", "es-ES")
	if err != nil || len(rs) != 1 || rs[0].ID != 7857 {
		t.Fatalf("got %+v, %v", rs, err)
	}
}

func TestMovie(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/movie/7857" || r.URL.Query().Get("append_to_response") != "credits" {
			t.Errorf("request %s", r.URL)
		}
		w.Write([]byte(`{"id":7857,"title":"Amarcord","original_title":"Amarcord","release_date":"1973-12-18",
			"runtime":123,"original_language":"it","overview":"Rimini.","imdb_id":"tt0071129",
			"poster_path":"/p.jpg","backdrop_path":"/b.jpg","genres":[{"id":35,"name":"Comedia"}],
			"production_countries":[{"iso_3166_1":"IT","name":"Italy"},{"iso_3166_1":"FR","name":"France"}],
			"belongs_to_collection":null,
			"credits":{"cast":[{"id":1,"name":"Magali Noël","character":"Gradisca","order":0}],
			"crew":[{"id":4415,"name":"Federico Fellini","job":"Director"},{"id":4415,"name":"Federico Fellini","job":"Director"},
			{"id":9,"name":"Tonino Guerra","job":"Screenplay"}]}}`))
	})
	d, err := c.Movie(context.Background(), 7857, "es-ES")
	if err != nil {
		t.Fatal(err)
	}
	if d.Year() != 1973 || d.Runtime != 123 || d.IMDbID != "tt0071129" || len(d.Genres) != 1 ||
		len(d.ProductionCountries) != 2 || d.Collection != nil || len(d.Credits.Cast) != 1 {
		t.Fatalf("got %+v", d)
	}
	if ds := d.Directors(); len(ds) != 1 || ds[0].Name != "Federico Fellini" {
		t.Fatalf("directors %+v", ds)
	}
}

func TestErrors(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/movie/1":
			w.WriteHeader(http.StatusNotFound)
		case "/3/movie/2":
			w.WriteHeader(http.StatusUnauthorized)
		case "/3/movie/3":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.Write([]byte(`not json`))
		}
	})
	ctx := context.Background()
	if _, err := c.Movie(ctx, 1, "es-ES"); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	if _, err := c.Movie(ctx, 2, "es-ES"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("401: %v", err)
	}
	if _, err := c.Movie(ctx, 3, "es-ES"); !errors.Is(err, ErrOffline) {
		t.Errorf("503: %v", err)
	}
	if _, err := c.Movie(ctx, 4, "es-ES"); err == nil || errors.Is(err, ErrOffline) {
		t.Errorf("bad JSON: %v", err)
	}
}

func TestImage(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/img/w342/p.jpg" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Write([]byte("JPEG"))
	})
	if b, err := c.Image(context.Background(), "/p.jpg", "w342"); err != nil || string(b) != "JPEG" {
		t.Fatalf("got %q, %v", b, err)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/tmdb/`
Expected: FAIL de compilación.

- [ ] **Step 3: Implementar**

Crear `internal/tmdb/tmdb.go`:

```go
// Package tmdb is a small client for The Movie Database API v3.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"cinexplorer/internal/httpx"
)

var (
	ErrOffline      = httpx.ErrOffline
	ErrUnauthorized = errors.New("token de TMDB inválido")
	ErrNotFound     = errors.New("no existe en TMDB")
)

type Client struct {
	BaseURL  string // "https://api.themoviedb.org/3"
	ImageURL string // "https://image.tmdb.org/t/p"
	HTTP     *httpx.Client
}

// New returns a client authenticated with a v4 read access token, limited to
// 20 requests per second (TMDB allows about 50).
func New(token string) *Client {
	return &Client{
		BaseURL:  "https://api.themoviedb.org/3",
		ImageURL: "https://image.tmdb.org/t/p",
		HTTP: &httpx.Client{
			Limiter: &httpx.Limiter{Rate: 20, Burst: 10},
			Header:  http.Header{"Authorization": {"Bearer " + token}, "Accept": {"application/json"}},
		},
	}
}

// Result is a movie as listed by search and find.
type Result struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	ReleaseDate   string `json:"release_date"`
	PosterPath    string `json:"poster_path"`
}

func (r Result) Year() int { return year(r.ReleaseDate) }

type Person struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Job       string `json:"job"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

// Details is a movie with its credits (append_to_response=credits).
type Details struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	OriginalTitle    string `json:"original_title"`
	ReleaseDate      string `json:"release_date"`
	Runtime          int    `json:"runtime"`
	OriginalLanguage string `json:"original_language"`
	Overview         string `json:"overview"`
	IMDbID           string `json:"imdb_id"`
	PosterPath       string `json:"poster_path"`
	BackdropPath     string `json:"backdrop_path"`
	Genres           []struct {
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCountries []struct {
		ISO string `json:"iso_3166_1"`
	} `json:"production_countries"`
	Collection *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"belongs_to_collection"`
	Credits struct {
		Cast []Person `json:"cast"`
		Crew []Person `json:"crew"`
	} `json:"credits"`
}

func (d Details) Year() int { return year(d.ReleaseDate) }

// Directors returns the crew members with job "Director", without repeats.
func (d Details) Directors() []Person {
	var out []Person
	seen := map[int]bool{}
	for _, p := range d.Credits.Crew {
		if p.Job == "Director" && !seen[p.ID] {
			seen[p.ID] = true
			out = append(out, p)
		}
	}
	return out
}

func year(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

// SearchMovie searches by title; year 0 means any year.
func (c *Client) SearchMovie(ctx context.Context, query string, year int, lang string) ([]Result, error) {
	v := url.Values{"query": {query}, "language": {lang}, "include_adult": {"false"}}
	if year > 0 {
		v.Set("year", strconv.Itoa(year))
	}
	var out struct {
		Results []Result `json:"results"`
	}
	err := c.get(ctx, "/search/movie?"+v.Encode(), &out)
	return out.Results, err
}

// FindIMDb looks a movie up by its IMDb id ("tt0071129").
func (c *Client) FindIMDb(ctx context.Context, imdbID, lang string) ([]Result, error) {
	v := url.Values{"external_source": {"imdb_id"}, "language": {lang}}
	var out struct {
		Results []Result `json:"movie_results"`
	}
	err := c.get(ctx, "/find/"+url.PathEscape(imdbID)+"?"+v.Encode(), &out)
	return out.Results, err
}

// Movie returns a movie's details and credits.
func (c *Client) Movie(ctx context.Context, id int, lang string) (Details, error) {
	v := url.Values{"language": {lang}, "append_to_response": {"credits"}}
	var d Details
	err := c.get(ctx, "/movie/"+strconv.Itoa(id)+"?"+v.Encode(), &d)
	return d, err
}

// Image downloads an image by its TMDB path ("/abc.jpg") at a size ("w342").
func (c *Client) Image(ctx context.Context, path, size string) ([]byte, error) {
	b, err := c.HTTP.Get(ctx, c.ImageURL+"/"+size+path)
	return b, mapErr(err)
}

func (c *Client) get(ctx context.Context, pathQuery string, out any) error {
	b, err := c.HTTP.Get(ctx, c.BaseURL+pathQuery)
	if err != nil {
		return mapErr(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("tmdb %s: %w", pathQuery, err)
	}
	return nil
}

func mapErr(err error) error {
	var se *httpx.StatusError
	if errors.As(err, &se) {
		switch se.Code {
		case http.StatusUnauthorized:
			return ErrUnauthorized
		case http.StatusNotFound:
			return ErrNotFound
		}
	}
	return err
}
```

- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/tmdb/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/tmdb
git commit -m "feat(tmdb): TMDB API client

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Cliente Wikidata (`internal/wikidata`)

**Files:**
- Create: `internal/wikidata/wikidata.go`
- Create: `internal/wikidata/testdata/lookup.json`
- Test: `internal/wikidata/wikidata_test.go`

Consulta SPARQL por lotes de hasta 50 ids de TMDB (P4947) que devuelve QID, IMDb, países ISO, directores (etiqueta en el idioma, fallback inglés) y año más temprano. `lookup.json` es una respuesta real de `query.wikidata.org` (Amarcord 7857, Fight Club 550 y un id inexistente), reducida a los campos `value`.

- [ ] **Step 1: Escribir el fixture y los tests que fallan**

Crear `internal/wikidata/testdata/lookup.json`:

```json
{"results":{"bindings":[{"item":{"value":"http://www.wikidata.org/entity/Q18428"},"countryIso":{"value":"IT"},"tmdb":{"value":"7857"},"imdb":{"value":"tt0071129"},"directorLabel":{"value":"Federico Fellini"},"date":{"value":"1973-12-18T00:00:00Z"}},
{"item":{"value":"http://www.wikidata.org/entity/Q18428"},"countryIso":{"value":"FR"},"tmdb":{"value":"7857"},"imdb":{"value":"tt0071129"},"directorLabel":{"value":"Federico Fellini"},"date":{"value":"1973-12-18T00:00:00Z"}},
{"item":{"value":"http://www.wikidata.org/entity/Q18428"},"countryIso":{"value":"IT"},"tmdb":{"value":"7857"},"imdb":{"value":"tt0071129"},"directorLabel":{"value":"Federico Fellini"},"date":{"value":"1974-03-22T00:00:00Z"}},
{"item":{"value":"http://www.wikidata.org/entity/Q18428"},"countryIso":{"value":"FR"},"tmdb":{"value":"7857"},"imdb":{"value":"tt0071129"},"directorLabel":{"value":"Federico Fellini"},"date":{"value":"1974-03-22T00:00:00Z"}},
{"item":{"value":"http://www.wikidata.org/entity/Q190050"},"countryIso":{"value":"US"},"tmdb":{"value":"550"},"imdb":{"value":"tt0137523"},"directorLabel":{"value":"David Fincher"},"date":{"value":"1999-09-10T00:00:00Z"}},
{"item":{"value":"http://www.wikidata.org/entity/Q190050"},"countryIso":{"value":"DE"},"tmdb":{"value":"550"},"imdb":{"value":"tt0137523"},"directorLabel":{"value":"David Fincher"},"date":{"value":"1999-09-10T00:00:00Z"}},
{"item":{"value":"http://www.wikidata.org/entity/Q190050"},"countryIso":{"value":"US"},"tmdb":{"value":"550"},"imdb":{"value":"tt0137523"},"directorLabel":{"value":"David Fincher"},"date":{"value":"1999-10-15T00:00:00Z"}},
{"item":{"value":"http://www.wikidata.org/entity/Q190050"},"countryIso":{"value":"DE"},"tmdb":{"value":"550"},"imdb":{"value":"tt0137523"},"directorLabel":{"value":"David Fincher"},"date":{"value":"1999-10-15T00:00:00Z"}},
{"item":{"value":"http://www.wikidata.org/entity/Q190050"},"countryIso":{"value":"US"},"tmdb":{"value":"550"},"imdb":{"value":"tt0137523"},"directorLabel":{"value":"David Fincher"},"date":{"value":"1999-11-11T00:00:00Z"}},
{"item":{"value":"http://www.wikidata.org/entity/Q190050"},"countryIso":{"value":"DE"},"tmdb":{"value":"550"},"imdb":{"value":"tt0137523"},"directorLabel":{"value":"David Fincher"},"date":{"value":"1999-11-11T00:00:00Z"}}]}}
```

Crear `internal/wikidata/wikidata_test.go`:

```go
package wikidata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLookup(t *testing.T) {
	fixture, err := os.ReadFile("testdata/lookup.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		if !strings.Contains(q, `VALUES ?tmdb { "7857" "550" "999999999" }`) || !strings.Contains(q, `wikibase:language "es,en"`) {
			t.Errorf("query %s", q)
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "cinexplorer/test (") {
			t.Errorf("User-Agent %q", r.Header.Get("User-Agent"))
		}
		w.Write(fixture)
	}))
	defer srv.Close()
	c := New("test")
	c.Endpoint = srv.URL
	c.HTTP.Limiter = nil
	c.HTTP.Backoff = time.Millisecond

	got, err := c.Lookup(context.Background(), []int{7857, 550, 999999999}, "es-ES")
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]Entity{
		7857: {QID: "Q18428", IMDbID: "tt0071129", Countries: []string{"FR", "IT"}, Directors: []string{"Federico Fellini"}, Year: 1973},
		550:  {QID: "Q190050", IMDbID: "tt0137523", Countries: []string{"DE", "US"}, Directors: []string{"David Fincher"}, Year: 1999},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestLookupLimits(t *testing.T) {
	c := New("test")
	if got, err := c.Lookup(context.Background(), nil, "es-ES"); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
	if _, err := c.Lookup(context.Background(), make([]int, BatchSize+1), "es-ES"); err == nil {
		t.Fatal("oversized batch accepted")
	}
}

func TestParseSkipsUnlabelledDirectors(t *testing.T) {
	got, err := parse([]byte(`{"results":{"bindings":[{"tmdb":{"value":"1"},"item":{"value":"http://www.wikidata.org/entity/Q1"},"directorLabel":{"value":"Q999"}}]}}`))
	if err != nil || len(got[1].Directors) != 0 || got[1].QID != "Q1" {
		t.Fatalf("got %+v, %v", got, err)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/wikidata/`
Expected: FAIL de compilación.

- [ ] **Step 3: Implementar**

Crear `internal/wikidata/wikidata.go`:

```go
// Package wikidata looks movies up in Wikidata by their TMDB id (P4947) to
// fill gaps TMDB leaves: IMDb id, countries, directors and year.
package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"cinexplorer/internal/httpx"
)

// BatchSize is the most TMDB ids Lookup accepts per call.
const BatchSize = 50

type Client struct {
	Endpoint string // "https://query.wikidata.org/sparql"
	HTTP     *httpx.Client
}

// New returns a client that identifies itself as Wikimedia asks, limited to
// one request per second.
func New(version string) *Client {
	return &Client{
		Endpoint: "https://query.wikidata.org/sparql",
		HTTP: &httpx.Client{
			Limiter: &httpx.Limiter{Rate: 1, Burst: 1},
			Header: http.Header{
				"Accept":     {"application/sparql-results+json"},
				"User-Agent": {"cinexplorer/" + version + " (https://github.com/martjanz/cinexplorer)"},
			},
		},
	}
}

// Entity is what Wikidata knows about one movie.
type Entity struct {
	QID       string   // "Q18428"
	IMDbID    string   // "tt0071129"
	Countries []string // ISO 3166-1 alpha-2, sorted
	Directors []string // labels in the requested language (fallback English), sorted
	Year      int      // earliest publication year
}

// Lookup returns the entities for up to BatchSize TMDB ids, keyed by TMDB id.
// Ids Wikidata does not know are absent. lang is a TMDB-style language
// ("es-ES"); labels use its first part with English as fallback.
func (c *Client) Lookup(ctx context.Context, ids []int, lang string) (map[int]Entity, error) {
	if len(ids) == 0 {
		return map[int]Entity{}, nil
	}
	if len(ids) > BatchSize {
		return nil, fmt.Errorf("wikidata: %d ids, max %d", len(ids), BatchSize)
	}
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = strconv.Quote(strconv.Itoa(id))
	}
	labelLang := strings.ToLower(strings.SplitN(lang, "-", 2)[0])
	if labelLang == "" {
		labelLang = "en"
	}
	q := `SELECT ?tmdb ?item ?imdb ?countryIso ?directorLabel ?date WHERE {
  VALUES ?tmdb { ` + strings.Join(values, " ") + ` }
  ?item wdt:P4947 ?tmdb .
  OPTIONAL { ?item wdt:P345 ?imdb . }
  OPTIONAL { ?item wdt:P495 ?country . ?country wdt:P297 ?countryIso . }
  OPTIONAL { ?item wdt:P57 ?director . }
  OPTIONAL { ?item wdt:P577 ?date . }
  SERVICE wikibase:label { bd:serviceParam wikibase:language "` + labelLang + `,en". }
}`
	body, err := c.HTTP.Get(ctx, c.Endpoint+"?"+url.Values{"query": {q}}.Encode())
	if err != nil {
		return nil, err
	}
	return parse(body)
}

type binding map[string]struct {
	Value string `json:"value"`
}

func parse(body []byte) (map[int]Entity, error) {
	var res struct {
		Results struct {
			Bindings []binding `json:"bindings"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("wikidata: %w", err)
	}
	type acc struct {
		e         Entity
		countries map[string]bool
		directors map[string]bool
	}
	byID := map[int]*acc{}
	for _, b := range res.Results.Bindings {
		id, err := strconv.Atoi(b["tmdb"].Value)
		if err != nil {
			continue
		}
		a := byID[id]
		if a == nil {
			a = &acc{countries: map[string]bool{}, directors: map[string]bool{}}
			a.e.QID = strings.TrimPrefix(b["item"].Value, "http://www.wikidata.org/entity/")
			byID[id] = a
		}
		if v := b["imdb"].Value; strings.HasPrefix(v, "tt") && a.e.IMDbID == "" {
			a.e.IMDbID = v
		}
		if v := b["countryIso"].Value; len(v) == 2 {
			a.countries[strings.ToUpper(v)] = true
		}
		// An unlabelled director comes back as its QID: useless as a name.
		if v := b["directorLabel"].Value; v != "" && !isQID(v) {
			a.directors[v] = true
		}
		if v := b["date"].Value; len(v) >= 4 {
			if y, err := strconv.Atoi(v[:4]); err == nil && y > 1800 && (a.e.Year == 0 || y < a.e.Year) {
				a.e.Year = y
			}
		}
	}
	out := make(map[int]Entity, len(byID))
	for id, a := range byID {
		a.e.Countries = keys(a.countries)
		a.e.Directors = keys(a.directors)
		out[id] = a.e
	}
	return out, nil
}

func isQID(s string) bool {
	if len(s) < 2 || s[0] != 'Q' {
		return false
	}
	_, err := strconv.Atoi(s[1:])
	return err == nil
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/wikidata/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/wikidata
git commit -m "feat(wikidata): look movies up by TMDB id

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Caché de imágenes (`internal/images`)

**Files:**
- Create: `internal/images/images.go`
- Test: `internal/images/images_test.go`

Afiches (w342) y escenas (w1280) en `cache/posters/<id>.jpg` y `cache/backdrops/<id>.jpg`, descargados una vez (escritura a `.tmp` + rename). Sin fetcher, sin red o con una ruta de TMDB inválida → `ErrUnavailable`. En modo consulta sirve sin guardar.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/images/images_test.go`:

```go
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
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/images/`
Expected: FAIL de compilación.

- [ ] **Step 3: Implementar**

Crear `internal/images/images.go`:

```go
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
```

- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/images/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/images
git commit -m "feat(images): on-demand TMDB image cache

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Store: películas, identificaciones y correcciones

**Files:**
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/store.go`
- Modify: `internal/store/technical.go`
- Create: `internal/store/identity.go`
- Create: `internal/store/movies.go`
- Test: `internal/store/identity_test.go`
- Test: `internal/store/versions_identity_test.go`

Tablas `movies` e `identifications` (spec §4). La clave es la huella del **representativo** de cada versión: el principal presente con huella de menor parte y, a igualdad, el más grande (el VOB más grande de un DVD). `SaveIdentifications` nunca pisa una corrección (`ON CONFLICT … WHERE status IN ('auto','unmatched')`). `Versions()` suma `fingerprint`, `identification` y `movie`, y `markBest` agrupa por id de TMDB cuando hay película. Un catálogo anterior abierto en modo consulta (sin las tablas) funciona con identidad vacía (`hasIdentity`).

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/store/identity_test.go`:

```go
package store

import (
	"errors"
	"reflect"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
)

// identityCatalog has: a two-part movie, a DVD, an identical copy of the
// two-part movie's first part in another folder, and a .nfo next to the DVD.
func identityCatalog(t *testing.T) *Store {
	t.Helper()
	s := open(t)
	if err := s.SyncFiles([]FileRow{
		{Path: "../cine/a/Amarcord CD1.avi", Size: 700, MTime: 1, Fingerprint: "a1", Kind: "video"},
		{Path: "../cine/a/Amarcord CD2.avi", Size: 710, MTime: 1, Fingerprint: "a2", Kind: "video"},
		{Path: "../cine/copy/Amarcord.avi", Size: 700, MTime: 1, Fingerprint: "a1", Kind: "video"},
		{Path: "../cine/d/VIDEO_TS/VTS_01_0.IFO", Size: 10, MTime: 1, Fingerprint: "ifo", Kind: "dvd"},
		{Path: "../cine/d/VIDEO_TS/VTS_01_1.VOB", Size: 900, MTime: 1, Fingerprint: "vob1", Kind: "dvd"},
		{Path: "../cine/d/VIDEO_TS/VTS_01_2.VOB", Size: 950, MTime: 1, Fingerprint: "vob2", Kind: "dvd"},
		{Path: "../cine/d/movie.nfo", Size: 1, MTime: 1, Kind: "info"},
		{Path: "../cine/d/b.nfo", Size: 1, MTime: 1, Kind: "info"},
	}, roots); err != nil {
		t.Fatal(err)
	}
	main := func(p string, part int) grouping.Member {
		return grouping.Member{Path: p, Role: grouping.RoleMain, Part: part}
	}
	if err := s.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/a", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973, Director: "Fellini"}, Size: 1410, Parts: 2,
			Members: []grouping.Member{main("../cine/a/Amarcord CD1.avi", 1), main("../cine/a/Amarcord CD2.avi", 2)}},
		{Dir: "../cine/copy", Parsed: nameparse.Parsed{Title: "Amarcord"}, Size: 700, Parts: 1,
			Members: []grouping.Member{main("../cine/copy/Amarcord.avi", 0)}},
		{Dir: "../cine/d", Parsed: nameparse.Parsed{Title: "Stalker", IMDbID: "tt0079944"}, Size: 1860, Parts: 1,
			Members: []grouping.Member{main("../cine/d/VIDEO_TS/VTS_01_0.IFO", 0), main("../cine/d/VIDEO_TS/VTS_01_1.VOB", 0),
				main("../cine/d/VIDEO_TS/VTS_01_2.VOB", 0)}},
	}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIdentifyTargets(t *testing.T) {
	s := identityCatalog(t)
	ts, err := s.IdentifyTargets()
	if err != nil {
		t.Fatal(err)
	}
	want := []IdentifyTarget{
		{Fingerprint: "a1", Dir: "../cine/a", Title: "Amarcord", Year: 1973, Director: "Fellini"},
		{Fingerprint: "vob2", Dir: "../cine/d", Title: "Stalker", IMDbID: "tt0079944", NFOs: []string{"../cine/d/b.nfo", "../cine/d/movie.nfo"}},
	}
	if !reflect.DeepEqual(ts, want) {
		t.Fatalf("got %+v\nwant %+v", ts, want)
	}
}

func TestSaveIdentificationsKeepsCorrections(t *testing.T) {
	s := identityCatalog(t)
	auto := Identification{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857, Confidence: 0.9, Query: "q", MatcherVersion: 1,
		Candidates: []Candidate{{TMDBID: 7857, Title: "Amarcord", Year: 1973, Score: 0.9}}}
	if err := s.SaveIdentifications([]Identification{auto}); err != nil {
		t.Fatal(err)
	}
	ts, _ := s.IdentifyTargets()
	if c := ts[0].Current; c == nil || !reflect.DeepEqual(*c, auto) {
		t.Fatalf("current %+v", ts[0].Current)
	}

	if err := s.SetCorrection("a1", StatusManual, 42); err != nil {
		t.Fatal(err)
	}
	// A matcher result computed before the correction must not undo it.
	if err := s.SaveIdentifications([]Identification{auto}); err != nil {
		t.Fatal(err)
	}
	ts, _ = s.IdentifyTargets()
	if c := ts[0].Current; c.Status != StatusManual || c.TMDBID != 42 || c.Query != "q" || len(c.Candidates) != 1 {
		t.Fatalf("after correction %+v", c)
	}

	if err := s.ResetIdentification("a1"); err != nil {
		t.Fatal(err)
	}
	if ts, _ = s.IdentifyTargets(); ts[0].Current != nil {
		t.Fatalf("after reset %+v", ts[0].Current)
	}
}

func TestSetCorrectionValidates(t *testing.T) {
	s := identityCatalog(t)
	if err := s.SetCorrection("nope", StatusIgnored, 0); !errors.Is(err, ErrUnknownFingerprint) {
		t.Errorf("unknown fingerprint: %v", err)
	}
	if err := s.SetCorrection("", StatusIgnored, 0); !errors.Is(err, ErrUnknownFingerprint) {
		t.Errorf("empty fingerprint: %v", err)
	}
	if err := s.SetCorrection("a1", StatusAuto, 1); err == nil {
		t.Error("auto accepted as a correction")
	}
}

func TestMoviesRoundTripAndEnrichQueue(t *testing.T) {
	s := identityCatalog(t)
	m := Movie{TMDBID: 7857, Title: "Amarcord", OriginalTitle: "Amarcord", Year: 1973, Runtime: 123, OriginalLang: "it",
		Overview: "Rimini.", Directors: []Person{{ID: 4415, Name: "Federico Fellini"}},
		Cast: []CastMember{{ID: 1, Name: "Magali Noël", Character: "Gradisca"}}, Genres: []string{"Comedia"},
		Countries: []string{"IT", "FR"}, PosterPath: "/p.jpg", BackdropPath: "/b.jpg", IMDbID: "tt0071129", Language: "es-ES"}
	if err := s.SaveMovie(m); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Movie(7857)
	if err != nil || !ok || !reflect.DeepEqual(got, m) {
		t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, _ := s.Movie(1); ok {
		t.Fatal("unknown movie found")
	}

	s.SaveIdentifications([]Identification{
		{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857},
		{Fingerprint: "vob2", Status: StatusAuto, TMDBID: 1398},
	})
	s.SetCorrection("a2", StatusExtra, 500)
	ids, err := s.MoviesToEnrich("es-ES")
	if err != nil || !reflect.DeepEqual(ids, []int{500, 1398}) {
		t.Fatalf("to enrich %v %v", ids, err)
	}
	if ids, _ := s.MoviesToEnrich("en-US"); !reflect.DeepEqual(ids, []int{500, 1398, 7857}) {
		t.Fatalf("other language %v", ids)
	}

	pending, err := s.PendingWikidata(10)
	if err != nil || len(pending) != 1 || pending[0].TMDBID != 7857 {
		t.Fatalf("wikidata %+v %v", pending, err)
	}
	m.WikidataDone, m.WikidataID = true, "Q18428"
	s.SaveMovie(m)
	if pending, _ := s.PendingWikidata(10); len(pending) != 0 {
		t.Fatalf("wikidata after %+v", pending)
	}
	if all, _ := s.Movies(); len(all) != 1 {
		t.Fatalf("movies %+v", all)
	}
}

func TestInvalidateMovie(t *testing.T) {
	s := identityCatalog(t)
	s.SaveMovie(Movie{TMDBID: 7857, Title: "Amarcord", Language: "es-ES"})
	s.SaveIdentifications([]Identification{{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857}})
	s.SetCorrection("vob2", StatusManual, 7857)
	if err := s.InvalidateMovie(7857); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Movie(7857); ok {
		t.Fatal("movie kept")
	}
	ts, _ := s.IdentifyTargets()
	if ts[0].Current != nil || ts[1].Current == nil || ts[1].Current.Status != StatusManual {
		t.Fatalf("targets %+v %+v", ts[0].Current, ts[1].Current)
	}
}
```

Crear `internal/store/versions_identity_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"
)

func TestVersionsIdentityAndBestByMovie(t *testing.T) {
	s := identityCatalog(t)
	s.SaveMovie(Movie{TMDBID: 7857, Title: "Amarcord", Year: 1973, Directors: []Person{{ID: 4415, Name: "Federico Fellini"}}, Language: "es-ES"})
	s.SaveIdentifications([]Identification{
		{Fingerprint: "a1", Status: StatusAuto, TMDBID: 7857, Confidence: 0.93},
		{Fingerprint: "vob2", Status: StatusUnmatched, Candidates: []Candidate{{TMDBID: 1398, Title: "Stalker", Score: 0.7}}},
	})
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	byDir := map[string]VersionView{}
	for _, v := range vs {
		byDir[v.Dir] = v
	}
	a, c, d := byDir["../cine/a"], byDir["../cine/copy"], byDir["../cine/d"]
	if a.Fingerprint != "a1" || a.Identification == nil || a.Identification.Confidence != 0.93 ||
		a.Movie == nil || a.Movie.Title != "Amarcord" || a.Movie.Directors[0].Name != "Federico Fellini" {
		t.Fatalf("a %+v %+v %+v", a, a.Identification, a.Movie)
	}
	// The copy shares a1: same movie, so the two versions compete for best
	// although the copy's parsed title lacks the year.
	if c.Movie == nil || a.Best == c.Best {
		t.Fatalf("best a=%v copy=%v", a.Best, c.Best)
	}
	if !a.Best {
		t.Fatal("the larger two-part version should be best")
	}
	if d.Movie != nil || d.Identification.Status != StatusUnmatched {
		t.Fatalf("d %+v", d.Identification)
	}

	un, err := s.Unidentified()
	if err != nil || len(un) != 1 || un[0].Dir != "../cine/d" || len(un[0].Candidates) != 1 || un[0].Candidates[0].TMDBID != 1398 {
		t.Fatalf("unidentified %+v %v", un, err)
	}
}

func TestOlderReadOnlyCatalogHasNoIdentity(t *testing.T) {
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
	if _, err := ro.Versions(); err != nil {
		t.Fatal(err)
	}
	if un, err := ro.Unidentified(); err != nil || len(un) != 0 {
		t.Fatalf("unidentified %v %v", un, err)
	}
	if ts, err := ro.IdentifyTargets(); err != nil || ts != nil {
		t.Fatalf("targets %v %v", ts, err)
	}
	if _, ok, err := ro.Movie(1); ok || err != nil {
		t.Fatalf("movie %v %v", ok, err)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/store/`
Expected: FAIL de compilación (`undefined: IdentifyTarget`, `Movie`, …).

- [ ] **Step 3: Esquema**

En `internal/store/schema.sql`, reemplazá:

```sql
  with_ffprobe  INTEGER NOT NULL DEFAULT 0  -- 1 if the ffprobe fallback was available
);
```

por:

```sql
  with_ffprobe  INTEGER NOT NULL DEFAULT 0  -- 1 if the ffprobe fallback was available
);

-- Movies from TMDB (stage 3). JSON columns hold arrays.
CREATE TABLE IF NOT EXISTS movies (
  tmdb_id        INTEGER PRIMARY KEY,
  title          TEXT    NOT NULL,
  original_title TEXT    NOT NULL DEFAULT '',
  year           INTEGER NOT NULL DEFAULT 0,
  runtime        INTEGER NOT NULL DEFAULT 0,    -- minutes
  original_lang  TEXT    NOT NULL DEFAULT '',
  overview       TEXT    NOT NULL DEFAULT '',
  directors      TEXT    NOT NULL DEFAULT '[]', -- JSON [{id,name}]
  cast_members   TEXT    NOT NULL DEFAULT '[]', -- JSON [{id,name,character}], first 10
  genres         TEXT    NOT NULL DEFAULT '[]', -- JSON localized names
  countries      TEXT    NOT NULL DEFAULT '[]', -- JSON ISO 3166-1 alpha-2
  collection_id  INTEGER NOT NULL DEFAULT 0,
  collection     TEXT    NOT NULL DEFAULT '',
  poster_path    TEXT    NOT NULL DEFAULT '',
  backdrop_path  TEXT    NOT NULL DEFAULT '',
  imdb_id        TEXT    NOT NULL DEFAULT '',
  wikidata_id    TEXT    NOT NULL DEFAULT '',
  language       TEXT    NOT NULL,              -- language requested from TMDB
  fetched_at     INTEGER NOT NULL,              -- unix milliseconds
  wikidata_state INTEGER NOT NULL DEFAULT 0     -- 0 pending, 1 looked up
);

-- What each content fingerprint is. auto/unmatched rows are the matcher's
-- and get recomputed; manual/ignored/extra rows are the user's corrections.
CREATE TABLE IF NOT EXISTS identifications (
  fingerprint     TEXT    PRIMARY KEY,
  status          TEXT    NOT NULL,              -- auto | manual | ignored | extra | unmatched
  tmdb_id         INTEGER NOT NULL DEFAULT 0,    -- the movie (auto/manual) or the movie it is an extra of
  confidence      REAL    NOT NULL DEFAULT 0,
  candidates      TEXT    NOT NULL DEFAULT '[]', -- JSON top 5 candidates
  query           TEXT    NOT NULL DEFAULT '',   -- "title|year|director|imdb" the matcher used
  matcher_version INTEGER NOT NULL DEFAULT 0,
  updated_at      INTEGER NOT NULL               -- unix milliseconds
);

CREATE INDEX IF NOT EXISTS identifications_tmdb ON identifications(tmdb_id);
```


- [ ] **Step 4: Películas**

Crear `internal/store/movies.go`:

```go
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Person struct {
	ID   int    `json:"id"` // TMDB person id; 0 when it came from Wikidata
	Name string `json:"name"`
}

type CastMember struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Character string `json:"character"`
}

type Movie struct {
	TMDBID        int          `json:"tmdbId"`
	Title         string       `json:"title"`
	OriginalTitle string       `json:"originalTitle"`
	Year          int          `json:"year"`
	Runtime       int          `json:"runtime"`
	OriginalLang  string       `json:"originalLang"`
	Overview      string       `json:"overview"`
	Directors     []Person     `json:"directors"`
	Cast          []CastMember `json:"cast"`
	Genres        []string     `json:"genres"`
	Countries     []string     `json:"countries"`
	CollectionID  int          `json:"collectionId"`
	Collection    string       `json:"collection"`
	PosterPath    string       `json:"posterPath"`
	BackdropPath  string       `json:"backdropPath"`
	IMDbID        string       `json:"imdbId"`
	WikidataID    string       `json:"wikidataId"`
	Language      string       `json:"language"`
	WikidataDone  bool         `json:"-"`
}

const movieColumns = `tmdb_id, title, original_title, year, runtime, original_lang, overview, directors,
	cast_members, genres, countries, collection_id, collection, poster_path, backdrop_path, imdb_id,
	wikidata_id, language, wikidata_state`

// SaveMovie inserts or replaces a movie.
func (s *Store) SaveMovie(m Movie) error {
	var js [4]string
	for i, v := range []any{m.Directors, m.Cast, m.Genres, m.Countries} {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if string(b) == "null" {
			b = []byte("[]")
		}
		js[i] = string(b)
	}
	_, err := s.db.Exec(`INSERT INTO movies (`+movieColumns+`, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tmdb_id) DO UPDATE SET title = excluded.title, original_title = excluded.original_title,
		year = excluded.year, runtime = excluded.runtime, original_lang = excluded.original_lang,
		overview = excluded.overview, directors = excluded.directors, cast_members = excluded.cast_members,
		genres = excluded.genres, countries = excluded.countries, collection_id = excluded.collection_id,
		collection = excluded.collection, poster_path = excluded.poster_path, backdrop_path = excluded.backdrop_path,
		imdb_id = excluded.imdb_id, wikidata_id = excluded.wikidata_id, language = excluded.language,
		wikidata_state = excluded.wikidata_state, fetched_at = excluded.fetched_at`,
		m.TMDBID, m.Title, m.OriginalTitle, m.Year, m.Runtime, m.OriginalLang, m.Overview, js[0], js[1], js[2], js[3],
		m.CollectionID, m.Collection, m.PosterPath, m.BackdropPath, m.IMDbID, m.WikidataID, m.Language,
		m.WikidataDone, time.Now().UnixMilli())
	return err
}

type scanner interface{ Scan(dest ...any) error }

func scanMovie(r scanner) (Movie, error) {
	var m Movie
	var dirs, cast, genres, countries string
	err := r.Scan(&m.TMDBID, &m.Title, &m.OriginalTitle, &m.Year, &m.Runtime, &m.OriginalLang, &m.Overview,
		&dirs, &cast, &genres, &countries, &m.CollectionID, &m.Collection, &m.PosterPath, &m.BackdropPath,
		&m.IMDbID, &m.WikidataID, &m.Language, &m.WikidataDone)
	if err != nil {
		return m, err
	}
	for _, p := range []struct {
		src string
		dst any
	}{{dirs, &m.Directors}, {cast, &m.Cast}, {genres, &m.Genres}, {countries, &m.Countries}} {
		if err := json.Unmarshal([]byte(p.src), p.dst); err != nil {
			return m, err
		}
	}
	return m, nil
}

// Movie returns the movie with a TMDB id; ok is false when it is not stored.
func (s *Store) Movie(id int) (m Movie, ok bool, err error) {
	if !s.hasIdentity {
		return m, false, nil
	}
	m, err = scanMovie(s.db.QueryRow(`SELECT `+movieColumns+` FROM movies WHERE tmdb_id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return m, false, nil
	}
	return m, err == nil, err
}

// Movies returns every stored movie by TMDB id.
func (s *Store) Movies() ([]Movie, error) {
	if !s.hasIdentity {
		return nil, nil
	}
	return s.queryMovies(`SELECT ` + movieColumns + ` FROM movies ORDER BY tmdb_id`)
}

// PendingWikidata returns up to limit movies not looked up in Wikidata yet.
func (s *Store) PendingWikidata(limit int) ([]Movie, error) {
	return s.queryMovies(`SELECT `+movieColumns+` FROM movies WHERE wikidata_state = 0 ORDER BY tmdb_id LIMIT ?`, limit)
}

func (s *Store) queryMovies(q string, args ...any) ([]Movie, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Movie
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MoviesToEnrich returns the TMDB ids that identifications point to and that
// are missing from movies or were fetched in another language.
func (s *Store) MoviesToEnrich(lang string) ([]int, error) {
	rows, err := s.db.Query(`SELECT DISTINCT i.tmdb_id FROM identifications i
		LEFT JOIN movies m ON m.tmdb_id = i.tmdb_id
		WHERE i.status IN ('auto', 'manual', 'extra') AND i.tmdb_id > 0
		  AND (m.tmdb_id IS NULL OR m.language != ?)
		ORDER BY i.tmdb_id`, lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// InvalidateMovie handles a TMDB id that no longer exists: the movie is
// dropped and the matcher's identifications pointing to it are forgotten
// (so they are identified again). The user's corrections are kept.
func (s *Store) InvalidateMovie(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM movies WHERE tmdb_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM identifications WHERE tmdb_id = ? AND status = 'auto'`, id); err != nil {
		return err
	}
	return tx.Commit()
}
```

- [ ] **Step 5: Identificaciones**

Crear `internal/store/identity.go`:

```go
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path"
	"time"
)

// Identification statuses. auto and unmatched belong to the matcher; the
// others are the user's corrections and are never overwritten by it.
const (
	StatusAuto      = "auto"
	StatusManual    = "manual"
	StatusIgnored   = "ignored"
	StatusExtra     = "extra"
	StatusUnmatched = "unmatched"
)

var ErrUnknownFingerprint = errors.New("huella desconocida")

// Candidate is a possible TMDB match for a version.
type Candidate struct {
	TMDBID        int     `json:"tmdbId"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"originalTitle"`
	Year          int     `json:"year"`
	PosterPath    string  `json:"posterPath"`
	Score         float64 `json:"score"`
}

type Identification struct {
	Fingerprint    string
	Status         string
	TMDBID         int
	Confidence     float64
	Candidates     []Candidate
	Query          string
	MatcherVersion int
}

// IdentifyTarget is a version to identify, keyed by the fingerprint of its
// representative main file.
type IdentifyTarget struct {
	Fingerprint string
	Dir         string
	Title       string
	Year        int
	Director    string
	IMDbID      string          // from the file or folder name
	NFOs        []string        // present .nfo files in Dir (catalog paths, sorted)
	Current     *Identification // nil when never identified
}

// representatives maps each version id to the fingerprint of its
// representative file: the present main file with the lowest part number,
// the largest on ties (the biggest VOB of a DVD).
func (s *Store) representatives(tx querier) (map[int64]string, error) {
	rows, err := tx.Query(`SELECT version_id, fingerprint FROM files
		WHERE version_id IS NOT NULL AND role = 'main' AND missing = 0 AND fingerprint != ''
		ORDER BY version_id, part, size DESC, path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var fp string
		if err := rows.Scan(&id, &fp); err != nil {
			return nil, err
		}
		if _, ok := out[id]; !ok {
			out[id] = fp
		}
	}
	return out, rows.Err()
}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// identifications loads every identification row by fingerprint.
func (s *Store) identifications(tx querier) (map[string]*Identification, error) {
	out := map[string]*Identification{}
	if !s.hasIdentity {
		return out, nil
	}
	rows, err := tx.Query(`SELECT fingerprint, status, tmdb_id, confidence, candidates, query, matcher_version FROM identifications`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var i Identification
		var cands string
		if err := rows.Scan(&i.Fingerprint, &i.Status, &i.TMDBID, &i.Confidence, &cands, &i.Query, &i.MatcherVersion); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(cands), &i.Candidates); err != nil {
			return nil, err
		}
		out[i.Fingerprint] = &i
	}
	return out, rows.Err()
}

// IdentifyTargets returns one target per distinct representative fingerprint
// (identical copies are identified once), in version order.
func (s *Store) IdentifyTargets() ([]IdentifyTarget, error) {
	if !s.hasIdentity {
		return nil, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	reps, err := s.representatives(tx)
	if err != nil {
		return nil, err
	}
	current, err := s.identifications(tx)
	if err != nil {
		return nil, err
	}
	nfos := map[string][]string{}
	nrows, err := tx.Query(`SELECT path FROM files WHERE kind = 'info' AND missing = 0 ORDER BY path`)
	if err != nil {
		return nil, err
	}
	for nrows.Next() {
		var p string
		if err := nrows.Scan(&p); err != nil {
			nrows.Close()
			return nil, err
		}
		nfos[path.Dir(p)] = append(nfos[path.Dir(p)], p)
	}
	nrows.Close()
	if err := nrows.Err(); err != nil {
		return nil, err
	}

	rows, err := tx.Query(`SELECT id, dir, title, year, director, imdb_id FROM versions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IdentifyTarget
	seen := map[string]bool{}
	for rows.Next() {
		var id int64
		var t IdentifyTarget
		if err := rows.Scan(&id, &t.Dir, &t.Title, &t.Year, &t.Director, &t.IMDbID); err != nil {
			return nil, err
		}
		fp, ok := reps[id]
		if !ok || seen[fp] {
			continue
		}
		seen[fp] = true
		t.Fingerprint, t.NFOs, t.Current = fp, nfos[t.Dir], current[fp]
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveIdentifications stores matcher results in one transaction. A row that
// became a correction in the meantime is left alone.
func (s *Store) SaveIdentifications(ids []Identification) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	up, err := tx.Prepare(`INSERT INTO identifications
		(fingerprint, status, tmdb_id, confidence, candidates, query, matcher_version, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET status = excluded.status, tmdb_id = excluded.tmdb_id,
		confidence = excluded.confidence, candidates = excluded.candidates, query = excluded.query,
		matcher_version = excluded.matcher_version, updated_at = excluded.updated_at
		WHERE identifications.status IN ('auto', 'unmatched')`)
	if err != nil {
		return err
	}
	defer up.Close()
	now := time.Now().UnixMilli()
	for _, i := range ids {
		cands := i.Candidates
		if cands == nil {
			cands = []Candidate{}
		}
		b, err := json.Marshal(cands)
		if err != nil {
			return err
		}
		if _, err := up.Exec(i.Fingerprint, i.Status, i.TMDBID, i.Confidence, string(b), i.Query, i.MatcherVersion, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetCorrection records the user's decision for a fingerprint: a movie
// (StatusManual), not a movie (StatusIgnored) or an extra of a movie
// (StatusExtra). Candidates and query of a previous match are kept.
func (s *Store) SetCorrection(fingerprint, status string, tmdbID int) error {
	if status != StatusManual && status != StatusIgnored && status != StatusExtra {
		return errors.New("store: estado de corrección inválido: " + status)
	}
	if fingerprint == "" {
		return ErrUnknownFingerprint
	}
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM files WHERE fingerprint = ? LIMIT 1`, fingerprint).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnknownFingerprint
	}
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO identifications (fingerprint, status, tmdb_id, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET status = excluded.status, tmdb_id = excluded.tmdb_id,
		updated_at = excluded.updated_at`, fingerprint, status, tmdbID, time.Now().UnixMilli())
	return err
}

// ResetIdentification forgets what a fingerprint is, so the matcher runs on
// it again.
func (s *Store) ResetIdentification(fingerprint string) error {
	_, err := s.db.Exec(`DELETE FROM identifications WHERE fingerprint = ?`, fingerprint)
	return err
}

// UnidentifiedView is a version the matcher could not decide on.
type UnidentifiedView struct {
	VersionView
	Candidates []Candidate `json:"candidates"`
}

// Unidentified returns the unmatched versions with their candidates, one per
// fingerprint.
func (s *Store) Unidentified() ([]UnidentifiedView, error) {
	vs, err := s.Versions()
	if err != nil {
		return nil, err
	}
	ids, err := s.identifications(s.db)
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
func (s *Store) attachIdentity(out []VersionView, pos map[int64]int) error {
	reps, err := s.representatives(s.db)
	if err != nil {
		return err
	}
	for id, fp := range reps {
		if i, ok := pos[id]; ok {
			out[i].Fingerprint = fp
		}
	}
	if !s.hasIdentity {
		return nil
	}
	ids, err := s.identifications(s.db)
	if err != nil {
		return err
	}
	refs := map[int]*MovieRef{}
	rows, err := s.db.Query(`SELECT tmdb_id, title, original_title, year, directors FROM movies`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var m MovieRef
		var dirs string
		if err := rows.Scan(&m.TMDBID, &m.Title, &m.OriginalTitle, &m.Year, &dirs); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(dirs), &m.Directors); err != nil {
			return err
		}
		refs[m.TMDBID] = &m
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range out {
		v := &out[i]
		id := ids[v.Fingerprint]
		if v.Fingerprint == "" || id == nil {
			continue
		}
		v.Identification = &IdentView{Status: id.Status, Confidence: id.Confidence, TMDBID: id.TMDBID}
		if id.Status == StatusAuto || id.Status == StatusManual {
			v.Movie = refs[id.TMDBID]
		}
	}
	return nil
}
```

- [ ] **Step 6: Versiones y mejor versión**

En `internal/store/store.go`, reemplazá:

```go
type Store struct {
	db *sql.DB
	// hasMedia is false for a stage-1 catalog opened read-only: the schema
	// only runs on writable opens, so the media table may not exist.
	hasMedia bool
}
```

por:

```go
type Store struct {
	db *sql.DB
	// hasMedia and hasIdentity are false for an older catalog opened
	// read-only: the schema only runs on writable opens, so the tables of
	// later stages may not exist.
	hasMedia    bool
	hasIdentity bool
}
```

En `internal/store/store.go`, reemplazá:

```go
	Subs       []probe.Track `json:"subs"` // embedded subtitle tracks
	Best       bool          `json:"best"` // best of several versions of the same movie
}
```

por:

```go
	Subs       []probe.Track `json:"subs"` // embedded subtitle tracks
	Best       bool          `json:"best"` // best of several versions of the same movie

	// Identity: Fingerprint is the representative main file's ("" when none
	// is present and hashed). Movie is set for versions identified as a
	// stored movie.
	Fingerprint    string     `json:"fingerprint"`
	Identification *IdentView `json:"identification"`
	Movie          *MovieRef  `json:"movie"`
}

type IdentView struct {
	Status     string  `json:"status"`
	Confidence float64 `json:"confidence"`
	TMDBID     int     `json:"tmdbId"` // the movie, or the movie it is an extra of
}

type MovieRef struct {
	TMDBID        int      `json:"tmdbId"`
	Title         string   `json:"title"`
	OriginalTitle string   `json:"originalTitle"`
	Year          int      `json:"year"`
	Directors     []Person `json:"directors"`
}
```

En `internal/store/store.go`, reemplazá:

```go
	}
	s := &Store{db: db}
	if err := db.QueryRow(`SELECT COUNT(*) > 0 FROM sqlite_master WHERE type = 'table' AND name = 'media'`).Scan(&s.hasMedia); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
```

por:

```go
	}
	s := &Store{db: db}
	for name, dst := range map[string]*bool{"media": &s.hasMedia, "identifications": &s.hasIdentity} {
		if err := db.QueryRow(`SELECT COUNT(*) > 0 FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(dst); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
```

En `internal/store/store.go`, reemplazá:

```go
		return nil, err
	}
	markBest(out)
	return out, nil
```

por:

```go
		return nil, err
	}
	if err := s.attachIdentity(out, pos); err != nil {
		return nil, err
	}
	markBest(out)
	return out, nil
```

En `internal/store/technical.go`, reemplazá:

```go
import (
	"encoding/json"

	"cinexplorer/internal/probe"
```

por:

```go
import (
	"encoding/json"
	"strconv"

	"cinexplorer/internal/probe"
```

En `internal/store/technical.go`, reemplazá:

```go

// markBest flags the best version in each group of two or more present
// versions that share a provisional identity (title + year). Full ties go to
// the lowest id, so the choice is stable.
func markBest(vs []VersionView) {
	best := map[string]int{}
```

por:

```go

// markBest flags the best version in each group of two or more present
// versions of the same movie: the same TMDB id when identified, else the
// same provisional identity (title + year). Full ties go to the lowest id,
// so the choice is stable. It must see every version, not a filtered subset.
func markBest(vs []VersionView) {
	best := map[string]int{}
```

En `internal/store/technical.go`, reemplazá:

```go
		v := &vs[i]
		key := quality.GroupKey(v.Title, v.Year)
		if key == "" || !hasPresentMain(v) {
			continue
```

por:

```go
		v := &vs[i]
		key := quality.GroupKey(v.Title, v.Year)
		if v.Movie != nil {
			key = "tmdb:" + strconv.Itoa(v.Movie.TMDBID)
		}
		if key == "" || !hasPresentMain(v) {
			continue
```


- [ ] **Step 7: Verificar que pasan**

Run: `go test ./internal/store/`
Expected: `ok` (también los tests anteriores de `store`).

- [ ] **Step 8: Commit**

```bash
git add internal/store
git commit -m "feat(store): movies, identifications and corrections by fingerprint

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Puntaje de confianza (`identify/score.go`)

**Files:**
- Create: `internal/identify/score.go`
- Test: `internal/identify/score_test.go`

Similitud de títulos (Levenshtein normalizado sobre `quality.NormTitle`), año (exacto 0.25, ±1 0.15, sin año 0.10) y coincidencia de director por la última palabra.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/identify/score_test.go`:

```go
package identify

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		want float64
	}{
		{"Amarcord", "AMARCORD", 1},
		{"El Ángel Exterminador", "angel exterminador", 1},
		{"Solyaris", "Solaris", 1 - 1.0/8},
		{"", "Solaris", 0},
		{"abc", "xyz", 0},
	}
	for _, c := range cases {
		if got := similarity(c.a, c.b); !near(got, c.want) {
			t.Errorf("similarity(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestYearScore(t *testing.T) {
	cases := []struct {
		parsed, cand int
		want         float64
	}{
		{1973, 1973, 0.25}, {1973, 1974, 0.15}, {1973, 1972, 0.15}, {1973, 1975, 0},
		{1973, 0, 0}, {0, 1973, 0.10}, {0, 0, 0.10},
	}
	for _, c := range cases {
		if got := yearScore(c.parsed, c.cand); !near(got, c.want) {
			t.Errorf("yearScore(%d, %d) = %v, want %v", c.parsed, c.cand, got, c.want)
		}
	}
}

func TestDirectorMatches(t *testing.T) {
	cases := []struct {
		parsed string
		dirs   []string
		want   bool
	}{
		{"Polanski", []string{"Roman Polański"}, true},
		{"Fellini", []string{"Federico Fellini"}, true},
		{"Federico Fellini", []string{"Federico Fellini"}, true},
		{"Wong Kar-wai", []string{"Wong Kar-wai"}, true},
		{"Lynch", []string{"David Lynch", "Mark Frost"}, true},
		{"Donner", []string{"Richard Lester"}, false},
		{"", []string{"Anyone"}, false},
	}
	for _, c := range cases {
		if got := directorMatches(c.parsed, c.dirs); got != c.want {
			t.Errorf("directorMatches(%q, %q) = %v", c.parsed, c.dirs, got)
		}
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/identify/`
Expected: FAIL de compilación (`undefined: similarity`, …).

- [ ] **Step 3: Implementar**

Crear `internal/identify/score.go`:

```go
package identify

import (
	"slices"

	"cinexplorer/internal/quality"
)

// Weights of the confidence score (they add up to 1).
const (
	titleWeight    = 0.60
	yearWeight     = 0.25
	directorWeight = 0.15
)

// similarity compares two titles after normalization: 1 − edit distance /
// longer length, over runes. Empty titles are not similar to anything.
func similarity(a, b string) float64 {
	ra, rb := []rune(quality.NormTitle(a)), []rune(quality.NormTitle(b))
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	return 1 - float64(levenshtein(ra, rb))/float64(max(len(ra), len(rb)))
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// titleScore is the best similarity of the parsed title to either title of
// the candidate, weighted.
func titleScore(parsed, title, original string) float64 {
	return titleWeight * max(similarity(parsed, title), similarity(parsed, original))
}

// yearScore rewards an exact year and tolerates ±1 (festival vs release
// dates). Without a parsed year it gives a neutral share.
func yearScore(parsed, candidate int) float64 {
	switch {
	case parsed == 0:
		return 0.10
	case candidate == parsed:
		return yearWeight
	case candidate != 0 && (candidate == parsed-1 || candidate == parsed+1):
		return 0.15
	}
	return 0
}

// directorMatches reports whether the parsed director (often just a
// surname: "Polanski", "Fellini") names one of the candidate's directors:
// its last word must be one of a director's words.
func directorMatches(parsed string, directors []string) bool {
	words := quality.Words(parsed)
	if len(words) == 0 {
		return false
	}
	last := words[len(words)-1]
	for _, d := range directors {
		if slices.Contains(quality.Words(d), last) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/identify/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/identify
git commit -m "feat(identify): confidence score parts

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Búsqueda e identificación (`identify/match.go`) y corpus de calibración

**Files:**
- Create: `internal/identify/match.go`
- Create: `internal/identify/fake_test.go`
- Test: `internal/identify/match_test.go`
- Test: `internal/identify/corpus_test.go`
- Existe ya: `internal/identify/testdata/tmdb_corpus.json` (commiteado con este plan)

`Search` puntúa candidatos: IMDb primero, variantes del título (fuera y dentro de paréntesis), búsqueda sin año si no hay resultados, títulos en inglés si no es concluyente, y director para los 3 mejores. `Identify` decide `auto` (≥ 0.80 y ventaja ≥ 0.10) o `unmatched` con 5 candidatos, y prueba la consulta alternativa por carpeta si hace falta. `corpus_test.go` corre 30 nombres reales contra respuestas de TMDB grabadas (`testdata/tmdb_corpus.json`, ya en la rama); regrabarlas: `CINEXPLORER_TMDB_RECORD=1 go test ./internal/identify -run Corpus -v` con el token en `CINEXPLORER_TMDB_TOKEN` o `~/.cinexplorer-tmdb-token`. `fake_test.go` tiene el TMDB/Wikidata falsos que usan estos tests y los del Runner.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/identify/fake_test.go`:

```go
package identify

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// fakeAPI answers from maps. Search keys are "title|year|lang".
type fakeAPI struct {
	mu     sync.Mutex
	search map[string][]tmdb.Result
	find   map[string][]tmdb.Result
	movies map[string]tmdb.Details // "id|lang"
	err    error                   // returned by every call when set
	calls  []string
}

func (f *fakeAPI) record(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	return f.err
}

func (f *fakeAPI) SearchMovie(ctx context.Context, q string, year int, lang string) ([]tmdb.Result, error) {
	key := fmt.Sprintf("%s|%d|%s", q, year, lang)
	if err := f.record("search " + key); err != nil {
		return nil, err
	}
	return f.search[key], nil
}

func (f *fakeAPI) FindIMDb(ctx context.Context, id, lang string) ([]tmdb.Result, error) {
	if err := f.record("find " + id); err != nil {
		return nil, err
	}
	return f.find[id], nil
}

func (f *fakeAPI) Movie(ctx context.Context, id int, lang string) (tmdb.Details, error) {
	key := strconv.Itoa(id) + "|" + lang
	if err := f.record("movie " + key); err != nil {
		return tmdb.Details{}, err
	}
	d, ok := f.movies[key]
	if !ok {
		return d, tmdb.ErrNotFound
	}
	return d, nil
}

func (f *fakeAPI) called(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if len(c) >= len(prefix) && c[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}

type fakeWikidata struct {
	ents  map[int]wikidata.Entity
	err   error
	calls [][]int
}

func (f *fakeWikidata) Lookup(ctx context.Context, ids []int, lang string) (map[int]wikidata.Entity, error) {
	f.calls = append(f.calls, ids)
	return f.ents, f.err
}

func details(id int, title, date, overview string, directors ...string) tmdb.Details {
	d := tmdb.Details{ID: id, Title: title, OriginalTitle: title, ReleaseDate: date, Overview: overview}
	for i, n := range directors {
		d.Credits.Crew = append(d.Credits.Crew, tmdb.Person{ID: 100 + i, Name: n, Job: "Director"})
	}
	return d
}
```

Crear `internal/identify/match_test.go`:

```go
package identify

import (
	"context"
	"reflect"
	"testing"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

func TestSearchFallbacks(t *testing.T) {
	api := &fakeAPI{
		search: map[string][]tmdb.Result{
			// Nothing with the year: retried without it.
			"Cries and Whispers|0|es-ES": {{ID: 10238, Title: "Gritos y susurros", OriginalTitle: "Viskningar och rop", ReleaseDate: "1972-03-05"}},
			// The English search names it as the file does.
			"Cries and Whispers|1972|en-US": {{ID: 10238, Title: "Cries and Whispers", ReleaseDate: "1972-03-05"}},
		},
		find: map[string][]tmdb.Result{},
	}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{Title: "Cries and Whispers", Year: 1972, IMDbID: "tt9999999"})
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusAuto || id.TMDBID != 10238 || id.Candidates[0].Title != "Gritos y susurros" {
		t.Fatalf("got %+v", id)
	}
	want := []string{"find tt9999999", "search Cries and Whispers|1972|es-ES", "search Cries and Whispers|0|es-ES",
		"search Cries and Whispers|1972|en-US"}
	if !reflect.DeepEqual(api.calls, want) {
		t.Fatalf("calls %q", api.calls)
	}
}

func TestSearchDirectorBreaksTies(t *testing.T) {
	api := &fakeAPI{
		search: map[string][]tmdb.Result{"Ordet|0|es-ES": {
			{ID: 262879, Title: "Ordet", ReleaseDate: "1943-01-01"},
			{ID: 48035, Title: "La palabra", OriginalTitle: "Ordet", ReleaseDate: "1955-01-10"},
		}},
		movies: map[string]tmdb.Details{
			"262879|es-ES": details(262879, "Ordet", "1943-01-01", "", "Gustaf Molander"),
			"48035|es-ES":  details(48035, "La palabra", "1955-01-10", "", "Carl Theodor Dreyer"),
		},
	}
	ctx := context.Background()
	id, fetched, err := Identify(ctx, api, "es-ES", Query{Title: "Ordet", Director: "Dreyer"})
	if err != nil {
		t.Fatal(err)
	}
	if id.Status != store.StatusAuto || id.TMDBID != 48035 || len(fetched) != 2 {
		t.Fatalf("got %+v fetched %d", id, len(fetched))
	}
	// Without the director it is a tie: left for review with both candidates.
	api.movies = nil
	id, _, _ = Identify(ctx, api, "es-ES", Query{Title: "Ordet"})
	if id.Status != store.StatusUnmatched || len(id.Candidates) != 2 || id.TMDBID != 0 {
		t.Fatalf("got %+v", id)
	}
}

func TestIdentifyWithoutTitle(t *testing.T) {
	api := &fakeAPI{}
	id, _, err := Identify(context.Background(), api, "es-ES", Query{})
	if err != nil || id.Status != store.StatusUnmatched || len(id.Candidates) != 0 || len(api.calls) != 0 {
		t.Fatalf("got %+v %v calls %v", id, err, api.calls)
	}
}

func TestTitleVariants(t *testing.T) {
	cases := map[string][]string{
		"Amarcord":                         {"Amarcord"},
		"La piel dura (L'argent de poche)": {"La piel dura", "L'argent de poche"},
		"Solyaris [Solaris]":               {"Solyaris", "Solaris"},
		"(Novecento)":                      {"Novecento"},
		"":                                 nil,
	}
	for in, want := range cases {
		if got := titleVariants(in); !reflect.DeepEqual(got, want) {
			t.Errorf("titleVariants(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueryKey(t *testing.T) {
	q := Query{Title: "Amarcord", Year: 1973, Director: "Fellini", IMDbID: "tt0071129"}
	if q.Key() != "Amarcord|1973|Fellini|tt0071129" {
		t.Fatal(q.Key())
	}
}
```

Crear `internal/identify/corpus_test.go`:

```go
package identify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// corpusFile holds trimmed TMDB responses keyed by request URI. Re-record
// with CINEXPLORER_TMDB_RECORD=1 and a token in CINEXPLORER_TMDB_TOKEN (or
// ~/.cinexplorer-tmdb-token).
const corpusFile = "testdata/tmdb_corpus.json"

// corpus lists real names from the collection and the expected outcome: a
// TMDB id for an automatic match, or 0 for "left for review".
var corpus = []struct {
	name string
	want int
}{
	{"Amarcord [Federico Fellini, 1973]", 7857},
	{"Chinatown (Polanski, USA, 1974)", 829},
	{"Arabian.Nights.1974.CC.1080p.BluRay.FLAC1.0.x264-ADE", 47406},
	{"A Woman Under the Influence [John Cassavetes, 1974]", 29845},
	{"Annie Hall [1977, USA]", 703},
	{"Apocalypse Now (Francis Ford Coppola, 1979)", 28},
	{"Cries and Whispers [Ingmar Bergman, 1972]", 10238},
	{"Eraserhead (Lynch, USA, 1976)", 985},
	{"La piel dura (L'argent de poche) (France, 1976)", 1660},
	{"La.Patagonia.rebelde.1974.Hector.Olivera.WEB-DL.1080p.SPA.RUS.Sub.SPA.ENG.RUS.(emule.via..clan-sudamerica.net)", 64978},
	{"Los Gauchos Judíos (Juan José Jusid, Argentina, 1974)", 537898},
	{"Los traidores (Raymundo Gleyzer, 1972)", 0},
	{"Mad Max (Miller, Australia, 1979) [1080p]", 9659},
	{"Nazareno.Cruz.Y.El.Lobo.1975.720p.WEB-DL.AAC2.0.H.264-gooz.(Found.via.clan-sudamerica.net)", 127424},
	{"Salo o le 120 giornate di Sodoma (1975) [Italy]", 5336},
	{"Scener ur ett äktenskap (1973)", 133919},
	{"Solyaris.[Solaris].1972.DVDRip.H264.AAC.Gopo", 593},
	{"Star.Wars.Episode.IV.A.New.Hope.1977.REMASTERED.1080p.BluRay.x265-RARBG", 0},
	{"The Omen [Donner] (UK, 1976)", 794},
	{"Paper Moon (1973)", 11293},
	{"Hamaca Paraguaya", 0},
	{"Shoah", 42044},
	{"Destino.final-zoe(Emule.via.clan-sudamerica.net)", 0},
	{"Ordet", 0},
	{"Our Litlle Sister", 0},
	{"El cazador (Shekarchi)", 0},
	{"Husbands (John Cassavetes, 1970) [Extended Cut, 142min] XVID-KG", 52105},
	{"1900 (Novecento) (1976) [mkvonly]", 3870},
	{"Diarios de motocicleta", 1653},
	{"Grey Gardens (1975) 1080p.BluRay.H264.AAC-RARBG", 17346},
}

func TestCorpus(t *testing.T) {
	c := corpusClient(t)
	ctx := context.Background()
	auto := 0
	for _, tc := range corpus {
		p := nameparse.Parse(tc.name)
		q := Query{Title: p.Title, Year: p.Year, Director: p.Director, IMDbID: p.IMDbID}
		id, _, err := Identify(ctx, c, "es-ES", q)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		got := 0
		if id.Status == store.StatusAuto {
			got = id.TMDBID
			auto++
		}
		t.Logf("%-50.50s → %-9s %6d  %.2f  %s", tc.name, id.Status, id.TMDBID, id.Confidence, describe(id.Candidates))
		if got != tc.want {
			t.Errorf("%s: got %s %d (%.2f), want %d", tc.name, id.Status, got, id.Confidence, tc.want)
		}
	}
	t.Logf("automáticas: %d/%d", auto, len(corpus))
}

func describe(cs []store.Candidate) string {
	var parts []string
	for _, c := range cs[:min(3, len(cs))] {
		parts = append(parts, fmt.Sprintf("%s (%d) %d %.2f", c.Title, c.Year, c.TMDBID, c.Score))
	}
	return strings.Join(parts, " | ")
}

// corpusClient serves the recorded responses, or records them first when
// CINEXPLORER_TMDB_RECORD is set.
func corpusClient(t *testing.T) *tmdb.Client {
	t.Helper()
	recorded := map[string]json.RawMessage{}
	if b, err := os.ReadFile(corpusFile); err == nil {
		if err := json.Unmarshal(b, &recorded); err != nil {
			t.Fatal(err)
		}
	}
	record := os.Getenv("CINEXPLORER_TMDB_RECORD") != ""
	var real *tmdb.Client
	if record {
		real = tmdb.New(token(t))
	}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.RequestURI()
		mu.Lock()
		body, ok := recorded[key]
		mu.Unlock()
		if !ok && record {
			raw, err := real.HTTP.Get(r.Context(), real.BaseURL+key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			body = trim(t, key, raw)
			mu.Lock()
			recorded[key] = body
			mu.Unlock()
			ok = true
		}
		if !ok {
			t.Errorf("sin respuesta grabada para %s: regrabá con CINEXPLORER_TMDB_RECORD=1", key)
			http.Error(w, "not recorded", http.StatusNotFound)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(func() {
		srv.Close()
		if record {
			save(t, recorded)
		}
	})
	c := tmdb.New("test")
	c.BaseURL = srv.URL
	c.HTTP.Limiter = nil
	c.HTTP.Attempts = 1
	return c
}

func token(t *testing.T) string {
	if tok := os.Getenv("CINEXPLORER_TMDB_TOKEN"); tok != "" {
		return tok
	}
	home, _ := os.UserHomeDir()
	b, err := os.ReadFile(filepath.Join(home, ".cinexplorer-tmdb-token"))
	if err != nil {
		t.Fatal("grabar necesita CINEXPLORER_TMDB_TOKEN o ~/.cinexplorer-tmdb-token")
	}
	return strings.TrimSpace(string(b))
}

// trim keeps only what the client reads, so the fixture stays small.
func trim(t *testing.T, key string, raw []byte) json.RawMessage {
	var out any
	switch {
	case strings.HasPrefix(key, "/search/movie"), strings.HasPrefix(key, "/find/"):
		var v struct {
			Results      []tmdb.Result `json:"results,omitempty"`
			MovieResults []tmdb.Result `json:"movie_results,omitempty"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(key, "/search/movie") && v.Results == nil {
			v.Results = []tmdb.Result{}
		}
		out = v
	case strings.HasPrefix(key, "/movie/"):
		var d tmdb.Details
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		crew := []tmdb.Person{}
		for _, p := range d.Credits.Crew {
			if p.Job == "Director" {
				crew = append(crew, p)
			}
		}
		d.Credits.Crew = crew
		d.Credits.Cast = d.Credits.Cast[:min(10, len(d.Credits.Cast))]
		out = d
	default:
		return raw
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func save(t *testing.T, recorded map[string]json.RawMessage) {
	keys := make([]string, 0, len(recorded))
	for k := range recorded {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("{\n")
	for i, k := range keys {
		kb, _ := json.Marshal(k)
		sb.Write(kb)
		sb.WriteString(": ")
		sb.Write(recorded[k])
		if i < len(keys)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	if err := os.MkdirAll(filepath.Dir(corpusFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpusFile, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/identify/`
Expected: FAIL de compilación (`undefined: Identify`, `Query`, …).

- [ ] **Step 3: Implementar**

Crear `internal/identify/match.go`:

```go
// Package identify matches versions to TMDB movies, enriches them with TMDB
// and Wikidata data, and keeps doing so in the background while offline.
package identify

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// MatcherVersion changes whenever the matching algorithm or its thresholds
// change, so earlier automatic results are recomputed.
const MatcherVersion = 1

// A version is assigned automatically when its best candidate scores at
// least AutoThreshold and beats the runner-up by at least AutoMargin.
const (
	AutoThreshold = 0.80
	AutoMargin    = 0.10
)

const (
	searchLimit    = 10 // search results scored
	directorChecks = 3  // best candidates whose credits are fetched
	keptCandidates = 5  // candidates stored for review
)

// API is the part of the TMDB client the matcher uses.
type API interface {
	SearchMovie(ctx context.Context, query string, year int, lang string) ([]tmdb.Result, error)
	FindIMDb(ctx context.Context, imdbID, lang string) ([]tmdb.Result, error)
	Movie(ctx context.Context, id int, lang string) (tmdb.Details, error)
}

// Query is what is known about a version from its names and .nfo.
type Query struct {
	Title    string
	Year     int
	Director string
	IMDbID   string
	// Folder is the parse of the version's folder name, tried when the
	// query itself is not conclusive (nil when the folder does not look like
	// a movie name).
	Folder *Query
}

// Key identifies the query; a stored result made for another key is stale.
func (q Query) Key() string {
	k := fmt.Sprintf("%s|%d|%s|%s", q.Title, q.Year, q.Director, q.IMDbID)
	if q.Folder != nil {
		k += "|" + q.Folder.Key()
	}
	return k
}

// Search returns scored candidates, best first, and the details fetched on
// the way (in lang), which enrichment can reuse. An IMDb id that TMDB knows
// yields that movie alone with score 1.
func Search(ctx context.Context, api API, lang string, q Query) ([]store.Candidate, map[int]tmdb.Details, error) {
	details := map[int]tmdb.Details{}
	if q.IMDbID != "" {
		rs, err := api.FindIMDb(ctx, q.IMDbID, lang)
		if err != nil {
			return nil, nil, err
		}
		if len(rs) > 0 {
			c := candidate(rs[0])
			c.Score = 1
			return []store.Candidate{c}, details, nil
		}
	}
	variants := titleVariants(q.Title)
	if len(variants) == 0 {
		return []store.Candidate{}, details, nil
	}
	hits, err := search(ctx, api, variants[0], q.Year, lang)
	if err != nil {
		return nil, nil, err
	}
	cands := score(q, variants, hits)
	// An alternative title in brackets ("Solyaris [Solaris]") is searched
	// too when the main one is not conclusive.
	for _, v := range variants[1:] {
		if conclusive(cands) {
			break
		}
		more, err := search(ctx, api, v, q.Year, lang)
		if err != nil {
			return nil, nil, err
		}
		hits = merge(hits, more, false)
		cands = score(q, variants, hits)
	}
	// Files are often named with the English title, which a localized
	// search shows neither as title nor as original title: when the result is
	// not conclusive, the English titles are looked at too.
	if lang != fallbackLang && !conclusive(cands) {
		en, err := search(ctx, api, variants[0], q.Year, fallbackLang)
		if err != nil {
			return nil, nil, err
		}
		hits = merge(hits, en, true)
		cands = score(q, variants, hits)
	}
	if q.Director != "" {
		for i := range min(directorChecks, len(cands)) {
			d, err := api.Movie(ctx, cands[i].TMDBID, lang)
			if err != nil {
				return nil, nil, err
			}
			details[d.ID] = d
			var names []string
			for _, p := range d.Directors() {
				names = append(names, p.Name)
			}
			if directorMatches(q.Director, names) {
				cands[i].Score += directorWeight
			}
		}
		sortCandidates(cands)
	}
	return cands, details, nil
}

// Identify decides what a query is: StatusAuto with the movie, or
// StatusUnmatched with the best candidates for review. When the query is not
// conclusive its folder query is tried, and the better outcome is kept.
func Identify(ctx context.Context, api API, lang string, q Query) (store.Identification, map[int]tmdb.Details, error) {
	id, details, err := identify(ctx, api, lang, q)
	if err != nil {
		return id, nil, err
	}
	if q.Folder != nil && id.Status != store.StatusAuto {
		alt, more, err := identify(ctx, api, lang, *q.Folder)
		if err != nil {
			return id, nil, err
		}
		for k, d := range more {
			details[k] = d
		}
		if alt.Status == store.StatusAuto || alt.Confidence > id.Confidence {
			id = alt
		}
	}
	id.Query = q.Key()
	return id, details, nil
}

func identify(ctx context.Context, api API, lang string, q Query) (store.Identification, map[int]tmdb.Details, error) {
	cands, details, err := Search(ctx, api, lang, q)
	if err != nil {
		return store.Identification{}, nil, err
	}
	id := store.Identification{Status: store.StatusUnmatched, MatcherVersion: MatcherVersion}
	if len(cands) > 0 {
		if conclusive(cands) {
			id.Status, id.TMDBID = store.StatusAuto, cands[0].TMDBID
		}
		id.Confidence = cands[0].Score
	}
	id.Candidates = cands[:min(keptCandidates, len(cands))]
	return id, details, nil
}

// fallbackLang is the language of the second search and of missing texts.
const fallbackLang = "en-US"

// hit is a search result plus its English title when known.
type hit struct {
	tmdb.Result
	english string
}

// titleVariants splits "La piel dura (L'argent de poche)" into the title
// outside brackets followed by each bracketed part; a title without
// brackets is its own only variant.
func titleVariants(title string) []string {
	var out []string
	outside := strings.TrimSpace(strings.Join(strings.Fields(bracketRe.ReplaceAllString(title, " ")), " "))
	if outside != "" {
		out = append(out, outside)
	}
	for _, m := range bracketRe.FindAllStringSubmatch(title, -1) {
		if v := strings.TrimSpace(m[1]); v != "" {
			out = append(out, v)
		}
	}
	return out
}

var bracketRe = regexp.MustCompile(`[(\[]([^()\[\]]*)[)\]]`)

// search looks a title up with its year, then without it if nothing comes
// back, keeping at most searchLimit results.
func search(ctx context.Context, api API, title string, year int, lang string) ([]hit, error) {
	rs, err := api.SearchMovie(ctx, title, year, lang)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 && year > 0 {
		if rs, err = api.SearchMovie(ctx, title, 0, lang); err != nil {
			return nil, err
		}
	}
	hits := make([]hit, 0, min(len(rs), searchLimit))
	for _, r := range rs[:min(len(rs), searchLimit)] {
		hits = append(hits, hit{Result: r})
	}
	return hits, nil
}

// merge adds the movies of more that hits lacks, up to searchLimit in total.
// With english set, more comes from an English search and its titles are
// recorded as the English titles of the movies.
func merge(hits, more []hit, english bool) []hit {
	pos := map[int]int{}
	for i, h := range hits {
		pos[h.ID] = i
	}
	for _, h := range more {
		if english {
			h.english = h.Title
		}
		if i, ok := pos[h.ID]; ok {
			if english {
				hits[i].english = h.Title
			}
		} else if len(hits) < searchLimit {
			pos[h.ID] = len(hits)
			hits = append(hits, h)
		}
	}
	return hits
}

// score rates hits on title and year (rescaled to 0..1 when no director is
// known, since the director share cannot be earned), best first. The title
// share is the best similarity between any variant of the parsed title and
// the movie's localized, original or English title.
func score(q Query, variants []string, hits []hit) []store.Candidate {
	cands := make([]store.Candidate, len(hits))
	for i, h := range hits {
		cands[i] = candidate(h.Result)
		t := 0.0
		for _, v := range variants {
			t = max(t, titleScore(v, h.Title, h.OriginalTitle), titleWeight*similarity(v, h.english))
		}
		cands[i].Score = t + yearScore(q.Year, h.Year())
		if q.Director == "" {
			cands[i].Score /= titleWeight + yearWeight
		}
	}
	sortCandidates(cands)
	return cands
}

// conclusive reports whether the best candidate would be assigned.
func conclusive(cands []store.Candidate) bool {
	return len(cands) > 0 && cands[0].Score >= AutoThreshold &&
		(len(cands) == 1 || cands[0].Score-cands[1].Score >= AutoMargin)
}

func candidate(r tmdb.Result) store.Candidate {
	return store.Candidate{TMDBID: r.ID, Title: r.Title, OriginalTitle: r.OriginalTitle, Year: r.Year(), PosterPath: r.PosterPath}
}

// sortCandidates orders by score, keeping TMDB's order (popularity) on ties.
func sortCandidates(cs []store.Candidate) {
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].Score > cs[j].Score })
}
```

- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/identify/ -v`
Expected: todos PASS; `TestCorpus` loguea `automáticas: 23/30`.

- [ ] **Step 5: Commit**

```bash
git add internal/identify
git commit -m "feat(identify): TMDB matching with calibrated thresholds

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: IMDb desde `.nfo` y conversión de películas

**Files:**
- Create: `internal/identify/nfo.go`
- Create: `internal/identify/enrich.go`
- Test: `internal/identify/nfo_test.go`
- Test: `internal/identify/enrich_test.go`

`imdbFromNFOs` lee hasta 64 KiB de cada `.nfo` buscando `tt\d{7,8}`. `movieFrom` convierte detalles de TMDB en `store.Movie` (directores sin repetir, 10 del reparto por orden, colección) y `fillFromWikidata` completa solo lo vacío.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/identify/nfo_test.go`:

```go
package identify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIMDbFromNFOs(t *testing.T) {
	disk := t.TempDir()
	app := filepath.Join(disk, "app")
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(disk, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.nfo", "no id here")
	write("b.nfo", "imdb: http://www.imdb.com/title/tt0071129/ and tt0000001")
	write("big.nfo", strings.Repeat("x", nfoLimit)+" tt0079944")
	if got := imdbFromNFOs(app, []string{"../missing.nfo", "../a.nfo", "../b.nfo"}); got != "tt0071129" {
		t.Errorf("got %q", got)
	}
	if got := imdbFromNFOs(app, []string{"../big.nfo"}); got != "" {
		t.Errorf("read past the limit: %q", got)
	}
}
```

Crear `internal/identify/enrich_test.go`:

```go
package identify

import (
	"testing"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

func TestMovieFromAndWikidata(t *testing.T) {
	d := details(7857, "Amarcord", "1973-12-18", "Rimini.", "Federico Fellini", "Federico Fellini")
	d.Credits.Crew[1].ID = d.Credits.Crew[0].ID
	for i := range 12 {
		d.Credits.Cast = append(d.Credits.Cast, tmdb.Person{ID: i, Name: "Actor", Order: 11 - i})
	}
	d.Collection = &struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{9, "Col"}
	m := movieFrom(d, "es-ES")
	if len(m.Directors) != 1 || len(m.Cast) != castLimit || m.Cast[0].ID != 11 || m.CollectionID != 9 || m.Year != 1973 ||
		m.Genres == nil || m.Countries == nil {
		t.Fatalf("got %+v", m)
	}
	fillFromWikidata(&m, wikidata.Entity{QID: "Q1", IMDbID: "tt1", Countries: []string{"IT"}, Directors: []string{"X"}, Year: 1900})
	if !m.WikidataDone || m.WikidataID != "Q1" || m.IMDbID != "tt1" || m.Countries[0] != "IT" ||
		m.Directors[0].Name != "Federico Fellini" || m.Year != 1973 {
		t.Fatalf("filled %+v", m)
	}
	empty := store.Movie{}
	fillFromWikidata(&empty, wikidata.Entity{})
	if !empty.WikidataDone || empty.WikidataID != "" {
		t.Fatalf("no entity %+v", empty)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/identify/`
Expected: FAIL de compilación (`undefined: imdbFromNFOs`, `movieFrom`, …).

- [ ] **Step 3: Implementar**

Crear `internal/identify/nfo.go`:

```go
package identify

import (
	"io"
	"os"
	"regexp"

	"cinexplorer/internal/appdir"
)

var imdbRe = regexp.MustCompile(`\btt\d{7,8}\b`)

// nfoLimit is how much of a .nfo is read; release notes put the IMDb link
// near the top, and some .nfo files are huge ASCII art.
const nfoLimit = 64 << 10

// imdbFromNFOs returns the first IMDb id found in the .nfo files (catalog
// paths, in order), or "". Unreadable files are skipped.
func imdbFromNFOs(appDir string, paths []string) string {
	for _, p := range paths {
		f, err := os.Open(appdir.Abs(appDir, p))
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(f, nfoLimit))
		f.Close()
		if err != nil {
			continue
		}
		if id := imdbRe.Find(b); id != nil {
			return string(id)
		}
	}
	return ""
}
```

Crear `internal/identify/enrich.go`:

```go
package identify

import (
	"sort"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// castLimit is how many cast members are kept, in billing order.
const castLimit = 10

// movieFrom converts TMDB details fetched in lang into a stored movie.
func movieFrom(d tmdb.Details, lang string) store.Movie {
	m := store.Movie{
		TMDBID: d.ID, Title: d.Title, OriginalTitle: d.OriginalTitle, Year: d.Year(), Runtime: d.Runtime,
		OriginalLang: d.OriginalLanguage, Overview: d.Overview, PosterPath: d.PosterPath,
		BackdropPath: d.BackdropPath, IMDbID: d.IMDbID, Language: lang,
		Directors: []store.Person{}, Cast: []store.CastMember{}, Genres: []string{}, Countries: []string{},
	}
	for _, p := range d.Directors() {
		m.Directors = append(m.Directors, store.Person{ID: p.ID, Name: p.Name})
	}
	cast := append([]tmdb.Person(nil), d.Credits.Cast...)
	sort.SliceStable(cast, func(i, j int) bool { return cast[i].Order < cast[j].Order })
	for _, p := range cast[:min(castLimit, len(cast))] {
		m.Cast = append(m.Cast, store.CastMember{ID: p.ID, Name: p.Name, Character: p.Character})
	}
	for _, g := range d.Genres {
		m.Genres = append(m.Genres, g.Name)
	}
	for _, c := range d.ProductionCountries {
		m.Countries = append(m.Countries, c.ISO)
	}
	if d.Collection != nil {
		m.CollectionID, m.Collection = d.Collection.ID, d.Collection.Name
	}
	return m
}

// fillFromWikidata completes the fields TMDB left empty and marks the movie
// as looked up.
func fillFromWikidata(m *store.Movie, e wikidata.Entity) {
	m.WikidataDone = true
	if e.QID != "" {
		m.WikidataID = e.QID
	}
	if m.IMDbID == "" {
		m.IMDbID = e.IMDbID
	}
	if len(m.Countries) == 0 && len(e.Countries) > 0 {
		m.Countries = e.Countries
	}
	if len(m.Directors) == 0 {
		for _, name := range e.Directors {
			m.Directors = append(m.Directors, store.Person{Name: name})
		}
	}
	if m.Year == 0 {
		m.Year = e.Year
	}
}
```

- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/identify/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/identify
git commit -m "feat(identify): IMDb ids from .nfo, TMDB/Wikidata to movie

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Runner de identificación en segundo plano

**Files:**
- Create: `internal/identify/runner.go`
- Test: `internal/identify/runner_test.go`

Fases identificar → enriquecer → Wikidata → prefetch de imágenes. Guarda cada 20. Sin red: estado `offline` y reintento programado (5 min, duplicando hasta 1 h; se reinicia tras una corrida sin problemas). Token inválido: `badToken`. `Trigger` corre en segundo plano y encola a lo sumo una corrida más; `Adopt` valida y guarda una película para la identificación manual. La consulta suma la alternativa por carpeta cuando el nombre de la carpeta trae año.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/identify/runner_test.go`:

```go
package identify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/httpx"
	"cinexplorer/internal/images"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// fixture builds a catalog with an Amarcord version and a Stalker DVD whose
// folder has a .nfo with the IMDb id, and a runner on a fake TMDB.
func fixture(t *testing.T) (*Runner, *fakeAPI, *fakeWikidata) {
	t.Helper()
	disk := t.TempDir()
	appDir := filepath.Join(disk, "cinexplorer")
	nfo := filepath.Join(disk, "cine", "d", "movie.nfo")
	if err := os.MkdirAll(filepath.Dir(nfo), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nfo, []byte("Stalker\nhttps://www.imdb.com/title/tt0079944/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(appDir, "cinexplorer.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.SyncFiles([]store.FileRow{
		{Path: "../cine/a/Amarcord.avi", Size: 700, MTime: 1, Fingerprint: "a1", Kind: "video"},
		{Path: "../cine/d/VIDEO_TS/VTS_01_1.VOB", Size: 900, MTime: 1, Fingerprint: "vob", Kind: "dvd"},
		{Path: "../cine/d/movie.nfo", Size: 50, MTime: 1, Kind: "info"},
	}, []string{"../cine"})
	setVersions(t, st, "Amarcord")

	api := &fakeAPI{
		search: map[string][]tmdb.Result{
			"Amarcord|1973|es-ES": {{ID: 7857, Title: "Amarcord", OriginalTitle: "Amarcord", ReleaseDate: "1973-12-18"}},
		},
		find: map[string][]tmdb.Result{"tt0079944": {{ID: 1398, Title: "Stalker", ReleaseDate: "1979-05-25"}}},
		movies: map[string]tmdb.Details{
			"7857|es-ES": details(7857, "Amarcord", "1973-12-18", "Rimini.", "Federico Fellini"),
			"1398|es-ES": details(1398, "Stalker", "1979-05-25", "", "Andrei Tarkovsky"),
			"1398|en-US": details(1398, "Stalker", "1979-05-25", "The Zone.", "Andrei Tarkovsky"),
		},
	}
	wd := &fakeWikidata{ents: map[int]wikidata.Entity{
		1398: {QID: "Q498906", IMDbID: "tt0079944", Countries: []string{"SU"}, Year: 1979},
	}}
	r := &Runner{AppDir: appDir, Store: st, TMDB: api, Wikidata: wd, Language: "es-ES",
		After: func(time.Duration, func()) func() { return func() {} }}
	return r, api, wd
}

// setVersions (re)builds the two versions, the first with the given title.
func setVersions(t *testing.T, st *store.Store, title string) {
	t.Helper()
	if err := st.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/a", Parsed: nameparse.Parsed{Title: title, Year: 1973}, Size: 700, Parts: 1,
			Members: []grouping.Member{{Path: "../cine/a/Amarcord.avi", Role: grouping.RoleMain}}},
		{Dir: "../cine/d", Parsed: nameparse.Parsed{Title: "d"}, Size: 900, Parts: 1,
			Members: []grouping.Member{{Path: "../cine/d/VIDEO_TS/VTS_01_1.VOB", Role: grouping.RoleMain}}},
	}); err != nil {
		t.Fatal(err)
	}
}

func current(t *testing.T, st *store.Store, fp string) *store.Identification {
	t.Helper()
	ts, err := st.IdentifyTargets()
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range ts {
		if x.Fingerprint == fp {
			return x.Current
		}
	}
	t.Fatalf("no target %s", fp)
	return nil
}

func TestRunIdentifiesAndEnriches(t *testing.T) {
	r, api, wd := fixture(t)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	a, d := current(t, r.Store, "a1"), current(t, r.Store, "vob")
	if a.Status != store.StatusAuto || a.TMDBID != 7857 || a.Query != "Amarcord|1973||" || a.MatcherVersion != MatcherVersion {
		t.Fatalf("amarcord %+v", a)
	}
	if d.Status != store.StatusAuto || d.TMDBID != 1398 || d.Confidence != 1 || d.Query != "d|0||tt0079944" {
		t.Fatalf("stalker %+v", d)
	}
	m, ok, _ := r.Store.Movie(7857)
	if !ok || m.Overview != "Rimini." || m.Directors[0].Name != "Federico Fellini" || !m.WikidataDone || m.WikidataID != "" {
		t.Fatalf("amarcord movie %+v", m)
	}
	s, _, _ := r.Store.Movie(1398)
	if s.Overview != "The Zone." || s.Language != "es-ES" || s.WikidataID != "Q498906" || !slices.Equal(s.Countries, []string{"SU"}) {
		t.Fatalf("stalker movie %+v", s)
	}
	// Amarcord's details came with the match (no director parsed, so no),
	// Stalker needed es-ES plus en-US for the empty overview.
	if n := api.called("movie 1398|"); n != 2 {
		t.Fatalf("stalker fetched %d times: %v", n, api.calls)
	}
	if len(wd.calls) != 1 || len(wd.calls[0]) != 2 {
		t.Fatalf("wikidata calls %v", wd.calls)
	}
	st := r.Status()
	if st.State != StateIdle || st.ToIdentify != 2 || st.Identified != 2 || st.ToEnrich != 2 || st.Enriched != 2 {
		t.Fatalf("status %+v", st)
	}

	// Nothing changed: a second run makes no TMDB calls.
	before := len(api.calls)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(api.calls) != before {
		t.Fatalf("second run called %v", api.calls[before:])
	}
}

func TestRunReusesMatchDetails(t *testing.T) {
	r, api, _ := fixture(t)
	// With a parsed director the matcher fetches credits; enrichment reuses them.
	api.search["Amarcord|1973|es-ES"] = []tmdb.Result{{ID: 7857, Title: "Amarcord", ReleaseDate: "1973-12-18"}}
	r.Store.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/a", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973, Director: "Fellini"}, Size: 700, Parts: 1,
			Members: []grouping.Member{{Path: "../cine/a/Amarcord.avi", Role: grouping.RoleMain}}},
	})
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a := current(t, r.Store, "a1"); a.Status != store.StatusAuto || a.Confidence != 1 {
		t.Fatalf("amarcord %+v", a)
	}
	if n := api.called("movie 7857|es-ES"); n != 1 {
		t.Fatalf("fetched %d times", n)
	}
}

func TestRunKeepsCorrectionsAndRematchesStale(t *testing.T) {
	r, api, _ := fixture(t)
	ctx := context.Background()
	r.Run(ctx)
	if err := r.Store.SetCorrection("a1", store.StatusIgnored, 0); err != nil {
		t.Fatal(err)
	}
	// Even a changed name does not redo a correction.
	setVersions(t, r.Store, "Amarcord!")
	api.calls = nil
	r.Run(ctx)
	if api.called("search") != 0 || current(t, r.Store, "a1").Status != store.StatusIgnored {
		t.Fatalf("correction redone: %v", api.calls)
	}

	// A changed name redoes an automatic match.
	r.Store.ResetIdentification("a1")
	r.Run(ctx)
	setVersions(t, r.Store, "Amarcord")
	api.calls = nil
	r.Run(ctx)
	if api.called("search Amarcord|1973") != 1 || current(t, r.Store, "a1").Query != "Amarcord|1973||" {
		t.Fatalf("stale query not redone: %v", api.calls)
	}

	// So does a result from an older matcher.
	old := *current(t, r.Store, "a1")
	old.MatcherVersion = MatcherVersion - 1
	r.Store.SaveIdentifications([]store.Identification{old})
	api.calls = nil
	r.Run(ctx)
	if api.called("search") != 1 {
		t.Fatalf("old matcher result not redone: %v", api.calls)
	}
}

func TestRunOfflineSchedulesRetries(t *testing.T) {
	r, api, _ := fixture(t)
	var delays []time.Duration
	cancels := 0
	r.After = func(d time.Duration, f func()) func() {
		delays = append(delays, d)
		return func() { cancels++ }
	}
	api.err = fmt.Errorf("%w: dial tcp", httpx.ErrOffline)
	ctx := context.Background()
	for range 6 {
		if err := r.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	want := []time.Duration{5 * time.Minute, 10 * time.Minute, 20 * time.Minute, 40 * time.Minute, time.Hour, time.Hour}
	if !slices.Equal(delays, want) || r.Status().State != StateOffline {
		t.Fatalf("delays %v state %s", delays, r.Status().State)
	}
	if cancels != 5 {
		t.Fatalf("each run should cancel the pending retry: %d", cancels)
	}
	if current(t, r.Store, "a1") != nil {
		t.Fatal("offline run stored a result")
	}

	api.err = nil
	r.Run(ctx)
	if r.Status().State != StateIdle || current(t, r.Store, "a1") == nil {
		t.Fatalf("after reconnect: %+v", r.Status())
	}
	api.err = fmt.Errorf("%w: again", httpx.ErrOffline)
	r.Store.ResetIdentification("a1")
	r.Run(ctx)
	if delays[len(delays)-1] != 5*time.Minute {
		t.Fatalf("backoff not reset: %v", delays)
	}
}

func TestRunBadTokenAndNoToken(t *testing.T) {
	r, api, _ := fixture(t)
	scheduled := false
	r.After = func(time.Duration, func()) func() { scheduled = true; return func() {} }
	api.err = tmdb.ErrUnauthorized
	if err := r.Run(context.Background()); !errors.Is(err, tmdb.ErrUnauthorized) {
		t.Fatalf("err %v", err)
	}
	if r.Status().State != StateBadToken || scheduled {
		t.Fatalf("state %s scheduled %v", r.Status().State, scheduled)
	}
	r.TMDB = nil
	if err := r.Run(context.Background()); err != nil || r.Status().State != StateNoToken {
		t.Fatalf("no token: %v %s", err, r.Status().State)
	}
}

func TestEnrichInvalidatesDeletedMovie(t *testing.T) {
	r, api, _ := fixture(t)
	api.search["Amarcord|1973|es-ES"] = []tmdb.Result{{ID: 999, Title: "Amarcord", ReleaseDate: "1973-01-01"}}
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	// 999 matched, but TMDB says it does not exist: the match is forgotten.
	if a := current(t, r.Store, "a1"); a != nil {
		t.Fatalf("amarcord %+v", a)
	}
}

func TestWikidataErrorSkipsPhase(t *testing.T) {
	r, _, wd := fixture(t)
	wd.err = errors.New("wikidata: bad JSON")
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m, _, _ := r.Store.Movie(7857); m.WikidataDone || len(wd.calls) != 1 {
		t.Fatalf("movie %+v calls %v", m, wd.calls)
	}
	wd.err = fmt.Errorf("%w: timeout", httpx.ErrOffline)
	r.Run(context.Background())
	if r.Status().State != StateOffline {
		t.Fatalf("state %s", r.Status().State)
	}
}

type fetcher struct{ paths []string }

func (f *fetcher) Image(ctx context.Context, path, size string) ([]byte, error) {
	f.paths = append(f.paths, size+path)
	return []byte("x"), nil
}

func TestPrefetch(t *testing.T) {
	r, api, _ := fixture(t)
	d := api.movies["7857|es-ES"]
	d.PosterPath, d.BackdropPath = "/p.jpg", "/b.jpg"
	api.movies["7857|es-ES"] = d
	f := &fetcher{}
	r.Images = &images.Cache{Dir: t.TempDir(), Fetch: f}
	r.Prefetch = PrefetchPosters
	r.Run(context.Background())
	if !slices.Equal(f.paths, []string{"w342/p.jpg"}) {
		t.Fatalf("posters: %v", f.paths)
	}
	r.Prefetch = PrefetchAll
	r.Run(context.Background())
	if !slices.Equal(f.paths, []string{"w342/p.jpg", "w1280/b.jpg"}) {
		t.Fatalf("all: %v", f.paths)
	}
}

func TestAdopt(t *testing.T) {
	r, _, _ := fixture(t)
	ctx := context.Background()
	if err := r.Adopt(ctx, 7857); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.Store.Movie(7857); !ok {
		t.Fatal("not stored")
	}
	if err := r.Adopt(ctx, 5); !errors.Is(err, tmdb.ErrNotFound) {
		t.Fatalf("unknown id: %v", err)
	}
}

func TestTriggerRunsInBackground(t *testing.T) {
	r, _, _ := fixture(t)
	r.Trigger()
	r.Trigger() // coalesced into one follow-up run
	deadline := time.Now().Add(5 * time.Second)
	for current(t, r.Store, "a1") == nil || r.busy() {
		if time.Now().After(deadline) {
			t.Fatal("background run did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("runner still busy: %v", err)
	}
}

func (r *Runner) busy() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func TestRunFallsBackToFolderName(t *testing.T) {
	r, api, _ := fixture(t)
	dir := "../cine/Los Gauchos Judíos (Juan José Jusid, Argentina, 1974)"
	r.Store.SyncFiles([]store.FileRow{{Path: dir + "/cd 01.avi", Size: 700, MTime: 1, Fingerprint: "g1", Kind: "video"}}, []string{"../cine"})
	r.Store.ReplaceVersions([]grouping.Version{{Dir: dir, Parsed: nameparse.Parsed{Title: "Los Gauchos Judios Rip mentecato"},
		Size: 700, Parts: 1, Members: []grouping.Member{{Path: dir + "/cd 01.avi", Role: grouping.RoleMain}}}})
	api.search["Los Gauchos Judíos|1974|es-ES"] = []tmdb.Result{{ID: 537898, Title: "Los gauchos judíos", ReleaseDate: "1975-05-01"}}
	api.movies["537898|es-ES"] = details(537898, "Los gauchos judíos", "1975-05-01", "Entre Ríos.", "Juan José Jusid")
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	g := current(t, r.Store, "g1")
	if g.Status != store.StatusAuto || g.TMDBID != 537898 ||
		g.Query != "Los Gauchos Judios Rip mentecato|0||"+"|Los Gauchos Judíos|1974|Juan José Jusid|" {
		t.Fatalf("gauchos %+v", g)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/identify/`
Expected: FAIL de compilación (`undefined: Runner`, …).

- [ ] **Step 3: Implementar**

Crear `internal/identify/runner.go`:

```go
package identify

import (
	"context"
	"errors"
	"log"
	"path"
	"sync"
	"time"

	"cinexplorer/internal/httpx"
	"cinexplorer/internal/images"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// Runner states, as shown by the UI.
const (
	StateIdle     = "idle"
	StateRunning  = "running"
	StateOffline  = "offline"  // waiting to retry
	StateNoToken  = "noToken"  // no TMDB token configured
	StateBadToken = "badToken" // TMDB rejected the token
)

// Image prefetch modes (config.json "imagePrefetch").
const (
	PrefetchNone    = "none"
	PrefetchPosters = "posters"
	PrefetchAll     = "all"
)

var ErrBusy = errors.New("ya hay una identificación en curso")

// Retry delays while offline: the first, doubled up to the last.
const (
	firstRetry = 5 * time.Minute
	maxRetry   = time.Hour
)

// saveBatch is how many identifications are committed at once.
const saveBatch = 20

type Status struct {
	State      string `json:"state"`
	ToIdentify int    `json:"toIdentify"`
	Identified int    `json:"identified"`
	ToEnrich   int    `json:"toEnrich"`
	Enriched   int    `json:"enriched"`
}

// Wikidata is the part of the Wikidata client the runner uses.
type Wikidata interface {
	Lookup(ctx context.Context, ids []int, lang string) (map[int]wikidata.Entity, error)
}

type Runner struct {
	AppDir   string
	Store    *store.Store
	TMDB     API      // nil: no token, nothing to do
	Wikidata Wikidata // nil: skip the Wikidata phase
	Images   *images.Cache
	Language string
	Prefetch string // PrefetchNone, PrefetchPosters or PrefetchAll
	// After schedules f to run after d and returns a function that cancels
	// it; nil means time.AfterFunc. Tests replace it.
	After func(d time.Duration, f func()) (cancel func())

	mu      sync.Mutex
	status  Status
	running bool
	again   bool // a Trigger arrived during a run
	backoff time.Duration
	cancel  func() // pending retry
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.status
	if s.State == "" {
		s.State = StateIdle
	}
	return s
}

// Trigger starts a run in the background. A trigger during a run makes one
// more run follow it.
func (r *Runner) Trigger() {
	r.mu.Lock()
	if r.running {
		r.again = true
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()
	go func() {
		for {
			if err := r.run(context.Background()); err != nil {
				log.Printf("identificación: %v", err)
			}
			r.mu.Lock()
			if !r.again {
				r.running = false
				r.mu.Unlock()
				return
			}
			r.again = false
			r.mu.Unlock()
		}
	}()
}

// Run performs one run synchronously; ErrBusy if one is in progress.
func (r *Runner) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrBusy
	}
	r.running = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()
	return r.run(ctx)
}

func (r *Runner) run(ctx context.Context) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if r.TMDB == nil {
		r.status = Status{State: StateNoToken}
		r.mu.Unlock()
		return nil
	}
	r.status = Status{State: StateRunning}
	r.mu.Unlock()

	// Details fetched while matching, reused by enrichment in this run.
	fetched := map[int]tmdb.Details{}
	err := r.identifyAll(ctx, fetched)
	if err == nil {
		err = r.enrichAll(ctx, fetched)
	}
	if err == nil {
		err = r.wikidataAll(ctx)
	}
	if err == nil {
		err = r.prefetchAll(ctx)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case errors.Is(err, httpx.ErrOffline):
		r.status.State = StateOffline
		if r.backoff == 0 {
			r.backoff = firstRetry
		} else {
			r.backoff = min(2*r.backoff, maxRetry)
		}
		after := r.After
		if after == nil {
			after = func(d time.Duration, f func()) func() { t := time.AfterFunc(d, f); return func() { t.Stop() } }
		}
		r.cancel = after(r.backoff, r.Trigger)
		log.Printf("identificación: sin conexión, se reintenta en %v", r.backoff)
		return nil
	case errors.Is(err, tmdb.ErrUnauthorized):
		r.status.State = StateBadToken
		return err
	}
	r.status.State = StateIdle
	r.backoff = 0
	return err
}

// fatal errors stop a run: no network, bad token, or cancellation. Other
// errors only skip the item at hand.
func fatal(err error) bool {
	return errors.Is(err, httpx.ErrOffline) || errors.Is(err, tmdb.ErrUnauthorized) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

type job struct {
	fingerprint string
	query       Query
}

// needsMatch reports whether a fingerprint has to be (re)matched: never
// matched, or matched by the matcher for another query or an older version
// of it. Corrections are never redone.
func needsMatch(cur *store.Identification, q Query) bool {
	if cur == nil {
		return true
	}
	if cur.Status != store.StatusAuto && cur.Status != store.StatusUnmatched {
		return false
	}
	return cur.Query != q.Key() || cur.MatcherVersion < MatcherVersion
}

func (r *Runner) identifyAll(ctx context.Context, fetched map[int]tmdb.Details) error {
	targets, err := r.Store.IdentifyTargets()
	if err != nil {
		return err
	}
	var jobs []job
	for _, t := range targets {
		q := Query{Title: t.Title, Year: t.Year, Director: t.Director, IMDbID: t.IMDbID}
		if id := imdbFromNFOs(r.AppDir, t.NFOs); id != "" {
			q.IMDbID = id
		}
		// A folder named like a movie ("Title (Director, 1974)") helps when
		// the file names do not: several versions in one folder keep their
		// own, often poorer, names.
		if f := nameparse.Parse(path.Base(t.Dir)); f.Year > 0 && (f.Title != q.Title || f.Year != q.Year) {
			q.Folder = &Query{Title: f.Title, Year: f.Year, Director: f.Director}
		}
		if needsMatch(t.Current, q) {
			jobs = append(jobs, job{t.Fingerprint, q})
		}
	}
	r.mu.Lock()
	r.status.ToIdentify = len(jobs)
	r.mu.Unlock()

	var batch []store.Identification
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := r.Store.SaveIdentifications(batch)
		batch = batch[:0]
		return err
	}
	for _, j := range jobs {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, flush())
		}
		id, details, err := Identify(ctx, r.TMDB, r.Language, j.query)
		if fatal(err) {
			return errors.Join(err, flush())
		}
		if err != nil {
			log.Printf("no se pudo identificar %q: %v", j.query.Title, err)
			continue
		}
		for k, d := range details {
			fetched[k] = d
		}
		id.Fingerprint = j.fingerprint
		batch = append(batch, id)
		r.mu.Lock()
		r.status.Identified++
		r.mu.Unlock()
		if len(batch) >= saveBatch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

// Adopt fetches a movie from TMDB and stores it; it is how a manual
// identification is validated. tmdb.ErrNotFound means the id does not exist.
func (r *Runner) Adopt(ctx context.Context, id int) error {
	if r.TMDB == nil {
		return errors.New("sin token de TMDB")
	}
	m, err := r.fetchMovie(ctx, id, nil)
	if err != nil {
		return err
	}
	return r.Store.SaveMovie(m)
}

// fetchMovie gets a movie in the configured language, completing an empty
// title or overview in English.
func (r *Runner) fetchMovie(ctx context.Context, id int, fetched map[int]tmdb.Details) (store.Movie, error) {
	d, ok := fetched[id]
	if !ok {
		var err error
		if d, err = r.TMDB.Movie(ctx, id, r.Language); err != nil {
			return store.Movie{}, err
		}
	}
	m := movieFrom(d, r.Language)
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
	}
	return m, nil
}

func (r *Runner) enrichAll(ctx context.Context, fetched map[int]tmdb.Details) error {
	ids, err := r.Store.MoviesToEnrich(r.Language)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.status.ToEnrich = len(ids)
	r.mu.Unlock()
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		m, err := r.fetchMovie(ctx, id, fetched)
		switch {
		case errors.Is(err, tmdb.ErrNotFound):
			log.Printf("la película %d ya no existe en TMDB", id)
			if err := r.Store.InvalidateMovie(id); err != nil {
				return err
			}
			continue
		case fatal(err):
			return err
		case err != nil:
			log.Printf("no se pudo obtener la película %d: %v", id, err)
			continue
		}
		if err := r.Store.SaveMovie(m); err != nil {
			return err
		}
		r.mu.Lock()
		r.status.Enriched++
		r.mu.Unlock()
	}
	return nil
}

func (r *Runner) wikidataAll(ctx context.Context) error {
	if r.Wikidata == nil {
		return nil
	}
	for {
		ms, err := r.Store.PendingWikidata(wikidata.BatchSize)
		if err != nil || len(ms) == 0 {
			return err
		}
		ids := make([]int, len(ms))
		for i, m := range ms {
			ids[i] = m.TMDBID
		}
		ents, err := r.Wikidata.Lookup(ctx, ids, r.Language)
		if fatal(err) {
			return err
		}
		if err != nil {
			// Not worth retrying in a loop: the next run tries again.
			log.Printf("wikidata: %v", err)
			return nil
		}
		for _, m := range ms {
			fillFromWikidata(&m, ents[m.TMDBID])
			if err := r.Store.SaveMovie(m); err != nil {
				return err
			}
		}
	}
}

type imageRef struct {
	kind images.Kind
	path string
}

func (r *Runner) prefetchAll(ctx context.Context) error {
	if r.Images == nil || (r.Prefetch != PrefetchPosters && r.Prefetch != PrefetchAll) {
		return nil
	}
	ms, err := r.Store.Movies()
	if err != nil {
		return err
	}
	for _, m := range ms {
		wanted := []imageRef{{images.Poster, m.PosterPath}}
		if r.Prefetch == PrefetchAll {
			wanted = append(wanted, imageRef{images.Backdrop, m.BackdropPath})
		}
		for _, w := range wanted {
			if w.path == "" || r.Images.Has(w.kind, m.TMDBID) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if _, err := r.Images.Get(ctx, w.kind, m.TMDBID, w.path); fatal(err) {
				return err
			} else if err != nil {
				log.Printf("imagen %s de %d: %v", w.kind, m.TMDBID, err)
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/identify/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/identify
git commit -m "feat(identify): background runner with offline retries

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Configuración `imagePrefetch` y aviso de fin de escaneo

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/scan/scan.go`
- Test: `internal/config/config_test.go`
- Test: `internal/scan/scan_test.go`

`config.json` suma `imagePrefetch` (`none` por defecto, `posters`, `all`). `Scanner.OnDone` se llama al terminar cada escaneo; `main` engancha ahí `Runner.Trigger`.

- [ ] **Step 1: Escribir los tests que fallan**

En `internal/config/config_test.go`, reemplazá:

```go
func TestSaveThenLoad(t *testing.T) {
	dir := t.TempDir()
	in := Config{Roots: []string{"../x"}, TMDBToken: "tok", Language: "es-ES"}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
```

por:

```go
func TestSaveThenLoad(t *testing.T) {
	dir := t.TempDir()
	in := Config{Roots: []string{"../x"}, TMDBToken: "tok", Language: "es-ES", ImagePrefetch: "posters"}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
```

Agregá al final de `internal/scan/scan_test.go`:

```go
func TestScanCallsOnDone(t *testing.T) {
	_, app, st := setup(t)
	calls := 0
	sc := &Scanner{AppDir: app, Roots: []string{"../missing"}, Store: st, OnDone: func() { calls++ }}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("OnDone called %d times", calls)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/config/ ./internal/scan/`
Expected: FAIL de compilación (`unknown field ImagePrefetch`, `unknown field OnDone`).

- [ ] **Step 3: Implementar**

En `internal/config/config.go`, reemplazá:

```go
type Config struct {
	Roots     []string `json:"roots"`     // catalog-form paths, relative to the app dir
	TMDBToken string   `json:"tmdbToken"` // used from stage 3 on
	Language  string   `json:"language"`
}
```

por:

```go
type Config struct {
	Roots     []string `json:"roots"`     // catalog-form paths, relative to the app dir
	TMDBToken string   `json:"tmdbToken"` // TMDB v4 read access token
	Language  string   `json:"language"`
	// ImagePrefetch downloads images ahead: "posters", "all", or "none"
	// (the default: on demand only).
	ImagePrefetch string `json:"imagePrefetch,omitempty"`
}
```

En `internal/scan/scan.go`, reemplazá:

```go
	Probe    func(ctx context.Context, path string) (probe.Info, error)
	ProbeEnv store.ProbeEnv

	mu     sync.Mutex
```

por:

```go
	Probe    func(ctx context.Context, path string) (probe.Info, error)
	ProbeEnv store.ProbeEnv
	// OnDone runs after every scan, finished or not (the identification
	// runner hangs here).
	OnDone func()

	mu     sync.Mutex
```

En `internal/scan/scan.go`, reemplazá:

```go
	}
	s.mu.Unlock()
	return err
}
```

por:

```go
	}
	s.mu.Unlock()
	if s.OnDone != nil {
		s.OnDone()
	}
	return err
}
```


- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/config/ ./internal/scan/`
Expected: ambos `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/config internal/scan
git commit -m "feat: imagePrefetch setting and scan completion hook

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: API: sin identificar, búsqueda manual, correcciones e imágenes

**Files:**
- Create: `internal/server/identify.go`
- Modify: `internal/server/server.go`
- Test: `internal/server/identify_test.go`

Endpoints del spec §10. `movie`/`extra` validan el id con `Runner.Adopt` (404 si TMDB no lo conoce, 503 sin red) y relanzan el Runner para Wikidata e imágenes. En modo consulta → 409 como `/api/scan`. `/img/{kind}/{id}.jpg` toma la ruta de `movies` o, para un candidato aún no guardado, de `?p=`. `/api/status` suma `identify`.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `internal/server/identify_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// fakeTMDB knows Amarcord (7857) and nothing else.
type fakeTMDB struct{ images []string }

func (f *fakeTMDB) SearchMovie(ctx context.Context, q string, year int, lang string) ([]tmdb.Result, error) {
	if strings.EqualFold(q, "amarcord") {
		return []tmdb.Result{{ID: 7857, Title: "Amarcord", ReleaseDate: "1973-12-18", PosterPath: "/p.jpg"}}, nil
	}
	return nil, nil
}

func (f *fakeTMDB) FindIMDb(ctx context.Context, id, lang string) ([]tmdb.Result, error) {
	if id == "tt0071129" {
		return []tmdb.Result{{ID: 7857, Title: "Amarcord", ReleaseDate: "1973-12-18"}}, nil
	}
	return nil, nil
}

func (f *fakeTMDB) Movie(ctx context.Context, id int, lang string) (tmdb.Details, error) {
	if id != 7857 {
		return tmdb.Details{}, tmdb.ErrNotFound
	}
	return tmdb.Details{ID: 7857, Title: "Amarcord", ReleaseDate: "1973-12-18", Overview: "Rimini.",
		PosterPath: "/p.jpg", BackdropPath: "/b.jpg"}, nil
}

func (f *fakeTMDB) Image(ctx context.Context, path, size string) ([]byte, error) {
	f.images = append(f.images, size+path)
	return []byte("\xff\xd8\xff\xe0 jpeg"), nil
}

func identifyServer(t *testing.T) (*Server, *fakeTMDB) {
	t.Helper()
	s, _ := newServer(t)
	f := &fakeTMDB{}
	s.TMDB, s.Language = f, "es-ES"
	s.Identifier = &identify.Runner{Store: s.Store, TMDB: f, Language: "es-ES"}
	s.Images = &images.Cache{Dir: t.TempDir(), Fetch: f}
	return s, f
}

func post(t *testing.T, s *Server, body string) int {
	t.Helper()
	return request(s.Handler(), "POST", "/api/identify", body, "application/json", "127.0.0.1").Code
}

func TestIdentifyManualAndVersions(t *testing.T) {
	s, _ := identifyServer(t)
	if code := post(t, s, `{"fingerprint":"f1","action":"movie","tmdbId":7857}`); code != http.StatusNoContent {
		t.Fatalf("status %d", code)
	}
	rec := request(s.Handler(), "GET", "/api/versions", "", "", "127.0.0.1")
	var vs []store.VersionView
	json.Unmarshal(rec.Body.Bytes(), &vs)
	if len(vs) != 1 || vs[0].Fingerprint != "f1" || vs[0].Movie == nil || vs[0].Movie.TMDBID != 7857 ||
		vs[0].Identification.Status != store.StatusManual {
		t.Fatalf("versions %s", rec.Body)
	}
}

func TestIdentifyErrors(t *testing.T) {
	s, _ := identifyServer(t)
	cases := []struct {
		body string
		want int
	}{
		{`{"fingerprint":"f1","action":"movie","tmdbId":5}`, http.StatusNotFound}, // unknown to TMDB
		{`{"fingerprint":"zz","action":"ignore"}`, http.StatusNotFound},           // unknown fingerprint
		{`{"fingerprint":"f1","action":"movie"}`, http.StatusBadRequest},          // no id
		{`{"fingerprint":"f1","action":"delete"}`, http.StatusBadRequest},         // unknown action
		{`{"fingerprint":"f1","action":"ignore"}`, http.StatusNoContent},
		{`{"fingerprint":"f1","action":"extra","tmdbId":7857}`, http.StatusNoContent},
		{`{"fingerprint":"f1","action":"reset"}`, http.StatusNoContent},
		{`not json`, http.StatusBadRequest},
	}
	for _, c := range cases {
		if code := post(t, s, c.body); code != c.want {
			t.Errorf("%s: status %d, want %d", c.body, code, c.want)
		}
	}
	s.ReadOnly = true
	if code := post(t, s, `{"fingerprint":"f1","action":"ignore"}`); code != http.StatusConflict {
		t.Errorf("read-only: %d", code)
	}
}

func TestUnidentifiedEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	s.Store.SaveIdentifications([]store.Identification{{Fingerprint: "f1", Status: store.StatusUnmatched,
		Candidates: []store.Candidate{{TMDBID: 7857, Title: "Amarcord", Score: 0.7}}}})
	rec := request(s.Handler(), "GET", "/api/unidentified", "", "", "127.0.0.1")
	var u []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil || len(u) != 1 || u[0]["dir"] != "../cine" {
		t.Fatalf("body %s", rec.Body)
	}
	if c, ok := u[0]["candidates"].([]any); !ok || len(c) != 1 {
		t.Fatalf("candidates %v", u[0]["candidates"])
	}
}

func TestSearchEndpoint(t *testing.T) {
	s, _ := identifyServer(t)
	for _, q := range []string{"/api/tmdb/search?q=Amarcord&year=1973", "/api/tmdb/search?q=tt0071129"} {
		rec := request(s.Handler(), "GET", q, "", "", "127.0.0.1")
		var cs []store.Candidate
		if err := json.Unmarshal(rec.Body.Bytes(), &cs); err != nil || len(cs) != 1 || cs[0].TMDBID != 7857 || cs[0].Score < 0.99 {
			t.Fatalf("%s: %s", q, rec.Body)
		}
	}
	s.TMDB = nil
	if rec := request(s.Handler(), "GET", "/api/tmdb/search?q=x", "", "", "127.0.0.1"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("without token: %d", rec.Code)
	}
}

func TestImageEndpoint(t *testing.T) {
	s, f := identifyServer(t)
	get := func(url string) int { return request(s.Handler(), "GET", url, "", "", "127.0.0.1").Code }
	// A candidate not stored yet: its path comes in the query.
	if code := get("/img/poster/7857.jpg?p=/p.jpg"); code != 200 {
		t.Fatalf("candidate poster %d", code)
	}
	s.Identifier.Adopt(context.Background(), 7857)
	if code := get("/img/backdrop/7857.jpg"); code != 200 {
		t.Fatalf("backdrop %d", code)
	}
	if code := get("/img/poster/7857.jpg"); code != 200 || len(f.images) != 2 {
		t.Fatalf("cached poster %d, downloads %v", code, f.images)
	}
	rec := request(s.Handler(), "GET", "/img/poster/7857.jpg", "", "", "127.0.0.1")
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
	for _, bad := range []string{"/img/thumb/7857.jpg", "/img/poster/abc.jpg", "/img/poster/7857.png", "/img/poster/1.jpg",
		"/img/poster/2.jpg?p=../../etc/passwd"} {
		if code := get(bad); code != http.StatusNotFound {
			t.Errorf("%s: %d", bad, code)
		}
	}
}

func TestStatusIncludesIdentify(t *testing.T) {
	s, _ := identifyServer(t)
	rec := request(s.Handler(), "GET", "/api/status", "", "", "127.0.0.1")
	var body struct {
		Identify *identify.Status `json:"identify"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Identify == nil || body.Identify.State != identify.StateIdle {
		t.Fatalf("status %s", rec.Body)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/server/`
Expected: FAIL de compilación (`s.TMDB undefined`, …).

- [ ] **Step 3: Implementar**

Crear `internal/server/identify.go`:

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

func (s *Server) unidentified(w http.ResponseWriter, r *http.Request) {
	u, err := s.Store.Unidentified()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, u)
}

// search is the manual TMDB search: by title (and optional year), or by
// IMDb id when q is one.
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	if s.TMDB == nil {
		http.Error(w, "sin token de TMDB", http.StatusServiceUnavailable)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	query := identify.Query{Title: q, Year: year}
	if imdbIDRe.MatchString(q) {
		query = identify.Query{IMDbID: q}
	}
	cands, _, err := identify.Search(r.Context(), s.TMDB, s.Language, query)
	if err != nil {
		tmdbError(w, err)
		return
	}
	writeJSON(w, cands)
}

// identify records the user's decision about a version's fingerprint.
func (s *Server) identify(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly || s.Identifier == nil {
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
	var err error
	switch req.Action {
	case "movie", "extra":
		if req.TMDBID <= 0 {
			http.Error(w, "falta tmdbId", http.StatusBadRequest)
			return
		}
		if err := s.Identifier.Adopt(r.Context(), req.TMDBID); err != nil {
			tmdbError(w, err)
			return
		}
		status := store.StatusManual
		if req.Action == "extra" {
			status = store.StatusExtra
		}
		if err = s.Store.SetCorrection(req.Fingerprint, status, req.TMDBID); err == nil {
			s.Identifier.Trigger() // Wikidata and images for the new movie
		}
	case "ignore":
		err = s.Store.SetCorrection(req.Fingerprint, store.StatusIgnored, 0)
	case "reset":
		if err = s.Store.ResetIdentification(req.Fingerprint); err == nil {
			s.Identifier.Trigger()
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
	case errors.Is(err, tmdb.ErrUnauthorized):
		http.Error(w, err.Error(), http.StatusBadGateway)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// image serves /img/{poster|backdrop}/{tmdbId}.jpg from the cache,
// downloading it once when missing. A candidate that is not a stored movie
// yet passes its TMDB path as ?p=.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	kind := images.Kind(r.PathValue("kind"))
	idText, ok := strings.CutSuffix(r.PathValue("file"), ".jpg")
	id, err := strconv.Atoi(idText)
	if s.Images == nil || !images.ValidKind(kind) || !ok || err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	path := r.URL.Query().Get("p")
	m, found, err := s.Store.Movie(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if found {
		path = m.PosterPath
		if kind == images.Backdrop {
			path = m.BackdropPath
		}
	}
	b, err := s.Images.Get(r.Context(), kind, id, path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(b)
}
```

En `internal/server/server.go`, reemplazá:

```go

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/store"
```

por:

```go

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/store"
```

En `internal/server/server.go`, reemplazá:

```go
	Opener   func(target string) error
	Revealer func(target string) error
}
```

por:

```go
	Opener   func(target string) error
	Revealer func(target string) error

	TMDB       identify.API     // nil without a TMDB token
	Identifier *identify.Runner // nil in read-only mode
	Images     *images.Cache
	Language   string
}
```

En `internal/server/server.go`, reemplazá:

```go
	mux.HandleFunc("POST /api/open", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Opener) }))
	mux.HandleFunc("POST /api/reveal", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Revealer) }))
	return localOnly(mux)
}
```

por:

```go
	mux.HandleFunc("POST /api/open", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Opener) }))
	mux.HandleFunc("POST /api/reveal", jsonOnly(func(w http.ResponseWriter, r *http.Request) { s.withFile(w, r, s.Revealer) }))
	mux.HandleFunc("GET /api/unidentified", s.unidentified)
	mux.HandleFunc("GET /api/tmdb/search", s.search)
	mux.HandleFunc("POST /api/identify", jsonOnly(s.identify))
	mux.HandleFunc("GET /img/{kind}/{file}", s.image)
	return localOnly(mux)
}
```

En `internal/server/server.go`, reemplazá:

```go
		st = s.Scanner.Status()
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "scan": st})
}
```

por:

```go
		st = s.Scanner.Status()
	}
	var id *identify.Status
	if s.Identifier != nil {
		st := s.Identifier.Status()
		id = &st
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "scan": st, "identify": id})
}
```


- [ ] **Step 4: Verificar que pasan**

Run: `go test ./internal/server/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/server/identify.go internal/server/identify_test.go internal/server/server.go
git commit -m "feat(server): identification API and image endpoint

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 14: Conexión en `main`

**Files:**
- Modify: `cmd/cinexplorer/main.go`

Crea el cliente TMDB solo si hay token (un `*tmdb.Client` nil dentro de la interfaz no se leería como "sin token"), la caché de imágenes, el Runner (fuera del modo consulta) con Wikidata, y engancha `OnDone`.

- [ ] **Step 1: Implementar**

En `cmd/cinexplorer/main.go`, reemplazá:

```go
	"cinexplorer/internal/appdir"
	"cinexplorer/internal/config"
	"cinexplorer/internal/platform"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/server"
	"cinexplorer/internal/store"
)

func main() {
```

por:

```go
	"cinexplorer/internal/appdir"
	"cinexplorer/internal/config"
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/platform"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/server"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// version identifies the build to Wikidata (User-Agent).
const version = "0.3"

func main() {
```

En `cmd/cinexplorer/main.go`, reemplazá:

```go
	defer st.Close()

	srv := &server.Server{AppDir: appDir, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal}
	if readOnly {
		log.Print("el directorio de la app no es escribible: modo consulta")
	} else {
		srv.Scanner = &scan.Scanner{AppDir: appDir, Roots: cfg.Roots, Store: st}
		go func() {
			if err := srv.Scanner.Run(context.Background()); err != nil {
```

por:

```go
	defer st.Close()

	srv := &server.Server{AppDir: appDir, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal,
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
```


- [ ] **Step 2: Verificar**

Run: `go vet ./... && go build ./...`
Expected: sin salida.

- [ ] **Step 3: Commit**

```bash
git add cmd/cinexplorer/main.go
git commit -m "feat: wire TMDB identification into the app

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 15: Página mínima: identidad y "Sin identificar"

**Files:**
- Modify: `internal/server/web/index.html`

Cada versión muestra título TMDB · año · director e indicador (`92%`, `manual`, `sin identificar`…). Pestaña "Sin identificar" con candidatos (afiche chico vía `/img/poster/<id>.jpg?p=…`), botón **Confirmar** y búsqueda manual (título o `tt…`, Enter o botón). La línea de estado muestra el progreso de la identificación y los estados `offline` / `noToken` / `badToken`.

- [ ] **Step 1: Implementar**

En `internal/server/web/index.html`, reemplazá:

```html
  .best { font-style: normal; font-size: 10px; font-weight: 700; letter-spacing: .08em; color: var(--bg);
          background: #3fb950; border-radius: 3px; padding: 1px 6px; margin-left: 6px; vertical-align: 2px; }
  @media (max-width: 600px) { header, main { padding-left: 16px; padding-right: 16px; } .row { grid-template-columns: 1fr; } }
</style>
```

por:

```html
  .best { font-style: normal; font-size: 10px; font-weight: 700; letter-spacing: .08em; color: var(--bg);
          background: #3fb950; border-radius: 3px; padding: 1px 6px; margin-left: 6px; vertical-align: 2px; }
  .conf { font-size: 11px; color: var(--muted); margin-left: 6px; }
  .conf.warn { color: #d29922; }
  .cands { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 8px; }
  .cand { display: flex; gap: 8px; align-items: center; background: #171a20; border-radius: 4px; padding: 6px 8px; }
  .cand img { width: 34px; height: 51px; object-fit: cover; background: #262a31; border-radius: 2px; }
  .cand button, .find button { background: none; border: 1px solid #30353d; color: var(--fg); border-radius: 3px;
                 padding: 2px 8px; cursor: pointer; font: inherit; font-size: 12px; }
  .find { display: flex; gap: 6px; margin-top: 8px; }
  .find input { margin: 0; max-width: 260px; padding: 4px 8px; }
  @media (max-width: 600px) { header, main { padding-left: 16px; padding-right: 16px; } .row { grid-template-columns: 1fr; } }
</style>
```

En `internal/server/web/index.html`, reemplazá:

```html
<header>
  <b>CINEXPLORER</b>
  <nav><button data-view="versions" class="on">Versiones</button><button data-view="copies">Copias idénticas</button></nav>
  <div id="status"></div>
</header>
```

por:

```html
<header>
  <b>CINEXPLORER</b>
  <nav><button data-view="versions" class="on">Versiones</button><button data-view="unidentified">Sin identificar</button><button data-view="copies">Copias idénticas</button></nav>
  <div id="status"></div>
</header>
```

En `internal/server/web/index.html`, reemplazá:

```html
<script>
const $ = (s) => document.querySelector(s);
let view = 'versions', versions = [], copies = [], wasRunning = false;

const gb = (n) => (n / 1e9).toFixed(1) + ' GB';
```

por:

```html
<script>
const $ = (s) => document.querySelector(s);
let view = 'versions', versions = [], copies = [], unidentified = [], wasRunning = false;

const gb = (n) => (n / 1e9).toFixed(1) + ' GB';
```

En `internal/server/web/index.html`, reemplazá:

```html
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
  const tech = [v.resolution, v.source, v.codec && codecName(v.codec)].filter(Boolean).map(esc);
  if (v.durationMs) tech.push(duration(v.durationMs));
```

por:

```html
  const r = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!r.ok) alert(await r.text());
  return r.ok;
}

async function load() {
  [versions, copies, unidentified] = await Promise.all([
    fetch('/api/versions').then((r) => r.json()),
    fetch('/api/duplicates').then((r) => r.json()),
    fetch('/api/unidentified').then((r) => r.json()),
  ]);
  render();
}

const pct = (x) => Math.round(x * 100) + '%';

function identity(v) {
  const id = v.identification;
  if (!id) return '';
  if (v.movie) return id.status === 'manual' ? '<span class="conf">manual</span>' : `<span class="conf">${pct(id.confidence)}</span>`;
  const label = { unmatched: 'sin identificar', ignored: 'no es película', extra: 'extra', manual: 'id inválido' }[id.status];
  return label ? `<span class="conf warn">${label}</span>` : '';
}

function candidateBox(fp, c) {
  const img = `/img/poster/${c.tmdbId}.jpg` + (c.posterPath ? '?p=' + encodeURIComponent(c.posterPath) : '');
  return `<div class="cand"><img src="${img}" alt="" loading="lazy" onerror="this.style.visibility='hidden'">
    <div><div>${esc(c.title)} <span class="meta">${c.year || ''}</span></div>
    <div class="meta">${c.originalTitle && c.originalTitle !== c.title ? esc(c.originalTitle) + ' · ' : ''}${pct(c.score)}</div></div>
    <button data-confirm="${esc(fp)}" data-id="${c.tmdbId}">Confirmar</button></div>`;
}

function unidentifiedRow(u) {
  const tokens = [u.title, u.year, u.director].filter(Boolean).map(esc).join(' · ');
  return `<div class="row"><div>
      <div class="t">${tokens || '—'}</div>
      <div class="path">${esc(u.dir)}</div>
      <div class="cands" id="c-${esc(u.fingerprint)}">${u.candidates.map((c) => candidateBox(u.fingerprint, c)).join('') || '<span class="meta">Sin candidatos.</span>'}</div>
      <div class="find"><input data-q="${esc(u.fingerprint)}" placeholder="Título o tt1234567" value="${esc(u.title)}"><button data-find="${esc(u.fingerprint)}">Buscar</button></div>
    </div><div class="acts"></div></div>`;
}

function versionRow(v) {
  const main = v.files.find((f) => f.role === 'main');
  const m = v.movie;
  const director = m ? m.directors.map((d) => d.name).join(', ') : v.director;
  const tech = [v.resolution, v.source, v.codec && codecName(v.codec)].filter(Boolean).map(esc);
  if (v.durationMs) tech.push(duration(v.durationMs));
```

En `internal/server/web/index.html`, reemplazá:

```html
  if (subs.length) media.push('subs ' + esc(subs.join(', ')));
  return `<div class="row"><div>
      <div class="t">${esc(v.title)}${v.best ? '<em class="best">MEJOR</em>' : ''} <span>${v.year || ''} ${v.director ? '· ' + esc(v.director) : ''}</span></div>
      <div class="meta">${tech.join(' · ')}</div>
      ${media.length ? `<div class="meta">${media.join(' · ')}</div>` : ''}
```

por:

```html
  if (subs.length) media.push('subs ' + esc(subs.join(', ')));
  return `<div class="row"><div>
      <div class="t">${esc(m ? m.title : v.title)}${v.best ? '<em class="best">MEJOR</em>' : ''} <span>${(m ? m.year : v.year) || ''} ${director ? '· ' + esc(director) : ''}</span>${identity(v)}</div>
      <div class="meta">${tech.join(' · ')}</div>
      ${media.length ? `<div class="meta">${media.join(' · ')}</div>` : ''}
```

En `internal/server/web/index.html`, reemplazá:

```html
function render() {
  const q = $('#q').value.trim().toLowerCase();
  const rows = view === 'versions'
    ? versions.filter((v) => !q || [v.title, v.director, v.dir].join(' ').toLowerCase().includes(q)).map(versionRow)
    : copies.filter((c) => !q || c.paths.join(' ').toLowerCase().includes(q)).map(copyRow);
  $('#list').innerHTML = rows.length ? rows.join('') : '<div class="empty">Sin resultados.</div>';
```

por:

```html
function render() {
  const q = $('#q').value.trim().toLowerCase();
  const text = (v) => [v.title, v.director, v.dir, v.movie && v.movie.title, v.movie && v.movie.originalTitle].join(' ').toLowerCase();
  const rows = view === 'versions'
    ? versions.filter((v) => !q || text(v).includes(q)).map(versionRow)
    : view === 'unidentified'
    ? unidentified.filter((u) => !q || text(u).includes(q)).map(unidentifiedRow)
    : copies.filter((c) => !q || c.paths.join(' ').toLowerCase().includes(q)).map(copyRow);
  $('#list').innerHTML = rows.length ? rows.join('') : '<div class="empty">Sin resultados.</div>';
```

En `internal/server/web/index.html`, reemplazá:

```html
    render();
  }
  if (b.dataset.open) post('/api/open', { path: b.dataset.open });
  if (b.dataset.reveal) post('/api/reveal', { path: b.dataset.reveal });
});
$('#q').addEventListener('input', render);

async function poll() {
```

por:

```html
    render();
  }
  if (b.dataset.confirm) {
    b.disabled = true;
    post('/api/identify', { fingerprint: b.dataset.confirm, action: 'movie', tmdbId: Number(b.dataset.id) })
      .then((ok) => (ok ? load() : (b.disabled = false)));
  }
  if (b.dataset.find) findCandidates(b.dataset.find);
  if (b.dataset.open) post('/api/open', { path: b.dataset.open });
  if (b.dataset.reveal) post('/api/reveal', { path: b.dataset.reveal });
});
$('#q').addEventListener('input', render);
document.addEventListener('keydown', (e) => { if (e.key === 'Enter' && e.target.dataset.q) findCandidates(e.target.dataset.q); });

async function findCandidates(fp) {
  const q = document.querySelector(`input[data-q="${CSS.escape(fp)}"]`).value.trim();
  if (!q) return;
  const box = document.getElementById('c-' + fp);
  const r = await fetch('/api/tmdb/search?q=' + encodeURIComponent(q));
  if (!r.ok) { box.innerHTML = `<span class="meta">${esc(await r.text())}</span>`; return; }
  const cs = await r.json();
  box.innerHTML = cs.map((c) => candidateBox(fp, c)).join('') || '<span class="meta">Sin resultados.</span>';
}

function identifyStatus(id) {
  if (!id) return '';
  return {
    running: id.toEnrich ? `Completando datos… ${id.enriched}/${id.toEnrich}` : `Identificando… ${id.identified}/${id.toIdentify}`,
    offline: 'Sin conexión: se reintenta más tarde',
    noToken: 'Falta tmdbToken en config.json',
    badToken: 'El token de TMDB no es válido',
  }[id.state] || '';
}

async function poll() {
```

En `internal/server/web/index.html`, reemplazá:

```html
    const s = await fetch('/api/status').then((r) => r.json());
    const sc = s.scan;
    $('#status').textContent = s.readOnly ? 'Modo consulta (solo lectura)'
      : sc.running && sc.toProbe ? `Analizando… ${sc.probed}/${sc.toProbe}`
      : sc.running ? `Escaneando… ${sc.files} archivos`
      : sc.lastError ? 'Error: ' + sc.lastError
      : `${versions.length} versiones`;
    if (wasRunning && !sc.running) load();
    wasRunning = sc.running;
    delay = sc.running ? 1000 : 5000;
  } catch (e) {
    console.error('poll', e);
```

por:

```html
    const s = await fetch('/api/status').then((r) => r.json());
    const sc = s.scan;
    const running = sc.running || (!!s.identify && s.identify.state === 'running');
    $('#status').textContent = s.readOnly ? 'Modo consulta (solo lectura)'
      : sc.running && sc.toProbe ? `Analizando… ${sc.probed}/${sc.toProbe}`
      : sc.running ? `Escaneando… ${sc.files} archivos`
      : sc.lastError ? 'Error: ' + sc.lastError
      : identifyStatus(s.identify) || `${versions.length} versiones · ${unidentified.length} sin identificar`;
    if (wasRunning && !running) load();
    wasRunning = running;
    delay = running ? 1000 : 5000;
  } catch (e) {
    console.error('poll', e);
```


- [ ] **Step 2: Verificar**

Run: `go test ./internal/server/`
Expected: `ok` (`TestServesIndex` sigue pasando).

- [ ] **Step 3: Commit**

```bash
git add internal/server/web/index.html
git commit -m "feat(web): show identification and review unidentified versions

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 16: README, verificación final y prueba real

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Documentar el token y `imagePrefetch`**

En `README.md`, después del párrafo que termina en "…no es obligatorio.", agregá:

````markdown
Para identificar las películas se usa TMDB (y Wikidata para completar datos).
Pegá tu token de lectura de TMDB (API Read Access Token, v4) en `config.json`:

```json
{
  "roots": ["../cine", "../cine-ordenar"],
  "tmdbToken": "eyJ…",
  "language": "es-ES",
  "imagePrefetch": "none"
}
```

Sin red, la identificación se reintenta sola. Las que no se pueden decidir
aparecen en la pestaña "Sin identificar" con candidatos para confirmar.
`imagePrefetch` baja imágenes por adelantado: `"posters"` (afiches), `"all"`
(afiches y escenas) o `"none"` (solo las que se van mirando).
````

Y en el bloque de "Desarrollo", después de la línea de `CINEXPLORER_PROBE_CORPUS`, agregá:

```bash
CINEXPLORER_TMDB_RECORD=1 go test ./internal/identify -run Corpus -v
```

- [ ] **Step 2: Suite completa y compilación cruzada**

Run: `go vet ./... && go test -count=1 ./... && GOOS=linux GOARCH=amd64 go vet ./... && GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/cinexplorer`
Expected: todos los paquetes `ok`, sin errores de compilación cruzada.

- [ ] **Step 3: Prueba real (manual, con token)**

Con un directorio de prueba al lado de la colección (p. ej. `D:\cinexplorer-prueba\`) que tenga el binario y un `config.json` con `"roots": ["../cine/1970s"]`, el token y `"imagePrefetch": "posters"`, correr `cinexplorer.exe -port 18733` y comprobar:
- `GET /api/status` termina con `identify.state = "idle"` y `identified = toIdentify`.
- En la página, las versiones identificadas muestran título TMDB y director correctos (referencia al prototipar: 33 automáticas de 41, todas correctas; *Amarcord* y *Annie Hall* son archivos vacíos y quedan sin identificar).
- La pestaña "Sin identificar" lista candidatos con afiche; **Confirmar** en uno lo saca de la lista y la versión pasa a `manual`.
- `cache/posters/` tiene los afiches.
Borrar el directorio de prueba al terminar.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: TMDB token, imagePrefetch and corpus recording

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
