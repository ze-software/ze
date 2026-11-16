// Design: docs/functional-tests.md -- exact child exit before local doctor
// Related: register_ddos_flowspec_orphan_cleared.go -- native driver registration

package fixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// doctorPluginFatalStop owns the daemon's full lifecycle. Its successful return
// means the actual child exited 1 and was reaped; stdout and stderr keep their
// original destinations so the .ci can judge the orderly fatal-stop logs.
func doctorPluginFatalStop(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/doctor-fatal-stop")
	}
	command := exec.CommandContext(ctx, "ze", "start", "running.conf")
	command.Env = os.Environ()
	return doctorPluginFatalWait(ctx, command, os.Stdout, os.Stderr)
}

func doctorPluginFatalWait(ctx context.Context, command *exec.Cmd, stdout, stderr io.Writer) error {
	command.Stdout, command.Stderr = stdout, stderr
	// Run owns Start and Wait synchronously, including CommandContext's kill.
	// Never infer success from output or return before that Wait reaps the child.
	err := command.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("the fatal-plugin daemon did not complete inside its context: %w", ctx.Err())
	}
	if err == nil {
		return errors.New("the fatal-plugin daemon exited 0, want its actual fatal-stop exit 1")
	}
	exit, exited := errors.AsType[*exec.ExitError](err)
	if !exited {
		return fmt.Errorf("the fatal-plugin daemon could not run: %w", err)
	}
	if exit.ExitCode() != 1 {
		return fmt.Errorf("the fatal-plugin daemon exited %d, want its actual normal exit 1: %w", exit.ExitCode(), err)
	}
	return nil
}
