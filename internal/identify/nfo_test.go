package identify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIMDbFromNFOs(t *testing.T) {
	disk := t.TempDir()
	app := filepath.Join(disk, "app")
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(disk, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.nfo", "no id here")
	write("b.nfo", "imdb: http://www.imdb.com/title/tt0071129/ and tt0000001")
	write("big.nfo", strings.Repeat("x", nfoLimit)+" tt0079944")
	if got := imdbFromNFOs(app, []string{"../missing.nfo", "../a.nfo", "../b.nfo"}); got != "tt0071129" {
		t.Errorf("got %q", got)
	}
	if got := imdbFromNFOs(app, []string{"../big.nfo"}); got != "" {
		t.Errorf("read past the limit: %q", got)
	}
}
