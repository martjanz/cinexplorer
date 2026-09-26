# Cinexplorer — Etapa 3: Identificación — Diseño

Fecha: 2026-09-26
Estado: aprobado en brainstorming; ajustado tras prototipar (§3 archivos vacíos, §6 variantes, inglés y carpeta)
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

**Archivos vacíos** (0 bytes: copias fallidas o marcadores; hay 32 en la colección real): todos tendrían la misma huella y una identificación se contagiaría entre ellos, además de figurar como copias idénticas. `fingerprint.Of` devuelve `""` para ellos, así que no tienen huella, no son copias ni se identifican; la versión conserva los datos del nombre. El escáner no reutiliza la huella guardada de un archivo vacío, de modo que los catálogos anteriores se corrigen en el siguiente escaneo.

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
  cast_members   TEXT    NOT NULL DEFAULT '[]', -- JSON [{id,name,character}], primeros 10
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

**IMDb desde `.nfo`**: si la carpeta de la versión contiene un archivo de tipo `info`, se buscan ids `tt\d{7,8}` en su contenido (tope de 64 KiB leídos). Tiene prioridad sobre el IMDb extraído del nombre. Si hay varios `.nfo`, gana el primero por nombre que contenga un id. En una carpeta con **varias versiones** (películas sueltas en una raíz o en `1970s/`), un `.nfo` solo cuenta para la versión cuyo archivo representativo empieza con el nombre del `.nfo` (`Amarcord.nfo` → `Amarcord CD1.avi`); si no, el IMDb de una película se impondría con confianza 1.0 a todas las de la carpeta.

**Mejor versión**: `markBest` agrupa por `tmdb_id` para versiones `auto`/`manual`; el resto sigue con `quality.GroupKey`. El cálculo sigue haciéndose sobre todas las versiones, no sobre un subconjunto filtrado.

## 5. Cliente TMDB (`internal/tmdb`)

El limitador y los reintentos viven en `internal/httpx` (compartido con Wikidata): `Limiter` (token bucket propio, sin dependencias) y `Client.Get` (GET con reintentos que devuelve el cuerpo de un 2xx, `*StatusError` para otros códigos finales y `ErrOffline` al agotar los intentos).

- Auth: `Authorization: Bearer <tmdbToken>` (token de lectura v4 de `config.json`). URL base inyectable para tests.
- Rate limit: 20 req/s, ráfaga 10.
- Reintentos: 429 (respeta `Retry-After`), 5xx y errores de red → backoff exponencial 1, 2, 4, 8 s con jitter; 5 intentos en total.
- Errores tipados: `ErrOffline` (= `httpx.ErrOffline`: red/DNS/timeout o 429/5xx tras agotar reintentos), `ErrUnauthorized` (401), `ErrNotFound` (404).
- Métodos:
  - `SearchMovie(ctx, query string, year int, lang string) ([]Result, error)` — `/3/search/movie`.
  - `FindIMDb(ctx, imdbID, lang string) ([]Result, error)` — `/3/find/{id}?external_source=imdb_id`.
  - `Movie(ctx, id int, lang string) (Details, error)` — `/3/movie/{id}?append_to_response=credits` (el IMDb id viene en el campo `imdb_id` de la película).
  - `Image(ctx, path, size string) ([]byte, error)` — `image.tmdb.org/t/p/{size}{path}`.


## 6. Identificación (`internal/identify`)

Por cada versión pendiente se arma una **consulta**: título, año y director del nombre, IMDb del `.nfo` o del nombre, y, si el nombre de la carpeta parece de película (el parser le encuentra año) y difiere del de la versión, una **consulta alternativa por carpeta**. Esto cubre carpetas con varias versiones que conservan nombres de archivo pobres (`Los Gauchos Judios Rip mentecato cd 01.avi` dentro de `Los Gauchos Judíos (Juan José Jusid, Argentina, 1974)`). La clave de la consulta (`query`) incluye la alternativa.

1. **IMDb** → `FindIMDb`. Con resultado: `auto`, confianza 1.0.
2. **Variantes del título**: el texto fuera de paréntesis/corchetes y cada parte entre ellos (`La piel dura (L'argent de poche)` → `La piel dura`, `L'argent de poche`). Se busca la primera; las demás solo si el resultado no es concluyente, y los resultados se suman (hasta 10 películas). Cada búsqueda es `SearchMovie(variante, año)` y, sin resultados, sin año.
3. **Títulos en inglés**: si con el idioma configurado no es concluyente, se busca también en `en-US` y el título inglés de cada película cuenta para la similitud (archivos nombrados en inglés: *Cries and Whispers* es *Gritos y susurros* / *Viskningar och rop*).
4. **Puntaje** (0..1):
   - Título (≤ 0.60): máxima similitud entre cualquier variante y `title` / `original_title` / título inglés del candidato, normalizados con `quality.NormTitle` (sin tildes, puntuación ni artículos iniciales), por Levenshtein normalizado (`1 − dist/max(len)`), × 0.60.
   - Año (≤ 0.25): exacto 0.25; ±1 0.15; sin año parseado 0.10; otro 0.
   - Director (≤ 0.15): solo si el nombre trae director. Para los 3 mejores candidatos se pide `Movie()` (se reutiliza al enriquecer) y suma 0.15 si la última palabra del director parseado es una palabra de algún director de TMDB (`Polanski` ↔ `Roman Polański`). Sin director parseado, título + año se reescala dividiendo por 0.85.
5. **Concluyente** si el mejor ≥ `AutoThreshold` (0.80) y supera al segundo por ≥ `AutoMargin` (0.10) → `auto`. Si no: `unmatched` con los 5 mejores candidatos.
6. Si la consulta no fue concluyente y hay alternativa por carpeta, se identifica con ella y se queda el mejor resultado (concluyente, o de mayor confianza).

`MatcherVersion` empieza en 1 y sube cuando cambia el algoritmo o los umbrales.

`quality.folds` suma letras de Europa del Este y del turco (`ł`, `ń`, `š`, `ş`, `ı`…) para que la normalización las pliegue.

**Calibración** (corpus de 30 nombres reales de `D:\cine`, respuestas de TMDB grabadas): 23 automáticas, todas correctas, sin falsos positivos. Quedan para revisión los empates reales (*Hamaca paraguaya* 2000/2006, *Ordet* 1943/1955), títulos que TMDB no tiene o no reconoce (*Los traidores*, *El cazador (Shekarchi)*, un nombre con errata) y *Star Wars Episode IV A New Hope* (TMDB la titula *Star Wars*). Prueba sobre `D:\cine\1970s` completo: 33 automáticas de 41 versiones, todas correctas.

## 7. Enriquecimiento

- Para cada `tmdb_id` referenciado por identificaciones `auto`/`manual`/`extra` que no está en `movies` o cuyo `language` difiere del configurado: `Movie(id, lang)` (o los datos ya pedidos al identificar, en la misma corrida). Si `title` u `overview` vienen vacíos: `Movie(id, "en-US")` y se completan. Directores: `credits.crew` con `job = "Director"`. Reparto: primeros 10 de `credits.cast` por `order`. Países: `production_countries[].iso_3166_1`. IMDb: `imdb_id`.
- **Wikidata** (`internal/wikidata`): películas con `wikidata_state = 0`, en lotes de hasta 50, consulta SPARQL a `query.wikidata.org/sparql` con `VALUES ?tmdb {…}` sobre P4947, trayendo QID, P345, P495 (→ ISO por P297), P57 (etiqueta en el idioma configurado, fallback `en`) y P577 (año más temprano). `User-Agent: cinexplorer/<versión> (https://github.com/martjanz/cinexplorer)`. 1 req/s, mismos reintentos. Solo llena campos vacíos; siempre guarda `wikidata_id` si lo hay. Sin coincidencia → `wikidata_state = 1` igual. Un error que no es de red saltea la fase hasta la próxima corrida.

## 8. Imágenes

- `GET /img/poster/{tmdbId}.jpg` (w342) y `GET /img/backdrop/{tmdbId}.jpg` (w1280): se sirven desde `cache/posters/` y `cache/backdrops/`; si faltan, se descargan una vez (cada descarga con su propio temporal + rename, así dos pedidos simultáneos no se pisan; guardar es best effort), se guardan y se sirven. La ruta de TMDB sale de `movies`; para un candidato que todavía no es película guardada, la página la pasa como `?p=/abc.jpg` (validada: `^/[A-Za-z0-9_-]+\.(jpg|png)$`) y la imagen se muestra **sin guardarse** (`Cache.Preview`): cualquier página puede mandar `?p=`, y no debe decidir qué imagen queda en caché para una película. Sin ruta, sin token o sin red → 404.
- En modo consulta (directorio no escribible) se sirven sin guardar.
- `config.json` suma `"imagePrefetch": "none" | "posters" | "all"` (vacío o desconocido = `"none"`). Con `posters`/`all` el Runner agrega una fase final que descarga lo que falte.

## 9. Runner y pipeline

`identify.Runner`: `Trigger()` lanza una corrida en segundo plano (un `Trigger` durante una corrida encola exactamente una más); `Run(ctx)` corre una sincrónicamente (tests). El escáner llama a `Trigger` al terminar cada escaneo (`Scanner.OnDone`).

1. Sin `tmdbToken`: no hace nada; estado `noToken`.
2. Fases en orden: **identificar** (huellas pendientes) → **enriquecer** → **Wikidata** → **prefetch** (opcional).
3. Guardado cada 20 identificaciones y al final. Cancelable por `ctx`.
4. `ErrOffline` (TMDB, Wikidata o imágenes): corta la corrida, estado `offline`, y programa un `Trigger` a los 5 min, duplicando hasta 1 h mientras siga offline. Cada corrida cancela el reintento pendiente; una corrida que termina sin estar offline reinicia el backoff.
5. `ErrUnauthorized`: corta todo, estado `badToken`, sin reintentos automáticos.
6. Otros errores de un ítem (JSON inesperado, etc.): se registran y se sigue con el siguiente.

`GET /api/status` suma `identify: {state, toIdentify, identified, toEnrich, enriched}` (`state`: `idle` | `running` | `offline` | `noToken` | `badToken`; `null` en modo consulta).

## 10. API y página

- `GET /api/versions`: suma `fingerprint`, `identification` (`{status, confidence, tmdbId}`) y `movie` (`{tmdbId, title, originalTitle, year, directors}`) cuando está identificada como película guardada.
- `GET /api/unidentified`: versiones `unmatched` (una por huella) con los campos de versión y `candidates`.
- `GET /api/tmdb/search?q=&year=`: búsqueda manual con candidatos puntuados; si `q` es `tt\d{7,8}`, usa `FindIMDb`. Sin token → 503.
- `POST /api/identify` `{fingerprint, action: "movie"|"ignore"|"extra"|"reset", tmdbId}`: `movie`/`extra` validan el id contra TMDB y guardan la película en el momento (404 si no existe, 503 sin red); después relanzan el Runner (Wikidata, imágenes). `reset` borra la fila y relanza el Runner. La huella se valida primero, antes de consultar TMDB: tiene que ser la del representativo de una versión (`store.IsRepresentative`); si no → 404, también en `reset`. Sin token → 503. En modo consulta → 409, como `/api/scan`.
- `GET /img/{poster|backdrop}/{id}.jpg[?p=]`.
- Página mínima: por versión, título TMDB · año · director e indicador (`92%`, `manual`, `sin identificar`, `no es película`, `extra`, `id inválido`); pestaña "Sin identificar" con candidatos (afiche chico, título, año, título original, %), botón **Confirmar** y campo de búsqueda manual (título o `tt…`). "Ignorar" y "extra de" quedan solo en la API (UI en Etapa 4). La línea de estado muestra el progreso y los estados `offline` / `noToken` / `badToken`.

## 11. Errores

- Sin red: reintento programado; nada se marca fallido.
- 404 de TMDB para un id guardado: se borra de `movies`; las identificaciones `auto` que lo apuntaban se borran (vuelven a pendiente); las `manual`/`extra` se conservan y se muestran como "id inválido".
- JSON inesperado: se registra y se omite ese ítem.
- Token inválido: estado `badToken`.
- `.nfo` ilegible: se ignora (se identifica por nombre).
- Catálogo de una etapa anterior abierto en modo consulta (sin tablas nuevas): la identidad queda vacía, sin errores.

**Pedidos de otros sitios:** además del control de `Host` (DNS rebinding), el servidor rechaza con 403 los pedidos con `Sec-Fetch-Site: cross-site` o `same-site` salvo navegaciones GET, y agrega `Cross-Origin-Resource-Policy: same-origin` a todas las respuestas. Sin esto, una página cualquiera podría usar `<img src="http://127.0.0.1:PUERTO/img/poster/ID.jpg">` para averiguar qué películas hay en el catálogo, o gastar la cuota de TMDB.

## 12. Pruebas

- **httpx**: reintentos 429/5xx, `Retry-After`, red caída → `ErrOffline`, estado final (401) sin reintento, encabezados, limitador.
- **tmdb**: `httptest.Server`; búsqueda, find, detalles con créditos, imágenes, 401/404/503/JSON roto.
- **wikidata**: respuesta SPARQL real grabada (`testdata/lookup.json`); agregación por película, etiquetas sin nombre descartadas, lotes.
- **images**: descarga única, sin red / sin fetcher / rutas inválidas → `ErrUnavailable`, modo consulta no guarda.
- **store**: tablas nuevas, representativo por versión (partes, DVD, copias idénticas), correcciones que el matcher no pisa, películas, cola de enriquecimiento y Wikidata, invalidación, `markBest` por `tmdb_id`, `Unidentified`, catálogo viejo en modo consulta.
- **identify**: puntaje por tabla; `Search` con API falso (sin año, inglés, director que desempata, IMDb inexistente); **corpus** (`corpus_test.go`) con 30 nombres reales y respuestas de TMDB grabadas en `testdata/tmdb_corpus.json` — se regraba con `CINEXPLORER_TMDB_RECORD=1` y un token en `CINEXPLORER_TMDB_TOKEN` o `~/.cinexplorer-tmdb-token`; Runner con TMDB/Wikidata falsos: identificación + enriquecimiento + Wikidata, reutilización de detalles, correcciones no se pisan, cambio de `query` y `MatcherVersion` re-identifican, reintentos offline con backoff, 401, id inexistente, error de Wikidata, prefetch, `Adopt`, `Trigger`, alternativa por carpeta.
- **scan**: `OnDone`, archivos vacíos sin huella (también en catálogos anteriores).
- **server**: identificación manual, errores de `POST /api/identify`, `unidentified`, búsqueda, imágenes (candidato con `?p=`, caché, tipos y rutas inválidas), estado.
- **Prueba real** (manual, fuera de CI): binario con `config.json` apuntando a una parte de la colección y token real; revisar la pestaña "Sin identificar" y que las automáticas sean correctas.

## 13. Pendientes (revisión final de la Etapa 3)

No bloquean; conviene tenerlos en cuenta en las etapas siguientes.

- **Imágenes cacheadas solo por id de TMDB**: si cambia `poster_path` (los afiches de TMDB dependen del idioma, o TMDB los actualiza), se sigue sirviendo el archivo viejo. Guardar con un nombre derivado de la ruta o borrar el archivo cuando `SaveMovie` cambia la ruta. Es lo primero que se va a notar: conviene hacerlo al principio de la Etapa 4.
- **Huérfanos**: identificaciones de huellas que ya no existen (archivo reescrito) y películas que solo ellas referencian no se borran; el prefetch y Wikidata las siguen procesando. Una corrección manual se pierde en silencio si su archivo se reescribe. Limpiar filas `auto`/`unmatched` sin archivo y restringir el prefetch a películas referenciadas.
- **Clave de consulta inestable ante errores transitorios**: un `.nfo` ilegible en una corrida saca el IMDb de la clave y re-identifica por título; dos copias idénticas con nombres distintos pueden alternar la consulta según el orden. Tratar un error de lectura como "conservar el IMDb anterior".
- **Errores por ítem que se reintentan en cada corrida**: un 4xx en la búsqueda o un 404 en `Movie()` durante el chequeo de directores hace fallar el ítem, que solo se registra en el log. Guardarlo como `unmatched` con los candidatos disponibles. Lo mismo para `manual`/`extra` que apuntan a un id inválido (un 404 por corrida).
- **Candidatos en el idioma anterior**: tras cambiar `language`, los títulos de `candidates` de las `unmatched` quedan en el idioma viejo hasta que cambie la consulta o `MatcherVersion`.
- **`Run` no participa del encolado de `Trigger`** (solo lo usan los tests); una cancelación deja el estado en `idle`.
- **Lecturas de `Versions()` sin transacción** (pendiente de la Etapa 2): con el Runner escribiendo en segundo plano, una respuesta puede mezclar estados. Envolver las consultas de `Versions()`/`Unidentified()` en una transacción de lectura.
