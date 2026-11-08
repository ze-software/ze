// Design: docs/contributing/spec-workflow.md -- the spec template and the checks that judge it
// Overview: specmechanisms.go -- Problem, which calls this judgment
// Related: criteria.go -- the other judgment a current-template spec owes

package specmechanisms

import (
	"strings"
)

// CensusHeading is the Changed-Wire Census heading under Data Flow, as
// plan/templates/TEMPLATE.md spells it.
const CensusHeading = "### Changed-Wire Census"

// searchEvidence opens the census line that records the search scope, terms
// and positive control the rows were derived from.
const searchEvidence = "**Search evidence:**"

// wireSpeakers is what the census rows say about one wire: whether some row
// names its writer and some row names its reader.
type wireSpeakers struct {
	wire   string
	writer bool
	reader bool
}

// censusProblem answers the refusal for a current-template spec whose
// Changed-Wire Census is absent, empty, or names a wire with no writer or no
// reader, and "" when the census holds. It judges structure the spec states
// about itself: every wire a census lists has two ends, so a census that names
// a wire and omits the only row for one end has omitted a known speaker,
// whichever language that speaker is written in. It cannot know a speaker the
// census never mentions; the harness-model design judgment compares the rows
// with the source.
func censusProblem(name, status, text string) string {
	lines, found := sectionLines(text, CensusHeading)
	if !found {
		return censusRefusal(name, status, "has no `"+CensusHeading+"` section under `## Data Flow`")
	}
	rows, prose := tableRows(lines)
	if len(rows) == 0 {
		if !statesReason(prose) {
			return censusRefusal(name, status, "has a census with no data row and no stated reason that no wire changes")
		}
		return ""
	}
	if !hasSearchEvidence(prose) {
		return censusRefusal(name, status, "has census rows but no filled `"+searchEvidence+"` line naming the search scope, terms and positive control")
	}
	wires, problems := censusWires(rows)
	for _, wire := range wires {
		if !wire.writer {
			problems = append(problems, "wire `"+wire.wire+"` has a reader row and no writer row")
		}
		if !wire.reader {
			problems = append(problems, "wire `"+wire.wire+"` has a writer row and no reader row")
		}
	}
	if len(problems) == 0 {
		return ""
	}
	return censusRefusal(name, status, "omits a speaker: "+strings.Join(problems, "; "))
}

// censusWires folds the census rows into one entry per wire, in first-seen
// order, and answers the row problems it met. A Wire cell of `same` or `"`
// continues the wire of the row above, as specs write a wire's second end.
func censusWires(rows []tableRow) ([]wireSpeakers, []string) {
	var wires []wireSpeakers
	var problems []string
	current := ""
	for _, row := range rows {
		wire := row.cell("Wire")
		if continuesWire(wire) {
			wire = current
		}
		if prompt(wire) {
			problems = append(problems, "a row names no wire")
			continue
		}
		// A cell usually spells the wire as code (`status.json`). The identity
		// drops those backticks, so the same wire written with and without them
		// is one wire, and a refusal that quotes it does not quote it twice.
		wire = strings.Trim(wire, "`")
		current = wire
		role := strings.ToLower(row.cell("Role"))
		writer := strings.Contains(role, "writer")
		reader := strings.Contains(role, "reader")
		if !writer && !reader {
			problems = append(problems, "the row for `"+wire+"` names neither writer nor reader in its Role cell")
			continue
		}
		if prompt(row.cell("Carrier")) {
			problems = append(problems, "the "+strings.TrimSpace(role)+" row for `"+wire+"` names no carrier (file and symbol, or script entry)")
			continue
		}
		index := wireIndex(wires, wire)
		if index < 0 {
			wires = append(wires, wireSpeakers{wire: wire})
			index = len(wires) - 1
		}
		wires[index].writer = wires[index].writer || writer
		wires[index].reader = wires[index].reader || reader
	}
	return wires, problems
}

// continuesWire reports whether a Wire cell refers to the row above.
func continuesWire(cell string) bool {
	switch strings.ToLower(strings.TrimSpace(cell)) {
	case "same", `"`, "ditto":
		return true
	}
	return false
}

// wireIndex answers the position of wire in wires, and -1 when it is absent.
func wireIndex(wires []wireSpeakers, wire string) int {
	for index, known := range wires {
		if known.wire == wire {
			return index
		}
	}
	return -1
}

// hasSearchEvidence reports whether a prose line records the search the rows
// came from, beyond the template's bracketed prompt.
func hasSearchEvidence(prose []string) bool {
	for _, line := range prose {
		rest, found := strings.CutPrefix(line, searchEvidence)
		if !found {
			continue
		}
		if !prompt(rest) {
			return true
		}
	}
	return false
}

// statesReason reports whether the census prose carries a reason, as the
// template asks of a spec whose work changes no wire: a line that is not the
// template's unfilled search-evidence prompt. Its wording is not judged.
func statesReason(prose []string) bool {
	for _, line := range prose {
		rest, evidence := strings.CutPrefix(line, searchEvidence)
		if !evidence {
			return true
		}
		if !prompt(rest) {
			return true
		}
	}
	return false
}

// censusRefusal names the section, why the spec owes it, and the fix
// (ai/rules/user-facing-errors.md).
func censusRefusal(name, status, what string) string {
	return "Incomplete section: " + CensusHeading + " -- " + name + " (Status " + status + ") " + what + ".\n" +
		"  Why: a changed wire breaks every reader and writer that still speaks the old form, in Go, shell, Python\n" +
		"  or any other carrier, and Files to Modify is the edit set, not that population. A spec from status\n" +
		"  design onward that is written from the current template owes the census (plan/templates/TEMPLATE.md).\n" +
		"  Fix: search the wire's format, fields and encode/decode entry points, record the scope, terms and a\n" +
		"  known producer or consumer on the `" + searchEvidence + "` line, and give every wire one row per writer and\n" +
		"  per reader: | Wire | Role (writer / reader) | Carrier (file + symbol, or script entry) | Source evidence |\n" +
		"  Migration or preservation disposition |. A reader outside the repository (a peer, a third-party\n" +
		"  plugin) is still a row. If no wire changes, delete the rows and state the source-supported reason."
}
