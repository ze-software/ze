// Design: docs/architecture/core-design.md -- the published hook check tables
// Overview: coverage.go -- the gate map this claim is compared against
// Overview: coverage_report.go -- the answer these problems are printed in
//
// hooktable.go checks the published claim against the native Go checks.
//
// `.claude/hooks/README.md` publishes one table for each hookruntime source
// that owns registered checks. Its Check and Enforces columns repeat the
// registry function names and binding comments, and its remaining column says
// what each check does, which is why the table is authored rather than
// generated.

package rules

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// hookDoc is the document that publishes the mapping, and tableHead opens each
// of its sub-tables.
const (
	hookDoc   = ".claude/hooks/README.md"
	tableHead = "| Check | Enforces |"
)

var (
	// A `##` to `####` heading naming a Go source opens that source's
	// sub-table. The heading is how a table is matched to its checks.
	sourceHeading = regexp.MustCompile("^#{2,4}[ \t\n\r\f\v].*`(?:internal/le/hookruntime/)?([A-Za-z0-9_-]+\\.go)`")
	backticked    = regexp.MustCompile("`([^`]+)`")
)

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

// publishedRows maps each hookruntime Go source name to its published rows. It
// reads the first `Check | Enforces` table below that source's heading.
func publishedRows(docText string) map[string][]publishedRow {
	out := map[string][]publishedRow{}
	current := ""
	collecting := false
	for number, line := range strings.Split(docText, "\n") {
		if heading := sourceHeading.FindStringSubmatch(line); heading != nil {
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

// hookTableProblems answers where the published hook tables differ from the
// registered native checks and their bindings.
//
// It fails closed for a missing or empty source table, a stale source/check row,
// a missing registered check row, and an Enforces cell that disagrees with the
// function's binding comments at rule granularity.
func hookTableProblems(gm gateMap, docText string, sources map[string]string) []string {
	var problems []string
	var tb textbuf.Buffer

	stems := map[string]bool{}
	for ref := range gm.Points {
		stem, _, _ := strings.Cut(ref, "/")
		stems[stem] = true
	}
	tables := publishedRows(docText)
	rosters := map[string]map[string]bool{}
	bindingsByFile := map[string][]Binding{}
	for _, binding := range gm.Bindings {
		if binding.Check == noCheck {
			continue
		}
		if rosters[binding.File] == nil {
			rosters[binding.File] = map[string]bool{}
		}
		rosters[binding.File][binding.Check] = true
		bindingsByFile[binding.File] = append(bindingsByFile[binding.File], binding)
	}
	for _, name := range sortedKeys(tables) {
		if _, exists := sources[name]; !exists || len(rosters[name]) == 0 {
			tb.Reset()
			problems = append(problems, tb.Str(hookDoc).Str(": heading `").
				Str(name).Str("` names no source with registered native checks").String())
		}
	}

	for _, name := range sortedKeys(rosters) {
		roster := rosters[name]
		bindings := bindingsByFile[name]
		rows := tables[name]
		if len(rows) == 0 {
			tb.Reset()
			problems = append(problems, tb.Str(hookDoc).Str(": no `").Str(tableHead).
				Str("...` table under a heading naming ").Str(name).Str("; ").
				Int(int64(len(roster))).Str(" check(s) are published nowhere").String())
			continue
		}

		seen := map[string]int{}
		for _, row := range rows {
			if first, twice := seen[row.Check]; twice {
				tb.Reset()
				problems = append(problems, tb.Str(hookDoc).Byte(':').
					Int(int64(row.Line)).Str(": `").Str(row.Check).
					Str("` has a second row (the first is line ").Int(int64(first)).Byte(')').String())
				continue
			}
			seen[row.Check] = row.Line
			if !roster[row.Check] {
				tb.Reset()
				problems = append(problems, tb.Str(hookDoc).Byte(':').
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
			problems = append(problems, tb.Str(hookDoc).Str(": `").Str(check).
				Str("` has no row under the heading naming ").Str(name).String())
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
	return tb.Str(hookDoc).Byte(':').Int(int64(row.Line)).
		Str(": `").Str(row.Check).Str("` Enforces names ").Str(held).
		Str(", its bindings say ").Str(wanted).String()
}
