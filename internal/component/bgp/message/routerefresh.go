// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc2918.md — route refresh message format
// RFC: rfc/short/rfc7313.md — enhanced route refresh
// Overview: message.go — Message interface and writeHeader
// Related: open.go — OPEN message parsing and encoding
// Related: notification.go — NOTIFICATION message parsing and encoding
// Related: keepalive.go — KEEPALIVE message encoding
// Related: update.go — UPDATE message wire representation

package message

import (
	"encoding/binary"
)

// RouteRefreshSubtype represents the message subtype for ROUTE-REFRESH.
// RFC 7313 Section 3.2 - Message Subtype values.
type RouteRefreshSubtype uint8

const (
	// RouteRefreshNormal is the normal route refresh request (RFC 2918, RFC 7313 Section 3.2).
	RouteRefreshNormal RouteRefreshSubtype = 0

	// RouteRefreshBoRR marks the beginning of a route refresh operation (RFC 7313 Section 3.2).
	RouteRefreshBoRR RouteRefreshSubtype = 1

	// RouteRefreshEoRR marks the ending of a route refresh operation (RFC 7313 Section 3.2).
	RouteRefreshEoRR RouteRefreshSubtype = 2
)

// RFC 2918 Section 3 - ROUTE-REFRESH Message Format:
//
//	0       7      15      23      31
//	+-------+-------+-------+-------+
//	|      AFI      | Res.  | SAFI  |
//	+-------+-------+-------+-------+
//
// RFC 7313 Section 3.2 - The "Reserved" field is redefined as "Message Subtype".
//
// RouteRefresh represents a BGP ROUTE-REFRESH message (RFC 2918, RFC 7313).
type RouteRefresh struct {
	// RFC 2918 Section 3 - AFI: Address Family Identifier (16 bit)
	AFI AFI
	// RFC 7313 Section 3.2 - Subtype: Message Subtype (was Reserved in RFC 2918)
	Subtype RouteRefreshSubtype
	// RFC 2918 Section 3 - SAFI: Subsequent Address Family Identifier (8 bit)
	SAFI SAFI
}

// RFC 2918 Section 3 - Type: 5 - ROUTE-REFRESH
// Type returns the message type (ROUTE-REFRESH).
func (r *RouteRefresh) Type() MessageType {
	return TypeROUTEREFRESH
}

// Len returns the total message length in bytes.
// RFC 2918 Section 3 - Header (19) + AFI (2) + Reserved/Subtype (1) + SAFI (1) = 23.
// Context is ignored (context-independent).
func (r *RouteRefresh) Len(_ *EncodingContext) int {
	return HeaderLen + 4
}

// WriteTo writes the complete ROUTE-REFRESH message to buf at offset.
// Returns number of bytes written.
// RFC 2918 Section 3 - Message Format.
func (r *RouteRefresh) WriteTo(buf []byte, off int, _ *EncodingContext) int {
	totalLen := HeaderLen + 4
	writeHeader(buf, off, TypeROUTEREFRESH, totalLen)

	// Body: AFI (2) + Subtype (1) + SAFI (1)
	binary.BigEndian.PutUint16(buf[off+HeaderLen:], uint16(r.AFI))
	buf[off+HeaderLen+2] = byte(r.Subtype)
	buf[off+HeaderLen+3] = byte(r.SAFI)

	return totalLen
}

// UnpackRouteRefresh parses a ROUTE-REFRESH message body.
// RFC 2918 Section 3 - Message Format: One <AFI, SAFI> encoded as 4 bytes.
// RFC 7313 Section 3.2 - Reserved field is now Message Subtype.
func UnpackRouteRefresh(data []byte) (*RouteRefresh, error) {
	if len(data) < 4 {
		return nil, ErrShortRead
	}
	return &RouteRefresh{
		// RFC 2918 Section 3 - AFI: Address Family Identifier (16 bit)
		AFI: AFI(binary.BigEndian.Uint16(data[0:2])),
		// RFC 7313 Section 3.2 - Message Subtype
		Subtype: RouteRefreshSubtype(data[2]),
		// RFC 2918 Section 3 - SAFI: Subsequent Address Family Identifier (8 bit)
		SAFI: SAFI(data[3]),
	}, nil
}
