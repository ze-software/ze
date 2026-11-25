// Design: docs/architecture/update-building.md -- one final ownership and duplicate boundary
// Related: session_ownership.go -- section and recordSection, the walks these visitors serve
// Related: adj_rib_out_store.go -- the entry slots recordNLRI installs
package reactor

import (
	"crypto/sha256"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// adjOutSectionWalk is one nlrisplit walk's state: the section's arguments and
// what its visitor accumulates. It lives in the pooled adjOutWrite, and the
// visitors are method values bound once per pooled write (adjOutWritePool), so
// a section walk captures nothing and allocates nothing per write. A func
// literal passed to the registered split function escapes with every variable
// it captures.
//
// section and recordSection each set it before their walk and zero it after,
// so it holds no reference into a pooled buffer between walks. Walks never
// nest: neither visitor reaches section or recordSection.
type adjOutSectionWalk struct {
	dst    []byte
	attrs  []byte
	stored []byte
	// signature and readable are section's argument, or recordSection's
	// signature read once per section (signatureRead).
	signature         adjRIBOutSignature
	family            family.Family
	sectionOctets     int
	n                 int
	occurrence        int
	cause             error
	withdraw          bool
	multiprotocol     bool
	readable          bool
	signatureRead     bool
	replayDigest      [sha256.Size]byte
	replayDigestRead  bool
	replayDigestValid bool
}

// initialRelayDigest lazily fingerprints the normalized route attribute
// bytes. The live scan reaches it only for a candidate original UPDATE, and
// the record walk shares one digest across the initial body section.
func (walk *adjOutSectionWalk) initialRelayDigest() []byte {
	if !walk.replayDigestRead {
		walk.replayDigestRead = true
		walk.replayDigestValid = walk.readInitialRelayDigest()
	}
	if !walk.replayDigestValid {
		return nil
	}
	return walk.replayDigest[:]
}

// readInitialRelayDigest retains one path's attributes, irrespective of what
// else shares the body. RFC 4760 Sections 3 and 4 put a family's next hop in
// MP_REACH, and its independent withdrawals in MP_UNREACH. RFC 4271 Section
// 4.3 makes the length width a property of the whole attribute, including
// its other NLRI. The digest keeps the next hop used by this path and writes
// a canonical header for that reduced MP_REACH payload.
func (walk *adjOutSectionWalk) readInitialRelayDigest() bool {
	digest := sha256.New()
	iter := attribute.NewAttrIterator(walk.attrs)
	start := 0
	reached := false
	for code, flags, value, ok := iter.Next(); ok; code, flags, value, ok = iter.Next() {
		end := iter.Offset()
		encoded := walk.attrs[start:end]
		start = end
		switch code {
		case attribute.AttrMPUnreachNLRI:
			continue
		case attribute.AttrNextHop:
			// RFC 4760 Section 3 uses only the MP_REACH next hop for this
			// path. NEXT_HOP belongs to an independent legacy sibling.
			if walk.multiprotocol {
				continue
			}
		case attribute.AttrMPReachNLRI:
			if !walk.multiprotocol {
				continue
			}
			fam, off, err := pathsLimitMP(value, true)
			// refuse-by-default: guards initialRelayDigest -- malformed next-hop data cannot prove a duplicate.
			if err != nil {
				return false
			}
			if fam != walk.family {
				continue
			}
			if reached {
				return false
			}
			if len(value)-off != walk.sectionOctets {
				return false
			}
			var header [4]byte
			n := attribute.WriteHeaderTo(header[:], 0, flags&^attribute.FlagExtLength, code, uint16(off)) //nolint:gosec // G115: fixed bytes plus the eight-bit next-hop length.
			_, _ = digest.Write(header[:n])
			_, _ = digest.Write(value[:off])
			reached = true
			continue
		}
		_, _ = digest.Write(encoded)
	}
	if iter.Remaining() != 0 {
		return false
	}
	if walk.multiprotocol != reached {
		return false
	}
	digest.Sum(walk.replayDigest[:0])
	return true
}

// allowNLRI is section's visitor: it admits one NLRI and copies it into dst.
func (w *adjOutWrite) allowNLRI(raw []byte) bool {
	walk := &w.walk
	if walk.cause != nil {
		return true
	}
	// RFC 7911 Section 5; final destination framing, original provenance.
	allowed, admissionErr := w.allow(walk.family, raw, walk.withdraw, walk.multiprotocol, walk.occurrence, walk.signature, walk.readable)
	walk.occurrence++
	// One refused NLRI fails the whole section. For errAdjOutPolicy that
	// relies on an invariant: every NLRI of an initial, replay or
	// recovery section shares one source and one original import epoch,
	// because the epoch is the item's (fwdItem.sourcePolicyEpoch) and a
	// packed section joins only items with equal epochs
	// (recoveryPackJoins, the forward bucket's run test). A stale NLRI
	// therefore means the whole section is stale.
	if admissionErr != nil {
		walk.cause = admissionErr
		return true
	}
	if !allowed {
		w.removed++
		return true
	}
	if walk.dst != nil {
		copy(walk.dst[walk.n:], raw)
	}
	walk.n += len(raw)
	return true
}

// recordNLRI is recordSection's visitor: it records one NLRI of the final body.
func (w *adjOutWrite) recordNLRI(raw []byte) bool {
	walk := &w.walk
	if walk.cause != nil {
		return true
	}
	key, keyErr := w.key(walk.family, raw, walk.withdraw)
	if keyErr != nil {
		walk.cause = keyErr
		return true
	}
	if walk.withdraw {
		w.recordDeletion(key)
		w.withdrawnRoutes++
		return true
	}
	owner, ok := w.owners[key]
	if !ok {
		walk.cause = errAdjOutProvenance
		return true
	}
	if w.sourceSet && w.source != owner.source {
		walk.cause = errAdjOutProvenance
		return true
	}
	if w.sourceSet && w.sourceReceipt != owner.sourceReceipt {
		walk.cause = errAdjOutProvenance
		return true
	}
	w.source, w.sourceSet = owner.source, true
	w.policyEpoch = owner.policyEpoch
	w.sourceReceipt = owner.sourceReceipt
	if w.session.onMessageReceived != nil {
		ordinal := w.ordinals[walk.family]
		if owner.addPath {
			if w.pathSources == nil {
				w.pathSources = make([]wireu.SentPathSource, 0, len(w.owners))
			}
			w.pathSources = append(w.pathSources, wireu.SentPathSource{
				Family: walk.family, Ordinal: ordinal, PathID: owner.received,
			})
		}
		w.ordinals[walk.family] = ordinal + 1
	}
	if w.table.routes == nil {
		w.table.routes = make(map[adjOutKey]*adjRIBOutRoute)
	}
	entry := w.table.routes[key]
	if entry == nil {
		entry = w.table.entries.take()
		w.table.routes[key] = entry
		if taken, held := w.table.takeReleased(key, owner); held {
			// The released receipt's slot, its identifier reservation
			// and ingress binding, passes to the entry; setOwner below
			// rebinds it if another owner takes the key.
			entry.owner = taken
		} else if key.addPath {
			fwdPathIDs.retain(key.path.received)
		}
	} else if !key.addPath && !adjOutSameOwner(entry.owner, owner) {
		// RFC 8212 Section 3: a recovery survivor replacing a receipt
		// its invalidation denied keeps the receipt's record, as a
		// removal does, so the abort restore can be owed to the
		// election between the two paths (oweReleasedRestore).
		w.keepReleased(key, entry)
	}
	w.table.setOwner(&key, entry, owner)
	if item := w.session.sentForward; item != nil && item.authority == adjOutSelected {
		w.table.setSelected(key, entry, item.selected.output)
		w.table.removeOwed(key, w.table.owed[key])
	} else if item == nil || item.authority != adjOutExpected {
		w.table.setSelected(key, entry, selectedOutput{})
	}
	if _, contested := w.contested[key]; contested {
		entry.contested = true
	}
	delete(w.table.tombstones, key) // An installed entry carries the bit itself.
	if !w.sourceBound() {
		// An originated route is not a candidate of the election and is
		// not overridden by it: it decides an owed key itself.
		w.table.removeOwed(key, w.table.owed[key])
	}
	if w.session.sentForward == nil || w.session.sentForward.authority != adjOutExpected {
		replay, _ := w.session.sentMeta["replay"].(bool)
		if !replay || entry.revision == 0 {
			entry.revision = w.session.sentReceipt.MessageID()
		}
	} else if entry.revision == 0 {
		// A re-installed released receipt keeps the revision its owner
		// names, so the owner's later exact removal still matches it.
		entry.revision = w.session.sentForward.expectedMessage
	}
	if item := w.session.sentForward; item != nil && item.authority == adjOutInitial {
		entry.revision = item.expectedMessage
	}
	entry.signature, entry.exactNLRI, entry.initialRelay = nil, nil, false
	if owner.source == nil && !walk.signatureRead {
		walk.signature, walk.readable = announceSignature(walk.attrs, walk.family, walk.sectionOctets)
		walk.signatureRead = true
	}
	if owner.source == nil && walk.readable {
		if walk.stored == nil {
			walk.stored = walk.signature.store()
		}
		entry.signature = walk.stored
	}
	if item := w.session.sentForward; item != nil && item.authority == adjOutForwarded && item.initialUpdate && owner.message != 0 && owner.sourceReceipt != 0 {
		if digest := walk.initialRelayDigest(); digest != nil {
			if walk.stored == nil {
				walk.stored = append([]byte(nil), digest...)
			}
			entry.signature, entry.initialRelay = walk.stored, true
		}
	}
	// Selected and relayed retirement reconstruct the withdrawal from this
	// current slot, without asking its source or re-reading a possibly replaced
	// candidate. Retain framing the semantic key intentionally discards for
	// these outputs as well as for local duplicate evidence.
	keepNative := entry.initialRelay || entry.selection.owner.Done() != nil
	if owner.source == nil {
		keepNative = walk.readable
	} else if !keepNative {
		if nlrisplit.GetWithdrawalWriter(walk.family) == nil {
			if !ribevents.IsFlowSpec(walk.family) {
				keepNative = true
			}
		}
	}
	if keepNative {
		if !adjOutExactNLRI(entry, key, raw) {
			entry.exactNLRI = append([]byte(nil), raw...)
		}
	}
	w.announcedRoutes++
	return true
}
