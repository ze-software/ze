// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package speccurrent registers `le spec current`. The lifecycle itself
// lives in package spec; this directory exists so the command sits at the path
// its name predicts.
package speccurrent

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/spec"
)

const commandName = "spec current"

func init() {
	// The table is built once here, so a table that cannot be dispatched
	// panics at init rather than on the first call.
	table := spec.CurrentArea()
	leroot.Register(commandName, leroot.GroupWorkflow, spec.Current, registry.Meta{
		ShortHelp: "the spec this session claimed",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  table.Subs,
	})
	leroot.RegisterActions(commandName, table.Actions)
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
