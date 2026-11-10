// Design: docs/contributing/error-inventory.md -- the error inventory, as one command
//
// Overview: inventory.go -- the row, its surface, status, source and pre-rank
//
// The three codes stay apart: 0 for a scope that adds no refused message
// site, 1 for one that adds one, and 2 for a run the command could not make
// (a change set git cannot answer, a root that is not a tree).

package archerrorinventory

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// area is the name this command is typed as.
const area = "arch error-inventory"

// checkoutRoot answers the tree both actions read. It is a variable so the
// action tests run the registered command over a testdata tree.
var checkoutRoot = lepath.Root

// checkoutFloors are the floors the real checkout's scan must meet. On
// 2026-11-10 the tree held about 230 error answers beside the handlers'
// returned errors, about 2,650 doctor-coverage sites against 191 declared
// codes, and over 1,500 writer rows (stderr writes and http.Error alone).
// DoctorCoverageSites and DoctorCodes are doctor-coverage's own floors
// (siteFloor, coveredCodesFloor in internal/le/arch/doctorcoverage/actions.go),
// which that package does not export. It is a variable so the action tests
// scan a testdata tree with no floor.
var checkoutFloors = Floors{
	OperatorErrorSites:  150,
	DoctorCoverageSites: 1000,
	DoctorCodes:         20,
	WriterRows:          1000,
}

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "check",
		Summary: "refuse an added message site that names no fix",
		Why:     "no changed web, REST, gNMI, MCP, local CLI or le message site lands as a plain string, so every message a user reads names what went wrong, why, and the command that fixes it; the operator-error and doctor-coverage checks judge their own sites, and the sites already in the tree stay visible in report",
		Answer:  runCheck,
	},
	leaction.Action{
		Verb:    "report",
		Summary: "list every user-visible message site, by pre-rank",
		Why:     "every place a message reaches a user, once, with its surface, location, owner package, door area, status and pre-rank, and one count line per surface, as the worklist for messages that do not name their fix",
		Answer:  runReport,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le arch error-inventory` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le arch error-inventory check` action.
//
// It judges the LINES in the unpushed range, committed or not, against the
// same base the other line-scoped arch checks read
// (repochanged.LinesSinceUpstream), so a site already in the tree is
// reported, never refused. A change set git cannot answer exits 2, never 0.
func runCheck() (any, int) {
	tree, err := checkoutRoot()
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
	report := checkLines(inventory.Rows, changed)
	report.Base = base
	report.Scope = changedScope(base, changed)
	return report, report.exitCode()
}

// runReport is the `le arch error-inventory report` action. It answers every
// row in the tree and exits 0 on them, because a reader auditing the tree is
// not a gate. A scan that could not run still exits 2.
func runReport() (any, int) {
	tree, err := checkoutRoot()
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

// scanCheckout scans the tree against checkoutFloors, and answers exit 2 for
// a scan that could not run or found too little.
func scanCheckout(tree string) (Inventory, int) {
	inventory, err := Scan(tree, checkoutFloors)
	if err != nil {
		leaction.ReportError(err)
		return Inventory{}, 2
	}
	return inventory, 0
}

// changedScope says in words what the check judged.
func changedScope(base repochanged.LineBase, changed repochanged.ChangedLines) string {
	var tb textbuf.Buffer
	tb.Str("the working tree against ").Str(base.Commit)
	if base.Reason != "" {
		tb.Str(" (").Str(strings.TrimSpace(base.Reason)).Byte(')')
	}
	tb.Str(", ").Int(int64(len(changed))).Str(" changed file(s), changed lines only")
	return tb.String()
}
