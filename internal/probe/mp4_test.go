package probe

import (
	"bytes"
	"errors"
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
