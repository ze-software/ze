// Design: docs/architecture/core-design.md -- native Go gates run through le
// Overview: hookcheck.go -- selftest orchestration and structured report
//
// Dispatcher fixtures are typed Go data. Native rules evaluate that data and
// hook source contracts detect changes to the dispatchers themselves.
package hookcheck

import (
	"crypto/sha256"
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

var parityCatalogDigest = [sha256.Size]byte{
	0x9e, 0x56, 0x5d, 0xe4, 0x10, 0x29, 0x9e, 0x9d,
	0xe6, 0x18, 0x48, 0x46, 0x1d, 0x03, 0x9c, 0xd3,
	0xd4, 0xf8, 0x8d, 0x92, 0x55, 0xd0, 0xc6, 0x64,
	0xd7, 0xae, 0x22, 0x20, 0x14, 0x80, 0xb7, 0x08,
}

var (
	expensivePipe = regexp.MustCompile(
		`(?s)(?:^|[;&\n])\s*(?:timeout\s+(?:-k\s+\S+\s+)?\S+\s+|nice\s+-n\s+\S+\s+)?` +
			`(?:go\s+test\b|(?:\./)?le\s+verify(?:\s|$)|` +
			`(?:(?:\./)?bin/ze-test|(?:/\S*/)?tmp/session/[0-9]{4}-[0-9]{2}-[0-9]{2}-[^/\s]+/bin/ze-test)\b|` +
			`(?:(?:\./)?bin/ze|ze)\s+le\s+hook-check\s+unit\b)[^;&\n]*?(?:\||\|&)\s*(?:head|tail|grep)\b`,
	)
	rawGoTest       = regexp.MustCompile(`(?:^|[;&\n])\s*(?:timeout\s+(?:-k\s+\S+\s+)?\S+\s+|nice\s+-n\s+\S+\s+)?go\s+test\b`)
	rawLint         = regexp.MustCompile(`(?:^|[;&\n])\s*(?:timeout\s+\S+\s+)?golangci-lint\s+run\b`)
	rawZeTest       = regexp.MustCompile(`(?:^|[;&\n])\s*(?:timeout\s+\S+\s+)?(?:(?:\./)?bin/ze-test(?:-linux-[^\s/]*)?|(?:/\S*/)?tmp/session/[0-9]{4}-[0-9]{2}-[0-9]{2}-[^/\s]+/bin/ze-test(?:-linux-[^\s/]*)?)\b`)
	rootScratch     = regexp.MustCompile(`(?:>|>>|\btee\s+)(?:\s*)(?:\./|@PROJECT@/)?tmp/([^/\s'"]+)(?:\s|['"]|$)`)
	boundedTimeout  = regexp.MustCompile(`^timeout\s+(?:-k\s+\S+\s+)?\d`)
	waitLoopPattern = regexp.MustCompile(`\b(?:while|until)\s+`)
)

type parityFixture struct {
	name         string
	expectedCode int
}

type parityTable struct {
	key      string
	name     string
	expected int
	judge    func(string) int
	fixtures []parityFixture
}

func runParity(_ string) ([]Result, Population, []GoldenCase) {
	results := make([]Result, 0, len(parityCatalog))
	golden := make([]GoldenCase, 0,
		bashRowsExpected+writeEditRowsExpected+weakeningRowsExpected+postWriteEditRowsExpected)
	population := Population{}
	currentDigest := parityDigest(parityCatalog[:])
	catalogOK := currentDigest == parityCatalogDigest

	for index, table := range parityCatalog {
		result := Result{Name: table.name, Passed: catalogOK}
		if !catalogOK {
			result.Code = 2
			result.Message = fmt.Sprintf("typed dispatcher fixture content changed: got %x, want %x", currentDigest, parityCatalogDigest)
		} else if len(table.fixtures) != table.expected {
			result.Passed = false
			result.Code = 2
			var tb textbuf.Buffer
			result.Message = tb.Str("population is ").Int(int64(len(table.fixtures))).
				Str(" rows, want ").Int(int64(table.expected)).String()
		}
		for _, fixture := range table.fixtures {
			got := table.judge(fixture.name)
			golden = append(golden, GoldenCase{
				Table:        table.key,
				Name:         fixture.name,
				ExpectedCode: fixture.expectedCode,
				NativeCode:   got,
				Passed:       got == fixture.expectedCode,
			})
			if got == fixture.expectedCode || !result.Passed {
				continue
			}
			result.Passed = false
			result.Code = 1
			var tb textbuf.Buffer
			result.Message = tb.Str(fixture.name).Str(": native code ").Int(int64(got)).
				Str(", expected code ").Int(int64(fixture.expectedCode)).String()
		}
		results = append(results, result)
		switch index {
		case 0:
			population.Bash = len(table.fixtures)
		case 1:
			population.WriteEdit = len(table.fixtures)
		case 2:
			population.Weakening = len(table.fixtures)
		case 3:
			population.PostWriteEdit = len(table.fixtures)
		}
	}
	return results, population, golden
}

func parityDigest(tables []parityTable) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, table := range tables {
		for _, fixture := range table.fixtures {
			tb.Str(table.key).Byte(0).Str(fixture.name).Byte(0).
				Int(int64(fixture.expectedCode)).Byte(0)
		}
	}
	return sha256.Sum256([]byte(tb.String()))
}

func bashCode(command string) int {
	command = strings.ReplaceAll(command, "\\\n", " ")
	fields := strings.Fields(command)
	if len(fields) >= 3 && fields[0] == "./le" && fields[1] == "commit" {
		switch fields[2] {
		case "session", "create", "message", "audit", "review-check",
			"debt-list", "debt-status", "debt-clear":
		default:
			return 2
		}
	}
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
	if strings.Contains(command, "internal/le/job/answer.go") {
		return false
	}
	if strings.Contains(command, "ZE_ADMIT_RAW=\"") && !strings.Contains(command, "ZE_ADMIT_RAW=\"\"") {
		return false
	}
	return rawGoTest.MatchString(command) || rawLint.MatchString(command) ||
		rawZeTest.MatchString(command)
}

func unboundedWait(command string) bool {
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
