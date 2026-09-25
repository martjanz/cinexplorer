# Cinexplorer — Diseño (v1)

Fecha: 2026-09-25
Estado: aprobado en brainstorming, pendiente de revisión escrita

## 1. Objetivo

Herramienta local para navegar, explorar y agrupar una colección de películas repartida en uno o más directorios, independientemente de cómo estén organizadas las carpetas en disco. Permite agrupar por múltiples criterios (año, director, género, país, colección, listas propias) y detectar duplicados y versiones múltiples de una misma película.

### Alcance v1

- **Solo lectura** sobre los archivos de video: nunca mueve, renombra ni borra.
- Catálogo con metadatos de **TMDB** (fuente principal) y **Wikidata** (complemento).
- **Listas y etiquetas propias** del usuario, e importación de carpetas `Collections/` como listas.
- Detección de duplicados (3 tipos, ver §3.3).

### Fuera de alcance v1 (explícito)

- Acciones sobre archivos (mover, renombrar, descartar duplicados) → fase 2.
- Datos de visionado (visto, puntaje, notas), importación de Letterboxd.
- Reproducción dentro del navegador (▶ abre el reproductor del sistema).
- Multiusuario.

### Datos de referencia

Colección actual: `D:\cine` (organizado por década `1900s…2020s` + `Collections/<director o lista>`, ~3.300 archivos) y `D:\cine-ordenar` (mezclado, ~1.300 archivos). Nombres heterogéneos: `Amarcord [Federico Fellini, 1973]`, `Chinatown (Polanski, USA, 1974)`, `Arabian.Nights.1974.CC.1080p.BluRay.FLAC1.0.x264-ADE.mkv`, `El.Castillo.Ambulante.Spanish.XviD.AC3.DVDRip.By.FreAk.TEAm.avi`. Hay películas en partes, extras, DVDs en `VIDEO_TS`, subtítulos externos (`.srt`, `.sub/.idx`) y archivos basura (`.nfo`, `.txt` de trackers, `.DS_Store`, `Thumbs.db`, `sync.ffs_db`).

## 2. Arquitectura y portabilidad

### 2.1 Forma de distribución

Un único ejecutable por plataforma, escrito en **Go**, con el frontend embebido. Al ejecutarse levanta un servidor HTTP en `127.0.0.1` (puerto libre) y abre el navegador predeterminado. Sin instalador ni runtime.

### 2.2 Estructura en disco (disco extraíble, exFAT)

```
<raíz del disco>/
├── cine/
├── cine-ordenar/
└── cinexplorer/
    ├── cinexplorer-windows.exe
    ├── cinexplorer-macos          (binario universal amd64+arm64)
    ├── cinexplorer-linux
    ├── cinexplorer.db             (SQLite: catálogo, listas, etiquetas, correcciones)
    ├── cache/posters/…            (imágenes descargadas de TMDB)
    └── config.json                (raíces, API key TMDB, idioma)
```

- El directorio de datos es **siempre el directorio del ejecutable**.
- Todas las rutas se guardan **relativas al directorio de la app**, con separador `/` (p. ej. `../cine/1970s/…`). El mismo catálogo funciona en `D:\` (Windows), `/Volumes/X` (macOS) y `/media/…` (Linux).
- `config.json` ejemplo: `{"roots": ["../cine", "../cine-ordenar"], "tmdbToken": "…", "language": "es-ES"}`.
- SQLite en modo journal `DELETE` (no WAL), por robustez ante desconexión del disco.
- **Modo consulta:** si el directorio de la app no es escribible, la app funciona en solo-lectura del catálogo existente y lo indica en la UI.

### 2.3 Stack

- Backend: Go, `modernc.org/sqlite` (Go puro, sin cgo → cross-compile trivial), FTS5 para búsqueda, API JSON.
- Frontend: Svelte + Vite, compilado y embebido con `go:embed`. Fuente Inter embebida (sin dependencia de red).
- Seguridad: escucha solo en loopback.

## 3. Modelo de datos

### 3.1 Entidades

- **Película** — identidad = id de TMDB. Título (localizado), título original, año, directores, reparto principal, géneros, países, idioma original, duración, sinopsis, colección TMDB, afiche, imagen de escena, id Wikidata/IMDb. Relación con listas y etiquetas del usuario.
- **Versión** — conjunto de archivos que se ven como una unidad (partes `Part 1/2`, `CD1/CD2`, `Disc N`, carpeta `VIDEO_TS`). Pertenece a una película (o está sin identificar). Atributos técnicos: resolución, codec de video, origen (BluRay/WEB-DL/DVDRip…), duración, pistas de audio (idioma), subtítulos internos y externos (idioma), tamaño total.
- **Archivo** — ruta relativa, tamaño, mtime, huella de contenido, tipo (`video`, `subtitulo`, `extra`, `basura`, `otro`), estado (`presente`, `no encontrado`). Pertenece a una versión (videos, subs) o a una película (extras).
- **Lista** — nombre, orden, películas (ordenadas). **Etiqueta** — texto libre, N:M con películas.
- **Corrección** — asociación manual huella → id TMDB (o "ignorar", o "extra de <película>").

### 3.2 Huella de contenido

`hash(primer MB + último MB + tamaño)`. Permite reconocer un archivo movido/renombrado (preserva correcciones y asignaciones) y detectar copias idénticas.

### 3.3 Tipos de duplicado

1. **Copia idéntica** — misma huella en dos o más rutas.
2. **Varias versiones** — mismo id TMDB, versiones con huellas distintas.
3. **Sospecha** — sin identificar, pero título normalizado + año (o título solo, si no hay año) coinciden con otra entrada.

### 3.4 Agrupación

Agrupar = filtrar por facetas combinables: década/año, director, género, país, idioma original, colección TMDB, lista, etiqueta, resolución, idioma de subtítulos disponible, ubicación (raíz/carpeta), estado (sin identificar / con duplicados).

## 4. Pipeline de catalogación

Corre en segundo plano con progreso visible en la UI. Incremental: un archivo solo se reprocesa si cambió (ruta, tamaño, mtime).

1. **Escaneo** — recorre las raíces; clasifica por extensión; ignora basura conocida (`.DS_Store`, `Thumbs.db`, `sync.ffs_db`, `desktop.ini`); calcula huella de archivos nuevos o modificados.
2. **Armado de versiones** — agrupa partes por patrón (`part|cd|disc|pt` + número) en la misma carpeta; trata `VIDEO_TS` como una versión; asocia subtítulos por prefijo de nombre, misma carpeta o subcarpeta `Subs/`, e infiere idioma por sufijo (`.es.srt`, `Spanish.srt`, `eng`…); marca extras por palabras clave (`bonus`, `extras`, `featurette`, `trailer`, `sample`, `making of`) o por tamaño < 15% del video principal de la carpeta.
3. **Parseo de nombre** — sobre nombre de archivo y de carpeta; se usa el resultado más informativo. Extrae título, año, director (patrones `[Director, Año]`, `(Director, País, Año)`, `Título_Director`), resolución, origen, codec, idioma, grupo de release. Limpia ruido (`www.…`, `emule.via.…`, `clan-sudamerica.net`, `[YTS.MX]`…). Si hay un `.nfo` con id IMDb (`tt\d+`), se usa para identificación exacta.
4. **Datos técnicos** — lectura nativa de headers MKV (EBML), MP4 (`moov`) y AVI (RIFF): resolución, codecs, duración, pistas de audio/subs con idioma. Si `ffprobe` está en el PATH se usa como alternativa; nunca es requisito.
5. **Identificación** — TMDB search por título + año, en idioma configurado y título original. Puntaje de confianza = similitud de título + coincidencia de año (±1) + coincidencia de director si se extrajo. Por encima del umbral: asignación automática. Debajo: cola **Revisar → Sin identificar** con candidatos. Correcciones manuales se atan a la huella.
6. **Enriquecimiento** — TMDB details + credits en idioma configurado, fallback `en-US`. Wikidata completa huecos vía id TMDB (P4947) / IMDb (P345). Imágenes descargadas una sola vez a `cache/` (afiche w342, escena w1280).

### 4.1 Importación de `Collections/`

Cada subcarpeta de una carpeta llamada `Collections` se propone como lista (Revisar → Colecciones). El usuario confirma o descarta cada una; no se importan automáticamente.

### 4.2 Errores

- Sin red: escaneo, parseo y datos técnicos funcionan; identificación/enriquecimiento quedan pendientes y se reintentan.
- Archivo ilegible/corrupto: se marca, se registra, no detiene el pipeline.
- Disco desconectado durante el escaneo: se aborta; la base queda en el último estado consistente (transacciones por lote).
- Archivos desaparecidos: estado `no encontrado`, no se borran (listas y correcciones se preservan). Se filtran de la vista por defecto.
- Límite de TMDB: cliente con rate limiting y reintentos con backoff.

## 5. Interfaz

Referencias estéticas: MUBI y Letterboxd. Tema oscuro, tipografía Inter, títulos en mayúsculas, imagen primero, sin configuración a la vista.

Navegación superior: **Inicio · Explorar · Listas · Revisar** + búsqueda (⌕).

### 5.1 Inicio (estilo MUBI)

Filas de imágenes de escena (16:9) con título en mayúsculas y línea "DIRECTOR PAÍS AÑO". Filas autogeneradas, algunas rotan por visita: agregadas recientemente, listas del usuario (más recientes primero), década al azar, director al azar con ≥3 películas, país o género al azar, colecciones TMDB. Cada fila tiene "ver todas →" que abre Explorar con esa faceta aplicada.

### 5.2 Explorar (estilo Letterboxd)

Grilla densa de afiches (2:3). Barra discreta de facetas en desplegables (Década, Director, Género, País, Lista, Etiqueta, Resolución, Subs, Ubicación, Estado) + contador + orden (año, título, agregado, tamaño). Facetas reflejadas en la URL (enlazables, navegación atrás/adelante).

### 5.3 Ficha de película

Cabecera con imagen de escena, título en mayúsculas, título original · año, director · países · duración · géneros. Afiche superpuesto, sinopsis, chips de listas/etiquetas con "+" para agregar.

**Versiones en disco en tarjetas** (una por versión): resolución grande, origen · codec · partes, audio, subs, tamaño, ruta relativa, botones **▶ Ver** (abre el archivo —o la primera parte— con la aplicación predeterminada del SO) y **Carpeta** (lo muestra en el explorador de archivos). Indicadores: verde "mejor" (mayor resolución; desempate por codec más moderno y luego tamaño), ámbar "copia idéntica". Extras listados aparte.

### 5.4 Listas

Índice de listas del usuario; cada lista abre su grilla. Crear, renombrar, reordenar, borrar listas (datos del usuario, no archivos).

### 5.5 Revisar

Tres pestañas:
- **Sin identificar** — por ítem: nombre y ruta, tokens parseados, candidatos TMDB (afiche, título, director, año, % confianza), búsqueda manual, "no es una película", "es un extra de…". Un clic confirma.
- **Duplicados** — películas con más de una versión o copias idénticas, con tipo marcado, ordenadas por espacio recuperable; total recuperable arriba. Clic → ficha. Sin acciones destructivas en v1.
- **Colecciones** — carpetas de `Collections/` pendientes de importar como lista o descartar.

### 5.6 Búsqueda

Instantánea mientras se escribe, sobre título localizado, título original, director y reparto; insensible a tildes y mayúsculas (FTS5 con normalización).

### 5.7 Primer uso

Asistente de 3 pasos: confirmar raíces (propone las carpetas hermanas de `cinexplorer/`), pegar token de TMDB (con enlace a dónde obtenerlo), idioma de metadatos (`es-ES` por defecto). Luego arranca el escaneo y el Inicio se va poblando.

## 6. Pruebas

- **Parser de nombres**: tests de tabla con corpus de nombres reales de la colección y resultado esperado; cada fallo en uso real se agrega al corpus.
- **Armado de versiones**: árboles de carpetas sintéticos (partes, VIDEO_TS, extras, `Subs/`).
- **Headers MKV/MP4/AVI**: archivos de muestra mínimos generados para test.
- **Identificación**: respuestas TMDB/Wikidata grabadas (sin red en tests) para calibrar el puntaje de confianza.
- **Portabilidad**: rutas relativas y separadores; CI compila y corre tests en Windows, macOS y Linux.
- **API/UI**: tests de endpoints con una base de prueba; smoke test end-to-end del binario sobre un árbol sintético.
