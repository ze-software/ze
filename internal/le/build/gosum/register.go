// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package buildgosum

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// areaDescription is what the gate judges. The area's long help and the action's Why both
// read it, so the two cannot drift.
const areaDescription = "the packed gokrazy/<instance>/builddir/**/go.sum files agree with the root module about what a version contains"

// checkVerb names the gate, which is also what the bare command line runs.
const checkVerb = "check"

// table declares the whole command surface. The one action takes no value: the
// tree is the checkout and the rendering is a pipe operator.
var table = leaction.New(name, leaction.Action{
	Verb:    checkVerb,
	Summary: "check the packed go.sum files are current",
	Why:     areaDescription,
	Answer:  answerCheck,
})

func init() {
	leroot.Register(name, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp:   "check the packed gokrazy go.sum files",
		Description: areaDescription,
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section:  registry.SectionTest,
		SubsFunc: table.Subs,
	})
	leroot.RegisterActions(name, table.Actions)

	// One key holds rows and the other two are the counts that say what was
	// compared, so the row operators act on the conflicts and every any-shape
	// operator renders the whole answer.
	leroot.RegisterShape(name, command.ShapeMap)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}
