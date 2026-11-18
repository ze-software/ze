// Design: docs/architecture/plugin/rib-storage-design.md -- finite replay reconciliation.
// Related: rr.go -- optional reads and required output have distinct outcomes.
package rr

import (
	"context"
	"errors"
)

// storedAdmissionError proves this complete page failed before its one output
// request was dispatched. Its current eligibility may safely be read again;
// it never proves an attempted writer joined. The zero value is invalid.
type storedAdmissionError struct{ cause error }

func (e *storedAdmissionError) Error() string { return e.cause.Error() }
func (e *storedAdmissionError) Unwrap() error { return e.cause }

// peerReplayDebt belongs to one current actor, not process error history.
// Successful pages have joined; a later reconciliation reads the current
// snapshot again. An actual failed output is separately retained by workErr
// and MUST be reconciled before this safe read retry can run.
type peerReplayDebt struct {
	target        peerTarget
	initialReplay uint64
	cause         error
}

func (rr *routeReflector) rememberReplay(target peerTarget, initialReplay uint64, cause error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	state := rr.peers[target.identity]
	if state == nil {
		return
	}
	if !state.Up {
		return
	}
	if state.DestinationSession != target.session {
		return
	}
	if state.ReplayGen != target.generation {
		return
	}
	if cause == nil {
		state.replayDebt = nil
		return
	}
	state.replayDebt = &peerReplayDebt{target: target, initialReplay: initialReplay, cause: cause}
}

// retryPeerReplays is called after joining the already admitted lifecycles.
// It serializes settlement callers, uses their finite original deadline and
// starts no background loop. Each live actor gets at most one retry per call.
func (rr *routeReflector) retryPeerReplays(ctx context.Context) error {
	for {
		rr.replayRetryMu.Lock()
		active := rr.replayRetryIdle
		if active == nil {
			rr.replayRetryIdle = make(chan struct{})
			rr.replayRetryMu.Unlock()
			defer func() {
				rr.replayRetryMu.Lock()
				close(rr.replayRetryIdle)
				rr.replayRetryIdle = nil
				rr.replayRetryMu.Unlock()
			}()
			break
		}
		rr.replayRetryMu.Unlock()
		select {
		case <-active:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := rr.unsettledWork(); err != nil {
		return err
	}
	rr.mu.RLock()
	pending := make([]*peerReplayDebt, 0)
	for _, state := range rr.peers {
		if state.Up {
			if state.replayDebt != nil {
				pending = append(pending, state.replayDebt)
			}
		}
	}
	rr.mu.RUnlock()
	for _, debt := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !rr.targetCurrent(debt.target) {
			continue
		}
		done := make(chan struct{})
		callCtx, admitted := rr.replays.begin(ctx, done)
		if !admitted {
			return errors.New("bgp-rr: replay stopped before current eligibility could be reconciled")
		}
		rr.replayForPeer(callCtx, debt.target, debt.initialReplay, done)
		if err := rr.unsettledWork(); err != nil {
			return err
		}
		rr.mu.RLock()
		state := rr.peers[debt.target.identity]
		var failed error
		if state != nil {
			if state.replayDebt != nil {
				failed = state.replayDebt.cause
			}
		}
		rr.mu.RUnlock()
		if failed != nil {
			return failed
		}
	}
	return nil
}

// wakeCurrentReplay uses only the existing fixed validation worker. With no
// outstanding current replay it enqueues nothing; no read runs on event input.
func (rr *routeReflector) wakeCurrentReplay() {
	if rr.validation == nil {
		return
	}
	rr.mu.RLock()
	pending := false
	for _, state := range rr.peers {
		if state.Up {
			if state.replayDebt != nil {
				pending = true
				break
			}
		}
	}
	rr.mu.RUnlock()
	if pending {
		rr.validation.signal()
	}
}

// pinPeerReplay MUST precede replay I/O and MUST pair with finishPeerReplay.
// The current tuple check and actor pin are atomic with normal DOWN retirement.
func (rr *routeReflector) pinPeerReplay(target peerTarget) (*peerState, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	state := rr.peers[target.identity]
	if state == nil {
		return nil, false
	}
	if !state.Up {
		return nil, false
	}
	if state.DestinationSession != target.session {
		return nil, false
	}
	if state.ReplayGen != target.generation {
		return nil, false
	}
	state.replayCalls++
	return state, true
}

// finishPeerReplay MUST follow each successful pin exactly once, after final
// marker/readiness completion has been recorded. Its empty existing-worker
// prune keeps no completed actor history; source retention remains separate.
func (rr *routeReflector) finishPeerReplay(source peerIdentity, state *peerState) {
	rr.mu.Lock()
	state.replayCalls--
	retire := false
	if rr.peers[source] == state {
		if !state.Up {
			if !state.Leased {
				retire = state.replayCalls == 0
			}
		}
	}
	rr.mu.Unlock()
	if retire {
		rr.oweCleanup(source)
		if !rr.enqueueDeparture(source, make(map[withdrawalKey]withdrawalInfo)) {
			rr.noteWorkFailure("finished original replay cleanup", errors.New("departure worker stopped before actor retirement"))
		}
	}
}
