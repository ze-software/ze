// Design: docs/contributing/spec-workflow.md -- the spec template and the checks that judge it
// Related: ../../hookruntime/specmechanisms.go -- the edit-time caller
// Related: ../../commit/specmechanisms.go -- the commit-time caller
// Detail: census.go -- the Changed-Wire Census judgment
// Detail: criteria.go -- the Acceptance Criteria judgment
//
// Package specmechanisms judges the structural floor of a spec's design: its
// Existing Mechanisms analysis, the table naming each mechanism Ze already has
// that does part of the job, and how the design extends or replaces it
// (ai/META-DECISIONS.md, "How agents work": "Every spec carries that analysis
// in its own section, and the spec tooling refuses a spec without it"); and,
// for a spec written from the current template, its Changed-Wire Census and
// its positive Acceptance Criteria (census.go, criteria.go). The harness-model
// design judgment still owns their meaning: this package refuses what can be
// judged from the spec's own structure, and proves no completeness.
//
// It is called only for a spec somebody is writing: the pre-write hook judges
// the content an edit proposes, and ./le commit create judges the specs a
// commit carries. No whole-tree check calls it, so a spec that predates the
// rule is refused only when it is next edited, never in another session's
// commit that does not touch it.
package specmechanisms

import (
	"slices"
	"strings"

	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// Heading is the section's heading prefix. The template spells it
// `## Existing Mechanisms (MANDATORY)`; any suffix after the prefix is accepted,
// as the other required sections accept theirs.
const Heading = "## Existing Mechanisms"

// gatedStatuses are the statuses whose spec claims a design. A skeleton has no
// design yet, a blocked spec waits on something else, and a done spec is
// closed, so none of those owes the analysis.
var gatedStatuses = []string{"design", "ready", "in-progress", "verification"}

// Problem answers the refusal for a spec whose status claims a design and whose
// Existing Mechanisms section is absent or holds no data row, or, when the spec
// is written from the current template (currentTemplate), whose Changed-Wire
// Census or Acceptance Criteria fail their structural judgment. It answers ""
// when the spec satisfies every rule or its status owes none. Several refusals
// are joined by a blank line. name is the path the refusal names.
func Problem(name, text string) string {
	status, err := specstatus.MetadataStatus(text)
	if err != nil {
		// Metadata validation owns missing or malformed declarations; this
		// predicate judges only a successfully declared design status.
		return ""
	}
	if !slices.Contains(gatedStatuses, status) {
		return ""
	}
	var problems []string
	if problem := mechanismsProblem(name, status, text); problem != "" {
		problems = append(problems, problem)
	}
	if currentTemplate(text) {
		if problem := censusProblem(name, status, text); problem != "" {
			problems = append(problems, problem)
		}
		if problem := criteriaProblem(name, status, text); problem != "" {
			problems = append(problems, problem)
		}
	}
	return strings.Join(problems, "\n\n")
}

// currentTemplate reports whether the spec carries either structure the
// current template introduced: a Changed-Wire Census heading, or an Acceptance
// Criteria table with a Permitted Survivors column. A spec written before the
// template carried them holds neither, and owes neither judgment until it
// adopts one; a spec that adopts either owes both, so dropping one does not
// escape the other. This is the scope the template change itself drew: the
// rule binds the specs written to it, never the hundreds that predate it.
func currentTemplate(text string) bool {
	if _, found := sectionLines(text, CensusHeading); found {
		return true
	}
	return survivorsTable(text)
}

// mechanismsProblem answers the Existing Mechanisms refusal, or "".
func mechanismsProblem(name, status, text string) string {
	section, found := sectionLines(text, Heading)
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

// sectionLines answers the lines under the first line that starts with heading,
// up to the next heading of the same level or above, and whether the heading
// exists. The level is the count of `#` that opens heading, so `## X` ends at
// the next `## ` or `# ` line and `### X` also ends at the next `### `.
func sectionLines(text, heading string) ([]string, bool) {
	level := headingLevel(heading)
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if !strings.HasPrefix(line, heading) {
			continue
		}
		body := lines[index+1:]
		for end, rest := range body {
			next := headingLevel(rest)
			if next == 0 {
				continue
			}
			if next <= level {
				return body[:end], true
			}
		}
		return body, true
	}
	return nil, false
}

// headingLevel answers the level of a markdown heading line (`## X` is 2), and
// 0 for a line that is no heading: a run of `#` not followed by a space.
func headingLevel(line string) int {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 {
		return 0
	}
	if level == len(line) {
		return 0
	}
	if line[level] != ' ' {
		return 0
	}
	return level
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

// cellsOf answers the trimmed cells of one table line. An escaped pipe (`\|`)
// stays inside its cell, as markdown renders it, so a cell quoting a shell
// pipeline does not shift the columns after it.
func cellsOf(line string) []string {
	escaped := strings.ReplaceAll(strings.Trim(line, "|"), `\|`, "\x00")
	parts := strings.Split(escaped, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(strings.ReplaceAll(part, "\x00", `\|`)))
	}
	return cells
}

// tableRow is one data row of a markdown table, with the header it sits under.
type tableRow struct {
	header []string
	cells  []string
}

// cell answers the row's cell under the first header cell that starts with
// column, and "" when the header has no such column or the row is short.
func (row tableRow) cell(column string) string {
	for index, name := range row.header {
		if !strings.HasPrefix(name, column) {
			continue
		}
		if index < len(row.cells) {
			return row.cells[index]
		}
		return ""
	}
	return ""
}

// tableRows answers every data row of every table in lines, each with its own
// header, skipping alignment rows and the template's bracketed placeholder
// rows. A table starts at the first `|` line after a line that is not one, so
// two tables in one section keep their own headers. prose answers the
// non-table, non-blank lines outside HTML comments.
func tableRows(lines []string) (rows []tableRow, prose []string) {
	var header []string
	inTable := false
	inComment := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inComment {
			inComment = !strings.Contains(trimmed, "-->")
			continue
		}
		if strings.HasPrefix(trimmed, "<!--") {
			inComment = !strings.Contains(trimmed, "-->")
			inTable = false
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			inTable = false
			if trimmed != "" {
				prose = append(prose, trimmed)
			}
			continue
		}
		cells := cellsOf(trimmed)
		if !inTable {
			inTable = true
			header = cells
			continue
		}
		if separator(cells) {
			continue
		}
		if placeholder(cells) {
			continue
		}
		rows = append(rows, tableRow{header: header, cells: cells})
	}
	return rows, prose
}

// prompt reports whether a cell is unfilled: empty, a dash, or the template's
// bracketed prompt such as `[source site]`.
func prompt(cell string) bool {
	trimmed := strings.TrimSpace(cell)
	switch trimmed {
	case "", "-", "--", "—", "n/a", "N/A":
		return true
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return true
	}
	return false
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
