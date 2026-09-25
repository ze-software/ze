// Design: docs/architecture/testing/ci-format.md -- the harness command `le test l2tp-wire`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testl2tpwire

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// suiteName is the suite's name, which is also its directory under test/.
const suiteName = "l2tp-wire"

// name is the le command this package registers.
const name = "test " + suiteName

// suite is the functional suite the command runs.
var suite = cli.CIRunnerConfig{
	Name:            suiteName,
	TestSubdir:      "l2tp-wire",
	Description:     "l2tp-wire",
	Detail:          "Run L2TP wire-level functional tests (.ci files in test/l2tp-wire/).\nCovers control message decode (SCCRQ) and truncated packet handling.",
	DefaultParallel: 0,
}

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.SuiteAnswer(suite), harnesstool.SuiteMeta(suite))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
