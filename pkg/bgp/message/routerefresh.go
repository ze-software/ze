package message

import "encoding/binary"

// RouteRefresh represents a BGP ROUTE-REFRESH message (RFC 2918).
type RouteRefresh struct {
	AFI  AFI
	SAFI SAFI
}

// Type returns the message type (ROUTE-REFRESH).
func (r *RouteRefresh) Type() MessageType {
	return TypeROUTEREFRESH
}

// Pack serializes the ROUTE-REFRESH to wire format.
func (r *RouteRefresh) Pack(neg *Negotiated) ([]byte, error) {
	body := make([]byte, 4)
	binary.BigEndian.PutUint16(body[0:2], uint16(r.AFI))
	body[2] = 0 // Reserved
	body[3] = byte(r.SAFI)
	return packWithHeader(TypeROUTEREFRESH, body), nil
}

// UnpackRouteRefresh parses a ROUTE-REFRESH message body.
func UnpackRouteRefresh(data []byte) (*RouteRefresh, error) {
	if len(data) < 4 {
		return nil, ErrShortRead
	}
	return &RouteRefresh{
		AFI:  AFI(binary.BigEndian.Uint16(data[0:2])),
		SAFI: SAFI(data[3]),
	}, nil
}
