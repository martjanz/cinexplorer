// Package search is the instant search of the catalog: a full-text index
// (SQLite FTS5) kept in memory and rebuilt from the catalog's items when the
// catalog changes. It ignores case and accents and matches the last word as
// a prefix, so results come while typing.
package search

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"cinexplorer/internal/quality"
)

// Doc is what the index keeps of one item of the catalog.
type Doc struct {
	Key       string // the item's key (catalog.ItemKey)
	Title     string
	Original  string
	Directors string
	Cast      string
	Files     string // titles parsed from the item's file names
}

// MinLength is how many letters or digits a query needs.
const MinLength = 2

// rank weighs the columns: title, original title, directors, cast, files
// (the key is not indexed).
const rank = `bm25(docs, 0, 10, 8, 4, 1, 2)`

type Index struct {
	mu      sync.Mutex
	db      *sql.DB
	changes int64
	built   bool
}

// New returns an empty index.
func New() (*Index, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	// An in-memory database lives in its connection: keep exactly one.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	if _, err := db.Exec(`CREATE VIRTUAL TABLE docs USING fts5(key UNINDEXED, title, original, directors, cast, files,
		tokenize = "unicode61 remove_diacritics 2")`); err != nil {
		db.Close()
		return nil, err
	}
	return &Index{db: db}, nil
}

func (x *Index) Close() error { return x.db.Close() }

// Refresh rebuilds the index from docs() when the catalog changed since the
// last build: changes is the catalog's change counter (store.Snapshot).
// docs are indexed in order, which breaks ties between equal ranks.
func (x *Index) Refresh(changes int64, docs func() []Doc) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.built && x.changes == changes {
		return nil
	}
	tx, err := x.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM docs`); err != nil {
		return err
	}
	ins, err := tx.Prepare(`INSERT INTO docs (key, title, original, directors, cast, files) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, d := range docs() {
		if _, err := ins.Exec(d.Key, d.Title, d.Original, d.Directors, d.Cast, d.Files); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	x.changes, x.built = changes, true
	return nil
}

// Terms splits a query into folded words (no case, no accents).
func Terms(q string) []string { return quality.Words(q) }

// expression is the FTS5 query for terms: all of them, the last as a prefix.
// Terms are letters and digits only, so quoting them is enough.
func expression(terms []string) string {
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = `"` + t + `"`
	}
	parts[len(parts)-1] += "*"
	return strings.Join(parts, " ")
}

// Query returns the keys of the items that match q, best first. A query
// with fewer than MinLength letters or digits matches nothing.
func (x *Index) Query(q string) ([]string, error) {
	terms := Terms(q)
	if len([]rune(strings.Join(terms, ""))) < MinLength {
		return nil, nil
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	rows, err := x.db.Query(fmt.Sprintf(`SELECT key FROM docs WHERE docs MATCH ? ORDER BY %s, rowid`, rank), expression(terms))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
