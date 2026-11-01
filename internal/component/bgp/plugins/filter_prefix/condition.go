// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: register.go -- registers the prefix-list condition for both views
// Related: match.go -- evaluatePrefix, the first-match walk the condition shares
// Related: config.go -- parsePrefixListEntries, the entry parse the condition shares
//
// The prefix-list match condition: a match instance's `prefix-list NAME`
// passes a route whose prefix the named list accepts. It is the only
// prefix-list check a match runs, and it reads the list's own definition, so
// it accepts exactly what the list accepts on a BGP session.

package filter_prefix

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/policy"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// compileRouteCondition compiles a match instance's prefix-list value, the
// name of one prefix-list instance, into its check over a Loc-RIB route. The
// route passes when the list's first matching entry accepts its prefix; a
// prefix no entry matches fails (the implicit deny of evaluatePrefix).
func compileRouteCondition(value any, resolve policy.InstanceResolver) (policy.RouteCheck, error) {
	entries, err := conditionEntries(value, resolve)
	if err != nil {
		return nil, err
	}
	return func(r *ribpolicy.Route) (bool, error) {
		return evaluatePrefix(entries, r.Prefix) == actionAccept, nil
	}, nil
}

// compileBGPCondition compiles the same value into its check over one NLRI
// prefix of a BGP UPDATE (policychain.MatchRoute): the prefix passes when the
// list's first matching entry accepts it. The match walks the UPDATE's
// prefixes and answers it from the per-prefix verdicts. An invalid prefix
// fails: absence is never a match.
func compileBGPCondition(value any, resolve policy.InstanceResolver) (policy.ConditionCheck[policychain.MatchRoute], error) {
	entries, err := conditionEntries(value, resolve)
	if err != nil {
		return nil, err
	}
	return func(r *policychain.MatchRoute) (bool, error) {
		if !r.Prefix.IsValid() {
			return false, nil
		}
		return evaluatePrefix(entries, r.Prefix) == actionAccept, nil
	}, nil
}

// The decision lines a match naming a prefix-list writes.
const (
	lineAccept = "prefix-list accept"
	lineReject = "prefix-list reject"
	lineModify = "prefix-list modify"
)

// bgpReportEnabled reports whether the decision line is written: Info on the
// bgp.filter.prefix logger, as the filter writes it.
func bgpReportEnabled() bool {
	return logger().Enabled(context.Background(), slog.LevelInfo)
}

// compileBGPReport binds the prefix-list condition's decision line to the
// list the value names: the verdict, the list as filter=, the peer, the
// judged prefixes, and for a modify the accepted and rejected counts.
func compileBGPReport(value any, _ policy.InstanceResolver) (policychain.MatchReport, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one prefix-list instance")
	}
	return func(_ *policychain.MatchRoute, outcome *policychain.MatchOutcome) {
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
		nlri := string(outcome.AppendPrefixes(nil))
		if outcome.Verdict == policychain.MatchVerdictModify {
			l.Info(line, "filter", name, "peer", outcome.Peer,
				"accepted", outcome.Accepted, "rejected", outcome.Rejected, "nlri", nlri)
			return
		}
		l.Info(line, "filter", name, "peer", outcome.Peer, "nlri", nlri)
	}, nil
}

// conditionEntries reads the prefix-list the condition value names and
// parses its entries.
func conditionEntries(value any, resolve policy.InstanceResolver) ([]prefixEntry, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one prefix-list instance")
	}
	if name == "" {
		return nil, errors.New("must name one prefix-list instance")
	}
	if resolve == nil {
		return nil, errors.New("BUG: compiled with no instance resolver")
	}
	instance, err := resolve(name, filterTypeName)
	if err != nil {
		return nil, err
	}
	return parsePrefixListEntries(name, instance)
}
