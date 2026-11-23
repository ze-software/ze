// Design: docs/guide/bgp-policy.md -- read-only export route-kind simulation.
// Related: policy_dryrun.go -- the simulation is never passed to live output.
package reactor

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/core/opererr"
)

// dryRunExportKind converts only the command's read-only simulation choice.
// The caller MUST NOT pass its result to a live route writer; live output MUST
// derive its kind from its authenticated source and dissemination owner.
func dryRunExportKind(value string) (filterapi.ExportKind, error) {
	switch value {
	case "", "learned":
		return filterapi.ExportOrdinaryLearned, nil
	case "local":
		return filterapi.ExportExplicit, nil
	default:
		return filterapi.ExportUnknown, opererr.New(
			"policy test route-kind "+strconv.Quote(value)+" is unknown",
			"Choose learned or local to select a read-only export prediction.",
			"ze help show policy test peer")
	}
}

// dryRunSimulationKind keeps import free of an export-only simulation input.
// Empty import input means no simulation, not a defaulted export class.
func dryRunSimulationKind(direction, value string) (filterapi.ExportKind, error) {
	if direction == directionImport {
		if value != "" {
			return filterapi.ExportUnknown, opererr.New(
				"policy test import does not accept route-kind "+strconv.Quote(value),
				"Route-kind selects an export prediction; import has one policy answer.",
				"ze help show policy test peer")
		}
		return filterapi.ExportUnknown, nil
	}
	return dryRunExportKind(value)
}

// dryRunMissingPeer preserves the lookup failure while refusing to substitute
// another peer at the same address after removal or concurrent replacement.
func dryRunMissingPeer(owner uint64) error {
	return opererr.Wrap(errPeerNotFound,
		"policy test peer owner "+strconv.FormatUint(owner, 10)+" is not available",
		"The selected peer was not found; resolve a current peer before testing its policy.",
		"ze show bgp")
}

// dryRunFinalAccepts uses the same policy methods as the live import/export
// steps. An empty chain and withdrawal preservation MUST use this same run's
// session; neither may infer local authority from absent source facts.
// session MUST NOT be nil.
func dryRunFinalAccepts(policy filterapi.Policy, session *policyview.Session) bool {
	if session.IsExport() {
		return policy.AcceptsExportAtEnd(session.EBGP, session.ExportKind())
	}
	return policy.AcceptsAtEnd(session.EBGP)
}
