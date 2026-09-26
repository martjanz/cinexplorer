package probe

import (
	"errors"
	"reflect"
	"testing"

	pt "cinexplorer/internal/probe/probetest"
)

// failingReaderAt fails any read that touches an offset at or past fail, to
// simulate a transient I/O error (e.g. an unplugged drive) partway through a
// read; reads entirely before fail are served from b.
type failingReaderAt struct {
	b    []byte
	fail int64
}

func (r failingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off+int64(len(p)) > r.fail {
		return 0, errors.New("device not ready")
	}
	n := copy(p, r.b[off:])
	return n, nil
}

func TestMKVBasic(t *testing.T) {
	info, err := readMKV(src(pt.MKV(1920, 800, 6_300_000, "spa")))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "matroska", DurationMs: 6_300_000, Width: 1920, Height: 800, VideoCodec: "h264",
		Audio: []Track{{Codec: "aac", Lang: "es", Channels: 2}}}
	if !reflect.DeepEqual(info, want) {
		t.Fatalf("got  %+v\nwant %+v", info, want)
	}
}

func mkvHeader(docType string) []byte { return pt.EBML(mkvEBML, pt.EBMLString(mkvDocType, docType)) }

func track(typ uint64, codec string, extra ...[]byte) []byte {
	return pt.EBML(mkvTrackEntry, append([][]byte{pt.EBMLUint(mkvTrackType, typ), pt.EBMLString(mkvCodecID, codec)}, extra...)...)
}

func TestMKVTracksAfterClusterViaSeekHead(t *testing.T) {
	// BITMAPINFOHEADER with biCompression = XVID.
	bih := append(make([]byte, 16), []byte("XVID")...)
	tracks := pt.EBML(mkvTracks,
		track(1, "V_MS/VFW/FOURCC",
			pt.EBML(mkvCodecPrivate, bih),
			pt.EBML(mkvVideo, pt.EBMLUint(mkvPixelWidth, 640), pt.EBMLUint(mkvPixelHeight, 272))),
		track(2, "A_AC3", pt.EBMLString(mkvLanguage, "ita"), pt.EBML(mkvAudio, pt.EBMLUint(mkvChannels, 6))),
		track(2, "A_MPEG/L3"), // no Language nor Channels: Matroska defaults are English, mono
		track(17, "S_TEXT/UTF8", pt.EBMLString(mkvLanguage, "spa"), pt.EBMLString(mkvLanguageIETF, "es-419")),
		track(17, "S_VOBSUB", pt.EBMLString(mkvLanguage, "und")),
	)
	info := pt.EBML(mkvInfo, pt.EBMLFloat(mkvDuration, 90_000)) // default TimecodeScale: 1 ms
	cluster := pt.EBML(mkvCluster, make([]byte, 100))
	seekHead := func(pos uint64) []byte {
		return pt.EBML(mkvSeekHead, pt.EBML(mkvSeek,
			pt.EBML(mkvSeekID, pt.U32BE(mkvTracks)), pt.EBMLUint(mkvSeekPosition, pos)))
	}
	pos := uint64(len(seekHead(0)) + len(info) + len(cluster))
	file := append(mkvHeader("webm"), pt.EBML(mkvSegment, seekHead(pos), info, cluster, tracks)...)

	got, err := readMKV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "webm", DurationMs: 90_000, Width: 640, Height: 272, VideoCodec: "mpeg4",
		Audio: []Track{{Codec: "ac3", Lang: "it", Channels: 6}, {Codec: "mp3", Lang: "en", Channels: 1}},
		Subs:  []Track{{Codec: "srt", Lang: "es"}, {Codec: "vobsub", Lang: ""}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestMKVAACChannelsFromCodecPrivate(t *testing.T) {
	// No Channels element (default 1), but the AudioSpecificConfig
	// (AAC LC, 48 kHz, stereo) says 2.
	file := append(mkvHeader("matroska"), pt.EBML(mkvSegment,
		pt.EBML(mkvTracks, track(2, "A_AAC", pt.EBML(mkvCodecPrivate, []byte{0x11, 0x90}))),
	)...)
	got, err := readMKV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Audio, []Track{{Codec: "aac", Lang: "en", Channels: 2}}) {
		t.Fatalf("audio %+v", got.Audio)
	}
}

func TestMKVUnknownSizeSegmentAndTimecodeScale(t *testing.T) {
	file := append(mkvHeader("matroska"), pt.EBMLUnknown(mkvSegment,
		pt.EBML(mkvInfo, pt.EBMLUint(mkvTimecodeScale, 1_000_000_000), pt.EBMLFloat(mkvDuration, 5400)),
		pt.EBML(mkvTracks, track(1, "V_MPEGH/ISO/HEVC",
			pt.EBML(mkvVideo, pt.EBMLUint(mkvPixelWidth, 3840), pt.EBMLUint(mkvPixelHeight, 1600)))),
	)...)
	got, err := readMKV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMs != 5_400_000 || got.VideoCodec != "hevc" || got.Width != 3840 {
		t.Fatalf("got %+v", got)
	}
}

func TestMKVIOErrorStaysErrIO(t *testing.T) {
	// TrackType is the last element in the file: its 9-byte header (id+size)
	// fits in readElem's 12-byte lookahead read, which also grabs the first
	// 3 bytes of its 8-byte value, leaving the last 5 value bytes to be read
	// separately by uintElem. Fail exactly at that boundary so the header
	// read succeeds (parsing reaches uintElem) but the value read fails with
	// a non-EOF error, which must surface as ErrIO, not ErrInvalid.
	head := mkvHeader("matroska")
	trackType := pt.EBMLUint(mkvTrackType, 1) // 1(id)+8(size)+8(data) = 17 bytes
	te := pt.EBML(mkvTrackEntry, trackType)
	tracks := pt.EBML(mkvTracks, te)
	seg := pt.EBML(mkvSegment, tracks)
	file := append(head, seg...)

	fail := int64(len(file) - 5)
	r := failingReaderAt{b: file, fail: fail}
	if _, err := readMKV(newSource(r, int64(len(file)))); !errors.Is(err, ErrIO) {
		t.Fatalf("err = %v, want ErrIO", err)
	}
}

func TestMKVNegativeDurationIgnored(t *testing.T) {
	file := append(mkvHeader("matroska"), pt.EBML(mkvSegment,
		pt.EBML(mkvInfo, pt.EBMLFloat(mkvDuration, -5)),
		pt.EBML(mkvTracks, track(1, "V_MPEG4/ISO/AVC")),
	)...)
	got, err := readMKV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMs != 0 {
		t.Fatalf("DurationMs = %d, want 0", got.DurationMs)
	}
}

func TestMKVAbsurdChannelsIgnored(t *testing.T) {
	file := append(mkvHeader("matroska"), pt.EBML(mkvSegment,
		pt.EBML(mkvTracks, track(2, "A_AC3", pt.EBML(mkvAudio, pt.EBMLUint(mkvChannels, 200)))))...)
	got, err := readMKV(src(file))
	if err != nil || got.Audio[0].Channels != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestMKVAbsurdPixelWidthIgnored(t *testing.T) {
	file := append(mkvHeader("matroska"), pt.EBML(mkvSegment,
		pt.EBML(mkvTracks, track(1, "V_MPEG4/ISO/AVC",
			pt.EBML(mkvVideo, pt.EBMLUint(mkvPixelWidth, 1<<40), pt.EBMLUint(mkvPixelHeight, 720)))),
	)...)
	got, err := readMKV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 0 {
		t.Fatalf("Width = %d, want 0", got.Width)
	}
}

func TestMKVInvalid(t *testing.T) {
	good := pt.MKV(1280, 720, 1000, "eng")
	for name, b := range map[string][]byte{
		"not ebml":   []byte("RIFF\x00\x00\x00\x00AVI LIST"),
		"truncated":  good[:len(good)-10],
		"bad doc":    append(mkvHeader("foo"), pt.EBML(mkvSegment)...),
		"no tracks":  append(mkvHeader("matroska"), pt.EBML(mkvSegment, pt.EBML(mkvInfo))...),
		"empty file": {},
	} {
		if _, err := readMKV(src(b)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}
