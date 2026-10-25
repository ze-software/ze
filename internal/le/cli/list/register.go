// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package clilist

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// name is the word this command is typed as.
const name = "cli list"

// shortHelp is what the report holds. The help line and the action's Why both
// read it, so the two cannot drift.
const shortHelp = "every registered command, by verb, read from the live handlers and schemas"

// reportVerb names the report, which is also what the bare command line runs.
const reportVerb = "report"

// table declares the whole command surface. The one action takes no value.
var table = leaction.New(name, leaction.Action{
	Verb:   reportVerb,
	Why:    shortHelp,
	Answer: answerReport,
})

func init() {
	leroot.Register(name, leroot.GroupReport, Answer, registry.Meta{
		ShortHelp: shortHelp,
		Mode:      "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section:  registry.SectionTest,
		SubsFunc: table.Subs,
	})
	leroot.RegisterActions(name, table.Actions)

	// The answer IS the rows, so the row operators act on the commands rather
	// than being refused. Declaring it lets the engine refuse what the shape
	// cannot support BEFORE the tool loads the registry (ai/rules/cli.md).
	leroot.RegisterShape(name, command.ShapeMap)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}
