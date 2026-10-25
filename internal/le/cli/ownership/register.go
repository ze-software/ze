// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package cliownership

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// shortHelp is what the gate judges. The help line and the action's Why both
// read it, so the two cannot drift.
const shortHelp = "each command is owned by exactly one plugin or component: owners are cmd/ze-free, root handlers are internal, and every central root states why it has no owner"

// checkVerb names the gate, which is also what the bare command line runs.
const checkVerb = "check"

// table declares the whole command surface. The one action takes no value: the
// tree is the checkout and the rendering is a pipe operator.
var table = leaction.New(name, leaction.Action{
	Verb:   checkVerb,
	Why:    shortHelp,
	Answer: answerCheck,
})

func init() {
	leroot.Register(name, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp: shortHelp,
		Mode:      "offline",
		// SectionTest is where ze files a tool rather than a product command.
		Section:  registry.SectionTest,
		SubsFunc: table.Subs,
	})
	leroot.RegisterActions(name, table.Actions)

	// The answer IS the rows, so the row operators act on the violations rather
	// than being refused (internal/component/command/answer_shape.go).
	leroot.RegisterShape(name, command.ShapeMap)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}
