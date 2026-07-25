// Design: plan/learned/956-ospf-2-wire.md -- packet and LSA checksum application
// RFC 2328 Appendix A.3.1: packet checksum excludes Authentication bytes.
// RFC 2328 Section 12.1.7: LSA checksum excludes LS Age.

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/types"

// PacketChecksum computes the OSPFv2 packet checksum over buf with bytes 12..13
// already zeroed and bytes 16..23 excluded from coverage. The auth exclusion is
// implemented as a two-segment RFC 1071 checksum in the types package, avoiding a
// temporary concatenated buffer.
func PacketChecksum(buf []byte) uint16 {
	if len(buf) < CommonHeaderLen {
		return 0
	}
	return types.InternetChecksumPair(buf[:offAuth], buf[offAuth+AuthFieldLen:])
}

// VerifyPacketChecksum verifies the packet checksum. For AuType 2 and 3 the
// common-header checksum is intentionally zero and authentication verifies the
// digest later (ospf-12).
func VerifyPacketChecksum(buf []byte) bool {
	h, _, err := DecodeHeader(buf)
	if err != nil {
		return false
	}
	if h.AuType == AuTypeCryptographic || h.AuType == AuTypeCryptographicESN {
		return h.Checksum == 0
	}
	wire := buf[:h.Length]
	return types.InternetChecksumPairValid(wire[:offAuth], wire[offAuth+AuthFieldLen:])
}

// FinalizeLSAChecksum computes and backfills the OSPF LSA Fletcher checksum over
// lsa[2:], excluding LS Age. The LSA checksum field must already be zeroed.
func FinalizeLSAChecksum(lsa []byte) uint16 {
	if len(lsa) < types.LSAHeaderLen {
		return 0
	}
	checksum := types.FletcherChecksum(lsa[2:])
	writeUint16(lsa, lsaChecksumOff, checksum)
	return checksum
}

// VerifyLSAChecksum verifies the Fletcher checksum over lsa[2:].
func VerifyLSAChecksum(lsa []byte) bool {
	if len(lsa) < types.LSAHeaderLen {
		return false
	}
	length := int(readUint16(lsa, lsaLengthOff))
	if length < types.LSAHeaderLen || length > len(lsa) {
		return false
	}
	return types.FletcherVerify(lsa[2:length])
}
