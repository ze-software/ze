// Design: docs/architecture/testing/ci-format.md -- the harness command `le test vpp`, and `le test vpp stub`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testvpp

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// name is the le command this package registers.
const name = "test vpp"

// stubWord is the first word that picks the GoVPP socket stub a VPP test
// starts, `le test vpp stub ...`, instead of the suite.
const stubWord = "stub"

func init() {
	// The suite runs the functional runner, so it admits its run. The stub is
	// a helper a running test starts inside the slot its suite already holds,
	// so it never admits (harnesstool.Answer).
	suite := harnesstool.RunnerAnswer(name, cli.CmdVpp)
	stub := harnesstool.Answer(cli.CmdVPPStub)
	answer := func(args []string) (any, int) {
		if len(args) > 0 && args[0] == stubWord {
			return stub(args[1:])
		}
		return suite(args)
	}
	leroot.Register(name, leroot.GroupSuite, answer, harnesstool.Meta("Run VPP stub-backed functional tests (test/vpp/*.ci), or the stub itself"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
