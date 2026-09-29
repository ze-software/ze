// Design: docs/architecture/wire/isis.md -- ISO 8473 Fletcher checksum arithmetic
// Related: checksum.go -- Checksum and VerifyChecksum, the producers under test
// Related: rfc905_checksum_test.go -- the closed-form X, Y and the verification rule

package packet

import "testing"

// senderChecksum is a test-local generator that follows RFC 905 Annex B.3 except
// where told not to: modulus picks the arithmetic, and c1First swaps the two
// steps of B.3.3 so C1 takes C0 before C0 takes the octet. It stands for a
// non-compliant sender whose output a compliant receiver must refuse.
func senderChecksum(data []byte, checkOff, modulus int, c1First bool) (x, y byte) {
	c0, c1 := 0, 0
	for i, octet := range data {
		b := int(octet)
		if i == checkOff || i == checkOff+1 {
			b = 0
		}
		if c1First {
			c1 = (c1 + c0) % modulus
			c0 = (c0 + b) % modulus
			continue
		}
		c0 = (c0 + b) % modulus
		c1 = (c1 + c0) % modulus
	}
	m := len(data) - checkOff
	xv := (((m-1)*c0-c1)%modulus + modulus) % modulus
	yv := ((c1-m*c0)%modulus + modulus) % modulus
	return byte(xv), byte(yv)
}

// rfc905Region returns a 24-octet region with its checksum field at checkOff 12,
// the offset of an LSP's checksum inside its checksummed region.
func rfc905Region() []byte {
	region := make([]byte, 24)
	for i := range region {
		region[i] = byte(0xA0 + 7*i)
	}
	return region
}

// VALIDATES: RFC 905 Annex B.2 mode (b): an octet or a running sum of 255 is read
// as 0. A region of 0xFF octets verifies, and a region whose generated checksum
// octet is 255 (Checksum stores a computed 0 as 255) verifies.
// PREVENTS: a verifier that keeps minus zero (255) distinct from plus zero (0).
//
// RFC requirement: RFC905-x-1 positive -- VerifyChecksum accepts a region whose C0 and C1 reach 255 (all octets 0xFF), and a region where Checksum stored 255 for a computed zero octet.
func TestRFC905MinusZeroReadAsZero(t *testing.T) {
	ones := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	if !VerifyChecksum(ones) {
		t.Fatalf("region %x of minus-zero octets refused", ones)
	}

	found := false
	for seed := range 255 {
		region := rfc905Region()
		region[0] = byte(seed)
		high, low := Checksum(region, 12)
		if high != 0xFF && low != 0xFF {
			continue
		}
		found = true
		region[12], region[13] = high, low
		if !VerifyChecksum(region) {
			t.Fatalf("seed %d: checksum %02x%02x holding minus zero refused", seed, high, low)
		}
	}
	if !found {
		t.Fatal("no seed produced a checksum octet of 255; the vector does not reach the clause")
	}
}

// VALIDATES: RFC 905 Annex B.2 and B.3: a receiver refuses a checksum that a
// sender computed with the wrong arithmetic (modulo 256) or with the two steps of
// B.3.3 in the wrong order, while the compliant checksum of the same region is
// accepted.
// PREVENTS: a verifier whose arithmetic or step order matches the faulty sender.
// Method: senderChecksum builds each faulty X, Y; the test first requires that it
// differs from Checksum's, so the vector discriminates, then that VerifyChecksum
// refuses it.
//
// RFC requirement: RFC905-x-1 negative -- a checksum computed modulo 256 instead of modulo 255 is refused by VerifyChecksum.
// RFC requirement: RFC905-x-2 negative -- a checksum computed with C1 += C0 before C0 += octet is refused by VerifyChecksum, and differs from the one Checksum computes.
func TestRFC905FaultySenderChecksumRefused(t *testing.T) {
	cases := []struct {
		name    string
		modulus int
		c1First bool
	}{
		{"modulo-256", 256, false},
		{"c1-before-c0", 255, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			region := rfc905Region()
			high, low := Checksum(region, 12)
			x, y := senderChecksum(region, 12, tc.modulus, tc.c1First)
			if x == high && y == low {
				t.Fatalf("faulty sender produced the compliant checksum %02x%02x; pick another region", x, y)
			}
			region[12], region[13] = high, low
			if !VerifyChecksum(region) {
				t.Fatalf("compliant checksum %02x%02x refused", high, low)
			}
			region[12], region[13] = x, y
			if VerifyChecksum(region) {
				t.Fatalf("faulty checksum %02x%02x accepted", x, y)
			}
		})
	}
}
