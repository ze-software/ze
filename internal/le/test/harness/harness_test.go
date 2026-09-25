package testharness

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// fakeHarness writes an executable bin/le-test under a fresh root that records
// its argv in argv.txt and exits with code. It sets LE_TEST_NO_BUILD, because
// the fresh root holds no source to build; TestHarnessBuildsOnEveryCall clears
// it again.
func fakeHarness(t *testing.T, code string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"" + filepath.Join(root, "argv.txt") + "\"\nexit " + code + "\n"
	if err := os.WriteFile(filepath.Join(root, "bin", "le-test"), []byte(script), 0o700); err != nil { //nolint:gosec // the fake harness must be executable
		t.Fatalf("write: %v", err)
	}
	t.Setenv("LE_TEST_BIN", "")
	t.Setenv("ZE_TEST_BIN", "")
	t.Setenv("LE_TEST_NO_BUILD", "1")
	t.Setenv("ZE_TEST_NO_BUILD", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	return root
}

// TestHarnessExecsLeTestWithTheTrailingArgv proves `test harness <argv>` runs
// bin/le-test of the checkout with the argv unchanged and answers its exit
// code (AC-7), and that the bare form answers 0 after the harness lists its
// commands.
//
// VALIDATES: AC-7, exec of bin/le-test and exit-code passthrough.
// PREVENTS: a harness verdict lost between the harness and le.
func TestHarnessExecsLeTestWithTheTrailingArgv(t *testing.T) {
	root := fakeHarness(t, "3")
	_, code := answerIn(root, []string{"bgp", "--list"})
	if code != 3 {
		t.Errorf("exit code %d, want the harness's 3", code)
	}
	recorded, err := os.ReadFile(filepath.Join(root, "argv.txt"))
	if err != nil {
		t.Fatalf("the harness did not run: %v", err)
	}
	if got := strings.TrimSpace(string(recorded)); got != "bgp --list" {
		t.Errorf("harness argv %q, want %q", got, "bgp --list")
	}

	bare := fakeHarness(t, "1")
	if _, code := answerIn(bare, nil); code != 0 {
		t.Errorf("bare form exit code %d, want 0", code)
	}
}

// TestHarnessBuildsOnEveryCall proves `test harness` builds before it runs,
// even when bin/le-test already exists (AC-7). The fresh root holds no
// feature-gates.txt, so the build fails, and the harness already on disk
// MUST NOT run in its place.
//
// VALIDATES: AC-7, the harness is built on every call.
// PREVENTS: a stale bin/le-test answering for code it was not built from.
func TestHarnessBuildsOnEveryCall(t *testing.T) {
	root := fakeHarness(t, "0")
	t.Setenv("LE_TEST_NO_BUILD", "")
	env.ResetCache()
	if _, code := answerIn(root, []string{"bgp", "--list"}); code != cannotRun {
		t.Errorf("exit code %d, want %d from the failed build", code, cannotRun)
	}
	if _, err := os.Stat(filepath.Join(root, "argv.txt")); err == nil {
		t.Error("the harness already on disk ran without a build")
	}
}

// TestHarnessNoBuildRefusesAbsentHarness proves LE_TEST_NO_BUILD runs the
// harness as it is and refuses when there is none, rather than building one.
//
// VALIDATES: the explicit opt-out never builds.
// PREVENTS: LE_TEST_NO_BUILD silently compiling a harness.
func TestHarnessNoBuildRefusesAbsentHarness(t *testing.T) {
	root := fakeHarness(t, "0")
	if err := os.Remove(filepath.Join(root, "bin", "le-test")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, code := answerIn(root, []string{"bgp"}); code != cannotRun {
		t.Errorf("exit code %d, want %d for an absent harness", code, cannotRun)
	}
}

// TestHarnessHonorsLeTestBin proves a caller-named harness is run instead of
// bin/le-test, through the le.test.bin resolver.
//
// VALIDATES: the exec path is bin/ of the checkout or LE_TEST_BIN, never PATH.
// PREVENTS: a stale harness on PATH answering for the checkout.
func TestHarnessHonorsLeTestBin(t *testing.T) {
	root := fakeHarness(t, "0")
	t.Setenv("LE_TEST_BIN", "elsewhere/le-test")
	env.ResetCache()
	if got, want := harnessPath(root), filepath.Join(root, "elsewhere", "le-test"); got != want {
		t.Errorf("harnessPath %q, want %q", got, want)
	}
}

// TestHarnessBuildUsesFeatureTags proves the harness build carries ze_test and
// every feature gate of feature-gates.txt, and writes bin/le-test (AC-7).
//
// VALIDATES: AC-7, the harness tags derive from featuretags.
// PREVENTS: a harness built without the gates the daemon ships with.
func TestHarnessBuildUsesFeatureTags(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	gates, err := os.ReadFile(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		t.Fatalf("read feature gates: %v", err)
	}
	binary := filepath.Join(root, "bin", "le-test")
	argv, err := buildArgv(root, binary)
	if err != nil {
		t.Fatalf("buildArgv: %v", err)
	}
	index := slices.Index(argv, "-tags")
	if index < 0 || index+1 >= len(argv) {
		t.Fatalf("argv %v carries no -tags", argv)
	}
	tags := strings.Fields(strings.ReplaceAll(argv[index+1], ",", " "))
	if !slices.Contains(tags, "ze_test") {
		t.Errorf("tags %v lack ze_test", tags)
	}
	for line := range strings.Lines(string(gates)) {
		gate := strings.TrimSpace(line)
		if gate == "" || strings.HasPrefix(gate, "#") {
			continue
		}
		gate = strings.Fields(gate)[0]
		if !slices.Contains(tags, gate) {
			t.Errorf("tags %v lack the feature gate %s", tags, gate)
		}
	}
	if !slices.Contains(argv, binary) || argv[len(argv)-1] != "./cmd/ze" {
		t.Errorf("argv %v does not write %s from ./cmd/ze", argv, binary)
	}
}
