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

// Len returns the payload length in bytes (without path-id).
func (w *WireNLRI) Len() int {
	if w.hasAddPath {
		return len(w.data) - 4
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

// SupportsAddPath returns true - WireNLRI is a passthrough and supports ADD-PATH.
func (w *WireNLRI) SupportsAddPath() bool { return true }

// Bytes returns raw data as-is (including path-id if present).
func (w *WireNLRI) Bytes() []byte {
	return w.data
}

// WriteTo writes the NLRI payload (without path-id) into buf at offset.
// Returns number of bytes written.
//
// RFC 7911 Section 3: Path ID is NOT written by this method.
// Use WriteNLRI() for ADD-PATH encoding with path identifier.
func (w *WireNLRI) WriteTo(buf []byte, off int) int {
	if w.hasAddPath {
		// Strip 4-byte path-id (RFC 7911)
		return copy(buf[off:], w.data[4:])
	}
	return copy(buf[off:], w.data)
}
