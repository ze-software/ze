// RFC: rfc/short/rfc9568.md -- Section 8.1.2 (gratuitous ARP on Master transition)
// RFC: errata 7947/7949 -- gratuitous ARP target link-layer = Virtual Router MAC
// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- pure GARP frame builder (testable on darwin)
//
// garp.go builds the gratuitous-ARP Ethernet frame VRRP broadcasts when a router
// becomes Master, so bridges relearn the virtual MAC's port and hosts repoint
// their ARP caches at the new Master. The frame is built into a caller-provided
// buffer (buffer-first, ai/rules/performance.md); the AF_PACKET sender
// (garp_linux.go) transmits it verbatim. This file has no build tag so the
// golden-byte test runs on the native `make ze-verify` host (umbrella R-5).

package transport

// GARPFrameLen is the exact on-wire length of the gratuitous-ARP frame: a 14-byte
// Ethernet header plus a 28-byte ARP body (RFC 826).
const GARPFrameLen = 42

// broadcastMAC is the Ethernet broadcast destination for a gratuitous ARP.
var broadcastMAC = [6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// BuildGARP writes the 42-byte gratuitous-ARP request that announces vip owned by
// virtualMAC into buf and returns the number of bytes written (GARPFrameLen). The
// caller MUST provide a buffer of at least GARPFrameLen bytes; BuildGARP indexes
// directly (buffer-first). Both the Ethernet source and the ARP sha/tha carry the
// Virtual Router MAC:
//
//	RFC 9568 Section 7.3: the source MAC MUST be the Virtual Router MAC so bridges
//	learn the virtual MAC on the new Master's port.
//	RFC 9568 errata 7947/7949: the gratuitous ARP carries the virtual IPv4 address,
//	with the Virtual Router MAC as the Target Link-Layer address (sha == tha ==
//	virtual MAC; supersedes the earlier zero-tha reading, orchestrator D-E).
func BuildGARP(buf []byte, virtualMAC [6]byte, vip [4]byte) int {
	// Ethernet header: broadcast destination, virtual-MAC source, EtherType ARP
	// (0x0806, RFC 7042 Section 2.3.1).
	copy(buf[0:6], broadcastMAC[:])
	copy(buf[6:12], virtualMAC[:])
	buf[12] = 0x08
	buf[13] = 0x06

	// ARP body (RFC 826): htype 1 (Ethernet), ptype 0x0800 (IPv4), hlen 6, plen 4.
	buf[14] = 0x00
	buf[15] = 0x01
	buf[16] = 0x08
	buf[17] = 0x00
	buf[18] = 0x06
	buf[19] = 0x04
	// oper 1 (request): a gratuitous ARP is a request for the sender's own address.
	buf[20] = 0x00
	buf[21] = 0x01

	// sha = Virtual Router MAC; spa = VIP.
	copy(buf[22:28], virtualMAC[:])
	copy(buf[28:32], vip[:])
	// tha = Virtual Router MAC (errata 7947/7949); tpa = VIP.
	copy(buf[32:38], virtualMAC[:])
	copy(buf[38:42], vip[:])

	return GARPFrameLen
}
