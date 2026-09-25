// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// Package reporewrite keeps the repository's four source-maintenance
// workflows together while exposing each workflow as its own native action.
package reporewrite

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		ShortHelp: "deterministic repository rewrites: rules, BGP expectations, replacements, and activity HTML",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)
}
