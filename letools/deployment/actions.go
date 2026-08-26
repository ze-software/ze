// Design: docs/architecture/core-design.md -- the deployment area, as one command
// Detail: l2tp.go -- the L2TP peer proof this table reaches
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are letools/leaction, which every ported area shares.
//
// The gate names are the family: ze-deployment-l2tp-test, -vpp-test,
// -vpp-iface-test and the rest all begin ze-deployment-, so each verb is that
// name with the area's prefix removed and every new proof is one row here.

package deployment

import (
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "deployment"

var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-deployment-l2tp-test",
		Why: "the L2TP control session against an external peer." +
			" Needs Docker and a privileged container",
		Answer: runL2TPHere,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le deployment` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runL2TPHere proves L2TP over the checkout this command was run in.
func runL2TPHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runL2TP(NewL2TP(root))
}

// runL2TP answers the proof over one run. A step that could not be performed is
// an error and answers 1 with no verdict; a session that did not establish is
// the verdict, and it answers 1 with the daemon's last lines behind it.
func runL2TP(run *L2TP) (L2TPReport, int) {
	report, err := run.Run()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if !report.Established {
		return report, 1
	}
	return report, 0
}
