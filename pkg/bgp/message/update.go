package message

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/wire"
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

// Pack serializes the UPDATE to wire format.
//
// RFC 4271 Section 4.3 - The minimum length of the UPDATE message is 23 octets:
// 19 octets for the fixed header + 2 octets for Withdrawn Routes Length +
// 2 octets for Total Path Attribute Length.
func (u *Update) Pack(neg *Negotiated) ([]byte, error) {
	withdrawnLen := len(u.WithdrawnRoutes)
	attrLen := len(u.PathAttributes)
	nlriLen := len(u.NLRI)

	// Body = WithdrawnLen(2) + Withdrawn + AttrLen(2) + Attrs + NLRI
	bodyLen := 2 + withdrawnLen + 2 + attrLen + nlriLen
	body := make([]byte, bodyLen)

	// RFC 4271 Section 4.3 - Withdrawn Routes Length: 2-octet unsigned integer
	// indicating the total length of the Withdrawn Routes field in octets.
	// A value of 0 indicates no routes are being withdrawn.
	binary.BigEndian.PutUint16(body[0:2], uint16(withdrawnLen)) //nolint:gosec // BGP max message 65535

	// RFC 4271 Section 4.3 - Withdrawn Routes: list of IP address prefixes
	// for routes being withdrawn, each encoded as <length, prefix>.
	copy(body[2:], u.WithdrawnRoutes)

	// RFC 4271 Section 4.3 - Total Path Attribute Length: 2-octet unsigned integer
	// indicating the total length of the Path Attributes field in octets.
	// A value of 0 indicates neither NLRI nor Path Attributes are present.
	offset := 2 + withdrawnLen
	binary.BigEndian.PutUint16(body[offset:offset+2], uint16(attrLen)) //nolint:gosec // BGP max message 65535

	// RFC 4271 Section 4.3 - Path Attributes: variable-length sequence of
	// path attributes, each a triple <type, length, value>.
	copy(body[offset+2:], u.PathAttributes)

	// RFC 4271 Section 4.3 - NLRI: list of IP address prefixes, each encoded
	// as <length, prefix>. Length is calculated implicitly from message size.
	copy(body[offset+2+attrLen:], u.NLRI)

	return packWithHeader(TypeUPDATE, body), nil
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
func UnpackUpdate(data []byte) (*Update, error) {
	// RFC 4271 Section 4.3 - Minimum UPDATE body is 4 octets:
	// 2 octets Withdrawn Routes Length + 2 octets Total Path Attribute Length.
	if len(data) < 4 {
		return nil, ErrShortRead
	}

	// RFC 4271 Section 4.3 - Withdrawn Routes Length: 2-octet unsigned integer
	// indicating total length of Withdrawn Routes field in octets.
	withdrawnLen := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+withdrawnLen+2 {
		return nil, ErrShortRead
	}

	// RFC 4271 Section 4.3 - Total Path Attribute Length: 2-octet unsigned integer
	// indicating total length of Path Attributes field in octets.
	attrOffset := 2 + withdrawnLen
	attrLen := int(binary.BigEndian.Uint16(data[attrOffset : attrOffset+2]))
	if len(data) < attrOffset+2+attrLen {
		return nil, ErrShortRead
	}

	// RFC 4271 Section 4.3 - NLRI length is not encoded explicitly but calculated as:
	// UPDATE message Length - 23 - Total Path Attributes Length - Withdrawn Routes Length
	// (Here we calculate from remaining bytes since header already stripped.)
	nlriOffset := attrOffset + 2 + attrLen
	nlriLen := len(data) - nlriOffset

	u := &Update{
		rawData: data, // Keep reference for passthrough
	}

	// Extract sections (still references original data - zero-copy)
	if withdrawnLen > 0 {
		u.WithdrawnRoutes = data[2 : 2+withdrawnLen]
	}
	if attrLen > 0 {
		u.PathAttributes = data[attrOffset+2 : attrOffset+2+attrLen]
	}
	if nlriLen > 0 {
		u.NLRI = data[nlriOffset:]
	}

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
	for i := 0; i < MarkerLen; i++ {
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
	withdrawnLen := len(u.WithdrawnRoutes)
	buf[off] = byte(withdrawnLen >> 8)
	buf[off+1] = byte(withdrawnLen)
	off += 2

	// RFC 4271 Section 4.3 - Withdrawn Routes
	off += copy(buf[off:], u.WithdrawnRoutes)

	// RFC 4271 Section 4.3 - Total Path Attribute Length (2 octets)
	attrLen := len(u.PathAttributes)
	buf[off] = byte(attrLen >> 8)
	buf[off+1] = byte(attrLen)
	off += 2

	// RFC 4271 Section 4.3 - Path Attributes
	off += copy(buf[off:], u.PathAttributes)

	// RFC 4271 Section 4.3 - NLRI
	off += copy(buf[off:], u.NLRI)

	// Backfill total length
	totalLen := off - start
	buf[lengthPos] = byte(totalLen >> 8)
	buf[lengthPos+1] = byte(totalLen)

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
