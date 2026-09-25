// Related: actions.go -- the table and the forwarding these tests drive
//
// VALIDATES: AC-24 and AC-25 of plan/spec-le-subject-first-command-tree.md.
// `perf run` takes the keywords of perf-bench run and the step and DUT
// selection of the retired ze-perf-run, and every perf verb answers as the
// command it replaced: suggest as suggestion-report, send, report and track as
// ze-perf run, report and track, with the trailing words and the exit code.
// PREVENTS: a `step` typo that spends minutes of Docker on both steps, and a
// forwarded verb that drops the words after it or reports success over a
// regression.

package perf

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/test/perfrunner"
)

// TestPerfRunAcceptsRunnerOptionsAsKeywords walks each row of the spec's
// "keywords of perf run" table through the table's own parser into stepsOf and
// splitDUTs, which are what runHere hands the runner.
func TestPerfRunAcceptsRunnerOptionsAsKeywords(t *testing.T) {
	tests := []struct {
		name      string
		words     []string
		wantSteps perfrunner.Steps
		wantDUTs  []string
	}{
		{name: "bare run is --build --test", words: nil, wantSteps: perfrunner.Steps{Build: true, Test: true}},
		{name: "dut names", words: []string{"dut", "ze bird"}, wantSteps: perfrunner.Steps{Build: true, Test: true}, wantDUTs: []string{"ze", "bird"}},
		{name: "--build alone", words: []string{"step", "build"}, wantSteps: perfrunner.Steps{Build: true}},
		{name: "--test alone", words: []string{"step", "test"}, wantSteps: perfrunner.Steps{Test: true}},
		{name: "step and dut", words: []string{"step", "build", "dut", "ze"}, wantSteps: perfrunner.Steps{Build: true}, wantDUTs: []string{"ze"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := parseRun(t, tt.words)
			steps, err := stepsOf(args)
			if err != nil {
				t.Fatalf("stepsOf(%q): %v", tt.words, err)
			}
			if steps != tt.wantSteps {
				t.Fatalf("steps = %+v, want %+v", steps, tt.wantSteps)
			}
			if got := splitDUTs(args.One(dutKeyword)); !slices.Equal(got, tt.wantDUTs) {
				t.Fatalf("DUTs = %q, want %q", got, tt.wantDUTs)
			}
		})
	}

	// A step that is neither build nor test exits 2 and names the value, from
	// the entry point, before any image is built.
	var code int
	stderr := captureStderr(t, func() { _, code = Answer([]string{runVerb, stepKeyword, "bogus"}) })
	if code != 2 {
		t.Fatalf("`perf run step bogus` exited %d, want 2", code)
	}
	if !strings.Contains(stderr, `"bogus"`) {
		t.Fatalf("the refusal %q does not name the value", stderr)
	}

	// An unknown DUT is refused by validateDUTs, as RunCLI refused it.
	bench, rec := fixtureBench(t)
	if _, code := bench.Run(bothSteps(), []string{"zebra"}); code != 1 || len(rec.steps) != 0 {
		t.Fatalf("an unknown DUT answered %d after %d steps, want 1 before any", code, len(rec.steps))
	}
}

// TestPerfVerbsAnswerAsTheirPredecessors checks the eight verbs are the table,
// that suggest answers the nudge, and that the three program verbs reach
// internal/perf/cli with their trailing words and its exit code.
func TestPerfVerbsAnswerAsTheirPredecessors(t *testing.T) {
	var verbs []string
	for _, row := range Actions().Actions {
		verbs = append(verbs, row.Verb)
	}
	want := []string{suggestVerb, recordVerb, runVerb, historyVerb, evidenceVerb, sendVerb, reportVerb, trackVerb}
	if !slices.Equal(verbs, want) {
		t.Fatalf("perf verbs = %q, want %q", verbs, want)
	}

	// suggest is the nudge suggestion-report was: a Report, and always 0.
	answer, code := Answer([]string{suggestVerb})
	if code != 0 {
		t.Fatalf("`perf suggest` exited %d, want 0", code)
	}
	if _, ok := answer.(Report); !ok {
		t.Fatalf("`perf suggest` answered %T, want the nudge Report", answer)
	}

	dir := t.TempDir()
	regressing := writeHistory(t, dir, "regressing.ndjson", 2500)
	steady := writeHistory(t, dir, "steady.ndjson", 1050)

	// track --check keeps the exit code ze-perf track --check gave.
	captureStderr(t, func() { _, code = Answer([]string{trackVerb, "--check", regressing}) })
	if code != 1 {
		t.Fatalf("`perf track --check` over a regression exited %d, want 1", code)
	}
	captureStderr(t, func() { _, code = Answer([]string{trackVerb, "--check", steady}) })
	if code != 0 {
		t.Fatalf("`perf track --check` over a steady history exited %d, want 0", code)
	}

	// report and send reach their subcommands with the trailing words: a
	// missing file and an unknown option are the programs' own refusals.
	captureStderr(t, func() { _, code = Answer([]string{reportVerb, filepath.Join(dir, "absent.json")}) })
	if code == 0 {
		t.Fatal("`perf report <absent file>` exited 0, so the file never reached ze-perf report")
	}
	stderr := captureStderr(t, func() { _, code = Answer([]string{sendVerb, "--no-such-option"}) })
	if code == 0 || !strings.Contains(stderr, "no-such-option") {
		t.Fatalf("`perf send --no-such-option` exited %d with %q, want the sender's own refusal", code, stderr)
	}
}

// parseRun answers the arguments the table parses for `perf run <words>`. It
// declares the table's own runParameters and catches the result where runHere
// would receive it, because runHere itself goes on to start Docker.
func parseRun(t *testing.T, words []string) leaction.Arguments {
	t.Helper()
	var caught leaction.Arguments
	area := leaction.New(area, leaction.Action{
		Verb:       runVerb,
		Why:        "catch the parsed arguments",
		Parameters: runParameters,
		AnswerArgs: func(args leaction.Arguments) (any, int) { caught = args; return nil, 0 },
	})
	if _, code := area.Answer(append([]string{runVerb}, words...)); code != 0 {
		t.Fatalf("the grammar refused %q with %d", words, code)
	}
	return caught
}

// writeHistory writes a two-line NDJSON history whose second run converged in
// convergenceMs, against a first run of 1000 ms.
func writeHistory(t *testing.T, dir, name string, convergenceMs int) string {
	t.Helper()
	line := func(ms string) string {
		return `{"dut-name":"ze","family":"ipv4/unicast","convergence-ms":` + ms +
			`,"convergence-stddev-ms":50,"throughput-avg":50000,"throughput-avg-stddev":1000,` +
			`"latency-p99-ms":10,"latency-p99-stddev-ms":2}`
	}
	path := filepath.Join(dir, name)
	body := line("1000") + "\n" + line(strconv.Itoa(convergenceMs)) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("history: %v", err)
	}
	return path
}

// captureStderr answers what run writes to os.Stderr. It swaps a process
// global, so no test in this package that calls it runs in parallel.
func captureStderr(t *testing.T, run func()) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = write
	collected := make(chan string, 1)
	go func() {
		var buf strings.Builder
		chunk := make([]byte, 4096)
		for {
			n, readErr := read.Read(chunk)
			buf.Write(chunk[:n])
			if readErr != nil {
				break
			}
		}
		collected <- buf.String()
	}()
	run()
	os.Stderr = saved
	if err := write.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	text := <-collected
	if err := read.Close(); err != nil {
		t.Fatalf("close pipe reader: %v", err)
	}
	return text
}
