// Design: docs/contributing/feature-maturity.md -- per-feature extra criteria
// Related: declaration.go -- the Meta table the Extra criteria cell is read from
// Related: check.go -- the ceiling an unmet criterion lowers
//
// An Extra criteria cell carries items separated by "; ", because an
// attestation's prose may itself hold ", ". Each item is
// `<level>: <criterion> = <evidence>`: the level it gates, what it asks, and
// either a pointer (a repository path, a real-path test item, or an interop
// `<suite>/<scenario>`) or a dated attestation `YYYY-MM-DD: what was judged`.

package feature

import (
	"strings"
	"time"
)

// fieldExtraCriteria is the Meta row that carries the extra criteria.
const fieldExtraCriteria = "Extra criteria"

// extraSeparator splits the items of the Extra criteria cell.
const extraSeparator = "; "

// extraEvidenceSeparator splits an item's criterion from its evidence.
const extraEvidenceSeparator = " = "

// ExtraCriterion is one per-feature completion criterion. Exactly one of
// Pointer and Attestation is set: the parse refuses an item with neither.
type ExtraCriterion struct {
	Gates       Level
	Criterion   string
	Pointer     string
	Attestation Attestation
}

// extraCriteria parses the Extra criteria cell.
func extraCriteria(cell, where string) ([]ExtraCriterion, error) {
	if cell == "" {
		return nil, nil
	}
	var out []ExtraCriterion
	for item := range strings.SplitSeq(cell, extraSeparator) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		criterion, err := extraCriterion(item, where)
		if err != nil {
			return nil, err
		}
		out = append(out, criterion)
	}
	return out, nil
}

func extraCriterion(item, where string) (ExtraCriterion, error) {
	gate, rest, found := strings.Cut(item, ": ")
	if !found {
		return ExtraCriterion{}, refusal(where, fieldExtraCriteria, "item '"+item+"' does not open with '<level>: '")
	}
	level, known := lookup(levelNames, strings.TrimSpace(gate))
	if !known {
		return ExtraCriterion{}, refusal(where, fieldExtraCriteria, "item '"+item+"' gates '"+gate+
			"', which is not one of: "+strings.Join(LevelNames(), ", "))
	}
	text, evidence, found := strings.Cut(rest, extraEvidenceSeparator)
	if !found {
		return ExtraCriterion{}, refusal(where, fieldExtraCriteria, "item '"+item+
			"' names no evidence after ' = '; a criterion without evidence or attestation is unmet by construction")
	}
	criterion := ExtraCriterion{Gates: level, Criterion: strings.TrimSpace(text)}
	evidence = strings.Trim(strings.TrimSpace(evidence), "`")
	if criterion.Criterion == "" {
		return ExtraCriterion{}, refusal(where, fieldExtraCriteria, "item '"+item+"' names no criterion")
	}
	if evidence == "" {
		return ExtraCriterion{}, refusal(where, fieldExtraCriteria, "item '"+item+"' names no evidence after ' = '")
	}
	date, judged, dated := datedEvidence(evidence)
	if !dated {
		criterion.Pointer = evidence
		return criterion, nil
	}
	if judged == "" {
		return ExtraCriterion{}, refusal(where, fieldExtraCriteria, "item '"+item+
			"' carries a date and nothing judged; a date alone attests nothing")
	}
	criterion.Attestation = Attestation{Date: date, Judged: judged}
	return criterion, nil
}

// datedEvidence answers whether evidence opens with an attestation date, and
// if so the date and what follows its colon.
func datedEvidence(evidence string) (time.Time, string, bool) {
	date, judged, _ := strings.Cut(evidence, ":")
	parsed, err := time.Parse(attestationLayout, strings.TrimSpace(date))
	if err != nil {
		return time.Time{}, "", false
	}
	return parsed, strings.TrimSpace(judged), true
}

// criterionExtra is AC-11: an extra criterion whose pointer does not resolve
// leaves the level it gates, and every level above it, unmet.
func (in *evidence) criterionExtra(d *Declaration, verdict *Verdict) {
	for _, extra := range d.Extra {
		if extra.Pointer == "" {
			continue // a dated attestation is the evidence; staleness is the reviewer's.
		}
		problem := in.extraPointer(d.ID, extra.Pointer)
		if problem == "" {
			continue
		}
		reason := "extra criterion '" + extra.Criterion + "': " + problem
		for _, entry := range levelNames {
			if entry.value >= extra.Gates {
				verdict.Unmet[entry.value] = append(verdict.Unmet[entry.value], reason)
			}
		}
	}
}

// extraPointer resolves a pointer as its own field would: an interop entry
// through the suite catalog and, as it counts toward a level, its recorded
// green run (D-6); a test item through testItem; anything else as a
// repository path.
func (in *evidence) extraPointer(id, pointer string) string {
	if in.isInteropPointer(pointer) {
		if problem := in.interopItem(pointer); problem != "" {
			return problem
		}
		return in.interopRun(id, pointer)
	}
	if strings.Contains(pointer, goTestSeparator) {
		return in.testItem(pointer)
	}
	if strings.HasSuffix(pointer, ".ci") {
		return in.testItem(pointer)
	}
	return in.repoPath(pointer)
}
