// Design: docs/architecture/bgp/structural-forwarding.md -- one source decision owns removals and surviving output.
// Related: filter_atomic_subset.go -- owned original-source withdrawal bodies.
// Related: forward_pool.go -- the parent item's sole dispatch, retain and release.
// Related: forward_rs_direct.go -- writes the same bounded group under one fence.
// RFC 4271 Section 4.3 -- UPDATE body: Withdrawn Length(2), Withdrawn Routes,
// Total Path Attribute Length(2), Path Attributes, reachable NLRI.

package reactor

import (
	"encoding/binary"
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

const policyRemovalBodiesMax = 2

var errPolicyRemovalBodies = errors.New("marked policy removal exceeds the two source multiprotocol sections")
var errPolicyRemovalEncoding = errors.New("encoding marked policy source withdrawals")

// fwdPolicyRemoval owns encoded withdrawal bytes and their provenance. Queue,
// completion, recovery and cache ownership belong exclusively to its parent.
type fwdPolicyRemoval struct {
	rawBodies  [][]byte
	updates    []*message.Update
	provenance *fwdProvenance
}

func policyRemovalItem(parent *fwdItem, removal *fwdPolicyRemoval) fwdItem {
	authority := parent.authority
	if removal.provenance != nil && authority == adjOutRecovery {
		// A removal owns its original received path, even when the surviving
		// output is subsequently staged as a recovery decision. Exact-receipt
		// and initial replay authority MUST retain their stricter fences.
		authority = adjOutForwarded
	}
	return fwdItem{
		peer: parent.peer, session: parent.session, authority: authority,
		meta: parent.meta, sourcePeerStr: parent.sourcePeerStr,
		sourceMessageID: parent.sourceMessageID, sourcePolicyEpoch: parent.sourcePolicyEpoch,
		sourceReceipt: parent.sourceReceipt,
		receivedPeer: parent.receivedPeer, receivedGeneration: parent.receivedGeneration,
		exportGeneration: parent.exportGeneration, initialUpdate: parent.initialUpdate,
		expectedMessage: parent.expectedMessage, initialReplay: parent.initialReplay,
		initialLocal: parent.initialLocal, initialSourcePath: parent.initialSourcePath,
		initialSourceAddPath: parent.initialSourceAddPath, initialSourceState: parent.initialSourceState,
		ownRouteWithdrawal: parent.ownRouteWithdrawal || parent.authority == adjOutUnspecified,
		originated:         parent.originated,
		rawBodies:          removal.rawBodies, updates: removal.updates, provenance: removal.provenance,
	}
}

// writeOriginPolicyRemovals borrows owned policy bodies synchronously while the
// caller holds writeMu. An unowned local decision MUST spare received owners.
func (s *Session) writeOriginPolicyRemovals(bodies [][]byte) error {
	if len(bodies) > policyRemovalBodiesMax {
		return errPolicyRemovalBodies
	}
	local := fwdItem{session: s, originated: true}
	parent := s.sentForward
	if parent == nil {
		parent = &local
	}
	group := policyRemovalItem(parent, &fwdPolicyRemoval{})
	for _, body := range bodies {
		group.policyRemovals = append(group.policyRemovals, fwdPolicyRemoval{rawBodies: [][]byte{body}})
	}
	// RFC 4271 Section 9.1.3.
	return s.writePolicyRemovals(&group)
}

// prepareOriginPolicyRemovals materializes bounded withdrawal sections before
// queue publication. Each section can split to the destination's message bound.
// The caller MUST retain the owner until synchronous completion or queue transfer.
//
// RFC 8654 Section 4: "Applications generating information that might be
// encapsulated within BGP messages MUST limit the size of their payload to
// take the maximum message size into account."
func prepareOriginPolicyRemovals(owner *fwdItem, bodies [][]byte, maxSize int, addPath bool) error {
	if len(bodies) > policyRemovalBodiesMax {
		return errPolicyRemovalBodies
	}
	if len(bodies) == 0 {
		return nil
	}
	owner.policyRemovals = make([]fwdPolicyRemoval, len(bodies))
	splitter := message.GetSplitter()
	defer message.PutSplitter(splitter)
	for i, body := range bodies {
		sections, err := wire.ParseUpdateSections(body)
		if err != nil {
			return err
		}
		update := message.Update{WithdrawnRoutes: sections.Withdrawn(body), PathAttributes: sections.Attrs(body), NLRI: sections.NLRI(body)}
		removal := &owner.policyRemovals[i]
		// RFC 8654 Section 4.
		if err := splitter.Split(&update, maxSize, func(family.Family) bool { return addPath }, func(chunk *message.Update) error {
			removal.rawBodies = append(removal.rawBodies, fwdPackUpdateBody(chunk))
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// prepareFwdPolicy preserves the primary body's provenance and materializes
// its source removals before the caller publishes its dedup entry or dispatch.
// A child never owns a queue slot, cache retain, completion or nested group.
// The caller MUST release the parent through fwdPool.releaseItem on failure
// and after dispatch; it releases every already-materialized child too.
//
// RFC 4271 Section 9.1.3: "If a route in Loc-RIB is excluded from a particular
// Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST
// be withdrawn from service by means of an UPDATE message (see 9.2)."
func prepareFwdPolicy(item *fwdItem, owner *ReceivedUpdate, original, effective *wireu.WireUpdate, synthesized bool, removals [][]byte, facts *peerForwardFacts) error {
	// RFC 7911 Section 2.
	if err := prepareFwdProvenance(item, original, effective, synthesized); err != nil {
		return err
	}
	return prepareFwdPolicyRemovals(item, owner, original, removals, facts)
}

// prepareFwdPolicyRemovals stages the accepted source decision independently
// of survivor construction. The caller transfers the group to its survivor or
// dispatches this removal-only parent through the same pending slot and fence.
// RFC 4271 Section 9.1.3: an excluded previously advertised route "MUST
// be withdrawn from service by means of an UPDATE message".
func prepareFwdPolicyRemovals(item *fwdItem, owner *ReceivedUpdate, original *wireu.WireUpdate, removals [][]byte, facts *peerForwardFacts) error {
	if len(removals) == 0 {
		return nil
	}
	if len(removals) > policyRemovalBodiesMax {
		return errPolicyRemovalBodies
	}
	var negotiated *capability.Negotiated
	if item.session != nil {
		negotiated = item.session.Negotiated()
	}
	item.policyRemovals = make([]fwdPolicyRemoval, 0, len(removals))
	item.supersedeKey = 0 // Equal surviving bytes can owe different source removals.
	var cache fwdParseCache
	for _, body := range removals {
		// RFC 4760 Section 8: project before encoding an indivisible native key.
		body, err := projectFwdPolicyRemoval(body, negotiated)
		if err != nil {
			return err
		}
		if body == nil {
			continue
		}
		item.policyRemovals = append(item.policyRemovals, fwdPolicyRemoval{})
		removal := &item.policyRemovals[len(item.policyRemovals)-1]
		child := policyRemovalItem(item, removal)
		wire := wireu.NewWireUpdate(body, original.SourceCtxID())
		wire.SetSourceID(original.SourceID())
		// RFC 4271 Section 4.3; RFC 7911 Section 3; RFC 8654 Section 4.
		built, ok := buildFwdBody(wire, facts.maxMsgSize, facts.sendCtxID, item.peer, facts.addr, &cache)
		if !ok {
			return errPolicyRemovalEncoding
		}
		owner.adoptFwdHandle(built.transcodeBuf)
		removal.rawBodies, removal.updates = built.rawBodies, built.updates
		child.rawBodies, child.updates = built.rawBodies, built.updates
		// RFC 7911 Section 2: each child binds its own source path identifiers.
		err = prepareFwdProvenance(&child, original, wire, true)
		removal.provenance = child.provenance
		if err != nil {
			return err
		}
	}
	return nil
}

// projectFwdPolicyRemoval keeps only negotiated sections of a synthesized
// withdrawal. The final writer still performs source/receipt admission; this
// projection only avoids materializing sections it must refuse regardless of
// ownership. A fully omitted body is nil, never a generic End-of-RIB marker.
// RFC 4760 Section 8 requires both peers to advertise support for the
// particular AFI/SAFI before exchanging its routing information.
func projectFwdPolicyRemoval(body []byte, negotiated *capability.Negotiated) ([]byte, error) {
	if negotiated == nil {
		return body, nil
	}
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return nil, err
	}
	legacy, attrs := sections.Withdrawn(body), sections.Attrs(body)
	changed := false
	if len(legacy) != 0 && !negotiated.SupportsFamily(family.IPv4Unicast) {
		legacy, changed = nil, true
	}
	mp, err := wireu.NewWireUpdate(body, 0).MPUnreach()
	if err != nil {
		return nil, err
	}
	if mp != nil && !negotiated.SupportsFamily(mp.Family()) {
		// atomicSubsetScratch.body emits only MP_UNREACH in this attribute field.
		attrs, changed = nil, true
	}
	if !changed {
		return body, nil
	}
	if len(legacy) == 0 && len(attrs) == 0 {
		return nil, nil
	}
	// RFC 4271 Section 4.3: preserve the eligible sibling's native bytes.
	out := make([]byte, 4+len(legacy)+len(attrs))
	binary.BigEndian.PutUint16(out, uint16(len(legacy))) //nolint:gosec // G115: bounded by the parsed UPDATE field
	copy(out[2:], legacy)
	off := 2 + len(legacy)
	binary.BigEndian.PutUint16(out[off:], uint16(len(attrs))) //nolint:gosec // G115: bounded by the parsed UPDATE field
	copy(out[off+2:], attrs)
	return out, nil
}

// writePolicyRemovals writes at most two non-nesting children before the
// surviving output or EOR. The caller MUST hold the parent's destination
// write lock and have checked its session/source/replay fences once. Children
// MUST carry the same source and destination incarnation as that parent.
// An originated group MUST be flushed before its survivor can fail admission;
// forwarded groups retain their caller's existing batching/flush contract.
//
// RFC 4271 Section 9.1.3: "If a route in Loc-RIB is excluded from a particular
// Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST
// be withdrawn from service by means of an UPDATE message (see 9.2)."
func (s *Session) writePolicyRemovals(item *fwdItem) error {
	if len(item.policyRemovals) == 0 {
		return nil
	}
	if len(item.policyRemovals) > policyRemovalBodiesMax {
		return errPolicyRemovalBodies
	}
	parent := s.sentForward
	costWithheld := s.sentAIGPCostWithheld
	s.sentAIGPCostWithheld = false
	defer func() { s.sentForward = parent; s.sentAIGPCostWithheld = costWithheld }()
	for i := range item.policyRemovals {
		child := policyRemovalItem(item, &item.policyRemovals[i])
		s.sentForward = &child
		child.writeBody = 0
		for _, body := range child.rawBodies {
			// RFC 4271 Section 9.1.3; RFC 7911 Section 5.
			var err error
			if child.originated {
				// RFC 7606 Section 5.1; RFC 8654 Section 4. A policy body
				// still owes ordinary shaping and destination-size splitting.
				err = s.writeOrdinaryUpdateBody(body, false, true)
			} else {
				err = s.writeRawUpdateBody(body)
			}
			if err != nil {
				return err
			}
			child.writeBody++
		}
		for _, update := range child.updates {
			// RFC 4271 Section 9.1.3; RFC 7911 Section 5.
			if err := s.writeUpdatePreFiltered(update); err != nil {
				return err
			}
			child.writeBody++
		}
	}
	if item.originated {
		// A later main-body next-hop refusal is route-scoped. Immediate source
		// removals MUST be flushed first; an active RIB owner's deferred
		// withdrawal or replacement remains owed until its decision is flushed.
		// RFC 4271 Section 9.1.3.
		return s.flushWrites()
	}
	return nil
}
