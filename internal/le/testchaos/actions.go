// Design: docs/architecture/core-design.md -- the chaos test area, as one command
// Detail: register.go -- the command registration and parity claims
//
// Package testchaos runs the chaos simulator's Go tests, its reduced-tag CLI
// tests, and its linter. The external tools are the implementation under test.
// This package owns their exact arguments and environment.
package testchaos

import (
	"strconv"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// Area is the root command name. It keeps the Python GateSet area unchanged.
const Area = "test-chaos"

const (
	chaosPackages = "./internal/chaos/..."
	chaosCLITags  = "ze_core ze_bgp ze_chaos"
)

type commandKind uint8

const (
	commandUnspecified commandKind = iota
	commandLint
	commandUnit
	commandCLI
)

// Gate defines one chaos gate. Name, Why, and Writes are the Python registry
// contract. The command kind derives both argv and environment, so a race flag
// cannot drift away from CGO_ENABLED=1.
type Gate struct {
	Name   string
	Why    string
	Writes bool
	kind   commandKind
}

// Argv returns the exact command line for this gate.
func (g Gate) Argv(tc gotoolchain.Toolchain) []string {
	switch g.kind {
	case commandLint:
		return []string{"golangci-lint", "run", "-j", strconv.Itoa(tc.Procs), chaosPackages}
	case commandUnit:
		return tc.GoTest(gotoolchain.TestOptions{Race: true}, chaosPackages)
	case commandCLI:
		return []string{
			"go", "test", "-timeout", tc.Timeout, "-tags", chaosCLITags, "./cmd/ze",
		}
	case commandUnspecified:
		panic("BUG: testchaos.Gate has no command kind")
	default:
		panic("BUG: testchaos.Gate has an unknown command kind")
	}
}

// envOptions returns the optional toolchain ceilings for this gate.
func (g Gate) envOptions() gotoolchain.EnvOptions {
	switch g.kind {
	case commandLint:
		return gotoolchain.EnvOptions{MemLimit: true}
	case commandUnit:
		return gotoolchain.EnvOptions{CGO: true, Procs: true}
	case commandCLI:
		return gotoolchain.EnvOptions{Procs: true}
	case commandUnspecified:
		panic("BUG: testchaos.Gate has no command kind")
	default:
		panic("BUG: testchaos.Gate has an unknown command kind")
	}
}

// Overrides returns the environment entries that this gate sets, in the order
// that Toolchain.Environment appends them. Tests use this narrower view because
// inherited host variables are not part of the gate contract.
func (g Gate) Overrides(tc gotoolchain.Toolchain) []string {
	return tc.Overrides(g.envOptions())
}

// environment returns the complete child environment.
func (g Gate) environment(tc gotoolchain.Toolchain) []string {
	return tc.Environment(g.envOptions())
}

// Table returns all three gates in execution order. The linter runs before the
// simulator tests, and the reduced-tag CLI tests run last.
func Table() []Gate {
	return []Gate{
		{
			Name: "ze-chaos-lint",
			Why: "the chaos orchestrator lints clean, under the same two ceilings" +
				" every run has",
			kind: commandLint,
		},
		{
			Name: "ze-chaos-unit-test",
			Why: "the chaos simulator: fault injection, scheduling, the in-process" +
				" reactor",
			kind: commandUnit,
		},
		{
			Name: "ze-chaos-cli-unit-test",
			Why: "the orchestrator's CLI surface, which only a ze_chaos build compiles;" +
				" the default tag set excludes it and reports nothing",
			kind: commandCLI,
		},
	}
}

type gateRunner func(
	gate string,
	argv []string,
	dir string,
	environ []string,
) (gaterun.GateReport, int)

// table builds one invocation's action table. The resolved toolchain is shared
// by all selected gates, so the sweep reads the checkout once.
func table(tc gotoolchain.Toolchain, run gateRunner) leaction.Area {
	gates := Table()
	actions := make([]leaction.Action, 0, len(gates))
	for _, gate := range gates {
		actions = append(actions, leaction.Action{
			Gate:   gate.Name,
			Why:    gate.Why,
			Writes: gate.Writes,
			Forks:  gate.Argv(tc),
			Answer: gateAnswer(tc, run, gate),
		})
	}
	return leaction.New(Area, actions...)
}

// gateAnswer runs one external tool and preserves its output and exit code.
func gateAnswer(tc gotoolchain.Toolchain, run gateRunner, gate Gate) func() (any, int) {
	return func() (any, int) {
		return run(gate.Name, gate.Argv(tc), tc.Root, gate.environment(tc))
	}
}

// metadataOnly supplies command metadata without reading the checkout.
func metadataOnly() leaction.Area { return table(gotoolchain.Toolchain{}, nil) }

// Gates returns each Make target that this area serves.
func Gates() []string { return metadataOnly().Gates() }

// Actions returns the command surface as structured data.
func Actions() leaction.List { return metadataOnly().Actions() }

// Subs returns the help hint derived from the action table.
func Subs() string { return metadataOnly().Subs() }

// Answer is the `le test-chaos` command. A bare command runs all three gates.
// Named gates run in the caller's order. Every action runs after all names pass
// validation, and the first failing tool's own exit code wins.
func Answer(args []string) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	tc, err := gotoolchain.New(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return answerWith(tc, gaterun.Run, args)
}

// answerWith is the resolved half of Answer. The runner parameter lets tests
// observe the exact process boundary without substituting repository logic.
func answerWith(tc gotoolchain.Toolchain, run gateRunner, args []string) (any, int) {
	area := table(tc, run)
	selected := args
	if len(selected) == 0 {
		listing := area.Actions()
		selected = make([]string, 0, len(listing.Actions))
		for _, action := range listing.Actions {
			selected = append(selected, action.Verb)
		}
	}
	return area.Sweep(selected, leaction.RunEveryAction)
}
