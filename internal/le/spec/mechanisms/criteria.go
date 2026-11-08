// Design: docs/contributing/spec-workflow.md -- the spec template and the checks that judge it
// Overview: specmechanisms.go -- Problem, which calls this judgment
// Related: census.go -- the other judgment a current-template spec owes

package specmechanisms

import (
	"slices"
	"strings"
)

// CriteriaHeading is the Acceptance Criteria section heading.
const CriteriaHeading = "## Acceptance Criteria"

// Column prefixes of the current template's Acceptance Criteria header:
// | AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |.
const (
	expectedColumn  = "Expected Behavior"
	survivorsColumn = "Permitted Survivors"
)

// clauseBreaks turn the separators of an Expected Behavior cell into clause
// boundaries. A conjunction or condition word stays at the head of its clause,
// so absenceOnly can read it.
var clauseBreaks = strings.NewReplacer(
	"; ", "\x00", ", ", "\x00", ": ", "\x00", ". ", "\x00", " -- ", "\x00", " — ", "\x00",
	" and ", "\x00and ", " but ", "\x00but ", " so ", "\x00so ",
	" when ", "\x00when ", " if ", "\x00if ", " unless ", "\x00unless ",
	" until ", "\x00until ", " while ", "\x00while ", " because ", "\x00because ",
	" after ", "\x00after ", " before ", "\x00before ", " once ", "\x00once ",
)

// coordinators open a clause that still states an outcome.
var coordinators = []string{"and", "but", "so", "then", "or", "yet"}

// conditions open a clause that states when the outcome holds, not the
// outcome: "returns ErrShort when no byte remains" observes the error.
var conditions = []string{"when", "if", "unless", "until", "while", "because", "after", "before", "once", "since", "where"}

// negations mark a clause that states an absence.
var negations = []string{"no", "not", "never", "nothing", "none", "neither", "nor", "without", "cannot"}

// survivorsTable reports whether the spec's Acceptance Criteria section holds a
// table whose header carries the Permitted Survivors column.
func survivorsTable(text string) bool {
	lines, found := sectionLines(text, CriteriaHeading)
	if !found {
		return false
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		for _, cell := range cellsOf(trimmed) {
			if strings.HasPrefix(cell, survivorsColumn) {
				return true
			}
		}
	}
	return false
}

// criteriaProblem answers the refusal for a current-template spec whose
// Acceptance Criteria section is absent, has no Permitted Survivors table, or
// holds a row whose Expected Behavior is empty or states only an absence, or
// whose Permitted Survivors cell is empty. "" when every row holds. A criterion
// that dropping every input would satisfy proves no useful output, so each row
// owes a positive observable and the population it leaves standing.
func criteriaProblem(name, status, text string) string {
	lines, found := sectionLines(text, CriteriaHeading)
	if !found {
		return criteriaRefusal(name, status, "has no `"+CriteriaHeading+"` section")
	}
	rows, _ := tableRows(lines)
	var problems []string
	judged := 0
	for _, row := range rows {
		if !headed(row.header, survivorsColumn) {
			continue
		}
		judged++
		id := row.cells[0]
		expected := row.cell(expectedColumn)
		if prompt(expected) {
			problems = append(problems, id+" states no Expected Behavior")
		}
		if !prompt(expected) && absenceOnly(expected) {
			problems = append(problems, id+" states only an absence (\""+expected+"\")")
		}
		if prompt(row.cell(survivorsColumn)) {
			problems = append(problems, id+" leaves Permitted Survivors and Why empty")
		}
	}
	if judged == 0 {
		return criteriaRefusal(name, status, "has no Acceptance Criteria row under a `| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |` header")
	}
	if len(problems) == 0 {
		return ""
	}
	return criteriaRefusal(name, status, "has a criterion that proves no output: "+strings.Join(problems, "; "))
}

// headed reports whether a header carries a cell starting with column.
func headed(header []string, column string) bool {
	for _, cell := range header {
		if strings.HasPrefix(cell, column) {
			return true
		}
	}
	return false
}

// absenceOnly reports whether every outcome clause of an Expected Behavior
// cell states an absence. Condition clauses are skipped, because they say when
// the outcome holds; a cell made only of conditions states no absence and is
// not refused here. One clause free of negation is the positive observable.
func absenceOnly(expected string) bool {
	outcomes := 0
	for clause := range strings.SplitSeq(clauseBreaks.Replace(strings.ToLower(expected)), "\x00") {
		words := clauseWords(clause)
		if len(words) > 0 && member(coordinators, words[0]) {
			words = words[1:]
		}
		if len(words) == 0 {
			continue
		}
		if member(conditions, words[0]) {
			continue
		}
		outcomes++
		if !negated(words) {
			return false
		}
	}
	return outcomes > 0
}

// clauseWords answers a clause's words with surrounding punctuation removed.
// A backticked identifier such as `no-op` keeps its hyphen and is no negation.
func clauseWords(clause string) []string {
	var words []string
	for field := range strings.FieldsSeq(clause) {
		word := strings.Trim(field, ".,;:()\"'*_`")
		if word != "" {
			words = append(words, word)
		}
	}
	return words
}

// negated reports whether a clause carries a negation word or a contraction
// ending in n't.
func negated(words []string) bool {
	for _, word := range words {
		if member(negations, word) {
			return true
		}
		if strings.HasSuffix(word, "n't") {
			return true
		}
	}
	return false
}

// member reports whether word is in set.
func member(set []string, word string) bool {
	return slices.Contains(set, word)
}

// criteriaRefusal names the section, why the spec owes it, and the fix
// (ai/rules/user-facing-errors.md).
func criteriaRefusal(name, status, what string) string {
	return "Incomplete section: " + CriteriaHeading + " -- " + name + " (Status " + status + ") " + what + ".\n" +
		"  Why: a criterion written as an absence (\"no X\", \"never Y\") is met by code that drops or refuses every\n" +
		"  input, so it proves no useful output. A spec from status design onward that is written from the\n" +
		"  current template owes positive criteria (plan/templates/TEMPLATE.md).\n" +
		"  Fix: state in Expected Behavior the output a user or test observes (the answer, the bytes, the state),\n" +
		"  keeping the absence as a second clause if it matters, and name in Permitted Survivors and Why what\n" +
		"  remains permitted and why, or why nothing does."
}
