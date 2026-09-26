package identify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// corpusFile holds trimmed TMDB responses keyed by request URI. Re-record
// with CINEXPLORER_TMDB_RECORD=1 and a token in CINEXPLORER_TMDB_TOKEN (or
// ~/.cinexplorer-tmdb-token).
const corpusFile = "testdata/tmdb_corpus.json"

// corpus lists real names from the collection and the expected outcome: a
// TMDB id for an automatic match, or 0 for "left for review".
var corpus = []struct {
	name string
	want int
}{
	{"Amarcord [Federico Fellini, 1973]", 7857},
	{"Chinatown (Polanski, USA, 1974)", 829},
	{"Arabian.Nights.1974.CC.1080p.BluRay.FLAC1.0.x264-ADE", 47406},
	{"A Woman Under the Influence [John Cassavetes, 1974]", 29845},
	{"Annie Hall [1977, USA]", 703},
	{"Apocalypse Now (Francis Ford Coppola, 1979)", 28},
	{"Cries and Whispers [Ingmar Bergman, 1972]", 10238},
	{"Eraserhead (Lynch, USA, 1976)", 985},
	{"La piel dura (L'argent de poche) (France, 1976)", 1660},
	{"La.Patagonia.rebelde.1974.Hector.Olivera.WEB-DL.1080p.SPA.RUS.Sub.SPA.ENG.RUS.(emule.via..clan-sudamerica.net)", 64978},
	{"Los Gauchos Judíos (Juan José Jusid, Argentina, 1974)", 537898},
	{"Los traidores (Raymundo Gleyzer, 1972)", 0},
	{"Mad Max (Miller, Australia, 1979) [1080p]", 9659},
	{"Nazareno.Cruz.Y.El.Lobo.1975.720p.WEB-DL.AAC2.0.H.264-gooz.(Found.via.clan-sudamerica.net)", 127424},
	{"Salo o le 120 giornate di Sodoma (1975) [Italy]", 5336},
	{"Scener ur ett äktenskap (1973)", 133919},
	{"Solyaris.[Solaris].1972.DVDRip.H264.AAC.Gopo", 593},
	{"Star.Wars.Episode.IV.A.New.Hope.1977.REMASTERED.1080p.BluRay.x265-RARBG", 0},
	{"The Omen [Donner] (UK, 1976)", 794},
	{"Paper Moon (1973)", 11293},
	{"Hamaca Paraguaya", 0},
	{"Shoah", 42044},
	{"Destino.final-zoe(Emule.via.clan-sudamerica.net)", 0},
	{"Ordet", 0},
	{"Our Litlle Sister", 0},
	{"El cazador (Shekarchi)", 0},
	{"Husbands (John Cassavetes, 1970) [Extended Cut, 142min] XVID-KG", 52105},
	{"1900 (Novecento) (1976) [mkvonly]", 3870},
	{"Diarios de motocicleta", 1653},
	{"Grey Gardens (1975) 1080p.BluRay.H264.AAC-RARBG", 17346},
}

func TestCorpus(t *testing.T) {
	c := corpusClient(t)
	ctx := context.Background()
	auto := 0
	for _, tc := range corpus {
		p := nameparse.Parse(tc.name)
		q := Query{Title: p.Title, Year: p.Year, Director: p.Director, IMDbID: p.IMDbID}
		id, _, err := Identify(ctx, c, "es-ES", q)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		got := 0
		if id.Status == store.StatusAuto {
			got = id.TMDBID
			auto++
		}
		t.Logf("%-50.50s → %-9s %6d  %.2f  %s", tc.name, id.Status, id.TMDBID, id.Confidence, describe(id.Candidates))
		if got != tc.want {
			t.Errorf("%s: got %s %d (%.2f), want %d", tc.name, id.Status, got, id.Confidence, tc.want)
		}
	}
	t.Logf("automáticas: %d/%d", auto, len(corpus))
}

func describe(cs []store.Candidate) string {
	var parts []string
	for _, c := range cs[:min(3, len(cs))] {
		parts = append(parts, fmt.Sprintf("%s (%d) %d %.2f", c.Title, c.Year, c.TMDBID, c.Score))
	}
	return strings.Join(parts, " | ")
}

// corpusClient serves the recorded responses, or records them first when
// CINEXPLORER_TMDB_RECORD is set.
func corpusClient(t *testing.T) *tmdb.Client {
	t.Helper()
	recorded := map[string]json.RawMessage{}
	if b, err := os.ReadFile(corpusFile); err == nil {
		if err := json.Unmarshal(b, &recorded); err != nil {
			t.Fatal(err)
		}
	}
	record := os.Getenv("CINEXPLORER_TMDB_RECORD") != ""
	var real *tmdb.Client
	if record {
		real = tmdb.New(token(t))
	}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.RequestURI()
		mu.Lock()
		body, ok := recorded[key]
		mu.Unlock()
		if !ok && record {
			raw, err := real.HTTP.Get(r.Context(), real.BaseURL+key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			body = trim(t, key, raw)
			mu.Lock()
			recorded[key] = body
			mu.Unlock()
			ok = true
		}
		if !ok {
			t.Errorf("sin respuesta grabada para %s: regrabá con CINEXPLORER_TMDB_RECORD=1", key)
			http.Error(w, "not recorded", http.StatusNotFound)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(func() {
		srv.Close()
		if record {
			save(t, recorded)
		}
	})
	c := tmdb.New("test")
	c.BaseURL = srv.URL
	c.HTTP.Limiter = nil
	c.HTTP.Attempts = 1
	return c
}

func token(t *testing.T) string {
	if tok := os.Getenv("CINEXPLORER_TMDB_TOKEN"); tok != "" {
		return tok
	}
	home, _ := os.UserHomeDir()
	b, err := os.ReadFile(filepath.Join(home, ".cinexplorer-tmdb-token"))
	if err != nil {
		t.Fatal("grabar necesita CINEXPLORER_TMDB_TOKEN o ~/.cinexplorer-tmdb-token")
	}
	return strings.TrimSpace(string(b))
}

// trim keeps only what the client reads, so the fixture stays small.
func trim(t *testing.T, key string, raw []byte) json.RawMessage {
	var out any
	switch {
	case strings.HasPrefix(key, "/search/movie"), strings.HasPrefix(key, "/find/"):
		var v struct {
			Results      []tmdb.Result `json:"results,omitempty"`
			MovieResults []tmdb.Result `json:"movie_results,omitempty"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(key, "/search/movie") && v.Results == nil {
			v.Results = []tmdb.Result{}
		}
		out = v
	case strings.HasPrefix(key, "/movie/"):
		var d tmdb.Details
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		crew := []tmdb.Person{}
		for _, p := range d.Credits.Crew {
			if p.Job == "Director" {
				crew = append(crew, p)
			}
		}
		d.Credits.Crew = crew
		d.Credits.Cast = d.Credits.Cast[:min(10, len(d.Credits.Cast))]
		out = d
	default:
		return raw
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func save(t *testing.T, recorded map[string]json.RawMessage) {
	keys := make([]string, 0, len(recorded))
	for k := range recorded {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("{\n")
	for i, k := range keys {
		kb, _ := json.Marshal(k)
		sb.Write(kb)
		sb.WriteString(": ")
		sb.Write(recorded[k])
		if i < len(keys)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	if err := os.MkdirAll(filepath.Dir(corpusFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpusFile, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
