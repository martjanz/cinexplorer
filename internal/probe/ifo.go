package probe

import "encoding/binary"

var be = binary.BigEndian

// Offsets in a DVD title set information file (VTS_xx_0.IFO).
const (
	ifoPGCITSector = 0x0CC
	ifoVideoAttr   = 0x200
	ifoAudioCount  = 0x202
	ifoAudioAttr   = 0x204 // 8 entries of 8 bytes
	ifoSubpCount   = 0x254
	ifoSubpAttr    = 0x256 // 32 entries of 6 bytes
	ifoHeaderLen   = ifoSubpAttr + 32*6
	dvdSector      = 2048
)

func readIFO(s *source) (Info, error) {
	h, err := s.read(0, ifoHeaderLen)
	if err != nil {
		return Info{}, err
	}
	if string(h[0:12]) != "DVDVIDEO-VTS" {
		return Info{}, invalid("not a DVD title set IFO")
	}
	info := Info{Container: "dvd", VideoCodec: "mpeg2"}
	video := be.Uint16(h[ifoVideoAttr:])
	if video>>14 == 0 {
		info.VideoCodec = "mpeg1"
	}
	pal := video>>12&3 == 1
	info.Width = [4]int{720, 704, 352, 352}[video>>2&3]
	info.Height = 480
	if pal {
		info.Height = 576
	}
	if video>>2&3 == 3 {
		info.Height /= 2
	}

	for i := range min(int(be.Uint16(h[ifoAudioCount:])), 8) {
		a := h[ifoAudioAttr+8*i:]
		info.Audio = append(info.Audio, Track{Codec: ifoAudioCodecs[a[0]>>5], Lang: ifoLang(a[0]>>2&3, a[2:4]), Channels: int(a[1]&7) + 1})
	}
	for i := range min(int(be.Uint16(h[ifoSubpCount:])), 32) {
		sp := h[ifoSubpAttr+6*i:]
		info.Subs = append(info.Subs, Track{Codec: "vobsub", Lang: ifoLang(sp[0]&3, sp[2:4])})
	}

	dur, err := s.ifoLongestPGC(int64(be.Uint32(h[ifoPGCITSector:])) * dvdSector)
	if err != nil {
		return Info{}, err
	}
	info.DurationMs = dur
	return info, nil
}

// ifoAudioCodecs maps the audio coding mode (3 bits) to codec names.
var ifoAudioCodecs = [8]string{0: "ac3", 2: "mp2", 3: "mp2", 4: "pcm", 6: "dts"}

func ifoLang(langType byte, code []byte) string {
	if langType != 1 {
		return ""
	}
	return normLang(string(code))
}

// ifoLongestPGC returns the playback time of the longest program chain in the
// VTS_PGCIT table at off: the main feature of the title set.
func (s *source) ifoLongestPGC(off int64) (int64, error) {
	if off == 0 {
		return 0, nil
	}
	hdr, err := s.read(off, 8)
	if err != nil {
		return 0, err
	}
	n := min(int(be.Uint16(hdr)), 256)
	srps, err := s.read(off+8, 8*n)
	if err != nil {
		return 0, err
	}
	var best int64
	for i := range n {
		pgc, err := s.read(off+int64(be.Uint32(srps[8*i+4:])), 8)
		if err != nil {
			return 0, err
		}
		best = max(best, dvdTime(pgc[4:8]))
	}
	return best, nil
}

// dvdTime decodes a BCD playback time: hours, minutes, seconds, frames (the
// two top bits of the last byte give the frame rate). Invalid times return 0
// so the PGC is ignored: some DVDs carry bogus PGCs as copy protection, and
// one of them must not pass for the longest title.
func dvdTime(t []byte) int64 {
	var v [4]int64
	for i, b := range t[:4] {
		if i == 3 {
			b &= 0x3F
		}
		if b>>4 > 9 || b&0x0F > 9 {
			return 0
		}
		v[i] = int64(b>>4)*10 + int64(b&0x0F)
	}
	var fps int64
	switch t[3] >> 6 {
	case 1:
		fps = 25
	case 3:
		fps = 30
	}
	if v[1] >= 60 || v[2] >= 60 || (fps > 0 && v[3] >= fps) {
		return 0
	}
	ms := v[0]*3_600_000 + v[1]*60_000 + v[2]*1000
	if fps > 0 {
		ms += v[3] * 1000 / fps
	}
	return ms
}
