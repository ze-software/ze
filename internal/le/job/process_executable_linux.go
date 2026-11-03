//go:build linux

// Design: docs/contributing/running-commands.md -- descriptor-bound child execution.
package job

import (
	"os"
	"os/exec"
)

// processExecutable maps a held executable into the child before exec. Keeping
// the descriptor inherited also lets a script interpreter read that same file.
func processExecutable(cmd *exec.Cmd, executable *os.File) (func() error, error) {
	if executable == nil {
		return nil, nil
	}
	cmd.ExtraFiles = []*os.File{executable}
	cmd.Path = "/proc/self/fd/3"
	return nil, nil
}
