// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: storage/peerrib.go -- the in-place denial bound selection reads
// Related: rib_sent_lifecycle.go -- the sent inventory a denial withdraws
// Related: rib.go -- peer down drops the transaction

package rib

import (
	"context"
	"errors"
	"net/netip"
	"strconv"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/sourcepolicy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourcePolicyOwner opens every refusal bgp-rib's ledger returns.
const sourcePolicyOwner sourcepolicy.Owner = "bgp-rib"

// sourcePolicyPin names one stored path by the identity it had at prepare.
// The path itself stays in its PeerRIB: a pin restores only while the store
// still holds a path with this received message and attribute fingerprint, so
// a path the source withdrew, GR purged or a later UPDATE replaced is never
// restored.
type sourcePolicyPin struct {
	message     uint64
	fingerprint uint64
	attrLen     uint32
}

// sourcePolicySent is a forwarded copy the apply withdrew from a destination.
// It keeps the sent entry's pool reference, so abort can announce it again and
// return it to the sent inventory; finalize, abort and peer down release every
// reference the inventory did not take back, exactly once.
type sourcePolicySent struct {
	destination netip.Addr
	fam         family.Family
	key         ribOutKey
	entry       ribOutEntry
}

// sourcePolicyTxn is one source's transaction and its undo pins. Guarded by
// RIBManager.peerMu; the received pin maps are never mutated after prepare.
// previousBound is the PeerRIB's denial bound before apply raised it, which an
// abort puts back.
type sourcePolicyTxn struct {
	ledger        sourcepolicy.Ledger
	previousBound uint64
	addPath       map[family.Family]bool
	received      map[family.Family]map[string]sourcePolicyPin
	sent          []sourcePolicySent
}

// ledgerOf is the transaction's ledger, nil when the source holds none.
func (txn *sourcePolicyTxn) ledgerOf() *sourcepolicy.Ledger {
	if txn == nil {
		return nil
	}
	return &txn.ledger
}

// releaseSent drops the transaction's sent references. MUST hold peerMu.
func (txn *sourcePolicyTxn) releaseSent() {
	for i := range txn.sent {
		txn.sent[i].entry.release()
	}
	txn.sent = nil
}

// handleSourcePolicy is bgp-rib's source-policy lifecycle handler. Source.Peer
// is matched as the peer address the received-UPDATE events carry, parsed to the
// netip.Addr that keys bgpPeers. It runs on the SDK source-policy worker,
// concurrently with event delivery: each phase changes state under peerMu, then
// releases it before any engine RPC, election or best-change publication, so
// no lock is held while downstream settles. Each phase returns only after its
// sent-inventory writes were answered by the engine and its re-elections were
// emitted on the bus (reelectSourcePolicy says what that settles and what it
// does not). Safe for concurrent use.
func (r *RIBManager) handleSourcePolicy(_ context.Context, input rpc.SourcePolicyInput) error {
	return sourcePolicyOwner.Handle(input, sourcepolicy.Phases{
		Prepare: r.prepareSourcePolicy, Apply: r.applySourcePolicy,
		Abort: r.abortSourcePolicy, Finalize: r.finalizeSourcePolicy,
	})
}

// prepareSourcePolicy validates the captured identity and pins the identity
// of every path the source holds. It refuses a source whose session is down, a
// token older than a path this RIB already holds, and a source another open
// transaction holds, before pinning anything. A pin copies the path's NLRI key
// and its identity numbers; no route, attribute or UPDATE is copied.
func (r *RIBManager) prepareSourcePolicy(peer netip.Addr, input rpc.SourcePolicyInput) error {
	r.peerMu.Lock()
	defer r.peerMu.Unlock()
	if done, err := sourcePolicyOwner.Reprepare(r.sourcePolicy[peer].ledgerOf(), input); done {
		return err
	}
	if !r.peerUp[peer] {
		return errors.New("bgp-rib: source " + input.Source.Peer + " has no established session to prepare")
	}
	txn := &sourcePolicyTxn{
		ledger:   sourcepolicy.NewLedger(input),
		addPath:  make(map[family.Family]bool),
		received: make(map[family.Family]map[string]sourcePolicyPin),
	}
	if peerRIB := r.bgpPeers[peer]; peerRIB != nil {
		if stale := pinSourcePolicy(peerRIB, txn, input.Source.PolicyEpoch); stale {
			return errors.New("bgp-rib: transaction " + input.TransactionID + " captured policy epoch of source " +
				input.Source.Peer + " is older than a path already admitted; capture the source again")
		}
	}
	if old := r.sourcePolicy[peer]; old != nil {
		old.releaseSent()
	}
	if r.sourcePolicy == nil {
		r.sourcePolicy = make(map[netip.Addr]*sourcePolicyTxn)
	}
	r.sourcePolicy[peer] = txn
	return txn.ledger.Record(input, sourcepolicy.StagePrepared, nil)
}

// pinSourcePolicy records the identity of every path in peerRIB and reports a
// path admitted under an epoch newer than captured.
func pinSourcePolicy(peerRIB *storage.PeerRIB, txn *sourcePolicyTxn, captured uint64) (stale bool) {
	for _, fam := range peerRIB.Families() {
		// Read before IterateFamily: IsAddPath takes the lock the walk holds.
		txn.addPath[fam] = peerRIB.IsAddPath(fam)
		pins := make(map[string]sourcePolicyPin)
		peerRIB.IterateFamily(fam, func(raw []byte, entry storage.RouteEntry) bool {
			if entry.SourcePolicyEpoch > captured {
				stale = true
				return false
			}
			pins[string(raw)] = sourcePolicyPin{message: entry.MsgID, fingerprint: entry.AttrFingerprint, attrLen: entry.AttrLen}
			return true
		})
		txn.received[fam] = pins
	}
	return stale
}

// applySourcePolicy makes every path the source held at or below the captured
// epoch ineligible in its PeerRIB, withdraws the forwarded copies those paths
// own at every destination, and re-elects each pinned route, so an
// alternative replaces the denied best or the route is withdrawn downstream.
// Paths the source's session admits later carry a newer epoch and stay
// eligible; other sources and local origins are not touched. No DOWN or UP is
// synthesized and no GR state changes. The bound the PeerRIB held before is
// kept for abort.
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (r *RIBManager) applySourcePolicy(peer netip.Addr, input rpc.SourcePolicyInput) error {
	r.peerMu.Lock()
	txn := r.sourcePolicy[peer]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		r.peerMu.Unlock()
		return err
	}
	if duplicate {
		r.peerMu.Unlock()
		return nil
	}
	if peerRIB := r.bgpPeers[peer]; peerRIB != nil {
		txn.previousBound = peerRIB.SourcePolicyDeniedThrough()
		peerRIB.DenySourcePolicyThrough(txn.ledger.Source().PolicyEpoch)
	}
	var denied removedReceivedOwners
	for fam, pins := range txn.received {
		for raw, pin := range pins {
			denied.add(fam, []byte(raw), txn.addPath[fam], pin.message)
		}
	}
	writes := r.pinDeniedSentLocked(peer, denied, txn)
	err = txn.ledger.Record(input, sourcepolicy.StageApplied, nil)
	received, addPath := txn.received, txn.addPath
	r.peerMu.Unlock()

	// RFC 8212 Section 3: settle every destination before the receipt.
	r.dispatchSentLifecycle(writes)
	r.reelectSourcePolicy(received, addPath)
	return err
}

// pinDeniedSentLocked withdraws the forwarded copies the denied received
// owners hold and keeps their sent entries in the transaction for abort.
// MUST hold peerMu and dispatch the writes after releasing it.
func (r *RIBManager) pinDeniedSentLocked(source netip.Addr, denied removedReceivedOwners, txn *sourcePolicyTxn) []sentLifecycleWrite {
	r.fenceSentLifecycleLocked(source)
	var writes []sentLifecycleWrite
	sourcePeer := source.String()
	for destination, destinations := range r.ribOut {
		for fam, owners := range denied {
			routes := destinations[fam]
			for key, entry := range routes {
				if entry.SourcePeer != sourcePeer {
					continue
				}
				owner, ok := sentReceivedOwner(fam, key, &entry)
				if !ok {
					continue
				}
				if _, deny := owners[owner]; !deny {
					continue
				}
				writes = append(writes, sentRemoval(destination, fam, key, &entry))
				delete(routes, key)
				txn.sent = append(txn.sent, sourcePolicySent{destination: destination, fam: fam, key: key, entry: entry})
			}
			if len(routes) == 0 {
				delete(destinations, fam)
			}
		}
		if len(destinations) == 0 {
			delete(r.ribOut, destination)
		}
	}
	return writes
}

// abortSourcePolicy restores each pinned path the PeerRIB still holds as the
// same path, restamped to the abort's fresh epoch so the engine's writers
// accept it again. After an apply it also drops every other denied path rather
// than keeping it, puts back the denial bound the PeerRIB held before the
// apply, returns the forwarded copies of restored paths to the sent inventory
// and announces them again. It re-elects, and runs no import evaluation and
// asks for no Route Refresh.
//
// RFC 8212 Section 3 (the decision this abort reverses).
func (r *RIBManager) abortSourcePolicy(peer netip.Addr, input rpc.SourcePolicyInput) error {
	r.peerMu.Lock()
	txn := r.sourcePolicy[peer]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		r.peerMu.Unlock()
		return err
	}
	if duplicate {
		r.peerMu.Unlock()
		return nil
	}
	bound, target := txn.ledger.Source().PolicyEpoch, input.NextPolicyEpoch
	applied := txn.ledger.Stage() == sourcepolicy.StageApplied
	var restored removedReceivedOwners
	affected := make(map[family.Family]map[string]sourcePolicyPin, len(txn.received))
	if peerRIB := r.bgpPeers[peer]; peerRIB != nil {
		for fam, pins := range txn.received {
			addPath := txn.addPath[fam]
			changed := make(map[string]sourcePolicyPin)
			peerRIB.ModifyFamilyAllKeyed(fam, func(raw []byte, entry *storage.RouteEntry) {
				pin, pinned := pins[string(raw)]
				if !pinned {
					return
				}
				if entry.SourcePolicyEpoch > bound {
					return
				}
				if entry.MsgID != pin.message {
					return
				}
				if entry.AttrFingerprint != pin.fingerprint {
					return
				}
				if entry.AttrLen != pin.attrLen {
					return
				}
				entry.SourcePolicyEpoch = target
				restored.add(fam, raw, addPath, entry.MsgID)
				changed[string(raw)] = pin
			})
			if applied {
				peerRIB.RemoveFamilyMatching(fam, func(entry storage.RouteEntry) bool {
					return entry.SourcePolicyEpoch <= bound
				}, func(raw []byte, message uint64, _ bool) {
					changed[string(raw)] = sourcePolicyPin{message: message}
				})
			}
			affected[fam] = changed
		}
		if applied {
			peerRIB.RestoreSourcePolicyDenial(txn.previousBound)
		}
	}
	announces := r.restoreSentLocked(txn, restored, target)
	txn.releaseSent()
	txn.received = nil
	err = txn.ledger.Record(input, sourcepolicy.StageAborted, nil)
	addPath := txn.addPath
	r.peerMu.Unlock()

	// RFC 8212 Section 3: settle every destination before the receipt.
	for _, write := range announces {
		var digits [20]byte
		meta := map[string]any{metaKeyReplay: true,
			bgptypes.SentOwnerMessageMeta: string(strconv.AppendUint(digits[:0], write.message, 10))}
		r.updateRouteWithProvenance(write.peer, write.command, meta, target) //nolint:errcheck // The helper logs the failure; each destination's announcement stands alone.
	}
	r.reelectSourcePolicy(affected, addPath)
	return err
}

// restoreSentLocked returns each withdrawn forwarded copy whose received owner
// was restored to the sent inventory, stamped with the restored path's epoch so
// the copy reads as current, and builds its announcement. The replay
// announcement does not record a sent entry (handleSentStructured skips replay
// meta), so without this the destination would hold a route the inventory
// never withdraws. The inventory takes over the copy's pool reference; a copy
// not restored, or whose key a later announcement already holds at that
// destination, stays in txn.sent for releaseSent. MUST hold peerMu.
func (r *RIBManager) restoreSentLocked(txn *sourcePolicyTxn, restored removedReceivedOwners, target uint64) []sentLifecycleWrite {
	var writes []sentLifecycleWrite
	kept := txn.sent[:0]
	for i := range txn.sent {
		s := txn.sent[i]
		owner, ok := sentReceivedOwner(s.fam, s.key, &s.entry)
		if !ok {
			kept = append(kept, s)
			continue
		}
		if _, back := restored[s.fam][owner]; !back {
			kept = append(kept, s)
			continue
		}
		destinations := r.ribOut[s.destination]
		if _, taken := destinations[s.fam][s.key]; taken {
			kept = append(kept, s)
			continue
		}
		writes = append(writes, sentLifecycleWrite{peer: s.destination.String(),
			command: bgp.FormatAnnounceCommand(reconstructRoute(s.entry, s.fam, s.key)), message: s.entry.MsgID})
		if destinations == nil {
			destinations = make(map[family.Family]map[ribOutKey]ribOutEntry)
			r.ribOut[s.destination] = destinations
		}
		if destinations[s.fam] == nil {
			destinations[s.fam] = make(map[ribOutKey]ribOutEntry)
		}
		s.entry.SourcePolicyEpoch = target
		destinations[s.fam][s.key] = s.entry
	}
	clear(txn.sent[len(kept):])
	txn.sent = kept
	return writes
}

// finalizeSourcePolicy accepts an applied transaction. The ledger admits
// finalize only after apply (abort is the path after a prepare that was not
// applied). It removes the denied paths from the PeerRIB, so no denied state
// is retained for a later recovery, and releases the held sent entries; the
// denial bound stays, so a path still carrying the old epoch can never become
// eligible again. The paths were already excluded and re-elected at apply, so
// no election runs here.
func (r *RIBManager) finalizeSourcePolicy(peer netip.Addr, input rpc.SourcePolicyInput) error {
	r.peerMu.Lock()
	defer r.peerMu.Unlock()
	txn := r.sourcePolicy[peer]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		return err
	}
	if duplicate {
		return nil
	}
	if peerRIB := r.bgpPeers[peer]; peerRIB != nil {
		bound := txn.ledger.Source().PolicyEpoch
		for _, fam := range peerRIB.Families() {
			peerRIB.RemoveFamilyMatching(fam, func(entry storage.RouteEntry) bool {
				return entry.SourcePolicyEpoch <= bound
			}, func([]byte, uint64, bool) {})
		}
	}
	txn.releaseSent()
	txn.received = nil
	return txn.ledger.Record(input, sourcepolicy.StageFinalized, nil)
}

// reelectSourcePolicy re-runs the election of every route keys names and
// publishes the changes, one batch per family, before returning. Publication
// is the bus emit: an engine subscriber has run when it returns, while a
// plugin-process subscriber (redistribution, which feeds destination writes)
// has the batch queued, not applied. MUST NOT hold peerMu.
func (r *RIBManager) reelectSourcePolicy(keys map[family.Family]map[string]sourcePolicyPin, addPath map[family.Family]bool) {
	for fam, pins := range keys {
		var changes []bestChangeEntry
		for raw := range pins {
			if change, changed := r.checkBestPathChange(fam, []byte(raw), addPath[fam], nil); changed {
				changes = append(changes, change)
			}
		}
		if len(changes) > 0 {
			publishBestChanges(changes, fam)
		}
	}
	r.reconcileFlowSpecs()
}

// dropSourcePolicyLocked is the ordinary cleanup when the source's session
// ends: the transaction's references are released, and a later phase for it is
// refused rather than applied to a successor session. A denial bound on a
// PeerRIB that GR retains stays, so its denied paths remain ineligible until
// GR purges them. A transaction lost here is not restored locally: the
// coordinator's later abort is refused, and the coordinator reports the
// compensation incomplete, naming the peer and this owner (fail-closed).
// MUST hold peerMu.
func (r *RIBManager) dropSourcePolicyLocked(peer netip.Addr) {
	if txn := r.sourcePolicy[peer]; txn != nil {
		txn.releaseSent()
		delete(r.sourcePolicy, peer)
	}
}
