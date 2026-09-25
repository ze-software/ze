// Design: docs/architecture/testing/ci-format.md -- the harness command `le test pppoe`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testpppoe

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// suiteName is the suite's name, which is also its directory under test/.
const suiteName = "pppoe"

// name is the le command this package registers.
const name = "test " + suiteName

// suite is the functional suite the command runs.
var suite = cli.CIRunnerConfig{
	Name:            suiteName,
	TestSubdir:      "pppoe",
	Description:     "PPPoE",
	Detail:          "Run PPPoE access-concentrator functional tests (.ci files in test/pppoe/).\nCovers RFC 2516 discovery over a real veth pair: PADI/PADO with AC-Name and\nAC-Cookie, PADR/PADS session allocation, forged-cookie rejection, an 802.1Q\nsub-interface, and PPPoE running alongside L2TP on one daemon. Every test\ndeclares option=netns-link, so `./le test qemu pppoe-test` runs them and they SKIP\neverywhere else. That action supplies both halves: the per-test netns launch\nmode, and ze's runtime kernel, whose CONFIG_PPPOE the AF_PPPOX session behind\nPADS needs.",
	DefaultParallel: 0,
}

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.SuiteAnswer(suite), harnesstool.SuiteMeta(suite))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
