// Design: docs/architecture/testing/ci-format.md -- the harness command `le test runner`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testrunner

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// suiteName is the suite's name, which is also its directory under test/.
const suiteName = "runner"

// name is the le command this package registers.
const name = "test " + suiteName

// suite is the functional suite the command runs.
var suite = cli.CIRunnerConfig{
	Name:            suiteName,
	TestSubdir:      "runner",
	Description:     "runner",
	Detail:          "Run test-runner primitive functional tests (.ci files in test/runner/).\nCovers the .ci orchestration grammar itself: naming a background process and stopping it mid-test (cmd=background:name=, cmd=stop).",
	DefaultParallel: 0,
}

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.SuiteAnswer(suite), harnesstool.SuiteMeta(suite))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
