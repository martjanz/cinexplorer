package catalog

import (
	"cinexplorer/internal/probe"
	"cinexplorer/internal/store"
)

var roots = []string{"../cine", "../cine-ordenar"}

// version builds a present one-file version.
func version(id int64, dir, file, fp, title string, year int, res string, size, added int64) store.VersionView {
	return store.VersionView{ID: id, Dir: dir, Title: title, Year: year, Resolution: res, Size: size, Parts: 1,
		Added: added, Fingerprint: fp, Audio: []probe.Track{}, Subs: []probe.Track{},
		Files: []store.FileView{{Path: dir + "/" + file, Size: size, Role: "main"}}}
}

func identified(v store.VersionView, status string, tmdbID int) store.VersionView {
	v.Identification = &store.IdentView{Status: status, TMDBID: tmdbID, Confidence: 0.9}
	return v
}

// snapshot is a small catalog:
//   - Amarcord (7857): a 1080p version (best) and an SD version present twice
//     (identical copies), plus a making-of marked as its extra;
//   - Roma (7858): one version, in a TMDB collection;
//   - Novecento: unmatched; Stalker: never identified; a sample marked as not
//     a movie; a file not hashed yet; a version identified as a movie that is
//     not stored (yet); a version whose file is missing.
func snapshot() store.Snapshot {
	a1 := identified(version(1, "../cine/1970s/Amarcord", "Amarcord.1973.1080p.mkv", "a1", "Amarcord", 1973, "1080p", 9800, 100), store.StatusAuto, 7857)
	a1.Best = true
	a1.Subs = []probe.Track{{Codec: "srt", Lang: "en"}}
	a2 := identified(version(2, "../cine-ordenar", "Amarcord CD1.avi", "a2", "Amarcord", 0, "SD", 1400, 200), store.StatusManual, 7857)
	a2.SubLangs = "es,?"
	a3 := identified(version(3, "../cine/Collections/Fellini", "Amarcord CD1.avi", "a2", "Amarcord", 0, "SD", 1400, 300), store.StatusManual, 7857)
	a3.SubLangs = "es"
	extra := identified(version(4, "../cine/1970s/Amarcord/Extras", "Making of.avi", "e1", "Making of", 0, "", 350, 100), store.StatusExtra, 7857)
	roma := identified(version(5, "../cine/1970s/Roma", "Roma.mkv", "r1", "Roma", 1972, "1080p", 5000, 50), store.StatusAuto, 7858)
	nove := identified(version(6, "../cine/1970s", "Novecento.avi", "n1", "Novecento", 1976, "720p", 700, 400), store.StatusUnmatched, 0)
	nove.Director = "Bertolucci"
	stalker := version(7, "../cine-ordenar/Tarkovsky", "Stalker.avi", "s1", "Stalker", 1979, "576p", 800, 500)
	sample := identified(version(8, "../cine-ordenar", "sample.mkv", "x1", "sample", 0, "", 10, 500), store.StatusIgnored, 0)
	unhashed := version(9, "../cine-ordenar", "Vacío.avi", "", "Vacío", 0, "", 0, 600)
	solaris := identified(version(10, "../cine-ordenar", "Solaris.1972.avi", "p1", "Solaris", 1972, "", 900, 700), store.StatusAuto, 555)
	gone := version(11, "../cine/1980s", "Gone.avi", "g1", "Gone", 1985, "", 100, 10)
	gone.Files[0].Missing = true

	snap := store.Snapshot{
		Versions: []store.VersionView{a1, a2, a3, extra, roma, nove, stalker, sample, unhashed, solaris, gone},
		Identifications: map[string]*store.Identification{
			"n1": {Fingerprint: "n1", Status: store.StatusUnmatched, Candidates: []store.Candidate{{TMDBID: 1, Title: "Novecento", Year: 1976, Score: 0.7}}},
		},
		Movies: map[int]store.Movie{
			7857: {TMDBID: 7857, Title: "Amarcord", OriginalTitle: "Amarcord", Year: 1973, OriginalLang: "it",
				Directors: []store.Person{{ID: 4415, Name: "Federico Fellini"}}, Genres: []string{"Comedia", "Drama"},
				Countries: []string{"IT", "FR"}, PosterPath: "/pa.jpg"},
			7858: {TMDBID: 7858, Title: "Roma", OriginalTitle: "Roma", Year: 1972, OriginalLang: "it",
				Directors: []store.Person{{ID: 4415, Name: "Federico Fellini"}}, Genres: []string{"Drama"},
				Countries: []string{"IT"}, CollectionID: 99, Collection: "Fellini"},
		},
	}
	// The identification rows behind the versions' identity.
	for _, v := range snap.Versions {
		if id := v.Identification; id != nil && snap.Identifications[v.Fingerprint] == nil {
			snap.Identifications[v.Fingerprint] = &store.Identification{Fingerprint: v.Fingerprint, Status: id.Status, TMDBID: id.TMDBID}
		}
	}
	return snap
}
