package probe

import (
	"encoding/binary"
	"math"
	"strings"
)

const (
	mkvEBML          = 0x1A45DFA3
	mkvDocType       = 0x4282
	mkvSegment       = 0x18538067
	mkvSeekHead      = 0x114D9B74
	mkvSeek          = 0x4DBB
	mkvSeekID        = 0x53AB
	mkvSeekPosition  = 0x53AC
	mkvInfo          = 0x1549A966
	mkvTimecodeScale = 0x2AD7B1
	mkvDuration      = 0x4489
	mkvTracks        = 0x1654AE6B
	mkvTrackEntry    = 0xAE
	mkvTrackType     = 0x83
	mkvCodecID       = 0x86
	mkvCodecPrivate  = 0x63A2
	mkvLanguage      = 0x22B59C
	mkvLanguageIETF  = 0x22B59D
	mkvVideo         = 0xE0
	mkvPixelWidth    = 0xB0
	mkvPixelHeight   = 0xBA
	mkvAudio         = 0xE1
	mkvChannels      = 0x9F
	mkvCluster       = 0x1F43B675
)

const unknownSize = -1

type ebmlElem struct {
	id      uint32
	dataOff int64
	size    int64 // unknownSize when the element does not declare it
}

func (e ebmlElem) end(parentEnd int64) int64 {
	if e.size == unknownSize {
		return parentEnd
	}
	return e.dataOff + e.size
}

// readElem reads the element header at off; limit is the end of the parent.
func readElem(s *source, off, limit int64) (ebmlElem, error) {
	b, err := s.readUpTo(off, 12)
	if err != nil {
		return ebmlElem{}, err
	}
	if len(b) == 0 || b[0] == 0 {
		return ebmlElem{}, invalid("bad EBML id at %d", off)
	}
	idLen := vintLen(b[0])
	if idLen > 4 || idLen > len(b) {
		return ebmlElem{}, invalid("bad EBML id at %d", off)
	}
	var id uint32
	for _, c := range b[:idLen] {
		id = id<<8 | uint32(c)
	}
	rest := b[idLen:]
	if len(rest) == 0 || rest[0] == 0 {
		return ebmlElem{}, invalid("bad EBML size at %d", off)
	}
	sizeLen := vintLen(rest[0])
	if sizeLen > len(rest) {
		return ebmlElem{}, invalid("bad EBML size at %d", off)
	}
	size := uint64(rest[0]) & (0xFF >> sizeLen)
	allOnes := size == 0xFF>>sizeLen
	for _, c := range rest[1:sizeLen] {
		size = size<<8 | uint64(c)
		allOnes = allOnes && c == 0xFF
	}
	e := ebmlElem{id: id, dataOff: off + int64(idLen+sizeLen), size: int64(size)}
	if allOnes {
		e.size = unknownSize
	} else if size > uint64(limit-e.dataOff) || e.dataOff > limit {
		return ebmlElem{}, invalid("EBML element at %d overflows its parent", off)
	}
	return e, nil
}

// vintLen returns the length of an EBML varint from its first byte.
func vintLen(b byte) int {
	n := 1
	for mask := byte(0x80); mask != 0 && b&mask == 0; mask >>= 1 {
		n++
	}
	return n
}

// children calls fn for each child of the element spanning [off, end).
// fn returns false to stop early.
func children(s *source, off, end int64, fn func(ebmlElem) (bool, error)) error {
	for off < end {
		e, err := readElem(s, off, end)
		if err != nil {
			return err
		}
		more, err := fn(e)
		if err != nil || !more {
			return err
		}
		if e.size == unknownSize {
			return nil // cannot skip it
		}
		off = e.dataOff + e.size
	}
	return nil
}

func (s *source) data(e ebmlElem) ([]byte, error) {
	if e.size == unknownSize || e.size > 1<<20 {
		return nil, invalid("EBML value too large")
	}
	return s.read(e.dataOff, int(e.size))
}

func (s *source) uintElem(e ebmlElem) (uint64, error) {
	b, err := s.data(e)
	if err != nil {
		return 0, err
	}
	if len(b) > 8 {
		return 0, invalid("bad EBML uint")
	}
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v, nil
}

func (s *source) floatElem(e ebmlElem) (float64, error) {
	b, err := s.data(e)
	if err != nil {
		return 0, err
	}
	switch len(b) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b))), nil
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(b)), nil
	}
	return 0, invalid("bad EBML float")
}

func (s *source) stringElem(e ebmlElem) (string, error) {
	b, err := s.data(e)
	return strings.TrimRight(string(b), "\x00"), err
}

func readMKV(s *source) (Info, error) {
	head, err := readElem(s, 0, s.size)
	if err != nil {
		return Info{}, err
	}
	if head.id != mkvEBML {
		return Info{}, invalid("not an EBML file")
	}
	docType := "matroska"
	err = children(s, head.dataOff, head.end(s.size), func(e ebmlElem) (bool, error) {
		var err error
		if e.id == mkvDocType {
			docType, err = s.stringElem(e)
		}
		return true, err
	})
	if err != nil {
		return Info{}, err
	}
	if docType != "matroska" && docType != "webm" {
		return Info{}, invalid("EBML doctype %q", docType)
	}

	seg, err := readElem(s, head.end(s.size), math.MaxInt64) // may claim more than a truncated file holds
	if err != nil {
		return Info{}, err
	}
	if seg.id != mkvSegment {
		return Info{}, invalid("no Matroska segment")
	}
	segEnd := min(seg.end(s.size), s.size)

	info := Info{Container: docType}
	var gotInfo, gotTracks bool
	seeks := map[uint32]int64{}
	parse := func(e ebmlElem) (bool, error) {
		var err error
		switch e.id {
		case mkvInfo:
			err = s.mkvInfo(e, &info)
			gotInfo = true
		case mkvTracks:
			err = s.mkvTracks(e, segEnd, &info)
			gotTracks = true
		case mkvSeekHead:
			err = s.mkvSeekHead(e, seg.dataOff, seeks)
		case mkvCluster:
			return false, nil // media data starts: rely on SeekHead from here
		}
		return !(gotInfo && gotTracks), err
	}
	if err := children(s, seg.dataOff, segEnd, parse); err != nil {
		return Info{}, err
	}
	for _, want := range []struct {
		id  uint32
		got *bool
	}{{mkvInfo, &gotInfo}, {mkvTracks, &gotTracks}} {
		pos, ok := seeks[want.id]
		if *want.got || !ok {
			continue
		}
		e, err := readElem(s, pos, segEnd)
		if err != nil {
			return Info{}, err
		}
		if e.id != want.id {
			return Info{}, invalid("SeekHead points to the wrong element")
		}
		if _, err := parse(e); err != nil {
			return Info{}, err
		}
	}
	if !gotTracks {
		return Info{}, invalid("no Matroska Tracks element")
	}
	return info, nil
}

func (s *source) mkvSeekHead(e ebmlElem, segData int64, seeks map[uint32]int64) error {
	return children(s, e.dataOff, e.dataOff+e.size, func(seek ebmlElem) (bool, error) {
		if seek.id != mkvSeek {
			return true, nil
		}
		var id uint32
		var pos int64 = -1
		err := children(s, seek.dataOff, seek.dataOff+seek.size, func(c ebmlElem) (bool, error) {
			switch c.id {
			case mkvSeekID:
				b, err := s.data(c)
				if err != nil {
					return false, err
				}
				for _, x := range b {
					id = id<<8 | uint32(x)
				}
			case mkvSeekPosition:
				v, err := s.uintElem(c)
				if err != nil {
					return false, err
				}
				pos = segData + int64(v)
			}
			return true, nil
		})
		if err == nil && pos >= 0 {
			seeks[id] = pos
		}
		return true, err
	})
}

func (s *source) mkvInfo(e ebmlElem, info *Info) error {
	scale := uint64(1000000)
	var dur float64
	err := children(s, e.dataOff, e.dataOff+e.size, func(c ebmlElem) (bool, error) {
		var err error
		switch c.id {
		case mkvTimecodeScale:
			scale, err = s.uintElem(c)
		case mkvDuration:
			dur, err = s.floatElem(c)
		}
		return true, err
	})
	if d := dur * float64(scale) / 1e6; d > 0 && d < math.MaxInt64 { // NaN fails both comparisons
		info.DurationMs = int64(d)
	}
	return err
}

func (s *source) mkvTracks(e ebmlElem, segEnd int64, info *Info) error {
	return children(s, e.dataOff, e.end(segEnd), func(t ebmlElem) (bool, error) {
		if t.id != mkvTrackEntry {
			return true, nil
		}
		var typ, w, h uint64
		ch := uint64(1) // Matroska default when Channels is absent
		var codec, lang, ietf string
		var private []byte
		lang = "eng" // Matroska default when Language is absent
		err := children(s, t.dataOff, t.dataOff+t.size, func(c ebmlElem) (bool, error) {
			var err error
			switch c.id {
			case mkvTrackType:
				typ, err = s.uintElem(c)
			case mkvCodecID:
				codec, err = s.stringElem(c)
			case mkvCodecPrivate:
				if c.size <= 64 {
					private, err = s.data(c)
				} else {
					private, err = s.read(c.dataOff, 64)
				}
			case mkvLanguage:
				lang, err = s.stringElem(c)
			case mkvLanguageIETF:
				ietf, err = s.stringElem(c)
			case mkvVideo:
				err = children(s, c.dataOff, c.dataOff+c.size, func(v ebmlElem) (bool, error) {
					var err error
					switch v.id {
					case mkvPixelWidth:
						w, err = s.uintElem(v)
					case mkvPixelHeight:
						h, err = s.uintElem(v)
					}
					return true, err
				})
			case mkvAudio:
				err = children(s, c.dataOff, c.dataOff+c.size, func(a ebmlElem) (bool, error) {
					var err error
					if a.id == mkvChannels {
						ch, err = s.uintElem(a)
					}
					return true, err
				})
			}
			return true, err
		})
		if err != nil {
			return false, err
		}
		// Absurd values (corrupt or hostile headers) count as unknown.
		for _, v := range []*uint64{&w, &h, &ch} {
			if *v > 65535 {
				*v = 0
			}
		}
		if ietf != "" {
			lang = ietf
		}
		switch typ {
		case 1:
			if info.VideoCodec == "" {
				info.VideoCodec = mkvVideoCodec(codec, private)
				info.Width, info.Height = int(w), int(h)
			}
		case 2:
			a := Track{Codec: mkvAudioCodec(codec, private), Lang: normLang(lang), Channels: int(ch)}
			if a.Codec == "aac" && len(private) >= 2 {
				// The AudioSpecificConfig is authoritative; Channels is
				// often missing and then defaults to 1.
				if n := aacChannels(private); n > 0 {
					a.Channels = n
				}
			}
			info.Audio = append(info.Audio, a)
		case 17:
			info.Subs = append(info.Subs, Track{Codec: mkvSubCodec(codec), Lang: normLang(lang)})
		}
		return true, nil
	})
}

func mkvVideoCodec(id string, private []byte) string {
	switch {
	case id == "V_MPEG4/ISO/AVC":
		return "h264"
	case id == "V_MPEGH/ISO/HEVC":
		return "hevc"
	case id == "V_AV1":
		return "av1"
	case id == "V_VP9":
		return "vp9"
	case id == "V_VP8":
		return "vp8"
	case strings.HasPrefix(id, "V_MPEG4/ISO/"), strings.HasPrefix(id, "V_MPEG4/MS/"):
		return "mpeg4"
	case id == "V_MPEG2":
		return "mpeg2"
	case id == "V_MPEG1":
		return "mpeg1"
	case strings.HasPrefix(id, "V_REAL/"):
		return "rv"
	case id == "V_THEORA":
		return "theora"
	case id == "V_MJPEG":
		return "mjpeg"
	case id == "V_MS/VFW/FOURCC" && len(private) >= 20:
		return normFourCC(private[16:20]) // BITMAPINFOHEADER.biCompression
	}
	return strings.ToLower(strings.TrimPrefix(id, "V_"))
}

func mkvAudioCodec(id string, private []byte) string {
	switch {
	case strings.HasPrefix(id, "A_AAC"):
		return "aac"
	case id == "A_AC3":
		return "ac3"
	case id == "A_EAC3":
		return "eac3"
	case strings.HasPrefix(id, "A_DTS"):
		return "dts"
	case id == "A_TRUEHD":
		return "truehd"
	case id == "A_FLAC":
		return "flac"
	case id == "A_MPEG/L3":
		return "mp3"
	case id == "A_MPEG/L2":
		return "mp2"
	case id == "A_OPUS":
		return "opus"
	case id == "A_VORBIS":
		return "vorbis"
	case strings.HasPrefix(id, "A_PCM/"):
		return "pcm"
	case id == "A_MS/ACM" && len(private) >= 2:
		return wavFormat(binary.LittleEndian.Uint16(private))
	}
	return strings.ToLower(strings.TrimPrefix(id, "A_"))
}

func mkvSubCodec(id string) string {
	switch id {
	case "S_TEXT/UTF8", "S_TEXT/ASCII":
		return "srt"
	case "S_TEXT/SSA", "S_TEXT/ASS", "S_SSA", "S_ASS":
		return "ass"
	case "S_HDMV/PGS":
		return "pgs"
	case "S_VOBSUB":
		return "vobsub"
	case "S_DVBSUB":
		return "dvbsub"
	case "S_TEXT/WEBVTT":
		return "webvtt"
	}
	return strings.ToLower(strings.TrimPrefix(id, "S_"))
}
