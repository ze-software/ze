// Design: docs/architecture/ospf/ospf-12-auth.md -- engine glue for OSPFv2 authentication.
// Related: internal/plugins/ospf/transport -- the TX path the signer hooks into.
// RFC: rfc/short/rfc2328.md (App D), rfc/short/rfc5709.md, rfc/short/rfc7474.md

package ospf

import (
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

// installAuthHooks wires the sign step into the transport TX path and the verify
// chokepoint into the RX dispatcher. Called once at engine construction.
//
// OSPFv2 ONLY. RFC 5340 §A.3.1 removed the AuType and Authentication fields from the
// OSPFv3 common header, so signPacket/verifyPacket (RFC 2328 App D, RFC 5709, RFC 7474)
// have no header to act on there -- offset 15 is the v3 Reserved octet, not AuType.
// Installing the signer on an OSPFv3 transport is also actively harmful: SendPacket
// treats a non-nil signer as "the RFC 7166 trailer owns integrity" and SKIPS
// FinalizePacketChecksum (v3/transport/transport.go:496-502), so every OSPFv3 packet
// would go out with a zero IPv6 upper-layer checksum. OSPFv3 authentication is IPsec
// (RFC 4552), installed separately by installIPsecHooks.
func (e *engine) installAuthHooks() {
	if e.dispatch == nil {
		return
	}
	if _, isV2 := e.dispatch.codec.(v4Codec); !isV2 {
		return
	}
	if e.transport != nil {
		e.transport.SetSigner(e.signPacket)
	}
	if e.dispatch != nil {
		e.dispatch.authOK = e.verifyPacket
	}
}

// signPacket authenticates an outgoing OSPF packet for name (transport signer hook).
// The OSPF encoders default to AuType 0, so this rewrites the AuType, fixes the
// checksum (recomputed for AuType 1, kept zero for crypto -- trap #10), sets the 8-byte
// auth field, and appends any digest. Returns the original payload unchanged when no
// auth chain is configured for the interface.
func (e *engine) signPacket(name string, payload []byte) []byte {
	key, au, seq, src, ok := e.auth.signKey(name)
	if !ok || au == packet.AuTypeNull || len(payload) < packet.CommonHeaderLen {
		return payload
	}
	// RFC 6549 sec 2: offset 14 is the Instance ID (already stamped by the encoder), offset
	// 15 is the 8-bit AuType. Write only the AuType octet so the Instance ID is preserved --
	// the encoders default AuType 0, so this rewrites it to the configured value. Offset 14
	// must NOT be touched here (a clobber would drop a non-zero instance's packets on peers).
	payload[15] = byte(uint16(au))
	payload[12], payload[13] = 0, 0
	if au == packet.AuTypeSimple {
		// AuType 1 keeps a normal checksum; recompute it over the new AuType (it excludes
		// the 8-byte auth field, so the password Sign writes next does not affect it).
		ck := packet.PacketChecksum(payload)
		payload[12] = byte(ck >> 8)
		payload[13] = byte(ck)
	}
	signed, err := packet.Sign(payload, au, key, seq, src)
	if err != nil {
		return payload
	}
	return signed
}

// verifyPacket authenticates a received OSPF packet before it is routed to a handler
// (dispatcher authOK hook). A verification failure increments
// ze_ospf_auth_failures_total{interface,reason} and drops the packet.
func (e *engine) verifyPacket(rp transport.RawPacket, h Header) bool {
	e.mu.Lock()
	_, name, ok := e.interfaceByIfIndexLocked(rp.IfIndex)
	e.mu.Unlock()
	if !ok {
		return true // unknown interface: leave to the existing area/handler checks
	}
	// rp.Src is the IPv4 source from the IP header, bound into the AuType 3 digest per RFC
	// 7474 §5. Guard As4: a zero/non-IPv4 Addr (synthetic or malformed) must not panic --
	// it yields a zero source, which only matters for AuType 3 and never for a real packet.
	var src [4]byte
	if a := rp.Src.Unmap(); a.Is4() {
		src = a.As4()
	}
	reason, pass := e.auth.verify(name, h.RouterID, src, rp.Payload)
	if !pass {
		e.mAuthFailures.With(name, reason).Inc()
		return false
	}
	return true
}
