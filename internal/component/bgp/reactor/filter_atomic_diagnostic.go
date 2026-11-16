// Design: docs/architecture/bgp/egress-attribute-rules.md -- marked-route policy admission.
// Related: filter_library.go -- the shared import/export result guard.
// Related: docs/architecture/doctor-and-health-checks.md -- bounded historical events.

package reactor

import (
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	codeAtomicAggregatePolicy        = "doctor-bgp-atomic-aggregate-policy"
	atomicAggregatePolicyFix         = "ze config edit"
	atomicAggregatePolicyMessage     = "BGP policy result refused: a marked route must keep ATOMIC_AGGREGATE and its original destinations; the source routes are withdrawn. Edit the import or export filter to preserve the marker and retain only original routes, then commit now."
	codeAtomicAggregateRetirement    = "doctor-bgp-atomic-aggregate-retirement"
	atomicAggregateRetirementMessage = "BGP policy source removal could not be prepared; previously advertised routes may remain. Edit the import or export filter to keep the original routes, then commit now."
)

// init makes the transient refusal visible through the shared recent-events
// doctor check. It is history, not an active warning that outlives a repair.
func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeAtomicAggregatePolicy,
		Title:       "BGP policy changed a marked route's destinations or marker",
		Description: atomicAggregatePolicyMessage,
		Examples:    []string{atomicAggregatePolicyFix, "ze show diagnostic doctor-bgp-atomic-aggregate-policy"},
	}); err != nil {
		panic("BUG: bgp reactor: atomic aggregate policy diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeAtomicAggregatePolicy}); err != nil {
		panic("BUG: bgp reactor: atomic aggregate policy event registration failed: " + err.Error())
	}
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeAtomicAggregateRetirement,
		Title:       "BGP policy source removal could not be prepared",
		Description: atomicAggregateRetirementMessage,
		Examples:    []string{atomicAggregatePolicyFix, "ze show diagnostic doctor-bgp-atomic-aggregate-retirement"},
	}); err != nil {
		panic("BUG: bgp reactor: atomic aggregate retirement diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeAtomicAggregateRetirement}); err != nil {
		panic("BUG: bgp reactor: atomic aggregate retirement event registration failed: " + err.Error())
	}
}

// reportAtomicAggregatePolicyRefusal records at most one refusal per subject
// per second in the bounded error ring. Safe for concurrent chain runs; the
// ring supplies the rate-limit ledger without a second peer-keyed cache.
func reportAtomicAggregatePolicyRefusal(peer, direction string, cause error) {
	if !report.RaiseErrorFixOnce(reportSourceBGP, codeAtomicAggregatePolicy, peer,
		atomicAggregatePolicyMessage, atomicAggregatePolicyFix, time.Second) {
		return
	}
	fwdLogger().Warn(atomicAggregatePolicyMessage,
		"peer", peer, "direction", direction, "error", cause,
		"fix", atomicAggregatePolicyFix, "code", codeAtomicAggregatePolicy)
}

// reportAtomicAggregateRetirementFailure records a failed preparation, not a
// completed source withdrawal. The caller MUST pass its non-nil cause.
// Safe for concurrent forwarding; the shared event ring bounds repeated errors.
func reportAtomicAggregateRetirementFailure(peer string, cause error) {
	message := report.FitMessage(atomicAggregateRetirementMessage + " Cause: " + cause.Error())
	if !report.RaiseErrorFixOnce(reportSourceBGP, codeAtomicAggregateRetirement, peer,
		message, atomicAggregatePolicyFix, time.Second) {
		return
	}
	fwdLogger().Warn(message, "peer", peer, "error", cause,
		"fix", atomicAggregatePolicyFix, "code", codeAtomicAggregateRetirement)
}
