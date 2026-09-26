// Package config reads and writes config.json next to the executable.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const FileName = "config.json"

type Config struct {
	Roots     []string `json:"roots"`     // catalog-form paths, relative to the app dir
	TMDBToken string   `json:"tmdbToken"` // TMDB v4 read access token
	Language  string   `json:"language"`
	// ImagePrefetch downloads images ahead: "posters", "all", or "none"
	// (the default: on demand only).
	ImagePrefetch string `json:"imagePrefetch,omitempty"`
}

// Load reads config.json from appDir. When the file does not exist it returns
// defaults (the sibling directories of appDir as roots) and created=true.
func Load(appDir string) (Config, bool, error) {
	data, err := os.ReadFile(filepath.Join(appDir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		roots, err := DefaultRoots(appDir)
		if err != nil {
			return Config{}, false, err
		}
		return Config{Roots: roots, Language: "es-ES"}, true, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("%s: %w", FileName, err)
	}
	if cfg.Language == "" {
		cfg.Language = "es-ES"
	}
	return cfg, false, nil
}

func Save(appDir string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(appDir, FileName), append(data, '\n'), 0o644)
}

// DefaultRoots lists the sibling directories of appDir as "../<name>",
// skipping hidden and system folders.
func DefaultRoots(appDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(appDir))
	if err != nil {
		return nil, err
	}
	self := filepath.Base(appDir)
	roots := []string{}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name == self || skipRoot(name) {
			continue
		}
		roots = append(roots, "../"+name)
	}
	return roots, nil
}

func skipRoot(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
		return true
	}
	switch strings.ToLower(name) {
	case "system volume information", "recycler", "lost+found":
		return true
	}
	return false
}
