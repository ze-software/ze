// Design: ai/rules/protocol.md -- the protocol-skeleton area, as one command
//
// actions.go is the Python area, ported. `le repository
// ze-protocol-skeleton-report` selected one gate out of a GateSet; `le
// protocol-skeleton report` selects one action out of the table below.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction. What stays here is the TABLE.

package protocolskeleton

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "protocol-skeleton"

// selftestFailure is the code the selftest answers when a fixture did not
// behave as declared. It is the script's, unchanged: 1.
const selftestFailure = 1

// actions is the whole command surface. Only the report is a Make target; the
// selftest has no gate of its own and carries its verb instead.
var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-protocol-skeleton-report",
		Why:    "which protocol implementations are still a skeleton rather than a daemon",
		Answer: runReport,
	},
	leaction.Action{
		Verb: "selftest",
		Why:  "the classifier still tells the five classes apart, which the advisory's own page cannot show",
		Answer: func() (any, int) {
			result := Selftest()
			return result, result.Code(selftestFailure)
		},
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le protocol-skeleton` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runReport classifies the checkout. It is ADVISORY: a report that was built
// always answers 0, whatever it found, because an enforced skeleton would need
// an allowlist this repository has already decided against. Only a tree that
// could not be READ answers non-zero.
func runReport() (any, int) {
	tree, err := treeOfRecord()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	report, err := Build(tree, Manifest())
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// treeOfRecord answers the checkout this lens judges.
func treeOfRecord() (string, error) { return lepath.Root() }
