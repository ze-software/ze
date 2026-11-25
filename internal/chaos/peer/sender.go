// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation

package peer

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/component/bgp/message"
	evpn "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/evpn"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/family"
)

// SenderConfig holds the parameters for building UPDATE messages.
type SenderConfig struct {
	// ASN is the local autonomous system number.
	ASN uint32

	// IsIBGP indicates whether this is an iBGP session.
	IsIBGP bool

	// NextHop is the next-hop address for announced routes.
	NextHop netip.Addr

	// RouteSet is the exact routes the peer announces, each with its own next
	// hop and AS path. Nil means every route is built from NextHop and the
	// path the session kind implies. With a set, BuildRoute answers only for
	// its prefixes, with the announcement the set holds for each.
	RouteSet *scenario.RouteSet
}

// Sender builds UPDATE messages for route announcements.
type Sender struct {
	builder *message.UpdateBuilder
	nextHop netip.Addr

	// routeSetMode says the peer announces a given route set. It is the one
	// test of the mode: announcements is empty for an empty set and for no set alike.
	routeSetMode bool
	// announcements maps each prefix of the route set to its announcement.
	announcements map[netip.Prefix]scenario.Announcement
}

// NewSender creates a new Sender with the given config.
func NewSender(cfg SenderConfig) *Sender {
	sender := &Sender{
		builder: message.NewUpdateBuilder(cfg.ASN, cfg.IsIBGP, true, false),
		nextHop: cfg.NextHop,
	}
	if cfg.RouteSet == nil {
		return sender
	}
	sender.routeSetMode = true
	sender.announcements = make(map[netip.Prefix]scenario.Announcement, len(cfg.RouteSet.Announcements))
	for _, announcement := range cfg.RouteSet.Announcements {
		sender.announcements[announcement.Prefix] = announcement
	}
	return sender
}

// BuildAnnouncement constructs the serialized UPDATE of one route of a given
// route set: the prefix with exactly the next hop and the AS path of
// announcement. On an iBGP session the path goes out as given and LOCAL_PREF is
// present. On an eBGP session the path goes out as given too, and it MUST
// already start with the speaker's own AS, because nothing is prepended.
//
// An announcement the sender cannot send as given is an error, never a route
// with another path.
func (s *Sender) BuildAnnouncement(announcement scenario.Announcement) ([]byte, error) {
	if err := s.announcementProblem(announcement); err != nil {
		return nil, err
	}
	params := message.UnicastParams{
		Prefix:  announcement.Prefix,
		NextHop: announcement.NextHop,
		Origin:  attribute.OriginIGP,
		ASPath:  announcement.ASPath,
	}
	update, err := s.builder.BuildUnicast(&params)
	if err != nil {
		return nil, fmt.Errorf("announcement of %s with next hop %s: %w", announcement.Prefix, announcement.NextHop, err)
	}
	return serializeMessage(update), nil
}

// announcementProblem answers why announcement cannot go out as given, or nil.
//
// On an eBGP session the builder prepends the local AS to a path that does not
// start with it, and sends the local AS alone for an empty path, which is the
// prepending RFC 4271 Section 5.1.2 asks of a speaker that originates or
// propagates a route. A given path is the whole path on the wire, so a path
// the builder would change is refused here.
func (s *Sender) announcementProblem(announcement scenario.Announcement) error {
	if !announcement.Prefix.IsValid() {
		return fmt.Errorf("announcement with next hop %s has no valid prefix: give it one", announcement.NextHop)
	}
	if !announcement.NextHop.IsValid() {
		return fmt.Errorf("announcement of %s has no valid next hop: give it one", announcement.Prefix)
	}
	if s.builder.IsIBGP {
		return nil
	}
	if len(announcement.ASPath) == 0 {
		return fmt.Errorf("announcement of %s has an empty AS path on an eBGP session of AS %d: the path must start with %d",
			announcement.Prefix, s.builder.LocalAS, s.builder.LocalAS)
	}
	if announcement.ASPath[0] != s.builder.LocalAS {
		return fmt.Errorf("announcement of %s has an AS path starting with %d on an eBGP session of AS %d: the path must start with %d",
			announcement.Prefix, announcement.ASPath[0], s.builder.LocalAS, s.builder.LocalAS)
	}
	return nil
}

// RouteSetProblem answers why the sender cannot announce its route set as
// given, naming one announcement it refuses, or nil. It builds every UPDATE of
// the set, so a set it accepts is one BuildRoute answers for whole. A sender
// with no route set has none.
func (s *Sender) RouteSetProblem() error {
	for _, announcement := range s.announcements {
		if _, err := s.BuildAnnouncement(announcement); err != nil {
			return err
		}
	}
	return nil
}

// BuildRoute constructs a serialized unicast UPDATE for a single IPv4 or IPv6 prefix.
// IPv4 prefixes use the UPDATE NLRI field; IPv6 uses MP_REACH_NLRI.
//
// With a route set, the UPDATE is the one BuildAnnouncement builds for the
// prefix, and a prefix the set does not hold gives nil: the peer has no route
// to announce for it.
func (s *Sender) BuildRoute(prefix netip.Prefix) []byte {
	if s.routeSetMode {
		announcement, held := s.announcements[prefix]
		if !held {
			return nil
		}
		data, err := s.BuildAnnouncement(announcement)
		if err != nil {
			return nil
		}
		return data
	}

	params := message.UnicastParams{
		Prefix:  prefix,
		NextHop: s.nextHop,
		Origin:  attribute.OriginIGP,
	}

	update, err := s.builder.BuildUnicast(&params)
	if err != nil {
		return nil
	}

	return serializeMessage(update)
}

// buildMulticastRoute constructs a serialized multicast UPDATE for a single prefix.
func (s *Sender) buildMulticastRoute(prefix netip.Prefix) []byte {
	params := message.UnicastParams{
		Prefix:  prefix,
		NextHop: s.nextHop,
		Origin:  attribute.OriginIGP,
		SAFI:    attribute.SAFIMulticast,
	}

	update, err := s.builder.BuildUnicast(&params)
	if err != nil {
		return nil
	}

	return serializeMessage(update)
}

// buildWithdrawal constructs a serialized IPv4/unicast withdrawal UPDATE.
// RFC 4271 Section 4.3: withdrawals use the Withdrawn Routes field with no
// path attributes and no NLRI. Returns nil for an empty prefix list.
//
//	0          19                 21                 21 + withdrawn length
//	| header   | withdrawn length | withdrawn routes | attribute length (0) |
func buildWithdrawal(prefixes []netip.Prefix) []byte {
	if len(prefixes) == 0 {
		return nil
	}

	// Encode prefixes into wire format: [prefix_len_bits, addr_bytes...] per prefix.
	var withdrawn []byte
	for _, p := range prefixes {
		bits := p.Bits()
		byteLen := withdrawalPrefixSize(p) - 1
		addr := p.Addr().As4()
		withdrawn = append(withdrawn, byte(bits))
		withdrawn = append(withdrawn, addr[:byteLen]...)
	}

	update := &message.Update{WithdrawnRoutes: withdrawn}
	return serializeMessage(update)
}

func withdrawalPrefixSize(prefix netip.Prefix) int {
	return 1 + (prefix.Bits()+7)/8
}

// withdrawalBatchSize counts how many IPv4 prefixes fit in a legacy UPDATE.
// RFC 4271 Section 4: "The maximum message size is 4096 octets."
func withdrawalBatchSize(prefixes []netip.Prefix) int {
	remaining := message.MaxMsgLen - message.MinUpdateLen
	for index, prefix := range prefixes {
		remaining -= withdrawalPrefixSize(prefix)
		if remaining < 0 {
			return index
		}
	}
	return len(prefixes)
}

// buildMalformedUpdate constructs a BGP UPDATE with an invalid ORIGIN attribute
// and a non-colliding IPv4 NLRI (10.255.255.0/24; 10/8 is excluded from seeded
// route generation). ORIGIN must be 0 (IGP), 1 (EGP), or 2 (INCOMPLETE); value
// 0xFF is invalid. Including a reachable NLRI exercises RFC 7606 treat-as-withdraw
// instead of escalating to RFC 7606 Section 5.2 session-reset (attrs with no NLRI).
func buildMalformedUpdate() []byte {
	// UPDATE body: withdrawn=0, invalid ORIGIN attribute (0xFF), NLRI=10.255.255.0/24.
	body := []byte{
		0x00, 0x00, // withdrawn routes length = 0
		0x00, 0x04, // total path attribute length = 4
		0x40, 0x01, // ORIGIN: flags=transitive, type code=1
		0x01,                   // attribute length = 1
		0xFF,                   // INVALID origin value (valid: 0, 1, 2)
		0x18, 0x0a, 0xff, 0xff, // NLRI: 10.255.255.0/24 (non-colliding 10/8)
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

// buildVPNRoute constructs a serialized VPN UPDATE for a single VPN route.
func (s *Sender) buildVPNRoute(route scenario.VPNRoute) []byte {
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

	return serializeMessage(update)
}

// buildEVPNRoute constructs a serialized EVPN Type-2 UPDATE for a single route.
func (s *Sender) buildEVPNRoute(route scenario.EVPNRoute) []byte {
	rd := evpn.RouteDistinguisher{
		Type:  nlri.RDType(uint16(route.RDBytes[0])<<8 | uint16(route.RDBytes[1])),
		Value: [6]byte(route.RDBytes[2:]),
	}

	evpnNLRI := evpn.NewEVPNType2(rd, [10]byte{}, route.EthernetTag, route.MAC, route.IP, route.Labels)
	// RFC 7432 Section 9.2.1 -- see rfc/short/rfc7432.md.
	// A generated MAC route needs an import target too. Use the scenario's
	// RD administrator and assigned value as its deterministic target.
	target := route.RDBytes
	target[0], target[1] = target[1], 2
	params := message.EVPNParams{
		NLRI:              evpnNLRI.Bytes(),
		NextHop:           s.nextHop,
		Origin:            attribute.OriginIGP,
		ExtCommunityBytes: target[:],
	}

	update, err := s.builder.BuildEVPN(params)
	if err != nil {
		return nil
	}

	return serializeMessage(update)
}

// buildFlowSpecRoute constructs a serialized FlowSpec UPDATE for a single route.
// It returns nil when the route cannot be encoded, and the caller then sends
// nothing for that route.
func (s *Sender) buildFlowSpecRoute(route scenario.FlowSpecRoute) []byte {
	var family flowspec.Family
	if route.IsIPv6 {
		family = flowspec.Family{AFI: flowspec.AFIIPv6, SAFI: flowspec.SAFIFlowSpec}
	} else {
		family = flowspec.Family{AFI: flowspec.AFIIPv4, SAFI: flowspec.SAFIFlowSpec}
	}

	fs := flowspec.NewFlowSpec(family)
	if err := fs.AddComponent(flowspec.NewFlowDestPrefixComponent(route.DestPrefix)); err != nil {
		return nil
	}
	if err := fs.AddComponent(flowspec.NewFlowSourcePrefixComponent(route.SourcePrefix)); err != nil {
		return nil
	}

	params := message.FlowSpecParams{
		IsIPv6:  route.IsIPv6,
		NLRI:    fs.Bytes(),
		NextHop: s.nextHop,
	}

	update := s.builder.BuildFlowSpec(params)
	if update == nil {
		return nil
	}

	return serializeMessage(update)
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
	return serializeMessage(eor)
}
