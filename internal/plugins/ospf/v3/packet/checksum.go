// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 LSA Fletcher and IPv6 packet checksums.
// RFC: rfc/short/rfc5340.md (§A.3.1 IPv6 upper-layer packet checksum, §A.4.2.1 LS checksum)
//
// The LSA Fletcher checksum is byte-identical to OSPFv2 (RFC 2328 §12.1.7) but is
// owned here under ospfv3/packet per the no-cross-version rule (the OSPFv3 codec
// imports no OSPFv2 code). The packet checksum, however, diverges: OSPFv3 uses
// the IPv6 upper-layer checksum (RFC 8200 §8.1 / RFC 1071) over the IPv6
// pseudo-header plus the OSPF packet, so it binds the IPv6 source and
// destination supplied by transport.

package packet

import "net/netip"

const (
	fletcherModulus = 255
	// lsaChecksumOffsetInCoveredRegion is the LS Checksum field's offset inside
	// the covered window. The window starts at LSA offset 2 (LS Age excluded) and
	// the checksum field is at LSA offset 16, so 16-2 = 14.
	lsaChecksumOffsetInCoveredRegion = 14
	// minLSAChecksumRegionLen is the smallest covered window (LSA header minus the
	// 2 LS Age octets).
	minLSAChecksumRegionLen = LSAHeaderLen - 2
	// ipv6PseudoHeaderLen is the OSPFv3 pseudo-header: src(16) + dst(16) +
	// upper-layer-length(4) + zeros(3) + Next Header(1).
	ipv6PseudoHeaderLen = 40
	// nextHeaderOSPF is the IANA protocol number for OSPF (RFC 5340 §1: proto 89).
	nextHeaderOSPF = 89
)

// FinalizeLSAChecksum computes and backfills the OSPFv3 LSA Fletcher checksum
// over lsa[2:length] (LS Age excluded), where length is the LSA header's Length
// field -- which the caller MUST have written before calling this. Bounding the
// coverage by Length (rather than len(lsa)) keeps it identical to
// VerifyLSAChecksum even if the caller passes an over-long buffer. The LSA
// checksum field must already be zeroed. RFC 5340 §A.4.2.1 keeps the OSPFv2 (RFC
// 2328 §12.1.7) Fletcher checksum unchanged. The result is non-zero by construction.
func FinalizeLSAChecksum(lsa []byte) uint16 {
	if len(lsa) < LSAHeaderLen {
		return 0
	}
	length := int(readUint16(lsa, lsaLengthOff))
	if length < LSAHeaderLen || length > len(lsa) {
		return 0
	}
	checksum := fletcherChecksum(lsa[2:length])
	writeUint16(lsa, lsaChecksumOff, checksum)
	return checksum
}

// VerifyLSAChecksum verifies the Fletcher checksum over lsa[2:length].
func VerifyLSAChecksum(lsa []byte) bool {
	if len(lsa) < LSAHeaderLen {
		return false
	}
	length := int(readUint16(lsa, lsaLengthOff))
	if length < LSAHeaderLen || length > len(lsa) {
		return false
	}
	return fletcherVerifyNonZero(lsa[2:length])
}

// fletcherChecksum computes the OSPFv3 LSA Fletcher-16 checksum over a covered
// window (the LSA bytes starting at LS Type, with LS Age already excluded). The
// checksum field is treated as zero during generation, then two octets are
// chosen so the running sums over the final region are both zero (RFC 905
// Annex B). The two chosen octets are normalized so neither is zero, which makes
// the 16-bit result non-zero.
func fletcherChecksum(data []byte) uint16 {
	if len(data) < minLSAChecksumRegionLen {
		return 0
	}
	high, low := fletcherGenerate(data, lsaChecksumOffsetInCoveredRegion)
	return uint16(high)<<8 | uint16(low)
}

// fletcherVerifyNonZero reports whether data carries a valid non-zero LSA
// checksum.
func fletcherVerifyNonZero(data []byte) bool {
	if len(data) < minLSAChecksumRegionLen {
		return false
	}
	if data[lsaChecksumOffsetInCoveredRegion] == 0 && data[lsaChecksumOffsetInCoveredRegion+1] == 0 {
		return false
	}
	var c0, c1 int
	for _, b := range data {
		c0 = (c0 + int(b)) % fletcherModulus
		c1 = (c1 + c0) % fletcherModulus
	}
	return c0 == 0 && c1 == 0
}

func fletcherGenerate(data []byte, checkOff int) (byte, byte) {
	if len(data) == 0 || checkOff < 0 || checkOff+1 >= len(data) {
		return 0, 0
	}
	var c0, c1 int
	for i := range data {
		b := int(data[i])
		if i == checkOff || i == checkOff+1 {
			b = 0
		}
		c0 = (c0 + b) % fletcherModulus
		c1 = (c1 + c0) % fletcherModulus
	}
	m := len(data) - checkOff
	x := subMod(mulMod(m-1, c0), c1)
	y := subMod(c1, mulMod(m, c0))
	return normalizeFletcher(x), normalizeFletcher(y)
}

func mulMod(a, b int) int {
	r := (a * b) % fletcherModulus
	if r < 0 {
		r += fletcherModulus
	}
	return r
}

func subMod(a, b int) int {
	r := (a - b) % fletcherModulus
	if r < 0 {
		r += fletcherModulus
	}
	return r
}

func normalizeFletcher(v int) byte {
	v %= fletcherModulus
	if v < 0 {
		v += fletcherModulus
	}
	if v == 0 {
		return fletcherModulus
	}
	return byte(v)
}

// PacketChecksum computes the OSPFv3 packet checksum: the IPv6 upper-layer
// checksum (RFC 1071 one's-complement) over the IPv6 pseudo-header (src, dst,
// 32-bit upper-layer length = len(pkt), three zero octets, Next Header 89)
// followed by the OSPF packet with the checksum field zeroed. Transport supplies
// src and dst; the codec cannot derive them from the packet bytes alone
// (RFC 5340 §A.3.1).
func PacketChecksum(src, dst netip.Addr, pkt []byte) uint16 {
	if len(pkt) < CommonHeaderLen {
		return 0
	}
	var ph [ipv6PseudoHeaderLen]byte
	buildPseudoHeader(ph[:], src, dst, len(pkt))
	sum := internetSum(ph[:]) + internetSumZeroAt(pkt, offChecksum)
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// FinalizePacketChecksum computes the OSPFv3 packet checksum for pkt given the
// IPv6 source and destination and writes it into the checksum field at
// offChecksum, returning the value. PacketChecksum already excludes the checksum
// field from its own sum (internetSumZeroAt), so the field's prior contents do
// not matter; Packet.WriteTo conventionally leaves it zero. The transport calls
// this on TX once it knows the egress link-local source and the destination
// (RFC 5340 §A.3.1: the checksum binds the IPv6 pseudo-header). When an RFC 7166
// Authentication Trailer is appended the checksum is left zero instead (RFC 7166
// §2.2), so this is called only when no trailer is used.
func FinalizePacketChecksum(src, dst netip.Addr, pkt []byte) uint16 {
	if len(pkt) < CommonHeaderLen {
		return 0
	}
	sum := PacketChecksum(src, dst, pkt)
	writeUint16(pkt, offChecksum, sum)
	return sum
}

// VerifyPacketChecksum verifies the OSPFv3 packet checksum. A spoofed source or
// destination changes the pseudo-header and fails verification (RFC 5340 §A.3.1).
func VerifyPacketChecksum(src, dst netip.Addr, pkt []byte) bool {
	if len(pkt) < CommonHeaderLen {
		return false
	}
	var ph [ipv6PseudoHeaderLen]byte
	buildPseudoHeader(ph[:], src, dst, len(pkt))
	// The packet checksum field is included as-is here (not zeroed): a correct
	// checksum makes the total one's-complement sum all-ones.
	sum := internetSum(ph[:]) + internetSum(pkt)
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(sum) == 0xffff
}

// buildPseudoHeader lays the 40-octet IPv6 upper-layer pseudo-header into ph
// (RFC 8200 §8.1): src(16), dst(16), upper-layer packet length(4), zero(3),
// Next Header(1). A non-IPv6 address contributes all-zero octets, which makes a
// mismatched-family verification fail.
func buildPseudoHeader(ph []byte, src, dst netip.Addr, upperLen int) {
	for i := range ph {
		ph[i] = 0
	}
	if src.Is6() {
		s := src.As16()
		copy(ph[0:16], s[:])
	}
	if dst.Is6() {
		d := dst.As16()
		copy(ph[16:32], d[:])
	}
	writeUint32(ph, 32, uint32(upperLen))
	ph[39] = nextHeaderOSPF
}

// internetSum adds 16-bit big-endian words, padding an odd final octet.
func internetSum(data []byte) uint32 {
	var sum uint32
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	return sum
}

// internetSumZeroAt sums data with the two octets at zeroOff treated as zero
// (the checksum field is excluded from its own computation) without copying.
func internetSumZeroAt(data []byte, zeroOff int) uint32 {
	var sum uint32
	for i := 0; i+1 < len(data); i += 2 {
		hi := uint32(data[i])
		lo := uint32(data[i+1])
		if i == zeroOff {
			hi, lo = 0, 0
		}
		sum += hi<<8 | lo
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	return sum
}
