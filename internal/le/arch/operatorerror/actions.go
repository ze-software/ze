// Design: docs/contributing/ze-go-style.md -- the operator-error area, as one command
//
// Overview: operatorerror.go -- the boundary set and the return-site statuses
//
// The three codes stay apart: 0 for a scope whose handler refusals all carry
// the type, 1 for one that adds a plain refusal, and 2 for a run the gate
// could not make (a change set git cannot answer, a file that does not
// parse, a scan that found too few registrations to judge).

package archoperatorerror

import (
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// area is the name this command is typed as.
const area = "arch operator-error"

// registrationFloor is the fewest registrations the real tree's scan may
// find. The tree held about 600 when the gate was written; a scan below this
// matched a shape that no longer exists, and its zero plain sites would read
// as a clean tree.
const registrationFloor = 300

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "check",
		Summary: "refuse a changed handler line that returns a plain error",
		Why:     "no changed return of a registered command handler, RPC handler or config verifier hands the operator a plain errors.New or fmt.Errorf, which names no fix; the sites already in the tree stay visible in report",
		Answer:  runCheck,
	},
	leaction.Action{
		Verb:    "report",
		Summary: "list every handler return site in the tree, typed, plain or pass-through",
		Why:     "every refusal a registered operator handler returns, with its status, file and line, handler and registration kind, for a reader deciding which to convert and for the error inventory",
		Answer:  runReport,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le arch operator-error` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le arch operator-error check` action.
//
// It judges the LINES in the unpushed range, committed or not, against the
// same base arch compound-guard reads (repochanged.LinesSinceUpstream). A
// package or file scope would make every plain refusal already in a touched
// package due on the first edit beside it; line scope stops the class
// growing without making an old site any session's debt. A change set git
// cannot answer exits 2, never 0.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	changed, base, err := repochanged.LinesSinceUpstream(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	inventory, code := scanCheckout(tree)
	if code != 0 {
		return nil, code
	}
	report := checkLines(inventory.Sites, changed)
	report.Base = base
	report.Scope = changedScope(base, changed)
	return report, report.exitCode()
}

// runReport is the `le arch operator-error report` action. It answers every
// site in the tree and exits 0 on them, because a reader deciding what to
// convert is not a gate. A scan that could not run still exits 2.
func runReport() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	inventory, code := scanCheckout(tree)
	if code != 0 {
		return nil, code
	}
	return TreeReport{Registrations: len(inventory.Registrations), Sites: inventory.Sites, Unresolved: inventory.Unresolved}, 0
}

// scanCheckout scans the checkout and refuses a thin scan.
func scanCheckout(tree string) (Inventory, int) {
	inventory, err := Scan(tree)
	if err != nil {
		leaction.ReportError(err)
		return Inventory{}, 2
	}
	if err := checkFloor(inventory, registrationFloor); err != nil {
		leaction.ReportError(err)
		return Inventory{}, 2
	}
	return inventory, 0
}
