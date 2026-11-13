//go:build linux

// Design: docs/architecture/testing/runner-architecture.md -- three things end a process: the runner dying outright.
// Related: fork_attributes_other.go -- the same merge where Pdeathsig does not exist
// Related: runner_exec_util.go -- startWithETXTBSYRetry, the .ci caller.
// Related: server_process.go -- StartServerProcess, the owned server caller.

package runner

import (
	"syscall"

	"github.com/ze-software/ze/internal/component/plugin"
)

// forkAttributes answers the attributes an owned child is forked with:
// the caller's requested attributes (including credentials and capabilities) with the
// plugin package's fork attributes laid over them, its own process group and
// the kernel's SIGKILL when the forking thread dies. The values come from
// plugin.NewSysProcAttr, never a second literal. requested is copied, never
// changed, so each retry builds the same attributes from it.
func forkAttributes(requested *syscall.SysProcAttr) *syscall.SysProcAttr {
	fork := plugin.NewSysProcAttr()
	if requested == nil {
		return fork
	}
	merged := *requested
	merged.Setpgid = fork.Setpgid
	merged.Pdeathsig = fork.Pdeathsig
	return &merged
}
