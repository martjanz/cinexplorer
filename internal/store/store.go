// Package store persists the catalog in a SQLite file next to the executable.
package store

import (
	"database/sql"
	_ "embed"
	"strings"

	_ "modernc.org/sqlite"

	"cinexplorer/internal/grouping"
)

//go:embed schema.sql
var schema string

type Store struct {
	db *sql.DB
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
	}
	return &Store{db: db}, nil
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

	up, err := tx.Prepare(`INSERT INTO files (path, size, mtime, fingerprint, kind, missing) VALUES (?, ?, ?, ?, ?, 0)
		ON CONFLICT(path) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
		fingerprint = excluded.fingerprint, kind = excluded.kind, missing = 0`)
	if err != nil {
		return err
	}
	defer up.Close()
	present := make(map[string]bool, len(seen))
	for _, f := range seen {
		if _, err := up.Exec(f.Path, f.Size, f.MTime, f.Fingerprint, f.Kind); err != nil {
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
			if _, err := updF.Exec(id, string(m.Role), m.Part, m.Lang, m.Path); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// Versions returns every version with its files, ordered by title and year.
func (s *Store) Versions() ([]VersionView, error) {
	rows, err := s.db.Query(`SELECT id, dir, title, year, director, countries, resolution, source, codec, language,
		size, parts, sub_langs FROM versions ORDER BY title COLLATE NOCASE, year, id`)
	if err != nil {
		return nil, err
	}
	out := []VersionView{}
	pos := map[int64]int{}
	for rows.Next() {
		v := VersionView{Files: []FileView{}}
		if err := rows.Scan(&v.ID, &v.Dir, &v.Title, &v.Year, &v.Director, &v.Countries, &v.Resolution,
			&v.Source, &v.Codec, &v.Language, &v.Size, &v.Parts, &v.SubLangs); err != nil {
			rows.Close()
			return nil, err
		}
		pos[v.ID] = len(out)
		out = append(out, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	frows, err := s.db.Query(`SELECT version_id, path, role, part, lang, missing FROM files
		WHERE version_id IS NOT NULL
		ORDER BY version_id, CASE role WHEN 'main' THEN 0 WHEN 'subtitle' THEN 1 ELSE 2 END, part, path`)
	if err != nil {
		return nil, err
	}
	defer frows.Close()
	for frows.Next() {
		var id int64
		var f FileView
		if err := frows.Scan(&id, &f.Path, &f.Role, &f.Part, &f.Lang, &f.Missing); err != nil {
			return nil, err
		}
		if i, ok := pos[id]; ok {
			out[i].Files = append(out[i].Files, f)
		}
	}
	return out, frows.Err()
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
