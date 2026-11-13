// Design: docs/functional-tests.md -- route-policy fixture command failures.
// Related: rib_import_daemon_fixture.go -- isolated link setup and cleanup.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

const importCommandOutputLimit = 8192

// importCommandOutput runs one fixture-selected network mutation with the
// observer's network authority. It MUST join the child before returning; an
// earlier caller or cleanup deadline MUST remain authoritative.
func importCommandOutput(ctx context.Context, name string, args ...string) (output string, result error) {
	attributes, err := importCommandAttributes()
	if err != nil {
		return "", err
	}
	commandCtx, cancel := context.WithTimeout(ctx, WaitBudget(10, 15*time.Second))
	defer cancel()
	command := exec.CommandContext(commandCtx, name, args...) //nolint:gosec // the fixture selects the ip program and its complete arguments, never shell text.
	command.SysProcAttr = attributes
	var stdout, stderr importCommandCapture
	command.Stdout = &stdout
	command.Stderr = &stderr
	command.WaitDelay = time.Second
	child, err := runner.StartServerProcess(command)
	if err != nil {
		return "", fmt.Errorf("start route-policy fixture %s %q: %w", name, args, err)
	}
	defer func() {
		if err := errors.Join(result, child.Close(), commandCtx.Err()); err != nil {
			result = fmt.Errorf("route-policy fixture %s %q failed (stderr %q): %w", name, args, stderr.buffer[:stderr.used], err)
		}
	}()
	<-child.Done()
	return string(stdout.buffer[:stdout.used]), nil
}

// importCommandCapture owns one bounded mutation-command stream. Its zero value
// is an empty valid capture; a nil receiver is invalid. The child owner MUST
// finish copying before a caller reads buffer or used.
type importCommandCapture struct {
	buffer [importCommandOutputLimit]byte
	used   int
}

func (capture *importCommandCapture) Write(data []byte) (int, error) {
	written := copy(capture.buffer[capture.used:], data)
	capture.used += written
	if written < len(data) {
		return written, fmt.Errorf("route-policy fixture command output exceeds %d bytes", importCommandOutputLimit)
	}
	return written, nil
}
