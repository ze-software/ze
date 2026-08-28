// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// Package sourcerewrite keeps the repository's four source-maintenance
// workflows together while exposing each workflow as its own native action.
package sourcerewrite

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "deterministic repository rewrites: rules, BGP expectations, replacements, and activity HTML",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)
}
