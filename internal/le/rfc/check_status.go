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
		if req.Annotation == nil || req.Annotation.Kind != annotationGap {
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
			Str("} but docs/features/rfc-status.md says ").Str(req.RFC).Str(" is '").Str(row.Status).
			Str("' with '").Str(truncateRunes(row.Remaining, 40)).Str("'. A known unmet MUST cannot be advertised as clean support -- update the row's Status/Remaining").String())
	}
	return errs
}

func nonNormativeReasonCitesDocument(reason string) bool {
	return documentCategoryRE.MatchString(reason) || siteKeywordRE.MatchString(reason)
}

func checkSummaryDisposition(stems, enrolled map[string]bool, dispositions map[string]Disposition,
	baseline map[string]bool) []string {
	declared := map[string]bool{}
	for stem := range dispositions {
		declared[stem] = true
	}
	var errs []string
	for _, stem := range sortedSet(stems) {
		if enrolled[stem] || declared[stem] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str("rfc/short/").Str(stem).
			Str(".md is in neither rfc/enrolled.txt nor rfc/not-enrolled.txt. Every summary is enrolled or declared: an un-enrolled summary with no recorded reason cannot be told apart from one nobody has got to yet. Enroll it, or declare it with a kind from ['backlog', 'blocked', 'non-normative']").String())
	}
	for _, stem := range sortedShared(enrolled, dispositions) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(stem).Str(" is in BOTH rfc/enrolled.txt and rfc/not-enrolled.txt. The two files partition the summaries; a stem in both is a contradiction, and resolving it by precedence would let one file quietly overrule the other. Remove the rfc/not-enrolled.txt row -- enrolment is the discharge").String())
	}
	for _, stem := range sortedMissing(dispositions, stems) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str("rfc/not-enrolled.txt declares ").Str(stem).Str(", but rfc/short/").Str(stem).
			Str(".md does not exist. A disposition for a summary nobody wrote records a decision about nothing, and it hides the fact that the row is stale").String())
	}
	for _, stem := range sortedShared(dispositions, stems) {
		disposition := dispositions[stem]
		if disposition.Kind != dispositionNonNormative {
			continue
		}
		if nonApplicabilityRE.MatchString(disposition.Reason) {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str("rfc/not-enrolled.txt: ").Str(stem).
				Str(" is declared non-normative with a reason that judges what ZE owes rather than what the DOCUMENT states: ").Str(pyRepr(truncateRunes(disposition.Reason, 80))).
				Str(". 'non-normative' means the RFC imposes no MUST-level obligation on any speaker. Whether an obligation applies to Ze is a conformance judgement (ai/rules/rfc-compliance.md reserves it to the owner) -- record 'backlog' or 'blocked' instead").String())
			continue
		}
		if nonNormativeReasonCitesDocument(disposition.Reason) {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str("rfc/not-enrolled.txt: ").Str(stem).
			Str(" is declared non-normative with a reason that cites nothing about the DOCUMENT: ").Str(pyRepr(truncateRunes(disposition.Reason, 80))).
			Str(". 'non-normative' is the one kind that makes a claim about conformance, so its reason must rest on something a reviewer can check in the text: the RFC's IETF category (Informational, Experimental, Historic, Best Current Practice), the presence or absence of the RFC 2119 / RFC 8174 / BCP 14 key-words machinery, or the result of a capitalised MUST/SHALL/REQUIRED scan over the source. A reason that cites none of those cannot be checked or contradicted -- record 'backlog' or 'blocked' instead").String())
	}
	for _, stem := range sortedSet(baseline) {
		if !stems[stem] || declared[stem] || enrolled[stem] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str("rfc/short/").Str(stem).Str(".md is still in the tree, but ").Str(stem).
			Str(" left rfc/not-enrolled.txt without entering rfc/enrolled.txt. A disposition over a LIVE summary is discharged by ENROLMENT and by nothing else: deleting the row returns the summary to the undeclared state the file exists to abolish. To retire the RFC instead, delete rfc/short/").Str(stem).Str(".md in the same commit").String())
	}
	return errs
}

func checkStatusCompleteness(enrolled map[string]bool, rows map[string]LedgerRow,
	baselineRows map[string]LedgerRow, baselineRowsKnown bool, newly, baselineEnrolled map[string]bool) []string {
	var errs []string
	for _, stem := range sortedSet(newly) {
		if _, held := rows[stem]; held {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(stem).
			Str(" is newly enrolled but has no row in docs/features/rfc-status.md. Enrolling gates its MUST-level requirements, so the public ledger must disclose the RFC: add a row with a Status, an Implemented coverage note and a Remaining note. RFCs enrolled before this ratchet existed are grandfathered and unaffected").String())
	}
	if !baselineRowsKnown {
		return errs
	}
	for _, stem := range sortedSet(enrolled) {
		if !baselineEnrolled[stem] {
			continue
		}
		if _, existed := baselineRows[stem]; !existed {
			continue
		}
		if _, held := rows[stem]; held {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(stem).
			Str(" had a row in docs/features/rfc-status.md at HEAD and does not now, while it stays enrolled. Deleting a row retires a public claim without retiring the obligation behind it, and it is the one edit that can make check_status_agreement's missing-row branch fire on unrelated work later. Restore the row, or correct it in place").String())
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

func checkUnprovenSupport(requirements []Requirement, rows map[string]LedgerRow,
	stems map[string]bool, dispositions map[string]Disposition, signed map[string]Extraction,
	derived map[string]string) []string {
	gated := gatedCounts(requirements)
	var errs []string
	for _, stem := range sortedSet(stems) {
		row, held := rows[stem]
		if !held || gated[stem] > 0 || !statusIsSupportClaim(row.Status) {
			continue
		}
		if disposition, held := dispositions[stem]; held && disposition.Kind == dispositionNonNormative {
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
			Str("); or, if the document genuinely imposes none, record the evidence -- a non-normative disposition in rfc/not-enrolled.txt, or a manual-walk extraction sign-off whose register-reason says why zero is a property of the text, over a source the derivation does not grade 'rfc2119'. Rows naming an RFC with no summary at all are outside this check").String())
	}
	return errs
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
		if req.Annotation != nil && req.Annotation.Kind == annotationGap {
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
		errs = append(errs, tb.Str("docs/features/rfc-status.md says ").Str(stem).Str(" has ").Int(int64(claimed)).
			Str(" MUST-level gap(s), but rfc/short/").Str(stem).Str(".md carries ").Int(int64(gaps[stem])).
			Str(" {gap} annotation(s). One of the two is wrong. Only a spelled number sitting immediately before MUST or SHALL is read as a gap count; a digit count, or a number further from the keyword, is outside this check").String())
	}
	return errs
}
