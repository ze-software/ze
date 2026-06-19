// Design: plan/spec-isis-2-wire.md -- Fletcher checksum vector + corruption tests
package packet

import (
	"math/rand"
	"testing"
)

// VALIDATES: AC-3, AC-4 -- the ISO 8473 Fletcher two-step adjustment produces a
// checksum that, when re-verified over the full region (checksum field in
// place), yields zero. This is the bootstrapping property that the whole LSP
// codec depends on (spec A-1, R-1).
// PREVENTS: shipping LSPs every peer rejects because the checksum field was
// computed with only one direction (encode) correct.
func TestISISChecksumVectors(t *testing.T) {
	// The checksum field sits at a fixed offset inside the checksummed region.
	// For an IS-IS LSP the region begins at the octet after Remaining Lifetime
	// and the checksum field is at offset lspChecksumRegionCheckOff within it
	// (LSPID 8 + sequence 4 = 12). We exercise a range of offsets and lengths.
	cases := []struct {
		name     string
		length   int
		checkOff int
	}{
		{"lsp-typical", 40, lspChecksumRegionCheckOff},
		{"check-at-zero", 16, 0},
		{"check-near-end", 20, 18},
		{"minimal", 3, 0},
		{"short-after-field", 4, 0},
		{"large", 1492, lspChecksumRegionCheckOff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(int64(tc.length*131 + tc.checkOff)))
			data := make([]byte, tc.length)
			for i := range data {
				data[i] = byte(rng.Intn(256))
			}
			// Compute and place the checksum.
			high, low := Checksum(data, tc.checkOff)
			data[tc.checkOff] = high
			data[tc.checkOff+1] = low

			// The reserved value 0 must never be stored in a checksum octet.
			if high == 0 || low == 0 {
				t.Fatalf("checksum octet is the reserved 0: high=%d low=%d", high, low)
			}

			// Verification over the full region (field in place) must be zero.
			if !VerifyChecksum(data) {
				t.Fatalf("VerifyChecksum(encode(x)) != 0 for len=%d checkOff=%d (high=%#02x low=%#02x)",
					tc.length, tc.checkOff, high, low)
			}
		})
	}
}

// VALIDATES: AC-4 -- a fixed, hand-checkable vector. With a region of four
// octets [00 00 03 04] (checksum field at offset 0), the two-step adjustment
// must produce octets that verify to zero, and the result must be stable
// across runs (a regression pin on the arithmetic, not just the property).
// PREVENTS: a refactor silently changing the modular arithmetic.
func TestISISChecksumFixedVector(t *testing.T) {
	data := []byte{0x00, 0x00, 0x03, 0x04}
	high, low := Checksum(data, 0)
	data[0] = high
	data[1] = low
	if !VerifyChecksum(data) {
		t.Fatalf("fixed vector does not verify: % x", data)
	}
	// Pin the exact octets so the arithmetic cannot drift unnoticed. These are
	// the values the verified two-step adjustment produces for [00 00 03 04]
	// with the checksum field at offset 0 (confirmed by VerifyChecksum above).
	const wantHigh, wantLow = 0x0b, 0xed
	if high != wantHigh || low != wantLow {
		t.Fatalf("fixed vector checksum = %#02x %#02x, want %#02x %#02x", high, low, wantHigh, wantLow)
	}
}

// VALIDATES: AC-3 -- flipping any single octet of a checksummed region makes
// VerifyChecksum fail. This is the detection property that protects the LSDB
// from accepting a corrupted LSP.
// PREVENTS: a checksum that passes regardless of content (e.g. an all-zero or
// constant implementation).
func TestISISChecksumDetectsCorruption(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	data := make([]byte, 64)
	for i := range data {
		data[i] = byte(rng.Intn(256))
	}
	const checkOff = lspChecksumRegionCheckOff
	high, low := Checksum(data, checkOff)
	data[checkOff] = high
	data[checkOff+1] = low
	if !VerifyChecksum(data) {
		t.Fatal("baseline must verify before corruption test")
	}

	for i := range data {
		orig := data[i]
		// Flip to a different value and confirm verification fails.
		data[i] = orig ^ 0xff
		if VerifyChecksum(data) {
			t.Fatalf("corruption at octet %d (%#02x -> %#02x) was not detected", i, orig, data[i])
		}
		data[i] = orig // restore
	}
	// Sanity: after restoring everything, it verifies again.
	if !VerifyChecksum(data) {
		t.Fatal("region must verify again after restoring all octets")
	}
}

// VALIDATES: the Fletcher sums are taken modulo 255 (GF(255)), not 256: a
// single octet of 0xFF folds correctly and does not produce a stored 0.
// PREVENTS: an off-by-one modulus (mod 256) that would silently disagree with
// every other IS-IS implementation.
func TestISISChecksumModulus(t *testing.T) {
	// A region full of 0xFF is a good modulus probe: under mod 256 the sums
	// would wrap differently than under mod 255.
	data := make([]byte, 32)
	for i := range data {
		data[i] = 0xff
	}
	high, low := Checksum(data, 4)
	data[4] = high
	data[5] = low
	if !VerifyChecksum(data) {
		t.Fatalf("0xFF-filled region failed to verify (mod-255 check): high=%#02x low=%#02x", high, low)
	}
}

// VALIDATES: the defensive bounds guard in Checksum returns a defined (0,0)
// instead of indexing out of range when the checksum field would not fit in
// the region. Every production caller passes a compile-time-constant offset
// over a long-enough region, so this guard never fires in normal operation;
// the test pins the contract for fuzz/garbage input.
// PREVENTS: a future caller (or a fuzz harness) feeding an empty/short region
// or a bad offset and getting a panic or silently wrong arithmetic.
func TestISISChecksumOutOfRangeGuard(t *testing.T) {
	cases := []struct {
		name     string
		data     []byte
		checkOff int
	}{
		{"empty", nil, 0},
		{"empty-nonzero-off", []byte{}, 4},
		{"negative-off", []byte{1, 2, 3, 4}, -1},
		{"off-at-last-octet", []byte{1, 2, 3, 4}, 3}, // checkOff+1 == len, field would overrun
		{"off-past-end", []byte{1, 2, 3, 4}, 10},     // far past the end
		{"single-octet-region", []byte{0x42}, 0},     // field needs 2 octets, region has 1
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			high, low := Checksum(tc.data, tc.checkOff)
			if high != 0 || low != 0 {
				t.Fatalf("out-of-range Checksum = %#02x %#02x, want 0x00 0x00", high, low)
			}
		})
	}
}
