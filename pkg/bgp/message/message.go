package message

import bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"

// EncodingContext is an alias for bgpctx.EncodingContext.
// Use this for WireWriter method signatures.
type EncodingContext = bgpctx.EncodingContext

// Message is the interface all BGP messages implement.
// Embeds WireWriter for unified buffer-based encoding.
type Message interface {
	bgpctx.WireWriter

	// Type returns the BGP message type.
	Type() MessageType

	// Pack serializes the message to wire format (including header).
	// NOTE: Prefer WriteTo for zero-allocation encoding. Pack will be removed
	// in a future release - see spec-pack-removal plan.
	Pack(neg *Negotiated) ([]byte, error)
}

// Negotiated holds the result of capability negotiation between peers.
// Used during packing/unpacking to handle capability-dependent formats.
type Negotiated struct {
	// ASN4 indicates 4-byte AS number support (RFC 6793)
	ASN4 bool

	// AddPath indicates ADD-PATH support per family (RFC 7911)
	AddPath map[Family]bool

	// ExtendedMessage indicates extended message support (RFC 8654)
	ExtendedMessage bool

	// LocalAS is our AS number
	LocalAS uint32

	// PeerAS is the peer's AS number
	PeerAS uint32

	// HoldTime is the negotiated hold time
	HoldTime uint16
}

// Family represents an address family (AFI/SAFI combination).
type Family struct {
	AFI  uint16
	SAFI uint8
}

// packWithHeader creates a complete message with header.
func packWithHeader(msgType MessageType, body []byte) []byte {
	totalLen := HeaderLen + len(body)
	data := make([]byte, totalLen)

	// Marker
	for i := 0; i < MarkerLen; i++ {
		data[i] = 0xFF
	}

	// Length
	data[16] = byte(totalLen >> 8)
	data[17] = byte(totalLen)

	// Type
	data[18] = byte(msgType)

	// Body
	copy(data[HeaderLen:], body)

	return data
}

// writeHeader writes a BGP message header into buf at offset.
// RFC 4271 Section 4.1 - Message Header format.
// totalLen is the complete message length (including header).
func writeHeader(buf []byte, off int, msgType MessageType, totalLen int) {
	// 16-byte marker (all 0xFF)
	for i := 0; i < MarkerLen; i++ {
		buf[off+i] = 0xFF
	}

	// Length (2 bytes, big-endian)
	buf[off+16] = byte(totalLen >> 8)
	buf[off+17] = byte(totalLen)

	// Type (1 byte)
	buf[off+18] = byte(msgType)
}

// PackTo allocates a buffer and writes the message using WriteTo.
// This is a convenience function for callers migrating from Pack().
// For zero-allocation, use WriteTo with a pre-allocated buffer instead.
func PackTo(msg Message, ctx *EncodingContext) []byte {
	size := msg.Len(ctx)
	buf := make([]byte, size)
	msg.WriteTo(buf, 0, ctx)
	return buf
}
