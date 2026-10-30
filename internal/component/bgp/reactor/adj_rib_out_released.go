// Design: docs/architecture/bgp/structural-forwarding.md -- released receipts of an in-place import invalidation
// RFC: rfc/short/rfc8212.md -- Section 3, the abort restore of a denied receipt
// Related: adj_rib_out.go -- the Adj-RIB-Out table that holds the records
// Related: session_ownership.go -- the final writer that keeps, re-installs and owes them
// Related: source_policy_reload.go -- the invalidation window whose release drops them
package reactor

import (
	"slices"

	"github.com/ze-software/ze/internal/core/family"
)

// adjOutReleased is one released receipt: the removed entry's owner and
// revision, and window, the apply epoch of the invalidation that removed it
// (sourcePolicyWindow.applied).
type adjOutReleased struct {
	owner    adjOutPath
	revision uint64
	window   uint64
}

// keepReleased keeps entry as a released receipt, with its slot, when this
// deletion removes an exact receipt because its source's import admission was
// superseded by an invalidation still open: the owner's exact removal
// (adjOutExpected) or the recovery decision of an owed one (adjOutRecovery),
// of a source-owned entry whose session is current, while the source's live
// epoch is the one that invalidation's apply issued and the entry's is older.
// The record is stamped with that epoch, so only that invalidation's abort
// restore can re-install it (sentOwnerCurrent); any other deletion keeps no
// record. A key keeps one record per owner: at a destination without ADD-PATH
// send, one reload can invalidate two sources whose receipts replaced each
// other on the key, and each abort restore needs its own record. It reports
// whether it kept the slot. MUST hold mu.
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (w *adjOutWrite) keepReleased(key adjOutKey, entry *adjRIBOutRoute) bool {
	item := w.session.sentForward
	if item == nil {
		return false
	}
	if item.authority != adjOutExpected && item.authority != adjOutRecovery {
		return false
	}
	if entry.owner.source == nil {
		return false
	}
	if entry.revision == 0 {
		return false
	}
	if sourcePolicyCurrent(entry.owner.source, entry.owner.policyEpoch) {
		return false
	}
	if !adjOutOwnerSessionCurrent(entry.owner) {
		return false
	}
	open := entry.owner.source.sourcePolicyWindow.Load()
	if open == nil {
		return false
	}
	if !sourcePolicyCurrent(entry.owner.source, open.applied) {
		return false
	}
	record := adjOutReleased{owner: entry.owner, revision: entry.revision, window: open.applied}
	if w.table.released == nil {
		w.table.released = make(map[adjOutKey][]adjOutReleased)
	}
	records := w.table.released[key]
	for i := range records {
		if adjOutSameOwner(records[i].owner, entry.owner) {
			// One record per owner: the newer receipt replaces it.
			records[i] = record
			return true
		}
	}
	w.table.released[key] = append(records, record)
	return true
}

// releasedReceipt answers a sent-owner write that meets no entry: an
// announcement naming the revision of a released receipt whose source session
// is still current gets that record, for the caller's fence
// (sentOwnerCurrent). The owner's removal of the receipt drops the record and
// writes nothing, since the destination holds no route to remove. MUST hold
// mu.
func (w *adjOutWrite) releasedReceipt(key adjOutKey, revision uint64, withdraw bool) (adjOutReleased, bool) {
	if revision == 0 {
		return adjOutReleased{}, false
	}
	i, kept := w.table.releasedRevision(key, revision)
	if !kept {
		return adjOutReleased{}, false
	}
	released := w.table.released[key][i]
	if withdraw {
		w.table.dropReleasedAt(key, i)
		return adjOutReleased{}, false
	}
	if !adjOutOwnerSessionCurrent(released.owner) {
		w.table.dropReleasedAt(key, i)
		return adjOutReleased{}, false
	}
	return released, true
}

// oweReleasedRestore owes the abort restore of a released receipt whose
// non-ADD-PATH key another owner took while it was released: a recovery
// survivor, or the abort restore of another source invalidated in the same
// reload. The restore names the receipt by its revision and the record
// supplies the owner. The fence is the one a restore meeting no entry passes
// (sentOwnerCurrent), so a restore of another invalidation, of a superseded
// epoch or of a departed session stays refused. An owed restore is not
// written: the recovery election re-runs over the key and writes the winner
// among the restored paths and the survivor. MUST hold mu.
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (w *adjOutWrite) oweReleasedRestore(fam family.Family, key adjOutKey, entry *adjRIBOutRoute, revision, epoch uint64, raw []byte) {
	if key.addPath {
		return // Another owner never takes a released ADD-PATH slot.
	}
	if revision == 0 {
		return
	}
	i, held := w.table.releasedRevision(key, revision)
	if !held {
		return
	}
	released := w.table.released[key][i]
	if adjOutSameOwner(entry.owner, released.owner) {
		return
	}
	if !adjOutOwnerSessionCurrent(released.owner) {
		return
	}
	if !sentOwnerCurrent(released.owner, released.window, epoch) {
		return
	}
	owner := released.owner
	owner.policyEpoch = epoch
	w.oweAnnouncement(fam, key, entry, owner, raw)
}

// takeReleased hands the released receipt an install of key re-uses to the
// new entry, with its slot (identifier reservation and ingress binding): an
// ADD-PATH key's only record, whoever installs it (setOwner rebinds it), or
// the record of owner itself. Another owner taking a non-ADD-PATH key (a
// recovery survivor, or another source's abort restore) holds no slot of a
// record, so the records stay: each abort restore is then owed to the recovery
// election (oweReleasedRestore), and each invalidation's release drops its own
// (Reactor.dropReleasedSource). MUST hold mu.
func (a *adjRIBOut) takeReleased(key adjOutKey, owner adjOutPath) (adjOutPath, bool) {
	records := a.released[key]
	for i := range records {
		if !key.addPath && !adjOutSameOwner(records[i].owner, owner) {
			continue
		}
		taken := records[i].owner
		a.removeReleased(key, i)
		return taken, true
	}
	return adjOutPath{}, false
}

// releasedByOther reports whether key holds a released receipt of an owner
// other than owner: its slot stays reserved for that receipt's abort restore.
// MUST hold mu.
func (a *adjRIBOut) releasedByOther(key adjOutKey, owner adjOutPath) bool {
	for _, released := range a.released[key] {
		if !adjOutSameOwner(released.owner, owner) {
			return true
		}
	}
	return false
}

// releasedRevision finds key's released receipt whose revision is revision.
// A revision is a sent message ID, so at most one record of a key carries it.
// MUST hold mu.
func (a *adjRIBOut) releasedRevision(key adjOutKey, revision uint64) (int, bool) {
	for i, released := range a.released[key] {
		if released.revision == revision {
			return i, true
		}
	}
	return 0, false
}

// releasedCount counts the released receipts the table holds, over every key.
// MUST hold mu.
func (a *adjRIBOut) releasedCount() int {
	count := 0
	for _, records := range a.released {
		count += len(records)
	}
	return count
}

// dropReleasedSource drops every receipt source released here and frees its
// slot. It takes mu, which every final writer holds for its whole write.
func (a *adjRIBOut) dropReleasedSource(source *Peer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, records := range a.released {
		// Backwards, so a removal does not shift a record still to visit.
		for i, released := range slices.Backward(records) {
			if released.owner.source == source {
				a.dropReleasedAt(key, i)
			}
		}
	}
}

// dropReleasedOwner drops key's released receipt of owner, the source's own
// withdrawal naming the same path. MUST hold mu.
func (a *adjRIBOut) dropReleasedOwner(key adjOutKey, owner adjOutPath) {
	for i, released := range a.released[key] {
		if adjOutSameOwner(released.owner, owner) {
			a.dropReleasedAt(key, i)
			return
		}
	}
}

// dropReleasedAt forgets key's released receipt at index i and frees its
// slot. MUST hold mu.
func (a *adjRIBOut) dropReleasedAt(key adjOutKey, i int) {
	owner := a.released[key][i].owner
	a.removeReleased(key, i)
	a.releasePath(&key, &adjRIBOutRoute{owner: owner})
}

// removeReleased deletes key's record at index i without freeing its slot,
// and the key once it holds none. MUST hold mu.
func (a *adjRIBOut) removeReleased(key adjOutKey, i int) {
	records := a.released[key]
	records = append(records[:i], records[i+1:]...)
	if len(records) == 0 {
		delete(a.released, key)
		return
	}
	a.released[key] = records
}
