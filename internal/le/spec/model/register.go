// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package specmodel registers `le spec model`. The lifecycle itself
// lives in package spec; this directory exists so the command sits at the path
// its name predicts.
package specmodel

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/spec"
)

const commandName = "spec model"

func init() {
	leroot.Register(commandName, leroot.GroupWorkflow, spec.Model, registry.Meta{
		ShortHelp: "the model a session transcript records",
		Mode:      "offline",
		Section:   registry.SectionTest,
		Subs:      "current [transcript <absolute-path>]",
	})
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
