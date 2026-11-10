// Design: docs/contributing/running-commands.md -- le test attribution report
// Overview: attribution.go -- the two runs these readings compare
//
// observe.go reads what one `go test -v -run '^Name$'` run answered, and
// classify turns the two readings of one test into its attribution.

package testattribution

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	testoverlay "github.com/ze-software/ze/internal/le/test/overlay"
)

// outcome is what one run of one test answered. The zero value is no reading.
type outcome uint8

const (
	outcomeUnspecified outcome = iota
	// outcomePassed says the test ran and passed.
	outcomePassed
	// outcomeFailed says the test ran and failed, or the package's test
	// binary failed before any test ran (an init panic, a failing TestMain).
	outcomeFailed
	// outcomeUnbuilt says the package did not compile, so no test ran.
	outcomeUnbuilt
	// outcomeAbsent says the package compiled and holds no test of that name.
	outcomeAbsent
)

// String names the outcome as the report shows it.
func (o outcome) String() string {
	switch o {
	case outcomePassed:
		return "passed"
	case outcomeFailed:
		return "failed"
	case outcomeUnbuilt:
		return "did not compile"
	case outcomeAbsent:
		return "no such test"
	case outcomeUnspecified:
		return "unread"
	}
	return "unread"
}

// observation is one reading: the outcome, and for a package that did not
// compile, the compiler's first line.
type observation struct {
	outcome outcome
	line    string
}

// buildFailedMarks are the words `go test` puts after the package on its FAIL
// line when it never produced a test binary.
var buildFailedMarks = []string{"[build failed]", "[setup failed]"}

// observe reads one run of the top-level test name.
func observe(passed bool, output, name string) observation {
	ran := testoverlay.GoTestLineNames(output, "=== RUN   ", name) >= 0
	if ran {
		if passed {
			return observation{outcome: outcomePassed}
		}
		return observation{outcome: outcomeFailed}
	}
	if passed {
		// `go test -run` that matches nothing passes with "no tests to run".
		return observation{outcome: outcomeAbsent}
	}
	for _, mark := range buildFailedMarks {
		if strings.Contains(output, mark) {
			return observation{outcome: outcomeUnbuilt, line: compilerLine(output)}
		}
	}
	if packageFailed(output) {
		return observation{outcome: outcomeFailed}
	}
	// No test ran, no test binary failed: the go command itself refused, as
	// it does for an overlay it cannot read. Its first line says why.
	return observation{outcome: outcomeUnbuilt, line: firstLine(output)}
}

// packageFailed reports whether the test binary ran and failed: `go test`
// then ends the package with `FAIL<tab><package>` and no build mark.
func packageFailed(output string) bool {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "FAIL\t") {
			return true
		}
	}
	return false
}

// compilerLine answers the compiler's first error: the line after the first
// `# <package>` header, or the output's first line when there is no header.
func compilerLine(output string) string {
	header := false
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if header {
			return trimmed
		}
		header = strings.HasPrefix(trimmed, "# ")
	}
	return firstLine(output)
}

// firstLine answers the first non-blank line of output.
func firstLine(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return "(the run printed nothing)"
}

// Class is the attribution of one failing test.
type Class string

// The classes. Each pair of readings lands in exactly one, and none is folded
// into another: a red that did not come back is not a red the session caused,
// and a run that could not compile is no verdict at all.
const (
	// ClassCaused: red as on disk, green with the named files at HEAD.
	ClassCaused Class = "caused"
	// ClassAdded: red as on disk, and the package with the named files at
	// HEAD holds no such test, so the named files add the test that is red.
	ClassAdded Class = "added"
	// ClassNotCaused: red both ways.
	ClassNotCaused Class = "not-caused"
	// ClassUndetermined: one of the two builds did not compile.
	ClassUndetermined Class = "undetermined"
	// ClassRedAtHeadOnly: green as on disk, red with the named files at HEAD.
	ClassRedAtHeadOnly Class = "red-at-head-only"
	// ClassNotReproduced: green both ways, so the log's red did not come back.
	ClassNotReproduced Class = "not-reproduced"
	// ClassNotRun: the package as on disk holds no test of that name.
	ClassNotRun Class = "not-run"
)

// classify turns the two readings of one test into its class and the sentence
// that says why.
func classify(disk, head observation) (Class, string) {
	switch disk.outcome {
	case outcomeUnbuilt:
		return ClassUndetermined, "the package as on disk does not compile now: " + disk.line
	case outcomeAbsent:
		return ClassNotRun, "the package as on disk holds no test of this name; " +
			"the log may come from another package: name the package the log ran"
	case outcomeFailed:
		return classifyRed(head)
	case outcomePassed:
		return classifyGreen(head)
	case outcomeUnspecified:
		panic("BUG: attribution: an on-disk run was classified before it was read")
	}
	panic("BUG: attribution: an on-disk outcome has no class")
}

// classifyRed classifies a test that is red as on disk.
func classifyRed(head observation) (Class, string) {
	switch head.outcome {
	case outcomePassed:
		return ClassCaused, "red as on disk, green with the named files at HEAD"
	case outcomeFailed:
		return ClassNotCaused, "red as on disk and with the named files at HEAD"
	case outcomeUnbuilt:
		var tb textbuf.Buffer
		return ClassUndetermined, tb.Str("the package with the named files at HEAD does not compile, ").
			Str("so the red cannot be compared; name every file the change needs: ").Str(head.line).String()
	case outcomeAbsent:
		return ClassAdded, "red as on disk; with the named files at HEAD the test does not exist, " +
			"so the named files add it"
	case outcomeUnspecified:
		panic("BUG: attribution: a HEAD run was classified before it was read")
	}
	panic("BUG: attribution: a HEAD outcome has no class")
}

// classifyGreen classifies a test the log reported red that is green now.
func classifyGreen(head observation) (Class, string) {
	switch head.outcome {
	case outcomePassed:
		return ClassNotReproduced, "green both ways now: the log's red did not come back, " +
			"so the test may be flaky or the tree changed since the log"
	case outcomeFailed:
		return ClassRedAtHeadOnly, "green as on disk, red with the named files at HEAD: " +
			"the log's red did not come back, and the named files fix a red rather than cause one"
	case outcomeUnbuilt:
		var tb textbuf.Buffer
		return ClassUndetermined, tb.Str("green as on disk, and the package with the named files at HEAD ").
			Str("does not compile: ").Str(head.line).String()
	case outcomeAbsent:
		return ClassNotReproduced, "green as on disk now, so the log's red did not come back; " +
			"with the named files at HEAD the test does not exist, so the named files add it"
	case outcomeUnspecified:
		panic("BUG: attribution: a HEAD run was classified before it was read")
	}
	panic("BUG: attribution: a HEAD outcome has no class")
}
