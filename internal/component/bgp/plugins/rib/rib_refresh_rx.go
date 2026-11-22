// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 7313 received BoRR/EoRR stale marking and purge
// RFC: rfc/short/rfc7313.md -- Section 4 Enhanced Route Refresh receiver procedures
// Related: rib.go -- JSON dispatch for EventKindBoRR / EventKindEoRR
// Related: rib_structured.go -- DirectBridge dispatch for ROUTE-REFRESH subtypes 1 and 2

package rib

import (
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (r *RIBManager) handleBoRR(event *Event) {
	fam := family.Family{AFI: event.AFI, SAFI: event.SAFI}
	r.handleBoRRForPeer(event.PeerOwner, event.GetPeerAddress(), fam)
}

func (r *RIBManager) handleEoRR(event *Event) {
	fam := family.Family{AFI: event.AFI, SAFI: event.SAFI}
	r.handleEoRRForPeer(event.PeerOwner, event.GetPeerAddress(), fam)
}

func (r *RIBManager) handleBoRRStructured(se *rpc.StructuredEvent) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok {
		return
	}
	if msg == nil {
		return
	}
	if len(msg.RawBytes) < 4 {
		return
	}
	fam := family.Family{
		AFI:  family.AFI(uint16(msg.RawBytes[0])<<8 | uint16(msg.RawBytes[1])),
		SAFI: family.SAFI(msg.RawBytes[3]),
	}
	r.handleBoRRForPeer(se.PeerOwner, se.PeerAddress, fam)
}

func (r *RIBManager) handleEoRRStructured(se *rpc.StructuredEvent) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok {
		return
	}
	if msg == nil {
		return
	}
	if len(msg.RawBytes) < 4 {
		return
	}
	fam := family.Family{
		AFI:  family.AFI(uint16(msg.RawBytes[0])<<8 | uint16(msg.RawBytes[1])),
		SAFI: family.SAFI(msg.RawBytes[3]),
	}
	r.handleEoRRForPeer(se.PeerOwner, se.PeerAddress, fam)
}

// handleBoRRForPeer enforces RFC 7313 Section 4: "When a BGP speaker receives
// a BoRR message from a peer, it MUST mark all the routes with the given
// Address Family Identifier and Subsequent Address Family Identifier,
// <AFI, SAFI> [RFC2918], from that peer as stale."
func (r *RIBManager) handleBoRRForPeer(peerOwner uint64, peerAddr string, fam family.Family) {
	if peerOwner == 0 {
		return
	}

	r.peerMu.Lock()
	if r.borrActive == nil {
		r.borrActive = make(map[uint64]map[family.Family]bool)
	}
	if r.borrActive[peerOwner] == nil {
		r.borrActive[peerOwner] = make(map[family.Family]bool)
	}
	r.borrActive[peerOwner][fam] = true
	if peerRIB := r.bgpPeers[peerOwner]; peerRIB != nil {
		peerRIB.MarkFamilyStale(fam, storage.StaleLevelFresh+1)
	}
	r.peerMu.Unlock()

	logger().Debug("received BoRR marker, marked family routes stale",
		"peer", peerAddr, "family", fam.String())
}

// handleEoRRForPeer enforces RFC 7313 Section 4: "When a BGP speaker receives
// an EoRR message from a peer, it MUST immediately remove any routes from the
// peer that are still marked as stale for that <AFI, SAFI>."
//
// RFC 7313 Section 4: "A BGP speaker MAY ignore any EoRR message received
// without a prior receipt of an associated BoRR message." Ignoring an EoRR
// without an active BoRR prevents purging unrelated Graceful Restart stale
// routes.
func (r *RIBManager) handleEoRRForPeer(peerOwner uint64, peerAddr string, fam family.Family) {
	if peerOwner == 0 {
		return
	}

	type staleNLRI struct {
		nlri    []byte
		addPath bool
	}
	var affected []staleNLRI
	var removed removedReceivedOwners
	var writes []sentLifecycleWrite
	purged := 0

	r.peerMu.Lock()
	famActive := r.borrActive[peerOwner]
	if famActive == nil {
		r.peerMu.Unlock()
		logger().Debug("ignoring EoRR marker without prior BoRR",
			"peer", peerAddr, "family", fam.String())
		return
	}
	if !famActive[fam] {
		r.peerMu.Unlock()
		logger().Debug("ignoring EoRR marker without prior BoRR",
			"peer", peerAddr, "family", fam.String())
		return
	}
	delete(famActive, fam)
	if len(famActive) == 0 {
		delete(r.borrActive, peerOwner)
	}

	if peerRIB := r.bgpPeers[peerOwner]; peerRIB != nil {
		purged = peerRIB.RemoveFamilyMatching(fam, func(entry storage.RouteEntry) bool {
			return entry.StaleLevel > storage.StaleLevelFresh
		}, func(raw []byte, message uint64, addPath bool) {
			affected = append(affected, staleNLRI{nlri: raw, addPath: addPath})
			removed.add(fam, raw, addPath, message)
		})
	}
	if len(affected) > 0 {
		writes = r.withdrawRemovedSentLocked(peerOwner, removed)
	}
	r.peerMu.Unlock()

	if len(writes) > 0 {
		_ = r.dispatchSentLifecycle(writes)
	}
	for _, a := range affected {
		if change, ok := r.checkBestPathChange(fam, a.nlri, a.addPath, nil); ok {
			publishBestChanges([]bestChangeEntry{change}, fam)
		}
	}

	logger().Debug("received EoRR marker, purged remaining stale routes",
		"peer", peerAddr, "family", fam.String(), "purged", purged)
}
