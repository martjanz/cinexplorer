// Package config reads and writes config.json next to the executable.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"cinexplorer/internal/appdir"
)

const FileName = "config.json"

// DefaultLanguage is the metadata language of a new configuration.
const DefaultLanguage = "es-AR"

// Languages are the metadata languages offered in the settings.
var Languages = []string{"es-AR", "en-US", "pt-BR"}

// PrefetchModes are the values of imagePrefetch; the first is the default.
var PrefetchModes = []string{"none", "posters", "all"}

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
		return Config{Roots: roots, Language: DefaultLanguage}, true, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("%s: %w", FileName, err)
	}
	if cfg.Language == "" {
		cfg.Language = DefaultLanguage
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

// Root turns a folder the user typed (absolute, or relative to the app
// directory) into catalog form: relative to appDir, '/'-separated. The folder
// must exist, and neither be, contain nor sit inside the app directory.
func Root(appDir, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("falta la carpeta")
	}
	abs := input
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(appDir, filepath.FromSlash(abs))
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("no existe la carpeta %s", input)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("%s no es una carpeta", input)
	}
	rel, err := appdir.Rel(appDir, abs)
	if err != nil {
		// Another drive on Windows: the catalog only keeps relative paths.
		return "", fmt.Errorf("%s tiene que estar en el mismo disco que la app", input)
	}
	rel = path.Clean(rel)
	switch {
	case rel == ".":
		return "", errors.New("esa es la carpeta de la app")
	case onlyParents(rel):
		return "", errors.New("esa carpeta contiene a la de la app")
	case rel != ".." && !strings.HasPrefix(rel, "../"):
		return "", errors.New("esa carpeta está dentro de la de la app")
	}
	return rel, nil
}

// onlyParents reports whether rel is "..", "../.." and so on.
func onlyParents(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if part != ".." {
			return false
		}
	}
	return true
}

// CheckRoots validates the roots of a configuration. Roots in saved (the
// current configuration) are accepted as they are, even when unavailable
// (an unplugged drive); new ones must pass Root and already be in catalog
// form. No root may repeat or contain another.
func CheckRoots(appDir string, roots, saved []string) error {
	if len(roots) == 0 {
		return errors.New("elegí al menos una carpeta")
	}
	for i, r := range roots {
		if !slices.Contains(saved, r) {
			rel, err := Root(appDir, r)
			if err != nil {
				return err
			}
			if rel != r {
				return fmt.Errorf("carpeta mal escrita: %s", r)
			}
		}
		for _, o := range roots[:i] {
			switch {
			case o == r:
				return fmt.Errorf("carpeta repetida: %s", r)
			case strings.HasPrefix(r, o+"/"), strings.HasPrefix(o, r+"/"):
				return fmt.Errorf("%s y %s están una dentro de la otra", o, r)
			}
		}
	}
	return nil
}

// Available reports whether a root in catalog form is a folder right now.
func Available(appDir, root string) bool {
	st, err := os.Stat(appdir.Abs(appDir, root))
	return err == nil && st.IsDir()
}
