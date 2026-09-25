// Design: docs/architecture/testing/ci-format.md -- the harness command `le test fail-syscall`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testfailsyscall

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/failsyscall"
)

// name is the le command this package registers.
const name = "test fail-syscall"

func init() {
	// Fault injection under a daemon, with no tracer: a seccomp filter answers
	// one errno for one syscall, then the launcher execs the daemon. A tracer
	// charges the failing call to itself, which is what made the earlier
	// failing-socket measurement unreadable (internal/test/failsyscall).
	leroot.Register(name, leroot.GroupSuite, harnesstool.Answer(failsyscall.Run), harnesstool.Meta("Launch a command with one syscall failing in the kernel (seccomp SECCOMP_RET_ERRNO; Linux only)"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
