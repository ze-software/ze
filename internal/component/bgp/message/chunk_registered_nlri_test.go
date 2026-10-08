// Design: docs/architecture/wire/nlri.md -- registered native framing.

package message

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestMUPChunkNativeBoundaries exercises the real chunk consumers with complete
// ISD and T2ST routes, concatenation, and zero/nonzero ADD-PATH identifiers.
// draft-ietf-bess-mup-safi Sections 3.1, 3.1.1 and 3.1.4 define these envelopes.
func TestMUPChunkNativeBoundaries(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		t.Run(strconv.FormatBool(addPath), func(t *testing.T) {
			first, err := hex.DecodeString("0100010c0000fde90000000118c00002")
			if err != nil {
				t.Fatal(err)
			}
			second, err := hex.DecodeString("010004110000fde90000000140c633640101020304")
			if err != nil {
				t.Fatal(err)
			}
			if addPath {
				first = append([]byte{0, 0, 0, 0}, first...)
				second = append([]byte{0, 0, 0, 17}, second...)
			}
			data := append(append([]byte(nil), first...), second...)
			// A limit one octet before the second route ends must keep the first whole.
			limit := len(data) - 1
			fitting, remaining, err := SplitMPNLRI(data, family.AFIIPv4, family.SAFIMUP, addPath, limit)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(fitting, first) || !bytes.Equal(remaining, second) {
				t.Fatalf("split = %x / %x, want %x / %x", fitting, remaining, first, second)
			}
			if &fitting[0] != &data[0] || &remaining[0] != &data[len(first)] {
				t.Fatal("native split copied the input")
			}
			chunks, err := ChunkMPNLRI(data, family.AFIIPv4, family.SAFIMUP, addPath, limit, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(chunks) != 2 || !bytes.Equal(chunks[0], first) || !bytes.Equal(chunks[1], second) {
				t.Fatalf("chunks = %x, want the two complete routes", chunks)
			}
			// First-boundary reads stop after the first non-fitting complete route,
			// rather than rescanning a tail that belongs to the next call.
			withTail := append(append([]byte(nil), data...), 1)
			fitting, remaining, err = SplitMPNLRI(withTail, family.AFIIPv4, family.SAFIMUP, addPath, limit)
			if err != nil || !bytes.Equal(fitting, first) || !bytes.Equal(remaining, withTail[len(first):]) {
				t.Fatalf("bounded split = %x / %x, error %v", fitting, remaining, err)
			}
			if _, _, err = SplitMPNLRI(remaining, family.AFIIPv4, family.SAFIMUP, addPath, len(remaining)); !errors.Is(err, ErrNLRIMalformed) {
				t.Fatalf("malformed next chunk error = %v", err)
			}
			if _, err = ChunkMPNLRI(withTail, family.AFIIPv4, family.SAFIMUP, addPath, len(withTail), nil); !errors.Is(err, ErrNLRIMalformed) {
				t.Fatalf("malformed complete section error = %v", err)
			}
			if _, _, err = SplitMPNLRI(first, family.AFIIPv4, family.SAFIMUP, addPath, len(first)-1); !errors.Is(err, ErrNLRITooLarge) {
				t.Fatalf("oversized route error = %v", err)
			}
		})
	}
}

// TestChunkLabeledCompatibilityFraming keeps chunking envelope-only: RFC 8277
// Section 2.4's Compatibility field is not an announcement label-stack walk.
func TestChunkLabeledCompatibilityFraming(t *testing.T) {
	route := []byte{48, 0x80, 0, 0, 192, 0, 2}
	data := append(append([]byte(nil), route...), route...)
	chunks, err := ChunkMPNLRI(data, family.AFIIPv4, family.SAFIMPLSLabel, false, len(route), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 || !bytes.Equal(chunks[0], route) || !bytes.Equal(chunks[1], route) {
		t.Fatalf("Compatibility chunks = %x", chunks)
	}
}
