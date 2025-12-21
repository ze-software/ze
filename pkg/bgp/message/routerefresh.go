package message

import "encoding/binary"

// RouteRefreshSubtype represents the message subtype for ROUTE-REFRESH.
// RFC 7313 Section 3.2 - Message Subtype values.
type RouteRefreshSubtype uint8

const (
	// RouteRefreshNormal is the normal route refresh request (RFC 2918).
	// RFC 7313 Section 3.2: "0 - Normal route refresh request"
	RouteRefreshNormal RouteRefreshSubtype = 0

	// RouteRefreshBoRR marks the beginning of a route refresh operation.
	// RFC 7313 Section 3.2: "1 - Demarcation of the beginning of a route refresh (BoRR)"
	RouteRefreshBoRR RouteRefreshSubtype = 1

	// RouteRefreshEoRR marks the ending of a route refresh operation.
	// RFC 7313 Section 3.2: "2 - Demarcation of the ending of a route refresh (EoRR)"
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

// Pack serializes the ROUTE-REFRESH to wire format.
// RFC 2918 Section 3 - Message Format: One <AFI, SAFI> encoded as 4 bytes.
// RFC 7313 Section 3.2 - Reserved field is now Message Subtype.
func (r *RouteRefresh) Pack(neg *Negotiated) ([]byte, error) {
	body := make([]byte, 4)
	// RFC 2918 Section 3 - AFI: Address Family Identifier (16 bit)
	binary.BigEndian.PutUint16(body[0:2], uint16(r.AFI))
	// RFC 7313 Section 3.2 - Message Subtype (was Reserved in RFC 2918)
	body[2] = byte(r.Subtype)
	// RFC 2918 Section 3 - SAFI: Subsequent Address Family Identifier (8 bit)
	body[3] = byte(r.SAFI)
	return packWithHeader(TypeROUTEREFRESH, body), nil
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
