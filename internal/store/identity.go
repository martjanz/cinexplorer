package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"time"
)

// Identification statuses. auto and unmatched belong to the matcher; the
// others are the user's corrections and are never overwritten by it.
const (
	StatusAuto      = "auto"
	StatusManual    = "manual"
	StatusIgnored   = "ignored"
	StatusExtra     = "extra"
	StatusUnmatched = "unmatched"
)

var ErrUnknownFingerprint = errors.New("huella desconocida")

// Candidate is a possible TMDB match for a version.
type Candidate struct {
	TMDBID        int     `json:"tmdbId"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"originalTitle"`
	Year          int     `json:"year"`
	PosterPath    string  `json:"posterPath"`
	Score         float64 `json:"score"`
}

type Identification struct {
	Fingerprint    string
	Status         string
	TMDBID         int
	Confidence     float64
	Candidates     []Candidate
	Query          string
	MatcherVersion int
}

// IdentifyTarget is a version to identify, keyed by the fingerprint of its
// representative main file.
type IdentifyTarget struct {
	Fingerprint string
	Dir         string
	Title       string
	Year        int
	Director    string
	IMDbID      string          // from the file or folder name
	NFOs        []string        // present .nfo files in Dir (catalog paths, sorted)
	Current     *Identification // nil when never identified
}

// representative is the file that stands for a version.
type representative struct {
	fingerprint string
	path        string
}

// representatives maps each version id to its representative file: the
// present main file with the lowest part number, the largest on ties (the
// biggest VOB of a DVD).
func (s *Store) representatives(tx querier) (map[int64]representative, error) {
	rows, err := tx.Query(`SELECT version_id, fingerprint, path FROM files
		WHERE version_id IS NOT NULL AND role = 'main' AND missing = 0 AND fingerprint != ''
		ORDER BY version_id, part, size DESC, path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]representative{}
	for rows.Next() {
		var id int64
		var r representative
		if err := rows.Scan(&id, &r.fingerprint, &r.path); err != nil {
			return nil, err
		}
		if _, ok := out[id]; !ok {
			out[id] = r
		}
	}
	return out, rows.Err()
}

// IsRepresentative reports whether fingerprint identifies a version: it is
// the fingerprint of some version's representative file. Corrections are
// only accepted for such fingerprints (a second part's would never show).
func (s *Store) IsRepresentative(fingerprint string) (bool, error) {
	if fingerprint == "" {
		return false, nil
	}
	reps, err := s.representatives(s.db)
	if err != nil {
		return false, err
	}
	for _, r := range reps {
		if r.fingerprint == fingerprint {
			return true, nil
		}
	}
	return false, nil
}

// nfosFor picks the .nfo files that describe a version. In a folder of its
// own every .nfo counts; in a folder shared by several versions (loose movies
// in a root or a decade folder) only a .nfo named like the version's file
// does ("Amarcord.nfo" for "Amarcord CD1.avi"), or one movie's IMDb id would
// be forced on all of them.
func nfosFor(nfos []string, shared bool, repPath string) []string {
	if !shared {
		return nfos
	}
	rep := strings.ToLower(stem(repPath))
	var out []string
	for _, n := range nfos {
		if s := strings.ToLower(stem(n)); s != "" && strings.HasPrefix(rep, s) {
			out = append(out, n)
		}
	}
	return out
}

func stem(p string) string {
	b := path.Base(p)
	return strings.TrimSuffix(b, path.Ext(b))
}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// identifications loads every identification row by fingerprint.
func (s *Store) identifications(tx querier) (map[string]*Identification, error) {
	out := map[string]*Identification{}
	if !s.hasIdentity {
		return out, nil
	}
	rows, err := tx.Query(`SELECT fingerprint, status, tmdb_id, confidence, candidates, query, matcher_version FROM identifications`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var i Identification
		var cands string
		if err := rows.Scan(&i.Fingerprint, &i.Status, &i.TMDBID, &i.Confidence, &cands, &i.Query, &i.MatcherVersion); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(cands), &i.Candidates); err != nil {
			return nil, err
		}
		out[i.Fingerprint] = &i
	}
	return out, rows.Err()
}

// IdentifyTargets returns one target per distinct representative fingerprint
// (identical copies are identified once), in version order.
func (s *Store) IdentifyTargets() ([]IdentifyTarget, error) {
	if !s.hasIdentity {
		return nil, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	reps, err := s.representatives(tx)
	if err != nil {
		return nil, err
	}
	current, err := s.identifications(tx)
	if err != nil {
		return nil, err
	}
	nfos := map[string][]string{}
	nrows, err := tx.Query(`SELECT path FROM files WHERE kind = 'info' AND missing = 0 ORDER BY path`)
	if err != nil {
		return nil, err
	}
	for nrows.Next() {
		var p string
		if err := nrows.Scan(&p); err != nil {
			nrows.Close()
			return nil, err
		}
		nfos[path.Dir(p)] = append(nfos[path.Dir(p)], p)
	}
	nrows.Close()
	if err := nrows.Err(); err != nil {
		return nil, err
	}

	rows, err := tx.Query(`SELECT id, dir, title, year, director, imdb_id FROM versions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type version struct {
		id int64
		t  IdentifyTarget
	}
	var vs []version
	perDir := map[string]int{}
	for rows.Next() {
		var v version
		if err := rows.Scan(&v.id, &v.t.Dir, &v.t.Title, &v.t.Year, &v.t.Director, &v.t.IMDbID); err != nil {
			return nil, err
		}
		vs = append(vs, v)
		perDir[v.t.Dir]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []IdentifyTarget
	seen := map[string]bool{}
	for _, v := range vs {
		rep, ok := reps[v.id]
		if !ok || seen[rep.fingerprint] {
			continue
		}
		seen[rep.fingerprint] = true
		t := v.t
		t.Fingerprint, t.Current = rep.fingerprint, current[rep.fingerprint]
		t.NFOs = nfosFor(nfos[t.Dir], perDir[t.Dir] > 1, rep.path)
		out = append(out, t)
	}
	return out, nil
}

// SaveIdentifications stores matcher results in one transaction. A row that
// became a correction in the meantime is left alone.
func (s *Store) SaveIdentifications(ids []Identification) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	up, err := tx.Prepare(`INSERT INTO identifications
		(fingerprint, status, tmdb_id, confidence, candidates, query, matcher_version, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET status = excluded.status, tmdb_id = excluded.tmdb_id,
		confidence = excluded.confidence, candidates = excluded.candidates, query = excluded.query,
		matcher_version = excluded.matcher_version, updated_at = excluded.updated_at
		WHERE identifications.status IN ('auto', 'unmatched')`)
	if err != nil {
		return err
	}
	defer up.Close()
	now := time.Now().UnixMilli()
	for _, i := range ids {
		cands := i.Candidates
		if cands == nil {
			cands = []Candidate{}
		}
		b, err := json.Marshal(cands)
		if err != nil {
			return err
		}
		if _, err := up.Exec(i.Fingerprint, i.Status, i.TMDBID, i.Confidence, string(b), i.Query, i.MatcherVersion, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetCorrection records the user's decision for a version's fingerprint: a
// movie (StatusManual), not a movie (StatusIgnored) or an extra of a movie
// (StatusExtra). Candidates and query of a previous match are kept.
func (s *Store) SetCorrection(fingerprint, status string, tmdbID int) error {
	if status != StatusManual && status != StatusIgnored && status != StatusExtra {
		return errors.New("store: estado de corrección inválido: " + status)
	}
	ok, err := s.IsRepresentative(fingerprint)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUnknownFingerprint
	}
	_, err = s.db.Exec(`INSERT INTO identifications (fingerprint, status, tmdb_id, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET status = excluded.status, tmdb_id = excluded.tmdb_id,
		updated_at = excluded.updated_at`, fingerprint, status, tmdbID, time.Now().UnixMilli())
	return err
}

// ResetIdentification forgets what a fingerprint is, so the matcher runs on
// it again.
func (s *Store) ResetIdentification(fingerprint string) error {
	_, err := s.db.Exec(`DELETE FROM identifications WHERE fingerprint = ?`, fingerprint)
	return err
}

// attachIdentity fills Fingerprint, Identification and Movie of each version.
func (s *Store) attachIdentity(tx querier, out []VersionView, pos map[int64]int) error {
	reps, err := s.representatives(tx)
	if err != nil {
		return err
	}
	for id, r := range reps {
		if i, ok := pos[id]; ok {
			out[i].Fingerprint = r.fingerprint
		}
	}
	if !s.hasIdentity {
		return nil
	}
	ids, err := s.identifications(tx)
	if err != nil {
		return err
	}
	refs := map[int]*MovieRef{}
	rows, err := tx.Query(`SELECT tmdb_id, title, original_title, year, directors FROM movies`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var m MovieRef
		var dirs string
		if err := rows.Scan(&m.TMDBID, &m.Title, &m.OriginalTitle, &m.Year, &dirs); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(dirs), &m.Directors); err != nil {
			return err
		}
		refs[m.TMDBID] = &m
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range out {
		v := &out[i]
		id := ids[v.Fingerprint]
		if v.Fingerprint == "" || id == nil {
			continue
		}
		v.Identification = &IdentView{Status: id.Status, Confidence: id.Confidence, TMDBID: id.TMDBID}
		if id.Status == StatusAuto || id.Status == StatusManual {
			v.Movie = refs[id.TMDBID]
		}
	}
	return nil
}
