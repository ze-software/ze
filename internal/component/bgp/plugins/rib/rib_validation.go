// Design: docs/architecture/plugin/rib-storage-design.md -- validation and route selection
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 mitigation policy
package rib

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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

// receivedFamilyAttrs restores the next hop split out of raw.attributes by JSON.
// RFC 4760 Section 3, Network Address of Next Hop: "A variable-length field that
// contains the Network Address of the next router on the path to the destination system."
// The existing writer supplies AFI(2), SAFI(1), NH-length(1), next hop, reserved(1).
// No NLRI is duplicated here: the route storage receives it separately.
func receivedFamilyAttrs(event *Event, fam family.Family, raw []byte) ([]byte, error) {
	nextHop := sentFamilyNextHop(event, fam)
	if nextHop == "" {
		return raw, nil
	}
	addr, err := netip.ParseAddr(nextHop)
	if err != nil {
		return nil, fmt.Errorf("received next hop: %w", err)
	}
	if fam == family.IPv4Unicast && addr.Is4() {
		return raw, nil
	}
	iter := attribute.NewAttrIterator(raw)
	for code, _, _, ok := iter.Next(); ok; code, _, _, ok = iter.Next() {
		if code == attribute.AttrMPReachNLRI {
			return raw, nil
		}
	}
	// RFC 4760 Section 3: preserve the family's advertised next-hop address.
	mp := attribute.NewMPReachNLRI(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI), []netip.Addr{addr}, nil)
	if err := mp.ValidateNextHops(); err != nil {
		return nil, fmt.Errorf("received MP next hop: %w", err)
	}
	// JSON separated these fields; one family-local buffer reunites them before
	// the existing attribute pools take ownership of their values.
	attrs := make([]byte, len(raw)+3+mp.Len())
	copy(attrs, raw)
	attribute.WriteAttrTo(mp, attrs, len(raw))
	return attrs, nil
}

// reconcileReceived runs after the JSON receive path releases peerMu, matching
// the structured path's best-path publication for both adds and withdrawals.
func (r *RIBManager) reconcileReceived(event *Event) {
	for _, fam := range event.RawWithdrawnFamilies() {
		r.reconcileReceivedNLRIs(fam, event.GetRawWithdrawnBytes(fam), event.AddPath[fam], true)
	}
	for _, fam := range event.RawNLRIFamilies() {
		r.reconcileReceivedNLRIs(fam, event.GetRawNLRIBytes(fam), event.AddPath[fam], false)
	}
}

// reconcileReceivedNLRIs preserves the native action through framing and identity.
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field MUST be ignored."
// Native layout after an optional four-byte ADD-PATH identifier:
//
//	Offset  0          1..3                 4..
//	        Length     Compatibility       Prefix (VPN includes its RD)
//
// Announcements instead carry a label stack. Registered keys strip those fields
// for CIDR elections; opaque elections retain their native NLRI and action.
func (r *RIBManager) reconcileReceivedNLRIs(fam family.Family, data []byte, addPath, withdraw bool) {
	if !nlrisplit.Supported(fam) {
		return
	}
	split := nlrisplit.Split
	if withdraw {
		split = nlrisplit.SplitWithdrawn
	}
	prefixes, err := split(fam, data, addPath)
	if err != nil {
		logger().Warn("receive reconciliation: split error", "family", fam.String(), "error", err, "parsed", len(prefixes))
	}
	cidr := nlrisplit.KeysByCIDR(fam)
	for _, wire := range prefixes {
		electionAddPath := addPath
		if cidr {
			raw, ok := routeKeyOf(wire, addPath)
			if !ok {
				continue
			}
			var scratch [nlrisplit.PrefixKeyScratchSize]byte
			// RFC 8277 Section 2.4: the Compatibility value does not identify a route.
			wire, err = nlrisplit.RouteCIDR(fam, raw, scratch[:], withdraw)
			if err != nil {
				continue
			}
			// The election spans all path identifiers of the normalized prefix.
			electionAddPath = false
		}
		// RFC 8277 Section 2.4: opaque route keys still need withdrawal context.
		change, changed := r.checkRouteBestChange(fam, wire, electionAddPath, withdraw, nil)
		if changed {
			publishBestChanges([]bestChangeEntry{change}, fam)
		}
	}
}

// replaySourceEligible excludes a previously advertised path when validation
// has since rejected its source. Locally injected routes have no source peer.
func replaySourceEligible(fam family.Family, key ribOutKey, source string) bool {
	if !key.Prefix.IsValid() {
		return true
	}
	if !ribevents.ValidationEnabled() {
		return true
	}
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
