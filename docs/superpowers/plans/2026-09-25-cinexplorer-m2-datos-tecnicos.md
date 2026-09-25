# Cinexplorer — Etapa 2: Datos técnicos — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Leer de los encabezados de cada video su resolución, codecs, duración y pistas de audio/subtítulos (lectores propios de MKV, MP4, AVI e IFO de DVD, con `ffprobe` solo como respaldo), guardarlos en el catálogo y marcar la mejor versión cuando hay varias de una misma película.

**Architecture:** Paquete nuevo `internal/probe` con un lector por formato sobre un `io.ReaderAt` acotado (16 MiB por archivo) y despacho por firma de contenido; `internal/quality` compara versiones y define la identidad provisoria (título normalizado + año). El escaneo gana una segunda fase que analiza solo los archivos nuevos o cambiados y guarda por lotes en la tabla `media`; `store.Versions()` agrega esos datos por versión y marca `best`.

**Tech Stack:** Go (sin dependencias nuevas), `modernc.org/sqlite`, `ffprobe` opcional en el PATH.

Spec: `docs/superpowers/specs/2026-09-25-cinexplorer-m2-datos-tecnicos-design.md` (detalle de esta etapa) y `docs/superpowers/specs/2026-09-25-cinexplorer-design.md` §4 paso 4, §5.3.

**Nota sobre el código de este plan:** todo el código se prototipó y se probó antes de escribir el plan, incluida una comparación contra `ffprobe` sobre la colección real (`D:\cine` + `D:\cine-ordenar`). Copialo tal cual; si algo no compila o un test no da lo esperado, es un error del plan: reportalo en lugar de improvisar.

---

## Hoja de ruta (etapas)

| Etapa | Contenido | Estado |
|---|---|---|
| 1 — Núcleo local | escaneo, parser de nombres, versiones, SQLite, API mínima, build, CI | ✅ en `main` |
| **2 — Datos técnicos (este plan)** | lectores nativos MKV/MP4/AVI/IFO, fallback `ffprobe`, tabla `media`, mejor versión | |
| 3 — Identificación | TMDB + Wikidata, puntaje de confianza, películas, correcciones por huella | |
| 4 — Interfaz | Svelte + Vite, Inicio, Explorar, Ficha, búsqueda FTS5, primer uso | |
| 5 — Curaduría | Revisar, listas, etiquetas, importación de `Collections/` | |

---

## Estructura de archivos (Etapa 2)

```
internal/probe/probe.go            Info, Track, errores (ErrIO/ErrInvalid/ErrUnsupported), lector acotado
internal/probe/normalize.go        ResolutionLabel, CodecFromName, codecs, idiomas, FourCC, WAVEFORMATEX
internal/probe/mkv.go              lector Matroska/WebM (EBML)
internal/probe/mp4.go              lector MP4/MOV (cajas ISO BMFF, esds, dac3)
internal/probe/avi.go              lector AVI (RIFF hdrl, OpenDML)
internal/probe/ifo.go              lector DVD VTS_xx_0.IFO
internal/probe/ffprobe.go          invocación y parseo de ffprobe
internal/probe/prober.go           Prober, Probe, despacho por firma de contenido
internal/probe/probetest/probetest.go  constructores de archivos mínimos para tests
internal/probe/corpus_test.go      comparación opcional contra ffprobe sobre una colección real
internal/quality/quality.go        Compare (mejor versión) y GroupKey (identidad provisoria)
internal/store/schema.sql          + tabla media
internal/store/media.go            PendingProbes, SaveProbes
internal/store/technical.go        datos técnicos por versión y marca best
internal/store/store.go            VersionView con campos técnicos; Versions() los completa
internal/scan/scan.go              fase de análisis con progreso
internal/server/web/index.html     muestra duración, audio, subs internos, MEJOR y progreso de análisis
```

Dependencias nuevas entre paquetes: `store → {probe, quality}`, `scan → probe`. `probe` y `quality` no dependen de nada del proyecto. Sin ciclos.

Convenciones que ya usa el proyecto y hay que mantener: comentarios en inglés, mensajes de log y de UI en español, tests en el mismo paquete (`package x`, no `x_test`), commits en inglés con prefijo convencional.

En Windows con Git Bash, `go` debería estar en el PATH; si no: `export PATH="$PATH:/c/Program Files/Go/bin"`.

---

### Task 1: probe — tipos, errores, lector acotado y normalización

**Files:**
- Create: `internal/probe/probe.go`
- Create: `internal/probe/normalize.go`
- Test: `internal/probe/normalize_test.go`

- [ ] **Step 1: Test que falla**

`internal/probe/normalize_test.go`:

```go
package probe

import "testing"

func TestResolutionLabel(t *testing.T) {
	for _, c := range []struct {
		w, h int
		want string
	}{
		{3840, 2160, "2160p"}, {3840, 1600, "2160p"}, {1920, 1080, "1080p"}, {1920, 800, "1080p"},
		{1440, 1080, "1080p"}, {1280, 720, "720p"}, {1280, 544, "720p"}, {720, 576, "576p"},
		{1024, 576, "576p"}, {720, 480, "480p"}, {640, 480, "480p"}, {640, 272, "SD"}, {352, 240, "SD"},
		{0, 0, ""},
	} {
		if got := ResolutionLabel(c.w, c.h); got != c.want {
			t.Errorf("ResolutionLabel(%d, %d) = %q, want %q", c.w, c.h, got, c.want)
		}
	}
}

func TestCodecFromName(t *testing.T) {
	for in, want := range map[string]string{"H.264": "h264", "H.265": "hevc", "XviD": "mpeg4", "DivX": "mpeg4", "AV1": "av1", "": ""} {
		if got := CodecFromName(in); got != want {
			t.Errorf("CodecFromName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormLang(t *testing.T) {
	for in, want := range map[string]string{
		"spa": "es", "es": "es", "es-419": "es", "en_US": "en", "ger": "de", "deu": "de", "ENG": "en",
		"und": "", "": "", "xx": "", "tlh": "",
	} {
		if got := normLang(in); got != want {
			t.Errorf("normLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormFFCodec(t *testing.T) {
	for in, want := range map[string]string{
		"h264": "h264", "mpeg2video": "mpeg2", "msmpeg4v3": "mpeg4", "rv40": "rv", "pcm_s16le": "pcm",
		"subrip": "srt", "hdmv_pgs_subtitle": "pgs", "dvd_subtitle": "vobsub", "wmav2": "wma", "cook": "cook",
	} {
		if got := normFFCodec(in); got != want {
			t.Errorf("normFFCodec(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/probe/`
Expected: FAIL — `undefined: ResolutionLabel` (y los demás)

- [ ] **Step 3: Implementación**

`internal/probe/probe.go`:

```go
// Package probe reads technical data (resolution, codecs, duration, audio and
// subtitle tracks) from the headers of video files. Matroska, MP4, AVI and DVD
// IFO headers are read natively; ffprobe is an optional fallback.
package probe

import (
	"errors"
	"fmt"
	"io"
)

type Track struct {
	Codec    string `json:"codec"`
	Lang     string `json:"lang"`               // ISO 639-1, "" when unknown
	Channels int    `json:"channels,omitempty"` // audio only, 0 when unknown
}

type Info struct {
	Container  string
	DurationMs int64
	Width      int
	Height     int
	VideoCodec string
	Audio      []Track
	Subs       []Track
	Prober     string // "native" or "ffprobe"
}

var (
	// ErrIO wraps failures to open or read the file. They may be transient
	// (a flaky or unplugged drive), so callers retry them later.
	ErrIO = errors.New("probe: read error")
	// ErrInvalid means the header could not be parsed.
	ErrInvalid = errors.New("probe: invalid header")
	// ErrUnsupported means there is no reader for the format.
	ErrUnsupported = errors.New("probe: unsupported format")
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// maxRead caps the bytes a native reader may read from one file, so a
// malformed file never makes us walk a whole movie on an external drive.
const maxRead = 16 << 20

// source is a bounded random-access view of a file.
type source struct {
	r      io.ReaderAt
	size   int64
	budget int64
}

func newSource(r io.ReaderAt, size int64) *source {
	return &source{r: r, size: size, budget: maxRead}
}

// read returns exactly n bytes at off. Reading past the end of the file is a
// format error (a truncated or lying header); any other failure is ErrIO.
func (s *source) read(off int64, n int) ([]byte, error) {
	if off < 0 || n < 0 || off+int64(n) > s.size {
		return nil, invalid("read of %d bytes at %d past end of file (%d)", n, off, s.size)
	}
	if int64(n) > s.budget {
		return nil, invalid("header larger than %d bytes", maxRead)
	}
	s.budget -= int64(n)
	b := make([]byte, n)
	got, err := s.r.ReadAt(b, off)
	if got == n {
		return b, nil
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, invalid("truncated file")
	}
	return nil, fmt.Errorf("%w: %v", ErrIO, err)
}

// readUpTo reads min(n, size-off) bytes at off.
func (s *source) readUpTo(off int64, n int) ([]byte, error) {
	if rest := s.size - off; int64(n) > rest {
		n = int(max(rest, 0))
	}
	return s.read(off, n)
}
```

`internal/probe/normalize.go`:

```go
package probe

import "strings"

// ResolutionLabel classifies a frame size. Width counts as much as height, so
// a 1920×800 scope picture is still 1080p.
func ResolutionLabel(w, h int) string {
	switch {
	case w <= 0 && h <= 0:
		return ""
	case w >= 3200 || h >= 2000:
		return "2160p"
	case w >= 1800 || h >= 1000:
		return "1080p"
	case w >= 1200 || h >= 700:
		return "720p"
	case h >= 540:
		return "576p"
	case h >= 400:
		return "480p"
	}
	return "SD"
}

// CodecFromName maps the codec names produced by nameparse ("H.264", "XviD"…)
// to the normalized video codec names used by the readers.
func CodecFromName(s string) string {
	switch strings.ToLower(s) {
	case "":
		return ""
	case "h.264":
		return "h264"
	case "h.265":
		return "hevc"
	case "xvid", "divx":
		return "mpeg4"
	case "av1":
		return "av1"
	}
	return strings.ToLower(s)
}

// ffCodecs maps ffprobe codec_name values to normalized names. The native
// readers map their own identifiers onto the same set.
var ffCodecs = map[string]string{
	"h264": "h264", "hevc": "hevc", "av1": "av1", "vp9": "vp9", "vp8": "vp8",
	"mpeg4": "mpeg4", "msmpeg4v1": "mpeg4", "msmpeg4v2": "mpeg4", "msmpeg4v3": "mpeg4",
	"mpeg2video": "mpeg2", "mpeg1video": "mpeg1",
	"wmv1": "wmv", "wmv2": "wmv", "wmv3": "wmv", "vc1": "wmv",
	"rv10": "rv", "rv20": "rv", "rv30": "rv", "rv40": "rv",
	"theora": "theora", "mjpeg": "mjpeg",
	"aac": "aac", "ac3": "ac3", "eac3": "eac3", "dts": "dts", "truehd": "truehd", "flac": "flac",
	"mp3": "mp3", "mp2": "mp2", "opus": "opus", "vorbis": "vorbis",
	"wmav1": "wma", "wmav2": "wma", "wmapro": "wma",
	"subrip": "srt", "srt": "srt", "ass": "ass", "ssa": "ass", "hdmv_pgs_subtitle": "pgs",
	"dvd_subtitle": "vobsub", "dvb_subtitle": "dvbsub", "mov_text": "mov_text", "webvtt": "webvtt",
}

func normFFCodec(s string) string {
	s = strings.ToLower(s)
	if strings.HasPrefix(s, "pcm_") {
		return "pcm"
	}
	if n, ok := ffCodecs[s]; ok {
		return n
	}
	return s
}

// lang3 maps ISO 639-2 codes (bibliographic and terminology forms) to ISO 639-1.
var lang3 = map[string]string{
	"spa": "es", "eng": "en", "fre": "fr", "fra": "fr", "ger": "de", "deu": "de", "ita": "it",
	"por": "pt", "rus": "ru", "jpn": "ja", "chi": "zh", "zho": "zh", "kor": "ko", "dut": "nl",
	"nld": "nl", "swe": "sv", "dan": "da", "nor": "no", "nob": "no", "fin": "fi", "pol": "pl",
	"cze": "cs", "ces": "cs", "hun": "hu", "gre": "el", "ell": "el", "tur": "tr", "ara": "ar",
	"heb": "he", "hin": "hi", "per": "fa", "fas": "fa", "rum": "ro", "ron": "ro", "cat": "ca",
	"glg": "gl", "baq": "eu", "eus": "eu", "ukr": "uk", "bul": "bg", "hrv": "hr", "srp": "sr",
	"slv": "sl", "slo": "sk", "slk": "sk", "tha": "th", "vie": "vi", "ind": "id", "may": "ms",
	"msa": "ms", "ice": "is", "isl": "is", "lat": "la",
}

var lang1 = func() map[string]bool {
	m := map[string]bool{}
	for _, v := range lang3 {
		m[v] = true
	}
	return m
}()

// normLang turns "spa", "es", "es-419" or "en_US" into ISO 639-1; anything
// unknown (including "und") becomes "".
func normLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "-_"); i >= 0 {
		s = s[:i]
	}
	switch len(s) {
	case 2:
		if lang1[s] {
			return s
		}
	case 3:
		return lang3[s]
	}
	return ""
}

// fourccs maps AVI/VfW video FourCCs (lowercased) to normalized names.
var fourccs = map[string]string{
	"xvid": "mpeg4", "divx": "mpeg4", "dx50": "mpeg4", "fmp4": "mpeg4", "mp4v": "mpeg4",
	"3iv2": "mpeg4", "m4s2": "mpeg4", "xvix": "mpeg4", "dxgm": "mpeg4",
	"div3": "mpeg4", "div4": "mpeg4", "mp43": "mpeg4", "mp42": "mpeg4", "mpg4": "mpeg4",
	"h264": "h264", "x264": "h264", "avc1": "h264", "hevc": "hevc", "h265": "hevc", "hvc1": "hevc",
	"mpg2": "mpeg2", "mpg1": "mpeg1", "mjpg": "mjpeg", "wmv1": "wmv", "wmv2": "wmv", "wmv3": "wmv",
	"wvc1": "wmv", "vp80": "vp8", "vp90": "vp9", "av01": "av1",
}

func normFourCC(b []byte) string {
	if len(b) == 4 && b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0 {
		return "rawvideo"
	}
	s := strings.ToLower(strings.TrimRight(string(b), "\x00 "))
	if n, ok := fourccs[s]; ok {
		return n
	}
	return s
}

// wavFormat maps WAVEFORMATEX format tags to normalized audio codec names.
func wavFormat(tag uint16) string {
	switch tag {
	case 0x0001, 0x0003, 0xFFFE:
		return "pcm"
	case 0x0050:
		return "mp2"
	case 0x0055:
		return "mp3"
	case 0x2000:
		return "ac3"
	case 0x2001:
		return "dts"
	case 0x00FF, 0x1600, 0x1601, 0x706D:
		return "aac"
	case 0x0161, 0x0162, 0x0163:
		return "wma"
	case 0x674F, 0x6750, 0x6751, 0x676F, 0x6770, 0x6771:
		return "vorbis"
	case 0xF1AC:
		return "flac"
	}
	return ""
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/probe/ && go test ./internal/probe/ -v`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/probe
git commit -m "feat(probe): media info types, bounded reader and normalization"
```

---

### Task 2: probe — lector Matroska

**Files:**
- Create: `internal/probe/probetest/probetest.go` (constructores para tests, lo usan también `scan` en la Task 11)
- Create: `internal/probe/helpers_test.go`
- Create: `internal/probe/mkv.go`
- Test: `internal/probe/mkv_test.go`

Referencia del formato: elementos EBML = ID (varint que conserva sus bits de marca, 1–4 bytes) + tamaño (varint de 1–8 bytes sin la marca; todos los bits en 1 = tamaño desconocido) + datos. IDs usados: EBML `1A45DFA3` (DocType `4282`), Segment `18538067`, SeekHead `114D9B74` (Seek `4DBB`: SeekID `53AB`, SeekPosition `53AC`, relativa al inicio de los datos del Segment), Info `1549A966` (TimecodeScale `2AD7B1`, por defecto 1 000 000 ns; Duration `4489`, float en ticks), Tracks `1654AE6B` (TrackEntry `AE`: TrackType `83` 1=video 2=audio 17=subs, CodecID `86`, CodecPrivate `63A2`, Language `22B59C` por defecto `eng`, LanguageIETF `22B59D`, Video `E0` PixelWidth `B0` / PixelHeight `BA`, Audio `E1` Channels `9F` por defecto 1), Cluster `1F43B675`. Para AAC, los canales se toman del AudioSpecificConfig en `CodecPrivate` (en la colección real hay MKV sin `Channels`, que por defecto valdría 1, con audio estéreo).

- [ ] **Step 1: Test que falla**

`internal/probe/probetest/probetest.go`:

```go
// Package probetest builds minimal media files in memory for tests: EBML
// (Matroska), ISO BMFF boxes (MP4), RIFF chunks (AVI) and DVD IFO headers.
// It is only imported from tests.
package probetest

import (
	"bytes"
	"encoding/binary"
	"math"
)

func U16BE(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }
func U32BE(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
func U64BE(v uint64) []byte { return binary.BigEndian.AppendUint64(nil, v) }
func U16LE(v uint16) []byte { return binary.LittleEndian.AppendUint16(nil, v) }
func U32LE(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

// --- EBML (Matroska) ---

// ebmlID encodes an element ID; IDs already carry their length marker.
func ebmlID(id uint32) []byte {
	switch {
	case id > 0xFFFFFF:
		return U32BE(id)
	case id > 0xFFFF:
		return U32BE(id)[1:]
	case id > 0xFF:
		return U16BE(uint16(id))
	}
	return []byte{byte(id)}
}

// ebmlSize encodes a data size as an 8-byte EBML varint.
func ebmlSize(n int) []byte {
	b := U64BE(uint64(n))
	b[0] = 0x01
	return b
}

// EBML returns an element whose data is the concatenation of children.
func EBML(id uint32, children ...[]byte) []byte {
	data := cat(children...)
	return cat(ebmlID(id), ebmlSize(len(data)), data)
}

// EBMLUnknown returns a master element with the "unknown size" marker.
func EBMLUnknown(id uint32, children ...[]byte) []byte {
	return cat(ebmlID(id), []byte{0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, cat(children...))
}

func EBMLUint(id uint32, v uint64) []byte   { return EBML(id, U64BE(v)) }
func EBMLString(id uint32, s string) []byte { return EBML(id, []byte(s)) }
func EBMLFloat(id uint32, f float64) []byte { return EBML(id, U64BE(math.Float64bits(f))) }

// --- ISO BMFF (MP4) ---

// Box returns a box with a 32-bit size.
func Box(typ string, payload ...[]byte) []byte {
	data := cat(payload...)
	return cat(U32BE(uint32(8+len(data))), []byte(typ), data)
}

// Box64 returns a box using the 64-bit "largesize" form.
func Box64(typ string, payload ...[]byte) []byte {
	data := cat(payload...)
	return cat(U32BE(1), []byte(typ), U64BE(uint64(16+len(data))), data)
}

// FullBox returns a box whose payload starts with version and zero flags.
func FullBox(typ string, version byte, payload ...[]byte) []byte {
	return Box(typ, append([]byte{version, 0, 0, 0}, cat(payload...)...))
}

// --- RIFF (AVI) ---

// Chunk returns a RIFF chunk, padded to an even length.
func Chunk(id string, data []byte) []byte {
	out := cat([]byte(id), U32LE(uint32(len(data))), data)
	if len(data)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

func List(typ string, children ...[]byte) []byte {
	return Chunk("LIST", cat([]byte(typ), cat(children...)))
}

func RIFF(form string, children ...[]byte) []byte {
	return Chunk("RIFF", cat([]byte(form), cat(children...)))
}

// --- Ready-made files used outside the probe package ---

// MKV returns a minimal Matroska file with one H.264 video track of w×h, one
// AAC stereo audio track in lang and the given duration.
func MKV(w, h int, durationMs int64, lang string) []byte {
	return cat(
		EBML(0x1A45DFA3, EBMLString(0x4282, "matroska")),
		EBML(0x18538067,
			EBML(0x1549A966,
				EBMLUint(0x2AD7B1, 1000000),
				EBMLFloat(0x4489, float64(durationMs)),
			),
			EBML(0x1654AE6B,
				EBML(0xAE,
					EBMLUint(0x83, 1),
					EBMLString(0x86, "V_MPEG4/ISO/AVC"),
					EBML(0xE0, EBMLUint(0xB0, uint64(w)), EBMLUint(0xBA, uint64(h))),
				),
				EBML(0xAE,
					EBMLUint(0x83, 2),
					EBMLString(0x86, "A_AAC"),
					EBMLString(0x22B59C, lang),
					EBML(0xE1, EBMLUint(0x9F, 2)),
				),
			),
		),
	)
}
```

`internal/probe/helpers_test.go`:

```go
package probe

import "bytes"

// src wraps b as a source for the native readers.
func src(b []byte) *source { return newSource(bytes.NewReader(b), int64(len(b))) }

func zeros(n int) []byte { return make([]byte, n) }

func join(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
```

`internal/probe/mkv_test.go`:

```go
package probe

import (
	"errors"
	"reflect"
	"testing"

	pt "cinexplorer/internal/probe/probetest"
)

func TestMKVBasic(t *testing.T) {
	info, err := readMKV(src(pt.MKV(1920, 800, 6_300_000, "spa")))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "matroska", DurationMs: 6_300_000, Width: 1920, Height: 800, VideoCodec: "h264",
		Audio: []Track{{Codec: "aac", Lang: "es", Channels: 2}}}
	if !reflect.DeepEqual(info, want) {
		t.Fatalf("got  %+v\nwant %+v", info, want)
	}
}

func mkvHeader(docType string) []byte { return pt.EBML(mkvEBML, pt.EBMLString(mkvDocType, docType)) }

func track(typ uint64, codec string, extra ...[]byte) []byte {
	return pt.EBML(mkvTrackEntry, append([][]byte{pt.EBMLUint(mkvTrackType, typ), pt.EBMLString(mkvCodecID, codec)}, extra...)...)
}

func TestMKVTracksAfterClusterViaSeekHead(t *testing.T) {
	// BITMAPINFOHEADER with biCompression = XVID.
	bih := append(make([]byte, 16), []byte("XVID")...)
	tracks := pt.EBML(mkvTracks,
		track(1, "V_MS/VFW/FOURCC",
			pt.EBML(mkvCodecPrivate, bih),
			pt.EBML(mkvVideo, pt.EBMLUint(mkvPixelWidth, 640), pt.EBMLUint(mkvPixelHeight, 272))),
		track(2, "A_AC3", pt.EBMLString(mkvLanguage, "ita"), pt.EBML(mkvAudio, pt.EBMLUint(mkvChannels, 6))),
		track(2, "A_MPEG/L3"), // no Language nor Channels: Matroska defaults are English, mono
		track(17, "S_TEXT/UTF8", pt.EBMLString(mkvLanguage, "spa"), pt.EBMLString(mkvLanguageIETF, "es-419")),
		track(17, "S_VOBSUB", pt.EBMLString(mkvLanguage, "und")),
	)
	info := pt.EBML(mkvInfo, pt.EBMLFloat(mkvDuration, 90_000)) // default TimecodeScale: 1 ms
	cluster := pt.EBML(mkvCluster, make([]byte, 100))
	seekHead := func(pos uint64) []byte {
		return pt.EBML(mkvSeekHead, pt.EBML(mkvSeek,
			pt.EBML(mkvSeekID, pt.U32BE(mkvTracks)), pt.EBMLUint(mkvSeekPosition, pos)))
	}
	pos := uint64(len(seekHead(0)) + len(info) + len(cluster))
	file := append(mkvHeader("webm"), pt.EBML(mkvSegment, seekHead(pos), info, cluster, tracks)...)

	got, err := readMKV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "webm", DurationMs: 90_000, Width: 640, Height: 272, VideoCodec: "mpeg4",
		Audio: []Track{{Codec: "ac3", Lang: "it", Channels: 6}, {Codec: "mp3", Lang: "en", Channels: 1}},
		Subs:  []Track{{Codec: "srt", Lang: "es"}, {Codec: "vobsub", Lang: ""}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestMKVAACChannelsFromCodecPrivate(t *testing.T) {
	// No Channels element (default 1), but the AudioSpecificConfig
	// (AAC LC, 48 kHz, stereo) says 2.
	file := append(mkvHeader("matroska"), pt.EBML(mkvSegment,
		pt.EBML(mkvTracks, track(2, "A_AAC", pt.EBML(mkvCodecPrivate, []byte{0x11, 0x90}))),
	)...)
	got, err := readMKV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Audio, []Track{{Codec: "aac", Lang: "en", Channels: 2}}) {
		t.Fatalf("audio %+v", got.Audio)
	}
}

func TestMKVUnknownSizeSegmentAndTimecodeScale(t *testing.T) {
	file := append(mkvHeader("matroska"), pt.EBMLUnknown(mkvSegment,
		pt.EBML(mkvInfo, pt.EBMLUint(mkvTimecodeScale, 1_000_000_000), pt.EBMLFloat(mkvDuration, 5400)),
		pt.EBML(mkvTracks, track(1, "V_MPEGH/ISO/HEVC",
			pt.EBML(mkvVideo, pt.EBMLUint(mkvPixelWidth, 3840), pt.EBMLUint(mkvPixelHeight, 1600)))),
	)...)
	got, err := readMKV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMs != 5_400_000 || got.VideoCodec != "hevc" || got.Width != 3840 {
		t.Fatalf("got %+v", got)
	}
}

func TestMKVInvalid(t *testing.T) {
	good := pt.MKV(1280, 720, 1000, "eng")
	for name, b := range map[string][]byte{
		"not ebml":   []byte("RIFF\x00\x00\x00\x00AVI LIST"),
		"truncated":  good[:len(good)-10],
		"bad doc":    append(mkvHeader("foo"), pt.EBML(mkvSegment)...),
		"no tracks":  append(mkvHeader("matroska"), pt.EBML(mkvSegment, pt.EBML(mkvInfo))...),
		"empty file": {},
	} {
		if _, err := readMKV(src(b)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/probe/`
Expected: FAIL — `undefined: readMKV` (y las constantes `mkv…`)

- [ ] **Step 3: Implementación**

`internal/probe/mkv.go`:

```go
package probe

import (
	"encoding/binary"
	"math"
	"strings"
)

const (
	mkvEBML          = 0x1A45DFA3
	mkvDocType       = 0x4282
	mkvSegment       = 0x18538067
	mkvSeekHead      = 0x114D9B74
	mkvSeek          = 0x4DBB
	mkvSeekID        = 0x53AB
	mkvSeekPosition  = 0x53AC
	mkvInfo          = 0x1549A966
	mkvTimecodeScale = 0x2AD7B1
	mkvDuration      = 0x4489
	mkvTracks        = 0x1654AE6B
	mkvTrackEntry    = 0xAE
	mkvTrackType     = 0x83
	mkvCodecID       = 0x86
	mkvCodecPrivate  = 0x63A2
	mkvLanguage      = 0x22B59C
	mkvLanguageIETF  = 0x22B59D
	mkvVideo         = 0xE0
	mkvPixelWidth    = 0xB0
	mkvPixelHeight   = 0xBA
	mkvAudio         = 0xE1
	mkvChannels      = 0x9F
	mkvCluster       = 0x1F43B675
)

const unknownSize = -1

type ebmlElem struct {
	id      uint32
	dataOff int64
	size    int64 // unknownSize when the element does not declare it
}

func (e ebmlElem) end(parentEnd int64) int64 {
	if e.size == unknownSize {
		return parentEnd
	}
	return e.dataOff + e.size
}

// readElem reads the element header at off; limit is the end of the parent.
func readElem(s *source, off, limit int64) (ebmlElem, error) {
	b, err := s.readUpTo(off, 12)
	if err != nil {
		return ebmlElem{}, err
	}
	if len(b) == 0 || b[0] == 0 {
		return ebmlElem{}, invalid("bad EBML id at %d", off)
	}
	idLen := vintLen(b[0])
	if idLen > 4 || idLen > len(b) {
		return ebmlElem{}, invalid("bad EBML id at %d", off)
	}
	var id uint32
	for _, c := range b[:idLen] {
		id = id<<8 | uint32(c)
	}
	rest := b[idLen:]
	if len(rest) == 0 || rest[0] == 0 {
		return ebmlElem{}, invalid("bad EBML size at %d", off)
	}
	sizeLen := vintLen(rest[0])
	if sizeLen > len(rest) {
		return ebmlElem{}, invalid("bad EBML size at %d", off)
	}
	size := uint64(rest[0]) & (0xFF >> sizeLen)
	allOnes := size == 0xFF>>sizeLen
	for _, c := range rest[1:sizeLen] {
		size = size<<8 | uint64(c)
		allOnes = allOnes && c == 0xFF
	}
	e := ebmlElem{id: id, dataOff: off + int64(idLen+sizeLen), size: int64(size)}
	if allOnes {
		e.size = unknownSize
	} else if size > uint64(limit-e.dataOff) || e.dataOff > limit {
		return ebmlElem{}, invalid("EBML element at %d overflows its parent", off)
	}
	return e, nil
}

// vintLen returns the length of an EBML varint from its first byte.
func vintLen(b byte) int {
	n := 1
	for mask := byte(0x80); mask != 0 && b&mask == 0; mask >>= 1 {
		n++
	}
	return n
}

// children calls fn for each child of the element spanning [off, end).
// fn returns false to stop early.
func children(s *source, off, end int64, fn func(ebmlElem) (bool, error)) error {
	for off < end {
		e, err := readElem(s, off, end)
		if err != nil {
			return err
		}
		more, err := fn(e)
		if err != nil || !more {
			return err
		}
		if e.size == unknownSize {
			return nil // cannot skip it
		}
		off = e.dataOff + e.size
	}
	return nil
}

func (s *source) data(e ebmlElem) ([]byte, error) {
	if e.size == unknownSize || e.size > 1<<20 {
		return nil, invalid("EBML value too large")
	}
	return s.read(e.dataOff, int(e.size))
}

func (s *source) uintElem(e ebmlElem) (uint64, error) {
	b, err := s.data(e)
	if err != nil || len(b) > 8 {
		return 0, invalid("bad EBML uint")
	}
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v, nil
}

func (s *source) floatElem(e ebmlElem) (float64, error) {
	b, err := s.data(e)
	if err != nil {
		return 0, err
	}
	switch len(b) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b))), nil
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(b)), nil
	}
	return 0, invalid("bad EBML float")
}

func (s *source) stringElem(e ebmlElem) (string, error) {
	b, err := s.data(e)
	return strings.TrimRight(string(b), "\x00"), err
}

func readMKV(s *source) (Info, error) {
	head, err := readElem(s, 0, s.size)
	if err != nil {
		return Info{}, err
	}
	if head.id != mkvEBML {
		return Info{}, invalid("not an EBML file")
	}
	docType := "matroska"
	err = children(s, head.dataOff, head.end(s.size), func(e ebmlElem) (bool, error) {
		var err error
		if e.id == mkvDocType {
			docType, err = s.stringElem(e)
		}
		return true, err
	})
	if err != nil {
		return Info{}, err
	}
	if docType != "matroska" && docType != "webm" {
		return Info{}, invalid("EBML doctype %q", docType)
	}

	seg, err := readElem(s, head.end(s.size), math.MaxInt64) // may claim more than a truncated file holds
	if err != nil {
		return Info{}, err
	}
	if seg.id != mkvSegment {
		return Info{}, invalid("no Matroska segment")
	}
	segEnd := min(seg.end(s.size), s.size)

	info := Info{Container: docType}
	var gotInfo, gotTracks bool
	seeks := map[uint32]int64{}
	parse := func(e ebmlElem) (bool, error) {
		var err error
		switch e.id {
		case mkvInfo:
			err = s.mkvInfo(e, &info)
			gotInfo = true
		case mkvTracks:
			err = s.mkvTracks(e, segEnd, &info)
			gotTracks = true
		case mkvSeekHead:
			err = s.mkvSeekHead(e, seg.dataOff, seeks)
		case mkvCluster:
			return false, nil // media data starts: rely on SeekHead from here
		}
		return !(gotInfo && gotTracks), err
	}
	if err := children(s, seg.dataOff, segEnd, parse); err != nil {
		return Info{}, err
	}
	for _, want := range []struct {
		id  uint32
		got *bool
	}{{mkvInfo, &gotInfo}, {mkvTracks, &gotTracks}} {
		pos, ok := seeks[want.id]
		if *want.got || !ok {
			continue
		}
		e, err := readElem(s, pos, segEnd)
		if err != nil {
			return Info{}, err
		}
		if e.id != want.id {
			return Info{}, invalid("SeekHead points to the wrong element")
		}
		if _, err := parse(e); err != nil {
			return Info{}, err
		}
	}
	if !gotTracks {
		return Info{}, invalid("no Matroska Tracks element")
	}
	return info, nil
}

func (s *source) mkvSeekHead(e ebmlElem, segData int64, seeks map[uint32]int64) error {
	return children(s, e.dataOff, e.dataOff+e.size, func(seek ebmlElem) (bool, error) {
		if seek.id != mkvSeek {
			return true, nil
		}
		var id uint32
		var pos int64 = -1
		err := children(s, seek.dataOff, seek.dataOff+seek.size, func(c ebmlElem) (bool, error) {
			switch c.id {
			case mkvSeekID:
				b, err := s.data(c)
				if err != nil {
					return false, err
				}
				for _, x := range b {
					id = id<<8 | uint32(x)
				}
			case mkvSeekPosition:
				v, err := s.uintElem(c)
				if err != nil {
					return false, err
				}
				pos = segData + int64(v)
			}
			return true, nil
		})
		if err == nil && pos >= 0 {
			seeks[id] = pos
		}
		return true, err
	})
}

func (s *source) mkvInfo(e ebmlElem, info *Info) error {
	scale := uint64(1000000)
	var dur float64
	err := children(s, e.dataOff, e.dataOff+e.size, func(c ebmlElem) (bool, error) {
		var err error
		switch c.id {
		case mkvTimecodeScale:
			scale, err = s.uintElem(c)
		case mkvDuration:
			dur, err = s.floatElem(c)
		}
		return true, err
	})
	info.DurationMs = int64(dur * float64(scale) / 1e6)
	return err
}

func (s *source) mkvTracks(e ebmlElem, segEnd int64, info *Info) error {
	return children(s, e.dataOff, e.end(segEnd), func(t ebmlElem) (bool, error) {
		if t.id != mkvTrackEntry {
			return true, nil
		}
		var typ, w, h uint64
		ch := uint64(1) // Matroska default when Channels is absent
		var codec, lang, ietf string
		var private []byte
		lang = "eng" // Matroska default when Language is absent
		err := children(s, t.dataOff, t.dataOff+t.size, func(c ebmlElem) (bool, error) {
			var err error
			switch c.id {
			case mkvTrackType:
				typ, err = s.uintElem(c)
			case mkvCodecID:
				codec, err = s.stringElem(c)
			case mkvCodecPrivate:
				if c.size <= 64 {
					private, err = s.data(c)
				} else {
					private, err = s.read(c.dataOff, 64)
				}
			case mkvLanguage:
				lang, err = s.stringElem(c)
			case mkvLanguageIETF:
				ietf, err = s.stringElem(c)
			case mkvVideo:
				err = children(s, c.dataOff, c.dataOff+c.size, func(v ebmlElem) (bool, error) {
					var err error
					switch v.id {
					case mkvPixelWidth:
						w, err = s.uintElem(v)
					case mkvPixelHeight:
						h, err = s.uintElem(v)
					}
					return true, err
				})
			case mkvAudio:
				err = children(s, c.dataOff, c.dataOff+c.size, func(a ebmlElem) (bool, error) {
					var err error
					if a.id == mkvChannels {
						ch, err = s.uintElem(a)
					}
					return true, err
				})
			}
			return true, err
		})
		if err != nil {
			return false, err
		}
		if ietf != "" {
			lang = ietf
		}
		switch typ {
		case 1:
			if info.VideoCodec == "" {
				info.VideoCodec = mkvVideoCodec(codec, private)
				info.Width, info.Height = int(w), int(h)
			}
		case 2:
			a := Track{Codec: mkvAudioCodec(codec, private), Lang: normLang(lang), Channels: int(ch)}
			if a.Codec == "aac" && len(private) >= 2 {
				// The AudioSpecificConfig is authoritative; Channels is
				// often missing and then defaults to 1.
				if n := aacChannels(private); n > 0 {
					a.Channels = n
				}
			}
			info.Audio = append(info.Audio, a)
		case 17:
			info.Subs = append(info.Subs, Track{Codec: mkvSubCodec(codec), Lang: normLang(lang)})
		}
		return true, nil
	})
}

func mkvVideoCodec(id string, private []byte) string {
	switch {
	case id == "V_MPEG4/ISO/AVC":
		return "h264"
	case id == "V_MPEGH/ISO/HEVC":
		return "hevc"
	case id == "V_AV1":
		return "av1"
	case id == "V_VP9":
		return "vp9"
	case id == "V_VP8":
		return "vp8"
	case strings.HasPrefix(id, "V_MPEG4/ISO/"), strings.HasPrefix(id, "V_MPEG4/MS/"):
		return "mpeg4"
	case id == "V_MPEG2":
		return "mpeg2"
	case id == "V_MPEG1":
		return "mpeg1"
	case strings.HasPrefix(id, "V_REAL/"):
		return "rv"
	case id == "V_THEORA":
		return "theora"
	case id == "V_MJPEG":
		return "mjpeg"
	case id == "V_MS/VFW/FOURCC" && len(private) >= 20:
		return normFourCC(private[16:20]) // BITMAPINFOHEADER.biCompression
	}
	return strings.ToLower(strings.TrimPrefix(id, "V_"))
}

func mkvAudioCodec(id string, private []byte) string {
	switch {
	case strings.HasPrefix(id, "A_AAC"):
		return "aac"
	case id == "A_AC3":
		return "ac3"
	case id == "A_EAC3":
		return "eac3"
	case strings.HasPrefix(id, "A_DTS"):
		return "dts"
	case id == "A_TRUEHD":
		return "truehd"
	case id == "A_FLAC":
		return "flac"
	case id == "A_MPEG/L3":
		return "mp3"
	case id == "A_MPEG/L2":
		return "mp2"
	case id == "A_OPUS":
		return "opus"
	case id == "A_VORBIS":
		return "vorbis"
	case strings.HasPrefix(id, "A_PCM/"):
		return "pcm"
	case id == "A_MS/ACM" && len(private) >= 2:
		return wavFormat(binary.LittleEndian.Uint16(private))
	}
	return strings.ToLower(strings.TrimPrefix(id, "A_"))
}

func mkvSubCodec(id string) string {
	switch id {
	case "S_TEXT/UTF8", "S_TEXT/ASCII":
		return "srt"
	case "S_TEXT/SSA", "S_TEXT/ASS", "S_SSA", "S_ASS":
		return "ass"
	case "S_HDMV/PGS":
		return "pgs"
	case "S_VOBSUB":
		return "vobsub"
	case "S_DVBSUB":
		return "dvbsub"
	case "S_TEXT/WEBVTT":
		return "webvtt"
	}
	return strings.ToLower(strings.TrimPrefix(id, "S_"))
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/probe/... && go test ./internal/probe/ -v -run MKV`
Expected: PASS (5 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/probe
git commit -m "feat(probe): native Matroska header reader"
```

---

### Task 3: probe — lector MP4/MOV

**Files:**
- Create: `internal/probe/mp4.go`
- Test: `internal/probe/mp4_test.go`

Referencia: caja = tamaño uint32 BE + tipo (4 bytes) + datos; tamaño 1 = tamaño uint64 a continuación; tamaño 0 = hasta el fin del padre. Se recorren las cajas de primer nivel saltando `mdat` (solo se leen encabezados) hasta `moov`. Offsets dentro de los datos de cada caja (después de las 8/16 bytes de encabezado):

| Caja | versión 0 | versión 1 |
|---|---|---|
| `mvhd` | timescale @12, duration u32 @16 | timescale @20, duration u64 @24 |
| `tkhd` | track_ID @12 | track_ID @20 |
| `mdhd` | language u16 @20 | language u16 @32 |
| `hdlr` | handler_type @8 | — |
| `stsd` | entry_count @4, primera entrada @8 | — |

En la entrada de muestra (datos después de su encabezado de 8 bytes): video → ancho u16 @24, alto @26, cajas hijas desde @78; audio → versión u16 @8, canales u16 @16, cajas hijas desde @28 (v0), @44 (v1 QuickTime), @64 (v2). Idioma empaquetado: 3 letras de 5 bits + 0x60; valores < 0x400 son códigos Macintosh viejos (se ignoran).

Lo que enseñó la colección real y cubren los tests: los MOV/MP4 de QuickTime tienen un segundo `hdlr` dentro de `minf` que no es el de la pista; `stsd` suele decir 2 canales aunque el AAC sea mono o 5.1 (el dato real está en el AudioSpecificConfig de `esds`) y lo mismo con AC3 (dato real en `dac3`: `acmod` + `lfeon`).

- [ ] **Step 1: Test que falla**

`internal/probe/mp4_test.go`:

```go
package probe

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	pt "cinexplorer/internal/probe/probetest"
)

func packLang(l string) []byte {
	return pt.U16BE(uint16(l[0]-0x60)<<10 | uint16(l[1]-0x60)<<5 | uint16(l[2]-0x60))
}

func mp4Trak(id uint32, handler, lang string, entry []byte, extra ...[]byte) []byte {
	return pt.Box("trak", append([][]byte{
		pt.FullBox("tkhd", 0, zeros(8), pt.U32BE(id), zeros(68)),
		pt.Box("mdia",
			pt.FullBox("mdhd", 0, zeros(16), packLang(lang), zeros(2)),
			pt.FullBox("hdlr", 0, zeros(4), []byte(handler), zeros(12)),
			pt.Box("minf", pt.Box("stbl", pt.FullBox("stsd", 0, pt.U32BE(1), entry))),
		),
	}, extra...)...)
}

func videoEntry(fourcc string, w, h uint16, children ...[]byte) []byte {
	return pt.Box(fourcc, zeros(24), pt.U16BE(w), pt.U16BE(h), zeros(50), bytes.Join(children, nil))
}

func audioEntry(fourcc string, ch uint16, children ...[]byte) []byte {
	return pt.Box(fourcc, zeros(16), pt.U16BE(ch), zeros(10), bytes.Join(children, nil))
}

// esds with an ES_Descriptor holding a DecoderConfigDescriptor.
func esds(objectType byte) []byte {
	return pt.FullBox("esds", 0, []byte{0x03, 0x80, 0x80, 0x80, 0x19, 0x00, 0x01, 0x00, 0x04, 0x11, objectType, 0x15})
}

func TestMP4FastStart(t *testing.T) {
	file := bytes.Join([][]byte{
		pt.Box("ftyp", []byte("isom"), zeros(4)),
		pt.Box("moov",
			pt.FullBox("mvhd", 0, zeros(8), pt.U32BE(1000), pt.U32BE(7_200_000)),
			mp4Trak(1, "vide", "und", videoEntry("avc1", 1920, 1080), pt.Box("tref", pt.Box("chap", pt.U32BE(5)))),
			mp4Trak(2, "soun", "spa", audioEntry("mp4a", 6, esds(0x40))),
			mp4Trak(3, "soun", "eng", audioEntry("mp4a", 2, esds(0x6B))),
			mp4Trak(4, "sbtl", "spa", pt.Box("tx3g", zeros(8))),
			mp4Trak(5, "text", "eng", pt.Box("text", zeros(8))), // chapter list, not a subtitle
		),
		pt.Box("mdat", zeros(64)),
	}, nil)
	got, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "mp4", DurationMs: 7_200_000, Width: 1920, Height: 1080, VideoCodec: "h264",
		Audio: []Track{{Codec: "aac", Lang: "es", Channels: 6}, {Codec: "mp3", Lang: "en", Channels: 2}},
		Subs:  []Track{{Codec: "mov_text", Lang: "es"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestMP4MoovAtEndAfterLargeMdat(t *testing.T) {
	file := bytes.Join([][]byte{
		pt.Box("ftyp", []byte("mp42"), zeros(4)),
		pt.Box64("mdat", zeros(1000)),
		pt.Box("moov",
			pt.FullBox("mvhd", 1, zeros(16), pt.U32BE(600), pt.U64BE(600*5400)),
			mp4Trak(1, "vide", "eng", videoEntry("hvc1", 3840, 2160)),
			mp4Trak(2, "soun", "fre", audioEntry("ac-3", 6)),
		),
	}, nil)
	got, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMs != 5_400_000 || got.VideoCodec != "hevc" || got.Height != 2160 ||
		!reflect.DeepEqual(got.Audio, []Track{{Codec: "ac3", Lang: "fr", Channels: 6}}) {
		t.Fatalf("got %+v", got)
	}
}

func TestMP4Invalid(t *testing.T) {
	noMoov := bytes.Join([][]byte{pt.Box("ftyp", zeros(8)), pt.Box("mdat", zeros(8))}, nil)
	truncated := bytes.Join([][]byte{pt.Box("ftyp", zeros(8)), pt.Box("moov", zeros(100))}, nil)[:60]
	for name, b := range map[string][]byte{
		"no moov":   noMoov,
		"truncated": truncated,
		"binary":    {0x1A, 0x45, 0xDF, 0xA3, 0, 0, 0, 0, 1, 2, 3},
		"empty":     {},
	} {
		if _, err := readMP4(src(b)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestMP4QuickTimeHandlerAndAC3Channels(t *testing.T) {
	// QuickTime puts a data-reference hdlr ("alis") inside minf; the media
	// handler is the one in mdia. dac3: acmod 3/2 + LFE = 5.1.
	trak := pt.Box("trak",
		pt.FullBox("tkhd", 0, zeros(8), pt.U32BE(1), zeros(68)),
		pt.Box("mdia",
			pt.FullBox("mdhd", 0, zeros(16), packLang("eng"), zeros(2)),
			pt.FullBox("hdlr", 0, zeros(4), []byte("soun"), zeros(12)),
			pt.Box("minf",
				pt.FullBox("hdlr", 0, zeros(4), []byte("alis"), zeros(12)),
				pt.Box("stbl", pt.FullBox("stsd", 0, pt.U32BE(1),
					audioEntry("ac-3", 2, pt.Box("dac3", []byte{0x10, 0x3C, 0x00})))),
			),
		),
	)
	file := join(pt.Box("ftyp", []byte("qt  "), zeros(4)), pt.Box("moov", pt.FullBox("mvhd", 0, zeros(8), pt.U32BE(600), pt.U32BE(600)), trak))
	got, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Audio, []Track{{Codec: "ac3", Lang: "en", Channels: 6}}) {
		t.Fatalf("audio %+v", got.Audio)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/probe/`
Expected: FAIL — `undefined: readMP4`

- [ ] **Step 3: Implementación**

`internal/probe/mp4.go`:

```go
package probe

import (
	"encoding/binary"
	"strings"
)

type mp4Box struct {
	typ     string
	dataOff int64
	end     int64
}

// readBox reads the box header at off; limit is the end of the parent.
func readBox(s *source, off, limit int64) (mp4Box, error) {
	h, err := s.read(off, 8)
	if err != nil {
		return mp4Box{}, err
	}
	size := int64(binary.BigEndian.Uint32(h))
	b := mp4Box{typ: string(h[4:8]), dataOff: off + 8}
	switch size {
	case 0: // extends to the end of the parent
		b.end = limit
	case 1:
		ext, err := s.read(off+8, 8)
		if err != nil {
			return mp4Box{}, err
		}
		b.dataOff = off + 16
		b.end = off + int64(binary.BigEndian.Uint64(ext))
	default:
		b.end = off + size
	}
	if b.end < b.dataOff || b.end > limit {
		return mp4Box{}, invalid("MP4 box %q at %d overflows its parent", b.typ, off)
	}
	return b, nil
}

// boxes calls fn for each box in [off, end). fn returns false to stop early.
func boxes(s *source, off, end int64, fn func(mp4Box) (bool, error)) error {
	for off+8 <= end {
		b, err := readBox(s, off, end)
		if err != nil {
			return err
		}
		more, err := fn(b)
		if err != nil || !more {
			return err
		}
		off = b.end
	}
	return nil
}

func (s *source) boxData(b mp4Box, max int) ([]byte, error) {
	n := b.end - b.dataOff
	if n > int64(max) {
		n = int64(max)
	}
	return s.read(b.dataOff, int(n))
}

type mp4Track struct {
	id      uint32
	handler string
	lang    string
	codec   string
	w, h    int
	ch      int
	chapter []uint32 // track ids referenced as chapter lists
}

func readMP4(s *source) (Info, error) {
	first, err := readBox(s, 0, s.size)
	if err != nil {
		return Info{}, err
	}
	if !isFourCC(first.typ) {
		return Info{}, invalid("not an MP4 file")
	}
	var moov *mp4Box
	err = boxes(s, 0, s.size, func(b mp4Box) (bool, error) {
		if b.typ == "moov" {
			moov = &b
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return Info{}, err
	}
	if moov == nil {
		return Info{}, invalid("MP4 without moov")
	}

	info := Info{Container: "mp4"}
	var tracks []mp4Track
	err = boxes(s, moov.dataOff, moov.end, func(b mp4Box) (bool, error) {
		switch b.typ {
		case "mvhd":
			d, err := s.boxData(b, 32)
			if err != nil {
				return false, err
			}
			var scale, dur uint64
			if len(d) >= 32 && d[0] == 1 {
				scale, dur = uint64(binary.BigEndian.Uint32(d[20:])), binary.BigEndian.Uint64(d[24:])
			} else if len(d) >= 20 {
				scale, dur = uint64(binary.BigEndian.Uint32(d[12:])), uint64(binary.BigEndian.Uint32(d[16:]))
			}
			if scale > 0 {
				info.DurationMs = int64(dur * 1000 / scale)
			}
		case "trak":
			t, err := s.mp4Trak(b)
			if err != nil {
				return false, err
			}
			tracks = append(tracks, t)
		}
		return true, nil
	})
	if err != nil {
		return Info{}, err
	}

	chapters := map[uint32]bool{}
	for _, t := range tracks {
		for _, id := range t.chapter {
			chapters[id] = true
		}
	}
	for _, t := range tracks {
		switch t.handler {
		case "vide":
			if info.VideoCodec == "" {
				info.VideoCodec, info.Width, info.Height = t.codec, t.w, t.h
			}
		case "soun":
			info.Audio = append(info.Audio, Track{Codec: t.codec, Lang: t.lang, Channels: t.ch})
		case "sbtl", "text", "subt", "clcp", "subp":
			if !chapters[t.id] {
				info.Subs = append(info.Subs, Track{Codec: t.codec, Lang: t.lang})
			}
		}
	}
	return info, nil
}

func isFourCC(s string) bool {
	for _, c := range []byte(s) {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}

func (s *source) mp4Trak(trak mp4Box) (mp4Track, error) {
	var t mp4Track
	var stsd *mp4Box
	var walk func(off, end int64) error
	walk = func(off, end int64) error {
		return boxes(s, off, end, func(b mp4Box) (bool, error) {
			switch b.typ {
			case "mdia", "minf", "stbl", "tref":
				return true, walk(b.dataOff, b.end)
			case "tkhd":
				d, err := s.boxData(b, 24)
				if err != nil {
					return false, err
				}
				if len(d) >= 24 && d[0] == 1 {
					t.id = binary.BigEndian.Uint32(d[20:])
				} else if len(d) >= 16 {
					t.id = binary.BigEndian.Uint32(d[12:])
				}
			case "chap":
				d, err := s.boxData(b, 256)
				if err != nil {
					return false, err
				}
				for i := 0; i+4 <= len(d); i += 4 {
					t.chapter = append(t.chapter, binary.BigEndian.Uint32(d[i:]))
				}
			case "hdlr":
				d, err := s.boxData(b, 12)
				if err != nil {
					return false, err
				}
				// QuickTime files carry a second, data-reference hdlr inside
				// minf; the media handler is the one in mdia, seen first.
				if len(d) >= 12 && t.handler == "" {
					t.handler = string(d[8:12])
				}
			case "mdhd":
				d, err := s.boxData(b, 34)
				if err != nil {
					return false, err
				}
				at := 20
				if len(d) > 0 && d[0] == 1 {
					at = 32
				}
				if len(d) >= at+2 {
					t.lang = mp4Lang(binary.BigEndian.Uint16(d[at:]))
				}
			case "stsd":
				stsd = &b
			}
			return true, nil
		})
	}
	if err := walk(trak.dataOff, trak.end); err != nil {
		return t, err
	}
	if stsd != nil {
		if err := s.mp4SampleEntry(*stsd, &t); err != nil {
			return t, err
		}
	}
	return t, nil
}

// mp4Lang unpacks an ISO-639-2/T code stored as three 5-bit letters.
// Values below 0x400 are old QuickTime language numbers and are ignored.
func mp4Lang(v uint16) string {
	if v < 0x400 {
		return ""
	}
	b := []byte{byte(v>>10&0x1F) + 0x60, byte(v>>5&0x1F) + 0x60, byte(v&0x1F) + 0x60}
	return normLang(string(b))
}

// mp4SampleEntry reads the first sample description of a track.
func (s *source) mp4SampleEntry(stsd mp4Box, t *mp4Track) error {
	if stsd.end-stsd.dataOff < 16 {
		return nil
	}
	entry, err := readBox(s, stsd.dataOff+8, stsd.end)
	if err != nil {
		return err
	}
	d, err := s.boxData(entry, 64)
	if err != nil {
		return err
	}
	switch t.handler {
	case "vide":
		if len(d) >= 28 {
			t.w, t.h = int(binary.BigEndian.Uint16(d[24:])), int(binary.BigEndian.Uint16(d[26:]))
		}
		t.codec = mp4VideoCodec(entry.typ)
		if entry.typ == "mp4v" {
			if es, ok := s.mp4ESDS(entry, 78); ok {
				t.codec = mpeg4VideoObject(es.objectType)
			}
		}
	case "soun":
		if len(d) >= 18 {
			t.ch = int(binary.BigEndian.Uint16(d[16:]))
		}
		t.codec = mp4AudioCodec(entry.typ)
		start := int64(0)
		if len(d) >= 10 {
			start = map[uint16]int64{0: 28, 1: 44, 2: 64}[binary.BigEndian.Uint16(d[8:])]
		}
		if entry.typ == "ac-3" {
			if ch := s.mp4AC3Channels(entry, start); ch > 0 {
				t.ch = ch // stsd often says 2 whatever the stream has
			}
		}
		if entry.typ == "mp4a" {
			if es, ok := s.mp4ESDS(entry, start); ok {
				t.codec = mpeg4AudioObject(es.objectType)
				if es.channels > 0 {
					t.ch = es.channels // stsd often says 2 whatever the stream has
				}
			}
		}
	default:
		t.codec = mp4SubCodec(entry.typ)
	}
	return nil
}

// mp4AC3Channels reads the channel layout from the dac3 box of an "ac-3"
// sample entry: acmod gives the full-range channels, lfeon adds the LFE.
func (s *source) mp4AC3Channels(entry mp4Box, childOff int64) int {
	if childOff == 0 {
		return 0
	}
	ch := 0
	boxes(s, entry.dataOff+childOff, entry.end, func(b mp4Box) (bool, error) {
		if b.typ != "dac3" {
			return true, nil
		}
		if d, err := s.boxData(b, 3); err == nil && len(d) == 3 {
			v := uint32(d[0])<<16 | uint32(d[1])<<8 | uint32(d[2])
			ch = []int{2, 1, 2, 3, 3, 4, 4, 5}[v>>11&7] + int(v>>10&1)
		}
		return false, nil
	})
	return ch
}

// esdsInfo is what we use from an MPEG-4 elementary stream descriptor.
type esdsInfo struct {
	objectType byte
	channels   int // from the AAC AudioSpecificConfig; 0 when absent
}

// mp4ESDS finds the esds box among the children of a sample entry (directly
// or inside a QuickTime "wave" box) and parses it.
func (s *source) mp4ESDS(entry mp4Box, childOff int64) (esdsInfo, bool) {
	if childOff == 0 {
		return esdsInfo{}, false
	}
	var es esdsInfo
	var found bool
	var walk func(off, end int64)
	walk = func(off, end int64) {
		boxes(s, off, end, func(b mp4Box) (bool, error) {
			switch b.typ {
			case "wave":
				walk(b.dataOff, b.end)
			case "esds":
				if d, err := s.boxData(b, 128); err == nil {
					es, found = parseESDS(d)
				}
			}
			return !found, nil
		})
	}
	walk(entry.dataOff+childOff, entry.end)
	return es, found
}

// parseESDS parses an esds payload: version/flags, then an ES_Descriptor
// holding a DecoderConfigDescriptor, which may hold a DecoderSpecificInfo.
func parseESDS(d []byte) (esdsInfo, bool) {
	p := 4
	descr := func(want byte) (int, bool) { // returns the descriptor length
		if p >= len(d) || d[p] != want {
			return 0, false
		}
		p++
		n := 0
		for i := 0; i < 4 && p < len(d); i++ { // expandable length
			c := d[p]
			p++
			n = n<<7 | int(c&0x7F)
			if c&0x80 == 0 {
				break
			}
		}
		return n, p < len(d)
	}
	if _, ok := descr(0x03); !ok || p+3 > len(d) {
		return esdsInfo{}, false
	}
	flags := d[p+2]
	p += 3
	if flags&0x80 != 0 {
		p += 2
	}
	if flags&0x40 != 0 && p < len(d) {
		p += 1 + int(d[p])
	}
	if flags&0x20 != 0 {
		p += 2
	}
	if _, ok := descr(0x04); !ok {
		return esdsInfo{}, false
	}
	es := esdsInfo{objectType: d[p]}
	p += 13 // objectType, streamType, bufferSize, maxBitrate, avgBitrate
	if n, ok := descr(0x05); ok && n >= 2 && p+2 <= len(d) {
		es.channels = aacChannels(d[p:min(p+n, len(d))])
	}
	return es, true
}

// aacChannels reads channelConfiguration from an AudioSpecificConfig.
func aacChannels(asc []byte) int {
	if len(asc) < 2 || asc[0]>>3 == 31 { // escaped object types are rare: skip
		return 0
	}
	var cfg byte
	if freq := (asc[0]&7)<<1 | asc[1]>>7; freq == 15 { // explicit 24-bit frequency
		if len(asc) < 5 {
			return 0
		}
		cfg = (asc[4] >> 3) & 0xF
	} else {
		cfg = (asc[1] >> 3) & 0xF
	}
	switch {
	case cfg >= 1 && cfg <= 6:
		return int(cfg)
	case cfg == 7:
		return 8
	}
	return 0
}

func mpeg4AudioObject(ot byte) string {
	switch ot {
	case 0x40, 0x66, 0x67, 0x68:
		return "aac"
	case 0x69, 0x6B:
		return "mp3"
	case 0xA5:
		return "ac3"
	case 0xA6:
		return "eac3"
	case 0xA9:
		return "dts"
	case 0xDD:
		return "vorbis"
	}
	return "aac"
}

func mpeg4VideoObject(ot byte) string {
	switch {
	case ot == 0x20:
		return "mpeg4"
	case ot >= 0x60 && ot <= 0x65:
		return "mpeg2"
	case ot == 0x6A:
		return "mpeg1"
	case ot == 0x6C:
		return "mjpeg"
	case ot == 0x21:
		return "h264"
	}
	return "mpeg4"
}

func mp4VideoCodec(f string) string {
	switch f {
	case "avc1", "avc3":
		return "h264"
	case "hvc1", "hev1", "dvh1", "dvhe":
		return "hevc"
	case "av01":
		return "av1"
	case "vp09":
		return "vp9"
	case "vp08":
		return "vp8"
	case "mp4v":
		return "mpeg4"
	case "jpeg", "mjpa", "mjpb", "mjp2":
		return "mjpeg"
	}
	return normFourCC([]byte(f))
}

func mp4AudioCodec(f string) string {
	switch f {
	case "mp4a":
		return "aac"
	case "ac-3":
		return "ac3"
	case "ec-3":
		return "eac3"
	case "dtsc", "dtsh", "dtsl", "dtse":
		return "dts"
	case "mlpa":
		return "truehd"
	case "fLaC":
		return "flac"
	case "Opus":
		return "opus"
	case ".mp3", "ms\x00U":
		return "mp3"
	case "lpcm", "sowt", "twos", "in24", "in32", "fl32", "fl64", "raw ":
		return "pcm"
	}
	return strings.ToLower(strings.TrimSpace(f))
}

func mp4SubCodec(f string) string {
	switch f {
	case "tx3g", "text":
		return "mov_text"
	case "wvtt":
		return "webvtt"
	case "stpp":
		return "ttml"
	case "c608":
		return "eia_608"
	case "mp4s":
		return "vobsub"
	}
	return strings.ToLower(strings.TrimSpace(f))
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/probe/... && go test ./internal/probe/ -v -run MP4`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/probe
git commit -m "feat(probe): native MP4/MOV header reader"
```

---

### Task 4: probe — lector AVI

**Files:**
- Create: `internal/probe/avi.go`
- Test: `internal/probe/avi_test.go`

Referencia: `RIFF` + tamaño LE + `AVI `; chunks = id (4) + tamaño uint32 LE + datos, con relleno a longitud par; `LIST` = `LIST` + tamaño + tipo (4) + hijos. `LIST hdrl` (primero, suele ocupar pocos KB) contiene `avih` (µs/frame @0, frames totales @16, ancho @32, alto @36), un `LIST strl` por stream con `strh` (tipo `vids`/`auds` @0, scale @20, rate @24, length @32) y `strf` (video: `BITMAPINFOHEADER`, ancho @4, alto @8 con signo, FourCC @16; audio: `WAVEFORMATEX`, formato @0, canales @2, cbSize @16, extra @18), y en OpenDML `LIST odml` → `dmlh` (frames totales @0). Duración: `length × scale / rate` del stream de video; si no hay rate, frames de `dmlh` y luego de `avih` × µs/frame (en OpenDML `avih` cuenta solo el primer RIFF).

Lo que enseñó la colección real: los AVI con AC3/DTS declaran `nChannels = 5` para audio 5.1 (se toma como 6); el AAC en AVI declara canales poco fiables, el dato real está en el AudioSpecificConfig de los bytes extra.

- [ ] **Step 1: Test que falla**

`internal/probe/avi_test.go`:

```go
package probe

import (
	"errors"
	"reflect"
	"testing"

	pt "cinexplorer/internal/probe/probetest"
)

func avih(usPerFrame, frames, w, h uint32) []byte {
	return pt.Chunk("avih", join(pt.U32LE(usPerFrame), zeros(12), pt.U32LE(frames), zeros(12),
		pt.U32LE(w), pt.U32LE(h), zeros(16)))
}

func strh(typ, handler string, scale, rate, length uint32) []byte {
	return pt.Chunk("strh", join([]byte(typ), []byte(handler), zeros(12), pt.U32LE(scale), pt.U32LE(rate),
		zeros(4), pt.U32LE(length), zeros(20)))
}

func vidsStrf(w, h int32, fourcc string) []byte {
	return pt.Chunk("strf", join(pt.U32LE(40), pt.U32LE(uint32(w)), pt.U32LE(uint32(h)), zeros(4), []byte(fourcc), zeros(20)))
}

func audsStrf(tag, channels uint16) []byte {
	return pt.Chunk("strf", join(pt.U16LE(tag), pt.U16LE(channels), zeros(14)))
}

func TestAVIXviD(t *testing.T) {
	file := pt.RIFF("AVI ",
		pt.List("hdrl",
			avih(40000, 150000, 640, 272),
			pt.List("strl", strh("vids", "xvid", 1, 25, 150000), vidsStrf(640, 272, "XVID")),
			pt.List("strl", strh("auds", "\x00\x00\x00\x00", 1152, 48000, 0), audsStrf(0x0055, 2)),
			pt.List("strl", strh("auds", "\x00\x00\x00\x00", 1, 1, 0), audsStrf(0x2000, 6)),
		),
		pt.List("movi", zeros(32)),
	)
	got, err := readAVI(src(file))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "avi", DurationMs: 6_000_000, Width: 640, Height: 272, VideoCodec: "mpeg4",
		Audio: []Track{{Codec: "mp3", Channels: 2}, {Codec: "ac3", Channels: 6}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestAVIOpenDMLAndJunk(t *testing.T) {
	file := pt.RIFF("AVI ",
		pt.Chunk("JUNK", zeros(7)),
		pt.List("hdrl",
			avih(41708, 1000, 0, 0), // first RIFF only: the real total is in dmlh
			pt.List("strl", strh("vids", "H264", 1, 0, 0), vidsStrf(1280, -720, "H264")),
			pt.List("odml", pt.Chunk("dmlh", join(pt.U32LE(172_000), zeros(244)))),
		),
	)
	got, err := readAVI(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMs != 172_000*41708/1000 || got.Width != 1280 || got.Height != 720 || got.VideoCodec != "h264" {
		t.Fatalf("got %+v", got)
	}
}

func TestAVIInvalid(t *testing.T) {
	good := pt.RIFF("AVI ", pt.List("hdrl", avih(40000, 10, 320, 240)))
	for name, b := range map[string][]byte{
		"not riff":  []byte("\x1aE\xdf\xa3 not an avi file"),
		"no avih":   pt.RIFF("AVI ", pt.List("hdrl", pt.Chunk("JUNK", zeros(4)))),
		"no hdrl":   pt.RIFF("AVI ", pt.List("movi", zeros(8))),
		"truncated": good[:len(good)-20],
		"wave":      pt.RIFF("WAVE", pt.Chunk("fmt ", zeros(16))),
	} {
		if _, err := readAVI(src(b)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestAVIAudioChannelFixes(t *testing.T) {
	// AAC: nChannels says 2, the AudioSpecificConfig (LC, 48 kHz) says mono.
	aac := pt.Chunk("strf", join(pt.U16LE(0x00FF), pt.U16LE(2), zeros(12), pt.U16LE(2), []byte{0x11, 0x88}))
	file := pt.RIFF("AVI ", pt.List("hdrl",
		avih(40000, 10, 640, 480),
		pt.List("strl", strh("auds", "\x00\x00\x00\x00", 1, 1, 0), aac),
		pt.List("strl", strh("auds", "\x00\x00\x00\x00", 1, 1, 0), audsStrf(0x2000, 5)), // AC3 "5" is 5.1
	))
	got, err := readAVI(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if want := []Track{{Codec: "aac", Channels: 1}, {Codec: "ac3", Channels: 6}}; !reflect.DeepEqual(got.Audio, want) {
		t.Fatalf("audio %+v, want %+v", got.Audio, want)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/probe/`
Expected: FAIL — `undefined: readAVI`

- [ ] **Step 3: Implementación**

`internal/probe/avi.go`:

```go
package probe

import "encoding/binary"

var le = binary.LittleEndian

// readAVI reads the RIFF "hdrl" list at the start of an AVI file.
func readAVI(s *source) (Info, error) {
	h, err := s.read(0, 12)
	if err != nil {
		return Info{}, err
	}
	if string(h[0:4]) != "RIFF" || string(h[8:12]) != "AVI " {
		return Info{}, invalid("not an AVI file")
	}
	off := int64(12)
	for i := 0; i < 8 && off+12 <= s.size; i++ {
		c, err := s.read(off, 12)
		if err != nil {
			return Info{}, err
		}
		size := int64(le.Uint32(c[4:8]))
		if string(c[0:4]) == "LIST" && string(c[8:12]) == "hdrl" {
			if size < 4 || size > 1<<20 {
				return Info{}, invalid("AVI hdrl of %d bytes", size)
			}
			body, err := s.read(off+12, int(size-4))
			if err != nil {
				return Info{}, err
			}
			return parseHdrl(body)
		}
		off += 8 + size + size&1
	}
	return Info{}, invalid("AVI without hdrl")
}

// riffChunks calls fn for each chunk in b. For LIST chunks, data starts with
// the list type.
func riffChunks(b []byte, fn func(id string, data []byte)) {
	for len(b) >= 8 {
		id, size := string(b[0:4]), int(le.Uint32(b[4:8]))
		if size > len(b)-8 {
			size = len(b) - 8
		}
		fn(id, b[8:8+size])
		b = b[min(8+size+size&1, len(b)):]
	}
}

func parseHdrl(hdrl []byte) (Info, error) {
	info := Info{Container: "avi"}
	var usPerFrame, totalFrames, dmlFrames uint32
	var avihW, avihH int
	var gotAvih bool
	var videoMs int64 = -1

	riffChunks(hdrl, func(id string, d []byte) {
		switch {
		case id == "avih" && len(d) >= 40:
			gotAvih = true
			usPerFrame, totalFrames = le.Uint32(d[0:]), le.Uint32(d[16:])
			avihW, avihH = int(le.Uint32(d[32:])), int(le.Uint32(d[36:]))
		case id == "LIST" && len(d) >= 4 && string(d[:4]) == "odml":
			riffChunks(d[4:], func(id string, d []byte) {
				if id == "dmlh" && len(d) >= 4 {
					dmlFrames = le.Uint32(d)
				}
			})
		case id == "LIST" && len(d) >= 4 && string(d[:4]) == "strl":
			var strh, strf []byte
			riffChunks(d[4:], func(id string, d []byte) {
				switch id {
				case "strh":
					strh = d
				case "strf":
					strf = d
				}
			})
			if len(strh) < 36 {
				return
			}
			switch string(strh[0:4]) {
			case "vids":
				if info.VideoCodec != "" || len(strf) < 20 {
					return
				}
				info.VideoCodec = normFourCC(strf[16:20])
				info.Width = int(int32(le.Uint32(strf[4:])))
				info.Height = abs(int(int32(le.Uint32(strf[8:]))))
				scale, rate, length := uint64(le.Uint32(strh[20:])), uint64(le.Uint32(strh[24:])), uint64(le.Uint32(strh[32:]))
				if rate > 0 {
					videoMs = int64(length * scale * 1000 / rate)
				}
			case "auds":
				info.Audio = append(info.Audio, aviAudio(strf))
			}
		}
	})
	if !gotAvih {
		return Info{}, invalid("AVI without avih")
	}
	if info.Width == 0 && info.Height == 0 {
		info.Width, info.Height = avihW, avihH
	}
	switch {
	case videoMs > 0:
		info.DurationMs = videoMs
	case dmlFrames > 0:
		info.DurationMs = int64(dmlFrames) * int64(usPerFrame) / 1000
	default:
		info.DurationMs = int64(totalFrames) * int64(usPerFrame) / 1000
	}
	return info, nil
}

// aviAudio reads a WAVEFORMATEX. Its channel count is what the muxer wrote,
// which for AAC is refined from the AudioSpecificConfig in the extra bytes
// and for AC3/DTS commonly says 5 for a 5.1 stream (the LFE is left out).
func aviAudio(strf []byte) Track {
	var t Track
	if len(strf) < 4 {
		return t
	}
	t.Codec = wavFormat(le.Uint16(strf[0:]))
	t.Channels = int(le.Uint16(strf[2:]))
	switch t.Codec {
	case "aac":
		if len(strf) >= 20 {
			extra := strf[18:min(18+int(le.Uint16(strf[16:])), len(strf))]
			if ch := aacChannels(extra); ch > 0 {
				t.Channels = ch
			}
		}
	case "ac3", "dts":
		if t.Channels == 5 {
			t.Channels = 6
		}
	}
	return t
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/probe/... && go test ./internal/probe/ -v -run AVI`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/probe
git commit -m "feat(probe): native AVI header reader"
```

---

### Task 5: probe — lector IFO de DVD

**Files:**
- Create: `internal/probe/ifo.go`
- Test: `internal/probe/ifo_test.go`

Referencia (`VTS_xx_0.IFO`, big-endian): firma `DVDVIDEO-VTS` @0; sector de `VTS_PGCIT` @0xCC (× 2048 bytes); atributos de video u16 @0x200 (bits 15–14 MPEG-1/2, bits 13–12 NTSC/PAL, bits 3–2 tamaño 720/704/352/352×240|288); cantidad de audios u16 @0x202 y atributos de 8 bytes @0x204 (byte 0: formato en bits 7–5 — 0 AC3, 2/3 MPEG, 4 LPCM, 6 DTS —, tipo de idioma en bits 3–2; byte 1: canales−1 en bits 2–0; bytes 2–3: idioma ISO 639-1); cantidad de subpicture u16 @0x254 y atributos de 6 bytes @0x256 (tipo de idioma en bits 1–0 del byte 0; idioma en bytes 2–3). `VTS_PGCIT`: cantidad u16 @0, entradas de 8 bytes desde @8 con el offset de cada PGC u32 @+4; cada PGC tiene el tiempo de reproducción en BCD @4 (horas, minutos, segundos, frames con la tasa en los 2 bits altos: 01 = 25 fps, 11 = 30 fps). La duración del IFO es la PGC más larga.

- [ ] **Step 1: Test que falla**

`internal/probe/ifo_test.go`:

```go
package probe

import (
	"errors"
	"reflect"
	"testing"
)

type ifoAudio struct {
	format, channels byte
	lang             string
}

// buildIFO returns a VTS IFO with a PGCIT in sector 1 holding one PGC per
// playback time (4 BCD bytes each).
func buildIFO(video uint16, audio []ifoAudio, subs []string, times ...[4]byte) []byte {
	b := make([]byte, 2*dvdSector)
	copy(b, "DVDVIDEO-VTS")
	be.PutUint32(b[ifoPGCITSector:], 1)
	be.PutUint16(b[ifoVideoAttr:], video)
	be.PutUint16(b[ifoAudioCount:], uint16(len(audio)))
	for i, a := range audio {
		at := b[ifoAudioAttr+8*i:]
		at[0] = a.format<<5 | 1<<2 // lang_type 1: language present
		at[1] = a.channels - 1
		copy(at[2:4], a.lang)
	}
	be.PutUint16(b[ifoSubpCount:], uint16(len(subs)))
	for i, l := range subs {
		at := b[ifoSubpAttr+6*i:]
		at[0] = 1
		copy(at[2:4], l)
	}
	pgcit := b[dvdSector:]
	be.PutUint16(pgcit, uint16(len(times)))
	for i, tm := range times {
		start := 8 + 8*len(times) + 16*i
		be.PutUint32(pgcit[8+8*i+4:], uint32(start))
		copy(pgcit[start+4:], tm[:])
	}
	return b
}

func TestIFOPAL(t *testing.T) {
	const mpeg2PAL = 0x4000 | 0x1000
	ifo := buildIFO(mpeg2PAL,
		[]ifoAudio{{0, 6, "es"}, {0, 2, "en"}},
		[]string{"es", "\x00\x00"},
		[4]byte{0x00, 0x03, 0x10, 0x40 | 0x12}, // 3:10 + 12 frames at 25 fps
		[4]byte{0x01, 0x52, 0x07, 0x40},        // 1:52:07: the main feature
	)
	got, err := readIFO(src(ifo))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "dvd", DurationMs: (1*3600 + 52*60 + 7) * 1000, Width: 720, Height: 576, VideoCodec: "mpeg2",
		Audio: []Track{{Codec: "ac3", Lang: "es", Channels: 6}, {Codec: "ac3", Lang: "en", Channels: 2}},
		Subs:  []Track{{Codec: "vobsub", Lang: "es"}, {Codec: "vobsub", Lang: ""}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestIFONTSCHalfD1(t *testing.T) {
	const mpeg2NTSC352x240 = 0x4000 | 3<<2
	got, err := readIFO(src(buildIFO(mpeg2NTSC352x240, []ifoAudio{{6, 6, "fr"}}, nil, [4]byte{0, 0, 0x30, 0xC0 | 0x15})))
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 352 || got.Height != 240 || got.DurationMs != 30_000+15*1000/30 || got.Audio[0].Codec != "dts" {
		t.Fatalf("got %+v", got)
	}
}

func TestIFOInvalid(t *testing.T) {
	good := buildIFO(0x5000, nil, nil, [4]byte{1, 0, 0, 0x40})
	vmg := append([]byte("DVDVIDEO-VMG"), good[12:]...)
	for name, b := range map[string][]byte{
		"menu ifo": vmg,
		"short":    good[:100],
		"no pgcit": good[:dvdSector+4],
	} {
		if _, err := readIFO(src(b)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/probe/`
Expected: FAIL — `undefined: readIFO`

- [ ] **Step 3: Implementación**

`internal/probe/ifo.go`:

```go
package probe

import "encoding/binary"

var be = binary.BigEndian

// Offsets in a DVD title set information file (VTS_xx_0.IFO).
const (
	ifoPGCITSector = 0x0CC
	ifoVideoAttr   = 0x200
	ifoAudioCount  = 0x202
	ifoAudioAttr   = 0x204 // 8 entries of 8 bytes
	ifoSubpCount   = 0x254
	ifoSubpAttr    = 0x256 // 32 entries of 6 bytes
	ifoHeaderLen   = ifoSubpAttr + 32*6
	dvdSector      = 2048
)

func readIFO(s *source) (Info, error) {
	h, err := s.read(0, ifoHeaderLen)
	if err != nil {
		return Info{}, err
	}
	if string(h[0:12]) != "DVDVIDEO-VTS" {
		return Info{}, invalid("not a DVD title set IFO")
	}
	info := Info{Container: "dvd", VideoCodec: "mpeg2"}
	video := be.Uint16(h[ifoVideoAttr:])
	if video>>14 == 0 {
		info.VideoCodec = "mpeg1"
	}
	pal := video>>12&3 == 1
	info.Width = [4]int{720, 704, 352, 352}[video>>2&3]
	info.Height = 480
	if pal {
		info.Height = 576
	}
	if video>>2&3 == 3 {
		info.Height /= 2
	}

	for i := range min(int(be.Uint16(h[ifoAudioCount:])), 8) {
		a := h[ifoAudioAttr+8*i:]
		codec := map[byte]string{0: "ac3", 2: "mp2", 3: "mp2", 4: "pcm", 6: "dts"}[a[0]>>5]
		info.Audio = append(info.Audio, Track{Codec: codec, Lang: ifoLang(a[0]>>2&3, a[2:4]), Channels: int(a[1]&7) + 1})
	}
	for i := range min(int(be.Uint16(h[ifoSubpCount:])), 32) {
		sp := h[ifoSubpAttr+6*i:]
		info.Subs = append(info.Subs, Track{Codec: "vobsub", Lang: ifoLang(sp[0]&3, sp[2:4])})
	}

	dur, err := s.ifoLongestPGC(int64(be.Uint32(h[ifoPGCITSector:])) * dvdSector)
	if err != nil {
		return Info{}, err
	}
	info.DurationMs = dur
	return info, nil
}

func ifoLang(langType byte, code []byte) string {
	if langType != 1 {
		return ""
	}
	return normLang(string(code))
}

// ifoLongestPGC returns the playback time of the longest program chain in the
// VTS_PGCIT table at off: the main feature of the title set.
func (s *source) ifoLongestPGC(off int64) (int64, error) {
	if off == 0 {
		return 0, nil
	}
	hdr, err := s.read(off, 8)
	if err != nil {
		return 0, err
	}
	n := min(int(be.Uint16(hdr)), 256)
	srps, err := s.read(off+8, 8*n)
	if err != nil {
		return 0, err
	}
	var best int64
	for i := range n {
		pgc, err := s.read(off+int64(be.Uint32(srps[8*i+4:])), 8)
		if err != nil {
			return 0, err
		}
		best = max(best, dvdTime(pgc[4:8]))
	}
	return best, nil
}

// dvdTime decodes a BCD playback time: hours, minutes, seconds, frames (the
// two top bits of the last byte give the frame rate).
func dvdTime(t []byte) int64 {
	bcd := func(b byte) int64 { return int64(b>>4)*10 + int64(b&0x0F) }
	ms := bcd(t[0])*3_600_000 + bcd(t[1])*60_000 + bcd(t[2])*1000
	fps := map[byte]int64{1: 25, 3: 30}[t[3]>>6]
	if fps > 0 {
		ms += bcd(t[3]&0x3F) * 1000 / fps
	}
	return ms
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/probe/... && go test ./internal/probe/ -v -run IFO`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/probe
git commit -m "feat(probe): DVD title set IFO reader"
```

---

### Task 6: probe — ffprobe y despacho por contenido

**Files:**
- Create: `internal/probe/ffprobe.go`
- Create: `internal/probe/prober.go`
- Test: `internal/probe/prober_test.go`

`Probe` abre el archivo (fallo → `ErrIO`), elige el lector por los primeros 16 bytes (la colección tiene `.avi` que son Matroska o ASF) y, si el lector nativo da `ErrInvalid` o no hay lector, prueba `ffprobe` cuando está configurado. El test usa el propio binario de test como `ffprobe` falso (`TestMain` responde según una variable de entorno), así no depende de que `ffprobe` esté instalado.

- [ ] **Step 1: Test que falla**

`internal/probe/prober_test.go`:

```go
package probe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	pt "cinexplorer/internal/probe/probetest"
)

const fakeFFprobeEnv = "CINEXPLORER_FAKE_FFPROBE"

// Recorded from `ffprobe -v error -print_format json -show_format -show_streams`
// on a RealMedia file (trimmed).
const rmvbJSON = `{
  "streams": [
    {"index": 0, "codec_name": "cook", "codec_type": "audio", "channels": 2, "tags": {"language": "spa"}},
    {"index": 1, "codec_name": "rv40", "codec_type": "video", "width": 640, "height": 352},
    {"index": 2, "codec_name": "mjpeg", "codec_type": "video", "width": 300, "height": 300, "disposition": {"attached_pic": 1}}
  ],
  "format": {"format_name": "rm", "duration": "5821.370000"}
}`

// TestMain lets the test binary stand in for ffprobe: when the environment
// variable is set, it prints a canned answer instead of running the tests.
func TestMain(m *testing.M) {
	switch os.Getenv(fakeFFprobeEnv) {
	case "ok":
		os.Stdout.WriteString(rmvbJSON)
		os.Exit(0)
	case "fail":
		os.Stderr.WriteString("Invalid data found when processing input\n")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestParseFFprobe(t *testing.T) {
	got, err := parseFFprobe([]byte(rmvbJSON))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "rm", DurationMs: 5_821_370, Width: 640, Height: 352, VideoCodec: "rv",
		Audio: []Track{{Codec: "cook", Lang: "es", Channels: 2}}, Prober: "ffprobe"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if _, err := parseFFprobe([]byte(`{"streams": [], "format": {}}`)); err == nil {
		t.Fatal("no streams must be an error")
	}
}

func write(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProbeNative(t *testing.T) {
	p := write(t, "Stalker.1979.MKV", pt.MKV(1920, 1080, 9_720_000, "rus"))
	got, err := Prober{}.Probe(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Prober != "native" || got.Height != 1080 || got.Audio[0].Lang != "ru" {
		t.Fatalf("got %+v", got)
	}
}

func TestProbeTrustsContentOverExtension(t *testing.T) {
	got, err := Prober{}.Probe(context.Background(), write(t, "Soy Cuba.avi", pt.MKV(720, 576, 1000, "spa")))
	if err != nil || got.Container != "matroska" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestProbeErrorsWithoutFFprobe(t *testing.T) {
	ctx := context.Background()
	if _, err := (Prober{}).Probe(ctx, filepath.Join(t.TempDir(), "missing.mkv")); !errors.Is(err, ErrIO) {
		t.Errorf("missing file: %v, want ErrIO", err)
	}
	if _, err := (Prober{}).Probe(ctx, write(t, "cut.avi", []byte("RIFF\x10\x00\x00\x00AVI LIST"))); !errors.Is(err, ErrInvalid) {
		t.Errorf("truncated avi: %v, want ErrInvalid", err)
	}
	if _, err := (Prober{}).Probe(ctx, write(t, "movie.rmvb", []byte(".RMF"))); !errors.Is(err, ErrUnsupported) {
		t.Errorf("rmvb: %v, want ErrUnsupported", err)
	}
}

func TestProbeFallsBackToFFprobe(t *testing.T) {
	ctx := context.Background()
	fake := Prober{FFprobe: os.Args[0]}

	t.Setenv(fakeFFprobeEnv, "ok")
	got, err := fake.Probe(ctx, write(t, "movie.rmvb", []byte(".RMF")))
	if err != nil {
		t.Fatal(err)
	}
	if got.Prober != "ffprobe" || got.VideoCodec != "rv" {
		t.Fatalf("got %+v", got)
	}
	// A valid native file never reaches ffprobe.
	if got, _ := fake.Probe(ctx, write(t, "a.mkv", pt.MKV(1280, 720, 1000, "eng"))); got.Prober != "native" {
		t.Fatalf("native file probed by %q", got.Prober)
	}

	t.Setenv(fakeFFprobeEnv, "fail")
	mkv := pt.MKV(1280, 720, 1000, "eng")
	_, err = fake.Probe(ctx, write(t, "cut.mkv", mkv[:40]))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("failed fallback: %v, want the native ErrInvalid", err)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/probe/`
Expected: FAIL — `undefined: parseFFprobe`, `undefined: Prober`

- [ ] **Step 3: Implementación**

`internal/probe/ffprobe.go`:

```go
package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const defaultFFprobeTimeout = 30 * time.Second

func (p Prober) ffprobe(ctx context.Context, path string) (Info, error) {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultFFprobeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, p.FFprobe,
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", "-i", path).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return Info{}, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return Info{}, err
	}
	return parseFFprobe(out)
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType   string            `json:"codec_type"`
		CodecName   string            `json:"codec_name"`
		Width       int               `json:"width"`
		Height      int               `json:"height"`
		Channels    int               `json:"channels"`
		Tags        map[string]string `json:"tags"`
		Disposition map[string]int    `json:"disposition"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
}

func parseFFprobe(out []byte) (Info, error) {
	var o ffprobeOutput
	if err := json.Unmarshal(out, &o); err != nil {
		return Info{}, fmt.Errorf("ffprobe output: %w", err)
	}
	info := Info{Container: o.Format.FormatName, Prober: "ffprobe"}
	if d, err := strconv.ParseFloat(o.Format.Duration, 64); err == nil {
		info.DurationMs = int64(d * 1000)
	}
	for _, st := range o.Streams {
		lang := normLang(st.Tags["language"])
		switch st.CodecType {
		case "video":
			if info.VideoCodec == "" && st.Disposition["attached_pic"] == 0 {
				info.VideoCodec = normFFCodec(st.CodecName)
				info.Width, info.Height = st.Width, st.Height
			}
		case "audio":
			info.Audio = append(info.Audio, Track{Codec: normFFCodec(st.CodecName), Lang: lang, Channels: st.Channels})
		case "subtitle":
			info.Subs = append(info.Subs, Track{Codec: normFFCodec(st.CodecName), Lang: lang})
		}
	}
	if info.VideoCodec == "" && len(info.Audio) == 0 {
		return Info{}, fmt.Errorf("ffprobe found no audio or video streams")
	}
	return info, nil
}
```

`internal/probe/prober.go`:

```go
package probe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Prober reads headers natively and falls back to ffprobe when the native
// reader cannot parse the file or there is no reader for its extension.
type Prober struct {
	FFprobe string        // path to ffprobe; "" disables the fallback
	Timeout time.Duration // per ffprobe run; 0 means 30 s
}

var defaultProber = sync.OnceValue(func() Prober {
	path, _ := exec.LookPath("ffprobe")
	return Prober{FFprobe: path}
})

// Probe reads path with the default Prober, which uses ffprobe only if it is
// on the PATH.
func Probe(ctx context.Context, path string) (Info, error) {
	return defaultProber().Probe(ctx, path)
}

// nativeReader picks a reader from the first bytes of the file. Extensions
// cannot be trusted: libraries hold ".avi" files that are really Matroska or ASF.
func nativeReader(head []byte) func(*source) (Info, error) {
	switch {
	case len(head) >= 4 && string(head[:4]) == "\x1a\x45\xdf\xa3":
		return readMKV
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "AVI ":
		return readAVI
	case len(head) >= 12 && string(head[:12]) == "DVDVIDEO-VTS":
		return readIFO
	case len(head) >= 8 && mp4FirstBoxes[string(head[4:8])]:
		return readMP4
	}
	return nil
}

var mp4FirstBoxes = map[string]bool{"ftyp": true, "moov": true, "mdat": true, "free": true, "skip": true, "wide": true, "pnot": true}

func (p Prober) Probe(ctx context.Context, path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}

	head := make([]byte, 16)
	n, err := f.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	nativeErr := fmt.Errorf("%w: unrecognized content in %q", ErrUnsupported, filepath.Base(path))
	if read := nativeReader(head[:n]); read != nil {
		info, err := read(newSource(f, st.Size()))
		if err == nil {
			info.Prober = "native"
			return info, nil
		}
		if !errors.Is(err, ErrInvalid) {
			return Info{}, err
		}
		nativeErr = err
	}
	if p.FFprobe == "" {
		return Info{}, nativeErr
	}
	info, err := p.ffprobe(ctx, path)
	if err != nil {
		if ctx.Err() != nil {
			return Info{}, ctx.Err()
		}
		return Info{}, fmt.Errorf("%w; ffprobe: %v", nativeErr, err)
	}
	return info, nil
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/probe/... && go test ./internal/probe/ -v`
Expected: PASS (todos los tests del paquete, incluidos `TestProbeFallsBackToFFprobe` y `TestProbeTrustsContentOverExtension`)

- [ ] **Step 5: Commit**

```bash
git add internal/probe
git commit -m "feat(probe): content sniffing and optional ffprobe fallback"
```

---

### Task 7: probe — comparación contra ffprobe sobre la colección real

**Files:**
- Test: `internal/probe/corpus_test.go`

Test opcional: sin `CINEXPLORER_PROBE_CORPUS` se saltea (así corre en CI sin hacer nada). Solo lee archivos.

- [ ] **Step 1: Escribir el test**

`internal/probe/corpus_test.go`:

```go
package probe

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestCorpusAgainstFFprobe compares the native readers with ffprobe on a real
// movie folder. It never runs in CI:
//
//	CINEXPLORER_PROBE_CORPUS=D:/cine go test ./internal/probe -run Corpus -v -timeout 0
//
// CINEXPLORER_PROBE_LIMIT caps the number of files compared.
func TestCorpusAgainstFFprobe(t *testing.T) {
	root := os.Getenv("CINEXPLORER_PROBE_CORPUS")
	if root == "" {
		t.Skip("CINEXPLORER_PROBE_CORPUS not set")
	}
	ff, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}
	limit, _ := strconv.Atoi(os.Getenv("CINEXPLORER_PROBE_LIMIT"))
	ref := Prober{FFprobe: ff}
	var compared, mismatched, nativeFailed int

	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".mkv", ".webm", ".mp4", ".m4v", ".mov", ".avi", ".divx": // ffprobe cannot read IFO files
		default:
			return nil
		}
		if limit > 0 && compared >= limit {
			return filepath.SkipAll
		}
		want, err := ref.ffprobe(context.Background(), p)
		if err != nil {
			return nil // ffprobe cannot read it either
		}
		compared++
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		st, _ := f.Stat()
		head := make([]byte, 16)
		n, _ := f.ReadAt(head, 0)
		read := nativeReader(head[:n])
		if read == nil {
			return nil // not a format we read natively, whatever the extension says
		}
		got, err := read(newSource(f, st.Size()))
		if err != nil {
			nativeFailed++
			t.Errorf("%s: native failed: %v", p, err)
			return nil
		}
		if diffs := corpusDiffs(got, want); len(diffs) > 0 {
			mismatched++
			t.Errorf("%s:\n  %s", p, strings.Join(diffs, "\n  "))
		}
		return nil
	})
	t.Logf("compared %d files: %d mismatched, %d native failures", compared, mismatched, nativeFailed)
}

func corpusDiffs(got, want Info) []string {
	var d []string
	add := func(field string, g, w any) { d = append(d, fmt.Sprintf("%s: native %v, ffprobe %v", field, g, w)) }
	if got.Width != want.Width || got.Height != want.Height {
		add("size", fmt.Sprintf("%dx%d", got.Width, got.Height), fmt.Sprintf("%dx%d", want.Width, want.Height))
	}
	if diff := got.DurationMs - want.DurationMs; diff > 2000 || diff < -2000 {
		add("duration", got.DurationMs, want.DurationMs)
	}
	if got.VideoCodec != want.VideoCodec {
		add("video", got.VideoCodec, want.VideoCodec)
	}
	if len(got.Audio) != len(want.Audio) {
		add("audio tracks", got.Audio, want.Audio)
	} else {
		for i := range got.Audio {
			g, w := got.Audio[i], want.Audio[i]
			if g.Codec != w.Codec || g.Channels != w.Channels || !sameLang(g.Lang, w.Lang) {
				add(fmt.Sprintf("audio %d", i), g, w)
			}
		}
	}
	if len(got.Subs) != len(want.Subs) {
		add("subtitle tracks", got.Subs, want.Subs)
	} else {
		for i := range got.Subs {
			g, w := got.Subs[i], want.Subs[i]
			if (w.Codec != "" && g.Codec != w.Codec) || !sameLang(g.Lang, w.Lang) {
				add(fmt.Sprintf("subs %d", i), got.Subs[i], want.Subs[i])
			}
		}
	}
	return d
}

// sameLang ignores a side that has no language: the native readers know
// fields ffprobe leaves empty (Matroska LanguageIETF) and AVI has none.
func sameLang(a, b string) bool { return a == "" || b == "" || a == b }
```

- [ ] **Step 2: Verificar que en CI se saltea**

Run: `go test ./internal/probe/ -run Corpus -v`
Expected: `--- SKIP: TestCorpusAgainstFFprobe` y `ok`

- [ ] **Step 3: Correrlo contra la colección real** (necesita `ffprobe` en el PATH; en esta máquina está en `C:\portables\ffmpeg\bin`)

Run: `CINEXPLORER_PROBE_CORPUS=D:/cine go test ./internal/probe/ -run Corpus -v -timeout 0 2>&1 | tail -40`
Expected: al final `compared N files: M mismatched, 0 native failures` con M chico. La comparación de idiomas ignora el lado vacío (ffprobe no lee `LanguageIETF` de Matroska y AVI no declara idiomas). Referencia al escribir este plan: `D:\cine` 1264 archivos / 13 discrepancias y `D:\cine-ordenar` 531 / 8, cero fallas nativas. Las discrepancias aceptadas son: duraciones de AVI incompletos o con índice raro (el encabezado declara la duración original y ffprobe calcula otra, a veces absurda; p. ej. *Husbands*: 142 min según el encabezado y el nombre, 136 según ffprobe); HE-AAC v2 cuyo AudioSpecificConfig declara mono y ffprobe decodifica estéreo (Parametric Stereo implícito); y algún AC3/DTS donde el encabezado y el bitstream no coinciden en canales. Cualquier `native failed`, o discrepancias de tamaño, codec o pistas, se investigan: cada archivo raro se reproduce con un test mínimo en el `*_test.go` del lector antes de corregirlo.

- [ ] **Step 4: Commit**

```bash
git add internal/probe/corpus_test.go
git commit -m "test(probe): optional comparison against ffprobe on a real library"
```

---

### Task 8: quality — mejor versión e identidad provisoria

**Files:**
- Create: `internal/quality/quality.go`
- Test: `internal/quality/quality_test.go`

- [ ] **Step 1: Test que falla**

`internal/quality/quality_test.go`:

```go
package quality

import "testing"

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b Candidate
		want int // sign
	}{
		{"resolution wins over codec and size", Candidate{"1080p", "h264", 1}, Candidate{"720p", "hevc", 9}, 1},
		{"1080i counts as 1080p", Candidate{"1080i", "h264", 5}, Candidate{"1080p", "h264", 5}, 0},
		{"codec breaks resolution ties", Candidate{"1080p", "hevc", 1}, Candidate{"1080p", "h264", 9}, 1},
		{"xvid beats mpeg2", Candidate{"576p", "mpeg4", 1}, Candidate{"576p", "mpeg2", 9}, 1},
		{"unranked codec beats unknown", Candidate{"SD", "cinepak", 1}, Candidate{"SD", "", 9}, 1},
		{"size breaks full ties", Candidate{"720p", "h264", 1}, Candidate{"720p", "h264", 2}, -1},
		{"known resolution beats unknown", Candidate{"SD", "", 1}, Candidate{"", "av1", 9}, 1},
	} {
		got := Compare(c.a, c.b)
		if sign(got) != c.want {
			t.Errorf("%s: Compare = %d, want sign %d", c.name, got, c.want)
		}
		if sign(Compare(c.b, c.a)) != -c.want {
			t.Errorf("%s: Compare is not antisymmetric", c.name)
		}
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

func TestGroupKey(t *testing.T) {
	same := [][2]any{
		{"El Ángel Exterminador", "ángel exterminador"},
		{"The Good, the Bad and the Ugly", "good the bad and the ugly"},
		{"Amarcord", "AMARCORD"},
		{"8½", "8 ½"},
	}
	for _, p := range same {
		if a, b := GroupKey(p[0].(string), 1973), GroupKey(p[1].(string), 1973); a != b {
			t.Errorf("GroupKey(%q) = %q, GroupKey(%q) = %q; want equal", p[0], a, p[1], b)
		}
	}
	if GroupKey("Amarcord", 1973) == GroupKey("Amarcord", 1974) {
		t.Error("different years must not group")
	}
	if GroupKey("The", 2000) != "the|2000" {
		t.Errorf("a lone article is the title: %q", GroupKey("The", 2000))
	}
	if GroupKey("", 1973) != "" || GroupKey("…", 1973) != "" {
		t.Error("empty titles must not group")
	}
	if got := GroupKey("La Strada", 1954); got != "strada|1954" {
		t.Errorf("GroupKey = %q", got)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/quality/`
Expected: FAIL — `undefined: Compare`, `undefined: Candidate`, `undefined: GroupKey`

- [ ] **Step 3: Implementación**

`internal/quality/quality.go`:

```go
// Package quality ranks versions of the same movie to pick the best one.
package quality

import (
	"cmp"
	"strconv"
	"strings"
	"unicode"
)

type Candidate struct {
	Resolution string // "2160p", "1080p", … as produced by probe.ResolutionLabel or nameparse
	Codec      string // normalized video codec ("hevc", "h264", …)
	Size       int64
}

var resRank = map[string]int{"2160p": 7, "1080p": 6, "1080i": 6, "720p": 5, "576p": 4, "480p": 3, "SD": 2}

// codecOrder lists video codecs from most to least efficient.
var codecOrder = []string{"av1", "hevc", "vp9", "h264", "vp8", "mpeg4", "wmv", "rv", "mpeg2", "mpeg1"}

func codecRank(c string) int {
	for i, name := range codecOrder {
		if c == name {
			return len(codecOrder) + 1 - i
		}
	}
	if c != "" {
		return 1 // known to exist, but not ranked
	}
	return 0
}

// Compare orders candidates by resolution, then codec, then size. It returns
// a positive number when a is better than b.
func Compare(a, b Candidate) int {
	if c := cmp.Compare(resRank[a.Resolution], resRank[b.Resolution]); c != 0 {
		return c
	}
	if c := cmp.Compare(codecRank(a.Codec), codecRank(b.Codec)); c != 0 {
		return c
	}
	return cmp.Compare(a.Size, b.Size)
}

var articles = map[string]bool{"the": true, "el": true, "la": true, "los": true, "las": true, "le": true, "les": true, "il": true, "lo": true}

var folds = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a", "å", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o", "ø", "o",
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

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/quality/ && go test ./internal/quality/ -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/quality
git commit -m "feat(quality): rank versions and provisional movie identity"
```

---

### Task 9: store — tabla media, pendientes y guardado

**Files:**
- Modify: `internal/store/schema.sql` (agregar al final)
- Create: `internal/store/media.go`
- Test: `internal/store/media_test.go`

`CREATE TABLE IF NOT EXISTS` alcanza como migración: un catálogo de la Etapa 1 gana la tabla vacía al abrirse (`openDB` ya ejecuta el esquema completo) y el próximo escaneo la llena.

- [ ] **Step 1: Test que falla**

`internal/store/media_test.go`:

```go
package store

import (
	"reflect"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/probe"
)

// mediaLibrary catalogs a movie with an extra, a DVD and a missing file.
func mediaLibrary(t *testing.T, s *Store) {
	t.Helper()
	files := []FileRow{
		{Path: "../cine/a/Amarcord.mkv", Size: 100, MTime: 1, Fingerprint: "a", Kind: "video"},
		{Path: "../cine/a/Trailer.mkv", Size: 5, MTime: 1, Fingerprint: "t", Kind: "video"},
		{Path: "../cine/a/Amarcord.spa.srt", Size: 1, MTime: 1, Kind: "subtitle"},
		{Path: "../cine/b/Stalker.avi", Size: 90, MTime: 1, Fingerprint: "s", Kind: "video"},
		{Path: "../cine/c/VIDEO_TS/VIDEO_TS.IFO", Size: 10, MTime: 1, Fingerprint: "v0", Kind: "dvd"},
		{Path: "../cine/c/VIDEO_TS/VTS_01_0.IFO", Size: 10, MTime: 1, Fingerprint: "v1", Kind: "dvd"},
		{Path: "../cine/c/VIDEO_TS/VTS_01_1.VOB", Size: 900, MTime: 1, Fingerprint: "v2", Kind: "dvd"},
		{Path: "../cine/d/Gone.mkv", Size: 70, MTime: 1, Fingerprint: "g", Kind: "video"},
	}
	if err := s.SyncFiles(files, roots); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncFiles(files[:7], roots); err != nil { // Gone.mkv disappears
		t.Fatal(err)
	}
	main := func(p string) grouping.Member { return grouping.Member{Path: p, Role: grouping.RoleMain} }
	err := s.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/a", Parsed: nameparse.Parsed{Title: "Amarcord"}, Size: 100, Parts: 1, Members: []grouping.Member{
			main("../cine/a/Amarcord.mkv"),
			{Path: "../cine/a/Amarcord.spa.srt", Role: grouping.RoleSubtitle, Lang: "es"},
			{Path: "../cine/a/Trailer.mkv", Role: grouping.RoleExtra},
		}},
		{Dir: "../cine/b", Parsed: nameparse.Parsed{Title: "Stalker"}, Size: 90, Parts: 1,
			Members: []grouping.Member{main("../cine/b/Stalker.avi")}},
		{Dir: "../cine/c", Parsed: nameparse.Parsed{Title: "c"}, Size: 920, Parts: 1, Members: []grouping.Member{
			main("../cine/c/VIDEO_TS/VIDEO_TS.IFO"), main("../cine/c/VIDEO_TS/VTS_01_0.IFO"), main("../cine/c/VIDEO_TS/VTS_01_1.VOB"),
		}},
		{Dir: "../cine/d", Parsed: nameparse.Parsed{Title: "Gone"}, Size: 70, Parts: 1,
			Members: []grouping.Member{main("../cine/d/Gone.mkv")}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func pendingPaths(t *testing.T, s *Store) []string {
	t.Helper()
	ts, err := s.PendingProbes()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, x := range ts {
		out = append(out, x.Path)
	}
	return out
}

func TestPendingProbes(t *testing.T) {
	s := open(t)
	mediaLibrary(t, s)
	want := []string{"../cine/a/Amarcord.mkv", "../cine/b/Stalker.avi", "../cine/c/VIDEO_TS/VTS_01_0.IFO"}
	if got := pendingPaths(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("pending = %v, want %v", got, want)
	}

	ts, _ := s.PendingProbes()
	err := s.SaveProbes([]ProbeResult{
		{FileID: ts[0].FileID, Size: ts[0].Size, MTime: ts[0].MTime, Info: probe.Info{Height: 1080}},
		{FileID: ts[1].FileID, Size: ts[1].Size, MTime: ts[1].MTime, Err: "probe: invalid header: truncated file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := pendingPaths(t, s); !reflect.DeepEqual(got, want[2:]) {
		t.Fatalf("after saving, pending = %v", got)
	}

	// A changed file is probed again, even if the previous attempt failed.
	if err := s.SyncFiles([]FileRow{{Path: "../cine/b/Stalker.avi", Size: 91, MTime: 2, Fingerprint: "s2", Kind: "video"}}, nil); err != nil {
		t.Fatal(err)
	}
	if got := pendingPaths(t, s); !reflect.DeepEqual(got, []string{"../cine/b/Stalker.avi", "../cine/c/VIDEO_TS/VTS_01_0.IFO"}) {
		t.Fatalf("after change, pending = %v", got)
	}
}

func TestSaveProbesUpserts(t *testing.T) {
	s := open(t)
	mediaLibrary(t, s)
	ts, _ := s.PendingProbes()
	r := ProbeResult{FileID: ts[0].FileID, Size: 100, MTime: 1, Info: probe.Info{Height: 720}}
	if err := s.SaveProbes([]ProbeResult{r}); err != nil {
		t.Fatal(err)
	}
	r.Info.Height = 1080
	if err := s.SaveProbes([]ProbeResult{r}); err != nil {
		t.Fatal(err)
	}
	var n, h int
	s.db.QueryRow(`SELECT COUNT(*), MAX(height) FROM media`).Scan(&n, &h)
	if n != 1 || h != 1080 {
		t.Fatalf("rows=%d height=%d", n, h)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/store/`
Expected: FAIL — `undefined: ProbeResult`, `s.PendingProbes undefined`

- [ ] **Step 3: Implementación**

Agregar al final de `internal/store/schema.sql`:

```sql
-- Technical data read from each main video file (or DVD title set IFO).
-- size/mtime are the file's at probe time: a mismatch means "probe again".
CREATE TABLE IF NOT EXISTS media (
  file_id     INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
  size        INTEGER NOT NULL,
  mtime       INTEGER NOT NULL,
  prober      TEXT    NOT NULL DEFAULT '',
  error       TEXT    NOT NULL DEFAULT '',
  container   TEXT    NOT NULL DEFAULT '',
  duration_ms INTEGER NOT NULL DEFAULT 0,
  width       INTEGER NOT NULL DEFAULT 0,
  height      INTEGER NOT NULL DEFAULT 0,
  video_codec TEXT    NOT NULL DEFAULT '',
  audio       TEXT    NOT NULL DEFAULT '[]', -- JSON []probe.Track
  subs        TEXT    NOT NULL DEFAULT '[]'  -- JSON []probe.Track
);
```

`internal/store/media.go`:

```go
package store

import (
	"encoding/json"

	"cinexplorer/internal/probe"
)

// ProbeTarget is a file whose headers need to be read.
type ProbeTarget struct {
	FileID int64
	Path   string
	Size   int64
	MTime  int64
}

// ProbeResult is what reading a target produced. Err is set when the file was
// read but could not be understood; such results are kept so the file is not
// read again until it changes.
type ProbeResult struct {
	FileID int64
	Size   int64
	MTime  int64
	Info   probe.Info
	Err    string
}

// PendingProbes returns the present main video files, and the title set IFOs
// of DVD versions, that have never been probed or changed since.
func (s *Store) PendingProbes() ([]ProbeTarget, error) {
	rows, err := s.db.Query(`SELECT f.id, f.path, f.size, f.mtime FROM files f
		LEFT JOIN media m ON m.file_id = f.id
		WHERE f.missing = 0 AND f.role = 'main'
		  AND (f.kind = 'video' OR (f.kind = 'dvd' AND upper(f.path) GLOB '*/VTS_[0-9][0-9]_0.IFO'))
		  AND (m.file_id IS NULL OR m.size != f.size OR m.mtime != f.mtime)
		ORDER BY f.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProbeTarget
	for rows.Next() {
		var t ProbeTarget
		if err := rows.Scan(&t.FileID, &t.Path, &t.Size, &t.MTime); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveProbes stores a batch of results in one transaction.
func (s *Store) SaveProbes(rs []ProbeResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	up, err := tx.Prepare(`INSERT INTO media (file_id, size, mtime, prober, error, container, duration_ms,
		width, height, video_codec, audio, subs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_id) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
		prober = excluded.prober, error = excluded.error, container = excluded.container,
		duration_ms = excluded.duration_ms, width = excluded.width, height = excluded.height,
		video_codec = excluded.video_codec, audio = excluded.audio, subs = excluded.subs`)
	if err != nil {
		return err
	}
	defer up.Close()
	for _, r := range rs {
		audio, err := tracksJSON(r.Info.Audio)
		if err != nil {
			return err
		}
		subs, err := tracksJSON(r.Info.Subs)
		if err != nil {
			return err
		}
		i := r.Info
		if _, err := up.Exec(r.FileID, r.Size, r.MTime, i.Prober, r.Err, i.Container, i.DurationMs,
			i.Width, i.Height, i.VideoCodec, audio, subs); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func tracksJSON(ts []probe.Track) (string, error) {
	if ts == nil {
		ts = []probe.Track{}
	}
	b, err := json.Marshal(ts)
	return string(b), err
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/store/ && go test ./internal/store/ -v`
Expected: PASS (los tests existentes + `TestPendingProbes`, `TestSaveProbesUpserts`)

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): media table with pending probes and batched saves"
```

---

### Task 10: store — datos técnicos por versión y mejor versión

**Files:**
- Modify: `internal/store/store.go` (imports, `VersionView`, `Versions()`)
- Create: `internal/store/technical.go`
- Test: `internal/store/technical_test.go`

Reglas (spec §6): la parte 1 aporta ancho, alto y codec; la duración suma las partes; audio y subs se juntan sin repetir en orden de aparición. En un DVD, de los IFO analizados gana el de mayor duración. Solo cuentan resultados sin error de archivos presentes cuyo tamaño/mtime coincide con el analizado. La resolución y el codec del análisis reemplazan a los del nombre; sin análisis, el codec del nombre pasa a los mismos valores (`XviD` → `mpeg4`). `best` se marca en grupos de 2 o más versiones presentes con igual `quality.GroupKey`.

- [ ] **Step 1: Test que falla**

`internal/store/technical_test.go`:

```go
package store

import (
	"reflect"
	"testing"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/probe"
)

// saveProbe stores a probe result for the catalogued file at path.
func saveProbe(t *testing.T, s *Store, path string, info probe.Info, errText string) {
	t.Helper()
	var r ProbeResult
	if err := s.db.QueryRow(`SELECT id, size, mtime FROM files WHERE path = ?`, path).Scan(&r.FileID, &r.Size, &r.MTime); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	r.Info, r.Err = info, errText
	if err := s.SaveProbes([]ProbeResult{r}); err != nil {
		t.Fatal(err)
	}
}

func versionByDir(t *testing.T, s *Store, dir string) VersionView {
	t.Helper()
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if v.Dir == dir {
			return v
		}
	}
	t.Fatalf("no version in %s", dir)
	return VersionView{}
}

func TestVersionsTechnicalData(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{
		{Path: "../cine/e/1900 - Part 1.mkv", Size: 10, MTime: 1, Fingerprint: "p1", Kind: "video"},
		{Path: "../cine/e/1900 - Part 2.mkv", Size: 12, MTime: 1, Fingerprint: "p2", Kind: "video"},
		{Path: "../cine/c/VIDEO_TS/VTS_01_0.IFO", Size: 10, MTime: 1, Fingerprint: "v1", Kind: "dvd"},
		{Path: "../cine/c/VIDEO_TS/VTS_01_1.VOB", Size: 900, MTime: 1, Fingerprint: "v2", Kind: "dvd"},
		{Path: "../cine/c/VIDEO_TS/VTS_02_0.IFO", Size: 10, MTime: 1, Fingerprint: "v3", Kind: "dvd"},
		{Path: "../cine/b/Stalker.avi", Size: 90, MTime: 1, Fingerprint: "s", Kind: "video"},
	}, roots)
	main := func(p string, part int) grouping.Member {
		return grouping.Member{Path: p, Role: grouping.RoleMain, Part: part}
	}
	err := s.ReplaceVersions([]grouping.Version{
		{Dir: "../cine/e", Parsed: nameparse.Parsed{Title: "1900", Resolution: "720p"}, Size: 22, Parts: 2,
			Members: []grouping.Member{main("../cine/e/1900 - Part 1.mkv", 1), main("../cine/e/1900 - Part 2.mkv", 2)}},
		{Dir: "../cine/c", Parsed: nameparse.Parsed{Title: "c"}, Size: 920, Parts: 1, Members: []grouping.Member{
			main("../cine/c/VIDEO_TS/VTS_01_0.IFO", 0), main("../cine/c/VIDEO_TS/VTS_01_1.VOB", 0), main("../cine/c/VIDEO_TS/VTS_02_0.IFO", 0)}},
		{Dir: "../cine/b", Parsed: nameparse.Parsed{Title: "Stalker", Resolution: "576p"}, Size: 90, Parts: 1,
			Members: []grouping.Member{main("../cine/b/Stalker.avi", 0)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	es6, en2 := probe.Track{Codec: "ac3", Lang: "es", Channels: 6}, probe.Track{Codec: "aac", Lang: "en", Channels: 2}
	saveProbe(t, s, "../cine/e/1900 - Part 1.mkv", probe.Info{DurationMs: 3_600_000, Width: 1920, Height: 800, VideoCodec: "h264",
		Audio: []probe.Track{es6}, Subs: []probe.Track{{Codec: "srt", Lang: "it"}}}, "")
	saveProbe(t, s, "../cine/e/1900 - Part 2.mkv", probe.Info{DurationMs: 3_000_000, Width: 1920, Height: 800, VideoCodec: "h264",
		Audio: []probe.Track{es6, en2}}, "")
	saveProbe(t, s, "../cine/c/VIDEO_TS/VTS_01_0.IFO", probe.Info{DurationMs: 600_000, Width: 720, Height: 576, VideoCodec: "mpeg2"}, "")
	saveProbe(t, s, "../cine/c/VIDEO_TS/VTS_02_0.IFO", probe.Info{DurationMs: 6_000_000, Width: 720, Height: 480, VideoCodec: "mpeg2",
		Audio: []probe.Track{{Codec: "ac3", Lang: "fr", Channels: 2}}}, "")
	saveProbe(t, s, "../cine/b/Stalker.avi", probe.Info{}, "probe: invalid header: truncated file")

	v := versionByDir(t, s, "../cine/e")
	if v.DurationMs != 6_600_000 || v.Width != 1920 || v.Resolution != "1080p" || v.Codec != "h264" || v.VideoCodec != "h264" {
		t.Fatalf("multi-part: %+v", v)
	}
	if !reflect.DeepEqual(v.Audio, []probe.Track{es6, en2}) || !reflect.DeepEqual(v.Subs, []probe.Track{{Codec: "srt", Lang: "it"}}) {
		t.Fatalf("tracks: %+v / %+v", v.Audio, v.Subs)
	}

	dvd := versionByDir(t, s, "../cine/c")
	if dvd.DurationMs != 6_000_000 || dvd.Height != 480 || dvd.Resolution != "480p" || len(dvd.Audio) != 1 {
		t.Fatalf("dvd: %+v", dvd)
	}

	// A failed probe keeps the name-parsed data.
	if st := versionByDir(t, s, "../cine/b"); st.Width != 0 || st.DurationMs != 0 || st.Resolution != "576p" || st.Audio == nil {
		t.Fatalf("failed probe: %+v", st)
	}

	// A probe result for an older state of the file is ignored.
	s.SyncFiles([]FileRow{{Path: "../cine/e/1900 - Part 1.mkv", Size: 11, MTime: 2, Fingerprint: "p1b", Kind: "video"}}, nil)
	if v := versionByDir(t, s, "../cine/e"); v.DurationMs != 3_000_000 {
		t.Fatalf("stale part still counted: %+v", v)
	}
}

func TestVersionsNormalizesNameCodec(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{{Path: "../cine/x.avi", Size: 1, MTime: 1, Fingerprint: "x", Kind: "video"}}, roots)
	s.ReplaceVersions([]grouping.Version{{Dir: "../cine", Parsed: nameparse.Parsed{Title: "X", Codec: "XviD"}, Size: 1, Parts: 1,
		Members: []grouping.Member{{Path: "../cine/x.avi", Role: grouping.RoleMain}}}})
	if v := versionByDir(t, s, "../cine"); v.Codec != "mpeg4" {
		t.Fatalf("codec = %q", v.Codec)
	}
}

func TestVersionsMarksBest(t *testing.T) {
	s := open(t)
	type vdef struct {
		dir, file, title, res, codec string
		size                         int64
	}
	defs := []vdef{
		{"../cine/1", "a.mkv", "Amarcord", "720p", "H.264", 4},
		{"../cine/2", "a.avi", "Amarcord", "1080p", "XviD", 9},
		{"../cine/3", "a.mkv", "Amarcord", "1080p", "H.265", 3},
		{"../cine/4", "a.mkv", "amarcord", "2160p", "H.265", 50}, // file missing: not a candidate
		{"../cine/5", "s.mkv", "Stalker", "1080p", "H.264", 5},   // only version: never "best"
	}
	var rows []FileRow
	var gv []grouping.Version
	for i, d := range defs {
		p := d.dir + "/" + d.file
		rows = append(rows, FileRow{Path: p, Size: d.size, MTime: 1, Fingerprint: string(rune('a' + i)), Kind: "video"})
		gv = append(gv, grouping.Version{Dir: d.dir, Size: d.size, Parts: 1,
			Parsed:  nameparse.Parsed{Title: d.title, Year: 1973, Resolution: d.res, Codec: d.codec},
			Members: []grouping.Member{{Path: p, Role: grouping.RoleMain}}})
	}
	s.SyncFiles(rows, roots)
	s.SyncFiles(append(rows[:3:3], rows[4]), roots) // ../cine/4 goes missing
	if err := s.ReplaceVersions(gv); err != nil {
		t.Fatal(err)
	}
	vs, err := s.Versions()
	if err != nil {
		t.Fatal(err)
	}
	var best []string
	for _, v := range vs {
		if v.Best {
			best = append(best, v.Dir)
		}
	}
	if !reflect.DeepEqual(best, []string{"../cine/3"}) {
		t.Fatalf("best = %v", best)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/store/`
Expected: FAIL — `v.DurationMs undefined (type VersionView has no field or method DurationMs)` (y `Best`, `Audio`…)

- [ ] **Step 3: Implementación**

En `internal/store/store.go`:

1. Agregar el import de probe:

```go
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/probe"
)
```

2. Agregar al final del struct `VersionView` (después de `Files`):

```go
	Files      []FileView `json:"files"`

	// Technical data read from the files (zero when not probed yet).
	DurationMs int64         `json:"durationMs"`
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	VideoCodec string        `json:"videoCodec"`
	Audio      []probe.Track `json:"audio"`
	Subs       []probe.Track `json:"subs"` // embedded subtitle tracks
	Best       bool          `json:"best"` // best of several versions of the same movie
}
```

3. En `Versions()`, cambiar el comentario de la función por:

```go
// Versions returns every version with its files and technical data, ordered
// by title and year.
```

4. En el primer bucle de `Versions()`, inicializar las pistas y normalizar el codec del nombre:

```go
		v := VersionView{Files: []FileView{}, Audio: []probe.Track{}, Subs: []probe.Track{}}
```

y, justo antes de `pos[v.ID] = len(out)`:

```go
		v.Codec = probe.CodecFromName(v.Codec)
```

5. Reemplazar el final de `Versions()` (desde `return out, frows.Err()`) por:

```go
	if err := frows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachMedia(out, pos); err != nil {
		return nil, err
	}
	markBest(out)
	return out, nil
}
```

(`frows` ya se cerró solo al terminar el bucle, así que la consulta de `attachMedia` no compite por la única conexión.)

`internal/store/technical.go`:

```go
package store

import (
	"encoding/json"

	"cinexplorer/internal/probe"
	"cinexplorer/internal/quality"
)

type mediaRow struct {
	kind       string
	durationMs int64
	width      int
	height     int
	videoCodec string
	audio      []probe.Track
	subs       []probe.Track
}

// attachMedia fills the technical fields of each version from the probe
// results of its present main files. Stale results (the file changed since)
// and failed ones are ignored, so the name-parsed values remain.
func (s *Store) attachMedia(out []VersionView, pos map[int64]int) error {
	rows, err := s.db.Query(`SELECT f.version_id, f.kind, m.duration_ms, m.width, m.height, m.video_codec, m.audio, m.subs
		FROM files f JOIN media m ON m.file_id = f.id
		WHERE f.version_id IS NOT NULL AND f.role = 'main' AND f.missing = 0 AND m.error = ''
		  AND m.size = f.size AND m.mtime = f.mtime
		ORDER BY f.version_id, f.part, f.path`)
	if err != nil {
		return err
	}
	defer rows.Close()
	byVersion := map[int64][]mediaRow{}
	var order []int64
	for rows.Next() {
		var id int64
		var m mediaRow
		var audio, subs string
		if err := rows.Scan(&id, &m.kind, &m.durationMs, &m.width, &m.height, &m.videoCodec, &audio, &subs); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(audio), &m.audio); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(subs), &m.subs); err != nil {
			return err
		}
		if _, seen := byVersion[id]; !seen {
			order = append(order, id)
		}
		byVersion[id] = append(byVersion[id], m)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range order {
		if i, ok := pos[id]; ok {
			applyMedia(&out[i], byVersion[id])
		}
	}
	return nil
}

// applyMedia merges the probe results of a version's parts, in part order.
func applyMedia(v *VersionView, ms []mediaRow) {
	if ms[0].kind == "dvd" {
		// A DVD holds several title sets; the longest one is the movie.
		longest := ms[0]
		for _, m := range ms[1:] {
			if m.durationMs > longest.durationMs {
				longest = m
			}
		}
		ms = []mediaRow{longest}
	}
	first := ms[0]
	v.Width, v.Height, v.VideoCodec = first.width, first.height, first.videoCodec
	for _, m := range ms {
		v.DurationMs += m.durationMs
		v.Audio = appendUnique(v.Audio, m.audio)
		v.Subs = appendUnique(v.Subs, m.subs)
	}
	if label := probe.ResolutionLabel(v.Width, v.Height); label != "" {
		v.Resolution = label
	}
	if v.VideoCodec != "" {
		v.Codec = v.VideoCodec
	}
}

func appendUnique(dst, src []probe.Track) []probe.Track {
	for _, t := range src {
		dup := false
		for _, d := range dst {
			if d == t {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, t)
		}
	}
	return dst
}

// markBest flags the best version in each group of two or more present
// versions that share a provisional identity (title + year). Full ties go to
// the lowest id, so the choice is stable.
func markBest(vs []VersionView) {
	best := map[string]int{}
	count := map[string]int{}
	for i := range vs {
		v := &vs[i]
		key := quality.GroupKey(v.Title, v.Year)
		if key == "" || !hasPresentMain(v) {
			continue
		}
		count[key]++
		j, ok := best[key]
		if !ok {
			best[key] = i
			continue
		}
		if c := quality.Compare(candidate(v), candidate(&vs[j])); c > 0 || (c == 0 && v.ID < vs[j].ID) {
			best[key] = i
		}
	}
	for key, i := range best {
		if count[key] > 1 {
			vs[i].Best = true
		}
	}
}

func candidate(v *VersionView) quality.Candidate {
	return quality.Candidate{Resolution: v.Resolution, Codec: v.Codec, Size: v.Size}
}

func hasPresentMain(v *VersionView) bool {
	for _, f := range v.Files {
		if f.Role == "main" && !f.Missing {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/store/ && go test ./internal/store/ -v && go test ./...`
Expected: PASS en todo el módulo (el servidor sigue compilando: los campos nuevos solo se agregan)

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): technical data per version and best version flag"
```

---

### Task 11: scan — fase de análisis

**Files:**
- Modify: `internal/scan/scan.go`
- Test: `internal/scan/probe_test.go`

- [ ] **Step 1: Test que falla**

`internal/scan/probe_test.go`:

```go
package scan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cinexplorer/internal/probe"
	"cinexplorer/internal/probe/probetest"
)

func TestScanProbesMainVideosOnce(t *testing.T) {
	disk, app, st := setup(t)
	dir := filepath.Join(disk, "cine", "Stalker (1979)")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Stalker.mkv"), probetest.MKV(1920, 1040, 9_720_000, "rus"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "Stalker.spa.srt"), 100, 's')
	writeFile(t, filepath.Join(dir, "Extras", "Trailer.mkv"), 10, 't')

	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st, Probe: probe.Prober{}.Probe}
	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := sc.Status(); got.ToProbe != 1 || got.Probed != 1 {
		t.Fatalf("status %+v", got)
	}
	vs, err := st.Versions()
	if err != nil || len(vs) != 1 {
		t.Fatalf("versions %+v, %v", vs, err)
	}
	if v := vs[0]; v.Resolution != "1080p" || v.DurationMs != 9_720_000 || v.Codec != "h264" || len(v.Audio) != 1 || v.Audio[0].Lang != "ru" {
		t.Fatalf("version %+v", v)
	}

	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := sc.Status(); got.ToProbe != 0 {
		t.Fatalf("second scan probed again: %+v", got)
	}
}

func TestScanRetriesOnlyReadErrors(t *testing.T) {
	disk, app, st := setup(t)
	writeFile(t, filepath.Join(disk, "cine", "a", "Locked.mkv"), 4096, 'l')
	writeFile(t, filepath.Join(disk, "cine", "b", "Broken.mkv"), 4096, 'b')
	calls := map[string]int{}
	fake := func(ctx context.Context, p string) (probe.Info, error) {
		calls[filepath.Base(p)]++
		if filepath.Base(p) == "Locked.mkv" {
			return probe.Info{}, fmt.Errorf("%w: sharing violation", probe.ErrIO)
		}
		return probe.Info{}, fmt.Errorf("%w: truncated file", probe.ErrInvalid)
	}
	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st, Probe: fake}
	for range 2 {
		if err := sc.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if calls["Locked.mkv"] != 2 || calls["Broken.mkv"] != 1 {
		t.Fatalf("calls %v", calls)
	}
}

func TestScanCancelKeepsProbedResults(t *testing.T) {
	disk, app, st := setup(t)
	for _, n := range []string{"a", "b", "c"} {
		writeFile(t, filepath.Join(disk, "cine", n, n+".mkv"), 4096, n[0])
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake := func(context.Context, string) (probe.Info, error) {
		cancel() // the drive goes away after the first file
		return probe.Info{Width: 640, Height: 480}, nil
	}
	sc := &Scanner{AppDir: app, Roots: []string{"../cine"}, Store: st, Probe: fake}
	if err := sc.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	pending, err := st.PendingProbes()
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending %+v, %v", pending, err)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/scan/`
Expected: FAIL — `unknown field Probe in struct literal of type Scanner`, `got.ToProbe undefined`

- [ ] **Step 3: Implementación**

En `internal/scan/scan.go`:

1. Import:

```go
	"cinexplorer/internal/mediafile"
	"cinexplorer/internal/probe"
	"cinexplorer/internal/store"
)
```

2. En `Status`, después de `Versions`:

```go
	Versions  int       `json:"versions"`
	ToProbe   int       `json:"toProbe"` // files whose headers this run reads
	Probed    int       `json:"probed"`
```

3. En `Scanner`, después de `Store`:

```go
	Store  *store.Store
	// Probe reads a file's technical data; nil means probe.Probe.
	Probe func(ctx context.Context, path string) (probe.Info, error)
```

4. Al final de `run`, reemplazar `return nil` (el que sigue a `s.status.Versions = len(versions)`) por `return s.probeAll(ctx)` y agregar después de `run`:

```go
// probeBatch is how many probe results are committed at once, so an
// interrupted run keeps most of its work.
const probeBatch = 50

// probeAll reads the headers of the files that are new or changed since they
// were last probed. Read failures are left for the next scan; format errors
// are stored so the file is not read again until it changes.
func (s *Scanner) probeAll(ctx context.Context) error {
	targets, err := s.Store.PendingProbes()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.status.ToProbe = len(targets)
	s.mu.Unlock()
	read := s.Probe
	if read == nil {
		read = probe.Probe
	}

	var batch []store.ProbeResult
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := s.Store.SaveProbes(batch)
		batch = batch[:0]
		return err
	}
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, flush())
		}
		info, err := read(ctx, appdir.Abs(s.AppDir, t.Path))
		r := store.ProbeResult{FileID: t.FileID, Size: t.Size, MTime: t.MTime, Info: info}
		switch {
		case err == nil:
			batch = append(batch, r)
		case ctx.Err() != nil:
			return errors.Join(ctx.Err(), flush())
		case errors.Is(err, probe.ErrIO):
			log.Printf("no se puede leer %s, se reintentará: %v", t.Path, err)
		default:
			r.Err = err.Error()
			batch = append(batch, r)
		}
		s.mu.Lock()
		s.status.Probed++
		s.mu.Unlock()
		if len(batch) >= probeBatch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go vet ./internal/scan/ && go test ./internal/scan/ -v`
Expected: PASS (los tests existentes + los 3 nuevos). Los tests viejos escriben archivos de relleno: el análisis los marca con error de formato (o los manda a `ffprobe` si está instalado, que también falla) y el escaneo termina igual.

- [ ] **Step 5: Commit**

```bash
git add internal/scan
git commit -m "feat(scan): probe new and changed videos after grouping"
```

---

### Task 12: server — datos técnicos en la API y en la página

**Files:**
- Modify: `internal/server/server_test.go` (agregar 2 tests al final)
- Modify: `internal/server/web/index.html`

La API no necesita código nuevo: `/api/versions` serializa `VersionView` y `/api/status` serializa `scan.Status`. Los tests fijan el contrato JSON que va a usar la interfaz de la Etapa 4.

- [ ] **Step 1: Tests**

Agregar al final de `internal/server/server_test.go`:

```go
func TestVersionsEndpointIncludesTechnicalFields(t *testing.T) {
	s, _ := newServer(t)
	rec := request(s.Handler(), "GET", "/api/versions", "", "", "127.0.0.1:8080")
	var vs []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &vs); err != nil || len(vs) != 1 {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	for _, k := range []string{"durationMs", "width", "height", "videoCodec", "best"} {
		if _, ok := vs[0][k]; !ok {
			t.Errorf("missing %q in %v", k, vs[0])
		}
	}
	for _, k := range []string{"audio", "subs"} {
		if _, ok := vs[0][k].([]any); !ok {
			t.Errorf("%q must be a JSON array, got %v", k, vs[0][k])
		}
	}
}

func TestStatusReportsProbeProgress(t *testing.T) {
	s, _ := newServer(t)
	s.Scanner = &scan.Scanner{}
	rec := request(s.Handler(), "GET", "/api/status", "", "", "127.0.0.1:8080")
	var body struct {
		Scan map[string]any `json:"scan"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body.Scan["toProbe"]; !ok {
		t.Fatalf("status %s", rec.Body)
	}
	if _, ok := body.Scan["probed"]; !ok {
		t.Fatalf("status %s", rec.Body)
	}
}
```

- [ ] **Step 2: Verificar**

Run: `go test ./internal/server/ -v -run 'Technical|ProbeProgress'`
Expected: PASS (las Tasks 10 y 11 ya agregaron los campos; si falla, alguna de esas tareas quedó incompleta)

- [ ] **Step 3: Página**

En `internal/server/web/index.html`:

1. En el `<style>`, después de la regla `.empty`:

```css
  .best { font-style: normal; font-size: 10px; font-weight: 700; letter-spacing: .08em; color: var(--bg);
          background: #3fb950; border-radius: 3px; padding: 1px 6px; margin-left: 6px; vertical-align: 2px; }
```

2. Después de `const gb = …`:

```js
const codecNames = { h264: 'H.264', hevc: 'HEVC', av1: 'AV1', vp9: 'VP9', vp8: 'VP8', mpeg4: 'MPEG-4', mpeg2: 'MPEG-2', mpeg1: 'MPEG-1', wmv: 'WMV', rv: 'RealVideo' };
const codecName = (c) => codecNames[c] || c.toUpperCase();
const duration = (ms) => { const m = Math.round(ms / 60000); return m >= 60 ? `${Math.floor(m / 60)} h ${m % 60} min` : `${m} min`; };
const channels = (n) => ({ 1: '1.0', 2: '2.0', 6: '5.1', 8: '7.1' }[n] || (n ? n + ' can.' : ''));
const track = (t) => [t.lang || '?', channels(t.channels), t.codec].filter(Boolean).join(' ');
```

3. Reemplazar la función `versionRow` completa por:

```js
function versionRow(v) {
  const main = v.files.find((f) => f.role === 'main');
  const tech = [v.resolution, v.source, v.codec && codecName(v.codec)].filter(Boolean).map(esc);
  if (v.durationMs) tech.push(duration(v.durationMs));
  if (v.parts > 1) tech.push(v.parts + ' partes');
  tech.push(gb(v.size));
  const media = [];
  if (v.audio.length) media.push('audio ' + esc(v.audio.map(track).join(', ')));
  const subs = [...new Set([...v.subs.map((t) => t.lang || '?'), ...(v.subLangs ? v.subLangs.split(',') : [])])];
  if (subs.length) media.push('subs ' + esc(subs.join(', ')));
  return `<div class="row"><div>
      <div class="t">${esc(v.title)}${v.best ? '<em class="best">MEJOR</em>' : ''} <span>${v.year || ''} ${v.director ? '· ' + esc(v.director) : ''}</span></div>
      <div class="meta">${tech.join(' · ')}</div>
      ${media.length ? `<div class="meta">${media.join(' · ')}</div>` : ''}
      <div class="path">${esc(v.dir)}</div>
    </div><div class="acts">${main ? `<button data-open="${esc(main.path)}">▶ Ver</button><button data-reveal="${esc(main.path)}">Carpeta</button>` : ''}</div></div>`;
}
```

4. En `poll()`, agregar el estado de análisis antes del de escaneo:

```js
      : sc.running && sc.toProbe ? `Analizando… ${sc.probed}/${sc.toProbe}`
      : sc.running ? `Escaneando… ${sc.files} archivos`
```

- [ ] **Step 4: Verificar todo**

Run: `gofmt -l ./internal ./cmd; go vet ./... && go test ./...`
Expected: sin salida de `gofmt` y `ok` en todos los paquetes.

- [ ] **Step 5: Commit**

```bash
git add internal/server
git commit -m "feat(server): show technical data, best version and probe progress"
```

---

### Task 13: README y verificación con la colección real

No escribe nada en `D:\cine` ni en `D:\cine-ordenar`: la app solo escribe en `.run/` (ignorado por git).

**Files:**
- Modify: `README.md`

- [ ] **Step 1: README**

En `README.md`, después del párrafo del modo consulta, agregar:

```markdown
Además de los nombres, la app lee los encabezados de los videos (MKV, MP4/MOV,
AVI e IFO de DVD) para conocer resolución, codecs, duración y pistas de audio y
subtítulos. Si `ffprobe` está instalado y en el PATH, se usa para los formatos
que no lee por su cuenta (RMVB, MPG, WMV…); no es obligatorio.
```

Y en "Desarrollo", después de `go test ./...`:

```bash
CINEXPLORER_PROBE_CORPUS=D:/cine go test ./internal/probe -run Corpus -v -timeout 0
```

- [ ] **Step 2: Ejecutar con la colección real**

Si `.run/config.json` no existe (se crea en la Task 14 de la Etapa 1):

```bash
mkdir -p .run
printf '{\n  "roots": ["../../../cine", "../../../cine-ordenar"],\n  "tmdbToken": "",\n  "language": "es-ES"\n}\n' > .run/config.json
```

Run: `go run ./cmd/cinexplorer -dir .run`
Expected: el estado pasa por "Escaneando… N archivos" y luego "Analizando… n/m" (unos 1.900 videos la primera vez) y termina en "N versiones".

- [ ] **Step 3: Verificar a mano**

- Las versiones muestran resolución, codec, duración (`1 h 52 min`) y audio (`es 5.1 ac3`).
- Una película con dos versiones (buscar en "Copias idénticas" o por título repetido) tiene la marca verde **MEJOR** en la de mayor resolución.
- `Soy Cuba` (un `.avi` que en realidad es Matroska) tiene datos técnicos.
- Un segundo arranque no vuelve a analizar nada (el estado nunca muestra "Analizando…", o muestra `0/0` por un instante).

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: technical data and optional ffprobe"
```
