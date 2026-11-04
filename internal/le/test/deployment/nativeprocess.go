// Design: docs/functional-tests.md -- native proof cancellation and owned descendants.
// Related: deployment.go -- watched native daemons use the same shared runner owner.
package testdeployment

import (
	"context"
	"errors"
	"os/exec"

	"github.com/ze-software/ze/internal/test/runner"
)

// runNativeCommand runs one CommandContext under its originating context. The
// shared owner MUST be closed before returning, including a natural parent exit:
// descendants may still hold files or output after that parent has gone.
func runNativeCommand(ctx context.Context, command *exec.Cmd) (err error) {
	owner, err := runner.StartServerProcess(command)
	if err != nil {
		return errors.Join(ctx.Err(), err)
	}
	defer func() {
		cleanupErr := owner.Close()
		err = errors.Join(ctx.Err(), cleanupErr)
	}()
	<-owner.Done()
	return nil
}
