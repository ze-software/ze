// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn match condition
// Related: register.go -- registers the reject-asn condition and its obligation
// Related: config.go -- parseOneList, the list parse the condition shares
// Related: match.go -- matchPositions and matchPattern, the judgement it shares
//
// The reject-asn match condition: a match instance's `reject-asn NAME` passes
// a route the named reject-asn list does not reject. It reads the list's own
// definition and judges the same AS-path text with the same position and
// pattern arms (match.go), with the sender read off the view's session, which
// is known on import only.

package filter_path_asn

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"unsafe"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/policy"
)

// compileBGPCondition compiles a match instance's reject-asn value, the name
// of one reject-asn instance, into its check over a BGP UPDATE. The route
// passes when the list rejects it on neither arm. A reject that refuses the
// route on a live run (MatchRoute.Reports: action accept, not a display-only
// trace) is counted under the direction and the slot that decided it. A list rejecting a
// route under action reject lets it pass, so nothing is refused and nothing
// is counted. A match judging each NLRI entry counts one refusal per UPDATE
// (MatchRoute.Counted). An AS_PATH the view refuses as malformed is an error, so the
// member fails closed.
//
// RFC 7454 Section 9: "This loose policy could be combined with filters for
// specific 2-byte or 4-byte AS paths that must not be accepted if advertised by
// the customer, such as upstream transit providers or peer ASNs."
func compileBGPCondition(value any, resolve policy.InstanceResolver) (policy.ConditionCheck[policychain.MatchRoute], error) {
	list, err := conditionList(value, resolve)
	if err != nil {
		return nil, err
	}
	return conditionCheck(list), nil
}

// conditionCheck is the check over one parsed reject-asn list.
func conditionCheck(list *rejectList) policy.ConditionCheck[policychain.MatchRoute] {
	return func(r *policychain.MatchRoute) (bool, error) {
		text, err := r.View.ASPathText()
		if err != nil {
			return false, err
		}
		// The text is the view's own buffer, unchanged until the run ends, and
		// no judgement below keeps the string past its return.
		asPath := unsafe.String(unsafe.SliceData(text), len(text)) //nolint:gosec // audited: the view's buffer outlives this synchronous check
		session := r.View.Session()
		direction := directionExport
		sender := senderASN{}
		if session.SenderKnown {
			direction = directionImport
			sender = senderASN{asn: session.Sender, known: true}
		}
		if found, ok := matchPositions(asPath, sender, list.positions, list.nth); ok {
			if r.Reports && !r.Counted {
				recordReject(direction, rejectSlot(found.at))
				r.Counted = true
			}
			return false, nil
		}
		if _, ok := matchPattern(asPath, list.patterns); ok {
			if r.Reports && !r.Counted {
				recordReject(direction, slotPattern)
				r.Counted = true
			}
			return false, nil
		}
		return true, nil
	}
}

// conditionList reads the reject-asn list the condition value names.
func conditionList(value any, resolve policy.InstanceResolver) (*rejectList, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one reject-asn instance")
	}
	if name == "" {
		return nil, errors.New("must name one reject-asn instance")
	}
	if resolve == nil {
		return nil, errors.New("BUG: compiled with no instance resolver")
	}
	instance, err := resolve(name, filterTypeName)
	if err != nil {
		return nil, err
	}
	return parseOneList(name, instance)
}

// bgpReportEnabled reports whether a decision line is written: Info on the
// bgp.filter.path.asn logger for a reject, as the filter writes it. The
// accept line is Debug, so it is on whenever this is.
func bgpReportEnabled() bool {
	return logger().Enabled(context.Background(), slog.LevelInfo)
}

// compileBGPReport binds the reject-asn condition's decision lines to the
// list the value names, the lines the filter writes (handleFilterUpdate):
// a reject at Info naming the listed ASN and its position, or the pattern
// (logListedReject, logPatternReject), an accept at Debug, each with the
// list as filter=, the direction, the peer and the AS path.
func compileBGPReport(value any, resolve policy.InstanceResolver) (policychain.MatchReport, error) {
	list, err := conditionList(value, resolve)
	if err != nil {
		return nil, err
	}
	return func(route *policychain.MatchRoute, outcome *policychain.MatchOutcome) {
		text, err := route.View.ASPathText()
		if err != nil {
			return // the check already failed the member on this read
		}
		// The text is the view's own buffer; each line clones what it keeps.
		asPath := unsafe.String(unsafe.SliceData(text), len(text)) //nolint:gosec // audited: the view's buffer outlives this synchronous report
		if outcome.Verdict != policychain.MatchVerdictReject {
			l := logger()
			if l.Enabled(context.Background(), slog.LevelDebug) {
				l.Debug("reject-asn accept", "filter", list.name, "direction", outcome.Direction,
					"peer", outcome.Peer, "as-path", string(text))
			}
			return
		}
		sender := senderASN{}
		if session := route.View.Session(); session.SenderKnown {
			sender = senderASN{asn: session.Sender, known: true}
		}
		if found, ok := matchPositions(asPath, sender, list.positions, list.nth); ok {
			logListedReject(list.name, outcome.Direction, outcome.Peer, found, asPath)
			return
		}
		if pattern, ok := matchPattern(asPath, list.patterns); ok {
			logPatternReject(list.name, outcome.Direction, outcome.Peer, pattern, asPath)
			return
		}
		// A match under action reject refuses the route the list passes.
		l := logger()
		if l.Enabled(context.Background(), slog.LevelInfo) {
			l.Info("reject-asn reject", "filter", list.name, "direction", outcome.Direction,
				"peer", outcome.Peer, "as-path", string(text))
		}
	}, nil
}

// logListedReject logs one reject a listed ASN decided, at Info: the line
// the operator reads to learn which peer, which ASN and which rule. The
// filter (handleFilterUpdate) and the reject-asn match condition both write
// it. asPath is cloned only when the line is written, so a caller may pass a
// string over a buffer it reuses after the call.
func logListedReject(list, direction, peer string, found hit, asPath string) {
	l := logger()
	if !l.Enabled(context.Background(), slog.LevelInfo) {
		return
	}
	asPath = strings.Clone(asPath)
	// An nth reject names the collapsed position it matched at, because
	// "nth" alone does not tell the operator which of their rules fired.
	if found.at == positionNth {
		l.Info("reject-asn reject",
			"filter", list, "direction", direction, "peer", peer,
			"asn", found.asn, "position", found.at.String(), "index", found.index,
			"as-path", asPath)
		return
	}
	l.Info("reject-asn reject",
		"filter", list, "direction", direction, "peer", peer,
		"asn", found.asn, "position", found.at.String(), "as-path", asPath)
}

// logPatternReject logs one reject a regex pattern decided, at Info, as
// logListedReject does for a listed ASN.
func logPatternReject(list, direction, peer string, pattern *regexp.Regexp, asPath string) {
	l := logger()
	if !l.Enabled(context.Background(), slog.LevelInfo) {
		return
	}
	l.Info("reject-asn reject",
		"filter", list, "direction", direction, "peer", peer,
		"pattern", pattern.String(), "as-path", strings.Clone(asPath))
}
