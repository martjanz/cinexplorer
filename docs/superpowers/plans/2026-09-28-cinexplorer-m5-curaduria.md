# Etapa 5: Curaduría — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Listas propias del usuario (películas identificadas o contenido sin identificar) e importación de carpetas `Collections/` como listas.

**Architecture:** Cuatro tablas de datos del usuario (`lists`, `list_entries`, `collection_folders`, `collection_seen`) que un reescaneo no toca; el `Snapshot` las trae. El catálogo (puro, sin SQL) resuelve cada entrada (`movie:<id>` o `fp:<huella>`) contra los ítems de Explorar, y una lista es la faceta `lista` de Explorar con su propio orden (`agregado-lista`). Las carpetas pendientes de `Collections/` se calculan de las rutas del snapshot. La API suma `/api/lists…` y `/api/collections…`; la interfaz suma la página Listas, chips en las fichas y la pestaña Revisar → Colecciones.

**Tech Stack:** Go (stdlib, `modernc.org/sqlite`), Svelte 5 + Vite + Vitest.

**Spec:** `docs/superpowers/specs/2026-09-28-cinexplorer-m5-curaduria-design.md`

## Global Constraints

- Solo lectura sobre los videos: nunca se mueven, renombran ni borran (README). Borrar una lista nunca toca archivos.
- Rutas del catálogo con `/`, relativas a la carpeta de la app (`../cine/...`).
- Catálogos viejos abiertos en modo consulta pueden no tener tablas nuevas: todo acceso a las cuatro tablas nuevas se protege con un flag `hasLists`, como `hasPartLinks`.
- Toda escritura de la API pasa por `jsonOnly` y responde 409 `modo consulta: el catálogo no se puede modificar` con `ReadOnly`.
- Nombres de lista: se recortan los espacios de los extremos; vacíos o de más de 100 caracteres → 400; repetidos sin distinguir mayúsculas → 409.
- Textos de la interfaz y del servidor en español (es-AR).
- Mensajes de commit sin `Co-Authored-By` (instrucción global del usuario, prevalece sobre cualquier otra).
- `internal/server/dist` está commiteado: después de tocar `web/`, `npm run build` y commitear el resultado (CI lo verifica). Se hace una vez, en la Task 13.
- Usar `git add` con rutas explícitas (hay archivos locales sin versionar).

## Estructura de archivos

| Archivo | Responsabilidad |
|---|---|
| `internal/store/schema.sql` (modificar) | Tablas `lists`, `list_entries`, `collection_folders`, `collection_seen` |
| `internal/store/store.go` (modificar) | Flag `hasLists` |
| `internal/store/lists.go` (crear) | Tipos, errores, `RefMovie`/`RefFingerprint`, `ListName`, CRUD de listas y entradas, carga en el snapshot |
| `internal/store/collections.go` (crear) | `ImportCollection`, `DismissCollection`, carga de decisiones |
| `internal/store/snapshot.go` (modificar) | `Snapshot.Lists`, `Snapshot.CollectionFolders` |
| `internal/store/lists_test.go`, `collections_test.go` (crear) | Pruebas del store |
| `internal/catalog/items.go` (modificar) | Campos `lists`, `listAdded` del ítem; `build` adjunta listas |
| `internal/catalog/lists.go` (crear) | Resolución de entradas, `ItemRef`, `RefsTo`, `ListCards`, `listsOf`, `ApplyLists` |
| `internal/catalog/facets.go`, `sort.go` (modificar) | Faceta `lista`, orden `agregado-lista`, `SortQuery` |
| `internal/catalog/detail.go` (modificar) | `Lists` en las fichas |
| `internal/catalog/collections.go` (crear) | Carpetas pendientes de `Collections/` |
| `internal/catalog/home.go` (modificar) | Fila `list` |
| `internal/catalog/lists_test.go`, `collections_test.go` (crear); `facets_test.go`, `home_test.go` (modificar) | Pruebas del catálogo |
| `internal/server/lists.go`, `collections.go` (crear) | Handlers |
| `internal/server/server.go`, `catalog.go` (modificar) | Rutas; Explorar con listas |
| `internal/server/lists_test.go`, `collections_test.go` (crear); `catalog_test.go` (modificar) | Pruebas del servidor |
| `web/src/lib/listas.js` (crear) + `listas.test.js` | Lógica pura de listas y Colecciones |
| `web/src/lib/api.js`, `facets.js`, `router.js` (+ tests) (modificar) | Llamadas, faceta y orden, rutas |
| `web/src/pages/Listas.svelte` (crear) | Página Listas |
| `web/src/components/ChipsListas.svelte`, `Colecciones.svelte` (crear) | Chips "+" de las fichas; pestaña Colecciones |
| `web/src/App.svelte`, `components/Nav.svelte`, `components/BarraFacetas.svelte`, `pages/Explorar.svelte`, `pages/Pelicula.svelte`, `pages/Version.svelte`, `pages/Revisar.svelte` (modificar) | Integración |
| `internal/server/dist/**` (regenerar), `README.md`, `README.es.md` | Build y documentación |

---

### Task 1: Store — listas y entradas

**Files:**
- Modify: `internal/store/schema.sql`, `internal/store/store.go`, `internal/store/snapshot.go`
- Create: `internal/store/lists.go`
- Test: `internal/store/lists_test.go`

**Interfaces:**
- Consumes: `querier`, `execQuerier` (ya existen en `identity.go` y `part_links.go`).
- Produces:
  - `var ErrListName, ErrListExists, ErrUnknownList error`
  - `type ListEntry struct { Ref string; AddedAt int64 }`
  - `type List struct { ID int64; Name string; CreatedAt, UpdatedAt int64; Entries []ListEntry }`
  - `func RefMovie(tmdbID int) string` → `"movie:<id>"`; `func RefFingerprint(fp string) string` → `"fp:<fp>"`
  - `func ListName(name string) (string, error)`
  - `func (s *Store) CreateList(name string) (List, error)`, `RenameList(id int64, name string) error`, `DeleteList(id int64) error`, `AddEntries(id int64, refs []string) error`, `RemoveEntries(id int64, refs []string) error`
  - internas: `createList(tx execQuerier, name string, now int64) (List, error)`, `listExists(tx querier, id int64) error`, `(s *Store) lists(tx querier) ([]List, error)`
  - `Snapshot.Lists []List` (por id, entradas por `added_at`)

- [ ] **Step 1: Escribir las pruebas que fallan**

Crear `internal/store/lists_test.go`:

```go
package store

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func listStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func snapLists(t *testing.T, s *Store) []List {
	t.Helper()
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return snap.Lists
}

func TestListName(t *testing.T) {
	long := strings.Repeat("á", 100)
	for in, want := range map[string]string{"  Noir ": "Noir", "Películas de Fellini": "Películas de Fellini", long: long} {
		if got, err := ListName(in); err != nil || got != want {
			t.Errorf("ListName(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "   ", strings.Repeat("a", 101)} {
		if _, err := ListName(in); !errors.Is(err, ErrListName) {
			t.Errorf("ListName(%q): %v", in, err)
		}
	}
}

func TestCreateRenameDeleteList(t *testing.T) {
	s := listStore(t)
	noir, err := s.CreateList(" Noir ")
	if err != nil || noir.ID == 0 || noir.Name != "Noir" || noir.CreatedAt == 0 || noir.UpdatedAt != noir.CreatedAt {
		t.Fatalf("create %+v, %v", noir, err)
	}
	if _, err := s.CreateList("NOIR"); !errors.Is(err, ErrListExists) {
		t.Fatalf("repeated name: %v", err)
	}
	if _, err := s.CreateList(""); !errors.Is(err, ErrListName) {
		t.Fatalf("empty name: %v", err)
	}
	// Case is ignored beyond ASCII too.
	if _, err := s.CreateList("Películas"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateList("PELÍCULAS"); !errors.Is(err, ErrListExists) {
		t.Fatalf("repeated accented name: %v", err)
	}
	if err := s.RenameList(noir.ID, "Cine negro"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameList(noir.ID, "cine NEGRO"); err != nil {
		t.Fatalf("its own name in other case: %v", err)
	}
	if err := s.RenameList(noir.ID, "películas"); !errors.Is(err, ErrListExists) {
		t.Fatalf("rename to a taken name: %v", err)
	}
	if err := s.RenameList(noir.ID, " "); !errors.Is(err, ErrListName) {
		t.Fatalf("rename to nothing: %v", err)
	}
	if err := s.RenameList(999, "Otra"); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("rename unknown: %v", err)
	}
	ls := snapLists(t, s)
	if len(ls) != 2 || ls[0].Name != "cine NEGRO" || ls[1].Name != "Películas" || len(ls[0].Entries) != 0 {
		t.Fatalf("lists %+v", ls)
	}
	if err := s.DeleteList(noir.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteList(noir.ID); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("delete twice: %v", err)
	}
	if ls := snapLists(t, s); len(ls) != 1 {
		t.Fatalf("lists %+v", ls)
	}
}

func TestListEntries(t *testing.T) {
	s := listStore(t)
	l, err := s.CreateList("Noir")
	if err != nil {
		t.Fatal(err)
	}
	if RefMovie(7857) != "movie:7857" || RefFingerprint("s1") != "fp:s1" {
		t.Fatalf("refs %q %q", RefMovie(7857), RefFingerprint("s1"))
	}
	if err := s.AddEntries(l.ID, []string{RefMovie(7857), RefFingerprint("s1")}); err != nil {
		t.Fatal(err)
	}
	first := snapLists(t, s)[0].Entries
	if len(first) != 2 || first[0].AddedAt == 0 {
		t.Fatalf("entries %+v", first)
	}
	// Adding again changes nothing: the entries keep their date.
	if err := s.AddEntries(l.ID, []string{RefMovie(7857)}); err != nil {
		t.Fatal(err)
	}
	if got := snapLists(t, s)[0].Entries; !reflect.DeepEqual(got, first) {
		t.Fatalf("entries %+v, want %+v", got, first)
	}
	if err := s.RemoveEntries(l.ID, []string{RefFingerprint("s1"), RefFingerprint("nada")}); err != nil {
		t.Fatal(err)
	}
	if got := snapLists(t, s)[0].Entries; len(got) != 1 || got[0].Ref != "movie:7857" {
		t.Fatalf("entries %+v", got)
	}
	if err := s.AddEntries(999, []string{RefMovie(1)}); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("add to unknown: %v", err)
	}
	if err := s.RemoveEntries(999, nil); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("remove from unknown: %v", err)
	}
	// Deleting the list deletes its entries.
	if err := s.DeleteList(l.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM list_entries`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("entries left %d, %v", n, err)
	}
}
```

- [ ] **Step 2: Correr las pruebas y ver que fallan**

Run: `go test ./internal/store/ -run 'TestListName|TestCreateRenameDeleteList|TestListEntries'`
Expected: FAIL de compilación (`undefined: ListName`, `CreateList`, …).

- [ ] **Step 3: Implementar**

Agregar al final de `internal/store/schema.sql`:

```sql

-- The user's lists. Rescans never touch them.
CREATE TABLE IF NOT EXISTS lists (
  id         INTEGER PRIMARY KEY,
  name       TEXT    NOT NULL UNIQUE COLLATE NOCASE,
  created_at INTEGER NOT NULL,          -- unix milliseconds
  updated_at INTEGER NOT NULL           -- last rename or change of entries
);

-- What each list holds: "movie:<tmdb id>" or "fp:<fingerprint>".
CREATE TABLE IF NOT EXISTS list_entries (
  list_id  INTEGER NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
  ref      TEXT    NOT NULL,
  added_at INTEGER NOT NULL,            -- unix milliseconds
  PRIMARY KEY (list_id, ref)
);

-- Folders of Collections/ the user decided on.
CREATE TABLE IF NOT EXISTS collection_folders (
  path       TEXT    PRIMARY KEY,       -- relative to the app dir, '/'-separated
  status     TEXT    NOT NULL,          -- imported | dismissed
  list_id    INTEGER REFERENCES lists(id) ON DELETE SET NULL,
  decided_at INTEGER NOT NULL           -- unix milliseconds
);

-- Contents of each folder already offered (imported or dismissed).
CREATE TABLE IF NOT EXISTS collection_seen (
  path        TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  PRIMARY KEY (path, fingerprint)
);
```

En `internal/store/store.go`, en `type Store`, agregar el flag debajo de `hasPartLinks`:

```go
	hasPartLinks bool
	hasLists     bool // lists, list_entries, collection_folders and collection_seen
```

y en `openDB` sumar la tabla al mapa de flags:

```go
	for name, dst := range map[string]*bool{"media": &s.hasMedia, "identifications": &s.hasIdentity, "part_links": &s.hasPartLinks, "lists": &s.hasLists} {
```

Crear `internal/store/lists.go`:

```go
package store

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors of the lists, shown by the interface as they are.
var (
	ErrListName    = errors.New("el nombre de la lista no puede estar vacío ni pasar de 100 caracteres")
	ErrListExists  = errors.New("ya existe una lista con ese nombre")
	ErrUnknownList = errors.New("la lista no existe")
)

const maxListName = 100

// ListEntry is something a list holds: a movie or a content.
type ListEntry struct {
	Ref     string // RefMovie or RefFingerprint
	AddedAt int64  // unix milliseconds
}

// List is one of the user's lists with its entries, oldest first.
type List struct {
	ID        int64
	Name      string
	CreatedAt int64 // unix milliseconds
	UpdatedAt int64 // last rename or change of entries
	Entries   []ListEntry
}

// RefMovie is the entry of a TMDB movie.
func RefMovie(tmdbID int) string { return "movie:" + strconv.Itoa(tmdbID) }

// RefFingerprint is the entry of a content, identified or not: it follows
// the content's identification.
func RefFingerprint(fp string) string { return "fp:" + fp }

// ListName trims a list name and checks it: not empty, at most 100
// characters.
func ListName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxListName {
		return "", ErrListName
	}
	return name, nil
}

// nameTaken reports whether a list other than except is called name,
// ignoring case (beyond ASCII, which the column's NOCASE does not).
func nameTaken(tx querier, name string, except int64) (bool, error) {
	rows, err := tx.Query(`SELECT id, name FROM lists`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n string
		if err := rows.Scan(&id, &n); err != nil {
			return false, err
		}
		if id != except && strings.EqualFold(n, name) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func listExists(tx querier, id int64) error {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM lists WHERE id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknownList
	}
	return nil
}

// createList adds an empty list inside tx.
func createList(tx execQuerier, name string, now int64) (List, error) {
	name, err := ListName(name)
	if err != nil {
		return List{}, err
	}
	taken, err := nameTaken(tx, name, 0)
	if err != nil {
		return List{}, err
	}
	if taken {
		return List{}, ErrListExists
	}
	res, err := tx.Exec(`INSERT INTO lists (name, created_at, updated_at) VALUES (?, ?, ?)`, name, now, now)
	if err != nil {
		return List{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return List{}, err
	}
	return List{ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Entries: []ListEntry{}}, nil
}

// CreateList adds an empty list.
func (s *Store) CreateList(name string) (List, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return List{}, err
	}
	defer tx.Rollback()
	l, err := createList(tx, name, time.Now().UnixMilli())
	if err != nil {
		return List{}, err
	}
	if err := tx.Commit(); err != nil {
		return List{}, err
	}
	return l, nil
}

// RenameList changes a list's name.
func (s *Store) RenameList(id int64, name string) error {
	name, err := ListName(name)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := listExists(tx, id); err != nil {
		return err
	}
	taken, err := nameTaken(tx, name, id)
	if err != nil {
		return err
	}
	if taken {
		return ErrListExists
	}
	if _, err := tx.Exec(`UPDATE lists SET name = ?, updated_at = ? WHERE id = ?`, name, time.Now().UnixMilli(), id); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteList deletes a list and its entries. A folder imported into it
// keeps its decision, without a list.
func (s *Store) DeleteList(id int64) error {
	res, err := s.db.Exec(`DELETE FROM lists WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknownList
	}
	return nil
}

// AddEntries adds refs to a list; the ones it already holds keep their
// date.
func (s *Store) AddEntries(id int64, refs []string) error {
	return s.changeEntries(id, refs, func(tx *sql.Tx, ref string, now int64) (sql.Result, error) {
		return tx.Exec(`INSERT OR IGNORE INTO list_entries (list_id, ref, added_at) VALUES (?, ?, ?)`, id, ref, now)
	})
}

// RemoveEntries removes refs from a list; refs it does not hold are
// ignored.
func (s *Store) RemoveEntries(id int64, refs []string) error {
	return s.changeEntries(id, refs, func(tx *sql.Tx, ref string, _ int64) (sql.Result, error) {
		return tx.Exec(`DELETE FROM list_entries WHERE list_id = ? AND ref = ?`, id, ref)
	})
}

// changeEntries runs change for each ref in one transaction and, when an
// entry was added or removed, dates the list's change.
func (s *Store) changeEntries(id int64, refs []string, change func(tx *sql.Tx, ref string, now int64) (sql.Result, error)) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := listExists(tx, id); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	changed := false
	for _, ref := range refs {
		res, err := change(tx, ref, now)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		changed = changed || n > 0
	}
	if changed {
		if _, err := tx.Exec(`UPDATE lists SET updated_at = ? WHERE id = ?`, now, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// lists loads every list, by id, with its entries, oldest first.
func (s *Store) lists(tx querier) ([]List, error) {
	out := []List{}
	if !s.hasLists {
		return out, nil
	}
	rows, err := tx.Query(`SELECT id, name, created_at, updated_at FROM lists ORDER BY id`)
	if err != nil {
		return nil, err
	}
	pos := map[int64]int{}
	for rows.Next() {
		l := List{Entries: []ListEntry{}}
		if err := rows.Scan(&l.ID, &l.Name, &l.CreatedAt, &l.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		pos[l.ID] = len(out)
		out = append(out, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	erows, err := tx.Query(`SELECT list_id, ref, added_at FROM list_entries ORDER BY list_id, added_at, ref`)
	if err != nil {
		return nil, err
	}
	defer erows.Close()
	for erows.Next() {
		var id int64
		var e ListEntry
		if err := erows.Scan(&id, &e.Ref, &e.AddedAt); err != nil {
			return nil, err
		}
		if i, ok := pos[id]; ok {
			out[i].Entries = append(out[i].Entries, e)
		}
	}
	return out, erows.Err()
}
```

En `internal/store/snapshot.go`, sumar el campo a `Snapshot` (debajo de `Movies`):

```go
	Lists           []List                     // by id, entries oldest first
```

y en `Snapshot()`, inicializarlo y cargarlo antes del corte por `hasIdentity`:

```go
	snap := Snapshot{Identifications: map[string]*Identification{}, Movies: map[int]Movie{}, Lists: []List{}}
	err := s.read(func(tx querier) error {
		if err := tx.QueryRow(`SELECT total_changes()`).Scan(&snap.Changes); err != nil {
			return err
		}
		vs, err := s.versions(tx)
		if err != nil {
			return err
		}
		snap.Versions = vs
		if snap.Lists, err = s.lists(tx); err != nil {
			return err
		}
		if !s.hasIdentity {
			return nil
		}
```

(el resto de `Snapshot()` queda igual).

- [ ] **Step 4: Correr las pruebas**

Run: `go test ./internal/store/`
Expected: PASS (todas, también las existentes).

- [ ] **Step 5: Commit**

```bash
git add internal/store/schema.sql internal/store/store.go internal/store/snapshot.go internal/store/lists.go internal/store/lists_test.go
git commit -m "feat(store): user lists and their entries"
```

---

### Task 2: Store — decisiones sobre carpetas de `Collections/`

**Files:**
- Create: `internal/store/collections.go`
- Modify: `internal/store/snapshot.go`
- Test: `internal/store/collections_test.go`

**Interfaces:**
- Consumes (Task 1): `createList`, `listExists`, `RefFingerprint`, `hasLists`, `execQuerier`.
- Produces:
  - `const CollectionImported = "imported"`, `CollectionDismissed = "dismissed"`
  - `type CollectionDecision struct { Status string; ListID int64; Seen map[string]bool }` (`ListID` 0: descartada o lista borrada)
  - `func (s *Store) ImportCollection(path, name string, listID int64, fps []string) (int64, error)` — `name != ""` crea la lista; si no, usa `listID`. Devuelve el id de la lista.
  - `func (s *Store) DismissCollection(path string, fps []string) error`
  - `Snapshot.CollectionFolders map[string]CollectionDecision` (por ruta)

- [ ] **Step 1: Escribir las pruebas que fallan**

Crear `internal/store/collections_test.go`:

```go
package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestImportAndDismissCollections(t *testing.T) {
	s := listStore(t)
	const kubrick, otra = "../cine/Collections/Kubrick", "../cine/Collections/Otra"
	id, err := s.ImportCollection(kubrick, " Kubrick ", 0, []string{"k1", "k2"})
	if err != nil || id == 0 {
		t.Fatalf("import %d, %v", id, err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	d := snap.CollectionFolders[kubrick]
	if d.Status != CollectionImported || d.ListID != id || len(d.Seen) != 2 || !d.Seen["k1"] || !d.Seen["k2"] {
		t.Fatalf("decision %+v", d)
	}
	if l := snap.Lists[0]; l.Name != "Kubrick" || len(l.Entries) != 2 || l.Entries[0].Ref != "fp:k1" || l.Entries[1].Ref != "fp:k2" {
		t.Fatalf("list %+v", l)
	}

	// New contents go to the same list; declining others keeps the import.
	if _, err := s.ImportCollection(kubrick, "", id, []string{"k3"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DismissCollection(kubrick, []string{"k4"}); err != nil {
		t.Fatal(err)
	}
	snap, _ = s.Snapshot()
	if d := snap.CollectionFolders[kubrick]; d.Status != CollectionImported || d.ListID != id || len(d.Seen) != 4 {
		t.Fatalf("decision %+v", d)
	}
	if n := len(snap.Lists[0].Entries); n != 3 {
		t.Fatalf("entries %d", n)
	}

	// A taken name or an unknown list fail and record nothing.
	if _, err := s.ImportCollection(otra, "kubrick", 0, []string{"o1"}); !errors.Is(err, ErrListExists) {
		t.Fatalf("taken name: %v", err)
	}
	if _, err := s.ImportCollection(otra, "", 999, []string{"o1"}); !errors.Is(err, ErrUnknownList) {
		t.Fatalf("unknown list: %v", err)
	}
	snap, _ = s.Snapshot()
	if _, ok := snap.CollectionFolders[otra]; ok || len(snap.Lists) != 1 {
		t.Fatalf("recorded a failed import: %+v", snap.CollectionFolders)
	}

	// Dismissing a folder never imported; deleting a list leaves its import
	// without a list.
	if err := s.DismissCollection(otra, []string{"o1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteList(id); err != nil {
		t.Fatal(err)
	}
	snap, _ = s.Snapshot()
	if d := snap.CollectionFolders[kubrick]; d.Status != CollectionImported || d.ListID != 0 {
		t.Fatalf("kubrick %+v", d)
	}
	if d := snap.CollectionFolders[otra]; d.Status != CollectionDismissed || d.ListID != 0 || !d.Seen["o1"] {
		t.Fatalf("otra %+v", d)
	}
}

func TestSnapshotOfCatalogWithoutLists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cinexplorer.db")
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"collection_seen", "collection_folders", "list_entries", "lists"} {
		if _, err := s.db.Exec(`DROP TABLE ` + table); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	snap, err := ro.Snapshot()
	if err != nil || snap.Lists == nil || len(snap.Lists) != 0 || snap.CollectionFolders == nil || len(snap.CollectionFolders) != 0 {
		t.Fatalf("snapshot %+v, %v", snap, err)
	}
}
```

- [ ] **Step 2: Correr las pruebas y ver que fallan**

Run: `go test ./internal/store/ -run 'Collections|WithoutLists'`
Expected: FAIL de compilación (`undefined: ImportCollection`, `snap.CollectionFolders`…).

- [ ] **Step 3: Implementar**

Crear `internal/store/collections.go`:

```go
package store

import "time"

// Decisions about a folder of Collections/.
const (
	CollectionImported  = "imported"
	CollectionDismissed = "dismissed"
)

// CollectionDecision is what the user decided about a folder of
// Collections/.
type CollectionDecision struct {
	Status string
	ListID int64           // the list it was imported into; 0 when dismissed or when that list was deleted
	Seen   map[string]bool // fingerprints already offered
}

// ImportCollection adds the contents fps of the folder path to a list — a
// new one called name, or listID when name is "" — and marks them as
// offered. It returns the list's id.
func (s *Store) ImportCollection(path, name string, listID int64, fps []string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	if name != "" {
		l, err := createList(tx, name, now)
		if err != nil {
			return 0, err
		}
		listID = l.ID
	} else if err := listExists(tx, listID); err != nil {
		return 0, err
	}
	for _, fp := range fps {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO list_entries (list_id, ref, added_at) VALUES (?, ?, ?)`,
			listID, RefFingerprint(fp), now); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(`UPDATE lists SET updated_at = ? WHERE id = ?`, now, listID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO collection_folders (path, status, list_id, decided_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET status = excluded.status, list_id = excluded.list_id, decided_at = excluded.decided_at`,
		path, CollectionImported, listID, now); err != nil {
		return 0, err
	}
	if err := markSeen(tx, path, fps); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return listID, nil
}

// DismissCollection marks the contents fps of the folder path as offered
// without importing them. A folder never imported is dismissed for good;
// an imported one keeps its list (only these new contents are declined).
func (s *Store) DismissCollection(path string, fps []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO collection_folders (path, status, list_id, decided_at) VALUES (?, ?, NULL, ?)
		ON CONFLICT(path) DO UPDATE SET decided_at = excluded.decided_at`,
		path, CollectionDismissed, time.Now().UnixMilli()); err != nil {
		return err
	}
	if err := markSeen(tx, path, fps); err != nil {
		return err
	}
	return tx.Commit()
}

func markSeen(tx execQuerier, path string, fps []string) error {
	for _, fp := range fps {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO collection_seen (path, fingerprint) VALUES (?, ?)`, path, fp); err != nil {
			return err
		}
	}
	return nil
}

// collectionDecisions loads the decisions by folder path.
func (s *Store) collectionDecisions(tx querier) (map[string]CollectionDecision, error) {
	out := map[string]CollectionDecision{}
	if !s.hasLists {
		return out, nil
	}
	rows, err := tx.Query(`SELECT path, status, COALESCE(list_id, 0) FROM collection_folders`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p string
		d := CollectionDecision{Seen: map[string]bool{}}
		if err := rows.Scan(&p, &d.Status, &d.ListID); err != nil {
			rows.Close()
			return nil, err
		}
		out[p] = d
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	srows, err := tx.Query(`SELECT path, fingerprint FROM collection_seen`)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var p, fp string
		if err := srows.Scan(&p, &fp); err != nil {
			return nil, err
		}
		if d, ok := out[p]; ok {
			d.Seen[fp] = true
		}
	}
	return out, srows.Err()
}
```

En `internal/store/snapshot.go`, sumar el campo a `Snapshot` (debajo de `Lists`):

```go
	CollectionFolders map[string]CollectionDecision // by folder path
```

inicializarlo en `Snapshot()`:

```go
	snap := Snapshot{Identifications: map[string]*Identification{}, Movies: map[int]Movie{}, Lists: []List{},
		CollectionFolders: map[string]CollectionDecision{}}
```

y cargarlo justo después de las listas:

```go
		if snap.Lists, err = s.lists(tx); err != nil {
			return err
		}
		if snap.CollectionFolders, err = s.collectionDecisions(tx); err != nil {
			return err
		}
```

- [ ] **Step 4: Correr las pruebas**

Run: `go test ./internal/store/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/collections.go internal/store/collections_test.go internal/store/snapshot.go
git commit -m "feat(store): decisions about Collections/ folders"
```

---

### Task 3: Catálogo — entradas resueltas, fichas y tarjetas de listas

**Files:**
- Modify: `internal/catalog/items.go`, `internal/catalog/detail.go`
- Create: `internal/catalog/lists.go`
- Test: `internal/catalog/lists_test.go`

**Interfaces:**
- Consumes (Tasks 1–2): `store.List`, `store.ListEntry`, `store.RefMovie`, `store.RefFingerprint`, `Snapshot.Lists`.
- Produces:
  - `type ListRef struct { ID int64 \`json:"id"\`; Name string \`json:"name"\` }`
  - `type ListCard struct { ID int64; Name string; Count int; UpdatedAt int64; Cover *Cover }` (JSON: `id`, `name`, `count`, `updatedAt`, `cover`)
  - `type Cover struct { TMDBID int \`json:"tmdbId"\`; Backdrop string \`json:"backdrop"\` }`
  - `func ListCards(snap store.Snapshot) []ListCard`
  - `func ItemRef(snap store.Snapshot, tmdbID int, key string) (string, bool)`
  - `func RefsTo(snap store.Snapshot, listID int64, tmdbID int, key string) []string`
  - internas: `type inList struct{ id int64; name string; added int64 }`, `Item.lists []inList`, `(*Item).addedTo(id string) (int64, bool)`, `targets([]*entry) map[string]*entry`, `attachLists`, `listsOf(snap, itemID string) []ListRef`
  - `MovieDetail.Lists []ListRef` y `VersionDetail.Lists []ListRef` (JSON `lists`, nunca `null`)

Regla de resolución: una entrada lleva al ítem que hoy tiene su película (`movie:<id>`) o su contenido (`fp:<huella>` → el ítem de la versión presente con esa huella: la película si está identificada, la propia versión si no). Contenidos marcados como extra o como "no es una película", ausentes del disco o películas no guardadas no llevan a nada. Un ítem al que llegan dos entradas de una lista cuenta una vez, con la fecha más antigua.

- [ ] **Step 1: Escribir las pruebas que fallan**

Crear `internal/catalog/lists_test.go`:

```go
package catalog

import (
	"reflect"
	"slices"
	"testing"

	"cinexplorer/internal/store"
)

// withLists adds three lists to the fixture snapshot (see fixture_test.go).
func withLists(snap store.Snapshot) store.Snapshot {
	snap.Lists = []store.List{
		{ID: 1, Name: "Fellini", UpdatedAt: 20, Entries: []store.ListEntry{
			{Ref: "fp:a2", AddedAt: 5},      // Amarcord, through one of its contents
			{Ref: "movie:7857", AddedAt: 3}, // Amarcord again: counted once, added at 3
			{Ref: "movie:7858", AddedAt: 7},
			{Ref: "fp:e1", AddedAt: 8},   // an extra: leads nowhere
			{Ref: "fp:g1", AddedAt: 9},   // missing from disk
			{Ref: "movie:1", AddedAt: 9}, // not stored
		}},
		{ID: 2, Name: "Por ver", UpdatedAt: 30, Entries: []store.ListEntry{
			{Ref: "fp:s1", AddedAt: 1}, // Stalker, never identified
			{Ref: "fp:p1", AddedAt: 2}, // Solaris, identified as a movie not stored
		}},
		{ID: 3, Name: "vacía", UpdatedAt: 10},
	}
	return snap
}

func TestListsOnItems(t *testing.T) {
	got := map[string][]inList{}
	for _, it := range Items(withLists(snapshot()), roots) {
		if len(it.lists) > 0 {
			got[it.id()] = it.lists
		}
	}
	want := map[string][]inList{
		"movie:7857": {{id: 1, name: "Fellini", added: 3}},
		"movie:7858": {{id: 1, name: "Fellini", added: 7}},
		"s1":         {{id: 2, name: "Por ver", added: 1}},
		"p1":         {{id: 2, name: "Por ver", added: 2}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lists %+v", got)
	}
}

func TestListEntryFollowsIdentification(t *testing.T) {
	snap := withLists(snapshot())
	for i := range snap.Versions {
		if snap.Versions[i].Fingerprint == "s1" {
			snap.Versions[i] = identified(snap.Versions[i], store.StatusManual, 7858)
		}
	}
	d, _ := Movie(snap, 7858)
	if want := []ListRef{{1, "Fellini"}, {2, "Por ver"}}; !reflect.DeepEqual(d.Lists, want) {
		t.Fatalf("lists %+v", d.Lists)
	}
}

func TestDetailLists(t *testing.T) {
	snap := withLists(snapshot())
	if d, _ := Movie(snap, 7857); !reflect.DeepEqual(d.Lists, []ListRef{{1, "Fellini"}}) {
		t.Fatalf("movie %+v", d.Lists)
	}
	if d, _ := Version(snap, "s1"); !reflect.DeepEqual(d.Lists, []ListRef{{2, "Por ver"}}) {
		t.Fatalf("version %+v", d.Lists)
	}
	if d, _ := Version(snap, "n1"); d.Lists == nil || len(d.Lists) != 0 {
		t.Fatalf("no lists %+v", d.Lists)
	}
}

func TestItemRef(t *testing.T) {
	snap := snapshot()
	cases := []struct {
		tmdbID int
		key    string
		want   string
		ok     bool
	}{
		{7857, "", "movie:7857", true},
		{1, "", "", false},       // not stored
		{0, "s1", "fp:s1", true}, // a present content
		{0, "g1", "", false},     // missing from disk
		{0, "id:9", "", false},   // no fingerprint
		{0, "", "", false},
	}
	for _, c := range cases {
		if got, ok := ItemRef(snap, c.tmdbID, c.key); got != c.want || ok != c.ok {
			t.Errorf("ItemRef(%d, %q) = %q, %v", c.tmdbID, c.key, got, ok)
		}
	}
}

func TestRefsTo(t *testing.T) {
	snap := withLists(snapshot())
	got := RefsTo(snap, 1, 7857, "")
	slices.Sort(got)
	if want := []string{"fp:a2", "movie:7857"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("amarcord %v", got)
	}
	if got := RefsTo(snap, 2, 0, "s1"); !reflect.DeepEqual(got, []string{"fp:s1"}) {
		t.Fatalf("stalker %v", got)
	}
	if got := RefsTo(snap, 1, 0, "s1"); len(got) != 0 {
		t.Fatalf("other list %v", got)
	}
	// An entry whose content is gone can still be removed by its own ref.
	if got := RefsTo(snap, 1, 0, "g1"); !reflect.DeepEqual(got, []string{"fp:g1"}) {
		t.Fatalf("gone %v", got)
	}
}

func TestListCards(t *testing.T) {
	snap := withLists(snapshot())
	roma := snap.Movies[7858]
	roma.BackdropPath = "/r.jpg"
	snap.Movies[7858] = roma
	want := []ListCard{
		{ID: 2, Name: "Por ver", Count: 2, UpdatedAt: 30},
		{ID: 1, Name: "Fellini", Count: 2, UpdatedAt: 20, Cover: &Cover{TMDBID: 7858, Backdrop: "r"}},
		{ID: 3, Name: "vacía", Count: 0, UpdatedAt: 10},
	}
	if got := ListCards(snap); !reflect.DeepEqual(got, want) {
		t.Fatalf("cards %+v", got)
	}
	if got := ListCards(snapshot()); got == nil || len(got) != 0 {
		t.Fatalf("no lists %+v", got)
	}
}
```

- [ ] **Step 2: Correr las pruebas y ver que fallan**

Run: `go test ./internal/catalog/ -run 'List|ItemRef|RefsTo|DetailLists'`
Expected: FAIL de compilación (`undefined: inList`, `ListRef`, …).

- [ ] **Step 3: Implementar**

En `internal/catalog/items.go`, en `type Item`, sumar al final del bloque "What the facets look at":

```go
	norm         string // title as compared when sorting
	lists        []inList
	listAdded    int64 // when it was added to the list being sorted by (SortQuery)
```

y en `build`, adjuntar las listas después de `finish`:

```go
	for _, e := range out {
		e.finish(roots)
	}
	attachLists(out, snap.Lists)
	return out
}
```

Crear `internal/catalog/lists.go`:

```go
package catalog

import (
	"cmp"
	"slices"
	"strconv"

	"cinexplorer/internal/images"
	"cinexplorer/internal/quality"
	"cinexplorer/internal/store"
)

// ListRef names a list.
type ListRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// inList is an item's place in one list.
type inList struct {
	id    int64
	name  string
	added int64 // unix ms: the earliest entry that leads to the item
}

// targets maps the entries a list can hold to the items built from a
// snapshot: RefMovie of every movie item, and RefFingerprint of every
// present content, which leads to its movie's item once identified and to
// its own otherwise. Contents that are extras, not movies or missing lead
// nowhere, since build leaves them out.
func targets(es []*entry) map[string]*entry {
	out := map[string]*entry{}
	for _, e := range es {
		if e.item.Kind == KindMovie {
			out[store.RefMovie(e.item.TMDBID)] = e
		}
		for _, v := range e.versions {
			if v.Fingerprint != "" {
				out[store.RefFingerprint(v.Fingerprint)] = e
			}
		}
	}
	return out
}

// attachLists records on each item the lists that lead to it.
func attachLists(es []*entry, lists []store.List) {
	to := targets(es)
	for _, l := range lists {
		for _, en := range l.Entries {
			if e := to[en.Ref]; e != nil {
				e.item.addList(l.ID, l.Name, en.AddedAt)
			}
		}
	}
}

func (it *Item) addList(id int64, name string, added int64) {
	for i := range it.lists {
		if it.lists[i].id == id {
			it.lists[i].added = min(it.lists[i].added, added)
			return
		}
	}
	it.lists = append(it.lists, inList{id: id, name: name, added: added})
}

// addedTo tells when the item was added to the list id (as the lista facet
// has it), and whether it is in it at all.
func (it *Item) addedTo(id string) (int64, bool) {
	for _, l := range it.lists {
		if strconv.FormatInt(l.id, 10) == id {
			return l.added, true
		}
	}
	return 0, false
}

// listsOf names the lists that lead to the item itemID (see Item.id), by
// name.
func listsOf(snap store.Snapshot, itemID string) []ListRef {
	out := []ListRef{}
	for _, e := range build(snap, nil) {
		if e.item.id() != itemID {
			continue
		}
		for _, l := range e.item.lists {
			out = append(out, ListRef{ID: l.id, Name: l.name})
		}
	}
	slices.SortFunc(out, func(a, b ListRef) int {
		return cmp.Or(cmp.Compare(quality.NormTitle(a.Name), quality.NormTitle(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// ItemRef is the entry that adds an item to a list: a stored movie by its
// TMDB id, or a present content by its fingerprint (the item's key). ok is
// false for anything else.
func ItemRef(snap store.Snapshot, tmdbID int, key string) (string, bool) {
	if tmdbID > 0 {
		if _, ok := snap.Movies[tmdbID]; ok {
			return store.RefMovie(tmdbID), true
		}
		return "", false
	}
	for i := range snap.Versions {
		if v := &snap.Versions[i]; key != "" && v.Fingerprint == key && hasPresentMain(v) {
			return store.RefFingerprint(key), true
		}
	}
	return "", false
}

// RefsTo lists the entries of the list listID that lead to an item — the
// movie tmdbID, or the content key — plus the item's own entry, so that
// removing the item removes them all.
func RefsTo(snap store.Snapshot, listID int64, tmdbID int, key string) []string {
	direct := store.RefFingerprint(key)
	if tmdbID > 0 {
		direct = store.RefMovie(tmdbID)
	}
	to := targets(build(snap, nil))
	target := to[direct]
	var out []string
	for _, l := range snap.Lists {
		if l.ID != listID {
			continue
		}
		for _, en := range l.Entries {
			if en.Ref == direct || (target != nil && to[en.Ref] == target) {
				out = append(out, en.Ref)
			}
		}
	}
	return out
}

// ListCard is a list as the Listas page shows it.
type ListCard struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Count     int    `json:"count"`     // items present
	UpdatedAt int64  `json:"updatedAt"` // unix ms
	Cover     *Cover `json:"cover"`     // nil when none of its movies has a still
}

// Cover is the still of a list's card.
type Cover struct {
	TMDBID   int    `json:"tmdbId"`
	Backdrop string `json:"backdrop"` // image version
}

// ListCards returns every list, the most recently changed first. A card's
// cover is the still of the movie added last to the list that has one.
func ListCards(snap store.Snapshot) []ListCard {
	es := build(snap, nil)
	out := []ListCard{}
	for _, l := range snap.Lists {
		c := ListCard{ID: l.ID, Name: l.Name, UpdatedAt: l.UpdatedAt}
		id := strconv.FormatInt(l.ID, 10)
		var coverAdded int64
		for _, e := range es {
			added, ok := e.item.addedTo(id)
			if !ok {
				continue
			}
			c.Count++
			if e.item.Kind != KindMovie {
				continue
			}
			m := snap.Movies[e.item.TMDBID]
			if m.BackdropPath != "" && (c.Cover == nil || added > coverAdded) {
				c.Cover, coverAdded = &Cover{TMDBID: m.TMDBID, Backdrop: images.Version(m.BackdropPath)}, added
			}
		}
		out = append(out, c)
	}
	slices.SortStableFunc(out, func(a, b ListCard) int {
		return cmp.Or(-cmp.Compare(a.UpdatedAt, b.UpdatedAt), cmp.Compare(a.ID, b.ID))
	})
	return out
}
```

En `internal/catalog/detail.go`:

- en `MovieDetail`, agregar al final: `Lists         []ListRef     \`json:"lists"\` // the lists that hold it`
- en `VersionDetail`, agregar al final: `Lists          []ListRef       \`json:"lists"\` // the lists that hold it`
- en `Movie`, antes de `sortCards(d.Versions)`: `d.Lists = listsOf(snap, store.RefMovie(id)) // a movie item's id is its RefMovie`
- en `Version`, antes de `sortCards(d.Versions)`: `d.Lists = listsOf(snap, key)`

- [ ] **Step 4: Correr las pruebas**

Run: `go test ./internal/catalog/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/items.go internal/catalog/detail.go internal/catalog/lists.go internal/catalog/lists_test.go
git commit -m "feat(catalog): resolve list entries to items; lists in the pages"
```

---

### Task 4: Catálogo — faceta `lista` y orden `agregado-lista`

**Files:**
- Modify: `internal/catalog/facets.go`, `internal/catalog/sort.go`, `internal/catalog/lists.go`
- Test: `internal/catalog/facets_test.go`, `internal/catalog/lists_test.go`

**Interfaces:**
- Consumes (Task 3): `Item.lists`, `Item.listAdded`, `(*Item).addedTo`, `ListRef`, `withLists` (test).
- Produces:
  - `const FacetList = "lista"`, `const OrderListAdded = "agregado-lista"`; `FacetNames` incluye `FacetList` después de `FacetCollection`.
  - `func ApplyLists(q Query, snap store.Snapshot) (Query, *ListRef)`
  - `func SortQuery(items []Item, q Query)`

Reglas: con `lista` aplicada, el orden por defecto es `agregado-lista` (más reciente primero); `orden=anio` explícito se respeta. `agregado-lista` sin lista se ignora. Un id de lista que no existe se descarta en `ApplyLists` (y con él su orden, que pasa a año descendente).

- [ ] **Step 1: Escribir las pruebas que fallan**

En `internal/catalog/facets_test.go`, agregar a la tabla `cases` de `TestParseQuery`:

```go
		// A list sorts by date added to it unless another order is asked for.
		{"lista=7", Query{Facets: map[string]string{"lista": "7"}, Order: OrderListAdded, Dir: Desc}},
		{"lista=7&orden=anio", Query{Facets: map[string]string{"lista": "7"}, Order: OrderYear, Dir: Desc}},
		{"lista=7&orden=titulo", Query{Facets: map[string]string{"lista": "7"}, Order: OrderTitle, Dir: Asc}},
		{"orden=agregado-lista", Query{Facets: map[string]string{}, Order: OrderYear, Dir: Desc}},
		{"lista=0&orden=agregado-lista", Query{Facets: map[string]string{}, Order: OrderYear, Dir: Desc}},
```

Agregar al final de `internal/catalog/lists_test.go`:

```go
func TestListFacet(t *testing.T) {
	items := Items(withLists(snapshot()), roots)
	for list, want := range map[string][]string{"1": {"movie:7857", "movie:7858"}, "2": {"p1", "s1"}, "3": {}} {
		if got := ids(Filter(items, map[string]string{FacetList: list})); !reflect.DeepEqual(got, want) {
			t.Errorf("lista=%s: %v", list, got)
		}
	}
	want := []FacetValue{{Value: "1", Label: "Fellini", Count: 2}, {Value: "2", Label: "Por ver", Count: 2}}
	if got := Counts(items, map[string]string{})[FacetList]; !reflect.DeepEqual(got, want) {
		t.Fatalf("counts %+v", got)
	}
}

func TestSortByListAdded(t *testing.T) {
	items := Items(withLists(snapshot()), roots)
	order := func(list, dir string) []string {
		q := Query{Facets: map[string]string{FacetList: list}, Order: OrderListAdded, Dir: dir}
		got := Filter(items, q.Facets)
		SortQuery(got, q)
		out := []string{}
		for _, it := range got {
			out = append(out, it.id())
		}
		return out
	}
	if got := order("1", Desc); !reflect.DeepEqual(got, []string{"movie:7858", "movie:7857"}) {
		t.Fatalf("fellini %v", got)
	}
	if got := order("2", Desc); !reflect.DeepEqual(got, []string{"p1", "s1"}) {
		t.Fatalf("por ver %v", got)
	}
	if got := order("2", Asc); !reflect.DeepEqual(got, []string{"s1", "p1"}) {
		t.Fatalf("por ver asc %v", got)
	}
}

func TestApplyLists(t *testing.T) {
	snap := withLists(snapshot())
	q := Query{Facets: map[string]string{FacetList: "2", FacetDecade: "1970"}, Order: OrderListAdded, Dir: Asc}
	if got, list := ApplyLists(q, snap); !reflect.DeepEqual(got, q) || list == nil || *list != (ListRef{2, "Por ver"}) {
		t.Fatalf("known list: %+v %+v", got, list)
	}
	q.Facets[FacetList] = "9" // deleted
	got, list := ApplyLists(q, snap)
	want := Query{Facets: map[string]string{FacetDecade: "1970"}, Order: OrderYear, Dir: Desc}
	if !reflect.DeepEqual(got, want) || list != nil {
		t.Fatalf("unknown list: %+v %+v", got, list)
	}
	if q.Facets[FacetList] != "9" {
		t.Fatal("ApplyLists changed the query it got")
	}
	plain := Query{Facets: map[string]string{}, Order: OrderTitle, Dir: Asc}
	if got, list := ApplyLists(plain, snap); !reflect.DeepEqual(got, plain) || list != nil {
		t.Fatalf("no list: %+v %+v", got, list)
	}
}
```

- [ ] **Step 2: Correr las pruebas y ver que fallan**

Run: `go test ./internal/catalog/`
Expected: FAIL de compilación (`undefined: FacetList`, `OrderListAdded`, `SortQuery`, `ApplyLists`).

- [ ] **Step 3: Implementar**

En `internal/catalog/facets.go`:

- en el bloque de facetas, debajo de `FacetCollection`: `FacetList       = "lista"      // a user's list id`
- `FacetNames` pasa a ser:

```go
var FacetNames = []string{FacetDecade, FacetYear, FacetDirector, FacetGenre, FacetCountry, FacetLanguage,
	FacetCollection, FacetList, FacetResolution, FacetSubs, FacetLocation, FacetState}
```

- en el bloque de órdenes, debajo de `OrderSize`: `OrderListAdded = "agregado-lista" // only with FacetList`
- en `valid`, el caso numérico suma la faceta: `case FacetYear, FacetDirector, FacetCollection, FacetList:`
- en `ParseQuery`, reemplazar la elección del orden:

```go
	q := Query{Facets: map[string]string{}, Order: OrderYear}
	for _, f := range FacetNames {
		if val := v.Get(f); valid(f, val) {
			q.Facets[f] = val
		}
	}
	// A list sorts by date added to it unless another order is asked for.
	if q.Facets[FacetList] != "" {
		q.Order = OrderListAdded
	}
	switch o := v.Get("orden"); o {
	case OrderYear, OrderTitle, OrderAdded, OrderSize:
		q.Order = o
	}
```

(el cálculo de `q.Dir` que sigue queda igual). Actualizar el comentario de `ParseQuery`: "the order defaults to year, newest first (date added to the list, with a list)".

- en `values`, antes de `case FacetResolution:`:

```go
	case FacetList:
		out := make([]string, len(it.lists))
		for i, l := range it.lists {
			out[i] = strconv.FormatInt(l.id, 10)
		}
		return out
```

- en `label`, antes del `return value` final, sumar:

```go
	case FacetList:
		for _, l := range it.lists {
			if strconv.FormatInt(l.id, 10) == value {
				return l.name
			}
		}
```

En `internal/catalog/sort.go`, en el `switch order` de `Sort`, antes de `default:`:

```go
		case OrderListAdded:
			c = cmp.Compare(a.listAdded, b.listAdded)
```

y al final del archivo:

```go
// SortQuery sorts items as q asks, including by date added to q's list.
func SortQuery(items []Item, q Query) {
	if q.Order == OrderListAdded {
		for i := range items {
			items[i].listAdded, _ = items[i].addedTo(q.Facets[FacetList])
		}
	}
	Sort(items, q.Order, q.Dir)
}
```

En `internal/catalog/lists.go`, sumar `"maps"` a los imports y al final:

```go
// ApplyLists checks q's lista facet against the lists of snap. A list that
// no longer exists is dropped like a malformed value, and so is its order.
// It returns the list applied, nil when none.
func ApplyLists(q Query, snap store.Snapshot) (Query, *ListRef) {
	id := q.Facets[FacetList]
	if id == "" {
		return q, nil
	}
	for _, l := range snap.Lists {
		if strconv.FormatInt(l.ID, 10) == id {
			return q, &ListRef{ID: l.ID, Name: l.Name}
		}
	}
	q.Facets = maps.Clone(q.Facets)
	delete(q.Facets, FacetList)
	if q.Order == OrderListAdded {
		q.Order, q.Dir = OrderYear, Desc
	}
	return q, nil
}
```

- [ ] **Step 4: Correr las pruebas**

Run: `go test ./internal/catalog/ ./internal/server/`
Expected: PASS (el servidor cuenta las facetas con `len(catalog.FacetNames)`: sigue coincidiendo).

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/facets.go internal/catalog/sort.go internal/catalog/lists.go internal/catalog/facets_test.go internal/catalog/lists_test.go
git commit -m "feat(catalog): lista facet and sorting by date added to the list"
```

---

### Task 5: Catálogo — carpetas pendientes de `Collections/`

**Files:**
- Create: `internal/catalog/collections.go`
- Test: `internal/catalog/collections_test.go`

**Interfaces:**
- Consumes: `build`, `entry`, `Item`, `ListRef` (Task 3); `store.CollectionDecision`, `store.CollectionImported` (Task 2).
- Produces:
  - `type CollectionFolder struct { Path, Name string; Total, New int; Preview []Item; List *ListRef; Fingerprints []string }` (JSON: `path`, `name`, `total`, `new`, `preview`, `list`; `Fingerprints` no se serializa)
  - `func Collections(snap store.Snapshot) []CollectionFolder` — ordenadas por ruta, nunca `nil`.

- [ ] **Step 1: Escribir las pruebas que fallan**

Crear `internal/catalog/collections_test.go`:

```go
package catalog

import (
	"reflect"
	"testing"

	"cinexplorer/internal/store"
)

// collectionsSnapshot adds folders of Collections/ to the fixture, which
// already has an SD Amarcord (fingerprint a2) in ../cine/Collections/Fellini.
func collectionsSnapshot() store.Snapshot {
	snap := snapshot()
	gone := version(24, "../cine/Collections/Nada/Gone", "Gone.avi", "g2", "Gone", 1990, "", 10, 900)
	gone.Files[0].Missing = true
	snap.Versions = append(snap.Versions,
		identified(version(20, "../cine/Collections/Fellini/Roma (1972)", "Roma.avi", "r2", "Roma", 1972, "SD", 600, 900), store.StatusAuto, 7858),
		version(21, "../cine/collections/Kubrick", "The Shining.avi", "k1", "The Shining", 1980, "", 700, 900),
		identified(version(22, "../cine/collections/Kubrick/Extras", "Trailer.avi", "k2", "Trailer", 0, "", 10, 900), store.StatusExtra, 7857),
		version(23, "../cine/Collections", "Suelta.avi", "c0", "Suelta", 0, "", 10, 900), // not in a subfolder
		gone,
		version(25, "../cine/Collections/Wong/Collections/Otra", "Chungking.avi", "w1", "Chungking", 1994, "", 10, 900),
	)
	return snap
}

func folderSummary(fs []CollectionFolder) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.Path)
	}
	return out
}

func TestCollectionsFound(t *testing.T) {
	fs := Collections(collectionsSnapshot())
	// The first Collections of a path counts, in any case; extras, missing
	// files and loose files do not.
	want := []string{"../cine/Collections/Fellini", "../cine/Collections/Wong", "../cine/collections/Kubrick"}
	if got := folderSummary(fs); !reflect.DeepEqual(got, want) {
		t.Fatalf("folders %v", got)
	}
	f := fs[0]
	if f.Name != "Fellini" || f.Total != 2 || f.New != 2 || f.List != nil || !reflect.DeepEqual(f.Fingerprints, []string{"a2", "r2"}) {
		t.Fatalf("fellini %+v", f)
	}
	if len(f.Preview) != 2 || f.Preview[0].TMDBID != 7857 || f.Preview[1].TMDBID != 7858 {
		t.Fatalf("preview %+v", f.Preview)
	}
	if k := fs[2]; k.Name != "Kubrick" || k.Total != 1 || !reflect.DeepEqual(k.Fingerprints, []string{"k1"}) || k.Preview[0].Key != "k1" {
		t.Fatalf("kubrick %+v", k)
	}
}

func TestCollectionsDecided(t *testing.T) {
	snap := collectionsSnapshot()
	snap.Lists = []store.List{{ID: 4, Name: "Fellini"}}
	snap.CollectionFolders = map[string]store.CollectionDecision{
		"../cine/Collections/Fellini": {Status: store.CollectionImported, ListID: 4, Seen: map[string]bool{"a2": true}},
		"../cine/Collections/Wong":    {Status: store.CollectionDismissed, Seen: map[string]bool{}},
		"../cine/collections/Kubrick": {Status: store.CollectionImported, ListID: 0, Seen: map[string]bool{}}, // its list was deleted
	}
	fs := Collections(snap)
	if len(fs) != 1 {
		t.Fatalf("folders %v", folderSummary(fs))
	}
	f := fs[0]
	if f.Total != 2 || f.New != 1 || f.List == nil || *f.List != (ListRef{4, "Fellini"}) ||
		!reflect.DeepEqual(f.Fingerprints, []string{"r2"}) || len(f.Preview) != 1 || f.Preview[0].TMDBID != 7858 {
		t.Fatalf("fellini %+v", f)
	}
	// Once everything was offered, nothing is pending.
	snap.CollectionFolders["../cine/Collections/Fellini"].Seen["r2"] = true
	if fs := Collections(snap); len(fs) != 0 {
		t.Fatalf("folders %v", folderSummary(fs))
	}
}

func TestCollectionsEmpty(t *testing.T) {
	if fs := Collections(store.Snapshot{}); fs == nil || len(fs) != 0 {
		t.Fatalf("folders %+v", fs)
	}
}
```

- [ ] **Step 2: Correr las pruebas y ver que fallan**

Run: `go test ./internal/catalog/ -run Collections`
Expected: FAIL de compilación (`undefined: Collections`, `CollectionFolder`).

- [ ] **Step 3: Implementar**

Crear `internal/catalog/collections.go`:

```go
package catalog

import (
	"path"
	"slices"
	"strings"

	"cinexplorer/internal/store"
)

// previewSize is how many items a folder of Collections/ shows.
const previewSize = 8

// CollectionFolder is a folder of Collections/ with contents to offer as a
// list.
type CollectionFolder struct {
	Path    string   `json:"path"`
	Name    string   `json:"name"`
	Total   int      `json:"total"`   // contents present in the folder
	New     int      `json:"new"`     // contents not offered before
	Preview []Item   `json:"preview"` // the items of some of the new contents
	List    *ListRef `json:"list"`    // the list an earlier import went to
	// Fingerprints are the new contents: what importing adds.
	Fingerprints []string `json:"-"`
}

// collectionPath gives the folder of Collections/ that dir is in: the path
// down to the subfolder of the first folder named Collections (any case).
func collectionPath(dir string) (string, bool) {
	segs := strings.Split(dir, "/")
	for i, s := range segs {
		if strings.EqualFold(s, "Collections") {
			if i+1 < len(segs) {
				return strings.Join(segs[:i+2], "/"), true
			}
			return "", false
		}
	}
	return "", false
}

// Collections lists the folders of Collections/ with something to offer:
// never decided on, or imported with new contents since. Dismissed folders,
// and imported ones whose list was deleted, are not offered again. A
// folder's contents are the present versions under it, at any depth, that
// are neither extras nor marked as not movies, by fingerprint.
func Collections(snap store.Snapshot) []CollectionFolder {
	type folder struct {
		fps   []string
		items map[string]*entry // by fingerprint
	}
	folders := map[string]*folder{}
	for _, e := range build(snap, nil) {
		for _, v := range e.versions {
			p, ok := collectionPath(v.Dir)
			if !ok || v.Fingerprint == "" {
				continue
			}
			f := folders[p]
			if f == nil {
				f = &folder{items: map[string]*entry{}}
				folders[p] = f
			}
			if f.items[v.Fingerprint] == nil {
				f.items[v.Fingerprint] = e
				f.fps = append(f.fps, v.Fingerprint)
			}
		}
	}
	names := map[int64]string{}
	for _, l := range snap.Lists {
		names[l.ID] = l.Name
	}
	out := []CollectionFolder{}
	for p, f := range folders {
		c := CollectionFolder{Path: p, Name: path.Base(p), Total: len(f.fps), Preview: []Item{}, Fingerprints: []string{}}
		d, decided := snap.CollectionFolders[p]
		if decided {
			name, ok := names[d.ListID]
			if d.Status != store.CollectionImported || !ok {
				continue
			}
			c.List = &ListRef{ID: d.ListID, Name: name}
		}
		shown := map[*entry]bool{}
		for _, fp := range f.fps {
			if d.Seen[fp] {
				continue
			}
			c.Fingerprints = append(c.Fingerprints, fp)
			if e := f.items[fp]; !shown[e] && len(c.Preview) < previewSize {
				shown[e] = true
				c.Preview = append(c.Preview, e.item)
			}
		}
		if c.New = len(c.Fingerprints); c.New > 0 {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b CollectionFolder) int { return strings.Compare(a.Path, b.Path) })
	return out
}
```

(`d.Seen` es `nil` para una carpeta sin decisión: leer un mapa `nil` da `false`.)

- [ ] **Step 4: Correr las pruebas**

Run: `go test ./internal/catalog/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/collections.go internal/catalog/collections_test.go
git commit -m "feat(catalog): folders of Collections/ pending import"
```

---

### Task 6: Catálogo — filas de listas en Inicio

**Files:**
- Modify: `internal/catalog/home.go`
- Test: `internal/catalog/home_test.go`

**Interfaces:**
- Consumes: `(*Item).addedTo` (Task 3), `FacetList` (Task 4), `row`, `group` (existen).
- Produces: `const RowList = "list"`. Hasta 2 filas: las listas de `UpdatedAt` más reciente con al menos 3 películas identificadas presentes; sus películas, de la agregada más recientemente a la más antigua; `Value` = id, `Label` = nombre, `Href` = `/explorar?lista=<id>`. Entran al sorteo del orden de filas como las demás (Inicio ya mezcla todas sus filas, "agregadas recientemente" incluida).

- [ ] **Step 1: Escribir la prueba que falla**

Agregar al final de `internal/catalog/home_test.go`:

```go
func TestHomePageLists(t *testing.T) {
	snap := homeSnapshot()
	refs := func(added int64, rs ...string) []store.ListEntry {
		var out []store.ListEntry
		for i, r := range rs {
			out = append(out, store.ListEntry{Ref: r, AddedAt: added + int64(i)})
		}
		return out
	}
	snap.Lists = []store.List{
		{ID: 1, Name: "Vieja", UpdatedAt: 1, Entries: refs(1, "movie:100", "movie:101", "movie:102")},
		{ID: 2, Name: "Corta", UpdatedAt: 5, Entries: refs(1, "movie:100", "movie:101")}, // fewer than 3
		{ID: 3, Name: "Nueva", UpdatedAt: 9, Entries: []store.ListEntry{
			{Ref: "fp:f5", AddedAt: 1}, // movie 105, through its content
			{Ref: "movie:106", AddedAt: 3},
			{Ref: "movie:107", AddedAt: 2},
			{Ref: "fp:s1", AddedAt: 4}, // not identified: not in a home row
		}},
		{ID: 4, Name: "Otra", UpdatedAt: 3, Entries: refs(1, "movie:103", "movie:104", "movie:105")},
	}
	var lists []HomeRow
	for _, r := range HomePage(snap, 1).Rows {
		if r.Kind == RowList {
			lists = append(lists, r)
		}
	}
	slices.SortFunc(lists, func(a, b HomeRow) int { return strings.Compare(a.Value, b.Value) }) // rows are shuffled
	if len(lists) != 2 {
		t.Fatalf("list rows %+v", lists)
	}
	var got [][]int
	for _, r := range lists {
		var movies []int
		for _, it := range r.Items {
			movies = append(movies, it.TMDBID)
		}
		got = append(got, movies)
	}
	if n := lists[0]; n.Value != "3" || n.Label != "Nueva" || n.Href != "/explorar?lista=3" {
		t.Fatalf("nueva %+v", n)
	}
	if o := lists[1]; o.Value != "4" || o.Label != "Otra" || o.Href != "/explorar?lista=4" {
		t.Fatalf("otra %+v", o)
	}
	if want := [][]int{{106, 107, 105}, {105, 104, 103}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("movies %v", got)
	}
}
```

y sumar `"strings"` a los imports de `home_test.go`.

- [ ] **Step 2: Correr la prueba y ver que falla**

Run: `go test ./internal/catalog/ -run TestHomePageLists`
Expected: FAIL de compilación (`undefined: RowList`).

- [ ] **Step 3: Implementar**

En `internal/catalog/home.go`:

- en el bloque de tipos de fila, debajo de `RowCollection`: `RowList       = "list"`
- debajo de `const rowSize = 20`:

```go
// maxListRows is how many lists Inicio shows at most; minListRow, how
// many movies a list needs for a row.
const (
	maxListRows = 2
	minListRow  = 3
)
```

- en `HomePage`, después de la última llamada a `add(RowCollection, …)` y antes de `rng.Shuffle(len(home.Rows), …)`:

```go
	// The lists changed last, with their movies added last first.
	lists := slices.Clone(snap.Lists)
	slices.SortStableFunc(lists, func(a, b store.List) int {
		return cmp.Or(-cmp.Compare(a.UpdatedAt, b.UpdatedAt), cmp.Compare(a.ID, b.ID))
	})
	shown := 0
	for _, l := range lists {
		if shown == maxListRows {
			break
		}
		id := strconv.FormatInt(l.ID, 10)
		var in []*entry
		for _, e := range movies {
			if _, ok := e.item.addedTo(id); ok {
				in = append(in, e)
			}
		}
		if len(in) < minListRow {
			continue
		}
		slices.SortStableFunc(in, func(a, b *entry) int {
			ta, _ := a.item.addedTo(id)
			tb, _ := b.item.addedTo(id)
			return cmp.Or(-cmp.Compare(ta, tb), cmp.Compare(a.item.norm, b.item.norm))
		})
		home.Rows = append(home.Rows, row(snap, RowList, group{value: id, label: l.Name, movies: in},
			"/explorar?"+url.Values{FacetList: {id}}.Encode()))
		shown++
	}
```

- actualizar el comentario de `HomePage`: "…and TMDB collection with enough movies, and the lists changed last, in a random order."

- [ ] **Step 4: Correr las pruebas**

Run: `go test ./internal/catalog/`
Expected: PASS (las pruebas existentes de Inicio no tienen listas: no cambian).

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/home.go internal/catalog/home_test.go
git commit -m "feat(catalog): home rows for the lists changed last"
```

---

### Task 7: Servidor — API de listas y Explorar con listas

**Files:**
- Create: `internal/server/lists.go`
- Modify: `internal/server/server.go` (rutas), `internal/server/catalog.go` (`explore`)
- Test: `internal/server/lists_test.go`, `internal/server/catalog_test.go` (`exploreBody`)

**Interfaces:**
- Consumes: `store.CreateList`, `RenameList`, `DeleteList`, `AddEntries`, `RemoveEntries`, `ErrListName`, `ErrListExists`, `ErrUnknownList` (Task 1); `catalog.ListCards`, `ItemRef`, `RefsTo`, `ListRef` (Task 3); `catalog.ApplyLists`, `SortQuery` (Task 4).
- Produces (usadas en la Task 8): `(s *Server) writable(w) bool`, `listError(w, err)`, `decode(w, r, v) bool`.
- Rutas:
  - `GET /api/lists` → `[]catalog.ListCard`
  - `POST /api/lists` `{name}` → 201 `{id, name}`
  - `PATCH /api/lists/{id}` `{name}` → 204
  - `DELETE /api/lists/{id}` → 204
  - `POST /api/lists/{id}/entries` `{tmdbId}` o `{key}` → 204
  - `DELETE /api/lists/{id}/entries` `{tmdbId}` o `{key}` → 204
  - `GET /api/explore` suma `"list": {id, name} | null`

- [ ] **Step 1: Escribir las pruebas que fallan**

En `internal/server/catalog_test.go`, sumar el campo a `exploreBody`:

```go
	List   *catalog.ListRef                `json:"list"`
```

Crear `internal/server/lists_test.go`:

```go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"cinexplorer/internal/catalog"
)

func send(t *testing.T, s *Server, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	return request(s.Handler(), method, url, body, "application/json", "127.0.0.1")
}

func TestListsEndpoints(t *testing.T) {
	s, _ := identifyServer(t)
	rec := send(t, s, "POST", "/api/lists", `{"name":" Por ver "}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d: %s", rec.Code, rec.Body)
	}
	var l catalog.ListRef
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil || l.ID == 0 || l.Name != "Por ver" {
		t.Fatalf("created %s (%v)", rec.Body, err)
	}
	for body, want := range map[string]int{`{"name":"por ver"}`: http.StatusConflict, `{"name":"  "}`: http.StatusBadRequest, `{`: http.StatusBadRequest} {
		if code := send(t, s, "POST", "/api/lists", body).Code; code != want {
			t.Errorf("create %s: %d, want %d", body, code, want)
		}
	}

	path := fmt.Sprintf("/api/lists/%d", l.ID)
	// The unidentified Amarcord, by its key (its fingerprint).
	if code := send(t, s, "POST", path+"/entries", `{"key":"f1"}`).Code; code != http.StatusNoContent {
		t.Fatalf("add %d", code)
	}
	for _, c := range []struct {
		url, body string
		want      int
	}{
		{path + "/entries", `{"key":"nada"}`, http.StatusNotFound},
		{path + "/entries", `{"tmdbId":7857}`, http.StatusNotFound}, // not stored yet
		{path + "/entries", `{}`, http.StatusBadRequest},
		{path + "/entries", `{"tmdbId":7857,"key":"f1"}`, http.StatusBadRequest},
		{"/api/lists/999/entries", `{"key":"f1"}`, http.StatusNotFound},
		{"/api/lists/x/entries", `{"key":"f1"}`, http.StatusNotFound},
	} {
		if code := send(t, s, "POST", c.url, c.body).Code; code != c.want {
			t.Errorf("add %s %s: %d, want %d", c.url, c.body, code, c.want)
		}
	}
	var cards []catalog.ListCard
	if code := getJSON(t, s, "/api/lists", &cards); code != 200 || len(cards) != 1 || cards[0].Count != 1 || cards[0].Name != "Por ver" {
		t.Fatalf("lists %d %+v", code, cards)
	}

	// Once identified, the entry leads to the movie, whose page names the list.
	post(t, s, `{"fingerprint":"f1","action":"movie","tmdbId":7857}`)
	var d catalog.MovieDetail
	getJSON(t, s, "/api/movies/7857", &d)
	if len(d.Lists) != 1 || d.Lists[0] != l {
		t.Fatalf("movie lists %+v", d.Lists)
	}
	var b exploreBody
	getJSON(t, s, fmt.Sprintf("/api/explore?lista=%d", l.ID), &b)
	if b.Total != 1 || b.Items[0].TMDBID != 7857 || b.Query.Order != catalog.OrderListAdded || b.List == nil || *b.List != l {
		t.Fatalf("explore %+v", b)
	}

	// Removing the movie removes the entry that led to it.
	if code := send(t, s, "DELETE", path+"/entries", `{"tmdbId":7857}`).Code; code != http.StatusNoContent {
		t.Fatalf("remove %d", code)
	}
	getJSON(t, s, "/api/lists", &cards)
	if cards[0].Count != 0 {
		t.Fatalf("after remove %+v", cards)
	}

	if code := send(t, s, "PATCH", path, `{"name":"Vistas"}`).Code; code != http.StatusNoContent {
		t.Fatalf("rename %d", code)
	}
	if code := send(t, s, "PATCH", "/api/lists/999", `{"name":"Otra"}`).Code; code != http.StatusNotFound {
		t.Fatalf("rename unknown %d", code)
	}
	if code := send(t, s, "DELETE", path, `{}`).Code; code != http.StatusNoContent {
		t.Fatalf("delete %d", code)
	}
	if code := send(t, s, "DELETE", path, `{}`).Code; code != http.StatusNotFound {
		t.Fatalf("delete twice %d", code)
	}
	// A deleted list is dropped from Explorar's query, and so is its order.
	b = exploreBody{}
	getJSON(t, s, fmt.Sprintf("/api/explore?lista=%d", l.ID), &b)
	if b.List != nil || b.Query.Facets["lista"] != "" || b.Query.Order != catalog.OrderYear || b.Total != 1 {
		t.Fatalf("explore after delete %+v", b)
	}
}

func TestListsReadOnly(t *testing.T) {
	s, _ := newServer(t)
	s.ReadOnly = true
	for _, c := range []struct{ method, url string }{
		{"POST", "/api/lists"}, {"PATCH", "/api/lists/1"}, {"DELETE", "/api/lists/1"},
		{"POST", "/api/lists/1/entries"}, {"DELETE", "/api/lists/1/entries"},
	} {
		if code := send(t, s, c.method, c.url, `{"name":"x","key":"f1"}`).Code; code != http.StatusConflict {
			t.Errorf("%s %s: %d", c.method, c.url, code)
		}
	}
	if code := request(s.Handler(), "POST", "/api/lists", `{"name":"x"}`, "text/plain", "127.0.0.1").Code; code != http.StatusUnsupportedMediaType {
		t.Fatalf("without JSON: %d", code)
	}
	var cards []catalog.ListCard
	if code := getJSON(t, s, "/api/lists", &cards); code != 200 || cards == nil {
		t.Fatalf("read %d %+v", code, cards)
	}
}
```

- [ ] **Step 2: Correr las pruebas y ver que fallan**

Run: `go test ./internal/server/ -run Lists`
Expected: FAIL (las rutas no existen: 404/405 en lugar de los códigos esperados).

- [ ] **Step 3: Implementar**

Crear `internal/server/lists.go`:

```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/store"
)

// writable answers 409 when the catalog is open read-only.
func (s *Server) writable(w http.ResponseWriter) bool {
	if s.ReadOnly {
		http.Error(w, "modo consulta: el catálogo no se puede modificar", http.StatusConflict)
		return false
	}
	return true
}

// listError answers an error of the lists' store calls.
func listError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrListName):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, store.ErrListExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, store.ErrUnknownList):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// decode reads a JSON body into v, answering 400 when it is not one.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return false
	}
	return true
}

// pathList reads the list id of the URL, answering 404 when malformed.
func pathList(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, store.ErrUnknownList.Error(), http.StatusNotFound)
		return 0, false
	}
	return id, true
}

// itemRequest names an item of Explorar: a movie or a content.
type itemRequest struct {
	TMDBID int    `json:"tmdbId"`
	Key    string `json:"key"`
}

// readItem reads the item of an entries request: exactly one of tmdbId and
// key.
func readItem(w http.ResponseWriter, r *http.Request) (itemRequest, bool) {
	var req itemRequest
	if !decode(w, r, &req) {
		return req, false
	}
	if (req.TMDBID > 0) == (req.Key != "") {
		http.Error(w, "falta tmdbId o key", http.StatusBadRequest)
		return req, false
	}
	return req, true
}

func (s *Server) lists(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.ListCards(snap))
}

func (s *Server) createList(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	l, err := s.Store.CreateList(req.Name)
	if err != nil {
		listError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(catalog.ListRef{ID: l.ID, Name: l.Name})
}

func (s *Server) renameList(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := pathList(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.Store.RenameList(id, req.Name); err != nil {
		listError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteList(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := pathList(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteList(id); err != nil {
		listError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addEntry(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := pathList(w, r)
	if !ok {
		return
	}
	req, ok := readItem(w, r)
	if !ok {
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	ref, found := catalog.ItemRef(snap, req.TMDBID, req.Key)
	if !found {
		http.Error(w, "no está en el catálogo", http.StatusNotFound)
		return
	}
	if err := s.Store.AddEntries(id, []string{ref}); err != nil {
		listError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeEntry removes an item from a list: every entry that leads to it.
func (s *Server) removeEntry(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := pathList(w, r)
	if !ok {
		return
	}
	req, ok := readItem(w, r)
	if !ok {
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	if err := s.Store.RemoveEntries(id, catalog.RefsTo(snap, id, req.TMDBID, req.Key)); err != nil {
		listError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

En `internal/server/server.go`, en `Handler`, después de `mux.HandleFunc("GET /api/home", s.home)`:

```go
	mux.HandleFunc("GET /api/lists", s.lists)
	mux.HandleFunc("POST /api/lists", jsonOnly(s.createList))
	mux.HandleFunc("PATCH /api/lists/{id}", jsonOnly(s.renameList))
	mux.HandleFunc("DELETE /api/lists/{id}", jsonOnly(s.deleteList))
	mux.HandleFunc("POST /api/lists/{id}/entries", jsonOnly(s.addEntry))
	mux.HandleFunc("DELETE /api/lists/{id}/entries", jsonOnly(s.removeEntry))
```

En `internal/server/catalog.go`, `explore` pasa a ser:

```go
// explore answers Explorar: the items that match the facets in the query,
// sorted, plus every facet's values counted over the other facets, and the
// list applied, if any.
func (s *Server) explore(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	q, list := catalog.ApplyLists(catalog.ParseQuery(r.URL.Query()), snap)
	all := catalog.Items(snap, s.rt().Config.Roots)
	items := catalog.Filter(all, q.Facets)
	catalog.SortQuery(items, q)
	writeJSON(w, map[string]any{"total": len(items), "query": q, "items": items, "facets": catalog.Counts(all, q.Facets), "list": list})
}
```

- [ ] **Step 4: Correr las pruebas**

Run: `go test ./internal/server/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/lists.go internal/server/lists_test.go internal/server/server.go internal/server/catalog.go internal/server/catalog_test.go
git commit -m "feat(server): lists API; Explorar filters and sorts by list"
```

---

### Task 8: Servidor — API de Colecciones

**Files:**
- Create: `internal/server/collections.go`
- Modify: `internal/server/server.go` (rutas)
- Test: `internal/server/collections_test.go`

**Interfaces:**
- Consumes: `catalog.Collections`, `CollectionFolder` (Task 5); `store.ImportCollection`, `DismissCollection` (Task 2); `writable`, `decode`, `listError` (Task 7).
- Rutas:
  - `GET /api/collections` → `[]catalog.CollectionFolder`
  - `POST /api/collections/import` `{path, name}` o `{path, listId}` → 200 `{"listId": N}`
  - `POST /api/collections/dismiss` `{path}` → 204
  - Una ruta que no es una carpeta pendiente → 404; `name` y `listId` juntos o ninguno → 400.

- [ ] **Step 1: Escribir las pruebas que fallan**

Crear `internal/server/collections_test.go`:

```go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
)

const kubrick = "../cine/Collections/Kubrick"

// putKubrick leaves Amarcord (f1) plus n films in ../cine/Collections/Kubrick
// (fingerprints k1…kn) in the catalog.
func putKubrick(t *testing.T, s *Server, n int) {
	t.Helper()
	rows := []store.FileRow{{Path: moviePath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"}}
	vs := []grouping.Version{{Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
		Members: []grouping.Member{{Path: moviePath, Role: grouping.RoleMain}}}}
	for i := 1; i <= n; i++ {
		p := fmt.Sprintf("%s/Film %d.mkv", kubrick, i)
		rows = append(rows, store.FileRow{Path: p, Size: 10, MTime: 1, Fingerprint: fmt.Sprintf("k%d", i), Kind: "video"})
		vs = append(vs, grouping.Version{Dir: kubrick, Parsed: nameparse.Parsed{Title: fmt.Sprintf("Film %d", i)}, Size: 10, Parts: 1,
			Members: []grouping.Member{{Path: p, Role: grouping.RoleMain}}})
	}
	if err := s.Store.SyncFiles(rows, []string{"../cine"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ReplaceVersions(vs); err != nil {
		t.Fatal(err)
	}
}

func folders(t *testing.T, s *Server) []catalog.CollectionFolder {
	t.Helper()
	var fs []catalog.CollectionFolder
	if code := getJSON(t, s, "/api/collections", &fs); code != 200 {
		t.Fatalf("collections %d", code)
	}
	return fs
}

func listCount(t *testing.T, s *Server) int {
	t.Helper()
	var cards []catalog.ListCard
	getJSON(t, s, "/api/lists", &cards)
	if len(cards) != 1 {
		t.Fatalf("lists %+v", cards)
	}
	return cards[0].Count
}

func TestCollectionsEndpoints(t *testing.T) {
	s, _ := newServer(t)
	putKubrick(t, s, 2)
	fs := folders(t, s)
	if len(fs) != 1 || fs[0].Path != kubrick || fs[0].Name != "Kubrick" || fs[0].New != 2 || len(fs[0].Preview) != 2 || fs[0].List != nil {
		t.Fatalf("folders %+v", fs)
	}
	for body, want := range map[string]int{
		`{"path":"` + kubrick + `"}`:                            http.StatusBadRequest, // neither a name nor a list
		`{"path":"` + kubrick + `","name":"K","listId":1}`:      http.StatusBadRequest, // both
		`{"path":"../cine/Collections/Nada","name":"Nada"}`:     http.StatusNotFound,
		`{"path":"` + kubrick + `","listId":999}`:               http.StatusNotFound,
		`{"path":"` + kubrick + `","name":" "}`:                 http.StatusBadRequest, // a blank name
	} {
		if code := send(t, s, "POST", "/api/collections/import", body).Code; code != want {
			t.Errorf("import %s: %d, want %d", body, code, want)
		}
	}
	rec := send(t, s, "POST", "/api/collections/import", `{"path":"`+kubrick+`","name":"Kubrick"}`)
	var res struct {
		ListID int64 `json:"listId"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &res) != nil || res.ListID == 0 {
		t.Fatalf("import %d: %s", rec.Code, rec.Body)
	}
	if n := listCount(t, s); n != 2 {
		t.Fatalf("count %d", n)
	}
	if fs := folders(t, s); len(fs) != 0 {
		t.Fatalf("still pending %+v", fs)
	}

	// A new film in the folder is offered as an addition to the list.
	putKubrick(t, s, 3)
	fs = folders(t, s)
	if len(fs) != 1 || fs[0].New != 1 || fs[0].Total != 3 || fs[0].List == nil || fs[0].List.ID != res.ListID {
		t.Fatalf("folders %+v", fs)
	}
	if code := send(t, s, "POST", "/api/collections/import", `{"path":"`+kubrick+`","name":"kubrick"}`).Code; code != http.StatusConflict {
		t.Fatalf("taken name %d", code)
	}
	body := fmt.Sprintf(`{"path":%q,"listId":%d}`, kubrick, res.ListID)
	if code := send(t, s, "POST", "/api/collections/import", body).Code; code != 200 {
		t.Fatalf("append %d", code)
	}
	if n := listCount(t, s); n != 3 {
		t.Fatalf("count %d", n)
	}

	// Declining the next one keeps the list as it is.
	putKubrick(t, s, 4)
	if code := send(t, s, "POST", "/api/collections/dismiss", `{"path":"`+kubrick+`"}`).Code; code != http.StatusNoContent {
		t.Fatalf("dismiss %d", code)
	}
	if fs := folders(t, s); len(fs) != 0 {
		t.Fatalf("still pending %+v", fs)
	}
	if n := listCount(t, s); n != 3 {
		t.Fatalf("count %d", n)
	}
	if code := send(t, s, "POST", "/api/collections/dismiss", `{"path":"`+kubrick+`"}`).Code; code != http.StatusNotFound {
		t.Fatalf("dismiss twice %d", code)
	}
}

func TestCollectionsReadOnly(t *testing.T) {
	s, _ := newServer(t)
	putKubrick(t, s, 1)
	s.ReadOnly = true
	for _, url := range []string{"/api/collections/import", "/api/collections/dismiss"} {
		if code := send(t, s, "POST", url, `{"path":"`+kubrick+`","name":"K"}`).Code; code != http.StatusConflict {
			t.Errorf("%s: %d", url, code)
		}
	}
	if fs := folders(t, s); len(fs) != 1 {
		t.Fatalf("folders %+v", fs)
	}
}
```

- [ ] **Step 2: Correr las pruebas y ver que fallan**

Run: `go test ./internal/server/ -run Collections`
Expected: FAIL (las rutas no existen).

- [ ] **Step 3: Implementar**

Crear `internal/server/collections.go`:

```go
package server

import (
	"net/http"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/store"
)

func (s *Server) collections(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	writeJSON(w, catalog.Collections(snap))
}

// pendingFolder finds the folder of Collections/ at path among those with
// something to offer, answering 404 when it is not one.
func pendingFolder(w http.ResponseWriter, snap store.Snapshot, path string) (catalog.CollectionFolder, bool) {
	for _, f := range catalog.Collections(snap) {
		if f.Path == path {
			return f, true
		}
	}
	http.Error(w, "la carpeta no tiene nada para importar", http.StatusNotFound)
	return catalog.CollectionFolder{}, false
}

// importCollection adds a folder's new contents to a new list (name) or to
// an existing one (listId).
func (s *Server) importCollection(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	var req struct {
		Path   string `json:"path"`
		Name   string `json:"name"`
		ListID int64  `json:"listId"`
	}
	if !decode(w, r, &req) {
		return
	}
	if (req.Name != "") == (req.ListID > 0) {
		http.Error(w, "indicá el nombre de una lista nueva o una lista existente", http.StatusBadRequest)
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	f, ok := pendingFolder(w, snap, req.Path)
	if !ok {
		return
	}
	id, err := s.Store.ImportCollection(req.Path, req.Name, req.ListID, f.Fingerprints)
	if err != nil {
		listError(w, err)
		return
	}
	writeJSON(w, map[string]int64{"listId": id})
}

// dismissCollection declines a folder's new contents.
func (s *Server) dismissCollection(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if !decode(w, r, &req) {
		return
	}
	snap, ok := s.snapshot(w)
	if !ok {
		return
	}
	f, ok := pendingFolder(w, snap, req.Path)
	if !ok {
		return
	}
	if err := s.Store.DismissCollection(req.Path, f.Fingerprints); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

En `internal/server/server.go`, debajo de las rutas de listas:

```go
	mux.HandleFunc("GET /api/collections", s.collections)
	mux.HandleFunc("POST /api/collections/import", jsonOnly(s.importCollection))
	mux.HandleFunc("POST /api/collections/dismiss", jsonOnly(s.dismissCollection))
```

- [ ] **Step 4: Correr las pruebas**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/collections.go internal/server/collections_test.go internal/server/server.go
git commit -m "feat(server): import or dismiss Collections/ folders"
```

---

### Task 9: Web — lógica pura (API, listas, facetas, rutas)

**Files:**
- Create: `web/src/lib/listas.js`, `web/src/lib/listas.test.js`
- Modify: `web/src/lib/api.js`, `web/src/lib/facets.js`, `web/src/lib/facets.test.js`, `web/src/lib/router.js`, `web/src/lib/router.test.js`, `web/src/lib/home.test.js`

**Interfaces:**
- Produces:
  - `listas.js`: `MAX_NAME`, `checkName(raw) → {name} | {error}`, `itemBody(item) → {tmdbId} | {key}`, `sameName(a, b)`, `filterLists(lists, q)`, `importChoice(folder, lists, typed) → {kind: 'append'|'existing'|'new', label, body} | {kind: 'invalid', error}`, `movieCount(n)`
  - `api`: `lists()`, `createList(name)`, `renameList(id, name)`, `deleteList(id)`, `addToList(id, item)`, `removeFromList(id, item)`, `collections()`, `importCollection(body)`, `dismissCollection(path)`
  - `facets.js`: faceta `lista` en `FACETS`; orden `agregado-lista` (`list: true`) en `ORDERS`; `defaultOrder(facets)`; `ordersFor(query)`; `withFacet` restablece el orden al cambiar la lista.
  - `router.js`: `/listas` → `{page: 'listas'}`; `/revisar/colecciones` → `{page: 'revisar', tab: 'colecciones'}`.

- [ ] **Step 1: Escribir las pruebas que fallan**

Crear `web/src/lib/listas.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { checkName, filterLists, importChoice, itemBody, movieCount, sameName } from './listas.js'

describe('checkName', () => {
  it('trims and accepts', () => {
    expect(checkName('  Noir ')).toEqual({ name: 'Noir' })
    expect(checkName('á'.repeat(100))).toEqual({ name: 'á'.repeat(100) })
  })
  it('rejects empty and long names', () => {
    expect(checkName('   ').error).toBeTruthy()
    expect(checkName(undefined).error).toBeTruthy()
    expect(checkName('a'.repeat(101)).error).toMatch(/100/)
  })
})

describe('itemBody', () => {
  it('names a movie by its TMDB id and a content by its key', () => {
    expect(itemBody({ kind: 'movie', tmdbId: 7857, key: '' })).toEqual({ tmdbId: 7857 })
    expect(itemBody({ kind: 'version', key: 'f1' })).toEqual({ key: 'f1' })
  })
})

describe('sameName and filterLists', () => {
  const lists = [
    { id: 1, name: 'Películas de Fellini' },
    { id: 2, name: 'Noir' },
  ]
  it('compares names ignoring case', () => {
    expect(sameName('NOIR', 'noir')).toBe(true)
    expect(sameName('Noir', 'Noir 2')).toBe(false)
  })
  it('filters ignoring case and accents', () => {
    expect(filterLists(lists, 'FELLÍNI').map((l) => l.id)).toEqual([1])
    expect(filterLists(lists, '  ')).toEqual(lists)
  })
})

describe('importChoice', () => {
  const lists = [{ id: 4, name: 'Kubrick' }]
  const folder = { path: '../cine/Collections/Kubrick', name: 'Kubrick', total: 3, new: 3, list: null }
  it('adds to the list that already has the name', () => {
    expect(importChoice(folder, lists, 'kubrick')).toEqual({
      kind: 'existing',
      label: 'Agregar a Kubrick',
      body: { path: folder.path, listId: 4 },
    })
  })
  it('creates a list', () => {
    expect(importChoice(folder, lists, ' Kubrick completo ')).toEqual({
      kind: 'new',
      label: 'Importar como lista',
      body: { path: folder.path, name: 'Kubrick completo' },
    })
  })
  it('needs a name', () => {
    expect(importChoice(folder, lists, ' ').kind).toBe('invalid')
  })
  it('adds new contents to an earlier import', () => {
    const again = { ...folder, new: 1, list: { id: 4, name: 'Kubrick' } }
    expect(importChoice(again, lists, '')).toEqual({
      kind: 'append',
      label: 'Agregar 1 a Kubrick',
      body: { path: folder.path, listId: 4 },
    })
  })
})

describe('movieCount', () => {
  it('reads singular and plural', () => {
    expect(movieCount(1)).toBe('1 película')
    expect(movieCount(3)).toBe('3 películas')
    expect(movieCount(0)).toBe('0 películas')
  })
})
```

En `web/src/lib/facets.test.js`, sumar `ordersFor` al import (`import { chips, ordersFor, parse, toSearch, valueLabel, withFacet, withOrder } from './facets.js'`) y agregar al final:

```js
describe('lists', () => {
  it('sorts a list by date added to it by default', () => {
    expect(parse('?lista=7')).toEqual({ facets: { lista: '7' }, order: 'agregado-lista', dir: 'desc' })
    expect(toSearch(parse('?lista=7'))).toBe('?lista=7')
    expect(parse('?lista=7&orden=anio')).toEqual({ facets: { lista: '7' }, order: 'anio', dir: 'desc' })
    expect(toSearch(parse('?lista=7&orden=anio'))).toBe('?lista=7&orden=anio')
    expect(parse('?orden=agregado-lista')).toEqual({ facets: {}, order: 'anio', dir: 'desc' })
  })
  it('resets the order when the list changes', () => {
    expect(withFacet(parse('?lista=7&orden=titulo'), 'lista', null)).toEqual({ facets: {}, order: 'anio', dir: 'desc' })
    expect(withFacet(parse('?decada=1970&orden=titulo'), 'lista', '3')).toEqual({
      facets: { decada: '1970', lista: '3' },
      order: 'agregado-lista',
      dir: 'desc',
    })
  })
  it('offers the list order only with a list', () => {
    expect(ordersFor(parse('')).map((o) => o.value)).not.toContain('agregado-lista')
    expect(ordersFor(parse('?lista=7')).map((o) => o.value)).toContain('agregado-lista')
  })
})
```

En `web/src/lib/router.test.js`, sumar a la tabla de `resolve`:

```js
    ['/listas', { page: 'listas' }],
    ['/revisar/colecciones', { page: 'revisar', tab: 'colecciones' }],
```

En `web/src/lib/home.test.js`, sumar a la tabla de `rowTitle`:

```js
    [{ kind: 'list', value: '3', label: 'Por ver' }, 'Por ver'],
```

- [ ] **Step 2: Correr las pruebas y ver que fallan**

Run: `cd web && npm test`
Expected: FAIL (`listas.js` no existe; `ordersFor` no existe; rutas `notfound`). La fila `list` de `home.test.js` ya pasa: `rowTitle` devuelve la etiqueta por defecto.

- [ ] **Step 3: Implementar**

Crear `web/src/lib/listas.js`:

```js
// Lists: their names, the items they hold, and the Colecciones tab's choices.

export const MAX_NAME = 100

function fold(s) {
  return s
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
}

// checkName trims a list name: { name } when it can be saved, { error }
// when not (the server checks the same).
export function checkName(raw) {
  const name = (raw ?? '').trim()
  if (!name) return { error: 'Escribí un nombre.' }
  if ([...name].length > MAX_NAME) return { error: `El nombre no puede pasar de ${MAX_NAME} caracteres.` }
  return { name }
}

// itemBody is how the API names an item: a movie by its TMDB id, a content
// by its key.
export function itemBody(item) {
  return item.kind === 'movie' ? { tmdbId: item.tmdbId } : { key: item.key }
}

// sameName compares list names as the server does: ignoring case.
export function sameName(a, b) {
  return a.trim().toLowerCase() === b.trim().toLowerCase()
}

// filterLists keeps the lists whose name contains q, ignoring case and
// accents.
export function filterLists(lists, q) {
  const needle = fold(q.trim())
  return needle ? lists.filter((l) => fold(l.name).includes(needle)) : lists
}

// importChoice is the Colecciones tab's main action for a folder, given the
// name typed for it: adding the new contents to the list of an earlier
// import, adding to the list that already has that name, or creating a
// list. body is what POST /api/collections/import takes.
export function importChoice(folder, lists, typed) {
  if (folder.list) {
    return {
      kind: 'append',
      label: `Agregar ${folder.new} a ${folder.list.name}`,
      body: { path: folder.path, listId: folder.list.id },
    }
  }
  const checked = checkName(typed)
  if (checked.error) return { kind: 'invalid', error: checked.error }
  const same = lists.find((l) => sameName(l.name, checked.name))
  if (same) return { kind: 'existing', label: `Agregar a ${same.name}`, body: { path: folder.path, listId: same.id } }
  return { kind: 'new', label: 'Importar como lista', body: { path: folder.path, name: checked.name } }
}

// movieCount reads how many movies a list or a folder has.
export function movieCount(n) {
  return n === 1 ? '1 película' : `${n} películas`
}
```

En `web/src/lib/api.js`:

- sumar arriba: `import { itemBody } from './listas.js'`
- debajo de `const put = …`:

```js
const patch = (path, body) => call('PATCH', path, body)
const del = (path, body) => call('DELETE', path, body ?? {})
```

- dentro de `api`, después de `home: …`:

```js
  lists: () => get('/api/lists'),
  createList: (name) => post('/api/lists', { name }),
  renameList: (id, name) => patch(`/api/lists/${id}`, { name }),
  deleteList: (id) => del(`/api/lists/${id}`),
  addToList: (id, item) => post(`/api/lists/${id}/entries`, itemBody(item)),
  removeFromList: (id, item) => del(`/api/lists/${id}/entries`, itemBody(item)),
  collections: () => get('/api/collections'),
  importCollection: (body) => post('/api/collections/import', body),
  dismissCollection: (path) => post('/api/collections/dismiss', { path }),
```

En `web/src/lib/facets.js`:

- en `FACETS`, después de `{ name: 'subs', label: 'Subtítulos' },`: `{ name: 'lista', label: 'Lista' },`
- `ORDERS` pasa a ser:

```js
export const ORDERS = [
  { value: 'anio', label: 'Año' },
  { value: 'titulo', label: 'Título' },
  { value: 'agregado', label: 'Agregado' },
  { value: 'tamano', label: 'Tamaño' },
  { value: 'agregado-lista', label: 'Agregado a la lista', list: true },
]
```

- debajo de `defaultDir`:

```js
// defaultOrder is the order a query gets when none is chosen: by date added
// to the list when there is one, by year otherwise.
export function defaultOrder(facets) {
  return facets.lista ? 'agregado-lista' : 'anio'
}

// ordersFor lists the orders a query can use: the list's own only with a
// list.
export function ordersFor(query) {
  return ORDERS.filter((o) => !o.list || query.facets.lista)
}
```

- en `parse`, reemplazar la línea de `order`:

```js
  const asked = ORDERS.find((o) => o.value === params.get('orden') && (!o.list || facets.lista))
  const order = asked ? asked.value : defaultOrder(facets)
```

- en `toSearch`, reemplazar las dos líneas de orden:

```js
  const natural = defaultOrder(facets)
  if (order && order !== natural) params.set('orden', order)
  if (dir && dir !== defaultDir(order || natural)) params.set('dir', dir)
```

- en `withFacet`, reemplazar el `return { ...query, facets }` final:

```js
  // Another list (or none) starts from its natural order.
  if (name === 'lista') {
    const order = defaultOrder(facets)
    return { ...query, facets, order, dir: defaultDir(order) }
  }
  return { ...query, facets }
```

En `web/src/lib/router.js`, en `resolve`, después de la ruta de `/bienvenida`:

```js
  if (pathname === '/listas') return { page: 'listas' }
```

y después de `/revisar/duplicados`:

```js
  if (pathname === '/revisar/colecciones') return { page: 'revisar', tab: 'colecciones' }
```

- [ ] **Step 4: Correr las pruebas**

Run: `cd web && npm test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/listas.js web/src/lib/listas.test.js web/src/lib/api.js web/src/lib/facets.js web/src/lib/facets.test.js web/src/lib/router.js web/src/lib/router.test.js web/src/lib/home.test.js
git commit -m "feat(web): lists logic, API calls, lista facet and routes"
```

---

### Task 10: Web — página Listas, navegación y Explorar

**Files:**
- Create: `web/src/pages/Listas.svelte`
- Modify: `web/src/App.svelte`, `web/src/components/Nav.svelte`, `web/src/components/BarraFacetas.svelte`, `web/src/pages/Explorar.svelte`

**Interfaces:**
- Consumes (Task 9): `api.lists`, `createList`, `renameList`, `deleteList`, `removeFromList`, `backdropURL`; `checkName`, `movieCount`; `ordersFor`.
- Produces: página `/listas`; en Explorar, título con el nombre de la lista (`data.list`) y "Quitar de la lista" en cada afiche.

- [ ] **Step 1: Página Listas**

Crear `web/src/pages/Listas.svelte`:

```svelte
<script>
  import { api, backdropURL } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { checkName, movieCount } from '../lib/listas.js'

  let cards = $state(null)
  let creating = $state(false)
  let name = $state('')
  let renaming = $state(null) // id of the list being renamed
  let newName = $state('')
  let menu = $state(null) // id of the list whose ⋯ menu is open
  let loads = 0

  const readOnly = $derived(!!app.status?.readOnly)

  async function load() {
    const id = ++loads
    try {
      const c = await api.lists()
      if (id === loads) cards = c
    } catch (e) {
      if (id === loads) notify(e.message)
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  async function create(event) {
    event.preventDefault()
    const checked = checkName(name)
    if (checked.error) return notify(checked.error)
    try {
      await api.createList(checked.name)
      creating = false
      name = ''
      load()
    } catch (e) {
      notify(e.message)
    }
  }

  async function rename(event, id) {
    event.preventDefault()
    const checked = checkName(newName)
    if (checked.error) return notify(checked.error)
    try {
      await api.renameList(id, checked.name)
      renaming = null
      load()
    } catch (e) {
      notify(e.message)
    }
  }

  async function remove(l) {
    menu = null
    if (!confirm(`¿Borrar la lista “${l.name}”? Las películas y los archivos no se tocan.`)) return
    try {
      await api.deleteList(l.id)
      load()
    } catch (e) {
      notify(e.message)
    }
  }

  function toggleMenu(event, id) {
    event.stopPropagation()
    menu = menu === id ? null : id
  }
</script>

<svelte:head><title>Listas · Cinexplorer</title></svelte:head>
<svelte:window onclick={() => (menu = null)} onkeydown={(e) => e.key === 'Escape' && (menu = null)} />

<header class="top">
  <h1>Listas</h1>
  {#if !readOnly}
    {#if creating}
      <form onsubmit={create}>
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:value={name} placeholder="Nombre de la lista" maxlength="100" autofocus />
        <button class="primary" type="submit">Crear</button>
        <button type="button" onclick={() => ((creating = false), (name = ''))}>Cancelar</button>
      </form>
    {:else}
      <button onclick={() => (creating = true)}>Nueva lista</button>
    {/if}
  {/if}
</header>

{#if cards}
  {#if cards.length}
    <div class="grid">
      {#each cards as l (l.id)}
        <div class="card">
          <a
            class="frame"
            href={`/explorar?lista=${l.id}`}
            style:background-image={l.cover ? `url(${backdropURL(l.cover.tmdbId, l.cover.backdrop)})` : null}
          >
            <span class="shade"></span>
            <span class="caption">
              <span class="name">{l.name}</span>
              <span class="count">{movieCount(l.count)}</span>
            </span>
          </a>
          {#if renaming === l.id}
            <form class="rename" onsubmit={(e) => rename(e, l.id)}>
              <!-- svelte-ignore a11y_autofocus -->
              <input bind:value={newName} maxlength="100" aria-label="Nuevo nombre" autofocus />
              <button class="primary" type="submit">Guardar</button>
              <button type="button" onclick={() => (renaming = null)}>Cancelar</button>
            </form>
          {/if}
          {#if !readOnly}
            <button class="more" aria-label="Más acciones" aria-expanded={menu === l.id} onclick={(e) => toggleMenu(e, l.id)}>⋯</button>
            {#if menu === l.id}
              <div class="menu">
                <button onclick={() => ((menu = null), (renaming = l.id), (newName = l.name))}>Renombrar</button>
                <button onclick={() => remove(l)}>Borrar</button>
              </div>
            {/if}
          {/if}
        </div>
      {/each}
    </div>
  {:else}
    <p class="empty">
      Todavía no hay listas. Creá una con “Nueva lista” o desde la ficha de una película con “+”. Las carpetas de
      <code>Collections/</code> se importan en <a href="/revisar/colecciones">Revisar → Colecciones</a>.
    </p>
  {/if}
{/if}

<style>
  .top {
    display: flex;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
    margin-bottom: 16px;
  }
  h1 {
    margin: 0;
    font-size: 20px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--strong);
  }
  form {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
  input {
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 4px 10px;
    min-width: 0;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 16px;
  }
  .card {
    position: relative;
    min-width: 0;
  }
  .frame {
    position: relative;
    display: block;
    aspect-ratio: 16 / 9;
    border-radius: 3px;
    overflow: hidden;
    background: linear-gradient(160deg, #262c35, #15181d) center / cover no-repeat;
  }
  .frame:hover {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .shade {
    position: absolute;
    inset: 0;
    background: linear-gradient(0deg, rgba(20, 23, 28, 0.9), transparent 60%);
  }
  .caption {
    position: absolute;
    left: 12px;
    right: 12px;
    bottom: 10px;
    display: flex;
    flex-direction: column;
  }
  .name {
    color: var(--strong);
    font-weight: 600;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    overflow-wrap: anywhere;
  }
  .count {
    color: var(--muted);
    font-size: 13px;
  }
  .rename {
    margin-top: 8px;
  }
  .more {
    position: absolute;
    top: 8px;
    right: 8px;
    padding: 0 8px;
    background: rgba(20, 23, 28, 0.7);
  }
  .menu {
    position: absolute;
    top: 38px;
    right: 8px;
    z-index: 15;
    display: flex;
    flex-direction: column;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
  }
  .menu button {
    border: 0;
    text-align: left;
    padding: 6px 14px;
  }
</style>
```

- [ ] **Step 2: Navegación y ruta**

En `web/src/components/Nav.svelte`, entre los enlaces de Explorar y Revisar:

```svelte
    <a href="/listas" class:active={page === 'listas'}>Listas</a>
```

En `web/src/App.svelte`: sumar `import Listas from './pages/Listas.svelte'` (en orden alfabético con los demás imports de páginas) y, después del bloque de `explorar`:

```svelte
  {:else if current.page === 'listas'}
    <Listas />
```

- [ ] **Step 3: Orden de la barra de facetas**

En `web/src/components/BarraFacetas.svelte`, cambiar el import a `import { chips, FACETS, ordersFor, valueLabel, withFacet, withOrder } from '../lib/facets.js'` y el `{#each ORDERS as o (o.value)}` del `<select>` por `{#each ordersFor(query) as o (o.value)}`.

- [ ] **Step 4: Explorar con una lista**

En `web/src/pages/Explorar.svelte`:

- sumar la función de quitar, debajo de `change`:

```js
  // Takes an item out of the list shown; the grid reloads in place.
  async function remove(item) {
    try {
      await api.removeFromList(data.list.id, item)
      load(route.search)
    } catch (e) {
      notify(e.message)
    }
  }
```

- reemplazar el `<svelte:head>` por:

```svelte
<svelte:head><title>{data?.list ? `${data.list.name} · Cinexplorer` : 'Explorar · Cinexplorer'}</title></svelte:head>
```

- reemplazar el bloque `{#if data} … {/if}` por:

```svelte
{#if data}
  {#if data.list}<h1 class="list-title">{data.list.name}</h1>{/if}
  <BarraFacetas {query} facets={data.facets} total={data.total} onchange={change} />
  {#if data.items.length}
    <div class="grid">
      {#each data.items.slice(0, shown) as item (item.kind === 'movie' ? `m${item.tmdbId}` : `v${item.key}`)}
        <div class="cell">
          <Afiche {item} />
          {#if data.list && !app.status?.readOnly}
            <button class="remove" onclick={() => remove(item)} title="Quitar de la lista" aria-label="Quitar de la lista">✕</button>
          {/if}
        </div>
      {/each}
    </div>
    <div bind:this={sentinel}></div>
  {:else if data.list && Object.keys(query.facets).length === 1}
    <p class="empty">Esta lista está vacía. Agregá películas desde su ficha con “+”.</p>
  {:else if filtered}
    <p class="empty">Nada con estos filtros. <a href="/explorar">Quitar filtros</a></p>
  {:else if scanning}
    <p class="empty">Escaneando… Las películas van a aparecer a medida que se encuentren.</p>
  {:else}
    <p class="empty">El catálogo está vacío. Revisá las carpetas en <a href="/ajustes">Ajustes</a>.</p>
  {/if}
{/if}
```

- sumar al `<style>`:

```css
  .list-title {
    margin: 0 0 12px;
    font-size: 20px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--strong);
  }
  .cell {
    position: relative;
    min-width: 0;
  }
  .remove {
    position: absolute;
    top: 6px;
    right: 6px;
    padding: 0 7px;
    font-size: 12px;
    background: rgba(20, 23, 28, 0.8);
    opacity: 0;
  }
  .cell:hover .remove,
  .remove:focus-visible {
    opacity: 1;
  }
  @media (hover: none) {
    .remove {
      opacity: 1;
    }
  }
```

- [ ] **Step 5: Verificar**

Run: `cd web && npm test && npm run build`
Expected: PASS y build sin errores ni advertencias nuevas de Svelte.

Luego abrir la app (`go run ./cmd/cinexplorer` desde la raíz, o el flujo del skill `run`) y comprobar a mano: Nav muestra Listas; crear, renombrar (incluido un nombre repetido → aviso) y borrar una lista; la tarjeta abre `/explorar?lista=<id>` con el título de la lista y el orden "Agregado a la lista"; el orden no aparece sin lista.

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/Listas.svelte web/src/App.svelte web/src/components/Nav.svelte web/src/components/BarraFacetas.svelte web/src/pages/Explorar.svelte
git commit -m "feat(web): Listas page; Explorar shows and edits a list"
```

(`internal/server/dist` se regenera y commitea en la Task 13.)

---

### Task 11: Web — chips de listas en las fichas

**Files:**
- Create: `web/src/components/ChipsListas.svelte`
- Modify: `web/src/pages/Pelicula.svelte`, `web/src/pages/Version.svelte`

**Interfaces:**
- Consumes (Tasks 7, 9): `d.lists` de las fichas; `api.lists`, `createList`, `addToList`, `removeFromList`; `checkName`, `filterLists`, `sameName`.
- Produces: `<ChipsListas lists item onchanged />` — `lists`: `[{id, name}]`; `item`: `{kind: 'movie', tmdbId}` o `{kind: 'version', key}`; `onchanged()`: recargar la ficha.

- [ ] **Step 1: Componente**

Crear `web/src/components/ChipsListas.svelte`:

```svelte
<script>
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { checkName, filterLists, sameName } from '../lib/listas.js'

  // lists: the lists that hold the item ({id, name}); item: {kind: 'movie',
  // tmdbId} or {kind: 'version', key}; onchanged(): its lists changed.
  let { lists, item, onchanged } = $props()

  let open = $state(false)
  let all = $state([])
  let q = $state('')
  let busy = $state(false)
  let box = $state()

  const readOnly = $derived(!!app.status?.readOnly)
  const member = $derived(new Set(lists.map((l) => l.id)))
  const shown = $derived(filterLists(all, q))
  const canCreate = $derived(!checkName(q).error && !all.some((l) => sameName(l.name, q)))

  async function toggleMenu() {
    open = !open
    q = ''
    if (!open) return
    try {
      all = await api.lists()
    } catch (e) {
      notify(e.message)
      open = false
    }
  }

  async function toggle(l) {
    if (busy) return
    busy = true
    try {
      await (member.has(l.id) ? api.removeFromList(l.id, item) : api.addToList(l.id, item))
      onchanged()
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }

  async function create(event) {
    event.preventDefault()
    if (busy || !canCreate) return
    busy = true
    try {
      const l = await api.createList(q.trim())
      await api.addToList(l.id, item)
      all = [...all, l]
      q = ''
      onchanged()
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }

  function onwindowclick(event) {
    if (open && box && !box.contains(event.target)) open = false
  }
</script>

<svelte:window onclick={onwindowclick} onkeydown={(e) => e.key === 'Escape' && (open = false)} />

<div class="chips" bind:this={box}>
  {#each lists as l (l.id)}
    <a class="chip" href={`/explorar?lista=${l.id}`}>{l.name}</a>
  {/each}
  {#if !readOnly}
    <button class="chip add" onclick={toggleMenu} aria-expanded={open} aria-label="Agregar a una lista" title="Agregar a una lista">
      +
    </button>
    {#if open}
      <div class="menu">
        <form onsubmit={create}>
          <!-- svelte-ignore a11y_autofocus -->
          <input type="search" bind:value={q} placeholder="Buscar o crear una lista…" maxlength="100" autofocus />
        </form>
        <ul>
          {#each shown as l (l.id)}
            <li>
              <label>
                <input type="checkbox" checked={member.has(l.id)} disabled={busy} onchange={() => toggle(l)} />
                {l.name}
              </label>
            </li>
          {:else}
            {#if !q.trim()}<li class="none">Todavía no hay listas.</li>{/if}
          {/each}
        </ul>
        {#if canCreate}
          <button class="create" onclick={create} disabled={busy}>Crear “{q.trim()}”</button>
        {/if}
      </div>
    {/if}
  {/if}
</div>

<style>
  .chips {
    position: relative;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 12px 0;
  }
  .chip {
    border: 1px solid var(--line-strong);
    border-radius: 999px;
    padding: 1px 10px;
    font-size: 13px;
    color: var(--muted);
  }
  a.chip:hover {
    color: var(--strong);
    border-color: var(--accent);
  }
  .add {
    padding: 1px 9px;
  }
  .menu {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    z-index: 15;
    width: 260px;
    max-height: 50vh;
    overflow-y: auto;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
    padding: 8px;
  }
  input[type='search'] {
    width: 100%;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 4px 8px;
  }
  ul {
    list-style: none;
    margin: 6px 0 0;
    padding: 0;
  }
  label {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 2px;
    cursor: pointer;
  }
  .none {
    color: var(--faint);
    padding: 4px 2px;
  }
  .create {
    width: 100%;
    margin-top: 6px;
    text-align: left;
  }
</style>
```

- [ ] **Step 2: Ficha de película**

En `web/src/pages/Pelicula.svelte`: sumar `import ChipsListas from '../components/ChipsListas.svelte'` y, dentro de `<div class="body">`, como primer hijo:

```svelte
    <ChipsListas lists={d.lists} item={{ kind: 'movie', tmdbId: m.tmdbId }} onchanged={load} />
```

- [ ] **Step 3: Ficha de versión**

En `web/src/pages/Version.svelte`: sumar `import ChipsListas from '../components/ChipsListas.svelte'` y, justo después del cierre `</section>` de `<section class="head">`:

```svelte
  {#if !key.startsWith('id:')}
    <ChipsListas lists={d.lists} item={{ kind: 'version', key }} onchanged={load} />
  {/if}
```

(una versión sin huella —`id:<n>`— no se puede guardar en una lista).

- [ ] **Step 4: Verificar**

Run: `cd web && npm test && npm run build`
Expected: PASS, sin advertencias nuevas.

A mano: en una ficha de película, "+" abre el menú; marcar agrega (aparece el chip), desmarcar quita; escribir un nombre nuevo muestra "Crear “…”" y crea la lista con la película; un nombre existente no ofrece crear. Lo mismo en la ficha de una versión sin identificar. En modo consulta no aparece "+".

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ChipsListas.svelte web/src/pages/Pelicula.svelte web/src/pages/Version.svelte
git commit -m "feat(web): list chips with a + menu in the movie and version pages"
```

---

### Task 12: Web — Revisar → Colecciones

**Files:**
- Create: `web/src/components/Colecciones.svelte`
- Modify: `web/src/pages/Revisar.svelte`

**Interfaces:**
- Consumes (Tasks 8, 9): `api.collections`, `importCollection`, `dismissCollection`, `lists`; `importChoice`, `movieCount`; `Afiche`.
- Produces: `<Colecciones folders ondone />` — `ondone(path)`: la carpeta se importó o se descartó.

- [ ] **Step 1: Componente**

Crear `web/src/components/Colecciones.svelte`:

```svelte
<script>
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { importChoice, movieCount } from '../lib/listas.js'
  import Afiche from './Afiche.svelte'

  // folders: the folders of Collections/ with something to offer;
  // ondone(path): one was imported or dismissed.
  let { folders, ondone } = $props()

  let lists = $state([])
  let names = $state({}) // the name typed for each folder, by path
  let busy = $state(false)

  const readOnly = $derived(!!app.status?.readOnly)

  async function loadLists() {
    try {
      lists = await api.lists()
    } catch (e) {
      notify(e.message)
    }
  }

  $effect(() => {
    loadLists()
  })

  const nameOf = (f) => names[f.path] ?? f.name

  async function run(f, action) {
    if (busy) return
    busy = true
    try {
      await action()
      ondone(f.path)
      loadLists()
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }

  function addTo(f, event) {
    const id = Number(event.currentTarget.value)
    event.currentTarget.value = ''
    if (id) run(f, () => api.importCollection({ path: f.path, listId: id }))
  }
</script>

{#if folders.length === 0}
  <p class="empty">No hay carpetas de <code>Collections/</code> para importar.</p>
{/if}

{#each folders as f (f.path)}
  {@const choice = importChoice(f, lists, nameOf(f))}
  <section class="folder">
    <header>
      <div class="info">
        <h2>{f.name}</h2>
        <div class="path">{f.path}</div>
        <div class="count">
          {#if f.list}
            {f.new} {f.new === 1 ? 'nueva' : 'nuevas'} · ya importada en <a href={`/explorar?lista=${f.list.id}`}>{f.list.name}</a>
          {:else}
            {movieCount(f.total)}
          {/if}
        </div>
      </div>
      {#if !readOnly}
        <div class="actions">
          {#if !f.list}
            <input
              value={nameOf(f)}
              oninput={(e) => (names[f.path] = e.currentTarget.value)}
              maxlength="100"
              aria-label="Nombre de la lista"
            />
          {/if}
          {#if choice.kind === 'invalid'}
            <span class="error">{choice.error}</span>
          {:else}
            <button class="primary" disabled={busy} onclick={() => run(f, () => api.importCollection(choice.body))}>
              {choice.label}
            </button>
          {/if}
          {#if !f.list && lists.length}
            <select disabled={busy} onchange={(e) => addTo(f, e)} aria-label="Agregar a una lista existente">
              <option value="">Agregar a…</option>
              {#each lists as l (l.id)}
                <option value={l.id}>{l.name}</option>
              {/each}
            </select>
          {/if}
          <button disabled={busy} onclick={() => run(f, () => api.dismissCollection(f.path))}>
            {f.list ? 'Descartar novedades' : 'Descartar'}
          </button>
        </div>
      {/if}
    </header>
    <div class="strip">
      {#each f.preview as item (item.kind === 'movie' ? `m${item.tmdbId}` : `v${item.key}`)}
        <Afiche {item} />
      {/each}
    </div>
  </section>
{/each}

<style>
  .folder {
    padding: 16px 0;
    border-bottom: 1px solid var(--line);
  }
  header {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
  }
  .info {
    min-width: 0;
  }
  h2 {
    margin: 0;
    font-size: 16px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--strong);
  }
  .path {
    color: var(--faint);
    font-size: 13px;
    overflow-wrap: anywhere;
  }
  .count {
    color: var(--muted);
    font-size: 13px;
    margin-top: 2px;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
  input,
  select {
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 4px 8px;
    min-width: 0;
  }
  .error {
    color: var(--warn);
    font-size: 13px;
  }
  .strip {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
    gap: 10px;
    margin-top: 12px;
    max-width: 900px;
  }
</style>
```

- [ ] **Step 2: Pestaña en Revisar**

En `web/src/pages/Revisar.svelte`:

- sumar `import Colecciones from '../components/Colecciones.svelte'`
- sumar el estado `let folders = $state(null)` debajo de `let dups`
- en `load`, pedir las tres cosas:

```js
      const [q, d, c] = await Promise.all([api.unidentified(), api.duplicates(), api.collections()])
      if (id !== loads) return // a newer request is on its way
      ;[queue, dups, folders] = [q, d, c]
```

- debajo de `resolved`:

```js
  // A folder was imported or dismissed: drop it without reloading.
  function folderDone(path) {
    loads++
    folders = folders.filter((f) => f.path !== path)
  }
```

- en `<nav class="tabs">`, después del enlace de Duplicados:

```svelte
  <a href="/revisar/colecciones" class:on={tab === 'colecciones'}>
    Colecciones {#if folders}({folders.length}){/if}
  </a>
```

- después de `{:else if tab === 'duplicados' && dups}` y su contenido:

```svelte
{:else if tab === 'colecciones' && folders}
  <Colecciones {folders} ondone={folderDone} />
```

- [ ] **Step 3: Verificar**

Run: `cd web && npm test && npm run build`
Expected: PASS, sin advertencias nuevas.

A mano, con una carpeta `…/Collections/<nombre>/` en una raíz: aparece en Revisar → Colecciones con su conteo y afiches; "Importar como lista" crea la lista y la carpeta desaparece; si el nombre ya existe, el botón pasa a "Agregar a <lista>"; "Agregar a…" suma a otra lista; "Descartar" la saca para siempre.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Colecciones.svelte web/src/pages/Revisar.svelte
git commit -m "feat(web): Revisar → Colecciones imports Collections/ folders as lists"
```

---

### Task 13: Build, documentación y spec

**Files:**
- Regenerate: `internal/server/dist/**`
- Modify: `README.md`, `README.es.md`, `docs/superpowers/next-session-prompt.md`

- [ ] **Step 1: Build del frontend**

Run: `cd web && npm run build`
Expected: `internal/server/dist` actualizado sin errores.

- [ ] **Step 2: README.md**

- En la lista de funciones, después del ítem **Explore**, agregar:

```markdown
- **Lists** (*Listas*): your own lists of movies, created from the Lists page or
  with **+** on a movie's page. A list opens as Explore filtered by it, sorted
  by date added (or any other order). Unidentified files can go in a list too,
  and show up as their movie once identified. Home shows the lists you changed
  last.
```

- En el ítem **Explore**, sumar "list" a la enumeración de facetas ("…collection, list, disk location and status").
- En el ítem **Review**, agregar al final: `, and **Collections**: each subfolder of a \`Collections/\` folder, offered as a list to import or dismiss; files added to it later are offered again.`
- En la tabla de la API, después de la fila de `GET /api/home?seed=`:

```markdown
| `GET /api/lists`, `POST /api/lists` | The lists (name, count, cover), or create one: `{"name"}`. |
| `PATCH /api/lists/{id}`, `DELETE /api/lists/{id}` | Rename (`{"name"}`) or delete a list; files are never touched. |
| `POST /api/lists/{id}/entries`, `DELETE /api/lists/{id}/entries` | Add or remove an item: `{"tmdbId"}` or `{"key"}` (an unidentified item's fingerprint). |
| `GET /api/collections` | `Collections/` folders with something to import. |
| `POST /api/collections/import`, `POST /api/collections/dismiss` | Import a folder into a new list (`{"path", "name"}`) or an existing one (`{"path", "listId"}`), or dismiss it (`{"path"}`). |
```

- En la fila de `GET /api/explore?…`, sumar `&lista=` después de `coleccion=`; y en la frase "`POST` requests require `Content-Type: application/json`" cambiar `POST` por "Write (`POST`, `PUT`, `PATCH`, `DELETE`)".
- En la tabla del roadmap: `| 5. Curation | lists, importing \`Collections/\` | ✅ |`

- [ ] **Step 3: README.es.md**

Los mismos cambios, en español:

```markdown
- **Listas:** listas propias de películas, creadas desde la página Listas o con
  **+** en la ficha. Una lista se abre como Explorar filtrado por ella, ordenada
  por fecha de agregado (o cualquier otro orden). También pueden guardarse
  archivos sin identificar, que se muestran como su película cuando se
  identifican. Inicio muestra las listas cambiadas hace menos.
```

- faceta "lista" en la enumeración de Explorar; en Revisar: `, y **Colecciones**: cada subcarpeta de una carpeta \`Collections/\`, ofrecida como lista para importar o descartar; lo que se agregue después se vuelve a ofrecer.`
- filas de la API (descripciones en español: "Las listas (nombre, cantidad, portada), o crear una: `{"name"}`.", "Renombrar (`{"name"}`) o borrar una lista; nunca toca archivos.", "Agregar o quitar un ítem: `{"tmdbId"}` o `{"key"}` (la huella de un ítem sin identificar).", "Carpetas de `Collections/` con algo para importar.", "Importar una carpeta a una lista nueva (`{"path", "name"}`) o existente (`{"path", "listId"}`), o descartarla (`{"path"}`).")
- roadmap: `| 5. Curaduría | listas, importación de \`Collections/\` | ✅ |`

- [ ] **Step 4: Prompt de la próxima sesión**

Reemplazar el cuerpo de `docs/superpowers/next-session-prompt.md` (entre las líneas `---`) por un párrafo que diga que las etapas 1 a 5 están en `main`, que el alcance v1 del spec general está completo, y que la próxima sesión arranca con `superpowers:brainstorming` para lo que venga (fase 2 del spec general: acciones sobre archivos, datos de visionado).

- [ ] **Step 5: Verificación completa**

Run: `go test ./... && (cd web && npm test)`
Expected: PASS
Run: `git status --porcelain internal/server/dist`
Expected: solo los archivos regenerados en el Step 1 (se commitean ahora).

- [ ] **Step 6: Commit**

```bash
git add internal/server/dist README.md README.es.md docs/superpowers/next-session-prompt.md
git commit -m "docs: lists and Collections/ import; rebuild web"
```

---

### Task 14: Prueba real

Con la colección real (las raíces configuradas en la app):

- [ ] **Step 1:** Arrancar la app, esperar el escaneo, abrir Revisar → Colecciones. Anotar cuántas carpetas aparecen y que los conteos coincidan con las subcarpetas de `Collections/` en disco.
- [ ] **Step 2:** Importar dos carpetas (una con nombre nuevo, otra a una lista existente) y descartar una tercera.
- [ ] **Step 3:** Reescanear (Ajustes o `POST /api/scan`). Comprobar que las listas, sus entradas y las decisiones sobreviven, y que las carpetas decididas no reaparecen.
- [ ] **Step 4:** En una lista importada, corregir la identificación de un archivo (⋯ → buscar otra película) y comprobar que la lista muestra la película corregida.
- [ ] **Step 5:** Agregar a mano una película y un ítem sin identificar a una lista desde sus fichas; quitarlos desde Explorar; ver la fila de la lista en Inicio (≥3 películas).
- [ ] **Step 6:** Anotar cualquier desvío como issue o corregirlo antes del merge (con prueba que lo cubra).
