// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation

package peer

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/chaos/scenario"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	evpn "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/nlri/evpn"
	flowspec "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/nlri/flowspec"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
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

// BuildRoute constructs a serialized unicast UPDATE for a single IPv4 or IPv6 prefix.
// IPv4 prefixes use the UPDATE NLRI field; IPv6 uses MP_REACH_NLRI.
func (s *Sender) BuildRoute(prefix netip.Prefix) []byte {
	params := message.UnicastParams{
		Prefix:  prefix,
		NextHop: s.nextHop,
		Origin:  attribute.OriginIGP,
	}

	update := s.builder.BuildUnicast(&params)
	if update == nil {
		return nil
	}

	return SerializeMessage(update)
}

// BuildMulticastRoute constructs a serialized multicast UPDATE for a single prefix.
func (s *Sender) BuildMulticastRoute(prefix netip.Prefix) []byte {
	params := message.UnicastParams{
		Prefix:  prefix,
		NextHop: s.nextHop,
		Origin:  attribute.OriginIGP,
		SAFI:    attribute.SAFIMulticast,
	}

	update := s.builder.BuildUnicast(&params)
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

// BuildVPNRoute constructs a serialized VPN UPDATE for a single VPN route.
func (s *Sender) BuildVPNRoute(route scenario.VPNRoute) []byte {
	params := message.VPNParams{
		Prefix:  route.Prefix,
		NextHop: s.nextHop,
		Labels:  route.Labels,
		RDBytes: route.RDBytes,
		Origin:  attribute.OriginIGP,
	}

	update := s.builder.BuildVPN(&params)
	if update == nil {
		return nil
	}

	return SerializeMessage(update)
}

// BuildEVPNRoute constructs a serialized EVPN Type-2 UPDATE for a single route.
func (s *Sender) BuildEVPNRoute(route scenario.EVPNRoute) []byte {
	rd := evpn.RouteDistinguisher{
		Type:  nlri.RDType(uint16(route.RDBytes[0])<<8 | uint16(route.RDBytes[1])),
		Value: [6]byte(route.RDBytes[2:]),
	}

	evpnNLRI := evpn.NewEVPNType2(rd, [10]byte{}, route.EthernetTag, route.MAC, route.IP, route.Labels)
	params := message.EVPNParams{
		NLRI:    evpnNLRI.Bytes(),
		NextHop: s.nextHop,
		Origin:  attribute.OriginIGP,
	}

	update := s.builder.BuildEVPN(params)
	if update == nil {
		return nil
	}

	return SerializeMessage(update)
}

// BuildFlowSpecRoute constructs a serialized FlowSpec UPDATE for a single route.
func (s *Sender) BuildFlowSpecRoute(route scenario.FlowSpecRoute) []byte {
	var family flowspec.Family
	if route.IsIPv6 {
		family = flowspec.Family{AFI: flowspec.AFIIPv6, SAFI: flowspec.SAFIFlowSpec}
	} else {
		family = flowspec.Family{AFI: flowspec.AFIIPv4, SAFI: flowspec.SAFIFlowSpec}
	}

	fs := flowspec.NewFlowSpec(family)
	fs.AddComponent(flowspec.NewFlowDestPrefixComponent(route.DestPrefix))
	fs.AddComponent(flowspec.NewFlowSourcePrefixComponent(route.SourcePrefix))

	params := message.FlowSpecParams{
		IsIPv6:  route.IsIPv6,
		NLRI:    fs.Bytes(),
		NextHop: s.nextHop,
	}

	update := s.builder.BuildFlowSpec(params)
	if update == nil {
		return nil
	}

	return SerializeMessage(update)
}

// BuildEOR constructs a serialized End-of-RIB marker for the given family.
// RFC 4724: IPv4 unicast EOR is an empty UPDATE; others use MP_UNREACH_NLRI.
//
// The family name is resolved via family.LookupFamily; unregistered names
// return nil so callers emit no EOR (matching the previous behavior when
// the hardcoded lookup table did not cover the entry).
func BuildEOR(name string) []byte {
	fam, ok := family.LookupFamily(name)
	if !ok {
		return nil
	}
	eor := message.BuildEOR(fam)
	return SerializeMessage(eor)
}

// BuildEORIPv4Unicast constructs a serialized End-of-RIB marker for ipv4/unicast.
// RFC 4724: IPv4 unicast EOR is an empty UPDATE (no attributes, no NLRI).
//
// Deprecated: Use BuildEOR("ipv4/unicast") instead.
func BuildEORIPv4Unicast() []byte {
	eor := message.BuildEOR(family.IPv4Unicast)
	return SerializeMessage(eor)
}
