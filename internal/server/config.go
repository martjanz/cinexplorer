package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"cinexplorer/internal/config"
	"cinexplorer/internal/httpx"
	"cinexplorer/internal/tmdb"
)

type rootView struct {
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

// configView is the configuration as the settings show it: the token is
// never sent back, only its last characters.
type configView struct {
	SetupPending  bool       `json:"setupPending"`
	ReadOnly      bool       `json:"readOnly"`
	Roots         []rootView `json:"roots"`
	Suggested     []string   `json:"suggested"` // sibling folders that are not roots
	HasToken      bool       `json:"hasToken"`
	TokenHint     string     `json:"tokenHint"`
	Language      string     `json:"language"`
	Languages     []string   `json:"languages"`
	ImagePrefetch string     `json:"imagePrefetch"`
}

func (s *Server) configView() configView {
	cfg := s.rt().Config
	v := configView{SetupPending: s.Engine.SetupPending(), ReadOnly: s.ReadOnly, Roots: []rootView{}, Suggested: []string{},
		HasToken: cfg.TMDBToken != "", Language: cfg.Language, Languages: config.Languages,
		ImagePrefetch: cfg.ImagePrefetch}
	for _, r := range cfg.Roots {
		v.Roots = append(v.Roots, rootView{r, config.Available(s.AppDir, r)})
	}
	siblings, _ := config.DefaultRoots(s.AppDir)
	for _, r := range siblings {
		if !slices.Contains(cfg.Roots, r) {
			v.Suggested = append(v.Suggested, r)
		}
	}
	if len(cfg.TMDBToken) >= 8 {
		v.TokenHint = "…" + cfg.TMDBToken[len(cfg.TMDBToken)-4:]
	}
	if v.ImagePrefetch == "" {
		v.ImagePrefetch = config.PrefetchModes[0]
	}
	if !slices.Contains(v.Languages, v.Language) {
		v.Languages = append(slices.Clone(v.Languages), v.Language)
	}
	return v
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.configView())
}

// checkRoot turns a folder the user typed into a root, or says why it
// cannot be one.
func (s *Server) checkRoot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	root, err := config.Root(s.AppDir, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, rootView{root, true})
}

// verifyToken asks TMDB for a well-known movie with the token, once: the
// person is waiting for the answer.
func verifyToken(ctx context.Context, token string) error {
	c := tmdb.New(token)
	c.HTTP.Attempts = 1
	_, err := c.Movie(ctx, 550, "en-US")
	return err
}

// checkToken tells whether TMDB takes a token: valid true or false, or null
// when TMDB could not be reached.
func (s *Server) checkToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		http.Error(w, "falta el token", http.StatusBadRequest)
		return
	}
	verify := s.VerifyToken
	if verify == nil {
		verify = verifyToken
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	var valid *bool
	switch err := verify(ctx, token); {
	case errors.Is(err, tmdb.ErrUnauthorized):
		valid = new(bool)
	case errors.Is(err, httpx.ErrOffline), errors.Is(err, context.DeadlineExceeded):
	default: // any other answer means TMDB took the token
		valid = new(bool)
		*valid = true
	}
	writeJSON(w, map[string]*bool{"valid": valid})
}

// putConfig validates and saves the settings, and restarts what depends on
// them. A missing or null token keeps the saved one; "" removes it.
func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly {
		http.Error(w, "modo consulta: la configuración no se puede modificar", http.StatusConflict)
		return
	}
	var req struct {
		Roots         []string `json:"roots"`
		Token         *string  `json:"token"`
		Language      string   `json:"language"`
		ImagePrefetch string   `json:"imagePrefetch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	cur := s.rt().Config
	if err := config.CheckRoots(s.AppDir, req.Roots, cur.Roots); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !slices.Contains(config.Languages, req.Language) && req.Language != cur.Language {
		http.Error(w, "idioma desconocido: "+req.Language, http.StatusBadRequest)
		return
	}
	if req.ImagePrefetch != "" && !slices.Contains(config.PrefetchModes, req.ImagePrefetch) {
		http.Error(w, "descarga de imágenes desconocida: "+req.ImagePrefetch, http.StatusBadRequest)
		return
	}
	cfg := config.Config{Roots: req.Roots, TMDBToken: cur.TMDBToken, Language: req.Language, ImagePrefetch: req.ImagePrefetch}
	if req.Token != nil {
		cfg.TMDBToken = strings.TrimSpace(*req.Token)
	}
	if err := s.Engine.Apply(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.configView())
}
