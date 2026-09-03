// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that shares one ledger parse
//
// check_status.go cross-checks the public support page with the authored
// requirement and extraction records.
package rfc

import (
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	noGapRE            = regexp.MustCompile(`(?i)no tracked gap|none claimed|no separate gap|explicitly unsupported`)
	nonApplicabilityRE = regexp.MustCompile(`(?i)\b(?:not\s+applicable\s+to\s+ze|does\s+not\s+apply\s+to\s+ze|ze\s+(?:does\s+not|doesn't|never)\b|ze\s+has\s+no\b|we\s+(?:do\s+not|don't)\s+(?:implement|support|need)|out\s+of\s+scope\s+for\s+ze|no(?:t)?\s+relevant\s+to\s+ze)`)
	documentCategoryRE = regexp.MustCompile(`(?i)\bRFC\s*2119\b|\bRFC\s*8174\b|\bBCP\s*14\b|\bkey[-\s]?words?\b|\bInformational\b|\bExperimental\b|\bHistoric(?:al)?\b|\bBest\s+Current\s+Practice\b`)
)

func rowDisclosesGap(row LedgerRow) bool {
	if !strings.HasPrefix(row.Status, "Supported") {
		return true
	}
	remaining := strings.TrimSpace(row.Remaining)
	return remaining != "" && !noGapRE.MatchString(remaining)
}

func checkStatusAgreement(requirements []Requirement, rows map[string]LedgerRow,
	enrolled map[string]bool) []string {
	var errs []string
	for _, req := range requirements {
		if req.Annotation == nil || req.Annotation.Kind != AnnotationGap {
			continue
		}
		row, held := rows[req.RFC]
		if !held {
			if !enrolled[req.RFC] {
				continue
			}
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(req.RID).Str(" is annotated {gap} but ").Str(req.RFC).
				Str(" has no row in docs/features/rfc-status.md; the public ledger must disclose it").String())
			continue
		}
		if rowDisclosesGap(row) {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(req.RID).Str(" is annotated {gap: ").Str(truncateRunes(req.Annotation.Reason, 50)).
			Str("} but its public row says ").Str(req.RFC).Str(" is '").Str(row.Status).
			Str("' with '").Str(truncateRunes(row.Remaining, 40)).Str("'. A known unmet MUST cannot be advertised as clean support -- correct `Support status` or `Support remaining` in the summary's own Meta table, then run ./le rfc index-update. A hand edit to docs/features/rfc-status.md is destroyed by the next run").String())
	}
	return errs
}

// supportClaimExcused answers the ONE disposition under which a public support
// claim rests on something other than an extracted checklist.
//
// `non-normative` says the DOCUMENT imposes no MUST-level obligation on any
// speaker, so an empty checklist is a property of the text rather than a hole
// in the extraction, and its reason must cite something a reviewer can check.
//
// Every other disposition excuses nothing, `source-restricted` included, and
// that one is worth saying out loud because it excused a claim until
// 2026-09-02. The argument for it was that a text nobody may redistribute can
// never bound a checklist, so demanding one is demanding the impossible. The
// argument against it is stronger and is the one this repository already
// makes everywhere else: being unable to prove a claim is a reason to stop
// making the claim, not a reason to be excused from proving it. A standard Ze
// implements and cannot bound is disclosed by a Status that says so, which is
// what `Unsupported`, `Future` and a qualified `Partial` are for.
//
// It also had no users, which is how the hazard was found. The one summary it
// was built for was deleted the same day, because the project's answer to a
// restricted standard turned out to be an independent reconstruction held
// outside the repository rather than an exemption inside it.
func supportClaimExcused(kind string) bool {
	return kind == dispositionNonNormative
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
			errs = append(errs, checkOutOfScope(at, meta, gated[stem])...)
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

func statusIsSupportClaim(status string) bool {
	status = strings.TrimSpace(status)
	return status != "Unsupported" && status != "Future"
}

func derivedRegisters(deriver *Deriver, signed map[string]Extraction,
	requirements []Requirement) (map[string]string, error) {
	gated := gatedCounts(requirements)
	out := map[string]string{}
	for _, stem := range sortedKeysOf(signed) {
		inventory, err := deriver.Inventory(stem, gated[stem])
		if err != nil {
			return nil, err
		}
		if inventory != nil {
			out[stem] = inventory.Register
		}
	}
	return out, nil
}

// checkUnprovenSupport refuses a public support claim that nothing behind it
// can contradict, in the two shapes that state takes.
//
// The first is an EMPTY checklist: a claim over a summary declaring no
// MUST-level requirement at all. The second is a checklist nothing PASSES: a
// promise of conformance over gated requirements of which not one carries a
// test in both polarities. Both publish a claim no evidence in this repository
// bears on, and the second was invisible here until 2026-09-02 because the walk
// skipped every stem with a gated count.
//
// The two arms carry different populations on purpose, and the escapes belong
// to the first arm alone. `non-normative` and a manual-walk sign-off each state
// that the DOCUMENT imposes no MUST, which answers the empty-checklist question
// and says nothing about whether Ze meets an obligation that exists.
func checkUnprovenSupport(requirements []Requirement, rows map[string]LedgerRow,
	stems map[string]bool, dispositions map[string]Disposition, signed map[string]Extraction,
	derived map[string]string, coverage []CoverageRow) []string {
	gated := gatedCounts(requirements)
	proof := coverageByRFC(coverage)
	var errs []string
	for _, stem := range sortedSet(stems) {
		row, held := rows[stem]
		if !held || !statusIsSupportClaim(row.Status) {
			continue
		}
		if gated[stem] > 0 {
			errs = append(errs, unprovenChecklist(stem, row, gated[stem], proof[stem])...)
			continue
		}
		if disposition, held := dispositions[stem]; held && supportClaimExcused(disposition.Kind) {
			continue
		}
		status := strings.TrimSpace(row.Status)
		if status == "" {
			status = "(blank)"
		}
		if art, held := signed[stem]; held && art.Register == registerManualWalk && art.RegisterReason != "" {
			grade := derived[stem]
			if grade != "" && grade != registerRFC2119 {
				continue
			}
			says := "no register at all (its text could not be read)"
			if grade != "" {
				says = pyRepr(grade)
			}
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(art.Path).Str(" signs ").Str(stem).Str(" under 'manual-walk' with a register-reason, but the source derives ").Str(says).
				Str(", so nothing establishes that zero MUST-level requirements is a property of the DOCUMENT -- and docs/features/rfc-status.md claims ").Str(stem).Str(" is '").Str(status).
				Str("'. A source graded 'rfc2119' quotes capitalised MUST/SHALL in its own sentences: extract those obligations (/ze-rfc ").Str(stem).
				Str("). 'manual-walk' is the weakest grade and any stem may declare it, so the declared register cannot carry this claim").String())
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str("docs/features/rfc-status.md claims ").Str(stem).Str(" is '").Str(status).
			Str("', but rfc/short/").Str(stem).Str(".md declares no MUST-level requirement, so the claim rests on an empty checklist and nothing can contradict it. Extract the RFC's obligations (/ze-rfc ").Str(stem).
			Str("); or, if the document genuinely imposes none, record the evidence -- `| Enrolment | non-normative |` in the summary's own Meta table, or a manual-walk extraction sign-off whose register-reason says why zero is a property of the text, over a source the derivation does not grade 'rfc2119'. A standard whose text may not be redistributed can never bound this claim, and a source-restricted disposition does not excuse it: stop making the claim, with a Status of 'Unsupported' or 'Future', or `| Support | - |` for no row at all").String())
	}
	return errs
}

// unprovenChecklist refuses a public PROMISE of conformance over gated
// requirements of which none is proven in both polarities.
//
// The population is statusPromisesSupport, deliberately narrower than the
// disclosure question the caller's other arm asks. The public page defines
// `Partial` as "a named subset is missing, intentionally skipped, or not
// proven", so a `Partial` row publishing `0 proven` states exactly what is
// true, and it is the remedy this message names. Billing it here would leave the author no
// status that clears the refusal, which is a guard whose remedy does not work.
//
// A stem with no coverage row reaches this with a zero CoverageRow and is
// refused, because the caller derived the gated count from the same
// requirements: a reader that cannot see the proof says so rather than passing
// the claim (ai/rules/principles.md).
func unprovenChecklist(stem string, row LedgerRow, gated int, cover CoverageRow) []string {
	if !statusPromisesSupport(row.Status) || cover.Both > 0 {
		return nil
	}
	var tb textbuf.Buffer
	return []string{tb.Str("docs/features/rfc-status.md claims ").Str(stem).Str(" is ").
		Str(pyRepr(strings.TrimSpace(row.Status))).Str(", and not one of the ").Int(int64(gated)).
		Str(" MUST-level requirement(s) rfc/short/").Str(stem).
		Str(".md gates carries a test in both polarities, so the promise rests on a checklist nothing passes. An annotation is not a proof: {gap}, {not-applicable}, {single-polarity} and {lower-layer} each record why a requirement is NOT proven, and the row's Proof cell counts them apart. Prove one requirement -- a positive AND a negative test tagged `RFC requirement: <ID> <polarity>`, with the break each goes red under recorded by ./le rfc discriminate-record -- or state what is true in the summary's own Meta table, `| Support status | Partial |`, and run ./le rfc index-update").String()}
}

func spelledNumbers() map[string]int {
	units := strings.Fields("one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen")
	tens := []string{"twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	out := map[string]int{}
	var tb textbuf.Buffer
	for index, unit := range units {
		out[unit] = index + 1
	}
	for index, ten := range tens {
		out[ten] = 20 + 10*index
		for unitIndex, unit := range units[:9] {
			out[tb.Reset().Str(ten).Byte('-').Str(unit).String()] = 20 + 10*index + unitIndex + 1
		}
	}
	return out
}

var gapNumbers = spelledNumbers()
var gapCountRE = regexp.MustCompile(`(?i)\b(` + spelledAlternation() + `)\s+(?:MUST|SHALL)s?\b`)

func spelledAlternation() string {
	words := make([]string, 0, len(gapNumbers))
	for word := range gapNumbers {
		words = append(words, word)
	}
	sort.Slice(words, func(i, j int) bool { return len(words[i]) > len(words[j]) })
	return strings.Join(words, "|")
}

func spelledGapCount(remaining string) (int, bool) {
	match := gapCountRE.FindStringSubmatch(remaining)
	if match == nil {
		return 0, false
	}
	return gapNumbers[strings.ToLower(match[1])], true
}

func checkGapCountAgreement(requirements []Requirement, rows map[string]LedgerRow) []string {
	gaps := map[string]int{}
	for _, req := range requirements {
		if req.Annotation != nil && req.Annotation.Kind == AnnotationGap {
			gaps[req.RFC]++
		}
	}
	var errs []string
	for _, stem := range sortedKeysOf(rows) {
		claimed, held := spelledGapCount(rows[stem].Remaining)
		if !held || claimed == gaps[stem] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str("rfc/short/").Str(stem).Str(".md says in its `Support remaining` cell that it has ").Int(int64(claimed)).
			Str(" MUST-level gap(s), and its own checklist carries ").Int(int64(gaps[stem])).
			Str(" {gap} annotation(s). One of the two is wrong. Only a spelled number sitting immediately before MUST or SHALL is read as a gap count; a digit count, or a number further from the keyword, is outside this check").String())
	}
	return errs
}

// statusPromisesSupport answers whether a public Status cell positively promises
// that Ze MEETS the RFC.
//
// It is deliberately NARROWER than statusIsSupportClaim above, and the two are not
// interchangeable. They answer different questions about the same cell:
//
//   - statusIsSupportClaim asks the DISCLOSURE question -- does this row fail to warn
//     the reader. Everything except the literals 'Unsupported' and 'Future' fails to
//     warn, so 'Partial', 'Experimental' and 'Not supported' are all claims by that
//     measure. checkUnprovenSupport needs it, because a checklist declaring nothing
//     rests on nothing whatever the row's status says.
//   - this one asks the PROMISE question -- does this row assert conformance. Only the
//     bare word, the word with a scope after it ('Supported on Linux'), and the legacy
//     'Yes' cell do that. checkSupportedSignoff needs it, because only a promise of
//     conformance owes a checklist somebody bounded against the RFC's own text.
//
// A row that discloses a gap is incomplete disclosure when its checklist misses an
// obligation. A row that promises conformance is a FALSE public claim. Widening this
// predicate to its neighbor would put 144 of the page's 158 rows under a gate none of
// them can pass, so the two stay apart.
func statusPromisesSupport(status string) bool {
	status = strings.TrimSpace(status)
	if status == "Supported" || status == "Yes" {
		return true
	}
	return strings.HasPrefix(status, "Supported ")
}

// checkSupportedSignoff refuses a public promise of conformance that no extraction
// sign-off bounds.
//
// `./le rfc check` proves that every requirement a summary LISTS carries a test. It
// never asks whether the list is complete, so an obligation nobody extracted is owed no
// test and the gate is green for it forever. rfc/extraction/<stem>.json is the artifact
// that closes it: a recorded walk of the RFC's own text where every requirement-stating
// site is mapped to a requirement id or excluded with a reason from a closed set.
//
// The population is every summary that declares a public row. A row naming an RFC with
// no summary was outside every check in this package until 2026-09-01, and ten such rows
// sat on the page; a row is now declared BY the summary, so that state cannot be written.
//
// The membership test is `signed`, the set evaluateExtractions ACCEPTED, and never
// `credited`. credited() drops a sign-off whose stem is not enrolled, which is right for
// drain arithmetic and wrong here: a row can promise support for a stem nobody enrolled,
// and crediting would then exempt exactly the claim least covered by anything else.
// Reading `signed` also makes a generated skeleton worth nothing, because an artifact
// with one unclassified site earns no entry in it.
func checkSupportedSignoff(rows map[string]LedgerRow, signed map[string]Extraction) []string {
	var errs []string
	for _, stem := range sortedKeysOf(rows) {
		row := rows[stem]
		if !statusPromisesSupport(row.Status) {
			continue
		}
		if _, held := signed[stem]; held {
			continue
		}
		status := strings.TrimSpace(row.Status)
		var tb textbuf.Buffer
		errs = append(errs, tb.Str("docs/features/rfc-status.md claims ").Str(stem).Str(" is '").
			Str(status).Str("', but rfc/extraction/").Str(stem).
			Str(".json is not a valid extraction sign-off, so nothing bounds what rfc/short/").
			Str(stem).Str(".md left out: an obligation nobody extracted is owed no test, and this gate stays green for it forever. Walk the source and classify every site: ./le rfc extraction-create stem ").
			Str(stem).Str(". A generated skeleton is not a sign-off -- an unclassified site earns no credit here. Only a status that PROMISES conformance is asked for one; 'Partial', 'Experimental', 'Unsupported' and 'Future' disclose the gap instead and are outside this check").String())
	}
	return errs
}

// restrictedReasonRE is what a `source-restricted` reason must cite: the body
// that publishes the standard, or the license that stops it being copied.
//
// Positive rather than a blacklist, for the reason the non-normative citation
// requirement is positive. A reason that cites none of these cannot be checked
// and cannot be contradicted, which is exactly what a claim excusing a public
// support promise must not be.
var restrictedReasonRE = regexp.MustCompile(`(?i)\bISO\b|\bIEC\b|\bITU\b|\bIEEE\b|\bANSI\b|\bETSI\b|\bcopyright\b|\blicen[cs]e\b|\bpaywall|\bnot (?:freely )?redistribut|\bnot (?:publicly|freely) available`)

// checkSourceRestricted judges the reason a summary gives for declaring that
// its standard's text can never enter this repository.
//
// The kind excuses a public support claim, so its reason carries the same
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
			Str("redistributed, but that text IS in this repository. The disposition excuses a ").
			Str("public support claim, so it may not be written over a source a reviewer can ").
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
		Str(". This kind excuses a public support claim, so its reason must rest on the body that publishes the standard (ISO, IEC, ITU, IEEE, ANSI, ETSI) or on the license, copyright or paywall that stops the text being copied. Where the text IS fetchable, record 'blocked' and fetch it").String()}
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
// The first is a public support claim. Declining to build a feature and telling
// the world Ze supports it are contradictory, and this disposition gates
// nothing, so a Status above 'Unsupported' or 'Future' would rest on an
// unchecked checklist with no other guard reaching it -- checkUnprovenSupport
// lets a summary with requirements through, and this one has them all.
//
// The second is an undateable reason. Scope decisions are revisited; a gap
// recorded as out-of-scope in 2026 and never dated reads in 2030 as a decision
// somebody still stands behind.
func checkOutOfScope(at string, meta Meta, gated int) []string {
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
	if statusIsSupportClaim(meta.Status) && meta.HasRow() {
		status := strings.TrimSpace(meta.Status)
		if status == "" {
			status = "(blank)"
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(at).Str("is declared out-of-scope, so none of its ").
			Str("requirements is gated, but its `Support status` cell claims ").Str(pyRepr(status)).
			Str(". A feature the owner decided not to offer cannot be advertised as supported: ").
			Str("write 'Future' when it is tracked for later, or 'Unsupported' when it is not").String())
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
// seeing it -- including the one refusing an unsigned support claim, which is
// exactly the refusal an author under pressure would want to silence.
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
		if metas[stem].HasRow() {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(summaryRel).Byte('/').Str(stem).
			Str(".md is newly enrolled and declares `| Support | - |`, so it renders no row on ").
			Str(statusRel).
			Str(". Enrolling gates its MUST-level requirements, so the public ledger must disclose the RFC: name a section and write its Area, Status, Implemented coverage and Remaining cells. RFCs enrolled before this ratchet existed are grandfathered and unaffected").String())
	}
	if !baselineKnown {
		return errs
	}
	for _, stem := range sortMetaStems(metas) {
		meta := metas[stem]
		base, held := baseMetas[stem]
		// Keyed on the ROW alone, never on enrolment. The checks this
		// protects iterate the rows: checkSupportedSignoff bills any row
		// whose Status promises conformance, and its own comment says why
		// enrolment is the wrong population -- "a row can promise support
		// for a stem nobody enrolled, and crediting would then exempt
		// exactly the claim least covered by anything else". An enrolled-only
		// guard left that exemption open one population narrower, and
		// rfc/short/rfc9384.md is the live instance: `non-normative` with a
		// row reading "Supported within BFD".
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
			Str(" at HEAD and declares `| Support | - |` now, while the summary is still here. Deleting the row retires a public claim without retiring the obligation behind it, and it takes the stem out of every check that reads the public page -- the unproven-support guard and the extraction sign-off guard among them. Restore the section, or correct the row's cells in place. To retire the RFC itself, delete rfc/short/").String())
	}
	return errs
}
