// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that shares one audit load
// Related: freshness.go -- the single freshness derivation these checks consume
//
// check_audit.go makes each authored audit verdict load-bearing.
package rfc

import (
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var noteIdentRE = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{4,}`)

func checkAuditFiles(tree string, enrolled, stems map[string]bool) ([]string, error) {
	auditStems, err := auditStems(tree)
	if err != nil {
		return nil, err
	}
	var errs []string
	for _, stem := range sortedSet(auditStems) {
		var tb textbuf.Buffer
		rel := tb.Str(auditRel).Byte('/').Str(stem).Str("/").String()
		if !stems[stem] {
			var message textbuf.Buffer
			errs = append(errs, message.Str(rel).Str(": there is no rfc/short/").Str(stem).
				Str(".md, so these files record judgements about requirements that do not exist. Delete the directory, or write the summary").String())
			continue
		}
		if !enrolled[stem] {
			var message textbuf.Buffer
			errs = append(errs, message.Str(rel).Str(": ").Str(stem).
				Str(" does not declare `| Enrolment | enrolled |`, so nothing reads these verdicts -- an audit for an un-enrolled RFC is evidence the gate never loads. Enroll ").Str(stem).Str(", or delete the directory").String())
		}
	}
	return errs, nil
}

func checkAuditSchema(requirements []Requirement, tags []Tag, audits map[string]Audit) []string {
	byRID := tagsByRID(tags)
	known := map[string]Requirement{}
	for i := range requirements {
		req := &requirements[i]
		if _, held := known[req.RID]; !held {
			known[req.RID] = *req
		}
	}
	var errs []string
	for _, rfc := range sortedKeysOf(audits) {
		audit := audits[rfc]
		for _, rid := range sortedKeysOf(audit.Verdicts) {
			verdict, held := audit.Verdict(rid)
			if !held {
				continue
			}
			req, knownRID := known[rid]
			if !knownRID || req.RFC != rfc {
				var tb textbuf.Buffer
				errs = append(errs, tb.Str(auditRecordRel(rfc, rid)).Str(": ").Str(rid).
					Str(" is not a requirement of ").Str(rfc).Str(". A verdict may not describe a requirement that is not there: either the id was renumbered (which the id rules forbid) or the checklist line was deleted under it").String())
				continue
			}
			errs = append(errs, verdictClaims(rfc, rid, verdict, req, byRID[rid])...)
		}
	}
	return errs
}

func verdictClaims(rfc, rid string, verdict map[string]any, req Requirement, found []Tag) []string {
	value := verdictValue(verdict)
	polarities := map[string]bool{}
	for _, tag := range found {
		polarities[tag.Polarity] = true
	}
	var prefix textbuf.Buffer
	rel := prefix.Str(auditRecordRel(rfc, rid)).Str(": ").Str(rid).String()
	var errs []string
	var traits AnnotationTraits
	if req.Annotation != nil {
		var known bool
		traits, known = AnnotationTraitsOf(req.Annotation.Kind)
		if !known {
			return []string{rel + " has unknown annotation kind " + req.Annotation.Kind}
		}
	}
	if value == VerdictPartial {
		if req.Annotation == nil {
			return []string{rel + " is 'partial' without a matching {partial} annotation"}
		}
		if !traits.Partial {
			return []string{rel + " is 'partial' without a matching {partial} annotation"}
		}
		if why := partialScopeRefusal(&req); why != "" {
			errs = append(errs, rel+" has invalid partial scope: "+why)
		}
		for _, field := range []string{fingerprintTests, fingerprintUnits} {
			if len(recordedMap(verdict, field)) == 0 {
				errs = append(errs, rel+" is 'partial' with an empty '"+field+"' map")
			}
		}
		if recordedMap(verdict, fingerprintCode)[req.Annotation.Producer] == "" {
			errs = append(errs, rel+" is 'partial' without its gap-context producer in the code map")
		}
		for _, polarity := range []string{PolarityPositive, PolarityNegative} {
			if !polarities[polarity] {
				errs = append(errs, rel+" is 'partial' without a "+polarity+" test")
			}
		}
		note, _ := verdict[verdictFieldNote].(string)
		for _, field := range []string{req.Annotation.Tested, req.Annotation.Gap, req.Annotation.Producer} {
			if !strings.Contains(normalize(note), normalize(field)) {
				errs = append(errs, rel+" partial note must identify tested and missing behavior and boundary: "+field)
			}
		}
	}
	if value == VerdictEnforced {
		if traits.Partial {
			errs = append(errs, rel+" cannot be 'enforced' while a {partial} scope remains")
		}
		if len(recordedMap(verdict, fingerprintTests)) == 0 {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(" is 'enforced' with an empty 'tests' map. 'enforced' means the tests would fail if the code stopped complying, so it must cite at least one. If no reachable code path could satisfy or violate the requirement, the honest verdict is 'not-applicable' with a 'no_code_path' reason and an agreeing {not-applicable} or {feature-declined} annotation").String())
		}
		if !traits.SinglePolarity {
			var missing []string
			for _, polarity := range []string{PolarityNegative, PolarityPositive} {
				if !polarities[polarity] {
					missing = append(missing, polarity)
				}
			}
			if len(missing) > 0 {
				held := make([]string, 0, len(polarities))
				for polarity := range polarities {
					held = append(held, polarity)
				}
				slices.Sort(held)
				if len(held) == 0 {
					held = []string{"none"}
				}
				var tb textbuf.Buffer
				errs = append(errs, tb.Str(rel).Str(" is 'enforced' but has no ").Str(strings.Join(missing, "/")).
					Str(" test (only ").Str(strings.Join(held, "/")).Str("). One polarity cannot prove a requirement: add the missing test, or annotate the summary line {single-polarity: <polarity>; why}").String())
			}
		}
	}
	if value == VerdictUnimplemented {
		if len(recordedMap(verdict, fingerprintCode)) == 0 {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(" is 'unimplemented' with an empty 'code' map. A claim that the CODE does not comply must name the producing code, or it is unfalsifiable: with neither tests nor code fingerprinted, the verdict can never go stale and no one is ever asked to look again").String())
		}
		if !traits.AuditUnimplemented {
			var admitted []string
			for _, kind := range AnnotationKinds() {
				if annotationKinds[kind].traits.AuditUnimplemented {
					admitted = append(admitted, "{"+kind+"}")
				}
			}
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(" is 'unimplemented' but ").Str(req.Source).Byte(':').Int(int64(req.Line)).
				Str(" carries none of the annotations that admit this verdict: ").Str(strings.Join(admitted, ", ")).
				Str(". The record and the checklist must agree: a divergence Ze knows about must be declared where a reader of the summary will meet it").String())
		}
	}
	if value == VerdictNotApplicable {
		if tests := recordedMap(verdict, fingerprintTests); len(tests) > 0 {
			keys := make([]string, 0, len(tests))
			for key := range tests {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(" is 'not-applicable' but cites tests (").Str(strings.Join(keys, ", ")).Str("). If a test can exercise it, a reachable code path exists and the verdict is a judgement about that test").String())
		}
		reason, _ := verdict["no_code_path"].(string)
		if strings.TrimSpace(reason) == "" {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(" is 'not-applicable' with no 'no_code_path' reason. State WHY no reachable code path could satisfy or violate this requirement: a verdict whose only content is its own name is exactly the unfalsifiable entry this schema rejects").String())
		}
		var tb textbuf.Buffer
		kind := "no annotation"
		if req.Annotation != nil {
			if traits.AuditNotApplicable {
				return errs
			}
			kind = tb.Byte('{').Str(req.Annotation.Kind).Byte('}').String()
		}
		errs = append(errs, tb.Reset().Str(rel).Str(" is 'not-applicable' but ").Str(req.Source).Byte(':').Int(int64(req.Line)).
			Str(" carries ").Str(kind).Str(". Two committed facts must agree, so this verdict is legal only over a {not-applicable} or {feature-declined} checklist line -- the audit record cannot reclassify a requirement on its own").String())
	}
	return errs
}

func checkAuditFreshness(requirements []Requirement, states map[string]Freshness) []string {
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		state, held := states[req.RID]
		if !held || state.State == FreshState {
			continue
		}
		where := requirementWhere(*req)
		switch state.State {
		case ShiftedState:
			detail := strings.Join(state.Moved, ", ")
			if detail == "" {
				detail = "a line shift"
			}
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" has a SHIFTED audit verdict -- the tagged unit is byte-identical and only the file around it moved (").Str(detail).
				Str("), so nothing was re-judged. Re-stamp it mechanically: ./le rfc reseal stem ").Str(req.RFC).String())
		case StaleRequirementState:
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" has a STALE audit verdict -- the REQUIREMENT TEXT or declared partial scope changed since it was judged, so every judgement under it is void. Re-read ").Str(req.RFC).Str(" with the ze-rfc-audit skill (ai/skills/ze-rfc-audit.md)").String())
		default:
			var tb textbuf.Buffer
			var details []string
			for _, key := range state.Moved {
				scope := "file"
				if strings.Contains(key, "::") {
					scope = "func"
				}
				details = append(details, tb.Reset().Str(key).Str(" (").Str(scope).Str("-scoped)").String())
			}
			detail := strings.Join(details, ", ")
			if detail == "" {
				detail = "a tagged test is gone"
			}
			errs = append(errs, tb.Reset().Str(where).Str(": ").Str(req.RID).
				Str(" has a STALE audit verdict -- what it judged changed: ").Str(detail).
				Str(". This is NOT a line shift and ./le rfc reseal will refuse it. Re-read ").Str(req.RFC).Str(" with the ze-rfc-audit skill (ai/skills/ze-rfc-audit.md)").String())
		}
	}
	return errs
}

func checkAuditDisclosure(requirements []Requirement, rows map[string]LedgerRow,
	enrolled map[string]bool, audits map[string]Audit) []string {
	seen := map[string]bool{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		if !enrolled[req.RFC] || seen[req.RID] {
			continue
		}
		verdict, held := audits[req.RFC].Verdict(req.RID)
		if !held {
			continue
		}
		value := verdictValue(verdict)
		if !auditVerdictUnmet(value) {
			continue
		}
		seen[req.RID] = true
		if _, rowHeld := rows[req.RFC]; rowHeld {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(auditRecordRel(req.RFC, req.RID)).Str(": ").Str(req.RID).Str(" is ").Str(pyRepr(value)).Str(" but ").Str(req.RFC).Str(" has no row in docs/features/rfc-status.md; the public ledger must disclose it").String())
	}
	return errs
}

func checkAuditFindings(requirements []Requirement, enrolled map[string]bool, audits map[string]Audit,
	baseline map[string]map[string]map[string]any, known bool) []string {
	if !known {
		return nil
	}
	seen := map[string]bool{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		if !enrolled[req.RFC] || seen[req.RID] {
			continue
		}
		was := baseline[req.RFC][req.RID]
		oldValue := verdictValue(was)
		if oldValue != VerdictWeak && oldValue != VerdictWrong && oldValue != VerdictPartial {
			continue
		}
		seen[req.RID] = true
		now, held := audits[req.RFC].Verdict(req.RID)
		if !held {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(auditRecordRel(req.RFC, req.RID)).Str(": the ").Str(pyRepr(oldValue)).Str(" finding on ").Str(req.RID).Str(" was DELETED. A finding is resolved by fixing the test or retiring the requirement, never by removing the record of it -- deletion is the cheapest route from red to green and is the one this ratchet exists to close").String())
			continue
		}
		if !upgradeOverUnchangedUnits(was, now) {
			continue
		}
		if upgradeReasonGiven(now) {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(auditRecordRel(req.RFC, req.RID)).Str(": ").Str(req.RID).Str(" went from ").Str(pyRepr(oldValue)).Str(" to 'enforced' while every tagged unit stayed byte-identical. A finding cannot become proof with nothing changed: fix the test (which moves its unit fingerprint), or record an 'upgrade_reason' saying what you re-read and why the earlier judgement was wrong").String())
	}
	return errs
}

// findingUpgrade reports whether a verdict moved from a finding, 'weak' or
// 'wrong', to 'enforced'.
func findingUpgrade(was, now map[string]any) bool {
	oldValue := verdictValue(was)
	if oldValue != VerdictWeak && oldValue != VerdictWrong && oldValue != VerdictPartial {
		return false
	}
	return verdictValue(now) == VerdictEnforced
}

// upgradeOverUnchangedUnits reports whether a finding became 'enforced' while
// every tagged unit stayed byte-identical: the move that owes an
// 'upgrade_reason'. It is the one test for that move. checkAuditFindings
// applies it at commit against HEAD^, and rejudgeRefusal applies it when the
// verdict is stamped, so the stamp cannot write what the gate refuses.
func upgradeOverUnchangedUnits(was, now map[string]any) bool {
	if !findingUpgrade(was, now) {
		return false
	}
	return mapsEqualString(recordedMap(now, fingerprintUnits), recordedMap(was, fingerprintUnits))
}

// upgradeReasonGiven reports whether a verdict carries a non-blank
// 'upgrade_reason'.
func upgradeReasonGiven(verdict map[string]any) bool {
	reason, _ := verdict[verdictFieldUpgradeReason].(string)
	return strings.TrimSpace(reason) != ""
}

func mapsEqualString(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func checkAuditVerdictRatchet(requirements []Requirement, enrolled map[string]bool,
	audits map[string]Audit, baseline map[string]map[string]map[string]any, known bool,
	baselineEnrolled map[string]bool) []string {
	if !known {
		return nil
	}
	seen := map[string]bool{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		if !enrolled[req.RFC] || !baselineEnrolled[req.RFC] || seen[req.RID] {
			continue
		}
		if _, held := baseline[req.RFC][req.RID]; !held {
			continue
		}
		if _, held := audits[req.RFC].Verdict(req.RID); held {
			continue
		}
		seen[req.RID] = true
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(auditRecordRel(req.RFC, req.RID)).Str(": ").Str(req.RID).Str(" carried a verdict at HEAD^ and carries none now. Audit coverage is monotonic per requirement id: a judgement that was made cannot be un-made by deleting it. Re-judge it (the ze-rfc-audit skill, ai/skills/ze-rfc-audit.md) or re-stamp it (./le rfc reseal) -- removal is not an option").String())
	}
	return errs
}

func checkAuditNote(tree string, requirements []Requirement, tags []Tag, enrolled map[string]bool,
	audits map[string]Audit) []string {
	byRID := tagsByRID(tags)
	reader := newSourceReader(tree)
	index := newScopeIndex()
	seen := map[string]bool{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		if !enrolled[req.RFC] || seen[req.RID] {
			continue
		}
		verdict, held := audits[req.RFC].Verdict(req.RID)
		if !held || verdictValue(verdict) != VerdictEnforced || len(byRID[req.RID]) == 0 {
			continue
		}
		seen[req.RID] = true
		var blob textbuf.Buffer
		files := map[string]bool{}
		for _, tag := range byRID[req.RID] {
			content := reader.text(tag.File)
			if content == "" {
				continue
			}
			files[tag.File] = true
			name := index.funcNameAt(tag.File, content, tag.Line)
			if name == "" {
				blob.Str(content)
				continue
			}
			units := index.funcTexts(content, name)
			if len(units) == 1 {
				blob.Str(units[0])
			}
		}
		note, _ := verdict["note"].(string)
		tokens := noteIdentRE.FindAllString(note, -1)
		unit := blob.String()
		matched := false
		for _, token := range tokens {
			if strings.Contains(unit, token) {
				matched = true
			}
		}
		if matched {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(auditRecordRel(req.RFC, req.RID)).Str(": ").Str(req.RID).Str(" is 'enforced' but its note names nothing that occurs in the tagged unit(s). Searched ").Str(strings.Join(sortedSet(files), ", ")).Str("; tokens checked: ").Str(firstTokens(tokens, 12)).Str(". A note that cannot be tied to the test it judges is not evidence that the test was read -- name the assertion, the helper, or the constant the judgement turns on").String())
	}
	return errs
}

// partialProofClaims reads current native observations, never executing proof.
// Every distinct cover must have a verified mutant or revert, not an escape.
func partialProofClaims(tree string, req Requirement, tags []Tag) ([]string, error) {
	covers, err := tagCovers(newSourceReader(tree), newScopeIndex(), tags)
	if err != nil {
		return nil, err
	}
	// Only this requirement's records can prove its covers, and they are all in
	// its own file. No file is no record, which the refusals below name.
	file := evidenceFile{Stem: req.RFC, RID: req.RID, Rel: discriminationRecordRel(req.RFC, req.RID)}
	var records []DiscriminationRecord
	if _, statErr := os.Stat(treePath(tree, file.Rel)); statErr == nil {
		if records, err = loadDiscriminationFile(tree, file); err != nil {
			return nil, err
		}
	}
	verdicts, err := verifyDiscrimination(tree, records, covers)
	if err != nil {
		return nil, err
	}
	return partialCoverRefusals(req.RID, covers, verdicts), nil
}

func partialCoverRefusals(rid string, covers map[Cover][]Tag, verdicts []DiscriminationVerdict) []string {
	proven := map[Cover]bool{}
	for i := range verdicts {
		verdict := &verdicts[i]
		if verdict.Verified() && verdict.Record.Proves() {
			proven[verdict.Record.Cover()] = true
		}
	}
	var errs []string
	for cover := range covers {
		if cover.RID == rid && !proven[cover] {
			errs = append(errs, rid+" partial scope needs a current verified mutant/revert for "+cover.Polarity+" "+cover.Unit)
		}
	}
	slices.Sort(errs)
	return errs
}

func checkPartialProofs(requirements []Requirement, audits map[string]Audit,
	covers map[Cover][]Tag, verdicts []DiscriminationVerdict) []string {
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		verdict, held := audits[req.RFC].Verdict(req.RID)
		if held && verdictValue(verdict) == VerdictPartial {
			errs = append(errs, partialCoverRefusals(req.RID, covers, verdicts)...)
		}
	}
	return errs
}

func firstTokens(tokens []string, maximum int) string {
	if len(tokens) == 0 {
		return "(none)"
	}
	if len(tokens) > maximum {
		tokens = tokens[:maximum]
	}
	return strings.Join(tokens, ", ")
}
