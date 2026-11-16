// Design: docs/architecture/testing/interop.md -- read only known non-ESN sequence metadata.
// Related: replay.go -- no replay bitmap codec or replay writer.
package main

import (
	"encoding/binary"
	"testing"
)

// TestExtendedWindowSequencePreservesOSeq handles larger windows without requiring ESN.
func TestExtendedWindowSequencePreservesOSeq(t *testing.T) {
	value := extendedReplayFixture()
	got, err := extendedWindowSequence(value)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0xfedcba98 {
		t.Fatalf("OSeq = %#x, want the full unsigned 32-bit value", got)
	}
}

// TestExtendedWindowSequenceRefusesInvalidMetadata cannot truncate a high sequence or bitmap.
func TestExtendedWindowSequenceRefusesInvalidMetadata(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func([]byte) []byte
	}{
		{"short", func(value []byte) []byte { return value[:23] }},
		{"bitmap-short", func(value []byte) []byte { return value[:len(value)-4] }},
		{"bitmap-overflow", func(value []byte) []byte { binary.NativeEndian.PutUint32(value[:4], ^uint32(0)); return value }},
		{"high-oseq", func(value []byte) []byte { binary.NativeEndian.PutUint32(value[12:16], 1); return value }},
		{"window-too-large", func(value []byte) []byte { binary.NativeEndian.PutUint32(value[20:24], 65); return value }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := extendedWindowSequence(test.change(extendedReplayFixture())); err == nil {
				t.Fatal("invalid replay metadata was accepted")
			}
		})
	}
}

func extendedReplayFixture() []byte {
	value := make([]byte, 32)
	binary.NativeEndian.PutUint32(value[:4], 2)
	binary.NativeEndian.PutUint32(value[4:8], 0xfedcba98)
	binary.NativeEndian.PutUint32(value[20:24], 64)
	return value
}
