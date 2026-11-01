// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: match.go -- the evaluator that hands each decision to its reports
//
// The decision line of a match at a BGP chain. Each condition's package
// registers how its line reads, beside its check, so the operator reads the
// line the condition's own filter wrote (prefix-list accept, as-path-list
// reject, ...), on the same logger, at the same level.

package policychain

import (
	"cmp"
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/policy"
)

// MatchVerdict is a match member's answer on one UPDATE, as its decision
// line names it. The zero value is no verdict.
type MatchVerdict uint8

const (
	MatchVerdictUnspecified MatchVerdict = iota
	// MatchVerdictAccept passes the UPDATE whole.
	MatchVerdictAccept
	// MatchVerdictReject refuses the UPDATE.
	MatchVerdictReject
	// MatchVerdictModify passes the UPDATE with its rejected NLRI entries
	// dropped.
	MatchVerdictModify
)

// MatchOutcome is one match member's decision on one UPDATE, as a report
// reads it. Valid during the report call only: a report MUST NOT keep it.
type MatchOutcome struct {
	Verdict MatchVerdict
	// Direction ("import" or "export") and Peer name the session, as an
	// external filter's filter-update names it (ExternalRoute). Empty when the
	// caller named none.
	Direction, Peer string
	// Accepted and Rejected count the NLRI entries judged, and Prefixes holds
	// them in walk order, for a match whose conditions read prefix. Both are
	// zero for one that judges the UPDATE once.
	Accepted, Rejected int
	Prefixes           []netip.Prefix
}

// AppendPrefixes appends the judged prefixes to buf, one space between two.
func (o *MatchOutcome) AppendPrefixes(buf []byte) []byte {
	for i, prefix := range o.Prefixes {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = prefix.AppendTo(buf)
	}
	return buf
}

// MatchReport writes one condition's decision line for one match member.
// route is the route the member judged, still valid during the call. A
// report MUST check its own level before it formats anything.
type MatchReport func(route *MatchRoute, outcome *MatchOutcome)

// MatchReporter is how one condition's decision line is written.
type MatchReporter struct {
	// Enabled reports whether the condition's line would be written now. It
	// MUST NOT allocate: a live run asks it for every UPDATE, and judges the
	// UPDATE without collecting what a line reads when every report is off.
	Enabled func() bool
	// Compile binds the condition's config value, as the condition's check
	// compiler does. It MUST NOT change any state.
	Compile func(value any, resolve policy.InstanceResolver) (MatchReport, error)
}

// matchReporters holds the registered reporters by condition. Safe for
// concurrent use.
var matchReporters = struct {
	mu    sync.RWMutex
	byKey map[policy.MatchCondition]MatchReporter
}{byKey: make(map[policy.MatchCondition]MatchReporter)}

// MustAddMatchReporter registers how cond's decision line is written at a
// BGP chain. It MUST be called from init(), beside MatchConditions.MustAdd
// for the same condition. A reporter with no Enabled or Compile, and a
// second one for cond, are Ze defects and panic.
func MustAddMatchReporter(cond policy.MatchCondition, reporter MatchReporter) {
	if reporter.Enabled == nil {
		panic("BUG: policychain: match reporter " + cond.Key() + " has no Enabled")
	}
	if reporter.Compile == nil {
		panic("BUG: policychain: match reporter " + cond.Key() + " has no Compile")
	}
	matchReporters.mu.Lock()
	defer matchReporters.mu.Unlock()
	if _, dup := matchReporters.byKey[cond]; dup {
		panic("BUG: policychain: match reporter " + cond.Key() + " added twice")
	}
	matchReporters.byKey[cond] = reporter
}

// boundReport is one condition's report, bound to its config value.
type boundReport struct {
	enabled func() bool
	write   MatchReport
}

// compileMatchReports binds the report of every condition instance names, in
// condition key order so the lines of one decision always come in one order.
func compileMatchReports(instance map[string]any, resolve policy.InstanceResolver) ([]boundReport, error) {
	matchReporters.mu.RLock()
	conditions := make([]policy.MatchCondition, 0, len(matchReporters.byKey))
	for cond := range matchReporters.byKey {
		if _, named := instance[cond.Key()]; named {
			conditions = append(conditions, cond)
		}
	}
	reporters := make([]MatchReporter, 0, len(conditions))
	slices.SortFunc(conditions, func(a, b policy.MatchCondition) int {
		return cmp.Compare(a.Key(), b.Key())
	})
	for _, cond := range conditions {
		reporters = append(reporters, matchReporters.byKey[cond])
	}
	matchReporters.mu.RUnlock()

	reports := make([]boundReport, 0, len(reporters))
	for i, reporter := range reporters {
		write, err := reporter.Compile(instance[conditions[i].Key()], resolve)
		if err != nil {
			return nil, err
		}
		reports = append(reports, boundReport{enabled: reporter.Enabled, write: write})
	}
	return reports, nil
}

// reporting reports whether a decision of this run is written: the run is
// live (not Chain.Trace) and some report's level is on. It allocates nothing.
func (e *matchEvaluator) reporting(sets *Sets) bool {
	if sets.display {
		return false
	}
	for i := range e.reports {
		if e.reports[i].enabled() {
			return true
		}
	}
	return false
}

// report hands the member's decision to each report whose level is on. It
// runs only when reporting answered true, so what it builds costs nothing
// while every line is off.
func (e *matchEvaluator) report(w *matchWalk, decision Decision) {
	outcome := MatchOutcome{Verdict: MatchVerdictAccept,
		Accepted: w.accepted, Rejected: w.rejected, Prefixes: w.judged}
	if decision == DecisionReject {
		outcome.Verdict = MatchVerdictReject
	}
	if decision == DecisionPass && w.rejected > 0 {
		outcome.Verdict = MatchVerdictModify
	}
	if route := w.sets.route; route != nil {
		outcome.Direction, outcome.Peer = route.Direction, route.Peer
	}
	for i := range e.reports {
		if e.reports[i].enabled() {
			e.reports[i].write(w.route, &outcome)
		}
	}
}
