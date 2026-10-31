// Design: docs/contributing/rfc-conformance-gates.md -- the section counts of the RFCs Ze implements, the one
// answer the site home page and /quality/health/ state
// Related: sectioncounts.go holds the per-RFC section split this sums;
// meta.go holds the enrolment, implementation kind and public row that
// decide which RFCs are counted at all.
package rfc

import (
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ImplementedSections is the section counts of the RFCs Ze implements, with
// the wider population they are drawn from beside them.
//
// Every count carries its denominator (owner directive, 2026-09-01): a reader
// gets Ze proven of the MUST rows Ze and the OS owe, and Inspected RFCs with
// MustInspected MUST rows for everything the ledger has judged.
type ImplementedSections struct {
	// Sections sums SectionsOf over the walked RFCs Ze implements. An
	// unwalked RFC's rows are out of it: its list is unbounded, so a section
	// holding it would measure the list, not the software
	// (ai/rules/rfc-compliance.md).
	Sections Sections
	// NotWalked is how many of the RFCs Ze implements carry no valid
	// extraction sign-off. They add nothing to Sections, exactly as a profile
	// rollup sums its walked members and states how many are not walked.
	// Every surface that prints the sections prints this count beside them.
	NotWalked int
	// RFCs is how many enrolled RFCs Ze implements, and Inspected is how many
	// it has enrolled at all. MustInspected is the MUST row count across that
	// wider set, walked or not.
	RFCs          int
	Inspected     int
	MustInspected int
}

// ProvenPercent renders the Ze-proven MUST rows over the MUST rows Ze and the
// OS owe (Sections.Must.Kept), to one decimal place, without its sign.
//
// A row not owed (not applicable, declined, OS blocked) is in neither side:
// it does not bind Ze, so it is no part of what Ze could prove. A row the OS
// performs stays in the denominator and never in the numerator, verified or
// not: it is owed by the stack, and Ze's own tests do not prove it.
//
// It carries no guard for an empty population because ImplementedSectionsOf
// refuses to answer with one, so a caller cannot hold one whose Kept is zero.
func (s *ImplementedSections) ProvenPercent() string {
	var tb textbuf.Buffer
	return tb.Float(100*float64(s.Sections.Must.ZeProven)/float64(s.Sections.Must.Kept), 1).String()
}

// ImplementedSectionsOf sums the section counts over the RFCs Ze implements,
// from the per-RFC figures StemFiguresOf answers.
//
// What leaves the population is a whole RFC Implements says Ze does not
// implement, and an unwalked RFC, counted in NotWalked instead. An RFC must be
// enrolled to count either way, so obligations no gate checks never reach a
// published count.
func ImplementedSectionsOf(metas map[string]Meta, figures map[string]StemFigures) (ImplementedSections, error) {
	var out ImplementedSections
	for stem := range metas {
		meta := metas[stem]
		if !meta.Enrolled() {
			continue
		}
		one, held := figures[stem]
		if !held {
			var tb textbuf.Buffer
			return ImplementedSections{}, errors.New(tb.Str("rfc/short/").Str(stem).
				Str(".md is enrolled but has no figure, so the published section counts would omit it: " +
					"answer the figures with rfc.StemFiguresOf over the same corpus as the Meta tables").String())
		}
		out.Inspected++
		out.MustInspected += one.Mandatory.Total
		if !Implements(&meta) {
			continue
		}
		out.RFCs++
		if !one.Walked {
			out.NotWalked++
			continue
		}
		sections, err := SectionsOf(stem, &one)
		if err != nil {
			return ImplementedSections{}, err
		}
		out.Sections.add(&sections)
	}
	if out.Sections.Must.Kept == 0 {
		return ImplementedSections{}, errors.New(
			"no walked, enrolled RFC with a public row and an Implementation of `ze` or `mixed` declares a " +
				"MUST row Ze or the OS owes, so the published proven percentage would divide by zero: check that " +
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
// counts are for, and availability is a feature-level fact (features/<id>.md
// Scope), never a reason to leave this population.
//
// ImplementedSectionsOf, the satisfaction buckets the RFC compliance page
// partitions, and the per-RFC `implemented` flag each call this rather than
// restating it: a second spelling of the predicate is a future disagreement
// with nothing to arbitrate it (ai/rules/principles.md).
func Implements(meta *Meta) bool {
	if !meta.Enrolled() {
		return false
	}
	return meta.HasRow()
}
