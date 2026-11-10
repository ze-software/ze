// Design: docs/architecture/testing/runner-architecture.md -- which binaries a fixture reaches
// Related: runner.go -- Runner.Build finds a stale build, Runner.Run prints the notice

package runner

import (
	"context"
	"errors"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// A direct `le test <suite>` run executes every `le` exec head, compiled
// fixtures included, with the runner's own executable (Runner.lePath). When
// that executable is a launcher build under bin/builds/<time>-<sha>/, its
// fixture code is commit <sha>'s, never the working tree's, so an uncommitted
// fixture edit is not what runs. The notice says so before the first test and
// again under a failing summary. It changes no verdict and no exit code.

// staleLinkedRoot is the tree whose linked Go files decide whether a build is
// stale: the functional harness and its fixtures live under it.
const staleLinkedRoot = "internal/test/"

// stalePathsMax bounds the paths the notice prints; the rest are counted.
const stalePathsMax = 10

// staleCheckTimeout bounds the git and go commands of one check, so a hung
// command delays a run by at most this much.
const staleCheckTimeout = 60 * time.Second

// leBuildTag is the tag the launcher adds to the feature gates when it builds
// le from ./cmd/ze (the `le` script, `tags=ze_le,...`).
const leBuildTag = "ze_le"

// staleSource answers what the stale check needs from the checkout. The
// runner's own is checkoutStaleSource; a test replaces it.
type staleSource interface {
	// changedGoFiles answers the repo-relative, slash-separated non-test Go
	// files under staleLinkedRoot that differ between commit sha and the
	// working tree: committed and uncommitted changes, deletions, and
	// untracked files git does not ignore.
	changedGoFiles(ctx context.Context, root, sha string) ([]string, error)
	// linkedTestPackages answers the repo-relative, slash-separated
	// directories under staleLinkedRoot of the packages le links.
	linkedTestPackages(ctx context.Context, root string) (map[string]bool, error)
}

// staleCheck is what the stale check found to say. The zero value has nothing
// to say; Runner.stale holds nil for it.
type staleCheck struct {
	sha     string   // the commit the running le was built from
	changed []string // sorted linked Go files that differ from it
	err     error    // the check could not answer; changed is then empty
}

// findStaleBuild answers what to tell the operator about the le at
// executable, nil when it carries the working tree's fixture code. Only a
// launcher build (bin/builds/<time>-<sha>/le) can be stale. A `--name` build
// (bin/le-<name>/le) and the isolated set `./le test functional` builds are
// built from the working tree, and their directory names are not build names.
func findStaleBuild(ctx context.Context, executable, root string, source staleSource) *staleCheck {
	build, isBuild := gotoolchain.ParseBuildName(filepath.Base(filepath.Dir(executable)))
	if !isBuild {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, staleCheckTimeout)
	defer cancel()

	changed, err := source.changedGoFiles(ctx, root, build.Sha())
	if err != nil {
		return &staleCheck{sha: build.Sha(), err: err}
	}
	if len(changed) == 0 {
		return nil
	}
	linked, err := source.linkedTestPackages(ctx, root)
	if err != nil {
		return &staleCheck{sha: build.Sha(), err: err}
	}
	stale := make([]string, 0, len(changed))
	for _, file := range changed {
		if linked[path.Dir(file)] {
			stale = append(stale, file)
		}
	}
	if len(stale) == 0 {
		return nil
	}
	slices.Sort(stale)
	return &staleCheck{sha: build.Sha(), changed: stale}
}

// staleNotice renders the notice for check. suite is the functional suite
// name; invocation is the words the operator gave le after `./le`.
func staleNotice(check *staleCheck, colors *Colors, suite string, invocation []string) string {
	var b textbuf.Buffer
	if check.err != nil {
		b.Str(colors.Yellow("le may be stale:")).Byte(' ')
		b.Str("this le was built from commit ").Str(check.sha).
			Str(", and it could not check whether the Go files it links from ").Str(staleLinkedRoot).
			Str(" differ in the working tree: ").Str(check.err.Error()).Byte('\n')
	} else {
		b.Str(colors.Yellow("stale le:")).Byte(' ')
		b.Str("this le was built from commit ").Str(check.sha).Str(", and ").Int(int64(len(check.changed))).
			Str(" Go file(s) it links from ").Str(staleLinkedRoot).Str(" differ in the working tree:\n")
		for _, file := range check.changed[:min(len(check.changed), stalePathsMax)] {
			b.Str("  ").Str(file).Byte('\n')
		}
		if len(check.changed) > stalePathsMax {
			b.Str("  and ").Int(int64(len(check.changed) - stalePathsMax)).Str(" more\n")
		}
	}
	b.Str("The fixtures this run executes are that commit's code, not the working tree's, so a result can contradict your edit.\n")
	b.Str("Run the tests with an le built from the working tree:\n")
	b.Str("  ").Str(colors.Cyan("./le test functional " + suite)).Byte('\n')
	b.Str("  ").Str(colors.Cyan("./le --name session " + strings.Join(invocation, " ")))
	return b.String()
}

// checkoutStaleSource answers the stale check from git and the go command in
// the checkout. Not safe for concurrent use with an edit of the checkout, which
// is the condition it reports.
type checkoutStaleSource struct{}

// changedGoFiles runs `git diff --name-only <sha>` (the commit against the
// working tree, which covers committed and uncommitted changes) and `git
// ls-files --others --exclude-standard` (new files git does not ignore), both
// limited to staleLinkedRoot.
func (checkoutStaleSource) changedGoFiles(ctx context.Context, root, sha string) ([]string, error) {
	diffed, err := staleGit(ctx, root, "diff", "--name-only", "--no-renames", sha, "--", staleLinkedRoot)
	if err != nil {
		return nil, err
	}
	untracked, err := staleGit(ctx, root, "ls-files", "--others", "--exclude-standard", "--", staleLinkedRoot)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(diffed)+len(untracked))
	for _, file := range slices.Concat(diffed, untracked) {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		if strings.HasSuffix(file, ".go") {
			files = append(files, file)
		}
	}
	slices.Sort(files)
	return slices.Compact(files), nil
}

// linkedTestPackages runs `go list -deps` over ./cmd/ze with the tags the
// launcher builds le with, and keeps the directories under staleLinkedRoot.
// It runs only when a Go file there changed, so a run over a clean tree pays
// for git alone.
func (checkoutStaleSource) linkedTestPackages(ctx context.Context, root string) (map[string]bool, error) {
	tags, err := repofeaturetags.DaemonBuildTags(root, leBuildTag)
	if err != nil {
		return nil, err
	}
	// The go command reports directories under its working directory, read
	// from PWD when PWD names it and from getcwd otherwise. Running it in the
	// checkout's real path with PWD set to that path makes every directory it
	// reports comparable with realRoot, whatever symlink root holds (macOS
	// /tmp, a linked home directory).
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, staleCommandError("resolve the checkout path", err)
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-tags", tags, "-f", "{{.Dir}}", "./cmd/ze") //nolint:gosec // fixed program, tags from the checkout's manifest
	cmd.Dir = realRoot
	cmd.Env = childEnv("PWD=" + realRoot)
	output, err := cmd.Output()
	if err != nil {
		return nil, staleCommandError("go list -deps ./cmd/ze", err)
	}
	linked := make(map[string]bool)
	for _, dir := range staleLines(output) {
		// A directory outside the checkout (the standard library, a module
		// cache) answers a path that opens with "../" and matches nothing.
		relative, err := filepath.Rel(realRoot, dir)
		if err != nil {
			return nil, staleCommandError("go list -deps ./cmd/ze", err)
		}
		relative = filepath.ToSlash(relative)
		if strings.HasPrefix(relative+"/", staleLinkedRoot) {
			linked[relative] = true
		}
	}
	return linked, nil
}

// staleGit runs one git command in root and answers its output lines.
func staleGit(ctx context.Context, root string, args ...string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...) //nolint:gosec // fixed program, arguments from the runner
	cmd.Env = childEnv()
	output, err := cmd.Output()
	if err != nil {
		return nil, staleCommandError("git "+args[0], err)
	}
	return staleLines(output), nil
}

// staleLines answers the non-empty lines of a command's output. A path may
// hold a space, so the output is split on line ends only.
func staleLines(output []byte) []string {
	var lines []string
	for line := range strings.Lines(string(output)) {
		if trimmed := strings.TrimRight(line, "\r\n"); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// staleCommandError names the command that failed and what it printed.
func staleCommandError(command string, err error) error {
	var b textbuf.Buffer
	b.Str(command).Str(" failed: ").Str(err.Error())
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		b.Str(": ").Str(strings.TrimSpace(string(exitErr.Stderr)))
	}
	return errors.New(b.String())
}
