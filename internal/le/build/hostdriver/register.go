// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: hostdriver.go -- the build this registration exposes
//
// One package owns one command. Composition imports it from
// internal/le/register.go.

package buildhostdriver

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// shortHelp is what the build makes. The help line and the action's Why both
// read it, so the two cannot drift.
const shortHelp = "build ze-host, the `ze appliance ...` driver that runs on the build machine, at the checkout root"

// runVerb names the build, which is also what the bare command line runs.
const runVerb = "run"

// table declares the whole command surface. The one action takes no value:
// there is one host and one output path.
var table = leaction.New(name, leaction.Action{
	Verb:   runVerb,
	Why:    shortHelp,
	Answer: answerRun,
})

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp: shortHelp,
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  table.Subs,
	})
	leroot.RegisterActions(name, table.Actions)

	// The answer is one build report, so the map shape renders its fields.
	leroot.RegisterShape(name, command.ShapeMap)
}
