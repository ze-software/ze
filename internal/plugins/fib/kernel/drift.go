// Design: docs/architecture/diagnostics/production-diagnostics.md -- fib-kernel read-only drift checker
// Related: fibkernel.go -- fibKernel installed route state
// Related: register.go -- plugin engine registration of OnDriftCheck

package fibkernel

import (
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const driftStateAbsent = "absent"

func (f *fibKernel) handleDriftCheck(_ string) (rpc.DriftCheckOutput, error) {
	return f.checkDrift(), nil
}

// checkDrift snapshots desired main-table routes under f.mu.RLock, releases the
// lock before querying the kernel backend, and reports missing, extra, or
// next-hop-mismatched Ze routes without mutating kernel or plugin state.
// Safe for concurrent use.
func (f *fibKernel) checkDrift() rpc.DriftCheckOutput {
	f.mu.RLock()
	desired := make(map[string]string, len(f.installed))
	maps.Copy(desired, f.installed)
	backend := f.backend
	f.mu.RUnlock()

	if backend == nil {
		return rpc.DriftCheckOutput{
			Subsystem: pluginName,
			Status:    string(drift.StatusUnavailable),
			Checked:   0,
			Findings:  []rpc.DriftFinding{},
		}
	}

	actualList, err := backend.listZeRoutes()
	if err != nil {
		return rpc.DriftCheckOutput{
			Subsystem: pluginName,
			Status:    string(drift.StatusError),
			Checked:   0,
			Findings:  []rpc.DriftFinding{},
			Error:     err.Error(),
		}
	}

	actual := make(map[string]string, len(actualList))
	for _, r := range actualList {
		if r.table != 0 {
			continue
		}
		actual[r.prefix] = r.nextHop
	}

	findings := make([]rpc.DriftFinding, 0)
	drifted := 0
	record := func(finding rpc.DriftFinding) {
		drifted++
		if len(findings) < drift.MaxFindingsPerSubsystem {
			findings = append(findings, finding)
		}
	}

	for _, prefix := range slices.Sorted(maps.Keys(desired)) {
		desiredNH := desired[prefix]
		actualNH, exists := actual[prefix]
		if !exists {
			record(rpc.DriftFinding{
				Subject:  prefix,
				Kind:     string(drift.KindMissing),
				Expected: desiredNH,
				Actual:   driftStateAbsent,
			})
			continue
		}
		if actualNH != desiredNH {
			record(rpc.DriftFinding{
				Subject:  prefix,
				Kind:     string(drift.KindMismatch),
				Expected: desiredNH,
				Actual:   actualNH,
			})
		}
	}

	for _, prefix := range slices.Sorted(maps.Keys(actual)) {
		if _, wanted := desired[prefix]; wanted {
			continue
		}
		record(rpc.DriftFinding{
			Subject:  prefix,
			Kind:     string(drift.KindExtra),
			Expected: driftStateAbsent,
			Actual:   actual[prefix],
		})
	}

	status := string(drift.StatusClean)
	if drifted > 0 {
		status = string(drift.StatusDrifted)
	}
	return rpc.DriftCheckOutput{
		Subsystem: pluginName,
		Status:    status,
		Checked:   len(desired),
		Drifted:   drifted,
		Truncated: drifted > drift.MaxFindingsPerSubsystem,
		Findings:  findings,
	}
}
