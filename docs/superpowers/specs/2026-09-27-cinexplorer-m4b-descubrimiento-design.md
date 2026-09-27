# Cinexplorer — Etapa 4b: Descubrimiento y primer uso — Diseño

Fecha: 2026-09-27
Estado: aprobado en brainstorming, pendiente de revisión escrita y de prototipo
Spec general: `2026-09-25-cinexplorer-design.md` (§2.2, §5.1, §5.6, §5.7)
Etapa anterior: `2026-09-26-cinexplorer-m4a-catalogo-design.md` (§11 pendientes)

## 1. Objetivo

Completar la interfaz con lo que falta para descubrir la colección y para arrancar sin tocar archivos:

- **Inicio** estilo MUBI (§5.1): filas de imágenes de escena que rotan por visita, cada una con "ver todas →" hacia Explorar.
- **Búsqueda instantánea** (§5.6) con FTS5, insensible a tildes y mayúsculas, sobre las películas identificadas y lo que todavía no está identificado.
- **Asistente de primer uso** (§5.7): raíces, token de TMDB e idioma, antes del primer escaneo.
- **Ajustes**: las mismas opciones, cambiables después sin reiniciar la app.
- **Idiomas**: es-AR, en-US, pt-BR, con una cadena de respaldo para textos sin traducción regional.

## 2. Decisiones

| Tema | Decisión |
|---|---|
| Cambio de configuración | En caliente: la parte del programa que depende de `config.json` (TMDB, escáner, identificador, caché de imágenes, raíces, idioma) vive en un `Runtime` que se reemplaza entero. Sin setters por componente ni reinicio del proceso. |
| Primer uso | Sin `config.json` y con la carpeta escribible: no se guarda nada ni se escanea hasta completar el asistente. |
| Ajustes | Página `/ajustes` con raíces, token, idioma y descarga de imágenes. Modo consulta: solo lectura. |
| Raíces fuera de las hermanas | Se pueden agregar escribiendo la ruta (absoluta o relativa); el servidor la valida y la guarda relativa. Sin explorador de carpetas. |
| Raíz quitada | Sus archivos pasan a "no encontrado" (se conservan correcciones; reaparecen si se vuelve a agregar). Una raíz configurada pero desconectada sigue sin tocarse. |
| Idiomas | `es-AR` (por defecto), `en-US`, `pt-BR`. Un valor distinto ya guardado se conserva y se muestra como opción extra. |
| Textos sin traducción | Cadena por idioma: `es-AR → es-MX → es-ES → otra variante es → en-US`; `pt-BR → pt-PT → otra pt → en-US`; `en-US → en-GB → otra en`. Con las traducciones de TMDB en un pedido aparte. |
| Índice de búsqueda | FTS5 en una base SQLite **en memoria**, derivada de los ítems de Explorar y reconstruida cuando el catálogo cambió. No cambia el esquema de `cinexplorer.db` y funciona igual en modo consulta. |
| Alcance de la búsqueda | Películas identificadas (título, título original, directores, reparto, títulos parseados de sus archivos) e ítems sin identificar (título parseado). |
| Resultados | Desplegable bajo ⌕ (películas + directores) y página `/buscar?q=` con la grilla completa. |
| Inicio: ítems | Solo películas identificadas y presentes. Sin imagen de escena: el afiche recortado a 16:9 y desenfocado. |
| Inicio: rotación | Semilla sorteada al abrir la app, guardada en `sessionStorage` (estable con atrás y recarga); ↻ sortea otra. |
| Filas de listas del usuario | Fuera: las listas son de la Etapa 5. |
| Pendientes de la 4a (§11) | Ninguno entra en esta etapa. |

## 3. Runtime y cambio de configuración

### 3.1 Paquete `internal/engine`

Toma el armado que hoy hace `setup()` en `cmd/cinexplorer`:

```go
// Base is what does not depend on config.json.
type Base struct {
    AppDir   string
    Store    *store.Store
    ReadOnly bool
    Wikidata identify.Wikidata
}

// Runtime is everything built from one config.json.
type Runtime struct {
    Config     config.Config
    TMDB       identify.API     // nil sin token
    Images     *images.Cache    // uno por Runtime: Fetch no cambia después de armarlo
    Scanner    *scan.Scanner    // nil en modo consulta
    Identifier *identify.Runner // nil en modo consulta
}

func Start(b Base, cfg config.Config) *Runtime // arma y, si es escribible, lanza un escaneo
func (r *Runtime) Stop()                         // cancela escaneo e identificación y espera
```

- `Start` no escanea si `b.ReadOnly`.
- `Stop` cancela el contexto propio del `Runtime` (el escaneo y el `Runner` lo usan en lugar de `context.Background()`), cancela el reintento pendiente del `Runner` y espera a que el escaneo y la corrida en curso terminen. Lo guardado hasta ese momento queda (el escáner ya aborta sin marcar faltantes al cancelarse; el `Runner` guarda el lote en curso).
- `identify.Runner` suma un contexto base y `Close()`: `Trigger` y los reintentos lo usan; después de `Close`, `Trigger` no hace nada.
- `scan.Scanner.OnDone` sigue llamando a `Runner.Trigger` del mismo `Runtime`.

### 3.2 Servidor

- `Server` pierde `Roots`, `Scanner`, `TMDB`, `Identifier`, `Images` y `Language`; los lee de `s.rt()` (un `atomic.Pointer[engine.Runtime]`), **una vez por pedido**.
- `Server.Apply(cfg) error`: valida (§4.3), guarda `config.json`, `old.Stop()`, `engine.Start(nuevo)`, intercambia el puntero y limpia `setupPending`. Las llamadas a `Apply` se serializan con un mutex; mientras una corre, los pedidos siguen viendo el `Runtime` viejo (ya detenido) o el nuevo, nunca uno a medio armar.
- `Server.SetupPending bool` (atómico): verdadero cuando `config.json` no existía al arrancar y la carpeta es escribible.

### 3.3 Arranque (`cmd/cinexplorer`)

- `config.Load` devuelve `created=true` si falta el archivo: **ya no se guarda** en el arranque. Con la carpeta escribible, `setup` arma el `Runtime` con esa configuración propuesta **sin lanzar el escaneo** y marca `SetupPending`. En modo consulta, se usa la propuesta en memoria como hoy.
- Si el usuario cierra la app sin completar el asistente, el próximo arranque vuelve a mostrarlo.
- Con `-no-browser` y sin `config.json` la app espera el asistente (se informa en el log con la URL).

### 3.4 Raíces quitadas

`store.MarkOutsideRoots(roots []string) error`: marca `missing = 1` en los archivos presentes que no están bajo ninguna de las raíces configuradas. El escáner lo llama al empezar cada corrida, con **todas** las raíces configuradas (incluidas las no disponibles, que así no se tocan). Comparación con `underAny`, igual que `SyncFiles`.

## 4. Configuración: primer uso y Ajustes

### 4.1 Configuración

- `config.Load` completa `language` vacío con `es-AR` (antes `es-ES`), y la configuración propuesta usa `es-AR`.
- Idiomas ofrecidos: `es-AR`, `en-US`, `pt-BR` (lista fija en el cliente y en el servidor, `config.Languages`). `Apply` acepta además el idioma que ya estaba guardado (un `es-ES` existente no se rechaza).
- `imagePrefetch`: `none`, `posters`, `all`.

### 4.2 API

- `GET /api/config` →
  ```json
  {"setupPending": true, "readOnly": false,
   "roots": [{"path": "../cine", "available": true}],
   "suggested": ["../cine-ordenar"],
   "hasToken": true, "tokenHint": "…a1b2",
   "language": "es-AR", "languages": ["es-AR", "en-US", "pt-BR"],
   "imagePrefetch": "none"}
  ```
  `suggested`: carpetas hermanas (`config.DefaultRoots`) que no están en `roots`. El token nunca se devuelve; `tokenHint` son sus últimos 4 caracteres.
- `POST /api/config/root` `{"path": "D:\\otras"}` → `{"path": "../otras", "available": true}` o 400 con el motivo.
- `POST /api/config/token` `{"token": "eyJ…"}` → `{"valid": true}`, `{"valid": false}` (TMDB lo rechazó) o `{"valid": null}` (sin red: no se pudo verificar).
- `PUT /api/config` `{"roots": [...], "token": "…" | null, "language", "imagePrefetch"}` → el mismo cuerpo que `GET`. `token` ausente o `null` conserva el guardado; `""` lo quita. 400 si algo no valida; 409 en modo consulta.
- `/api/status` suma `setupPending`.

### 4.3 Validación

- **Raíz**: ruta absoluta o relativa a la carpeta de la app; debe existir y ser carpeta; no puede ser la carpeta de la app, ni contenerla, ni estar dentro de ella; no puede contener ni estar contenida en otra raíz de la lista. Se guarda en forma de catálogo (`appdir.Rel`, `/`), limpia con `path.Clean`. Una raíz ya guardada que no está disponible se acepta tal cual (disco desconectado).
- **Token**: se verifica con un pedido que el cliente ya hace (`Movie(ctx, 550, "en-US")`, *Fight Club*): `ErrUnauthorized` → inválido; `ErrOffline` → sin verificar; cualquier otro resultado (incluido 404) → válido. `PUT` no exige verificarlo.
- **Idioma** e **imagePrefetch**: de las listas de §4.1.
- Lista de raíces vacía: 400 ("elegí al menos una carpeta").

### 4.4 Qué dispara un cambio

`Apply` siempre reemplaza el `Runtime` y lanza un escaneo (incremental: si nada cambió en disco, es rápido). Efectos que ya existen y se mantienen: cambiar el idioma recalcula las identificaciones `auto`/`unmatched` (la clave de consulta incluye el idioma, 4a §8) y vuelve a pedir los datos de las películas; con una colección grande, pasar de `es-ES` a `es-AR` implica volver a consultar todo a TMDB. Ajustes lo advierte al cambiar el idioma.

## 5. Traducciones (es-AR)

- `tmdb.Client.Translations(ctx, id) ([]Translation, error)`: `GET /movie/{id}/translations` → `{translations: [{iso_639_1, iso_3166_1, data: {title, overview}}]}`.
- `identify.API` suma `Translations`. La identificación no lo usa (el corpus de calibración, grabado por URI, no cambia).
- `fetchMovie` (enriquecimiento y `Adopt`): pide los detalles en el idioma configurado como hoy y, si el idioma no es `en-US`, las traducciones. Título y sinopsis salen del primer idioma de la cadena (§2) con texto no vacío; si ninguno tiene, queda lo que devolvieron los detalles. Reemplaza al pedido extra en `en-US` de hoy (la cadena ya termina en inglés).
- `identify.Chain(lang) []string` es pura y tiene tests.
- Géneros y nombre de colección siguen saliendo de los detalles en el idioma configurado.
- **A verificar en la prueba real** (desde el entorno de diseño no hay acceso a TMDB): que `language=es-AR` sin traducción argentina devuelva título original/inglés y sinopsis vacía (el problema que la cadena resuelve), y la forma exacta de `/translations`.
- Limitación conocida: los candidatos de Revisar muestran el título que devuelve la búsqueda en el idioma configurado, sin la cadena.

## 6. Búsqueda

### 6.1 Índice (`internal/search`)

```go
type Index struct { /* *sql.DB en memoria, generación indexada */ }

func New() (*Index, error)
func (x *Index) Refresh(gen int64, items []catalog.SearchDoc) error // reconstruye si gen cambió
func (x *Index) Query(q string, limit int) ([]Hit, error)            // Hit: clave de ítem + rank
```

- Tabla `docs USING fts5(key UNINDEXED, title, original, directors, cast, files, tokenize = "unicode61 remove_diacritics 2")`.
- `catalog.SearchDocs(snap, roots) []SearchDoc`: un documento por ítem de Explorar (las mismas reglas de §5.3 de la 4a: presentes, sin `ignored`/`extra`). Película: título, original, directores, reparto y los títulos parseados de sus versiones. Sin identificar: título parseado en `title`.
- **Cambios**: `store.Snapshot` suma `Changes int64` = `SELECT total_changes()` leído en la misma transacción. El servidor reconstruye el índice cuando `Changes` difiere del indexado (un número distinto, no mayor: si la conexión se reabre, el contador empieza de nuevo). Un mutex evita dos reconstrucciones a la vez.
- **Consulta**: el texto se parte en palabras (letras y dígitos Unicode); cada una va entre comillas, la última con `*` (prefijo); todas obligatorias. Menos de 2 caracteres en total → sin resultados. Orden: `bm25(docs, 0, 10, 8, 4, 1, 2)` y desempate por título normalizado.
- **Directores**: de los ítems encontrados, los directores cuyo nombre normalizado (`quality.NormTitle`) contiene todas las palabras de la consulta (la última como prefijo), con cuántas películas del catálogo tienen. Hasta 5.

### 6.2 API

`GET /api/search?q=&limit=` (`limit` 1–200, por defecto 8) → `{"q": "...", "total": N, "items": [Item], "directors": [{"id", "name", "count"}]}`. `items` usa el mismo `Item` de Explorar (afiche, año, directores). `total`: cantidad de coincidencias sin el límite.

### 6.3 Interfaz

- En la barra: ⌕ que abre un campo; también **Ctrl+K / ⌘+K** desde cualquier página. Espera 150 ms sin teclear; descarta respuestas de consultas viejas.
- Desplegable: hasta 8 películas (miniatura del afiche, título, año · director; sin identificar con afiche genérico) y "Directores" (nombre y cantidad → `/explorar?director=<id>`). Al pie, "Ver los N resultados →". ↑/↓ mueven la selección, Enter abre la seleccionada o, sin selección, `/buscar?q=`; Esc cierra.
- `/buscar?q=`: campo con la consulta, fila de directores y la grilla de Explorar (el mismo `Afiche`, en tandas de 120). La URL se actualiza con `replaceState` al escribir.

## 7. Inicio

### 7.1 API

`GET /api/home?seed=<uint>` →

```json
{"total": 812,
 "rows": [
   {"kind": "recent", "value": "", "label": "", "href": "/explorar?orden=agregado", "items": [HomeItem]},
   {"kind": "decade", "value": "1970", "label": "1970s", "href": "/explorar?decada=1970", "items": [...]}
 ]}
```

`HomeItem`: `tmdbId, title, originalTitle, year, directors (nombres), countries, backdrop, poster` (versiones de imagen). `total`: películas identificadas presentes.

### 7.2 Filas (`catalog.Home(items, seed)`)

Sobre los ítems `movie` de Explorar. Hasta 20 ítems por fila. Una fila que no llega al mínimo no aparece. El sorteo usa `math/rand/v2` con PCG sembrado con `seed`: la misma semilla da las mismas filas y el mismo orden.

| Orden | Fila | Candidatos | Mínimo | Orden de ítems | `href` |
|---|---|---|---|---|---|
| 1 | Agregadas recientemente | todas | 1 | agregado desc | `?orden=agregado` |
| 2 | Década al azar | décadas | 6 | al azar | `?decada=` |
| 3 | Director al azar | directores con id TMDB | 3 | año asc | `?director=` |
| 4 | País al azar | países | 6 | al azar | `?pais=` |
| 5 | Género al azar | géneros | 6 | al azar | `?genero=` |
| 6 | Colección al azar | colecciones TMDB | 2 | año asc | `?coleccion=` |

Los `href` se arman con la misma función de facetas de Explorar (`catalog` no conoce la URL del cliente: la construye el servidor con los nombres de parámetro de §5.3 de la 4a).

### 7.3 Interfaz

- `/` pasa a ser el Inicio (antes redirigía a `/explorar`). Barra: `CINEXPLORER` · Inicio · Explorar · Revisar · ⌕ · estado · ⚙ (Ajustes).
- Título de fila en mayúsculas según el tipo: "AGREGADAS RECIENTEMENTE", "LOS 70" (década), "DIRIGIDAS POR <NOMBRE>", "CINE DE <PAÍS>" (país con `Intl.DisplayNames`), "<GÉNERO>", "<COLECCIÓN>". A la derecha, "ver todas →".
- Fila con scroll horizontal (`scroll-snap`), flechas ‹ › en pantallas con puntero; tarjetas 16:9 de ~320 px (~70 % del ancho en pantallas chicas).
- Tarjeta: `/img/backdrop/{id}.jpg?v=`; sin escena, el afiche con `object-fit: cover` y `filter: blur()`. Debajo, TÍTULO en mayúsculas y la línea "DIRECTOR PAÍS AÑO" (`format.creditLine`: primer director, países con nombre corto, año; lo que falte se omite). Clic → Ficha. Imágenes con `loading="lazy"`.
- Semilla: `sessionStorage` (`cx-home-seed`), sorteada si falta; ↻ junto al título de la página la reemplaza.
- Catálogo sin películas identificadas: mientras escanea o identifica, "Escaneando…"/"Identificando…" con el progreso de `status.js`; si no, un texto que lleva a Explorar y a Revisar (o a Ajustes si falta el token).
- Se recarga con `app.generation`, igual que Explorar.

## 8. Asistente y Ajustes (interfaz)

- `app.svelte.js` pide `/api/config` al arrancar. Con `setupPending`, cualquier ruta muestra `/bienvenida` (`replaceState`).
- **Bienvenida**, 3 pasos con "Atrás" / "Siguiente":
  1. **Carpetas**: casillas con `roots` (marcadas) y `suggested` (marcadas en el primer uso), y un campo "Agregar otra carpeta" que llama a `POST /api/config/root` y muestra el error si no valida.
  2. **Token de TMDB**: explicación corta (el *API Read Access Token*, el largo que empieza con `eyJ…`), enlace a `https://www.themoviedb.org/settings/api` (`target="_blank"`), campo, **Verificar** (✓ válido / ✕ rechazado / "no se pudo verificar, se guarda igual") y "Seguir sin token".
  3. **Idioma**: opciones con nombre (`Español (Argentina)`, `English (US)`, `Português (Brasil)`), `es-AR` elegido.
  **Empezar** → `PUT /api/config` → navega a `/`.
- **Ajustes** (`/ajustes`): las cuatro secciones en una página (carpetas, token con "…a1b2" y "Cambiar"/"Quitar", idioma con el aviso de §4.4, descarga de imágenes) y un **Guardar** que manda todo junto. En modo consulta, todo deshabilitado con el aviso.
- Los textos que hoy dicen "en config.json" (`status.js`, Explorar vacío) pasan a enlazar a Ajustes.

## 9. Estructura de los cambios

```
internal/engine/           nuevo: Base, Runtime, Start, Stop
internal/search/           nuevo: índice FTS5 en memoria
internal/catalog/          home.go (filas), search.go (SearchDocs, directores)
internal/config/           Languages, es-AR por defecto, validación de raíces
internal/identify/         Chain, Translations en API, Runner con contexto base y Close
internal/tmdb/             Translations
internal/store/            MarkOutsideRoots, Snapshot.Changes
internal/scan/             MarkOutsideRoots al empezar
internal/server/           Apply, rt(), config.go, search.go, home.go
cmd/cinexplorer/           setup sin guardar config ni escanear en el primer uso
web/src/lib/               search.js, home.js, settings.js (+ tests); format.creditLine; router
web/src/pages/             Inicio, Buscar, Bienvenida, Ajustes
web/src/components/        Busqueda (⌕ + desplegable), FilaInicio, TarjetaEscena, Raices, Token, Idioma
```

## 10. Pruebas

- **engine**: `Start` escribible lanza el escaneo y en modo consulta no; `Stop` cancela un escaneo y una identificación en curso y espera; después de `Stop`, `Trigger` no corre.
- **identify**: `Chain` por idioma; `fetchMovie` con traducciones (es-AR vacío → es-MX; ninguna → detalles; `en-US` no pide traducciones); `Runner.Close` cancela el reintento. El corpus sigue pasando sin regrabar.
- **tmdb**: `Translations` contra un servidor de prueba.
- **config**: `es-AR` por defecto; validación de raíces (inexistente, archivo, carpeta de la app, anidadas, absoluta → relativa, ya guardada no disponible).
- **store**: `MarkOutsideRoots` (fuera → faltante, bajo raíz no disponible → intacto, reaparece al reescanear); `Snapshot.Changes` cambia después de una escritura.
- **catalog**: `Home` (determinismo por semilla, mínimos, orden de ítems, `href`, sin sin-identificar); `SearchDocs`.
- **search**: tildes y mayúsculas, prefijo en la última palabra, AND, relevancia título > reparto, sin identificar, caracteres especiales de FTS5 en la consulta (`"`, `*`, `-`, `:`), reconstrucción solo si cambió la generación.
- **server**: `GET/PUT /api/config` (token oculto, `null` conserva, `""` quita, 400, 409 en modo consulta), `root`, `token` (válido/rechazado/sin red con un `API` falso), `Apply` reemplaza el `Runtime` y limpia `setupPending`, `/api/search`, `/api/home`, `/api/status` con `setupPending`, fallback a `index.html` en `/bienvenida`, `/ajustes`, `/buscar`.
- **Smoke test**: sin `config.json`, `setup` no crea el archivo ni escanea y `/api/status` informa `setupPending`; `PUT /api/config` crea el archivo, escanea y `/api/explore?decada=1970` lista la película; `/api/search?q=…` la encuentra.
- **web (Vitest)**: `search.js` (consulta mínima, navegación con teclado, destino de Enter), `home.js` (títulos de fila, semilla), `format.creditLine`, `settings.js` (cuerpo del `PUT`, cambios pendientes), `router` (`/` → inicio, `/buscar`, `/ajustes`, `/bienvenida`).
- **Prueba real** (manual, fuera de CI): carpeta nueva sin `config.json` → asistente completo con token real → Inicio poblándose; búsqueda de "almodovar", "bunuel", un actor y un título sin identificar; Ajustes: quitar una raíz, cambiar a `en-US` y volver; comprobar los títulos en es-AR de películas sin traducción argentina (§5).

## 11. Pendientes

- Los de la 4a (§11) siguen pendientes.
- Fila de listas del usuario en el Inicio: Etapa 5.
- `GET /api/movies?q=` (selector "es un extra de…") podría usar el índice; queda como está.
- Cadena de idiomas en los candidatos de Revisar (§5).
