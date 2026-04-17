// Design: docs/research/l2tpv2-ze-integration.md -- PPP -> IP handler event boundary
// Related: events.go -- lifecycle events sealed sum (EventsOut())
// Related: manager.go -- Driver owns IPEventsOut() and IPResponse()
// Related: session.go -- per-session ipRespCh

package ppp

import "net/netip"

// AddressFamily names the IP family a single NCP negotiates. Only
// IPv4 and IPv6 are in scope; IPX and AppleTalk NCPs (RFC 1552 /
// RFC 1378) are not implemented.
type AddressFamily uint8

const (
	AddressFamilyIPv4 AddressFamily = iota
	AddressFamilyIPv6
)

// String returns the lowercase family name used in logs and event
// JSON. Panics on an unregistered value: every AddressFamily const
// MUST appear here; drift is a programmer error.
func (f AddressFamily) String() string {
	switch f {
	case AddressFamilyIPv4:
		return "ipv4"
	case AddressFamilyIPv6:
		return "ipv6"
	}
	panic("BUG: unknown AddressFamily")
}

// IPEvent is the sealed sum emitted on Driver.IPEventsOut(). The
// consumer (l2tp-pool plugin in production, a test responder in unit
// tests) reads EventIPRequest, picks addresses / DNS servers, and
// calls Driver.IPResponse to unblock the session goroutine.
//
// Separated from Event (lifecycle) and AuthEvent so an operator can
// subscribe to one plugin's concern without being forced to pattern-
// match every message PPP emits. Mirrors the auth-channel split
// introduced in spec-l2tp-6b-auth.
type IPEvent interface {
	isIPEvent()
}

// EventIPRequest is the request to allocate an IP address (and, for
// IPv4, optional DNS server hints) to the peer. The consumer MUST
// call Driver.IPResponse(TunnelID, SessionID, Family, ...) within
// the session's configured ip-timeout or the session tears down.
//
// SuggestedLocal / SuggestedPeer are populated when the peer has
// carried non-zero values in an earlier IPCP exchange, so the
// consumer can bias toward the peer's preference on renegotiation.
// In the LNS-role initial flow (ze assigns first) all suggestion
// fields are zero and the handler picks freely from its pool.
//
// For family=ipv6 SuggestedInterfaceID is the peer's proposed 8-byte
// identifier (zero on the first request); the handler may return a
// non-zero PeerInterfaceID to force a specific value or zero to
// accept the peer's offer.
type EventIPRequest struct {
	TunnelID             uint16
	SessionID            uint16
	Family               AddressFamily
	SuggestedLocal       netip.Addr
	SuggestedPeer        netip.Addr
	SuggestedInterfaceID [8]byte
}

func (EventIPRequest) isIPEvent() {}

// ipResponseMsg carries the consumer's decision from
// Driver.IPResponse into the per-session goroutine via ipRespCh.
// Zero netip.Addr for Local / Peer / DNSPrimary / DNSSecondary means
// "option absent"; for family=ipv4 Local and Peer MUST both be set
// on accept. For family=ipv6 hasPeerInterface == true forces a
// specific peer interface-id; false accepts the peer's choice.
type ipResponseMsg struct {
	accept           bool
	family           AddressFamily
	reason           string
	local            netip.Addr
	peer             netip.Addr
	dnsPrimary       netip.Addr
	dnsSecondary     netip.Addr
	peerInterfaceID  [8]byte
	hasPeerInterface bool
}

// Compile-time exhaustiveness pin. Bumping the array length without
// updating every consumer's switch produces a build failure at the
// consumer rather than here.
var _ = [1]IPEvent{
	EventIPRequest{},
}
