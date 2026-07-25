// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

package runner

import "github.com/ze-software/ze/internal/core/hostload"

const (
	// FailTypeNearTimeout classifies a failure where the test consumed >80%
	// of its timeout budget without the context deadline actually firing.
	// Distinguishes CPU-starvation near-misses from genuine "unknown" failures.
	FailTypeNearTimeout = "near_timeout"

	// nearTimeoutThreshold is the fraction of timeout elapsed above which
	// a non-specific failure is reclassified as near-timeout.
	nearTimeoutThreshold = 0.80
)

// HostLoad is the shared host-load snapshot type. The "contended" verdict and
// the load/process sampling live in internal/core/hostload so this runner and
// the verify status tool (scripts/status) share one definition.
type HostLoad = hostload.Load

// SnapshotHostLoad samples the current host load. See hostload.Snapshot.
func SnapshotHostLoad() HostLoad {
	return hostload.Snapshot()
}

// IsNearTimeout returns true when a test failure should be reclassified
// from "unknown" to "near_timeout". This happens when the test consumed
// more than 80% of its timeout budget and the failure type is non-specific
// (empty or "unknown"), indicating CPU starvation rather than a real bug.
func IsNearTimeout(elapsedRatio float64, failureType string) bool {
	if elapsedRatio <= nearTimeoutThreshold {
		return false
	}
	return failureType == "" || failureType == stateUnknown
}
