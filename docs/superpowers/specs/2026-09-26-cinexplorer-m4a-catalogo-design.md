# Cinexplorer — Etapa 4a: Catálogo navegable — Diseño

Fecha: 2026-09-26
Estado: aprobado; ajustado tras prototipar (Node, resolución, orden de facetas, forma de algunas respuestas, refresco durante corridas largas, `first_seen` de archivos movidos)
Spec general: `2026-09-25-cinexplorer-design.md` (§2.3, §3.3, §3.4, §5.2, §5.3, §5.5)
Etapa anterior: `2026-09-26-cinexplorer-m3-identificacion-design.md` (§13 pendientes)

## 1. Objetivo

Reemplazar la página mínima de las etapas 1–3 por la interfaz real: una app Svelte embebida en el binario con **Explorar** (grilla de afiches con facetas en la URL), **Ficha** de película con sus versiones en disco y **Revisar** (Sin identificar y Duplicados). Resolver de paso los pendientes de la Etapa 3 que afectan lo que la interfaz muestra.

La Etapa 4 se divide en dos sub-etapas, cada una con su spec, plan y merge a `main`:

- **4a — Catálogo navegable** (este documento).
- **4b — Descubrimiento y primer uso**: Inicio estilo MUBI (§5.1), búsqueda instantánea con FTS5 (§5.6) y asistente de primer uso (§5.7).

## 2. Decisiones

| Tema | Decisión |
|---|---|
| Stack del frontend | Svelte 5 (runes) en JavaScript, Vite 8, Vitest 5. Sin TypeScript. Requieren Node ≥ 22.12; la CI usa Node 24. |
| Ubicación y build | Proyecto en `web/`; la salida del build se **commitea** en `internal/server/dist/` y se embebe con `go:embed`. `go build`/`go test` no necesitan Node. La CI reconstruye y falla si la salida difiere. |
| Pruebas del frontend | Vitest solo sobre lógica pura en módulos `.js`. Los componentes se validan con el smoke test del binario y revisión en el navegador. |
| Rutas | History API con router propio mínimo, sin dependencias. Go sirve `index.html` para rutas que no son archivos del build ni `/api/`, `/img/`. |
| Ítems de Explorar | Películas identificadas (`auto`/`manual`) agrupadas + cada huella pendiente o `unmatched` como ítem propio. `ignored` y `extra` no aparecen. |
| Facetas | Se filtran y cuentan en Go, sobre una instantánea leída en una transacción. Un valor por faceta, AND entre facetas. |
| Facetas de TMDB | Director, Género, País, Idioma original y Colección solo filtran películas identificadas (no se mezclan datos parseados del nombre: ensucian los valores). |
| Grilla | Afiche 2:3 + título + título original (chico, si difiere) + año. |
| Versiones en la Ficha | Tarjetas apiladas anchas, una por renglón, con menú ⋯ de correcciones. |
| Cola de Sin identificar | Lista con el ítem actual expandido; atajos de teclado. |
| "Es un extra de…" | Selector sobre el catálogo local: primero las películas de la misma carpeta o la padre, más un filtro por título. |
| Duplicados | Copias idénticas y varias versiones. La "sospecha" (§3.3 tipo 3) queda para más adelante. |
| Pendientes de la Etapa 3 | Imágenes por ruta, transacción de lectura, idioma en la clave de consulta, errores por ítem como `unmatched`. |

## 3. Estructura y build

```
web/                          proyecto Svelte 5 + Vite
  package.json, package-lock.json, vite.config.js, index.html
  src/main.js, src/App.svelte
  src/lib/         lógica pura con tests: router.js, facets.js, format.js, keys.js, status.js
                   con estado: api.js, nav.svelte.js (ruta actual), app.svelte.js (estado del servidor, avisos)
  src/pages/       Explorar.svelte, Pelicula.svelte, Version.svelte, Revisar.svelte
  src/components/  Nav, Estado, BarraFacetas, Afiche, TarjetaVersion, Identificar,
                   Candidatos, SelectorExtra, SinIdentificar, Duplicados
internal/server/dist/         salida del build (commiteada, embebida)
```

- `vite.config.js`: `build.outDir = ../internal/server/dist`, `emptyOutDir: true`; en desarrollo, proxy de `/api` e `/img` a `http://127.0.0.1:8080`. Flujo de desarrollo: `go run ./cmd/cinexplorer -port 8080 -no-browser` + `npm run dev` en `web/`.
- Tipografía: `@fontsource-variable/inter`, empaquetada en el build (sin red).
- `.gitattributes`: `internal/server/dist/** -text`, para que el build sea idéntico byte a byte en Windows y Linux.
- `go:embed dist` exige que la carpeta exista y no esté vacía; como la salida está commiteada (con `index.html`), siempre se cumple.
- `scripts/build.sh`: `(cd web && npm ci && npm test && npm run build)` antes de compilar los binarios.
- CI: job `web` en `ubuntu-latest` con Node 24: `npm ci`, `npm test`, `npm run build`, y falla si `git status --porcelain internal/server/dist` no está vacío. Los jobs de Go de las tres plataformas no cambian.
- `.gitignore` ignoraba `dist/` en cualquier nivel: pasa a `/dist/` (la salida de `scripts/build.sh`), para que `internal/server/dist/` se pueda commitear. Suma `node_modules/`.
- La página mínima de las etapas 1–3 (`internal/server/web/index.html`) se mueve a `internal/server/dist/` hasta que el primer build de Svelte la reemplaza.

## 4. Servidor

- `GET /`: si la ruta corresponde a un archivo de `dist`, se sirve; si no, `index.html`. Las rutas bajo `/api/` e `/img/` que no existen siguen dando 404 (no caen en `index.html`), igual que una ruta con extensión que no es un archivo (`/assets/falta.js`): un asset faltante no debe responder con HTML.
- `Server.Static fs.FS` permite a los tests servir una app falsa; `nil` es el build embebido. `Server.Roots` recibe las raíces de `config.json` (la faceta Ubicación las necesita).
- Caché del navegador: `/assets/*` (nombres con hash) con `Cache-Control: public, max-age=31536000, immutable`; `index.html` con `no-cache`; imágenes con `?v=` (§7) con `public, max-age=31536000, immutable`, sin `v` con `no-cache`.
- `localOnly` y `jsonOnly` no cambian.

## 5. Datos

### 5.1 Instantánea

`store.Snapshot()` lee en **una transacción de lectura** (`BEGIN` diferido; en SQLite las lecturas dentro de una transacción ven un estado consistente): versiones con archivos, datos técnicos e identidad (lo que hoy arma `Versions()`), y todas las películas referenciadas. `Versions()` y `Unidentified()` pasan a leer dentro de una transacción igual (pendiente de la Etapa 2).

```go
type Snapshot struct {
    Versions        []VersionView              // como las devuelve Versions (con Best marcado)
    Identifications map[string]*Identification // por huella
    Movies          map[int]Movie              // todas las guardadas
}
```

La cola de Sin identificar también sale de la instantánea (`catalog.Unidentified`), así que `store.Unidentified` desaparece.

### 5.2 Fecha de alta

`files.first_seen INTEGER NOT NULL DEFAULT 0` (unix ms). Es la primera columna agregada a una tabla existente, así que el esquema deja de alcanzar con `CREATE TABLE IF NOT EXISTS`: `openDB` (solo en aperturas escribibles) consulta `PRAGMA table_info(files)` y, si falta, ejecuta `ALTER TABLE files ADD COLUMN first_seen …` y `UPDATE files SET first_seen = mtime`. `SyncFiles` la llena al insertar una ruta nueva y no la toca al actualizarla. Una ruta nueva hereda la fecha más antigua de su huella (un archivo movido o renombrado no es "agregado recientemente"); sin huella, o con una huella nueva, recibe la hora actual. En modo consulta sobre un catálogo viejo sin la columna, la fecha de alta es `0` (`Snapshot` detecta la columna como hoy detecta las tablas `media` e `identifications`).

La fecha de alta de una versión (`VersionView.Added`) es la menor `first_seen` de sus archivos principales presentes, y la de un ítem, la menor de sus versiones. `FileView` suma el tamaño de cada archivo (`size`), que la Ficha usa para los extras.

### 5.3 Paquete `internal/catalog`

Funciones puras sobre `Snapshot`, sin SQL:

```go
type Item struct {
    Kind          string   // "movie" | "version"
    TMDBID        int      // kind == movie
    Fingerprint   string   // kind == version
    Title         string   // localizado; o el parseado del nombre
    OriginalTitle string
    Year          int
    Directors     []string
    Countries     []string
    Genres        []string
    OriginalLang  string
    CollectionID  int
    Collection    string
    Poster        string   // versión de imagen ("" sin afiche)
    Resolution    string   // la mejor de sus versiones: "4K" | "1080p" | "720p" | "SD" | ""
    Size          int64    // suma de sus versiones presentes
    Added         int64
    Versions      int
    // campos internos para filtrar (no se serializan)
}

func Items(s store.Snapshot) []Item
func Filter(items []Item, f Facets) []Item
func Counts(items []Item, f Facets) map[string][]FacetValue // cada faceta con las demás aplicadas
func Sort(items []Item, by, dir string)
func Duplicates(s store.Snapshot) DuplicateReport
```

**Armado de ítems**:
- Versión con identificación `auto`/`manual` a una película guardada → se agrupa en el ítem de esa película.
- Versión sin fila de identificación, `unmatched`, o `auto`/`manual` que apunta a un id que no está en `movies` (todavía sin enriquecer, o id inválido) → un ítem `version` por huella (las copias idénticas comparten ítem). Título, año, director, países del nombre. Versiones sin huella (sin hashear o archivo vacío) → ítem `version` con clave `id:<version id>`.
- `ignored` y `extra` → no son ítems.
- Versiones cuyos archivos principales faltan todos (`missing`) no cuentan; un ítem sin versiones presentes no aparece.

**Facetas** (nombre en la URL → valor):

| Faceta | Parámetro | Valor | Aplica a |
|---|---|---|---|
| Década | `decada` | `1970` | todos (año de TMDB o parseado) |
| Año | `anio` | `1973` | todos |
| Director | `director` | id de persona TMDB | identificadas |
| Género | `genero` | nombre localizado | identificadas |
| País | `pais` | ISO 3166-1 alfa-2 | identificadas |
| Idioma original | `idioma` | ISO 639-1 | identificadas |
| Colección | `coleccion` | id de colección TMDB | identificadas |
| Resolución | `resolucion` | `4K`, `1080p`, `720p`, `SD` | todos (alguna versión con esa resolución) |
| Subtítulos | `subs` | código de idioma | todos (alguna versión con subs internos o externos en ese idioma) |
| Ubicación | `ubicacion` | `cine` o `cine/1970s` (raíz, o raíz + carpeta de primer nivel, sin `../`) | todos (alguna versión bajo esa carpeta) |
| Estado | `estado` | `sin-identificar`, `varias-versiones`, `copia-identica` | todos |

- Resolución a partir de la etiqueta de la versión (la leída del archivo o, si no se leyó, la del nombre): `2160p` → 4K, `1080p`/`1080i` → 1080p, `720p` → 720p, cualquier otra (`576p`, `480p`, `SD`) → SD.
- Valores desconocidos o parámetros que no son facetas se ignoran; la respuesta informa las facetas aplicadas.
- `Counts`: para cada faceta, los valores presentes en los ítems que cumplen **las demás** facetas, con su conteo. Década y Año van del más reciente al más viejo; Ubicación, alfabética (se lee como un árbol); el resto, por conteo descendente y luego etiqueta. Las etiquetas: nombre del director o colección, década `1970s`, país e idioma como código (el cliente los traduce con `Intl.DisplayNames`).
- **Orden** (`orden`): `anio` (por defecto), `titulo`, `agregado`, `tamano`; `dir`: `asc` o `desc` (por defecto `desc` para año, agregado y tamaño; `asc` para título). Desempate por título y luego clave. Título comparado con `quality.NormTitle`.

**Duplicados**:
- **Copia idéntica**: huella presente en ≥ 2 rutas de archivos principales. Recuperable = (copias − 1) × tamaño de la versión.
- **Varias versiones**: película identificada con ≥ 2 versiones de huellas distintas. Recuperable = suma de los contenidos que no son el de la mejor versión (marcada por `markBest`; si no hay, la más grande), cada huella una vez; sus copias ya se cuentan como copia idéntica.
- Un grupo por película (identificadas) o por huella (sin identificar), con sus tipos, recuperable total, y las versiones. Orden: recuperable descendente. Total arriba.

## 6. API

Endpoints nuevos:

- `GET /api/explore?<facetas>&orden=&dir=` → `{total, query: {facets, order, dir}, items: [Item], facets: {<faceta>: [{value, label, count}]}}`. `query` es lo que se aplicó (sin las facetas mal formadas). Sin paginación.
- `GET /api/movies/{id}` → `{movie, poster, backdrop, versions, extras: [{path, size, missing}], extraVersions}`. `poster`/`backdrop`: versiones de imagen; `versions` y `extraVersions` son `VersionView` más `copies` (versiones presentes con esa huella, ella incluida), la mejor primero y las que faltan al final; `extras`: archivos de rol `extra` de sus versiones; `extraVersions`: versiones corregidas como "extra de" esta película. 404 si no existe.
- `GET /api/versions/{key}` (`key` = huella o `id:<n>`) → `{key, title, year, versions, identification: {status, tmdbId, confidence, candidates} | null, movie: MovieRef | null}`. 404 si no existe.
- `GET /api/movies?q=&near=` → hasta 20 `{tmdbId, title, originalTitle, year, directors}`. Con `near` (huella), primero las películas identificadas cuyas versiones están en la misma carpeta que alguna versión de esa huella o en su carpeta padre; `q` filtra por título o título original normalizados (`quality.NormTitle`, contiene).
- `GET /api/duplicates` (reemplaza al de la Etapa 1) → `{recoverable, groups: [{types, tmdbId, fingerprint, title, year, recoverable, versions}]}`.

Cambios:

- `GET /api/unidentified` suma `pending` (huellas de versiones presentes sin fila de identificación) y por ítem los tokens parseados (ya están en `VersionView`). Pasa de lista a objeto: `{pending, items}`.
- `/api/versions`, `/api/identify`, `/api/tmdb/search`, `/api/open`, `/api/reveal`, `/api/scan`, `/api/status` no cambian.

## 7. Imágenes (pendiente 1 de la Etapa 3)

- El archivo en caché se nombra con la ruta de TMDB: `cache/posters/{id}-{nombre}.jpg` (`/kqjL17yufvn9OVLyXYpvtyrFfak.jpg` → `12345-kqjL17yufvn9OVLyXYpvtyrFfak.jpg`). Al guardar uno, se borran los demás de esa película y tipo, incluido el `{id}.jpg` de la Etapa 3. `Has` recibe la ruta.
- La versión de imagen que la API manda a la página es el nombre sin extensión; la página pide `/img/poster/12345.jpg?v=kqjL17…`. El servidor sigue usando la ruta guardada en `movies` (el `v` solo sirve para la caché del navegador; uno que no coincide se sirve igual, sin caché larga).
- El prefetch del Runner usa `Has(kind, id, path)`.

## 8. Identificación (pendientes 3 y 4 de la Etapa 3)

- La clave de consulta (`query`) suma el idioma configurado: al cambiar `language`, las `auto`/`unmatched` se recalculan con candidatos en el idioma nuevo.
- Al revisar directores, un error que no es de red ni de token en `Movie()` (una película que TMDB ya no tiene) solo le quita a ese candidato la parte del director; la búsqueda sigue.
- Cualquier otro error que no es de red ni de token al identificar un ítem (4xx de la búsqueda, JSON inesperado) lo guarda como `unmatched` sin candidatos, en lugar de dejarlo sin fila y reintentarlo en cada corrida. Aparece en Revisar, donde se puede buscar a mano.
- `MatcherVersion` pasa a 2. El corpus de calibración no se regraba.

## 9. Frontend

### 9.1 Rutas y navegación

| Ruta | Pantalla |
|---|---|
| `/` | redirige a `/explorar` |
| `/explorar?…` | Explorar |
| `/pelicula/{tmdbId}` | Ficha |
| `/version/{key}` | Ficha provisoria |
| `/revisar` · `/revisar/duplicados` | Revisar |

Barra superior: `CINEXPLORER` · `Explorar` · `Revisar` y, a la derecha, el indicador de estado. Tema oscuro, Inter, títulos en mayúsculas.

**Indicador de estado**: resume `/api/status` (escaneando con archivos o analizados x/y —el escaneo no conoce el total de antemano—, identificando x/y, completando datos x/y, sin red, falta token, token inválido, modo consulta, al día). Clic → panel con el detalle y "Volver a escanear" (oculto en modo consulta). Consulta `/api/status` cada 3 s mientras un escaneo o una identificación corren y cada 30 s si no. La vista actual vuelve a pedir sus datos (sin perder scroll ni facetas) al terminar una corrida y, durante una corrida larga, cada 20 s si hubo progreso. En pantallas chicas el indicador queda solo como un punto de color.

### 9.2 Explorar

- Barra de facetas: Década (con los años de la década dentro), Director, Género, País, Resolución, Subs y "Más ▾" (Idioma original, Colección, Ubicación, Estado). Cada desplegable muestra valores y conteos; con más de 12 valores, un filtro de texto. Facetas activas como chips con ✕. A la derecha, "N películas" y el orden.
- Grilla: afiche 2:3, título (una línea con elipsis), título original chico y gris si difiere, año. Tarjeta sin identificar: afiche rayado con borde punteado, título y año del nombre, "sin identificar". Afiche que no carga → el mismo afiche genérico con el título.
- Se dibuja en tandas de 120 (un centinela con `IntersectionObserver` agrega la siguiente); imágenes con `loading="lazy"`.
- Cambiar facetas u orden hace `pushState`; atrás/adelante restaura facetas y posición de scroll. Si la respuesta dice que se ignoró una faceta, la URL se corrige con `replaceState`.
- Catálogo vacío mientras se escanea: "Escaneando…" con el progreso.

### 9.3 Ficha

- Cabecera: imagen de escena de fondo (degradado si no hay), afiche superpuesto, TÍTULO en mayúsculas, "título original · año" (el original solo si difiere), "director · países · duración · géneros" (director, países y géneros son enlaces a Explorar con esa faceta), sinopsis, reparto principal (hasta 10).
- "En disco · N versiones": tarjetas apiladas, la mejor primero. Cada una: resolución grande, MEJOR (verde) y COPIA IDÉNTICA ×n (ámbar), "origen · codec · partes · tamaño · duración", audio y subtítulos (internos y externos) por idioma, ruta relativa, **▶ Ver** (primera parte), **Carpeta** y **⋯**: Cambiar película (búsqueda TMDB), Es un extra de…, No es una película, Volver a automática (solo si la versión tiene una corrección). Archivos faltantes: tarjeta atenuada, "no encontrado", sin ▶ ni Carpeta.
- "Extras": archivos extra y versiones marcadas como extra de esta película, con tamaño y ▶.
- 404 → "Esta película ya no está en el catálogo" con enlace a Explorar.

### 9.4 Ficha provisoria

Título y año del nombre con afiche genérico, las tarjetas de versión y el bloque **Identificar** (el mismo componente que Revisar). Al confirmar una película, navega a su Ficha; al marcar "no es película" o "extra de", vuelve atrás.

### 9.5 Revisar

- **Sin identificar**: pestaña "Sin identificar (N) · M en espera". Lista donde el ítem actual está expandido: nombre, ruta, tokens parseados (título, año, director, resolución, origen, codec, idioma), hasta 5 candidatos (afiche, título, año · director, %), búsqueda manual (título o `tt…`) con resultados en el mismo formato, **No es una película**, **Es un extra de…** (selector local, §6), ▶. Los demás ítems en una línea con el mejor puntaje o "sin candidatos". Un clic en un candidato confirma; tras cualquier acción el ítem sale y se abre el siguiente. Atajos: ↑/↓ cambiar de ítem, 1–5 confirmar candidato, `/` buscar, `N` no es película, `E` extra de. Sin token o sin red: aviso arriba y búsqueda deshabilitada; no es película y extra de siguen andando.
- **Duplicados**: pestaña "Duplicados (N · X GB)". Total recuperable arriba; grupos con tipo (Copia idéntica / Varias versiones), título, año, recuperable y una línea por versión (resolución · tamaño · ruta). Clic → Ficha o ficha provisoria.

### 9.6 Modo consulta

Sin acciones de identificación ni "Volver a escanear"; el indicador de estado lo muestra fijo.

### 9.7 Errores en el cliente

`api.js` convierte las respuestas no 2xx en un error con el texto del servidor; la app lo muestra en una franja arriba que se va sola a los 6 s. En acciones de identificación: 503 → aviso de token o red, el ítem queda; 404 → se refresca la lista; 409 → aviso de modo consulta.

### 9.8 Lógica pura (Vitest)

- `facets.js`: URL ↔ facetas (lectura, escritura, orden estable de parámetros, facetas por defecto omitidas), chips.
- `router.js`: resolución de rutas a pantalla y parámetros.
- `format.js`: tamaños (`9,8 GB`), duración (`2 h 3 min`), nombres de países e idiomas con `Intl.DisplayNames`, resolución, codec, pistas de audio y subtítulos, línea técnica de una versión. (La línea "DIRECTOR PAÍS AÑO" es del Inicio: 4b.)
- `keys.js`: tecla → acción de la cola.
- `status.js`: si hay algo corriendo, el resumen del indicador, el progreso y por qué no se puede buscar en TMDB.

## 10. Pruebas

- **catalog**: tablas para armado de ítems (agrupación, pendientes y `unmatched` como ítems, `ignored`/`extra` excluidos, id inválido, faltantes, sin huella), cada faceta, conteos con las demás facetas aplicadas, facetas desconocidas ignoradas, órdenes y desempates, duplicados y recuperable (copias dentro de varias versiones sin doble conteo).
- **store**: `Snapshot` (contenido; la consistencia la da la transacción, sin test de concurrencia), migración de `first_seen` (catálogo sin la columna, relleno con `mtime`, archivo nuevo con hora actual, reescaneo que la conserva, archivo movido que hereda la fecha), catálogo viejo en modo consulta.
- **images**: nombre por ruta, borrado de anteriores y del `{id}.jpg` heredado, `Has` con ruta.
- **identify**: idioma en la clave recalcula `unmatched`; error por ítem guardado como `unmatched`; `MatcherVersion` 2; el corpus sigue pasando.
- **server**: cada endpoint nuevo (casos felices, 404, parámetros inválidos), `unidentified` con `pending`, fallback a `index.html` (sí en `/explorar` y `/pelicula/1`, no en `/api/nada` ni `/img/x`), encabezados de caché de assets, `index.html` e imágenes con `v`.
- **Smoke test** (`cmd/cinexplorer`, en las tres plataformas): `run` se separa en `setup` (config, catálogo, servidor, escaneo) y la escucha HTTP. El test llama a `setup` sobre un árbol sintético temporal, sin token; espera el fin del escaneo; con `httptest.NewServer`, `/` devuelve el `index.html` embebido, `/explorar?decada=1970` devuelve la app, `/api/explore?decada=1970` lista la película y `/api/status` informa `noToken`.
- **web**: Vitest sobre `src/lib`.
- **Prueba real** (manual, fuera de CI): binario con token sobre una parte de `D:\cine`, recorriendo Explorar con facetas, una Ficha con varias versiones, la cola de Revisar con atajos y Duplicados.

## 11. Pendientes

Fuera de 4a, registrados para más adelante:

- **Duplicados por sospecha** (§3.3 tipo 3): sin identificar, mismo título normalizado + año.
- **Título principal elegible**: preferencia para mostrar el título localizado o el original como principal, independiente del idioma de los metadatos.
- **Varios valores por faceta** (Italia *o* Francia), si hace falta.
- Pendientes de la Etapa 3 no incluidos: huérfanos (identificaciones y películas sin archivos), clave de consulta inestable ante un `.nfo` ilegible, `Run` fuera del encolado de `Trigger`.
- **Partes con separadores distintos** (encontrado al probar Duplicados sobre `D:\cine\1970s`): `Rip mentecato cd 01` y `Rip.mentecato.cd.02` quedan como dos versiones, y Duplicados las muestra como "varias versiones". Es del armado de versiones de la Etapa 1; queda como tarea aparte.
- `store.Duplicates` (copias idénticas a nivel de archivo) ya no lo usa el servidor; solo lo usan los tests del escáner.
- **4b**: Inicio (§5.1), búsqueda con FTS5 (§5.6, puede reemplazar el filtro de `GET /api/movies?q=`), asistente de primer uso (§5.7).
