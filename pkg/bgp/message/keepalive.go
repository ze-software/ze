package message

// RFC 4271 Section 4.4 - KEEPALIVE Message Format
//
// "A KEEPALIVE message consists of only the message header and has a
// length of 19 octets."
//
// "KEEPALIVE messages are exchanged between peers often enough not to
// cause the Hold Timer to expire."

// Keepalive represents a BGP KEEPALIVE message (RFC 4271 Section 4.4).
// RFC 4271 Section 4.4 - KEEPALIVE messages have no body, only the 19-octet header.
type Keepalive struct{}

// Singleton instance - KEEPALIVE is stateless.
// RFC 4271 Section 4.4 - All KEEPALIVE messages are identical (header only).
var keepaliveSingleton = &Keepalive{}

// NewKeepalive returns the singleton KEEPALIVE instance.
func NewKeepalive() *Keepalive {
	return keepaliveSingleton
}

// Type returns the message type (KEEPALIVE).
func (k *Keepalive) Type() MessageType {
	return TypeKEEPALIVE
}

// Pack serializes the KEEPALIVE to wire format.
// RFC 4271 Section 4.4 - "A KEEPALIVE message consists of only the message
// header and has a length of 19 octets."
// KEEPALIVE has no body, so this returns just the header.
func (k *Keepalive) Pack(neg *Negotiated) ([]byte, error) {
	return packWithHeader(TypeKEEPALIVE, nil), nil
}

// UnpackKeepalive parses a KEEPALIVE message body.
// RFC 4271 Section 4.4 - KEEPALIVE has no body; the message is header-only.
// RFC 4271 Section 4.4: "A KEEPALIVE message consists of only the message
// header and has a length of 19 octets."
// RFC 4271 Section 6.1: "if the Length field of a KEEPALIVE message is not
// equal to 19, then the Error Subcode MUST be set to Bad Message Length"
func UnpackKeepalive(data []byte) (*Keepalive, error) {
	// RFC 4271 Section 4.4 - KEEPALIVE MUST have no body
	if len(data) > 0 {
		// RFC 4271 Section 6.1 - Return Message Header Error / Bad Message Length
		return nil, &Notification{
			ErrorCode:    NotifyMessageHeader,
			ErrorSubcode: NotifyHeaderBadLength,
			// RFC 4271 Section 6.1: "Data field MUST contain the erroneous Length field"
			// The actual length is HeaderLen + len(data)
			Data: []byte{byte((HeaderLen + len(data)) >> 8), byte(HeaderLen + len(data))},
		}
	}
	return keepaliveSingleton, nil
}
