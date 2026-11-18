// Design: docs/architecture/traffic/cos-dynamic.md -- original failed-owner recovery
// Related: shared_access.go -- failed departures retain their original owner

//go:build ze_l2tp

package cos

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func executeCoSCleanup(args []string, _ string) (string, any, error) {
	if len(args) != 0 {
		return sharedCoSCleanupFailure(sharedCoSCleanupResult{}, errors.New("class-of-service cleanup does not accept arguments"))
	}
	if dynamicHandler == nil {
		return sharedCoSCleanupFailure(sharedCoSCleanupResult{}, errors.New("the original class-of-service cleanup owner is not running"))
	}
	h := dynamicHandler
	if !h.mu.TryLock() {
		return sharedCoSCleanupFailure(sharedCoSCleanupResult{}, errors.New("the original class-of-service owner is processing another operation"))
	}
	defer h.mu.Unlock()
	if h.stopped {
		return sharedCoSCleanupFailure(sharedCoSCleanupResult{}, errors.New("the original class-of-service runner is stopped"))
	}
	result, err := h.retrySharedCleanup()
	if err != nil {
		return sharedCoSCleanupFailure(result, err)
	}
	return rpc.StatusDone, result, nil
}

func registerCleanupCommand(handlers map[string]func([]string, string) (string, any, error)) {
	handlers[cleanupCommand] = executeCoSCleanup
}

func sharedCoSCleanupFailure(result sharedCoSCleanupResult, cause error) (string, any, error) {
	fix := cleanupFix
	var unavailable *originalCoSTargetUnavailable
	if errors.As(cause, &unavailable) {
		fix = originalCoSTargetFix
	}
	refusal := opererr.Wrap(cause, fmt.Sprintf("Class-of-service cleanup did not finish: attempted=%d recovered=%d", result.Attempted, result.Recovered),
		"Original owners retain unfinished native cleanup or warning-clear publication; confirmed native work is not repeated",
		fix)
	response := plugin.OperatorErrorResponse(refusal)
	return response.Status, response, refusal
}

// retrySharedCleanup visits each retained failed access once. Healthy users,
// live transactions and their compensatable tokens are not cleanup work.
// Caller MUST hold h.mu. No second Down or new baseline is constructed.
func (h *cosHandler) retrySharedCleanup() (sharedCoSCleanupResult, error) {
	var result sharedCoSCleanupResult
	var failures []error
	names := make([]string, 0, len(h.accesses))
	for name := range h.accesses {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		state := h.accesses[name]
		access := state.sharedOwner()
		switch state.(type) {
		case *sharedCoSCapturePending, *sharedCoSReleasePending:
			result.Attempted++
			if err := h.finishRetainedSharedAccess(state); err != nil {
				failures = append(failures, h.cleanupFailure(access, sessionDiagnostic{}, err))
				continue
			}
			if err := h.clearSharedCleanupIfConfirmed(access); err != nil {
				failures = append(failures, err)
				continue
			}
			result.Recovered++
			continue
		}
		if failed, retained := h.pending.(*failedCoSChange); retained {
			if failed.access == access {
				if len(access.departures) == 0 {
					result.Attempted++
					if err := access.target.allowRecovery(); err != nil {
						failures = append(failures, h.cleanupFailure(access, failed.labels, err))
						continue
					}
					if err := h.rollbackSessionCleanup(); err != nil {
						failures = append(failures, err)
						continue
					}
					result.Recovered++
					continue
				}
			}
		}
		if _, reportOnly := state.(*sharedCoSReportPending); reportOnly {
			result.Attempted++
			if err := h.clearSharedCleanupIfConfirmed(access); err != nil {
				failures = append(failures, err)
				continue
			}
			result.Recovered++
			continue
		}
		_, uncertain := state.(*sharedCoSUncertain)
		if !uncertain {
			if len(access.departures) == 0 {
				if _, pending := access.warning.(*sharedCoSWarningClearPending); !pending {
					continue
				}
				result.Attempted++
				if err := h.clearSharedCleanupIfConfirmed(access); err != nil {
					failures = append(failures, err)
					continue
				}
				result.Recovered++
				continue
			}
		}
		result.Attempted++
		if err := access.target.allowRecovery(); err != nil {
			failures = append(failures, h.cleanupFailure(access, sessionDiagnostic{}, err))
			continue
		}
		var err error
		keys := make([]sessionKey, 0, len(access.departures))
		for key := range access.departures {
			keys = append(keys, key)
		}
		if len(keys) == 0 {
			err = h.repairSharedUncertainty(access)
			if err == nil {
				err = h.releaseUnusedSharedAccess(access)
			}
		} else {
			for _, key := range keys {
				if cause := h.departSharedSession(key); cause != nil {
					err = errors.Join(err, cause)
				}
			}
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("access %q: %w", name, h.cleanupFailure(access, sessionDiagnostic{}, err)))
			continue
		}
		if err := h.clearSharedCleanupIfConfirmed(access); err != nil {
			failures = append(failures, err)
			continue
		}
		result.Recovered++
	}
	return result, errors.Join(failures...)
}
