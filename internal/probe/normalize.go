package probe

import "strings"

// ResolutionLabel classifies a frame size. Width counts as much as height, so
// a 1920×800 scope picture is still 1080p.
func ResolutionLabel(w, h int) string {
	switch {
	case w <= 0 && h <= 0:
		return ""
	case w >= 3200 || h >= 2000:
		return "2160p"
	case w >= 1800 || h >= 1000:
		return "1080p"
	case w >= 1200 || h >= 700:
		return "720p"
	case h >= 540:
		return "576p"
	case h >= 400:
		return "480p"
	}
	return "SD"
}

// CodecFromName maps the codec names produced by nameparse ("H.264", "XviD"…)
// to the normalized video codec names used by the readers.
func CodecFromName(s string) string {
	switch strings.ToLower(s) {
	case "":
		return ""
	case "h.264":
		return "h264"
	case "h.265":
		return "hevc"
	case "xvid", "divx":
		return "mpeg4"
	case "av1":
		return "av1"
	}
	return strings.ToLower(s)
}

// ffCodecs maps ffprobe codec_name values to normalized names. The native
// readers map their own identifiers onto the same set.
var ffCodecs = map[string]string{
	"h264": "h264", "hevc": "hevc", "av1": "av1", "vp9": "vp9", "vp8": "vp8",
	"mpeg4": "mpeg4", "msmpeg4v1": "mpeg4", "msmpeg4v2": "mpeg4", "msmpeg4v3": "mpeg4",
	"mpeg2video": "mpeg2", "mpeg1video": "mpeg1",
	"wmv1": "wmv", "wmv2": "wmv", "wmv3": "wmv", "vc1": "wmv",
	"rv10": "rv", "rv20": "rv", "rv30": "rv", "rv40": "rv",
	"theora": "theora", "mjpeg": "mjpeg",
	"aac": "aac", "ac3": "ac3", "eac3": "eac3", "dts": "dts", "truehd": "truehd", "flac": "flac",
	"mp3": "mp3", "mp2": "mp2", "opus": "opus", "vorbis": "vorbis",
	"wmav1": "wma", "wmav2": "wma", "wmapro": "wma",
	"subrip": "srt", "srt": "srt", "ass": "ass", "ssa": "ass", "hdmv_pgs_subtitle": "pgs",
	"dvd_subtitle": "vobsub", "dvb_subtitle": "dvbsub", "mov_text": "mov_text", "webvtt": "webvtt",
}

func normFFCodec(s string) string {
	s = strings.ToLower(s)
	if strings.HasPrefix(s, "pcm_") {
		return "pcm"
	}
	if n, ok := ffCodecs[s]; ok {
		return n
	}
	return s
}

// lang3 maps ISO 639-2 codes (bibliographic and terminology forms) to ISO 639-1.
var lang3 = map[string]string{
	"spa": "es", "eng": "en", "fre": "fr", "fra": "fr", "ger": "de", "deu": "de", "ita": "it",
	"por": "pt", "rus": "ru", "jpn": "ja", "chi": "zh", "zho": "zh", "kor": "ko", "dut": "nl",
	"nld": "nl", "swe": "sv", "dan": "da", "nor": "no", "nob": "no", "fin": "fi", "pol": "pl",
	"cze": "cs", "ces": "cs", "hun": "hu", "gre": "el", "ell": "el", "tur": "tr", "ara": "ar",
	"heb": "he", "hin": "hi", "per": "fa", "fas": "fa", "rum": "ro", "ron": "ro", "cat": "ca",
	"glg": "gl", "baq": "eu", "eus": "eu", "ukr": "uk", "bul": "bg", "hrv": "hr", "srp": "sr",
	"slv": "sl", "slo": "sk", "slk": "sk", "tha": "th", "vie": "vi", "ind": "id", "may": "ms",
	"msa": "ms", "ice": "is", "isl": "is", "lat": "la",
}

var lang1 = func() map[string]bool {
	m := map[string]bool{}
	for _, v := range lang3 {
		m[v] = true
	}
	return m
}()

// normLang turns "spa", "es", "es-419" or "en_US" into ISO 639-1; anything
// unknown (including "und") becomes "".
func normLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "-_"); i >= 0 {
		s = s[:i]
	}
	switch len(s) {
	case 2:
		if lang1[s] {
			return s
		}
	case 3:
		return lang3[s]
	}
	return ""
}

// fourccs maps AVI/VfW video FourCCs (lowercased) to normalized names.
var fourccs = map[string]string{
	"xvid": "mpeg4", "divx": "mpeg4", "dx50": "mpeg4", "fmp4": "mpeg4", "mp4v": "mpeg4",
	"3iv2": "mpeg4", "m4s2": "mpeg4", "xvix": "mpeg4", "dxgm": "mpeg4",
	"div3": "mpeg4", "div4": "mpeg4", "mp43": "mpeg4", "mp42": "mpeg4", "mpg4": "mpeg4",
	"h264": "h264", "x264": "h264", "avc1": "h264", "hevc": "hevc", "h265": "hevc", "hvc1": "hevc",
	"mpg2": "mpeg2", "mpg1": "mpeg1", "mjpg": "mjpeg", "wmv1": "wmv", "wmv2": "wmv", "wmv3": "wmv",
	"wvc1": "wmv", "vp80": "vp8", "vp90": "vp9", "av01": "av1",
}

func normFourCC(b []byte) string {
	if len(b) == 4 && b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0 {
		return "rawvideo"
	}
	s := strings.ToLower(strings.TrimRight(string(b), "\x00 "))
	if n, ok := fourccs[s]; ok {
		return n
	}
	return s
}

// wavFormat maps WAVEFORMATEX format tags to normalized audio codec names.
func wavFormat(tag uint16) string {
	switch tag {
	case 0x0001, 0x0003, 0xFFFE:
		return "pcm"
	case 0x0050:
		return "mp2"
	case 0x0055:
		return "mp3"
	case 0x2000:
		return "ac3"
	case 0x2001:
		return "dts"
	case 0x00FF, 0x1600, 0x1601, 0x706D:
		return "aac"
	case 0x0161, 0x0162, 0x0163:
		return "wma"
	case 0x674F, 0x6750, 0x6751, 0x676F, 0x6770, 0x6771:
		return "vorbis"
	case 0xF1AC:
		return "flac"
	}
	return ""
}

// aacChannels reads channelConfiguration from an AudioSpecificConfig.
func aacChannels(asc []byte) int {
	if len(asc) < 2 || asc[0]>>3 == 31 { // escaped object types are rare: skip
		return 0
	}
	var cfg byte
	if freq := (asc[0]&7)<<1 | asc[1]>>7; freq == 15 { // explicit 24-bit frequency
		if len(asc) < 5 {
			return 0
		}
		cfg = (asc[4] >> 3) & 0xF
	} else {
		cfg = (asc[1] >> 3) & 0xF
	}
	switch {
	case cfg >= 1 && cfg <= 6:
		return int(cfg)
	case cfg == 7:
		return 8
	}
	return 0
}

// maxDurationMs bounds durations read from headers; anything longer (a week)
// comes from a corrupt header and is reported as unknown.
const maxDurationMs = 7 * 24 * 3600 * 1000

// saneMs converts a duration in milliseconds, reporting NaN, negative, zero
// and absurdly long values as unknown (0).
func saneMs(ms float64) int64 {
	if ms > 0 && ms < maxDurationMs {
		return int64(ms)
	}
	return 0
}
