// Design: docs/architecture/core-design.md -- native module migration workflows
package gomodule

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// Both reports render themselves. Text has a pointer receiver, so each action
// MUST answer a pointer; a value would render as a bare JSON dump.
var (
	_ leroot.Prose = (*MoveReport)(nil)
	_ leroot.Prose = (*RenameReport)(nil)
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "Preview or apply a move of an internal package tree, or a rename of the repository's Go module path, with every import and spelling rewritten.",
		ShortHelp:   "move package trees or rename the module",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)

}
