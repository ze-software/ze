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

func init() {
	leroot.Register(name, leroot.GroupWorkflow, answer, registry.Meta{
		ShortHelp: "run a program with its caches under the per-user cache, and Ze's patched GOROOT only inside a checkout",
		Mode:      "offline",
		Section:   registry.SectionTest,
	})
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the program path is the program's own command line, so
	// a trailing help word reaches the program and it prints its own help.
	leroot.RegisterForwarding(name)
}

// answer is `le exec <path> [args...]`. On success it never returns: le
// becomes the program. It answers 0 for a help request, 2 for a usage error,
// and 1 for a program it refuses to run.
func answer(args []string) (any, int) {
	return nil, run(args, os.Stdout, os.Stderr)
}
