package probe

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func TestSourceRead(t *testing.T) {
	t.Run("read inside file", func(t *testing.T) {
		s := newSource(bytes.NewReader(make([]byte, 10)), 10)
		b, err := s.read(2, 5)
		if err != nil {
			t.Fatalf("read(2, 5) error = %v, want nil", err)
		}
		if len(b) != 5 {
			t.Errorf("read(2, 5) returned %d bytes, want 5", len(b))
		}
	})

	t.Run("read past end", func(t *testing.T) {
		s := newSource(bytes.NewReader(make([]byte, 10)), 10)
		_, err := s.read(8, 5)
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("read(8, 5) error = %v, want ErrInvalid", err)
		}
	})

	t.Run("offset overflow", func(t *testing.T) {
		s := newSource(bytes.NewReader(make([]byte, 10)), 10)
		_, err := s.read(math.MaxInt64-2, 8)
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("read(MaxInt64-2, 8) error = %v, want ErrInvalid", err)
		}
	})

	t.Run("readUpTo truncates to remaining bytes", func(t *testing.T) {
		s := newSource(bytes.NewReader(make([]byte, 10)), 10)
		b, err := s.readUpTo(8, 12)
		if err != nil {
			t.Fatalf("readUpTo(8, 12) error = %v, want nil", err)
		}
		if len(b) != 2 {
			t.Errorf("readUpTo(8, 12) returned %d bytes, want 2", len(b))
		}
	})

	t.Run("exceeding budget", func(t *testing.T) {
		s := newSource(bytes.NewReader(make([]byte, 10)), 10)
		s.budget = 4
		_, err := s.read(0, 5)
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("read(0, 5) with budget 4 error = %v, want ErrInvalid", err)
		}
	})
}
