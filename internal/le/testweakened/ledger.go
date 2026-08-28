// Design: docs/architecture/testing/test-health.md -- the per-commit weakening ledger
// Related: testweakened.go -- pairing parsed rows with HEAD/worktree findings.
package testweakened

import (
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ContractPath is the per-commit ledger both live and fixture checks read.
const ContractPath = "test/weakened.md"

var separatorCellPattern = regexp.MustCompile(`^:?-{2,}:?$`)

// Row is one accepted weakening and its reason in the per-commit ledger.
type Row struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
	Line   int    `json:"line"`
}

func parseLedger(contents, path string) ([]Row, []string) {
	lines := strings.Split(contents, "\n")
	starts := make([]int, 0, 1)
	for index, line := range lines {
		cells, tableRow := tableCells(line)
		if tableRow && len(cells) == 2 && cells[0] == "Test" && cells[1] == "Reason" {
			starts = append(starts, index)
		}
	}
	var message textbuf.Buffer
	if len(starts) == 0 {
		return nil, []string{message.Str(path).
			Str(" has no `| Test | Reason |` table header, so no row in it can be read").String()}
	}

	problems := make([]string, 0)
	if len(starts) > 1 {
		problems = append(problems, message.Reset().Str(path).Str(" has ").Int(int64(len(starts))).
			Str(" `| Test | Reason |` tables; keep one, or the gate has two answers").String())
	}
	rows := make([]Row, 0)
	seen := make(map[string]int)
	for index := starts[0] + 1; index < len(lines); index++ {
		cells, tableRow := tableCells(lines[index])
		if !tableRow {
			break
		}
		line := index + 1
		if len(cells) != 2 {
			problems = append(problems, message.Reset().Str(path).Byte(':').Int(int64(line)).
				Str(" has ").Int(int64(len(cells))).Str(" cells; a row is `| Test | Reason |`").String())
			continue
		}
		name, reason := cells[0], cells[1]
		if separatorRow(cells) {
			continue
		}
		if name == "" {
			problems = append(problems,
				message.Reset().Str(path).Byte(':').Int(int64(line)).Str(" names no test").String())
			continue
		}
		if reason == "" {
			problems = append(problems, message.Reset().Str(path).Byte(':').Int(int64(line)).
				Str(" gives no reason for ").Str(name).
				Str("; a row with no reason accepts nothing").String())
			continue
		}
		if previous, duplicate := seen[name]; duplicate {
			problems = append(problems, message.Reset().Str(path).Byte(':').Int(int64(line)).
				Str(" names ").Str(name).Str(" again (already on line ").Int(int64(previous)).
				Str("); one test, one reason").String())
			continue
		}
		seen[name] = line
		rows = append(rows, Row{Name: name, Reason: reason, Line: line})
	}
	return rows, problems
}

// ParseLedger parses either per-commit test ledger with the one canonical
// two-column table contract.
func ParseLedger(contents, path string) ([]Row, []string) {
	return parseLedger(contents, path)
}

// RowMatches reports whether a ledger name covers one package/test pair. It
// carries no Path, so a path-scoped row (see scopedRowMatches in
// testweakened.go) never matches through this entry point; callers inside this
// package that need scoped rows to match call rowMatches directly with a
// Finding carrying Path.
func RowMatches(rowName, packageName, testName string) bool {
	return rowMatches(rowName, Finding{Package: packageName, Name: testName})
}

func tableCells(line string) ([]string, bool) {
	body := strings.TrimSpace(line)
	if !strings.HasPrefix(body, "|") {
		return nil, false
	}
	parts := strings.Split(body, "|")
	if len(parts) != 0 && strings.TrimSpace(parts[0]) == "" {
		parts = parts[1:]
	}
	if len(parts) != 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts, true
}

func separatorRow(cells []string) bool {
	filled := 0
	for _, cell := range cells {
		if cell == "" {
			continue
		}
		filled++
		if !separatorCellPattern.MatchString(cell) {
			return false
		}
	}
	return filled != 0
}
