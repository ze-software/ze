// Design: docs/architecture/testing/ci-format.md -- the harness command `le test replay`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testreplay

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// name is the le command this package registers.
const name = "test replay"

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.Answer(cli.CmdReplay), harnesstool.Meta("Replay a captured BGP session (le test replay <capture-file|->) through the real read path with a deterministic clock"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
