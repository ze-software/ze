// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: worktreeplace.go -- placeOf and destinationOf, the placement these cases drive
// Related: worktreehanded.go -- wordOf, where `~`, `$HOME` and a bound `$name` are read
// Related: worktreecopy_test.go -- worktreeSession and the expect helpers these use

package hookruntime

import (
	"strings"
	"testing"
)

// worktreeHome points HOME at a fresh directory outside every checkout a
// test makes, so `~` and `$HOME` name a place the guard can test.
func worktreeHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// TestWorktreeCopyPlacesAHomePath asks bashWorktreeCopy alone about lines a
// session runs from inside a worktree that write under the home directory,
// once with the session rooted at the main checkout and once at the worktree.
//
// VALIDATES: a leading `~`, `~/`, `$HOME` or `${HOME}` is the home directory
// the hook reads, outside the checkout, so a test log teed or redirected to
// ~/tmp, a copy between two home paths, and a remote run whose output lands
// in ~/tmp pass, an expanded name after the home head included. A line that
// assigns HOME leaves `~` a place the hook cannot name, so its copy stays
// refused.
// PREVENTS: `~/tmp/a` read as a path under the worktree cwd, and `~/...` read
// as unplaced, which refused every log an agent keeps under ~/tmp.
func TestWorktreeCopyPlacesAHomePath(t *testing.T) {
	main, worktree := worktreeSession(t)
	worktreeHome(t)
	for _, root := range []string{main, worktree} {
		expectWorktreeCopyPassed(t, root, worktree, "it writes under the home directory, outside the checkout",
			"go test ./... 2>&1 | tee ~/tmp/x.log",
			"git log > ~/tmp/l",
			"cp ~/tmp/a ~/tmp/b",
			"rex -S laptop run -s S -- sh -c 'cd /Users/t/main/.claude/worktrees/x && ./le test unit p' > ~/tmp/out.json",
			`go test ./... > "$HOME/tmp/x.log"`,
			"go test ./... > ${HOME}/tmp/x.log",
			"go test ./... > ~/tmp/run-$n.log",
		)
		expectWorktreeCopyRefused(t, root, worktree, "the line assigns HOME, so ~ is a place the hook cannot name",
			"HOME=/elsewhere; cp a.go ~/a.go",
		)
	}
}

// TestWorktreeCopyRefusesAHomePathInTheMainTree asks bashWorktreeCopy alone
// about copies from a worktree to a home path, with HOME set to the main
// checkout.
//
// VALIDATES: `~` and `$HOME` are resolved to the home directory, and judged
// where they land: in the main checkout, outside its worktrees and tmp/, so
// the copy is refused.
// PREVENTS: a home head passed as outside the checkout without being
// resolved.
func TestWorktreeCopyRefusesAHomePathInTheMainTree(t *testing.T) {
	main, worktree := worktreeSession(t)
	t.Setenv("HOME", main)
	for _, root := range []string{main, worktree} {
		expectWorktreeCopyRefused(t, root, worktree, "the home directory is the main checkout",
			"cp a.go ~/internal/a.go",
			`cat a.go > "$HOME/internal/a.go"`,
		)
	}
}

// TestWorktreeCopyJudgesARemoteLineByWhereItWrites asks bashWorktreeCopy
// alone about lines another host runs, from a session in the main checkout
// and from sessions inside the worktree.
//
// VALIDATES: a remote line writes on that host. Its absolute destination
// counts as outside the checkout, whether or not it names .claude/worktrees,
// so a remote test run whose log lands in /tmp passes, and so does a remote
// cat into /home/t/main/internal/a.go, which writes on the other host, never
// in this checkout. A remote `cd` whose target names .claude/worktrees/<name>
// places the relative paths after it in that worktree. A remote relative path
// with no such cd stays unplaced: refused when the line also read a worktree
// file, and passed when it read none, since a remote directory the hook
// cannot name is no read. A remote read whose output an outer redirection
// writes into this checkout's main tree stays refused.
// PREVENTS: every redirection of a remote line refused, because a path of
// another host was always unplaced and its directory always read.
func TestWorktreeCopyJudgesARemoteLineByWhereItWrites(t *testing.T) {
	main, worktree := worktreeSession(t)
	sessions := []struct{ root, cwd string }{{main, main}, {main, worktree}, {worktree, worktree}}
	for _, session := range sessions {
		expectWorktreeCopyPassed(t, session.root, session.cwd, "the remote line writes on the other host, outside this checkout or in a remote worktree",
			"ssh claude 'cd ~/Code/z/main/.claude/worktrees/x && ./le test unit p > /tmp/x.log 2>&1'",
			"ssh claude 'cd ~/Code/z/main/.claude/worktrees/x && go test ./... > log.txt'",
			"ssh claude 'cat .claude/worktrees/x/a.go > /home/t/main/internal/a.go'",
		)
		expectWorktreeCopyRefused(t, session.root, session.cwd, "its relative destination is unplaced and the line read a worktree file",
			"ssh claude 'cat .claude/worktrees/x/a.go > a.go'",
		)
	}
	expectWorktreeCopyPassed(t, main, worktree, "the remote line reads no worktree file, or writes outside the checkout",
		"ssh claude 'git log > /tmp/l'",
		"rex -S laptop run -s S -- sh -c 'git log > /tmp/x'",
		"ssh claude 'git log > l.txt'",
	)
	expectWorktreeCopyRefused(t, main, main, "the outer redirection writes what the remote line read into the main tree",
		"ssh claude 'cat ~/main/.claude/worktrees/x/a.go' > internal/a.go",
		"ssh claude 'cd ~/z/.claude/worktrees/x && cat a.go' > internal/a.go",
	)
}

// TestWorktreeCopyPlacesADestinationBoundToAPathTheLinePlaced asks
// bashWorktreeCopy alone about destinations that expand a variable the line
// bound, from inside the worktree and from the main checkout.
//
// VALIDATES: a destination that starts with a plain `$f` of a for loop's
// variable bound only to worktree paths, with no `..` after it, stays in the
// worktree; a name assigned once by a bare assignment outside a loop or a
// function is its value, placed where it lands (tmp/ of the worktree, /tmp,
// or the main tree). A loop word the shell expands into any path, a `..`
// after the variable, a name assigned twice, a prefix assignment, which sets
// the name for its command only, and an operator that cuts the worktree off
// the value (`${f#.claude/worktrees/x/}`) leave the destination unplaced, or
// in the main tree, and refused.
// PREVENTS: every redirection to a variable refused although the line placed
// it, and a placement read from a binding the write may not see.
func TestWorktreeCopyPlacesADestinationBoundToAPathTheLinePlaced(t *testing.T) {
	main, worktree := worktreeSession(t)
	for _, root := range []string{main, worktree} {
		expectWorktreeCopyPassed(t, root, worktree, "the destination is bound to a path in the worktree or outside the checkout",
			`for f in *.go; do gofmt -l "$f" > "$f.out"; done`,
			`out=tmp/x.log; go test ./... > "$out"`,
			`out=/tmp/x; go test ./... > "$out"`,
			"out=/tmp/x; go test ./... > ${out}.log",
		)
		expectWorktreeCopyRefused(t, root, worktree, "the destination may be the main tree",
			// The loop's words may expand to any path, so $f is not placed
			// in the worktree.
			`for f in $X/*.go; do gofmt -l "$f" > "$f.out"; done`,
			// A `..` after the variable may climb out of the worktree.
			`for f in *.go; do gofmt -l "$f" > "$f/../../../../internal/a.go"; done`,
			// A name assigned twice may hold either value when the write runs.
			`out=/tmp/x; f() { out=../../../internal/a.go; }; f; go test ./... > "$out"`,
			// A prefix assignment sets the name for its command only.
			`out=/tmp/x go test ./... > "$out"`,
		)
	}
	expectWorktreeCopyRefused(t, main, main, "the destination cuts the worktree off the path, or is bound to the main tree",
		`for f in .claude/worktrees/x/*.go; do cp "$f" "${f#.claude/worktrees/x/}"; done`,
		`out=internal/a.go; cat .claude/worktrees/x/a.go > "$out"`,
	)
}

// TestWorktreeCopyPlacesABoundDestinationOnlyFromWhereItWasBound asks
// bashWorktreeCopy alone about destinations that start with a variable the
// line bound to a relative worktree path, written after a `cd` out of the
// worktree, from inside the worktree with the session rooted at the main
// checkout and at the worktree.
//
// VALIDATES: a relative value names a path from the directory it was bound
// in, so a `$out` or `$f` the line placed in the worktree stays placed only
// where the write runs from that directory; after `cd ../../..` into the
// main checkout the destination is unplaced, and the copy is refused.
// PREVENTS: a destination placed in the worktree by a binding whose relative
// value now lands in the main checkout.
func TestWorktreeCopyPlacesABoundDestinationOnlyFromWhereItWasBound(t *testing.T) {
	main, worktree := worktreeSession(t)
	for _, root := range []string{main, worktree} {
		expectWorktreeCopyRefused(t, root, worktree, "the relative value is written from the main checkout, not from the worktree it was bound in",
			`export out=a.go; cd ../../..; cat .claude/worktrees/x/a.go > "$out"`,
			`for f in internal/a.go; do :; done; cd ../../.. && cat .claude/worktrees/x/a.go > "$f"`,
		)
	}
}

// TestWorktreeCopyReadsTheValueAtTheHeadOfAnExpandedDestination asks
// bashWorktreeCopy alone about test logs written to a destination that
// starts with a variable the line assigned once and expands a second
// variable after it, from inside the worktree with the session rooted at the
// main checkout and at the worktree.
//
// VALIDATES: a leading `$name` the line bound to a value is that value, as a
// leading `$HOME` is the home directory, so `"$log/run-$i.log"` with log
// bound to ~/tmp and `"$out/$n.log"` with out bound to /tmp/ze land outside
// the checkout, and pass.
// PREVENTS: a destination that expands one variable the hook cannot read
// counted unplaced although its head is a value the line bound, which
// refused a test log kept under ~/tmp or /tmp.
func TestWorktreeCopyReadsTheValueAtTheHeadOfAnExpandedDestination(t *testing.T) {
	main, worktree := worktreeSession(t)
	worktreeHome(t)
	for _, root := range []string{main, worktree} {
		expectWorktreeCopyPassed(t, root, worktree, "the destination's head is a value outside the checkout",
			`log=~/tmp; go test ./... > "$log/run-$i.log"`,
			`out=/tmp/ze; go test ./... > "$out/$n.log"`,
		)
	}
}

// TestWorktreeCopyJudgesARemoteRelativePathThatClimbsOutOfTheWorktree asks
// bashWorktreeCopy alone about a remote line that copies a worktree file to
// a relative path after a `cd` into that worktree, from a session in the
// main checkout and from sessions inside the worktree.
//
// VALIDATES: a remote relative path is placed in the worktree a remote `cd`
// named only while it stays inside that worktree; `../../../internal/a.go`
// climbs out of it, so it is unplaced, and the copy is refused.
// PREVENTS: any relative path after a remote `cd` into a worktree passed as
// outside the checkout, wherever it climbs to.
func TestWorktreeCopyJudgesARemoteRelativePathThatClimbsOutOfTheWorktree(t *testing.T) {
	main, worktree := worktreeSession(t)
	sessions := []struct{ root, cwd string }{{main, main}, {main, worktree}, {worktree, worktree}}
	for _, session := range sessions {
		expectWorktreeCopyRefused(t, session.root, session.cwd, "the relative destination climbs out of the remote worktree, so it is unplaced",
			"ssh claude 'cd ~/Code/z/main/.claude/worktrees/x && cp a.go ../../../internal/a.go'",
		)
	}
}

// TestWorktreeCopyPlacesARemoteRelativePathInTheRemoteCheckoutTmp asks
// bashWorktreeCopy alone about remote lines that write, after a `cd` into a
// remote worktree, to a relative path that climbs into the remote checkout's
// tmp/, from a session in the main checkout and from sessions inside the
// worktree.
//
// VALIDATES: the remote worktree tail models the remote checkout root as `/`,
// so `../../../tmp/out.log` from .claude/worktrees/x is under that checkout's
// gitignored tmp/, which holds no tracked work: a remote test run with its log
// there, and a copy of a worktree file there, pass, as the local tmp/
// exemption passes them. A path that climbs to a sibling whose name only
// starts with tmp (`../../../tmpx/out.log`) is no tmp/ path, stays unplaced,
// and is refused, since the line runs inside the remote worktree.
// PREVENTS: a remote run that keeps its log in the remote checkout's tmp/
// refused as a copy into the main tree, and the tmp/ exemption read as a
// prefix of the name.
func TestWorktreeCopyPlacesARemoteRelativePathInTheRemoteCheckoutTmp(t *testing.T) {
	main, worktree := worktreeSession(t)
	sessions := []struct{ root, cwd string }{{main, main}, {main, worktree}, {worktree, worktree}}
	for _, session := range sessions {
		expectWorktreeCopyPassed(t, session.root, session.cwd, "the relative destination climbs into the remote checkout's tmp/",
			"ssh h 'cd ~/Code/z/main/.claude/worktrees/x && ./le test unit pkg > ../../../tmp/out.log'",
			"ssh h 'cd ~/Code/z/main/.claude/worktrees/x && cp a.go ../../../tmp/a.go'",
		)
		expectWorktreeCopyRefused(t, session.root, session.cwd, "tmpx is no tmp/ path, so the relative destination is unplaced",
			"ssh h 'cd ~/Code/z/main/.claude/worktrees/x && ./le test unit pkg > ../../../tmpx/out.log'",
		)
	}
}

// TestWorktreeCopyRefusalSaysOnlyOfAnUnplacedDestinationThatItCouldNotPlace
// asks bashWorktreeCopy alone for its refusals of two copies out of a
// worktree: one to a destination that starts with a `$X` the hook cannot
// read, one to internal/x.go.
//
// VALIDATES: the first refusal names the destination as typed and says that
// a destination the hook cannot place counts as the main tree; the second,
// whose destination the hook placed in the main tree, does not say it.
// PREVENTS: the could-not-place line given to every refusal, or to none, and
// a line that still names `~` as a head the hook cannot place.
func TestWorktreeCopyRefusalSaysOnlyOfAnUnplacedDestinationThatItCouldNotPlace(t *testing.T) {
	const unplacedLine = "A destination the hook cannot place, such as one that starts with a `$VAR` the hook cannot read, counts as the main tree."
	unplaced := guardVerdict(bashWorktreeCopy, t.TempDir(), "cp .claude/worktrees/a/x.go $X/internal/x.go")
	if unplaced == nil || unplaced.code != 2 {
		t.Fatalf("verdict %+v, want exit 2: its destination may be the main tree", unplaced)
	}
	for _, want := range []string{"It writes $X/internal/x.go from .claude/worktrees/a/x.go.", unplacedLine} {
		if !strings.Contains(unplaced.message, want) {
			t.Errorf("the refusal does not name %q: %s", want, unplaced.message)
		}
	}
	placed := guardVerdict(bashWorktreeCopy, t.TempDir(), "cp .claude/worktrees/a/x.go internal/x.go")
	if placed == nil || placed.code != 2 {
		t.Fatalf("verdict %+v, want exit 2: it copies into the main tree", placed)
	}
	if strings.Contains(placed.message, "cannot place") {
		t.Errorf("the refusal says the hook could not place internal/x.go, which it placed in the main tree: %s", placed.message)
	}
}
