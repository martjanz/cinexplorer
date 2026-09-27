// Package store persists the catalog in a SQLite file next to the executable.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"path"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"cinexplorer/internal/grouping"
	"cinexplorer/internal/probe"
)

//go:embed schema.sql
var schema string

type Store struct {
	db *sql.DB
	// hasMedia and hasIdentity are false for an older catalog opened
	// read-only: the schema only runs on writable opens, so the tables of
	// later stages may not exist.
	hasMedia    bool
	hasIdentity bool
	// hasFirstSeen is false for a catalog from before files.first_seen
	// opened read-only.
	hasFirstSeen bool
}

type FileRow struct {
	Path        string
	Size        int64
	MTime       int64
	Fingerprint string
	Kind        string
	Missing     bool
}

type FileView struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Role    string `json:"role"`
	Part    int    `json:"part"`
	Lang    string `json:"lang"`
	Missing bool   `json:"missing"`
}

type VersionView struct {
	ID         int64      `json:"id"`
	Dir        string     `json:"dir"`
	Title      string     `json:"title"`
	Year       int        `json:"year"`
	Director   string     `json:"director"`
	Countries  string     `json:"countries"`
	Resolution string     `json:"resolution"`
	Source     string     `json:"source"`
	Codec      string     `json:"codec"`
	Language   string     `json:"language"`
	Size       int64      `json:"size"`
	Parts      int        `json:"parts"`
	SubLangs   string     `json:"subLangs"`
	Files      []FileView `json:"files"`
	Added      int64      `json:"added"` // unix ms: first_seen of its earliest present main file

	// Technical data read from the files (zero when not probed yet).
	DurationMs int64         `json:"durationMs"`
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	VideoCodec string        `json:"videoCodec"`
	Audio      []probe.Track `json:"audio"`
	Subs       []probe.Track `json:"subs"` // embedded subtitle tracks
	Best       bool          `json:"best"` // best of several versions of the same movie

	// Identity: Fingerprint is the representative main file's ("" when none
	// is present and hashed). Movie is set for versions identified as a
	// stored movie.
	Fingerprint    string     `json:"fingerprint"`
	Identification *IdentView `json:"identification"`
	Movie          *MovieRef  `json:"movie"`
}

type IdentView struct {
	Status     string  `json:"status"`
	Confidence float64 `json:"confidence"`
	TMDBID     int     `json:"tmdbId"` // the movie, or the movie it is an extra of
}

type MovieRef struct {
	TMDBID        int      `json:"tmdbId"`
	Title         string   `json:"title"`
	OriginalTitle string   `json:"originalTitle"`
	Year          int      `json:"year"`
	Directors     []Person `json:"directors"`
}

type Duplicate struct {
	Fingerprint string   `json:"fingerprint"`
	Size        int64    `json:"size"`
	Paths       []string `json:"paths"`
}

// Open opens (creating if needed) the catalog. Journal mode DELETE instead of
// WAL: safer on removable drives. readOnly opens with query_only.
func Open(path string, readOnly bool) (*Store, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if readOnly {
		dsn += "&_pragma=query_only(1)"
	} else {
		dsn += "&_pragma=journal_mode(DELETE)"
	}
	return openDB(dsn, !readOnly)
}

// OpenMemory returns an empty in-memory catalog (read-only mode without a db file).
func OpenMemory() (*Store, error) {
	return openDB(":memory:?_pragma=foreign_keys(1)", true)
}

func openDB(dsn string, migrate bool) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if migrate {
		if _, err := db.Exec(schema); err != nil {
			db.Close()
			return nil, err
		}
		if err := addFirstSeen(db); err != nil {
			db.Close()
			return nil, err
		}
	}
	s := &Store{db: db}
	for name, dst := range map[string]*bool{"media": &s.hasMedia, "identifications": &s.hasIdentity} {
		if err := db.QueryRow(`SELECT COUNT(*) > 0 FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(dst); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := db.QueryRow(`SELECT COUNT(*) > 0 FROM pragma_table_info('files') WHERE name = 'first_seen'`).Scan(&s.hasFirstSeen); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// addFirstSeen adds files.first_seen to a catalog from before it existed
// (CREATE TABLE IF NOT EXISTS does not add columns). The files already
// catalogued get their modification time.
func addFirstSeen(db *sql.DB) error {
	var has bool
	if err := db.QueryRow(`SELECT COUNT(*) > 0 FROM pragma_table_info('files') WHERE name = 'first_seen'`).Scan(&has); err != nil || has {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`ALTER TABLE files ADD COLUMN first_seen INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE files SET first_seen = mtime`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }

// FileIndex returns every known file by path, including missing ones.
func (s *Store) FileIndex() (map[string]FileRow, error) {
	rows, err := s.db.Query(`SELECT path, size, mtime, fingerprint, kind, missing FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	idx := map[string]FileRow{}
	for rows.Next() {
		var f FileRow
		if err := rows.Scan(&f.Path, &f.Size, &f.MTime, &f.Fingerprint, &f.Kind, &f.Missing); err != nil {
			return nil, err
		}
		idx[f.Path] = f
	}
	return idx, rows.Err()
}

// SyncFiles upserts the files seen in a scan and marks as missing the known
// files under scannedRoots that were not seen. Files under other roots keep
// their state, so an unmounted folder does not wipe its entries.
func (s *Store) SyncFiles(seen []FileRow, scannedRoots []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// A new path gets the date its content was first seen under any path
	// (a moved file is not new), or now. Known paths keep theirs.
	up, err := tx.Prepare(`INSERT INTO files (path, size, mtime, fingerprint, kind, missing, first_seen)
		VALUES (?1, ?2, ?3, ?4, ?5, 0, COALESCE((SELECT MIN(first_seen) FROM files WHERE fingerprint = ?4 AND ?4 != ''), ?6))
		ON CONFLICT(path) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
		fingerprint = excluded.fingerprint, kind = excluded.kind, missing = 0`)
	if err != nil {
		return err
	}
	defer up.Close()
	now := time.Now().UnixMilli()
	present := make(map[string]bool, len(seen))
	for _, f := range seen {
		if _, err := up.Exec(f.Path, f.Size, f.MTime, f.Fingerprint, f.Kind, now); err != nil {
			return err
		}
		present[f.Path] = true
	}

	rows, err := tx.Query(`SELECT path FROM files WHERE missing = 0`)
	if err != nil {
		return err
	}
	var gone []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		if !present[p] && underAny(p, scannedRoots) {
			gone = append(gone, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range gone {
		if _, err := tx.Exec(`UPDATE files SET missing = 1 WHERE path = ?`, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// MarkOutsideRoots marks as missing the present files that are not under any
// of roots (the configured ones, available or not): what a removed root
// leaves behind. They come back if the root is added again.
func (s *Store) MarkOutsideRoots(roots []string) error {
	clean := make([]string, len(roots))
	for i, r := range roots {
		clean[i] = path.Clean(r)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT path FROM files WHERE missing = 0`)
	if err != nil {
		return err
	}
	var gone []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		if !underAny(p, clean) {
			gone = append(gone, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range gone {
		if _, err := tx.Exec(`UPDATE files SET missing = 1 WHERE path = ?`, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func underAny(p string, roots []string) bool {
	for _, r := range roots {
		if strings.HasPrefix(p, strings.TrimSuffix(r, "/")+"/") {
			return true
		}
	}
	return false
}

// ReplaceVersions rebuilds the versions table and the file→version links.
func (s *Store) ReplaceVersions(vs []grouping.Version) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE files SET version_id = NULL, role = '', part = 0, lang = ''`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM versions`); err != nil {
		return err
	}
	insV, err := tx.Prepare(`INSERT INTO versions (dir, title, year, director, countries, resolution, source, codec,
		language, release_group, imdb_id, size, parts, sub_langs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insV.Close()
	updF, err := tx.Prepare(`UPDATE files SET version_id = ?, role = ?, part = ?, lang = ? WHERE path = ?`)
	if err != nil {
		return err
	}
	defer updF.Close()

	for _, v := range vs {
		p := v.Parsed
		res, err := insV.Exec(v.Dir, p.Title, p.Year, p.Director, strings.Join(p.Countries, ", "), p.Resolution,
			p.Source, p.Codec, p.Language, p.Group, p.IMDbID, v.Size, v.Parts, strings.Join(v.SubLangs(), ","))
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for _, m := range v.Members {
			res, err := updF.Exec(id, string(m.Role), m.Part, m.Lang, m.Path)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if n == 0 {
				return fmt.Errorf("store: version member references unknown file %q", m.Path)
			}
		}
	}
	return tx.Commit()
}

// Versions returns every version with its files, technical data and
// identity, ordered by title and year.
func (s *Store) Versions() ([]VersionView, error) {
	var vs []VersionView
	err := s.read(func(tx querier) (err error) {
		vs, err = s.versions(tx)
		return err
	})
	return vs, err
}

func (s *Store) versions(tx querier) ([]VersionView, error) {
	rows, err := tx.Query(`SELECT id, dir, title, year, director, countries, resolution, source, codec, language,
		size, parts, sub_langs FROM versions ORDER BY title COLLATE NOCASE, year, id`)
	if err != nil {
		return nil, err
	}
	out := []VersionView{}
	pos := map[int64]int{}
	for rows.Next() {
		v := VersionView{Files: []FileView{}, Audio: []probe.Track{}, Subs: []probe.Track{}}
		if err := rows.Scan(&v.ID, &v.Dir, &v.Title, &v.Year, &v.Director, &v.Countries, &v.Resolution,
			&v.Source, &v.Codec, &v.Language, &v.Size, &v.Parts, &v.SubLangs); err != nil {
			rows.Close()
			return nil, err
		}
		v.Codec = probe.CodecFromName(v.Codec)
		pos[v.ID] = len(out)
		out = append(out, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	firstSeen := "0"
	if s.hasFirstSeen {
		firstSeen = "first_seen"
	}
	frows, err := tx.Query(`SELECT version_id, path, size, role, part, lang, missing, ` + firstSeen + ` FROM files
		WHERE version_id IS NOT NULL
		ORDER BY version_id, CASE role WHEN 'main' THEN 0 WHEN 'subtitle' THEN 1 ELSE 2 END, part, path`)
	if err != nil {
		return nil, err
	}
	defer frows.Close()
	for frows.Next() {
		var id, seen int64
		var f FileView
		if err := frows.Scan(&id, &f.Path, &f.Size, &f.Role, &f.Part, &f.Lang, &f.Missing, &seen); err != nil {
			return nil, err
		}
		if i, ok := pos[id]; ok {
			v := &out[i]
			v.Files = append(v.Files, f)
			if f.Role == "main" && !f.Missing && (v.Added == 0 || seen < v.Added) {
				v.Added = seen
			}
		}
	}
	if err := frows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachMedia(tx, out, pos); err != nil {
		return nil, err
	}
	if err := s.attachIdentity(tx, out, pos); err != nil {
		return nil, err
	}
	markBest(out)
	return out, nil
}

// Duplicates returns groups of present files with the same fingerprint,
// largest first.
func (s *Store) Duplicates() ([]Duplicate, error) {
	rows, err := s.db.Query(`SELECT fingerprint, size, path FROM files
		WHERE missing = 0 AND fingerprint != '' AND fingerprint IN (
			SELECT fingerprint FROM files WHERE missing = 0 AND fingerprint != ''
			GROUP BY fingerprint HAVING COUNT(*) > 1)
		ORDER BY size DESC, fingerprint, path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Duplicate{}
	for rows.Next() {
		var fp, p string
		var size int64
		if err := rows.Scan(&fp, &size, &p); err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].Fingerprint == fp {
			out[n-1].Paths = append(out[n-1].Paths, p)
			continue
		}
		out = append(out, Duplicate{Fingerprint: fp, Size: size, Paths: []string{p}})
	}
	return out, rows.Err()
}

// HasFile reports whether path is a present file in the catalog. The API uses
// it so that only catalogued files can be opened.
func (s *Store) HasFile(path string) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM files WHERE path = ? AND missing = 0`, path).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
