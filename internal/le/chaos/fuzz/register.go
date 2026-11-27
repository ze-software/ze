// Design: docs/guide/chaos-testing.md -- the chaos swarm fuzzer command
// Overview: fuzz.go -- the entry this registration exposes
//
// One package owns one command. Composition imports it from
// internal/le/register.go.

package chaosfuzz

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(name, leroot.GroupSuite, Answer, registry.Meta{
		ShortHelp: "sweep random in-process chaos seeds with swarm faults",
		Description: "Run VOPR-style multi-seed in-process swarm fuzzing against Ze's BGP reactor. " +
			"Each seed randomizes peer topology, BGP chaos actions, route dynamics, and wire-level " +
			"stream faults (fragmentation, asymmetric blackholes, mid-frame resets), checking " +
			"online FSM/router-ID invariants and BGP properties on every seed. " +
			"Takes options such as `--seeds 20 --duration 15s --check-determinism`.",
		Mode:    "offline",
		Section: registry.SectionTest,
	})

	// The command writes its own per-seed progress and summary; no structured payload.
	leroot.RegisterShape(name, command.ShapeDoc)

	// Forward options directly to fuzz.CLIRun so `--help` prints the fuzzer's own option list.
	leroot.RegisterForwarding(name, "the chaos swarm fuzzer")

	// Runs in-process reactors across multiple seeds, so the job scheduler admits it as heavy.
	leroot.RegisterHeavy(name)
}
