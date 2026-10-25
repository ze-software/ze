// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// This file owns registration. Each composition root needs only a blank import
// of this package.

package speccitation

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	// The table is built once here, so a table that cannot be dispatched
	// panics at init rather than on the first call.
	actions := table()
	leroot.Register(name, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp: shortHelp,
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  actions.Subs,
	})
	leroot.RegisterActions(name, actions.Actions)
	leroot.RegisterShape(name, command.ShapeDoc)

}
