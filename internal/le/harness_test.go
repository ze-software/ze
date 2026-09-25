// VALIDATES: every harness command is the le command `test <name>`, registered
// by its own package through internal/le/test/harnesstool, no package under
// internal/test registers a ze root, and every test/<dir> holding .ci files is
// reached by a suite command or a big runner (AC-32 to AC-34 of
// plan/spec-le-subject-first-command-tree.md, and AC-1 of
// spec-fixit-pppoe-orphaned-tests, whose guard moved here from
// internal/test/cli when the suites became le commands).
package le

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/test/cli"
	"github.com/ze-software/ze/internal/test/runner"
)

// harnessRegistration is the call a harness command's register.go makes. It is
// the one mark that separates a harness command from an le member of `test`.
const harnessRegistration = "harnesstool."

// harnessDirectories answers every directory under internal/le/test whose
// register.go registers a harness command, as the command's name under `test`
// without hyphens.
func harnessDirectories(t *testing.T) []string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	namespace := filepath.Join(root, "internal", "le", "test")
	entries, err := os.ReadDir(namespace)
	if err != nil {
		t.Fatalf("read %s: %v", namespace, err)
	}
	found := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		source, readErr := os.ReadFile(filepath.Join(namespace, entry.Name(), "register.go"))
		if readErr != nil {
			continue
		}
		if strings.Contains(string(source), harnessRegistration) {
			found = append(found, entry.Name())
		}
	}
	return found
}

// TestEveryHarnessCommandForwards proves the registration shape: every harness
// package registers a `test` member that forwards its words to the harness
// handler, sits in the suite group, and carries help. The set of forwarding
// `test` members and the set of harness packages must be the same set, so a
// harness command registered anywhere else, or a package that registers under
// another name, fails here by name.
func TestEveryHarnessCommandForwards(t *testing.T) {
	packages := harnessDirectories(t)
	if len(packages) == 0 {
		t.Fatal("no package under internal/le/test registers a harness command")
	}

	forwarding := make([]string, 0, len(packages))
	for _, command := range commandsAtStart {
		member, ok := strings.CutPrefix(command.Name, "test ")
		if !ok || !leroot.Forwards(command.Name) {
			continue
		}
		forwarding = append(forwarding, directoryFor(member))
		group, declared := leroot.GroupOf(command.Name)
		if !declared || group != leroot.GroupSuite {
			t.Errorf("harness command %q is in group %q, want %q", command.Name, group, leroot.GroupSuite)
		}
		if command.Meta.ShortHelp == "" {
			t.Errorf("harness command %q has no help", command.Name)
		}
	}
	slices.Sort(forwarding)
	slices.Sort(packages)
	if !slices.Equal(forwarding, packages) {
		t.Errorf("forwarding test members and harness packages differ:\nmembers:  %v\npackages: %v", forwarding, packages)
	}
}

// TestNoHarnessPackageRegistersARoot proves the half of AC-33 that lets le
// link the harness at all: a root registered under internal/test would collide
// with a ze root of the same name (bgp, web, peer) and panic at init. The
// method reads every non-test Go file under internal/test for the two root
// registration calls.
func TestNoHarnessPackageRegistersARoot(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	walkErr := filepath.WalkDir(filepath.Join(root, "internal", "test"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(source), "RegisterRootHandler(") || strings.Contains(string(source), "registry.RegisterRoot(") {
			t.Errorf("%s registers a root; a harness command is an le member of `test` (harnesstool.Answer)", path)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk internal/test: %v", walkErr)
	}
}

// draftDirName is the incubator directory under test/, taken from the runner
// that resolves it (runner.SuiteDir) rather than re-spelled here.
const draftDirName = runner.DraftDirName

// suiteReached reports whether test/<name> is run by some harness command:
// the suite command `le test <name>`, whose name equals its directory, or a
// big runner that walks it as a subcommand (cli.BigRunnerCIDirs).
func suiteReached(name string) bool {
	if leroot.LookupCommand("test "+name) != nil {
		return true
	}
	return slices.Contains(cli.BigRunnerCIDirs(), name)
}

// draftSuiteNames returns the suite names that have drafts: the immediate
// subdirectories of test/draft holding at least one .ci file.
func draftSuiteNames(draftDir string) []string {
	entries, err := os.ReadDir(draftDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || !dirHasCIFiles(filepath.Join(draftDir, e.Name())) {
			continue
		}
		names = append(names, e.Name())
	}
	return names
}

// dirHasCIFiles reports whether dir contains at least one .ci file anywhere
// beneath it (exabgp-compat nests its .ci under an encoding/ subdirectory).
func dirHasCIFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && !d.IsDir() && filepath.Ext(path) == ".ci" {
			found = true
			return fs.SkipAll
		}
		// Propagate a walk error (aborts this dir's walk); the caller treats an
		// unreadable dir as "no .ci". Repo test dirs are always readable, so this
		// is defensive only.
		return walkErr
	})
	return found
}

// TestCIRootsRegistered is the recurrence guard for orphaned functional-test
// suites: every top-level test/<dir> that holds .ci files MUST be reachable by
// some harness command, either as a suite command `le test <dir>`
// (harnesstool.SuiteAnswer, name == directory; or a big runner such as vpp)
// or as a big-runner subcommand directory (cli.BigRunnerCIDirs).
//
// A .ci directory that no runner walks is silently dead: nothing discovers it,
// so its tests never fail. That is exactly how test/pppoe/ survived unrun from
// May to July 2026 (spec-fixit-pppoe-orphaned-tests). This guard turns such an
// orphan into a loud, immediate test failure.
//
// VALIDATES: AC-1 (spec-fixit-pppoe-orphaned-tests) — every test/ subdirectory
// holding .ci files has a registered command (or is absent from the tree).
func TestCIRootsRegistered(t *testing.T) {
	baseDir, err := lepath.Root()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	testDir := filepath.Join(baseDir, "test")

	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("read %s: %v", testDir, err)
	}

	var orphans []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		full := filepath.Join(testDir, name)
		if !dirHasCIFiles(full) {
			continue
		}
		if name == draftDirName {
			// test/draft is the incubator, not a suite. A runner reaches it as
			// test/draft/<suite> under --draft (runner.SuiteDir), never as
			// test/draft itself. Its CHILDREN carry the orphan risk instead.
			// A draft filed under a misspelled or unregistered suite name is
			// discovered by nothing. That is the same silent death this guard
			// exists to prevent, so each child meets the same predicate.
			for _, suite := range draftSuiteNames(full) {
				if suiteReached(suite) {
					continue
				}
				orphans = append(orphans, filepath.Join(draftDirName, suite))
			}
			continue
		}
		if suiteReached(name) {
			continue
		}
		orphans = append(orphans, name)
	}

	if len(orphans) > 0 {
		slices.Sort(orphans)
		t.Fatalf("orphaned .ci suite(s) with no harness command: %v\n"+
			"Each test/<dir> holding .ci files must be reached. Fix by either:\n"+
			"  - registering the suite: add internal/le/test/%s/register.go calling harnesstool.SuiteAnswer, or\n"+
			"  - if the directory is a subcommand of a big runner, adding it to bgpCIRunnerDirs (internal/test/cli/cmd_bgp.go), or\n"+
			"  - deleting the directory if its coverage is stale/redundant.",
			orphans, directoryFor(orphans[0]))
	}

	// Keep the big-runner sources honest: a name they claim to walk but that no
	// longer has .ci files is a stale exception that would mask a future orphan
	// sharing its name. Fail so the dead entry is removed at its source.
	for _, name := range cli.BigRunnerCIDirs() {
		full := filepath.Join(testDir, name)
		if !dirHasCIFiles(full) {
			t.Errorf("stale big-runner CI dir %q: test/%s has no .ci files; remove it from its source (bgpCIRunnerDirs in cmd_bgp.go, or predecessorTestDir in cmd_exabgp.go)", name, name)
		}
	}
}
