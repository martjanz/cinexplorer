package probe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	pt "cinexplorer/internal/probe/probetest"
)

const fakeFFprobeEnv = "CINEXPLORER_FAKE_FFPROBE"

// Recorded from `ffprobe -v error -print_format json -show_format -show_streams`
// on a RealMedia file (trimmed).
const rmvbJSON = `{
  "streams": [
    {"index": 0, "codec_name": "cook", "codec_type": "audio", "channels": 2, "tags": {"language": "spa"}},
    {"index": 1, "codec_name": "rv40", "codec_type": "video", "width": 640, "height": 352},
    {"index": 2, "codec_name": "mjpeg", "codec_type": "video", "width": 300, "height": 300, "disposition": {"attached_pic": 1}}
  ],
  "format": {"format_name": "rm", "duration": "5821.370000"}
}`

// TestMain lets the test binary stand in for ffprobe: when the environment
// variable is set, it prints a canned answer instead of running the tests.
func TestMain(m *testing.M) {
	switch os.Getenv(fakeFFprobeEnv) {
	case "ok":
		os.Stdout.WriteString(rmvbJSON)
		os.Exit(0)
	case "fail":
		os.Stderr.WriteString("Invalid data found when processing input\n")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestParseFFprobe(t *testing.T) {
	got, err := parseFFprobe([]byte(rmvbJSON))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "rm", DurationMs: 5_821_370, Width: 640, Height: 352, VideoCodec: "rv",
		Audio: []Track{{Codec: "cook", Lang: "es", Channels: 2}}, Prober: "ffprobe"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if _, err := parseFFprobe([]byte(`{"streams": [], "format": {}}`)); err == nil {
		t.Fatal("no streams must be an error")
	}
	absurd := `{"streams": [{"codec_type": "video", "codec_name": "h264", "width": 100000, "height": -1},
		{"codec_type": "audio", "codec_name": "aac", "channels": 1000}], "format": {"duration": "-5"}}`
	got, err = parseFFprobe([]byte(absurd))
	if err != nil || got.DurationMs != 0 || got.Width != 0 || got.Height != 0 || got.Audio[0].Channels != 0 {
		t.Fatalf("absurd values: %+v, %v", got, err)
	}
}

func write(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProbeNative(t *testing.T) {
	p := write(t, "Stalker.1979.MKV", pt.MKV(1920, 1080, 9_720_000, "rus"))
	got, err := Prober{}.Probe(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Prober != "native" || got.Height != 1080 || got.Audio[0].Lang != "ru" {
		t.Fatalf("got %+v", got)
	}
}

func TestProbeTrustsContentOverExtension(t *testing.T) {
	got, err := Prober{}.Probe(context.Background(), write(t, "Soy Cuba.avi", pt.MKV(720, 576, 1000, "spa")))
	if err != nil || got.Container != "matroska" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestProbeErrorsWithoutFFprobe(t *testing.T) {
	ctx := context.Background()
	if _, err := (Prober{}).Probe(ctx, filepath.Join(t.TempDir(), "missing.mkv")); !errors.Is(err, ErrIO) {
		t.Errorf("missing file: %v, want ErrIO", err)
	}
	if _, err := (Prober{}).Probe(ctx, write(t, "cut.avi", []byte("RIFF\x10\x00\x00\x00AVI LIST"))); !errors.Is(err, ErrInvalid) {
		t.Errorf("truncated avi: %v, want ErrInvalid", err)
	}
	if _, err := (Prober{}).Probe(ctx, write(t, "movie.rmvb", []byte(".RMF"))); !errors.Is(err, ErrUnsupported) {
		t.Errorf("rmvb: %v, want ErrUnsupported", err)
	}
}

func TestProbeFallsBackToFFprobe(t *testing.T) {
	ctx := context.Background()
	fake := Prober{FFprobe: os.Args[0]}

	t.Setenv(fakeFFprobeEnv, "ok")
	got, err := fake.Probe(ctx, write(t, "movie.rmvb", []byte(".RMF")))
	if err != nil {
		t.Fatal(err)
	}
	if got.Prober != "ffprobe" || got.VideoCodec != "rv" {
		t.Fatalf("got %+v", got)
	}
	// A valid native file never reaches ffprobe.
	if got, _ := fake.Probe(ctx, write(t, "a.mkv", pt.MKV(1280, 720, 1000, "eng"))); got.Prober != "native" {
		t.Fatalf("native file probed by %q", got.Prober)
	}

	t.Setenv(fakeFFprobeEnv, "fail")
	mkv := pt.MKV(1280, 720, 1000, "eng")
	_, err = fake.Probe(ctx, write(t, "cut.mkv", mkv[:40]))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("failed fallback: %v, want the native ErrInvalid", err)
	}
}

func TestProbeMissingFFprobeIsRetryable(t *testing.T) {
	// An ffprobe that cannot start is a problem of this machine, not of the
	// file: the scanner must try the file again later.
	p := Prober{FFprobe: filepath.Join(t.TempDir(), "no-ffprobe.exe")}
	if _, err := p.Probe(context.Background(), write(t, "movie.rmvb", []byte(".RMF"))); !errors.Is(err, ErrIO) {
		t.Fatalf("err = %v, want ErrIO", err)
	}
}
