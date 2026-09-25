// Design: docs/architecture/testing/runner-architecture.md -- the harness binary and its variables
// Related: ../../../test/harnessbin/harnessbin.go -- the harness file name and variables

// Package testharness was `le test harness`, and registers nothing since the
// harness commands became `le test <name>` (D-8); Phase 1d package b deletes
// it with the other harness builders. It builds the harness binary
// bin/le-test on every call, then runs it with the trailing argv and
// answers its exit code.
//
// The harness cannot be linked into le. Its roots (bgp, web, lg, peer, ...)
// collide with the roots of ze in a ze_le build, and a duplicate root panics at
// registration. The harness must also exist as a file, because containers and
// QEMU guests run it. So le execs it.
package testharness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	"github.com/ze-software/ze/internal/test/harnessbin"
)

// Area is the words this command is typed as.
const Area = "test harness"

// harnessBase is the personality tag the harness carries before the feature
// gates, which repofeaturetags reads from feature-gates.txt.
const harnessBase = "ze_test"

// jobLabel names the harness build in the job registry.
const jobLabel = "test-harness-build"

// cannotRun is the exit code when the harness cannot be built or started.
const cannotRun = 1

// Answer builds the harness, then runs it with args.
func Answer(args []string) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, cannotRun
	}
	return answerIn(root, args)
}

// answerIn is Answer over a named checkout root.
//
// The harness is built on EVERY call, so the file that answers is always the
// tree's: a harness kept because it exists answers for the code it was built
// from. Go's build cache makes an unchanged rebuild cheap. LE_TEST_NO_BUILD is
// the explicit opt-out, and it runs the named harness as it is, refusing when
// that file is absent.
//
// A bare `test harness` runs the harness with no argv, which prints its
// command list and exits 1 because it was given no command. The list IS the
// answer to the bare form, so the bare form exits 0, as a bare le namespace
// token does. Every other form answers the harness's own exit code.
func answerIn(root string, args []string) (any, int) {
	binary := harnessPath(root)
	if harnessbin.NoBuild() {
		if _, err := os.Stat(binary); err != nil {
			leaction.ReportError(fmt.Errorf("LE_TEST_NO_BUILD is set and the harness %s is not there: %w", binary, err))
			return nil, cannotRun
		}
	} else if code := build(root, binary); code != 0 {
		return nil, code
	}
	code := run(binary, args)
	if code == cannotStart {
		return nil, cannotRun
	}
	if len(args) == 0 {
		return nil, 0
	}
	return nil, code
}

// harnessPath answers the harness file: le.test.bin (or its retired spelling)
// when a caller names one, else bin/le-test of the checkout. It never looks the
// harness up on PATH. A relative name is read against the checkout root.
func harnessPath(root string) string {
	named := harnessbin.TestBin()
	if named == "" {
		return filepath.Join(root, "bin", harnessbin.Name)
	}
	if filepath.IsAbs(named) {
		return named
	}
	return filepath.Join(root, named)
}

// buildArgv answers the go build command that writes the harness to binary,
// with the ze_test personality and every feature gate.
func buildArgv(root, binary string) ([]string, error) {
	tags, err := repofeaturetags.DaemonBuildTags(root, harnessBase)
	if err != nil {
		return nil, err
	}
	return []string{"go", "build", "-tags", tags, "-o", binary, "./cmd/ze"}, nil
}

// build compiles the harness under job admission, then writes its retired
// name beside it (harnessbin.LinkRetired). It answers an exit code.
func build(root, binary string) int {
	argv, err := buildArgv(root, binary)
	if err != nil {
		leaction.ReportError(err)
		return cannotRun
	}
	admission, err := job.NewIn(root)
	if err != nil {
		leaction.ReportError(err)
		return cannotRun
	}
	ticket, err := admission.Admit(jobLabel, argv)
	if err != nil {
		leaction.ReportError(err)
		return cannotRun
	}
	code := ticket.Code
	if ticket.Kind != job.KindAttached {
		tc, tcErr := gotoolchain.New(root)
		if tcErr != nil {
			ticket.Release(cannotRun)
			leaction.ReportError(tcErr)
			return cannotRun
		}
		code = gaterun.Stream(argv, root, tc.Environment(gotoolchain.EnvOptions{}))
		ticket.Release(code)
	}
	if code != 0 {
		return code
	}
	if _, err := harnessbin.LinkRetired(binary); err != nil {
		leaction.ReportError(err)
		return cannotRun
	}
	return 0
}

// cannotStart is the code run answers when the harness did not start at all.
// It is outside the range an exiting process reports.
const cannotStart = -1

// run executes the harness with args on this process's terminal and answers
// its exit code, or cannotStart.
func run(binary string, args []string) int {
	command := exec.CommandContext(context.Background(), binary, args...) //nolint:gosec // the checkout's own harness, or the one the caller named
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	leaction.ReportError(err)
	return cannotStart
}
