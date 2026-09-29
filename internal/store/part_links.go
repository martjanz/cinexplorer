package store

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

// ErrPartLink is returned for a link that makes no sense.
var ErrPartLink = errors.New("una película no puede ser parte de sí misma")

type execQuerier interface {
	querier
	Exec(query string, args ...any) (sql.Result, error)
}

// SetPartLink records that the version represented by follower is a further
// part of the one represented by leader, and merges them right away. Both
// must be versions now.
func (s *Store) SetPartLink(follower, leader string) error {
	if follower == leader {
		return ErrPartLink
	}
	for _, fp := range []string{follower, leader} {
		ok, err := s.IsRepresentative(fp)
		if err != nil {
			return err
		}
		if !ok {
			return ErrUnknownFingerprint
		}
	}
	links, err := s.partLinks(s.db)
	if err != nil {
		return err
	}
	if end, ok := finalLeader(links, leader); !ok || end == follower {
		return ErrPartLink // it would close a cycle
	}
	if _, err := s.db.Exec(`INSERT INTO part_links (fingerprint, leader, created_at) VALUES (?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET leader = excluded.leader, created_at = excluded.created_at`,
		follower, leader, time.Now().UnixMilli()); err != nil {
		return err
	}
	_, err = s.ApplyPartLinks()
	return err
}

// Unlink removes the links that lead to leader, directly or through other
// linked versions, and returns how many. The versions come apart at the next
// scan.
func (s *Store) Unlink(leader string) (int, error) {
	if !s.hasPartLinks {
		return 0, nil
	}
	res, err := s.db.Exec(`WITH RECURSIVE t(f) AS (
			SELECT fingerprint FROM part_links WHERE leader = ?
			UNION
			SELECT p.fingerprint FROM part_links p JOIN t ON p.leader = t.f)
		DELETE FROM part_links WHERE fingerprint IN (SELECT f FROM t)`, leader)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// partLinks loads follower → leader.
func (s *Store) partLinks(tx querier) (map[string]string, error) {
	out := map[string]string{}
	if !s.hasPartLinks {
		return out, nil
	}
	rows, err := tx.Query(`SELECT fingerprint, leader FROM part_links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f, l string
		if err := rows.Scan(&f, &l); err != nil {
			return nil, err
		}
		out[f] = l
	}
	return out, rows.Err()
}

// finalLeader follows fp through links to the version that is nobody's
// follower. ok is false when the chain loops.
func finalLeader(links map[string]string, fp string) (string, bool) {
	for range len(links) + 1 {
		next, linked := links[fp]
		if !linked {
			return fp, true
		}
		fp = next
	}
	return "", false
}

// attachPartLinks marks the versions that have parts merged in by hand.
func (s *Store) attachPartLinks(tx querier, out []VersionView) error {
	links, err := s.partLinks(tx)
	if err != nil || len(links) == 0 {
		return err
	}
	leaders := map[string]bool{}
	for f := range links {
		if end, ok := finalLeader(links, f); ok {
			leaders[end] = true
		}
	}
	for i := range out {
		out[i].PartLinked = out[i].Fingerprint != "" && leaders[out[i].Fingerprint]
	}
	return nil
}

// ApplyPartLinks merges into their leader the versions that the user linked
// as parts, on what is stored now, and returns how many versions it merged.
// A link whose two versions are not both present (the leader's disk is
// unplugged, or it is already applied) is left alone and kept. It is safe to
// run any number of times.
func (s *Store) ApplyPartLinks() (int, error) {
	if !s.hasPartLinks {
		return 0, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	links, err := s.partLinks(tx)
	if err != nil || len(links) == 0 {
		return 0, err
	}
	reps, err := s.representatives(tx)
	if err != nil {
		return 0, err
	}
	byFP := map[string]int64{} // the lowest version id wins among identical copies
	for id, r := range reps {
		if cur, ok := byFP[r.fingerprint]; !ok || id < cur {
			byFP[r.fingerprint] = id
		}
	}
	followers := map[string][]string{} // final leader → followers
	for f := range links {
		if end, ok := finalLeader(links, f); ok {
			followers[end] = append(followers[end], f)
		}
	}
	leaders := make([]string, 0, len(followers))
	for l := range followers {
		leaders = append(leaders, l)
	}
	sort.Strings(leaders)

	merged := 0
	for _, l := range leaders {
		lid, ok := byFP[l]
		if !ok {
			continue
		}
		fs := followers[l]
		sort.Slice(fs, func(i, j int) bool { return reps[byFP[fs[i]]].path < reps[byFP[fs[j]]].path })
		changed := false
		for _, f := range fs {
			fid, ok := byFP[f]
			if !ok || fid == lid {
				continue
			}
			if !changed {
				// A single-file leader has part 0: it becomes part 1.
				if _, err := tx.Exec(`UPDATE files SET part = 1 WHERE version_id = ? AND role = 'main' AND part = 0`, lid); err != nil {
					return 0, err
				}
			}
			if err := mergeVersion(tx, lid, fid); err != nil {
				return 0, err
			}
			changed = true
			merged++
		}
		if changed {
			if err := refreshVersion(tx, lid); err != nil {
				return 0, err
			}
		}
	}
	return merged, tx.Commit()
}

// mergeVersion moves the files of version from into version to: main files
// as the next parts (keeping their order), subtitles and extras as they are.
func mergeVersion(tx execQuerier, to, from int64) error {
	var last int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(part), 0) FROM files WHERE version_id = ? AND role = 'main'`, to).Scan(&last); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT id FROM files WHERE version_id = ? AND role = 'main' ORDER BY part, path`, from)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		last++
		if _, err := tx.Exec(`UPDATE files SET version_id = ?, part = ? WHERE id = ?`, to, last, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE files SET version_id = ? WHERE version_id = ?`, to, from); err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM versions WHERE id = ?`, from)
	return err
}

// refreshVersion recomputes what a version derives from its files.
func refreshVersion(tx execQuerier, id int64) error {
	rows, err := tx.Query(`SELECT DISTINCT lang FROM files WHERE version_id = ? AND role = 'subtitle'`, id)
	if err != nil {
		return err
	}
	var langs []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			rows.Close()
			return err
		}
		if l == "" {
			l = "?"
		}
		langs = append(langs, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	sort.Strings(langs)
	_, err = tx.Exec(`UPDATE versions SET
		size = (SELECT COALESCE(SUM(size), 0) FROM files WHERE version_id = ? AND role = 'main'),
		parts = (SELECT COUNT(*) FROM files WHERE version_id = ? AND role = 'main'),
		sub_langs = ? WHERE id = ?`, id, id, strings.Join(langs, ","), id)
	return err
}
