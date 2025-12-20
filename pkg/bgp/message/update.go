package message

import "encoding/binary"

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
