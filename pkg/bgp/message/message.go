package message

// Message is the interface all BGP messages implement.
type Message interface {
	// Type returns the BGP message type.
	Type() MessageType

	// Pack serializes the message to wire format (including header).
	Pack(neg *Negotiated) ([]byte, error)
}

// Negotiated holds the result of capability negotiation between peers.
// Used during packing/unpacking to handle capability-dependent formats.
type Negotiated struct {
	// ASN4 indicates 4-byte AS number support (RFC 6793)
	ASN4 bool

	// AddPath indicates ADD-PATH support per family (RFC 7911)
	AddPath map[Family]bool

	// ExtendedMessage indicates extended message support (RFC 8654)
	ExtendedMessage bool

	// LocalAS is our AS number
	LocalAS uint32

	// PeerAS is the peer's AS number
	PeerAS uint32

	// HoldTime is the negotiated hold time
	HoldTime uint16
}

// Family represents an address family (AFI/SAFI combination).
type Family struct {
	AFI  uint16
	SAFI uint8
}

// packWithHeader creates a complete message with header.
func packWithHeader(msgType MessageType, body []byte) []byte {
	totalLen := HeaderLen + len(body)
	data := make([]byte, totalLen)

	// Marker
	for i := 0; i < MarkerLen; i++ {
		data[i] = 0xFF
	}

	// Length
	data[16] = byte(totalLen >> 8)
	data[17] = byte(totalLen)

	// Type
	data[18] = byte(msgType)

	// Body
	copy(data[HeaderLen:], body)

	return data
}
