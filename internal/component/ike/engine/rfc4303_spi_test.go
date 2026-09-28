// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the ESP SPI a Child SA advertises

package engine

import (
	"crypto/rand"
	"encoding/binary"
	"testing"
)

// scriptedSPIReader answers each read with the next 4-octet word of words, and
// repeats the last word once the script runs out.
type scriptedSPIReader struct {
	words []uint32
	reads int
}

func (r *scriptedSPIReader) Read(p []byte) (int, error) {
	i := min(r.reads, len(r.words)-1)
	r.reads++
	var word [4]byte
	binary.BigEndian.PutUint32(word[:], r.words[i])
	return copy(p, word[:]), nil
}

// TestGenerateESPSPISkipsTheReservedZero proves the generator never answers
// the reserved SPI 0, deterministically.
//
// Goal: TestGenerateESPSPI draws 100 real random SPIs, and a draw of zero has a
// chance near 2^-32, so it cannot fail when the zero check is removed. Method:
// crypto/rand.Reader is replaced for this test with a script that answers zero
// three times and then 0x00000100. generateESPSPI MUST discard the zero draws
// and return 0x00000100. The test runs sequentially: no test in this package
// calls t.Parallel, and the reader is restored in Cleanup.
//
// RFC requirement: RFC4303-2.1-1 negative -- when the random source draws the reserved SPI 0, generateESPSPI does not return it: three zero draws are discarded.
// RFC requirement: RFC4303-2.1-1 positive -- the first non-zero draw (0x00000100) is the SPI generateESPSPI returns, unchanged.
func TestGenerateESPSPISkipsTheReservedZero(t *testing.T) {
	saved := rand.Reader
	t.Cleanup(func() { rand.Reader = saved })
	script := &scriptedSPIReader{words: []uint32{0, 0, 0, 0x00000100}}
	rand.Reader = script

	spi, err := generateESPSPI()
	if err != nil {
		t.Fatalf("generateESPSPI: %v", err)
	}
	if spi == 0 {
		t.Fatal("generateESPSPI returned the reserved SPI 0 (RFC 4303 Section 2.1)")
	}
	if spi != 0x00000100 {
		t.Errorf("generateESPSPI = %#08x, want the first non-zero draw 0x00000100", spi)
	}
}
