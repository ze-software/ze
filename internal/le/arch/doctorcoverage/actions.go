// Design: docs/architecture/doctor-and-health-checks.md -- the doctor-coverage gate, as one command
//
// Overview: doctorcoverage.go -- the site kinds and statuses, and the scan
//
// The three codes stay apart: 0 for a scope whose problem sites all name a
// doctor code, 1 for one that adds a site that does not, and 2 for a run the
// gate could not make (a change set git cannot answer, a file that does not
// parse, a door page with no header, a scan or covered set too thin to judge).

package archdoctorcoverage

import (
	"github.com/ze-software/ze/internal/le/hookruntime"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// area is the name this command is typed as.
const area = "arch doctor-coverage"

// siteFloor is the fewest problem sites the real tree's scan may find. The
// tree held about 2,300 Warn and Error lines when the gate was written; a scan
// below this matched a shape that no longer exists.
const siteFloor = 1000

// coveredCodesFloor is the fewest codes the tree's doctor check declarations
// may list. The tree registered over 60 doctor checks when the gate was
// written.
const coveredCodesFloor = 20

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "check",
		Summary: "refuse a changed problem site with no doctor code",
		Why:     "no changed Warn or Error log line or report-bus raise lands without a code that a registered doctor check declares, so ze doctor answers for every problem an operator can hit; the sites already in the tree stay visible in report",
		Answer:  runCheck,
	},
	leaction.Action{
		Verb:    "report",
		Summary: "list problem sites with no doctor code, by door",
		Why:     "every problem site the tree holds that no doctor check covers, grouped by door area and owner package with counts, for the doctor coverage audit",
		Answer:  runReport,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le arch doctor-coverage` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le arch doctor-coverage check` action.
//
// It judges the LINES in the unpushed range, committed or not, against the
// same base the other line-scoped arch checks read
// (repochanged.LinesSinceUpstream). A package or file scope would make every
// uncovered site already in a touched package due on the first edit beside
// it; line scope stops the class growing without making an old site any
// session's debt. A change set git cannot answer exits 2, never 0.
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

// runReport is the `le arch doctor-coverage report` action. It answers every
// uncovered site in the tree and exits 0 on them, because a reader auditing
// the tree is not a gate. A scan that could not run still exits 2.
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
	return treeReport(inventory), 0
}

// scanCheckout reads the doors, scans the checkout with the covered set its
// own doctor check declarations list, and refuses a thin covered set or a
// thin scan.
func scanCheckout(tree string) (Inventory, int) {
	doorsOf, err := hookruntime.DoorsCovering(tree)
	if err != nil {
		leaction.ReportError(err)
		return Inventory{}, 2
	}
	inventory, err := Scan(tree, coveredCodesFloor, doorsOf)
	if err != nil {
		leaction.ReportError(err)
		return Inventory{}, 2
	}
	if err := checkSitesFloor(inventory, siteFloor); err != nil {
		leaction.ReportError(err)
		return Inventory{}, 2
	}
	return inventory, 0
}
