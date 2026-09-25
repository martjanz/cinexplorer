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
		codec := map[byte]string{0: "ac3", 2: "mp2", 3: "mp2", 4: "pcm", 6: "dts"}[a[0]>>5]
		info.Audio = append(info.Audio, Track{Codec: codec, Lang: ifoLang(a[0]>>2&3, a[2:4]), Channels: int(a[1]&7) + 1})
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
// two top bits of the last byte give the frame rate).
func dvdTime(t []byte) int64 {
	bcd := func(b byte) int64 { return int64(b>>4)*10 + int64(b&0x0F) }
	ms := bcd(t[0])*3_600_000 + bcd(t[1])*60_000 + bcd(t[2])*1000
	fps := map[byte]int64{1: 25, 3: 30}[t[3]>>6]
	if fps > 0 {
		ms += bcd(t[3]&0x3F) * 1000 / fps
	}
	return ms
}
