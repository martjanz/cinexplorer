package probe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Prober reads headers natively and falls back to ffprobe when the native
// reader cannot parse the file or there is no native reader for its content.
type Prober struct {
	FFprobe string        // path to ffprobe; "" disables the fallback
	Timeout time.Duration // per ffprobe run; 0 means 30 s
}

var defaultProber = sync.OnceValue(func() Prober {
	// An error (including exec.ErrDot: found only relative to the current
	// directory, which exec refuses to run) means no fallback.
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		path = ""
	}
	return Prober{FFprobe: path}
})

// Probe reads path with the default Prober, which uses ffprobe only if it is
// on the PATH.
func Probe(ctx context.Context, path string) (Info, error) {
	return defaultProber().Probe(ctx, path)
}

// nativeReader picks a reader from the first bytes of the file. Extensions
// cannot be trusted: libraries hold ".avi" files that are really Matroska or ASF.
func nativeReader(head []byte) func(*source) (Info, error) {
	switch {
	case len(head) >= 4 && string(head[:4]) == "\x1a\x45\xdf\xa3":
		return readMKV
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "AVI ":
		return readAVI
	case len(head) >= 12 && string(head[:12]) == "DVDVIDEO-VTS":
		return readIFO
	case len(head) >= 8 && mp4FirstBoxes[string(head[4:8])]:
		return readMP4
	}
	return nil
}

var mp4FirstBoxes = map[string]bool{"ftyp": true, "moov": true, "mdat": true, "free": true, "skip": true, "wide": true, "pnot": true}

func (p Prober) Probe(ctx context.Context, path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}

	head := make([]byte, 16)
	n, err := f.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	nativeErr := fmt.Errorf("%w: unrecognized content in %q", ErrUnsupported, filepath.Base(path))
	if read := nativeReader(head[:n]); read != nil {
		info, err := read(newSource(f, st.Size()))
		if err == nil {
			info.Prober = "native"
			return info, nil
		}
		if !errors.Is(err, ErrInvalid) {
			return Info{}, err
		}
		nativeErr = err
	}
	if p.FFprobe == "" {
		return Info{}, nativeErr
	}
	info, err := p.ffprobe(ctx, path)
	if err != nil {
		if ctx.Err() != nil {
			return Info{}, ctx.Err()
		}
		if errors.Is(err, ErrIO) {
			return Info{}, err
		}
		return Info{}, fmt.Errorf("%w; ffprobe: %v", nativeErr, err)
	}
	return info, nil
}
