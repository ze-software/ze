// Design: docs/contributing/running-commands.md -- the Bash guard reads what each area declared
// Related: bash.go -- expensiveWords, which asks leroot.HeavyLine
// Related: bash_test.go -- the piped-command tests that judge these areas' lines
//
// The piped-command guard holds no list of areas: it answers from what each
// area declared where its verb registers (leroot.HeavyLine). A test of this
// package therefore judges a line of an area only when the test binary links
// that area, as the le that runs the hook links every one. This file links
// the areas this package's tests name and can import, and registers a
// stand-in for the one it cannot.

package hookruntime

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"

	// The areas whose lines the piped-command tests and the recorded verdict
	// table judge: `verify current`, `verify worktree`, `verify changed`,
	// `verify deps unit-cached` and `go lint run`. Each declares its heavy
	// actions on its own rows, and none imports this package.
	_ "github.com/ze-software/ze/internal/le/go/lint"
	_ "github.com/ze-software/ze/internal/le/verify"
	_ "github.com/ze-software/ze/internal/le/verify/deps"
)

// unitStandInArea is the name of the unit-test area, `test unit`.
const unitStandInArea = "test unit"

// The initializer registers a stand-in for `test unit` before any test runs. The
// real area (internal/le/test/unit) imports this package, so no test of this
// package can link it. The stand-in declares what the real area declares: its
// rows, and the area registered heavy (leroot.RegisterHeavy), so the bare
// name is a listing and every other line is a run. It proves that the guard
// reads a declaration made under that name. It does not prove the real area
// declares it: TestCensusActionsDeclareHeavy (internal/le) reads the real one.
var _ = registerUnitStandIn()

// registerUnitStandIn registers the stand-in and answers true. The
// registration stays for the process: leroot exports no way to withdraw one,
// and the real area can never be linked beside it.
func registerUnitStandIn() bool {
	nothing := func() (any, int) { return nil, 0 }
	answer := func([]string) (any, int) { return nil, 0 }
	rows := leaction.New(unitStandInArea,
		leaction.Action{Verb: "all", Summary: "stand in for the whole-checkout run", Why: "A row of the stand-in area.", Answer: nothing},
		leaction.Action{Verb: "pkg", Summary: "stand in for the named-package run", Why: "A row of the stand-in area.", AnswerWords: answer},
	)
	leroot.Register(unitStandInArea, leroot.GroupGate, answer, registry.Meta{
		Description: "A stand-in for the unit-test area, which this package cannot link.",
		ShortHelp:   "stand in for the unit-test area",
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
	leroot.RegisterActions(unitStandInArea, rows.Actions)
	leroot.RegisterHeavy(unitStandInArea)
	return true
}

// expensiveProbeArea is the probe area TestExpensiveActionIsReadFromItsRegistration
// registers its action under.
const expensiveProbeArea = "expensive-probe"

// The initializer registers `expensive-probe` before any test runs: an area
// with two light rows, `run` and `list`. An expensive registration is read
// only for a line that resolves to a registered command (leroot.HeavyLine),
// so the test that registers `expensive-probe run` as expensive needs the
// area to exist. Neither row declares anything, so the verdict of a line
// still depends on that registration alone.
var _ = registerExpensiveProbe()

// registerExpensiveProbe registers the probe area and answers true.
func registerExpensiveProbe() bool {
	nothing := func() (any, int) { return nil, 0 }
	answer := func([]string) (any, int) { return nil, 0 }
	rows := leaction.New(expensiveProbeArea,
		leaction.Action{Verb: "run", Summary: "run the probe's selftest", Why: "A light row a test registers as expensive.", Answer: nothing},
		leaction.Action{Verb: "list", Summary: "list what the probe holds", Why: "A light row of the probe.", Answer: nothing},
	)
	leroot.Register(expensiveProbeArea, leroot.GroupGate, answer, registry.Meta{
		Description: "A probe area for the expensive-action test.",
		ShortHelp:   "a probe with an expensive action",
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
	leroot.RegisterActions(expensiveProbeArea, rows.Actions)
	return true
}
