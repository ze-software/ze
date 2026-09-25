// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package specwip registers `le spec wip`. The lifecycle itself
// lives in package spec; this directory exists so the command sits at the path
// its name predicts.
package specwip

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/spec"
)

const commandName = "spec wip"

func init() {
	leroot.Register(commandName, leroot.GroupWorkflow, spec.WIP, registry.Meta{
		ShortHelp: "the in-progress specs, against the work-in-progress cap",
		Mode:      "offline",
		Section:   registry.SectionTest,
	})
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
