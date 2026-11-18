// Design: docs/architecture/plugin/rib-storage-design.md -- selecting a session by its name
// Overview: rib.go -- RIBManager.peerNames, filled by handleState and handleStructuredState

package rib

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/selector"
)

// recordSentDestinationLocked records display metadata even when the SENT
// event precedes UP. It never creates live-state or Session authority.
func (r *RIBManager) recordSentDestinationLocked(owner uint64, address netip.Addr) {
	if r.peerMeta[owner] == nil {
		r.peerMeta[owner] = &peerMetadata{Address: address}
	}
}

// recordPeerName stores the configured name a state event states for the
// session at peer. A state event with no name is a session configured without
// one, so its entry is removed rather than kept from an earlier config.
//
// The caller MUST hold peerMu for writing.
func (r *RIBManager) recordPeerName(peer uint64, name string) {
	if name == "" {
		delete(r.peerNames, peer)
		return
	}
	if r.peerNames == nil {
		r.peerNames = make(map[uint64]string)
	}
	r.peerNames[peer] = name
}

// selectsPeer reports whether sel selects the BGP session at peer.
//
// The owner selects one configured Peer. Address/name selectors inspect that
// owner's observed facts; neither can replace its identity for a lifecycle call.
// A deliberate address selector can match several independently owned sources.
//
// The caller MUST hold peerMu, for reading or writing.
func (r *RIBManager) selectsPeer(sel *selector.Selector, peer uint64) bool {
	if sel.SelectorKind() != selector.KindName {
		address, known := r.peerAddressLocked(peer)
		return known && sel.Matches(address)
	}
	named := r.peerNames[peer] == sel.NameValue()
	if sel.IsExclude() {
		return !named
	}
	return named
}

// peerAddressLocked reads observed display/selector data from the one owner
// population. An absent observation answers unknown, never another Peer.
// Caller MUST hold peerMu for reading or writing.
func (r *RIBManager) peerAddressLocked(owner uint64) (netip.Addr, bool) {
	meta := r.peerMeta[owner]
	if meta == nil {
		return netip.Addr{}, false
	}
	return meta.Address, meta.Address.IsValid()
}

// selectsDestination is an operator selector over exact destination owners.
// Caller MUST hold peerMu for reading or writing.
func (r *RIBManager) selectsDestination(sel *selector.Selector, owner uint64) bool {
	return r.selectsPeer(sel, owner)
}

// sourceSessionCurrentLocked refuses an unrecorded or predecessor DOWN.
// Caller MUST hold peerMu. Peer removal instead names the exact Peer owner.
func (r *RIBManager) sourceSessionCurrentLocked(owner, session uint64) bool {
	if session == 0 {
		return false
	}
	meta := r.peerMeta[owner]
	return meta != nil && meta.Session == session
}

// recordPeerStateLocked preserves the route's captured protocol facts while
// recording this configured owner's observed address and current Session.
// Caller MUST hold peerMu for writing.
func (r *RIBManager) recordPeerStateLocked(owner uint64, address netip.Addr, session, connectionID uint64, up bool) {
	meta := r.peerMeta[owner]
	if meta == nil {
		meta = &peerMetadata{}
		r.peerMeta[owner] = meta
	}
	if up {
		if meta.ConnectionID != connectionID || meta.Address != address {
			meta.processedMsgID = 0
			meta.processingFailed = false
		}
		meta.Session = session
		meta.ConnectionID = connectionID
	}
	meta.Address = address
}

// clearPeerRouteMetadataLocked keeps only configured-owner observation after
// its unretained routes leave. Caller MUST hold peerMu for writing.
func (r *RIBManager) clearPeerRouteMetadataLocked(owner uint64) {
	if meta := r.peerMeta[owner]; meta != nil {
		*meta = peerMetadata{Address: meta.Address, Session: meta.Session, ConnectionID: meta.ConnectionID, processedMsgID: meta.processedMsgID, processingFailed: meta.processingFailed}
	}
}

// forgetPeerSourceLocked drops permanently removed source bookkeeping.
// Its caller MUST purge that owner's routes and source-policy pins first.
// Permanent destination ownership ends here too; physical DOWN never calls it.
func (r *RIBManager) forgetPeerSourceLocked(owner uint64) {
	r.selectedDispatch.retireOriginOwner(owner)
	r.retireSentDestinationLocked(owner)
	r.stopGRExpiryLocked(r.grState[owner])
	delete(r.grState, owner)
	delete(r.peerMeta, owner)
	delete(r.peerNames, owner)
	delete(r.peerUp, owner)
	delete(r.sourceApplied, owner)
	delete(r.retainedPeers, owner)
}

// sourceOwnersAtAddressLocked implements the deliberate address-wide operator
// selector over the one owner population. Internal lifecycle callers MUST carry
// their captured owner instead; this result never authenticates a command.
// Caller MUST hold peerMu. The current configured owner population bounds it.
func (r *RIBManager) sourceOwnersAtAddressLocked(address netip.Addr) []uint64 {
	var owners []uint64
	for owner, meta := range r.peerMeta {
		if meta.Address == address {
			owners = append(owners, owner)
		}
	}
	return owners
}
