// Design: docs/architecture/route-selection.md
// RFC: rfc/short/rfc7311.md -- Sections 3.3 and 3.4.3
package reactor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
)

func payloadAIGP(body []byte) []byte {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return nil
	}
	_, _, value, _ := attribute.AttrFind(sections.Attrs(body), attribute.AttrAIGP)
	return value
}

// forwardUpdateSelected splits mixed AIGP UPDATEs before egress edits. RFC 7606
// Section 5.1 requires accepting every combination of NLRI fields; RFC 7311
// Section 3.4.3 accumulates distance to each route's received next hop.
// SplitWireUpdate owns its output bytes and preserves source identity.
func (a *reactorAPIAdapter) forwardUpdateSelected(update *ReceivedUpdate, updateID uint64, matchingPeers []*Peer, srcInfo forwardSourceInfo, sourceWire *wireu.WireUpdate) error {
	if !sourceWire.MixesNLRIFields() || len(payloadAIGP(sourceWire.Payload())) == 0 {
		return a.forwardUpdateSection(update, updateID, matchingPeers, srcInfo, sourceWire)
	}
	// The first section's worker can finish before the remaining sections have
	// borrowed their buffers. This retain MUST survive the whole split fan-out.
	a.r.recentUpdates.RetainN(updateID, 1)
	defer a.r.recentUpdates.Release(updateID)
	sections, err := wireu.SplitWireUpdate(sourceWire, len(sourceWire.Payload()), bgpctx.Registry.Get(sourceWire.SourceCtxID()))
	if err != nil {
		return err
	}
	delivered := false
	for _, section := range sections {
		err = a.forwardUpdateSection(update, updateID, matchingPeers, srcInfo, section)
		if err == nil {
			delivered = true
		} else if !errors.Is(err, errAllDestinationsSuppressed) {
			return err
		}
	}
	if delivered {
		return nil
	}
	return errAllDestinationsSuppressed
}

// aigpNextHop ignores attributes that govern no announced route. The caller
// MUST split mixed legacy and MP announcements before accumulating AIGP.
func aigpNextHop(body []byte) nextHopValue {
	nh := payloadNextHop(body)
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return nextHopValue{}
	}
	if len(sections.NLRI(body)) == 0 {
		nh.legacy = netip.Addr{}
	}
	_, _, reach, found := attribute.AttrFind(sections.Attrs(body), attribute.AttrMPReachNLRI)
	if !found || len(reach) < 5 || len(reach) <= 5+int(reach[3]) {
		nh.mp, nh.mpLL = netip.Addr{}, netip.Addr{}
	}
	return nh
}

func aigpMetricValue(value []byte) (uint64, bool) {
	offset, err := attribute.AIGPMetricOffset(value)
	if err != nil || offset < 0 {
		return 0, false
	}
	return binary.BigEndian.Uint64(value[offset:]), true
}

func aigpIncrement(nh, sourcePeer netip.Addr, sourceLinkMetric uint64) (uint64, bool) {
	distance := igpcost.Lookup(nh)
	if distance.MissingAIGP {
		return 0, false
	}
	var increment uint64
	if distance.Resolved {
		increment = distance.Cost
	}
	if increment == 0 && nh.Unmap() == sourcePeer.Unmap() {
		increment = sourceLinkMetric
	}
	return increment, increment != 0
}

// sourceAIGPLinkMetric belongs to the link toward the received next hop, never
// to the destination peer. Settings snapshots are immutable between reloads.
func (r *Reactor) sourceAIGPLinkMetric(addr netip.Addr) uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	peer, ok := r.findPeerByAddr(addr)
	if !ok {
		return 0
	}
	return peer.Settings().AIGPLinkMetric
}

// applyFactsAIGP runs after policy and next-hop edits on both forward rails.
// It uses the received value as the accumulation base, not a previously sent
// value, so replay or a metric change cannot add the same distance twice.
func applyFactsAIGP(f *peerForwardFacts, received []byte, before nextHopValue, base []byte, sourcePeer netip.Addr, sourceLinkMetric uint64, mods *filterapi.ModAccumulator) {
	out := payloadAIGP(base)
	for _, op := range mods.Ops() {
		if op.Code != uint8(attribute.AttrAIGP) {
			continue
		}
		switch op.Action {
		case filterapi.AttrModSet:
			out = op.Buf
		case filterapi.AttrModSuppress:
			out = nil
		}
	}
	if len(received) == 0 && len(out) == 0 {
		return
	}
	suppress := func() {
		if len(out) != 0 {
			mods.Op(uint8(attribute.AttrAIGP), filterapi.AttrModSuppress, nil)
		}
	}
	if !f.aigpEnabled || len(received) == 0 {
		// Origination is an explicit route operation, never an accidental
		// side effect of a transit filter inserting an attribute.
		suppress()
		return
	}
	off, err := attribute.AIGPMetricOffset(received)
	if err != nil {
		suppress()
		return
	}
	after := aigpNextHop(base)
	if edited, set := modsNextHop(mods); set {
		if before.legacy.IsValid() && edited.legacy.IsValid() {
			after.legacy = edited.legacy
		}
		if before.mp.IsValid() && edited.mp.IsValid() {
			after.mp, after.mpLL = edited.mp, edited.mpLL
		}
	}
	if before.legacy == after.legacy && before.mp == after.mp {
		// Section 3.4.3: "the AIGP attribute MUST be passed along unchanged."
		if !bytes.Equal(received, out) {
			mods.OpCopy(uint8(attribute.AttrAIGP), filterapi.AttrModSet, received)
		}
		return
	}
	if off < 0 {
		suppress()
		return
	}
	var scope *receiveNextHopScope
	if f.localScope != nil {
		scope = f.localScope.Load()
	}
	if !isLocalAIGPNextHop(after, f.localAddr, scope) {
		// The RFC's distance calculation only authorizes next-hop-self.
		suppress()
		return
	}
	receivedNextHop := before.legacy
	if before.mp.IsValid() {
		receivedNextHop = before.mp
	}
	increment, usable := aigpIncrement(receivedNextHop, sourcePeer, sourceLinkMetric)
	if !usable {
		// Section 3.4.3 requires a non-zero distance and forbids accumulating
		// through a recursive BGP next hop that has no AIGP.
		suppress()
		return
	}
	mods.OpCopy(uint8(attribute.AttrAIGP), filterapi.AttrModSet, received)
	ops := mods.Ops()
	value := ops[len(ops)-1].Buf
	binary.BigEndian.PutUint64(value[off:], igpcost.Add(binary.BigEndian.Uint64(received[off:]), increment))
}
