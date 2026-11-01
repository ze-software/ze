// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: register.go -- registers the community condition for the BGP view
// Related: config.go -- parseCommunityEntries, the entry parse the condition shares
// Related: match.go -- the community entry types the check walks
//
// The community match condition: a match instance's `community NAME` passes
// a route whose communities the named community-match list accepts. It reads
// the list's own definition and walks its entries in order, first match wins
// and no match denies, exactly as the list's filter does, over the
// attributes' octets instead of the text form.

package filter_community_match

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
)

// wireEntry is one community-match entry as the octets it matches.
type wireEntry struct {
	reader policyview.Reader // the attribute the entry's kind reads
	value  [12]byte          // the community, in its first octets
	octets int               // 4 standard, 12 large, 8 extended
	action action
}

// compileBGPCondition compiles a match instance's community value, the name
// of one community-match instance, into its check over a BGP UPDATE. The
// route passes when the list's first entry whose community the UPDATE
// carries accepts; a route no entry matches fails (implicit deny). An attribute the view refuses as malformed is an
// error, so the member fails closed.
func compileBGPCondition(value any, resolve policy.InstanceResolver) (policy.ConditionCheck[policychain.MatchRoute], error) {
	entries, err := conditionEntries(value, resolve)
	if err != nil {
		return nil, err
	}
	return func(r *policychain.MatchRoute) (bool, error) {
		return judgeEntries(entries, r.View)
	}, nil
}

// judgeEntries walks the entries in order and answers the first matching
// entry's action, false when none matches.
func judgeEntries(entries []wireEntry, view *policyview.View) (bool, error) {
	for i := range entries {
		e := &entries[i]
		attr, err := view.Read(e.reader)
		if err != nil {
			return false, err
		}
		if !attr.Present() {
			continue
		}
		if carries(attr.Wire(), e.value[:e.octets]) {
			return e.action == actionAccept, nil
		}
	}
	return false, nil
}

// carries reports whether the attribute value holds community among its
// entries. The view has checked the value is a non-zero multiple of the
// entry size (RFC 7606 Section 7.8 and 7.14, RFC 8092 Section 6).
//
// RFC 4360 Section 2: "Two extended communities are declared equal only when
// all 8 octets of the community are equal."
func carries(attr, community []byte) bool {
	for off := 0; off+len(community) <= len(attr); off += len(community) {
		if bytes.Equal(attr[off:off+len(community)], community) {
			return true
		}
	}
	return false
}

// conditionEntries reads the community-match list the condition value names
// and turns its entries into the octets each matches.
func conditionEntries(value any, resolve policy.InstanceResolver) ([]wireEntry, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one community-match instance")
	}
	if name == "" {
		return nil, errors.New("must name one community-match instance")
	}
	if resolve == nil {
		return nil, errors.New("BUG: compiled with no instance resolver")
	}
	instance, err := resolve(name, filterTypeName)
	if err != nil {
		return nil, err
	}
	parsed, err := parseCommunityEntries(name, instance)
	if err != nil {
		return nil, err
	}
	out := make([]wireEntry, 0, len(parsed))
	for _, e := range parsed {
		reader, err := policyview.ReaderFor(kindField(e.ctype))
		if err != nil {
			return nil, fmt.Errorf("BUG: filter-community-match: %w", err)
		}
		wire, octets, err := filtertext.CommunityWire(e.community, e.ctype)
		if err != nil {
			return nil, fmt.Errorf("community-match %q entry %q: %w", name, e.community, err)
		}
		out = append(out, wireEntry{reader: reader, value: wire, octets: octets, action: e.action})
	}
	return out, nil
}

// kindField answers the route field that carries a community kind.
func kindField(kind communityType) policy.Field {
	switch kind {
	case communityStandard:
		return policy.FieldCommunity
	case communityLarge:
		return policy.FieldLargeCommunity
	case communityExtended:
		return policy.FieldExtendedCommunity
	default:
		panic("BUG: invalid community kind")
	}
}

// The decision lines a match naming a community-match list writes.
const (
	lineAccept = "community-match accept"
	lineReject = "community-match reject"
	lineModify = "community-match modify"
)

// bgpReportEnabled reports whether the decision line is written: Info on the
// package logger, as the filter writes it.
func bgpReportEnabled() bool {
	return logger().Enabled(context.Background(), slog.LevelInfo)
}

// compileBGPReport binds the community condition's decision line to the list
// the value names: the verdict, the list as filter=, the peer.
func compileBGPReport(value any, _ policy.InstanceResolver) (policychain.MatchReport, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one community-match instance")
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
		l.Info(line, "filter", name, "peer", outcome.Peer)
	}, nil
}
