// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package configclaims

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// areaDescription is what the gate judges. The area's long help and the action's Why both
// read it, so the two cannot drift.
const areaDescription = "every config subtree an operator can write is delivered to a plugin, a hub handler, or a recorded exception"

// checkVerb names the gate, which is also what the bare command line runs.
const checkVerb = "check"

// table declares the whole command surface. The one action takes no value: what
// it judges is the linked registry and the rendering is a pipe operator.
var table = leaction.New(name, leaction.Action{
	Verb:    checkVerb,
	Summary: "check every config subtree is claimed and delivered",
	Why:     areaDescription,
	Answer:  answerCheck,
})

func init() {
	leroot.Register(name, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp:   "check every config subtree is delivered",
		Description: areaDescription,
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section:  registry.SectionTest,
		SubsFunc: table.Subs,
	})
	leroot.RegisterActions(name, table.Actions)

	// The answer is ONE document holding two row sets, the exceptions and the
	// findings, so the row operators are refused by name while `| json`,
	// `| yaml` and `| table` render it (internal/component/command/answer_shape.go).
	leroot.RegisterShape(name, command.ShapeDoc)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}
