// Design: docs/guide/chaos-testing.md -- the chaos orchestrator command
// Overview: run.go -- the entry this registration exposes
//
// One package owns one command. Composition imports it from
// internal/le/register.go.

package chaosrun

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(name, leroot.GroupSuite, Answer, registry.Meta{
		ShortHelp: "run the chaos orchestrator against ze, FRR or BIRD: `le chaos run [options]`",
		Description: "Takes the orchestrator's options, for example " +
			"`--seed 42 --peers 4 --duration 30s`, `--config-only`, or `--in-process --web :8080`. " +
			"A help word asks le, so docs/guide/chaos-testing.md shows the options in use.",
		Mode:    "offline",
		Section: registry.SectionTest,
	})

	// The command answers no payload: the program it runs writes its own
	// output. The document shape is declared so no shape is inherited, and it
	// renders nothing when the payload is nil.
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the program's own command line, so a
	// trailing help word reaches the program and it prints its own help.
	leroot.RegisterForwarding(name)
}
