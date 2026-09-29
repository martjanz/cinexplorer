package store

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors of the lists, shown by the interface as they are.
var (
	ErrListName    = errors.New("el nombre de la lista no puede estar vacío ni pasar de 100 caracteres")
	ErrListExists  = errors.New("ya existe una lista con ese nombre")
	ErrUnknownList = errors.New("la lista no existe")
)

const maxListName = 100

// ListEntry is something a list holds: a movie or a content.
type ListEntry struct {
	Ref     string // RefMovie or RefFingerprint
	AddedAt int64  // unix milliseconds
}

// List is one of the user's lists with its entries, oldest first.
type List struct {
	ID        int64
	Name      string
	CreatedAt int64 // unix milliseconds
	UpdatedAt int64 // last rename or change of entries
	Entries   []ListEntry
}

// RefMovie is the entry of a TMDB movie.
func RefMovie(tmdbID int) string { return "movie:" + strconv.Itoa(tmdbID) }

// RefFingerprint is the entry of a content, identified or not: it follows
// the content's identification.
func RefFingerprint(fp string) string { return "fp:" + fp }

// ListName trims a list name and checks it: not empty, at most 100
// characters.
func ListName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxListName {
		return "", ErrListName
	}
	return name, nil
}

// nameTaken reports whether a list other than except is called name,
// ignoring case (beyond ASCII, which the column's NOCASE does not).
func nameTaken(tx querier, name string, except int64) (bool, error) {
	rows, err := tx.Query(`SELECT id, name FROM lists`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n string
		if err := rows.Scan(&id, &n); err != nil {
			return false, err
		}
		if id != except && strings.EqualFold(n, name) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func listExists(tx querier, id int64) error {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM lists WHERE id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknownList
	}
	return nil
}

// createList adds an empty list inside tx.
func createList(tx execQuerier, name string, now int64) (List, error) {
	name, err := ListName(name)
	if err != nil {
		return List{}, err
	}
	taken, err := nameTaken(tx, name, 0)
	if err != nil {
		return List{}, err
	}
	if taken {
		return List{}, ErrListExists
	}
	res, err := tx.Exec(`INSERT INTO lists (name, created_at, updated_at) VALUES (?, ?, ?)`, name, now, now)
	if err != nil {
		return List{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return List{}, err
	}
	return List{ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Entries: []ListEntry{}}, nil
}

// CreateList adds an empty list.
func (s *Store) CreateList(name string) (List, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return List{}, err
	}
	defer tx.Rollback()
	l, err := createList(tx, name, time.Now().UnixMilli())
	if err != nil {
		return List{}, err
	}
	if err := tx.Commit(); err != nil {
		return List{}, err
	}
	return l, nil
}

// RenameList changes a list's name.
func (s *Store) RenameList(id int64, name string) error {
	name, err := ListName(name)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := listExists(tx, id); err != nil {
		return err
	}
	taken, err := nameTaken(tx, name, id)
	if err != nil {
		return err
	}
	if taken {
		return ErrListExists
	}
	if _, err := tx.Exec(`UPDATE lists SET name = ?, updated_at = ? WHERE id = ?`, name, time.Now().UnixMilli(), id); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteList deletes a list and its entries. A folder imported into it
// keeps its decision, without a list.
func (s *Store) DeleteList(id int64) error {
	res, err := s.db.Exec(`DELETE FROM lists WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknownList
	}
	return nil
}

// AddEntries adds refs to a list; the ones it already holds keep their
// date.
func (s *Store) AddEntries(id int64, refs []string) error {
	return s.changeEntries(id, refs, func(tx *sql.Tx, ref string, now int64) (sql.Result, error) {
		return tx.Exec(`INSERT OR IGNORE INTO list_entries (list_id, ref, added_at) VALUES (?, ?, ?)`, id, ref, now)
	})
}

// RemoveEntries removes refs from a list; refs it does not hold are
// ignored.
func (s *Store) RemoveEntries(id int64, refs []string) error {
	return s.changeEntries(id, refs, func(tx *sql.Tx, ref string, _ int64) (sql.Result, error) {
		return tx.Exec(`DELETE FROM list_entries WHERE list_id = ? AND ref = ?`, id, ref)
	})
}

// changeEntries runs change for each ref in one transaction and, when an
// entry was added or removed, dates the list's change.
func (s *Store) changeEntries(id int64, refs []string, change func(tx *sql.Tx, ref string, now int64) (sql.Result, error)) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := listExists(tx, id); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	changed := false
	for _, ref := range refs {
		res, err := change(tx, ref, now)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		changed = changed || n > 0
	}
	if changed {
		if _, err := tx.Exec(`UPDATE lists SET updated_at = ? WHERE id = ?`, now, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// lists loads every list, by id, with its entries, oldest first.
func (s *Store) lists(tx querier) ([]List, error) {
	out := []List{}
	if !s.hasLists {
		return out, nil
	}
	rows, err := tx.Query(`SELECT id, name, created_at, updated_at FROM lists ORDER BY id`)
	if err != nil {
		return nil, err
	}
	pos := map[int64]int{}
	for rows.Next() {
		l := List{Entries: []ListEntry{}}
		if err := rows.Scan(&l.ID, &l.Name, &l.CreatedAt, &l.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		pos[l.ID] = len(out)
		out = append(out, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	erows, err := tx.Query(`SELECT list_id, ref, added_at FROM list_entries ORDER BY list_id, added_at, ref`)
	if err != nil {
		return nil, err
	}
	defer erows.Close()
	for erows.Next() {
		var id int64
		var e ListEntry
		if err := erows.Scan(&id, &e.Ref, &e.AddedAt); err != nil {
			return nil, err
		}
		if i, ok := pos[id]; ok {
			out[i].Entries = append(out[i].Entries, e)
		}
	}
	return out, erows.Err()
}
