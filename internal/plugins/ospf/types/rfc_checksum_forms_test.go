// Design: docs/architecture/ospf/ospf-1-types.md -- OSPF checksum algorithm vectors and boundaries

package types

import "testing"

// TestInternetChecksumEvenLength pins the even-count form of the RFC 1071 sum.
//
// Goal: an even-length window is summed as the big-endian 16-bit words [A,B],
// [C,D] and so on, with no pad octet added. Method: exact vectors computed by
// hand. [12 34 56 78] sums to 0x68ac, so the checksum is 0x9753. Summing the
// words little-endian gives 0x5397, and adding a pad of the last octet gives
// 0x1f53, so either break fails here. The six-octet vector sums to 0x10368,
// which folds to 0x0369 and gives 0xfc96.
//
// RFC requirement: RFC1071-1-4 positive -- an even-length window is summed as big-endian words [a,b] = a*256+b with no pad octet: internetChecksum answers exactly 0x9753 for [12 34 56 78] and 0xfc96 for [12 34 56 78 9a bc].
func TestInternetChecksumEvenLength(t *testing.T) {
	cases := []struct {
		data []byte
		want uint16
	}{
		{[]byte{0x12, 0x34, 0x56, 0x78}, 0x9753},
		{[]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc}, 0xfc96},
	}
	for _, c := range cases {
		if got := internetChecksum(c.data); got != c.want {
			t.Errorf("internetChecksum(% x) = %#04x, want %#04x", c.data, got, c.want)
		}
	}
}

// fletcherClosedForm computes X and Y the way RFC 905 Annex B.3 states them,
// independently of the producer: the field at 1-based octet n is taken as zero,
// C0 and C1 are summed octet by octet (B.3.3), and X = -C1 + (L-n).C0,
// Y = C1 - (L-n+1).C0 (B.3.4), all modulo 255. A zero result is reported as
// 255, because B.2 b) reads 255 as zero and Ze stores the non-zero form.
func fletcherClosedForm(data []byte, checkOff int) (byte, byte) {
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

// TestFletcherGenerateMatchesClosedForm proves the generator computes the
// Annex B.3.4 formulas at every field position.
//
// Goal: X and Y depend on L-n, so a vector at one offset does not pin the
// formula. Method: a 24-octet region, and the field at every offset from 0 to
// L-2. At each offset fletcherGenerate MUST return the values the formula in
// fletcherClosedForm gives. The stored octets are not re-verified here, so a
// verifier defect cannot hide a formula defect.
//
// RFC requirement: RFC905-x-3 positive -- at every field offset of a 24-octet region, fletcherGenerate returns exactly X = -C1 + (L-n).C0 and Y = C1 - (L-n+1).C0 (mod 255), computed independently from the Annex B.3.3 sums.
func TestFletcherGenerateMatchesClosedForm(t *testing.T) {
	data := make([]byte, 24)
	for i := range data {
		data[i] = byte(i*37 + 11)
	}
	for off := 0; off+1 < len(data); off++ {
		wantX, wantY := fletcherClosedForm(data, off)
		x, y := fletcherGenerate(data, off)
		if x != wantX || y != wantY {
			t.Errorf("offset %d: fletcherGenerate = %#02x %#02x, want %#02x %#02x", off, x, y, wantX, wantY)
		}
	}
}

// TestFletcherVerifyRequiresBothSumsZero proves the verifier fails a region
// when either sum is non-zero, not only when both are.
//
// Goal: RFC 905 B.4.3 fails the check when "either or both" of C0 and C1 is
// non-zero. A single-octet flip changes both sums, so it cannot tell a verifier
// that reads one sum from one that reads both. Method: start from a valid
// region. Swapping two unequal adjacent octets keeps C0 at zero and moves C1.
// Adding 1 to the second-to-last octet and subtracting 2 from the last moves C0
// by -1 and C1 by 2*1 + 1*(-2) = 0. fletcherVerify MUST refuse both, and MUST
// accept the unmodified region.
//
// RFC requirement: RFC905-x-4 negative -- fletcherVerify refuses a region whose C0 is zero and C1 is not (two adjacent octets swapped), and a region whose C1 is zero and C0 is not (+1 then -2 on the last two octets).
// RFC requirement: RFC905-x-4 positive -- fletcherVerify accepts the same region when both C0 and C1 are zero after the generated X and Y are placed.
func TestFletcherVerifyRequiresBothSumsZero(t *testing.T) {
	valid := make([]byte, 24)
	for i := range valid {
		valid[i] = byte(i*37 + 11)
	}
	const off = lsaChecksumOffsetInCoveredRegion
	valid[off], valid[off+1] = fletcherGenerate(valid, off)
	if !fletcherVerify(valid) {
		t.Fatalf("generated region does not verify: % x", valid)
	}

	swapped := append([]byte(nil), valid...)
	swapped[2], swapped[3] = swapped[3], swapped[2]
	if swapped[2] == swapped[3] {
		t.Fatalf("test data: octets 2 and 3 are equal, the swap changes nothing")
	}
	if fletcherVerify(swapped) {
		t.Errorf("C0 zero, C1 non-zero: fletcherVerify accepted % x", swapped)
	}

	shifted := append([]byte(nil), valid...)
	last := len(shifted) - 1
	shifted[last-1]++
	shifted[last] -= 2
	if fletcherVerify(shifted) {
		t.Errorf("C1 zero, C0 non-zero: fletcherVerify accepted % x", shifted)
	}
}

// TestFletcherGenerateRefusesMisplacedXY proves X and Y verify only in the
// octets the formula was solved for.
//
// Goal: X and Y from B.3.4 depend on n, so the same two values at other octets
// are not a valid checksum. Method: generate X and Y for the field at n. Put X
// at n and Y at n+1, which fletcherVerify MUST accept. Put Y at n and X at
// n+1, and put X at n-1 and Y at n with n+1 zero. fletcherVerify MUST refuse
// both.
//
// RFC requirement: RFC905-x-3 negative -- the X,Y that fletcherGenerate computes for field octet n verify at n,n+1 and are refused when swapped (Y at n, X at n+1) or shifted one octet earlier (n-1,n).
func TestFletcherGenerateRefusesMisplacedXY(t *testing.T) {
	region := make([]byte, 24)
	for i := range region {
		region[i] = byte(i*37 + 11)
	}
	const n = lsaChecksumOffsetInCoveredRegion
	region[n], region[n+1] = 0, 0
	x, y := fletcherGenerate(region, n)
	if x == y {
		t.Fatalf("test data: X == Y (%#02x), the swap changes nothing", x)
	}

	placed := append([]byte(nil), region...)
	placed[n], placed[n+1] = x, y
	if !fletcherVerify(placed) {
		t.Errorf("X,Y at n,n+1: fletcherVerify refused % x", placed)
	}

	swapped := append([]byte(nil), region...)
	swapped[n], swapped[n+1] = y, x
	if fletcherVerify(swapped) {
		t.Errorf("Y at n, X at n+1: fletcherVerify accepted % x", swapped)
	}

	shifted := append([]byte(nil), region...)
	shifted[n-1], shifted[n] = x, y
	if fletcherVerify(shifted) {
		t.Errorf("X,Y at n-1,n: fletcherVerify accepted % x", shifted)
	}
}
