// Design: docs/functional-tests.md -- le test invocation and the daemon build
// Overview: run.go -- orchestration owns cancellation and result classification

package teststressrepro

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	coreenv "github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

type realProcessRunner struct{}

func (realProcessRunner) Invoke(ctx context.Context, spec invocation) processResult {
	argv, err := spec.argv()
	if err != nil {
		return processResult{code: 2, err: err}
	}
	invokeCtx, cancel := context.WithTimeout(ctx, spec.timeout)
	defer cancel()
	// The child runner sees only its own selected cases. Carry contention
	// from sibling invocations through the existing runner factor, once.
	factor := max(runner.ChildParallelFactor(), spec.parallelFactor)
	env := coreenv.Without(os.Environ(), runner.ParallelFactorEnv)
	env = setEnvironment(env, strings.ReplaceAll(runner.ParallelFactorEnv, ".", "_"), strconv.Itoa(factor))
	env = setEnvironment(env, "ze.bin", spec.zeBin)
	env = setEnvironment(env, runner.EnvNoBuild, "1")
	env = setEnvironment(env, "GOTRACEBACK", "all")
	if spec.extraTags != "" {
		env = setEnvironment(env, "ze.tags", spec.extraTags)
	}
	result := runCommand(invokeCtx, spec.root, env, spec.testBin, argv...)
	if errors.Is(result.err, context.DeadlineExceeded) {
		result.output += fmt.Sprintf(
			"\n[stress-repro: invocation timed out after %.0fs]\n", spec.timeout.Seconds())
	}
	return result
}

// argv is shared by execution and capture so evidence retains the exact child
// argument boundaries, including quoted selectors and registered suite options.
func (spec invocation) argv() ([]string, error) {
	testWords, err := shellWords(spec.test)
	if err != nil {
		return nil, fmt.Errorf("split test: %w", err)
	}
	argv := make([]string, 0, len(spec.harness)+2+max(1, len(testWords)))
	argv = append(argv, "test")
	argv = append(argv, spec.harness...)
	argv = append(argv, "-v")
	if len(testWords) == 0 {
		argv = append(argv, "--all")
	} else {
		argv = append(argv, testWords...)
	}
	return argv, nil
}

// build compiles the daemon under test from the working tree in root.
// A race build needs cgo, so it enables it; a plain build keeps the caller's.
func (realProcessRunner) build(ctx context.Context, root, output, tags string, race bool) processResult {
	env := append([]string(nil), os.Environ()...)
	argv := []string{"build"}
	if race {
		env = setEnvironment(env, "CGO_ENABLED", "1")
		argv = append(argv, "-race")
	}
	argv = append(argv, "-tags", tags,
		"-ldflags", "-X main.version=stress -X main.buildDate=stress",
		"-o", output, "./cmd/ze")
	return runCommand(ctx, root, env, "go", argv...)
}

// buildPlugin compiles the test plugin main package pkg from the working tree
// in root to output, with the caller's environment and no feature tag.
func (realProcessRunner) buildPlugin(ctx context.Context, root, output, pkg string) processResult {
	return runCommand(ctx, root, os.Environ(), "go", "build", "-o", output, pkg)
}

func runCommand(ctx context.Context, dir string, env []string, program string, argv ...string) processResult {
	cmd := exec.CommandContext(ctx, program, argv...) //nolint:gosec // program and argv come only from the closed tool grammar
	cmd.Dir = dir
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return processResult{code: 2, output: stdout.String() + stderr.String(), err: err}
	}
	err := cmd.Wait()
	output := stdout.String() + stderr.String()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return processResult{code: 124, output: output, err: context.DeadlineExceeded}
	}
	if err == nil {
		return processResult{output: output}
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return processResult{code: exitErr.ExitCode(), output: output}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return processResult{code: 1, output: output, err: context.Canceled}
	}
	return processResult{code: 2, output: output, err: err}
}

func setEnvironment(environment []string, key, value string) []string {
	prefix := key + "="
	for index, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			environment[index] = prefix + value
			return environment
		}
	}
	return append(environment, prefix+value)
}
