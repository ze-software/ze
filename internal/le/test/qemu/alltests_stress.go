// Design: docs/architecture/testing/qemu-integration.md -- selected native stress.
// Related: alltests.go -- canonical selection and shared preparation.

package testqemu

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	teststressrepro "github.com/ze-software/ze/internal/le/test/stressrepro"
)

const (
	stressKeyword         = "stress"
	stressParallelKeyword = "stress-parallel"
)

// selectedStress is a validated bounded request. Nil means ordinary execution;
// a zero value is uninitialized and cannot supply a valid duration.
type selectedStress struct {
	iterations int
	parallel   int
}

func selectedStressOf(args leaction.Arguments) (*selectedStress, error) {
	if !args.Has(stressKeyword) {
		if args.Has(stressParallelKeyword) {
			return nil, selectedStressError("stress-parallel requires stress <iterations>")
		}
		return nil, nil
	}
	if args.One(testKeyword) == "" {
		return nil, selectedStressError("stress requires test <path> naming one .ci")
	}
	if _, err := vmSuiteFor(args.One(testKeyword)); err != nil {
		return nil, selectedStressError(err.Error())
	}
	iterations, err := selectedStressNumber(args, stressKeyword, 1000)
	if err != nil {
		return nil, err
	}
	parallel := 1
	if args.Has(stressParallelKeyword) {
		parallel, err = selectedStressNumber(args, stressParallelKeyword, 4)
		if err != nil {
			return nil, err
		}
	}
	return &selectedStress{iterations: iterations, parallel: parallel}, nil
}

func selectedStressNumber(args leaction.Arguments, keyword string, limit int) (int, error) {
	value := args.One(keyword)
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, selectedStressError(keyword + " " + value + " is not a bounded integer")
	}
	if count < 1 {
		return 0, selectedStressError(keyword + " " + value + " must be positive")
	}
	if count > limit {
		return 0, selectedStressError(keyword + " " + value + " exceeds " + strconv.Itoa(limit))
	}
	return count, nil
}

// budget rounds the batch count up and includes bounded child cleanup plus one
// minute for capture setup and final cleanup. Its zero/nil receiver is invalid.
func (s *selectedStress) budget(timeout time.Duration) (time.Duration, error) {
	if s == nil {
		return 0, selectedStressError("selected stress has no request")
	}
	if s.iterations < 1 {
		return 0, selectedStressError("stress iterations must be positive")
	}
	if s.iterations > 1000 {
		return 0, selectedStressError("stress iterations exceed 1000")
	}
	if s.parallel < 1 {
		return 0, selectedStressError("stress workers must be positive")
	}
	if s.parallel > 4 {
		return 0, selectedStressError("stress workers exceed 4")
	}
	if timeout <= 0 {
		return 0, selectedStressError("stress invocation timeout must be positive")
	}
	const maxDuration = time.Duration(1<<63 - 1)
	if timeout > maxDuration-suiteKillGrace {
		return 0, selectedStressError("stress invocation timeout overflows its cleanup bound")
	}
	batches := (s.iterations + s.parallel - 1) / s.parallel
	perBatch := timeout + suiteKillGrace
	if perBatch > (maxDuration-time.Minute-suiteKillGrace)/time.Duration(batches) {
		return 0, selectedStressError("stress iterations and timeout overflow the whole-run deadline")
	}
	return time.Duration(batches)*perBatch + time.Minute, nil
}

func selectedStressVerdict(s *selectedStress, report teststressrepro.Report, code int) error {
	if code != 1 {
		return selectedStressError("stress adapter did not finish without a reproduction")
	}
	if report.SetupError != "" {
		return selectedStressError(report.SetupError)
	}
	if report.Reproduced {
		return selectedStressError("stress reproduced a failing invocation")
	}
	if report.Interrupted {
		return selectedStressError("stress was interrupted")
	}
	if report.Failed != 0 {
		return selectedStressError("stress contains failed invocations")
	}
	if report.Exit != 0 {
		return selectedStressError("stress contains a failing child exit")
	}
	if report.Iterations != s.iterations {
		return selectedStressError("stress report does not describe the requested iteration count")
	}
	if report.Completed != s.iterations {
		return selectedStressError("stress did not finish every requested invocation")
	}
	if report.Executed != s.iterations {
		return selectedStressError("stress did not execute exactly one selected test per invocation")
	}
	return nil
}

func selectedStressCount(output string) (int, error) {
	var tally suiteTally
	if _, err := tally.Write([]byte(output)); err != nil {
		return 0, err
	}
	if tally.Summaries != 1 {
		return 0, selectedStressError("stress child did not print exactly one test summary")
	}
	if tally.Verdict != tallyPass {
		return 0, selectedStressError("stress child printed a failing test summary")
	}
	if tally.Tests != 1 {
		return 0, selectedStressError("stress child did not execute exactly one selected test")
	}
	if tally.Passed != 1 {
		return 0, selectedStressError("stress child did not pass its selected test")
	}
	if tally.Skipped {
		return 0, selectedStressError("stress child skipped a test")
	}
	return tally.Tests, nil
}

func selectedStressError(problem string) error {
	return opererr.New("qemu: "+problem,
		"Selected stress requires every requested invocation to execute its selected test successfully",
		"./le test qemu all test test/<dir>/<name>.ci stress 80 stress-parallel 1")
}

// selectedStressPhase reuses the already prepared suite and environment. The
// adapter owns the individual children; this outer context covers the full
// checked sequence rather than applying the ordinary suite cap to all runs.
func (a *allTestsRun) selectedStressPhase(suite vmSuite, environ []string) PhaseResult {
	phase := PhaseResult{Name: suitePhaseName(suite.Name), Namespace: suite.Namespace.String()}
	if slices.Contains(a.Skip, suite.Name) {
		phase.Code = 1
		phase.Reason = "selected stress suite is excluded by ZE_QEMU_SKIP_SUITES"
		return phase
	}
	timeout, err := a.suiteTimeout()
	if err != nil {
		phase.Code, phase.Reason = 1, err.Error()
		return phase
	}
	budget, err := a.Stress.budget(timeout)
	if err != nil {
		phase.Code, phase.Reason = 1, err.Error()
		return phase
	}
	executable := filepath.Join(a.BinDir, leName)
	if suite.Namespace == perTest {
		executable = filepath.Join(netnsCapDir, leName)
	}
	request := teststressrepro.SelectedRequest{
		Suite: suite.Name, Runner: slices.Clone(suite.Args), Selector: suite.Only,
		Executable: executable, Environment: a.suiteEnvironment(suite, environ),
		Iterations: a.Stress.iterations, Parallel: a.Stress.parallel,
		Timeout: timeout, Budget: budget, Count: selectedStressCount,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	bounded, cancel := context.WithTimeout(ctx, budget+suiteKillGrace)
	defer cancel()
	run := a.StressRun
	if run == nil {
		run = teststressrepro.RunSelected
	}
	report, code := run(bounded, a.Workspace, request)
	phase.Stress = &report
	phase.Command = slices.Clone(report.Command)
	phase.Tests, phase.Counted = report.Executed, true
	if err := selectedStressVerdict(a.Stress, report, code); err != nil {
		phase.Code, phase.Reason = 1, err.Error()
		a.note(err.Error())
	}
	var note textbuf.Buffer
	a.note(note.Str("selected stress: ").Str(a.Test).Str(" namespace=").Str(suite.Namespace.String()).
		Str(" iterations=").Int(int64(request.Iterations)).Str(" completed=").Int(int64(report.Completed)).
		Str(" executed=").Int(int64(report.Executed)).Str(" budget=").Str(budget.String()).
		Str(" capture=").Str(report.Log).String())
	var command textbuf.Buffer
	a.note(command.Str("selected stress command: ").Join(report.Command, " ").String())
	return phase
}

// validOneCIPath rejects traversal and runner-option selectors before the shim
// or any kernel preparation. vmSuiteFor remains the only suite resolver.
func validOneCIPath(path string) error {
	if filepath.Clean(path) != path {
		return fmt.Errorf("%w, got %s", errNotOneCI, path)
	}
	stem := strings.TrimSuffix(filepath.Base(path), ".ci")
	if stem == "" {
		return fmt.Errorf("%w, got %s", errNotOneCI, path)
	}
	if strings.HasPrefix(stem, "-") {
		return fmt.Errorf("%w, got %s", errNotOneCI, path)
	}
	return nil
}
