CREATE TABLE IF NOT EXISTS versions (
  id            INTEGER PRIMARY KEY,
  dir           TEXT    NOT NULL,
  title         TEXT    NOT NULL,
  year          INTEGER NOT NULL DEFAULT 0,
  director      TEXT    NOT NULL DEFAULT '',
  countries     TEXT    NOT NULL DEFAULT '',
  resolution    TEXT    NOT NULL DEFAULT '',
  source        TEXT    NOT NULL DEFAULT '',
  codec         TEXT    NOT NULL DEFAULT '',
  language      TEXT    NOT NULL DEFAULT '',
  release_group TEXT    NOT NULL DEFAULT '',
  imdb_id       TEXT    NOT NULL DEFAULT '',
  size          INTEGER NOT NULL,
  parts         INTEGER NOT NULL,
  sub_langs     TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS files (
  id          INTEGER PRIMARY KEY,
  path        TEXT    NOT NULL UNIQUE,  -- relative to the app dir, '/'-separated
  size        INTEGER NOT NULL,
  mtime       INTEGER NOT NULL,         -- unix milliseconds
  fingerprint TEXT    NOT NULL DEFAULT '',
  kind        TEXT    NOT NULL,
  missing     INTEGER NOT NULL DEFAULT 0,
  version_id  INTEGER REFERENCES versions(id) ON DELETE SET NULL,
  role        TEXT    NOT NULL DEFAULT '',
  part        INTEGER NOT NULL DEFAULT 0,
  lang        TEXT    NOT NULL DEFAULT '',
  first_seen  INTEGER NOT NULL DEFAULT 0    -- unix milliseconds: when the content was first catalogued
);

CREATE INDEX IF NOT EXISTS files_fingerprint ON files(fingerprint);
CREATE INDEX IF NOT EXISTS files_version ON files(version_id);

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
  subs        TEXT    NOT NULL DEFAULT '[]', -- JSON []probe.Track
  probe_version INTEGER NOT NULL DEFAULT 0, -- probe.Version that produced the row
  with_ffprobe  INTEGER NOT NULL DEFAULT 0  -- 1 if the ffprobe fallback was available
);

-- Movies from TMDB (stage 3). JSON columns hold arrays.
CREATE TABLE IF NOT EXISTS movies (
  tmdb_id        INTEGER PRIMARY KEY,
  title          TEXT    NOT NULL,
  original_title TEXT    NOT NULL DEFAULT '',
  year           INTEGER NOT NULL DEFAULT 0,
  runtime        INTEGER NOT NULL DEFAULT 0,    -- minutes
  original_lang  TEXT    NOT NULL DEFAULT '',
  overview       TEXT    NOT NULL DEFAULT '',
  directors      TEXT    NOT NULL DEFAULT '[]', -- JSON [{id,name}]
  cast_members   TEXT    NOT NULL DEFAULT '[]', -- JSON [{id,name,character}], first 10
  genres         TEXT    NOT NULL DEFAULT '[]', -- JSON localized names
  countries      TEXT    NOT NULL DEFAULT '[]', -- JSON ISO 3166-1 alpha-2
  collection_id  INTEGER NOT NULL DEFAULT 0,
  collection     TEXT    NOT NULL DEFAULT '',
  poster_path    TEXT    NOT NULL DEFAULT '',
  backdrop_path  TEXT    NOT NULL DEFAULT '',
  imdb_id        TEXT    NOT NULL DEFAULT '',
  wikidata_id    TEXT    NOT NULL DEFAULT '',
  language       TEXT    NOT NULL,              -- language requested from TMDB
  fetched_at     INTEGER NOT NULL,              -- unix milliseconds
  wikidata_state INTEGER NOT NULL DEFAULT 0     -- 0 pending, 1 looked up
);

-- What each content fingerprint is. auto/unmatched rows are the matcher's
-- and get recomputed; manual/ignored/extra rows are the user's corrections.
CREATE TABLE IF NOT EXISTS identifications (
  fingerprint     TEXT    PRIMARY KEY,
  status          TEXT    NOT NULL,              -- auto | manual | ignored | extra | unmatched
  tmdb_id         INTEGER NOT NULL DEFAULT 0,    -- the movie (auto/manual) or the movie it is an extra of
  confidence      REAL    NOT NULL DEFAULT 0,
  candidates      TEXT    NOT NULL DEFAULT '[]', -- JSON top 5 candidates
  query           TEXT    NOT NULL DEFAULT '',   -- "title|year|director|imdb" the matcher used
  matcher_version INTEGER NOT NULL DEFAULT 0,
  updated_at      INTEGER NOT NULL               -- unix milliseconds
);

CREATE INDEX IF NOT EXISTS identifications_tmdb ON identifications(tmdb_id);
