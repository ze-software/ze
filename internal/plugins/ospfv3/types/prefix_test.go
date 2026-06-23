// VALIDATES: spec-ospfv3-1-types AC-7/AC-8 -- PrefixLength byte/word length matches the
// RFC 5340 padded-word rule across /0../128, 129 is rejected, and padding validation
// rejects non-zero bits after the prefix length; PrefixOptions predicates decode.
// PREVENTS: prefix padding bugs that self-test OK but fail FRR (R-3).
package types

import "testing"

func TestOSPFv3PrefixEncodingBoundaries(t *testing.T) {
	cases := []struct {
		bits    uint8
		byteLen int
		wordLen int
	}{
		{0, 0, 0}, {1, 4, 1}, {31, 4, 1}, {32, 4, 1},
		{33, 8, 2}, {64, 8, 2}, {127, 16, 4}, {128, 16, 4},
	}
	for _, c := range cases {
		p, err := NewPrefixLength(c.bits)
		if err != nil {
			t.Fatalf("NewPrefixLength(%d): %v", c.bits, err)
		}
		if p.ByteLen() != c.byteLen {
			t.Errorf("/%d ByteLen = %d, want %d", c.bits, p.ByteLen(), c.byteLen)
		}
		if p.wordLen() != c.wordLen {
			t.Errorf("/%d wordLen = %d, want %d", c.bits, p.wordLen(), c.wordLen)
		}
	}
	if _, err := NewPrefixLength(129); err == nil {
		t.Error("NewPrefixLength(129) accepted")
	}

	// Padding: /1 needs bit 0 set is fine; any of bits 1..31 set must be rejected.
	p1, _ := NewPrefixLength(1)
	if err := p1.ValidatePadding([]byte{0x80, 0x00, 0x00, 0x00}); err != nil {
		t.Errorf("clean /1 padding rejected: %v", err)
	}
	if err := p1.ValidatePadding([]byte{0x80, 0x00, 0x00, 0x01}); err == nil {
		t.Error("non-zero /1 padding accepted")
	}
	if err := p1.ValidatePadding([]byte{0x80, 0x00}); err == nil {
		t.Error("short /1 prefix bytes accepted")
	}
	// /128 has no padding word -- a full 16-byte address is fine.
	p128, _ := NewPrefixLength(128)
	if err := p128.ValidatePadding(make([]byte, 16)); err != nil {
		t.Errorf("/128 full address rejected: %v", err)
	}

	// PrefixOptions bits: NU (0x01), LA (0x02), P (0x08), DN (0x10).
	po := OptPrefixNU | OptPrefixP
	if !po.NoUnicast() || !po.Propagate() {
		t.Errorf("prefix option predicates: %#02x", uint8(po))
	}
	if po.LocalAddress() || po.Down() {
		t.Error("unexpected prefix option bits set")
	}
}
