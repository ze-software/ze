// Design: docs/functional-tests.md -- the suite action surface and the selection contract.

package testfunctional

import (
	"io"
	"os"
	"strings"
	"testing"
)

// TestSuiteActionRefusesASelectorAndNamesTheRunnerThatTakesOne types the
// selector forms the selection contract documents after a suite verb of
// `le test functional`, through Answer, the command's own entry.
//
// VALIDATES: each line is refused with code 2 and no payload before anything
// runs (no build, no host preflight), and the refusal names the suite's own
// runner with the selector, `./le test bgp plugin --pattern TEXT`, which is the
// form docs/functional-tests.md documents for choosing tests.
// PREVENTS: the 2026-10-07 journal row: a reader who typed the documented
// `--pattern` after `le test functional plugin` met "no such action", which
// never said where the selector goes, and then paid for the whole suite.
//
// MUTATION: (actions.go, `if refusedSelector(args) {`, `if false && refusedSelector(args) {`): the table's generic refusal answers, and the stderr assertion goes red because it names no runner.
// MUTATION: (suites.go, `if arg == allTests {`, `if arg == "" {`): the named command keeps `--all` and the stderr assertion goes red.
func TestSuiteActionRefusesASelectorAndNamesTheRunnerThatTakesOne(t *testing.T) {
	for _, tc := range []struct {
		line []string
		want string
	}{
		{line: []string{suitePlugin, "--pattern", "data-"}, want: "./le test bgp plugin --pattern TEXT"},
		{line: []string{suitePlugin, "--start", "data-"}, want: "./le test bgp plugin --pattern TEXT"},
		{line: []string{suiteUi, "--pattern", "decode"}, want: "./le test ui --pattern TEXT"},
	} {
		t.Run(strings.Join(tc.line, " "), func(t *testing.T) {
			var payload any
			var code int
			stderr := stderrOf(t, func() { payload, code = Answer(tc.line) })
			if code != 2 {
				t.Errorf("answered %d, want 2: the line was not refused", code)
			}
			if payload != nil {
				t.Errorf("answered %v, want no payload: something ran", payload)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr %q does not name %q", stderr, tc.want)
			}
		})
	}
}

// stderrOf answers what fn wrote to os.Stderr.
func stderrOf(t *testing.T, fn func()) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = write
	defer func() { os.Stderr = saved }()

	done := make(chan string, 1)
	go readAll(read, done)
	fn()
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	return <-done
}

// readAll sends everything r carries to done once r reaches EOF.
func readAll(r *os.File, done chan<- string) {
	data, _ := io.ReadAll(r) //nolint:errcheck // a short read leaves the assertion to fail on the text
	done <- string(data)
}
