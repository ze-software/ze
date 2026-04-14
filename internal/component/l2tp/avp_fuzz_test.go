package l2tp

import (
	"errors"
	"testing"
)

// FuzzAVPIterator exercises the iterator on arbitrary input; bounds-safety
// is the invariant. Any panic or out-of-range slice access is a bug.
func FuzzAVPIterator(f *testing.F) {
	seed := [][]byte{
		{0x80, 0x08, 0, 0, 0, 0, 0, 0},
		{0x80, 0x06, 0, 0, 0, 0},
		{},
		{0xFF, 0xFF},
		{0x00, 0x05, 0, 0, 0, 0}, // length < 6
	}
	for _, s := range seed {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		it := NewAVPIterator(in)
		var total int
		for {
			_, _, _, val, ok := it.Next()
			if !ok {
				break
			}
			total += AVPHeaderLen + len(val)
			if total > len(in) {
				t.Fatalf("iterator advanced past input: total=%d len=%d", total, len(in))
			}
		}
		// Err may be nil or ErrInvalidAVPLen; either is acceptable.
		if err := it.Err(); err != nil && !errors.Is(err, ErrInvalidAVPLen) {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}
