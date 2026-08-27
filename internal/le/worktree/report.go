// Design: ai/rules/git-safety.md -- what may rewrite a checkout, and what may not
// Overview: worktree.go -- the update this report describes
//
// report.go holds what an update ANSWERS, apart from what produced it.
//
// The report has one row for each worktree.
// Each row gives the tree, branch, stash state, and resulting HEAD.

package worktree

import "github.com/ze-software/ze/internal/core/textbuf"

// Result is one worktree the update touched.
//
// Stashed is explicit because developers need it after an incomplete run.
// A stash that was not restored remains as work in `git stash list`.
type Result struct {
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Head    string `json:"head"`
	Stashed bool   `json:"stashed"`
}

// Report is the whole answer of one run.
type Report struct {
	Worktrees []Result `json:"worktrees"`
}

// Text renders one line for each worktree and ends with a newline.
// A run that touched nothing returns an explicit line.
// Silence after a history-rewriting command CAN otherwise look like a command that never ran.
func (r Report) Text() string {
	var tb textbuf.Buffer
	if len(r.Worktrees) == 0 {
		return tb.Str("no linked worktree to update\n").String()
	}
	for _, result := range r.Worktrees {
		tb.Str(result.Path).Str(" (branch ").Str(result.Branch).Str(") is now at ").Str(result.Head)
		if result.Stashed {
			tb.Str(", and its uncommitted work was stashed and restored")
		}
		tb.Byte('\n')
	}
	return tb.String()
}
