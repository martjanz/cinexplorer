//go:build !windows

package platform

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// Open opens target (file or URL) with its default application.
func Open(target string) error {
	if runtime.GOOS == "darwin" {
		return start(exec.Command("open", target))
	}
	return start(exec.Command("xdg-open", target))
}

// Reveal shows target in the file manager (selected on macOS, its folder on Linux).
func Reveal(target string) error {
	if runtime.GOOS == "darwin" {
		return start(exec.Command("open", "-R", target))
	}
	return start(exec.Command("xdg-open", filepath.Dir(target)))
}
