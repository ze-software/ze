// Design: docs/functional-tests.md -- ze-test invocation and test-only race build
// Overview: run.go -- orchestration owns cancellation and result classification

package stressrepro

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type realProcessRunner struct{}

func (realProcessRunner) Invoke(ctx context.Context, spec invocation) processResult {
	words, err := shellWords(spec.suite)
	if err != nil {
		return processResult{code: 2, err: fmt.Errorf("split suite: %w", err)}
	}
	testWords, err := shellWords(spec.test)
	if err != nil {
		return processResult{code: 2, err: fmt.Errorf("split test: %w", err)}
	}
	argv := append(words, "-v")
	if len(testWords) == 0 {
		argv = append(argv, "--all")
	} else {
		argv = append(argv, testWords...)
	}

	invokeCtx, cancel := context.WithTimeout(ctx, spec.timeout)
	defer cancel()
	env := append([]string(nil), os.Environ()...)
	env = setEnvironment(env, "ze.bin", spec.zeBin)
	env = setEnvironment(env, "ze.test.bin", spec.testBin)
	env = setEnvironment(env, "ZE_TEST_NO_BUILD", "1")
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

func (realProcessRunner) buildRace(ctx context.Context, root, output, tags string) processResult {
	env := setEnvironment(append([]string(nil), os.Environ()...), "CGO_ENABLED", "1")
	return runCommand(ctx, root, env, "go",
		"build", "-race", "-tags", tags,
		"-ldflags", "-X main.version=stress -X main.buildDate=stress",
		"-o", output, "./cmd/ze")
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
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
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
