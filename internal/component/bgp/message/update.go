// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4271.md — UPDATE message format (Section 4.3)
// Overview: message.go — Message interface and writeHeader
// Detail: update_build.go — UPDATE builder infrastructure
// Detail: update_split.go — UPDATE splitting and chunking
// Related: open.go — OPEN message parsing and encoding
// Related: notification.go — NOTIFICATION message parsing and encoding
// Related: keepalive.go — KEEPALIVE message encoding
// Related: routerefresh.go — ROUTE-REFRESH message encoding
// Related: eor.go — end-of-RIB marker UPDATE construction

package message

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wire"
)

// Update represents a BGP UPDATE message (RFC 4271 Section 4.3).
//
// RFC 4271 Section 4.3 - UPDATE messages are used to transfer routing
// information between BGP peers. An UPDATE message can advertise feasible
// routes with common path attributes, or withdraw multiple unfeasible routes.
//
// RFC 4271 Section 4.3 - UPDATE message format:
//
//	+-----------------------------------------------------+
//	|   Withdrawn Routes Length (2 octets)                |
//	+-----------------------------------------------------+
//	|   Withdrawn Routes (variable)                       |
//	+-----------------------------------------------------+
//	|   Total Path Attribute Length (2 octets)            |
//	+-----------------------------------------------------+
//	|   Path Attributes (variable)                        |
//	+-----------------------------------------------------+
//	|   Network Layer Reachability Information (variable) |
//	+-----------------------------------------------------+
//
// This is a wire-level representation storing raw bytes.
// Detailed parsing of path attributes and NLRI is handled separately
// to support lazy parsing and zero-copy passthrough.
type Update struct {
	// Raw message body for passthrough optimization
	rawData []byte

	// Parsed sections (raw bytes, not fully parsed)
	// RFC 4271 Section 4.3 - Withdrawn Routes: variable-length field containing
	// IP address prefixes for routes being withdrawn, each encoded as <length, prefix>.
	WithdrawnRoutes []byte
	// RFC 4271 Section 4.3 - Path Attributes: variable-length sequence of path
	// attributes, each a triple <attribute type, attribute length, attribute value>.
	PathAttributes []byte
	// RFC 4271 Section 4.3 - Network Layer Reachability Information: variable-length
	// field containing IP address prefixes, each encoded as <length, prefix>.
	NLRI []byte
}

// Type returns the message type (UPDATE).
func (u *Update) Type() MessageType {
	return TypeUPDATE
}

// UnpackUpdate parses an UPDATE message body.
//
// RFC 4271 Section 4.3 - Parses the UPDATE message format which consists of:
// Withdrawn Routes Length (2 octets), Withdrawn Routes (variable),
// Total Path Attribute Length (2 octets), Path Attributes (variable),
// and NLRI (variable, length calculated implicitly).
//
// This performs minimal parsing - just extracts the three sections.
// Detailed attribute and NLRI parsing is done lazily.
// Uses wire.ParseUpdateSections for shared parsing logic with WireUpdate.
func UnpackUpdate(data []byte) (*Update, error) {
	// Use shared parser for consistent validation
	sections, err := wire.ParseUpdateSections(data)
	if err != nil {
		return nil, ErrShortRead
	}

	u := &Update{
		rawData: data, // Keep reference for passthrough
	}

	// Extract sections using shared parser (zero-copy)
	u.WithdrawnRoutes = sections.Withdrawn(data)
	u.PathAttributes = sections.Attrs(data)
	u.NLRI = sections.NLRI(data)

	return u, nil
}

// RawData returns the original unparsed message body.
// Used for passthrough optimization when forwarding unchanged updates.
func (u *Update) RawData() []byte {
	return u.rawData
}

// Len returns the total message length (header + body) in bytes.
//
// RFC 4271 Section 4.3 - UPDATE message format:
// Header(19) + WithdrawnLen(2) + Withdrawn + AttrLen(2) + Attrs + NLRI.
// Context is ignored for basic UPDATE (wire bytes already built).
func (u *Update) Len(_ *EncodingContext) int {
	return HeaderLen + 2 + len(u.WithdrawnRoutes) + 2 + len(u.PathAttributes) + len(u.NLRI)
}

// WriteTo writes the complete UPDATE message (header + body) into buf.
// Returns total bytes written. Implements WireWriter interface.
//
// RFC 4271 Section 4.3 - UPDATE message format with 19-byte header.
// This is the zero-allocation path for sending UPDATEs.
// Context is ignored for basic UPDATE (wire bytes already built).
func (u *Update) WriteTo(buf []byte, off int, _ *EncodingContext) int {
	start := off

	// RFC 4271 Section 4.1 - BGP Header: 16-byte marker (all 0xFF)
	for i := range MarkerLen {
		buf[off+i] = 0xFF
	}
	off += MarkerLen

	// Length placeholder (fill after body is written)
	lengthPos := off
	off += 2

	// Type
	buf[off] = byte(TypeUPDATE)
	off++

	// RFC 4271 Section 4.3 - Withdrawn Routes Length (2 octets)
	binary.BigEndian.PutUint16(buf[off:off+2], uint16(len(u.WithdrawnRoutes))) //nolint:gosec // bounded by BGP max
	off += 2

	// RFC 4271 Section 4.3 - Withdrawn Routes
	off += copy(buf[off:], u.WithdrawnRoutes)

	// RFC 4271 Section 4.3 - Total Path Attribute Length (2 octets)
	binary.BigEndian.PutUint16(buf[off:off+2], uint16(len(u.PathAttributes))) //nolint:gosec // bounded by BGP max
	off += 2

	// RFC 4271 Section 4.3 - Path Attributes
	off += copy(buf[off:], u.PathAttributes)

	// RFC 4271 Section 4.3 - NLRI
	off += copy(buf[off:], u.NLRI)

	// Backfill total length
	totalLen := off - start
	binary.BigEndian.PutUint16(buf[lengthPos:lengthPos+2], uint16(totalLen)) //nolint:gosec // bounded by BGP max

	return totalLen
}

// CheckedWriteTo validates capacity before writing.
func (u *Update) CheckedWriteTo(buf []byte, off int, ctx *EncodingContext) (int, error) {
	needed := u.Len(ctx)
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return u.WriteTo(buf, off, ctx), nil
}

// IsEndOfRIB returns true if this is an End-of-RIB marker.
//
// RFC 4271 Section 4.3 - An UPDATE message with Withdrawn Routes Length = 0
// and Total Path Attribute Length = 0 (and therefore no NLRI) represents
// the minimum valid UPDATE (23 octets total with header). This minimal UPDATE
// is used as the End-of-RIB marker per RFC 4724.
func (u *Update) IsEndOfRIB() bool {
	return len(u.WithdrawnRoutes) == 0 &&
		len(u.PathAttributes) == 0 &&
		len(u.NLRI) == 0
}

// ChunkNLRI splits NLRI data into chunks that fit within maxSize bytes.
//
// RFC 4271 Section 4.3 - UPDATE messages must not exceed the maximum message
// size (4096 bytes for standard BGP, 65535 with extended message capability).
// When NLRI data would cause the message to exceed this limit, it must be
// split across multiple UPDATE messages.
//
// The function respects prefix boundaries - it will not split a prefix across
// chunks. Each prefix in IPv4 NLRI format is: length(1 byte) + prefix bytes.
// The prefix bytes = ceil(length / 8).
//
// Parameters:
//   - nlri: Raw NLRI bytes (sequence of length-prefixed prefixes)
//   - maxSize: Maximum bytes per chunk
//
// Returns:
//   - Slice of NLRI byte chunks, each <= maxSize (except for oversized prefixes)
//
// If a single prefix exceeds maxSize, it is placed in its own chunk (best effort).
// The caller should ensure maxSize is reasonable (at least 5 bytes for /32).
func ChunkNLRI(nlri []byte, maxSize int) [][]byte {
	if len(nlri) == 0 {
		return nil
	}

	// If everything fits, return as-is
	if len(nlri) <= maxSize {
		return [][]byte{nlri}
	}

	var chunks [][]byte
	var currentChunk []byte
	offset := 0

	for offset < len(nlri) {
		// Read prefix length (in bits)
		// #nosec G602 -- offset bounds checked by loop condition
		prefixLen := int(nlri[offset])

		// Calculate prefix size in bytes: 1 (length byte) + ceil(prefixLen/8)
		prefixBytes := (prefixLen + 7) / 8
		totalSize := 1 + prefixBytes

		// Bounds check
		if offset+totalSize > len(nlri) {
			// Malformed NLRI - include remaining bytes and break
			currentChunk = append(currentChunk, nlri[offset:]...)
			break
		}

		// Extract this prefix
		prefix := nlri[offset : offset+totalSize]

		// Check if adding this prefix would exceed maxSize
		if len(currentChunk)+totalSize > maxSize {
			// Save current chunk if non-empty
			if len(currentChunk) > 0 {
				chunks = append(chunks, currentChunk)
				currentChunk = nil
			}
		}

		// Add prefix to current chunk (even if oversized - best effort)
		currentChunk = append(currentChunk, prefix...)
		offset += totalSize
	}

	// Don't forget the last chunk
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}
