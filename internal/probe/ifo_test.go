package probe

import (
	"errors"
	"reflect"
	"testing"
)

type ifoAudio struct {
	format, channels byte
	lang             string
}

// buildIFO returns a VTS IFO with a PGCIT in sector 1 holding one PGC per
// playback time (4 BCD bytes each).
func buildIFO(video uint16, audio []ifoAudio, subs []string, times ...[4]byte) []byte {
	b := make([]byte, 2*dvdSector)
	copy(b, "DVDVIDEO-VTS")
	be.PutUint32(b[ifoPGCITSector:], 1)
	be.PutUint16(b[ifoVideoAttr:], video)
	be.PutUint16(b[ifoAudioCount:], uint16(len(audio)))
	for i, a := range audio {
		at := b[ifoAudioAttr+8*i:]
		at[0] = a.format<<5 | 1<<2 // lang_type 1: language present
		at[1] = a.channels - 1
		copy(at[2:4], a.lang)
	}
	be.PutUint16(b[ifoSubpCount:], uint16(len(subs)))
	for i, l := range subs {
		at := b[ifoSubpAttr+6*i:]
		at[0] = 1
		copy(at[2:4], l)
	}
	pgcit := b[dvdSector:]
	be.PutUint16(pgcit, uint16(len(times)))
	for i, tm := range times {
		start := 8 + 8*len(times) + 16*i
		be.PutUint32(pgcit[8+8*i+4:], uint32(start))
		copy(pgcit[start+4:], tm[:])
	}
	return b
}

func TestIFOPAL(t *testing.T) {
	const mpeg2PAL = 0x4000 | 0x1000
	ifo := buildIFO(mpeg2PAL,
		[]ifoAudio{{0, 6, "es"}, {0, 2, "en"}},
		[]string{"es", "\x00\x00"},
		[4]byte{0x00, 0x03, 0x10, 0x40 | 0x12}, // 3:10 + 12 frames at 25 fps
		[4]byte{0x01, 0x52, 0x07, 0x40},        // 1:52:07: the main feature
	)
	got, err := readIFO(src(ifo))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Container: "dvd", DurationMs: (1*3600 + 52*60 + 7) * 1000, Width: 720, Height: 576, VideoCodec: "mpeg2",
		Audio: []Track{{Codec: "ac3", Lang: "es", Channels: 6}, {Codec: "ac3", Lang: "en", Channels: 2}},
		Subs:  []Track{{Codec: "vobsub", Lang: "es"}, {Codec: "vobsub", Lang: ""}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestIFONTSCHalfD1(t *testing.T) {
	const mpeg2NTSC352x240 = 0x4000 | 3<<2
	got, err := readIFO(src(buildIFO(mpeg2NTSC352x240, []ifoAudio{{6, 6, "fr"}}, nil, [4]byte{0, 0, 0x30, 0xC0 | 0x15})))
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 352 || got.Height != 240 || got.DurationMs != 30_000+15*1000/30 || got.Audio[0].Codec != "dts" {
		t.Fatalf("got %+v", got)
	}
}

func TestIFOInvalid(t *testing.T) {
	good := buildIFO(0x5000, nil, nil, [4]byte{1, 0, 0, 0x40})
	vmg := append([]byte("DVDVIDEO-VMG"), good[12:]...)
	for name, b := range map[string][]byte{
		"menu ifo": vmg,
		"short":    good[:100],
		"no pgcit": good[:dvdSector+4],
	} {
		if _, err := readIFO(src(b)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestIFOInvalidBCDIsIgnored(t *testing.T) {
	ifo := buildIFO(0x5000, nil, nil,
		[4]byte{0xFF, 0xFF, 0xFF, 0xFF},        // nibbles above 9
		[4]byte{0x00, 0x99, 0x99, 0x40},        // 99 minutes, 99 seconds
		[4]byte{0x0A, 0x00, 0x00, 0x40},        // hour nibble 0xA
		[4]byte{0x01, 0x52, 0x07, 0x40},        // the real feature: 1:52:07
		[4]byte{0x00, 0x00, 0x01, 0x40 | 0x25}, // frame 25 at 25 fps
	)
	got, err := readIFO(src(ifo))
	if err != nil {
		t.Fatal(err)
	}
	if want := int64((1*3600 + 52*60 + 7) * 1000); got.DurationMs != want {
		t.Fatalf("DurationMs = %d, want %d", got.DurationMs, want)
	}
}
