package probe

import (
	"encoding/binary"
	"errors"
	"math"
	"strings"
)

type mp4Box struct {
	typ     string
	dataOff int64
	end     int64
}

// readBox reads the box header at off; limit is the end of the parent.
func readBox(s *source, off, limit int64) (mp4Box, error) {
	h, err := s.read(off, 8)
	if err != nil {
		return mp4Box{}, err
	}
	size := int64(binary.BigEndian.Uint32(h))
	b := mp4Box{typ: string(h[4:8]), dataOff: off + 8}
	switch size {
	case 0: // extends to the end of the parent
		b.end = limit
	case 1:
		ext, err := s.read(off+8, 8)
		if err != nil {
			return mp4Box{}, err
		}
		b.dataOff = off + 16
		b.end = off + int64(binary.BigEndian.Uint64(ext))
	default:
		b.end = off + size
	}
	if b.end < b.dataOff || b.end > limit {
		return mp4Box{}, invalid("MP4 box %q at %d overflows its parent", b.typ, off)
	}
	return b, nil
}

// boxes calls fn for each box in [off, end). fn returns false to stop early.
func boxes(s *source, off, end int64, fn func(mp4Box) (bool, error)) error {
	for off+8 <= end {
		b, err := readBox(s, off, end)
		if err != nil {
			return err
		}
		more, err := fn(b)
		if err != nil || !more {
			return err
		}
		off = b.end
	}
	return nil
}

func (s *source) boxData(b mp4Box, max int) ([]byte, error) {
	n := b.end - b.dataOff
	if n > int64(max) {
		n = int64(max)
	}
	return s.read(b.dataOff, int(n))
}

type mp4Track struct {
	id      uint32
	handler string
	lang    string
	codec   string
	w, h    int
	ch      int
	chapter []uint32 // track ids referenced as chapter lists
}

func readMP4(s *source) (Info, error) {
	first, err := readBox(s, 0, s.size)
	if err != nil {
		return Info{}, err
	}
	if !isFourCC(first.typ) {
		return Info{}, invalid("not an MP4 file")
	}
	var moov *mp4Box
	err = boxes(s, 0, s.size, func(b mp4Box) (bool, error) {
		if b.typ == "moov" {
			moov = &b
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return Info{}, err
	}
	if moov == nil {
		return Info{}, invalid("MP4 without moov")
	}

	info := Info{Container: "mp4"}
	var tracks []mp4Track
	err = boxes(s, moov.dataOff, moov.end, func(b mp4Box) (bool, error) {
		switch b.typ {
		case "mvhd":
			d, err := s.boxData(b, 32)
			if err != nil {
				return false, err
			}
			if len(d) >= 32 && d[0] == 1 {
				info.DurationMs = mp4Duration(binary.BigEndian.Uint64(d[24:]), uint64(binary.BigEndian.Uint32(d[20:])), math.MaxUint64)
			} else if len(d) >= 20 {
				info.DurationMs = mp4Duration(uint64(binary.BigEndian.Uint32(d[16:])), uint64(binary.BigEndian.Uint32(d[12:])), math.MaxUint32)
			}
		case "trak":
			t, err := s.mp4Trak(b)
			if err != nil {
				return false, err
			}
			tracks = append(tracks, t)
		}
		return true, nil
	})
	if err != nil {
		return Info{}, err
	}

	chapters := map[uint32]bool{}
	for _, t := range tracks {
		for _, id := range t.chapter {
			chapters[id] = true
		}
	}
	for _, t := range tracks {
		switch t.handler {
		case "vide":
			if info.VideoCodec == "" {
				info.VideoCodec, info.Width, info.Height = t.codec, t.w, t.h
			}
		case "soun":
			info.Audio = append(info.Audio, Track{Codec: t.codec, Lang: t.lang, Channels: t.ch})
		case "sbtl", "text", "subt", "clcp", "subp":
			if !chapters[t.id] {
				info.Subs = append(info.Subs, Track{Codec: t.codec, Lang: t.lang})
			}
		}
	}
	return info, nil
}

func isFourCC(s string) bool {
	for _, c := range []byte(s) {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}

// mp4Duration converts a movie duration to milliseconds without overflowing.
// A duration of all ones means "unknown" (fragmented or live files).
func mp4Duration(dur, scale, unknown uint64) int64 {
	if scale == 0 || dur == unknown || dur/scale > math.MaxInt64/1000 {
		return 0
	}
	return int64(dur/scale*1000 + dur%scale*1000/scale)
}

// childBox returns the first child of parent with the given type, or nil.
func (s *source) childBox(parent mp4Box, typ string) (*mp4Box, error) {
	var found *mp4Box
	err := boxes(s, parent.dataOff, parent.end, func(b mp4Box) (bool, error) {
		if b.typ == typ {
			found = &b
			return false, nil
		}
		return true, nil
	})
	return found, err
}

// mp4Trak reads a track. It only descends along the paths that hold what we
// need (trak/tkhd, trak/tref/chap, trak/mdia/{hdlr,mdhd}, trak/mdia/minf/stbl/
// stsd), so nesting in a crafted file cannot drive recursion. Taking hdlr only
// from mdia also skips the data-reference hdlr QuickTime puts inside minf.
func (s *source) mp4Trak(trak mp4Box) (mp4Track, error) {
	var t mp4Track
	var stsd *mp4Box
	err := boxes(s, trak.dataOff, trak.end, func(b mp4Box) (bool, error) {
		switch b.typ {
		case "tkhd":
			d, err := s.boxData(b, 24)
			if err != nil {
				return false, err
			}
			if len(d) >= 24 && d[0] == 1 {
				t.id = binary.BigEndian.Uint32(d[20:])
			} else if len(d) >= 16 {
				t.id = binary.BigEndian.Uint32(d[12:])
			}
		case "tref":
			chap, err := s.childBox(b, "chap")
			if err != nil || chap == nil {
				return err == nil, err
			}
			d, err := s.boxData(*chap, 256)
			if err != nil {
				return false, err
			}
			for i := 0; i+4 <= len(d); i += 4 {
				t.chapter = append(t.chapter, binary.BigEndian.Uint32(d[i:]))
			}
		case "mdia":
			return true, boxes(s, b.dataOff, b.end, func(c mp4Box) (bool, error) {
				switch c.typ {
				case "hdlr":
					d, err := s.boxData(c, 12)
					if err != nil {
						return false, err
					}
					if len(d) >= 12 {
						t.handler = string(d[8:12])
					}
				case "mdhd":
					d, err := s.boxData(c, 34)
					if err != nil {
						return false, err
					}
					at := 20
					if len(d) > 0 && d[0] == 1 {
						at = 32
					}
					if len(d) >= at+2 {
						t.lang = mp4Lang(binary.BigEndian.Uint16(d[at:]))
					}
				case "minf":
					stbl, err := s.childBox(c, "stbl")
					if err != nil || stbl == nil {
						return err == nil, err
					}
					stsd, err = s.childBox(*stbl, "stsd")
					return err == nil, err
				}
				return true, nil
			})
		}
		return true, nil
	})
	if err != nil {
		return t, err
	}
	if stsd != nil {
		if err := s.mp4SampleEntry(*stsd, &t); err != nil {
			return t, err
		}
	}
	return t, nil
}

// mp4Lang unpacks an ISO-639-2/T code stored as three 5-bit letters.
// Values below 0x400 are old QuickTime language numbers and are ignored.
func mp4Lang(v uint16) string {
	if v < 0x400 {
		return ""
	}
	b := []byte{byte(v>>10&0x1F) + 0x60, byte(v>>5&0x1F) + 0x60, byte(v&0x1F) + 0x60}
	return normLang(string(b))
}

// maxChannels caps channel counts read from headers; larger values come from
// corrupt files and are reported as unknown.
const maxChannels = 64

// mp4SampleEntry reads the first sample description of a track.
func (s *source) mp4SampleEntry(stsd mp4Box, t *mp4Track) error {
	if stsd.end-stsd.dataOff < 16 {
		return nil
	}
	entry, err := readBox(s, stsd.dataOff+8, stsd.end)
	if err != nil {
		return err
	}
	d, err := s.boxData(entry, 64)
	if err != nil {
		return err
	}
	switch t.handler {
	case "vide":
		if len(d) >= 28 {
			t.w, t.h = int(binary.BigEndian.Uint16(d[24:])), int(binary.BigEndian.Uint16(d[26:]))
		}
		t.codec = mp4VideoCodec(entry.typ)
		if entry.typ == "mp4v" {
			es, ok, err := s.mp4ESDS(entry, 78)
			if err != nil {
				return err
			}
			if ok {
				t.codec = mpeg4VideoObject(es.objectType)
			}
		}
	case "soun":
		version := uint16(0)
		if len(d) >= 10 {
			version = binary.BigEndian.Uint16(d[8:])
		}
		switch {
		case version == 2 && len(d) >= 44:
			// QuickTime SoundDescription v2: @16 is a constant 3, the
			// real count is a u32 at @40.
			t.ch = int(min(binary.BigEndian.Uint32(d[40:]), maxChannels+1))
		case len(d) >= 18:
			t.ch = int(binary.BigEndian.Uint16(d[16:]))
		}
		if t.ch > maxChannels {
			t.ch = 0
		}
		t.codec = mp4AudioCodec(entry.typ)
		start := map[uint16]int64{0: 28, 1: 44, 2: 64}[version]
		switch entry.typ {
		case "ac-3":
			ch, err := s.mp4AC3Channels(entry, start)
			if err != nil {
				return err
			}
			if ch > 0 {
				t.ch = ch // stsd often says 2 whatever the stream has
			}
		case "mp4a":
			es, ok, err := s.mp4ESDS(entry, start)
			if err != nil {
				return err
			}
			if ok {
				t.codec = mpeg4AudioObject(es.objectType)
				if t.codec == "aac" && es.channels > 0 {
					t.ch = es.channels // stsd often says 2 whatever the stream has
				}
			}
		}
	default:
		t.codec = mp4SubCodec(entry.typ)
	}
	return nil
}

// optional drops format errors in boxes that only refine what we already
// know, but keeps read errors: those must reach the caller to be retried.
func optional(err error) error {
	if errors.Is(err, ErrInvalid) {
		return nil
	}
	return err
}

// sampleEntryChildren is the span holding a sample entry's child boxes.
func sampleEntryChildren(entry mp4Box, childOff int64) mp4Box {
	return mp4Box{typ: entry.typ, dataOff: entry.dataOff + childOff, end: entry.end}
}

// mp4AC3Channels reads the channel layout from the dac3 box of an "ac-3"
// sample entry: acmod gives the full-range channels, lfeon adds the LFE.
func (s *source) mp4AC3Channels(entry mp4Box, childOff int64) (int, error) {
	if childOff == 0 {
		return 0, nil
	}
	dac3, err := s.childBox(sampleEntryChildren(entry, childOff), "dac3")
	if err != nil || dac3 == nil {
		return 0, optional(err)
	}
	d, err := s.boxData(*dac3, 3)
	if err != nil || len(d) < 3 {
		return 0, optional(err)
	}
	v := uint32(d[0])<<16 | uint32(d[1])<<8 | uint32(d[2])
	return []int{2, 1, 2, 3, 3, 4, 4, 5}[v>>11&7] + int(v>>10&1), nil
}

// esdsInfo is what we use from an MPEG-4 elementary stream descriptor.
type esdsInfo struct {
	objectType byte
	channels   int // from the AAC AudioSpecificConfig; 0 when absent
}

// mp4ESDS finds the esds box among the children of a sample entry, or inside
// its QuickTime "wave" child, and parses it.
func (s *source) mp4ESDS(entry mp4Box, childOff int64) (esdsInfo, bool, error) {
	if childOff == 0 {
		return esdsInfo{}, false, nil
	}
	kids := sampleEntryChildren(entry, childOff)
	esds, err := s.childBox(kids, "esds")
	if err == nil && esds == nil {
		var wave *mp4Box
		if wave, err = s.childBox(kids, "wave"); err == nil && wave != nil {
			esds, err = s.childBox(*wave, "esds")
		}
	}
	if err != nil || esds == nil {
		return esdsInfo{}, false, optional(err)
	}
	d, err := s.boxData(*esds, 128)
	if err != nil {
		return esdsInfo{}, false, optional(err)
	}
	es, ok := parseESDS(d)
	return es, ok, nil
}

// parseESDS parses an esds payload: version/flags, then an ES_Descriptor
// holding a DecoderConfigDescriptor, which may hold a DecoderSpecificInfo.
func parseESDS(d []byte) (esdsInfo, bool) {
	p := 4
	descr := func(want byte) (int, bool) { // returns the descriptor length
		if p >= len(d) || d[p] != want {
			return 0, false
		}
		p++
		n := 0
		for i := 0; i < 4 && p < len(d); i++ { // expandable length
			c := d[p]
			p++
			n = n<<7 | int(c&0x7F)
			if c&0x80 == 0 {
				break
			}
		}
		return n, p < len(d)
	}
	if _, ok := descr(0x03); !ok || p+3 > len(d) {
		return esdsInfo{}, false
	}
	flags := d[p+2]
	p += 3
	if flags&0x80 != 0 {
		p += 2
	}
	if flags&0x40 != 0 && p < len(d) {
		p += 1 + int(d[p])
	}
	if flags&0x20 != 0 {
		p += 2
	}
	if _, ok := descr(0x04); !ok {
		return esdsInfo{}, false
	}
	es := esdsInfo{objectType: d[p]}
	p += 13 // objectType, streamType, bufferSize, maxBitrate, avgBitrate
	if n, ok := descr(0x05); ok && n >= 2 && p+2 <= len(d) {
		es.channels = aacChannels(d[p:min(p+n, len(d))])
	}
	return es, true
}

func mpeg4AudioObject(ot byte) string {
	switch ot {
	case 0x40, 0x66, 0x67, 0x68:
		return "aac"
	case 0x69, 0x6B:
		return "mp3"
	case 0xA5:
		return "ac3"
	case 0xA6:
		return "eac3"
	case 0xA9:
		return "dts"
	case 0xDD:
		return "vorbis"
	}
	return "aac"
}

func mpeg4VideoObject(ot byte) string {
	switch {
	case ot == 0x20:
		return "mpeg4"
	case ot >= 0x60 && ot <= 0x65:
		return "mpeg2"
	case ot == 0x6A:
		return "mpeg1"
	case ot == 0x6C:
		return "mjpeg"
	case ot == 0x21:
		return "h264"
	}
	return "mpeg4"
}

func mp4VideoCodec(f string) string {
	switch f {
	case "avc1", "avc3":
		return "h264"
	case "hvc1", "hev1", "dvh1", "dvhe":
		return "hevc"
	case "av01":
		return "av1"
	case "vp09":
		return "vp9"
	case "vp08":
		return "vp8"
	case "mp4v":
		return "mpeg4"
	case "jpeg", "mjpa", "mjpb", "mjp2":
		return "mjpeg"
	}
	return normFourCC([]byte(f))
}

func mp4AudioCodec(f string) string {
	switch f {
	case "mp4a":
		return "aac"
	case "ac-3":
		return "ac3"
	case "ec-3":
		return "eac3"
	case "dtsc", "dtsh", "dtsl", "dtse":
		return "dts"
	case "mlpa":
		return "truehd"
	case "fLaC":
		return "flac"
	case "Opus":
		return "opus"
	case ".mp3", "ms\x00U":
		return "mp3"
	case "lpcm", "sowt", "twos", "in24", "in32", "fl32", "fl64", "raw ":
		return "pcm"
	}
	return strings.ToLower(strings.TrimSpace(f))
}

func mp4SubCodec(f string) string {
	switch f {
	case "tx3g", "text":
		return "mov_text"
	case "wvtt":
		return "webvtt"
	case "stpp":
		return "ttml"
	case "c608":
		return "eia_608"
	case "mp4s":
		return "vobsub"
	}
	return strings.ToLower(strings.TrimSpace(f))
}
