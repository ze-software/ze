// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Design: docs/architecture/core-design.md -- the in-process BGP chain
// Related: register.go -- registers the match evaluator
// Related: intents.go -- DropNLRI, the subset a mixed verdict answers
//
// The match filter type at a BGP chain. A match judges routes, and an UPDATE
// carries several: a match whose conditions read prefix judges every NLRI
// entry the UPDATE names, announced and withdrawn, and answers the UPDATE.

package policychain

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// MatchRoute is the route a BGP match condition judges: the UPDATE's view,
// and the one NLRI prefix under judgement. Prefix is valid only for a match
// whose conditions read prefix; a condition that reads it on an invalid
// prefix fails, as absence is never a match. A check MUST NOT keep either
// past its return.
type MatchRoute struct {
	View   *policyview.View
	Prefix netip.Prefix
}

// MatchConditions holds the checks of the match conditions a BGP chain can
// judge. Each condition's package adds its check from init().
var MatchConditions = policy.NewConditions[MatchRoute]("a BGP UPDATE")

var (
	errMatchCIDRFraming  = errors.New("an NLRI entry is longer than its family's address")
	errMatchLabelFraming = errors.New("a labeled NLRI entry's label field runs past its length")
)

// matchEvaluator is one compiled match instance at a BGP chain.
type matchEvaluator struct {
	match  *policy.Match[MatchRoute]
	prefix policyview.Reader
	// perPrefix is set when a condition reads prefix: the match then judges
	// each NLRI entry rather than the UPDATE once.
	perPrefix bool
}

// compileMatchEvaluator compiles a match instance over MatchConditions. A
// condition with no BGP check is refused naming it (policy.CompileMatch).
func compileMatchEvaluator(_ string, instance map[string]any, resolve policy.InstanceResolver) (Evaluator, error) {
	match, err := policy.CompileMatch(MatchConditions, instance, false, resolve)
	if err != nil {
		return nil, err
	}
	reader, err := policyview.ReaderFor(policy.FieldPrefix)
	if err != nil {
		return nil, fmt.Errorf("BUG: policychain: %w", err)
	}
	e := &matchEvaluator{match: match, prefix: reader,
		perPrefix: slices.Contains(policy.MatchFields(instance, false), policy.FieldPrefix)}
	return e.evaluate, nil
}

// evaluate answers the UPDATE: with no prefix condition, the match's verdict
// on the UPDATE. Judging each entry, it passes an UPDATE that names no prefix
// or none the match rejects, and rejects one whose every entry is rejected,
// one whose FlowSpec destination is absent, and one with a rejected entry in
// a section a drop cannot rewrite (FlowSpec). Otherwise it passes and drops
// each rejected entry (DropNLRI), announced and withdrawn.
func (e *matchEvaluator) evaluate(view *policyview.View, sets *Sets) (Decision, error) {
	sets.matchRoute = MatchRoute{View: view}
	decision, err := e.judge(view, sets)
	sets.matchRoute = MatchRoute{} // the view MUST NOT outlive the run
	return decision, err
}

func (e *matchEvaluator) judge(view *policyview.View, sets *Sets) (Decision, error) {
	w := matchWalk{e: e, sets: sets, route: &sets.matchRoute}
	if !e.perPrefix {
		return w.verdict()
	}
	announced, err := view.Read(e.prefix)
	if err != nil {
		return DecisionUnspecified, err
	}
	if err := w.section(NLRISectionAnnounce, family.IPv4Unicast, announced.Wire()); err != nil {
		return DecisionUnspecified, err
	}
	if mp := announced.MPReach(); mp != nil {
		if err := w.section(NLRISectionMPReach, mp.Family(), mp.NLRIBytes()); err != nil {
			return DecisionUnspecified, err
		}
	}
	withdrawn, mpUnreach, err := view.Withdrawn()
	if err != nil {
		return DecisionUnspecified, err
	}
	if err := w.section(NLRISectionWithdrawn, family.IPv4Unicast, withdrawn); err != nil {
		return DecisionUnspecified, err
	}
	if mpUnreach != nil {
		if err := w.section(NLRISectionMPUnreach, mpUnreach.Family(), mpUnreach.WithdrawnBytes()); err != nil {
			return DecisionUnspecified, err
		}
	}
	return w.decision(), nil
}

// matchWalk is one evaluate's walk over the NLRI sections.
type matchWalk struct {
	e     *matchEvaluator
	sets  *Sets
	route *MatchRoute // the Sets' scratch route
	// accepted and rejected count the entries judged.
	accepted, rejected int
	// whole is set when the UPDATE is rejected whatever the counts: a FlowSpec
	// destination that is absent or rejected, or a section that does not frame.
	whole bool
}

func (w *matchWalk) verdict() (Decision, error) {
	rejects, err := w.e.match.Rejects(w.route)
	if err != nil {
		return DecisionUnspecified, err
	}
	if rejects {
		return DecisionReject, nil
	}
	return DecisionPass, nil
}

func (w *matchWalk) decision() Decision {
	if w.whole {
		return DecisionReject
	}
	if w.rejected == 0 {
		return DecisionPass
	}
	if w.accepted == 0 {
		return DecisionReject
	}
	return DecisionPass // with the rejected entries dropped
}

// section judges each entry of one NLRI section of family fam. A family whose
// entries are neither plain prefixes nor FlowSpec rules names no prefix.
func (w *matchWalk) section(section NLRISection, fam family.Family, nlri []byte) error {
	if len(nlri) == 0 {
		return nil
	}
	addPath, err := w.route.View.AddPath(fam)
	if err != nil {
		return err
	}
	if fam.SAFI == family.SAFIUnicast || fam.SAFI == family.SAFIMulticast {
		return w.cidrSection(section, fam, nlri, addPath, false)
	}
	if fam.SAFI == family.SAFIMPLSLabel {
		return w.cidrSection(section, fam, nlri, addPath, true)
	}
	if nlrisplit.IsFlowSpec(fam) {
		return w.flowSpecSection(fam, nlri, addPath)
	}
	return nil
}

// cidrSection judges each entry of a unicast, multicast or labeled-unicast
// section, and drops the rejected ones at their offset in the section the view
// answered. A labeled entry's prefix is read past its label field in place
// (nlrisplit.LabeledPrefix): a withdrawal's one Compatibility field, an
// announcement's label stack. A dropped entry keeps its offset, so the
// engine removes it whole, label field included (wireu.DropCIDREntries).
//
// RFC 4271 Section 4.3: "The Prefix field contains an IP address prefix, followed by
// the minimum number of trailing bits needed to make the end
// of the field fall on an octet boundary.  Note that the value
// of trailing bits is irrelevant."
func (w *matchWalk) cidrSection(section NLRISection, fam family.Family, nlri []byte, addPath, labeled bool) error {
	withdraw := section == NLRISectionWithdrawn || section == NLRISectionMPUnreach
	cursor := wireu.NewCIDRCursor(nlri, addPath)
	for {
		_, cidr, start, more := cursor.Next()
		if !more {
			break
		}
		bits, octets := int(cidr[0]), cidr[1:]
		if labeled {
			// RFC 8277 Section 2.3, Section 2.4
			var framed bool
			if bits, octets, framed = nlrisplit.LabeledPrefix(cidr, withdraw); !framed {
				return errors.Join(policyview.ErrMalformed, errMatchLabelFraming)
			}
		}
		prefix, ok := cidrPrefix(bits, octets, fam.AFI == family.AFIIPv4)
		if !ok {
			return errors.Join(policyview.ErrMalformed, errMatchCIDRFraming)
		}
		w.route.Prefix = prefix
		rejects, err := w.e.match.Rejects(w.route)
		if err != nil {
			return err
		}
		if !rejects {
			w.accepted++
			continue
		}
		w.rejected++
		if err := w.sets.DropNLRI(section, uint16(start)); err != nil { //nolint:gosec // G115: an offset in an UPDATE body, at most 65535 octets
			return err
		}
	}
	if cursor.Bad() {
		return errors.Join(policyview.ErrMalformed, fmt.Errorf("the %s NLRI does not frame", fam))
	}
	return nil
}

// flowSpecSection judges the destination prefix of each FlowSpec rule. A
// rule is never rewritten into a prefix, so a rejected or absent destination
// rejects the whole UPDATE (nlrisplit.FlowSpecDestination).
func (w *matchWalk) flowSpecSection(fam family.Family, nlri []byte, addPath bool) error {
	var judgeErr error
	_, err := nlrisplit.SplitFlowSpec(nlri, addPath, func(one []byte) bool {
		if addPath {
			one = one[4:]
		}
		destination := nlrisplit.FlowSpecDestination(fam, one)
		if !destination.IsValid() {
			w.whole = true
			return true
		}
		w.route.Prefix = destination
		rejects, err := w.e.match.Rejects(w.route)
		if err != nil {
			judgeErr = err
			return false
		}
		if rejects {
			w.rejected++
			w.whole = true
			return true
		}
		w.accepted++
		return true
	})
	if judgeErr != nil {
		return judgeErr
	}
	if err != nil {
		w.whole = true // a rule that does not frame has no destination to judge
	}
	return nil
}

// cidrPrefix answers the prefix of bits bits that octets carries, its
// trailing bits masked off; false when bits exceeds the address. For a CIDR
// entry (Length, then the prefix octets) bits is Length and octets follow it;
// for a labeled entry both are read past the label field.
//
//	+0 Length(1) +1 Prefix(ceil(Length/8))
func cidrPrefix(bits int, octets []byte, is4 bool) (netip.Prefix, bool) {
	var addr [16]byte
	if is4 {
		if bits > 32 {
			return netip.Prefix{}, false
		}
		copy(addr[:4], octets)
		return netip.PrefixFrom(netip.AddrFrom4([4]byte(addr[:4])), bits).Masked(), true
	}
	if bits > 128 {
		return netip.Prefix{}, false
	}
	copy(addr[:], octets)
	return netip.PrefixFrom(netip.AddrFrom16(addr), bits).Masked(), true
}
