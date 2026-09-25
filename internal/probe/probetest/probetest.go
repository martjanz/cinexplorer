// Package probetest builds minimal media files in memory for tests: EBML
// (Matroska), ISO BMFF boxes (MP4), RIFF chunks (AVI) and DVD IFO headers.
// It is only imported from tests.
package probetest

import (
	"bytes"
	"encoding/binary"
	"math"
)

func U16BE(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }
func U32BE(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
func U64BE(v uint64) []byte { return binary.BigEndian.AppendUint64(nil, v) }
func U16LE(v uint16) []byte { return binary.LittleEndian.AppendUint16(nil, v) }
func U32LE(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

// --- EBML (Matroska) ---

// ebmlID encodes an element ID; IDs already carry their length marker.
func ebmlID(id uint32) []byte {
	switch {
	case id > 0xFFFFFF:
		return U32BE(id)
	case id > 0xFFFF:
		return U32BE(id)[1:]
	case id > 0xFF:
		return U16BE(uint16(id))
	}
	return []byte{byte(id)}
}

// ebmlSize encodes a data size as an 8-byte EBML varint.
func ebmlSize(n int) []byte {
	b := U64BE(uint64(n))
	b[0] = 0x01
	return b
}

// EBML returns an element whose data is the concatenation of children.
func EBML(id uint32, children ...[]byte) []byte {
	data := cat(children...)
	return cat(ebmlID(id), ebmlSize(len(data)), data)
}

// EBMLUnknown returns a master element with the "unknown size" marker.
func EBMLUnknown(id uint32, children ...[]byte) []byte {
	return cat(ebmlID(id), []byte{0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, cat(children...))
}

func EBMLUint(id uint32, v uint64) []byte   { return EBML(id, U64BE(v)) }
func EBMLString(id uint32, s string) []byte { return EBML(id, []byte(s)) }
func EBMLFloat(id uint32, f float64) []byte { return EBML(id, U64BE(math.Float64bits(f))) }

// --- ISO BMFF (MP4) ---

// Box returns a box with a 32-bit size.
func Box(typ string, payload ...[]byte) []byte {
	data := cat(payload...)
	return cat(U32BE(uint32(8+len(data))), []byte(typ), data)
}

// Box64 returns a box using the 64-bit "largesize" form.
func Box64(typ string, payload ...[]byte) []byte {
	data := cat(payload...)
	return cat(U32BE(1), []byte(typ), U64BE(uint64(16+len(data))), data)
}

// FullBox returns a box whose payload starts with version and zero flags.
func FullBox(typ string, version byte, payload ...[]byte) []byte {
	return Box(typ, append([]byte{version, 0, 0, 0}, cat(payload...)...))
}

// --- RIFF (AVI) ---

// Chunk returns a RIFF chunk, padded to an even length.
func Chunk(id string, data []byte) []byte {
	out := cat([]byte(id), U32LE(uint32(len(data))), data)
	if len(data)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

func List(typ string, children ...[]byte) []byte {
	return Chunk("LIST", cat([]byte(typ), cat(children...)))
}

func RIFF(form string, children ...[]byte) []byte {
	return Chunk("RIFF", cat([]byte(form), cat(children...)))
}

// --- Ready-made files used outside the probe package ---

// MKV returns a minimal Matroska file with one H.264 video track of w×h, one
// AAC stereo audio track in lang and the given duration.
func MKV(w, h int, durationMs int64, lang string) []byte {
	return cat(
		EBML(0x1A45DFA3, EBMLString(0x4282, "matroska")),
		EBML(0x18538067,
			EBML(0x1549A966,
				EBMLUint(0x2AD7B1, 1000000),
				EBMLFloat(0x4489, float64(durationMs)),
			),
			EBML(0x1654AE6B,
				EBML(0xAE,
					EBMLUint(0x83, 1),
					EBMLString(0x86, "V_MPEG4/ISO/AVC"),
					EBML(0xE0, EBMLUint(0xB0, uint64(w)), EBMLUint(0xBA, uint64(h))),
				),
				EBML(0xAE,
					EBMLUint(0x83, 2),
					EBMLString(0x86, "A_AAC"),
					EBMLString(0x22B59C, lang),
					EBML(0xE1, EBMLUint(0x9F, 2)),
				),
			),
		),
	)
}
