// Design: rfc/short/rfc5082.md -- GTSM, the TTL 255 rule for related ICMP messages
// Overview: internal/component/gtsm -- the component that owns the kernel state this file publishes
// Related: session_connection.go -- tuneTCPConnectionForSettings, the per-connection half of GTSM
//
// The socket options a GTSM peer needs are per connection and are installed
// when the connection is made. Two other pieces of kernel state are per PEER
// and outlive any one connection: the route metric that gives a locally
// generated ICMP error its TTL, and the input filter that refuses an ICMP
// error arriving below the peer's floor. This file publishes the peer set
// those two are derived from, once for each peer reconcile.

package reactor

import (
	"github.com/ze-software/ze/internal/component/gtsm"
)

// publishGTSMKernelState hands the current GTSM peer set to the gtsm
// component, which reconciles the kernel to it.
//
// A failure is reported and not returned. The caller is a config apply or a
// start, and neither can usefully undo a peer set because of it: the sessions
// themselves are correct, and what is missing is the protection of their
// related ICMP messages. The gtsm component names the peer in its own line,
// and the next reconcile tries again.
func (r *Reactor) publishGTSMKernelState() {
	if err := setGTSMPeers(r.gtsmPeers()); err != nil {
		reactorLogger().Warn("GTSM kernel state not published", "error", err)
	}
}

// setGTSMPeers is the gtsm component's reconcile entry point. It is a var so a
// test can read the peer set this reactor derives without a kernel to publish
// it to.
var setGTSMPeers = gtsm.SetPeers

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
		s := p.Settings()
		if s.OutTTL == 0 && s.MinTTL == 0 {
			continue
		}
		peers = append(peers, gtsm.Peer{
			Addr:     s.Address,
			Port:     uint16(r.peerListenPort(s)), //nolint:gosec // peerListenPort answers a port, which is a uint16 everywhere it is parsed
			HopLimit: s.OutTTL,
			Floor:    s.MinTTL,
		})
	}
	return peers
}
