// Design: docs/architecture/appliance/builder.md -- full image build (assemble + gok + ext4)
// Related: gok_execute_other.go -- the refusal on a host gokrazy cannot pack on

//go:build linux || darwin

package appliance

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gokrazy/tools/gok"
)

// ExecuteGok runs gok's command line over args, writing to this process's
// stdout and stderr. Both `ze appliance build` and `le build gokrazy` reach gok
// only through here, so the host restriction lives in one place.
// The caller's working directory is restored afterwards: see keepWorkingDir.
func ExecuteGok(ctx context.Context, args []string) error {
	return keepWorkingDir(func() error {
		return gok.Context{
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Args:   args,
		}.Execute(ctx)
	})
}

// keepWorkingDir runs run, then returns this process to the directory it was
// in. gok's commands chdir into the instance directory and never chdir back
// (gokrazy/tools internal/gok, overwrite.go), and `le build gokrazy` builds
// from a prepared instance it removes afterwards. A caller that keeps working
// after the build, as `./le setup docker host start` does when it then runs
// Colima, would otherwise hand every child a removed working directory: Lima
// then fails to cd into it inside the guest and Colima cannot enable Rosetta.
func keepWorkingDir(run func() error) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("read the working directory before running gok: %w", err)
	}
	runErr := run()
	if err := os.Chdir(wd); err != nil {
		return errors.Join(runErr, fmt.Errorf("return to %s after running gok: %w", wd, err))
	}
	return runErr
}
