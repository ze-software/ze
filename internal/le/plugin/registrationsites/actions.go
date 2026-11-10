// Design: docs/architecture/core-design.md -- the plugin registration-sites area, as one command
//
// actions.go holds the TABLE. The dispatch, the listing, the help line and the
// refusals live in internal/le/le/action, which every le area shares.

package pluginregistrationsites

import (
	"context"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "plugin registration-sites"

// actions is the whole command surface. The check's Why is the rule a refusal
// sends its reader to (ruleCommand).
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "check",
		Summary: "check every registration sits in package initialization",
		Why: "the plugin registry seals when main starts, so every call to a registering function of the registry " +
			"(an exported registry function that reaches the seal guard) must run while packages initialize. " +
			"The rule, over the committed HEAD's Ze packages: the call sits in a register*.go file; " +
			"it is a direct call, never a function value and never inside a function literal or a method; " +
			"and it sits in init(), in a package-level initializer, or in a function whose every reference is, " +
			"in turn, a direct call only package initialization of a register*.go file reaches. " +
			"The registry must also not be dot-imported",
		Answer: runCheck,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le plugin registration-sites` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le plugin registration-sites check` action.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the registration-sites check found no Ze checkout",
			"it reads the committed source of the repository it runs in",
			"run ./le plugin registration-sites check from inside a Ze checkout"))
		return nil, 2
	}
	report, err := Check(context.Background(), tree)
	if err != nil {
		// 2 rather than 1: a tree the check could not read is a different fact
		// from a tree holding a wrong site.
		leaction.ReportError(opererr.Wrap(err, "the registration-sites check could not read the committed tree",
			"it reads HEAD's Ze packages under the Linux le build context, and a tree it cannot read has no verdict",
			"commit or fix what the cause names, then run ./le plugin registration-sites check again"))
		return nil, 2
	}
	if len(report.Refused) != 0 {
		return report, 1
	}
	return report, 0
}
