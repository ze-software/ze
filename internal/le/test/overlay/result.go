// Design: docs/contributing/rfc-conformance-gates.md -- what a `.ci` or Go unit observation must show
// Overview: overlay.go -- the overlay whose run these readers judge
// Related: interop.go -- the interop scenario's verdict readers

package overlay

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The outcome words the functional runner prints on a .ci's result line
// (internal/test/runner/display.go, TestFinished).
const (
	CIPass    = "PASS"
	CIFail    = "FAIL"
	CITimeout = "TIME"
	CISkip    = "SKIP"
)

// CIResult answers the outcome word on the runner's result line for the .ci
// named stem, or the empty string when no line names it.
//
// The line is `<elapsed>  <i>/<n>  <OUTCOME>  <nick>  <name>`, and the name is
// left out when it equals the nick, so the outcome is the third field from the
// end or, failing that, the second.
func CIResult(output, stem string) string {
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[len(fields)-1] != stem {
			continue
		}
		for _, word := range fields[len(fields)-3 : len(fields)-1] {
			if isCIOutcome(word) {
				return word
			}
		}
	}
	return ""
}

// isCIOutcome answers whether word is one of the runner's outcome words.
func isCIOutcome(word string) bool {
	return word == CIPass || word == CIFail || word == CITimeout || word == CISkip
}

// DescribeCIResult words what the runner's output said about one .ci.
func DescribeCIResult(stem, result string) string {
	var tb textbuf.Buffer
	if result == "" {
		return tb.Str("the output carries no result line for ").Str(stem).String()
	}
	return tb.Str("the runner reported ").Str(stem).Str(" as ").Str(result).String()
}

// DescribeUnobservedBreak words the refusal of a broken run that did not run
// the unit.
func DescribeUnobservedBreak(unit, what, output string) string {
	var tb textbuf.Buffer
	return tb.Str(unit).Str(" did not run under the break: ").Str(what).
		Str(". Nothing is written: a red this run did not observe is never recorded:\n").
		Str(Excerpt(output)).String()
}

// GoTestLineNames answers the byte offset of the first `go test` line that
// opens with prefix and names unit or one of its subtests, or -1.
//
// `go test` indents a subtest's line, and follows the name with a space and
// the duration, so the name is the text up to the first space.
func GoTestLineNames(output, prefix, unit string) int {
	offset := 0
	for line := range strings.SplitAfterSeq(output, "\n") {
		start := offset
		offset += len(line)
		rest, found := strings.CutPrefix(strings.TrimLeft(line, " \t"), prefix)
		if !found {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimSpace(rest), " ")
		if name == unit || strings.HasPrefix(name, unit+"/") {
			return start
		}
	}
	return -1
}

// SkipReason answers the indented log lines `go test -v` printed directly above
// a `--- SKIP:` line, which before holds the output up to that line. The test's
// t.Skip text is the last of them.
func SkipReason(before string) string {
	lines := strings.Split(strings.TrimRight(before, "\n"), "\n")
	first := len(lines)
	for first > 0 {
		if !strings.HasPrefix(lines[first-1], "    ") {
			break
		}
		first--
	}
	reason := make([]string, 0, len(lines)-first)
	for _, line := range lines[first:] {
		reason = append(reason, strings.TrimSpace(line))
	}
	if len(reason) == 0 {
		return "it printed no reason"
	}
	return strings.Join(reason, " ")
}

// excerptLines is how much of a run's output a refusal and a record carry.
//
// The tail, because that is where `go test` prints the failure and where a
// suite prints its summary. Enough to read, bounded so a record stays a record.
const excerptLines = 40

// Excerpt answers the tail of one run's output.
func Excerpt(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) > excerptLines {
		lines = lines[len(lines)-excerptLines:]
	}
	return strings.Join(lines, "\n")
}
