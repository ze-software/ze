// Design: docs/architecture/testing/ci-format.md -- the harness command `le mock record-plugin`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package mockrecordplugin

import (
	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// name is the le command this package registers.
const name = "mock record-plugin"

func init() {
	// Spawned BY test daemons as an external plugin
	// (plugin { external record-plugin { run "le mock record-plugin" } })
	// to drive test/plugin/plugin-owned-command-streams.ci,
	// plugin-reads-engine-answer.ci and plugin-command-partial-fault.ci.
	leroot.Register(name, leroot.GroupTool, harnesstool.Answer(cli.CmdRecordPlugin), harnesstool.Meta("answer commands with a record walk"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Ze starts it, a person never types it: help and completion leave it
	// out, and `le help ai` lists it marked hidden.
	leroot.RegisterHidden(name)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name, harnesstool.Helper)
}
