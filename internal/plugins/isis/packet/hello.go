// Design: plan/learned/928-isis-2-wire.md -- LAN L1/L2 IIH and P2P IIH body codec
// ISO/IEC 10589 clause 9.5 (LAN IIH), clause 9.6 (P2P IIH).

package packet

import "github.com/ze-software/ze/internal/plugins/isis/types"

// CircuitType is the 1-octet circuit type field in an IIH (ISO/IEC 10589 clause
// 9.5): the low two bits select the level(s) the sender uses on this circuit.
type CircuitType uint8

// Circuit type values (ISO/IEC 10589 clause 9.5: only the low two bits are
// defined; the rest are reserved and ignored on receipt).
const (
	CircuitL1   CircuitType = 1 // Level 1 only
	CircuitL2   CircuitType = 2 // Level 2 only
	CircuitL1L2 CircuitType = 3 // Level 1 and Level 2
)

// PDU-specific fixed-header lengths (the octets after the common header, before
// the TLVs). All assume the 6-octet System ID (Ze fixes ID length at 6).
const (
	// lanHelloFixedLen: circuit type (1) + System ID (6) + holding time (2) +
	// PDU length (2) + priority (1) + LAN ID (SourceID, 7). ISO/IEC 10589 clause
	// 9.5: Priority is a single octet whose high bit is reserved (masked off); there
	// is NO separate reserved octet between Priority and the LAN ID. With the
	// 6-octet System ID this fixed part is 19 octets, so the full IIH fixed header
	// (common header 8 + 19) is 27 -- the value FRR isisd validates against.
	lanHelloFixedLen = 1 + types.SystemIDLen + types.LifetimeLen + 2 + 1 + types.SourceIDLen

	// p2pHelloFixedLen: circuit type (1) + System ID (6) + holding time (2) +
	// PDU length (2) + local circuit ID (1).
	p2pHelloFixedLen = 1 + types.SystemIDLen + types.LifetimeLen + 2 + 1

	// MaxDISPriority is the largest DIS priority (LAN IIH).
	MaxDISPriority = 127
)

// LANHello is a decoded Level 1 or Level 2 LAN IS-IS Hello (ISO/IEC 10589
// clause 9.5). The PDU type (set by the caller on encode, read on decode)
// distinguishes L1 (0x0f) from L2 (0x10). LANID is the System ID + pseudonode
// of the current DIS (zero when none elected). TLVs are retained in order.
type LANHello struct {
	PDUType          PDUType // PDUTypeL1LANHello or PDUTypeL2LANHello
	CircuitType      CircuitType
	SystemID         types.SystemID
	HoldingTime      types.HoldingTime
	Priority         uint8 // 0..127 (DIS election)
	LANID            types.SourceID
	MaxAreaAddresses uint8 // common-header field; 0 = the default 3
	TLVs             []TLV
}

// EncodedLen returns the total on-wire size of the LAN IIH.
func (h *LANHello) EncodedLen() int {
	return CommonHeaderLen + lanHelloFixedLen + tlvsEncodedLen(h.TLVs)
}

// WriteTo serializes the LAN IIH into buf at off and returns the new offset.
// The PDU Length field is filled by skip-and-backfill (ai/rules/buffer-first.md)
// rather than a Len()-then-WriteTo() double pass. Buffer-first: the caller
// guarantees room (>= EncodedLen()).
//
// ISO/IEC 10589 clause 9.5: priority is a 7-bit field; the high bit is
// reserved and sent zero, so the priority is masked to 0..127.
func (h *LANHello) WriteTo(buf []byte, off int) int {
	start := off
	off = writeCommonHeader(buf, off, h.PDUType, CommonHeaderLen+uint8(lanHelloFixedLen), h.MaxAreaAddresses)
	buf[off] = byte(h.CircuitType)
	off++
	off += h.SystemID.WriteTo(buf, off)
	off += h.HoldingTime.WriteTo(buf, off)
	pduLenPos := off // skip the 2-octet PDU Length; backfill after TLVs
	off += 2
	// ISO/IEC 10589 clause 9.5: Priority is one octet whose high bit is reserved
	// (sent zero via the mask). There is NO separate reserved octet after it -- the
	// LAN ID follows immediately. Emitting an extra reserved byte here made ze's IIH
	// fixed header 28 octets, which a strict peer (FRR isisd: "Expected fixed header
	// length = 27 but got 28") rejects, so no adjacency forms.
	buf[off] = h.Priority & MaxDISPriority
	off++
	off += h.LANID.WriteTo(buf, off)
	off = writeTLVs(buf, off, h.TLVs)
	total := off - start
	buf[pduLenPos] = byte(total >> 8)
	buf[pduLenPos+1] = byte(total)
	return off
}

// DecodeLANHello parses a LAN IIH body following the common header. body is the
// slice after the 8-octet common header. pt is the PDU type from the header
// (L1/L2). Every field is bound-checked before slicing (security review). The
// returned TLVs alias body; the caller copies any it retains.
func DecodeLANHello(pt PDUType, body []byte) (LANHello, error) {
	if len(body) < lanHelloFixedLen {
		return LANHello{}, ErrTruncated
	}
	off := 0
	h := LANHello{PDUType: pt}
	// MaxAreaAddresses is recovered from the common header by the caller via
	// DecodeHeader; DecodePDU threads it through (see pdu.go).
	h.CircuitType = CircuitType(body[off])
	off++
	sys, _ := types.SystemIDFromBytes(body[off : off+types.SystemIDLen])
	h.SystemID = sys
	off += types.SystemIDLen
	hold, _ := types.HoldingTimeFromBytes(body[off : off+types.LifetimeLen])
	h.HoldingTime = hold
	off += types.LifetimeLen
	pduLen := int(body[off])<<8 | int(body[off+1])
	off += 2
	// ISO/IEC 10589 clause 9.5: Priority is one octet (high bit reserved, masked);
	// the LAN ID follows immediately with no separate reserved octet.
	h.Priority = body[off] & MaxDISPriority
	off++
	lanID, _ := types.SourceIDFromBytes(body[off : off+types.SourceIDLen])
	h.LANID = lanID
	off += types.SourceIDLen

	tlvRegion, err := pduTLVRegion(body, off, pduLen)
	if err != nil {
		return LANHello{}, err
	}
	tlvs, err := DecodeTLVs(tlvRegion)
	if err != nil {
		return LANHello{}, err
	}
	h.TLVs = tlvs
	return h, nil
}

// P2PHello is a decoded Point-to-Point IS-IS Hello (ISO/IEC 10589 clause 9.6).
// It is level-agnostic (its circuit type carries the level). LocalCircuitID is
// the 1-octet circuit ID on the sender's side. TLVs are retained in order
// (notably TLV 240 for the RFC 5303 three-way handshake).
type P2PHello struct {
	CircuitType      CircuitType
	SystemID         types.SystemID
	HoldingTime      types.HoldingTime
	LocalCircuitID   uint8
	MaxAreaAddresses uint8 // common-header field; 0 = the default 3
	TLVs             []TLV
}

// EncodedLen returns the total on-wire size of the P2P IIH.
func (h *P2PHello) EncodedLen() int {
	return CommonHeaderLen + p2pHelloFixedLen + tlvsEncodedLen(h.TLVs)
}

// WriteTo serializes the P2P IIH into buf at off; PDU Length via
// skip-and-backfill. Buffer-first.
func (h *P2PHello) WriteTo(buf []byte, off int) int {
	start := off
	off = writeCommonHeader(buf, off, PDUTypeP2PHello, CommonHeaderLen+uint8(p2pHelloFixedLen), h.MaxAreaAddresses)
	buf[off] = byte(h.CircuitType)
	off++
	off += h.SystemID.WriteTo(buf, off)
	off += h.HoldingTime.WriteTo(buf, off)
	pduLenPos := off
	off += 2
	buf[off] = h.LocalCircuitID
	off++
	off = writeTLVs(buf, off, h.TLVs)
	total := off - start
	buf[pduLenPos] = byte(total >> 8)
	buf[pduLenPos+1] = byte(total)
	return off
}

// DecodeP2PHello parses a P2P IIH body following the common header.
func DecodeP2PHello(body []byte) (P2PHello, error) {
	if len(body) < p2pHelloFixedLen {
		return P2PHello{}, ErrTruncated
	}
	off := 0
	var h P2PHello
	h.CircuitType = CircuitType(body[off])
	off++
	sys, _ := types.SystemIDFromBytes(body[off : off+types.SystemIDLen])
	h.SystemID = sys
	off += types.SystemIDLen
	hold, _ := types.HoldingTimeFromBytes(body[off : off+types.LifetimeLen])
	h.HoldingTime = hold
	off += types.LifetimeLen
	pduLen := int(body[off])<<8 | int(body[off+1])
	off += 2
	h.LocalCircuitID = body[off]
	off++

	tlvRegion, err := pduTLVRegion(body, off, pduLen)
	if err != nil {
		return P2PHello{}, err
	}
	tlvs, err := DecodeTLVs(tlvRegion)
	if err != nil {
		return P2PHello{}, err
	}
	h.TLVs = tlvs
	return h, nil
}

// pduTLVRegion returns the TLV region of a decoded PDU given the body slice,
// the offset where TLVs begin (within body), and the PDU Length field value
// (which counts from the START of the common header). It bounds the region to
// the declared PDU length when that length is sane, otherwise to the available
// body (so a zero or oversized PDU-length field does not over- or under-read).
//
// pduLen counts the common header (CommonHeaderLen) + fixed fields + TLVs. The
// TLV region within body is therefore [tlvStart : pduLen-CommonHeaderLen].
func pduTLVRegion(body []byte, tlvStart, pduLen int) ([]byte, error) {
	// The TLV region cannot start past the body.
	if tlvStart > len(body) {
		return nil, ErrTruncated
	}
	// A PDU Length of 0 (or one that does not extend past the fixed header) is
	// treated as "use the rest of the body": some senders/captures omit it.
	tlvEndInBody := len(body)
	if pduLen > CommonHeaderLen {
		want := pduLen - CommonHeaderLen
		if want < tlvStart {
			// PDU length ends inside the fixed header: malformed.
			return nil, ErrLength
		}
		if want <= len(body) {
			tlvEndInBody = want
		}
		// If want > len(body) the declared length overruns the buffer; clamp to
		// the body so decode never reads past it (the caller's framing already
		// bounded the buffer).
	}
	return body[tlvStart:tlvEndInBody], nil
}
