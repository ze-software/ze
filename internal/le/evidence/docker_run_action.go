// Design: docs/architecture/core-design.md -- evidence actions and signal-aware execution
// Overview: docker_run.go -- the container lifecycle this action starts
// Related: actions.go -- the evidence action table

package evidence

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// runDockerHere runs one evidence script over the current checkout.
func runDockerHere(args leaction.Arguments) (any, int) {
	options, err := ParseDockerRunArguments(args, os.Environ())
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	defer signal.Stop(signals)
	return runDockerAction(NewDockerRun(root, options), signals)
}

func runDockerAction(run *DockerRun, signals <-chan os.Signal) (any, int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received := make(chan os.Signal, 1)
	go func() {
		select {
		case caught := <-signals:
			received <- caught
			cancel()
		case <-ctx.Done():
		}
	}()

	report, err := run.Execute(ctx)
	select {
	case caught := <-received:
		signalNumber, ok := caught.(syscall.Signal)
		if !ok {
			leaction.ReportError(errors.New("evidence run received an unknown signal"))
			return nil, 1
		}
		report.Signal = int(signalNumber)
		report.Code = 128 + report.Signal
		report.Verdict = DockerRunVerdictSignal
		return report, report.Code
	default:
	}
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, dockerRunExitCode(report)
}

func dockerRunExitCode(report DockerRunReport) int {
	if report.Verdict == DockerRunVerdictPass {
		return 0
	}
	if report.Verdict == DockerRunVerdictFail && report.InnerExitCode != 0 {
		return report.InnerExitCode
	}
	if report.Verdict == DockerRunVerdictSignal && report.Signal > 0 {
		return 128 + report.Signal
	}
	return 1
}
