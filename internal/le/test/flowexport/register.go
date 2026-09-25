// Design: docs/architecture/testing/ci-format.md -- the harness command `le test flow-export`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testflowexport

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// suiteName is the suite's name, which is also its directory under test/.
const suiteName = "flow-export"

// name is the le command this package registers.
const name = "test " + suiteName

// suite is the functional suite the command runs.
var suite = cli.CIRunnerConfig{
	Name:            suiteName,
	TestSubdir:      "flow-export",
	Description:     "flow-export",
	Detail:          "Run flow-export functional tests (.ci files in test/flow-export/).\nCovers sFlow v5, NetFlow v9, and IPFIX counter export over UDP, the show flow export handler, packet-sampling wiring, and reload-time reconfiguration.",
	DefaultParallel: 0,
}

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.SuiteAnswer(suite), harnesstool.SuiteMeta(suite))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
