// Design: docs/architecture/core-design.md -- the published Hook-to-Rule Mapping
// Overview: coverage.go -- the gate map this claim is compared against
// Overview: coverage_report.go -- the answer these problems are printed in
//
// hooktable.go checks the PUBLISHED claim against the bindings.
//
// `ai/rules/repo-maintenance.md` publishes one table for each PreToolUse
// dispatcher. Its Check and Enforces columns repeat the binding comments. These
// copies drifted: four checks had no row, and one row named a deleted function.
//
// The table is NOT generated because each row includes authored prose in the
// Triggers on and What it does cells. A generator would have to edit authored
// markdown with escaped pipes and trailing HTML comments. This check gives most
// of the same guarantee. The Check column must match the roster, and Enforces
// must match bindings at RULE GRANULARITY.
//
// Check enumeration requires a read of Python. Go has no Python parser, so this
// code scans only four shapes: top-level functions, a module CHECKS table,
// calls from main(), and prefixed names. It fails CLOSED when the scan fails.
// Otherwise, no check can verify the rows for that dispatcher.

package rules

import (
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// docRule is the rule that publishes the mapping, and tableHead opens each of
// its sub-tables.
const (
	docRule   = "repo-maintenance"
	tableHead = "| Check | Enforces |"
)

var (
	// A `##` to `####` heading naming a dispatcher by its file name opens that
	// dispatcher's sub-table. The heading is how a table is MATCHED to a
	// dispatcher, so no list of table locations is kept.
	dispatchHeading = regexp.MustCompile("^#{2,4}[ \t\n\r\f\v].*`([A-Za-z0-9_-]+\\.py)`")
	backticked      = regexp.MustCompile("`([^`]+)`")
	// A check is a top-level `c_` or `check_` function. Every check must also
	// carry a binding. Thus, even a check without one still needs a row.
	checkDef = regexp.MustCompile(`(?m)^def ((?:c|check)_[a-z0-9_]+)[ \t\n\r\f\v]*\(`)
	// A call whose callee is a bare NAME. Its preceding character cannot be a
	// dot or word character. This excludes `sys.exit(`, as Python's ast.Name
	// test does.
	bareCall = regexp.MustCompile(`(^|[^.\w])([A-Za-z_]\w*)[ \t\n\r\f\v]*\(`)
	// A module-level def, at column zero, async or not.
	moduleDef = regexp.MustCompile(`(?m)^(?:async )?def ([A-Za-z_]\w*)[ \t\n\r\f\v]*\(`)
	// One Python identifier and nothing else, which is what an element of the
	// CHECKS table must be to name a function rather than an expression.
	pyIdentifier = regexp.MustCompile(`^[A-Za-z_]\w*$`)
)

// dispatcherChecks answers every check in one dispatcher, derived from what
// that dispatcher RUNS.
//
// Four sources form the union because dispatchers use two shapes, and the
// `c_`/`check_` prefix is not universal:
//
//   - The module CHECKS table. It is the dispatch table for two dispatchers.
//   - Top-level functions that main() calls by name when no CHECKS table exists.
//     One dispatcher calls two unprefixed gates directly. The scan excludes a
//     `_`-prefixed name because dispatcher helpers use that form.
//
//   - Any `c_` or `check_` function. A function absent from its dispatch table
//     still needs a row, and the absence must remain visible.
//   - Any function named by a binding. An unusually shaped check that declares
//     enforcement still cannot escape the published table.
//
// Prefix and binding scans alone missed unprefixed, unbound gates. Two of the
// three dispatchers already use such gates.
func dispatcherChecks(text string, bindings []Binding) (map[string]bool, error) {
	scrubbed, err := scrubPython(text)
	if err != nil {
		return nil, err
	}

	top := map[string]bool{}
	for _, found := range moduleDef.FindAllStringSubmatch(scrubbed, -1) {
		top[found[1]] = true
	}

	out := map[string]bool{}
	for _, name := range checksTable(scrubbed) {
		out[name] = true
	}
	if len(out) == 0 {
		for _, name := range callsInMain(scrubbed, top) {
			out[name] = true
		}
	}
	for _, found := range checkDef.FindAllStringSubmatch(text, -1) {
		out[found[1]] = true
	}
	for _, binding := range bindings {
		if binding.Check != noCheck {
			out[binding.Check] = true
		}
	}
	return out, nil
}

// checksTable answers the bare names listed in a module-level CHECKS tuple or
// list, which IS the dispatch table where one exists.
func checksTable(scrubbed string) []string {
	lines := strings.Split(scrubbed, "\n")
	var out []string
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed != "CHECKS = (" && trimmed != "CHECKS = [" {
			continue
		}
		closer := byte(')')
		if strings.HasSuffix(trimmed, "[") {
			closer = ']'
		}
		for _, body := range lines[i+1:] {
			if body != "" && body[0] == closer {
				break
			}
			for element := range strings.SplitSeq(body, ",") {
				if name := strings.TrimSpace(element); pyIdentifier.MatchString(name) {
					out = append(out, name)
				}
			}
		}
		break
	}
	return out
}

// callsInMain answers the top-level functions a module's main() calls by name,
// private ones excluded. It is the fallback for a dispatcher with no CHECKS
// table.
func callsInMain(scrubbed string, top map[string]bool) []string {
	lines := strings.Split(scrubbed, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "def main(") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		end = i
		break
	}

	seen := map[string]bool{}
	body := strings.Join(lines[start+1:end], "\n")
	for _, found := range bareCall.FindAllStringSubmatch(body, -1) {
		name := found[2]
		if top[name] && !strings.HasPrefix(name, "_") {
			seen[name] = true
		}
	}
	return sortedUnique(seen)
}

// scrubPython replaces each comment and string literal with spaces. It keeps the
// line structure.
//
// This prevents two errors. Calls in docstrings are not calls. Also, an
// unterminated literal is the only detectable Python syntax error here. Its
// rejection keeps enumeration fail-closed instead of returning a short roster.
func scrubPython(text string) (string, error) {
	out := []byte(text)
	i := 0
	for i < len(text) {
		switch text[i] {
		case '#':
			for i < len(text) && text[i] != '\n' {
				out[i] = ' '
				i++
			}
		case '\'', '"':
			quote := text[i : i+1]
			if strings.HasPrefix(text[i:], strings.Repeat(quote, 3)) {
				quote = strings.Repeat(quote, 3)
			}
			end := closingQuote(text, i, quote)
			if end < 0 {
				var tb textbuf.Buffer
				return "", errors.New(tb.Str("unterminated string literal at offset ").
					Int(int64(i)).String())
			}
			for j := i; j < end; j++ {
				if out[j] != '\n' {
					out[j] = ' '
				}
			}
			i = end
		default:
			i++
		}
	}
	return string(out), nil
}

// closingQuote answers the offset one past the literal that opens at start with
// the given quote, or -1 when the literal never closes.
func closingQuote(text string, start int, quote string) int {
	for i := start + len(quote); i < len(text); {
		if text[i] == '\\' {
			i += 2
			continue
		}
		if strings.HasPrefix(text[i:], quote) {
			return i + len(quote)
		}
		// A single-quoted literal cannot span a line. Answering -1 here rather
		// than running to the end of the file is what stops one stray quote
		// swallowing the whole module.
		if len(quote) == 1 && text[i] == '\n' {
			return -1
		}
		i++
	}
	return -1
}

// cells answers the cells of one markdown table row, in order, stripped. An
// escaped pipe inside a cell is content rather than a boundary.
func cells(line string) []string {
	line = strings.TrimSpace(line)
	var out []string
	start := 0
	for i := range len(line) {
		if line[i] != '|' || (i > 0 && line[i-1] == '\\') {
			continue
		}
		out = append(out, strings.TrimSpace(line[start:i]))
		start = i + 1
	}
	return append(out, strings.TrimSpace(line[start:]))
}

// publishedRow is one row of a published sub-table: where it is, the check it
// names, and the Enforces cell verbatim.
type publishedRow struct {
	Line     int
	Check    string
	Enforces string
}

// publishedRows maps each dispatcher filename to its published rows.
//
// It reads the first `Check | Enforces` table below that dispatcher's heading.
// The first non-row line ends the table.
func publishedRows(docText string) map[string][]publishedRow {
	out := map[string][]publishedRow{}
	current := ""
	collecting := false
	for number, line := range strings.Split(docText, "\n") {
		if heading := dispatchHeading.FindStringSubmatch(line); heading != nil {
			current, collecting = heading[1], false
			continue
		}
		if collecting {
			if !strings.HasPrefix(line, "|") {
				collecting = false
				continue
			}
			fields := cells(line)
			if len(fields) > 2 && strings.Trim(fields[1], "-:") != "" {
				out[current] = append(out[current], publishedRow{
					Line: number + 1, Check: strings.Trim(fields[1], "`"), Enforces: fields[2],
				})
			}
			continue
		}
		if _, seen := out[current]; current != "" && strings.HasPrefix(line, tableHead) && !seen {
			out[current] = []publishedRow{}
			collecting = true
		}
	}
	return out
}

// ruleStemsNamed answers the rule stems named by a published Enforces cell.
//
// Only a backticked `<stem>.md` token counts. A path such as
// `.claude/rules/planning.md` names a rule outside this corpus. It must not claim
// anything about `planning.md`.
func ruleStemsNamed(cell string, stems map[string]bool) map[string]bool {
	found := map[string]bool{}
	for _, token := range backticked.FindAllStringSubmatch(cell, -1) {
		if !strings.HasSuffix(token[1], ".md") {
			continue
		}
		if stem := strings.TrimSuffix(token[1], ".md"); stems[stem] {
			found[stem] = true
		}
	}
	return found
}

// hookTableProblems answers where the published Hook-to-Rule Mapping differs
// from the bindings.
//
// It fails closed at each step. A missing dispatcher table and an empty table
// are both problems. The table must name every check. An empty table names none
// but appears complete.
//
// The Enforces comparison uses RULE granularity because the cell publishes that
// level. Rebinding a check within the SAME rule leaves the cell correct. The
// binding comment is the detailed record, and gatedRegressions ratchets it.
func hookTableProblems(gm GateMap, docText string, sources map[string]string) []string {
	var problems []string
	var tb textbuf.Buffer

	stems := map[string]bool{}
	for ref := range gm.Points {
		stem, _, _ := strings.Cut(ref, "/")
		stems[stem] = true
	}
	tables := publishedRows(docText)

	for _, name := range sortedKeys(sources) {
		var bindings []Binding
		for _, binding := range gm.Bindings {
			if binding.File == name {
				bindings = append(bindings, binding)
			}
		}
		roster, err := dispatcherChecks(sources[name], bindings)
		if err != nil {
			tb.Reset()
			problems = append(problems, tb.Str(name).Str(": cannot be parsed: ").Err(err).
				Str("; its checks cannot be enumerated").String())
			continue
		}
		rows := tables[name]
		if len(rows) == 0 {
			tb.Reset()
			problems = append(problems, tb.Str(name).Str(": no `").Str(tableHead).
				Str("...` table under a heading naming it; ").Int(int64(len(roster))).
				Str(" check(s) are published nowhere").String())
			continue
		}

		seen := map[string]int{}
		for _, row := range rows {
			if first, twice := seen[row.Check]; twice {
				tb.Reset()
				problems = append(problems, tb.Str(rulesRel).Byte('/').Str(docRule).Str(".md:").
					Int(int64(row.Line)).Str(": `").Str(row.Check).
					Str("` has a second row (the first is line ").Int(int64(first)).Byte(')').String())
				continue
			}
			seen[row.Check] = row.Line
			if !roster[row.Check] {
				tb.Reset()
				problems = append(problems, tb.Str(rulesRel).Byte('/').Str(docRule).Str(".md:").
					Int(int64(row.Line)).Str(": row `").Str(row.Check).Str("` names no check in ").
					Str(name).Str("; delete the row or restore the function").String())
				continue
			}
			want := map[string]bool{}
			for _, binding := range bindings {
				if binding.Check == row.Check && binding.Ref != "" {
					stem, _, _ := strings.Cut(binding.Ref, "/")
					want[stem] = true
				}
			}
			have := ruleStemsNamed(row.Enforces, stems)
			if slices.Equal(sortedKeys(want), sortedKeys(have)) {
				continue
			}
			problems = append(problems, enforcesProblem(row, want, have))
		}
		for _, check := range sortedKeys(roster) {
			if _, published := seen[check]; published {
				continue
			}
			tb.Reset()
			problems = append(problems, tb.Str(name).Str(": `").Str(check).
				Str("` has no row in the Hook-to-Rule Mapping table").String())
		}
	}
	return problems
}

// enforcesProblem renders the disagreement between a published Enforces cell
// and the bindings behind it.
func enforcesProblem(row publishedRow, want, have map[string]bool) string {
	var tb textbuf.Buffer
	wanted := "no rule"
	if len(want) > 0 {
		quoted := make([]string, 0, len(want))
		for _, stem := range sortedKeys(want) {
			tb.Reset()
			quoted = append(quoted, tb.Byte('`').Str(stem).Str(".md`").String())
		}
		tb.Reset()
		wanted = tb.Join(quoted, ", ").String()
	}
	held := "no rule"
	if len(have) > 0 {
		tb.Reset()
		held = tb.Join(sortedKeys(have), ", ").String()
	}
	tb.Reset()
	return tb.Str(rulesRel).Byte('/').Str(docRule).Str(".md:").Int(int64(row.Line)).
		Str(": `").Str(row.Check).Str("` Enforces names ").Str(held).
		Str(", its bindings say ").Str(wanted).String()
}
