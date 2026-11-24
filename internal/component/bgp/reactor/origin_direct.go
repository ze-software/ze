// Design: docs/architecture/api/process-protocol.md -- direct-path per-participant route ownership (E-2, AC-41).
// Related: origin_runtime.go -- selected-RIB originReceiptStore exit cleanup and EndOriginOwnerWithPolicy.
// Related: reactor_api_batch.go -- direct announce and withdraw rails.

package reactor

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// directOriginKey identifies one process-originated route slot on one concrete
// direct (no-bgp-rib) peer. Including peerOwner distinguishes a replaced peer
// at the same IP address.
type directOriginKey struct {
	peer      *Peer
	peerOwner uint64
	family    family.Family
	path      fwdPathKey
	pathID    uint32
	hasPathID bool
}

// directOriginContribution holds one participant's saved announce specification
// for a direct-path route slot. Immutable once recorded except for retained,
// which is mutated only under directOriginTable.mu when on-exit keep detaches
// an exited incarnation.
type directOriginContribution struct {
	owner      plugin.SelectedOwner
	senderName string
	seq        uint64
	retained   bool
	nlriItem   nlri.NLRI
	build      *originBuildValue
}

func (c *directOriginContribution) announceBatch() bgptypes.NLRIBatch {
	batch := bgptypes.NLRIBatch{
		Family:   c.build.family,
		NLRIs:    []nlri.NLRI{c.nlriItem},
		NextHop:  c.build.nextHop,
		OriginAS: c.build.originAS,
	}
	if c.build.wire != nil {
		batch.Wire = c.build.wire
	}
	return batch
}

// directOriginAction describes one post-lock wire operation produced by
// removing an exited participant's direct-path contributions: either
// re-announce a surviving participant's contribution, or withdraw the NLRI
// (honoring restore-configured) when no survivor remains.
type directOriginAction struct {
	peer     *Peer
	family   family.Family
	nlriItem nlri.NLRI
	survivor *directOriginContribution
	keyBytes string
}

// directOriginTable tracks process-originated routes written directly to peers
// that bypass originReceiptStore / bgp-rib OriginIntake. Bounded by active
// peers and originated prefixes per participant; entries are removed on
// explicit withdraw, participant exit (when keep is false), peer removal, and
// reactor stop.
//
// Zero value is ready for use. mu is a leaf lock: callers MUST NOT hold it
// across peer locks, socket I/O, or reactor calls.
// Safe for concurrent use.
type directOriginTable struct {
	mu      sync.Mutex
	seq     uint64
	entries map[directOriginKey][]*directOriginContribution
	byOwner map[plugin.SelectedOwner]map[directOriginKey]struct{}
	byPeer  map[*Peer]map[directOriginKey]struct{}
}

func directKeyFor(peer *Peer, identity *originIdentity) directOriginKey {
	return directOriginKey{
		peer:      peer,
		peerOwner: peer.sourceOwner,
		family:    identity.family,
		path:      identity.key,
		pathID:    identity.pathID,
		hasPathID: identity.hasPathID,
	}
}

func (t *directOriginTable) record(owner plugin.SelectedOwner, peer *Peer, nlriItem nlri.NLRI, build *originBuildValue) {
	if peer == nil {
		return
	}
	if build == nil {
		return
	}
	if build.originIdentity == nil {
		return
	}
	if owner.Done() == nil {
		return
	}
	senderName, _ := owner.Sender().Process()
	key := directKeyFor(peer, build.originIdentity)

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.entries == nil {
		t.entries = make(map[directOriginKey][]*directOriginContribution)
		t.byOwner = make(map[plugin.SelectedOwner]map[directOriginKey]struct{})
		t.byPeer = make(map[*Peer]map[directOriginKey]struct{})
	}
	t.seq++
	contrib := &directOriginContribution{
		owner:      owner,
		senderName: senderName,
		seq:        t.seq,
		nlriItem:   nlriItem,
		build:      build,
	}

	list := t.entries[key]
	replaced := false
	for i, existing := range list {
		if existing.owner.SameIncarnation(owner) {
			list[i] = contrib
			replaced = true
			break
		}
		if existing.retained && senderName != "" && existing.senderName == senderName {
			// A restarted same-name successor replaces its predecessor's kept route.
			list[i] = contrib
			replaced = true
			break
		}
	}
	if !replaced {
		list = append(list, contrib)
	}
	t.entries[key] = list

	ownerKeys := t.byOwner[owner]
	if ownerKeys == nil {
		ownerKeys = make(map[directOriginKey]struct{})
		t.byOwner[owner] = ownerKeys
	}
	ownerKeys[key] = struct{}{}

	peerKeys := t.byPeer[peer]
	if peerKeys == nil {
		peerKeys = make(map[directOriginKey]struct{})
		t.byPeer[peer] = peerKeys
	}
	peerKeys[key] = struct{}{}
}

func pickDirectSurvivor(list []*directOriginContribution) *directOriginContribution {
	var best *directOriginContribution
	for _, c := range list {
		if best == nil || c.seq > best.seq {
			best = c
		}
	}
	return best
}

func (t *directOriginTable) deleteKeyLocked(key directOriginKey) {
	delete(t.entries, key)
	if peerKeys := t.byPeer[key.peer]; peerKeys != nil {
		delete(peerKeys, key)
		if len(peerKeys) == 0 {
			delete(t.byPeer, key.peer)
		}
	}
}

func (t *directOriginTable) removeOwnerKeyLocked(owner plugin.SelectedOwner, key directOriginKey) {
	if ownerKeys := t.byOwner[owner]; ownerKeys != nil {
		delete(ownerKeys, key)
		if len(ownerKeys) == 0 {
			delete(t.byOwner, owner)
		}
	}
}

// removeForWithdraw removes owner's contribution (and any retained contribution
// from the same sender name) for (peer, identity). If another contributor still
// holds an entry for that key, returns that deterministic survivor so the caller
// re-announces it instead of withdrawing the route from the peer.
func (t *directOriginTable) removeForWithdraw(owner plugin.SelectedOwner, peer *Peer, identity *originIdentity) (*directOriginContribution, bool) {
	if peer == nil {
		return nil, false
	}
	if identity == nil {
		return nil, false
	}
	senderName, _ := owner.Sender().Process()
	key := directKeyFor(peer, identity)

	t.mu.Lock()
	defer t.mu.Unlock()

	list := t.entries[key]
	if len(list) == 0 {
		return nil, false
	}
	kept := list[:0]
	removed := false
	for _, c := range list {
		matchIncarnation := owner.Done() != nil && c.owner.SameIncarnation(owner)
		matchRetained := c.retained && senderName != "" && c.senderName == senderName
		if matchIncarnation {
			removed = true
			if !c.retained {
				t.removeOwnerKeyLocked(c.owner, key)
			}
			continue
		}
		if matchRetained {
			removed = true
			continue
		}
		kept = append(kept, c)
	}
	if !removed {
		return nil, false
	}
	if len(kept) == 0 {
		t.deleteKeyLocked(key)
		return nil, true
	}
	t.entries[key] = kept
	return pickDirectSurvivor(kept), true
}

// clearRetainedForWithdraw removes retained (on-exit keep) contributions for
// (peer, identity) when an operator explicitly withdraws the route so a later
// unrelated participant exit cannot resurrect it.
func (t *directOriginTable) clearRetainedForWithdraw(peer *Peer, identity *originIdentity) {
	if peer == nil {
		return
	}
	if identity == nil {
		return
	}
	key := directKeyFor(peer, identity)

	t.mu.Lock()
	defer t.mu.Unlock()

	list := t.entries[key]
	if len(list) == 0 {
		return
	}
	kept := list[:0]
	for _, c := range list {
		if c.retained {
			continue
		}
		kept = append(kept, c)
	}
	if len(kept) == 0 {
		t.deleteKeyLocked(key)
		return
	}
	t.entries[key] = kept
}

// endOwner reconciles direct-path contributions owned by an exited process
// incarnation. When keep is true, entries are detached as retained so routes
// remain advertised and survive unrelated exits. When keep is false, entries
// are removed and deterministic withdraw or survivor re-announce actions are
// returned for execution outside t.mu.
func (t *directOriginTable) endOwner(owner plugin.SelectedOwner, keep bool) []directOriginAction {
	if owner.Done() == nil {
		return nil
	}

	t.mu.Lock()
	ownerKeys := t.byOwner[owner]
	if len(ownerKeys) == 0 {
		t.mu.Unlock()
		return nil
	}
	delete(t.byOwner, owner)

	if keep {
		for key := range ownerKeys {
			for _, c := range t.entries[key] {
				if c.owner.SameIncarnation(owner) {
					c.retained = true
				}
			}
		}
		t.mu.Unlock()
		return nil
	}

	actions := make([]directOriginAction, 0, len(ownerKeys))
	for key := range ownerKeys {
		list := t.entries[key]
		if len(list) == 0 {
			continue
		}
		kept := list[:0]
		var removedItem nlri.NLRI
		var removedKeyBytes string
		for _, c := range list {
			if c.owner.SameIncarnation(owner) {
				removedItem = c.nlriItem
				removedKeyBytes = string(c.build.native)
				continue
			}
			kept = append(kept, c)
		}
		if removedItem == nil {
			continue
		}
		if len(kept) == 0 {
			t.deleteKeyLocked(key)
			actions = append(actions, directOriginAction{
				peer:     key.peer,
				family:   key.family,
				nlriItem: removedItem,
				keyBytes: removedKeyBytes,
			})
			continue
		}
		t.entries[key] = kept
		survivor := pickDirectSurvivor(kept)
		actions = append(actions, directOriginAction{
			peer:     key.peer,
			family:   key.family,
			survivor: survivor,
			keyBytes: string(survivor.build.native),
		})
	}
	t.mu.Unlock()

	slices.SortFunc(actions, func(a, b directOriginAction) int {
		if c := cmp.Compare(a.peer.addrString, b.peer.addrString); c != 0 {
			return c
		}
		if c := cmp.Compare(a.peer.sourceOwner, b.peer.sourceOwner); c != 0 {
			return c
		}
		if c := cmp.Compare(a.family.AFI, b.family.AFI); c != 0 {
			return c
		}
		if c := cmp.Compare(a.family.SAFI, b.family.SAFI); c != 0 {
			return c
		}
		return cmp.Compare(a.keyBytes, b.keyBytes)
	})
	return actions
}

// removePeer drops all direct-path entries belonging to peer on peer teardown.
func (t *directOriginTable) removePeer(peer *Peer) {
	if peer == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	peerKeys := t.byPeer[peer]
	if len(peerKeys) == 0 {
		return
	}
	delete(t.byPeer, peer)
	for key := range peerKeys {
		for _, c := range t.entries[key] {
			if !c.retained {
				t.removeOwnerKeyLocked(c.owner, key)
			}
		}
		delete(t.entries, key)
	}
}

// clear drops every direct-path entry on reactor shutdown.
func (t *directOriginTable) clear() {
	t.mu.Lock()
	clear(t.entries)
	clear(t.byOwner)
	clear(t.byPeer)
	t.mu.Unlock()
}

func directOriginOwner(ctx context.Context, batch bgptypes.NLRIBatch, sender plugin.Sender) (plugin.SelectedOwner, bool) {
	if sender.IsOperator() {
		return plugin.SelectedOwner{}, false
	}
	if dest, _ := sender.Destination(); dest != 0 {
		return plugin.SelectedOwner{}, false
	}
	if !originBatchEligible(batch) {
		return plugin.SelectedOwner{}, false
	}
	if batch.OriginAdmissionID != 0 {
		return plugin.SelectedOwner{}, false
	}
	owner := batch.OriginOwner
	if !owner.Sender().IsSet() {
		var err error
		owner, err = plugin.OriginProducerFromContext(ctx)
		if err != nil {
			return plugin.SelectedOwner{}, false
		}
	}
	if !owner.Sender().SameIssuer(sender) {
		return plugin.SelectedOwner{}, false
	}
	if owner.Done() == nil {
		return plugin.SelectedOwner{}, false
	}
	if owner.Current() != nil {
		return plugin.SelectedOwner{}, false
	}
	return owner, true
}

func directOriginOwnerForWithdraw(ctx context.Context, batch bgptypes.NLRIBatch, sender plugin.Sender) (plugin.SelectedOwner, bool) {
	if sender.IsOperator() {
		return plugin.SelectedOwner{}, false
	}
	if dest, _ := sender.Destination(); dest != 0 {
		return plugin.SelectedOwner{}, false
	}
	if batch.OriginAdmissionID != 0 {
		return plugin.SelectedOwner{}, false
	}
	if batch.SentOwnerMessage != 0 {
		return plugin.SelectedOwner{}, false
	}
	if batch.InitialReplay != 0 {
		return plugin.SelectedOwner{}, false
	}
	if batch.SourcePolicyEpoch != 0 {
		return plugin.SelectedOwner{}, false
	}
	if batch.RecoverySource.IsValid() {
		return plugin.SelectedOwner{}, false
	}
	if batch.Stale != 0 {
		return plugin.SelectedOwner{}, false
	}
	if batch.Replay {
		return plugin.SelectedOwner{}, false
	}
	owner := batch.OriginOwner
	if !owner.Sender().IsSet() {
		var err error
		owner, err = plugin.OriginProducerFromContext(ctx)
		if err != nil {
			return plugin.SelectedOwner{}, false
		}
	}
	if !owner.Sender().SameIssuer(sender) {
		return plugin.SelectedOwner{}, false
	}
	if owner.Done() == nil {
		return plugin.SelectedOwner{}, false
	}
	if owner.Current() != nil && !originRetirementOwner(ctx, owner) {
		return plugin.SelectedOwner{}, false
	}
	return owner, true
}

func captureDirectOriginBuilds(batch bgptypes.NLRIBatch) []*originBuildValue {
	builds := make([]*originBuildValue, len(batch.NLRIs))
	for i, n := range batch.NLRIs {
		build, err := captureOriginBuild(batch, n)
		if err != nil {
			continue
		}
		builds[i] = build
	}
	return builds
}

func (a *reactorAPIAdapter) recordDirectOrigins(owner plugin.SelectedOwner, peer *Peer, nlris []nlri.NLRI, builds []*originBuildValue) {
	if len(builds) == 0 {
		return
	}
	for i, n := range nlris {
		if i >= len(builds) {
			continue
		}
		if builds[i] == nil {
			continue
		}
		a.r.directOrigins.record(owner, peer, n, builds[i])
	}
}

func (a *reactorAPIAdapter) endDirectOriginOwner(ctx context.Context, owner plugin.SelectedOwner, keep bool) error {
	if owner.Done() == nil {
		return nil
	}
	actions := a.r.directOrigins.endOwner(owner, keep)
	if len(actions) == 0 {
		return nil
	}
	var failures []error
	for _, act := range actions {
		if act.peer == nil {
			continue
		}
		if act.peer.stopping.Load() {
			continue
		}
		if act.survivor != nil {
			if err := a.reannounceDirectOrigin(ctx, act.peer, act.survivor); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		if err := a.withdrawDirectOrigin(ctx, act.peer, act.family, act.nlriItem); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (a *reactorAPIAdapter) reannounceDirectOrigin(ctx context.Context, peer *Peer, survivor *directOriginContribution) error {
	batch := survivor.announceBatch()
	batch.DestinationOwner = peer.sourceOwner
	var attrs []attribute.Attribute
	var userASPath *attribute.ASPath
	if batch.Wire != nil {
		var err error
		attrs, err = batch.Wire.All()
		if err != nil {
			return fmt.Errorf("direct origin survivor attributes: %w", err)
		}
		if asPathAttr, err := batch.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				userASPath = asp
			}
		}
	} else {
		attrs = append(attrs, attribute.OriginIGP)
	}
	var output rpc.UpdateRouteOutput
	err := a.writeDirectAnnounceNLRIBatch(ctx, []*Peer{peer}, batch, plugin.OperatorSender(), attrs, userASPath, nil, 0, PeerState(0), &output, plugin.SelectedOwner{}, nil)
	if errors.Is(err, route.ErrNoPeersAcceptedFamily) {
		return nil
	}
	return err
}

func (a *reactorAPIAdapter) withdrawDirectOrigin(ctx context.Context, peer *Peer, fam family.Family, item nlri.NLRI) error {
	batch := bgptypes.NLRIBatch{
		Family:            fam,
		NLRIs:             []nlri.NLRI{item},
		DestinationOwner:  peer.sourceOwner,
		RestoreConfigured: true,
	}
	var output rpc.UpdateRouteOutput
	err := a.writeWithdrawNLRIBatch(ctx, []*Peer{peer}, batch, plugin.OperatorSender(), &output)
	if errors.Is(err, route.ErrWithdrawWithheld) {
		return nil
	}
	if errors.Is(err, route.ErrNoPeersAcceptedFamily) {
		return nil
	}
	return err
}

// withdrawDirectWithOwnership updates directOrigins for direct-path withdrawals.
// If another participant still owns a route on a peer, that survivor is
// re-announced instead of withdrawing the route from the peer.
func (a *reactorAPIAdapter) withdrawDirectWithOwnership(ctx context.Context, peers []*Peer, batch bgptypes.NLRIBatch, sender plugin.Sender, output *rpc.UpdateRouteOutput) error {
	if sender.IsOperator() {
		for _, n := range batch.NLRIs {
			if n == nil {
				continue
			}
			identity, err := newOriginIdentity(batch.Family, n.Bytes(), n.PathID(), nlri.HasPathID(n))
			if err != nil {
				continue
			}
			for _, peer := range peers {
				a.r.directOrigins.clearRetainedForWithdraw(peer, identity)
			}
		}
		return a.writeWithdrawNLRIBatch(ctx, peers, batch, sender, output)
	}

	owner, tracked := directOriginOwnerForWithdraw(ctx, batch, sender)
	if !tracked {
		return a.writeWithdrawNLRIBatch(ctx, peers, batch, sender, output)
	}

	type peerWithdrawPlan struct {
		peer       *Peer
		toWithdraw []nlri.NLRI
		survivors  []*directOriginContribution
	}

	anySurvivor := false
	plans := make([]peerWithdrawPlan, len(peers))
	for pi, peer := range peers {
		plans[pi].peer = peer
		for _, n := range batch.NLRIs {
			if n == nil {
				plans[pi].toWithdraw = append(plans[pi].toWithdraw, n)
				continue
			}
			identity, err := newOriginIdentity(batch.Family, n.Bytes(), n.PathID(), nlri.HasPathID(n))
			if err != nil {
				plans[pi].toWithdraw = append(plans[pi].toWithdraw, n)
				continue
			}
			survivor, _ := a.r.directOrigins.removeForWithdraw(owner, peer, identity)
			if survivor != nil {
				anySurvivor = true
				plans[pi].survivors = append(plans[pi].survivors, survivor)
				continue
			}
			plans[pi].toWithdraw = append(plans[pi].toWithdraw, n)
		}
	}

	if !anySurvivor {
		return a.writeWithdrawNLRIBatch(ctx, peers, batch, sender, output)
	}

	var failures []error
	for _, plan := range plans {
		for _, survivor := range plan.survivors {
			if err := a.reannounceDirectOrigin(ctx, plan.peer, survivor); err != nil {
				failures = append(failures, err)
			}
		}
		if len(plan.toWithdraw) > 0 {
			peerBatch := batch
			peerBatch.NLRIs = plan.toWithdraw
			var subOut rpc.UpdateRouteOutput
			if err := a.writeWithdrawNLRIBatch(ctx, []*Peer{plan.peer}, peerBatch, sender, &subOut); err != nil {
				failures = append(failures, err)
			}
			output.Warnings = append(output.Warnings, subOut.Warnings...)
		}
	}
	if err := errors.Join(failures...); err != nil {
		return err
	}
	output.Withdrawn = uint32(len(batch.NLRIs)) //nolint:gosec // bounded parsed native batch
	return nil
}
