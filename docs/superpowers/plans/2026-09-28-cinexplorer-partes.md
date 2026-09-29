# Películas en partes — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que una película en varios archivos (p. ej. *Shoah*, 7 partes) sea una sola versión, agrupada automáticamente cuando los nombres varían después del marcador de parte, o a mano desde la interfaz.

**Architecture:** `grouping.buildDir` agrupa por el texto anterior al marcador cuando los nombres difieren después de él. El agrupado manual es una tabla `part_links` (huella del seguidor → huella del líder) que `Store.ApplyPartLinks` aplica sobre lo ya persistido después de cada escaneo y después de cada vínculo nuevo. El servidor suma las acciones `part-of` y `unlink` al endpoint `/api/identify`; la tarjeta de versión suma dos entradas al menú ⋯.

**Tech Stack:** Go (stdlib, `modernc.org/sqlite`), Svelte 5 + Vite + Vitest.

**Spec:** `docs/superpowers/specs/2026-09-28-cinexplorer-partes-design.md`

## Global Constraints

- Solo lectura sobre los videos: nunca se mueven, renombran ni borran (README).
- Rutas del catálogo con `/`, relativas a la carpeta de la app (`../cine/...`).
- Una identificación se guarda por huella del archivo representante de la versión (el de menor `part`).
- Catálogos viejos abiertos en modo consulta pueden no tener tablas nuevas: todo acceso a `part_links` se protege con un flag `hasPartLinks`, como `hasIdentity`.
- Textos de la interfaz y del servidor en español (es-AR).
- Mensajes de commit sin `Co-Authored-By` (instrucción global del usuario, prevalece sobre cualquier otra).
- `internal/server/dist` está commiteado: después de tocar `web/`, `npm run build` y commitear el resultado (CI lo verifica).
- Los cambios sin commitear que ya hay en `internal/identify/match.go`, `internal/quality/*` no son de este trabajo: no incluirlos (usar `git add` con rutas explícitas).

## Estructura de archivos

| Archivo | Responsabilidad |
|---|---|
| `internal/grouping/grouping.go` (modificar) | Agrupar por prefijo del marcador; exentar de la regla de extra por tamaño |
| `internal/grouping/grouping_test.go` (modificar) | Pruebas del agrupado |
| `internal/store/schema.sql` (modificar) | Tabla `part_links` |
| `internal/store/store.go` (modificar) | Flag `hasPartLinks`, campo `PartLinked` |
| `internal/store/part_links.go` (crear) | `SetPartLink`, `ApplyPartLinks`, `Unlink`, `attachPartLinks` |
| `internal/store/part_links_test.go` (crear) | Pruebas del store |
| `internal/scan/scan.go` (modificar) | Llamar `ApplyPartLinks` tras `ReplaceVersionsIn` |
| `internal/scan/scan_test.go` (modificar) | Prueba de escaneo con vínculo |
| `internal/server/identify.go` (modificar) | Acciones `part-of` y `unlink` |
| `internal/server/identify_test.go` (modificar) | Pruebas de las acciones |
| `web/src/lib/partes.js` (crear) + `partes.test.js` | Filtro de versiones candidatas a líder |
| `web/src/lib/api.js` (modificar) | `versions`, `partOf`, `unlink` |
| `web/src/components/SelectorVersion.svelte` (crear) | Selector de la versión líder |
| `web/src/components/TarjetaVersion.svelte` (modificar) | Entradas del menú ⋯ |
| `internal/server/dist/**` (regenerar), `README.md`, `README.es.md` | Build y documentación |

---

### Task 1: Agrupado automático por prefijo del marcador

**Files:**
- Modify: `internal/grouping/grouping.go` (`buildDir`, `isExtra`, `splitPart`)
- Modify: `docs/superpowers/specs/2026-09-28-cinexplorer-partes-design.md` (§3)
- Test: `internal/grouping/grouping_test.go`

**Interfaces:**
- Consumes: `splitPart(name) (base string, part int)`, `partRe`, `extraRe`, `extraDirs`, `nameparse.Parse` (ya existen).
- Produces (internas del paquete): `splitPartPrefix(name string) (prefix string, part int)`; `type partSet struct{ varying bool }`; `partSets(es []Entry) map[string]partSet` (clave: prefijo en minúsculas); `isExtra(e Entry, name string, largest int64, sized bool) bool`.

Regla: en una carpeta, los videos con marcador de parte cuyo prefijo (texto anterior al marcador, en minúsculas) coincide forman un **conjunto de partes** si hay ≥ 2 números de parte distintos y **ningún número se repite** (si se repite, hay varias versiones de la misma película y se conserva el comportamiento actual). Un conjunto es **variable** si los nombres base completos difieren entre sus archivos; solo entonces se agrupa por prefijo y el nombre parseado sale del prefijo. Todo conjunto (variable o no) queda exento de la regla de extra por tamaño; los extras por nombre o carpeta siguen valiendo y no cuentan para formar el conjunto.

- [ ] **Step 1: Escribir las pruebas que fallan**

Agregar al final de `internal/grouping/grouping_test.go`:

```go
func TestPartsWithDifferentTitlesAfterMarkerGroup(t *testing.T) {
	dir := "../cine-ordenar/Shoah (1985)"
	vs := Build([]Entry{
		{dir + "/Shoah - Part 1 - Auschwitz.mkv", 5_000_000_000, mediafile.Video},
		{dir + "/Shoah - Part 2 - Treblinka.mkv", 4_000_000_000, mediafile.Video},
		{dir + "/Shoah - Part 3 - Sobibor.mkv", 3_000_000_000, mediafile.Video},
		{dir + "/Shoah - Part 4 - Coda.mkv", 100_000_000, mediafile.Video}, // <15% del mayor: no es un extra
		{dir + "/Shoah - Part 2 - Treblinka.es.srt", 50_000, mediafile.Subtitle},
	}, roots)
	v := only(t, vs)
	if v.Parts != 4 || v.Parsed.Title != "Shoah" || v.Size != 12_100_000_000 {
		t.Fatalf("got parts=%d title=%q size=%d", v.Parts, v.Parsed.Title, v.Size)
	}
	var parts []int
	for _, m := range v.Members {
		if m.Role == RoleExtra {
			t.Fatalf("a part became an extra: %+v", m)
		}
		if m.Role == RoleMain {
			parts = append(parts, m.Part)
		}
	}
	if !reflect.DeepEqual(parts, []int{1, 2, 3, 4}) {
		t.Fatalf("parts %v", parts)
	}
	if got := v.SubLangs(); !reflect.DeepEqual(got, []string{"es"}) {
		t.Fatalf("sub langs %v", got)
	}
}

func TestLonePartWithSuffixDoesNotGroup(t *testing.T) {
	dir := "../cine-ordenar"
	vs := Build([]Entry{
		{dir + "/Movie - Part 1 - Subtitle.mkv", 1_000_000_000, mediafile.Video},
		{dir + "/Other.mkv", 1_000_000_000, mediafile.Video},
	}, roots)
	if len(vs) != 2 || vs[0].Parts != 1 || vs[1].Parts != 1 {
		t.Fatalf("got %+v", vs)
	}
}

// Two rips of the same two-disc film in one folder are two versions, not
// four parts.
func TestRepeatedPartNumbersKeepSeparateVersions(t *testing.T) {
	dir := "../cine-ordenar/Movie"
	vs := Build([]Entry{
		{dir + "/Movie.CD1.720p.avi", 700_000_000, mediafile.Video},
		{dir + "/Movie.CD2.720p.avi", 700_000_000, mediafile.Video},
		{dir + "/Movie.CD1.1080p.avi", 700_000_000, mediafile.Video},
		{dir + "/Movie.CD2.1080p.avi", 700_000_000, mediafile.Video},
	}, roots)
	if len(vs) != 2 || vs[0].Parts != 2 || vs[1].Parts != 2 {
		t.Fatalf("got %+v", vs)
	}
}

func TestSameNamePartsAreNotExtrasBySize(t *testing.T) {
	dir := "../cine-ordenar/Movie"
	v := only(t, Build([]Entry{
		{dir + "/Movie - CD1.avi", 1_000_000_000, mediafile.Video},
		{dir + "/Movie - CD2.avi", 50_000_000, mediafile.Video},
	}, roots))
	if v.Parts != 2 {
		t.Fatalf("parts %d, members %+v", v.Parts, v.Members)
	}
}

func TestMarkedExtrasDoNotFormPartSets(t *testing.T) {
	dir := "../cine-ordenar/Movie"
	v := only(t, Build([]Entry{
		{dir + "/Movie - Part 1 - Main.mkv", 1_000_000_000, mediafile.Video},
		{dir + "/Movie - Part 2 - Making of.mkv", 900_000_000, mediafile.Video},
	}, roots))
	if v.Parts != 1 {
		t.Fatalf("parts %d, members %+v", v.Parts, v.Members)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/grouping/ -run 'TestPartsWithDifferentTitles|TestLonePart|TestRepeatedPartNumbers|TestSameNameParts|TestMarkedExtras' -v`
Expected: `TestPartsWithDifferentTitlesAfterMarkerGroup` y `TestSameNamePartsAreNotExtrasBySize` FALLAN; los otros tres pasan (son guardas contra regresiones).

- [ ] **Step 3: Implementar**

En `internal/grouping/grouping.go`, reemplazar `isExtra` por:

```go
// markedExtra is true for what the name or the folder says is an extra.
func markedExtra(e Entry, name string) bool {
	if extraDirs[strings.ToLower(path.Base(path.Dir(e.Path)))] {
		return true
	}
	return extraRe.MatchString(name)
}

// isExtra also treats a video under 15% of the largest one as an extra,
// unless it is known to be a part of a multi-part film (sized).
func isExtra(e Entry, name string, largest int64, sized bool) bool {
	if markedExtra(e, name) {
		return true
	}
	return !sized && largest > 0 && e.Size*100 < largest*15
}
```

Debajo de `splitPart`, agregar:

```go
// splitPartPrefix returns the text before the part marker of name (without
// the separators that lead into it) and the part number; 0 when there is no
// marker.
func splitPartPrefix(name string) (string, int) {
	loc := partRe.FindStringSubmatchIndex(name)
	if loc == nil {
		return name, 0
	}
	n, _ := strconv.Atoi(name[loc[2]:loc[3]])
	return strings.TrimSpace(name[:loc[0]]), n
}

// partSet describes videos of one folder that are the parts of one film:
// the same text before the part marker and different part numbers.
type partSet struct {
	// varying: the names differ after the marker ("Shoah - Part 1 -
	// Auschwitz"), so the text before it is what groups them.
	varying bool
}

// partSets finds the part sets of a folder, by lowercase prefix. A prefix
// needs at least two part numbers and none repeated: a repeated number means
// several rips of the same film side by side, which stay separate versions.
func partSets(es []Entry) map[string]partSet {
	type seen struct {
		parts map[int]int
		bases map[string]bool
	}
	found := map[string]*seen{}
	for _, e := range es {
		if e.Kind != mediafile.Video {
			continue
		}
		name := stem(e.Path)
		if markedExtra(e, name) {
			continue
		}
		prefix, part := splitPartPrefix(name)
		if part == 0 || prefix == "" {
			continue
		}
		k := strings.ToLower(prefix)
		s := found[k]
		if s == nil {
			s = &seen{parts: map[int]int{}, bases: map[string]bool{}}
			found[k] = s
		}
		base, _ := splitPart(name)
		s.parts[part]++
		s.bases[strings.ToLower(base)] = true
	}
	out := map[string]partSet{}
	for k, s := range found {
		if len(s.parts) < 2 {
			continue
		}
		repeated := false
		for _, n := range s.parts {
			if n > 1 {
				repeated = true
			}
		}
		if !repeated {
			out[k] = partSet{varying: len(s.bases) > 1}
		}
	}
	return out
}
```

En `buildDir`, justo después de `var dvd *Version`, agregar `sets := partSets(es)`, y reemplazar el bloque del caso `mediafile.Video` hasta `base, part := splitPart(name)` por:

```go
		case mediafile.Video:
			name := stem(e.Path)
			prefix, n := splitPartPrefix(name)
			set, inSet := sets[strings.ToLower(prefix)]
			inSet = inSet && n > 0
			if isExtra(e, name, largest, inSet) {
				extras = append(extras, Member{Path: e.Path, Role: RoleExtra})
				continue
			}
			base, part := splitPart(name)
			if inSet && set.varying {
				base = prefix // the titles after the marker differ: group by what precedes it
			}
```

El resto del caso (comentario sobre `key`, `key := strings.ToLower(base)`, etc.) queda igual.

- [ ] **Step 4: Verificar que pasan todas**

Run: `go vet ./internal/grouping/ && go test ./internal/grouping/ -v`
Expected: PASS de todo el paquete (incluidos `TestPartsAndExtras`, `TestSplitPart` y los previos). Si algún test viejo llama a `isExtra` con la firma anterior, agregarle el argumento `false`.

- [ ] **Step 5: Completar el spec (§3) con lo que se refinó**

En `docs/superpowers/specs/2026-09-28-cinexplorer-partes-design.md`, en la lista de salvaguardas de §3, agregar al final:

```markdown
6. Si un número de parte se repite dentro del mismo prefijo (`Movie.CD1.720p`, `Movie.CD2.720p`, `Movie.CD1.1080p`, `Movie.CD2.1080p`), son varias copias de la misma película y no se agrupan por prefijo: queda el comportamiento actual.
7. Se agrupa por prefijo solo si los nombres base difieren; si son iguales salvo el marcador, sigue valiendo el agrupado de siempre (que conserva las etiquetas de calidad que siguen al marcador, como `720p`). La exención de la regla de extra por tamaño vale para todo conjunto de partes, también para los de mismo nombre.
```

- [ ] **Step 6: Commit**

```bash
git add internal/grouping/grouping.go internal/grouping/grouping_test.go docs/superpowers/specs/2026-09-28-cinexplorer-partes-design.md
git commit -m "feat(grouping): group multi-part films whose names differ after the part marker"
```

---

### Task 2: Vínculos de partes en el store

**Files:**
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/store.go` (struct `Store`, struct `VersionView`, `openDB`, `versions`)
- Create: `internal/store/part_links.go`
- Test: `internal/store/part_links_test.go`

**Interfaces:**
- Consumes: `(*Store).representatives(tx querier) (map[int64]representative, error)` con `representative{fingerprint, path string}`; `(*Store).IsRepresentative(fp string) (bool, error)`; `ErrUnknownFingerprint`; `querier`.
- Produces:
  - `var ErrPartLink = errors.New("una película no puede ser parte de sí misma")`
  - `func (s *Store) SetPartLink(follower, leader string) error` — valida (ambas huellas representantes, distintas, sin ciclo), guarda el vínculo y lo aplica.
  - `func (s *Store) ApplyPartLinks() (int, error)` — devuelve cuántas versiones se fusionaron.
  - `func (s *Store) Unlink(leader string) (int, error)` — borra los vínculos que llegan a `leader` (directos y encadenados); devuelve cuántos.
  - `VersionView.PartLinked bool` (`json:"partLinked"`): la versión recibió partes a mano.

Comportamiento de `ApplyPartLinks` (todo en una transacción): por cada seguidor cuyo líder final (siguiendo la cadena; se ignoran los ciclos) es una versión presente y distinta, los archivos `main` del seguidor pasan a la versión del líder con `part` a continuación del mayor (los del líder con `part = 0` pasan a 1 antes), ordenados por `(part, path)`; subtítulos y extras pasan con su rol; se borra la versión vacía; se recalculan `size`, `parts` y `sub_langs` del líder. Seguidores de un mismo líder se aplican ordenados por la ruta de su representante. Si el seguidor o el líder no son representantes ahora (ya aplicado, ausente), no se hace nada y el vínculo se conserva. Limitación aceptada: si dos versiones tienen el mismo representante (copias idénticas), solo la de menor id se fusiona.

- [ ] **Step 1: Escribir las pruebas que fallan**

Crear `internal/store/part_links_test.go`:

```go
package store

import (
	"errors"
	"reflect"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/mediafile"
	"cinexplorer/internal/nameparse"
)

const (
	pA  = "../cine/Shoah/Shoah - Part 1.mkv"
	pB  = "../cine/Shoah/Shoah - Part 2 - Treblinka.mkv"
	pBS = "../cine/Shoah/Shoah - Part 2 - Treblinka.es.srt"
	pC  = "../cine/Shoah/Shoah - Part 3 - Sobibor.mkv"
)

func partVersions(only ...string) []grouping.Version {
	main := func(p string) grouping.Member { return grouping.Member{Path: p, Role: grouping.RoleMain} }
	all := map[string]grouping.Version{
		"A": {Dir: "../cine/Shoah", Parsed: nameparse.Parsed{Title: "Shoah"}, Size: 100, Parts: 1, Members: []grouping.Member{main(pA)}},
		"B": {Dir: "../cine/Shoah", Parsed: nameparse.Parsed{Title: "Shoah 2"}, Size: 90, Parts: 1,
			Members: []grouping.Member{main(pB), {Path: pBS, Role: grouping.RoleSubtitle, Lang: "es"}}},
		"C": {Dir: "../cine/Shoah", Parsed: nameparse.Parsed{Title: "Shoah 3"}, Size: 80, Parts: 1, Members: []grouping.Member{main(pC)}},
	}
	var out []grouping.Version
	for _, k := range only {
		out = append(out, all[k])
	}
	return out
}

func partCatalog(t *testing.T) *Store {
	t.Helper()
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.SyncFiles([]FileRow{
		{Path: pA, Size: 100, MTime: 1, Fingerprint: "s1", Kind: "video"},
		{Path: pB, Size: 90, MTime: 1, Fingerprint: "s2", Kind: "video"},
		{Path: pBS, Size: 1, MTime: 1, Kind: string(mediafile.Subtitle)},
		{Path: pC, Size: 80, MTime: 1, Fingerprint: "s3", Kind: "video"},
	}, []string{"../cine"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceVersions(partVersions("A", "B", "C")); err != nil {
		t.Fatal(err)
	}
	return s
}

func mustVersions(t *testing.T, s *Store) []VersionView {
	t.Helper()
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	return vs
}

func TestSetPartLinkMergesVersions(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s2", "s1"); err != nil {
		t.Fatal(err)
	}
	vs := mustVersions(t, s)
	if len(vs) != 2 {
		t.Fatalf("want 2 versions, got %+v", vs)
	}
	var shoah VersionView
	for _, v := range vs {
		if v.Title == "Shoah" {
			shoah = v
		}
	}
	if shoah.Parts != 2 || shoah.Size != 190 || shoah.SubLangs != "es" || !shoah.PartLinked || shoah.Fingerprint != "s1" {
		t.Fatalf("merged version %+v", shoah)
	}
	var got []string
	for _, f := range shoah.Files {
		got = append(got, f.Role+":"+f.Path)
		if f.Role == "main" && f.Part == 0 {
			t.Fatalf("main file without part: %+v", f)
		}
	}
	want := []string{"main:" + pA, "main:" + pB, "subtitle:" + pBS}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files %v, want %v", got, want)
	}
}

func TestSetPartLinkValidates(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s1", "s1"); !errors.Is(err, ErrPartLink) {
		t.Errorf("self: %v", err)
	}
	if err := s.SetPartLink("nope", "s1"); !errors.Is(err, ErrUnknownFingerprint) {
		t.Errorf("unknown follower: %v", err)
	}
	if err := s.SetPartLink("s2", "nope"); !errors.Is(err, ErrUnknownFingerprint) {
		t.Errorf("unknown leader: %v", err)
	}
}

func TestPartLinkChainOrdersPartsByLinkOrder(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s3", "s2"); err != nil { // C joins B
		t.Fatal(err)
	}
	if err := s.SetPartLink("s2", "s1"); err != nil { // B (with C) joins A
		t.Fatal(err)
	}
	vs := mustVersions(t, s)
	if len(vs) != 1 || vs[0].Parts != 3 || vs[0].Size != 270 || vs[0].Fingerprint != "s1" {
		t.Fatalf("got %+v", vs)
	}
	var paths []string
	for i, f := range vs[0].Files {
		if f.Role == "main" {
			paths = append(paths, f.Path)
			if f.Part != len(paths) {
				t.Fatalf("file %d has part %d", i, f.Part)
			}
		}
	}
	if !reflect.DeepEqual(paths, []string{pA, pB, pC}) {
		t.Fatalf("order %v", paths)
	}
}

func TestPartLinksSurviveRebuildAndWaitForMissingLeader(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s2", "s1"); err != nil {
		t.Fatal(err)
	}
	// A rescan rebuilds the versions apart...
	if err := s.ReplaceVersions(partVersions("B", "C")); err != nil { // ...and A is not there
		t.Fatal(err)
	}
	if n, err := s.ApplyPartLinks(); n != 0 || err != nil {
		t.Fatalf("leader absent: merged %d, err %v", n, err)
	}
	if len(mustVersions(t, s)) != 2 {
		t.Fatal("versions changed without the leader")
	}
	// ...and the link applies again when the leader is back.
	if err := s.ReplaceVersions(partVersions("A", "B", "C")); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ApplyPartLinks(); n != 1 || err != nil {
		t.Fatalf("merged %d, err %v", n, err)
	}
	if n, err := s.ApplyPartLinks(); n != 0 || err != nil { // idempotent
		t.Fatalf("second apply: merged %d, err %v", n, err)
	}
	if len(mustVersions(t, s)) != 2 {
		t.Fatal("want 2 versions after re-apply")
	}
}

func TestPartLinkCycleIsIgnored(t *testing.T) {
	s := partCatalog(t)
	for _, q := range [][2]string{{"s1", "s2"}, {"s2", "s1"}} {
		if _, err := s.db.Exec(`INSERT INTO part_links (fingerprint, leader, created_at) VALUES (?, ?, 0)`, q[0], q[1]); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := s.ApplyPartLinks(); n != 0 || err != nil {
		t.Fatalf("cycle: merged %d, err %v", n, err)
	}
	if len(mustVersions(t, s)) != 3 {
		t.Fatal("a cycle merged versions")
	}
}

func TestUnlink(t *testing.T) {
	s := partCatalog(t)
	if err := s.SetPartLink("s3", "s2"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPartLink("s2", "s1"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Unlink("s1"); n != 2 || err != nil { // both, the chained one included
		t.Fatalf("unlinked %d, err %v", n, err)
	}
	if n, err := s.Unlink("s1"); n != 0 || err != nil {
		t.Fatalf("second unlink: %d, %v", n, err)
	}
	// The next scan rebuilds three separate versions and nothing merges them.
	if err := s.ReplaceVersions(partVersions("A", "B", "C")); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ApplyPartLinks(); n != 0 || err != nil {
		t.Fatalf("apply after unlink: %d, %v", n, err)
	}
	vs := mustVersions(t, s)
	if len(vs) != 3 {
		t.Fatalf("want 3 versions, got %d", len(vs))
	}
	for _, v := range vs {
		if v.PartLinked {
			t.Fatalf("%q still marked linked", v.Title)
		}
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/store/ -run 'PartLink|Unlink' -v`
Expected: no compila (`s.SetPartLink undefined`, `PartLinked`, `ErrPartLink`, `part_links`).

- [ ] **Step 3: Esquema, flag y campo**

Agregar al final de `internal/store/schema.sql`:

```sql

-- A version the user says is a further part of another one: what the scan
-- rebuilds apart is merged again by ApplyPartLinks. Fingerprints are the
-- representative files' of each version.
CREATE TABLE IF NOT EXISTS part_links (
  fingerprint TEXT    PRIMARY KEY,
  leader      TEXT    NOT NULL,
  created_at  INTEGER NOT NULL          -- unix milliseconds
);
```

En `internal/store/store.go`:

- En el struct `Store`, cambiar `hasIdentity bool` por `hasIdentity bool` + nueva línea `hasPartLinks bool` (mismo comentario: es false en un catálogo viejo abierto en modo consulta).
- En `VersionView`, después de `Movie *MovieRef ...`, agregar:

```go
	// PartLinked: the user merged other versions into this one as its parts.
	PartLinked bool `json:"partLinked"`
```

- En `openDB`, cambiar el mapa a `map[string]*bool{"media": &s.hasMedia, "identifications": &s.hasIdentity, "part_links": &s.hasPartLinks}`.
- En `versions()`, después de `s.attachIdentity(tx, out, pos)`, agregar:

```go
	if err := s.attachPartLinks(tx, out); err != nil {
		return nil, err
	}
```

- [ ] **Step 4: Implementar `part_links.go`**

Crear `internal/store/part_links.go`:

```go
package store

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// ErrPartLink is returned for a link that makes no sense.
var ErrPartLink = errors.New("una película no puede ser parte de sí misma")

// SetPartLink records that the version represented by follower is a further
// part of the one represented by leader, and merges them right away. Both
// must be versions now.
func (s *Store) SetPartLink(follower, leader string) error {
	if follower == leader {
		return ErrPartLink
	}
	for _, fp := range []string{follower, leader} {
		ok, err := s.IsRepresentative(fp)
		if err != nil {
			return err
		}
		if !ok {
			return ErrUnknownFingerprint
		}
	}
	links, err := s.partLinks(s.db)
	if err != nil {
		return err
	}
	if end, ok := finalLeader(links, leader); !ok || end == follower {
		return ErrPartLink // it would close a cycle
	}
	if _, err := s.db.Exec(`INSERT INTO part_links (fingerprint, leader, created_at) VALUES (?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET leader = excluded.leader, created_at = excluded.created_at`,
		follower, leader, time.Now().UnixMilli()); err != nil {
		return err
	}
	_, err = s.ApplyPartLinks()
	return err
}

// Unlink removes the links that lead to leader, directly or through other
// linked versions, and returns how many. The versions come apart at the next
// scan.
func (s *Store) Unlink(leader string) (int, error) {
	if !s.hasPartLinks {
		return 0, nil
	}
	res, err := s.db.Exec(`WITH RECURSIVE t(f) AS (
			SELECT fingerprint FROM part_links WHERE leader = ?
			UNION
			SELECT p.fingerprint FROM part_links p JOIN t ON p.leader = t.f)
		DELETE FROM part_links WHERE fingerprint IN (SELECT f FROM t)`, leader)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// partLinks loads follower → leader.
func (s *Store) partLinks(tx querier) (map[string]string, error) {
	out := map[string]string{}
	if !s.hasPartLinks {
		return out, nil
	}
	rows, err := tx.Query(`SELECT fingerprint, leader FROM part_links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f, l string
		if err := rows.Scan(&f, &l); err != nil {
			return nil, err
		}
		out[f] = l
	}
	return out, rows.Err()
}

// finalLeader follows fp through links to the version that is nobody's
// follower. ok is false when the chain loops.
func finalLeader(links map[string]string, fp string) (string, bool) {
	for range len(links) + 1 {
		next, linked := links[fp]
		if !linked {
			return fp, true
		}
		fp = next
	}
	return "", false
}

// attachPartLinks marks the versions that have parts merged in by hand.
func (s *Store) attachPartLinks(tx querier, out []VersionView) error {
	links, err := s.partLinks(tx)
	if err != nil || len(links) == 0 {
		return err
	}
	leaders := map[string]bool{}
	for f := range links {
		if end, ok := finalLeader(links, f); ok {
			leaders[end] = true
		}
	}
	for i := range out {
		out[i].PartLinked = out[i].Fingerprint != "" && leaders[out[i].Fingerprint]
	}
	return nil
}

// ApplyPartLinks merges into their leader the versions that the user linked
// as parts, on what is stored now, and returns how many versions it merged.
// A link whose two versions are not both present (the leader's disk is
// unplugged, or it is already applied) is left alone and kept. It is safe to
// run any number of times.
func (s *Store) ApplyPartLinks() (int, error) {
	if !s.hasPartLinks {
		return 0, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	links, err := s.partLinks(tx)
	if err != nil || len(links) == 0 {
		return 0, err
	}
	reps, err := s.representatives(tx)
	if err != nil {
		return 0, err
	}
	byFP := map[string]int64{} // the lowest version id wins among identical copies
	for id, r := range reps {
		if cur, ok := byFP[r.fingerprint]; !ok || id < cur {
			byFP[r.fingerprint] = id
		}
	}
	followers := map[string][]string{} // final leader → followers
	for f := range links {
		if end, ok := finalLeader(links, f); ok {
			followers[end] = append(followers[end], f)
		}
	}
	leaders := make([]string, 0, len(followers))
	for l := range followers {
		leaders = append(leaders, l)
	}
	sort.Strings(leaders)

	merged := 0
	for _, l := range leaders {
		lid, ok := byFP[l]
		if !ok {
			continue
		}
		fs := followers[l]
		sort.Slice(fs, func(i, j int) bool { return reps[byFP[fs[i]]].path < reps[byFP[fs[j]]].path })
		changed := false
		for _, f := range fs {
			fid, ok := byFP[f]
			if !ok || fid == lid {
				continue
			}
			if !changed {
				// A single-file leader has part 0: it becomes part 1.
				if _, err := tx.Exec(`UPDATE files SET part = 1 WHERE version_id = ? AND role = 'main' AND part = 0`, lid); err != nil {
					return 0, err
				}
			}
			if err := mergeVersion(tx, lid, fid); err != nil {
				return 0, err
			}
			changed = true
			merged++
		}
		if changed {
			if err := refreshVersion(tx, lid); err != nil {
				return 0, err
			}
		}
	}
	return merged, tx.Commit()
}

// mergeVersion moves the files of version from into version to: main files
// as the next parts (keeping their order), subtitles and extras as they are.
func mergeVersion(tx execQuerier, to, from int64) error {
	var last int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(part), 0) FROM files WHERE version_id = ? AND role = 'main'`, to).Scan(&last); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT id FROM files WHERE version_id = ? AND role = 'main' ORDER BY part, path`, from)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		last++
		if _, err := tx.Exec(`UPDATE files SET version_id = ?, part = ? WHERE id = ?`, to, last, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE files SET version_id = ? WHERE version_id = ?`, to, from); err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM versions WHERE id = ?`, from)
	return err
}

// refreshVersion recomputes what a version derives from its files.
func refreshVersion(tx execQuerier, id int64) error {
	rows, err := tx.Query(`SELECT DISTINCT lang FROM files WHERE version_id = ? AND role = 'subtitle'`, id)
	if err != nil {
		return err
	}
	var langs []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			rows.Close()
			return err
		}
		if l == "" {
			l = "?"
		}
		langs = append(langs, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	sort.Strings(langs)
	_, err = tx.Exec(`UPDATE versions SET
		size = (SELECT COALESCE(SUM(size), 0) FROM files WHERE version_id = ? AND role = 'main'),
		parts = (SELECT COUNT(*) FROM files WHERE version_id = ? AND role = 'main'),
		sub_langs = ? WHERE id = ?`, id, id, strings.Join(langs, ","), id)
	return err
}
```

`mergeVersion` y `refreshVersion` reciben la transacción; definir en el mismo archivo (o junto a `querier` en `identity.go`) el tipo que necesitan:

```go
type execQuerier interface {
	querier
	Exec(query string, args ...any) (sql.Result, error)
}
```

y agregar `"database/sql"` a los imports de `part_links.go`. (`*sql.Tx` lo cumple.)

- [ ] **Step 5: Verificar que pasan**

Run: `go vet ./internal/store/ && go test ./internal/store/ -v`
Expected: PASS de todo el paquete, incluido `TestOlderReadOnlyCatalogHasNoIdentity` (prueba que `hasPartLinks` protege el acceso a `part_links` en catálogos viejos).

- [ ] **Step 6: Commit**

```bash
git add internal/store/schema.sql internal/store/store.go internal/store/part_links.go internal/store/part_links_test.go
git commit -m "feat(store): link versions as parts of one film"
```

---

### Task 3: Aplicar los vínculos al escanear

**Files:**
- Modify: `internal/scan/scan.go` (`run`, ~línea 211)
- Test: `internal/scan/scan_test.go`

**Interfaces:**
- Consumes: `(*Store).ApplyPartLinks() (int, error)`, `(*Store).SetPartLink(follower, leader string) error`, `(*Store).Versions() ([]store.VersionView, error)`; helpers de prueba `setup(t)`, `writeFile`.
- Produces: tras cada escaneo, las versiones vinculadas quedan fusionadas; `Status.Versions` cuenta las versiones ya fusionadas.

- [ ] **Step 1: Escribir la prueba que falla**

Agregar al final de `internal/scan/scan_test.go`:

```go
func TestScanKeepsPartLinks(t *testing.T) {
	disk, app, st := setup(t)
	writeFile(t, filepath.Join(disk, "cine", "Film A", "a.mkv"), 4096, 'a')
	writeFile(t, filepath.Join(disk, "cine", "Film B", "b.mkv"), 4096, 'b')
	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	vs, err := st.Versions()
	if err != nil || len(vs) != 2 {
		t.Fatalf("versions %+v, err %v", vs, err)
	}
	if err := st.SetPartLink(vs[1].Fingerprint, vs[0].Fingerprint); err != nil {
		t.Fatal(err)
	}
	// A new scan rebuilds the versions from the files: the link must hold.
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	vs, err = st.Versions()
	if err != nil || len(vs) != 1 || vs[0].Parts != 2 || !vs[0].PartLinked {
		t.Fatalf("after rescan: %+v, err %v", vs, err)
	}
	if got := sc.Status().Versions; got != 1 {
		t.Fatalf("status counts %d versions, want 1", got)
	}
}
```


- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/scan/ -run TestScanKeepsPartLinks -v`
Expected: FAIL (`after rescan` con 2 versiones).

- [ ] **Step 3: Implementar**

En `internal/scan/scan.go`, reemplazar:

```go
	n, err := s.Store.ReplaceVersionsIn(grouping.Build(entries, attempted), roots, s.Roots)
	if err != nil {
		return err
	}
```

por:

```go
	n, err := s.Store.ReplaceVersionsIn(grouping.Build(entries, attempted), roots, s.Roots)
	if err != nil {
		return err
	}
	// What the user linked as parts of one film is merged again: the
	// grouping above only knows about names.
	merged, err := s.Store.ApplyPartLinks()
	if err != nil {
		return err
	}
	n -= merged
```

- [ ] **Step 4: Verificar**

Run: `go vet ./... && go test ./internal/scan/ ./internal/store/ ./internal/grouping/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/scan.go internal/scan/scan_test.go
git commit -m "feat(scan): apply part links after grouping"
```

---

### Task 4: Acciones `part-of` y `unlink` en el servidor

**Files:**
- Modify: `internal/server/identify.go` (`identify`)
- Test: `internal/server/identify_test.go`

**Interfaces:**
- Consumes: `(*Store).SetPartLink`, `(*Store).Unlink`, `store.ErrPartLink`, `store.ErrUnknownFingerprint`; `(*Server).rt().Scan()`; helpers de prueba `identifyServer(t)`, `post(t, s, body) int`.
- Produces: `POST /api/identify` acepta `{"fingerprint": <seguidor>, "action": "part-of", "leader": <huella del líder>}` y `{"fingerprint": <líder actual>, "action": "unlink"}`. Códigos: 204 ok; 404 huella desconocida (seguidor o líder); 400 vínculo inválido (`ErrPartLink`); 409 modo consulta (ya lo maneja el encabezado).

- [ ] **Step 1: Escribir las pruebas que fallan**

Agregar al final de `internal/server/identify_test.go` (agregar a los imports `"cinexplorer/internal/grouping"` y `"cinexplorer/internal/nameparse"` si faltan):

```go
// twoFilmsServer adds a second version, Shoah 2 (fingerprint f2), next to Amarcord (f1).
func twoFilmsServer(t *testing.T) *Server {
	t.Helper()
	s, _ := identifyServer(t)
	const p = "../otra/Shoah 2.mkv"
	if err := s.Store.SyncFiles([]store.FileRow{{Path: p, Size: 5, MTime: 1, Fingerprint: "f2", Kind: "video"}}, []string{"../otra"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.ReplaceVersionsIn([]grouping.Version{{
		Dir: "../otra", Parsed: nameparse.Parsed{Title: "Shoah 2"}, Size: 5, Parts: 1,
		Members: []grouping.Member{{Path: p, Role: grouping.RoleMain}},
	}}, []string{"../otra"}, []string{"../cine", "../otra"}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIdentifyPartOfAndUnlink(t *testing.T) {
	s := twoFilmsServer(t)
	if code := post(t, s, `{"fingerprint":"f2","action":"part-of","leader":"f1"}`); code != http.StatusNoContent {
		t.Fatalf("part-of: status %d", code)
	}
	rec := request(s.Handler(), "GET", "/api/versions", "", "", "127.0.0.1")
	var vs []store.VersionView
	json.Unmarshal(rec.Body.Bytes(), &vs)
	if len(vs) != 1 || vs[0].Parts != 2 || !vs[0].PartLinked {
		t.Fatalf("versions %s", rec.Body)
	}
	if code := post(t, s, `{"fingerprint":"f1","action":"unlink"}`); code != http.StatusNoContent {
		t.Fatalf("unlink: status %d", code)
	}
	if n, _ := s.Store.Unlink("f1"); n != 0 { // the first unlink already removed it
		t.Fatalf("link left: %d", n)
	}
}

func TestIdentifyPartOfErrors(t *testing.T) {
	s := twoFilmsServer(t)
	cases := []struct {
		body string
		want int
	}{
		{`{"fingerprint":"f2","action":"part-of","leader":"zz"}`, http.StatusNotFound}, // unknown leader
		{`{"fingerprint":"zz","action":"part-of","leader":"f1"}`, http.StatusNotFound}, // unknown follower
		{`{"fingerprint":"f1","action":"part-of","leader":"f1"}`, http.StatusBadRequest},
		{`{"fingerprint":"f1","action":"part-of"}`, http.StatusNotFound}, // no leader
	}
	for _, c := range cases {
		if code := post(t, s, c.body); code != c.want {
			t.Errorf("%s: status %d, want %d", c.body, code, c.want)
		}
	}
}

func TestIdentifyPartOfReadOnly(t *testing.T) {
	s := twoFilmsServer(t)
	s.ReadOnly = true
	if code := post(t, s, `{"fingerprint":"f2","action":"part-of","leader":"f1"}`); code != http.StatusConflict {
		t.Fatalf("status %d", code)
	}
}
```

- [ ] **Step 2: Verificar que fallan**

Run: `go test ./internal/server/ -run 'PartOf' -v`
Expected: FAIL (`acción desconocida` → 400 en vez de 204/404).

- [ ] **Step 3: Implementar**

En `internal/server/identify.go`, en el struct de la petición agregar el campo y actualizar el comentario de `Action`:

```go
		Action      string `json:"action"` // movie | extra | ignore | reset | part-of | unlink
		TMDBID      int    `json:"tmdbId"`
		Leader      string `json:"leader"` // part-of: fingerprint of the version this one is a part of
```

En el `switch req.Action`, antes de `default:`, agregar:

```go
	case "part-of":
		err = s.Store.SetPartLink(req.Fingerprint, req.Leader)
	case "unlink":
		var n int
		if n, err = s.Store.Unlink(req.Fingerprint); err == nil && n > 0 {
			s.rt().Scan() // the next scan rebuilds the versions apart
		}
```

Y en el `switch` final de errores, agregar un caso antes de `case err != nil:`:

```go
	case errors.Is(err, store.ErrPartLink):
		http.Error(w, err.Error(), http.StatusBadRequest)
```

- [ ] **Step 4: Verificar**

Run: `go vet ./... && go test ./...`
Expected: PASS de todo (Go completo).

- [ ] **Step 5: Commit**

```bash
git add internal/server/identify.go internal/server/identify_test.go
git commit -m "feat(server): part-of and unlink actions"
```

---

### Task 5: Interfaz, build y documentación

**Files:**
- Create: `web/src/lib/partes.js`, `web/src/lib/partes.test.js`
- Create: `web/src/components/SelectorVersion.svelte`
- Modify: `web/src/lib/api.js`, `web/src/components/TarjetaVersion.svelte`
- Regenerate: `internal/server/dist/**`
- Modify: `README.md`, `README.es.md`

**Interfaces:**
- Consumes: `GET /api/versions` (array de versiones con `fingerprint`, `title`, `year`, `dir`, `parts`, `partLinked`); `POST /api/identify` con `part-of`/`unlink` (Task 4); en `TarjetaVersion`: `v.fingerprint`, `v.partLinked`, `correctable`, `notify`, `refreshStatus`, `onchanged`.
- Produces: `matchVersions(versions, q, self, limit = 20)` en `partes.js`; `api.versions()`, `api.partOf(fingerprint, leader)`, `api.unlink(fingerprint)`; componente `SelectorVersion` con props `{ self, onpick(version), disabled }`.

- [ ] **Step 1: Escribir la prueba que falla**

Crear `web/src/lib/partes.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { matchVersions } from './partes.js'

const v = (fingerprint, title, extra = {}) => ({ fingerprint, title, year: 0, dir: '../cine/x', ...extra })

describe('matchVersions', () => {
  const list = [
    v('a', 'Shoah', { year: 1985 }),
    v('b', 'Shoah 2'),
    v('c', 'Amarcord', { dir: '../cine/Fellini' }),
    v('', 'Sin huella'),
  ]
  it('needs something typed', () => {
    expect(matchVersions(list, '  ', 'zz')).toEqual([])
  })
  it('ignores case and accents, in the title and in the folder', () => {
    expect(matchVersions(list, 'SHOÁH', 'zz').map((x) => x.fingerprint)).toEqual(['a', 'b'])
    expect(matchVersions(list, 'fellini', 'zz').map((x) => x.fingerprint)).toEqual(['c'])
  })
  it('leaves out the version itself and those without a fingerprint', () => {
    expect(matchVersions(list, 'shoah', 'a').map((x) => x.fingerprint)).toEqual(['b'])
    expect(matchVersions(list, 'sin', 'zz')).toEqual([])
  })
  it('caps the list', () => {
    const many = Array.from({ length: 30 }, (_, i) => v(`f${i}`, `Parte ${i}`))
    expect(matchVersions(many, 'parte', 'zz', 5)).toHaveLength(5)
  })
})
```

- [ ] **Step 2: Verificar que falla**

Run: `cd web && npx vitest run src/lib/partes.test.js`
Expected: FAIL (no existe `./partes.js`).

- [ ] **Step 3: Implementar el filtro, la API y el selector**

Crear `web/src/lib/partes.js`:

```js
// Picking the version that a film in several files is a part of.

function fold(s) {
  return s
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
}

// matchVersions lists the versions whose title or folder contains q
// (ignoring case and accents), never `self` nor one without a fingerprint
// (it cannot be linked to).
export function matchVersions(versions, q, self, limit = 20) {
  const needle = fold(q.trim())
  if (!needle) return []
  return versions
    .filter((v) => v.fingerprint && v.fingerprint !== self && (fold(v.title).includes(needle) || fold(v.dir).includes(needle)))
    .slice(0, limit)
}
```

En `web/src/lib/api.js`, después de la línea de `identify:`, agregar:

```js
  versions: () => get('/api/versions'),
  partOf: (fingerprint, leader) => post('/api/identify', { fingerprint, action: 'part-of', leader }),
  unlink: (fingerprint) => post('/api/identify', { fingerprint, action: 'unlink' }),
```

Crear `web/src/components/SelectorVersion.svelte`:

```svelte
<script>
  import { api } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { matchVersions } from '../lib/partes.js'

  // self: fingerprint of the version being linked (left out of the list);
  // onpick(version): the version this one is a part of.
  let { self, onpick, disabled = false } = $props()

  let q = $state('')
  let all = $state([])

  $effect(() => {
    api.versions().then(
      (list) => (all = list),
      (e) => notify(e.message),
    )
  })

  const list = $derived(matchVersions(all, q, self))
</script>

<div class="selector">
  <!-- svelte-ignore a11y_autofocus -->
  <input type="search" placeholder="¿De qué película es parte? (título o carpeta)" bind:value={q} autofocus />
  <ul>
    {#each list as v (v.fingerprint)}
      <li>
        <button onclick={() => onpick(v)} {disabled}>
          {v.title}
          <span class="meta">{v.year || 's/f'} · {v.dir}{v.parts > 1 ? ` · ${v.parts} partes` : ''}</span>
        </button>
      </li>
    {:else}
      <li class="none">{q.trim() ? 'Ninguna versión coincide.' : 'Escribí parte del título o de la carpeta.'}</li>
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

- [ ] **Step 4: Verificar la prueba**

Run: `cd web && npx vitest run src/lib/partes.test.js`
Expected: PASS.

- [ ] **Step 5: Menú de la tarjeta**

En `web/src/components/TarjetaVersion.svelte`:

1. Importar: `import SelectorVersion from './SelectorVersion.svelte'`.
2. Comentario y estado: `panel` ahora también puede ser `'part'` (`// "search", "extra" or "part"`).
3. Agregar, junto a `correct`, la función:

```js
  async function partOf(leader) {
    if (busy) return
    busy = true
    try {
      await api.partOf(v.fingerprint, leader.fingerprint)
      refreshStatus()
      done({ action: 'part-of' })
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }
```

y `let busy = $state(false)` junto a `menu`/`panel`.

4. En el menú ⋯, después del botón "Es un extra de…", agregar:

```svelte
          <button onclick={() => ((panel = 'part'), (menu = false))}>Es una parte de…</button>
          {#if v.partLinked}
            <button onclick={() => correct('unlink')}>Separar las partes</button>
          {/if}
```

5. En el bloque `{#if panel}`, reemplazar el contenido de `<div class="panel">` por:

```svelte
      {#if panel === 'part'}
        <SelectorVersion self={v.fingerprint} onpick={partOf} disabled={busy} />
      {:else}
        {#key panel}
          <Identificar fingerprint={v.fingerprint} startWith={panel} ondone={done} />
        {/key}
      {/if}
      <button class="close" onclick={() => (panel = '')}>Cancelar</button>
```

6. `correct(action)` llama a `api.identify(v.fingerprint, action)`, que ya manda `{fingerprint, action, tmdbId: 0}`: sirve para `unlink` sin cambios.

- [ ] **Step 6: Build y pruebas web**

Run: `cd web && npm ci && npm test && npm run build`
Expected: vitest PASS; build sin errores ni advertencias nuevas de Svelte (accesibilidad). Verificar en el navegador integrado con la app real: `go run ./cmd/cinexplorer` (ver README para el arranque), abrir una película con una versión, ⋯ → "Es una parte de…", buscar otra versión y elegirla; comprobar que la tarjeta pasa a "2 partes" y que aparece "Separar las partes".

- [ ] **Step 7: Documentación**

En `README.md`, en la lista *Features*, dentro de la viñeta **Movie page**, agregar al final: «A film stored in several files (`Part 1`, `CD2`…) is one version; when the names differ after the marker they are grouped too, and ⋯ → *Is a part of…* merges any two versions by hand.». Agregar la traducción equivalente en `README.es.md` (misma viñeta, en español: «⋯ → *Es una parte de…*»).

- [ ] **Step 8: Commit**

```bash
git add web/src internal/server/dist README.md README.es.md
git commit -m "feat(web): mark a version as a part of another"
```

Verificar que `git status --short internal/server/dist` queda vacío después del build (lo exige el CI).

---

## Autorrevisión

**Cobertura del spec:** §3 agrupado automático → Task 1 (salvaguardas 1–5 y las dos nuevas, 6 y 7, agregadas al spec en el mismo task). §4.1 datos → Task 2 (esquema). §4.2 aplicación → Task 2 (`ApplyPartLinks`: líder ausente, cadenas, ciclos, idempotencia) y Task 3 (se llama tras cada escaneo). §4.3 API → Task 4 (`part-of`, `unlink`, modo consulta; `unlink` lanza reescaneo con `rt.Scan()`). §4.4 interfaz → Task 5. §5 pruebas → una por comportamiento en cada task. §6 fuera de alcance: sin tareas.

**Tipos y nombres:** `SetPartLink(follower, leader string) error`, `ApplyPartLinks() (int, error)`, `Unlink(leader string) (int, error)`, `ErrPartLink`, `VersionView.PartLinked` / JSON `partLinked`, acciones `part-of` / `unlink` y campo `leader` son iguales en las cinco tareas.

**Riesgos que el ejecutor debe vigilar:** (1) tests existentes que llamen a `isExtra` con la firma anterior; (2) que `nameparse.Parse("Shoah").Title` sea `"Shoah"`.
