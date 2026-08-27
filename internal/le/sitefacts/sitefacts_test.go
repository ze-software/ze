// The goal is that a published number is a claim about what git holds, not
// about what happens to be on disk. The method is a fixture checkout whose
// working tree and whose commit disagree on purpose, so a counter that read the
// wrong one cannot pass.

package sitefacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/leroot"
)

// fixtureModule is the go.mod every fixture carries, because the derivation
// asks the toolchain what a package is and the toolchain answers for a module.
// The version is one every supported toolchain already has, so `go list` never
// reaches the network for another one.
const fixtureModule = "module fixture\n\ngo 1.21\n"

// newCheckout answers a fixture checkout holding the named files, every one of
// them committed. The caller edits a file or adds an untracked one afterwards
// when it wants the working tree and the last commit to disagree.
//
// Each Go file goes in a directory of its own in these tests, because one
// directory holding two package names is not a module the toolchain can list.
func newCheckout(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	run := fixtureGit(t, root)

	run("init", "-q")
	writeFixtureFile(t, root, "go.mod", fixtureModule)
	for name, body := range files {
		writeFixtureFile(t, root, name, body)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "the fixture")

	// The update writes into it, and a checkout of ze always has it.
	if err := os.MkdirAll(filepath.Join(root, "website", "data"), 0o750); err != nil {
		t.Fatalf("create the fixture data directory: %v", err)
	}
	return root
}

// fixtureGit answers a git runner for the fixture checkout at root.
//
// The fixture answers for itself: a git that read this machine's global config
// would sign a commit made here, or refuse it for want of an identity, and
// either one makes the test a report about the machine.
func fixtureGit(t *testing.T, root string) func(args ...string) {
	t.Helper()

	empty := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("write the fixture git config: %v", err)
	}
	environment := append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+empty,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=sitefacts fixture",
		"GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=sitefacts fixture",
		"GIT_COMMITTER_EMAIL=fixture@example.invalid",
	)

	return func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
		cmd.Env = environment
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in the fixture: %v\n%s", args, err, out)
		}
	}
}

// commit lands everything the fixture holds, which is how a regeneration
// reaches a commit: the file goes in with whatever moved the counts.
func commit(t *testing.T, root string) {
	t.Helper()

	git := fixtureGit(t, root)
	git("add", "-A")
	git("commit", "-q", "-m", "the regenerated facts")
}

// checked runs the staleness gate over the fixture at root and answers what it
// reported, holding it to the exit code the caller expects.
func checked(t *testing.T, root string, want int) Report {
	t.Helper()

	answer, code := judge(root)
	if code != want {
		t.Fatalf("check answered %d, want %d: %v", code, want, answer)
	}
	report, ok := answer.(Report)
	if !ok {
		t.Fatalf("check answered a %T, want the report the pipe operators render", answer)
	}
	return report
}

// writeFixtureFile writes one file of a fixture checkout, creating its parents.
func writeFixtureFile(t *testing.T, root, name, body string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create the parent of %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// updated runs the regeneration over the fixture at root and answers what it
// reported, failing the test when the action itself failed.
func updated(t *testing.T, root string) written {
	t.Helper()

	answer, code := update(root)
	if code != 0 {
		t.Fatalf("update answered %d, want 0", code)
	}
	report, ok := answer.(written)
	if !ok {
		t.Fatalf("update answered a %T, want the report the pipe operators render", answer)
	}
	return report
}

// readFacts answers the facts the update wrote, decoded from the file itself
// rather than from the value the action returned: the file is what the site
// build reads, so the file is what the test judges.
func readFacts(t *testing.T, root string) facts {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(root, factsFile))
	if err != nil {
		t.Fatalf("read the committed facts: %v", err)
	}

	var decoded facts
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode the committed facts: %v", err)
	}
	return decoded
}

// TestUpdateWritesTheDerivedFacts is the regeneration end to end: the action
// walks the fixture, writes the file, and records what each number is a claim
// about.
func TestUpdateWritesTheDerivedFacts(t *testing.T) {
	root := newCheckout(t, map[string]string{
		"a/a.go": "// Design: one\n// Detail: two\npackage a\n",
		"b/b.go": "// Design: three\n// Overview: four\n// Related: five\npackage b\n",
	})

	answer, code := update(root)
	if code != 0 {
		t.Fatalf("update answered %d, want 0", code)
	}
	if answer == nil {
		t.Fatal("update answered nothing, so no pipe operator has rows to act on")
	}

	written := readFacts(t, root)
	for name, want := range map[string]int{
		"repo.design_comments": 2,
		"repo.detail_comments": 3,
	} {
		got, ok := written.Facts[name]
		if !ok {
			t.Fatalf("%s is absent from %s", name, factsFile)
		}
		if got.Value != want {
			t.Errorf("%s is %d, want %d", name, got.Value, want)
		}
		if got.Category != categoryCommitted {
			t.Errorf("%s is categorized %q, want %q", name, got.Category, categoryCommitted)
		}
		if got.Source == "" {
			t.Errorf("%s records no source, so a reader cannot re-derive it", name)
		}
	}
}

// TestUpdateCountsWhatGitHolds is the property the whole file exists for. An
// untracked Go file is on disk and in no commit, so it moves no published
// number.
func TestUpdateCountsWhatGitHolds(t *testing.T) {
	root := newCheckout(t, map[string]string{
		"a/a.go": "// Design: one\npackage a\n",
	})
	writeFixtureFile(t, root, "untracked/untracked.go", "// Design: two\n// Design: three\npackage untracked\n")

	if _, code := update(root); code != 0 {
		t.Fatalf("update answered %d, want 0", code)
	}

	if got := readFacts(t, root).Facts["repo.design_comments"].Value; got != 1 {
		t.Errorf("repo.design_comments is %d, want 1: the untracked file was counted", got)
	}
}

// TestUpdateCountsThePackagesGitHolds holds the package count to the same
// property as the annotation counts. The toolchain says what a package is, and
// git says which of them the repository holds: the untracked directory here is
// a package `go list` reports and no commit carries.
func TestUpdateCountsThePackagesGitHolds(t *testing.T) {
	root := newCheckout(t, map[string]string{
		"a/a.go":    "package a\n",
		"b/b.go":    "package b\n",
		"b/more.go": "package b\n",
	})
	writeFixtureFile(t, root, "c/c.go", "package c\n")

	if _, code := update(root); code != 0 {
		t.Fatalf("update answered %d, want 0", code)
	}

	// Two, not three: `c` is on this disk alone. Not four either: two files of
	// one directory are one package.
	if got := readFacts(t, root).Facts["repo.go_packages"].Value; got != 2 {
		t.Errorf("repo.go_packages is %d, want 2", got)
	}
}

// TestUpdateNamesTheGoFilesNoCommitHolds is the warning half of the
// regeneration, and the reason it is not noise: a clean tree answers nothing,
// and a tree carrying work answers which files that work is in. Several
// sessions share one checkout of ze, so the files a regeneration describes are
// as often somebody else's as they are the runner's own.
func TestUpdateNamesTheGoFilesNoCommitHolds(t *testing.T) {
	root := newCheckout(t, map[string]string{"a/a.go": "// Design: one\npackage a\n"})

	if clean := updated(t, root).Uncommitted; len(clean) != 0 {
		t.Fatalf("a clean tree named %v as uncommitted, want nothing", clean)
	}

	writeFixtureFile(t, root, "a/a.go", "// Design: one\n// Design: two\npackage a\n")
	writeFixtureFile(t, root, "c/c.go", "package c\n")

	named := map[string]string{}
	for _, entry := range updated(t, root).Uncommitted {
		named[entry.Path] = entry.Status
	}
	if got, ok := named["a/a.go"]; !ok || got != " M" {
		t.Errorf("the edited file is named %q, %v, want \" M\", true", got, ok)
	}
	if got, ok := named["c/c.go"]; !ok || got != "??" {
		t.Errorf("the untracked file is named %q, %v, want \"??\", true", got, ok)
	}
	if len(named) != 2 {
		t.Errorf("the update named %v, want those two files alone", named)
	}
}

// TestUpdateWritesTheSameBytesTwice is what lets a gate compare the committed
// file against a fresh derivation. A run that wrote a clock reading, or that
// ordered its facts by a map walk, would answer differently each time.
func TestUpdateWritesTheSameBytesTwice(t *testing.T) {
	root := newCheckout(t, map[string]string{
		"a/a.go": "// Design: one\n// Detail: two\npackage a\n",
		"b/b.go": "// Design: three\npackage b\n",
	})

	if _, code := update(root); code != 0 {
		t.Fatalf("the first update answered %d, want 0", code)
	}
	first, err := os.ReadFile(filepath.Join(root, factsFile))
	if err != nil {
		t.Fatalf("read the first answer: %v", err)
	}

	if _, code := update(root); code != 0 {
		t.Fatalf("the second update answered %d, want 0", code)
	}
	second, err := os.ReadFile(filepath.Join(root, factsFile))
	if err != nil {
		t.Fatalf("read the second answer: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("two runs over one tree wrote different bytes:\n%s\n%s", first, second)
	}
}

// TestUpdateCountsTheInteropSuiteGitHolds holds the two interop figures to the
// same property as the rest: a scenario directory another session is part-way
// through writing is on this disk and in no commit, so it publishes nothing.
func TestUpdateCountsTheInteropSuiteGitHolds(t *testing.T) {
	root := newCheckout(t, map[string]string{
		"a/a.go":                                "package a\n",
		"test/interop/Dockerfile.bird":          "FROM bird\n",
		"test/interop/Dockerfile.gobgp":         "FROM gobgp\n",
		"test/interop/Dockerfile.ze":            "FROM ze\n",
		"test/interop/README.md":                "the suite\n",
		"test/interop/scenarios/one/ze.conf":    "\n",
		"test/interop/scenarios/two/ze.conf":    "\n",
		"test/interop/scenarios/two/check.py":   "\n",
		"test/interop/scenarios/.draft/ze.conf": "\n",
	})
	writeFixtureFile(t, root, "test/interop/scenarios/three/ze.conf", "\n")
	writeFixtureFile(t, root, "test/interop/Dockerfile.frr", "FROM frr\n")

	if _, code := update(root); code != 0 {
		t.Fatalf("update answered %d, want 0", code)
	}

	written := readFacts(t, root)
	for name, want := range map[string]int{
		// Two peer images git holds, plus the one a run names by variable.
		// Neither the ze image, which is not a peer, nor the Dockerfile that
		// sits on this disk alone.
		"interop.targets": 3,
		// Three directories on this disk, two of them in the commit and
		// visible. Two files of one directory are one scenario.
		"interop.scenarios": 2,
		// The hidden directory the figure above leaves out.
		"interop.scenario_dirs_raw": 3,
	} {
		if got := written.Facts[name].Value; got != want {
			t.Errorf("%s is %d, want %d", name, got, want)
		}
	}
}

// TestTheFileRecordsTheFactsItCannotDerive is why a reader can tell what each
// published number is a claim about. Two of them come from running the built
// ze, so no commit holds them, and a file silent about that would leave a
// reader to work out whether a missing number is uncommitted or forgotten.
func TestTheFileRecordsTheFactsItCannotDerive(t *testing.T) {
	root := newCheckout(t, map[string]string{"a/a.go": "package a\n"})
	updated(t, root)

	written := readFacts(t, root)
	for _, name := range []string{"cli_commands", "config_sections"} {
		held, ok := written.Live[name]
		if !ok {
			t.Fatalf("%s is absent from %s", name, factsFile)
		}
		if held.Category != categoryBuiltBinary {
			t.Errorf("%s is categorized %q, want %q", name, held.Category, categoryBuiltBinary)
		}
		if held.Source == "" {
			t.Errorf("%s records no source, so a reader cannot tell what produces it", name)
		}
	}
	if _, counted := written.Facts["cli_commands"]; counted {
		t.Error("cli_commands carries a value, and a number here would be a claim about a binary rather than about a commit")
	}
}

// TestRenderPublishesWhatTheSiteShows holds this package's idea of a published
// figure to the site's own. Every string below was measured from fmt_int
// (website/tools/sitefacts.py), because a check judging by another rule would
// report a page stale that no reader can see change.
func TestRenderPublishesWhatTheSiteShows(t *testing.T) {
	for value, want := range map[int]string{
		0:       "0",
		9:       "9",
		99:      "99",
		100:     "100",
		101:     "100+",
		130:     "130",
		687:     "680+",
		689:     "680+",
		690:     "690",
		999:     "990+",
		1000:    "1,000",
		1001:    "1,000+",
		3699:    "3,600+",
		3700:    "3,700",
		3852:    "3,800+",
		3899:    "3,800+",
		3900:    "3,900",
		12345:   "12,300+",
		999999:  "999,900+",
		1000000: "1,000,000",
		1234567: "1,200,000+",
	} {
		if got := render(value); got != want {
			t.Errorf("render(%d) is %q, want %q", value, got, want)
		}
	}
}

// TestCheckAgreesWithTheCommitItJudges is the gate saying nothing when there is
// nothing to say. A regeneration committed with the tree it described leaves
// every published figure agreeing with that commit.
func TestCheckAgreesWithTheCommitItJudges(t *testing.T) {
	root := newCheckout(t, map[string]string{
		"a/a.go": "// Design: one\n// Detail: two\npackage a\n",
	})
	updated(t, root)
	commit(t, root)

	if stale := checked(t, root, 0).Stale; len(stale) != 0 {
		t.Errorf("the gate reported %v stale, want nothing", stale)
	}
}

// TestCheckJudgesTheCommitAndNotTheWorkingTree is the property the gate exists
// for, and the one it could most easily fail at. Several sessions share a
// checkout of ze, so a gate that read the tree would answer differently in two
// of them at one moment -- which is the defect it is looking for, not a way to
// look for it.
func TestCheckJudgesTheCommitAndNotTheWorkingTree(t *testing.T) {
	root := newCheckout(t, map[string]string{"a/a.go": "// Design: one\npackage a\n"})
	updated(t, root)
	commit(t, root)

	// Five more annotations on this disk and in no commit. The counts here are
	// under a hundred, where nothing is rounded, so a gate reading the tree
	// would answer six against the commit's one and report it stale.
	writeFixtureFile(t, root, "a/a.go", "// Design: one\n// Design: two\n// Design: three\n// Design: four\n// Design: five\n// Design: six\npackage a\n")
	writeFixtureFile(t, root, "b/b.go", "// Design: seven\npackage b\n")

	if stale := checked(t, root, 0).Stale; len(stale) != 0 {
		t.Errorf("the gate reported %v stale, and every one of them is a working-tree edit no commit holds", stale)
	}
}

// TestCheckNamesTheStaleFactAndTheFix is what a red is worth: which figure the
// site would publish wrongly, and the command that clears it.
func TestCheckNamesTheStaleFactAndTheFix(t *testing.T) {
	root := newCheckout(t, map[string]string{"a/a.go": "// Design: one\npackage a\n"})
	updated(t, root)
	tamper(t, root, "repo.design_comments", 40)
	commit(t, root)

	report := checked(t, root, 1)
	if len(report.Stale) != 1 {
		t.Fatalf("the gate reported %v, want the one fact that moved", report.Stale)
	}
	got := report.Stale[0]
	if got.Fact != "repo.design_comments" {
		t.Errorf("the gate named %q, want repo.design_comments", got.Fact)
	}
	if got.Committed != "40" || got.Derived != "1" {
		t.Errorf("the gate reported committed %q derived %q, want \"40\" and \"1\"", got.Committed, got.Derived)
	}
	if !strings.Contains(report.Fix, "ze-site-facts-update") {
		t.Errorf("the gate offers %q as the fix, which names no action to run", report.Fix)
	}
	if !strings.Contains(report.Text(), "repo.design_comments") {
		t.Errorf("the rendering a person reads is %q, and it does not name the stale fact", report.Text())
	}
}

// TestCheckReportsAFileTheCommitDoesNotHold is the other half of a red: a
// commit carrying no facts at all publishes nothing checkable, and the gate
// says so fact by fact rather than passing over an absent file.
func TestCheckReportsAFileTheCommitDoesNotHold(t *testing.T) {
	root := newCheckout(t, map[string]string{"a/a.go": "// Design: one\npackage a\n"})

	report := checked(t, root, 1)
	if len(report.Stale) == 0 {
		t.Fatal("the gate reported nothing for a commit that holds no facts file")
	}
	for _, entry := range report.Stale {
		if entry.Committed != "absent" {
			t.Errorf("%s reads committed %q, want \"absent\"", entry.Fact, entry.Committed)
		}
	}
}

// TestCheckSweepsTheWorktreeAKilledRunLeft holds the gate to leaving nothing
// behind in a checkout several sessions share. A run interrupted between the
// checkout and its removal leaves a directory AND a registration, and the
// registration alone holds its commit against garbage collection for three
// months.
func TestCheckSweepsTheWorktreeAKilledRunLeft(t *testing.T) {
	root := newCheckout(t, map[string]string{"a/a.go": "// Design: one\npackage a\n"})
	updated(t, root)
	commit(t, root)

	git := fixtureGit(t, root)
	abandoned := filepath.Join(root, worktreeRoot, "20260101T000000Z-abandoned")
	git("worktree", "add", "--quiet", "--detach", abandoned, "HEAD")
	stale := time.Now().Add(-2 * abandonedAfter)
	if err := os.Chtimes(abandoned, stale, stale); err != nil {
		t.Fatalf("age the abandoned worktree: %v", err)
	}

	checked(t, root, 0)

	if _, err := os.Stat(abandoned); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the abandoned worktree is still at %s (%v)", abandoned, err)
	}
	if registered := worktreeCount(t, root); registered != 1 {
		t.Errorf("git holds %d worktree registrations, want 1: the checkout itself", registered)
	}
}

// worktreeCount answers how many worktrees the fixture at root has registered.
func worktreeCount(t *testing.T, root string) int {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", "-C", root, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list the fixture worktrees: %v", err)
	}
	return strings.Count(string(out), "worktree ")
}

// tamper rewrites one value in the committed file, which is what a fact going
// stale looks like from the gate's side: the file says one thing and the commit
// says another.
func tamper(t *testing.T, root, name string, value int) {
	t.Helper()

	held := readFacts(t, root)
	entry, ok := held.Facts[name]
	if !ok {
		t.Fatalf("%s holds no %s to rewrite", factsFile, name)
	}
	entry.Value = value
	held.Facts[name] = entry

	raw, err := json.MarshalIndent(held, "", "  ")
	if err != nil {
		t.Fatalf("render the rewritten facts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, factsFile), append(raw, '\n'), 0o600); err != nil {
		t.Fatalf("write the rewritten facts: %v", err)
	}
}

// TestCommandIsRegistered is the wiring: the blank import in internal/le/register.go
// reaches this init(), and le owns the name.
func TestCommandIsRegistered(t *testing.T) {
	if !leroot.Owns(area) {
		t.Fatalf("le does not own %q, so nothing can type it", area)
	}
}

// TestTheWriterIsMarked holds the listing to the one fact a reader must not
// have to look up before running an action.
func TestTheWriterIsMarked(t *testing.T) {
	for _, row := range actions.Actions().Actions {
		switch row.Verb {
		case "update":
			if !row.Writes {
				t.Error("update is not marked as writing, and it rewrites a committed file")
			}
		case "check":
			if row.Writes {
				t.Error("check is marked as writing, and it must not write")
			}
		default:
			t.Errorf("unknown action %q: this test does not judge it", row.Verb)
		}
	}
}
