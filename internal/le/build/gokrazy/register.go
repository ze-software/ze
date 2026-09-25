// Design: docs/guide/appliance.md -- gokrazy build tool wrapper
// Overview: gokrazy.go -- the gok run this registration exposes
//
// One package owns one command. Composition imports it from
// internal/le/register.go.

package buildgokrazy

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp: "run gokrazy's gok with the checked-in module cache, offline, from a prepared instance: `le build gokrazy <gok args>`",
		Description: "Takes gok's own arguments, for example " +
			"`--parent_dir gokrazy -i ze overwrite --full ze.img`. " +
			"`overwrite` builds from a prepared copy of the instance under tmp/, so the tracked gokrazy/ directory is never written.",
		Mode:    "offline",
		Section: registry.SectionTest,
	})

	// The command answers no payload: the program it runs writes its own
	// output. The document shape is declared so no shape is inherited, and it
	// renders nothing when the payload is nil.
	leroot.RegisterShape(name, command.ShapeDoc)
}

// Answer runs gok with args and answers its exit code. The payload is nil
// because gok writes its own progress and image: a build is a program, not a
// value for a pipe operator to render.
func Answer(args []string) (any, int) {
	return nil, Run(args)
}
