// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package specrelease registers `le spec release`. The lifecycle itself
// lives in package spec; this directory exists so the command sits at the path
// its name predicts.
package specrelease

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/spec"
)

const commandName = "spec release"

func init() {
	leroot.Register(commandName, leroot.GroupWorkflow, spec.Release, registry.Meta{
		ShortHelp: "release this session's spec claim",
		Mode:      "offline",
		Section:   registry.SectionTest,
	})
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
