// Design: docs/architecture/update-building.md -- one final ownership and duplicate boundary
// Related: session_ownership.go -- section and recordSection, the walks these visitors serve
// Related: adj_rib_out_store.go -- the entry slots recordNLRI installs
package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/wireu"
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
	signature     adjRIBOutSignature
	family        family.Family
	sectionOctets int
	n             int
	occurrence    int
	cause         error
	withdraw      bool
	multiprotocol bool
	readable      bool
	signatureRead bool
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
	w.source, w.sourceSet = owner.source, true
	w.policyEpoch = owner.policyEpoch
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
	entry.signature, entry.exactNLRI = nil, nil
	if owner.source == nil && !walk.signatureRead {
		walk.signature, walk.readable = announceSignature(walk.attrs, walk.family, walk.sectionOctets)
		walk.signatureRead = true
	}
	if owner.source == nil && walk.readable {
		if walk.stored == nil {
			walk.stored = walk.signature.store()
		}
		entry.signature = walk.stored
		if !adjOutExactNLRI(entry, key, raw) {
			entry.exactNLRI = append([]byte(nil), raw...)
		}
	}
	w.announcedRoutes++
	return true
}
