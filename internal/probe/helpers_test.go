package probe

import "bytes"

// src wraps b as a source for the native readers.
func src(b []byte) *source { return newSource(bytes.NewReader(b), int64(len(b))) }

func zeros(n int) []byte { return make([]byte, n) }

func join(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
