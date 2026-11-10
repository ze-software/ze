// Design: docs/architecture/testing/runner-architecture.md -- which binaries a fixture reaches.
// Related: ../runner/stale.go -- the notice this fixture drives.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/test/runner"
)

// staleNoticeMark opens the notice the runner prints (runner.staleNotice).
const staleNoticeMark = "stale le:"

// staleProbeLinked is the throwaway tree's fixture file le links; the
// unlinked one sits under internal/test/ but no package of cmd/ze imports it.
const (
	staleProbeLinked   = "internal/test/linked/linked.go"
	staleProbeUnlinked = "internal/test/unlinked/unlinked.go"
)

// staleRunnerNotice runs the direct `le test runner -a` route three times in
// a throwaway checkout whose one test fails, each with a copy of the native le
// placed where the launcher or `--name` would put it:
//
//   - a launcher build of HEAD, over a clean tree: no notice;
//   - the same build after a linked and an unlinked fixture file changed: the
//     notice before the first test and again after the failing summary, with
//     the commit, the linked path alone, its count and both fixes;
//   - a `--name` build over the same changed tree: no notice.
//
// The copy is the real le, so the path from the operator's words to the notice
// is the one a stale run takes: Runner.Build, findStaleBuild, git and go list.
func staleRunnerNotice(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("runner/stale-runner-notice takes no arguments")
	}
	checkout, err := filepath.Abs(os.Getenv(envRepoRoot))
	if err != nil {
		return fmt.Errorf("resolve %s: %w", envRepoRoot, err)
	}
	here, _, err := temporaryLEFixtureWorkspace("stale-runner-notice-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture-owned scratch only

	native, err := nativeLEBinary()
	if err != nil {
		return err
	}
	tree, treeEnv, err := staleRunnerTree(ctx, here, checkout)
	if err != nil {
		return err
	}
	head, err := leChecksRunEnvironment(ctx, tree, treeEnv, "git", "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return err
	}
	sha := strings.TrimSpace(head.stdout)
	if _, isBuild := gotoolchain.ParseBuildName("20000102T120000Z-" + sha); !isBuild {
		return fmt.Errorf("git answered %q for HEAD, which is no build name's commit", sha)
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	launcherBuild := filepath.Join(tree, "bin", "builds", stamp+"-"+sha, "le")
	namedBuild := filepath.Join(tree, "bin", "le-probe", "le")
	if err := staleRunnerPlace(native, launcherBuild); err != nil {
		return err
	}
	// From the first placement, which sits on the tree's own file system.
	if err := staleRunnerPlace(launcherBuild, namedBuild); err != nil {
		return err
	}

	current, err := staleRunnerRun(ctx, tree, treeEnv, launcherBuild)
	if err != nil {
		return err
	}
	if strings.Contains(current, staleNoticeMark) {
		return fmt.Errorf("a launcher build of HEAD over a clean tree printed the notice:\n%s", current)
	}

	for _, file := range []string{staleProbeLinked, staleProbeUnlinked} {
		if err := staleRunnerWrite(tree, file, staleRunnerPackage(file)+"\n// Changed after the build.\n"); err != nil {
			return err
		}
	}
	stale, err := staleRunnerRun(ctx, tree, treeEnv, launcherBuild)
	if err != nil {
		return err
	}
	if err := staleRunnerJudge(stale, sha); err != nil {
		return err
	}

	named, err := staleRunnerRun(ctx, tree, treeEnv, namedBuild)
	if err != nil {
		return err
	}
	if strings.Contains(named, staleNoticeMark) {
		return fmt.Errorf("a --name build, which carries the working tree's code, printed the notice:\n%s", named)
	}

	fmt.Println("stale-runner-notice: named before the test and after the failing summary; silent for a clean tree and a --name build")
	return nil
}

// staleRunnerJudge checks the stale run's output: two notices around the
// failing summary, the commit, the linked path and its count, both fixes, and
// no unlinked path.
func staleRunnerJudge(output, sha string) error {
	if got := strings.Count(output, staleNoticeMark); got != 2 {
		return fmt.Errorf("the stale run printed the notice %d times, want 2 (before the test, after the failing summary):\n%s", got, output)
	}
	summary := strings.Index(output, "failed 1")
	if summary < 0 {
		return fmt.Errorf("the stale run printed no failing summary, so the probe test never ran:\n%s", output)
	}
	if strings.Index(output, staleNoticeMark) > summary {
		return fmt.Errorf("the first notice came after the summary, not before the test:\n%s", output)
	}
	if strings.LastIndex(output, staleNoticeMark) < summary {
		return fmt.Errorf("the failing summary was not followed by the notice:\n%s", output)
	}
	for _, want := range []string{
		"built from commit " + sha + ", and 1 Go file(s) it links from internal/test/ differ",
		"  " + staleProbeLinked + "\n",
		"./le test functional runner",
		"./le --name session test runner -a",
	} {
		if !strings.Contains(output, want) {
			return fmt.Errorf("the stale run's notice lacks %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, staleProbeUnlinked) {
		return fmt.Errorf("the notice names %s, which le does not link:\n%s", staleProbeUnlinked, output)
	}
	return nil
}

// staleRunnerTree writes and commits a checkout the runner accepts: a module
// whose cmd/ze links one internal/test package, a feature manifest, the
// fakefilter main Runner.Build compiles, and one runner test that fails.
func staleRunnerTree(ctx context.Context, parent, checkout string) (string, []string, error) {
	tree := filepath.Join(parent, "tree")
	files := map[string]string{
		fileGoMod:           "module fixture.invalid/stale\n\ngo 1.26\n",
		fileFeatureGates:    "ze_probe\tinternal/p0\n",
		".gitignore":        "/bin/\n/tmp/\n",
		"internal/p0/p0.go": "package p0\n",
		"cmd/ze/main.go":    "package main\n\nimport _ \"fixture.invalid/stale/internal/test/linked\"\n\nfunc main() {}\n",
		staleProbeLinked:    staleRunnerPackage(staleProbeLinked),
		staleProbeUnlinked:  staleRunnerPackage(staleProbeUnlinked),
		strings.TrimPrefix(runner.FakeFilterPackage, "./") + "/main.go": contentEmptyMain,
		"test/runner/stale-probe.ci": "# Fails on purpose, so the run ends in a failing summary.\n" +
			"option=timeout:value=20s\n\ncmd=foreground:seq=1:exec=false\nexpect=exit:code=0\n",
	}
	for relative, body := range files {
		if err := staleRunnerWrite(tree, relative, body); err != nil {
			return "", nil, err
		}
	}
	base := withoutEnv(os.Environ(), runner.EnvNoBuild, "ZE_BIN", "ZE_LE_BUILD_NAME")
	treeEnv := make([]string, 0, len(base))
	for _, entry := range base {
		if !strings.HasPrefix(entry, "GIT_") {
			treeEnv = append(treeEnv, entry)
		}
	}
	treeEnv = childEnvironment(treeEnv, map[string]string{
		envRepoRoot:        tree,
		"GOCACHE":          gotoolchain.GoCache(checkout),
		"GOFLAGS":          "",
		envGitConfigGlobal: os.DevNull,
		envGitConfigSystem: "1",
		envGitAuthorName:   gitFixtureName,
		envGitAuthorEmail:  gitFixtureEmail,
		envGitCommitName:   gitFixtureName,
		envGitCommitEmail:  gitFixtureEmail,
	})
	for _, gitArgs := range [][]string{
		{argInit, argQuiet, gitInitNoTemplate, gitInitFixtureBranch},
		{argAdd, "--", "."},
		{"-c", "core.hooksPath=" + os.DevNull, "-c", gitCommitNoSign, argCommit, argQuiet, "-m", "Stale runner fixture"},
	} {
		result, err := leChecksRunEnvironment(ctx, tree, treeEnv, "git", gitArgs...)
		if err != nil {
			return "", nil, err
		}
		if result.code != 0 {
			return "", nil, fmt.Errorf("git %v exited %d: %s%s", gitArgs, result.code, result.stdout, result.stderr)
		}
	}
	return tree, treeEnv, nil
}

// staleRunnerRun runs `<le> test runner -a` in the tree and answers stdout
// and stderr together. The probe test fails, so a non-zero exit is expected;
// the output says whether the run reached it.
func staleRunnerRun(ctx context.Context, tree string, treeEnv []string, le string) (string, error) {
	result, err := leChecksRunEnvironment(ctx, tree, treeEnv, le, "test", "runner", "-a")
	if err != nil {
		return "", err
	}
	if result.code == 0 {
		return "", fmt.Errorf("%s test runner -a passed a test written to fail:\n%s%s", le, result.stdout, result.stderr)
	}
	return result.stdout + result.stderr, nil
}

// staleRunnerPlace puts the native le at target. A hard link costs nothing on
// the same file system; a copy covers another one. os.Executable then answers
// target, which is what the runner's stale check reads.
func staleRunnerPlace(native, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
	}
	if err := os.Link(native, target); err == nil {
		return nil
	}
	source, err := os.Open(native)
	if err != nil {
		return fmt.Errorf("open native le: %w", err)
	}
	defer source.Close()                                                         //nolint:errcheck // read-only handle
	copied, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700) //nolint:gosec // fixture-owned path
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	if _, err := io.Copy(copied, source); err != nil {
		_ = copied.Close()
		return fmt.Errorf("copy native le to %s: %w", target, err)
	}
	return copied.Close()
}

// staleRunnerWrite writes one file of the tree.
func staleRunnerWrite(tree, relative, body string) error {
	target := filepath.Join(tree, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", relative, err)
	}
	return nil
}

// staleRunnerPackage is the source of a package file named by its directory.
func staleRunnerPackage(relative string) string {
	return "package " + filepath.Base(filepath.Dir(relative)) + "\n"
}
