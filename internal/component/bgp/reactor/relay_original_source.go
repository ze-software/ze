// Design: docs/architecture/api/process-protocol.md -- original source facts on every relay.
// Related: source_receipt.go -- Session-wide immutable facts, never per-route eligibility.
package reactor

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errRelayOriginalSource = errors.New("stored relay original source is missing or inconsistent; reconcile its original received snapshot")

// originalRelaySource does not acquire a lease or make a GR decision. Recorded
// rows resolve only their original Session. The all-zero operator variant is
// intentional manual reconstruction and cannot be reached by a partial tuple.
func (a *reactorAPIAdapter) originalRelaySource(address netip.Addr, route *rpc.StoredRoute, sender plugin.Sender) (relaySource, error) {
	source, err := a.originalRelayFacts(address, route, sender)
	if err != nil {
		return relaySource{}, err
	}
	if !forwardSourceCurrent(source.info.peer, source.generation) {
		return relaySource{}, errRelayNoSource
	}
	return source, nil
}

func (a *reactorAPIAdapter) originalRelayFacts(address netip.Addr, route *rpc.StoredRoute, sender plugin.Sender) (relaySource, error) {
	if route.SourceOwner == 0 {
		if route.SourceReceipt != 0 {
			return relaySource{}, errRelayOriginalSource
		}
		if route.SourceConnectionID != 0 {
			return relaySource{}, errRelayOriginalSource
		}
		if !sender.IsOperator() {
			return relaySource{}, errRelayOriginalSource
		}
		return a.resolveRelaySource(address), nil
	}
	if route.SourceReceipt == 0 {
		return relaySource{}, errRelayOriginalSource
	}
	if route.SourceConnectionID == 0 {
		return relaySource{}, errRelayOriginalSource
	}
	source := sourceReceipts.recoverySource(route.SourceReceipt, route.SourceOwner)
	if !source.ok {
		return relaySource{}, errRelayOriginalSource
	}
	if source.addr != address {
		return relaySource{}, errRelayOriginalSource
	}
	if source.info.filterInfo.ConnectionID != route.SourceConnectionID {
		return relaySource{}, errRelayOriginalSource
	}
	if !route.Withdraw {
		if !sourcePolicyCurrent(source.info.peer, route.SourcePolicyEpoch) {
			return relaySource{}, errAdjOutPolicy
		}
	}
	// Live relay requires the original generation. A retained variant supplies
	// its existing process-owned lease separately; receipt knowledge grants none.
	return source, nil
}

// originalCachedSource resolves the immutable producer, never a current Peer
// found by display address. It requires its real cached Peer/generation as well
// as the original receipt; manual/local announcements use their own intake.
func originalCachedSource(update *ReceivedUpdate) (forwardSourceInfo, error) {
	source, err := originalCachedFacts(update)
	if err != nil {
		return forwardSourceInfo{}, err
	}
	if !forwardSourceCurrent(source.info.peer, source.generation) {
		return forwardSourceInfo{}, errForwardNoSource
	}
	return source.info, nil
}

func originalCachedFacts(update *ReceivedUpdate) (relaySource, error) {
	if update == nil {
		return relaySource{}, errForwardNoSource
	}
	if update.receivedPeer == nil {
		return relaySource{}, errForwardNoSource
	}
	if update.SourceOwner != update.receivedPeer.sourceOwner {
		return relaySource{}, errForwardNoSource
	}
	source := sourceReceipts.recoverySource(update.SourceReceipt, update.SourceOwner)
	if !source.ok {
		return relaySource{}, errForwardNoSource
	}
	if source.info.peer != update.receivedPeer {
		return relaySource{}, errForwardNoSource
	}
	if source.addr != update.SourcePeerIP {
		return relaySource{}, errForwardNoSource
	}
	if source.generation != update.receivedGeneration {
		return relaySource{}, errForwardNoSource
	}
	if source.srcID != update.WireUpdate.SourceID() {
		return relaySource{}, errForwardNoSource
	}
	if source.ctxID != fwdContextIDWithASN4(update.WireUpdate.SourceCtxID(), true) {
		return relaySource{}, errForwardNoSource
	}
	if !sourcePolicyCurrent(source.info.peer, update.SourcePolicyEpoch) {
		return relaySource{}, errAdjOutPolicy
	}
	return source, nil
}
