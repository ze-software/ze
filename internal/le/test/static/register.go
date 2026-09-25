// Design: docs/architecture/testing/ci-format.md -- the harness command `le test static`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package teststatic

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// suiteName is the suite's name, which is also its directory under test/.
const suiteName = "static"

// name is the le command this package registers.
const name = "test " + suiteName

// suite is the functional suite the command runs.
var suite = cli.CIRunnerConfig{
	Name:            suiteName,
	TestSubdir:      "static",
	Description:     "static",
	Detail:          "Run static route functional tests (.ci files in test/static/).\nCovers boot-time apply, reload add/remove, and show output.",
	DefaultParallel: 1,
}

func init() {
	// Serial (1), not parallel: every test in this suite programs routes into
	// the ONE kernel routing table the VM has, so concurrent daemons see each
	// other's prefixes and delete each other's routes on shutdown. Run in
	// parallel under QEMU it fails 5 of 7 ("initial static route not programmed
	// before reload", "reload did not remove 172.16.0.0/12" while another test's
	// blackhole shows up in the dump); serial, those same tests pass.
	leroot.Register(name, leroot.GroupSuite, harnesstool.SuiteAnswer(suite), harnesstool.SuiteMeta(suite))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
