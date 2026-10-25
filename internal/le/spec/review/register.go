// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package specreview registers `le spec review`. The lifecycle itself
// lives in package spec; this directory exists so the command sits at the path
// its name predicts.
package specreview

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/spec"
)

const commandName = "spec review"

func init() {
	// The table is built once here, so a table that cannot be dispatched
	// panics at init rather than on the first call.
	table := spec.ReviewArea()
	leroot.Register(commandName, leroot.GroupWorkflow, spec.Review, registry.Meta{
		Description: "Independent review artifacts: hash the files a review covers, record a session's review of them, and check that a recorded review still covers them as they are now.",
		ShortHelp:   "hash, record and check independent reviews",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    table.Subs,
	})
	leroot.RegisterActions(commandName, table.Actions)
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
