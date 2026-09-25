package probe

import (
	"encoding/binary"
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
			var scale, dur uint64
			if len(d) >= 32 && d[0] == 1 {
				scale, dur = uint64(binary.BigEndian.Uint32(d[20:])), binary.BigEndian.Uint64(d[24:])
			} else if len(d) >= 20 {
				scale, dur = uint64(binary.BigEndian.Uint32(d[12:])), uint64(binary.BigEndian.Uint32(d[16:]))
			}
			if scale > 0 {
				info.DurationMs = int64(dur * 1000 / scale)
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

func (s *source) mp4Trak(trak mp4Box) (mp4Track, error) {
	var t mp4Track
	var stsd *mp4Box
	var walk func(off, end int64) error
	walk = func(off, end int64) error {
		return boxes(s, off, end, func(b mp4Box) (bool, error) {
			switch b.typ {
			case "mdia", "minf", "stbl", "tref":
				return true, walk(b.dataOff, b.end)
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
			case "chap":
				d, err := s.boxData(b, 256)
				if err != nil {
					return false, err
				}
				for i := 0; i+4 <= len(d); i += 4 {
					t.chapter = append(t.chapter, binary.BigEndian.Uint32(d[i:]))
				}
			case "hdlr":
				d, err := s.boxData(b, 12)
				if err != nil {
					return false, err
				}
				// QuickTime files carry a second, data-reference hdlr inside
				// minf; the media handler is the one in mdia, seen first.
				if len(d) >= 12 && t.handler == "" {
					t.handler = string(d[8:12])
				}
			case "mdhd":
				d, err := s.boxData(b, 34)
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
			case "stsd":
				stsd = &b
			}
			return true, nil
		})
	}
	if err := walk(trak.dataOff, trak.end); err != nil {
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
			if es, ok := s.mp4ESDS(entry, 78); ok {
				t.codec = mpeg4VideoObject(es.objectType)
			}
		}
	case "soun":
		if len(d) >= 18 {
			t.ch = int(binary.BigEndian.Uint16(d[16:]))
		}
		t.codec = mp4AudioCodec(entry.typ)
		start := int64(0)
		if len(d) >= 10 {
			start = map[uint16]int64{0: 28, 1: 44, 2: 64}[binary.BigEndian.Uint16(d[8:])]
		}
		if entry.typ == "ac-3" {
			if ch := s.mp4AC3Channels(entry, start); ch > 0 {
				t.ch = ch // stsd often says 2 whatever the stream has
			}
		}
		if entry.typ == "mp4a" {
			if es, ok := s.mp4ESDS(entry, start); ok {
				t.codec = mpeg4AudioObject(es.objectType)
				if es.channels > 0 {
					t.ch = es.channels // stsd often says 2 whatever the stream has
				}
			}
		}
	default:
		t.codec = mp4SubCodec(entry.typ)
	}
	return nil
}

// mp4AC3Channels reads the channel layout from the dac3 box of an "ac-3"
// sample entry: acmod gives the full-range channels, lfeon adds the LFE.
func (s *source) mp4AC3Channels(entry mp4Box, childOff int64) int {
	if childOff == 0 {
		return 0
	}
	ch := 0
	boxes(s, entry.dataOff+childOff, entry.end, func(b mp4Box) (bool, error) {
		if b.typ != "dac3" {
			return true, nil
		}
		if d, err := s.boxData(b, 3); err == nil && len(d) == 3 {
			v := uint32(d[0])<<16 | uint32(d[1])<<8 | uint32(d[2])
			ch = []int{2, 1, 2, 3, 3, 4, 4, 5}[v>>11&7] + int(v>>10&1)
		}
		return false, nil
	})
	return ch
}

// esdsInfo is what we use from an MPEG-4 elementary stream descriptor.
type esdsInfo struct {
	objectType byte
	channels   int // from the AAC AudioSpecificConfig; 0 when absent
}

// mp4ESDS finds the esds box among the children of a sample entry (directly
// or inside a QuickTime "wave" box) and parses it.
func (s *source) mp4ESDS(entry mp4Box, childOff int64) (esdsInfo, bool) {
	if childOff == 0 {
		return esdsInfo{}, false
	}
	var es esdsInfo
	var found bool
	var walk func(off, end int64)
	walk = func(off, end int64) {
		boxes(s, off, end, func(b mp4Box) (bool, error) {
			switch b.typ {
			case "wave":
				walk(b.dataOff, b.end)
			case "esds":
				if d, err := s.boxData(b, 128); err == nil {
					es, found = parseESDS(d)
				}
			}
			return !found, nil
		})
	}
	walk(entry.dataOff+childOff, entry.end)
	return es, found
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
