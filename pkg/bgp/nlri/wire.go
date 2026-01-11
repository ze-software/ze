package nlri

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/wire"
)

// WireNLRI wraps raw wire-encoded NLRI bytes.
// Implements NLRI interface for use in NLRIGroup.
// Used for wire mode API input where bytes are passed through without parsing.
//
// IMPORTANT: Caller must not modify data after calling NewWireNLRI.
// WireNLRI takes ownership of the slice (no copy for zero-allocation).
type WireNLRI struct {
	family     Family
	data       []byte // Raw wire bytes (with or without path-id based on hasAddPath)
	hasAddPath bool   // True if data starts with 4-byte path-id
}

// NewWireNLRI creates a WireNLRI from raw bytes.
// Data should be a single NLRI in wire format (already split from concatenated input).
// hasAddPath indicates if data includes 4-byte path-id prefix.
// Takes ownership of data slice - caller must not modify after this call.
// Returns error if hasAddPath but len(data) < 4 (malformed).
func NewWireNLRI(family Family, data []byte, hasAddPath bool) (*WireNLRI, error) {
	if hasAddPath && len(data) < 4 {
		return nil, fmt.Errorf("malformed NLRI: addpath flag set but data < 4 bytes")
	}
	return &WireNLRI{family: family, data: data, hasAddPath: hasAddPath}, nil
}

// Family returns the AFI/SAFI for this NLRI.
func (w *WireNLRI) Family() Family { return w.family }

// Len returns the full raw length in bytes (including path-id if present).
func (w *WireNLRI) Len() int { return len(w.data) }

// LenWithContext returns the wire length adapted for context.
// Accounts for ADD-PATH path-id addition/removal.
func (w *WireNLRI) LenWithContext(ctx *PackContext) int {
	targetAddPath := ctx != nil && ctx.AddPath
	if w.hasAddPath && !targetAddPath {
		return len(w.data) - 4 // Strip path-id
	}
	if !w.hasAddPath && targetAddPath {
		return len(w.data) + 4 // Add path-id
	}
	return len(w.data)
}

// String returns a human-readable representation.
func (w *WireNLRI) String() string {
	return fmt.Sprintf("wire[%s](%d bytes)", w.family, len(w.data))
}

// HasAddPath returns true if data includes path-id prefix.
func (w *WireNLRI) HasAddPath() bool { return w.hasAddPath }

// PathID extracts path-id from data (0 if !hasAddPath or data too short).
// RFC 7911 Section 3: Path Identifier is a 4-octet field.
func (w *WireNLRI) PathID() uint32 {
	if !w.hasAddPath || len(w.data) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(w.data[:4])
}

// Bytes returns raw data as-is (including path-id if present).
func (w *WireNLRI) Bytes() []byte {
	return w.data
}

// WriteTo writes the NLRI into buf at offset, adapting for context.
// Returns number of bytes written.
// Writes directly to buf without allocation.
func (w *WireNLRI) WriteTo(buf []byte, off int, ctx *PackContext) int {
	targetAddPath := ctx != nil && ctx.AddPath

	if w.hasAddPath && !targetAddPath {
		// Strip 4-byte path-id (RFC 7911)
		return copy(buf[off:], w.data[4:])
	}

	if !w.hasAddPath && targetAddPath {
		// Prepend NOPATH (path-id = 0)
		// buf[off:off+4] = 0 (NOPATH)
		buf[off] = 0
		buf[off+1] = 0
		buf[off+2] = 0
		buf[off+3] = 0
		copy(buf[off+4:], w.data)
		return 4 + len(w.data)
	}

	return copy(buf[off:], w.data)
}

// CheckedWriteTo validates capacity before writing.
func (w *WireNLRI) CheckedWriteTo(buf []byte, off int, ctx *PackContext) (int, error) {
	needed := w.LenWithContext(ctx)
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return w.WriteTo(buf, off, ctx), nil
}

// Pack returns wire bytes adapted for context.
// Allocates a new buffer and calls WriteTo.
// For zero-allocation, use WriteTo directly with pre-allocated buffer.
func (w *WireNLRI) Pack(ctx *PackContext) []byte {
	size := w.LenWithContext(ctx)
	buf := make([]byte, size)
	w.WriteTo(buf, 0, ctx)
	return buf
}
