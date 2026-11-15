// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: worktreecopy.go -- worktreeCopyLine, the judgment these cases drive
// Related: bash.go -- bashWorktreeCopy, the guard they ask

package hookruntime

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWorktreeCopyRefusesACopyOutOfAWorktree asks bashWorktreeCopy alone
// about lines that copy a file from under .claude/worktrees to a destination
// outside it, each through another route the shell offers.
//
// VALIDATES: a copy verb run as a command (directly, behind a launcher, after a
// `cd` into the worktree, through `-t`, `find -exec`, `xargs`, or in a line
// handed to `bash -c` or `ssh`), a redirection of what a command read from a
// worktree (directly, through a pipe, through a command substitution, or out
// of a subshell that ran inside the worktree, to a file outside it), and a
// line the parser cannot read are refused. So are the routes the first
// command judgment missed: a handed line whose output an outer redirection
// writes, a `cd` inside a loop or a conditional, a script a shell reads from
// stdin, a launcher or shell the reading does not name (nohup, doas, zsh,
// `env -C`, a copy behind find's or xargs's shell), a `cd` that fails or runs
// in the background, a quote inside the worktree path, and tee, ln, dd,
// tar -x, git apply and patch. The checkout holds .claude/worktrees/x, so a
// `cd` into it is a directory the hook can name (cdTarget).
// PREVENTS: the move from a text judgment to a command judgment clearing the
// copies the guard exists to stop.
func TestWorktreeCopyRefusesACopyOutOfAWorktree(t *testing.T) {
	root := worktreeCheckout(t)
	for _, command := range []string{
		"cp .claude/worktrees/x/foo.go internal/foo.go",
		"rsync -a .claude/worktrees/x/ ./",
		"cat .claude/worktrees/x/a.go > internal/a.go",
		// An absolute destination inside the checkout is the main tree.
		"mv .claude/worktrees/x/a.go " + root + "/internal/a.go",
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
		// A line handed to a shell reads a worktree file, and the redirection
		// outside the handed line writes what it printed.
		"bash -c 'cat .claude/worktrees/x/a.go' > internal/a.go",
		"sh -c 'cd .claude/worktrees/x && cat a.go' > internal/a.go",
		// A `cd` inside a compound statement moves the commands after it in
		// the same body.
		"for d in .claude/worktrees/x; do cd $d; cp a.go ../../../internal/a.go; done",
		"if cd .claude/worktrees/x; then cp a.go ../../../internal/a.go; fi",
		// A shell reads its script from stdin: a here-document or a pipe.
		"bash <<'EOF'\ncp .claude/worktrees/x/a.go internal/a.go\nEOF",
		"echo 'cp .claude/worktrees/x/a.go internal/a.go' | sh",
		// A launcher, a shell, or a program run by find, xargs or rex runs the copy.
		"nohup cp .claude/worktrees/x/a.go internal/a.go",
		"stdbuf -o0 cp .claude/worktrees/x/a.go internal/a.go",
		"setsid cp .claude/worktrees/x/a.go internal/a.go",
		"doas cp .claude/worktrees/x/a.go internal/a.go",
		"zsh -c 'cp .claude/worktrees/x/a.go internal/a.go'",
		"dash -c 'cp .claude/worktrees/x/a.go internal/a.go'",
		`find .claude/worktrees/x -exec sudo cp {} internal/ \;`,
		`find .claude/worktrees/x -name '*.go' -exec sh -c 'cp "$1" internal/' _ {} \;`,
		`ls .claude/worktrees/x/*.go | xargs sh -c 'cp "$@" internal/' _`,
		"rex -S laptop run -s S -- sh -c 'cp .claude/worktrees/x/a.go internal/a.go'",
		// env -C runs cp inside the worktree, so ../../../internal is the main tree.
		"env -C .claude/worktrees/x cp a.go ../../../internal/a.go",
		// A `cd` that fails, or runs in the background, leaves the shell
		// inside the worktree, where the cp runs.
		"cd .claude/worktrees/x && cd /nonexistent; cp a.go ../../../internal/a.go",
		"cd .claude/worktrees/x && { cd /tmp & }; cp a.go ../../../internal/a.go",
		// Quotes inside the path hide .claude/worktrees from a text pre-filter.
		"cp '.claude/'worktrees/x/a.go internal/",
		// Other programs that write a worktree file into the main tree.
		"cat .claude/worktrees/x/a.go | tee internal/a.go",
		"ln -f .claude/worktrees/x/a.go internal/a.go",
		"dd if=.claude/worktrees/x/a.go of=internal/a.go",
		"tar -c -C .claude/worktrees/x . | tar -x -C internal",
		"git -C .claude/worktrees/x diff | git apply",
		"patch -p1 < .claude/worktrees/x/fix.diff",
		// Other spellings of the program, the source, the redirection and the `cd`.
		"/bin/cp .claude/worktrees/x/a.go internal/a.go",
		"command cp .claude/worktrees/x/a.go internal/a.go",
		`\cp .claude/worktrees/x/a.go internal/a.go`,
		`cp ".claude/worktrees/x/a.go" internal/a.go`,
		"cp <(cat .claude/worktrees/x/a.go) internal/a.go",
		"cat .claude/worktrees/x/a.go &> internal/a.go",
		"cat .claude/worktrees/x/a.go >& internal/a.go",
		`cd "$(ls -d .claude/worktrees/*)" && cp a.go ../../../internal/a.go`,
		"pushd .claude/worktrees/x && cp a.go ../../../internal/a.go",
		"ls .claude/worktrees/x/*.go | xargs -I{} cp {} internal/",
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
// worktree too, pass; a redirection to a device or a descriptor passes, and so
// does one to a file outside the checkout or into its gitignored tmp/, which
// holds no tracked work. A subshell that ran in the worktree and is redirected
// into the checkout is no pass case: its file lands in the main tree
// (TestWorktreeCopyRefusesACopyOutOfAWorktree).
// PREVENTS: the false refusals of the text judgment, which refused each of
// these for a substring anywhere in the line.
func TestWorktreeCopyPassesWorkThatCopiesNothingOutOfAWorktree(t *testing.T) {
	root := worktreeCheckout(t)
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
		// A redirection to a file outside the checkout copies nothing into it.
		"rex -S laptop run -s S --json --wait -- sh -c 'cd /Users/thomas/Code/ze/main/.claude/worktrees/x && ./le test unit pkg' > /home/thomas/.claude/jobs/x/tmp/out.json",
		"cat .claude/worktrees/x/a.go > /tmp/a.go",
		"cat .claude/worktrees/x/a.go >&2",
		"git -C .claude/worktrees/x log > /tmp/l",
		// The checkout's gitignored tmp/ holds no tracked work.
		"git worktree add .claude/worktrees/x FETCH_HEAD > tmp/session/s/wt.log",
		// A program that writes what it reads, writing outside the main tree.
		"cat .claude/worktrees/x/a.go | tee /tmp/a.go",
		"tar -c -C .claude/worktrees/x . | tar -x -C tmp/session/s",
	} {
		if result := guardVerdict(bashWorktreeCopy, root, command); result != nil {
			t.Errorf("%q: verdict %+v, want none: it copies nothing out of a worktree", command, result)
		}
	}
}

// TestWorktreeCopyJudgesALineFromTheSessionCwd asks bashWorktreeCopy alone
// about lines a session runs from inside a worktree, where no word names
// .claude/worktrees.
//
// VALIDATES: a line that starts in the session's cwd under
// .claude/worktrees is read, so a copy from there to an absolute path in the
// main tree is refused, and a run there whose log stays there passes.
// PREVENTS: a guard that reads only lines whose text names a worktree,
// which a session inside one never types.
func TestWorktreeCopyJudgesALineFromTheSessionCwd(t *testing.T) {
	root := worktreeCheckout(t)
	cwd := filepath.Join(root, ".claude", "worktrees", "x")
	refused := "cp internal/a.go " + root + "/internal/a.go"
	if result := guardVerdictIn(bashWorktreeCopy, root, cwd, refused); result == nil || result.code != 2 {
		t.Errorf("%q from %s: verdict %+v, want exit 2: it copies a worktree file into the main tree", refused, cwd, result)
	}
	passed := "./le test unit pkg > log.txt"
	if result := guardVerdictIn(bashWorktreeCopy, root, cwd, passed); result != nil {
		t.Errorf("%q from %s: verdict %+v, want none: its log stays in the worktree", passed, cwd, result)
	}
}

// worktreeCheckout answers a checkout root holding the worktree
// .claude/worktrees/x, so a `cd` into it reaches a directory, and clears
// CDPATH, which makes every `cd` one the hook cannot follow (cdTarget).
func worktreeCheckout(t *testing.T) string {
	t.Helper()
	t.Setenv("CDPATH", "")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "worktrees", "x"), 0o750); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestWorktreeCopyJudgesASessionStartedInAWorktree asks bashWorktreeCopy
// alone about lines of a session whose project directory is the worktree
// itself, so the hook's root is the worktree and the main checkout holds it.
//
// VALIDATES: the main checkout is the one that holds .claude/worktrees, so a
// copy from the worktree into it, by an absolute path or by `../../../`, is
// refused.
// PREVENTS: the worktree read as the main tree and the real main tree read as
// a place outside the checkout, which cleared every such copy.
func TestWorktreeCopyJudgesASessionStartedInAWorktree(t *testing.T) {
	root := worktreeCheckout(t)
	worktree := filepath.Join(root, ".claude", "worktrees", "x")
	for _, command := range []string{
		"cp internal/a.go " + root + "/internal/a.go",
		"cat internal/a.go > ../../../internal/a.go",
	} {
		if result := guardVerdictIn(bashWorktreeCopy, worktree, worktree, command); result == nil || result.code != 2 {
			t.Errorf("%q in a session rooted at %s: verdict %+v, want exit 2: it copies into the main tree", command, worktree, result)
		}
	}
}

// TestWorktreeCopyCountsADestinationItCannotPlace asks bashWorktreeCopy alone
// about copies from a worktree to a destination that starts with an
// expansion, which may land anywhere.
//
// VALIDATES: a destination starting with a `$X` the hook cannot read counts
// as the main tree unless its own components name a worktree. A `~` head is
// the home directory, which TestWorktreeCopyPlacesAHomePath judges.
// PREVENTS: such a word resolved against the worktree cwd and read as a path
// inside the worktree.
func TestWorktreeCopyCountsADestinationItCannotPlace(t *testing.T) {
	root := worktreeCheckout(t)
	worktree := filepath.Join(root, ".claude", "worktrees", "x")
	for _, command := range []string{
		`cp internal/a.go "$CLAUDE_PROJECT_DIR/internal/a.go"`,
		`cat internal/a.go > "$OLDPWD/internal/a.go"`,
		"cp internal/a.go $X/main/internal/a.go",
	} {
		if result := guardVerdictIn(bashWorktreeCopy, root, worktree, command); result == nil || result.code != 2 {
			t.Errorf("%q from %s: verdict %+v, want exit 2: its destination may be the main tree", command, worktree, result)
		}
	}
}

// TestWorktreeCopyPlacesAnExpandedSourceByItsComponents asks bashWorktreeCopy
// alone about reads of a worktree file through a path that starts with an
// expansion.
//
// VALIDATES: an expanded word whose own components name .claude/worktrees is
// a worktree source, wherever the expansion lands.
// PREVENTS: the expanded word resolved against the checkout root and read as
// a place the hook cannot name.
func TestWorktreeCopyPlacesAnExpandedSourceByItsComponents(t *testing.T) {
	root := worktreeCheckout(t)
	for _, command := range []string{
		`cat "$PWD/.claude/worktrees/x/a.go" > internal/a.go`,
		"cat ~/Code/z/main/.claude/worktrees/x/a.go > internal/a.go",
		`cat "$CLAUDE_PROJECT_DIR/.claude/worktrees/x/a.go" | tee internal/a.go`,
	} {
		if result := guardVerdict(bashWorktreeCopy, root, command); result == nil || result.code != 2 {
			t.Errorf("%q: verdict %+v, want exit 2: it copies a worktree file into the main tree", command, result)
		}
	}
}

// TestWorktreeCopyRefusesOtherProgramsThatWriteWhatTheyRead asks
// bashWorktreeCopy alone about `git am` and `sponge` fed from a worktree.
//
// VALIDATES: git am applies its input to the checkout it runs in, and sponge
// writes its input to its operand.
// PREVENTS: a worktree's patch or file landing in the main tree through a
// writer the guard does not name.
func TestWorktreeCopyRefusesOtherProgramsThatWriteWhatTheyRead(t *testing.T) {
	root := worktreeCheckout(t)
	for _, command := range []string{
		"git -C .claude/worktrees/x format-patch -1 --stdout | git am",
		"cat .claude/worktrees/x/a.go | sponge internal/a.go",
	} {
		if result := guardVerdict(bashWorktreeCopy, root, command); result == nil || result.code != 2 {
			t.Errorf("%q: verdict %+v, want exit 2: it writes a worktree's content into the main tree", command, result)
		}
	}
}

// TestWorktreeCopyJudgesADestinationThroughItsLinks asks bashWorktreeCopy
// alone about a copy into tmp/ through a link that leads into the main tree.
//
// VALIDATES: a destination is judged where its links lead, so tmp/l, a link
// to ../internal, is the main tree.
// PREVENTS: the tmp/ exemption read from the path as typed.
func TestWorktreeCopyJudgesADestinationThroughItsLinks(t *testing.T) {
	root := worktreeCheckout(t)
	for _, directory := range []string{"internal", "tmp"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../internal", filepath.Join(root, "tmp", "l")); err != nil {
		t.Fatal(err)
	}
	const command = "cp .claude/worktrees/x/a.go tmp/l/a.go"
	if result := guardVerdict(bashWorktreeCopy, root, command); result == nil || result.code != 2 {
		t.Errorf("%q: verdict %+v, want exit 2: tmp/l leads into internal/", command, result)
	}
}

// worktreeSession answers the checkout worktreeCheckout makes and its
// worktree x: the root and cwd of a session started inside that worktree.
func worktreeSession(t *testing.T) (root, worktree string) {
	t.Helper()
	root = worktreeCheckout(t)
	return root, filepath.Join(root, ".claude", "worktrees", "x")
}

// expectWorktreeCopyRefused asks bashWorktreeCopy alone about each command,
// run from cwd in a session rooted at root, and wants every one refused.
func expectWorktreeCopyRefused(t *testing.T, root, cwd, why string, commands ...string) {
	t.Helper()
	for _, command := range commands {
		if result := guardVerdictIn(bashWorktreeCopy, root, cwd, command); result == nil || result.code != 2 {
			t.Errorf("%q from %s: verdict %+v, want exit 2: %s", command, cwd, result, why)
		}
	}
}

// expectWorktreeCopyPassed asks bashWorktreeCopy alone about each command,
// run from cwd in a session rooted at root, and wants every one passed.
func expectWorktreeCopyPassed(t *testing.T, root, cwd, why string, commands ...string) {
	t.Helper()
	for _, command := range commands {
		if result := guardVerdictIn(bashWorktreeCopy, root, cwd, command); result != nil {
			t.Errorf("%q from %s: verdict %+v, want none: %s", command, cwd, result, why)
		}
	}
}

// TestWorktreeCopyJudgesAHandedLineFromItsWords asks bashWorktreeCopy alone
// about lines handed to a shell in a word the outer shell expands first.
//
// VALIDATES: such a line is parsed from the word's value, each outer
// expansion standing for a word the hook cannot read and an outer `$PWD` for
// the directory the outer shell runs in. In a session started inside the
// worktree, a test run whose log stays there passes, and so does a gofmt find
// runs with its output on /dev/null. A `cd` to a directory the hook cannot
// name, and a destination built from the file name xargs hands the shell,
// cannot be shown to stay in the worktree, so those stay refused, as does an
// outer expansion inside the inner quotes. A line whose program an outer
// expansion names is judged by its text. An outer `$PWD` after an inner `cd`
// still names the outer directory.
// PREVENTS: the text judgment refusing every such line for the `>` in it, and
// the parsed judgment reading an outer `$PWD` as the inner shell's.
func TestWorktreeCopyJudgesAHandedLineFromItsWords(t *testing.T) {
	root, worktree := worktreeSession(t)
	expectWorktreeCopyPassed(t, worktree, worktree, "its output stays in the worktree or goes to a device",
		`bash -c "go test ./... > $PWD/out.log"`,
		`find . -name '*.go' -exec sh -c 'gofmt -l {} > /dev/null' \;`,
	)
	expectWorktreeCopyRefused(t, worktree, worktree, "its destination cannot be shown to stay in the worktree",
		// The cd goes where $d points, which the hook cannot name, so log.txt
		// may land in the main tree.
		`sh -c "cd $d && go test > log.txt"`,
		// The destination is a file name xargs reads, plus .bak: the hook
		// cannot place it, so it counts as the main tree.
		`ls | xargs -I{} sh -c 'cat {} > {}.bak'`,
		// The outer expansion lands inside the inner quotes, and is still a
		// word the hook cannot place.
		`bash -c "cat a.go > '$X'"`,
		// An outer expansion names the program, so the line is judged by
		// its text.
		`bash -c "$CMD a.go > out.log"`,
	)
	// The outer shell expands $PWD before the inner cd runs, so the copy
	// lands in the main checkout's internal/.
	expectWorktreeCopyRefused(t, root, root, "its $PWD is the main checkout",
		`bash -c "cd .claude/worktrees/x && cp a.go $PWD/internal/a.go"`,
	)
}

// TestWorktreeCopyReadsALineThatNamesTheClaudeDirectory asks bashWorktreeCopy
// alone about copies out of a worktree whose words never spell
// .claude/worktrees in one piece.
//
// VALIDATES: a line is read when a word has a `.claude` or a `worktrees`
// component, or a glob component that matches .claude, or when it starts
// under the checkout's .claude/; each of these copies is then refused. A
// bare `*` never matches .claude (bash needs a literal leading dot), so a
// copy between two main-tree directories through one stays unread.
// PREVENTS: a copy that splits the path across a `cd`, a glob or a variable
// passing unread, and every glob copy in the main tree being read and
// refused for a source the hook cannot place.
func TestWorktreeCopyReadsALineThatNamesTheClaudeDirectory(t *testing.T) {
	root, _ := worktreeSession(t)
	expectWorktreeCopyPassed(t, root, root, "no word of it may name .claude",
		"cp internal/x/* internal/y/",
		"cp .[a-b]* internal/y/",
	)
	expectWorktreeCopyRefused(t, root, root, "it copies a worktree file into the main tree",
		"cd .claude && cp worktrees/x/a.go ../internal/a.go",
		"cp .claude/work*/x/a.go internal/a.go",
		"cp .claude/worktree[s]/x/a.go internal/",
		"W=.claude/work; cp ${W}trees/x/a internal/a",
	)
	expectWorktreeCopyRefused(t, root, filepath.Join(root, ".claude"), "it starts under .claude/ and copies a worktree file into the main tree",
		"cp worktrees/x/a.go ../internal/a.go",
	)
}

// TestWorktreeCopyPlacesAVariableBoundToAWorktreePath asks bashWorktreeCopy
// alone about copies whose source is a variable the line bound to a worktree
// path.
//
// VALIDATES: a name assigned (bare, by export, or as a for loop's variable)
// from a word under a worktree places a later `$name` there, in the line
// itself and in a line it hands a shell.
// PREVENTS: a variable laundering the worktree path past the guard.
func TestWorktreeCopyPlacesAVariableBoundToAWorktreePath(t *testing.T) {
	root, _ := worktreeSession(t)
	expectWorktreeCopyRefused(t, root, root, "the variable names a worktree file, which the line copies into the main tree",
		`f=.claude/worktrees/x/a.go; cat "$f" > internal/a.go`,
		`for f in .claude/worktrees/x/*.go; do cat "$f" > "internal/$(basename "$f")"; done`,
		`export f=.claude/worktrees/x/a.go; cat "$f" > internal/a.go`,
		`f=.claude/worktrees/x/a.go; bash -c "cat $f > internal/a.go"`,
	)
}

// TestWorktreeCopyPassesTheWorkOfASessionInsideAWorktree asks
// bashWorktreeCopy alone about the everyday lines of a session started inside
// a worktree, whose root and cwd are the worktree.
//
// VALIDATES: a test run with its log in the worktree's tmp/, a search written
// outside the checkout, a commit, a here-document written into the worktree,
// a job run, git status and a second worktree added inside this one pass.
// PREVENTS: the guard refusing the work it exists to keep inside the worktree.
func TestWorktreeCopyPassesTheWorkOfASessionInsideAWorktree(t *testing.T) {
	_, worktree := worktreeSession(t)
	expectWorktreeCopyPassed(t, worktree, worktree, "it writes nothing into the main tree",
		"go test ./... > tmp/x.log",
		"grep -rn foo internal > /tmp/h",
		`./le commit create subject "s" file internal/x.go`,
		"cat > internal/x.go <<'EOF'\nx\nEOF",
		"./le job run label le-test quiet command ./le test unit pkg path ./internal/le/hookruntime/",
		"git status",
		"git worktree add --detach .claude/worktrees/y FETCH_HEAD",
	)
}

// TestWorktreeCopyFollowsACdItCannotNameInACompound asks bashWorktreeCopy
// alone about a loop body that moves into the worktree through a quoted or
// escaped `cd`, or a program word the shell expands.
//
// VALIDATES: each of these may change the directory, so the body is judged
// from one the hook cannot name, and the relative destination counts as the
// main tree.
// PREVENTS: a spelling of cd the walk does not read as cd leaving the body in
// a directory that is known and wrong.
func TestWorktreeCopyFollowsACdItCannotNameInACompound(t *testing.T) {
	root, _ := worktreeSession(t)
	expectWorktreeCopyRefused(t, root, root, "the body may run in the worktree, and its destination may be the main tree",
		`for x in 1; do "cd" .claude/worktrees/x; cp a.go ../../../internal/a.go; done`,
		`for x in 1; do \cd .claude/worktrees/x; cp a.go ../../../internal/a.go; done`,
		`for x in 1; do $CD .claude/worktrees/x; cp a.go ../../../internal/a.go; done`,
	)
}

// TestWorktreeCopyJudgesAnRsyncHostPath asks bashWorktreeCopy alone about
// rsync's `host:path` operands.
//
// VALIDATES: a `host:path` destination is another host, never the main tree,
// and a `host:path` source under a worktree is a worktree file; cp, which has
// no remote form, writes `internal:a.go` into the main tree.
// PREVENTS: a copy of a worktree to another machine read as a copy into the
// main tree, and a remote worktree file pulled into it read as no worktree file.
func TestWorktreeCopyJudgesAnRsyncHostPath(t *testing.T) {
	root, _ := worktreeSession(t)
	expectWorktreeCopyPassed(t, root, root, "it writes to another host",
		"rsync -a .claude/worktrees/x/ claude:tmp/x/",
	)
	expectWorktreeCopyRefused(t, root, root, "it copies a worktree file into the main tree",
		"rsync -a claude:Code/ze/main/.claude/worktrees/x/a.go internal/a.go",
		"cp .claude/worktrees/x/a.go internal:a.go",
	)
}

// TestWorktreeCopyJudgesTheCommandAnLeCarrierRuns asks bashWorktreeCopy
// alone about `./le job run` lines, whose `command` le runs on this host,
// from inside the worktree with the session rooted at the main checkout and
// at the worktree, and from the main checkout. `le job run` runs its argv at
// the checkout root, and the guard judges it from the directory the line
// runs in, which is that root when the line runs from one, as agents run
// ./le. HOME is set so that `~/Code/github.com/ze-software/ze/main` is the
// main checkout.
//
// VALIDATES: the words after the argv keyword of an le carrier's row
// (leroot.CarriedArgv) are judged as a command of this host, from the
// line's directory: a cp among them that copies a worktree file into the
// main tree is refused, and a shell among them is handed a line judged on
// this host, where `~` is the home directory and an absolute path is a path
// of this checkout, so its copy into the main tree is refused too. A carried
// test run whose log lands in the worktree's tmp/, and a carried le command,
// pass.
// PREVENTS: the carried shell judged as a line another host runs, which
// passed its home and absolute destinations as outside the checkout, and a
// carried cp read as an operand of le, which copies nothing.
func TestWorktreeCopyJudgesTheCommandAnLeCarrierRuns(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CDPATH", "")
	main := filepath.Join(home, "Code", "github.com", "ze-software", "ze", "main")
	worktree := filepath.Join(main, ".claude", "worktrees", "x")
	if err := os.MkdirAll(worktree, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{main, worktree} {
		expectWorktreeCopyRefused(t, root, worktree, "the carried command copies a worktree file into the main tree, run on this host",
			"./le job run label t quiet command sh -c 'cp internal/x/a.go ~/Code/github.com/ze-software/ze/main/internal/x/a.go'",
			"./le job run label t quiet command sh -c 'cat internal/a.go > "+main+"/internal/a.go'",
		)
		expectWorktreeCopyPassed(t, root, worktree, "the carried command writes in the worktree, or copies nothing",
			"./le job run label t quiet command sh -c 'go test ./... > tmp/x.log'",
			"./le job run label le-test quiet command ./le test unit pkg path ./internal/le/hookruntime/",
		)
	}
	expectWorktreeCopyRefused(t, main, main, "the carried cp copies a worktree file into the main tree",
		"./le job run label t quiet command cp .claude/worktrees/x/a.go internal/a.go",
	)
}
