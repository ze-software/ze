// Design: docs/architecture/testing/ci-format.md -- the harness command `le test json-ip-bytes`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testjsonipbytes

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// name is the le command this package registers.
const name = "test json-ip-bytes"

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.Answer(cli.CmdJSONIPBytes), harnesstool.Meta("Sum byte counters for an IP across named JSON arrays"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
