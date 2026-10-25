// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package specstate registers `le spec state`. The lifecycle itself
// lives in package spec; this directory exists so the command sits at the path
// its name predicts.
package specstate

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/spec"
)

const commandName = "spec state"

func init() {
	// The table is built once here, so a table that cannot be dispatched
	// panics at init rather than on the first call.
	table := spec.StateArea()
	leroot.Register(commandName, leroot.GroupWorkflow, spec.State, registry.Meta{
		Description: "The path of a per-spec session state file: this session's own, or the newest any session wrote for one spec, which is where a handoff is read from.",
		ShortHelp:   "print a per-spec session state file path",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    table.Subs,
	})
	leroot.RegisterActions(commandName, table.Actions)
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
