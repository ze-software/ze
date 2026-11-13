// Design: docs/architecture/appliance/remote-operations.md -- bounded complete upload frames.

package configupload

import (
	"bytes"
	"testing"
)

// FuzzRead classifies raw frames from their wire bytes, independently of Write.
// Only a nonzero, bounded uint32BE length with exactly that many payload bytes
// is accepted, and rejection must never expose a partial configuration.
func FuzzRead(f *testing.F) {
	for _, frame := range [][]byte{
		{},
		{0},
		{0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 1},
		{0, 0, 0, 2, 'x'},
		{0, 0, 0, 1, 'x'},
		{0, 0, 0, 1, 'x', 'y'},
		{0, 0, 0, 3, 0, '\n', 255},
		{0, 64, 0, 0},
		{0, 64, 0, 1},
		{255, 255, 255, 255},
	} {
		f.Add(frame)
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		content, err := Read(bytes.NewReader(frame))
		if len(frame) < 4 {
			if err == nil {
				t.Fatal("short wire header accepted")
			}
			if content != nil {
				t.Fatal("short wire header exposed configuration bytes")
			}
			return
		}
		declared := uint32(frame[0])<<24 | uint32(frame[1])<<16 | uint32(frame[2])<<8 | uint32(frame[3])
		valid := declared > 0 && declared <= 4194304 && uint64(declared) == uint64(len(frame)-4)
		if !valid {
			if err == nil {
				t.Fatal("incomplete, oversized, empty, or trailing wire frame accepted")
			}
			if content != nil {
				t.Fatal("invalid wire frame exposed partial configuration")
			}
			return
		}
		if err != nil {
			t.Fatalf("complete bounded wire frame refused: %v", err)
		}
		if !bytes.Equal(content, frame[4:]) {
			t.Fatal("accepted configuration differs from the exact wire payload")
		}
	})
}
