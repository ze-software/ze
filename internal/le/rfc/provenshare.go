// Design: the ONE published answer to "how much of what Ze implements is proven
// by test". Three surfaces state that number -- the site home page,
// /quality/health/ and /quality/rfc-compliance/ -- and they state the same one
// because each reads this.
// Related: coverage.go holds the per-RFC partition this sums; meta.go holds the
// public row whose status decides which RFCs are counted at all.
package rfc

import (
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ProvenShare is the proof standing of the RFCs Ze implements, with the wider
// population it is drawn from beside it.
//
// Every count carries its denominator (owner directive, 2026-09-01), and this
// type is what makes that possible on a page: a reader gets Proven of Gated for
// what Ze implements, and Inspected RFCs with GatedInspected obligations for
// everything the ledger has judged. An absolute number alone reads as an
// achievement, because "3,264 gated MUSTs" counts obligations JUDGED and is
// read as obligations MET.
type ProvenShare struct {
	// Proven counts gated requirements carrying a passing tagged test: both
	// polarities, or one polarity whose annotation records why the other side
	// has no input to drive it.
	Proven int
	// Gated counts every gated requirement of the RFCs Ze implements. A
	// not-applicable requirement STAYS in this denominator.
	Gated int
	// RFCs is how many enrolled RFCs Ze implements, and Inspected is how many
	// it has enrolled at all. GatedInspected is the obligation count across
	// that wider set.
	RFCs           int
	Inspected      int
	GatedInspected int
}

// Percent renders Proven over Gated to one decimal place, without its sign.
//
// It carries no guard for an empty population because ProvenShareOf refuses to
// answer with one, so a caller cannot hold a ProvenShare whose Gated is zero.
func (p ProvenShare) Percent() string {
	var tb textbuf.Buffer
	return tb.Float(100*float64(p.Proven)/float64(p.Gated), 1).String()
}

// ProvenShareOf sums the proof standing over the RFCs Ze implements.
//
// Two decisions are load-bearing, and both were the owner's on 2026-09-02.
//
// What leaves the denominator is a whole RFC Ze does not implement, declared as
// such by its own public row. A not-applicable REQUIREMENT stays in. Excluding
// those was published until this date and it let the annotations raise the
// score: the owner ruling of 2026-08-31 presumes `binds-another-role` wrong and
// calls a requirement met through a lower layer MET and owing a test, so the
// excluded set is the one least entitled to count in Ze's favor. Whole-RFC
// scope is a decision a reader can check on the status page; a per-requirement
// exclusion is 915 judgements a reader cannot.
//
// The numerator counts a single-polarity requirement beside a both-polarity
// one. Both carry a passing tagged test bound to the requirement id. Only the
// second is one-sided, and its annotation states why no input exists to drive
// the other side.
//
// An RFC must be enrolled to count either way. Requirement.Gated reads the
// LEVEL, so an un-enrolled summary carrying MUST rows still yields a coverage
// row, and counting it would put obligations no gate checks into a published
// denominator.
func ProvenShareOf(metas map[string]Meta, requirements []Requirement, tags []Tag,
	carriers []Carrier,
) (ProvenShare, error) {
	singles := singlePolarityCounts(requirements)

	var out ProvenShare
	for _, row := range CoverageRows(requirements, tags, carriers) {
		meta, held := metas[row.RFC]
		if !held || !meta.Enrolled() {
			continue
		}
		out.Inspected++
		out.GatedInspected += row.Gated
		if !Implements(meta) {
			continue
		}
		out.RFCs++
		out.Gated += row.Gated
		out.Proven += row.Both + singles[row.RFC]
	}
	if out.Gated == 0 {
		return ProvenShare{}, errors.New(
			"no enrolled RFC declares a public row claiming support, so the published proof " +
				"share would divide by zero: check that rfc/short/*.md still carry their " +
				"`| Support status |` cells, and re-run `./le rfc index-update`")
	}
	return out, nil
}

// Implements is the ONE definition of "an RFC Ze implements", and every
// published population taken over that set reads it.
//
// Three things have to hold. The summary is ENROLLED, so a gate runs over its
// obligations. It renders a public row, so it makes a claim at all. And that
// row does not say Unsupported or Future, which are the two ways a row says Ze
// does not implement the document.
//
// Holding Ze to the obligations of a document it does not implement measures a
// decision rather than a defect, so those RFCs leave every share taken over
// this set. The proof share here, the satisfaction buckets the RFC compliance
// page partitions, and the home page card each call this rather than restating
// it: a second spelling of the predicate is a future disagreement with nothing
// to arbitrate it (ai/rules/principles.md).
func Implements(meta Meta) bool {
	if !meta.Enrolled() {
		return false
	}
	if !meta.HasRow() {
		return false
	}
	return statusIsSupportClaim(meta.Status)
}

// singlePolarityCounts counts, for each RFC, the gated requirements whose
// annotation records a passing test on one side and no input for the other.
func singlePolarityCounts(requirements []Requirement) map[string]int {
	out := map[string]int{}
	for _, requirement := range requirements {
		if !requirement.Gated() || requirement.Annotation == nil {
			continue
		}
		if requirement.Annotation.Kind != AnnotationSinglePolarity {
			continue
		}
		out[requirement.RFC]++
	}
	return out
}
