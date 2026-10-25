// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package specwip registers `le spec wip`. The lifecycle itself
// lives in package spec; this directory exists so the command sits at the path
// its name predicts.
package specwip

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/spec"
)

const commandName = "spec wip"

func init() {
	// The table is built once here, so a table that cannot be dispatched
	// panics at init rather than on the first call.
	table := spec.WIPArea()
	leroot.Register(commandName, leroot.GroupWorkflow, spec.WIP, registry.Meta{
		ShortHelp: "the in-progress specs, against the work-in-progress cap",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  table.Subs,
	})
	leroot.RegisterActions(commandName, table.Actions)
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
