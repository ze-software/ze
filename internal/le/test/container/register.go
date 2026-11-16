// Design: docs/architecture/testing/ci-format.md -- the harness command `le test container`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testcontainer

import (
	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// suiteName is the suite's name, which is also its directory under test/.
const suiteName = "container"

// name is the le command this package registers.
const name = "test " + suiteName

// suite is the functional suite the command runs.
var suite = cli.CIRunnerConfig{
	Name:            suiteName,
	TestSubdir:      suiteName,
	Description:     "Containers",
	Detail:          "Run container functional tests (.ci files in test/container/).\nCovers the containers Ze runs from OCI images: start, confinement, limits, supervision, stop and cleanup, and the image store, network, shell and operator surface built on them. A test that starts a container carries option=needs-linux and runs as root in the QEMU guest.",
	DefaultParallel: 0,
}

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.SuiteAnswer(suite), harnesstool.SuiteMeta(suite))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name, harnesstool.Runner)
}
