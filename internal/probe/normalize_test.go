package probe

import "testing"

func TestResolutionLabel(t *testing.T) {
	for _, c := range []struct {
		w, h int
		want string
	}{
		{3840, 2160, "2160p"}, {3840, 1600, "2160p"}, {1920, 1080, "1080p"}, {1920, 800, "1080p"},
		{1440, 1080, "1080p"}, {1280, 720, "720p"}, {1280, 544, "720p"}, {720, 576, "576p"},
		{1024, 576, "576p"}, {720, 480, "480p"}, {640, 480, "480p"}, {640, 272, "SD"}, {352, 240, "SD"},
		{0, 0, ""},
	} {
		if got := ResolutionLabel(c.w, c.h); got != c.want {
			t.Errorf("ResolutionLabel(%d, %d) = %q, want %q", c.w, c.h, got, c.want)
		}
	}
}

func TestCodecFromName(t *testing.T) {
	for in, want := range map[string]string{"H.264": "h264", "H.265": "hevc", "XviD": "mpeg4", "DivX": "mpeg4", "AV1": "av1", "": ""} {
		if got := CodecFromName(in); got != want {
			t.Errorf("CodecFromName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormLang(t *testing.T) {
	for in, want := range map[string]string{
		"spa": "es", "es": "es", "es-419": "es", "en_US": "en", "ger": "de", "deu": "de", "ENG": "en",
		"und": "", "": "", "xx": "", "tlh": "",
	} {
		if got := normLang(in); got != want {
			t.Errorf("normLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormFFCodec(t *testing.T) {
	for in, want := range map[string]string{
		"h264": "h264", "mpeg2video": "mpeg2", "msmpeg4v3": "mpeg4", "rv40": "rv", "pcm_s16le": "pcm",
		"subrip": "srt", "hdmv_pgs_subtitle": "pgs", "dvd_subtitle": "vobsub", "wmav2": "wma", "cook": "cook",
	} {
		if got := normFFCodec(in); got != want {
			t.Errorf("normFFCodec(%q) = %q, want %q", in, got, want)
		}
	}
}
