// Design: plan/spec-ospf-12-auth.md -- engine glue for OSPFv2 authentication.
// Related: internal/plugins/ospf/transport -- the TX path the signer hooks into.
// RFC: rfc/short/rfc2328.md (App D), rfc/short/rfc5709.md, rfc/short/rfc7474.md

package ospf

import (
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
)

// installAuthHooks wires the sign step into the transport TX path and the verify
// chokepoint into the RX dispatcher. Called once at engine construction.
func (e *engine) installAuthHooks() {
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
	payload[14] = byte(uint16(au) >> 8)
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
