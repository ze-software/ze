// Design: docs/architecture/plugin/rib-storage-design.md -- original cold relay catch-up.
// Related: receive_publication.go -- publication alone is not applied state.
package reactor

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// coldBoundRelaySource runs after exact original facts and before dispatch or
// destination locks. Live ForwardCached never reaches it. The original physical
// generation is rechecked before waiting, so an ended retained source cannot
// borrow the same Peer's successor publication or processing position.
func (a *reactorAPIAdapter) coldBoundRelaySource(ctx context.Context, address netip.Addr, route *rpc.StoredRoute, sender plugin.Sender, bound *boundForwardRequest) (relaySource, error) {
	source, err := a.originalRelayFacts(address, route, sender)
	if err != nil {
		return relaySource{}, err
	}
	if source.info.peer == nil {
		return source, nil
	} // Explicit operator-local input.
	peer := source.info.peer
	if !forwardSourceCurrent(peer, source.generation) {
		return a.retainedColdRelaySource(&source, bound)
	}
	cut := peer.receiveCut()
	if !forwardSourceCurrent(peer, source.generation) {
		return a.retainedColdRelaySource(&source, bound)
	}
	if err := cut.wait(ctx); err != nil {
		return relaySource{}, err
	}
	current, err := a.originalRelayFacts(address, route, sender)
	if err != nil {
		return relaySource{}, err
	}
	if current.info.peer != peer || current.generation != source.generation {
		return relaySource{}, errRelayOriginalSource
	}
	if !forwardSourceCurrent(peer, source.generation) {
		return a.retainedColdRelaySource(&source, bound)
	}
	if cut.state(peer) != receiveCutCurrent {
		return relaySource{}, errReceivePublicationStopped
	}
	fam, ok := family.LookupFamily(route.Family)
	if !ok {
		return relaySource{}, errRelayFamily
	}
	if !ribevents.IsFlowSpec(fam) && !ribevents.ValidationEnabled() {
		return current, nil
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return relaySource{}, errors.New("cold stored relay has no finite request deadline; preserve its original bounded replay context")
	}
	// The stored row's own original message is a minimum too. A missing
	// publication object is never proof that a mandatory provider caught up.
	floor := max(cut.messageID, route.MsgID)
	if floor == 0 {
		return relaySource{}, errors.New("cold stored relay has no original received message position; preserve its received message identity")
	}
	key := ribevents.ValidationRoute{Peer: address, PeerOwner: route.SourceOwner, ConnectionID: route.SourceConnectionID, Family: fam}
	clk := a.r.clock
	if clk == nil {
		clk = clock.RealClock{}
	}
	for {
		if err := ctx.Err(); err != nil {
			return relaySource{}, err
		}
		if !forwardSourceCurrent(peer, source.generation) {
			return a.retainedColdRelaySource(&source, bound)
		}
		pending := false
		if ribevents.IsFlowSpec(fam) {
			answer, err := ribevents.LookupFlowSpecRequest(ctx, ribevents.FlowSpecRequest{Queries: []ribevents.FlowSpecQuery{{Operation: ribevents.FlowSpecPresent, Route: key, AfterMsgID: floor}}})
			if err != nil {
				return relaySource{}, err
			}
			pending = answer.Pending
		} else if ribevents.ValidationEnabled() {
			answer, err := ribevents.LookupEligibilityRequest(ctx, ribevents.EligibilityRequest{Queries: []ribevents.EligibilityQuery{{Route: key, AfterMsgID: floor, Present: true}}})
			if err != nil {
				return relaySource{}, err
			}
			pending = answer.Pending
		}
		if !pending {
			return current, nil
		}
		// Known lag is a finite read-only retry, never route absence or a sticky
		// application failure. The ordinary validator still decides actual NLRIs.
		timer := clk.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return relaySource{}, ctx.Err()
		case <-timer.C():
		}
	}
}

func (a *reactorAPIAdapter) retainedColdRelaySource(source *relaySource, bound *boundForwardRequest) (relaySource, error) {
	if bound == nil {
		return relaySource{}, errRelayNoSource
	}
	lease, err := sourceReceipts.ownedLease(source.receipt, bound.owner)
	if err != nil {
		return relaySource{}, err
	}
	lease.store.mu.Lock()
	retained := lease.retained
	current := lease.current()
	lease.store.mu.Unlock()
	if current != nil {
		return relaySource{}, current
	}
	if !retained {
		return relaySource{}, errors.New("original cold relay source ended without retained membership; use its existing retained lease before replay")
	}
	return *source, nil
}
