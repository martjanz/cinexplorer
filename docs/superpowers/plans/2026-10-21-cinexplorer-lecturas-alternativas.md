# Identificación: lecturas alternativas del nombre — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cuando la búsqueda actual en TMDB no es concluyente, probar otras lecturas del nombre ("Título - Director", "Director - Título", coma, palabras finales, título/director cruzados), juntar los candidatos y ordenar los dudosos por duración, para que la cola de revisión llegue con la película correcta arriba.

**Architecture:** Todo en `internal/identify`. Un archivo nuevo `readings.go` genera las lecturas (`[]Query`) de una query, sin red. `match.go` pasa a puntuar con un tipo interno `scored` que recuerda si el mejor puntaje vino de una lectura alternativa y si TMDB confirmó su director; `identify()` recorre las lecturas hasta que la lista juntada asignaría automáticamente, y al final reordena por duración. `store.IdentifyTarget` trae la duración de la versión y el runner la pasa en `Query.Runtime`. `Search` (búsqueda manual del servidor) no cambia.

**Tech Stack:** Go (stdlib, `modernc.org/sqlite`).

**Spec:** `docs/superpowers/specs/2026-10-21-cinexplorer-lecturas-alternativas-design.md`

## Global Constraints

- Umbrales sin cambio: `AutoThreshold = 0.80`, `AutoMargin = 0.10`, `keptCandidates = 5`.
- Un candidato cuyo mejor puntaje viene de una lectura alternativa se asigna automáticamente únicamente si TMDB confirma el director de esa lectura.
- La duración no suma puntaje ni habilita asignación automática: solo reordena lo que va a revisión. Coincide si `|runtime TMDB − duración| ≤ max(5 min, 8 % de la duración)`; runtime TMDB 0 no coincide.
- `MatcherVersion` pasa de 4 a 5.
- `Search` (exportada, usada por `internal/server/identify.go` para la búsqueda manual) mantiene exactamente su comportamiento.
- Los casos existentes de `TestCorpus` deben seguir dando lo mismo.
- `nameparse` no se toca.
- Comentarios de código en inglés, como el resto del paquete; mensajes de log en español.
- Commits terminan con `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. `git add` con rutas explícitas.
- Go: el `go` del PATH (Node no hace falta en este plan).

## Review Focus

1. Un nombre con guion que ya se identificaba con la búsqueda principal (`Ju-On - The Grudge`, `Les Luthiers - …`) no debe cambiar de resultado: las lecturas solo corren si lo actual no es concluyente → test `TestReadingsNotTriedWhenConclusive` (Task 2).
2. La búsqueda manual del servidor con "Azor - Andreas Fontana" no debe disparar lecturas → test `TestSearchDoesNotTryReadings` (Task 2).
3. Un error no fatal de TMDB en una lectura (p. ej. 500 en una búsqueda) no debe perder lo ya encontrado: se salta esa lectura → test `TestReadingErrorSkipsReading` (Task 2).
4. Guion largo y coma pegada (`Devoto, la invasión silenciosa – Martín Basterretche`, `L'ivresse du pouvoir,C Chabrol`) → casos en `TestReadings` (Task 1).
5. Versión en varias partes (CD1 + CD2) o DVD: la duración es la suma de las partes, o el título más largo del DVD, no la del archivo representativo → test `TestIdentifyTargetsDuration` (Task 3).

## Estructura de archivos

| Archivo | Responsabilidad |
|---|---|
| `internal/identify/readings.go` (crear) | `readings(q Query) []Query`: lecturas alternativas, sin red |
| `internal/identify/readings_test.go` (crear) | `TestReadings` |
| `internal/identify/match.go` (modificar) | Tipo `scored`, puntaje de lecturas, recorrido, compuerta de director, reorden por duración, `MatcherVersion = 5`, `Query.Runtime` |
| `internal/identify/match_test.go` (modificar) | Tests de recorrido, compuerta y duración |
| `internal/store/identity.go`, `internal/store/technical.go` (modificar) | `IdentifyTarget.DurationMs` |
| `internal/store/identity_test.go` (modificar) | `TestIdentifyTargetsDuration` |
| `internal/identify/runner.go` (modificar) | Pasa la duración a `Query.Runtime` (y a la de carpeta) |
| `internal/identify/corpus_test.go`, `testdata/tmdb_corpus.json` (modificar) | Casos reales nuevos, regrabados |

---

### Task 1: Generación de lecturas

**Files:**
- Create: `internal/identify/readings.go`
- Test: `internal/identify/readings_test.go`

**Interfaces:**
- Produces: `func readings(q Query) []Query` — cada lectura copia `Year` y `Runtime` de `q`, trae `Title` y `Director` propios, `IMDbID` vacío y `Folder` nil. Orden: guion (a: izq título / der director; b: izq director / der título), última coma, quitar 1 y luego 2 palabras finales, título/director cruzados.

Reglas (de la spec §3.1, más las decisiones de este plan):
- **Guion:** el primer ` - ` o ` – `. Si no hay ninguno, el primer `-` sin espacios entre dos letras cuando la izquierda tiene 1–3 palabras y la derecha 2 o más. Un lado formado solo por dígitos (tras recortar) no se usa como título ni como director: esa lectura no se genera.
- **Coma:** la última `,` del título (con o sin espacio después) cuando la derecha tiene 1–4 palabras.
- **Palabras finales:** solo si `q.Director == ""` y el título no tiene ningún punto de corte de guion (de los de arriba, aunque se haya descartado por dígitos) ni coma; y si tiene 3 palabras o más, o exactamente 2 y la segunda empieza con mayúscula. Quitar 1, luego 2, dejando al menos 1 palabra; lo quitado es el director. (Decisión de este plan: evita lecturas ruidosas como `Azor - Andreas` / `Fontana` cuando el guion ya dio las buenas.)
- **Cruzados:** solo si `q.Director != ""`: `Title = q.Director`, `Director = q.Title`.
- Se descartan lecturas con título vacío, con título igual (sin distinguir mayúsculas ni acentos, `quality.NormTitle`) al de `q`, y duplicadas (mismo título y director normalizados).
- Títulos y directores se recortan con espacios, `-`, `–`, `,` y `.` en los extremos.

- [ ] **Step 1: Write the failing test** `TestReadings` en `readings_test.go`, tabla `Query` → `[]Query` esperadas (comparar con `reflect.DeepEqual`; `Year` 0 salvo indicado):

```go
cases := []struct {
	in   Query
	want []Query
}{
	{Query{Title: "Azor - Andreas Fontana", Year: 2021, Runtime: 100}, []Query{
		{Title: "Azor", Director: "Andreas Fontana", Year: 2021, Runtime: 100},
		{Title: "Andreas Fontana", Director: "Azor", Year: 2021, Runtime: 100},
	}},
	// En dash; the comma's right side has 6 words, so no comma reading.
	{Query{Title: "Devoto, la invasión silenciosa – Martín Basterretche"}, []Query{
		{Title: "Devoto, la invasión silenciosa", Director: "Martín Basterretche"},
		{Title: "Martín Basterretche", Director: "Devoto, la invasión silenciosa"},
	}},
	{Query{Title: "C.Chabrol-Madame Bovary", Year: 1991}, []Query{
		{Title: "C.Chabrol", Director: "Madame Bovary", Year: 1991},
		{Title: "Madame Bovary", Director: "C.Chabrol", Year: 1991},
	}},
	{Query{Title: "Melancholia, Lars Von Trier", Year: 2011}, []Query{
		{Title: "Melancholia", Director: "Lars Von Trier", Year: 2011},
	}},
	{Query{Title: "L'ivresse du pouvoir,C Chabrol", Year: 2005}, []Query{
		{Title: "L'ivresse du pouvoir", Director: "C Chabrol", Year: 2005},
	}},
	{Query{Title: "300 - Rise of an Empire", Year: 2014}, nil}, // digit side: no dash reading, and a dash blocks trailing words
	{Query{Title: "Mediterraneo G Salvatore", Year: 1991}, []Query{
		{Title: "Mediterraneo G", Director: "Salvatore", Year: 1991},
		{Title: "Mediterraneo", Director: "G Salvatore", Year: 1991},
	}},
	{Query{Title: "Loulou Pialat", Year: 1980}, []Query{{Title: "Loulou", Director: "Pialat", Year: 1980}}},
	{Query{Title: "Sebastián Díaz", Director: "muerte y profanación 4 lonkos： Vida", Year: 2020}, []Query{
		{Title: "muerte y profanación 4 lonkos： Vida", Director: "Sebastián Díaz", Year: 2020},
	}},
	{Query{Title: "Spider-Man"}, nil},
	{Query{Title: "X-Men"}, nil},
	{Query{Title: "Jean-Luc"}, nil},
	{Query{Title: "Amarcord"}, nil},
	{Query{Title: "La notte"}, nil}, // 2 words, second lowercase
}
```

Compare `nil` and empty slices as equal (`len(got) == 0 && len(want) == 0`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identify -run TestReadings -v`
Expected: FAIL, `undefined: readings`.

- [ ] **Step 3: Implement `readings(q Query) []Query` in `internal/identify/readings.go`**

Add the field `Runtime int // minutes, 0 when unknown` to `Query` in `match.go` (not part of `Key()`). Use `quality.NormTitle` for the equality/dedup checks and `strings.Fields` for word counts; detect the unspaced dash with a rune scan (letter on both sides) rather than a regex.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identify -run TestReadings -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/identify/readings.go internal/identify/readings_test.go internal/identify/match.go
git commit -m "feat(identify): alternative readings of a parsed name"
```

---

### Task 2: Recorrido de lecturas y compuerta de director

**Files:**
- Modify: `internal/identify/match.go`
- Test: `internal/identify/match_test.go`

**Interfaces:**
- Consumes: `readings(q Query) []Query` (Task 1).
- Produces (internas, Task 3 las usa):
  - `type scored struct { store.Candidate; alt bool /* best score came from an alternative reading */; confirmed bool /* TMDB confirmed that reading's director */ }`
  - `func searchScored(ctx context.Context, api API, lang string, q Query, alt bool, details map[int]tmdb.Details) ([]scored, error)` — lo que hoy hace `Search` (IMDb, variantes, año/sin año, director), llenando y reutilizando `details` (no pide de nuevo un id que ya está). Con `alt` true: no hace la búsqueda en inglés; el puntaje de cada candidato es `(título + año) / (titleWeight + yearWeight)` si su director no se confirma, y `título + año + directorWeight` si se confirma. Con `alt` false: puntaje exactamente como hoy.
  - `func pool(dst, more []scored) []scored` — junta sin duplicar por `TMDBID`, cada id se queda con la entrada de mayor `Score` (con sus flags); ordena con el mismo criterio estable de `sortCandidates`.
  - `func assignable(cs []scored) bool` — `conclusive` sobre los puntajes y además `!cs[0].alt || cs[0].confirmed`.
  - `Search` queda como envoltorio: `searchScored(…, false, details)` y devuelve los `store.Candidate`.

`identify()` pasa a: `searchScored` principal → si `!assignable`, para cada `r` de `readings(q)`: `searchScored(ctx, api, lang, r, true, details)`; error fatal → devolver; otro error → `log.Printf("lectura %q: %v", r.Title, err)` y seguir; `pool`; cortar cuando `assignable`. Estado auto solo si `assignable`. `Confidence` = puntaje del primero; `Candidates` = los primeros `keptCandidates`. `MatcherVersion = 5` con su línea en el comentario ("5: alternative readings of the name; runtime orders the review candidates").

- [ ] **Step 1: Write the failing tests** en `match_test.go`, con `fakeAPI` (claves de búsqueda `"título|año|lang"`, de detalles `"id|lang"`):
  - `TestReadingsFindDashTitle`: query `{Title: "Azor - Andreas Fontana", Year: 2021}`; la búsqueda principal (`Azor - Andreas Fontana|2021|es-ES`, `|0|es-ES`, `|2021|en-US`) no devuelve nada; `Azor|2021|es-ES` → `{ID: 659954, Title: "Azor", ReleaseDate: "2021-09-10"}`; detalles `659954|es-ES` con director "Andreas Fontana". Espera `StatusAuto`, `TMDBID 659954`, y que no haya llamada `search Andreas Fontana|…` (cortó tras la primera lectura).
  - `TestReadingWithoutDirectorStaysForReview`: igual pero el detalle trae director "Otro Nombre". Espera `StatusUnmatched`, `Candidates[0].TMDBID == 659954`, `Confidence >= 0.80`.
  - `TestWeakMainLiftedByReadingIsGated`: query `{Title: "Mediterraneo G Salvatore", Year: 1991}`; la principal devuelve `{ID: 38251, Title: "Mediterráneo", ReleaseDate: "1991-02-02"}` (puntaje bajo); `Mediterraneo G|1991|es-ES` vacío; `Mediterraneo|1991|es-ES` lo devuelve otra vez; sin detalles para 38251 (`ErrNotFound`, se ignora). Espera `StatusUnmatched` con 38251 primero. Con detalles de director "Gabriele Salvatores"… **ojo**: `directorMatches` compara la última palabra ("Salvatore" ≠ "Salvatores"), así que usar director "Gabriele Salvatore" en el fake para el caso positivo; espera `StatusAuto`.
  - `TestReadingsNotTriedWhenConclusive`: `Ju-On - The Grudge|2003|es-ES` devuelve un único resultado de título exacto; espera auto y ninguna llamada `search Ju-On|…` ni `search The Grudge|…`.
  - `TestSearchDoesNotTryReadings`: `Search(ctx, api, "es-ES", Query{Title: "Azor - Andreas Fontana"})` hace solo búsquedas con ese título exacto (`api.called("search Azor|") == 0`).
  - `TestReadingErrorSkipsReading`: principal vacía; `fail["Azor|2021|es-ES"] = errors.New("500")`; `Andreas Fontana|2021|es-ES` devuelve `{ID: 1, Title: "Otra", ReleaseDate: "2021-01-01"}`; espera `err == nil`, `StatusUnmatched`, `Candidates[0].TMDBID == 1` (la lectura siguiente se intentó y su resultado quedó).
  - Los tests existentes de `match_test.go` y `runner_test.go` sin cambios deben seguir pasando (el de `MatcherVersion - 1` usa la constante).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/identify -run 'Reading|SearchDoesNot' -v`
Expected: FAIL (lecturas no se prueban: `StatusUnmatched` donde se espera auto, etc.).

- [ ] **Step 3: Implement** `scored`, `searchScored`, `pool`, `assignable`, el nuevo `identify()` y `MatcherVersion = 5` en `match.go`, según el bloque Interfaces. `score()` recibe el flag `alt` (o se le pasa la query sin director) para el reescalado; la confirmación de director se aplica en el bucle `directorChecks` existente.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/identify/ ./internal/server/`
Expected: PASS, incluido `TestCorpus` con los casos existentes (si alguno cambia, es un bug de esta task: las lecturas solo corren cuando la principal no asigna).

- [ ] **Step 5: Commit**

```bash
git add internal/identify/match.go internal/identify/match_test.go
git commit -m "feat(identify): try alternative readings when the match is not conclusive"
```

---

### Task 3: Duración de la versión como desempate

**Files:**
- Modify: `internal/store/identity.go` (`IdentifyTarget`, `IdentifyTargets`), `internal/store/technical.go`
- Modify: `internal/identify/runner.go:260-269`, `internal/identify/match.go`
- Test: `internal/store/identity_test.go`, `internal/identify/match_test.go`, `internal/identify/runner_test.go`

**Interfaces:**
- Consumes: `scored`, `identify()` (Task 2); `Query.Runtime` (Task 1).
- Produces:
  - `IdentifyTarget.DurationMs int64` — duración de la versión: con `kind == "dvd"` la del título más largo, si no la suma de las partes principales presentes con probe válido (mismos filtros que `attachMedia`: `role = 'main'`, `missing = 0`, `m.error = ''`, tamaño y mtime al día). 0 si no hay datos o `!s.hasMedia`.
  - `func (s *Store) versionDurations(tx querier) (map[int64]int64, error)` en `technical.go`, reutilizando la consulta y la regla DVD de `attachMedia`/`applyMedia` (extraer lo común a una función si queda duplicado).
  - `func runtimeMatches(runtime, minutes int) bool` en `match.go`: `runtime > 0 && |runtime − minutes| ≤ max(5, minutes*8/100)`.
  - En el runner: `q.Runtime = int((t.DurationMs + 30000) / 60000)`, y la misma en `q.Folder`.

`identify()`: al terminar el recorrido, si `!assignable` y `q.Runtime > 0`, para los primeros `keptCandidates` sin detalles en `details` pedir `api.Movie(ctx, id, lang)` (fatal → devolver; otro error → ese candidato no coincide), guardar en `details`, y reordenar esos primeros `keptCandidates` de forma estable poniendo primero los que `runtimeMatches`. `Confidence` se toma **antes** del reorden (es el mejor puntaje; `Identify` lo usa para elegir entre principal y carpeta). El estado no cambia.

- [ ] **Step 1: Write the failing tests**
  - `TestIdentifyTargetsDuration` (store): tres versiones — una con un solo archivo de 100 min, una con CD1 50 min + CD2 45 min (espera 95 min en ms), una DVD con VOBs de 5 y 110 min (espera 110 min). Seguir el armado de fixtures de los tests existentes de `identity_test.go` / `technical` para crear archivos y media.
  - `TestRuntimeOrdersTies` (identify): `Phoenix|2014|es-ES` devuelve `{ID: 1525682, Title: "Phoenix", 2014-10-01}` y `{ID: 254578, Title: "Phoenix", 2014-09-25}` (empate); detalles: 1525682 runtime 15, 254578 runtime 98; query `{Title: "Phoenix", Year: 2014, Runtime: 97}`. Espera `StatusUnmatched`, `Candidates[0].TMDBID == 254578`. Con `Runtime: 0` el orden es el de TMDB y no hay llamadas `movie `.
  - `TestRuntimeMatches`: `(98, 97)` sí, `(103, 97)` sí (≤ 5), `(150, 140)` sí (8 % de 140 = 11), `(15, 97)` no, `(0, 97)` no.
  - `TestRunnerPassesRuntime` (`runner_test.go`, con el armado de store de los tests existentes del runner): una versión "Phoenix (2014)" con media de 97 min y el fake de `TestRuntimeOrdersTies`; tras `Run`, la identificación guardada tiene `Candidates[0].TMDBID == 254578`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store -run TestIdentifyTargetsDuration -v; go test ./internal/identify -run 'Runtime' -v`
Expected: FAIL (`DurationMs` / `runtimeMatches` no definidos).

- [ ] **Step 3: Implement** según Interfaces.

- [ ] **Step 4: Run all tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/identity.go internal/store/technical.go internal/store/identity_test.go internal/identify/match.go internal/identify/match_test.go internal/identify/runner.go internal/identify/runner_test.go
git commit -m "feat(identify): order review candidates by the version's runtime"
```

---

### Task 4: Corpus real y medición

**Files:**
- Modify: `internal/identify/corpus_test.go`, `internal/identify/testdata/tmdb_corpus.json`

- [ ] **Step 1: Add cases** a `corpus` (nombres tal como en la colección; esperados verificados contra TMDB el 2026-10-21):

```go
{"Azor - Andreas Fontana (2021)", 659954},
{"El campo - Hernán Belón (2011)", 154552},
{"Jean-Luc Godard - A Bout de souffle", 269}, // may end in review: an unconfirmed "Jean-Luc Godard" documentary from reading a can block the margin; then want 0, with 269 first
{"Mediterraneo G Salvatore (1991)", 0}, // TMDB: Gabriele Salvatores — "Salvatore" ≠ "Salvatores", so review
{"Melancholia, Lars Von Trier (2011)", 62215},
{"Loulou Pialat (1980)", 39415},
```

- [ ] **Step 2: Record**

Con el token en `CINEXPLORER_TMDB_TOKEN` (tomarlo de `D:\cinexplorer-e2e-tmp\config.json`, campo `tmdbToken`, sin imprimirlo):
Run: `$env:CINEXPLORER_TMDB_RECORD=1; go test ./internal/identify -run TestCorpus -v`
Expected: PASS y el log `automáticas: N/37`. Si un caso nuevo da otro resultado, mirar su línea de log (candidatos y puntajes): si la película correcta aparece primera pero queda en revisión por la compuerta, cambiar el esperado a 0 con un comentario que diga por qué; si no aparece, es un bug de las Tasks 1–2. Los casos viejos no deben cambiar.

- [ ] **Step 3: Run without recording**

Run: `Remove-Item Env:CINEXPLORER_TMDB_RECORD; go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/identify/corpus_test.go internal/identify/testdata/tmdb_corpus.json
git commit -m "test(identify): corpus cases for alternative readings"
```

- [ ] **Step 5: Measure on a copy of the real catalog** (no se commitea nada)

Copiar `D:\cinexplorer-e2e-tmp\cinexplorer.db` y `config.json` a una carpeta temporal fuera del repo (las rutas `roots` son relativas: la copia debe quedar al lado de `D:\cinexplorer-e2e-tmp`, p. ej. `D:\cinexplorer-e2e-measure\`, para que `../cine` resuelva igual), compilar `./cmd/cinexplorer` y correr una identificación sobre la copia. Reportar antes/después con:

```sql
SELECT status, count(*), sum(candidates = '[]') FROM identifications GROUP BY status;
```

Antes (2026-10-21): auto 1086, manual 252, unmatched 396 (280 sin candidatos). Revisar a mano una muestra de 20 sin identificar que antes no tenían candidatos y contar en cuántas la correcta quedó primera. Borrar la copia al terminar.
