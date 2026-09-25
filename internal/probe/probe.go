// Package probe reads technical data (resolution, codecs, duration, audio and
// subtitle tracks) from the headers of video files. Matroska, MP4, AVI and DVD
// IFO headers are read natively; ffprobe is an optional fallback.
package probe

import (
	"errors"
	"fmt"
	"io"
)

type Track struct {
	Codec    string `json:"codec"`
	Lang     string `json:"lang"`               // ISO 639-1, "" when unknown
	Channels int    `json:"channels,omitempty"` // audio only, 0 when unknown
}

type Info struct {
	Container  string
	DurationMs int64
	Width      int
	Height     int
	VideoCodec string
	Audio      []Track
	Subs       []Track
	Prober     string // "native" or "ffprobe"
}

var (
	// ErrIO wraps failures to open or read the file. They may be transient
	// (a flaky or unplugged drive), so callers retry them later.
	ErrIO = errors.New("probe: read error")
	// ErrInvalid means the header could not be parsed.
	ErrInvalid = errors.New("probe: invalid header")
	// ErrUnsupported means there is no reader for the format.
	ErrUnsupported = errors.New("probe: unsupported format")
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// maxRead caps the bytes a native reader may read from one file, so a
// malformed file never makes us walk a whole movie on an external drive.
const maxRead = 16 << 20

// source is a bounded random-access view of a file.
type source struct {
	r      io.ReaderAt
	size   int64
	budget int64
}

func newSource(r io.ReaderAt, size int64) *source {
	return &source{r: r, size: size, budget: maxRead}
}

// read returns exactly n bytes at off. Reading past the end of the file is a
// format error (a truncated or lying header); any other failure is ErrIO.
func (s *source) read(off int64, n int) ([]byte, error) {
	if off < 0 || n < 0 || off > s.size-int64(n) {
		return nil, invalid("read of %d bytes at %d past end of file (%d)", n, off, s.size)
	}
	if int64(n) > s.budget {
		return nil, invalid("header larger than %d bytes", maxRead)
	}
	s.budget -= int64(n)
	b := make([]byte, n)
	got, err := s.r.ReadAt(b, off)
	if got == n {
		return b, nil
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, invalid("truncated file")
	}
	return nil, fmt.Errorf("%w: %v", ErrIO, err)
}

// readUpTo reads min(n, size-off) bytes at off.
func (s *source) readUpTo(off int64, n int) ([]byte, error) {
	if rest := s.size - off; int64(n) > rest {
		n = int(max(rest, 0))
	}
	return s.read(off, n)
}
