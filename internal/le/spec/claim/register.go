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
	leroot.Register(commandName, leroot.GroupWorkflow, spec.Claim, registry.Meta{
		ShortHelp: "claim a spec for this session and move a ready spec to in-progress",
		Mode:      "offline",
		Section:   registry.SectionTest,
		Subs:      "spec <spec>",
	})
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
