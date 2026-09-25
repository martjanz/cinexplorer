# Cinexplorer — Etapa 2: Datos técnicos — Diseño

Fecha: 2026-09-25
Estado: aprobado en brainstorming
Spec general: `2026-09-25-cinexplorer-design.md` (§4 paso 4, §5.3)

## 1. Objetivo

Leer de los propios archivos de video los datos técnicos de cada versión (resolución, codecs, duración, pistas de audio y subtítulos internos con idioma), sin depender de herramientas externas, y marcar la **mejor versión** cuando hay varias de una misma película.

Referencia de la colección real (`D:\cine` + `D:\cine-ordenar`): 1102 `.avi`, 415 `.mp4`, 393 `.mkv`, ~13 `.rmvb`, ~10 `.mpg`, algunos DVD en `VIDEO_TS`. Los tres lectores nativos cubren ~98% de los videos.

## 2. Decisiones

| Tema | Decisión |
|---|---|
| Lectores nativos | MKV/WebM (EBML), MP4/M4V/MOV (cajas ISO BMFF), AVI (RIFF), DVD (`VTS_xx_0.IFO`) |
| `ffprobe` | Solo **fallback**: si el lector nativo falla o no hay lector para la extensión, y `ffprobe` está en el PATH. Nunca requisito. |
| Mejor versión | Función de comparación pura, aplicada ya con agrupación **provisoria** por título normalizado + año. En la Etapa 3 la clave pasa a ser el id TMDB. |
| Extras | No se analizan. |

## 3. Paquete `internal/probe`

```go
type Track struct {
    Codec    string // normalizado, ver §3.3
    Lang     string // ISO 639-1 ("" = desconocido)
    Channels int    // solo audio; 0 = desconocido
}

type Info struct {
    Container  string // "matroska", "mp4", "avi", "dvd", o el format_name de ffprobe
    DurationMs int64
    Width      int
    Height     int
    VideoCodec string
    Audio      []Track
    Subs       []Track
    Prober     string // "native" | "ffprobe"
}

func Probe(ctx context.Context, path string) (Info, error)
```

`Probe` elige el lector por extensión. Si el nativo devuelve error de formato o la extensión no tiene lector, intenta `ffprobe` (si `exec.LookPath("ffprobe")` lo encuentra), con timeout de 30 s. Los errores se distinguen:

- `ErrIO` (envuelve el error de apertura/lectura): transitorio → **no** se persiste, se reintenta en el próximo escaneo.
- `ErrUnsupported` / `ErrInvalid`: definitivo → se persiste con el mensaje; no se reintenta hasta que cambie tamaño o mtime.

Todos los lectores trabajan sobre `io.ReaderAt` + tamaño, leen solo encabezados y tienen un tope de bytes leídos (p. ej. 16 MiB para buscar `moov` o `Tracks`) para no recorrer archivos completos en un disco externo.

### 3.1 Lectores

- **MKV** (`mkv.go`): EBML header (DocType `matroska`/`webm`) → Segment → `SeekHead` (para saltar a `Info`/`Tracks` si no están al principio), `Info` (`TimecodeScale`, `Duration` float), `Tracks/TrackEntry` (`TrackType`, `CodecID`, `Language`, `LanguageIETF` con prioridad, `Video/PixelWidth|PixelHeight`, `Audio/Channels`). Idioma por defecto `eng` según la especificación Matroska cuando falta `Language`; `und` → "".
- **MP4** (`mp4.go`): recorre cajas de nivel superior saltando `mdat`; `moov/mvhd` (duración/timescale, versión 0 y 1), por `trak`: `tkhd` (ancho/alto 16.16), `mdia/hdlr` (`vide`, `soun`, `sbtl`, `text`, `subp`, `clcp`), `mdia/mdhd` (idioma ISO-639-2 empaquetado, `und` → ""), `minf/stbl/stsd` (fourcc de la primera entrada; canales de la entrada de audio). Soporta cajas de tamaño 64 bits y tamaño 0 (hasta fin de archivo).
- **AVI** (`avi.go`): `RIFF AVI ` → `LIST hdrl` → `avih` (µs por frame, frames totales, ancho, alto) → `LIST strl` por stream: `strh` (`vids`/`auds`/`txts`, handler, scale, rate, length), `strf` (`biCompression` de `BITMAPINFOHEADER`; `wFormatTag` y `nChannels` de `WAVEFORMATEX`). OpenDML: `LIST odml/dmlh` con frames totales tiene prioridad sobre `avih`. Duración = frames × µs/frame. Idioma de audio: "" (AVI no lo declara de forma estándar).
- **IFO** (`ifo.go`): firma `DVDVIDEO-VTS`; atributos de video en 0x200 (norma NTSC/PAL → 720×480 / 720×576, codec `mpeg2` o `mpeg1`), audio: cantidad en 0x202 y atributos de 8 bytes desde 0x204 (modo de codificación, código de idioma, canales), subpicture: cantidad en 0x254 y atributos de 6 bytes desde 0x256 (idioma). Duración: tiempo de reproducción BCD de la PGC más larga de `VTS_PGCIT` (sector en 0xCC). Para una versión DVD se analizan todos sus `VTS_xx_0.IFO` y gana el de mayor duración.
- **ffprobe** (`ffprobe.go`): `ffprobe -v error -print_format json -show_format -show_streams <archivo>`; mapea `codec_type`, `codec_name`, `width`, `height`, `channels`, `tags.language`, `format.duration`, `format.format_name`.

### 3.2 Resolución

`probe.ResolutionLabel(w, h int) string` clasifica por ancho **o** alto (así 1920×800 es 1080p):

| Condición | Etiqueta |
|---|---|
| w ≥ 3200 o h ≥ 2000 | `2160p` |
| w ≥ 1800 o h ≥ 1000 | `1080p` |
| w ≥ 1200 o h ≥ 700 | `720p` |
| h ≥ 540 | `576p` |
| h ≥ 400 | `480p` |
| h > 0 | `SD` |
| sin datos | "" |

Los mismos valores que produce `nameparse` (`2160p`, `1080p`, `720p`, `576p`, `480p`), para que ambos orígenes sean comparables.

### 3.3 Nombres normalizados

- Video: `av1`, `hevc`, `vp9`, `h264`, `vp8`, `mpeg4` (MPEG-4 ASP, incluye XviD/DivX/DX50/FMP4), `mpeg2`, `mpeg1`, `wmv`, `rv`, `theora`, `mjpeg`; desconocido → el código crudo en minúsculas.
- Audio: `aac`, `ac3`, `eac3`, `dts`, `truehd`, `flac`, `mp3`, `mp2`, `opus`, `vorbis`, `pcm`, `wma`.
- Subs: `srt`, `ass`, `pgs`, `vobsub`, `dvbsub`, `mov_text`, `webvtt`.
- Idiomas: tabla ISO 639-2 (B y T) → 639-1 para los idiomas comunes; lo desconocido → "".

## 4. Persistencia

Tabla nueva:

```sql
CREATE TABLE IF NOT EXISTS media (
  file_id     INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
  size        INTEGER NOT NULL,   -- tamaño y mtime del archivo al analizarlo
  mtime       INTEGER NOT NULL,
  prober      TEXT    NOT NULL DEFAULT '',
  error       TEXT    NOT NULL DEFAULT '',
  container   TEXT    NOT NULL DEFAULT '',
  duration_ms INTEGER NOT NULL DEFAULT 0,
  width       INTEGER NOT NULL DEFAULT 0,
  height      INTEGER NOT NULL DEFAULT 0,
  video_codec TEXT    NOT NULL DEFAULT '',
  audio       TEXT    NOT NULL DEFAULT '[]',  -- JSON []Track
  subs        TEXT    NOT NULL DEFAULT '[]'   -- JSON []Track
);
```

`CREATE TABLE IF NOT EXISTS` basta como migración: un catálogo de la Etapa 1 gana la tabla vacía al abrirse y el siguiente escaneo la llena.

Store:
- `PendingProbes() ([]ProbeTarget, error)`: archivos presentes con `role = 'main'` de kind `video`, más los `VTS_\d\d_0.IFO` de versiones DVD, que no tienen fila en `media` o cuyo `size`/`mtime` difiere del archivo.
- `SaveProbes([]ProbeResult) error`: upsert por lote en una transacción.

## 5. Pipeline

`Scanner.Run` suma una fase después de `ReplaceVersions`:

1. `PendingProbes()`.
2. Para cada objetivo: `probe.Probe(ctx, abs)`. `ErrIO` → se registra en el log y se omite. Otro error → resultado con `error`. Éxito → resultado con datos.
3. Guardado cada 50 resultados y al final, para que un corte deje lo avanzado.
4. Cancelable por `ctx`.

`scan.Status` suma `Probed` y `ToProbe`. La página muestra "Analizando… n/m".

## 6. Datos por versión y mejor versión

`store.Versions()` suma a `VersionView`:

| Campo JSON | Origen |
|---|---|
| `durationMs` | suma de las partes (en DVD: el IFO elegido) |
| `width`, `height`, `videoCodec` | parte 1 (en DVD: el IFO elegido) |
| `audio`, `subs` | pistas de todas las partes sin repetir (codec+idioma+canales), en orden de aparición |
| `resolution` | `ResolutionLabel` si hay datos del análisis; si no, la del nombre |
| `codec` | `videoCodec` si hay; si no, el del nombre pasado por `probe.CodecFromName` (`H.264`→`h264`, `H.265`→`hevc`, `XviD`/`DivX`→`mpeg4`, `AV1`→`av1`) |
| `best` | ver abajo |

**Mejor versión** (`internal/quality`):

- `quality.Compare(a, b Candidate) int` con `Candidate{Resolution, Codec string; Size int64}`: primero rango de resolución (`2160p` > `1080p` = `1080i` > `720p` > `576p` > `480p` > `SD` > ""), luego rango de codec (`av1` > `hevc` > `vp9` > `h264` > `vp8` > `mpeg4` > `wmv` > `rv` > `mpeg2` > `mpeg1` > otros > ""), luego tamaño.
- `quality.GroupKey(title string, year int) string`: título en minúsculas, sin tildes, sin puntuación ni artículos iniciales comunes (`the`, `el`, `la`, `los`, `las`, `le`, `les`, `il`, `lo`), espacios colapsados, más el año. Títulos vacíos no agrupan.
- En `Versions()`, entre las versiones con al menos un archivo principal presente, se agrupan por `GroupKey`; en cada grupo con 2 o más versiones, la de mayor `Compare` recibe `best = true`. Empate total → la de menor id.

## 7. API y página

- `GET /api/versions` devuelve los campos nuevos.
- `GET /api/status` incluye `probed` y `toProbe` dentro de `scan`.
- La página mínima muestra resolución, codec de video, duración (`1 h 52 min`), audio (`es 2.0 ac3, en 5.1 dts`), subs internos y la marca verde **MEJOR**.

## 8. Errores

- Archivo que desaparece o no se puede leer durante el análisis → `ErrIO`, sin fila, se reintenta.
- Encabezado corrupto → fila con `error`; la versión usa datos del nombre.
- `ffprobe` que cuelga → timeout de 30 s, se trata como error de formato.
- Disco desconectado a mitad de la fase → los lotes ya guardados quedan; el resto se retoma en el próximo escaneo.

## 9. Pruebas

- **Lectores**: archivos mínimos construidos en bytes dentro de los tests (helpers `ebml`, `box`, `riff`, `ifo`), sin binarios en el repo. Casos: pistas múltiples con idiomas, `SeekHead` hacia `Tracks` al final, `moov` al final, caja de 64 bits, OpenDML, encabezado truncado → `ErrInvalid`.
- **ffprobe**: parseo del JSON desde una salida grabada; la invocación real se prueba solo si `ffprobe` está en el PATH.
- **Resolución, codecs, idiomas, `Compare`, `GroupKey`**: tests de tabla.
- **Store**: `PendingProbes` (nuevo, sin cambios, cambiado, faltante, IFO), `SaveProbes`, agregación en `Versions()` y marca `best`.
- **Escaneo**: árbol sintético con un MKV y un AVI mínimos → versiones con datos técnicos; segundo escaneo sin cambios no re-analiza.
- **Corpus real** (opcional, fuera de CI): con `CINEXPLORER_PROBE_CORPUS=<dir>` y `ffprobe` disponible, compara lector nativo contra ffprobe (dimensiones exactas, duración ±2 s, codecs, cantidad de pistas) y reporta discrepancias.
