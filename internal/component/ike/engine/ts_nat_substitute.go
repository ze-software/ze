// Design: docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md -- RFC 7296 Section 2.23.1 transport mode NAT traversal
// Detail: ts_narrow.go -- the two entry points that call these producers
// Related: sa.go -- NATDetected, BehindNAT and PeerBehindNAT, the verdict these read
// Related: transport_mode.go -- the proposal builder, which pins to the same address pair
// RFC: rfc/short/rfc7296.md -- transport mode NAT traversal (Section 2.23.1)

package engine

import "net"

// observedLocalAddress returns the local IP address of the IKE SA: the address this
// node's own stack sends from and receives on.
//
// It is the configured local-address, which is the address the IKE sockets bind
// (ikeListenHost, register.go). A NAT on the path never changes it: NAT A rewrites the
// datagram after it leaves this stack, so what the far end sees is not what this node
// has. That is exactly why the substitution exists.
//
// It returns nil when the peer has no usable local-address, and every caller then leaves
// the selector alone rather than substituting a zero address.
func observedLocalAddress(sa *SA) net.IP {
	return net.ParseIP(sa.PeerCfg.LocalAddress)
}

// observedRemoteAddress returns the remote IP address of the IKE SA: the address this
// node sees the peer's datagrams arrive from, which behind a NAT is the translated one.
//
// THE PEER NEVER CHOOSES IT. sa.remoteUDPAddr prefers sa.peerEndpoint, and
// adoptAuthenticatedEndpoint is its only writer, called only after a decrypt and a
// Message ID window check (sa.go). With no authenticated message yet it falls back to
// the CONFIGURED remote-address, never to the datagram in hand. On the responder those
// two are the same address: matchResponderPeer (register.go) accepts an unsolicited
// IKE_SA_INIT only from a source equal to the configured remote-address, so a peer
// behind a NAT is configured with its post-NAT address.
//
// It returns nil when neither source yields an address.
func observedRemoteAddress(sa *SA) net.IP {
	if addr := sa.remoteUDPAddr(); addr != nil && addr.IP != nil {
		return addr.IP
	}
	return net.ParseIP(sa.PeerCfg.RemoteAddress)
}

// transportSelectorsInPlay reports whether the exchange in hand is negotiating a
// TRANSPORT-mode Child SA, on either role.
//
// RFC 7296 Section 2.23.1 opens the responder's procedure with the same test: "the server
// should first check that the initiator requested transport mode, and then do address
// substitution on the Traffic Selectors." PeerRequestedTransport is that request, read
// by the responder before it has decided whether to accept. UseTransportMode is the
// decision itself, and it is what the client rules turn on: "If transport mode for the SA
// was selected (that is, if the server included USE_TRANSPORT_MODE notification in its
// response)."
//
// PeerRequestedTransport is a responder-side field and is false on every initiator SA, so
// one predicate serves both roles. keepSingleAddress is gated on the same pair
// (ts_narrow.go), which keeps the single-address filter and the substitution deciding
// together: a substituted host address that the filter then dropped would answer
// TS_UNACCEPTABLE to a proposal the substitution had just made acceptable.
func transportSelectorsInPlay(sa *SA) bool {
	return sa.UseTransportMode || sa.PeerRequestedTransport
}

// storeOriginalSelectorAddresses records the traffic-selector addresses as they arrived,
// before any substitution replaces them.
//
// RFC 7296 Section 2.23.1, responder rules: "Store the original Traffic Selector IP
// addresses as received source and destination address, in case undo address substitution
// is needed, to use as the 'real source and destination address' specified by [UDPENCAPS],
// and for TCP/UDP checksum fixup." The client rules state the same obligation: "Store the
// original Traffic Selectors as the received source and destination address."
//
// The section fixes the ORDER as well as the fact: "It needs to first store the old
// Traffic Selector IP addresses to be used later for the incremental checksum fixup."
// Every caller therefore calls this before it substitutes.
//
// It records the FIRST entry of each list. Section 2.23.1 permits several transport-mode
// selectors but binds them all to one address each ("all TSi entries must use the IP1-IP1
// range as the IP addresses"), so the first entry carries the whole fact.
func storeOriginalSelectorAddresses(sa *SA, iSels, rSels []tsSelector) {
	sa.OriginalTSiAddr = firstSelectorAddress(iSels)
	sa.OriginalTSrAddr = firstSelectorAddress(rSels)
}

// firstSelectorAddress returns a detached copy of the first selector's address, or nil
// when the list carries none. The copy matters: the caller's selectors are built over
// slices the receive buffer owns.
func firstSelectorAddress(sels []tsSelector) net.IP {
	if len(sels) == 0 || sels[0].Net == nil {
		return nil
	}
	return append(net.IP(nil), sels[0].Net.IP...)
}

// substituteResponderSelectors performs the RFC 7296 Section 2.23.1 address substitution
// on a proposal this node is ANSWERING, and returns the substituted lists.
//
// It runs before the SPD lookup, which in Ze is the narrowing against policyPairs, and
// before anything else reads the selectors. Section 2.23.1: "After this address
// substitution, both the Traffic Selectors and the IKE UDP source/destination addresses
// look the same, and the server does SPD lookup based on those new Traffic Selectors."
//
// The peer's asserted address is DISCARDED rather than trusted. Section 2.23.1 explains
// why it has to be: "Because IP1 does not really mean anything to the server (it is the
// address client has behind the NAT), it is useless to do a lookup based on that if
// transport mode is used." The replacement is an address this node OBSERVED, so a peer
// cannot name a third party's address and have Ze protect traffic to it.
//
// A nil observed address leaves that half alone. A substitution with no address is not
// available, and answering with a zero selector would be worse than answering with the
// one the peer sent (ai/rules/principles.md).
func substituteResponderSelectors(sa *SA, iSels, rSels []tsSelector) ([]tsSelector, []tsSelector) {
	if !transportSelectorsInPlay(sa) {
		return iSels, rSels
	}
	storeOriginalSelectorAddresses(sa, iSels, rSels)
	if !sa.NATDetected {
		return iSels, rSels
	}

	// RFC 7296 Section 2.23.1: "If the client is behind a NAT, substitute the IP address
	// in the TSi entries with the remote address of the IKE SA."
	if sa.PeerBehindNAT {
		iSels = pinSelectorAddresses(iSels, observedRemoteAddress(sa))
	}
	// RFC 7296 Section 2.23.1: "If the server is behind a NAT, substitute the IP address
	// in the TSr entries with the local address of the IKE SA."
	if sa.BehindNAT {
		rSels = pinSelectorAddresses(rSels, observedLocalAddress(sa))
	}
	return iSels, rSels
}

// substituteInitiatorSelectors performs the RFC 7296 Section 2.23.1 address substitution
// on the answer a responder returned, and returns the substituted lists.
//
// It is the mirror of substituteResponderSelectors: the responder answered in the
// addresses ITS stack sees, and this converts them back to the addresses THIS stack sees.
// Section 2.23.1: "It will replace the IP addresses in the Traffic Selectors with the ones
// from the IP header of the IKE packet: it will replace IPN1 with IP1 and IP2 with IPN2."
//
// It runs before the answer is checked or installed, which the section requires in so
// many words: "Do address substitution before using those Traffic Selectors for anything
// other than storing original content of them. This includes verification that Traffic
// Selectors were narrowed correctly by the other end, creation of the SAD entry, and so
// on."
//
// The ceiling checkAnswerWithin tests against is UNCHANGED by this. sa.ProposedChildPairs
// is what this node put on the wire, in this node's own address space, and the
// substitution puts the answer back into that same space. A responder that answers an
// address neither observed address covers still lands outside the ceiling and is refused
// with errTSWidened (ts_narrow.go).
func substituteInitiatorSelectors(sa *SA, iSels, rSels []tsSelector) ([]tsSelector, []tsSelector) {
	if !transportSelectorsInPlay(sa) {
		return iSels, rSels
	}
	storeOriginalSelectorAddresses(sa, iSels, rSels)
	if !sa.NATDetected {
		return iSels, rSels
	}

	// RFC 7296 Section 2.23.1: "If the client is behind a NAT, substitute the IP address
	// in the TSi entries with the local address of the IKE SA."
	if sa.BehindNAT {
		iSels = pinSelectorAddresses(iSels, observedLocalAddress(sa))
	}
	// RFC 7296 Section 2.23.1: "If the server is behind a NAT, substitute the IP address
	// in the TSr entries with the remote address of the IKE SA."
	if sa.PeerBehindNAT {
		rSels = pinSelectorAddresses(rSels, observedRemoteAddress(sa))
	}
	return iSels, rSels
}

// pinSelectorAddresses replaces the address of every selector with ip, keeping each
// selector's port and protocol.
//
// RFC 7296 Section 2.23.1 allows several transport-mode selectors, "for example, multiple
// port ranges that it wants to negotiate", provided every one carries the single address.
// The port and the protocol are what the peer is negotiating, so they survive; only the
// address is substituted.
//
// A nil ip returns the list unchanged, so a caller that could not observe an address
// leaves the selectors as they arrived rather than pinning them to nothing.
func pinSelectorAddresses(sels []tsSelector, ip net.IP) []tsSelector {
	if ip == nil {
		return sels
	}
	pinned := ipToFullNet(ip)
	out := make([]tsSelector, 0, len(sels))
	for _, s := range sels {
		out = append(out, tsSelector{Net: pinned, Port: s.Port, Proto: s.Proto})
	}
	return out
}
