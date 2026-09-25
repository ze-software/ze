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
	leroot.Register(commandName, leroot.GroupWorkflow, spec.Review, registry.Meta{
		ShortHelp: "independent review artifacts: hash, record and check",
		Mode:      "offline",
		Section:   registry.SectionTest,
		Subs:      "hash | record | check",
	})
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
