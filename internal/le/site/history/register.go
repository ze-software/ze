// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package sitehistory

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "The size of the repository per section on every day of its committed history: measure it into website/data/repo-history.json, or check that the file describes HEAD's history.",
		ShortHelp:   "measure or check the repository size history",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/le/site/facts registers le site facts under it for the same
		// reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, actionTable).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// Every answer this command can give carries one row set, so the row
	// operators act on it rather than being refused. Declaring the shape lets
	// the engine refuse what the shape cannot support BEFORE the tool reads
	// the history.
	leroot.RegisterShape(area, command.ShapeMap)
}
