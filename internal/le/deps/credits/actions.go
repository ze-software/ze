// Design: docs/architecture/third-party-credits.md -- the deps credits area, as one command
//
// The action table is the single source for dispatch, help, completion, the
// listing and the closed keyword grammar.

package credits

import (
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/deps"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "deps credits"

// checkCommand is the line that runs the whole-tree check again.
const checkCommand = "./le " + area + " check"

// treeInputs reads the checkout itself: the go command for the linked sets,
// git for the Go files.
var treeInputs = inputs{linked: linkedByFlavor, goFiles: deps.GoFiles}

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "check",
		Summary: "check the credits of every shipped third-party work",
		Why: "refuse a module a shipped flavor links that " + DeclarationRel + " does not declare, a declared holder or " +
			"licence the work's own files do not state, a work no package declares present, a bundled library no work " +
			"claims, and generated data in " + DataRel + " that differs from what update writes. It reads the whole " +
			"tree and runs go list once per flavor and system; it runs on demand and in CI, never as a commit gate",
		Heavy:  true,
		Answer: checkAnswer,
	},
	leaction.Action{
		Verb:    "update",
		Summary: "regenerate the credits data the binary embeds",
		Why: "write " + DataRel + " from " + DeclarationRel + ": one record per shipped work and each licence and notice " +
			"file copied byte for byte, then remove every file no work references. It refuses, writing nothing, when a " +
			"shipped module has no entry or a work has no licence file",
		Writes: true,
		Heavy:  true,
		Answer: updateAnswer,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le deps credits` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// checkAnswer runs Check over this checkout: exit 1 with refusals, 2 when the
// tree could not be read.
func checkAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "deps credits check cannot find the Ze checkout",
			"The declaration, the generated data and the modules each flavor links are read from it. Change to the Ze checkout containing go.mod and feature-gates.txt before running this command.",
			checkCommand))
		return nil, 2
	}
	report, err := Check(root, treeInputs)
	if err != nil {
		leaction.ReportError(treeError(err, "deps credits check cannot read the tree",
			"Nothing was judged. Correct the cause and run the check again; it is "+DeclarationRel+" that does not parse, a flavor go list cannot load, or a file that cannot be read",
			checkCommand))
		return nil, 2
	}
	if len(report.Refusals) != 0 {
		return report, 1
	}
	return report, 0
}

// updateAnswer runs Update over this checkout: exit 1 with the refusals that
// stopped it, 2 when the tree could not be read or written.
func updateAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "deps credits update cannot find the Ze checkout",
			"The data the binary embeds is generated from the declaration in it. Change to the Ze checkout containing go.mod and feature-gates.txt before running this command.",
			updateCommand))
		return nil, 2
	}
	report, refusals, err := Update(root, treeInputs)
	if err != nil {
		leaction.ReportError(treeError(err, "deps credits update cannot read or write the tree",
			"The data in "+DataRel+" can be incomplete when a write failed. Correct the cause and run update again; it is "+DeclarationRel+" that does not parse, a flavor go list cannot load, or a file that cannot be read or written",
			updateCommand))
		return nil, 2
	}
	if len(refusals) != 0 {
		return Report{Refusals: refusals}, 1
	}
	return report, 0
}

// treeError answers err as the operator error an action reports. An operator
// error a producer built already names its own fix and passes through
// unchanged; a bare one, whose text names the cause, is wrapped with what the
// action could not do and the command that runs it again once the cause is
// corrected.
func treeError(err error, problem, why, fix string) *opererr.Error {
	if refusal, built := errors.AsType[*opererr.Error](err); built {
		return refusal
	}
	return opererr.Wrap(err, problem, why, fix)
}
