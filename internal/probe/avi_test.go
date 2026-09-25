package probe

import (
	"errors"
	"reflect"
	"testing"

	pt "cinexplorer/internal/probe/probetest"
)

func avih(usPerFrame, frames, w, h uint32) []byte {
	return pt.Chunk("avih", join(pt.U32LE(usPerFrame), zeros(12), pt.U32LE(frames), zeros(12),
		pt.U32LE(w), pt.U32LE(h), zeros(16)))
}

func strh(typ, handler string, scale, rate, length uint32) []byte {
	return pt.Chunk("strh", join([]byte(typ), []byte(handler), zeros(12), pt.U32LE(scale), pt.U32LE(rate),
		zeros(4), pt.U32LE(length), zeros(20)))
}

func vidsStrf(w, h int32, fourcc string) []byte {
	return pt.Chunk("strf", join(pt.U32LE(40), pt.U32LE(uint32(w)), pt.U32LE(uint32(h)), zeros(4), []byte(fourcc), zeros(20)))
}

func audsStrf(tag, channels uint16) []byte {
	return pt.Chunk("strf", join(pt.U16LE(tag), pt.U16LE(channels), zeros(14)))
}

func TestAVIXviD(t *testing.T) {
	file := pt.RIFF("AVI ",
		pt.List("hdrl",
			avih(40000, 150000, 640, 272),
			pt.List("strl", strh("vids", "xvid", 1, 25, 150000), vidsStrf(640, 272, "XVID")),
			pt.List("strl", strh("auds", "\x00\x00\x00\x00", 1152, 48000, 0), audsStrf(0x0055, 2)),
			pt.List("strl", strh("auds", "\x00\x00\x00\x00", 1, 1, 0), audsStrf(0x2000, 6)),
		),
		pt.List("movi", zeros(32)),
	)
	got, err := readAVI(src(file))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "avi", DurationMs: 6_000_000, Width: 640, Height: 272, VideoCodec: "mpeg4",
		Audio: []Track{{Codec: "mp3", Channels: 2}, {Codec: "ac3", Channels: 6}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestAVIOpenDMLAndJunk(t *testing.T) {
	file := pt.RIFF("AVI ",
		pt.Chunk("JUNK", zeros(7)),
		pt.List("hdrl",
			avih(41708, 1000, 0, 0), // first RIFF only: the real total is in dmlh
			pt.List("strl", strh("vids", "H264", 1, 0, 0), vidsStrf(1280, -720, "H264")),
			pt.List("odml", pt.Chunk("dmlh", join(pt.U32LE(172_000), zeros(244)))),
		),
	)
	got, err := readAVI(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMs != 172_000*41708/1000 || got.Width != 1280 || got.Height != 720 || got.VideoCodec != "h264" {
		t.Fatalf("got %+v", got)
	}
}

func TestAVIInvalid(t *testing.T) {
	good := pt.RIFF("AVI ", pt.List("hdrl", avih(40000, 10, 320, 240)))
	for name, b := range map[string][]byte{
		"not riff":  []byte("\x1aE\xdf\xa3 not an avi file"),
		"no avih":   pt.RIFF("AVI ", pt.List("hdrl", pt.Chunk("JUNK", zeros(4)))),
		"no hdrl":   pt.RIFF("AVI ", pt.List("movi", zeros(8))),
		"truncated": good[:len(good)-20],
		"wave":      pt.RIFF("WAVE", pt.Chunk("fmt ", zeros(16))),
	} {
		if _, err := readAVI(src(b)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestAVIAudioChannelFixes(t *testing.T) {
	// AAC: nChannels says 2, the AudioSpecificConfig (LC, 48 kHz) says mono.
	aac := pt.Chunk("strf", join(pt.U16LE(0x00FF), pt.U16LE(2), zeros(12), pt.U16LE(2), []byte{0x11, 0x88}))
	file := pt.RIFF("AVI ", pt.List("hdrl",
		avih(40000, 10, 640, 480),
		pt.List("strl", strh("auds", "\x00\x00\x00\x00", 1, 1, 0), aac),
		pt.List("strl", strh("auds", "\x00\x00\x00\x00", 1, 1, 0), audsStrf(0x2000, 5)), // AC3 "5" is 5.1
	))
	got, err := readAVI(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if want := []Track{{Codec: "aac", Channels: 1}, {Codec: "ac3", Channels: 6}}; !reflect.DeepEqual(got.Audio, want) {
		t.Fatalf("audio %+v, want %+v", got.Audio, want)
	}
}

func TestAVIAbsurdValuesAreUnknown(t *testing.T) {
	file := pt.RIFF("AVI ", pt.List("hdrl",
		avih(0xFFFFFFFF, 0xFFFFFFFF, 1<<20, 1<<20),
		pt.List("strl", strh("vids", "XVID", 0xFFFFFFFF, 1, 0xFFFFFFFF), vidsStrf(1<<20, 480, "XVID")),
		pt.List("strl", strh("auds", "\x00\x00\x00\x00", 1, 1, 0), audsStrf(0x0055, 1000)),
	))
	got, err := readAVI(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMs != 0 || got.Width != 0 || got.Height != 480 || got.Audio[0].Channels != 0 {
		t.Fatalf("got %+v", got)
	}
}
