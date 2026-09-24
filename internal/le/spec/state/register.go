// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package specstate registers `le spec state`. The lifecycle itself
// lives in package spec; this directory exists so the command sits at the path
// its name predicts.
package specstate

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/spec"
)

const commandName = "spec state"

func init() {
	leroot.Register(commandName, leroot.GroupWorkflow, spec.State, registry.Meta{
		ShortHelp: "the path of a per-spec session state file",
		Mode:      "offline",
		Section:   registry.SectionTest,
		Subs:      "current | latest spec <stem>",
	})
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
