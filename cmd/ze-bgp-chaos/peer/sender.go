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

// BuildEORIPv4Unicast constructs a serialized End-of-RIB marker for ipv4/unicast.
// RFC 4724: IPv4 unicast EOR is an empty UPDATE (no attributes, no NLRI).
func BuildEORIPv4Unicast() []byte {
	eor := message.BuildEOR(nlri.IPv4Unicast)
	return SerializeMessage(eor)
}
