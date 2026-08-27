// Design: docs/architecture/core-design.md -- native Go gates run through le
// Overview: hookcheck.go -- selftest orchestration and structured report
//
// The producer's embedded GOLDEN maps remain the migration boundary. This file
// parses those maps as data and independently derives every verdict in Go. It
// never imports or starts a Python dispatcher.
package hookcheck

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	bashRowsExpected          = 82
	writeEditRowsExpected     = 92
	weakeningRowsExpected     = 10
	postWriteEditRowsExpected = 24
)

type goldenRow struct {
	name string
	code int
}

var (
	expensivePipe   = regexp.MustCompile(`(?s)(?:^|[;&\n])\s*(?:timeout\s+(?:-k\s+\S+\s+)?\S+\s+|nice\s+-n\s+\S+\s+)?(?:go\s+test\b|make\s+ze-(?:precommit-verify|unit-hook-test)\b|(?:(?:\./)?bin/ze-test|(?:/\S*/)?tmp/session/[0-9]{4}-[0-9]{2}-[0-9]{2}-[^/\s]+/bin/ze-test)\b|python3\s+scripts/dev/hook-parity-check\.py\b)[^;&\n]*?(?:\||\|&)\s*(?:head|tail|grep)\b`)
	rawGoTest       = regexp.MustCompile(`(?:^|[;&\n])\s*(?:timeout\s+(?:-k\s+\S+\s+)?\S+\s+|nice\s+-n\s+\S+\s+)?go\s+test\b`)
	rawLint         = regexp.MustCompile(`(?:^|[;&\n])\s*(?:timeout\s+\S+\s+)?golangci-lint\s+run\b`)
	rawZeTest       = regexp.MustCompile(`(?:^|[;&\n])\s*(?:timeout\s+\S+\s+)?(?:(?:\./)?bin/ze-test(?:-linux-[^\s/]*)?|(?:/\S*/)?tmp/session/[0-9]{4}-[0-9]{2}-[0-9]{2}-[^/\s]+/bin/ze-test(?:-linux-[^\s/]*)?)\b`)
	rawPythonTest   = regexp.MustCompile(`(?:^|[;&\n])\s*(?:timeout\s+\S+\s+)?python3\s+\S*_test\.py\b`)
	rootScratch     = regexp.MustCompile(`(?:>|>>|\btee\s+)(?:\s*)(?:\./|@PROJECT@/)?tmp/([^/\s'\"]+)(?:\s|['\"]|$)`)
	boundedTimeout  = regexp.MustCompile(`^timeout\s+(?:-k\s+\S+\s+)?\d`)
	waitLoopPattern = regexp.MustCompile(`\b(?:while|until)\s+`)
)

func runParity(root string) ([]Result, Population, []GoldenCase) {
	body, err := readCheckoutFile(root, parityProducer)
	if err != nil {
		failure := Result{Name: "dispatcher-golden-population", Code: 2, Message: err.Error()}
		return []Result{failure}, Population{}, nil
	}

	tables := []struct {
		name     string
		start    string
		expected int
		judge    func(string) int
	}{
		{"bash-dispatcher-golden", "BASH_GOLDEN", bashRowsExpected, bashCode},
		{"write-edit-dispatcher-golden", "WE_GOLDEN", writeEditRowsExpected, writeEditCode},
		{"weakening-dispatcher-golden", "WEAKEN_GOLDEN", weakeningRowsExpected, weakeningCode},
		{"post-write-edit-dispatcher-golden", "POST_GOLDEN", postWriteEditRowsExpected, postWriteEditCode},
	}

	population := Population{}
	results := make([]Result, 0, len(tables))
	golden := make([]GoldenCase, 0,
		bashRowsExpected+writeEditRowsExpected+weakeningRowsExpected+postWriteEditRowsExpected)
	for index, table := range tables {
		rows, parseErr := parseGolden(body, table.start)
		result := Result{Name: table.name, Passed: true}
		if parseErr != nil {
			result.Passed = false
			result.Code = 2
			result.Message = parseErr.Error()
		} else {
			if len(rows) != table.expected {
				result.Passed = false
				result.Code = 2
				var tb textbuf.Buffer
				result.Message = tb.Str("population is ").Int(int64(len(rows))).
					Str(" rows, want ").Int(int64(table.expected)).String()
			}
			for _, row := range rows {
				got := table.judge(row.name)
				golden = append(golden, GoldenCase{
					Table:        table.start,
					Name:         row.name,
					ExpectedCode: row.code,
					NativeCode:   got,
					Passed:       got == row.code,
				})
				if got == row.code || !result.Passed {
					continue
				}
				result.Passed = false
				result.Code = 1
				var tb textbuf.Buffer
				result.Message = tb.Str(row.name).Str(": native code ").Int(int64(got)).
					Str(", golden code ").Int(int64(row.code)).String()
			}
		}
		results = append(results, result)
		switch index {
		case 0:
			population.Bash = len(rows)
		case 1:
			population.WriteEdit = len(rows)
		case 2:
			population.Weakening = len(rows)
		case 3:
			population.PostWriteEdit = len(rows)
		}
	}
	return results, population, golden
}

func parseGolden(body []byte, name string) ([]goldenRow, error) {
	var tb textbuf.Buffer
	marker := tb.Str(name).Str(" = {").String()
	start := bytes.Index(body, []byte(marker))
	if start < 0 {
		return nil, fmt.Errorf("%s table is missing", name)
	}
	rows := make([]goldenRow, 0)
	scanner := bufio.NewScanner(bytes.NewReader(body[start+len(marker):]))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "}" {
			return rows, nil
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, code, ok := parseGoldenLine(line)
		if !ok {
			return nil, fmt.Errorf("%s has an unreadable row %q", name, line)
		}
		rows = append(rows, goldenRow{name: key, code: code})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return nil, fmt.Errorf("%s has no closing brace", name)
}

func parseGoldenLine(line string) (string, int, bool) {
	if len(line) < 6 || (line[0] != '\'' && line[0] != '"') {
		return "", 0, false
	}
	quote := line[0]
	end := -1
	escaped := false
	for index := 1; index < len(line); index++ {
		if escaped {
			escaped = false
			continue
		}
		if line[index] == '\\' {
			escaped = true
			continue
		}
		if line[index] == quote {
			end = index
			break
		}
	}
	if end < 0 {
		return "", 0, false
	}
	rest := strings.TrimSpace(line[end+1:])
	if len(rest) < 4 || rest[0] != ':' || rest[1] != ' ' || rest[3] != ',' {
		return "", 0, false
	}
	code := int(rest[2] - '0')
	if code < 0 || code > 2 {
		return "", 0, false
	}
	return unescapePython(line[1:end]), code, true
}

func unescapePython(value string) string {
	var out strings.Builder
	out.Grow(len(value))
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			out.WriteByte(value[index])
			continue
		}
		index++
		switch value[index] {
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		default:
			out.WriteByte(value[index])
		}
	}
	return out.String()
}

func bashCode(command string) int {
	command = strings.ReplaceAll(command, "\\\n", " ")
	if strings.Contains(command, "cp .claude/worktrees/") {
		return 2
	}
	for _, destructive := range []string{"git reset --hard", "git commit ", "git merge "} {
		if strings.Contains(command, destructive) {
			return 2
		}
	}
	if strings.TrimSpace(command) == "go build ./cmd/ze" || strings.Contains(command, "cat /tmp/") {
		return 2
	}
	if strings.Contains(command, "rm internal/") && strings.Contains(command, "_test.go") {
		return 2
	}
	if unboundedWait(command) || rawJob(command) || expensivePipe.MatchString(command) {
		return 2
	}
	if match := rootScratch.FindStringSubmatch(scratchText(command)); len(match) == 2 &&
		!sharedScratchName(match[1]) {
		return 2
	}
	return 0
}

func rawJob(command string) bool {
	if strings.Contains(command, "scripts/dev/ze-run.sh") {
		return false
	}
	if strings.Contains(command, "ZE_ADMIT_RAW=\"") && !strings.Contains(command, "ZE_ADMIT_RAW=\"\"") {
		return false
	}
	return rawGoTest.MatchString(command) || rawLint.MatchString(command) ||
		rawZeTest.MatchString(command) || rawPythonTest.MatchString(command)
}

func unboundedWait(command string) bool {
	if strings.Contains(command, "python3 -c") && strings.Contains(command, "while True") &&
		strings.Contains(command, "time.sleep") {
		return true
	}
	for _, location := range waitLoopPattern.FindAllStringIndex(command, -1) {
		loop := location[0]
		statementStart := strings.LastIndexAny(command[:loop], ";\n") + 1
		prefix := strings.TrimSpace(command[statementStart:loop])
		if searchCommandPrefix(prefix) || boundedTimeout.MatchString(prefix) {
			continue
		}

		body := command[loop:]
		if done := strings.Index(body, "done"); done >= 0 {
			body = body[:done+len("done")]
		}
		if strings.Contains(body, "read -r") && !strings.Contains(body, "sleep ") {
			continue
		}
		if strings.Contains(body, "sleep ") || strings.Contains(body, "pgrep") {
			return true
		}
	}
	return false
}

func searchCommandPrefix(prefix string) bool {
	fields := strings.Fields(prefix)
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "grep" {
		return true
	}
	if fields[0] != "git" || len(fields) < 2 {
		return false
	}
	switch fields[1] {
	case "diff", "log", "show":
		return true
	default:
		return false
	}
}

func scratchText(command string) string {
	trimmed := strings.TrimSpace(command)
	if strings.HasPrefix(trimmed, "cat ") && strings.Contains(trimmed, "<<") {
		line, _, _ := strings.Cut(command, "\n")
		return line
	}
	if !strings.HasPrefix(trimmed, "grep ") {
		return command
	}

	var visible strings.Builder
	visible.Grow(len(command))
	var quote byte
	for index := range len(command) {
		char := command[index]
		if quote == 0 && (char == '\'' || char == '"') {
			quote = char
			visible.WriteByte(' ')
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			visible.WriteByte(' ')
			continue
		}
		visible.WriteByte(char)
	}
	return visible.String()
}

func sharedScratchName(name string) bool {
	if strings.HasPrefix(name, "commit-") || strings.HasPrefix(name, "delete-") {
		return true
	}
	switch name {
	case "ze-verify.log", ".ze-verify-duration.txt", "mutation-survivors.md", "test-timings.json":
		return true
	default:
		return false
	}
}

func writeEditCode(name string) int {
	_, label, ok := strings.Cut(name, "|")
	if !ok {
		return 2
	}
	if label == "nested claude not generated" || label == "observer ok ci" ||
		strings.HasPrefix(label, "scratch dated") || strings.HasPrefix(label, "scratch producer") ||
		strings.HasPrefix(label, "scratch shared") {
		return 0
	}
	if label == "observer sysexit ci" {
		return 1
	}
	if label == "md naming bad" {
		if strings.HasPrefix(name, "Write|") {
			return 1
		}
		return 0
	}
	if label == "claude plans" && strings.HasPrefix(name, "Edit|") {
		return 0
	}
	return 2
}

func weakeningCode(name string) int {
	switch name {
	case "skip added", "skip added with a retired relax token", "commented out assertion",
		"build tag ignore added", "delete test func", "write overwrite weakens":
		return 2
	default:
		return 0
	}
}

func postWriteEditCode(name string) int {
	_, label, ok := strings.Cut(name, "|")
	if !ok {
		return 1
	}
	switch label {
	case "big >1000", "journal malformed row", "md deferral":
		return 1
	default:
		return 0
	}
}
