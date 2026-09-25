// Design: ai/rules/git-safety.md -- what may rewrite a checkout, and what may not
// Overview: worktree.go -- the update this table reaches
// Related: report.go -- what the verb answers
//
// actions.go defines the action table.
// internal/le/le/action supplies the shared dispatch, listing, help line, and every refusal.
//
// This area has one verb with a keyword before its value.
// `le worktree update` updates the current checkout.
// `le worktree update path <path>` updates a named checkout.
// `le worktree update all` updates every linked worktree in this repository.
// The shell command took either a bare path or --all. Both conflict with the
// CLI grammar (ai/rules/cli.md).
//
// Both keywords are DECLARED in the table below, so one parser reads the line.
// The verb published zero parameters until 2026-09-12 and read `path` and `all`
// itself. `le worktree update path -xh` therefore handed the option to git as a
// checkout path instead of refusing it.

package worktree

import (
	"errors"
	"os"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// errUsage is what a keyword this verb does not take is refused with.
func errUsage() error { return errors.New(usageLine) }

// workingDirectory answers the checkout the developer typed the command in,
// which is the tree the no-keyword form updates.
func workingDirectory() (string, error) { return os.Getwd() }

// The keywords the update takes. Each types the value that follows it, so a
// worktree path that happens to spell a keyword is still a path.
const (
	pathKeyword = "path"
	allKeyword  = "all"
)

// usageLine is what a refusal points at.
const usageLine = "usage: le worktree update [path <path> | all]"

var actions = leaction.New(area,
	leaction.Action{
		Verb: "update",
		Why: "rebase a linked worktree onto main, stashing and restoring its uncommitted work." +
			" Refuses the main working tree and a checkout with no branch",
		Writes: true,
		Parameters: []leaction.Parameter{
			// Optional: naming neither keyword updates the checkout the
			// command was typed in, which is the form the shell half had.
			{Keyword: pathKeyword, Value: "path", Requirement: leaction.Optional},
			{Keyword: allKeyword},
		},
		AnswerArgs: update,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le worktree` command. Every word of the line is read by the
// table above, which refuses an undeclared keyword and an option standing where
// a path goes.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// update runs the shape of update the keywords name.
//
// leaction has already read the line, so no git command and no checkout has
// been reached yet. That order is what stops a mistyped keyword from touching
// an unintended tree.
func update(args leaction.Arguments) (any, int) {
	all := args.Has(allKeyword)
	named := args.Has(pathKeyword)
	if all && named {
		// Each keyword names a different population, so a line carrying both
		// asks for two updates and states neither.
		leaction.ReportError(errUsage())
		return nil, 2
	}

	if !all && !named {
		return updateHere()
	}

	updater, err := here()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	if all {
		return answer(updater.All())
	}
	result, err := updater.One(args.One(pathKeyword))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return Report{Worktrees: []Result{result}}, 0
}

// answer carries a whole-run result out, reporting a failure once.
func answer(report Report, err error) (any, int) {
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	return report, 0
}

// updateHere updates the checkout this command was run in, which is the shell
// half's no-argument form.
func updateHere() (any, int) {
	updater, err := here()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	cwd, err := workingDirectory()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	result, err := updater.One(cwd)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return Report{Worktrees: []Result{result}}, 0
}

// here builds an updater for the repository this command was run against.
//
// lepath.Root() answers the MAIN working tree only when the command runs there;
// run from inside a worktree it answers that worktree. So the main tree is
// asked of git rather than assumed, which is what makes the main-tree refusal
// hold from either side.
func here() (Updater, error) {
	root, err := lepath.Root()
	if err != nil {
		return Updater{}, err
	}
	updater := Updater{Main: root}
	listed, err := updater.git(root, "worktree", "list", "--porcelain")
	if err != nil {
		return Updater{}, err
	}
	trees := parseWorktreeList(listed)
	if len(trees) > 0 {
		updater.Main = trees[0]
	}
	return updater, nil
}
