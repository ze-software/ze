// Design: docs/architecture/isis/isis-5-adjacency.md -- IIH origination (LAN + P2P) + padding.
// ISO/IEC 10589 clause 9.5/9.6 (IIH layout), clause 9.10 (Padding TLV 8),
// section 8.2 (hold time = hello-interval * hold-multiplier).
//
// RFC: rfc/short/rfc1195.md -- TLV 1 (Area Addresses), 129 (Protocols Supported), 132 (IP Interface Address)
// RFC: rfc/short/rfc5303.md -- TLV 240 (P2P Three-Way Adjacency)
//
// This file is the constructor of the FULL IIH bytes, INCLUDING the Padding TLV
// (8) sized to the interface MTU. Per the umbrella "Final PDU bytes: padding
// then authentication" contract, padding is added HERE during PDU construction,
// BEFORE authentication: build IIH (origination TLVs + TLV 8 padding) ->
// spec-isis-10 signs the padded PDU (TLV 10) -> Fletcher checksum (LSPs only) ->
// the spec-isis-3 transport adds ONLY 802.3+LLC framing and MUST NOT pad/alter.

package circuit

import (
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// MinHoldTime / MaxHoldTime bound the advertised holding time (a 16-bit seconds
// field, ISO/IEC 10589 section 8.2). hello-interval * hold-multiplier is clamped
// into this range so a boundary configuration cannot advertise 0 (immediate
// timeout) or overflow the 16-bit field.
const (
	MinHoldTime = 1
	MaxHoldTime = 65535
)

// HoldTime computes the advertised holding time from the hello interval and the
// hold multiplier (ISO/IEC 10589 section 8.2: hold time = interval * multiplier).
// The product is clamped to the 16-bit range [MinHoldTime, MaxHoldTime] so a
// zero interval or multiplier never advertises an instantly-expiring adjacency
// and a large product never overflows the wire field.
func HoldTime(helloInterval uint16, holdMult uint8) uint16 {
	if helloInterval == 0 || holdMult == 0 {
		return MinHoldTime
	}
	product := uint32(helloInterval) * uint32(holdMult)
	if product < MinHoldTime {
		return MinHoldTime
	}
	if product > MaxHoldTime {
		return MaxHoldTime
	}
	return uint16(product)
}

// originationTLVs returns the common origination TLVs every IIH carries (RFC
// 1195): TLV 1 (Area Addresses), TLV 129 (Protocols Supported), TLV 132 (IPv4
// Interface Address). TLV 6 (LAN) and TLV 240 (P2P) are appended by the LAN/P2P
// builders. Each TLV is serialized into its own value buffer so the generic
// packet.TLV carrier can re-emit it (the bodies fit one 255-octet TLV: a handful
// of areas, two NLPIDs, one address).
func (c *Circuit) originationTLVs() []packet.TLV {
	tlvs := make([]packet.TLV, 0, 5)
	tlvs = append(tlvs, c.areaAddressesTLV(), c.protocolsSupportedTLV())
	if v := c.ipv4InterfaceAddrTLV(); v.Type != 0 {
		tlvs = append(tlvs, v)
	}
	if v := c.ipv6InterfaceAddrTLV(); v.Type != 0 {
		tlvs = append(tlvs, v)
	}
	return tlvs
}

// areaAddressesTLV builds TLV 1 from the circuit's configured area addresses.
// Each entry is a 1-octet length prefix followed by the 1..13 octet Area Address
// (ISO/IEC 10589 clause 9.8). A single TLV value holds at most MaxTLVValueLen
// (255) octets, so an entry that would overflow the value buffer is skipped
// rather than written past the end (which would panic): with the maximum
// 13-octet area each entry costs 14 octets, so the 19th area no longer fits. The
// rare operator who configures that many areas advertises only the entries that
// fit one TLV; the protocol caps the advertised count anyway (clause 8.4.1
// MaxAreaAddresses, default 3).
func (c *Circuit) areaAddressesTLV() packet.TLV {
	var buf [packet.MaxTLVValueLen]byte
	off := 0
	for _, a := range c.areas {
		// ISO/IEC 10589 clause 9.8: one length octet + the Area Address octets.
		// Skip an entry that would write past the fixed value buffer; without this
		// guard a >=19-area configuration indexes out of range and panics.
		if off+1+a.Len() > len(buf) {
			break
		}
		buf[off] = byte(a.Len())
		off++
		off += a.WriteTo(buf[:], off)
	}
	return packet.TLV{Type: packet.TLVAreaAddresses, Value: append([]byte(nil), buf[:off]...)}
}

// protocolsSupportedTLV builds TLV 129 advertising the configured NLPIDs (IPv4
// always; IPv6 when an IPv6 address family is enabled on the circuit).
func (c *Circuit) protocolsSupportedTLV() packet.TLV {
	nlpids := make([]byte, 0, 2)
	nlpids = append(nlpids, packet.NLPIDIPv4)
	if c.advertiseIPv6 {
		nlpids = append(nlpids, packet.NLPIDIPv6)
	}
	return packet.TLV{Type: packet.TLVProtocolsSupported, Value: nlpids}
}

// ipv4InterfaceAddrTLV builds TLV 132 from the circuit's local IPv4 interface
// address. Returns a zero TLV (Type 0) when the circuit has no IPv4 address, so
// the caller omits it.
func (c *Circuit) ipv4InterfaceAddrTLV() packet.TLV {
	if !c.ipv4.IsValid() || !c.ipv4.Is4() {
		return packet.TLV{}
	}
	a4 := c.ipv4.As4()
	return packet.TLV{Type: packet.TLVIPInterfaceAddress, Value: append([]byte(nil), a4[:]...)}
}

// ipv6InterfaceAddrTLV builds TLV 232 (IPv6 Interface Address, RFC 5308 sec 3)
// from the circuit's IPv6 LINK-LOCAL address. RFC 5308 sec 3: a Hello carries
// ONLY link-local addresses, so this is the SPF next-hop source a dual-stack
// neighbor learns. Returns a zero TLV (Type 0) when IPv6 is not advertised on
// the circuit or there is no valid link-local address, so the caller omits it.
func (c *Circuit) ipv6InterfaceAddrTLV() packet.TLV {
	if !c.advertiseIPv6 {
		return packet.TLV{}
	}
	a := c.ipv6LinkLocal
	if !a.IsValid() || !a.Is6() || !a.IsLinkLocalUnicast() {
		return packet.TLV{}
	}
	a16 := a.As16()
	return packet.TLV{Type: packet.TLVIPv6InterfaceAddress, Value: append([]byte(nil), a16[:]...)}
}

// isNeighborsTLV builds TLV 6 (LAN only): the list of SNPAs (neighbor MACs) this
// circuit has heard Hellos from. A neighbor reaching Up requires seeing its own
// SNPA echoed here (the LAN three-way check, ISO/IEC 10589 section 8.2). Returns
// a zero TLV when there are no heard neighbors yet (an empty list is still valid
// but omitting it keeps the first Hello minimal).
func (c *Circuit) isNeighborsTLV(snpas []adjacency.SNPA) packet.TLV {
	if len(snpas) == 0 {
		return packet.TLV{Type: packet.TLVISNeighbors, Value: nil}
	}
	val := make([]byte, 0, len(snpas)*packet.SNPALen)
	for _, s := range snpas {
		val = append(val, s[:]...)
	}
	return packet.TLV{Type: packet.TLVISNeighbors, Value: val}
}

// threeWayTLV builds TLV 240 (P2P only) reporting our three-way state and, when
// we have heard the neighbor, echoing its System ID so it can complete the
// handshake (RFC 5303 sec 3.1). The value layout is: 1-octet state + 4-octet
// extended local circuit ID + (when haveNeighbor) 6-octet neighbor System ID +
// 4-octet neighbor extended circuit ID. We always emit the extended local
// circuit ID (the 5- or 15-octet form), never the bare 1-octet form, so a peer
// can match our circuit. The 15-octet form (with the neighbor echo) is what
// proves to the neighbor that we have heard it.
func (c *Circuit) threeWayTLV(state packet.AdjThreeWayState, neighborID types.SystemID, haveNeighbor bool) packet.TLV {
	val := make([]byte, 0, p2pThreeWayValueMaxLen)
	val = append(val, byte(state))
	cid := uint32(c.localCircuitID)
	val = append(val, byte(cid>>24), byte(cid>>16), byte(cid>>8), byte(cid))
	if haveNeighbor {
		val = append(val, neighborID[:]...)
		// Neighbor extended circuit ID: we do not track it separately, so echo 0;
		// RFC 5303 makes the System ID echo the load-bearing proof of having heard
		// the neighbor.
		val = append(val, 0, 0, 0, 0)
	}
	return packet.TLV{Type: packet.TLVP2PThreeWay, Value: val}
}

// p2pThreeWayValueMaxLen is the largest TLV 240 value (state + extended local
// circuit ID + neighbor System ID + neighbor extended circuit ID).
const p2pThreeWayValueMaxLen = 1 + 4 + types.SystemIDLen + 4

// buildLANHello assembles a full LAN IIH (without padding) carrying the
// origination TLVs and TLV 6, returning the encoded PDU bytes. level selects the
// PDU type (0x0f L1 / 0x10 L2). The result is the PDU BEFORE padding and
// authentication; padHello adds TLV 8 and signing happens afterward (isis-10).
func (c *Circuit) buildLANHello(level adjacency.Level, snpas []adjacency.SNPA, padMTU int) []byte {
	pt := packet.PDUTypeL1LANHello
	if level == adjacency.Level2 {
		pt = packet.PDUTypeL2LANHello
	}
	h := packet.LANHello{
		PDUType:     pt,
		CircuitType: c.circuitTypeField(),
		SystemID:    c.systemID,
		HoldingTime: types.HoldingTime(c.holdTime),
		Priority:    c.priority,
		LANID:       c.lanID,
		TLVs:        append(c.originationTLVs(), c.isNeighborsTLV(snpas)),
	}
	encodedLen := h.EncodedLen()
	buf := make([]byte, encodedLen, max(encodedLen, padMTU))
	n := h.WriteTo(buf, 0)
	return buf[:n]
}

// buildP2PHello assembles a full P2P IIH (without padding) carrying the
// origination TLVs and TLV 240. state/neighborID/haveNeighbor describe our
// three-way state toward the single P2P neighbor.
func (c *Circuit) buildP2PHello(state packet.AdjThreeWayState, neighborID types.SystemID, haveNeighbor bool, padMTU int) []byte {
	h := packet.P2PHello{
		CircuitType:    c.circuitTypeField(),
		SystemID:       c.systemID,
		HoldingTime:    types.HoldingTime(c.holdTime),
		LocalCircuitID: c.localCircuitID,
		TLVs:           append(c.originationTLVs(), c.threeWayTLV(state, neighborID, haveNeighbor)),
	}
	encodedLen := h.EncodedLen()
	buf := make([]byte, encodedLen, max(encodedLen, padMTU))
	n := h.WriteTo(buf, 0)
	return buf[:n]
}

// iihPDULengthOffset is the byte offset of the 2-octet PDU Length field in a LAN
// or P2P IIH: common header (8) + circuit type (1) + System ID (6) + holding
// time (2) = 17. Both IIH types place the PDU Length field at the same offset
// (ISO/IEC 10589 clause 9.5/9.6).
const iihPDULengthOffset = packet.CommonHeaderLen + 1 + 6 + 2

// padHello appends Padding TLVs (8) to an already-built IIH so the final PDU
// reaches the pad target (ISO/IEC 10589 clause 8.2.3 MTU-mismatch detection), and
// patches the PDU Length field to cover the padding. Padding is added here,
// during construction, BEFORE authentication, so the spec-isis-10 digest covers
// it (RFC 5304 signs padded Hellos). The transport frames the result and MUST NOT
// pad further. mtu here is the PDU-length budget the caller derives from the
// interface MTU (MTU minus the LLC header the transport prepends; see padMTU),
// NOT the raw interface MTU: the IS-IS PDU plus the LLC header must fit the link
// MTU, so padding the PDU to the full MTU would make the framed Hello exceed it. A
// single Padding TLV carries at most MaxTLVValueLen value octets, so multiple TLV
// 8s are emitted to fill a large MTU.
//
// padHello returns the padded PDU. When the PDU already meets or exceeds the
// MTU, or there is not enough room for even a 2-octet empty Padding TLV, it
// returns the input unchanged (the PDU Length already covers it).
func padHello(pdu []byte, mtu int) []byte {
	if mtu <= 0 || len(pdu) >= mtu {
		return pdu
	}
	var out []byte
	if cap(pdu) >= mtu {
		out = pdu
	} else {
		out = make([]byte, len(pdu), mtu)
		copy(out, pdu)
	}
	for len(out) < mtu {
		room := mtu - len(out)
		if room < packet.TLVHeaderLen {
			break // no room for even an empty Padding TLV
		}
		n := min(room-packet.TLVHeaderLen, packet.MaxTLVValueLen)
		// Grow by the TLV size, then write the Padding TLV into the new tail.
		base := len(out)
		out = out[:base+packet.TLVHeaderLen+n]
		packet.WritePaddingTLV(out, base, n)
	}
	// The IIH encoder backfilled the PDU Length field to the UNPADDED length
	// (skip-and-backfill); rewrite it to the full padded length so a receiver's
	// decoder includes the Padding TLVs in the TLV region (and so the field is
	// consistent on the wire). Guard the offset against an unexpectedly short PDU.
	if len(out) > iihPDULengthOffset+1 {
		out[iihPDULengthOffset] = byte(len(out) >> 8)
		out[iihPDULengthOffset+1] = byte(len(out))
	}
	return out
}
