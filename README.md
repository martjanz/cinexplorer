# Cinexplorer

Explorador local de una colección de películas. Recorre las carpetas que le
indiques, arma un catálogo (qué película es cada archivo, en qué versiones la
tenés, con qué calidad, subtítulos y duplicados) y lo muestra en el navegador.
No importa cómo estén organizadas las carpetas en disco.

- **Solo lectura:** nunca mueve, renombra ni borra archivos de video. Lo único
  que escribe es su propio catálogo, junto al ejecutable.
- **Portátil:** un ejecutable por sistema, sin instalador ni dependencias. El
  catálogo guarda rutas relativas, así que el mismo disco externo funciona en
  Windows, macOS y Linux.
- **Local:** el servidor escucha solo en `127.0.0.1`. La red se usa únicamente
  para consultar TMDB y Wikidata (opcional).

## Estado

El proyecto avanza por etapas (ver `docs/superpowers/specs/`):

| Etapa | Contenido | Estado |
|---|---|---|
| 1. Núcleo | escaneo, parser de nombres, versiones, catálogo SQLite, API | ✅ |
| 2. Datos técnicos | lectura de encabezados MKV/MP4/AVI/IFO, `ffprobe` opcional, mejor versión | ✅ |
| 3. Identificación | TMDB + Wikidata, puntaje de confianza, correcciones, afiches | ✅ |
| 4. Interfaz | interfaz Svelte (Inicio, Explorar, Ficha, Revisar, búsqueda) | pendiente |
| 5. Curaduría | listas, etiquetas, importación de `Collections/` | pendiente |

Por ahora la interfaz es una página mínima, pensada para revisar el catálogo.
Tiene tres pestañas:

- **Versiones:** cada versión con la película identificada, su calidad, audio,
  subtítulos, la marca **MEJOR** cuando tenés varias, y los botones ▶ Ver y
  Carpeta.
- **Sin identificar:** los casos dudosos, con candidatos para confirmar y
  búsqueda manual.
- **Copias idénticas:** archivos repetidos byte a byte.

## Requisitos

Para usarlo:

- Nada obligatorio: el ejecutable es autocontenido.
- **Token de TMDB** (gratuito), para identificar películas. Creá una cuenta en
  [themoviedb.org](https://www.themoviedb.org/), entrá a *Settings → API* y
  copiá el **API Read Access Token**: es el largo, empieza con `eyJ…`; no sirve
  la "API Key" corta. Sin token, la app funciona igual pero no identifica.
- **`ffprobe`** (opcional, parte de [FFmpeg](https://ffmpeg.org/)): si está en
  el `PATH`, se usa para los formatos que la app no lee por su cuenta (RMVB, MPG,
  WMV…) o cuando un encabezado no se puede leer. Si lo instalás después, hay que
  reiniciar la app; los archivos que habían fallado se vuelven a analizar solos.

Para compilarlo: Go 1.27 o posterior (ver [Desarrollo](#desarrollo)).

## Instalación

La disposición pensada es un disco (interno o externo) con las películas y, al
lado, la carpeta de la app:

```
<raíz del disco>/
├── cine/                       ← tus películas, organizadas como quieras
├── cine-ordenar/
└── cinexplorer/
    ├── cinexplorer-windows.exe
    ├── cinexplorer-macos       (binario universal Intel + Apple Silicon)
    ├── cinexplorer-linux
    ├── config.json             (se crea en el primer uso)
    ├── cinexplorer.db          (el catálogo, SQLite)
    └── cache/                  (afiches e imágenes de TMDB)
```

1. Copiá la carpeta `cinexplorer/` a la raíz del disco, al lado de tus carpetas
   de películas.
2. Ejecutá el binario de tu sistema:
   - Windows: `cinexplorer-windows.exe`
   - macOS: `cinexplorer-macos`. La primera vez, clic derecho → Abrir, porque
     no está firmado. Si macOS sigue bloqueándolo:
     `xattr -d com.apple.quarantine cinexplorer-macos`.
   - Linux: `./cinexplorer-linux` (puede hacer falta `chmod +x` antes).
3. Se abre el navegador con la app. En el primer uso se crea `config.json` con
   las carpetas hermanas como raíces (ignora las ocultas y las del sistema, como
   `$RECYCLE.BIN` o `System Volume Information`).
4. Cerrá la app, agregá tu token de TMDB en `config.json` y volvé a abrirla. El
   primer escaneo analiza todo; los siguientes solo lo que cambió.

Para salir, cerrá la ventana de la terminal (o Ctrl+C).

## Configuración

`config.json`, junto al ejecutable:

```json
{
  "roots": ["../cine", "../cine-ordenar"],
  "tmdbToken": "eyJ…",
  "language": "es-ES",
  "imagePrefetch": "none"
}
```

| Campo | Qué es |
|---|---|
| `roots` | Carpetas a escanear, **relativas a la carpeta de la app** y con `/` como separador. Una raíz que no está disponible (por ejemplo, un disco desenchufado) se saltea sin tocar lo que ya había en el catálogo. |
| `tmdbToken` | Token de lectura de TMDB (v4). Vacío: no se identifica. |
| `language` | Idioma de títulos, sinopsis y géneros, en formato TMDB (`es-ES`, `es-MX`, `en-US`, `pt-BR`…). Si lo cambiás, los datos se vuelven a pedir en el nuevo idioma. |
| `imagePrefetch` | `"none"` (por defecto): las imágenes se bajan al verlas por primera vez. `"posters"`: los afiches se bajan por adelantado. `"all"`: afiches y escenas por adelantado, para usar la app sin red; son varios cientos de MB para unas miles de películas. |

### Línea de comandos

| Opción | Descripción |
|---|---|
| `-dir <ruta>` | Carpeta de la app (config, catálogo, caché). Por defecto, la del ejecutable. También se puede definir con la variable `CINEXPLORER_DIR`. |
| `-port <n>` | Puerto HTTP. `0` (por defecto) elige uno libre. |
| `-no-browser` | No abrir el navegador al arrancar. |

## Cómo funciona

Cada escaneo corre en segundo plano (la página muestra el progreso) y es
incremental: un archivo solo se vuelve a procesar si cambió su tamaño o su
fecha de modificación.

1. **Escaneo:** recorre las raíces y clasifica por extensión (video, DVD,
   subtítulo, `.nfo`, basura conocida como `Thumbs.db` o `.DS_Store`).
2. **Huella de contenido:** un hash del primer y el último MB más el tamaño.
   Reconoce un archivo movido o renombrado y detecta copias idénticas. Los
   archivos vacíos (0 bytes) no tienen huella.
3. **Versiones:** agrupa partes (`CD1`/`CD2`, `Part 1`…), trata cada `VIDEO_TS`
   como una versión, asocia subtítulos externos (con idioma inferido del nombre)
   y separa extras (trailers, *making of*, archivos chicos).
4. **Nombres:** extrae título, año, director, país, resolución, origen y codec
   de nombres muy variados (`Amarcord [Federico Fellini, 1973]`,
   `Chinatown (Polanski, USA, 1974)`, `Arabian.Nights.1974.1080p.BluRay.x264-ADE`),
   y limpia el ruido de los sitios de descarga.
5. **Datos técnicos:** lee los encabezados de MKV/WebM, MP4/MOV, AVI e IFO de
   DVD sin herramientas externas (resolución, codecs, duración, pistas de audio
   y subtítulos con idioma). Solo lee encabezados, no el archivo entero.
6. **Identificación:** busca cada versión en TMDB y calcula un puntaje de
   confianza con título, año y director. Prueba variantes (el título entre
   paréntesis, el título en inglés, el nombre de la carpeta) y usa el IMDb id de
   un `.nfo` si lo hay. Por encima del umbral, la asignación es automática; si
   no, la versión va a **Sin identificar** con candidatos. En una muestra de la
   colección real, casi 8 de cada 10 se identifican solas, sin falsos
   positivos.
7. **Enriquecimiento:** datos de TMDB (títulos, directores, reparto, géneros,
   países, sinopsis, colección, afiches) y, desde Wikidata, lo que TMDB no
   tenga.

Cuando hay varias versiones de una misma película, se marca la **mejor**: mayor
resolución, luego el codec más moderno, luego el tamaño.

### Correcciones

Lo que decidís a mano (confirmar un candidato, "no es una película", "es un
extra de…") se guarda atado a la **huella** del archivo, no a su ruta: sobrevive
a mover o renombrar el archivo, y vale para todas sus copias idénticas. La
identificación automática nunca pisa una corrección.

### Sin red

El escaneo, los nombres y los datos técnicos no necesitan red. La
identificación queda pendiente y se reintenta sola: a los 5 minutos, y después
duplicando la espera hasta una vez por hora. También se reintenta con cada
escaneo nuevo.

### Modo consulta

Si la carpeta de la app no se puede escribir (por ejemplo, un disco NTFS en
macOS, que lo monta en solo lectura), la app arranca igual y muestra el
catálogo existente sin modificarlo. Las imágenes se muestran pero no se
guardan.

## Privacidad y red

- A **TMDB** se le envían los títulos y años extraídos de los nombres de
  archivo, IMDb ids y los ids de TMDB de las películas identificadas, con tu
  token.
- A **Wikidata** se le envían ids de TMDB (consultas SPARQL anónimas).
- Las imágenes se bajan de `image.tmdb.org`.
- No se envía nada más: ni rutas, ni nombres de carpeta completos, ni datos
  técnicos. No hay telemetría.
- El servidor solo atiende pedidos dirigidos a `127.0.0.1` / `localhost`, y
  rechaza los que vienen de páginas de otros sitios. Así, una web cualquiera no
  puede consultar tu catálogo desde tu navegador.
- El token de TMDB queda en texto plano en `config.json`: tenelo en cuenta si
  compartís el disco.

## Archivos que genera

Todo queda en la carpeta de la app; borrarlos no afecta a las películas.

- `cinexplorer.db`: el catálogo (SQLite, modo journal `DELETE`, más robusto
  ante una desconexión del disco que WAL). Si lo borrás, el próximo escaneo lo
  rearma desde cero, pero se pierden las correcciones manuales.
- `cache/posters/`, `cache/backdrops/`: imágenes de TMDB. Se pueden borrar;
  se vuelven a bajar.
- `config.json`: la configuración.

## API

La página usa una API JSON local; también sirve para scripts. Los `POST`
requieren `Content-Type: application/json`.

| Método y ruta | Qué hace |
|---|---|
| `GET /api/status` | Estado del escaneo y de la identificación. |
| `GET /api/versions` | Todas las versiones con archivos, datos técnicos, identificación y película. |
| `GET /api/unidentified` | Versiones sin identificar, con candidatos. |
| `GET /api/duplicates` | Grupos de copias idénticas. |
| `GET /api/tmdb/search?q=&year=` | Búsqueda manual en TMDB (`q` puede ser un IMDb id `tt…`). |
| `POST /api/identify` | Corrección: `{"fingerprint", "action": "movie"\|"extra"\|"ignore"\|"reset", "tmdbId"}`. |
| `POST /api/scan` | Lanza un escaneo. |
| `POST /api/open`, `POST /api/reveal` | Abre un archivo del catálogo con la aplicación del sistema, o muestra su carpeta. |
| `GET /img/{poster\|backdrop}/{tmdbId}.jpg` | Imágenes desde la caché. |

## Desarrollo

Requisitos: **Go 1.27+**. No hace falta cgo: SQLite es
[`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite), escrito en Go
puro, así que la compilación cruzada es directa. No hay otras dependencias.

```bash
go test ./...                          # suite completa (no usa red)
go run ./cmd/cinexplorer -dir .run     # corre la app con config y catálogo en .run/
bash scripts/build.sh                  # compila los tres binarios en dist/cinexplorer/
```

`scripts/build.sh` compila para Windows (amd64), Linux (amd64) y macOS (amd64 +
arm64), y une los dos binarios de macOS en uno universal con
[`makefat`](https://github.com/randall77/makefat), que se descarga con
`go run` y necesita red la primera vez. La CI (GitHub Actions) corre `go vet` y
`go test` en Windows, macOS y Linux.

Tests sobre datos reales (opcionales, fuera de CI):

```bash
# Compara los lectores propios contra ffprobe sobre una colección real
CINEXPLORER_PROBE_CORPUS=D:/cine go test ./internal/probe -run Corpus -v -timeout 0

# Regraba las respuestas de TMDB del corpus de calibración (30 nombres reales).
# Token en CINEXPLORER_TMDB_TOKEN o en ~/.cinexplorer-tmdb-token.
CINEXPLORER_TMDB_RECORD=1 go test ./internal/identify -run Corpus -v
```

### Estructura

```
cmd/cinexplorer/      punto de entrada: config, catálogo, escáner, identificación, servidor
internal/appdir       carpeta de la app y rutas relativas portables
internal/config       config.json
internal/scan         escaneo incremental y análisis técnico
internal/fingerprint  huella de contenido
internal/mediafile    clasificación de archivos por nombre
internal/grouping     armado de versiones (partes, DVD, subtítulos, extras)
internal/nameparse    parser de nombres de archivo y carpeta
internal/probe        lectores de encabezados MKV/MP4/AVI/IFO y ffprobe
internal/quality      mejor versión y normalización de títulos
internal/store        catálogo SQLite
internal/httpx        HTTP con límite de pedidos y reintentos
internal/tmdb         cliente de TMDB
internal/wikidata     cliente de Wikidata (SPARQL)
internal/images       caché de imágenes
internal/identify     puntaje, búsqueda, enriquecimiento y el proceso en segundo plano
internal/server       API JSON y página embebida (internal/server/web)
internal/platform     abrir archivos y carpetas con el sistema
```

El diseño y los planes de cada etapa están en `docs/superpowers/specs/` y
`docs/superpowers/plans/`.

## Problemas frecuentes

- **"Falta tmdbToken en config.json" / "El token de TMDB no es válido":**
  revisá que sea el *API Read Access Token* (v4), no la API Key corta.
- **"Sin conexión: se reintenta más tarde":** TMDB o Wikidata no respondieron.
  La app reintenta sola; mientras tanto, todo lo demás funciona.
- **Una película mal identificada o sin identificar:** confirmá el candidato
  correcto, o buscala a mano por título o por IMDb id (`tt0071129`), en la
  pestaña "Sin identificar".
- **Archivos que no aparecen:** revisá `roots` en `config.json`; las rutas son
  relativas a la carpeta de la app.
- **"Modo consulta (solo lectura)":** la carpeta de la app no se puede
  escribir. Ver [Modo consulta](#modo-consulta).

## Créditos

Los datos y las imágenes de películas vienen de [TMDB](https://www.themoviedb.org/).
This product uses the TMDB API but is not endorsed or certified by TMDB.

Los datos complementarios vienen de [Wikidata](https://www.wikidata.org/)
(CC0).
