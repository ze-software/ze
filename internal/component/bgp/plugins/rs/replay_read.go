// Design: docs/architecture/plugin/rib-storage-design.md -- optional RS read failures.
package rs

import (
	"context"
	"errors"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// replayOutputError marks a failed required relay, distinct from its caller's
// optional page read. The zero value is invalid; relayReplayRead produces it
// only for a non-nil output result. It carries no persistent history.
type replayOutputError struct{ cause error }

func (e *replayOutputError) Error() string { return e.cause.Error() }
func (e *replayOutputError) Unwrap() error { return e.cause }

func (rs *routeServer) relayReplayRead(ctx context.Context, destination string, routes []rpc.StoredRoute) error {
	if err := rs.relayReplayRoutes(ctx, destination, routes); err != nil {
		return &replayOutputError{cause: err}
	}
	return nil
}

func (rs *routeServer) finishOptionalReplayRead(parent context.Context, key workerKey, generation, cut, initial uint64, cause error) {
	var output *replayOutputError
	if errors.As(cause, &output) {
		rs.failReplay(key, generation, cause)
		return
	}
	absent := isDispatchUnknownCommand(cause)
	if absent {
		rs.adjRibInMissingOnce.Do(func() {
			rs.warnLifecycle("bgp-adj-rib-in not loaded; peer-up replay limited to authorized FlowSpec routes", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix)
		})
	} else {
		rs.warnLifecycle("optional RS receive-store read remains owed; current reconciliation will retry it", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix, "source-owner", key.owner, "connection-id", key.connectionID, "error", cause)
	}
	if err := rs.replayFlowSpecs(parent, key, generation, cut); err != nil {
		rs.failReplay(key, generation, err)
		return
	}
	rs.finishReplayOutput(parent, key, generation, initial)
	if absent {
		return
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	actor := rs.peers[key]
	if actor == nil {
		return
	}
	if actor.ReplayGen != generation {
		return
	}
	if actor.replayFailure != nil {
		return
	}
	actor.replayFailure = cause
}
