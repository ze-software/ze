// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that shares one ledger parse
//
// check_status.go cross-checks the public ledger's rows and the summaries'
// dispositions with the authored requirement records.
package rfc

import (
	"regexp"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	nonApplicabilityRE = regexp.MustCompile(`(?i)\b(?:not\s+applicable\s+to\s+ze|does\s+not\s+apply\s+to\s+ze|ze\s+(?:does\s+not|doesn't|never)\b|ze\s+has\s+no\b|we\s+(?:do\s+not|don't)\s+(?:implement|support|need)|out\s+of\s+scope\s+for\s+ze|no(?:t)?\s+relevant\s+to\s+ze)`)
	documentCategoryRE = regexp.MustCompile(`(?i)\bRFC\s*2119\b|\bRFC\s*8174\b|\bBCP\s*14\b|\bkey[-\s]?words?\b|\bInformational\b|\bExperimental\b|\bHistoric(?:al)?\b|\bBest\s+Current\s+Practice\b`)
)

// checkStatusAgreement refuses an unmet requirement on an enrolled RFC that
// renders no public row. The row's Unmet cell is derived from the annotations,
// so a row discloses every unmet requirement by construction; an RFC with no
// row discloses none of them.
func checkStatusAgreement(requirements []Requirement, rows map[string]LedgerRow,
	enrolled map[string]bool) []string {
	var errs []string
	for _, req := range requirements {
		if req.Annotation == nil {
			continue
		}
		traits, known := AnnotationTraitsOf(req.Annotation.Kind)
		if !known {
			errs = append(errs, requirementWhere(req)+": unknown annotation kind "+req.Annotation.Kind)
			continue
		}
		if !traits.Unmet {
			continue
		}
		if _, held := rows[req.RFC]; held {
			continue
		}
		if !enrolled[req.RFC] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(req.RID).Str(" is annotated {").Str(req.Annotation.Kind).Str("} but ").Str(req.RFC).
			Str(" has no row in docs/features/rfc-status.md; the public ledger must disclose it").String())
	}
	return errs
}

func nonNormativeReasonCitesDocument(reason string) bool {
	return documentCategoryRE.MatchString(reason) || siteKeywordRE.MatchString(reason)
}

// checkSummaryDisposition judges the ONE claim an un-enrolled summary makes
// about conformance.
//
// Three refusals lived here until 2026-09-01 and none of them can be written
// any more: a summary in neither ledger file, a stem in both, and a disposition
// naming a summary that does not exist. Each compared two copies of one fact.
// The fact is now declared once, in the summary's own `## Meta` table, so a
// summary that declares no enrolment does not parse, one field cannot hold two
// values, and a disposition dies with the file that carries it. Deleting a copy
// retires a check without weakening anything.
//
// What survives states a property of ONE document, and moves unchanged.
func checkSummaryDisposition(tree string, metas map[string]Meta, requirements []Requirement) []string {
	gated := gatedCounts(requirements)
	var errs []string
	for _, stem := range sortMetaStems(metas) {
		meta := metas[stem]
		var where textbuf.Buffer
		at := where.Str(summaryRel).Byte('/').Str(stem).Str(".md: ").String()
		if meta.Enrolment == dispositionSourceRestricted {
			errs = append(errs, checkSourceRestricted(at, meta.EnrolmentReason, stem, tree)...)
			continue
		}
		if meta.Enrolment == dispositionOutOfScope {
			errs = append(errs, checkOutOfScope(at, &meta, gated[stem])...)
			continue
		}
		if meta.Enrolment != dispositionNonNormative {
			continue
		}
		if nonApplicabilityRE.MatchString(meta.EnrolmentReason) {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(at).
				Str("is declared non-normative with a reason that judges what ZE owes rather than what the DOCUMENT states: ").Str(pyRepr(truncateRunes(meta.EnrolmentReason, 80))).
				Str(". 'non-normative' means the RFC imposes no MUST-level obligation on any speaker. Whether an obligation applies to Ze is a conformance judgement (ai/rules/rfc-compliance.md reserves it to the owner) -- record 'backlog' or 'blocked' instead").String())
			continue
		}
		if nonNormativeReasonCitesDocument(meta.EnrolmentReason) {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(at).
			Str("is declared non-normative with a reason that cites nothing about the DOCUMENT: ").Str(pyRepr(truncateRunes(meta.EnrolmentReason, 80))).
			Str(". 'non-normative' is the one kind that makes a claim about conformance, so its reason must rest on something a reviewer can check in the text: the RFC's IETF category (Informational, Experimental, Historic, Best Current Practice), the presence or absence of the RFC 2119 / RFC 8174 / BCP 14 key-words machinery, or the result of a capitalised MUST/SHALL/REQUIRED scan over the source. A reason that cites none of those cannot be checked or contradicted -- record 'backlog' or 'blocked' instead").String())
	}
	return errs
}

// restrictedReasonRE is what a `source-restricted` reason must cite: the body
// that publishes the standard, or the license that stops it being copied.
//
// Positive rather than a blacklist, for the reason the non-normative citation
// requirement is positive. A reason that cites none of these cannot be checked
// and cannot be contradicted, which is exactly what a reason taking an RFC out
// of the gated population must not be.
var restrictedReasonRE = regexp.MustCompile(`(?i)\bISO\b|\bIEC\b|\bITU\b|\bIEEE\b|\bANSI\b|\bETSI\b|\bcopyright\b|\blicen[cs]e\b|\bpaywall|\bnot (?:freely )?redistribut|\bnot (?:publicly|freely) available`)

// checkSourceRestricted judges the reason a summary gives for declaring that
// its standard's text can never enter this repository.
//
// The kind takes the RFC out of the gated population, so its reason carries the same
// weight as a non-normative one and is held to the same discipline: it states a
// property of the DOCUMENT's availability, and it does not judge what Ze owes.
// Whether Ze must comply is a conformance judgement ai/rules/rfc-compliance.md
// reserves to the owner, and a reason phrased that way would launder an
// unextracted obligation into a decision.
func checkSourceRestricted(at, reason, stem, tree string) []string {
	if _, held := sourceKeywordCount(tree, stem); held {
		var tb textbuf.Buffer
		return []string{tb.Str(at).
			Str("is declared source-restricted, which says the standard's own text may not be ").
			Str("redistributed, but that text IS in this repository. The disposition takes the RFC ").
			Str("out of the gated population, so it may not be written over a source a reviewer can ").
			Str("open: extract the obligations, or record 'blocked' if something else stops the ").
			Str("enrolment").String()}
	}
	if nonApplicabilityRE.MatchString(reason) {
		var tb textbuf.Buffer
		return []string{tb.Str(at).
			Str("is declared source-restricted with a reason that judges what ZE owes rather than what stops the TEXT reaching this repository: ").
			Str(pyRepr(truncateRunes(reason, 80))).
			Str(". 'source-restricted' means the standard's own text may not be redistributed, so no checklist can ever be bounded against it. Whether an obligation applies to Ze is a separate judgement (ai/rules/rfc-compliance.md reserves it to the owner)").String()}
	}
	if restrictedReasonRE.MatchString(reason) {
		return nil
	}
	var tb textbuf.Buffer
	return []string{tb.Str(at).
		Str("is declared source-restricted with a reason that names nothing a reviewer can check: ").
		Str(pyRepr(truncateRunes(reason, 80))).
		Str(". This kind takes the RFC out of the gated population, so its reason must rest on the body that publishes the standard (ISO, IEC, ITU, IEEE, ANSI, ETSI) or on the license, copyright or paywall that stops the text being copied. Where the text IS fetchable, record 'blocked' and fetch it").String()}
}

// scopeDecisionRE is what an `out-of-scope` reason must carry: the date the
// decision was taken.
//
// A scope decision is the one kind whose truth rests on a person rather than on
// the document, so the only thing a reviewer can check is WHEN it was taken and
// therefore whether it is still current. A reason with no date cannot be aged.
var scopeDecisionRE = regexp.MustCompile(`\b20\d{2}-\d{2}-\d{2}\b`)

// checkOutOfScope refuses the two ways `out-of-scope` could become an escape.
//
// The first is an empty checklist, which records no declined obligation.
//
// The second is an undateable reason. Scope decisions are revisited; a gap
// recorded as out-of-scope in 2026 and never dated reads in 2030 as a decision
// somebody still stands behind.
func checkOutOfScope(at string, meta *Meta, gated int) []string {
	var errs []string
	if gated == 0 {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(at).Str("is declared out-of-scope over a checklist declaring ").
			Str("NO MUST-level requirement. This disposition's whole premise is that the extraction ").
			Str("is DONE and only the feature was declined, so an empty checklist under it records ").
			Str("nothing and bills nobody -- checkNewSummaries asks an out-of-scope summary for no ").
			Str("enrolment, which is right only where the obligations are written down. Extract the ").
			Str("RFC first (/ze-rfc), or record 'backlog', which says the extraction is owed").String())
	}
	if !scopeDecisionRE.MatchString(meta.EnrolmentReason) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(at).Str("is declared out-of-scope with a reason carrying no ").
			Str("date: ").Str(pyRepr(truncateRunes(meta.EnrolmentReason, 80))).
			Str(". This is the one disposition whose truth rests on a decision rather than on the ").
			Str("document, so the reason must say when it was taken (YYYY-MM-DD) and by whom. A ").
			Str("scope decision nobody can age reads forever as one somebody still stands behind").String())
	}
	return errs
}

// checkPublicRowMonotonic refuses a public row that DISAPPEARS while its RFC
// stays gated, and a newly gated RFC that arrives with none.
//
// This is checkStatusCompleteness under a new name and over a new source, and
// the rename is the whole lesson. The migration retired four refusals that
// compared two copies of one fact, and this one LOOKED like a fifth: a row is a
// summary's `Support` declaration now, so "the row is missing" and "the summary
// says there is no row" are the same sentence. They are not the same CHECK.
// Deleting a row is still one cell's edit, `Meta.HasRow` still goes false, the
// stem still leaves rowsFrom, and every ledger check downstream still stops
// seeing it -- including the one refusing an unmet requirement the public
// ledger does not disclose, which is exactly the refusal an author under
// pressure would want to silence.
//
// checkRetiredRequirements does not cover it. That ratchet compares requirement
// IDS between HEAD and the tree and never reads a Support cell, so a summary
// keeping every id while dropping its public row passes it untouched. Claiming
// otherwise was wrong, and an independent review caught the claim before the
// hole it described was reachable by accident.
//
// The baseline costs nothing: check() already holds HEAD's metas for the
// ratchets, so the comparison is rowsFrom over a map it read anyway.
func checkPublicRowMonotonic(metas, baseMetas map[string]Meta, baselineKnown bool,
	newly map[string]bool) []string {
	var errs []string
	for _, stem := range sortedSet(newly) {
		meta := metas[stem]
		if meta.HasRow() {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(summaryRel).Byte('/').Str(stem).
			Str(".md is newly enrolled and declares `| Support | - |`, so it renders no row on ").
			Str(statusRel).
			Str(". Enrolling gates its MUST-level requirements, so the public ledger must disclose the RFC: name its aggregate in `| Support | <aggregate> |` and write its `Support area`; the row's proof cells are derived. RFCs enrolled before this ratchet existed are grandfathered and unaffected").String())
	}
	if !baselineKnown {
		return errs
	}
	for _, stem := range sortMetaStems(metas) {
		meta := metas[stem]
		base, held := baseMetas[stem]
		// Keyed on the ROW alone, never on enrolment. A row publishes the
		// RFC's derived proof whether or not the RFC is enrolled, and
		// rfc/short/rfc9384.md is the live instance: `non-normative` with a
		// public row. An enrolled-only guard would let that row vanish with
		// no refusal.
		//
		// A summary DELETED outright is not reached here, because the walk is
		// over the summaries that still exist. Retiring an RFC is deleting
		// its file, which takes its obligations with it; retiring only its
		// public claim is what this refuses.
		if !held || !base.HasRow() || meta.HasRow() {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(summaryRel).Byte('/').Str(stem).
			Str(".md rendered a row on ").Str(statusRel).
			Str(" at ").Str(priorRevision).Str(" and declares `| Support | - |` now, while the summary is still here. Deleting the row retires a public claim without retiring the obligation behind it, and it takes the stem out of every check that reads the public page -- the guard refusing an undisclosed unmet requirement among them. Restore the `| Support | <aggregate> |` row. To retire the RFC itself, delete rfc/short/").String())
	}
	return errs
}
