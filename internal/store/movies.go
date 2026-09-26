package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Person struct {
	ID   int    `json:"id"` // TMDB person id; 0 when it came from Wikidata
	Name string `json:"name"`
}

type CastMember struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Character string `json:"character"`
}

type Movie struct {
	TMDBID        int          `json:"tmdbId"`
	Title         string       `json:"title"`
	OriginalTitle string       `json:"originalTitle"`
	Year          int          `json:"year"`
	Runtime       int          `json:"runtime"`
	OriginalLang  string       `json:"originalLang"`
	Overview      string       `json:"overview"`
	Directors     []Person     `json:"directors"`
	Cast          []CastMember `json:"cast"`
	Genres        []string     `json:"genres"`
	Countries     []string     `json:"countries"`
	CollectionID  int          `json:"collectionId"`
	Collection    string       `json:"collection"`
	PosterPath    string       `json:"posterPath"`
	BackdropPath  string       `json:"backdropPath"`
	IMDbID        string       `json:"imdbId"`
	WikidataID    string       `json:"wikidataId"`
	Language      string       `json:"language"`
	WikidataDone  bool         `json:"-"`
}

const movieColumns = `tmdb_id, title, original_title, year, runtime, original_lang, overview, directors,
	cast_members, genres, countries, collection_id, collection, poster_path, backdrop_path, imdb_id,
	wikidata_id, language, wikidata_state`

// SaveMovie inserts or replaces a movie.
func (s *Store) SaveMovie(m Movie) error {
	var js [4]string
	for i, v := range []any{m.Directors, m.Cast, m.Genres, m.Countries} {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if string(b) == "null" {
			b = []byte("[]")
		}
		js[i] = string(b)
	}
	_, err := s.db.Exec(`INSERT INTO movies (`+movieColumns+`, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tmdb_id) DO UPDATE SET title = excluded.title, original_title = excluded.original_title,
		year = excluded.year, runtime = excluded.runtime, original_lang = excluded.original_lang,
		overview = excluded.overview, directors = excluded.directors, cast_members = excluded.cast_members,
		genres = excluded.genres, countries = excluded.countries, collection_id = excluded.collection_id,
		collection = excluded.collection, poster_path = excluded.poster_path, backdrop_path = excluded.backdrop_path,
		imdb_id = excluded.imdb_id, wikidata_id = excluded.wikidata_id, language = excluded.language,
		wikidata_state = excluded.wikidata_state, fetched_at = excluded.fetched_at`,
		m.TMDBID, m.Title, m.OriginalTitle, m.Year, m.Runtime, m.OriginalLang, m.Overview, js[0], js[1], js[2], js[3],
		m.CollectionID, m.Collection, m.PosterPath, m.BackdropPath, m.IMDbID, m.WikidataID, m.Language,
		m.WikidataDone, time.Now().UnixMilli())
	return err
}

type scanner interface{ Scan(dest ...any) error }

func scanMovie(r scanner) (Movie, error) {
	var m Movie
	var dirs, cast, genres, countries string
	err := r.Scan(&m.TMDBID, &m.Title, &m.OriginalTitle, &m.Year, &m.Runtime, &m.OriginalLang, &m.Overview,
		&dirs, &cast, &genres, &countries, &m.CollectionID, &m.Collection, &m.PosterPath, &m.BackdropPath,
		&m.IMDbID, &m.WikidataID, &m.Language, &m.WikidataDone)
	if err != nil {
		return m, err
	}
	for _, p := range []struct {
		src string
		dst any
	}{{dirs, &m.Directors}, {cast, &m.Cast}, {genres, &m.Genres}, {countries, &m.Countries}} {
		if err := json.Unmarshal([]byte(p.src), p.dst); err != nil {
			return m, err
		}
	}
	return m, nil
}

// Movie returns the movie with a TMDB id; ok is false when it is not stored.
func (s *Store) Movie(id int) (m Movie, ok bool, err error) {
	if !s.hasIdentity {
		return m, false, nil
	}
	m, err = scanMovie(s.db.QueryRow(`SELECT `+movieColumns+` FROM movies WHERE tmdb_id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return m, false, nil
	}
	return m, err == nil, err
}

// Movies returns every stored movie by TMDB id.
func (s *Store) Movies() ([]Movie, error) {
	if !s.hasIdentity {
		return nil, nil
	}
	return s.queryMovies(s.db, `SELECT `+movieColumns+` FROM movies ORDER BY tmdb_id`)
}

// PendingWikidata returns up to limit movies not looked up in Wikidata yet.
func (s *Store) PendingWikidata(limit int) ([]Movie, error) {
	return s.queryMovies(s.db, `SELECT `+movieColumns+` FROM movies WHERE wikidata_state = 0 ORDER BY tmdb_id LIMIT ?`, limit)
}

func (s *Store) queryMovies(tx querier, q string, args ...any) ([]Movie, error) {
	rows, err := tx.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Movie
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MoviesToEnrich returns the TMDB ids that identifications of present files
// point to and that are missing from movies or were fetched in another
// language. Identifications left behind by rewritten files are not followed.
func (s *Store) MoviesToEnrich(lang string) ([]int, error) {
	rows, err := s.db.Query(`SELECT DISTINCT i.tmdb_id FROM identifications i
		LEFT JOIN movies m ON m.tmdb_id = i.tmdb_id
		WHERE i.status IN ('auto', 'manual', 'extra') AND i.tmdb_id > 0
		  AND (m.tmdb_id IS NULL OR m.language != ?)
		  AND EXISTS (SELECT 1 FROM files f WHERE f.fingerprint = i.fingerprint AND f.missing = 0)
		ORDER BY i.tmdb_id`, lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// InvalidateMovie handles a TMDB id that no longer exists: the movie is
// dropped and the matcher's identifications pointing to it are forgotten
// (so they are identified again). The user's corrections are kept.
func (s *Store) InvalidateMovie(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM movies WHERE tmdb_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM identifications WHERE tmdb_id = ? AND status = 'auto'`, id); err != nil {
		return err
	}
	return tx.Commit()
}
