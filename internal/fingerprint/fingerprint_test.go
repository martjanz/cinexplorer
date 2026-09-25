package fingerprint

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fp-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(data)
	f.Close()
	return f.Name()
}

func must(t *testing.T, path string) string {
	t.Helper()
	fp, err := Of(path)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

func TestIdenticalContentSameFingerprint(t *testing.T) {
	data := bytes.Repeat([]byte("cine"), 1000)
	if must(t, write(t, data)) != must(t, write(t, data)) {
		t.Fatal("identical files must match")
	}
}

func TestDifferentContentDifferentFingerprint(t *testing.T) {
	a := bytes.Repeat([]byte{'a'}, 5000)
	b := bytes.Repeat([]byte{'a'}, 5000)
	b[4999] = 'b'
	if must(t, write(t, a)) == must(t, write(t, b)) {
		t.Fatal("different files must not match")
	}
}

func TestLargeFileOnlyEdgesAndSizeCount(t *testing.T) {
	a := bytes.Repeat([]byte{'x'}, 3*chunk)
	b := bytes.Repeat([]byte{'x'}, 3*chunk)
	b[chunk+10] = 'y' // middle byte: deliberately not covered
	if must(t, write(t, a)) != must(t, write(t, b)) {
		t.Fatal("middle bytes are not part of the fingerprint")
	}
	c := append(bytes.Repeat([]byte{'x'}, 3*chunk), 'x') // one byte longer
	if must(t, write(t, a)) == must(t, write(t, c)) {
		t.Fatal("size is part of the fingerprint")
	}
}

func TestMissingFile(t *testing.T) {
	if _, err := Of(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error")
	}
}
