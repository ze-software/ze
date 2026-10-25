// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// Package reporewrite keeps the repository's four source-maintenance
// workflows together while exposing each workflow as its own native action.
package reporewrite

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "Deterministic repository rewrites: the rule corpus into its canonical format, BGP expectations reordered without changing bytes, literal or regex replacements, and the activity dashboard.",
		ShortHelp:   "run deterministic repository rewrites",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)
}
