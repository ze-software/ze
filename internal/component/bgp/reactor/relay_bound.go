// Design: docs/architecture/api/process-protocol.md -- joined original destination replay.
// Related: selected_flush.go -- shared synchronous forwarding item consumer.
package reactor

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// boundForwardRequest holds one validated original destination and authenticated
// process lifetime. Its nil value denotes the existing unbound operation.
// It retains no routes or completion history after its synchronous call ends.
type boundForwardRequest struct {
	ctx         context.Context
	sender      plugin.Sender
	owner       plugin.SelectedOwner
	targets     []rpc.ForwardTarget
	resolved    map[*Peer]plugin.Sender // This call's captured target set; no persistent registry.
	targetError error
}

func requireBoundForwardContext(sender plugin.Sender, bound *boundForwardRequest) error {
	owner, _ := sender.Destination()
	if owner == 0 {
		return nil
	}
	if bound == nil {
		return errors.New("bound cached replay requires its context and authenticated process; use forward-cached or relay-stored-route with the original destination tuple")
	}
	return nil
}

func newBoundForwardRequest(ctx context.Context, sender plugin.Sender, owner plugin.SelectedOwner) (*boundForwardRequest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	destination, session := sender.Destination()
	if destination == 0 {
		return nil, errAdjOutSession
	}
	if session == 0 {
		return nil, errAdjOutSession
	}
	if !sender.SameIssuer(owner.Sender()) {
		return nil, errAdjOutProvenance
	}
	if err := owner.Current(); err != nil {
		return nil, err
	}
	return &boundForwardRequest{ctx: ctx, sender: sender, owner: owner}, nil
}
func (a *reactorAPIAdapter) RelayStoredRouteBound(ctx context.Context, destination netip.Addr, routes []rpc.StoredRoute, sender plugin.Sender, owner plugin.SelectedOwner) error {
	request, err := newBoundForwardRequest(ctx, sender, owner)
	if err != nil {
		return err
	}
	return a.relayStoredRoutes(ctx, destination, routes, sender, request)
}
func (a *reactorAPIAdapter) ForwardUpdatesBound(ctx context.Context, ids []uint64, targets []rpc.ForwardTarget, name string, sender plugin.Sender, owner plugin.SelectedOwner) error {
	if !sender.SameIssuer(owner.Sender()) {
		return errAdjOutProvenance
	}
	if name != "" {
		actual, named := sender.Process()
		if !named {
			return errAdjOutProvenance
		}
		if actual != name {
			return errAdjOutProvenance
		}
	}
	defer a.ackBatch(ids, name)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := owner.Current(); err != nil {
		return err
	}
	if len(targets) == 0 {
		return errNoDestinations
	}
	if len(targets) > maxForwardDestinations {
		return errNoDestinations
	}
	if err := rpc.ValidateForwardTargets(targets); err != nil {
		return err
	}
	request := &boundForwardRequest{ctx: ctx, sender: sender, owner: owner, targets: targets, resolved: make(map[*Peer]plugin.Sender, len(targets))}
	// Addresses constrain matching below. Their duplication is not deduplication
	// of owners: two captured peers at one display address stay distinct.
	destinations := make([]netip.AddrPort, 0, len(targets))
	for _, target := range targets {
		address, err := target.Address()
		if err != nil {
			return err
		}
		destinations = append(destinations, netip.AddrPortFrom(address, 0))
	}
	return a.forwardUpdates(ctx, ids, destinations, name, sender, request)
}
func (a *reactorAPIAdapter) relayDestinationPeers(address netip.Addr, sender plugin.Sender, bound *boundForwardRequest) ([]*Peer, error) {
	if bound == nil {
		return a.resolveDestinationPeers([]netip.AddrPort{netip.AddrPortFrom(address, 0)})
	}
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()
	return a.matchBoundDestination(selector.Addr(address), announceOrigin(sender))
}
func (a *reactorAPIAdapter) cachedDestinationPeers(destinations []netip.AddrPort, sender plugin.Sender, bound *boundForwardRequest) ([]*Peer, error) {
	if bound == nil {
		return a.resolveDestinationPeers(destinations)
	}
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()
	peers := make([]*Peer, 0, len(bound.targets))
	for _, target := range bound.targets {
		address, err := target.Address()
		if err != nil {
			bound.targetError = errors.Join(bound.targetError, err)
			continue
		}
		narrowed, err := sender.WithDestination(target.DestinationOwner, target.DestinationSession)
		if err != nil {
			bound.targetError = errors.Join(bound.targetError, err)
			continue
		}
		matching, err := a.matchBoundDestination(selector.Addr(address), announceOrigin(narrowed))
		if err != nil {
			bound.targetError = errors.Join(bound.targetError, err)
			continue
		}
		if len(matching) != 1 {
			bound.targetError = errors.Join(bound.targetError, errAdjOutSession)
			continue
		}
		peer := matching[0]
		bound.resolved[peer] = narrowed
		peers = append(peers, peer)
	}
	if len(peers) == 0 {
		return nil, errors.Join(errNoPeersMatch, bound.targetError)
	}
	return peers, nil
}

// senderFor returns only a captured target constraint. A missing target gives
// an unset sender so the final destination guard refuses it, never a fallback.
func (request *boundForwardRequest) senderFor(peer *Peer) plugin.Sender {
	if request.resolved != nil {
		return request.resolved[peer]
	}
	return request.sender
}

func relayOtherPeers(peers []*Peer, source *Peer) []*Peer {
	found := false
	for _, peer := range peers {
		if peer == source {
			found = true
			break
		}
	}
	if !found {
		return peers
	}
	selected := make([]*Peer, 0, len(peers)-1)
	for _, peer := range peers {
		if peer != source {
			selected = append(selected, peer)
		}
	}
	return selected
}
func (a *reactorAPIAdapter) relaySourceForRequest(ctx context.Context, address netip.Addr, route *rpc.StoredRoute, sender plugin.Sender, bound *boundForwardRequest) (relaySource, error) {
	if bound == nil {
		return a.originalRelaySource(address, route, sender)
	}
	return a.coldBoundRelaySource(ctx, address, route, sender, bound)
}

// boundForwardStage borrows the existing exact process-owned source write until
// flush completes. No callback can acknowledge queued work as final completion.
// It owns only items materialized for this one UPDATE, bounded by its sections.
type boundForwardStage struct {
	request *boundForwardRequest
	source  sourceReceiptWrite
	items   []fwdItem
}

// stage acquires one original-source write guard. On success the caller MUST
// call release exactly once on every exit, including materialization or flush
// failure. Every admitted flush MUST join before the caller releases the stage.
func (request *boundForwardRequest) stage(receipt, epoch uint64) (*boundForwardStage, error) {
	peer, err := sourceReceipts.originalPeer(receipt)
	if err != nil {
		return nil, err
	}
	source := sourceReceipts.recoverySource(receipt, peer.sourceOwner)
	if !source.ok {
		return nil, errRelayOriginalSource
	}
	if forwardSourceCurrent(peer, source.generation) {
		write, err := sourceReceipts.beginLive(receipt, request.owner, epoch)
		if err != nil {
			return nil, err
		}
		return &boundForwardStage{request: request, source: write}, nil
	}
	lease, err := sourceReceipts.ownedLease(receipt, request.owner)
	if err != nil {
		return nil, err
	}
	write, err := lease.begin(epoch)
	if err != nil {
		return nil, err
	}
	return &boundForwardStage{request: request, source: write}, nil
}
func cachedSourceForRequest(update *ReceivedUpdate, bound *boundForwardRequest) (forwardSourceInfo, *boundForwardStage, error) {
	if bound == nil {
		source, err := originalCachedSource(update)
		return source, nil, err
	}
	source, err := originalCachedFacts(update)
	if err != nil {
		return forwardSourceInfo{}, nil, err
	}
	stage, err := bound.stage(source.receipt, update.SourcePolicyEpoch)
	if err != nil {
		return forwardSourceInfo{}, nil, err
	}
	return source.info, stage, nil
}
func forwardOriginalCurrent(update *ReceivedUpdate, source forwardSourceInfo) bool {
	if update.receivedPeer != source.peer {
		return false
	}
	if source.bound != nil {
		return source.bound.source.current() == nil
	}
	return forwardSourceCurrent(update.receivedPeer, update.receivedGeneration)
}
func (stage *boundForwardStage) current(peer *Peer, session *Session) error {
	if err := stage.request.owner.Current(); err != nil {
		return err
	}
	sender := stage.request.senderFor(peer)
	destination, publication := sender.Destination()
	if destination == 0 {
		return errAdjOutSession
	}
	if publication == 0 {
		return errAdjOutSession
	}
	if err := destinationCurrent(peer, session, sender); err != nil {
		return err
	}
	return stage.source.current()
}

// release MUST follow a successful stage exactly once, after every admitted
// flush has joined. The caller MUST release on failure as well as success.
func (stage *boundForwardStage) release() {
	for i := range stage.items {
		item := &stage.items[i]
		item.peer.reactor.fwdPool.releaseItem(item)
		if item.done != nil {
			item.done()
			item.done = nil
		}
	}
	stage.items = nil
	stage.source.complete()
}

// flush MUST follow a successful stage and joins every admitted target before
// returning. The caller MUST wait for it to return before release, and MUST
// release the stage on both success and failure.
func (stage *boundForwardStage) flush() (result error) {
	if len(stage.items) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(stage.request.ctx)
	stop := context.AfterFunc(stage.source.lease.lifetime, cancel)
	stopLive := func() bool { return false }
	if stage.source.live {
		stopLive = context.AfterFunc(stage.source.lease.record.liveLifetime, cancel)
	}
	defer func() { stop(); stopLive(); cancel() }()
	// One carrier per actual target, bounded by the complete admitted target set.
	// All carriers MUST be dispatched before joining any blocked target.
	visited := make(map[*Peer]struct{}, len(stage.items))
	pending := make([]*boundForwardTargetWrite, 0, len(stage.items))
	defer func() {
		// This join also precedes source/cache release during panic unwinding.
		// Cancellation interrupts sockets but MUST NOT bypass admitted ownership.
		for _, write := range pending {
			<-write.done
			result = errors.Join(result, write.err)
		}
	}()
	for i := range stage.items {
		item := stage.items[i]
		if _, done := visited[item.peer]; done {
			continue
		}
		visited[item.peer] = struct{}{}
		write := &boundForwardTargetWrite{stage: stage, ctx: ctx, peer: item.peer, session: item.session, owner: item, done: make(chan struct{}), err: errBoundForwardUnsettled}
		carrier := fwdItem{peer: item.peer, session: item.session, boundWrite: write, done: func() { close(write.done) }}
		pool := item.peer.reactor.fwdPool
		if pool == nil {
			result = errors.Join(result, errBoundForwardUnsettled)
			continue
		}
		if !pool.TryDispatch(fwdKey{peerAddr: item.peer.settings.PeerKey()}, &carrier) {
			result = errors.Join(result, errBoundForwardUnsettled)
			continue
		}
		pending = append(pending, write)
	}
	return result
}

func (stage *boundForwardStage) flushPeer(ctx context.Context, target *Peer, session *Session, owner fwdItem) error {
	if err := stage.current(target, session); err != nil {
		return err
	}
	err := session.sendUpdateCounted(ctx, &message.Update{}, nil, false, &owner, func(s *Session, _ *message.Update) error {
		defer func() {
			s.sentMeta = nil
			s.sentSourcePeerStr = ""
			s.sentSourceMessageID = 0
			s.sentAIGPOrigin = sendOrigin{}
			s.sentAIGPRevision = 0
			s.sentAIGPCostWithheld = false
		}()
		if err := stage.current(target, s); err != nil {
			return err
		}
		for i := range stage.items {
			item := &stage.items[i]
			if item.peer != target {
				continue
			}
			if item.session != s {
				return errAdjOutSession
			}
			if err := s.writeSelectedItem(item); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return stage.current(target, session)
}

var errBoundForwardUnsettled = errors.New("captured forward target did not settle on its bounded worker; reconcile its original stored routes before reporting readiness")

// boundForwardTargetWrite is one current request obligation on an existing
// per-peer worker. The worker MUST call run before the carrier's done callback.
// The request MUST join done before releasing stage.items or its source guard.
// Its initial error makes stop cleanup and panic unwinding fail closed.
type boundForwardTargetWrite struct {
	stage   *boundForwardStage
	ctx     context.Context
	peer    *Peer
	session *Session
	owner   fwdItem
	done    chan struct{}
	err     error
}

func (write *boundForwardTargetWrite) run() {
	write.err = write.stage.flushPeer(write.ctx, write.peer, write.session, write.owner)
}
