// Package appdir resolves the application directory and converts between
// absolute paths and the portable, app-relative form stored in the catalog.
package appdir

import (
	"os"
	"path/filepath"
)

// Resolve returns override (made absolute) if set, otherwise the directory
// holding the running executable.
func Resolve(override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// Rel converts an absolute path into catalog form: relative to appDir and
// '/'-separated, so the same catalog works on every OS and mount point.
func Rel(appDir, abs string) (string, error) {
	r, err := filepath.Rel(appDir, abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(r), nil
}

// Abs converts a catalog path back into an absolute OS path.
func Abs(appDir, rel string) string {
	return filepath.Join(appDir, filepath.FromSlash(rel))
}

// Writable reports whether files can be created in dir.
func Writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".cx-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	return os.Remove(name) == nil
}
