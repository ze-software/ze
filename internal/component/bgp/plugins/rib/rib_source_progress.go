// Design: docs/architecture/plugin/rib-storage-design.md -- original selecting actor catch-up.
// Related: rib_peer_name.go -- current physical actor identity.
package rib

import (
	"errors"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"net/netip"
)

// sourceProgress reads the existing actor under its ordinary lock. Retained
// routes keep that original actor's final position; replacement physical UP
// resets it. No route history or unrelated-source message maximum is authority.
func (r *RIBManager) sourceProgress(route ribevents.ValidationRoute, min uint64) (bool, error) {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()
	meta := r.peerMeta[route.PeerOwner]
	if route.PeerOwner == 0 || route.ConnectionID == 0 || !route.Peer.IsValid() || meta == nil {
		return false, errors.New("FlowSpec source processing position is unknown; preserve the original peer owner and connection")
	}
	if meta.Address != route.Peer || meta.ConnectionID != route.ConnectionID {
		return false, errors.New("FlowSpec source processing identity changed; retry only its original physical actor")
	}
	if meta.processingFailed {
		return false, errors.New("original RIB UPDATE application failed; re-establish that physical source before certifying its current routes")
	}
	return meta.processedMsgID >= min, nil
}

// peerProcessedLocked preserves only this physical actor's scalar position
// when UPDATE metadata is rebuilt. Caller MUST hold peerMu.
func (r *RIBManager) peerProcessedLocked(owner uint64, address netip.Addr, connection uint64) uint64 {
	meta := r.peerMeta[owner]
	if meta == nil || meta.Address != address || meta.ConnectionID != connection {
		return 0
	}
	return meta.processedMsgID
}

// sourceUpdateCurrentLocked guards mutation of a known physical actor. An
// unrecorded passive observer may store routes but cannot certify a floor.
func (r *RIBManager) sourceUpdateCurrentLocked(owner uint64, address netip.Addr, connection uint64) bool {
	meta := r.peerMeta[owner]
	if meta == nil || meta.ConnectionID == 0 {
		return true
	}
	return meta.Address == address && meta.ConnectionID == connection
}

// noteSourceProcessed follows the whole original UPDATE's committed effects,
// including withdrawal-only and empty UPDATEs. It never creates an actor.
func (r *RIBManager) noteSourceProcessed(address netip.Addr, owner, connection, id uint64) {
	r.peerMu.Lock()
	defer r.peerMu.Unlock()
	meta := r.peerMeta[owner]
	if meta == nil || owner == 0 || connection == 0 {
		return
	}
	if meta.Address != address || meta.ConnectionID != connection {
		return
	}
	if meta.processingFailed {
		return
	}
	if id > meta.processedMsgID {
		meta.processedMsgID = id
	}
}

func (r *RIBManager) peerProcessingFailedLocked(owner uint64, address netip.Addr, connection uint64) bool {
	meta := r.peerMeta[owner]
	return meta != nil && meta.Address == address && meta.ConnectionID == connection && meta.processingFailed
}

// failSourceProcessed records only a current actor's unapplied gap. A later
// valid higher message cannot certify that missing UPDATE as current absence.
func (r *RIBManager) failSourceProcessed(address netip.Addr, owner, connection uint64) {
	r.peerMu.Lock()
	defer r.peerMu.Unlock()
	meta := r.peerMeta[owner]
	if meta == nil || owner == 0 || connection == 0 {
		return
	}
	if meta.Address != address || meta.ConnectionID != connection {
		return
	}
	meta.processingFailed = true
}

func (r *RIBManager) validEmptyReceived(event *Event) bool {
	for _, fam := range event.RawNLRIFamilies() {
		if len(event.GetRawNLRIBytes(fam)) > 0 || event.RawNLRI[fam] != "" {
			return false
		}
	}
	for _, fam := range event.RawWithdrawnFamilies() {
		if len(event.GetRawWithdrawnBytes(fam)) > 0 || event.RawWithdrawn[fam] != "" {
			return false
		}
	}
	attrs := event.GetRawAttributesBytes()
	if event.RawAttributes != "" && len(attrs) == 0 {
		return false
	}
	entry, _, _, err := storage.ParseRouteEntry(attrs)
	if err != nil {
		return false
	}
	entry.Release()
	return true
}
