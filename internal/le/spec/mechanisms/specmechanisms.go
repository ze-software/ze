// Design: docs/contributing/spec-workflow.md -- the spec template and the checks that judge it
// Related: ../../hookruntime/specmechanisms.go -- the edit-time caller
// Related: ../../commit/specmechanisms.go -- the commit-time caller
//
// Package specmechanisms judges whether a spec carries its Existing
// Mechanisms analysis: the table naming each mechanism Ze already has that does
// part of the job, and how the design extends or replaces it
// (ai/META-DECISIONS.md, "How agents work": "Every spec carries that analysis
// in its own section, and the spec tooling refuses a spec without it").
//
// It is called only for a spec somebody is writing: the pre-write hook judges
// the content an edit proposes, and ./le commit create judges the specs a
// commit carries. No whole-tree check calls it, so a spec that predates the
// rule is refused only when it is next edited, never in another session's
// commit that does not touch it.
package specmechanisms

import (
	"regexp"
	"slices"
	"strings"
)

// Heading is the section's heading prefix. The template spells it
// `## Existing Mechanisms (MANDATORY)`; any suffix after the prefix is accepted,
// as the other required sections accept theirs.
const Heading = "## Existing Mechanisms"

// gatedStatuses are the statuses whose spec claims a design. A skeleton has no
// design yet, a blocked spec waits on something else, and a done spec is
// closed, so none of those owes the analysis.
var gatedStatuses = []string{"design", "ready", "in-progress", "verification"}

var statusRow = regexp.MustCompile(`(?m)^\| Status \| *([a-z-]+)`)

// Problem answers the refusal for a spec whose status claims a design and whose
// Existing Mechanisms section is absent or holds no data row, and "" when the
// spec satisfies the rule or its status does not owe it. name is the path the
// refusal names.
func Problem(name, text string) string {
	match := statusRow.FindStringSubmatch(text)
	if match == nil {
		// The status validator refuses a spec with no Status row; this check
		// judges only a declared design status.
		return ""
	}
	status := match[1]
	if !slices.Contains(gatedStatuses, status) {
		return ""
	}
	section, found := section(text)
	if !found {
		return refusal(name, status, "has no `"+Heading+"` section")
	}
	if !hasDataRow(section) {
		return refusal(name, status, "has a `"+Heading+"` section whose table holds no data row")
	}
	return ""
}

// refusal names the section, why the spec owes it, and the fix
// (ai/rules/user-facing-errors.md).
func refusal(name, status, what string) string {
	return "Missing required section: " + Heading + " -- " + name + " (Status " + status + ") " + what + ".\n" +
		"  Why: a design starts from the mechanisms Ze already has, and a spec from status design onward claims\n" +
		"  one (ai/META-DECISIONS.md, \"How agents work\"). A design proposed beside an existing mechanism,\n" +
		"  instead of extending it, is refused at review.\n" +
		"  Fix: add `## Existing Mechanisms (MANDATORY)` between `## Current Behavior` and `## Data Flow`, as\n" +
		"  plan/templates/TEMPLATE.md shows, with one table row per existing mechanism that already does part of\n" +
		"  the job: | Mechanism | Where (file + symbol, or page + section) | What it does today | This spec\n" +
		"  (extends, replaces, or does not apply, with the reason) |. Find them in ai/doors/, docs/architecture/\n" +
		"  and plan/learned/DESIGN-HISTORY.md."
}

// section answers the lines under the Existing Mechanisms heading up to the
// next heading of level two or above, and whether the heading exists.
func section(text string) ([]string, bool) {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if !strings.HasPrefix(line, Heading) {
			continue
		}
		body := lines[index+1:]
		for end, rest := range body {
			if strings.HasPrefix(rest, "## ") {
				return body[:end], true
			}
			if strings.HasPrefix(rest, "# ") {
				return body[:end], true
			}
		}
		return body, true
	}
	return nil, false
}

// hasDataRow reports whether the section's table holds a row after its header
// and separator that is not the template's placeholder, whose every cell is a
// bracketed prompt such as `[mechanism]`.
func hasDataRow(lines []string) bool {
	header := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		if !header {
			header = true
			continue
		}
		cells := cellsOf(trimmed)
		if separator(cells) {
			continue
		}
		if placeholder(cells) {
			continue
		}
		return true
	}
	return false
}

// cellsOf answers the trimmed cells of one table line.
func cellsOf(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

// separator reports whether every cell is a markdown alignment row (`---`).
func separator(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return true
}

// placeholder reports whether every non-empty cell is a bracketed prompt, or
// the row is empty.
func placeholder(cells []string) bool {
	for _, cell := range cells {
		if cell == "" {
			continue
		}
		if !strings.HasPrefix(cell, "[") {
			return false
		}
		if !strings.HasSuffix(cell, "]") {
			return false
		}
	}
	return true
}
