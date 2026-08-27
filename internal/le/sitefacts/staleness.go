// Design: docs/architecture/core-design.md -- what makes a published number checkable
//
// staleness.go answers `le site-facts check`: does the committed file still
// publish what the commit says about this repository?
//
// Two properties make that question answerable, and both are deliberate.
//
// It judges a COMMIT, never the working tree. Several sessions share a checkout
// of ze, so a check that read the tree would answer differently in two of them
// at the same moment -- which is the defect this tool exists to remove, not a
// way to look for it. The commit is materialized in a throwaway worktree and
// judged there, which is what ze-verify-worktree does for the pre-commit gate
// (scripts/dev/verify_worktree.py).
//
// It judges the PUBLISHED figure, not the exact count. A count reaches a page
// through fmt_int, which floors a magnitude to one tenth of its visible unit
// (website/tools/sitefacts.py, display_step), so 3852 and 3899 are one string.
// Exact equality is not something the commit flow can deliver either: git
// ls-files answers from the INDEX, so a regeneration run before `git add`
// cannot count the files that same commit adds, and a gate demanding exactness
// would go red on the commit that fixed it. What the site publishes is the
// claim, so what the site publishes is what this compares.
//
// Related: sitefacts.go -- the derivation both actions share.

package sitefacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// worktreeRoot is where a materialized commit lands, under the checkout's own
// scratch directory. It sits beside tmp/verify-worktree, which the pre-commit
// gate uses for the same purpose (scripts/dev/verify_worktree.py, worktree_path).
const worktreeRoot = "tmp/site-facts-check"

// worktreeTimeout bounds the checkout of one commit. It writes every tracked
// file: 19,849 of them in 3.0 seconds on this machine with a warm page cache,
// so this leaves two orders of magnitude for a cold one and still refuses a git
// that has stopped.
const worktreeTimeout = 5 * time.Minute

// abandonedAfter is how old a leftover worktree has to be before a run treats
// it as one a killed run left behind.
//
// A run of this gate cannot last that long: the checkout, the git reads and the
// package listing are each bounded above, and their sum is under eight minutes.
// So a directory an hour old belongs to no run that is still going, and
// removing it is what keeps a killed run from leaving a registration that holds
// its commit against garbage collection for three months.
const abandonedAfter = time.Hour

// shaShown is how much of a commit id the report prints. It is what git itself
// abbreviates to in this repository, and the full id stays in the JSON.
const shaShown = 12

// fixAction is what a person runs when a fact has gone stale. The check names
// it in every rendering, because a gate that reports a red without the command
// that clears it leaves the reader to search for one.
const fixAction = "make ze-site-facts-update, then commit website/data/repo-facts.json with the change that moved the count"

// roundedMark is the suffix fmt_int puts on a figure it floored.
const roundedMark = "+"

// Difference is one fact the committed file and the commit disagree about, in
// the terms the site publishes rather than in the terms it counts.
type Difference struct {
	Fact      string `json:"fact"`
	Committed string `json:"committed"`
	Derived   string `json:"derived"`
}

// Report is the whole answer of one check: which file was judged, against which
// commit, what has gone stale in it, and what fixes that.
//
// Fix is filled whether or not anything is stale. A reader who pipes this to
// json gets the remediation with the finding rather than beside it.
type Report struct {
	File   string       `json:"file"`
	Commit string       `json:"commit"`
	Stale  []Difference `json:"stale"`
	Fix    string       `json:"fix"`
}

// Text renders the verdict for a person. It ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it (internal/le/leroot, Prose).
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str(r.File).Str(" at ").Str(r.Commit[:min(len(r.Commit), shaShown)]).Str(": ")

	if len(r.Stale) == 0 {
		return tb.Str("every published fact agrees with the commit\n").String()
	}

	tb.Int(int64(len(r.Stale))).Str(" published fact")
	if len(r.Stale) > 1 {
		tb.Byte('s')
	}
	tb.Str(" no longer agree with the commit\n")
	for _, entry := range r.Stale {
		tb.Str("  ").Str(entry.Fact).Str(": committed ").Str(entry.Committed).Str(", derived ").Str(entry.Derived).Byte('\n')
	}
	return tb.Str("run: ").Str(fixAction).Byte('\n').String()
}

// check answers whether the committed facts of the checkout at root still hold
// for its last commit.
func check(root string) (Report, error) {
	sha, err := headCommit(root)
	if err != nil {
		return Report{}, err
	}

	tree, remove, err := materialize(root, sha)
	if err != nil {
		return Report{}, err
	}
	defer remove()

	derived, err := derive(tree)
	if err != nil {
		return Report{}, err
	}
	committed, err := read(tree)
	if err != nil {
		return Report{}, err
	}

	return Report{
		File:   factsFile,
		Commit: sha,
		Stale:  compare(committed, derived),
		Fix:    fixAction,
	}, nil
}

// read answers the facts the committed file holds in the tree at root.
//
// A tree that holds no such file answers the empty set rather than an error:
// every derived fact is then absent from it, which is the finding, and the
// report says so fact by fact instead of failing with one sentence.
func read(root string) (facts, error) {
	raw, err := os.ReadFile(filepath.Join(root, factsFile)) //nolint:gosec // factsFile is a constant of this package; root is a checkout
	if errors.Is(err, fs.ErrNotExist) {
		return facts{}, nil
	}
	if err != nil {
		return facts{}, fmt.Errorf("sitefacts: read %s: %w", factsFile, err)
	}

	var held facts
	if err := json.Unmarshal(raw, &held); err != nil {
		return facts{}, fmt.Errorf("sitefacts: decode %s: %w", factsFile, err)
	}
	return held, nil
}

// compare answers every fact the committed file and the commit disagree about.
//
// Three disagreements count, and the first is the one the gate exists for. A
// value that PUBLISHES differently is stale. A fact the file does not hold, or
// holds and nothing derives any more, is stale. A fact whose category or source
// has been reworded is stale too: those are what tell a reader what kind of
// claim a number is, and a generated file that describes itself out of date is
// as stale as one that counts out of date.
func compare(committed, derived facts) []Difference {
	var stale []Difference

	for _, name := range slices.Sorted(maps.Keys(derived.Facts)) {
		want := derived.Facts[name]
		got, held := committed.Facts[name]
		switch {
		case !held:
			stale = append(stale, Difference{Fact: name, Committed: "absent", Derived: describe(want)})
		case render(got.Value) != render(want.Value):
			stale = append(stale, Difference{Fact: name, Committed: describe(got), Derived: describe(want)})
		case got.Category != want.Category || got.Source != want.Source:
			stale = append(stale, Difference{Fact: name, Committed: provenance(got.Category, got.Source), Derived: provenance(want.Category, want.Source)})
		}
	}
	for _, name := range slices.Sorted(maps.Keys(committed.Facts)) {
		if _, derives := derived.Facts[name]; !derives {
			stale = append(stale, Difference{Fact: name, Committed: describe(committed.Facts[name]), Derived: "nothing derives this fact"})
		}
	}

	for _, name := range slices.Sorted(maps.Keys(derived.Live)) {
		want := derived.Live[name]
		got, held := committed.Live[name]
		switch {
		case !held:
			stale = append(stale, Difference{Fact: name, Committed: "absent", Derived: provenance(want.Category, want.Source)})
		case got != want:
			stale = append(stale, Difference{Fact: name, Committed: provenance(got.Category, got.Source), Derived: provenance(want.Category, want.Source)})
		}
	}
	for _, name := range slices.Sorted(maps.Keys(committed.Live)) {
		if _, records := derived.Live[name]; !records {
			held := committed.Live[name]
			stale = append(stale, Difference{Fact: name, Committed: provenance(held.Category, held.Source), Derived: "nothing records this fact"})
		}
	}

	return stale
}

// describe says what a fact publishes, and what it counted when the two are not
// the same string. The exact value is what a person regenerating compares; the
// published one is what the gate judged.
func describe(held fact) string {
	shown := render(held.Value)
	if !strings.HasSuffix(shown, roundedMark) {
		return shown
	}

	var tb textbuf.Buffer
	return tb.Str(shown).Str(" (").Int(int64(held.Value)).Byte(')').String()
}

// provenance says what kind of claim a fact is and where it came from.
func provenance(category, source string) string {
	var tb textbuf.Buffer
	return tb.Str(category).Str(", ").Str(source).String()
}

// render answers the string the site publishes for a count.
//
// This is fmt_int, in Go (website/tools/sitefacts.py). The rule is one line of
// arithmetic and it is stated in two languages because the check runs here and
// the render runs there; a third statement of it would be one too many, so a
// change to either MUST move both together.
func render(value int) string {
	step := displayStep(value)
	floored := value / step * step
	shown := group(floored)
	if floored == value {
		return shown
	}

	var tb textbuf.Buffer
	return tb.Str(shown).Str(roundedMark).String()
}

// displayStep answers the precision a count is floored to: one tenth of its
// visible unit, so a figure in the thousands publishes to the nearest hundred.
// Below a hundred nothing is floored, because there the last digit is the
// answer rather than noise.
func displayStep(value int) int {
	switch {
	case value < 100:
		return 1
	case value < 1000:
		return 10
	}

	digits := len(strconv.Itoa(value))
	step := 1
	for range 3*((digits-1)/3) - 1 {
		step *= 10
	}
	return step
}

// group renders a count with a separator every three digits, which is what the
// site's own formatting writes and what every published figure carries.
func group(value int) string {
	digits := strconv.Itoa(value)
	var tb textbuf.Buffer
	for index := range len(digits) {
		if index > 0 && (len(digits)-index)%3 == 0 {
			tb.Byte(',')
		}
		tb.Byte(digits[index])
	}
	return tb.String()
}

// headCommit answers the commit the checkout at root last made.
func headCommit(root string) (string, error) {
	raw, err := output(root, gitTimeout, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("sitefacts: read the last commit of %s: %w", root, err)
	}

	sha := strings.TrimSpace(string(raw))
	if sha == "" {
		return "", fmt.Errorf("sitefacts: %s holds no commit, so there is nothing to judge the file against", root)
	}
	return sha, nil
}

// materialize checks a commit out into a throwaway worktree and answers where
// it landed, with the function that removes it. The caller MUST call that
// function on every path out, and discard MUST be able to run twice: a worktree
// left behind is a registration git keeps for three months.
//
// A NEW directory every time, named for the moment and the commit, because
// several sessions share this checkout and two of them running the gate at once
// must not meet in one directory.
func materialize(root, sha string) (string, func(), error) {
	tree := filepath.Join(root, worktreeRoot, time.Now().UTC().Format("20060102T150405Z")+"-"+sha[:min(len(sha), shaShown)])
	if err := os.MkdirAll(filepath.Dir(tree), 0o750); err != nil {
		return "", nil, fmt.Errorf("sitefacts: create %s: %w", filepath.Dir(tree), err)
	}
	sweep(root)

	if _, err := output(root, worktreeTimeout, "git", "worktree", "add", "--quiet", "--detach", tree, sha); err != nil {
		return "", nil, fmt.Errorf("sitefacts: check %s out into %s: %w", sha, tree, err)
	}
	return tree, func() { discard(root, tree) }, nil
}

// discard drops the worktree, whatever is left of its directory, and its
// registration.
//
// All three, because each one survives the others. `worktree remove` refuses a
// directory it no longer recognizes, and a registration left behind holds its
// commit against garbage collection for three months -- the expiry a bare
// `worktree prune` respects (ai/rules/git-safety.md, Worktree Cleanup).
func discard(root, tree string) {
	// Best effort, and the error is deliberately dropped: this runs on the way
	// out of a throwaway worktree that may already be gone, and a cleanup that
	// refused to finish would strand the tree it was removing. c_ignored_errors
	// matches the blank-assignment SHAPE, so the intent is written rather than
	// spelled `_, _ =`.
	if _, err := output(root, gitTimeout, "git", "worktree", "remove", "--force", tree); err != nil {
		ignored(err)
	}
	if err := os.RemoveAll(tree); err != nil {
		ignored(err)
	}
	if _, err := output(root, gitTimeout, "git", "worktree", "prune", "--expire", "now"); err != nil {
		ignored(err)
	}

	if _, err := os.Stat(tree); err == nil {
		var tb textbuf.Buffer
		fmt.Fprintln(os.Stderr, tb.Str("warning: ").Str(tree).Str(" survived its removal; run `git worktree prune --expire now` once it is gone").String()) //nolint:errcheck // CLI output
	}
}

// sweep drops what a killed run left behind, before this one adds a directory
// of its own.
//
// A run interrupted between `worktree add` and discard leaves both a directory
// and a registration, and neither goes on its own: `worktree prune` respects a
// three-month expiry and refuses a directory that is still there. Nothing here
// can reach a live run (see abandonedAfter), and every failure is ignored on
// purpose: a leftover this could not remove is not a reason to refuse to judge
// the commit.
func sweep(root string) {
	entries, err := os.ReadDir(filepath.Join(root, worktreeRoot))
	if err != nil {
		return
	}

	swept := false
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) < abandonedAfter {
			continue
		}
		if _, err := output(root, gitTimeout, "git", "worktree", "remove", "--force", filepath.Join(root, worktreeRoot, entry.Name())); err != nil {
			ignored(err)
		}
		if err := os.RemoveAll(filepath.Join(root, worktreeRoot, entry.Name())); err != nil {
			ignored(err)
		}
		swept = true
	}
	if swept {
		if _, err := output(root, gitTimeout, "git", "worktree", "prune", "--expire", "now"); err != nil {
			ignored(err)
		}
	}
}

// ignored names a dropped error at the one place a cleanup path drops one.
// A named function is what makes the drop searchable and reviewable, where a
// blank assignment is invisible to both.
func ignored(error) {}
