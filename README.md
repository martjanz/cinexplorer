# Cinexplorer

**English** · [Español](README.es.md)

![Cinexplorer home: rows of films with scene images](docs/images/home.jpg)

A local explorer for your movie collection. Point it at your folders and it
builds a catalog: which film each file is, which versions of it you have, their
quality, subtitles and duplicates. Then it lets you browse it all in your web
browser. It doesn't matter how the folders are organized on disk.

- **Read-only:** it never moves, renames or deletes video files. The only thing
  it writes is its own catalog, next to the executable.
- **Portable:** one executable per OS, no installer, no dependencies. The
  catalog stores relative paths, so the same external drive works on Windows,
  macOS and Linux.
- **Local:** the server listens only on `127.0.0.1`. The network is used only
  to query TMDB and Wikidata (optional).

> **Language note:** the interface is currently in Spanish. Movie data (titles,
> synopses, genres) can be fetched in English, Spanish or Portuguese; see
> [Configuration](#configuration).

## Features

- **Home** (*Inicio*): MUBI-style rows of scene images: recently added films and, shuffled
  on every visit, a decade, a director, a country, a genre, a collection and a
  random pick. Each row links to Explore with that filter; ↻ shuffles again.
- **Search** (magnifier icon, or Ctrl+K / ⌘K): results as you type, by title,
  original title, director, cast or file name, ignoring accents and case. Enter
  opens a page with all results.
- **Explore** (*Explorar*): the whole collection as a poster grid, with combinable facets
  (decade and year, director, genre, country, resolution, subtitles, original
  language, collection, disk location and status) and sorting by year, title,
  date added or size. Filters live in the URL, so you can bookmark them and the
  back button works. Unidentified files still show up, with a generic poster.
- **Movie page:** TMDB data and, below it, every version on disk with its
  quality, audio and subtitles, **BEST** and **IDENTICAL COPY** badges, and
  ▶ Play / Folder buttons. Each version's ⋯ menu fixes its identification.
- **Review** (*Revisar*): the **Unidentified** queue (candidates, manual search, "not a
  movie", "extra of…", with keyboard shortcuts) and **Duplicates** (identical
  copies and multiple versions of a film, with the space you could reclaim).
- **Settings** (*Ajustes*, gear icon): folders, TMDB token, data language and image
  downloads. Changes apply without restarting.

A status indicator in the top right shows scanning and identification in
progress; pages refresh on their own as they advance.

## Requirements

To use it:

- Nothing mandatory: the executable is self-contained.
- **A TMDB token** (free), to identify movies. Create an account at
  [themoviedb.org](https://www.themoviedb.org/), go to *Settings → API* and copy
  the **API Read Access Token**: the long one starting with `eyJ…`, not the
  short "API Key". Without a token the app still works, it just doesn't
  identify anything.
- **`ffprobe`** (optional, part of [FFmpeg](https://ffmpeg.org/)): if it's on
  your `PATH`, it's used for formats the app can't read on its own (RMVB, MPG,
  WMV…) or when a header can't be parsed. If you install it later, restart the
  app; files that failed before are re-analyzed automatically.

To build it: Go 1.27+; Node 22.12+ only if you want to change the interface
(see [Development](#development)).

## Installation

The intended layout is a drive (internal or external) with your movies and, next
to them, the app folder:

```
<drive root>/
├── movies/                     ← your films, organized however you like
├── movies-to-sort/
└── cinexplorer/
    ├── cinexplorer-windows.exe
    ├── cinexplorer-macos       (universal binary, Intel + Apple Silicon)
    ├── cinexplorer-linux
    ├── config.json             (created on first run)
    ├── cinexplorer.db          (the catalog, SQLite)
    └── cache/                  (TMDB posters and images)
```

1. Copy the `cinexplorer/` folder to the root of the drive, next to your movie
   folders.
2. Run the binary for your system:
   - Windows: `cinexplorer-windows.exe`
   - macOS: `cinexplorer-macos`. The first time, right-click → Open, since it's
     not signed. If macOS still blocks it:
     `xattr -d com.apple.quarantine cinexplorer-macos`.
   - Linux: `./cinexplorer-linux` (you may need `chmod +x` first).
3. Your browser opens a three-step setup wizard:
   - **Folders:** it suggests the ones next to `cinexplorer/` (skipping hidden
     and system folders like `$RECYCLE.BIN` or `System Volume Information`);
     you can uncheck them or add others by typing a path.
   - **TMDB token:** paste it and verify it (see [Requirements](#requirements)).
     You can skip it and add it later in Settings.
   - **Language** for titles and synopses: Spanish (Argentina), English (US) or
     Portuguese (Brazil).
4. When you finish, `config.json` is saved and scanning starts; Home fills in as
   movies are identified. The first scan analyzes everything; later ones only
   what changed.

To quit, close the terminal window (or press Ctrl+C).

## Configuration

Everything can be changed from **Settings** in the app. It's saved in
`config.json`, next to the executable:

```json
{
  "roots": ["../movies", "../movies-to-sort"],
  "tmdbToken": "eyJ…",
  "language": "en-US",
  "imagePrefetch": "none"
}
```

| Field | What it is |
|---|---|
| `roots` | Folders to scan, **relative to the app folder** and using `/` as separator (they must be on the same drive as the app). A root that isn't available (say, an unplugged drive) is skipped without touching what's already in the catalog. Files from a removed root become "not found". |
| `tmdbToken` | TMDB read access token (v4). Empty: no identification. |
| `language` | Language for titles, synopses and genres: `es-AR` (default), `en-US` or `pt-BR`. Whatever TMDB lacks in that variant is taken from the closest one (for `es-AR`: `es-MX`, then `es-ES`), and synopses fall back to English as a last resort. Changing it re-fetches the data in the new language. |
| `imagePrefetch` | `"none"` (default): images are downloaded the first time they're shown. `"posters"`: posters are downloaded ahead of time. `"all"`: posters and scene images ahead of time, to use the app offline; that's several hundred MB for a few thousand films. |

### Command line

| Option | Description |
|---|---|
| `-dir <path>` | App folder (config, catalog, cache). Defaults to the executable's folder. Can also be set with the `CINEXPLORER_DIR` environment variable. |
| `-port <n>` | HTTP port. `0` (default) picks a free one. |
| `-no-browser` | Don't open the browser on startup. Without `config.json`, the app waits for the setup wizard at the address it prints. |

## How it works

Each scan runs in the background (the page shows progress) and is incremental:
a file is only reprocessed if its size or modification time changed.

1. **Scan:** walks the roots and classifies files by extension (video, DVD,
   subtitle, `.nfo`, known junk like `Thumbs.db` or `.DS_Store`).
2. **Content fingerprint:** a hash of the first and last MB plus the size. It
   recognizes moved or renamed files and detects identical copies. Empty
   (0-byte) files have no fingerprint.
3. **Versions:** groups parts (`CD1`/`CD2`, `Part 1`…), treats each `VIDEO_TS`
   as a version, attaches external subtitles (inferring the language from the
   name) and separates extras (trailers, making-ofs, small files).
4. **Names:** extracts title, year, director, country, resolution, source and
   codec from wildly varied names (`Amarcord [Federico Fellini, 1973]`,
   `Chinatown (Polanski, USA, 1974)`, `Arabian.Nights.1974.1080p.BluRay.x264-ADE`),
   and strips download-site noise.
5. **Technical data:** reads MKV/WebM, MP4/MOV, AVI and DVD IFO headers without
   external tools (resolution, codecs, duration, audio and subtitle tracks with
   language). It only reads headers, not the whole file.
6. **Identification:** looks up each version on TMDB and computes a confidence
   score from title, year and director. It tries variants (the title in
   parentheses, the English title, the folder name) and uses the IMDb id from a
   `.nfo` if there is one. Above the threshold, the match is automatic;
   otherwise the version goes to **Unidentified** with candidates. On a sample
   of a real collection, nearly 8 in 10 are identified automatically, with no
   false positives.
7. **Enrichment:** TMDB data (titles, directors, cast, genres, countries,
   synopsis, collection, posters) and, from Wikidata, whatever TMDB is missing.

When a film has several versions, the **best** one is marked: highest
resolution, then the most modern codec, then size.

### Corrections

Manual decisions (confirming a candidate, "not a movie", "extra of…") are tied
to the file's **fingerprint**, not its path: they survive moving or renaming the
file, and apply to all its identical copies. Automatic identification never
overrides a correction.

### Offline

Scanning, name parsing and technical data don't need the network.
Identification stays pending and retries on its own: after 5 minutes, then
doubling the wait up to once an hour. It also retries on every new scan.

### Browse-only mode

If the app folder isn't writable (for example, an NTFS drive on macOS, which
mounts it read-only), the app still starts and shows the existing catalog
without changing it. Images are shown but not cached.

## Privacy and network

- **TMDB** receives the titles and years parsed from file names, IMDb ids and
  the TMDB ids of identified films, along with your token. Verifying the token
  in the wizard or Settings requests a well-known film (*Fight Club*).
- **Wikidata** receives TMDB ids (anonymous SPARQL queries).
- Images are downloaded from `image.tmdb.org`.
- Nothing else is sent: no paths, no full folder names, no technical data. No
  telemetry.
- The server only answers requests addressed to `127.0.0.1` / `localhost`, and
  rejects those coming from other sites' pages, so a random website can't query
  your catalog through your browser.
- The TMDB token is stored in plain text in `config.json`; keep that in mind if
  you share the drive.

## Generated files

Everything lives in the app folder; deleting it doesn't affect your movies.

- `cinexplorer.db`: the catalog (SQLite, `DELETE` journal mode, which handles a
  drive disconnect better than WAL). If you delete it, the next scan rebuilds it
  from scratch, but manual corrections are lost.
- `cache/posters/`, `cache/backdrops/`: TMDB images. Safe to delete; they're
  downloaded again.
- `config.json`: the configuration.

## API

The interface uses a local JSON API that also works for scripts. `POST`
requests require `Content-Type: application/json`. Some parameter names are in
Spanish (`decada` = decade, `anio` = year, `pais` = country, `orden` = sort…).

| Method and path | What it does |
|---|---|
| `GET /api/status` | Scan and identification status, and whether first-run setup is pending (`setupPending`). |
| `GET /api/home?seed=` | Home rows (the same seed yields the same rows). |
| `GET /api/search?q=&limit=` | Catalog search: matching items and directors. |
| `GET /api/config`, `PUT /api/config` | Settings (the token is never returned). On `PUT`, a missing or `null` `token` keeps the saved one and `""` clears it. |
| `POST /api/config/root`, `POST /api/config/token` | Validate a folder, or verify a token with TMDB. |
| `GET /api/explore?decada=&anio=&director=&genero=&pais=&idioma=&coleccion=&resolucion=&subs=&ubicacion=&estado=&orden=&dir=` | Explore items matching the facets, plus each facet's values with counts. |
| `GET /api/movies/{tmdbId}` | A movie's page: data, versions and extras. |
| `GET /api/movies?q=&near=` | Catalog movies for "extra of…". |
| `GET /api/versions/{fingerprint}` | Page for a version that isn't (yet) a catalog movie. |
| `GET /api/versions` | All versions with files, technical data, identification and movie. |
| `GET /api/unidentified` | Unidentified queue with candidates, and how many are awaiting identification. |
| `GET /api/duplicates` | Identical copies and multiple versions, with reclaimable space. |
| `GET /api/tmdb/search?q=&year=` | Manual TMDB search (`q` can be an IMDb id `tt…`). |
| `POST /api/identify` | Correction: `{"fingerprint", "action": "movie"\|"extra"\|"ignore"\|"reset", "tmdbId"}`. |
| `POST /api/scan` | Start a scan. |
| `POST /api/open`, `POST /api/reveal` | Open a catalog file with the system app, or reveal its folder. |
| `GET /img/{poster\|backdrop}/{tmdbId}.jpg` | Images from the cache (`?v=` identifies the image version). |

## Development

Requirements: **Go 1.27+**. No cgo needed: SQLite is
[`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite), pure Go, so
cross-compiling is straightforward.

The interface (Svelte 5 + Vite, in `web/`) is built into `internal/server/dist/`,
which is committed and embedded in the binary: building or testing the Go code
doesn't need Node. Changing the interface requires **Node 22.12+**.

```bash
go test ./...                          # full suite (no network)
go run ./cmd/cinexplorer -dir .run     # run the app with config and catalog in .run/
bash scripts/build.sh                  # build the interface and all three binaries into dist/cinexplorer/
```

To work on the interface with hot reload:

```bash
go run ./cmd/cinexplorer -dir .run -port 8080 -no-browser   # the API, in one terminal
cd web && npm install && npm run dev                        # the interface at http://localhost:5173
npm test                                                    # interface logic tests (Vitest)
npm run build                                               # updates internal/server/dist/ (commit it)
```

`scripts/build.sh` builds for Windows (amd64), Linux (amd64) and macOS (amd64 +
arm64), merging the two macOS binaries into a universal one with
[`makefat`](https://github.com/randall77/makefat), fetched via `go run` (needs
network the first time). CI (GitHub Actions) runs `go vet` and `go test` on
Windows, macOS and Linux, and in a separate job the interface tests and build,
which must match the committed one.

Tests against real data (optional, not in CI):

```bash
# Compare the built-in readers against ffprobe over a real collection
CINEXPLORER_PROBE_CORPUS=D:/movies go test ./internal/probe -run Corpus -v -timeout 0

# Re-record TMDB responses for the calibration corpus (30 real names).
# Token in CINEXPLORER_TMDB_TOKEN or ~/.cinexplorer-tmdb-token.
CINEXPLORER_TMDB_RECORD=1 go test ./internal/identify -run Corpus -v
```

### Layout

```
cmd/cinexplorer/      entry point: config, catalog, scanner, identification, server
internal/appdir       app folder and portable relative paths
internal/config       config.json
internal/scan         incremental scan and technical analysis
internal/fingerprint  content fingerprint
internal/mediafile    file classification by name
internal/grouping     version building (parts, DVD, subtitles, extras)
internal/nameparse    file and folder name parser
internal/probe        MKV/MP4/AVI/IFO header readers and ffprobe
internal/quality      best version and title normalization
internal/store        SQLite catalog
internal/httpx        HTTP with rate limiting and retries
internal/tmdb         TMDB client
internal/wikidata     Wikidata client (SPARQL)
internal/images       image cache
internal/identify     scoring, search, enrichment and the background worker
internal/engine       everything that depends on config.json (TMDB, scan, identification), hot-swappable
internal/catalog      items, facets, movie pages, duplicates and Home rows
internal/search       search index (in-memory FTS5)
internal/server       JSON API and the embedded interface (internal/server/dist)
internal/platform     open files and folders with the system
web/                  the interface (Svelte 5 + Vite)
```

Design docs and plans for each stage are in `docs/superpowers/specs/` and
`docs/superpowers/plans/` (in Spanish).

## Roadmap

| Stage | Scope | Status |
|---|---|---|
| 1. Core | scanning, name parser, versions, SQLite catalog, API | ✅ |
| 2. Technical data | MKV/MP4/AVI/IFO header reading, optional `ffprobe`, best version | ✅ |
| 3. Identification | TMDB + Wikidata, confidence score, corrections, posters | ✅ |
| 4a. Browsable catalog | Svelte interface: faceted Explore, movie page, Review | ✅ |
| 4b. Discovery | Home, instant search, setup wizard, Settings | ✅ |
| 5. Curation | lists, tags, importing `Collections/` | planned |

## Troubleshooting

The interface is in Spanish; the messages below are shown with their English
meaning.

- **"Falta el token de TMDB" / "Token de TMDB inválido"** (missing / invalid
  TMDB token): add or change it in Settings; it must be the *API Read Access
  Token* (v4), not the short API Key.
- **"Sin conexión: se reintenta más tarde"** (offline, will retry later): TMDB
  or Wikidata didn't respond. The app retries on its own; everything else keeps
  working meanwhile.
- **A movie is misidentified or unidentified:** in Review → Unidentified,
  confirm the right candidate or search manually by title or IMDb id
  (`tt0071129`). A wrong identification can be fixed from the version's ⋯ menu
  on the movie page.
- **Files don't show up:** check the folders in Settings.
- **"Modo consulta (solo lectura)"** (browse-only mode): the app folder isn't
  writable. See [Browse-only mode](#browse-only-mode).

## Credits

Movie data and images come from [TMDB](https://www.themoviedb.org/).
This product uses the TMDB API but is not endorsed or certified by TMDB.

Supplementary data comes from [Wikidata](https://www.wikidata.org/) (CC0).
