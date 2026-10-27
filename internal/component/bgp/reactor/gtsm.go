// Design: rfc/short/rfc5082.md -- GTSM, the TTL 255 rule for related ICMP messages
// Overview: internal/component/gtsm -- the component that owns the kernel state this file publishes
// Related: session_connection.go -- tuneTCPConnectionForSettings, the per-connection half of GTSM
//
// The socket options a GTSM peer needs are per connection and are installed
// when the connection is made. Other pieces of kernel state are per PEER and
// outlive any one connection: the route metric that gives a locally
// generated ICMP error its TTL, and the input filter that refuses an ICMP
// error, or a TCP segment to the peer's listen port, arriving below the
// peer's floor. A dynamic group's range carries the listen-port rule too,
// because its members' SYNs arrive before any member exists. This file
// publishes the peer set and the ranges those are derived from, once for each
// peer reconcile.

package reactor

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/gtsm"
)

// publishGTSMKernelState hands the current GTSM peer set to the gtsm
// component, which reconciles the kernel to it.
//
// A failure is reported and not returned. The caller is a config apply or a
// start, and neither can usefully undo a peer set because of it: the sessions
// themselves are correct, and what is missing is the protection of their
// related ICMP messages and of their listen port before accept, which the
// per-connection floor still backs once a connection exists. The error names
// each peer whose route the kernel
// refused, and the next reconcile tries again (gtsm.SetPeers).
func (r *Reactor) publishGTSMKernelState() {
	// RFC 5082 Section 3: the set names every GTSM-enabled session, so the
	// gtsm component can tell a Dangerous packet from an Unknown one.
	if err := setGTSMPeers(r.gtsmPeers(), r.gtsmRanges()); err != nil {
		reactorLogger().Warn("GTSM kernel state not published", "error", err)
	}
}

// setGTSMPeers is the gtsm component's reconcile entry point. It is a var so a
// test can read the set this reactor derives without a kernel to publish it
// to.
var setGTSMPeers = gtsm.SetPeers

// gtsmRanges is each dynamic group's ranges, with the floor and the listen
// port of the group's template, and the host prefix of each static peer that
// asked for no GTSM. A member exists only after its SYN has been accepted, so
// the listen-port rule that judges that SYN names the range.
//
// A source with no GTSM is published with a zero floor. It carries no drop:
// the gtsm component claims it ahead of a wider GTSM range on the same port,
// because the listener gives its SYN to its own session (gtsm.Range).
//
// A prefix is published once, for the session the listener gives it to. The
// listener tries a static peer before any group (handleConnection), and among
// groups whose ranges match equally long it keeps the first
// (findDynamicGroup). So a group's prefix that a static peer's host prefix or
// an earlier group already holds owns no SYN, and publishing it would let its
// floor judge another session's SYNs.
func (r *Reactor) gtsmRanges() []gtsm.Range {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var ranges []gtsm.Range
	held := make(map[netip.Prefix]bool)
	for _, p := range r.peers {
		s := p.Settings()
		if s.IsDynamic {
			continue
		}
		host := netip.PrefixFrom(s.Address, s.Address.BitLen())
		held[host] = true
		if _, enabled := gtsmPeerOf(s, r.config.Port); enabled {
			continue
		}
		port := uint16(listenPortFor(s, r.config.Port)) //nolint:gosec // listenPortFor answers a port, which is a uint16 everywhere it is parsed
		ranges = append(ranges, gtsm.Range{Prefix: host, Port: port})
	}
	for _, group := range r.dynamicGroups {
		// A group with no template settings has no session, so no source.
		if group.Settings == nil {
			continue
		}
		source := gtsm.Range{Port: uint16(listenPortFor(group.Settings, r.config.Port))} //nolint:gosec // listenPortFor answers a port, which is a uint16 everywhere it is parsed
		if template, enabled := gtsmPeerOf(group.Settings, r.config.Port); enabled {
			source.Floor = template.Floor
		}
		for _, prefix := range group.Ranges {
			if held[prefix.Masked()] {
				continue
			}
			held[prefix.Masked()] = true
			source.Prefix = prefix
			ranges = append(ranges, source)
		}
	}
	return ranges
}

// gtsmPeers is the peer set with GTSM enabled, as the gtsm component reads it.
//
// A peer is in the set when its configuration derived either TTL value.
// parseTTLSettings (config.go) writes both from one `ttl` block, so a peer
// with neither asked for no GTSM and carries no kernel state at all.
//
// The port is the one the peer's BGP session uses, which is what associates a
// quoted TCP header inside an ICMP error with that session.
func (r *Reactor) gtsmPeers() []gtsm.Peer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var peers []gtsm.Peer
	for _, p := range r.peers {
		if gtsmPeer, enabled := gtsmPeerOf(p.Settings(), r.config.Port); enabled {
			peers = append(peers, gtsmPeer)
		}
	}
	return peers
}

// gtsmPeerOf is the one reading of a peer's settings as GTSM kernel state,
// shared by the running reactor and by the configuration-only derivation
// below. It answers false for a peer whose configuration derived neither
// TTL value, which is a peer that asked for no GTSM.
func gtsmPeerOf(s *PeerSettings, daemonPort int) (gtsm.Peer, bool) {
	if s.OutTTL == 0 && s.MinTTL == 0 {
		return gtsm.Peer{}, false
	}
	return gtsm.Peer{
		Addr:     s.Address,
		Port:     uint16(listenPortFor(s, daemonPort)), //nolint:gosec // listenPortFor answers a port, which is a uint16 everywhere it is parsed
		HopLimit: s.OutTTL,
		Floor:    s.MinTTL,
	}, true
}

// gtsmPeersFromResolvedTree derives the GTSM peer set from a resolved bgp{}
// tree with no reactor running, for `ze doctor` in an operator's own process
// (internal/component/gtsm/doctor.go). It runs the same peer parse the
// reactor's config apply runs, so the set it answers is the set that apply
// would publish, and it takes the daemon port from the same test override
// the config loader reads (PortOverrideFromEnv).
//
// A dynamic group's members are not in it: they exist once a peer connects,
// and a configuration names only their range.
func gtsmPeersFromResolvedTree(bgpTree map[string]any) ([]gtsm.Peer, error) {
	settings, err := PeersFromTree(bgpTree)
	if err != nil {
		return nil, err
	}
	daemonPort := 0
	if port, ok := PortOverrideFromEnv(); ok {
		daemonPort = int(port)
	}
	var peers []gtsm.Peer
	for _, s := range settings {
		if gtsmPeer, enabled := gtsmPeerOf(s, daemonPort); enabled {
			peers = append(peers, gtsmPeer)
		}
	}
	return peers, nil
}

func init() {
	gtsm.SetConfigPeers(gtsmPeersFromResolvedTree)
}
