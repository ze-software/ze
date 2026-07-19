package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

// coveredByBigRunner reports whether test/<name> is walked by one of the "big"
// ze-test runners as a subcommand rather than through registerCIRoot (whose suite
// name equals its directory, so registry.HasRootHandler covers those). The two
// sources of truth are consumed directly so this guard never re-hardcodes them:
//
//   - bgpCIRunnerDirs (cmd_bgp.go): the "ze-test bgp <sub>" dirs
//     (encode/plugin/reload/decode/parse/chaos/chaos-web)
//   - predecessorTestDir (cmd_exabgp.go): "exabgp-compat", walked by "ze-test exabgp"
func coveredByBigRunner(name string) bool {
	return bgpCIRunnerDirs[name] || name == predecessorTestDir
}

// bigRunnerCIDirNames returns the test/<dir> subdirectories the big runners walk,
// derived from the same package-level sources coveredByBigRunner consults.
func bigRunnerCIDirNames() []string {
	names := make([]string, 0, len(bgpCIRunnerDirs)+1)
	for name := range bgpCIRunnerDirs {
		names = append(names, name)
	}
	names = append(names, predecessorTestDir)
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
// some ze-test runner, either as a registered root command (registerCIRoot,
// name == directory; or a big runner registered via registerRoot such as vpp)
// or as a big-runner subcommand directory listed in bigRunnerCIDirs.
//
// A .ci directory that no runner walks is silently dead: nothing discovers it,
// so its tests never fail. That is exactly how test/pppoe/ survived unrun from
// May to July 2026 (spec-fixit-pppoe-orphaned-tests). This guard turns such an
// orphan into a loud, immediate test failure.
//
// VALIDATES: AC-1 (spec-fixit-pppoe-orphaned-tests) — every test/ subdirectory
// holding .ci files has a registered root (or is absent from the tree).
func TestCIRootsRegistered(t *testing.T) {
	baseDir, err := FindBaseDir()
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
		if registry.HasRootHandler(name) || coveredByBigRunner(name) {
			continue
		}
		orphans = append(orphans, name)
	}

	if len(orphans) > 0 {
		sort.Strings(orphans)
		t.Fatalf("orphaned .ci suite(s) with no ze-test runner: %v\n"+
			"Each test/<dir> holding .ci files must be rooted. Fix by either:\n"+
			"  - registering the suite: add registerCIRoot(%q, ...) in internal/test/cli/register.go, or\n"+
			"  - if the directory is a subcommand of a big runner, adding it to bgpCIRunnerDirs (cmd_bgp.go), or\n"+
			"  - deleting the directory if its coverage is stale/redundant.",
			orphans, orphans[0])
	}

	// Keep the big-runner sources honest: a name they claim to walk but that no
	// longer has .ci files is a stale exception that would mask a future orphan
	// sharing its name. Fail so the dead entry is removed at its source.
	for _, name := range bigRunnerCIDirNames() {
		full := filepath.Join(testDir, name)
		if !dirHasCIFiles(full) {
			t.Errorf("stale big-runner CI dir %q: test/%s has no .ci files; remove it from its source (bgpCIRunnerDirs in cmd_bgp.go, or predecessorTestDir in cmd_exabgp.go)", name, name)
		}
	}
}
