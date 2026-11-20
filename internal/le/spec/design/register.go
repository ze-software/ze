// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Related: actions.go -- the seven actions this area registers
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

// Package specdesign registers `le spec design`: the derivation of the design
// documents a spec's files link, the check of its Design documents table, the
// independent judge's verdict, Thomas's approval, his away admission, and the
// list of ready specs that wait for him. Its package sits at the path its
// command name predicts.
package specdesign

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: areaDescription,
		ShortHelp:   "judge a spec's design and record Thomas's approval",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section:  registry.SectionTest,
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// Every answer is one document: a derivation, a verdict or a record.
	leroot.RegisterShape(area, command.ShapeDoc)
}
