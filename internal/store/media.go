package store

import (
	"encoding/json"

	"cinexplorer/internal/probe"
)

// ProbeTarget is a file whose headers need to be read.
type ProbeTarget struct {
	FileID int64
	Path   string
	Size   int64
	MTime  int64
}

// ProbeResult is what reading a target produced. Err is set when the file was
// read but could not be understood; such results are kept so the file is not
// read again until it changes.
type ProbeResult struct {
	FileID int64
	Size   int64
	MTime  int64
	Info   probe.Info
	Err    string
}

// PendingProbes returns the present main video files, and the title set IFOs
// of DVD versions, that have never been probed or changed since.
func (s *Store) PendingProbes() ([]ProbeTarget, error) {
	if !s.hasMedia {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT f.id, f.path, f.size, f.mtime FROM files f
		LEFT JOIN media m ON m.file_id = f.id
		WHERE f.missing = 0 AND f.role = 'main'
		  AND (f.kind = 'video' OR (f.kind = 'dvd' AND upper(f.path) GLOB '*/VTS_[0-9][0-9]_0.IFO'))
		  AND (m.file_id IS NULL OR m.size != f.size OR m.mtime != f.mtime)
		ORDER BY f.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProbeTarget
	for rows.Next() {
		var t ProbeTarget
		if err := rows.Scan(&t.FileID, &t.Path, &t.Size, &t.MTime); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveProbes stores a batch of results in one transaction.
func (s *Store) SaveProbes(rs []ProbeResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	up, err := tx.Prepare(`INSERT INTO media (file_id, size, mtime, prober, error, container, duration_ms,
		width, height, video_codec, audio, subs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_id) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
		prober = excluded.prober, error = excluded.error, container = excluded.container,
		duration_ms = excluded.duration_ms, width = excluded.width, height = excluded.height,
		video_codec = excluded.video_codec, audio = excluded.audio, subs = excluded.subs`)
	if err != nil {
		return err
	}
	defer up.Close()
	for _, r := range rs {
		audio, err := tracksJSON(r.Info.Audio)
		if err != nil {
			return err
		}
		subs, err := tracksJSON(r.Info.Subs)
		if err != nil {
			return err
		}
		i := r.Info
		if _, err := up.Exec(r.FileID, r.Size, r.MTime, i.Prober, r.Err, i.Container, i.DurationMs,
			i.Width, i.Height, i.VideoCodec, audio, subs); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func tracksJSON(ts []probe.Track) (string, error) {
	if ts == nil {
		ts = []probe.Track{}
	}
	b, err := json.Marshal(ts)
	return string(b), err
}
