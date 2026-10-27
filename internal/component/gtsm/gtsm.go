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
// Receive, IPv6 related messages: nothing here. The kernel compares the
// ICMPv6 error's own hop limit against IPV6_MINHOPCOUNT, which
// network.SetIPMinTTL already installs.
//
// Receive, the main protocol packets before accept, both families: the socket
// floor sits on the accepted connection, so a SYN from a GTSM peer's address
// below the peer's floor would still cost a request socket and a SYN-ACK. A
// listener-wide floor cannot replace it, because one listener serves GTSM
// peers, multihop peers and unconfigured sources alike, and RFC 5082 Section 3
// forbids dropping the last two. So each GTSM peer, and each GTSM dynamic
// group's range, gets an input rule that names its source and its listen
// port. The per-connection floor stays as the second check.
//
// Safe for concurrent use: SetPeers serializes on a package mutex.
package gtsm

import (
	"errors"
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

// Range is one GTSM-enabled dynamic group's address range, as the caller's
// configuration defines it. A member of the group exists only once it has
// connected, and the SYN that creates it arrives before then, so the
// listen-port rule names the range rather than a member.
//
// Port is the listen port the group's sessions use and Floor the lowest TTL
// or hop limit they accept. A zero in either means the group asked for no
// GTSM, and the range then carries no rule.
type Range struct {
	Prefix netip.Prefix
	Port   uint16
	Floor  uint8
}

var (
	mu      sync.Mutex
	current []Peer
	// currentRanges is the dynamic-group half of the set current holds.
	currentRanges []Range
	// routeMissing says the last reconcile left a peer without its host
	// route, most often because the peer's interface was not up yet. It makes
	// the next reconcile run even when it names the same peers, which is what
	// the daemon's warning and the doctor check promise (doctor.go).
	routeMissing bool
	// published says SetPeers has run in this process, so current is the
	// set this daemon asked the kernel for. It stays false in a process that
	// only reads the kernel, such as `ze doctor`, and the doctor check reads
	// it to choose which question it can answer (doctor.go).
	published bool
)

// SetPeers reconciles the kernel to the GTSM peer set and the GTSM dynamic
// group ranges it is given. The caller passes the WHOLE set on every call: a
// peer or a range absent from it has its host route and its filter terms
// withdrawn, and two empty sets withdraw everything. A range carries no host
// route, only its listen-port term.
//
// It is synchronous. The caller's config apply is what waits, and that is
// deliberate: a filter published after the session it protects is the window
// the rule exists to close.
//
// The error names every peer whose route the kernel refused, and the filter
// is published whatever the routes answered: a peer short of its route is
// protected in one direction rather than neither. The reconcile that follows
// retries the route, even when it names the same peers, because nothing in
// the configuration changes when the peer's interface comes up.
func SetPeers(peers []Peer, ranges []Range) error {
	mu.Lock()
	defer mu.Unlock()

	published = true
	wanted := sortedPeers(peers)
	wantedRanges := sortedRanges(ranges)
	if slices.Equal(wanted, current) && slices.Equal(wantedRanges, currentRanges) && !routeMissing {
		return nil
	}

	// The routes go first, so the order is fixed and a failure leaves a state
	// a later reconcile can still reach.
	routeErr := applyRoutes(wanted, current)
	routeMissing = routeErr != nil
	if err := publishFilter(FilterTables(wanted, wantedRanges)); err != nil {
		// current stays as it was, so the next reconcile republishes.
		return errors.Join(routeErr, fmt.Errorf("gtsm filter: %w", err))
	}

	current = wanted
	currentRanges = wantedRanges
	return routeErr
}

// publishedPeers answers the peers and ranges this process last asked the
// kernel for, and whether it ever asked. The slices are copies, so a caller
// reads them outside the lock.
func publishedPeers() ([]Peer, []Range, bool) {
	mu.Lock()
	defer mu.Unlock()
	return slices.Clone(current), slices.Clone(currentRanges), published
}

// sortedRanges copies the caller's ranges, drops an invalid prefix, masks each
// prefix to its network, and sorts them, for the reason sortedPeers gives.
func sortedRanges(ranges []Range) []Range {
	wanted := make([]Range, 0, len(ranges))
	for _, r := range ranges {
		if !r.Prefix.IsValid() {
			continue
		}
		r.Prefix = r.Prefix.Masked()
		wanted = append(wanted, r)
	}
	slices.SortFunc(wanted, func(a, b Range) int {
		if c := a.Prefix.Addr().Compare(b.Prefix.Addr()); c != 0 {
			return c
		}
		return a.Prefix.Bits() - b.Prefix.Bits()
	})
	return slices.Compact(wanted)
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

// FilterTables builds the nftables table that drops a Dangerous packet of a
// GTSM-enabled session, which RFC 5082 Section 3 permits: "MAY drop packets
// classified as Dangerous." Two kinds of packet are named: a related ICMP
// message (peerTerms), and a TCP segment to the session's listen port that
// arrives below the floor before any accepted socket carries one
// (listenPortTerm), for each peer and for each dynamic group range.
//
// It returns no table when no peer and no range owes terms, so the last one to
// lose its GTSM configuration withdraws the kernel table rather than leaving
// an empty one behind. A deployment with no GTSM session never loads the
// firewall backend.
//
// It is exported so a publisher of the set (the BGP reactor) can prove,
// without a kernel, which tables the set it publishes yields; SetPeers is the
// only caller that installs them.
func FilterTables(peers []Peer, ranges []Range) []firewall.Table {
	terms := make([]firewall.Term, 0, len(peers)*(len(relatedICMPTypes)*len(quotedPortSides)+1)+len(ranges))
	for _, p := range peers {
		terms = append(terms, peerTerms(p)...)
	}
	for _, r := range ranges {
		if term, ok := listenPortTerm(r.Prefix, r.Port, r.Floor); ok {
			terms = append(terms, term)
		}
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

// peerTerms builds one peer's drop terms: its listen-port term, then, for an
// IPv4 peer, one related-ICMP term for each related ICMP type, for each side
// of the quoted TCP header.
//
// Every ICMP term requires the error to QUOTE an IPv4 header whose destination is
// this peer and a TCP header carrying this peer's BGP port. The quoted header
// is what associates the message with the GTSM-enabled session. An ICMP error
// is generated by whichever router met the problem, and the kernel delivers
// it by the header it quotes (tcp_v4_err), so no term reads the outer source.
// RFC 5082 Section 3 requires the association. An error about any other
// destination or any other flow belongs to no GTSM session, so it is Unknown,
// and implementations "MUST NOT drop (as part of GTSM processing) packets
// classified as Trusted or Unknown".
//
// The term count is 11 for each IPv4 peer that enables GTSM and 1 for each
// IPv6 one, bounded by the peer count the operator configured and never by
// anything a peer sends.
//
// The ICMP terms are IPv4 only. The IPv6 half of the related-message rule is
// performed by the kernel against the IPV6_MINHOPCOUNT that
// network.SetIPMinTTL installs, so a rule here would be a second answer to a
// question already answered.
func peerTerms(p Peer) []firewall.Term {
	listen, ok := listenPortTerm(netip.PrefixFrom(p.Addr, p.Addr.BitLen()), p.Port, p.Floor)
	// refuse-by-default: guards FilterTables -- a peer with no floor or port asked for no GTSM (A-5), so it owes no term, and a set of such peers withdraws ze_gtsm
	if !ok {
		return nil
	}
	if !p.Addr.Is4() {
		return []firewall.Term{listen}
	}

	terms := make([]firewall.Term, 0, 1+len(relatedICMPTypes)*len(quotedPortSides))
	terms = append(terms, listen)
	for _, icmpType := range relatedICMPTypes {
		for _, side := range quotedPortSides {
			terms = append(terms, firewall.Term{
				Name: termName(p.Addr, icmpType, side),
				Matches: []firewall.Match{
					firewall.MatchICMPType{Type: icmpType},
					firewall.MatchICMPErrorQuotedDestination{Addr: p.Addr},
					firewall.MatchICMPErrorQuotedTCPPort{Port: p.Port, Side: side},
					firewall.MatchIPv4TTLBelow{Floor: p.Floor},
				},
				Actions: []firewall.Action{firewall.Drop{}},
			})
		}
	}
	return terms
}

// listenPortTerm builds the term that drops a TCP segment from source to the
// session's Ze listen port whose TTL (IPv4) or hop limit (IPv6) is below
// floor. It answers false for a zero port or floor, which is a session that
// asked for no GTSM (A-5): a term built from either would match nothing.
//
// RFC 5082 Section 3: "MAY drop packets classified as Dangerous."
//
// The source, the protocol and the destination port are what classify the
// segment. RFC 5082 Section 3 calls Dangerous "packets that have been
// identified as belonging to one of the GTSM-enabled sessions, but their TTL
// values are NOT within the expected range, and hence GTSM believes there is
// a risk that these packets have been spoofed." A segment from any other
// source, or to any other port, belongs to no GTSM-enabled session, and
// implementations "MUST NOT drop (as part of GTSM processing) packets
// classified as Trusted or Unknown." So the term never matches on the TTL
// alone, and a below-floor segment from a non-GTSM peer or an unconfigured
// source still reaches the TCP stack.
//
// The rule sits at the input hook because RFC 5082 Section 2.2 says: "For
// maximal protection, ingress filtering should be applied before the packet
// goes through the scarce resource." The accepted socket's own floor
// (network.SetIPMinTTL) comes after the handshake has already cost a request
// socket and a SYN-ACK.
func listenPortTerm(source netip.Prefix, port uint16, floor uint8) (firewall.Term, bool) {
	if !source.IsValid() {
		return firewall.Term{}, false
	}
	if port == 0 {
		return firewall.Term{}, false
	}
	if floor == 0 {
		return firewall.Term{}, false
	}

	var belowFloor firewall.Match = firewall.MatchIPv4TTLBelow{Floor: floor}
	if source.Addr().Is6() {
		belowFloor = firewall.MatchIPv6HopLimitBelow{Floor: floor}
	}
	return firewall.Term{
		Name: listenTermName(source),
		Matches: []firewall.Match{
			firewall.MatchSourceAddress{Prefix: source},
			firewall.MatchProtocol{Protocol: "tcp"},
			firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: port, Hi: port}}},
			belowFloor,
		},
		Actions: []firewall.Action{firewall.Drop{}},
	}, true
}

// listenTermName names a listen-port drop rule after its source: a host
// prefix by its address, a range by its address and its length. The name is
// stable across reloads, so the doctor check can name the rule it misses.
func listenTermName(source netip.Prefix) string {
	var tb textbuf.Buffer
	tb.Str("dangerous-")
	writeNameSafe(&tb, textbuf.StringAddr(source.Addr()))
	if !source.IsSingleIP() {
		tb.Byte('-').Uint(uint64(source.Bits()))
	}
	tb.Str("-listen")
	return tb.String()
}

// writeNameSafe writes an address's text with its dots and colons turned into
// hyphens, the characters a firewall name refuses (firewall.ValidateName).
func writeNameSafe(tb *textbuf.Buffer, text string) {
	for i := range len(text) {
		c := text[i]
		if c == '.' || c == ':' {
			c = '-'
		}
		tb.Byte(c)
	}
}

// termName names one drop rule. A firewall name accepts letters, digits, '-',
// '_' and '.', and refuses a leading '-' or '.' (firewall.ValidateName), so
// the dots of an address become hyphens and the name opens with a letter.
func termName(addr netip.Addr, icmpType uint8, side firewall.QuotedPortSide) string {
	text := textbuf.StringAddr(addr)

	var tb textbuf.Buffer
	tb.Str("dangerous-")
	writeNameSafe(&tb, text)
	tb.Str("-type").Uint(uint64(icmpType))
	if side == firewall.QuotedPortSource {
		tb.Str("-quoted-source")
		return tb.String()
	}
	tb.Str("-quoted-destination")
	return tb.String()
}
