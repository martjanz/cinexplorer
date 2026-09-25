package probe

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestCorpusAgainstFFprobe compares the native readers with ffprobe on a real
// movie folder. It never runs in CI:
//
//	CINEXPLORER_PROBE_CORPUS=D:/cine go test ./internal/probe -run Corpus -v -timeout 0
//
// CINEXPLORER_PROBE_LIMIT caps the number of files compared.
func TestCorpusAgainstFFprobe(t *testing.T) {
	root := os.Getenv("CINEXPLORER_PROBE_CORPUS")
	if root == "" {
		t.Skip("CINEXPLORER_PROBE_CORPUS not set")
	}
	ff, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}
	limit, _ := strconv.Atoi(os.Getenv("CINEXPLORER_PROBE_LIMIT"))
	ref := Prober{FFprobe: ff}
	var compared, mismatched, nativeFailed int

	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".mkv", ".webm", ".mp4", ".m4v", ".mov", ".avi", ".divx": // ffprobe cannot read IFO files
		default:
			return nil
		}
		if limit > 0 && compared >= limit {
			return filepath.SkipAll
		}
		want, err := ref.ffprobe(context.Background(), p)
		if err != nil {
			return nil // ffprobe cannot read it either
		}
		compared++
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		st, _ := f.Stat()
		head := make([]byte, 16)
		n, _ := f.ReadAt(head, 0)
		read := nativeReader(head[:n])
		if read == nil {
			return nil // not a format we read natively, whatever the extension says
		}
		got, err := read(newSource(f, st.Size()))
		if err != nil {
			nativeFailed++
			t.Errorf("%s: native failed: %v", p, err)
			return nil
		}
		if diffs := corpusDiffs(got, want); len(diffs) > 0 {
			mismatched++
			t.Errorf("%s:\n  %s", p, strings.Join(diffs, "\n  "))
		}
		return nil
	})
	t.Logf("compared %d files: %d mismatched, %d native failures", compared, mismatched, nativeFailed)
}

func corpusDiffs(got, want Info) []string {
	var d []string
	add := func(field string, g, w any) { d = append(d, fmt.Sprintf("%s: native %v, ffprobe %v", field, g, w)) }
	if got.Width != want.Width || got.Height != want.Height {
		add("size", fmt.Sprintf("%dx%d", got.Width, got.Height), fmt.Sprintf("%dx%d", want.Width, want.Height))
	}
	if diff := got.DurationMs - want.DurationMs; diff > 2000 || diff < -2000 {
		add("duration", got.DurationMs, want.DurationMs)
	}
	if got.VideoCodec != want.VideoCodec {
		add("video", got.VideoCodec, want.VideoCodec)
	}
	if len(got.Audio) != len(want.Audio) {
		add("audio tracks", got.Audio, want.Audio)
	} else {
		for i := range got.Audio {
			g, w := got.Audio[i], want.Audio[i]
			if g.Codec != w.Codec || g.Channels != w.Channels || !sameLang(g.Lang, w.Lang) {
				add(fmt.Sprintf("audio %d", i), g, w)
			}
		}
	}
	if len(got.Subs) != len(want.Subs) {
		add("subtitle tracks", got.Subs, want.Subs)
	} else {
		for i := range got.Subs {
			g, w := got.Subs[i], want.Subs[i]
			if (w.Codec != "" && g.Codec != w.Codec) || !sameLang(g.Lang, w.Lang) {
				add(fmt.Sprintf("subs %d", i), got.Subs[i], want.Subs[i])
			}
		}
	}
	return d
}

// sameLang ignores a side that has no language: the native readers know
// fields ffprobe leaves empty (Matroska LanguageIETF) and AVI has none.
func sameLang(a, b string) bool { return a == "" || b == "" || a == b }
