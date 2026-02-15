package peer

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// SenderConfig holds the parameters for building UPDATE messages.
type SenderConfig struct {
	// ASN is the local autonomous system number.
	ASN uint32

	// IsIBGP indicates whether this is an iBGP session.
	IsIBGP bool

	// NextHop is the next-hop address for announced routes.
	NextHop netip.Addr
}

// Sender builds UPDATE messages for route announcements.
type Sender struct {
	builder *message.UpdateBuilder
	nextHop netip.Addr
}

// NewSender creates a new Sender with the given config.
func NewSender(cfg SenderConfig) *Sender {
	return &Sender{
		builder: message.NewUpdateBuilder(cfg.ASN, cfg.IsIBGP, true, false),
		nextHop: cfg.NextHop,
	}
}

// BuildRoute constructs a serialized ipv4/unicast UPDATE for a single prefix.
func (s *Sender) BuildRoute(prefix netip.Prefix) []byte {
	params := message.UnicastParams{
		Prefix:  prefix,
		NextHop: s.nextHop,
		Origin:  attribute.OriginIGP,
	}

	update := s.builder.BuildUnicast(params)
	if update == nil {
		return nil
	}

	return SerializeMessage(update)
}

// BuildWithdrawal constructs a serialized IPv4/unicast withdrawal UPDATE.
// RFC 4271 Section 4.3: withdrawals use the Withdrawn Routes field with no
// path attributes and no NLRI. Returns nil for an empty prefix list.
func BuildWithdrawal(prefixes []netip.Prefix) []byte {
	if len(prefixes) == 0 {
		return nil
	}

	// Encode prefixes into wire format: [prefix_len_bits, addr_bytes...] per prefix.
	var withdrawn []byte
	for _, p := range prefixes {
		bits := p.Bits()
		byteLen := (bits + 7) / 8
		addr := p.Addr().As4()
		withdrawn = append(withdrawn, byte(bits))
		withdrawn = append(withdrawn, addr[:byteLen]...)
	}

	update := &message.Update{WithdrawnRoutes: withdrawn}
	return SerializeMessage(update)
}

// BuildMalformedUpdate constructs a BGP UPDATE with an invalid ORIGIN attribute.
// ORIGIN must be 0 (IGP), 1 (EGP), or 2 (INCOMPLETE); value 0xFF is invalid.
// This tests RFC 7606 revised error handling (treat-as-withdraw).
func BuildMalformedUpdate() []byte {
	// UPDATE body: withdrawn=0, one attribute with invalid ORIGIN value.
	body := []byte{
		0x00, 0x00, // withdrawn routes length = 0
		0x00, 0x04, // total path attribute length = 4
		0x40, 0x01, // ORIGIN: flags=transitive, type code=1
		0x01, // attribute length = 1
		0xFF, // INVALID origin value (valid: 0, 1, 2)
	}

	msgLen := message.HeaderLen + len(body)
	msg := make([]byte, msgLen)

	// BGP marker (16 bytes of 0xFF).
	for i := range 16 {
		msg[i] = 0xFF
	}
	msg[16] = byte(msgLen >> 8)
	msg[17] = byte(msgLen)
	msg[18] = 2 // Type: UPDATE

	copy(msg[message.HeaderLen:], body)

	return msg
}

// BuildEORIPv4Unicast constructs a serialized End-of-RIB marker for ipv4/unicast.
// RFC 4724: IPv4 unicast EOR is an empty UPDATE (no attributes, no NLRI).
func BuildEORIPv4Unicast() []byte {
	eor := message.BuildEOR(nlri.IPv4Unicast)
	return SerializeMessage(eor)
}
