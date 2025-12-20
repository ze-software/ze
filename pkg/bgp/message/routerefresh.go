package message

import "encoding/binary"

// RFC 2918 Section 3 - ROUTE-REFRESH Message Format:
//
//	0       7      15      23      31
//	+-------+-------+-------+-------+
//	|      AFI      | Res.  | SAFI  |
//	+-------+-------+-------+-------+
//
// RouteRefresh represents a BGP ROUTE-REFRESH message (RFC 2918).
type RouteRefresh struct {
	// RFC 2918 Section 3 - AFI: Address Family Identifier (16 bit)
	AFI AFI
	// RFC 2918 Section 3 - SAFI: Subsequent Address Family Identifier (8 bit)
	SAFI SAFI
}

// RFC 2918 Section 3 - Type: 5 - ROUTE-REFRESH
// Type returns the message type (ROUTE-REFRESH).
func (r *RouteRefresh) Type() MessageType {
	return TypeROUTEREFRESH
}

// RFC 2918 Section 3 - Message Format: One <AFI, SAFI> encoded as 4 bytes
// Pack serializes the ROUTE-REFRESH to wire format.
func (r *RouteRefresh) Pack(neg *Negotiated) ([]byte, error) {
	body := make([]byte, 4)
	// RFC 2918 Section 3 - AFI: Address Family Identifier (16 bit)
	binary.BigEndian.PutUint16(body[0:2], uint16(r.AFI))
	// RFC 2918 Section 3 - Res.: Reserved (8 bit) field. Should be set to 0 by the sender
	body[2] = 0
	// RFC 2918 Section 3 - SAFI: Subsequent Address Family Identifier (8 bit)
	body[3] = byte(r.SAFI)
	return packWithHeader(TypeROUTEREFRESH, body), nil
}

// RFC 2918 Section 3 - Message Format: One <AFI, SAFI> encoded as 4 bytes
// UnpackRouteRefresh parses a ROUTE-REFRESH message body.
// RFC 2918 Section 3 - Res.: Reserved field is ignored by the receiver
func UnpackRouteRefresh(data []byte) (*RouteRefresh, error) {
	if len(data) < 4 {
		return nil, ErrShortRead
	}
	return &RouteRefresh{
		// RFC 2918 Section 3 - AFI: Address Family Identifier (16 bit)
		AFI: AFI(binary.BigEndian.Uint16(data[0:2])),
		// RFC 2918 Section 3 - SAFI: Subsequent Address Family Identifier (8 bit)
		SAFI: SAFI(data[3]),
	}, nil
}
