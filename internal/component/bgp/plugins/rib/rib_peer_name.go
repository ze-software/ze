// Design: docs/architecture/plugin/rib-storage-design.md -- selecting a session by its name
// Overview: rib.go -- RIBManager.peerNames, filled by handleState and handleStructuredState

package rib

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/selector"
)

// recordPeerName stores the configured name a state event states for the
// session at peer. A state event with no name is a session configured without
// one, so its entry is removed rather than kept from an earlier config.
//
// The caller MUST hold peerMu for writing.
func (r *RIBManager) recordPeerName(peer netip.Addr, name string) {
	if name == "" {
		delete(r.peerNames, peer)
		return
	}
	if r.peerNames == nil {
		r.peerNames = make(map[netip.Addr]string)
	}
	r.peerNames[peer] = name
}

// selectsPeer reports whether sel selects the BGP session at peer.
//
// bgp-rib keys every session by its address, and selector.Matches answers
// false for a name selector because an address alone cannot answer it. A name
// selector is therefore matched against the name the session's state events
// carried (recordPeerName). A session whose state events named it nothing is
// selected by no name, the way `show bgp` lists it by its address only.
//
// The caller MUST hold peerMu, for reading or writing.
func (r *RIBManager) selectsPeer(sel *selector.Selector, peer netip.Addr) bool {
	if sel.SelectorKind() != selector.KindName {
		return sel.Matches(peer)
	}
	named := r.peerNames[peer] == sel.NameValue()
	if sel.IsExclude() {
		return !named
	}
	return named
}
