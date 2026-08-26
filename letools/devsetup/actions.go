// Design: docs/guide/developer-setup.md -- the setup area as one command
//
// actions.go contains the ported Python setup area. The dispatch, help line,
// and two refusals are in letools/leaction. Only the TABLE remains here.
//
// THREE ACTIONS REPLACE TWO FLAGS. Each action represents one effective
// invocation. The script declared --check and --no-vendor. These flags have
// four combinations, but only three can differ. The --no-vendor flag never
// reaches a check run because check mode returns before the vendoring step.
//
//	install   probe, install what is missing, then synchronize the vendor tree.
//	check     probe only, change nothing, and fail when a required tool is missing.
//	tools     install as above and do not change the vendor tree.
//
// THE BARE COMMAND RUNS INSTALL. This is what `./le setup` has always done and
// what every document that names it means. `le functional` made the same
// choice for the same reason. This area has one long run, so it must not show a
// list to the operator who starts that run.

package devsetup

import (
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// area is the name this command is typed as.
const area = "setup"

// The three verbs, named once because each is spelled in the table, in the
// dispatch below and in the tests.
const (
	installVerb = "install"
	checkVerb   = "check"
	toolsVerb   = "tools"
)

// actions is the whole command surface.
//
// No action carries a Gate: `./le gates --json` declares 156 gates and none of
// them is this tool. No Makefile target runs it, so the census counts this
// directory under script-files and the parity count does not move for the port.
var actions = leaction.New(area,
	leaction.Action{
		Verb:   installVerb,
		Writes: true,
		Why:    "install every tool a Ze dev or test workflow needs, then bring vendor/ in step with go.mod",
		Answer: func() (any, int) { return run(false, true) },
	},
	leaction.Action{
		Verb:   checkVerb,
		Why:    "probe only; change nothing, and fail when a required tool is missing",
		Answer: func() (any, int) { return run(true, true) },
	},
	leaction.Action{
		Verb:   toolsVerb,
		Writes: true,
		Why:    "install as `install` does, and leave vendor/ alone",
		Answer: func() (any, int) { return run(false, false) },
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le setup` command. A bare command is the install run.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return run(false, true)
	}
	return actions.Answer(args)
}

// run runs every action in this area. The two options that each action passes
// are the only differences.
//
// Use 2 instead of 1 when the checkout cannot be found. No probe ran in this
// case. The caller can distinguish it from a run that found a missing tool.
func run(check, vendor bool) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	setup := &Setup{Root: root, Check: check, Vendor: vendor}
	return setup.Run()
}
