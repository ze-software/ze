package message

import bgpctx "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/context"

// EncodingContext is an alias for bgpctx.EncodingContext.
// Use this for WireWriter method signatures.
type EncodingContext = bgpctx.EncodingContext

// Message is the interface all BGP messages implement.
// Embeds WireWriter for unified buffer-based encoding.
type Message interface {
	bgpctx.WireWriter

	// Type returns the BGP message type.
	Type() MessageType
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
// This is a convenience wrapper around WriteTo for callers that need a []byte.
// For zero-allocation, use WriteTo with a pre-allocated buffer instead.
func PackTo(msg Message, ctx *EncodingContext) []byte {
	size := msg.Len(ctx)
	buf := make([]byte, size)
	msg.WriteTo(buf, 0, ctx)
	return buf
}
