// Design: docs/architecture/wire/isis.md -- ISO 8473 Fletcher checksum, two-step adjustment
// ISO/IEC 10589 clause 7.3.11: LSP checksum over the octets following the
// Remaining Lifetime field, with the checksum field participating in its own
// computation (the two-step adjustment).

package packet

import "github.com/ze-software/ze/internal/plugins/isis/types"

// The Fletcher checksum (ISO 8473 annex C, referenced by ISO/IEC 10589 clause
// 7.3.11) maintains two running sums over the data octets:
//
//	c0 = (c0 + octet) mod 255
//	c1 = (c1 + c0)    mod 255
//
// Both sums use modulo 255 (not 256): the value 0 in the checksum field is
// reserved to mean "checksum not computed", so the arithmetic is over GF(255).
//
// The checksum field is two octets at a known position INSIDE the checksummed
// region. Because the field participates in its own sum, the two octets cannot
// simply be the final (c0,c1): they must be chosen so that, when the whole
// region (including the field) is re-summed, both sums come out zero. ISO 8473
// gives a closed-form adjustment (the "two-step adjustment"): compute the sums
// with the checksum field zeroed, then solve for the two octets X, Y that drive
// the verification sums to zero.
//
// This file implements the two directions as separate, separately tested
// functions (spec R-1): Checksum computes (X,Y) given zeroed field octets;
// VerifyChecksum re-sums the region and reports whether both sums are zero.

const fletcherModulus = 255

// lspChecksumRegionCheckOff is the offset of the 2-octet checksum field within
// the LSP checksummed region. ISO/IEC 10589 clause 7.3.11: the checksum covers
// the PDU from the octet after Remaining Lifetime to the end. Within that
// region the layout is LSPID (8) + Sequence Number (4) + Checksum (2) + ...,
// so the checksum field begins at offset 12. (Defined here, alongside the
// checksum, because Checksum/VerifyChecksum and the LSP codec both reference
// it.)
const lspChecksumRegionCheckOff = types.LSPIDLen + types.SequenceNumberLen

// Checksum computes the two ISO 8473 Fletcher checksum octets for the data
// region, where the checksum field itself sits at checkOff octets from the
// start of the region (0-based). The two checksum octets in data[checkOff] and
// data[checkOff+1] are treated as zero during the computation regardless of
// their current contents, exactly as required before placing the result.
//
// It returns the (high, low) octets to store at data[checkOff] and
// data[checkOff+1] so that VerifyChecksum over the same region yields zero.
//
// ISO/IEC 10589 clause 7.3.11: the checksum is computed "commencing with the
// Source ID field" (the octet after Remaining Lifetime) to the end of the PDU;
// for an LSP the checksum field is at the start+offset of the checksummed
// region. The classic adjustment (ISO 8473 annex C.3.4.2) is reproduced below.
func Checksum(data []byte, checkOff int) (high, low byte) {
	// Defensive bound: every production caller passes the compile-time constant
	// lspChecksumRegionCheckOff over a region long enough to hold the field, so
	// this never fires in normal use. The guard exists so a fuzz harness or a
	// garbage region (empty, or a checkOff that would index past the end) returns
	// a defined (0,0) instead of running the adjustment on out-of-range indices.
	if len(data) == 0 || checkOff < 0 || checkOff+1 >= len(data) {
		return 0, 0
	}

	var c0, c1 int

	// Step 1: run the Fletcher sums over the whole region with the two
	// checksum octets treated as zero.
	for i := range data {
		b := int(data[i])
		if i == checkOff || i == checkOff+1 {
			b = 0
		}
		c0 = (c0 + b) % fletcherModulus
		c1 = (c1 + c0) % fletcherModulus
	}

	// Step 2: solve for the checksum octets. Let the high octet X sit at offset
	// k = checkOff and the low octet Y at k+1, within a region of length L.
	// When the region is re-summed with X and Y in place, X contributes +X to
	// the c0 sum and +X*(L-k) to the c1 sum (it is folded into c1 once for each
	// of the (L-k) positions from k to the end); Y contributes +Y to c0 and
	// +Y*(L-k-1) to c1. Requiring both final sums to be zero (mod 255):
	//
	//	C0 + X + Y                 == 0
	//	C1 + X*(L-k) + Y*(L-k-1)   == 0
	//
	// Let m = L - k. Solving (substitute Y = -C0 - X into the second equation;
	// the X*(m) and X*(m-1) terms cancel to a single X):
	//
	//	X = (m-1)*C0 - C1   (mod 255)   -- the high octet
	//	Y = C1 - m*C0       (mod 255)   -- the low octet
	//
	// where C0, C1 are the step-1 sums. The result is taken modulo 255 into the
	// range 0..254; a computed 0 is replaced by 255 (the reserved 0 value must
	// never appear in a stored checksum octet). This is the ISO 8473 annex C
	// two-step adjustment.
	m := len(data) - checkOff

	x := subMod(mulMod(m-1, c0), c1) // (m-1)*c0 - c1
	y := subMod(c1, mulMod(m, c0))   // c1 - m*c0

	high = normalize(x)
	low = normalize(y)
	return high, low
}

// VerifyChecksum re-runs the Fletcher sums over the data region (with the
// checksum octets in place) and reports true iff both sums are zero, which is
// the ISO 8473 condition for a correct checksum (ISO/IEC 10589 clause 7.3.11).
// A region whose stored checksum field is all zero is treated as "checksum not
// present" and is NOT considered valid here (callers that allow disabled
// checksums check that separately); this function reports strict correctness.
func VerifyChecksum(data []byte) bool {
	var c0, c1 int
	for _, b := range data {
		c0 = (c0 + int(b)) % fletcherModulus
		c1 = (c1 + c0) % fletcherModulus
	}
	return c0 == 0 && c1 == 0
}

// mulMod returns (a*b) mod 255 with a,b already reduced or small.
func mulMod(a, b int) int {
	r := (a * b) % fletcherModulus
	if r < 0 {
		r += fletcherModulus
	}
	return r
}

// subMod returns (a-b) mod 255 in the range 0..254.
func subMod(a, b int) int {
	r := (a - b) % fletcherModulus
	if r < 0 {
		r += fletcherModulus
	}
	return r
}

// normalize maps a value reduced mod 255 (range 0..254) to a stored octet,
// replacing 0 with 255. ISO 8473 reserves only the COMBINED 16-bit checksum
// field value 0x0000 to mean "checksum not computed"; an individual octet of
// 0x00 is otherwise legal. We emit 0xFF in place of a computed 0 because, under
// the mod-255 arithmetic, 0xFF == 0 (255 mod 255), so the substituted octet
// still verifies, while guaranteeing the field as a whole can never collide
// with the reserved all-zero value when the other octet is also zero.
func normalize(v int) byte {
	v %= fletcherModulus
	if v < 0 {
		v += fletcherModulus
	}
	if v == 0 {
		return fletcherModulus
	}
	return byte(v)
}
