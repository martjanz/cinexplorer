package store

import "time"

// Decisions about a folder of Collections/.
const (
	CollectionImported  = "imported"
	CollectionDismissed = "dismissed"
)

// CollectionDecision is what the user decided about a folder of
// Collections/.
type CollectionDecision struct {
	Status string
	ListID int64           // the list it was imported into; 0 when dismissed or when that list was deleted
	Seen   map[string]bool // fingerprints already offered
}

// ImportCollection adds the contents fps of the folder path to a list — a
// new one called name, or listID when name is "" — and marks them as
// offered. It returns the list's id.
func (s *Store) ImportCollection(path, name string, listID int64, fps []string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	if name != "" {
		l, err := createList(tx, name, now)
		if err != nil {
			return 0, err
		}
		listID = l.ID
	} else if err := listExists(tx, listID); err != nil {
		return 0, err
	}
	for _, fp := range fps {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO list_entries (list_id, ref, added_at) VALUES (?, ?, ?)`,
			listID, RefFingerprint(fp), now); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(`UPDATE lists SET updated_at = ? WHERE id = ?`, now, listID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO collection_folders (path, status, list_id, decided_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET status = excluded.status, list_id = excluded.list_id, decided_at = excluded.decided_at`,
		path, CollectionImported, listID, now); err != nil {
		return 0, err
	}
	if err := markSeen(tx, path, fps); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return listID, nil
}

// DismissCollection marks the contents fps of the folder path as offered
// without importing them. A folder never imported is dismissed for good;
// an imported one keeps its list (only these new contents are declined).
func (s *Store) DismissCollection(path string, fps []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO collection_folders (path, status, list_id, decided_at) VALUES (?, ?, NULL, ?)
		ON CONFLICT(path) DO UPDATE SET decided_at = excluded.decided_at`,
		path, CollectionDismissed, time.Now().UnixMilli()); err != nil {
		return err
	}
	if err := markSeen(tx, path, fps); err != nil {
		return err
	}
	return tx.Commit()
}

func markSeen(tx execQuerier, path string, fps []string) error {
	for _, fp := range fps {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO collection_seen (path, fingerprint) VALUES (?, ?)`, path, fp); err != nil {
			return err
		}
	}
	return nil
}

// collectionDecisions loads the decisions by folder path.
func (s *Store) collectionDecisions(tx querier) (map[string]CollectionDecision, error) {
	out := map[string]CollectionDecision{}
	if !s.hasLists {
		return out, nil
	}
	rows, err := tx.Query(`SELECT path, status, COALESCE(list_id, 0) FROM collection_folders`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p string
		d := CollectionDecision{Seen: map[string]bool{}}
		if err := rows.Scan(&p, &d.Status, &d.ListID); err != nil {
			rows.Close()
			return nil, err
		}
		out[p] = d
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	srows, err := tx.Query(`SELECT path, fingerprint FROM collection_seen`)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var p, fp string
		if err := srows.Scan(&p, &fp); err != nil {
			return nil, err
		}
		if d, ok := out[p]; ok {
			d.Seen[fp] = true
		}
	}
	return out, srows.Err()
}
