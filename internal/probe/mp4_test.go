package probe

import (
	"bytes"
	"errors"
	"math"
	"reflect"
	"testing"

	pt "cinexplorer/internal/probe/probetest"
)

func packLang(l string) []byte {
	return pt.U16BE(uint16(l[0]-0x60)<<10 | uint16(l[1]-0x60)<<5 | uint16(l[2]-0x60))
}

func mp4Trak(id uint32, handler, lang string, entry []byte, extra ...[]byte) []byte {
	return pt.Box("trak", append([][]byte{
		pt.FullBox("tkhd", 0, zeros(8), pt.U32BE(id), zeros(68)),
		pt.Box("mdia",
			pt.FullBox("mdhd", 0, zeros(16), packLang(lang), zeros(2)),
			pt.FullBox("hdlr", 0, zeros(4), []byte(handler), zeros(12)),
			pt.Box("minf", pt.Box("stbl", pt.FullBox("stsd", 0, pt.U32BE(1), entry))),
		),
	}, extra...)...)
}

func videoEntry(fourcc string, w, h uint16, children ...[]byte) []byte {
	return pt.Box(fourcc, zeros(24), pt.U16BE(w), pt.U16BE(h), zeros(50), bytes.Join(children, nil))
}

func audioEntry(fourcc string, ch uint16, children ...[]byte) []byte {
	return pt.Box(fourcc, zeros(16), pt.U16BE(ch), zeros(10), bytes.Join(children, nil))
}

// esds with an ES_Descriptor holding a DecoderConfigDescriptor.
func esds(objectType byte) []byte {
	return pt.FullBox("esds", 0, []byte{0x03, 0x80, 0x80, 0x80, 0x19, 0x00, 0x01, 0x00, 0x04, 0x11, objectType, 0x15})
}

func TestMP4FastStart(t *testing.T) {
	file := bytes.Join([][]byte{
		pt.Box("ftyp", []byte("isom"), zeros(4)),
		pt.Box("moov",
			pt.FullBox("mvhd", 0, zeros(8), pt.U32BE(1000), pt.U32BE(7_200_000)),
			mp4Trak(1, "vide", "und", videoEntry("avc1", 1920, 1080), pt.Box("tref", pt.Box("chap", pt.U32BE(5)))),
			mp4Trak(2, "soun", "spa", audioEntry("mp4a", 6, esds(0x40))),
			mp4Trak(3, "soun", "eng", audioEntry("mp4a", 2, esds(0x6B))),
			mp4Trak(4, "sbtl", "spa", pt.Box("tx3g", zeros(8))),
			mp4Trak(5, "text", "eng", pt.Box("text", zeros(8))), // chapter list, not a subtitle
		),
		pt.Box("mdat", zeros(64)),
	}, nil)
	got, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "mp4", DurationMs: 7_200_000, Width: 1920, Height: 1080, VideoCodec: "h264",
		Audio: []Track{{Codec: "aac", Lang: "es", Channels: 6}, {Codec: "mp3", Lang: "en", Channels: 2}},
		Subs:  []Track{{Codec: "mov_text", Lang: "es"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestMP4MoovAtEndAfterLargeMdat(t *testing.T) {
	file := bytes.Join([][]byte{
		pt.Box("ftyp", []byte("mp42"), zeros(4)),
		pt.Box64("mdat", zeros(1000)),
		pt.Box("moov",
			pt.FullBox("mvhd", 1, zeros(16), pt.U32BE(600), pt.U64BE(600*5400)),
			mp4Trak(1, "vide", "eng", videoEntry("hvc1", 3840, 2160)),
			mp4Trak(2, "soun", "fre", audioEntry("ac-3", 6)),
		),
	}, nil)
	got, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMs != 5_400_000 || got.VideoCodec != "hevc" || got.Height != 2160 ||
		!reflect.DeepEqual(got.Audio, []Track{{Codec: "ac3", Lang: "fr", Channels: 6}}) {
		t.Fatalf("got %+v", got)
	}
}

func TestMP4Invalid(t *testing.T) {
	noMoov := bytes.Join([][]byte{pt.Box("ftyp", zeros(8)), pt.Box("mdat", zeros(8))}, nil)
	truncated := bytes.Join([][]byte{pt.Box("ftyp", zeros(8)), pt.Box("moov", zeros(100))}, nil)[:60]
	for name, b := range map[string][]byte{
		"no moov":   noMoov,
		"truncated": truncated,
		"binary":    {0x1A, 0x45, 0xDF, 0xA3, 0, 0, 0, 0, 1, 2, 3},
		"empty":     {},
	} {
		if _, err := readMP4(src(b)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestMP4QuickTimeHandlerAndAC3Channels(t *testing.T) {
	// QuickTime puts a data-reference hdlr ("alis") inside minf; the media
	// handler is the one in mdia. dac3: acmod 3/2 + LFE = 5.1.
	trak := pt.Box("trak",
		pt.FullBox("tkhd", 0, zeros(8), pt.U32BE(1), zeros(68)),
		pt.Box("mdia",
			pt.FullBox("mdhd", 0, zeros(16), packLang("eng"), zeros(2)),
			pt.FullBox("hdlr", 0, zeros(4), []byte("soun"), zeros(12)),
			pt.Box("minf",
				pt.FullBox("hdlr", 0, zeros(4), []byte("alis"), zeros(12)),
				pt.Box("stbl", pt.FullBox("stsd", 0, pt.U32BE(1),
					audioEntry("ac-3", 2, pt.Box("dac3", []byte{0x10, 0x3C, 0x00})))),
			),
		),
	)
	file := join(pt.Box("ftyp", []byte("qt  "), zeros(4)), pt.Box("moov", pt.FullBox("mvhd", 0, zeros(8), pt.U32BE(600), pt.U32BE(600)), trak))
	got, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Audio, []Track{{Codec: "ac3", Lang: "en", Channels: 6}}) {
		t.Fatalf("audio %+v", got.Audio)
	}
}

func TestMP4DeepNestingIsBounded(t *testing.T) {
	// moov > trak > mdia > mdia > … (16 MB): only real paths are followed,
	// so nesting cannot drive recursion. A generic recursive walker hit a
	// fatal stack overflow on this file.
	const depth = 2_000_000
	var b bytes.Buffer
	b.Write(pt.U32BE(uint32(16 + 8*depth)))
	b.WriteString("moov")
	b.Write(pt.U32BE(uint32(8 + 8*depth)))
	b.WriteString("trak")
	for i := range depth {
		b.Write(pt.U32BE(uint32(8 * (depth - i))))
		b.WriteString("mdia")
	}
	got, err := readMP4(src(join(pt.Box("ftyp", zeros(8)), b.Bytes())))
	if err != nil || got.VideoCodec != "" || len(got.Audio) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestMP4UnknownOrHugeDuration(t *testing.T) {
	for name, mvhd := range map[string][]byte{
		"v0 all ones": pt.FullBox("mvhd", 0, zeros(8), pt.U32BE(1000), pt.U32BE(0xFFFFFFFF)),
		"v1 all ones": pt.FullBox("mvhd", 1, zeros(16), pt.U32BE(1), pt.U64BE(math.MaxUint64)),
		"v1 overflow": pt.FullBox("mvhd", 1, zeros(16), pt.U32BE(1), pt.U64BE(1<<62)),
		"v1 max ms":   pt.FullBox("mvhd", 1, zeros(16), pt.U32BE(1000), pt.U64BE(9223372036854775999)),
	} {
		got, err := readMP4(src(join(pt.Box("ftyp", zeros(8)), pt.Box("moov", mvhd))))
		if err != nil || got.DurationMs != 0 {
			t.Errorf("%s: DurationMs = %d, err = %v; want 0, nil", name, got.DurationMs, err)
		}
	}
}

func TestMP4ReadErrorInESDSIsReported(t *testing.T) {
	file := join(pt.Box("ftyp", zeros(8)), pt.Box("moov",
		mp4Trak(1, "soun", "eng", audioEntry("mp4a", 2, pt.Box("free", zeros(40)), esds(0x40)))))
	// The padding keeps esds past the 64 bytes read from the sample entry, so
	// only the esds read itself hits the failing region.
	at := int64(bytes.LastIndex(file, []byte("esds")))
	r := failingReaderAt{b: file, fail: at + 5} // the esds header reads fine, its data does not
	if _, err := readMP4(newSource(r, int64(len(file)))); !errors.Is(err, ErrIO) {
		t.Fatalf("err = %v, want ErrIO", err)
	}
}

func TestMP4QuickTimeV2SoundChannels(t *testing.T) {
	// SoundDescription v2: @16 always says 3; the channel count is a u32 at @40.
	entry := pt.Box("lpcm", zeros(8), pt.U16BE(2), zeros(6), pt.U16BE(3), zeros(22), pt.U32BE(6), zeros(20))
	got, err := readMP4(src(join(pt.Box("ftyp", zeros(8)), pt.Box("moov", mp4Trak(1, "soun", "eng", entry)))))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Audio, []Track{{Codec: "pcm", Lang: "en", Channels: 6}}) {
		t.Fatalf("audio %+v", got.Audio)
	}
}
