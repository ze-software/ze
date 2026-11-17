// Design: docs/architecture/testing/ci-format.md -- the harness command `le test container-seed`
// Related: seed.go -- the seed the command runs
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in this package.

package testcontainerseed

import (
	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
)

// name is the le command this package registers.
const name = "test container-seed"

func init() {
	leroot.Register(name, leroot.GroupTool, harnesstool.Answer(run), harnesstool.Meta("Copy the pinned Redis image into the developer cache"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the seed's own command line, so a trailing
	// help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name, harnesstool.Helper)
}
