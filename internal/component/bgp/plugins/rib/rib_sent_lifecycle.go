// Design: docs/architecture/plugin/rib-storage-design.md -- one sent lifecycle inventory.
package rib

import (
	"context"
	"encoding/binary"
	"net/netip"
	"slices"
	"strconv"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// sentLifecycleWrite owns encoded command bytes, never borrowed pool storage.
// The collecting caller MUST hold peerMu; it MUST dispatch after releasing it.
type sentLifecycleWrite struct {
	peer    string
	command string
	message uint64
}

// receivedOwner retains native identity, received ADD-PATH presence/identifier,
// and generation. One UPDATE can carry several paths for the same prefix.
type receivedOwner struct {
	key     ribOutKey
	message uint64
	addPath bool
}

func receivedOwnerKey(fam family.Family, raw []byte, addPath bool, message uint64) (receivedOwner, bool) {
	if fam.SAFI == family.SAFIMPLSLabel {
		// The received FamilyRIB stores labeled routes as stripped CIDR bytes.
		fam.SAFI = family.SAFIUnicast
	}
	key, ok := ribOutRouteKey(fam, raw, addPath)
	return receivedOwner{key: key, message: message, addPath: addPath}, ok
}

// removedReceivedOwners is an operation-local index, never a retained inventory.
// Its zero value selects nothing. Callers MUST collect it before removing the
// received entries and while holding peerMu.
type removedReceivedOwners map[family.Family]map[receivedOwner]struct{}

func (removed *removedReceivedOwners) add(fam family.Family, raw []byte, addPath bool, message uint64) {
	owner, ok := receivedOwnerKey(fam, raw, addPath, message)
	if !ok {
		return
	}
	if *removed == nil {
		*removed = make(removedReceivedOwners)
	}
	owners := (*removed)[fam]
	if owners == nil {
		owners = make(map[receivedOwner]struct{})
		(*removed)[fam] = owners
	}
	owners[owner] = struct{}{}
}

// sentReceivedOwner translates egress framing back to the existing ingress
// receipt. Egress PathID, including zero, is never an ingress identifier.
func sentReceivedOwner(fam family.Family, key ribOutKey, entry *ribOutEntry) (receivedOwner, bool) {
	if fam.SAFI == family.SAFIMPLSLabel {
		_, cidr, err := nlrisplit.ExtractLabels([]byte(entry.NativeNLRI), entry.AddPath)
		if err != nil {
			return receivedOwner{}, false
		}
		owner, ok := receivedOwnerKey(fam, cidr, entry.AddPath, entry.SourceMessageID)
		if !ok {
			return receivedOwner{}, false
		}
		key = owner.key
	}
	key.PathID = entry.SourcePath
	return receivedOwner{key: key, message: entry.SourceMessageID, addPath: entry.SourceAddPath}, true
}

// withdrawRemovedSentLocked withdraws only the received owners this operation
// removed. A fresh received revision can precede both forwarding and sent
// projection; absence of an old revision alone is not a purge instruction.
// Caller MUST hold peerMu and MUST dispatch returned writes after unlocking.
func (r *RIBManager) withdrawRemovedSentLocked(source netip.Addr, removed removedReceivedOwners) []sentLifecycleWrite {
	r.fenceSentLifecycleLocked(source)
	var writes []sentLifecycleWrite
	sourcePeer := source.String()
	for destination, destinations := range r.ribOut {
		for fam, owners := range removed {
			routes := destinations[fam]
			for key, entry := range routes {
				if entry.SourcePeer != sourcePeer {
					continue
				}
				owner, ok := sentReceivedOwner(fam, key, &entry)
				if !ok {
					continue
				}
				if _, removed := owners[owner]; !removed {
					continue
				}
				writes = append(writes, sentRemoval(destination, fam, key, &entry))
				delete(routes, key)
				entry.release()
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

// pruneSentSourceLocked removes whole source families, not selected received
// generations. An empty keep list is the explicit release-routes operation.
// Caller MUST hold peerMu and MUST dispatch returned writes after unlocking.
func (r *RIBManager) pruneSentSourceLocked(source netip.Addr, keep []family.Family) []sentLifecycleWrite {
	r.fenceSentLifecycleLocked(source)
	var writes []sentLifecycleWrite
	sourcePeer := source.String()
	for destination, destinations := range r.ribOut {
		for fam, routes := range destinations {
			if slices.Contains(keep, fam) {
				continue
			}
			for key, entry := range routes {
				if entry.SourcePeer != sourcePeer {
					continue
				}
				writes = append(writes, sentRemoval(destination, fam, key, &entry))
				delete(routes, key)
				entry.release()
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

// sentRemoval MUST encode owned command bytes before the caller releases entry.
// It preserves the actual sent receipt for final-writer admission.
func sentRemoval(destination netip.Addr, fam family.Family, key ribOutKey, entry *ribOutEntry) sentLifecycleWrite {
	withdraw := ribOutEntry{NativeNLRI: entry.NativeNLRI, AddPath: entry.AddPath}
	return sentLifecycleWrite{peer: destination.String(),
		command: bgp.FormatWithdrawCommand(reconstructRoute(withdraw, fam, key)), message: entry.MsgID}
}

// attachSentSourceCommunityLocked decorates only matching stale received paths.
// The command deletes no received entry, so it MUST NOT infer withdrawals from
// an asynchronous sent projection. Caller MUST hold peerMu.
func (r *RIBManager) attachSentSourceCommunityLocked(source netip.Addr, fam family.Family, community []byte) {
	received := r.bgpPeers[source]
	if received == nil {
		return
	}
	owners := make(map[receivedOwner]uint8)
	addPath := received.IsAddPath(fam)
	received.IterateFamily(fam, func(raw []byte, entry storage.RouteEntry) bool {
		owner, ok := receivedOwnerKey(fam, raw, addPath, entry.MsgID)
		if ok {
			owners[owner] = entry.StaleLevel
		}
		return true
	})
	sourcePeer := source.String()
	for _, destinations := range r.ribOut {
		routes := destinations[fam]
		for key, entry := range routes {
			if entry.SourcePeer != sourcePeer {
				continue
			}
			owner, ok := sentReceivedOwner(fam, key, &entry)
			if !ok {
				continue
			}
			level, present := owners[owner]
			if !present {
				continue
			}
			if level < storage.DepreferenceThreshold {
				continue
			}
			if !attachSentCommunity(&entry, community) {
				continue
			}
			entry.StaleLevel = level
			routes[key] = entry
		}
	}
}

// attachSentCommunity replaces one owned reference to the deduplicated immutable
// attribute blob. It MUST never modify bytes returned by pool.RibOut.Get.
func attachSentCommunity(entry *ribOutEntry, community []byte) bool {
	wire, err := pool.RibOut.Get(entry.AttrHandle)
	if err != nil {
		return false
	}
	var replacement []byte
	found := false
	for offset := 0; offset < len(wire); {
		if len(wire)-offset < 3 {
			return false
		}
		header := 3
		length := int(wire[offset+2])
		if wire[offset]&0x10 != 0 {
			if len(wire)-offset < 4 {
				return false
			}
			header = 4
			length = int(binary.BigEndian.Uint16(wire[offset+2:]))
		}
		end := offset + header + length
		if end > len(wire) {
			return false
		}
		if wire[offset+1] == byte(attribute.AttrCommunity) {
			value := wire[offset+header : end]
			if containsCommunity(value, community) {
				return true
			}
			if len(value)+len(community) > 65535 {
				return false
			}
			newLength := len(value) + len(community)
			replacement = make([]byte, 0, len(wire)+len(community)+1)
			replacement = append(replacement, wire[:offset]...)
			flags := wire[offset] &^ 0x10
			if newLength > 255 {
				replacement = append(replacement, flags|0x10, byte(attribute.AttrCommunity), byte(newLength>>8), byte(newLength))
			} else {
				replacement = append(replacement, flags, byte(attribute.AttrCommunity), byte(newLength))
			}
			replacement = append(replacement, value...)
			replacement = append(replacement, community...)
			replacement = append(replacement, wire[end:]...)
			found = true
			break
		}
		offset = end
	}
	if !found {
		replacement = make([]byte, 0, len(wire)+3+len(community))
		replacement = append(replacement, wire...)
		replacement = appendAttr(replacement, byte(attribute.AttrCommunity), 0xc0, community)
	}
	handle, err := pool.RibOut.Intern(replacement)
	if err != nil {
		return false
	}
	entry.release()
	entry.AttrHandle = handle
	return true
}

func (r *RIBManager) dispatchSentLifecycle(writes []sentLifecycleWrite) {
	for _, write := range writes {
		r.updateRouteWithMeta(write.peer, write.command, sentLifecycleMeta(write)) //nolint:errcheck // updateRouteWithMeta logs the failure; each lifecycle write stands alone, so the rest still go
	}
}

// sentLifecycleMeta marks a lifecycle write as RIB feedback and carries the
// sent receipt the writer admits it against, as a decimal string.
func sentLifecycleMeta(write sentLifecycleWrite) map[string]any {
	var digits [20]byte
	return map[string]any{"rib-lifecycle": true, metaKeyReplay: true,
		bgptypes.SentOwnerMessageMeta: string(strconv.AppendUint(digits[:0], write.message, 10))}
}

// projectRecoveryWriteLocked is R-12 at the sent projection. A recovery write
// installs a survivor bgp-rib's own lookup elected; when a lifecycle purge
// removed that survivor between the lookup and this projection, the purge
// emitted no removal for this destination, because the entry was not yet
// projected. The entry is then removed here, in the same peerMu section that
// stored it, and the removal the purge would have emitted is queued for the
// recovery-removal dispatcher, never sent on the event path. Caller MUST hold
// peerMu.
func (r *RIBManager) projectRecoveryWriteLocked(destination netip.Addr, fam family.Family, key ribOutKey) {
	entry, stored := r.ribOut[destination][fam][key]
	if !stored {
		return
	}
	gone, provenance := r.recoveryCandidateGone(fam, key, entry)
	if !provenance {
		logger().Warn("recovery write projected without its candidate's provenance (a Ze defect in the recovery write); "+
			"stored without the purge check, so a survivor purged before this write stays at the destination until the next decision for the prefix",
			"destination", destination, "family", fam, "source", entry.SourcePeer)
		return
	}
	if !gone {
		return
	}
	r.removeProjectedLocked(destination, fam, key, entry)
}

// projectForwardedWriteLocked is R-12 for a forwarded write. A lifecycle
// removal (purge-stale, release-routes, restart-time expiry, a community
// purge, retain-routes pruning a family) emits a removal only where the source
// already owns the sent entry. The sent event of a forwarded write of the
// removed path can still be queued behind it, because commands and events
// reach bgp-rib on different goroutines: that entry is projected after the
// removal and would stay at the destination with nothing left to remove it.
// It is removed here when forwardedSourceGone says the path is gone, and the
// removal the lifecycle would have emitted is queued for the R-12 dispatcher.
// The caller MUST call it only for an event lifecycleFollowedLocked reported
// true for. Caller MUST hold peerMu.
func (r *RIBManager) projectForwardedWriteLocked(destination netip.Addr, fam family.Family, key ribOutKey) {
	entry, stored := r.ribOut[destination][fam][key]
	if !stored {
		return
	}
	if !r.forwardedSourceGone(fam, key, entry) {
		return
	}
	r.removeProjectedLocked(destination, fam, key, entry)
}

// removeProjectedLocked deletes one just-projected entry whose source path is
// gone and queues the removal for the R-12 dispatcher, never sending it on the
// event path. Caller MUST hold peerMu.
func (r *RIBManager) removeProjectedLocked(destination netip.Addr, fam family.Family, key ribOutKey, entry ribOutEntry) {
	r.queueRecoveryRemovalLocked(sentRemoval(destination, fam, key, &entry))
	// The family map stays even when empty: storeSentEntries is still
	// writing into it for the rest of the UPDATE.
	delete(r.ribOut[destination][fam], key)
	entry.release()
}

// sourceApplied records how far bgp-rib has applied one source's received
// UPDATEs, which a forwarded write's sent projection needs to tell a path a
// lifecycle removal took from one not applied yet (forwardedSourceGone).
type sourceApplied struct {
	// received is the highest received UPDATE message ID applied.
	received uint64
	// lifecycle is received's value when a lifecycle removal of the source's
	// sent copies last ran: every received message up to it was applied
	// before that removal.
	lifecycle uint64
}

// noteReceivedLocked records that the received UPDATE message of source was
// applied. Message IDs grow in receipt order, so the highest is kept. The
// entry lives as long as peerUp's: one per source address. Caller MUST hold
// peerMu.
func (r *RIBManager) noteReceivedLocked(source netip.Addr, message uint64) {
	applied := r.sourceApplied[source]
	if message <= applied.received {
		return
	}
	applied.received = message
	r.sourceApplied[source] = applied
}

// fenceSentLifecycleLocked marks a lifecycle removal of source's sent copies:
// a write of any received message applied so far can now have lost its path
// to this removal before its sent event was projected. Caller MUST hold
// peerMu.
func (r *RIBManager) fenceSentLifecycleLocked(source netip.Addr) {
	applied, known := r.sourceApplied[source]
	if !known {
		return
	}
	applied.lifecycle = applied.received
	r.sourceApplied[source] = applied
}

// lifecycleFollowedLocked reports whether a lifecycle removal of the source
// ran after bgp-rib applied the received message a forwarded write carries,
// once per stored NLRI section of a sent event (not per NLRI), so a write no
// removal followed (every write outside a GR or LLGR cleanup) pays one parse
// and one lookup per section and no inventory walk. A
// write with no source (a local origination) or no received message answers
// false. Caller MUST hold peerMu.
func (r *RIBManager) lifecycleFollowedLocked(sourcePeer string, message uint64) bool {
	if message == 0 {
		return false
	}
	source, err := netip.ParseAddr(sourcePeer)
	if err != nil {
		return false
	}
	return message <= r.sourceApplied[source].lifecycle
}

// recoveryRemovals is the long-lived bgp-rib lifecycle dispatcher that sends
// the removals projectRecoveryWriteLocked and projectForwardedWriteLocked
// queue (R-12). A synchronous engine RPC on
// the event path would stop bgp-rib applying events while it is in flight, so
// every destination's drain would wait behind it.
//
// Lifecycle: startRecoveryRemovals creates the one-slot wake channel and
// starts the one worker; queueRecoveryRemovalLocked appends under peerMu and
// wakes it, send-or-skip; stopRecoveryRemovals cancels the worker's context,
// which also cancels the send in flight, joins the worker, then drops what is
// still pending. Start and stop are not safe for concurrent use with each
// other: the plugin's run function owns both.
type recoveryRemovals struct {
	wake   chan struct{}
	done   chan struct{}
	cancel context.CancelFunc
	send   func(context.Context, sentLifecycleWrite) error
}

// startRecoveryRemovals starts the dispatcher under a context derived from
// parent. send performs one removal and MUST return when its context ends.
// The caller MUST call stopRecoveryRemovals before it drops the manager.
func (r *RIBManager) startRecoveryRemovals(parent context.Context, send func(context.Context, sentLifecycleWrite) error) {
	ctx, cancel := context.WithCancel(parent)
	d := &recoveryRemovals{wake: make(chan struct{}, 1), done: make(chan struct{}), cancel: cancel, send: send}
	r.peerMu.Lock()
	r.removals = d
	r.peerMu.Unlock()
	d.wake <- struct{}{} // Removals queued before the start go out now.
	go r.runRecoveryRemovals(ctx, d)
}

// stopRecoveryRemovals cancels, then joins, then drops the pending removals.
// It MUST be called after startRecoveryRemovals. Dropping is the fault window
// of a stopped bgp-rib: no recovery owner exists then, and a destination still
// holding a purged path keeps it until the next decision for its key.
func (r *RIBManager) stopRecoveryRemovals() {
	r.peerMu.Lock()
	d := r.removals
	r.removals = nil
	r.peerMu.Unlock()
	if d == nil {
		return
	}
	d.cancel()
	<-d.done
	r.peerMu.Lock()
	r.recoveryPending = nil
	r.peerMu.Unlock()
}

// queueRecoveryRemovalLocked appends one removal and wakes the dispatcher
// without blocking. The pending list is unbounded by design: it holds one
// entry per projected write whose source path was gone at its projection,
// never one per event. A forwarded write is checked only once a lifecycle
// removal of its source followed its received message, and each queued entry
// left ribOut, so the list stays below the source's routes times the
// destinations. Caller MUST hold peerMu.
func (r *RIBManager) queueRecoveryRemovalLocked(write sentLifecycleWrite) {
	r.recoveryPending = append(r.recoveryPending, write)
	if r.removals == nil {
		return
	}
	select {
	case r.removals.wake <- struct{}{}:
	default: // A wake is already pending; the worker takes the whole list.
	}
}

// runRecoveryRemovals is the dispatcher's worker. Its loop has no bound by
// design: it runs until stopRecoveryRemovals cancels ctx.
func (r *RIBManager) runRecoveryRemovals(ctx context.Context, d *recoveryRemovals) {
	defer close(d.done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.wake:
		}
		r.peerMu.Lock()
		writes := r.recoveryPending
		r.recoveryPending = nil
		r.peerMu.Unlock()
		for _, write := range writes {
			if ctx.Err() != nil {
				return
			}
			d.send(ctx, write) //nolint:errcheck // the send logs its failure; each removal stands alone, so the rest still go
		}
	}
}

// sendRecoveryRemoval is the dispatcher's production send: the lifecycle
// write through the engine, under the dispatcher's context.
func (r *RIBManager) sendRecoveryRemoval(ctx context.Context, write sentLifecycleWrite) error {
	return r.updateRouteWithMetaContext(ctx, write.peer, write.command, sentLifecycleMeta(write))
}
