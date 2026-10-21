package store

import (
	"encoding/json"
	"strconv"

	"cinexplorer/internal/probe"
	"cinexplorer/internal/quality"
)

type mediaRow struct {
	kind       string
	durationMs int64
	width      int
	height     int
	videoCodec string
	audio      []probe.Track
	subs       []probe.Track
}

// attachMedia fills the technical fields of each version from the probe
// results of its present main files. Stale results (the file changed since)
// and failed ones are ignored, so the name-parsed values remain.
func (s *Store) attachMedia(tx querier, out []VersionView, pos map[int64]int) error {
	if !s.hasMedia {
		return nil
	}
	byVersion, order, err := mediaByVersion(tx)
	if err != nil {
		return err
	}
	for _, id := range order {
		if i, ok := pos[id]; ok {
			applyMedia(&out[i], byVersion[id])
		}
	}
	return nil
}

// versionDurations returns each version's duration in ms (see mediaParts),
// from the same probe results attachMedia uses. Versions without usable
// results are absent.
func (s *Store) versionDurations(tx querier) (map[int64]int64, error) {
	out := map[int64]int64{}
	if !s.hasMedia {
		return out, nil
	}
	byVersion, _, err := mediaByVersion(tx)
	if err != nil {
		return nil, err
	}
	for id, ms := range byVersion {
		var d int64
		for _, m := range mediaParts(ms) {
			d += m.durationMs
		}
		out[id] = d
	}
	return out, nil
}

// mediaByVersion loads the probe results of present main files whose probe is
// current and valid, per version in part order, plus the versions' order.
func mediaByVersion(tx querier) (map[int64][]mediaRow, []int64, error) {
	rows, err := tx.Query(`SELECT f.version_id, f.kind, m.duration_ms, m.width, m.height, m.video_codec, m.audio, m.subs
		FROM files f JOIN media m ON m.file_id = f.id
		WHERE f.version_id IS NOT NULL AND f.role = 'main' AND f.missing = 0 AND m.error = ''
		  AND m.size = f.size AND m.mtime = f.mtime
		ORDER BY f.version_id, f.part, f.path`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	byVersion := map[int64][]mediaRow{}
	var order []int64
	for rows.Next() {
		var id int64
		var m mediaRow
		var audio, subs string
		if err := rows.Scan(&id, &m.kind, &m.durationMs, &m.width, &m.height, &m.videoCodec, &audio, &subs); err != nil {
			return nil, nil, err
		}
		if err := json.Unmarshal([]byte(audio), &m.audio); err != nil {
			return nil, nil, err
		}
		if err := json.Unmarshal([]byte(subs), &m.subs); err != nil {
			return nil, nil, err
		}
		if _, seen := byVersion[id]; !seen {
			order = append(order, id)
		}
		byVersion[id] = append(byVersion[id], m)
	}
	return byVersion, order, rows.Err()
}

// mediaParts returns the results that make up the movie: a DVD holds several
// title sets and the longest one is the movie; otherwise all parts count.
func mediaParts(ms []mediaRow) []mediaRow {
	if ms[0].kind == "dvd" {
		longest := ms[0]
		for _, m := range ms[1:] {
			if m.durationMs > longest.durationMs {
				longest = m
			}
		}
		return []mediaRow{longest}
	}
	return ms
}

// applyMedia merges the probe results of a version's parts, in part order.
func applyMedia(v *VersionView, ms []mediaRow) {
	ms = mediaParts(ms)
	first := ms[0]
	v.Width, v.Height, v.VideoCodec = first.width, first.height, first.videoCodec
	for _, m := range ms {
		v.DurationMs += m.durationMs
		v.Audio = appendUnique(v.Audio, m.audio)
		v.Subs = appendUnique(v.Subs, m.subs)
	}
	if label := probe.ResolutionLabel(v.Width, v.Height); label != "" {
		v.Resolution = label
	}
	if v.VideoCodec != "" {
		v.Codec = v.VideoCodec
	}
}

func appendUnique(dst, src []probe.Track) []probe.Track {
	for _, t := range src {
		dup := false
		for _, d := range dst {
			if d == t {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, t)
		}
	}
	return dst
}

// markBest flags the best version in each group of two or more present
// versions of the same movie: the same TMDB id when identified, else the
// same provisional identity (title + year). Full ties go to the lowest id,
// so the choice is stable. It must see every version, not a filtered subset.
func markBest(vs []VersionView) {
	best := map[string]int{}
	count := map[string]int{}
	for i := range vs {
		v := &vs[i]
		key := quality.GroupKey(v.Title, v.Year)
		if v.Movie != nil {
			key = "tmdb:" + strconv.Itoa(v.Movie.TMDBID)
		}
		if key == "" || !hasPresentMain(v) {
			continue
		}
		count[key]++
		j, ok := best[key]
		if !ok {
			best[key] = i
			continue
		}
		if c := quality.Compare(candidate(v), candidate(&vs[j])); c > 0 || (c == 0 && v.ID < vs[j].ID) {
			best[key] = i
		}
	}
	for key, i := range best {
		if count[key] > 1 {
			vs[i].Best = true
		}
	}
}

func candidate(v *VersionView) quality.Candidate {
	return quality.Candidate{Resolution: v.Resolution, Codec: v.Codec, Size: v.Size}
}

func hasPresentMain(v *VersionView) bool {
	for _, f := range v.Files {
		if f.Role == "main" && !f.Missing {
			return true
		}
	}
	return false
}
