package packet

import (
	"net/netip"
	"testing"
)

// referenceChecksum is an independent, straight-line RFC 1071 implementation
// used to cross-check the package's folded-32-bit variant (R-3). It must agree
// with checksum16 on every input, including odd-length data.
func referenceChecksum(data []byte, initial uint32) uint16 {
	sum := initial
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}

// VALIDATES: the package RFC 1071 sum/fold/complement agrees with an independent
// reference on even- and odd-length inputs, and that summing a packet including
// its own correct checksum folds to 0xFFFF (the rx verification invariant).
// PREVENTS: carry-fold bugs and odd-byte-padding bugs (R-3).
//
// RFC requirement: RFC1071-1-2 positive -- checksum16's 16-bit one's-complement sum with end-around carry matches an independent straight-line RFC 1071 reference on even- and odd-length inputs (onesComplementSum + fold, checksum.go:19-38).
func TestChecksumRFC1071(t *testing.T) {
	inputs := [][]byte{
		{},
		{0x00},
		{0xFF},
		{0x12, 0x34},
		{0x12, 0x34, 0x56},
		{0xDE, 0xAD, 0xBE, 0xEF, 0x01},
		mustHex(t, goldenV2Hex),
		mustHex(t, goldenV3v4Hex),
	}
	for i, in := range inputs {
		got := checksum16(in, 0)
		want := referenceChecksum(in, 0)
		if got != want {
			t.Fatalf("case %d: checksum16=%#04x, reference=%#04x", i, got, want)
		}
		// Summing data plus its own complement folds to all-ones. The checksum
		// field must sit on a 16-bit boundary (as it does in a real VRRP
		// message: even length, checksum at byte 6), so pad an odd input with
		// the same implicit zero byte the checksum computation used.
		full := append([]byte{}, in...)
		if len(full)%2 == 1 {
			full = append(full, 0)
		}
		full = append(full, byte(got>>8), byte(got))
		if !verifyChecksumSum(full, 0) {
			t.Fatalf("case %d: verify of data+checksum did not fold to 0xFFFF", i)
		}
	}
}

// VALIDATES: FillChecksum backfills the message-only sum for v2, the RFC 5798
// IPv4 pseudo-header sum for v3/IPv4 (the interoperable tx form), and the RFC
// 8200 pseudo-header sum for v3/IPv6, matching the golden vectors.
// PREVENTS: v3/IPv4 regressing to message-only (which keepalived rejects), the
// v6 pseudo-header being dropped, or a pseudo-header leaking into v2.
func TestFillChecksumFamilies(t *testing.T) {
	cases := []struct {
		name   string
		golden string
		src    string
		dst    netip.Addr
		maxLen int
	}{
		{"v2", goldenV2Hex, "192.0.2.251", MulticastV4, MaxLenV2},
		// v3/IPv4 tx is the pseudo-header form (G2c); 0xDEFB is its sum for
		// src 192.0.2.251 -> dst 224.0.0.18 (checksum.go FillChecksum).
		{"v3v4", goldenV3v4CompatHex, "192.0.2.251", MulticastV4, MaxLenV3v4},
		{"v3v6", goldenV3v6Hex, "fe80::c8", MulticastV6, MaxLenV3v6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := mustHex(t, tc.golden)
			buf := make([]byte, tc.maxLen)
			copy(buf, want)
			buf[6], buf[7] = 0, 0 // clear checksum then recompute
			FillChecksum(buf, 0, len(want), addr(t, tc.src), tc.dst)
			if buf[6] != want[6] || buf[7] != want[7] {
				t.Fatalf("checksum: got %02x%02x, want %02x%02x", buf[6], buf[7], want[6], want[7])
			}
		})
	}
}
