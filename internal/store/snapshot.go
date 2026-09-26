package store

// Snapshot is the whole catalog as the pages see it, read at one point in
// time.
type Snapshot struct {
	Versions        []VersionView              // as Versions returns them
	Identifications map[string]*Identification // by fingerprint
	Movies          map[int]Movie              // by TMDB id
}

// read runs f inside a transaction, so that everything f reads comes from
// one state of the catalog even while the scanner or the identification
// runner write. f must read through tx only: the catalog has a single
// connection, which the transaction holds.
func (s *Store) read(f func(tx querier) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return f(tx)
}

// Snapshot reads versions, identifications and movies in one transaction.
func (s *Store) Snapshot() (Snapshot, error) {
	snap := Snapshot{Identifications: map[string]*Identification{}, Movies: map[int]Movie{}}
	err := s.read(func(tx querier) error {
		vs, err := s.versions(tx)
		if err != nil {
			return err
		}
		snap.Versions = vs
		if !s.hasIdentity {
			return nil
		}
		if snap.Identifications, err = s.identifications(tx); err != nil {
			return err
		}
		ms, err := s.queryMovies(tx, `SELECT `+movieColumns+` FROM movies ORDER BY tmdb_id`)
		for _, m := range ms {
			snap.Movies[m.TMDBID] = m
		}
		return err
	})
	return snap, err
}
