// VALIDATES: Problem refuses a spec claiming a design (design, ready,
// in-progress, verification) whose Existing Mechanisms section is absent or
// holds no data row, and passes one with a row and every status that owes none.
// PREVENTS: a design proposed from first principles beside a mechanism Ze
// already has, with no analysis a reviewer can refuse it against.
package specmechanisms

import (
	"strings"
	"testing"
)

const mechanismsTable = "| Mechanism | Where | What it does today | This spec |\n|---|---|---|---|\n"

const mechanismsRow = "| DirectBridge | `pkg/plugin/rpc/bridge.go` DirectBridge | In-process path for a compiled-in plugin | Extends: the new hot path rides it |\n"

// mechanismsSpec builds a spec with status and, when section is not "", an
// Existing Mechanisms section holding it, followed by the next section.
func mechanismsSpec(status, section string) string {
	spec := "# Spec: widget\n\n| Field | Value |\n|---|---|\n| Status | " + status + " |\n\n## Current Behavior\n\nnothing\n\n"
	if section != "" {
		spec += "## Existing Mechanisms (MANDATORY)\n\n" + section + "\n"
	}
	return spec + "## Data Flow\n\n| A | B |\n|---|---|\n| entry | exit |\n"
}

// TestProblemJudgesTheSectionByStatus is the table. The empty-table and
// placeholder cases carry a table after the section (Data Flow's) to prove
// the section ends at the next heading.
func TestProblemJudgesTheSectionByStatus(t *testing.T) {
	cases := []struct {
		name    string
		spec    string
		refused string
	}{
		{"design-without-section", mechanismsSpec("design", ""), "has no `## Existing Mechanisms` section"},
		{"ready-without-section", mechanismsSpec("ready", ""), "has no `## Existing Mechanisms` section"},
		{"in-progress-without-section", mechanismsSpec("in-progress", ""), "has no `## Existing Mechanisms` section"},
		{"verification-without-section", mechanismsSpec("verification", ""), "has no `## Existing Mechanisms` section"},
		{"design-empty-table", mechanismsSpec("design", mechanismsTable), "whose table holds no data row"},
		{"design-prose-only", mechanismsSpec("design", "We looked and found nothing.\n"), "whose table holds no data row"},
		{"design-template-placeholder", mechanismsSpec("design", mechanismsTable+"| [mechanism] | [file + symbol] | [today] | [extends / replaces] |\n"), "whose table holds no data row"},
		{"design-with-row", mechanismsSpec("design", mechanismsTable+mechanismsRow), ""},
		{"in-progress-with-row", mechanismsSpec("in-progress", mechanismsTable+mechanismsRow), ""},
		{"skeleton-without-section", mechanismsSpec("skeleton", ""), ""},
		{"blocked-without-section", mechanismsSpec("blocked", ""), ""},
		{"done-without-section", mechanismsSpec("done", ""), ""},
		{"deferred-without-section", mechanismsSpec("deferred", ""), ""},
		{"no-status-row", "# Spec: widget\n\n## Task\n", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			problem := Problem("plan/x/spec-widget.md", test.spec)
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

// TestProblemRefusalNamesSectionWhyAndFix holds the refusal to
// ai/rules/user-facing-errors.md: the section, the reason, and the fix.
func TestProblemRefusalNamesSectionWhyAndFix(t *testing.T) {
	problem := Problem("plan/x/spec-widget.md", mechanismsSpec("design", ""))
	for _, want := range []string{
		"Missing required section: ## Existing Mechanisms",
		"plan/x/spec-widget.md (Status design)",
		"Why: a design starts from the mechanisms Ze already has",
		"Fix: add `## Existing Mechanisms (MANDATORY)` between `## Current Behavior` and `## Data Flow`",
		"plan/templates/TEMPLATE.md",
		"plan/learned/DESIGN-HISTORY.md",
	} {
		if !strings.Contains(problem, want) {
			t.Errorf("refusal lacks %q:\n%s", want, problem)
		}
	}
}
