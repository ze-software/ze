package message

// Keepalive represents a BGP KEEPALIVE message (RFC 4271 Section 4.4).
// KEEPALIVE messages have no body - just the header.
type Keepalive struct{}

// Singleton instance - KEEPALIVE is stateless.
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
// KEEPALIVE has no body, so this returns just the header.
func (k *Keepalive) Pack(neg *Negotiated) ([]byte, error) {
	return packWithHeader(TypeKEEPALIVE, nil), nil
}

// UnpackKeepalive parses a KEEPALIVE message body.
// The body should be empty; any extra data is ignored.
func UnpackKeepalive(data []byte) (*Keepalive, error) {
	// KEEPALIVE has no body - return singleton
	return keepaliveSingleton, nil
}
