// Design: docs/architecture/wire/isis.md -- Fletcher checksum vector + corruption tests

package packet

import "testing"

// isisClosedForm computes X and Y the way RFC 905 Annex B.3 states them,
// independently of the producer: the field at 1-based octet n is taken as zero,
// C0 and C1 are summed octet by octet (B.3.3), and X = -C1 + (L-n).C0,
// Y = C1 - (L-n+1).C0 (B.3.4), all modulo 255. A zero result is reported as
// 255, because B.2 b) reads 255 as zero and Ze stores the non-zero form.
func isisClosedForm(data []byte, checkOff int) (byte, byte) {
	c0, c1 := 0, 0
	for i, b := range data {
		v := int(b)
		if i == checkOff || i == checkOff+1 {
			v = 0
		}
		c0 = (c0 + v) % 255
		c1 = (c1 + c0) % 255
	}
	length, n := len(data), checkOff+1
	x := ((-c1+(length-n)*c0)%255 + 255) % 255
	y := ((c1-(length-n+1)*c0)%255 + 255) % 255
	if x == 0 {
		x = 255
	}
	if y == 0 {
		y = 255
	}
	return byte(x), byte(y)
}

// TestISISChecksumMatchesClosedForm proves Checksum computes the Annex B.3.4
// formulas at every field position.
//
// Goal: X and Y depend on L-n, and TestISISChecksumFixedVector pins them at
// offset 0 only. Method: a 24-octet region, and the field at every offset from
// 0 to L-2. At each offset Checksum MUST return the values the formula in
// isisClosedForm gives. The stored octets are not re-verified here, so a
// verifier defect cannot hide a formula defect.
//
// RFC requirement: RFC905-x-3 positive -- at every field offset of a 24-octet region, Checksum returns exactly X = -C1 + (L-n).C0 and Y = C1 - (L-n+1).C0 (mod 255), computed independently from the Annex B.3.3 sums.
func TestISISChecksumMatchesClosedForm(t *testing.T) {
	data := make([]byte, 24)
	for i := range data {
		data[i] = byte(i*37 + 11)
	}
	for off := 0; off+1 < len(data); off++ {
		wantX, wantY := isisClosedForm(data, off)
		x, y := Checksum(data, off)
		if x != wantX || y != wantY {
			t.Errorf("offset %d: Checksum = %#02x %#02x, want %#02x %#02x", off, x, y, wantX, wantY)
		}
	}
}

// TestISISVerifyRequiresBothSumsZero proves VerifyChecksum fails a region when
// either sum is non-zero, not only when both are.
//
// Goal: RFC 905 B.4.3 fails the check when "either or both" of C0 and C1 is
// non-zero. A single-octet flip changes both sums, so it cannot tell a verifier
// that reads one sum from one that reads both. Method: start from a valid
// region. Swapping two unequal adjacent octets keeps C0 at zero and moves C1.
// Adding 1 to the second-to-last octet and subtracting 2 from the last moves C0
// by -1 and C1 by 2*1 + 1*(-2) = 0. VerifyChecksum MUST refuse both, and MUST
// accept the unmodified region.
//
// RFC requirement: RFC905-x-4 negative -- VerifyChecksum refuses a region whose C0 is zero and C1 is not (two adjacent octets swapped), and a region whose C1 is zero and C0 is not (+1 then -2 on the last two octets).
// RFC requirement: RFC905-x-4 positive -- VerifyChecksum accepts the same region when both C0 and C1 are zero after the generated X and Y are placed.
func TestISISVerifyRequiresBothSumsZero(t *testing.T) {
	valid := make([]byte, 24)
	for i := range valid {
		valid[i] = byte(i*37 + 11)
	}
	const off = lspChecksumRegionCheckOff
	valid[off], valid[off+1] = Checksum(valid, off)
	if !VerifyChecksum(valid) {
		t.Fatalf("generated region does not verify: % x", valid)
	}

	swapped := append([]byte(nil), valid...)
	swapped[2], swapped[3] = swapped[3], swapped[2]
	if swapped[2] == swapped[3] {
		t.Fatalf("test data: octets 2 and 3 are equal, the swap changes nothing")
	}
	if VerifyChecksum(swapped) {
		t.Errorf("C0 zero, C1 non-zero: VerifyChecksum accepted % x", swapped)
	}

	shifted := append([]byte(nil), valid...)
	last := len(shifted) - 1
	shifted[last-1]++
	shifted[last] -= 2
	if VerifyChecksum(shifted) {
		t.Errorf("C1 zero, C0 non-zero: VerifyChecksum accepted % x", shifted)
	}
}

// TestISISChecksumRefusesMisplacedXY proves X and Y verify only in the
// octets the formula was solved for.
//
// Goal: X and Y from B.3.4 depend on n, so the same two values at other octets
// are not a valid checksum. Method: generate X and Y for the field at n. Put X
// at n and Y at n+1, which VerifyChecksum MUST accept. Put Y at n and X at n+1,
// and put X at n-1 and Y at n with n+1 zero. VerifyChecksum MUST refuse both.
//
// RFC requirement: RFC905-x-3 negative -- the X,Y that Checksum computes for field octet n verify at n,n+1 and are refused when swapped (Y at n, X at n+1) or shifted one octet earlier (n-1,n).
func TestISISChecksumRefusesMisplacedXY(t *testing.T) {
	region := make([]byte, 24)
	for i := range region {
		region[i] = byte(i*37 + 11)
	}
	const n = lspChecksumRegionCheckOff
	region[n], region[n+1] = 0, 0
	x, y := Checksum(region, n)
	if x == y {
		t.Fatalf("test data: X == Y (%#02x), the swap changes nothing", x)
	}

	placed := append([]byte(nil), region...)
	placed[n], placed[n+1] = x, y
	if !VerifyChecksum(placed) {
		t.Errorf("X,Y at n,n+1: VerifyChecksum refused % x", placed)
	}

	swapped := append([]byte(nil), region...)
	swapped[n], swapped[n+1] = y, x
	if VerifyChecksum(swapped) {
		t.Errorf("Y at n, X at n+1: VerifyChecksum accepted % x", swapped)
	}

	shifted := append([]byte(nil), region...)
	shifted[n-1], shifted[n] = x, y
	if VerifyChecksum(shifted) {
		t.Errorf("X,Y at n-1,n: VerifyChecksum accepted % x", shifted)
	}
}
