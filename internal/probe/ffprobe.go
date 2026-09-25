package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const defaultFFprobeTimeout = 30 * time.Second

func (p Prober) ffprobe(ctx context.Context, path string) (Info, error) {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultFFprobeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, p.FFprobe,
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", "-i", path).Output()
	if err != nil {
		var ee *exec.ExitError
		switch {
		case ctx.Err() != nil:
			return Info{}, fmt.Errorf("ffprobe gave no answer in %v", timeout)
		case errors.As(err, &ee):
			if len(ee.Stderr) > 0 {
				return Info{}, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(ee.Stderr)))
			}
			return Info{}, err
		}
		// ffprobe could not start at all: a problem of this machine, not of
		// the file, so the file is tried again on a later scan.
		return Info{}, fmt.Errorf("%w: cannot run ffprobe: %v", ErrIO, err)
	}
	return parseFFprobe(out)
}

// ffContainer turns ffprobe's format_name ("matroska,webm",
// "mov,mp4,m4a,3gp,3g2,mj2", "avi") into the names the native readers use,
// so a file read either way gets the same container.
func ffContainer(name string) string {
	switch {
	case strings.HasPrefix(name, "matroska"):
		return "matroska"
	case strings.HasPrefix(name, "mov,mp4"):
		return "mp4"
	}
	first, _, _ := strings.Cut(name, ",")
	return first
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType   string            `json:"codec_type"`
		CodecName   string            `json:"codec_name"`
		Width       int               `json:"width"`
		Height      int               `json:"height"`
		Channels    int               `json:"channels"`
		Tags        map[string]string `json:"tags"`
		Disposition map[string]int    `json:"disposition"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
}

func parseFFprobe(out []byte) (Info, error) {
	var o ffprobeOutput
	if err := json.Unmarshal(out, &o); err != nil {
		return Info{}, fmt.Errorf("ffprobe output: %w", err)
	}
	info := Info{Container: ffContainer(o.Format.FormatName), Prober: "ffprobe"}
	if d, err := strconv.ParseFloat(o.Format.Duration, 64); err == nil {
		info.DurationMs = saneMs(d * 1000)
	}
	for _, st := range o.Streams {
		lang := normLang(st.Tags["language"])
		switch st.CodecType {
		case "video":
			if info.VideoCodec == "" && st.Disposition["attached_pic"] == 0 {
				info.VideoCodec = normFFCodec(st.CodecName)
				info.Width, info.Height = saneDim(st.Width), saneDim(st.Height)
			}
		case "audio":
			ch := st.Channels
			if ch < 0 || ch > maxChannels {
				ch = 0
			}
			info.Audio = append(info.Audio, Track{Codec: normFFCodec(st.CodecName), Lang: lang, Channels: ch})
		case "subtitle":
			info.Subs = append(info.Subs, Track{Codec: normFFCodec(st.CodecName), Lang: lang})
		}
	}
	if info.VideoCodec == "" && len(info.Audio) == 0 {
		return Info{}, fmt.Errorf("ffprobe found no audio or video streams")
	}
	return info, nil
}
