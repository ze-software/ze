// Design: docs/architecture/README.md -- the page format: a decision row changes only with Thomas
// Overview: decisions.go -- the section parser and the row judgment
// Related: register.go -- registers judgeDecisionRows as a change-set check
//
// decisionschange.go judges what a commit does to the Design decisions
// sections of the pages it carries: a row it adds is judged as the tree check
// judges a row, and a row present at the parent that it changes or removes is
// refused unless Thomas admitted it, through an approved `departs` row of a
// spec or through `./le commit create ... decision-change "<his words>"`
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-9, AC-10).
// A row the commit leaves alone is never judged, whatever it holds.
package docconsistency

import (
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// DecisionChangedTrailer is `Decision-changed: <page>: <owner words>`, the
// trailer `./le commit create ... decision-change "<owner words>"` writes for
// each page whose decision rows the commit changes or removes. The commit
// package declares it (declaredTrailers), so a hand-written one is refused.
const DecisionChangedTrailer = "Decision-changed: "

// DepartureApproved answers whether an approved `departs` row of a spec names
// page and the decision row as it stood at the parent of the commit. An error
// is a lookup that could not answer, never "not approved".
type DepartureApproved func(root, page string, row DecisionRow) (bool, error)

var (
	departureMu       sync.Mutex
	departureApproved DepartureApproved
)

// RegisterDepartureLookup installs the one approval lookup this check asks,
// from the init() of the package that owns the approval records
// (internal/le/spec/design). It MUST be called at most once; a nil lookup or a
// second one is a Ze defect and panics.
func RegisterDepartureLookup(lookup DepartureApproved) {
	if lookup == nil {
		panic("BUG: docconsistency.RegisterDepartureLookup: a nil lookup")
	}
	departureMu.Lock()
	defer departureMu.Unlock()
	if departureApproved != nil {
		panic("BUG: docconsistency.RegisterDepartureLookup: a second lookup")
	}
	departureApproved = lookup
}

// approvedDeparture asks the registered lookup. Safe for concurrent use.
func approvedDeparture(root, page string, row DecisionRow) (bool, error) {
	departureMu.Lock()
	lookup := departureApproved
	departureMu.Unlock()
	// Guard: no registered lookup means this binary links no approval reader,
	// so no approval can be shown, and the change is refused. This "false" is
	// the fail-closed answer, never a lookup that ran.
	if lookup == nil {
		return false, nil
	}
	return lookup(root, page, row)
}

// LostDecision is a decision row the parent holds that the commit changes or
// removes. Changed is true when the commit adds a row of the same Date, which
// reads as the decision rewritten rather than dropped.
type LostDecision struct {
	Page    string
	Row     DecisionRow
	Changed bool
}

// pageSides is one carried page's section on both sides of the commit.
type pageSides struct {
	page          string
	before, after DecisionsSection
}

// carriedSections reads the section of every markdown page the commit
// carries or removes, on both sides, and keeps the pages where either side
// holds one. A read that fails is an error: the check could not judge.
func carriedSections(change changecheck.Change) ([]pageSides, error) {
	var sides []pageSides
	for _, page := range slices.Concat(change.Paths, change.Removed) {
		if !strings.HasSuffix(page, ".md") {
			continue
		}
		before, _, err := change.Before(page)
		if err != nil {
			return nil, err
		}
		after := ""
		if !slices.Contains(change.Removed, page) {
			after, _, err = change.After(page)
			if err != nil {
				return nil, err
			}
		}
		side := pageSides{page: page, before: ParseDecisions(before), after: ParseDecisions(after)}
		if !side.before.Present() && !side.after.Present() {
			continue
		}
		sides = append(sides, side)
	}
	return sides, nil
}

// compareRows answers the rows before holds that after does not (lost) and
// the rows after holds that before does not (added), each row counted by its
// identity, so a reordering loses and adds nothing.
func compareRows(before, after DecisionsSection) (lost, added []DecisionRow) {
	held := map[string]int{}
	for _, row := range after.Rows {
		held[row.key()]++
	}
	for _, row := range before.Rows {
		if held[row.key()] > 0 {
			held[row.key()]--
			continue
		}
		lost = append(lost, row)
	}
	had := map[string]int{}
	for _, row := range before.Rows {
		had[row.key()]++
	}
	for _, row := range after.Rows {
		if had[row.key()] > 0 {
			had[row.key()]--
			continue
		}
		added = append(added, row)
	}
	return lost, added
}

// lostDecisions answers the rows one page loses, each marked changed when an
// added row carries its Date.
func lostDecisions(side *pageSides) (lost []LostDecision, added []DecisionRow) {
	lostRows, added := compareRows(side.before, side.after)
	for _, row := range lostRows {
		changed := slices.ContainsFunc(added, func(candidate DecisionRow) bool { return candidate.Date == row.Date })
		lost = append(lost, LostDecision{Page: side.page, Row: row, Changed: changed})
	}
	return lost, added
}

// LostDecisions answers every decision row the parent holds that change
// changes or removes, over every page it carries. The commit package writes
// one Decision-changed: trailer per page it names.
func LostDecisions(change changecheck.Change) ([]LostDecision, error) {
	sides, err := carriedSections(change)
	if err != nil {
		return nil, err
	}
	var lost []LostDecision
	for i := range sides {
		pageLost, _ := lostDecisions(&sides[i])
		lost = append(lost, pageLost...)
	}
	return lost, nil
}

// ze point: documentation/directives/write-an-owner-decision-into-the-design-decisions-section-of-its-page
// judgeDecisionRows answers the refusal of a commit that adds a malformed
// decision row or a section shape problem its parent did not hold, or that
// changes or removes a row its parent holds without Thomas's admission:
// change.OwnerWords from his terminal, or an approved departure.
func judgeDecisionRows(change changecheck.Change) (string, error) {
	sides, err := carriedSections(change)
	if err != nil {
		return "", err
	}
	var malformed, unadmitted []string
	for i := range sides {
		side := &sides[i]
		for _, problem := range addedShapeProblems(side) {
			malformed = append(malformed, side.page+": the section "+problem.Text)
		}
		lost, added := lostDecisions(side)
		for _, row := range added {
			for _, problem := range JudgeDecisionRow(change.Root, row) {
				malformed = append(malformed, side.page+": "+problem.Text)
			}
		}
		for _, decision := range lost {
			admitted, err := decisionAdmitted(change, decision)
			if err != nil {
				return "", err
			}
			if admitted {
				continue
			}
			unadmitted = append(unadmitted, lostText(decision))
		}
	}
	return decisionRefusal(malformed, unadmitted), nil
}

// addedShapeProblems answers the shape problems the commit's side of a page
// holds that its parent's side did not, each counted by its text.
func addedShapeProblems(side *pageSides) []DecisionProblem {
	had := map[string]int{}
	for _, problem := range side.before.Problems {
		had[problem.Text]++
	}
	var added []DecisionProblem
	for _, problem := range side.after.Problems {
		if had[problem.Text] > 0 {
			had[problem.Text]--
			continue
		}
		added = append(added, problem)
	}
	return added
}

// decisionAdmitted answers whether Thomas admitted the loss of one row: his
// words from the owner terminal admit every row the commit changes, and an
// approved departure admits the row it names.
func decisionAdmitted(change changecheck.Change, decision LostDecision) (bool, error) {
	if strings.TrimSpace(change.OwnerWords) != "" {
		return true, nil
	}
	approved, err := approvedDeparture(change.Root, decision.Page, decision.Row)
	if err != nil {
		return false, errors.Join(errors.New("the approval of a departure from "+decision.Page+" could not be read"), err)
	}
	return approved, nil
}

// lostText names one changed or removed row: the page, its date and its
// decision.
func lostText(decision LostDecision) string {
	verb := "removed"
	if decision.Changed {
		verb = "changed"
	}
	var tb textbuf.Buffer
	tb.Str(decision.Page).Str(": the decision of ").Str(decision.Row.Date).Str(" is ").Str(verb).
		Str(": ").Quoted(decision.Row.Decision)
	return tb.String()
}

// decisionRefusal renders the refusal, or "" when nothing is refused. Each
// kind of problem carries its why and its fix once.
func decisionRefusal(malformed, unadmitted []string) string {
	var tb textbuf.Buffer
	for _, line := range malformed {
		tb.Str(line).Byte('\n')
	}
	if len(malformed) != 0 {
		tb.Str("  why: a decision row records what Thomas decided, and a row that does not resolve records nothing a reader can check\n").
			Str("  fix: correct each named row (").Str(pageFormat).Str("), then run ./le doc consistency\n")
	}
	for _, line := range unadmitted {
		tb.Str(line).Byte('\n')
	}
	if len(unadmitted) != 0 {
		tb.Str("  why: a decision row holds what Thomas decided, and it changes only with him\n").
			Str("  fix: record the departure in the spec's ## Design documents and have Thomas approve it ").
			Str(`(./le spec design approve spec <spec-path> words "<what he said>"),` + "\n").
			Str(`       or, outside a spec, give Thomas this commit command with decision-change "<what he said>" to run from his terminal` + "\n")
	}
	return strings.TrimSuffix(tb.String(), "\n")
}
