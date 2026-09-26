// Package fingerprint computes a cheap content fingerprint for large media files.
package fingerprint

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
)

const chunk = 1 << 20

// Of hashes the first and last MiB plus the file size. Identical files always
// match, and a moved or renamed file keeps its fingerprint. An empty file
// (a failed copy, a placeholder) has no content to recognize: its
// fingerprint is "".
func Of(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := st.Size()
	if size == 0 {
		return "", nil
	}
	h := sha256.New()
	if size <= 2*chunk {
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
	} else {
		if _, err := io.CopyN(h, f, chunk); err != nil {
			return "", err
		}
		if _, err := f.Seek(-chunk, io.SeekEnd); err != nil {
			return "", err
		}
		if _, err := io.CopyN(h, f, chunk); err != nil {
			return "", err
		}
	}
	var sz [8]byte
	binary.LittleEndian.PutUint64(sz[:], uint64(size))
	h.Write(sz[:])
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}
