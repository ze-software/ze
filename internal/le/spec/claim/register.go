// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package specclaim registers `le spec claim`. The lifecycle itself
// lives in package spec; this directory exists so the command sits at the path
// its name predicts.
package specclaim

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/spec"
)

const commandName = "spec claim"

func init() {
	// The table is built once here, so a table that cannot be dispatched
	// panics at init rather than on the first call.
	table := spec.ClaimArea()
	leroot.Register(commandName, leroot.GroupWorkflow, spec.Claim, registry.Meta{
		Description: "Claim a spec for this session and move a ready spec to in-progress. The claim names the per-session state file and drives recovery after compaction.",
		ShortHelp:   "claim a spec for this session",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    table.Subs,
	})
	leroot.RegisterActions(commandName, table.Actions)
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
