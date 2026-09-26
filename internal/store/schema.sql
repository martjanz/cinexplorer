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
  lang        TEXT    NOT NULL DEFAULT ''
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
