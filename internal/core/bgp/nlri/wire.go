// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding

package nlri

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errMalformedNlriAddpathFlagSetBut = errors.New("malformed NLRI: addpath flag set but data < 4 bytes")

// WireNLRI wraps raw wire-encoded NLRI bytes.
// Implements NLRI interface for use in NLRIGroup.
// Used for wire mode API input where bytes are passed through without parsing.
//
// IMPORTANT: Caller must not modify data after calling NewWireNLRI.
// WireNLRI takes ownership of the slice (no copy for zero-allocation).
type WireNLRI struct {
	fam        family.Family
	data       []byte // Raw wire bytes (with or without path-id based on hasAddPath)
	hasAddPath bool   // True if data starts with 4-byte path-id
}

// NewWireNLRI creates a WireNLRI from raw bytes.
// Data should be a single NLRI in wire format (already split from concatenated input).
// hasAddPath indicates if data includes 4-byte path-id prefix.
// Takes ownership of data slice - caller must not modify after this call.
// Returns error if hasAddPath but len(data) < 4 (malformed).
func NewWireNLRI(fam family.Family, data []byte, hasAddPath bool) (*WireNLRI, error) {
	if hasAddPath && len(data) < 4 {
		return nil, errMalformedNlriAddpathFlagSetBut
	}
	return &WireNLRI{fam: fam, data: data, hasAddPath: hasAddPath}, nil
}

// Family returns the AFI/SAFI for this NLRI.
func (w *WireNLRI) Family() family.Family { return w.fam }

// Len returns the payload length in bytes (without path-id).
func (w *WireNLRI) Len() int {
	if w.hasAddPath {
		return len(w.data) - 4
	}
	return len(w.data)
}

// String returns a human-readable representation.
//
// A family whose route is a CIDR behind a label stack (nlrisplit.RouteCIDR) is
// named by that prefix, the way INET names its own, so a route announced as a
// WireNLRI and withdrawn as an INET (wireu.ParseWithdrawnNLRIs) carry one name.
// A consumer keyed on the name, such as the route reflector's withdrawal map,
// then pairs the withdrawal with its announcement. The bytes are read as an
// announcement: a withdrawal of such a family is never carried as a WireNLRI.
func (w *WireNLRI) String() string {
	if prefix, ok := w.routePrefix(); ok {
		return prefix.String()
	}
	var b textbuf.Buffer
	return b.Reset().Str("wire[").Str(w.fam.String()).Str("](").Int(int64(len(w.data))).Str(" bytes)").String()
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

// routePrefix answers the prefix the route names when its family keys routes
// by a CIDR, and false for every other family and for malformed bytes.
func (w *WireNLRI) routePrefix() (netip.Prefix, bool) {
	payload := w.data
	if w.hasAddPath {
		if len(payload) < 4 {
			return netip.Prefix{}, false
		}
		payload = payload[4:]
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	cidr, err := nlrisplit.RouteCIDR(w.fam, payload, scratch[:], false)
	if err != nil {
		return netip.Prefix{}, false
	}
	n, _, err := ParseINET(w.fam.AFI, w.fam.SAFI, cidr, false)
	if err != nil {
		return netip.Prefix{}, false
	}
	inet, ok := n.(*INET)
	if !ok {
		return netip.Prefix{}, false
	}
	return inet.Prefix(), true
}
