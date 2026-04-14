package l2tp

import "testing"

// FuzzHiddenDecrypt ensures HiddenDecrypt never panics or reads out of bounds
// on arbitrary ciphertext input. Correctness of the recovered plaintext is
// covered by TestHidden* round-trip tests.
func FuzzHiddenDecrypt(f *testing.F) {
	seed := [][]byte{
		{},
		{0, 0},
		{0, 0x0A, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		make([]byte, 16),
		make([]byte, 40),
	}
	for _, s := range seed {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, ct []byte) {
		dst := make([]byte, len(ct))
		// Either error or successful extraction, never a panic.
		if _, err := HiddenDecrypt(dst, AVPHostName, []byte("s"), []byte("rv"), ct); err != nil {
			return // error outcome is acceptable; no-panic is the invariant.
		}
	})
}
