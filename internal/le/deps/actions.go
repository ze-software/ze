// Design: docs/contributing/dependency-approval.md -- the deps area, as one command
//
// The action table is the single source for dispatch, help, completion, the
// listing and the closed keyword grammar.

package deps

import (
	"errors"
	"strings"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "deps"

// The keywords of `deps approve`.
const (
	keyPackage  = "package"
	keyImporter = "importer"
	keyReason   = "reason"
)

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "approve",
		Summary: "approve one third-party package for one importer",
		Why: "record Thomas's approval of one third-party Go package for one Ze directory: the row joins " +
			LedgerRel + " and the untracked record " + ApprovalsRel + " that `./le commit create` reads. " +
			"A module is approved only for its approved use, so each new importer is a new approval. " +
			"Owner only: refused unless stdin and stdout are interactive terminals, and the refusal " +
			"prints the command for Thomas to run. Approving the same package and importer again replaces the reason",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyPackage, Kind: leaction.KindString, Help: "the third-party package import path", Value: "<import path>", Requirement: leaction.Required},
			{Keyword: keyImporter, Kind: leaction.KindString, Help: "the Ze directory allowed to import it, from the repo root", Value: "<directory>", Requirement: leaction.Required},
			{Keyword: keyReason, Kind: leaction.KindString, Help: "why Ze needs it there, one line", Value: keyReason, Requirement: leaction.Required},
		},
		PrepareArgs: approveAnswer},
	leaction.Action{
		Verb:    "check",
		Summary: "compare every third-party import with the ledger",
		Why: "parse the imports of every Go file of the Ze module and refuse any third-party import " +
			"the ledger does not approve for its directory; rows no file uses any more are listed as " +
			"stale without failing, because removing a use needs nobody's approval",
		Answer: checkAnswer},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le deps` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// approveAnswer admits the approval before anything is read or written: the
// terminal gate first, then the row's shape.
func approveAnswer(args leaction.Arguments) (leaction.Prepared, int) {
	if !args.Has(keyPackage) || !args.Has(keyImporter) || !args.Has(keyReason) {
		leaction.ReportError(errors.New("deps approve requires package <import path> importer <directory> reason \"<why>\""))
		return leaction.Prepared{}, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return leaction.Prepared{}, 2
	}
	// The reason is trimmed as Parse trims it, so the record and the ledger
	// spell the row the same way and the commit gate matches them.
	row := Row{Package: args.One(keyPackage), Importer: args.One(keyImporter), Reason: strings.TrimSpace(args.One(keyReason))}
	if err := admit(root, row); err != nil {
		leaction.ReportError(err)
		return leaction.Prepared{}, 2
	}
	return leaction.PreparedAnswer(func() (any, int) {
		approval, err := Approve(root, row)
		if err != nil {
			leaction.ReportError(err)
			return nil, 3
		}
		return approval, 0
	}), 0
}

// checkAnswer runs Check over this checkout: exit 1 when an import is not
// approved, 2 when the tree or the ledger could not be read.
func checkAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Check(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if len(report.Unapproved) != 0 {
		return report, 1
	}
	return report, 0
}
