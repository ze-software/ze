// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md

package packet

import (
	"encoding/binary"
	"errors"
	"testing"
)

// TestDecodeV3IPv6ChecksumCoversPseudoHeader proves that the IPv6 checksum ze
// verifies is taken over the RFC 8200 pseudo-header and the VRRP message, and
// not over the message alone.
//
// Method: the checksum field of the IPv6 golden is recomputed with refChecksum,
// which shares no code with checksum.go. Positive, a sum over the pseudo-header
// (source, destination, upper-layer length, next header 112) followed by the
// message decodes. Negative, a sum over the message alone, which is how RFC 9568
// Section 5.2.8 computes the IPv4 checksum, is refused with ErrChecksum.
//
// RFC requirement: RFC9568-5.2.8-3 positive -- an IPv6 VRRPv3 advert whose checksum an independent RFC 1071 sum computes over the RFC 8200 pseudo-header (source, destination, upper-layer length, next header 112) and the message decodes (verifyReceived checksum.go, pseudoSumV6 checksum.go).
// RFC requirement: RFC9568-5.2.8-3 negative -- the same advert carrying the sum over the message alone, without the pseudo-header, is discarded with ErrChecksum (verifyReceived checksum.go, Decode validate.go).
func TestDecodeV3IPv6ChecksumCoversPseudoHeader(t *testing.T) {
	meta := metaV3v6(t)
	lookup := lookupConst(VersionV3, 1000)
	source := meta.Src.As16()
	target := meta.Dst.As16()

	withPseudo := mustHex(t, goldenV3v6Hex)
	withPseudo[6], withPseudo[7] = 0, 0
	var lengthAndNext [8]byte
	binary.BigEndian.PutUint32(lengthAndNext[0:4], uint32(len(withPseudo)))
	lengthAndNext[7] = ProtoNumber
	sum := refChecksum(source[:], target[:], lengthAndNext[:], withPseudo)
	binary.BigEndian.PutUint16(withPseudo[6:8], sum)
	if _, err := Decode(withPseudo, meta, lookup); err != nil {
		t.Fatalf("IPv6 checksum over pseudo-header and message: %v, want accepted", err)
	}

	messageOnly := mustHex(t, goldenV3v6Hex)
	messageOnly[6], messageOnly[7] = 0, 0
	binary.BigEndian.PutUint16(messageOnly[6:8], refChecksum(messageOnly))
	if _, err := Decode(messageOnly, meta, lookup); !errors.Is(err, ErrChecksum) {
		t.Fatalf("IPv6 checksum over the message alone: got %v, want ErrChecksum", err)
	}
}
