// Design: docs/architecture/testing/ci-format.md -- the harness area `le test scale`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

// Package testscale registers `le test scale`, the area that groups the scale
// tests. Its first word picks the test: `le test scale l2tp`.
package testscale

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// name is the le command this package registers.
const name = "test scale"

// members are the scale tests, in listing order.
var members = []harnesstool.AreaMember{
	{Word: "l2tp", Help: "L2TP scale test: LAC simulator + mock RADIUS", Run: cli.CmdL2tpScale},
}

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.Answer(harnesstool.Area(name, members)), harnesstool.Meta("Run a scale test: l2tp"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name reaches the area, so the test word and a
	// trailing help word reach the harness and it prints its own help.
	leroot.RegisterForwarding(name)
}
