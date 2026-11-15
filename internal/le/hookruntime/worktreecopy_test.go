// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: worktreecopy.go -- worktreeCopyLine, the judgment these cases drive
// Related: bash.go -- bashWorktreeCopy, the guard they ask

package hookruntime

import "testing"

// TestWorktreeCopyRefusesACopyOutOfAWorktree asks bashWorktreeCopy alone
// about lines that copy a file from under .claude/worktrees to a destination
// outside it, each through another route the shell offers.
//
// VALIDATES: a copy verb run as a command (directly, behind a launcher, after a
// `cd` into the worktree, through `-t`, `find -exec`, `xargs`, or in a line
// handed to `bash -c` or `ssh`), a redirection of what a command read from a
// worktree (directly, through a pipe, through a command substitution, or out
// of a subshell that ran inside the worktree, to a file outside it), and a
// line the parser cannot read are refused.
// PREVENTS: the move from a text judgment to a command judgment clearing the
// copies the guard exists to stop.
func TestWorktreeCopyRefusesACopyOutOfAWorktree(t *testing.T) {
	root := t.TempDir()
	for _, command := range []string{
		"cp .claude/worktrees/x/foo.go internal/foo.go",
		"rsync -a .claude/worktrees/x/ ./",
		"cat .claude/worktrees/x/a.go > internal/a.go",
		"mv .claude/worktrees/x/a.go /home/thomas/Code/ze/main/internal/a.go",
		"install -m 644 .claude/worktrees/x/a.go internal/a.go",
		"sudo cp .claude/worktrees/x/a.go internal/a.go",
		"cp .claude/worktrees/x/*.go internal/",
		"cp -t internal .claude/worktrees/x/a.go",
		"cp -r --target-directory=internal .claude/worktrees/x/dir",
		"cd .claude/worktrees/x && cp foo.go ../../../internal/foo.go",
		"cat .claude/worktrees/x/a.go | sed s/a/b/ > internal/a.go",
		// The redirection is outside the subshell, so log.txt lands in the main tree.
		"(cd .claude/worktrees/x && cat a.go) > log.txt 2>&1",
		"cat < .claude/worktrees/x/a.go > internal/a.go",
		`echo "$(cat .claude/worktrees/x/a.go)" > internal/a.go`,
		"bash -c 'cp .claude/worktrees/x/a.go internal/a.go'",
		"eval cp .claude/worktrees/x/a.go internal/a.go",
		"ssh claude 'cd Code/ze/main && cp .claude/worktrees/x/a.go internal/a.go'",
		`find .claude/worktrees/x -name '*.go' -exec cp {} internal/ \;`,
		"ls .claude/worktrees/x/*.go | xargs cp -t internal",
		"cp .claude/worktrees/x/a.go internal/a.go (",
	} {
		if result := guardVerdict(bashWorktreeCopy, root, command); result == nil || result.code != 2 {
			t.Errorf("%q: verdict %+v, want exit 2: it copies a file out of a worktree", command, result)
		}
	}
}

// TestWorktreeCopyPassesWorkThatCopiesNothingOutOfAWorktree asks
// bashWorktreeCopy alone about lines that name .claude/worktrees and copy
// nothing out of one: the cross-machine worktree work ai/owner/machines.md
// requires, a redirection that writes inside the worktree or writes nothing a
// worktree path fed, a copy into a worktree, and prose.
//
// VALIDATES: `scp` is not read as `cp`; a here-document's body, a quoted
// argument and a search pattern are data; `git worktree add` and `remove`, and
// a test run inside the worktree whose redirection writes its log inside the
// worktree too, pass; a redirection to a device passes. A subshell that ran in
// the worktree and is redirected outside it is no pass case: its file lands in
// the main tree (TestWorktreeCopyRefusesACopyOutOfAWorktree).
// PREVENTS: the false refusals of the text judgment, which refused each of
// these for a substring anywhere in the line.
func TestWorktreeCopyPassesWorkThatCopiesNothingOutOfAWorktree(t *testing.T) {
	root := t.TempDir()
	for _, command := range []string{
		"git bundle create ~/tmp/x.bundle wt-x && scp ~/tmp/x.bundle claude:tmp/x.bundle && rm ~/tmp/x.bundle && git worktree add --detach .claude/worktrees/x FETCH_HEAD",
		"python3 - <<'EOF'\nopen('docs/guide/x.md', 'w').write('''Open .claude/worktrees/name, then scp ~/tmp/x.bundle claude:tmp/x.bundle.\ncp the log > there, mv it, rsync it, install it.\n''')\nEOF",
		"git worktree add --detach .claude/worktrees/x FETCH_HEAD && cd .claude/worktrees/x && ./le test unit internal/le/hookruntime",
		"git worktree remove .claude/worktrees/x",
		"cd .claude/worktrees/x && ./le test unit internal/le/hookruntime > log.txt",
		"git worktree add .claude/worktrees/x FETCH_HEAD > /dev/null 2>&1",
		"cp internal/a.go .claude/worktrees/x/internal/a.go",
		"cd .claude/worktrees/x && cp internal/a.go internal/b.go",
		`grep -rn "cp \|mv " .claude/worktrees/x/docs`,
		`echo "cp .claude/worktrees/x/a.go internal/a.go > b"`,
		"ssh claude 'export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH GOPATH=$HOME/go &&\n  cd ~/Code/github.com/ze-software/ze/main &&\n  git fetch ~/tmp/x.bundle wt-x &&\n  git worktree add --detach .claude/worktrees/x FETCH_HEAD &&\n  cd .claude/worktrees/x && ./le test unit internal/le/hookruntime'",
		"ssh claude 'cd ~/Code/github.com/ze-software/ze/main &&\n  git worktree remove .claude/worktrees/x && rm ~/tmp/x.bundle'",
		"sh -c 'export PATH=/opt/homebrew/bin:$HOME/go/bin:$PATH &&\n   cd /Users/thomas/Code/github.com/ze-software/ze/main &&\n   git fetch claude:Code/github.com/ze-software/ze/main wt-x &&\n   git worktree add --detach .claude/worktrees/x FETCH_HEAD &&\n   (cd .claude/worktrees/x && ./le test unit internal/le/hookruntime);\n   status=$?; git worktree remove .claude/worktrees/x; exit $status'",
	} {
		if result := guardVerdict(bashWorktreeCopy, root, command); result != nil {
			t.Errorf("%q: verdict %+v, want none: it copies nothing out of a worktree", command, result)
		}
	}
}
