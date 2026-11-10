// Design: docs/contributing/running-commands.md -- le test cost report
// Overview: cost.go -- the ranking this feeds
//
// golog.go reads per-test elapsed out of a Go test log. Two forms carry it:
// `go test -v` prints `--- PASS: TestX (1.23s)` at column 0 for a top-level
// test, and `go test -json` prints one event per line whose pass or fail
// event for a top-level test carries `Elapsed` in seconds. A plain `go test`
// prints neither for a passing test, so its log cannot be ranked.

package testcost

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// unnamedPackage groups the tests of a `-v` log whose package line never came,
// such as a run cut off before its package finished.
const unnamedPackage = "(package not named in the log)"

// logLineMax bounds one log line. A longer line (a test that printed a huge
// blob without a newline) refuses the read rather than growing without bound.
const logLineMax = 4 << 20

var (
	// testResultLine is a top-level test's verdict in a `-v` log. A subtest's
	// line is indented, so it never matches and is not counted twice.
	testResultLine = regexp.MustCompile(`^--- (?:PASS|FAIL): (\S+) \((\d+(?:\.\d+)?)s\)$`)
	// packageResultLine closes a package in a `-v` or plain log: every test
	// line since the previous package line belongs to it.
	packageResultLine = regexp.MustCompile(`^(?:ok|FAIL)\s+(\S+)(?:\s|$)`)
)

// testEvent is the part of a `go test -json` event the report reads.
type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
}

// testKey names one test in one package.
type testKey struct {
	group string
	test  string
}

// goLog accumulates the tests of one log. A test that ran more than once
// (-count) sums, because the share is of the whole run's time.
type goLog struct {
	order   []testKey
	elapsed map[testKey]time.Duration
	pending []pendingTest
}

// pendingTest is a `-v` result whose package line has not come yet.
type pendingTest struct {
	test    string
	elapsed time.Duration
}

// parseGoLog answers every top-level test the log timed. It answers an error
// when the log cannot be read, or when it holds no per-test elapsed at all:
// an empty ranking would read as "no test costs too much" when nothing was
// measured.
func parseGoLog(r io.Reader) ([]measured, error) {
	log := goLog{elapsed: map[testKey]time.Duration{}}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), logLineMax)
	for scanner.Scan() {
		log.readLine(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	log.close(unnamedPackage)
	if len(log.order) == 0 {
		return nil, errors.New("it holds no per-test elapsed (no `--- PASS:` or `--- FAIL:` line, no -json pass or fail event), " +
			"so no test can be ranked; run the package again with go test -v, as in " +
			"./le job run label unit-pkg quiet command go test -v -race -tags <tags> <package>")
	}
	tests := make([]measured, 0, len(log.order))
	for _, key := range log.order {
		tests = append(tests, measured{group: key.group, test: key.test, elapsed: log.elapsed[key], enough: true})
	}
	return tests, nil
}

// readLine reads one log line in either form.
func (l *goLog) readLine(line string) {
	if strings.HasPrefix(line, "{") {
		l.readEvent(line)
		return
	}
	if match := testResultLine.FindStringSubmatch(line); match != nil {
		elapsed, ok := secondsOf(match[2])
		if ok {
			l.pending = append(l.pending, pendingTest{test: match[1], elapsed: elapsed})
		}
		return
	}
	if match := packageResultLine.FindStringSubmatch(line); match != nil {
		l.close(match[1])
	}
}

// readEvent reads one `-json` event. Only a top-level test's pass or fail
// carries the elapsed the report ranks; a subtest's time is inside its
// parent's, and a package event has no test. A line that is not an event (a
// build message that opens with a brace) is not a test result, so it is
// skipped.
func (l *goLog) readEvent(line string) {
	var event testEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return
	}
	if event.Test == "" {
		return
	}
	if strings.Contains(event.Test, "/") {
		return
	}
	if event.Action != "pass" && event.Action != "fail" {
		return
	}
	if event.Elapsed < 0 {
		return
	}
	l.add(testKey{group: event.Package, test: event.Test}, millisecondsOf(event.Elapsed))
}

// close assigns every pending `-v` result to the package whose line came.
func (l *goLog) close(group string) {
	for _, test := range l.pending {
		l.add(testKey{group: group, test: test.test}, test.elapsed)
	}
	l.pending = l.pending[:0]
}

// add records one run of one test.
func (l *goLog) add(key testKey, elapsed time.Duration) {
	if _, seen := l.elapsed[key]; !seen {
		l.order = append(l.order, key)
	}
	l.elapsed[key] += elapsed
}

// secondsOf parses the seconds a `-v` line printed. The pattern admits only
// digits and one dot, so a refusal here is a value too large for a float.
func secondsOf(text string) (time.Duration, bool) {
	seconds, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	return millisecondsOf(seconds), true
}

// millisecondsOf converts seconds to a duration rounded to the millisecond, the
// precision both log forms print at, so 2.001s stays above a 2s floor and
// 2.00s stays exactly on it.
func millisecondsOf(seconds float64) time.Duration {
	return time.Duration(math.Round(seconds*1000)) * time.Millisecond
}
