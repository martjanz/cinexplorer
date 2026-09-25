// Package mediafile classifies files found in the library by name.
package mediafile

import (
	"path/filepath"
	"strings"
)

type Kind string

const (
	Video    Kind = "video"
	DVD      Kind = "dvd" // VOB/IFO/BUP, normally inside VIDEO_TS
	Subtitle Kind = "subtitle"
	Info     Kind = "info" // .nfo; may carry an IMDb id (used from stage 3)
	Junk     Kind = "junk"
	Other    Kind = "other"
)

var (
	videoExt  = set(".mkv", ".mp4", ".m4v", ".avi", ".mov", ".wmv", ".mpg", ".mpeg", ".ts", ".m2ts", ".webm", ".ogm", ".rmvb", ".rm", ".divx", ".flv", ".3gp")
	dvdExt    = set(".vob", ".ifo", ".bup")
	subExt    = set(".srt", ".sub", ".idx", ".ass", ".ssa", ".vtt", ".smi")
	junkExt   = set(".txt", ".url", ".sfv", ".md5", ".db", ".ini", ".torrent", ".html", ".htm", ".lnk", ".log", ".par2")
	junkNames = set("thumbs.db", "desktop.ini", ".ds_store", "sync.ffs_db")
)

func Classify(name string) Kind {
	lower := strings.ToLower(name)
	if junkNames[lower] || strings.HasPrefix(lower, "._") {
		return Junk
	}
	ext := filepath.Ext(lower)
	switch {
	case videoExt[ext]:
		return Video
	case dvdExt[ext]:
		return DVD
	case subExt[ext]:
		return Subtitle
	case ext == ".nfo":
		return Info
	case junkExt[ext]:
		return Junk
	}
	return Other
}

// Stored reports whether files of this kind are kept in the catalog.
func (k Kind) Stored() bool {
	return k == Video || k == DVD || k == Subtitle || k == Info
}

// Fingerprinted reports whether the scanner computes a content fingerprint.
func (k Kind) Fingerprinted() bool {
	return k == Video || k == DVD
}

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}
