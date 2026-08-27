// Design: ai/rules/git-safety.md -- what may rewrite a checkout, and what may not
// Overview: worktree.go -- the update this table reaches
// Related: report.go -- what the verb answers
//
// actions.go defines the action table.
// internal/le/leaction supplies the shared dispatch, listing, help line, and two refusals.
//
// This area has one verb with a keyword before its value.
// `le worktree update` updates the current checkout.
// `le worktree update path <path>` updates a named checkout.
// `le worktree update all` updates every linked worktree in this repository.
// The shell command instead used either a bare path or --all, which conflicts with the CLI grammar (ai/rules/cli.md).

package worktree

import (
	"errors"
	"os"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
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
		Answer: updateHere,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le worktree` command.
//
// The update's own keywords are read here rather than by leaction, which
// dispatches on the verb alone. Reaching them means the verb resolved, so a
// refusal below is about the keyword and says so.
func Answer(args []string) (any, int) {
	if len(args) <= 1 {
		return actions.Answer(args)
	}
	if args[0] != actions.Actions().Actions[0].Verb {
		return actions.Answer(args)
	}
	return update(args[1:])
}

// update reads the keywords and runs the right shape of update.
//
// The command reads keywords before it resolves the repository.
// An unsupported word is a usage error that needs no git command or checkout.
// This order also prevents a mistyped keyword from touching an unintended tree.
func update(args []string) (any, int) {
	all := len(args) == 1 && args[0] == allKeyword
	named := len(args) == 2 && args[0] == pathKeyword
	if !all && !named {
		leaction.ReportError(errUsage())
		return nil, 2
	}

	updater, err := here()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	if all {
		return answer(updater.All())
	}
	result, err := updater.One(args[1])
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
