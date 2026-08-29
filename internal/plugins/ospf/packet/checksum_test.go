// Design: docs/architecture/ospf/ospf-2-wire.md -- checksum covered-range tests

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: AC-3 - packet checksum is backfilled and verifies over packet minus auth field.
// PREVENTS: peers rejecting every non-authenticated OSPF packet.
//
// RFC requirement: RFC1071-1-1 positive -- WriteTo zeroes the Checksum field then stores the complemented PacketChecksum; the round-trip (backfilled non-zero, VerifyPacketChecksum accepts) proves the RFC 1071 generate procedure (header.go:301,322-323, PacketChecksum checksum.go:13-17).
func TestOSPFPacketChecksum(t *testing.T) {
	hello := sampleHello(t)
	p := Packet{Header: sampleHeader(t, PacketTypeHello), Hello: &hello}
	buf := encodePacket(t, p)
	if readUint16(buf, offChecksum) == 0 {
		t.Fatalf("checksum was not backfilled")
	}
	if !VerifyPacketChecksum(buf) {
		t.Fatalf("VerifyPacketChecksum rejected encoded packet % x", buf)
	}
}

// VALIDATES: AC-3 - the 8-byte Authentication field is outside packet-checksum coverage.
// PREVENTS: simple/auth data changes invalidating an otherwise correct packet checksum.
//
// RFC requirement: RFC1071-x-1 positive -- mutating the excluded 8-byte Auth field leaves the packet checksum valid, proving coverage excludes buf[16:24] (PacketChecksum two-segment sum, checksum.go:13-17).
// RFC requirement: RFC1071-x-1 negative -- mutating a covered body byte makes VerifyPacketChecksum reject the packet (VerifyPacketChecksum, checksum.go:23-32).
// RFC requirement: RFC2328-A.3.1-2 positive -- the packet header checksum is computed over the whole packet EXCLUDING the 64-bit Authentication field: mutating buf[16:24] leaves it valid while mutating a covered body octet invalidates it (PacketChecksum two-segment sum, checksum.go:13-18).
func TestOSPFPacketChecksumExcludesAuth(t *testing.T) {
	h := sampleHeader(t, PacketTypeHello)
	h.AuType = AuTypeSimple
	copy(h.Auth[:], "password")
	hello := sampleHello(t)
	buf := encodePacket(t, Packet{Header: h, Hello: &hello})
	mutAuth := append([]byte(nil), buf...)
	mutAuth[offAuth] ^= 0xff
	mutAuth[offAuth+7] ^= 0xff
	if !VerifyPacketChecksum(mutAuth) {
		t.Fatalf("auth-field mutation changed packet checksum coverage")
	}
	mutBody := append([]byte(nil), buf...)
	mutBody[len(mutBody)-1] ^= 0xff
	if VerifyPacketChecksum(mutBody) {
		t.Fatalf("body mutation did not invalidate packet checksum")
	}
}

// VALIDATES: AC-4 - AuType 2 leaves the packet checksum field zero.
// PREVENTS: ospf-12 signing packets that peers reject because Checksum is non-zero.
//
// RFC requirement: RFC1071-x-2 positive -- an AuType2 packet leaves the Checksum field zero and VerifyPacketChecksum accepts the zero checksum (WriteTo header.go:317-321, VerifyPacketChecksum checksum.go:28-30).
func TestOSPFPacketChecksumZeroForAuType2(t *testing.T) {
	h := sampleHeader(t, PacketTypeHello)
	h.AuType = AuTypeCryptographic
	h.Auth = AuthField{0, 0, 1, 16, 0, 0, 0, 1}
	hello := sampleHello(t)
	buf := encodePacket(t, Packet{Header: h, Hello: &hello})
	if got := readUint16(buf, offChecksum); got != 0 {
		t.Fatalf("AuType2 checksum = %#04x, want 0", got)
	}
	if !VerifyPacketChecksum(buf) {
		t.Fatalf("VerifyPacketChecksum should accept AuType2 zero checksum before crypto")
	}
}

// VALIDATES: AC-5 - LSA Fletcher is backfilled and verifies over LSA minus LS Age.
// PREVENTS: peers rejecting LSAs due to wrong Fletcher application.
//
// RFC requirement: RFC905-x-6 positive -- LSA Fletcher applied over lsa[2:] (LS Age excluded) and verifies (FinalizeLSAChecksum/VerifyLSAChecksum, checksum.go:37-56).
// RFC requirement: RFC905-x-7 positive -- encode (FinalizeLSAChecksum backfills X,Y) then decode (VerifyLSAChecksum accepts) round trip (checksum.go:37-56).
// RFC requirement: RFC2328-12.1.7-1 positive -- the LS (Fletcher) checksum is computed over the complete LSA excluding LS Age, backfilled non-zero into the LS Checksum field, and verifies (FinalizeLSAChecksum/VerifyLSAChecksum, checksum.go:37-56).
func TestOSPFLSAChecksum(t *testing.T) {
	wire := encodeLSA(t, sampleRouterLSA(t))
	if readUint16(wire, lsaChecksumOff) == 0 {
		t.Fatalf("LSA checksum was not backfilled")
	}
	if !VerifyLSAChecksum(wire) {
		t.Fatalf("VerifyLSAChecksum rejected encoded LSA % x", wire)
	}

	// RFC requirement: RFC2328-12.1.7-1 positive -- the backfilled octets satisfy the RFC 905
	// Annex B property (both Fletcher sums mod 255 zero over the covered region), judged by
	// fletcherSums in the test rather than by VerifyLSAChecksum, which shares fletcherModulus
	// with the generator (types/checksum.go).
	length := int(readUint16(wire, lsaLengthOff))
	c0, c1 := fletcherSums(wire[2:length])
	if c0 != 0 || c1 != 0 {
		t.Fatalf("independent Fletcher sums over the covered region = (%d, %d), want (0, 0)", c0, c1)
	}
}

// VALIDATES: AC-5 - LS Age bytes are excluded from LSA Fletcher coverage.
// PREVENTS: ordinary age increments invalidating flooded LSAs.
//
// RFC requirement: RFC905-x-6 negative -- mutating a covered octet (Options) makes VerifyLSAChecksum reject the LSA (VerifyLSAChecksum, checksum.go:47-56).
// RFC requirement: RFC905-x-4 negative -- the verification re-sum rejects a corrupted covered region (FletcherVerify, types/checksum.go:61-68).
// RFC requirement: RFC905-x-7 negative -- decode rejects a wrong vector, guarding against the encode-correct/verify-always-true bug (VerifyLSAChecksum, checksum.go:47-56).
// RFC requirement: RFC2328-12.1.7-1 positive -- the covered region starts after LS Age: mutating the two LS Age octets leaves the Fletcher checksum valid, while mutating a covered octet invalidates it (FletcherChecksum over lsa[2:], checksum.go:41-55).
func TestOSPFLSAChecksumExcludesAge(t *testing.T) {
	wire := encodeLSA(t, sampleRouterLSA(t))
	mutAge := append([]byte(nil), wire...)
	mutAge[0] ^= 0xff
	mutAge[1] ^= 0xff
	if !VerifyLSAChecksum(mutAge) {
		t.Fatalf("LS Age mutation changed Fletcher coverage")
	}
	mutCovered := append([]byte(nil), wire...)
	mutCovered[lsaOptionsOff] ^= byte(types.OptionDN)
	if VerifyLSAChecksum(mutCovered) {
		t.Fatalf("covered byte mutation did not invalidate LSA checksum")
	}
}
