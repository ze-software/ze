// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that shares one audit load
// Related: freshness.go -- the single freshness derivation these checks consume
//
// check_audit.go makes each authored audit verdict load-bearing.
package rfc

import (
	"regexp"
	"sort"
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
		rel := tb.Str(auditRel).Byte('/').Str(stem).Str(".json").String()
		if !stems[stem] {
			var message textbuf.Buffer
			errs = append(errs, message.Str(rel).Str(": there is no rfc/short/").Str(stem).
				Str(".md, so this file records judgements about requirements that do not exist. Delete it, or write the summary").String())
			continue
		}
		if !enrolled[stem] {
			var message textbuf.Buffer
			errs = append(errs, message.Str(rel).Str(": ").Str(stem).
				Str(" is not in rfc/enrolled.txt, so nothing reads these verdicts -- an audit file for an un-enrolled RFC is evidence the gate never loads. Enroll ").Str(stem).Str(", or delete the file").String())
		}
	}
	return errs, nil
}

func checkAuditSchema(requirements []Requirement, tags []Tag, audits map[string]Audit) []string {
	byRID := tagsByRID(tags)
	known := map[string]Requirement{}
	for _, req := range requirements {
		if _, held := known[req.RID]; !held {
			known[req.RID] = req
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
				errs = append(errs, tb.Str(auditRel).Byte('/').Str(rfc).Str(".json: ").Str(rid).
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
	rel := prefix.Str(auditRel).Byte('/').Str(rfc).Str(".json: ").Str(rid).String()
	var errs []string
	if value == verdictEnforced {
		if len(recordedMap(verdict, fingerprintTests)) == 0 {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(" is 'enforced' with an empty 'tests' map. 'enforced' means the tests would fail if the code stopped complying, so it must cite at least one. If no reachable code path could satisfy or violate the requirement, the honest verdict is 'not-applicable' with a 'no_code_path' reason and an agreeing {not-applicable} annotation").String())
		}
		if req.Annotation == nil || req.Annotation.Kind != annotationSinglePolarity {
			var missing []string
			for _, polarity := range []string{polarityNegative, polarityPositive} {
				if !polarities[polarity] {
					missing = append(missing, polarity)
				}
			}
			if len(missing) > 0 {
				held := make([]string, 0, len(polarities))
				for polarity := range polarities {
					held = append(held, polarity)
				}
				sort.Strings(held)
				if len(held) == 0 {
					held = []string{"none"}
				}
				var tb textbuf.Buffer
				errs = append(errs, tb.Str(rel).Str(" is 'enforced' but has no ").Str(strings.Join(missing, "/")).
					Str(" test (only ").Str(strings.Join(held, "/")).Str("). One polarity cannot prove a requirement: add the missing test, or annotate the summary line {single-polarity: <polarity>; why}").String())
			}
		}
	}
	if value == verdictUnimplemented {
		if len(recordedMap(verdict, fingerprintCode)) == 0 {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(" is 'unimplemented' with an empty 'code' map. A claim that the CODE does not comply must name the producing code, or it is unfalsifiable: with neither tests nor code fingerprinted, the verdict can never go stale and no one is ever asked to look again").String())
		}
		if req.Annotation == nil || (req.Annotation.Kind != annotationGap && req.Annotation.Kind != annotationNotApplicable) {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(" is 'unimplemented' but ").Str(req.Source).Byte(':').Int(int64(req.Line)).
				Str(" carries no {gap} or {not-applicable} annotation. The record and the checklist must agree: a divergence Ze knows about must be declared where a reader of the summary will meet it").String())
		}
	}
	if value == verdictNotApplicable {
		if tests := recordedMap(verdict, fingerprintTests); len(tests) > 0 {
			keys := make([]string, 0, len(tests))
			for key := range tests {
				keys = append(keys, key)
			}
			sort.Strings(keys)
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
			if req.Annotation.Kind == annotationNotApplicable {
				return errs
			}
			kind = tb.Byte('{').Str(req.Annotation.Kind).Byte('}').String()
		}
		errs = append(errs, tb.Reset().Str(rel).Str(" is 'not-applicable' but ").Str(req.Source).Byte(':').Int(int64(req.Line)).
			Str(" carries ").Str(kind).Str(". Two committed facts must agree, so this verdict is legal only over a {not-applicable} checklist line -- the audit record cannot reclassify a requirement on its own").String())
	}
	return errs
}

func checkAuditFreshness(requirements []Requirement, states map[string]Freshness) []string {
	var errs []string
	for _, req := range requirements {
		state, held := states[req.RID]
		if !held || state.State == FreshState {
			continue
		}
		where := requirementWhere(req)
		switch state.State {
		case ShiftedState:
			detail := strings.Join(state.Moved, ", ")
			if detail == "" {
				detail = "a line shift"
			}
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" has a SHIFTED audit verdict -- the tagged unit is byte-identical and only the file around it moved (").Str(detail).
				Str("), so nothing was re-judged. Re-stamp it mechanically: ./le rfc reseal").String())
		case StaleRequirementState:
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" has a STALE audit verdict -- the REQUIREMENT TEXT changed since it was judged, so every judgement under it is void. Re-read ").Str(req.RFC).Str(" with the ze-rfc-audit skill (ai/skills/ze-rfc-audit.md)").String())
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
	for _, req := range requirements {
		if !enrolled[req.RFC] || seen[req.RID] {
			continue
		}
		verdict, held := audits[req.RFC].Verdict(req.RID)
		if !held {
			continue
		}
		value := verdictValue(verdict)
		if value != verdictWrong && value != verdictUnimplemented {
			continue
		}
		seen[req.RID] = true
		row, rowHeld := rows[req.RFC]
		if !rowHeld {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(auditRel).Byte('/').Str(req.RFC).Str(".json: ").Str(req.RID).Str(" is ").Str(pyRepr(value)).Str(" but ").Str(req.RFC).Str(" has no row in docs/features/rfc-status.md; the public ledger must disclose it").String())
			continue
		}
		if rowDisclosesGap(row) {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(auditRel).Byte('/').Str(req.RFC).Str(".json: ").Str(req.RID).Str(" is ").Str(pyRepr(value)).Str(" but docs/features/rfc-status.md says ").Str(req.RFC).Str(" is '").Str(row.Status).Str("' with '").Str(truncateRunes(row.Remaining, 40)).Str("'. An audited requirement that is not met cannot be advertised as clean support -- update the row's Status/Remaining. Reverting the verdict is not an exit: the findings ratchet refuses that too").String())
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
	for _, req := range requirements {
		if !enrolled[req.RFC] || seen[req.RID] {
			continue
		}
		was := baseline[req.RFC][req.RID]
		oldValue := verdictValue(was)
		if oldValue != verdictWeak && oldValue != verdictWrong {
			continue
		}
		seen[req.RID] = true
		now, held := audits[req.RFC].Verdict(req.RID)
		if !held {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(auditRel).Byte('/').Str(req.RFC).Str(".json: the ").Str(pyRepr(oldValue)).Str(" finding on ").Str(req.RID).Str(" was DELETED. A finding is resolved by fixing the test or retiring the requirement, never by removing the record of it -- deletion is the cheapest route from red to green and is the one this ratchet exists to close").String())
			continue
		}
		if verdictValue(now) != verdictEnforced {
			continue
		}
		upgrade, _ := now["upgrade_reason"].(string)
		if strings.TrimSpace(upgrade) != "" || !mapsEqualString(recordedMap(now, fingerprintUnits), recordedMap(was, fingerprintUnits)) {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(auditRel).Byte('/').Str(req.RFC).Str(".json: ").Str(req.RID).Str(" went from ").Str(pyRepr(oldValue)).Str(" to 'enforced' while every tagged unit stayed byte-identical. A finding cannot become proof with nothing changed: fix the test (which moves its unit fingerprint), or record an 'upgrade_reason' saying what you re-read and why the earlier judgement was wrong").String())
	}
	return errs
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
	for _, req := range requirements {
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
		errs = append(errs, tb.Str(auditRel).Byte('/').Str(req.RFC).Str(".json: ").Str(req.RID).Str(" carried a verdict at HEAD and carries none now. Audit coverage is monotonic per requirement id: a judgement that was made cannot be un-made by deleting it. Re-judge it (the ze-rfc-audit skill, ai/skills/ze-rfc-audit.md) or re-stamp it (./le rfc reseal) -- removal is not an option").String())
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
	for _, req := range requirements {
		if !enrolled[req.RFC] || seen[req.RID] {
			continue
		}
		verdict, held := audits[req.RFC].Verdict(req.RID)
		if !held || verdictValue(verdict) != verdictEnforced || len(byRID[req.RID]) == 0 {
			continue
		}
		seen[req.RID] = true
		var blob strings.Builder
		files := map[string]bool{}
		for _, tag := range byRID[req.RID] {
			content := reader.text(tag.File)
			if content == "" {
				continue
			}
			files[tag.File] = true
			name := index.funcNameAt(tag.File, content, tag.Line)
			if name == "" {
				blob.WriteString(content)
				continue
			}
			units := index.funcTexts(content, name)
			if len(units) == 1 {
				blob.WriteString(units[0])
			}
		}
		note, _ := verdict["note"].(string)
		tokens := noteIdentRE.FindAllString(note, -1)
		matched := false
		for _, token := range tokens {
			if strings.Contains(blob.String(), token) {
				matched = true
			}
		}
		if matched {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(auditRel).Byte('/').Str(req.RFC).Str(".json: ").Str(req.RID).Str(" is 'enforced' but its note names nothing that occurs in the tagged unit(s). Searched ").Str(strings.Join(sortedSet(files), ", ")).Str("; tokens checked: ").Str(firstTokens(tokens, 12)).Str(". A note that cannot be tied to the test it judges is not evidence that the test was read -- name the assertion, the helper, or the constant the judgement turns on").String())
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
