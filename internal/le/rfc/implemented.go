// Design: docs/contributing/rfc-conformance-gates.md -- the section counts of the RFCs Ze implements, the one
// answer the site home page and /quality/health/ state
// Related: sectioncounts.go holds the per-RFC section split this sums;
// meta.go holds the enrolment, implementation kind and public row that
// decide which RFCs are counted at all.
package rfc

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// DraftSections holds the separate conformance section counts for implemented
// Internet-Drafts (`draft-*`). Internet-Drafts are not RFCs and are kept out
// of RFC conformance figures and release targets until published as RFCs.
type DraftSections struct {
	Sections      Sections
	NotWalked     int
	Drafts        int
	Inspected     int
	MustInspected int
}

// ProvenPercent renders the proven MUST rows of implemented Internet-Drafts
// over the MUST rows those drafts owe, to one decimal place without its sign.
func (d *DraftSections) ProvenPercent() (string, error) {
	if d.Sections.Must.Kept == 0 {
		return "", errors.New("no walked, enrolled Internet-Draft declares a kept MUST row, " +
			"so the draft proven percentage would divide by zero")
	}
	var tb textbuf.Buffer
	return tb.Float(100*float64(d.Sections.Must.Proven())/float64(d.Sections.Must.Kept), 1).String(), nil
}

// ProvenBuiltPercent renders the proven MUST rows of implemented
// Internet-Drafts over only the MUST rows of what Ze builds for those drafts,
// to one decimal place without its sign.
func (d *DraftSections) ProvenBuiltPercent() (string, error) {
	built := d.Sections.Must.Built()
	if built == 0 {
		return "", errors.New("no walked, enrolled Internet-Draft declares a built MUST row, " +
			"so the draft proven-built percentage would divide by zero")
	}
	var tb textbuf.Buffer
	return tb.Float(100*float64(d.Sections.Must.Proven())/float64(built), 1).String(), nil
}

// ImplementedSections is the section counts of the RFCs Ze implements, with
// the wider population they are drawn from and the separate Internet-Draft
// counts beside them.
//
// Every count carries its denominator (owner directive, 2026-09-01): a reader
// gets the proven rows (Ze proven and OS verified) of the MUST rows Ze and the
// OS owe, and Inspected RFCs with
// MustInspected MUST rows for everything the ledger has judged. Internet-Drafts
// (`draft-*`) are counted separately in Drafts rather than mixed into RFC
// conformance figures.
type ImplementedSections struct {
	// Sections sums SectionsOf over the walked non-draft RFCs Ze implements. An
	// unwalked RFC's rows are out of it: its list is unbounded, so a section
	// holding it would measure the list, not the software
	// (ai/rules/rfc-conformance.md).
	Sections Sections
	// NotWalked is how many of the non-draft RFCs Ze implements carry no valid
	// extraction sign-off. They add nothing to Sections, exactly as a profile
	// rollup sums its walked members and states how many are not walked.
	// Every surface that prints the sections prints this count beside them.
	NotWalked int
	// RFCs is how many enrolled non-draft RFCs Ze implements, and Inspected is
	// how many non-draft RFCs it has enrolled at all. MustInspected is the MUST
	// row count across that wider non-draft set, walked or not.
	RFCs          int
	Inspected     int
	MustInspected int
	// Drafts holds the separate section counts for enrolled Internet-Drafts
	// (`draft-*`), kept out of RFC conformance figures and release targets.
	Drafts DraftSections
}

// ProvenPercent renders the proven MUST rows (Sections.Must.Proven: Ze proven
// plus OS verified) over the MUST rows Ze and the OS owe (Sections.Must.Kept),
// to one decimal place, without its sign.
//
// A row not owed (not applicable, declined, OS blocked) is in neither side:
// it does not bind Ze, so it is no part of what Ze could prove. A row the OS
// performs is owed by the stack, so it is always in the denominator, and it is
// in the numerator once its record proves it (OS verified): a requirement the
// OS meets counts as done once its test proves it (owner, 2026-11-13). An OS
// testing gap, which covers a row on the no-break or kernel-producer escape,
// stays out of the numerator.
//
// It carries no guard for an empty population because ImplementedSectionsOf
// refuses to answer with one, so a caller cannot hold one whose Kept is zero.
func (s *ImplementedSections) ProvenPercent() string {
	var tb textbuf.Buffer
	return tb.Float(100*float64(s.Sections.Must.Proven())/float64(s.Sections.Must.Kept), 1).String()
}

// ProvenBuiltPercent renders the same proven MUST rows as ProvenPercent over
// only the MUST rows of what Ze builds (Sections.Must.Built: Kept less the not
// built rows), to one decimal place, without its sign.
//
// A not built row is one the summary or the audit says Ze does not meet
// because that part of the RFC is not built yet. It needs no test until the
// part is built, so this share shows how well the features Ze ships are
// tested, while ProvenPercent counts every MUST row Ze and the OS will owe
// (owner, 2026-11-23). A not built row is never proven, so the numerator is
// the same on both figures and only the denominator differs.
//
// It refuses when every owed row is not built: ImplementedSectionsOf only
// guarantees that Kept is not zero, and a share of no built row would divide
// by zero.
func (s *ImplementedSections) ProvenBuiltPercent() (string, error) {
	built := s.Sections.Must.Built()
	if built == 0 {
		return "", errors.New("every MUST row Ze or the OS owes in the walked RFCs Ze implements is not built, " +
			"so the proven share of what Ze builds would divide by zero: check the `not built` rows of " +
			"rfc/short/*.md and their test reviews, then re-run `./le rfc check`")
	}
	var tb textbuf.Buffer
	return tb.Float(100*float64(s.Sections.Must.Proven())/float64(built), 1).String(), nil
}

// ImplementedSectionsOf sums the section counts over the RFCs Ze implements,
// from the per-RFC figures StemFiguresOf answers, keeping Internet-Drafts
// (`draft-*`) separate in Drafts.
//
// What leaves the RFC population is a whole document Implements says Ze does
// not implement, an Internet-Draft (counted separately in Drafts), and an
// unwalked RFC, counted in NotWalked instead. A document must be enrolled to
// count either way, so obligations no gate checks never reach a published count.
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
		if IsDraft(stem) {
			if err := addDraftSection(&out.Drafts, stem, &meta, &one); err != nil {
				return ImplementedSections{}, err
			}
			continue
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

// addDraftSection accumulates one enrolled Internet-Draft into Drafts so draft
// conformance is tracked separately from RFC conformance.
func addDraftSection(drafts *DraftSections, stem string, meta *Meta, one *StemFigures) error {
	drafts.Inspected++
	drafts.MustInspected += one.Mandatory.Total
	if !Implements(meta) {
		return nil
	}
	drafts.Drafts++
	if !one.Walked {
		drafts.NotWalked++
		return nil
	}
	sections, err := SectionsOf(stem, one)
	if err != nil {
		return err
	}
	drafts.Sections.add(&sections)
	return nil
}

// IsDraft reports whether stem names an IETF Internet-Draft (`draft-*`) rather
// than a published RFC. Internet-Drafts are not RFCs and are counted separately
// from RFC conformance figures, targets, and release-profile obligations until
// published as RFCs.
func IsDraft(stem string) bool {
	return strings.HasPrefix(stem, "draft-")
}

// ImplementsRFC reports whether stem is an implemented non-draft RFC counted
// in RFC conformance figures: Implements(meta) and not an Internet-Draft.
func ImplementsRFC(stem string, meta *Meta) bool {
	if IsDraft(stem) {
		return false
	}
	return Implements(meta)
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
