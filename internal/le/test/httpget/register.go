// Design: docs/architecture/testing/ci-format.md -- the harness command `le test http-get`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testhttpget

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// name is the le command this package registers.
const name = "test http-get"

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.Answer(cli.CmdHTTPGet), harnesstool.Meta("Fetch one HTTP URL to stdout for container-local test tooling"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
