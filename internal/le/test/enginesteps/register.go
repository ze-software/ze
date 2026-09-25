// Design: docs/architecture/testing/ci-format.md -- the harness command `le test engine-steps`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testenginesteps

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// name is the le command this package registers.
const name = "test engine-steps"

func init() {
	// Spawned BY test daemons as an external plugin
	// (plugin { external engine-steps { run "le test engine-steps ./engine-steps.json" } })
	// to drive .ci command=/stream=/expect=output|event|stream directives.
	leroot.Register(name, leroot.GroupSuite, harnesstool.Answer(cli.CmdEngineSteps), harnesstool.Meta("Execute .ci engine-step directives as an external plugin (spawned by test daemons, not run directly)"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
