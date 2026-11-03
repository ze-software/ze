//go:build !linux

// Design: docs/contributing/running-commands.md -- descriptor-bound child execution.
package job

import (
	"os"
	"os/exec"
)

func processExecutable(cmd *exec.Cmd, executable *os.File) (func() error, error) {
	return stageProcessExecutable(cmd, executable)
}
