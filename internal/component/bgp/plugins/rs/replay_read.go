// Design: docs/architecture/plugin/rib-storage-design.md -- optional RS read failures.
package rs

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const departedSourceReplayRetries = 10

// replayOutputError marks a failed required relay, distinct from its caller's
// optional page read. The zero value is invalid; relayReplayRead produces it
// only for a non-nil output result. It carries no persistent history.
type replayOutputError struct{ cause error }

func (e *replayOutputError) Error() string { return e.cause.Error() }
func (e *replayOutputError) Unwrap() error { return e.cause }

// isDepartedSourceRelayError reports whether a joined relay failure came from
// an original source session going down mid-page rather than from the
// destination peer or transport. Only joined errors (every admitted write
// settled) are eligible for a bounded retry while the source's DOWN reaches
// this plugin.
func isDepartedSourceRelayError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "original cold relay source ended without retained membership") ||
		strings.Contains(msg, "source peer is not an established peer") ||
		strings.Contains(msg, "source peer is not established") ||
		strings.Contains(msg, "stored relay original source is missing or inconsistent")
}

func (rs *routeServer) relayReplayRead(ctx context.Context, destination string, routes []rpc.StoredRoute) error {
	var err error
	for attempt := range departedSourceReplayRetries + 1 {
		err = rs.relayReplayRoutes(ctx, destination, routes)
		if err == nil {
			return nil
		}
		if attempt == departedSourceReplayRetries {
			break
		}
		if !joinedOutputError(err) {
			break
		}
		if !isDepartedSourceRelayError(err) {
			break
		}
		target, ok := ctx.Value(replayTargetKey{}).(rpc.ForwardTarget)
		if !ok {
			break
		}
		if !rs.targetLive(target) {
			break
		}
		timer := time.NewTimer(replayConvergenceDelay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return &replayOutputError{cause: errors.Join(err, ctx.Err())}
		}
	}
	return &replayOutputError{cause: err}
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
