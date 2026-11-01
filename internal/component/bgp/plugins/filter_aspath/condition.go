// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: register.go -- registers the as-path condition at a BGP chain
// Related: match.go -- evaluateASPathText, the first-match walk the condition runs
// Related: config.go -- parseAsPathEntries, the entry parse the condition shares
//
// The as-path match condition: a match instance's `as-path NAME` passes an
// UPDATE whose AS path the named as-path-list accepts. It reads the list's
// own definition and walks the same regular expressions over the same text,
// so it accepts exactly what the list accepts as a chain member.

package filter_aspath

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/policy"
)

// compileBGPCondition compiles a match instance's as-path value, the name of
// one as-path-list instance, into its check over a BGP UPDATE: the UPDATE
// passes when the list's first matching entry accepts its AS path, read as
// the flat decimal text (policyview.View.ASPathText); a path no entry
// matches fails (the implicit deny of evaluateASPathText). An UPDATE with no
// AS path is judged as the empty text. A malformed AS path fails the member.
func compileBGPCondition(value any, resolve policy.InstanceResolver) (policy.ConditionCheck[policychain.MatchRoute], error) {
	entries, err := conditionEntries(value, resolve)
	if err != nil {
		return nil, err
	}
	return func(r *policychain.MatchRoute) (bool, error) {
		text, err := r.View.ASPathText()
		if err != nil {
			return false, err
		}
		return evaluateASPathText(entries, text) == actionAccept, nil
	}, nil
}

// conditionEntries reads the as-path-list the condition value names, from
// the instance config, and compiles its entries.
func conditionEntries(value any, resolve policy.InstanceResolver) ([]aspathEntry, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one as-path-list instance")
	}
	if name == "" {
		return nil, errors.New("must name one as-path-list instance")
	}
	if resolve == nil {
		return nil, errors.New("BUG: compiled with no instance resolver")
	}
	instance, err := resolve(name, filterTypeName)
	if err != nil {
		return nil, err
	}
	return parseAsPathEntries(name, instance)
}

// The decision lines a match naming an as-path-list writes.
const (
	lineAccept = "as-path-list accept"
	lineReject = "as-path-list reject"
	lineModify = "as-path-list modify"
)

// bgpReportEnabled reports whether the decision line is written: Info on the
// package logger, as the filter writes it.
func bgpReportEnabled() bool {
	return logger().Enabled(context.Background(), slog.LevelInfo)
}

// compileBGPReport binds the as-path condition's decision line to the list
// the value names: the verdict, the list as filter=, the peer and the AS path.
func compileBGPReport(value any, _ policy.InstanceResolver) (policychain.MatchReport, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one as-path-list instance")
	}
	return func(route *policychain.MatchRoute, outcome *policychain.MatchOutcome) {
		var line string
		switch outcome.Verdict {
		case policychain.MatchVerdictAccept:
			line = lineAccept
		case policychain.MatchVerdictReject:
			line = lineReject
		case policychain.MatchVerdictModify:
			line = lineModify
		case policychain.MatchVerdictUnspecified:
			return // the evaluator names a verdict for every decision
		}
		l := logger()
		if !l.Enabled(context.Background(), slog.LevelInfo) {
			return
		}
		text, err := route.View.ASPathText()
		if err != nil {
			return // the check already failed the member on this read
		}
		l.Info(line, "filter", name, "peer", outcome.Peer, "as-path", string(text))
	}, nil
}
