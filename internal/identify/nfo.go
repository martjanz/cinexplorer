package identify

import (
	"io"
	"os"
	"regexp"

	"cinexplorer/internal/appdir"
)

var imdbRe = regexp.MustCompile(`\btt\d{7,8}\b`)

// nfoLimit is how much of a .nfo is read; release notes put the IMDb link
// near the top, and some .nfo files are huge ASCII art.
const nfoLimit = 64 << 10

// imdbFromNFOs returns the first IMDb id found in the .nfo files (catalog
// paths, in order), or "". Unreadable files are skipped.
func imdbFromNFOs(appDir string, paths []string) string {
	for _, p := range paths {
		f, err := os.Open(appdir.Abs(appDir, p))
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(f, nfoLimit))
		f.Close()
		if err != nil {
			continue
		}
		if id := imdbRe.Find(b); id != nil {
			return string(id)
		}
	}
	return ""
}
