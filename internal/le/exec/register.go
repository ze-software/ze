// Design: docs/guide/developer-setup.md -- le exec: the launcher the tool shims run
// Related: exec.go -- the environment, the GOROOT rule and the refusals

package leexec

import (
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// name is the le command this package registers.
const name = "exec"

// noSlotReason is why `le exec` takes no job slot (leroot.RegisterNoSlot).
const noSlotReason = "it replaces its own process with the program it carries, so nothing remains to release a slot; " +
	"it will be admitted by the program it carries"

func init() {
	leroot.Register(name, leroot.GroupWorkflow, answer, registry.Meta{
		Description: "Run a program with its caches under the per-user cache directory, and with Ze's patched GOROOT active when it runs inside a checkout.",
		ShortHelp:   "run a program with Ze's caches and GOROOT",
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the program path is the program's own command line, so
	// a trailing help word reaches the program and it prints its own help.
	leroot.RegisterForwarding(name, "the program its first word names")
	// The program is the caller's choice, most often the Go toolchain, so every
	// line of this area is heavy work the main thread schedules
	// (leroot.HeavyLine).
	leroot.RegisterHeavy(name)
	// The dispatcher admits a heavy line by taking a slot and releasing it
	// when the body returns, and this body does not return: le becomes the
	// program (replace, exec.go), under the same PID, so the slot's entry
	// would stay alive for as long as the program runs. A tool shim's gopls
	// would hold a slot for the editor's life, and a linter started by
	// `le go lint run` would wait for a second slot inside its parent's.
	leroot.RegisterNoSlot(name, noSlotReason)
	// The area runs the program its first word names and prints that
	// program's output, so it carries a command and holds no verdict of its
	// own (leroot.CarrierLine).
	leroot.RegisterCarrier(name)
}

// answer is `le exec <path> [args...]`. On success it never returns: le
// becomes the program. It answers 0 for a help request, 2 for a usage error,
// and 1 for a program it refuses to run.
func answer(args []string) (any, int) {
	return nil, run(args, os.Stdout, os.Stderr)
}
