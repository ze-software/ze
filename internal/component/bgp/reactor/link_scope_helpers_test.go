package reactor

import "net/netip"

// newLinkScopeFrom builds the snapshot production builds over an interface
// table the test chooses: the neighbor table is read for real. This package's
// tests link no interface component, so that read answers
// network.ErrNeighborTableNotRegistered and a third-party next hop goes out
// with its global address alone; a test that exercises RFC 2545 Section 3's
// third-party Link-Local builds its view and calls newLinkScopeWith. The view
// carries the not-started watch state, as for a scope built outside a reactor.
func newLinkScopeFrom(connected []netip.Prefix, peerAddr netip.Addr) *linkScope {
	return newLinkScopeWith(connected, readNeighborView(neighborWatchState{}), peerAddr)
}

// refreshLinkScopeFrom re-settles a peer's snapshot over connected, reading the
// neighbor table as newLinkScopeFrom does, with the watch state of the peer's
// reactor.
func (p *Peer) refreshLinkScopeFrom(connected []netip.Prefix) {
	p.refreshLinkScopeWith(connected, readNeighborView(p.neighborWatchState()))
}
