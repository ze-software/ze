// Design: docs/functional-tests.md -- executable identity in isolated process proofs.
// Related: sysrib_execution_fixture.go -- authenticated daemon and child census.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/test/runner"
)

const (
	sysribExecutableOutputLimit  = 8192
	sysribExecutableProbeMaximum = 15 * time.Second
)

// sysribProcessExecutable reads one exact process identity. If procfs refuses
// a non-root observer access to a setcap daemon, the native guest's existing
// sudo policy permits this same identity read. A failed inspection MUST remain
// a failed observation, never evidence that a process is absent.
func sysribProcessExecutable(ctx context.Context, pid int) (string, error) {
	return readSysribProcessExecutable(ctx, pid, os.Readlink, runSysribExecutableCommand)
}

func readSysribProcessExecutable(ctx context.Context, pid int, readlink func(string) (string, error), privileged func(context.Context, []string) ([]byte, error)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if pid < 1 {
		return "", errors.New("process executable lookup requires a positive authenticated PID")
	}
	path := "/proc/" + strconv.Itoa(pid) + "/exe"
	identity, readErr := readlink(path)
	if err := ctx.Err(); err != nil {
		return "", errors.Join(readErr, err)
	}
	if readErr == nil {
		return identity, nil
	}
	if !errors.Is(readErr, syscall.EACCES) {
		return "", readErr
	}
	// This metadata read is capped even when the whole carrier has a larger
	// load-adjusted budget, and a shorter parent deadline remains authoritative.
	budget := min(WaitBudget(10, sysribExecutableProbeMaximum), sysribExecutableProbeMaximum)
	probeCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	argv := []string{"sudo", "-n", "readlink", path}
	output, err := privileged(probeCtx, argv)
	if err != nil {
		return "", sysribExecutableProbeError(path, readErr, err)
	}
	if err := probeCtx.Err(); err != nil {
		return "", sysribExecutableProbeError(path, readErr, err)
	}
	identity, err = parseSysribExecutableReply(output)
	if err != nil {
		return "", sysribExecutableProbeError(path, readErr, err)
	}
	slogutil.Logger("test.observer").Warn("read executable identity through privileged procfs probe", "pid", pid, "error", readErr)
	return identity, nil
}

func sysribExecutableProbeError(path string, permission, cause error) error {
	return fmt.Errorf("inspect executable with sudo -n readlink %s: %w; fix: run this carrier through ./le test qemu ci test <ci-path> with the native guest's passwordless sudo policy", path, errors.Join(permission, cause))
}

func parseSysribExecutableReply(output []byte) (string, error) {
	if len(output) > sysribExecutableOutputLimit {
		return "", fmt.Errorf("executable identity reply exceeds %d bytes", sysribExecutableOutputLimit)
	}
	if !strings.HasSuffix(string(output), "\n") {
		return "", errors.New("executable identity reply is not one complete line")
	}
	identity := string(output[:len(output)-1])
	if identity == "" {
		return "", errors.New("executable identity reply is empty")
	}
	if strings.ContainsRune(identity, '\n') {
		return "", errors.New("executable identity reply contains multiple lines")
	}
	if strings.ContainsRune(identity, 0) {
		return "", errors.New("executable identity reply contains a NUL byte")
	}
	if !filepath.IsAbs(identity) {
		return "", errors.New("executable identity reply is not an absolute path")
	}
	return identity, nil
}

// sysribExecutableCapture owns at most one bounded metadata response. Its zero
// value is an empty valid capture; a nil receiver is invalid. Only Write changes
// used, preserving its bound before bytes are exposed after the child joins.
type sysribExecutableCapture struct {
	buffer [sysribExecutableOutputLimit]byte
	used   int
}

func (capture *sysribExecutableCapture) Write(data []byte) (int, error) {
	written := copy(capture.buffer[capture.used:], data)
	capture.used += written
	if written < len(data) {
		return written, fmt.Errorf("executable identity command output exceeds %d bytes", sysribExecutableOutputLimit)
	}
	return written, nil
}

// runSysribExecutableCommand MUST use the caller's bounded context and join its
// sole child owner before releasing either bounded capture. StartServerProcess
// requires the checked deferred Close even after Wait reports an ordinary exit.
func runSysribExecutableCommand(ctx context.Context, argv []string) (output []byte, resultErr error) {
	if err := sysribExecutableChildControl(); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // fixed sudo/readlink argv and validated numeric procfs path, never shell text.
	var stdout, stderr sysribExecutableCapture
	command.Stdout = &stdout
	command.Stderr = &stderr
	command.WaitDelay = time.Second
	child, err := runner.StartServerProcess(command)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, child.Close())
	}()
	if err := child.Wait(); err != nil {
		return nil, fmt.Errorf("privileged executable reader failed (stderr %q): %w", stderr.buffer[:stderr.used], errors.Join(err, ctx.Err()))
	}
	return stdout.buffer[:stdout.used], nil
}
