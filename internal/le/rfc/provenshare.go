// Design: docs/contributing/rfc-conformance-gates.md -- the ONE published
// answer to "how much of what Ze implements is proven by test". Three surfaces
// state that number, the site home page, /quality/health/ and
// /quality/rfc-compliance/, and they state the same one because each reads this.
// Related: profilecoverage.go holds the per-RFC verified-record figures this
// sums; meta.go holds the enrolment, implementation kind and public row that
// decide which RFCs are counted at all.
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
	// Proven counts gated requirements a recorded break proves: a
	// discrimination record that still verifies against the tree, on a
	// summary whose requirement list carries a valid extraction sign-off. It
	// is the sum of StemFigures.Mandatory.Proven over the walked RFCs Ze
	// implements; a tag alone, an annotation alone or a stale record proves
	// nothing.
	Proven int
	// Gated counts every gated requirement of the walked RFCs Ze implements.
	// An unwalked RFC's rows are out of it: its list is unbounded, so a
	// denominator holding it would measure the list, not the software
	// (ai/rules/rfc-compliance.md). A not-applicable requirement STAYS in this denominator, and so
	// does a lower-layer one: that requirement is MET, by a layer under Ze,
	// and it is not proven BY ZE, so it belongs in neither the numerator nor a
	// smaller denominator. Annotating one may not move this share by a point.
	// A {rollup} row is the one exception, and it is out of both: it carries
	// no obligation of its own.
	Gated int
	// NotWalked is how many of the RFCs Ze implements carry no valid
	// extraction sign-off. They add nothing to Gated or Proven, exactly as a
	// profile rollup sums its walked members and states how many are not
	// walked (docs/contributing/rfc-conformance-gates.md, "Coverage
	// figures"). Every surface that prints the share prints this count beside
	// it, because the share bounds only the walked part.
	NotWalked int
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

// ProvenShareOf sums the proof standing over the RFCs Ze implements, from the
// per-RFC figures StemFiguresOf answers.
//
// What leaves the denominator is a whole RFC Implements says Ze does not
// implement. A not-applicable REQUIREMENT stays in (owner, 2026-09-02): the
// owner ruling of 2026-08-31 presumes `binds-another-role` wrong and calls a
// requirement met through a lower layer MET and owing a test, so the excluded
// set is the one least entitled to count in Ze's favor.
//
// The numerator is the verified-record figure and nothing else (owner,
// 2026-10-30): a requirement counts as proven only when a recorded break
// proved its test goes red. A single-polarity row counts only when its one
// side carries such a record. An unwalked RFC adds to neither side and is
// counted in NotWalked instead (rollupOf), so the share is a figure over
// walked lists only and says how many RFCs it leaves out.
//
// An RFC must be enrolled to count either way, so obligations no gate checks
// never reach a published denominator.
func ProvenShareOf(metas map[string]Meta, figures map[string]StemFigures) (ProvenShare, error) {
	var out ProvenShare
	var implemented []string
	for stem := range metas {
		meta := metas[stem]
		if !meta.Enrolled() {
			continue
		}
		one, held := figures[stem]
		if !held {
			var tb textbuf.Buffer
			return ProvenShare{}, errors.New(tb.Str("rfc/short/").Str(stem).
				Str(".md is enrolled but has no figure, so the published proof share would omit it: " +
					"answer the figures with rfc.StemFiguresOf over the same corpus as the Meta tables").String())
		}
		out.Inspected++
		out.GatedInspected += one.Mandatory.Total
		if Implements(&meta) {
			implemented = append(implemented, stem)
		}
	}
	// The documented rollup rule (docs/contributing/rfc-conformance-gates.md,
	// "Coverage figures"): only walked members contribute rows, and the rest
	// are counted. rollupOf is that rule, read here rather than restated.
	rollup := rollupOf(figures, implemented)
	out.RFCs = rollup.RFCs
	out.NotWalked = rollup.NotWalked
	out.Gated = rollup.Mandatory.Total
	out.Proven = rollup.Mandatory.Proven
	if out.Gated == 0 {
		return ProvenShare{}, errors.New(
			"no walked, enrolled RFC with a public row and an Implementation of `ze` or `mixed` declares a " +
				"gated requirement, so the published proof share would divide by zero: check that " +
				"rfc/short/*.md still carry `| Enrolment | enrolled |`, `| Implementation | ze |` and " +
				"`| Support | <aggregate> |` rows and their MUST-level requirements, that " +
				"at least one carries a valid sign-off in rfc/extraction/<stem>.json (`./le rfc extraction create stem <stem>`), " +
				"then re-run `./le rfc check`")
	}
	return out, nil
}

// Implements is the ONE definition of "an RFC Ze implements", and every
// published population taken over that set reads it.
//
// Two things have to hold (owner, 2026-10-30). The summary is ENROLLED, which
// already requires an Implementation kind whose obligations count against Ze's
// own Go, `ze` or `mixed` (implementationCounts), so a gate runs over its
// obligations. And it renders a public row. An enrolled RFC whose feature is
// still to come counts at its real figure: the proof it lacks is what the
// share is for, and availability is a feature-level fact (features/<id>.md
// Scope), never a reason to leave this population.
//
// The proof share here, the satisfaction buckets the RFC compliance page
// partitions, and the home page card each call this rather than restating
// it: a second spelling of the predicate is a future disagreement with nothing
// to arbitrate it (ai/rules/principles.md).
func Implements(meta *Meta) bool {
	if !meta.Enrolled() {
		return false
	}
	return meta.HasRow()
}
