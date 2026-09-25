// Design: docs/architecture/testing/ci-format.md -- the harness command `le test fixture`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testfixture

import (
	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/fixture"
)

// name is the le command this package registers.
const name = "test fixture"

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.Answer(fixture.Run), harnesstool.Meta("Run one compiled .ci fixture helper"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
