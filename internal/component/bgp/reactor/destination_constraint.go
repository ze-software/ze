// Design: docs/architecture/api/process-protocol.md -- exact replay destination narrowing.
// Related: session_ownership.go -- final write check; send_permission.go -- unchanged authority.
package reactor

import (
	"context"
	"slices"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selector"
)

// matchBoundDestination runs under reactor.mu. Every successful bounded request
// names an established original Session, never a current address replacement.
// Permission remains the ordinary filterPermittedPeers decision.
func (a *reactorAPIAdapter) matchBoundDestination(sel *selector.Selector, origin sendOrigin) ([]*Peer, error) {
	peer, _, err := a.matchCapturedDestination(sel, origin.sender)
	if err != nil {
		return nil, err
	}
	permitted, refused := filterPermittedPeers([]*Peer{peer}, origin)
	if refused != 0 {
		return nil, sendNotPermitted("original destination", origin)
	}
	return permitted, nil
}

// matchCapturedDestination runs under reactor.mu and grants no send authority.
// The caller MUST preserve the returned original Session and apply the actual
// operation's permission before writing to it.
func (a *reactorAPIAdapter) matchCapturedDestination(sel *selector.Selector, sender plugin.Sender) (*Peer, *Session, error) {
	owner, session := sender.Destination()
	for _, peer := range a.r.peers {
		if peer.sourceOwner != owner {
			continue
		}
		var matches bool
		switch sel.SelectorKind() {
		case selector.KindName, selector.KindASN:
			matches = slices.Contains(a.matchPositive(sel), peer)
			if sel.IsExclude() {
				matches = !matches
			}
		case 0, selector.KindAll, selector.KindAddr, selector.KindAddrs, selector.KindGlob:
			matches = sel.Matches(peer.settings.Address)
		default:
			// The selector parser owns this open boundary; an unknown kind selects none.
			return nil, nil, errAdjOutSession
		}
		if !matches {
			return nil, nil, errAdjOutSession
		}
		if peer.State() != PeerStateEstablished {
			return nil, nil, errAdjOutSession
		}
		peer.mu.RLock()
		captured := peer.session
		peer.mu.RUnlock()
		if captured == nil {
			return nil, nil, errAdjOutSession
		}
		if captured.initialReplay.Load() != session {
			return nil, nil, errAdjOutSession
		}
		if captured.tearingDown.Load() {
			return nil, nil, errAdjOutSession
		}
		return peer, captured, nil
	}
	return nil, nil, errAdjOutSession
}

// destinationCurrent validates only the already captured write target. Caller
// MUST pass the Session it will use, never resolve a new one after this check.
// A zero tuple is the explicit legacy unbounded request.
func destinationCurrent(peer *Peer, session *Session, sender plugin.Sender) error {
	owner, replay := sender.Destination()
	if owner == 0 {
		return nil
	}
	if peer.sourceOwner != owner {
		return errAdjOutSession
	}
	if peer.establishedSession.Load() != session {
		return errAdjOutSession
	}
	if peer.State() != PeerStateEstablished {
		return errAdjOutSession
	}
	if session == nil {
		return errAdjOutSession
	}
	if session.initialReplay.Load() != replay {
		return errAdjOutSession
	}
	if session.tearingDown.Load() {
		return errAdjOutSession
	}
	return nil
}

// settlePriorDestinationWrites waits only for this original peer's earlier
// forwards, never another peer. It creates no forwarding authority. Bound
// maintenance then writes synchronously and returns its actual final failure.
func (target announceTarget) settlePriorDestinationWrites(ctx context.Context) error {
	if target.destinationOwner == 0 {
		return nil
	}
	if pool := target.peer.reactor.fwdPool; pool != nil {
		if err := pool.barrierPeer(ctx, target.peer.settings.PeerKey()); err != nil {
			return err
		}
	}
	return ctx.Err()
}
