// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: worktreecopy.go -- worktreeCopyJudge, the walk whose assignments these bind
// Related: worktreehanded.go -- wordOf and boundValue, which read a bound `$name`
// Related: worktreeplace.go -- placeOf, where a bound word points
package hookruntime

import (
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// worktreeAssignments is what the walk learns of a line's assignments before
// it judges its commands: how many times the line assigns each name, a for
// loop's variable included, and the bare assignments, outside any loop or
// function body, of a name the line assigns once, whose value a later
// `$name` reads as (bind).
type worktreeAssignments struct {
	counts map[string]int
	sole   map[*syntax.Assign]bool
}

// worktreeLineAssignments answers the assignments of a parsed line
// (worktreeAssignments). A body that runs in an order the walk does not
// follow (a for or while loop, a function) marks every assignment inside it;
// the marking walks each outermost such body once more, so the cost is twice
// the line's length at most.
func worktreeLineAssignments(file *syntax.File) worktreeAssignments {
	assignments := worktreeAssignments{counts: map[string]int{}, sole: map[*syntax.Assign]bool{}}
	var bare []*syntax.Assign
	syntax.Walk(file, func(node syntax.Node) bool {
		switch node := node.(type) {
		case *syntax.Assign:
			if node.Name != nil {
				assignments.counts[node.Name.Value]++
			}
		case *syntax.WordIter:
			if node.Name != nil {
				assignments.counts[node.Name.Value]++
			}
		case *syntax.CallExpr:
			if len(node.Args) == 0 {
				bare = append(bare, node.Assigns...)
			}
		}
		return true
	})
	body := map[*syntax.Assign]bool{}
	syntax.Walk(file, func(node syntax.Node) bool {
		switch node.(type) {
		case *syntax.ForClause, *syntax.WhileClause, *syntax.FuncDecl:
			syntax.Walk(node, func(inner syntax.Node) bool {
				if assign, ok := inner.(*syntax.Assign); ok {
					body[assign] = true
				}
				return true
			})
			return false
		}
		return true
	})
	for _, assign := range bare {
		if assign.Name == nil {
			continue
		}
		if body[assign] {
			continue
		}
		if assignments.counts[assign.Name.Value] != 1 {
			continue
		}
		assignments.sole[assign] = true
	}
	return assignments
}

// bind records a variable the line assigns from word, run from directory: a
// name bound to a word under a worktree places a later `$name` there
// (namesInside), from directory when the word is relative (boundFrom), and
// one bound to any other word too is mixed; a sole
// assignment (worktreeLineAssignments) of a value read exactly makes a later
// `$name` that value (worktreeBindable); an assignment to PWD or HOME leaves
// `$PWD`, or `~` and `$HOME`, a word the hook cannot place.
func (judge *worktreeCopyJudge) bind(name string, word worktreeWord, directory worktreeDirectory, sole bool) {
	if name == "PWD" {
		judge.pwdAssigned = true
	}
	if name == "HOME" {
		judge.homeAssigned = true
	}
	if sole {
		if worktreeBindable(name, word) {
			judge.values[name] = word.token
		}
	}
	if judge.placeOf(word, directory) != worktreePlaceInside {
		judge.mixed[name] = true
		return
	}
	judge.inside[name] = true
	if filepath.IsAbs(word.token) {
		return
	}
	// A relative word names its path from directory alone, so a `$name` it
	// places is placed only where a write runs from directory (placedUnder).
	judge.boundFrom[name] = directory
}

// worktreeBindable reports whether a variable's value can stand for a later
// `$name`: the hook read it exactly, it holds only worktreeWorkingCharacters,
// so it reads back as the same word in any quoting, and the shell does not
// set the name itself, after a cd, a command or a read, so the value the
// line assigned does not last.
func worktreeBindable(name string, word worktreeWord) bool {
	if !word.exact {
		return false
	}
	if word.token == "" {
		return false
	}
	if strings.IndexFunc(word.token, worktreeUnsafeCharacter) >= 0 {
		return false
	}
	return !oneOf(name, "PWD", "OLDPWD", "_", "REPLY", "OPTARG", "OPTIND", "RANDOM", "SECONDS", "LINENO")
}

// bindAssigns binds each assignment of a command or a declaration (`f=x cmd`,
// `export f=x`) that has a value. bare reports a statement of assignments
// alone, whose values last after it; an assignment in front of a command
// sets the name for that command only.
func (judge *worktreeCopyJudge) bindAssigns(assigns []*syntax.Assign, directory worktreeDirectory, bare bool) {
	for _, assign := range assigns {
		if assign.Name == nil {
			continue
		}
		if assign.Value == nil {
			continue
		}
		sole := bare && judge.assignments.sole[assign] && !assign.Append && assign.Index == nil
		judge.bind(assign.Name.Value, judge.wordOf(assign.Value, directory), directory, sole)
	}
}

// bindLoop binds a for loop's variable from each word it runs over
// (`for f in .claude/worktrees/x/*.go`).
func (judge *worktreeCopyJudge) bindLoop(loop *syntax.ForClause, directory worktreeDirectory) {
	iteration, isWords := loop.Loop.(*syntax.WordIter)
	if !isWords {
		return
	}
	if iteration.Name == nil {
		return
	}
	for _, item := range iteration.Items {
		judge.bind(iteration.Name.Value, judge.wordOf(item, directory), directory, false)
	}
}
