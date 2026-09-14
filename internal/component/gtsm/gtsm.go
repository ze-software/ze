// Design: rfc/short/rfc5082.md -- GTSM, the TTL 255 rule for related ICMP messages
// Overview: route_linux.go -- the RTAX_HOPLIMIT host route this package installs
// Related: internal/core/network/ttl_linux.go -- the socket options that carry GTSM for the main protocol packets
//
// RFC 5082 Section 3: "The TTL field in all IP packets used for transmission
// of messages associated with GTSM-enabled protocol sessions MUST be set to
// 255. This also applies to the related ICMP error handling messages."
// Section 6.1: "This specification mandates setting and verifying TTL=255 of
// those as well as the main protocol packets."
//
// The socket options ze sets on a peer connection do not reach a related ICMP
// message in either direction, so this package installs the two pieces of
// kernel state that do.
//
// Transmit, both families: a locally generated ICMP error takes its TTL from
// the route to its destination, not from the protocol socket. A host route to
// the peer carrying the RTAX_HOPLIMIT metric is what makes the error leave
// with 255 (route_linux.go).
//
// Receive, IPv4: the kernel compares IP_MINTTL against the TTL QUOTED inside
// the ICMP payload, which the sender of the error chooses, so the socket
// option cannot gate the error's own TTL. An nftables input rule does, and
// this package builds it.
//
// Receive, IPv6: nothing here. The kernel compares the ICMPv6 error's own hop
// limit against IPV6_MINHOPCOUNT, which network.SetIPMinTTL already installs.
//
// Safe for concurrent use: SetPeers serializes on a package mutex.
package gtsm

import (
	"fmt"
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// filterTableName carries the "ze_" ownership prefix the firewall backend
	// uses to recognize a ze-managed kernel table. RegisterTables refuses a
	// name without it.
	filterTableName = "ze_gtsm"
	// filterChainName is the base chain at the input hook, where a packet
	// addressed to this router's control plane arrives.
	filterChainName = "input"
	// filterOwner is this package's identity in the firewall table registry.
	filterOwner = "gtsm"
)

// The ICMPv4 types that carry a quoted datagram and are related messages of
// the session the quoted datagram belongs to (RFC 792). The first four are the
// types the TCP stack acts on; a redirect is included because acting on a
// spoofed one diverts the session's packets, which is the same attack the TTL
// check exists to stop.
var relatedICMPTypes = [...]uint8{
	3,  // Destination Unreachable
	4,  // Source Quench
	5,  // Redirect
	11, // Time Exceeded
	12, // Parameter Problem
}

// The two sides of the quoted TCP header. Ze dials out with an ephemeral
// source port and the peer's port as the destination, and answers an inbound
// connection with the two reversed, so a quoted header of either session
// carries the peer's BGP port on one side or the other.
var quotedPortSides = [...]firewall.QuotedPortSide{
	firewall.QuotedPortSource,
	firewall.QuotedPortDestination,
}

// Peer is one GTSM-enabled protocol session's kernel state, as the caller's
// configuration defines it.
//
// HopLimit is the TTL the session transmits with, which RFC 5082 Section 3
// fixes at 255. Floor is the lowest TTL the session accepts on receive, which
// is 255 for a single-hop session and 255-N+1 for a session configured N hops
// away. Both are zero when the caller's configuration enables no GTSM for the
// peer, and a zero in either field means "install nothing for this half"
// rather than "install the default".
type Peer struct {
	Addr     netip.Addr
	Port     uint16
	HopLimit uint8
	Floor    uint8
}

var (
	mu      sync.Mutex
	current []Peer
)

// SetPeers reconciles the kernel to the GTSM peer set it is given. The caller
// passes the WHOLE set on every call: a peer absent from it has its host route
// and its filter terms withdrawn, and an empty set withdraws everything.
//
// It is synchronous. The caller's config apply is what waits, and that is
// deliberate: a filter published after the session it protects is the window
// the rule exists to close.
func SetPeers(peers []Peer) error {
	mu.Lock()
	defer mu.Unlock()

	wanted := sortedPeers(peers)
	if slices.Equal(wanted, current) {
		return nil
	}

	// The routes go first. A peer that gains a route before its filter is
	// protected in one direction rather than neither, and the order is fixed
	// so a failure leaves a state a later reconcile can still reach.
	if err := applyRoutes(wanted, current); err != nil {
		return fmt.Errorf("gtsm routes: %w", err)
	}
	if err := publishFilter(filterTables(wanted)); err != nil {
		return fmt.Errorf("gtsm filter: %w", err)
	}

	current = wanted
	return nil
}

// sortedPeers copies the caller's slice and sorts it by address, so the kernel
// rule order is stable across config applies and two equal sets compare equal.
func sortedPeers(peers []Peer) []Peer {
	wanted := make([]Peer, 0, len(peers))
	for _, p := range peers {
		if !p.Addr.IsValid() {
			continue
		}
		wanted = append(wanted, p)
	}
	slices.SortFunc(wanted, func(a, b Peer) int { return a.Addr.Compare(b.Addr) })
	return slices.Compact(wanted)
}

// applyRoutes and publishFilter are the two kernel writers this package has.
//
// They are vars so a test can read what this package asks the kernel for
// without a kernel to ask, which is the seam vrrp's accept filter already uses.
// The tagged Linux proofs drive the real ones.
var applyRoutes = applyHopLimitRoutes

// publishFilter hands the desired tables to the firewall component, which owns
// the kernel.
var publishFilter = func(tables []firewall.Table) error {
	if err := firewall.RegisterTables(filterOwner, tables); err != nil {
		return err
	}
	return firewall.ApplyAll()
}

// filterTables builds the nftables table that drops a Dangerous related ICMP
// message, which RFC 5082 Section 3 permits: "MAY drop packets classified as
// Dangerous". Ze already drops a Dangerous main packet, through the IP_MINTTL
// floor, so the related message takes the same answer.
//
// It returns no table when no peer owes terms, so the last peer to lose its
// GTSM configuration withdraws the kernel table rather than leaving an empty
// one behind. A deployment with no GTSM peer never loads the firewall backend.
func filterTables(peers []Peer) []firewall.Table {
	terms := make([]firewall.Term, 0, len(peers)*len(relatedICMPTypes)*len(quotedPortSides))
	for _, p := range peers {
		terms = append(terms, peerTerms(p)...)
	}
	if len(terms) == 0 {
		return nil
	}

	return []firewall.Table{{
		Name:   filterTableName,
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:   filterChainName,
			IsBase: true,
			Type:   firewall.ChainFilter,
			Hook:   firewall.HookInput,
			// Ordinary filter priority. A base chain that accepts a packet
			// does not stop another base chain at the same hook from running,
			// so this chain's drop stands wherever it sits relative to the
			// operator's own rules.
			Priority: 0,
			// Accept, so this table decides only what it names. A drop policy
			// would refuse every packet these terms do not mention.
			Policy: firewall.PolicyAccept,
			Terms:  terms,
		}},
	}}
}

// peerTerms builds one peer's drop terms: one for each related ICMP type, for
// each side of the quoted TCP header.
//
// Every term requires the error to QUOTE a TCP header carrying this peer's BGP
// port. That is what associates the message with the GTSM-enabled session, and
// RFC 5082 Section 3 requires it: an error the peer sends about any other flow
// belongs to no GTSM session, so it is Unknown, and implementations "MUST NOT
// drop (as part of GTSM processing) packets classified as Trusted or Unknown".
//
// The term count is 10 for each peer that enables GTSM, bounded by the peer
// count the operator configured and never by anything a peer sends.
//
// IPv4 only. The IPv6 half of the receive rule is performed by the kernel
// against the IPV6_MINHOPCOUNT that network.SetIPMinTTL installs, so a rule
// here would be a second answer to a question already answered.
func peerTerms(p Peer) []firewall.Term {
	if !p.Addr.Is4() || p.Floor == 0 || p.Port == 0 {
		return nil
	}

	host := netip.PrefixFrom(p.Addr, p.Addr.BitLen())
	terms := make([]firewall.Term, 0, len(relatedICMPTypes)*len(quotedPortSides))
	for _, icmpType := range relatedICMPTypes {
		for _, side := range quotedPortSides {
			terms = append(terms, firewall.Term{
				Name: termName(p.Addr, icmpType, side),
				Matches: []firewall.Match{
					firewall.MatchICMPType{Type: icmpType},
					firewall.MatchSourceAddress{Prefix: host},
					firewall.MatchICMPErrorQuotedTCPPort{Port: p.Port, Side: side},
					firewall.MatchIPv4TTLBelow{Floor: p.Floor},
				},
				Actions: []firewall.Action{firewall.Drop{}},
			})
		}
	}
	return terms
}

// termName names one drop rule. A firewall name accepts letters, digits, '-',
// '_' and '.', and refuses a leading '-' or '.' (firewall.ValidateName), so
// the dots of an address become hyphens and the name opens with a letter.
func termName(addr netip.Addr, icmpType uint8, side firewall.QuotedPortSide) string {
	text := textbuf.StringAddr(addr)

	var tb textbuf.Buffer
	tb.Str("dangerous-")
	for i := range len(text) {
		c := text[i]
		if c == '.' || c == ':' {
			c = '-'
		}
		tb.Byte(c)
	}
	tb.Str("-type").Uint(uint64(icmpType))
	if side == firewall.QuotedPortSource {
		tb.Str("-quoted-source")
		return tb.String()
	}
	tb.Str("-quoted-destination")
	return tb.String()
}
