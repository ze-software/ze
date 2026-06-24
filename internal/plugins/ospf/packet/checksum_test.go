// Design: plan/learned/956-ospf-2-wire.md -- checksum covered-range tests

package packet

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// VALIDATES: AC-3 - packet checksum is backfilled and verifies over packet minus auth field.
// PREVENTS: peers rejecting every non-authenticated OSPF packet.
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
func TestOSPFLSAChecksum(t *testing.T) {
	wire := encodeLSA(t, sampleRouterLSA(t))
	if readUint16(wire, lsaChecksumOff) == 0 {
		t.Fatalf("LSA checksum was not backfilled")
	}
	if !VerifyLSAChecksum(wire) {
		t.Fatalf("VerifyLSAChecksum rejected encoded LSA % x", wire)
	}
}

// VALIDATES: AC-5 - LS Age bytes are excluded from LSA Fletcher coverage.
// PREVENTS: ordinary age increments invalidating flooded LSAs.
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
