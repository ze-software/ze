package packet

import (
	"bytes"
	"errors"
	"testing"
)

// refChecksum is an independent straight-line RFC 1071 checksum over the
// concatenation of chunks: 16-bit big-endian words, an odd trailing octet
// padded with zero, end-around carry, then the one's complement. It shares no
// code with checksum.go, so an expected value built from it does not come from
// the code under test.
func refChecksum(chunks ...[]byte) uint16 {
	var all []byte
	for _, c := range chunks {
		all = append(all, c...)
	}
	if len(all)%2 == 1 {
		all = append(all, 0)
	}
	var sum uint32
	for i := 0; i < len(all); i += 2 {
		sum += uint32(all[i])<<8 | uint32(all[i+1])
	}
	for sum>>16 != 0 {
		sum = sum&0xFFFF + sum>>16
	}
	return ^uint16(sum)
}

// TestDecodeV2DiscardsTTLNot255 proves the VRRPv2 receive path discards any
// packet whose TTL is not 255, and keeps one that arrives with 255.
//
// Method: the v2 golden decodes at TTL 255; the same bytes at TTL 0, 1, 64 and
// 254 are each refused with ErrTTL.
//
// RFC requirement: RFC3768-5.2.3-2 positive -- a VRRPv2 advert received with TTL 255 is accepted (Decode validate.go).
// RFC requirement: RFC3768-5.2.3-2 negative -- a VRRPv2 advert received with TTL 0, 1, 64 or 254 is discarded with ErrTTL (Decode validate.go).
func TestDecodeV2DiscardsTTLNot255(t *testing.T) {
	if _, err := Decode(mustHex(t, goldenV2Hex), metaV2(t), lookupConst(VersionV2, 1000)); err != nil {
		t.Fatalf("v2 golden at TTL 255: %v, want accepted", err)
	}
	for _, ttl := range []uint8{0, 1, 64, 254} {
		meta := metaV2(t)
		meta.TTL = ttl
		if _, err := Decode(mustHex(t, goldenV2Hex), meta, lookupConst(VersionV2, 1000)); !errors.Is(err, ErrTTL) {
			t.Errorf("v2 advert at TTL %d: got %v, want ErrTTL", ttl, err)
		}
	}
}

// TestDecodeV2DiscardsUnknownType proves a VRRPv2 packet whose Type is not 1
// (ADVERTISEMENT) is discarded, and a Type 1 packet is kept.
//
// Method: the v2 golden (Type 1) decodes; the golden with Type 0, 2 and 15 and
// a recomputed checksum is each refused with ErrType.
//
// RFC requirement: RFC3768-5.3.2-1 positive -- a VRRPv2 packet of Type 1 (ADVERTISEMENT) is accepted (Decode validate.go).
// RFC requirement: RFC3768-5.3.2-1 negative -- a VRRPv2 packet of Type 0, 2 or 15, with a valid checksum, is discarded with ErrType (Decode validate.go).
func TestDecodeV2DiscardsUnknownType(t *testing.T) {
	meta := metaV2(t)
	if _, err := Decode(mustHex(t, goldenV2Hex), meta, lookupConst(VersionV2, 1000)); err != nil {
		t.Fatalf("v2 Type 1: %v, want accepted", err)
	}
	for _, typ := range []byte{0, 2, 15} {
		b := mustHex(t, goldenV2Hex)
		b[0] = VersionV2<<4 | typ
		reChecksum(b, meta.Src, meta.Dst)
		if _, err := Decode(b, meta, lookupConst(VersionV2, 1000)); !errors.Is(err, ErrType) {
			t.Errorf("v2 Type %d: got %v, want ErrType", typ, err)
		}
	}
}

// TestDecodeV2DiscardsUnknownOrUnconfiguredAuthType proves the VRRPv2 Auth
// Type check. Ze configures no authentication, so its local method is 0 (No
// Authentication): Types 1 and 2 are defined but do not match it, and 3 to 255
// are unknown. Each is discarded; Type 0 is kept.
//
// Method: the v2 golden (Auth Type 0) decodes; the golden with Auth Type 1, 2,
// 3, 128 and 255 and a recomputed checksum is each refused with ErrAuthType.
//
// RFC requirement: RFC3768-5.3.6-1 positive -- a VRRPv2 advert whose Auth Type is 0, the locally configured method, is accepted (Decode validate.go).
// RFC requirement: RFC3768-5.3.6-1 negative -- a VRRPv2 advert whose Auth Type is 1 or 2 (not the local method) or 3, 128 or 255 (unknown) is discarded with ErrAuthType (Decode validate.go).
func TestDecodeV2DiscardsUnknownOrUnconfiguredAuthType(t *testing.T) {
	meta := metaV2(t)
	if _, err := Decode(mustHex(t, goldenV2Hex), meta, lookupConst(VersionV2, 1000)); err != nil {
		t.Fatalf("v2 Auth Type 0: %v, want accepted", err)
	}
	for _, auth := range []byte{1, 2, 3, 128, 255} {
		b := mustHex(t, goldenV2Hex)
		b[4] = auth
		reChecksum(b, meta.Src, meta.Dst)
		if _, err := Decode(b, meta, lookupConst(VersionV2, 1000)); !errors.Is(err, ErrAuthType) {
			t.Errorf("v2 Auth Type %d: got %v, want ErrAuthType", auth, err)
		}
	}
}

// TestDecodeV2DiscardsIncompletePacket proves a VRRPv2 packet that lacks any
// part of the fixed fields, the IP Address(es) or the Authentication Data is
// discarded, and the complete 24-octet packet (Count 2) is kept.
//
// Method: the golden decodes. Cut inside the 8-octet fixed header (4 and 7
// octets): ErrTruncated. Cut only the 8-octet Authentication Data under an
// honest Count 2 (16, 20 and 23 octets, checksum recomputed): ErrLength.
// Count 3 over two addresses plus the trailer (checksum recomputed): ErrLength.
//
// RFC requirement: RFC3768-7.1-2 positive -- the complete VRRPv2 packet, fixed fields plus two IP Addresses plus the 8-octet Authentication Data, is accepted (Decode validate.go).
// RFC requirement: RFC3768-7.1-2 negative -- a VRRPv2 packet cut inside the fixed fields (ErrTruncated), cut in the Authentication Data under an honest Count (ErrLength), or whose Count claims an IP Address that is not present (ErrLength) is discarded (Decode validate.go).
func TestDecodeV2DiscardsIncompletePacket(t *testing.T) {
	meta := metaV2(t)
	lookup := lookupConst(VersionV2, 1000)
	if _, err := Decode(mustHex(t, goldenV2Hex), meta, lookup); err != nil {
		t.Fatalf("complete v2 packet: %v, want accepted", err)
	}
	for _, n := range []int{4, 7} {
		if _, err := Decode(mustHex(t, goldenV2Hex)[:n], meta, lookup); !errors.Is(err, ErrTruncated) {
			t.Errorf("v2 cut to %d octets (fixed fields): got %v, want ErrTruncated", n, err)
		}
	}
	for _, n := range []int{16, 20, 23} {
		b := mustHex(t, goldenV2Hex)[:n]
		reChecksum(b, meta.Src, meta.Dst)
		if _, err := Decode(b, meta, lookup); !errors.Is(err, ErrLength) {
			t.Errorf("v2 cut to %d octets (Authentication Data): got %v, want ErrLength", n, err)
		}
	}
	b := mustHex(t, goldenV2Hex)
	b[3] = 3
	reChecksum(b, meta.Src, meta.Dst)
	if _, err := Decode(b, meta, lookup); !errors.Is(err, ErrLength) {
		t.Errorf("v2 Count 3 over two addresses: got %v, want ErrLength", err)
	}
}

// TestDecodeV3DiscardsIncompletePacket proves a VRRPv3 packet that lacks part
// of the fixed fields or of the IPvX address list is discarded, for both
// families, and the complete packet is kept.
//
// Method: the IPv4 and IPv6 goldens decode. Cut inside the fixed header (7
// octets): ErrTruncated. IPv4 Count 2 carrying one address (12 octets) and
// Count 3 over two addresses (16 octets), checksum recomputed: ErrLength. IPv6
// Count 2 carrying one and a half addresses (32 octets), checksum recomputed:
// ErrLength.
//
// RFC requirement: RFC5798-7.1-2 positive -- a complete VRRPv3 packet, fixed fields plus the whole IPv4 or IPv6 address list, is accepted (Decode validate.go).
// RFC requirement: RFC5798-7.1-2 negative -- a VRRPv3 packet cut inside the fixed fields (ErrTruncated), or whose address list is shorter than Count says, for IPv4 and IPv6 (ErrLength), is discarded (Decode validate.go).
// RFC requirement: RFC9568-7.1-3 positive -- a complete VRRPv3 packet, fixed fields plus the whole IPv4 or IPv6 address list, is accepted (Decode validate.go).
// RFC requirement: RFC9568-7.1-3 negative -- a VRRPv3 packet cut inside the fixed fields (ErrTruncated), or whose address list is shorter than Count says, for IPv4 and IPv6 (ErrLength), is discarded (Decode validate.go).
func TestDecodeV3DiscardsIncompletePacket(t *testing.T) {
	lookup := lookupConst(VersionV3, 1000)
	meta4, meta6 := metaV3v4(t), metaV3v6(t)
	if _, err := Decode(mustHex(t, goldenV3v4CompatHex), meta4, lookup); err != nil {
		t.Fatalf("complete v3 IPv4 packet: %v, want accepted", err)
	}
	if _, err := Decode(mustHex(t, goldenV3v6Hex), meta6, lookup); err != nil {
		t.Fatalf("complete v3 IPv6 packet: %v, want accepted", err)
	}
	if _, err := Decode(mustHex(t, goldenV3v4CompatHex)[:7], meta4, lookup); !errors.Is(err, ErrTruncated) {
		t.Errorf("v3 cut to 7 octets: got %v, want ErrTruncated", err)
	}

	oneAddr := mustHex(t, goldenV3v4CompatHex)[:12]
	reChecksum(oneAddr, meta4.Src, meta4.Dst)
	if _, err := Decode(oneAddr, meta4, lookup); !errors.Is(err, ErrLength) {
		t.Errorf("v3 IPv4 Count 2 carrying one address: got %v, want ErrLength", err)
	}

	countLie := mustHex(t, goldenV3v4CompatHex)
	countLie[3] = 3
	reChecksum(countLie, meta4.Src, meta4.Dst)
	if _, err := Decode(countLie, meta4, lookup); !errors.Is(err, ErrLength) {
		t.Errorf("v3 IPv4 Count 3 over two addresses: got %v, want ErrLength", err)
	}

	halfV6 := mustHex(t, goldenV3v6Hex)[:32]
	reChecksum(halfV6, meta6.Src, meta6.Dst)
	if _, err := Decode(halfV6, meta6, lookup); !errors.Is(err, ErrLength) {
		t.Errorf("v3 IPv6 Count 2 carrying 1.5 addresses: got %v, want ErrLength", err)
	}
}

// TestEncodeV3ReserveZeroOverDirtyBuffer proves the Reserve nibble is written
// as zero, not inherited from the buffer.
//
// Method: WriteTo encodes into a buffer pre-filled with 0xFF, for IPv4 and
// IPv6. The high nibble of octet 4 must be zero and the encoded bytes (checksum
// filled) must equal the goldens.
//
// RFC requirement: RFC5798-5.2.6-1 positive -- WriteTo writes the rsvd nibble as zero on transmission even into a buffer whose octets are all ones, for IPv4 and IPv6 (WriteTo packet.go).
func TestEncodeV3ReserveZeroOverDirtyBuffer(t *testing.T) {
	cases := []struct {
		name   string
		adv    Advertisement
		golden string
	}{
		{"ipv4", advV3v4(t), goldenV3v4CompatHex},
		{"ipv6", advV3v6(t), goldenV3v6Hex},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.Repeat([]byte{0xFF}, MaxLenV3v6)
			n := tc.adv.WriteTo(buf, 0)
			if buf[4]&0xF0 != 0 {
				t.Fatalf("rsvd nibble = %#x over a dirty buffer, want 0", buf[4]&0xF0)
			}
			meta := metaV3v4(t)
			if tc.adv.Family == V6 {
				meta = metaV3v6(t)
			}
			FillChecksum(buf, 0, n, meta.Src, meta.Dst)
			if want := mustHex(t, tc.golden); !bytes.Equal(buf[:n], want) {
				t.Fatalf("encode over dirty buffer:\ngot:  % x\nwant: % x", buf[:n], want)
			}
		})
	}
}

// TestFillChecksumZeroesFieldAndCoversPseudoHeader proves the RFC 5798
// Section 5.2.8 computation: the checksum field is taken as zero while summing,
// and the sum covers the pseudo-header with next header 112.
//
// Method: positive, FillChecksum over the IPv4 and IPv6 goldens whose checksum
// octets hold 0xABCD must reproduce the goldens (a sum that read the stale
// field would differ). The expected IPv6 value is also checked against the
// independent refChecksum over src, dst, upper-layer length, next header 112
// and the message. Negative, the receiver refuses an IPv6 advert checksummed
// with next header 58 instead of 112, and one checksummed over the message
// alone without the pseudo-header, each with ErrChecksum.
//
// RFC requirement: RFC5798-5.2.8-1 positive -- FillChecksum ignores whatever the checksum field holds and produces the golden pseudo-header sum for IPv4 and for IPv6, the IPv6 value matching an independent RFC 1071 sum over the pseudo-header with next header 112 (FillChecksum checksum.go).
// RFC requirement: RFC5798-5.2.8-1 negative -- an IPv6 advert whose checksum was computed with next header 58, or without the pseudo-header, is discarded with ErrChecksum (verifyReceived checksum.go, Decode validate.go).
func TestFillChecksumZeroesFieldAndCoversPseudoHeader(t *testing.T) {
	meta4, meta6 := metaV3v4(t), metaV3v6(t)
	for _, tc := range []struct {
		golden string
		meta   RxMeta
	}{{goldenV3v4CompatHex, meta4}, {goldenV3v6Hex, meta6}} {
		b := mustHex(t, tc.golden)
		b[6], b[7] = 0xAB, 0xCD
		FillChecksum(b, 0, len(b), tc.meta.Src, tc.meta.Dst)
		if want := mustHex(t, tc.golden); !bytes.Equal(b, want) {
			t.Errorf("FillChecksum over a stale field:\ngot:  % x\nwant: % x", b, want)
		}
	}

	msg := mustHex(t, goldenV3v6Hex)
	msg[6], msg[7] = 0, 0
	src, dst := meta6.Src.As16(), meta6.Dst.As16()
	pseudo := func(nextHeader byte) []byte {
		p := make([]byte, 0, 40)
		p = append(p, src[:]...)
		p = append(p, dst[:]...)
		return append(p, 0, 0, 0, byte(len(msg)), 0, 0, 0, nextHeader)
	}
	want := mustHex(t, goldenV3v6Hex)
	if got := refChecksum(pseudo(ProtoNumber), msg); got != uint16(want[6])<<8|uint16(want[7]) {
		t.Fatalf("golden IPv6 checksum %#02x%02x is not the next-header-112 pseudo-header sum %#04x", want[6], want[7], got)
	}

	for name, sum := range map[string]uint16{
		"next header 58":   refChecksum(pseudo(58), msg),
		"no pseudo-header": refChecksum(msg),
	} {
		b := bytes.Clone(msg)
		b[6], b[7] = byte(sum>>8), byte(sum)
		if _, err := Decode(b, meta6, lookupConst(VersionV3, 1000)); !errors.Is(err, ErrChecksum) {
			t.Errorf("IPv6 advert checksummed with %s: got %v, want ErrChecksum", name, err)
		}
	}
}

// TestDecodeVerifiesChecksumBothFamilies proves the receiver verifies the
// VRRPv3 checksum for IPv4 and for IPv6.
//
// Method: positive, the IPv4 advert carrying the RFC 9568 Section 5.2.8
// message-only sum decodes (flagged MsgOnlyChecksum), and so does the IPv6
// golden. Negative, the IPv4 advert with a checksum that verifies under
// neither form, and the IPv6 golden with one checksum bit flipped, are each
// refused with ErrChecksum.
//
// RFC requirement: RFC9568-5.2.8-1 positive -- an IPv4 advert carrying the RFC 9568 message-only checksum, and an IPv6 advert carrying the RFC 8200 pseudo-header checksum, each verify and decode (verifyReceived checksum.go, Decode validate.go).
// RFC requirement: RFC9568-5.2.8-1 negative -- an IPv4 advert whose checksum verifies under no accepted form, and an IPv6 advert with one checksum bit flipped, are each discarded with ErrChecksum (verifyReceived checksum.go, Decode validate.go).
func TestDecodeVerifiesChecksumBothFamilies(t *testing.T) {
	lookup := lookupConst(VersionV3, 1000)
	adv, err := Decode(mustHex(t, goldenV3v4Hex), metaV3v4(t), lookup)
	if err != nil {
		t.Fatalf("IPv4 message-only checksum: %v, want accepted", err)
	}
	if !adv.MsgOnlyChecksum {
		t.Fatalf("IPv4 message-only checksum accepted but not flagged")
	}
	if _, err := Decode(mustHex(t, goldenV3v6Hex), metaV3v6(t), lookup); err != nil {
		t.Fatalf("IPv6 pseudo-header checksum: %v, want accepted", err)
	}

	bad4 := mustHex(t, goldenV3v4Hex)
	bad4[7] ^= 0x01
	if _, err := Decode(bad4, metaV3v4(t), lookup); !errors.Is(err, ErrChecksum) {
		t.Errorf("IPv4 checksum verifying under no form: got %v, want ErrChecksum", err)
	}
	bad6 := mustHex(t, goldenV3v6Hex)
	bad6[7] ^= 0x01
	if _, err := Decode(bad6, metaV3v6(t), lookup); !errors.Is(err, ErrChecksum) {
		t.Errorf("IPv6 checksum bit flipped: got %v, want ErrChecksum", err)
	}
}
