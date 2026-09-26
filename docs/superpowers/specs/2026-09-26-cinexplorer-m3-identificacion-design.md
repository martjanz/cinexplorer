# Cinexplorer — Etapa 3: Identificación — Diseño

Fecha: 2026-09-26
Estado: aprobado en brainstorming
Spec general: `2026-09-25-cinexplorer-design.md` (§3.1, §4 pasos 5–6, §4.2, §5.5)

## 1. Objetivo

Asociar cada versión en disco a una película de TMDB, con asignación automática cuando la confianza es alta y cola de revisión con candidatos cuando no. Enriquecer cada película con datos de TMDB (y huecos completados desde Wikidata), cachear imágenes y tolerar la falta de red reintentando más tarde. Las correcciones manuales se atan a la huella de contenido y sobreviven a renombres y movimientos.

## 2. Decisiones

| Tema | Decisión |
|---|---|
| Alcance manual | Backend completo + API de correcciones (movie / ignore / extra / reset, búsqueda manual) + lo mínimo en la página actual para confirmar candidatos. La pantalla Revisar real es de la Etapa 4. |
| Wikidata | Acotado: por P4947 (id TMDB), guarda el QID y **solo completa huecos** de TMDB: IMDb (P345), países (P495), directores (P57), año (P577). |
| Imágenes | Configurable con `imagePrefetch`: `"none"` (por defecto, bajo demanda), `"posters"` (afiches por adelantado, escenas bajo demanda), `"all"` (todo por adelantado). |
| Pipeline | Paquete `internal/identify` con un `Runner` propio que el escáner invoca después del análisis técnico; se reprograma solo si no hay red. |
| Clave | Huella de contenido del archivo principal representativo de la versión (ver §3). |

## 3. Clave de identificación

`ReplaceVersions` recrea todas las versiones en cada escaneo, así que nada se ata al id de versión. La clave es la **huella** del miembro principal (`role = 'main'`) de menor `part`; empate → el de mayor tamaño (en un DVD todos los miembros tienen parte 0, queda el VOB más grande). Las copias idénticas comparten huella: una corrección vale para todas.

Una versión sin huella (archivo aún no hasheado) no se identifica hasta que la tenga.

## 4. Persistencia

```sql
CREATE TABLE IF NOT EXISTS movies (
  tmdb_id        INTEGER PRIMARY KEY,
  title          TEXT    NOT NULL,             -- localizado (fallback en-US)
  original_title TEXT    NOT NULL DEFAULT '',
  year           INTEGER NOT NULL DEFAULT 0,
  runtime        INTEGER NOT NULL DEFAULT 0,   -- minutos
  original_lang  TEXT    NOT NULL DEFAULT '',
  overview       TEXT    NOT NULL DEFAULT '',
  directors      TEXT    NOT NULL DEFAULT '[]', -- JSON [{id,name}]
  cast           TEXT    NOT NULL DEFAULT '[]', -- JSON [{id,name,character}], primeros 10
  genres         TEXT    NOT NULL DEFAULT '[]', -- JSON nombres localizados
  countries      TEXT    NOT NULL DEFAULT '[]', -- JSON ISO 3166-1 alfa-2
  collection_id  INTEGER NOT NULL DEFAULT 0,
  collection     TEXT    NOT NULL DEFAULT '',
  poster_path    TEXT    NOT NULL DEFAULT '',
  backdrop_path  TEXT    NOT NULL DEFAULT '',
  imdb_id        TEXT    NOT NULL DEFAULT '',
  wikidata_id    TEXT    NOT NULL DEFAULT '',
  language       TEXT    NOT NULL,             -- idioma pedido a TMDB
  fetched_at     INTEGER NOT NULL,             -- unix ms
  wikidata_state INTEGER NOT NULL DEFAULT 0    -- 0 pendiente, 1 consultado
);

CREATE TABLE IF NOT EXISTS identifications (
  fingerprint     TEXT    PRIMARY KEY,
  status          TEXT    NOT NULL,            -- auto | manual | ignored | extra | unmatched
  tmdb_id         INTEGER NOT NULL DEFAULT 0,  -- película (auto/manual) o "extra de" (extra)
  confidence      REAL    NOT NULL DEFAULT 0,
  candidates      TEXT    NOT NULL DEFAULT '[]', -- JSON top 5 {tmdbId,title,originalTitle,year,posterPath,score}
  query           TEXT    NOT NULL DEFAULT '', -- "título|año|director|imdb" usados
  matcher_version INTEGER NOT NULL DEFAULT 0,
  updated_at      INTEGER NOT NULL             -- unix ms
);
```

`CREATE TABLE IF NOT EXISTS` basta como migración (igual que `media`).

Estados:
- **Correcciones** (`manual`, `ignored`, `extra`): nunca se sobrescriben automáticamente. `reset` borra la fila y la versión vuelve a identificarse.
- **Automáticos** (`auto`, `unmatched`): se recalculan si cambia `query` (el parser produce otro título/año/director/IMDb) o si `matcher_version < identify.MatcherVersion`.
- **Sin fila**: pendiente (nunca procesada, o falló por red).

**IMDb desde `.nfo`**: si la carpeta de la versión contiene un archivo de tipo `info`, se buscan ids `tt\d{7,8}` en su contenido (tope de 64 KiB leídos). Tiene prioridad sobre el IMDb extraído del nombre. Si hay varios `.nfo`, gana el primero por nombre que contenga un id.

**Mejor versión**: `markBest` agrupa por `tmdb_id` para versiones `auto`/`manual`; el resto sigue con `quality.GroupKey`. El cálculo sigue haciéndose sobre todas las versiones, no sobre un subconjunto filtrado.

## 5. Cliente TMDB (`internal/tmdb`)

- Auth: `Authorization: Bearer <tmdbToken>` (token de lectura v4 de `config.json`). URL base inyectable para tests.
- Rate limit: token bucket propio (sin dependencias), 20 req/s, ráfaga 10.
- Reintentos: 429 (respeta `Retry-After`), 5xx y errores de red → backoff exponencial 1, 2, 4, 8, 16 s con jitter; máximo 5 intentos.
- Errores tipados: `ErrOffline` (red/DNS/timeout tras agotar reintentos), `ErrUnauthorized` (401), `ErrNotFound` (404).
- Métodos:
  - `SearchMovie(ctx, query string, year int, lang string) ([]Result, error)` — `/3/search/movie`.
  - `FindIMDb(ctx, imdbID, lang string) ([]Result, error)` — `/3/find/{id}?external_source=imdb_id`.
  - `Movie(ctx, id int, lang string) (Details, error)` — `/3/movie/{id}?append_to_response=credits,external_ids`.
  - `Image(ctx, path, size string) ([]byte, error)` — `image.tmdb.org/t/p/{size}{path}`.

## 6. Identificación (`internal/identify`)

Por cada versión pendiente:

1. **IMDb** (del `.nfo` o del nombre) → `FindIMDb`. Con resultado: `auto`, confianza 1.0.
2. Si no: `SearchMovie(título, año)` en el idioma configurado; sin resultados → reintenta sin año. Se puntúan los primeros 10 resultados.
3. **Puntaje** (0..1):
   - Título (≤ 0.60): máxima similitud entre el título parseado y `title` / `original_title` del candidato, ambos normalizados como en `quality.GroupKey` (sin tildes, puntuación ni artículos iniciales), por Levenshtein normalizado (`1 − dist/max(len)`), × 0.60.
   - Año (≤ 0.25): exacto 0.25; ±1 0.15; sin año parseado 0.10; otro 0.
   - Director (≤ 0.15): solo si el nombre trae director. Para los 3 mejores candidatos por título+año se pide `Movie()` (se reutiliza luego al enriquecer) y suma 0.15 si algún director de TMDB coincide (apellido normalizado del parseado contenido en el nombre normalizado de TMDB). Sin director parseado, el puntaje título+año se reescala dividiendo por 0.85.
4. **Asignación automática** si mejor ≥ `AutoThreshold` (0.80) y supera al segundo por ≥ `AutoMargin` (0.10). Si no: `unmatched` con los 5 mejores candidatos. Umbrales como constantes, calibrados con el corpus grabado.
5. Sin resultados: `unmatched` sin candidatos.

`MatcherVersion` empieza en 1 y sube cuando cambia el algoritmo o los umbrales.

## 7. Enriquecimiento

- Para cada `tmdb_id` referenciado por identificaciones `auto`/`manual`/`extra` que no está en `movies` o cuyo `language` difiere del configurado: `Movie(id, lang)`. Si `title` u `overview` vienen vacíos: `Movie(id, "en-US")` y se completan. Directores: `credits.crew` con `job = "Director"`. Reparto: primeros 10 de `credits.cast` por `order`. Países: `production_countries[].iso_3166_1`. IMDb: `external_ids.imdb_id`.
- **Wikidata** (`internal/wikidata`): películas con `wikidata_state = 0`, en lotes de hasta 50, consulta SPARQL a `query.wikidata.org/sparql` con `VALUES ?tmdb {…}` sobre P4947, trayendo QID, P345, P495 (→ ISO por P297), P57 (etiqueta en el idioma configurado, fallback `en`) y P577 (año). `User-Agent: cinexplorer/<versión> (https://github.com/martjanz/cinexplorer)`. 1 req/s, mismos reintentos. Solo llena campos vacíos; siempre guarda `wikidata_id` si lo hay. Sin coincidencia → `wikidata_state = 1` igual.

## 8. Imágenes

- `GET /img/poster/{tmdbId}.jpg` (w342) y `GET /img/backdrop/{tmdbId}.jpg` (w1280): se sirven desde `cache/posters/` y `cache/backdrops/`; si faltan, se descargan una vez (escritura a `.tmp` + rename), se guardan y se sirven. Sin ruta en `movies` o sin red → 404.
- En modo consulta (directorio no escribible) se sirven sin guardar.
- `config.json` suma `"imagePrefetch": "none" | "posters" | "all"` (vacío o desconocido = `"none"`). Con `posters`/`all` el Runner agrega una fase final que descarga lo que falte.

## 9. Runner y pipeline

`identify.Runner.Run(ctx)`, invocado por el escáner después de la fase de análisis técnico:

1. Sin `tmdbToken`: no hace nada; estado `noToken`.
2. Fases en orden: **identificar** (huellas pendientes) → **enriquecer** → **Wikidata** → **prefetch** (opcional).
3. Guardado cada 20 resultados y al final. Cancelable por `ctx`.
4. `ErrOffline`: corta la fase, estado `offline`, y programa un nuevo `Run` a los 5 min, duplicando hasta 1 h mientras siga offline. Un escaneo manual también lo relanza (y reinicia el backoff).
5. `ErrUnauthorized`: corta todo, estado `badToken`, sin reintentos automáticos.
6. Un solo `Run` a la vez; un pedido mientras corre se encola (a lo sumo uno).

`scan.Status` suma `identified`, `toIdentify`, `enriched`, `toEnrich` e `identify` (`idle` | `running` | `offline` | `noToken` | `badToken`).

## 10. API y página

- `GET /api/versions`: suma `fingerprint`, `identification` (`{status, confidence}`) y `movie` (`{tmdbId, title, originalTitle, year, directors}`) cuando está identificada.
- `GET /api/unidentified`: versiones `unmatched` con ruta, tokens parseados y candidatos.
- `GET /api/tmdb/search?q=&year=`: búsqueda manual con candidatos puntuados; si `q` es `tt\d+`, usa `FindIMDb`.
- `POST /api/identify` `{fingerprint, action: "movie"|"ignore"|"extra"|"reset", tmdbId}`: `movie`/`extra` validan el id contra TMDB y enriquecen en el momento; `reset` borra la fila y relanza el Runner. En modo consulta → 403.
- `GET /img/{poster|backdrop}/{id}.jpg`.
- Página mínima: por versión, título TMDB · año · director e indicador de confianza; sección "Sin identificar" con candidatos (afiche chico, título, año, %), botón **Confirmar** y campo de búsqueda manual. "Ignorar" y "extra de" quedan solo en la API (UI en Etapa 4).

## 11. Errores

- Sin red: reintento programado; nada se marca fallido.
- 404 de TMDB para un id guardado: se borra de `movies`; las identificaciones `auto` que lo apuntaban se borran (vuelven a pendiente); las `manual`/`extra` se conservan y se muestran como "id inválido".
- JSON inesperado: se registra y se omite ese ítem.
- Token inválido: estado `badToken`.
- `.nfo` ilegible: se ignora (se identifica por nombre).

## 12. Pruebas

- **tmdb**: `httptest.Server` con JSON grabados en `testdata/`; rate limit, reintentos 429/5xx, `Retry-After`, `ErrOffline`, 401, 404.
- **identify**: puntaje por tabla (título, año ±1, director, reescalado); corpus de ~20 nombres reales con búsquedas grabadas para calibrar umbrales; Runner con TMDB/Wikidata falsos: correcciones no se pisan, cambio de `query` re-identifica, `MatcherVersion` re-identifica, offline programa reintento, 401 corta.
- **wikidata**: JSON SPARQL grabado; solo completa huecos.
- **store**: tablas nuevas, representativo por versión, `markBest` por `tmdb_id`, estados de corrección, `.nfo`.
- **server**: endpoints nuevos, caché de imágenes (hit, miss con descarga, sin red → 404, modo consulta).
- **Corpus real** (opcional, fuera de CI): `CINEXPLORER_IDENTIFY_CORPUS=<dir>` + `CINEXPLORER_TMDB_TOKEN`, reporta tasa de identificación automática y los `unmatched`.
