package message

import "encoding/binary"

// Update represents a BGP UPDATE message (RFC 4271 Section 4.3).
//
// This is a wire-level representation storing raw bytes.
// Detailed parsing of path attributes and NLRI is handled separately
// to support lazy parsing and zero-copy passthrough.
type Update struct {
	// Raw message body for passthrough optimization
	rawData []byte

	// Parsed sections (raw bytes, not fully parsed)
	WithdrawnRoutes []byte // Withdrawn Routes field
	PathAttributes  []byte // Path Attributes field
	NLRI            []byte // Network Layer Reachability Information
}

// Type returns the message type (UPDATE).
func (u *Update) Type() MessageType {
	return TypeUPDATE
}

// Pack serializes the UPDATE to wire format.
func (u *Update) Pack(neg *Negotiated) ([]byte, error) {
	withdrawnLen := len(u.WithdrawnRoutes)
	attrLen := len(u.PathAttributes)
	nlriLen := len(u.NLRI)

	// Body = WithdrawnLen(2) + Withdrawn + AttrLen(2) + Attrs + NLRI
	bodyLen := 2 + withdrawnLen + 2 + attrLen + nlriLen
	body := make([]byte, bodyLen)

	// Withdrawn Routes Length
	binary.BigEndian.PutUint16(body[0:2], uint16(withdrawnLen)) //nolint:gosec // BGP max message 65535

	// Withdrawn Routes
	copy(body[2:], u.WithdrawnRoutes)

	// Path Attributes Length
	offset := 2 + withdrawnLen
	binary.BigEndian.PutUint16(body[offset:offset+2], uint16(attrLen)) //nolint:gosec // BGP max message 65535

	// Path Attributes
	copy(body[offset+2:], u.PathAttributes)

	// NLRI
	copy(body[offset+2+attrLen:], u.NLRI)

	return packWithHeader(TypeUPDATE, body), nil
}

// UnpackUpdate parses an UPDATE message body.
// This performs minimal parsing - just extracts the three sections.
// Detailed attribute and NLRI parsing is done lazily.
func UnpackUpdate(data []byte) (*Update, error) {
	if len(data) < 4 {
		return nil, ErrShortRead
	}

	// Withdrawn Routes Length
	withdrawnLen := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+withdrawnLen+2 {
		return nil, ErrShortRead
	}

	// Path Attributes Length
	attrOffset := 2 + withdrawnLen
	attrLen := int(binary.BigEndian.Uint16(data[attrOffset : attrOffset+2]))
	if len(data) < attrOffset+2+attrLen {
		return nil, ErrShortRead
	}

	// NLRI is the remainder
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
// End-of-RIB is an UPDATE with no withdrawn routes, no attributes, and no NLRI.
func (u *Update) IsEndOfRIB() bool {
	return len(u.WithdrawnRoutes) == 0 &&
		len(u.PathAttributes) == 0 &&
		len(u.NLRI) == 0
}
