package probe

import "encoding/binary"

var le = binary.LittleEndian

// readAVI reads the RIFF "hdrl" list at the start of an AVI file.
func readAVI(s *source) (Info, error) {
	h, err := s.read(0, 12)
	if err != nil {
		return Info{}, err
	}
	if string(h[0:4]) != "RIFF" || string(h[8:12]) != "AVI " {
		return Info{}, invalid("not an AVI file")
	}
	off := int64(12)
	for i := 0; i < 8 && off+12 <= s.size; i++ {
		c, err := s.read(off, 12)
		if err != nil {
			return Info{}, err
		}
		size := int64(le.Uint32(c[4:8]))
		if string(c[0:4]) == "LIST" && string(c[8:12]) == "hdrl" {
			if size < 4 || size > 1<<20 {
				return Info{}, invalid("AVI hdrl of %d bytes", size)
			}
			body, err := s.read(off+12, int(size-4))
			if err != nil {
				return Info{}, err
			}
			return parseHdrl(body)
		}
		off += 8 + size + size&1
	}
	return Info{}, invalid("AVI without hdrl")
}

// riffChunks calls fn for each chunk in b. For LIST chunks, data starts with
// the list type.
func riffChunks(b []byte, fn func(id string, data []byte)) {
	for len(b) >= 8 {
		id, size := string(b[0:4]), int(le.Uint32(b[4:8]))
		if size > len(b)-8 {
			size = len(b) - 8
		}
		fn(id, b[8:8+size])
		b = b[min(8+size+size&1, len(b)):]
	}
}

func parseHdrl(hdrl []byte) (Info, error) {
	info := Info{Container: "avi"}
	var usPerFrame, totalFrames, dmlFrames uint32
	var avihW, avihH int
	var gotAvih bool
	var videoMs int64

	riffChunks(hdrl, func(id string, d []byte) {
		switch {
		case id == "avih" && len(d) >= 40:
			gotAvih = true
			usPerFrame, totalFrames = le.Uint32(d[0:]), le.Uint32(d[16:])
			avihW, avihH = int(le.Uint32(d[32:])), int(le.Uint32(d[36:]))
		case id == "LIST" && len(d) >= 4 && string(d[:4]) == "odml":
			riffChunks(d[4:], func(id string, d []byte) {
				if id == "dmlh" && len(d) >= 4 {
					dmlFrames = le.Uint32(d)
				}
			})
		case id == "LIST" && len(d) >= 4 && string(d[:4]) == "strl":
			var strh, strf []byte
			riffChunks(d[4:], func(id string, d []byte) {
				switch id {
				case "strh":
					strh = d
				case "strf":
					strf = d
				}
			})
			if len(strh) < 36 {
				return
			}
			switch string(strh[0:4]) {
			case "vids":
				if info.VideoCodec != "" || len(strf) < 20 {
					return
				}
				info.VideoCodec = normFourCC(strf[16:20])
				info.Width = saneDim(int(int32(le.Uint32(strf[4:]))))
				info.Height = saneDim(abs(int(int32(le.Uint32(strf[8:])))))
				scale, rate, length := uint64(le.Uint32(strh[20:])), uint64(le.Uint32(strh[24:])), uint64(le.Uint32(strh[32:]))
				if rate > 0 {
					videoMs = saneMs(float64(length) * float64(scale) * 1000 / float64(rate))
				}
			case "auds":
				info.Audio = append(info.Audio, aviAudio(strf))
			}
		}
	})
	if !gotAvih {
		return Info{}, invalid("AVI without avih")
	}
	if info.Width == 0 && info.Height == 0 {
		info.Width, info.Height = saneDim(avihW), saneDim(avihH)
	}
	switch {
	case videoMs > 0:
		info.DurationMs = videoMs
	case dmlFrames > 0:
		info.DurationMs = saneMs(float64(dmlFrames) * float64(usPerFrame) / 1000)
	default:
		info.DurationMs = saneMs(float64(totalFrames) * float64(usPerFrame) / 1000)
	}
	return info, nil
}

// aviAudio reads a WAVEFORMATEX. Its channel count is what the muxer wrote,
// which for AAC is refined from the AudioSpecificConfig in the extra bytes
// and for AC3/DTS commonly says 5 for a 5.1 stream (the LFE is left out).
func aviAudio(strf []byte) Track {
	var t Track
	if len(strf) < 4 {
		return t
	}
	t.Codec = wavFormat(le.Uint16(strf[0:]))
	t.Channels = int(le.Uint16(strf[2:]))
	switch t.Codec {
	case "aac":
		if len(strf) >= 20 {
			extra := strf[18:min(18+int(le.Uint16(strf[16:])), len(strf))]
			if ch := aacChannels(extra); ch > 0 {
				t.Channels = ch
			}
		}
	case "ac3", "dts":
		if t.Channels == 5 {
			t.Channels = 6
		}
	}
	if t.Channels > maxChannels {
		t.Channels = 0
	}
	return t
}

// saneDim reports frame sizes outside 1..65535 as unknown.
func saneDim(n int) int {
	if n < 0 || n > 65535 {
		return 0
	}
	return n
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
