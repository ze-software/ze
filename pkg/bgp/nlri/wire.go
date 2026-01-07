package nlri

import (
	"encoding/binary"
	"fmt"
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

// Pack returns wire bytes adapted for context.
// Handles ADD-PATH mismatch (loses zero-copy benefit):
//   - Source has path-id, target doesn't: strip 4 bytes (RFC 7911)
//   - Source no path-id, target expects: prepend NOPATH (0x00000000)
//
// Cannot fail: NewWireNLRI validates data length at construction.
func (w *WireNLRI) Pack(ctx *PackContext) []byte {
	targetAddPath := ctx != nil && ctx.AddPath

	if w.hasAddPath && !targetAddPath {
		// Strip 4-byte path-id (RFC 7911: same size for IPv4/IPv6/any AFI)
		// Safe: NewWireNLRI guarantees len >= 4 when hasAddPath
		return w.data[4:]
	}

	if !w.hasAddPath && targetAddPath {
		// Prepend NOPATH (path-id = 0) - allocates new slice
		buf := make([]byte, 4+len(w.data))
		// buf[0:4] already zero (NOPATH)
		copy(buf[4:], w.data)
		return buf
	}

	return w.data
}

// WriteTo writes the NLRI into buf at offset, adapting for context.
// Returns number of bytes written.
// Cannot fail: Pack() is guaranteed to succeed.
func (w *WireNLRI) WriteTo(buf []byte, off int, ctx *PackContext) int {
	return copy(buf[off:], w.Pack(ctx))
}
