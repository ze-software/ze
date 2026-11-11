// Design: docs/contributing/spec-workflow.md -- the spec template and the checks that judge it
// VALIDATES: the design check (specmechanisms.Problem, the predicate the
// pre-write hook and ./le commit create both apply) refuses a current-template
// spec whose Changed-Wire Census omits a known writer or reader, Go or not, and
// one whose Acceptance Criteria state only an absence or leave Permitted
// Survivors empty; it admits the complete forms and leaves legacy specs alone.
// PREVENTS: a census copied from Files to Modify, and a criterion that code
// dropping every input satisfies (spec-tooling-one-off-defects P-24, P-16).
package spec

import (
	"slices"
	"strings"
	"testing"

	specmechanisms "github.com/ze-software/ze/internal/le/spec/mechanisms"
)

const proofSpecName = "plan/x/spec-widget.md"

const proofMechanisms = "## Existing Mechanisms (MANDATORY)\n\n| Mechanism | Where | What it does today | This spec |\n|---|---|---|---|\n" +
	"| status file | `status.go` writeStatus | writes the status | extends |\n\n"

const proofCensusHeader = "| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |\n" +
	"|---|---|---|---|---|\n"

// proofCensusRows are the four speakers of two wires: a Go writer read by a
// shell script, and a Python writer read by Go. Each wire has exactly one row
// per end, so omitting any one row leaves its wire with a missing speaker.
var proofCensusRows = []string{
	"| `status.json` | writer | `internal/x/status.go` writeStatus | encodes the file | change: adds `state` |\n", // <!-- doc-links: ignore (a synthetic path in a test fixture) -->
	"| same | reader | `scripts/check-status.sh` main | `jq .state` reads it | change: reads `state` |\n",
	"| `report.txt` | writer | `tools/report.py` main | prints one line per peer | preserve |\n",   // <!-- doc-links: ignore (a synthetic path in a test fixture) -->
	"| `report.txt` | reader | `internal/x/report.go` readReport | scans the lines | preserve |\n", // <!-- doc-links: ignore (a synthetic path in a test fixture) -->
}

const proofEvidence = "**Search evidence:** scope `internal`, `scripts`, `tools`; terms `status.json`, `report.txt`; control: `writeStatus` found as the writer.\n\n"

const proofCriteriaHeader = "| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |\n|-------|---|---|---|\n"

const proofCriterion = "| AC-1 | A peer reports state up | `status.json` carries `\"state\":\"up\"` | none: every peer is written |\n"

// proofSpec builds a design spec with the given census body (omitted when
// census is "-") and Acceptance Criteria rows (header omitted when criteria is
// "-"). Files to Modify names only the Go writer: the edit set, not the census.
func proofSpec(status, census, criteria string) string {
	spec := "# Spec: widget\n\n| Field | Value |\n|---|---|\n| Status | " + status + " |\n\n" +
		"## Current Behavior\n\nnothing\n\n" + proofMechanisms + "## Data Flow\n\n### Entry Point\n- a peer\n\n"
	if census != "-" {
		spec += specmechanisms.CensusHeading + "\n\n" + census + "\n"
	}
	spec += "### Integration Points\n- none\n\n## Acceptance Criteria\n\n"
	if criteria != "-" {
		spec += proofCriteriaHeader + criteria
	}
	return spec + "\n## Files to Modify\n\n- `internal/x/status.go`\n" // <!-- doc-links: ignore (a synthetic path in a test fixture) -->
}

// fullCensus is the census with every speaker.
func fullCensus() string {
	return proofEvidence + proofCensusHeader + strings.Join(proofCensusRows, "")
}

// TestWireCensusIncludesNonGoSpeakers removes each of the four census rows in
// turn, Go and non-Go alike, and expects the refusal to name the wire left with
// one end. The complete census is the positive control, and a census holding
// only the Files to Modify writer is refused like any other omission.
//
// MUTATION: (census.go, `if !wire.writer {`, `if false {`): omitting the Go or Python writer is admitted.
// MUTATION: (census.go, `if !wire.reader {`, `if false {`): omitting the shell or Go reader is admitted.
// MUTATION: (census.go, `if continuesWire(wire) {`, `if false {`): the shell reader's `same` row becomes its own wire and the complete census is refused.
// MUTATION: (specmechanisms.go, `if currentTemplate(text) {`, `if false {`): no census is judged and every omission is admitted.
func TestWireCensusIncludesNonGoSpeakers(t *testing.T) {
	if problem := specmechanisms.Problem(proofSpecName, proofSpec("design", fullCensus(), proofCriterion)); problem != "" {
		t.Fatalf("complete census refused:\n%s", problem)
	}
	omissions := []struct {
		speaker string
		want    string
	}{
		{"Go writer", "wire `status.json` has a reader row and no writer row"},
		{"shell reader", "wire `status.json` has a writer row and no reader row"},
		{"Python writer", "wire `report.txt` has a reader row and no writer row"},
		{"Go reader", "wire `report.txt` has a writer row and no reader row"},
	}
	for index, omission := range omissions {
		t.Run(omission.speaker, func(t *testing.T) {
			rows := slices.Delete(slices.Clone(proofCensusRows), index, index+1)
			// The shell row continues the wire above with `same`; once the Go
			// writer is gone it must name the wire itself.
			if index == 0 {
				rows[0] = strings.Replace(rows[0], "| same |", "| `status.json` |", 1)
			}
			census := proofEvidence + proofCensusHeader + strings.Join(rows, "")
			problem := specmechanisms.Problem(proofSpecName, proofSpec("design", census, proofCriterion))
			if !strings.Contains(problem, omission.want) {
				t.Fatalf("omitting the %s: Problem = %q, want %q", omission.speaker, problem, omission.want)
			}
			if !strings.Contains(problem, "Incomplete section: "+specmechanisms.CensusHeading) {
				t.Fatalf("refusal does not name the census section:\n%s", problem)
			}
		})
	}
	editSetOnly := proofEvidence + proofCensusHeader + proofCensusRows[0]
	problem := specmechanisms.Problem(proofSpecName, proofSpec("design", editSetOnly, proofCriterion))
	if !strings.Contains(problem, "wire `status.json` has a writer row and no reader row") {
		t.Fatalf("census copied from Files to Modify admitted: %q", problem)
	}
}

// TestWireCensusStructure covers the census forms around the speaker rule:
// the no-wire reason, an empty census, missing search evidence, a row with no
// role or carrier, a missing census beside a survivors table, and the scope
// that leaves skeletons and legacy specs unjudged.
//
// MUTATION: (specmechanisms.go, `return survivorsTable(text)`, `return false`): a spec that adopted the survivors table but dropped the census is admitted.
// MUTATION: (census.go, `if !hasSearchEvidence(prose) {`, `if false {`): census rows with no recorded search are admitted.
// MUTATION: (specmechanisms.go, `if currentTemplate(text) {`, `if true {`): the legacy spec carrying neither structure is refused.
func TestWireCensusStructure(t *testing.T) {
	cases := []struct {
		name    string
		spec    string
		refused string
	}{
		{"no-wire-reason", proofSpec("ready", "None: tooling only, no wire format changes.\n", proofCriterion), ""},
		{"heading-only", proofSpec("design", "", proofCriterion), "no data row and no stated reason"},
		{"unfilled-template", proofSpec("design", "**Search evidence:** [scope, terms]\n\n"+proofCensusHeader+"| [format] | [role] | [source site] | [finding] | [disposition] |\n", proofCriterion), "no data row and no stated reason"},
		{"rows-without-evidence", proofSpec("design", proofCensusHeader+strings.Join(proofCensusRows, ""), proofCriterion), "no filled `**Search evidence:**` line"},
		{"row-without-role", proofSpec("design", fullCensus()+"| `extra.bin` | - | `x.go` f | read | preserve |\n", proofCriterion), "names neither writer nor reader"},
		{"row-without-carrier", proofSpec("design", fullCensus()+"| `report.txt` | reader | - | read | preserve |\n", proofCriterion), "names no carrier"},
		{"survivors-table-without-census", proofSpec("in-progress", "-", proofCriterion), "has no `### Changed-Wire Census` section"},
		{"skeleton-unjudged", proofSpec("skeleton", "", ""), ""},
		{"legacy-unjudged", strings.Replace(proofSpec("design", "-", "-"), "## Acceptance Criteria\n\n",
			"## Acceptance Criteria\n\n| AC ID | Input / Condition | Expected Behavior |\n|---|---|---|\n| AC-1 | x | never panics |\n", 1), ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			problem := specmechanisms.Problem(proofSpecName, test.spec)
			if test.refused == "" {
				if problem != "" {
					t.Fatalf("Problem refused a spec the rule admits:\n%s", problem)
				}
				return
			}
			if !strings.Contains(problem, test.refused) {
				t.Fatalf("Problem = %q, want a refusal containing %q", problem, test.refused)
			}
		})
	}
}

// TestCriteriaNamePositiveBehavior judges Expected Behavior and Permitted
// Survivors cells: an absence-only outcome or an empty survivors cell is
// refused, and an outcome a test can observe, with or without a trailing
// absence or a leading condition, is admitted.
//
// MUTATION: (criteria.go, `if !negated(words) {`, `if true {`): every absence-only criterion is admitted.
// MUTATION: (criteria.go, `if member(conditions, words[0]) {`, `if false {`): "never panics when the input is short" reads its condition as the positive outcome and is admitted.
// MUTATION: (criteria.go, `if prompt(row.cell(survivorsColumn)) {`, `if false {`): an empty Permitted Survivors cell is admitted.
// MUTATION: (criteria.go, `if prompt(expected) {`, `if false {`): an empty Expected Behavior cell is admitted.
func TestCriteriaNamePositiveBehavior(t *testing.T) {
	cases := []struct {
		name     string
		expected string
		survivor string
		refused  string
	}{
		{"positive", "the parser returns ErrShort naming the offset", "none: every short input errors", ""},
		{"positive-then-absence", "the route is installed, and no withdrawal is sent", "the old route until replaced", ""},
		{"condition-then-positive", "When the peer sends X, Ze answers with a NOTIFICATION", "none", ""},
		{"positive-with-negated-condition", "returns an error when no peer matches", "none", ""},
		{"zero-is-a-measure", "zero allocations per UPDATE in the benchmark", "none", ""},
		{"no-leading", "no reply carries option 15", "none", "AC-1 states only an absence"},
		{"never-leading", "Never panics", "none", "AC-1 states only an absence"},
		{"never-with-condition", "never panics when the input is short", "none", "AC-1 states only an absence"},
		{"does-not", "does not drop the route", "none", "AC-1 states only an absence"},
		{"without", "without a log line", "none", "AC-1 states only an absence"},
		{"contraction", "the warning isn't repeated", "none", "AC-1 states only an absence"},
		{"two-absences", "no panic; nothing is written", "none", "AC-1 states only an absence"},
		{"empty-survivors", "the parser returns ErrShort", "", "AC-1 leaves Permitted Survivors and Why empty"},
		{"dash-survivors", "the parser returns ErrShort", "-", "AC-1 leaves Permitted Survivors and Why empty"},
		{"prompt-expected", "[positive observable outcome]", "none", "AC-1 states no Expected Behavior"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			row := "| AC-1 | an input | " + test.expected + " | " + test.survivor + " |\n"
			problem := specmechanisms.Problem(proofSpecName, proofSpec("design", fullCensus(), row))
			if test.refused == "" {
				if problem != "" {
					t.Fatalf("Problem refused a positive criterion:\n%s", problem)
				}
				return
			}
			if !strings.Contains(problem, test.refused) {
				t.Fatalf("Problem = %q, want a refusal containing %q", problem, test.refused)
			}
			for _, want := range []string{"Incomplete section: ## Acceptance Criteria", "Why:", "Fix:"} {
				if !strings.Contains(problem, want) {
					t.Errorf("refusal lacks %q:\n%s", want, problem)
				}
			}
		})
	}
	problem := specmechanisms.Problem(proofSpecName, proofSpec("ready", fullCensus(), ""))
	if !strings.Contains(problem, "has no Acceptance Criteria row under") {
		t.Fatalf("survivors header with no row admitted: %q", problem)
	}
}
