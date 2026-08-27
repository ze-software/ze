// Design: docs/architecture/core-design.md -- the fuzz area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in internal/le/leaction. What stays here is the
// TABLE, and the four knobs the Make recipe passed as flags.
//
// THE KNOBS ARE ENVIRONMENT, not values typed after a keyword. `make
// ze-fuzz-test-one FUZZ=FuzzParseNLRIs PKG=./internal/... TIME=30s` already
// exports all three, because GNU make puts a command-line variable into the
// recipe environment. So the documented invocation keeps working unchanged, and
// the CLI grammar keeps its rule that a keyword comes before a value
// (ai/rules/cli.md). internal/le/trackedbuild took the same route for REV.

package fuzz

import (
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as.
const area = "fuzz"

// typeString is the env registry's word for a free-text value. It is a constant
// because four entries declare it and a typo in one of them would register a
// type nothing reads.
const typeString = "string"

// The keys the four run-time knobs are read under, each carrying the alias the
// Make recipe already spells.
const (
	// NameKey selects one target by a Go fuzz regexp.
	NameKey = "ze.fuzz.name"
	// PackageKey selects the package holding it, wildcard included.
	PackageKey = "ze.fuzz.package"
	// TimeKey is the per-target fuzz duration.
	TimeKey = "ze.fuzz.time"
	// TimeoutKey is the hard per-target ceiling above the fuzz duration.
	TimeoutKey = "ze.fuzz.timeout"
)

var nameEntry = env.MustRegister(env.EnvEntry{
	Key:         NameKey,
	Type:        typeString,
	Default:     "",
	Description: "one fuzz target to run, as the Go regexp go test takes; discovery is not consulted",
	Aliases:     []string{"FUZZ"},
	// Private keeps the key out of `ze env list`. It is a build-host knob and
	// an operator has nothing to do with it.
	Private: true,
})

var packageEntry = env.MustRegister(env.EnvEntry{
	Key:         PackageKey,
	Type:        typeString,
	Default:     "",
	Description: "the package a named run fuzzes; a wildcard reaches go test unaltered",
	Aliases:     []string{"PKG"},
	Private:     true,
})

var timeEntry = env.MustRegister(env.EnvEntry{
	Key:         TimeKey,
	Type:        typeString,
	Default:     "",
	Description: "the fuzz duration per target; the sweep defaults to 10s and a named run to 30s",
	Aliases:     []string{"TIME"},
	Private:     true,
})

var timeoutEntry = env.MustRegister(env.EnvEntry{
	Key:         TimeoutKey,
	Type:        typeString,
	Default:     DefaultTimeout,
	Description: "the hard per-target timeout above the fuzz duration",
	Private:     true,
})

// actions is the whole command surface.
//
// No action carries a Gate. `ze-fuzz-test` and `ze-fuzz-test-one` are Make targets.
// The Python le does not declare either target among its 156 gates.
// This area therefore claims no census gate, and the parity count does not change.
var actions = leaction.New(area,
	leaction.Action{
		Verb:   "run",
		Why:    "fuzz every `func Fuzz` under internal/, 10s each, stopping at the first crash",
		Answer: runSweep,
	},
	leaction.Action{
		Verb:   "list",
		Why:    "what a run would do: every target and its package, or the one argv a named run would exec",
		Answer: runList,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le fuzz` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// sweeper builds the run this invocation asked for, out of the checkout and the
// four knobs.
//
// env.Get first resolves an alias to its canonical key, then tries the supplied spelling.
// Asking by the alias therefore reads ZE_FUZZ_NAME before bare FUZZ.
func sweeper() (*Sweeper, error) {
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	chain, err := gotoolchain.New(root)
	if err != nil {
		return nil, err
	}
	return &Sweeper{
		Chain:    chain,
		Root:     root,
		Name:     env.Get(nameEntry.Aliases[0]),
		Package:  env.Get(packageEntry.Aliases[0]),
		FuzzTime: env.Get(timeEntry.Aliases[0]),
		Timeout:  env.Get(timeoutEntry.Key),
	}, nil
}

// runList is the `le fuzz list` action.
func runList() (any, int) {
	run, err := sweeper()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	plan, err := run.Plan()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return plan, 0
}

// runSweep implements `le fuzz run`.
// Its payload is the complete verdict, while per-target progress has already gone to stderr.
// Thus, `le fuzz run | json` returns one document for the sweep.
func runSweep() (any, int) {
	run, err := sweeper()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return run.Run()
}
