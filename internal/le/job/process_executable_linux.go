//go:build linux

// Design: docs/contributing/running-commands.md -- descriptor-bound child execution.
package job

import (
	"os"
	"os/exec"
)

// processExecutable maps a held executable into the child before exec. Keeping
// the descriptor inherited also lets a script interpreter read that same file.
// A nil cleanup with a nil error means preparation succeeded without an artifact to remove.
func processExecutable(cmd *exec.Cmd, executable *os.File) (func() error, error) {
	if executable == nil {
		return nil, nil //nolint:nilnil // runProcess accepts successful preparation with no cleanup obligation.
	}
	cmd.ExtraFiles = []*os.File{executable}
	cmd.Path = "/proc/self/fd/3"
	return nil, nil //nolint:nilnil // The inherited descriptor needs no preparation artifact cleanup.
}
