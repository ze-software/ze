// Design: docs/architecture/plugin/rib-storage-design.md -- validation and route selection
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 mitigation policy
package rib

import (
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/store"
)

// validationEligible reads the receive gate for this exact stored generation.
// Only CIDR families have the prefix identity used by validation decisions.
func (r *RIBManager) validationEligible(peer netip.Addr, routes *storage.PeerRIB, fam family.Family, wire []byte, msgID uint64) bool {
	if ribevents.IsFlowSpec(fam) {
		key, ok := flowSpecKey(peer, fam, wire, routes.IsAddPath(fam))
		return ok && r.flowSpecEligible(key, msgID)
	}
	if !ribevents.ValidationEnabled() || !storage.IsCIDRFamily(fam) {
		return true
	}
	pathID, prefix, ok := parsePrevKey(fam, wire, routes.IsAddPath(fam))
	if !ok {
		return false
	}
	return ribevents.RouteEligible(ribevents.ValidationRoute{
		Peer: peer, Family: fam, Prefix: prefix, PathID: pathID,
	}, msgID)
}

// validationChanged reruns the same selection and Loc-RIB publication used by
// received UPDATEs. Recovery therefore needs no new UPDATE from the neighbor.
func (r *RIBManager) validationChanged(changes []ribevents.ValidationRoute) {
	// FlowSpec notifications already came from this RIB's reconciliation.
	// Unicast validation changes must also reauthorize every dependent rule.
	revalidate := false
	defer func() {
		if revalidate {
			r.reconcileFlowSpecs()
		}
	}()
	for _, key := range changes {
		if key.Family.SAFI == family.SAFIUnicast || key.Family.SAFI == family.SAFIVPN {
			revalidate = true
		}
		if !key.Prefix.IsValid() {
			continue
		}
		r.peerMu.RLock()
		peer := r.bgpPeers[key.Peer]
		addPath := peer != nil && peer.IsAddPath(key.Family)
		r.peerMu.RUnlock()
		var buf [21]byte
		off := 0
		if addPath {
			binary.BigEndian.PutUint32(buf[:4], key.PathID)
			off = 4
		}
		wire := store.PrefixToNLRIInto(key.Prefix, buf[off:])
		wire = buf[:off+len(wire)]
		change, changed := r.checkBestPathChange(key.Family, wire, addPath, nil)
		if changed {
			publishBestChanges([]bestChangeEntry{change}, key.Family)
		}
	}
}

// reconcileReceived runs after the JSON receive path releases peerMu, matching
// the structured path's best-path publication for both adds and withdrawals.
func (r *RIBManager) reconcileReceived(event *Event) {
	for _, fam := range event.RawWithdrawnFamilies() {
		r.reconcileReceivedNLRIs(fam, event.GetRawWithdrawnBytes(fam), event.AddPath[fam])
	}
	for _, fam := range event.RawNLRIFamilies() {
		r.reconcileReceivedNLRIs(fam, event.GetRawNLRIBytes(fam), event.AddPath[fam])
	}
}

func (r *RIBManager) reconcileReceivedNLRIs(fam family.Family, data []byte, addPath bool) {
	if !nlrisplit.Supported(fam) {
		return
	}
	prefixes, _ := nlrisplit.Split(fam, data, addPath)
	for _, wire := range prefixes {
		change, changed := r.checkBestPathChange(fam, wire, addPath, nil)
		if changed {
			publishBestChanges([]bestChangeEntry{change}, fam)
		}
	}
}

// replaySourceEligible excludes a previously advertised path when validation
// has since rejected its source. Locally injected routes have no source peer.
func (r *RIBManager) replaySourceEligible(fam family.Family, key ribOutKey) bool {
	if !ribevents.ValidationEnabled() {
		return true
	}
	source := r.ribOutSourcePeer(fam, key)
	if source == "" {
		return true
	}
	peer, err := netip.ParseAddr(source)
	if err != nil {
		return false
	}
	return ribevents.RouteEligible(ribevents.ValidationRoute{
		Peer: peer, Family: fam, Prefix: key.Prefix, PathID: key.PathID,
	}, 0)
}
