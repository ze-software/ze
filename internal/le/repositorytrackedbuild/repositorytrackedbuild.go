// Design: docs/architecture/testing/tracked-build-gate.md -- compile what git holds
//
// Package repositorytrackedbuild COMPILES the repository as git holds it, which is the
// one population no other check in this repository compiles.
//
// The daemon build, the native verify gate, changed-file lint, and test actions
// all build and run the WORKING TREE, so they see uncommitted and untracked
// files. A commit that lands a CONSUMER while its PRODUCER stays uncommitted is
// therefore green for the session that wrote it and broken for anybody who
// builds what git contains. On 2026-08-04 four commits broke the daemon build at
// HEAD that way (7abe8a07e, 025a74b72, aa1b7a4d4, fa372140b), each with every
// gate green at the moment it was made.
//
// The mechanism: extract the commit with `git archive`, then build the
// extracted tree. `git archive` is preferred over `git worktree add --detach`
// because it writes nothing into `.git/`. A worktree registration under
// `.git/worktrees/` outlives a killed run and then needs `git worktree prune`
// by hand, and several sessions share this checkout. `git archive` also takes
// any commit-ish with no extra bookkeeping, so judging a past commit is the
// same code path as the default.

package repositorytrackedbuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

// The four run-time inputs the script took as flags. They are environment
// entries because every action of an le area takes a keyword and no value of
// its own: the tree is the checkout and the rendering is a pipe operator.
//
// RevKey carries the retained REV environment alias, so
// `REV=<sha> ./le repository-tracked-build check` keeps working.
const (
	RevKey      = "ze.tracked.build.rev"
	KeepKey     = "ze.tracked.build.keep"
	FloorKey    = "ze.tracked.build.package.floor"
	DeadlineKey = "ze.tracked.build.deadline"
)

// DefaultPackageFloor is a SHRINK detector, not a coverage measure. `go build
// ./...` silently skips every package its constraints exclude and still exits
// 0, so a tree that lost most of its content would otherwise pass as
// "compiles". Every flavor selects about 637 packages here (measured
// 2026-08-04); the floor sits far below that so it never needs maintenance, and
// it is the arithmetic that turns "no error" into "something was compiled".
const DefaultPackageFloor = 200

// DefaultDeadline bounds the whole run. A wedged `git archive` or `go build`
// would otherwise stall a pre-commit stage with no limit at all.
const DefaultDeadline = 25 * time.Minute

var revEntry = env.MustRegister(env.EnvEntry{
	Key:         RevKey,
	Type:        "string",
	Default:     "HEAD",
	Description: "the commit this gate compiles; a past sha reproduces a break that has already landed",
	// REV is retained as an environment alias so
	// `REV=<sha> ./le repository-tracked-build check` reproduces a past build.
	Aliases: []string{"REV"},
	// Private keeps the key out of `ze env list`. It names a build-host commit
	// and an operator has nothing to do with it.
	Private: true,
})

var keepEntry = env.MustRegister(env.EnvEntry{
	Key:         KeepKey,
	Type:        "bool",
	Default:     "false",
	Description: "leave the extracted tree behind so a failure can be inspected in place",
	Private:     true,
})

var floorEntry = env.MustRegister(env.EnvEntry{
	Key:         FloorKey,
	Type:        "int",
	Default:     strconv.Itoa(DefaultPackageFloor),
	Description: "the least packages a flavor must select before its clean build counts as a build",
	Private:     true,
})

var deadlineEntry = env.MustRegister(env.EnvEntry{
	Key:         DeadlineKey,
	Type:        "duration",
	Default:     DefaultDeadline.String(),
	Description: "how long the whole run may take before it is declared incomplete",
	Private:     true,
})

// Options is what one run was asked for, apart from where the answer comes
// from. Every field is a parameter rather than a package variable, so a test
// drives the same code the command runs and cannot leave a lowered floor behind
// for the next caller.
type Options struct {
	// Rev is the commit-ish to compile.
	Rev string
	// Keep leaves the extracted tree in place.
	Keep bool
	// PackageFloor is the shrink detector's threshold.
	PackageFloor int
	// Deadline bounds the whole run.
	Deadline time.Duration
}

// defaultOptions answers what the environment asked for, and the reason a
// declared value could not be used.
//
// env.Get resolves an alias to its canonical key first and falls back to the
// spelling it was handed, so asking by the ALIAS reads ZE_TRACKED_BUILD_REV and
// then bare REV, in that order.
func defaultOptions() (Options, error) {
	return optionsFrom(
		env.Get(revEntry.Aliases[0]),
		env.Get(keepEntry.Key),
		env.Get(floorEntry.Key),
		env.Get(deadlineEntry.Key),
	)
}

// optionsFrom is the parse, apart from where the values came from.
//
// The split keeps environment reads out of tests: internal/core/env caches the
// whole environment on its first read, so a test that sets a variable afterwards
// would be reading a value that is no longer there. An empty string means the
// field keeps its default.
func optionsFrom(rev, keep, floor, deadline string) (Options, error) {
	options := Options{Rev: "HEAD", PackageFloor: DefaultPackageFloor, Deadline: DefaultDeadline}

	if trimmed := strings.TrimSpace(rev); trimmed != "" {
		options.Rev = trimmed
	}
	switch strings.ToLower(strings.TrimSpace(keep)) {
	case "", "false", "0", "no", "off":
	case "true", "1", "yes", "on":
		options.Keep = true
	default:
		var tb textbuf.Buffer
		return Options{}, errors.New(tb.Str(KeepKey).Str(" needs a boolean, got ").Quoted(keep).String())
	}

	if trimmed := strings.TrimSpace(floor); trimmed != "" {
		parsed, err := strconv.Atoi(trimmed)
		if err != nil || parsed < 1 {
			var tb textbuf.Buffer
			return Options{}, errors.New(tb.Str(FloorKey).Str(" needs a positive integer, got ").Quoted(floor).String())
		}
		options.PackageFloor = parsed
	}
	if trimmed := strings.TrimSpace(deadline); trimmed != "" {
		parsed, err := time.ParseDuration(trimmed)
		if err != nil || parsed <= 0 {
			var tb textbuf.Buffer
			return Options{}, errors.New(tb.Str(DeadlineKey).Str(" needs a positive duration, got ").Quoted(deadline).String())
		}
		options.Deadline = parsed
	}
	return options, nil
}

// Run extracts the commit and compiles every flavor of it.
//
// The int is the exit code a caller answers: 0 for a commit that compiles, 1
// for one that does not, and 2 for a run that could not judge it. Those three
// stay apart because a killed build and a broken commit send a reader after
// completely different things.
func Run(ctx context.Context, repo string, options Options) (Report, int, error) {
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Report{}, 2, fmt.Errorf("resolve the repository %s: %w", repo, err)
	}
	commit, err := resolveRev(ctx, absRepo, options.Rev)
	if err != nil {
		// Say WHICH failure this is. An expired deadline makes every child
		// process fail at once, and "does not name a commit" would send the
		// reader after a bad revision that is perfectly fine.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Report{}, 2, fmt.Errorf(
				"the %s deadline expired before the commit could be resolved, so nothing was judged: %w",
				options.Deadline, ctxErr)
		}
		return Report{}, 2, err
	}

	dir, err := scratchTree(ctx, absRepo)
	if err != nil {
		return Report{}, 2, err
	}
	// A killed run must not leave a ~180MB tree behind in a shared checkout.
	defer func() {
		if options.Keep {
			return
		}
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			var tb textbuf.Buffer
			fmt.Fprintln(os.Stderr, tb.Str("tracked-build: could not remove ").Str(dir).Str(": ").Err(rmErr).String()) //nolint:errcheck // CLI output
		}
	}()

	if err := extract(ctx, absRepo, commit, dir); err != nil {
		return Report{}, 2, err
	}
	if err := sanityCheck(ctx, absRepo, commit, dir); err != nil {
		return Report{}, 2, err
	}
	features, err := featureTags(dir)
	if err != nil {
		return Report{}, 2, err
	}
	if err := setBuildCache(absRepo); err != nil {
		return Report{}, 2, err
	}

	report := Report{
		Rev: options.Rev, Commit: commit, Tree: dir, Features: features,
		PackageFloor: options.PackageFloor, Keep: options.Keep, OK: true,
	}

	var expired error
	var expiredFlavor string
	for _, flavor := range buildMatrix {
		result := Build(ctx, dir, flavor, features, options.PackageFloor)
		report.Results = append(report.Results, result)
		if !result.OK {
			report.OK = false
		}
		// A flavor killed by the deadline compiled nothing and proved nothing.
		// Its red must NOT be read as "the commit does not compile", which
		// would send somebody hunting an uncommitted producer that does not
		// exist. The report is still answered, because a flavor that failed
		// BEFORE the deadline found a real break and that finding must not be
		// swallowed by a later timeout.
		//
		// The check is UNCONDITIONAL, including after the final flavor. Build
		// runs under exec.CommandContext, so an expired deadline means the
		// flavor just measured was killed part-way. An earlier revision skipped
		// the check on the last iteration and thereby reported a killed final
		// build as "the tree GIT HOLDS does not compile". Failing closed on a
		// hair-thin race is far cheaper than that.
		if ctxErr := ctx.Err(); ctxErr != nil {
			expired, expiredFlavor = ctxErr, flavor.Name
			// OK is a claim about EVERY flavor. A run that judged two of six
			// has not earned it, whatever those two said.
			report.Incomplete, report.OK = true, false
			break
		}
	}

	if expired != nil {
		return report, 2, fmt.Errorf(
			"the %s flavor was still building when the %s deadline expired, "+
				"so the run is INCOMPLETE and its verdict is not a verdict on the commit "+
				"(any FAIL above it is still real): %w", expiredFlavor, options.Deadline, expired)
	}
	if !report.OK {
		return report, 1, nil
	}
	return report, 0, nil
}

// resolveRev turns a commit-ish into a full sha.
//
// `git rev-list -n 1` is used rather than `rev-parse` for two reasons. It
// resolves to a commit and needs no `^{commit}` suffix. It also fails on a
// repository with no commits, which MUST be an error here: an empty build is a
// passing build.
func resolveRev(ctx context.Context, repo, rev string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", repo, "rev-list", "-n", "1", rev).Output() //nolint:gosec // rev is an operator-supplied commit-ish
	if err != nil {
		return "", fmt.Errorf("%s does not name a commit in %s", rev, repo)
	}
	commit := strings.TrimSpace(string(out))
	if commit == "" {
		return "", fmt.Errorf("%s does not name a commit in %s", rev, repo)
	}
	return commit, nil
}

// scratchTree answers an EMPTY directory to extract into, under this session's
// scratch directory so two concurrent sessions never share one. Never the
// system temp directory: this repository keeps its scratch inside the checkout
// on purpose.
func scratchTree(ctx context.Context, repo string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("resolve tracked-build scratch: %w", err)
	}
	paths, err := lepath.ResolveSession(repo, true)
	if err != nil {
		return "", fmt.Errorf("resolve tracked-build scratch: %w", err)
	}

	// The pid keeps two runs in ONE session apart, and the directory is cleared
	// first. `tar -x` overwrites archived paths but never removes extras, so a
	// reused non-empty directory CAN put a file back into the view that the
	// commit under test deleted.
	dir := filepath.Join(repo, paths.Scratch, "tracked-build", strconv.Itoa(os.Getpid()))
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("clear scratch %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create scratch %s: %w", dir, err)
	}
	return dir, nil
}

// extract materializes the commit, streaming `git archive` straight into `tar`
// so no second copy of the archive is written.
//
// vendor/ is INCLUDED: this module is vendored, so a tree without vendor/
// cannot build at all.
func extract(ctx context.Context, repo, commit, dest string) error {
	archive := exec.CommandContext(ctx, "git", "-C", repo, "archive", "--format=tar", commit) //nolint:gosec // commit is a resolved sha
	untar := exec.CommandContext(ctx, "tar", "-xf", "-", "-C", dest)                          //nolint:gosec // dest is ours

	pipe, err := archive.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe git archive: %w", err)
	}
	untar.Stdin = pipe
	var archiveErr, untarErr strings.Builder
	archive.Stderr = &archiveErr
	untar.Stderr = &untarErr

	if err := archive.Start(); err != nil {
		return fmt.Errorf("start git archive: %w", err)
	}
	if err := untar.Start(); err != nil {
		if killErr := archive.Process.Kill(); killErr != nil {
			var tb textbuf.Buffer
			fmt.Fprintln(os.Stderr, tb.Str("tracked-build: kill git archive: ").Err(killErr).String()) //nolint:errcheck // CLI output
		}
		if waitErr := archive.Wait(); waitErr != nil {
			var tb textbuf.Buffer
			fmt.Fprintln(os.Stderr, tb.Str("tracked-build: reap git archive: ").Err(waitErr).String()) //nolint:errcheck // CLI output
		}
		return fmt.Errorf("start tar: %w", err)
	}

	// tar is waited on first: it is the reader. Then the pipe is drained --
	// THIS PROCESS still holds its read end, so a tar that died early would not
	// give `git archive` an EPIPE and it would block on a full pipe forever.
	tarWait := untar.Wait()
	if _, drainErr := io.Copy(io.Discard, pipe); drainErr != nil && tarWait == nil {
		return fmt.Errorf("drain git archive output: %w", drainErr)
	}
	gitWait := archive.Wait()

	if gitWait != nil {
		return fmt.Errorf("git archive: %w: %s", gitWait, strings.TrimSpace(archiveErr.String()))
	}
	if tarWait != nil {
		return fmt.Errorf("tar -x: %w: %s", tarWait, strings.TrimSpace(untarErr.String()))
	}
	return nil
}

// sanityCheck fails CLOSED. A build with nothing to compile would report a
// clean commit for a tree that never existed.
//
// What the COMMIT owes is asked of GIT, never of the working tree: the working
// tree is the population this gate exists to distrust, and a vendor/ that is
// present on disk but absent from the commit is exactly the shape of break
// being hunted.
func sanityCheck(ctx context.Context, repo, commit, dest string) error {
	if _, err := os.Stat(filepath.Join(dest, "go.mod")); err != nil {
		return fmt.Errorf("the extracted tree has no go.mod, so nothing would be compiled: %w", err)
	}
	tracked, err := commitHasPath(ctx, repo, commit, "vendor/modules.txt")
	if err != nil {
		return err
	}
	if !tracked {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dest, "vendor", "modules.txt")); err != nil {
		return fmt.Errorf("the commit tracks vendor/modules.txt but the extracted tree has none, so extraction was partial: %w", err)
	}
	return nil
}

// commitHasPath reports whether the commit's tree holds a path.
//
// `git ls-tree` rather than `git cat-file -e`, because the two report absence
// differently: `cat-file -e <sha>:<absent path>` exits 128, the same code it
// uses for a corrupt repository or a bad revision, so absence and breakage are
// indistinguishable. `ls-tree` exits 0 and prints nothing for an absent path,
// and reserves a nonzero exit for a real failure. That distinction matters: an
// error read as "absent" would SKIP the partial-extraction check above and
// quietly restore the fail-open this guard exists to close.
func commitHasPath(ctx context.Context, repo, commit, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "ls-tree", "--name-only", commit, "--", path) //nolint:gosec // commit is a resolved sha, path is a constant
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git ls-tree %s -- %s: %w: %s", commit, path, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// featureTags reads the feature manifest FROM THE EXTRACTED TREE, so the native
// toolchain expands the tag set that commit declared.
func featureTags(dest string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(dest, "feature-gates.txt")) //nolint:gosec // fixed in-repo path
	if err != nil {
		return nil, fmt.Errorf("read feature-gates.txt from the extracted tree: %w", err)
	}
	seen := map[string]bool{}
	var out []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "ze_") || seen[fields[0]] {
			continue
		}
		seen[fields[0]] = true
		out = append(out, fields[0])
	}
	sort.Strings(out)
	return out, nil
}

// setBuildCache points the extracted tree's builds at the repository's own
// build cache, so a second run of this gate is seconds rather than a minute. A
// caller-supplied GOCACHE wins; otherwise this function reconstructs the native
// default path.
func setBuildCache(repo string) error {
	if os.Getenv("GOCACHE") != "" {
		return nil
	}
	if err := os.Setenv("GOCACHE", filepath.Join(repo, "cache", "go-cache")); err != nil {
		return fmt.Errorf("set GOCACHE: %w", err)
	}
	return nil
}

// goEnv answers the explicit CGO-free environment for one flavor's toolchain
// calls. A cross-target flavor also sets GOOS.
func goEnv(flavor Flavor) []string {
	environ := append(os.Environ(), "CGO_ENABLED=0")
	if flavor.GOOS != "" {
		var tb textbuf.Buffer
		environ = append(environ, tb.Str("GOOS=").Str(flavor.GOOS).String())
	}
	return environ
}

// Build compiles one flavor of the extracted tree, then proves the compile was
// not vacuous.
//
// -trimpath is load-bearing, not hygiene. Without it the compile action id
// carries the absolute source directory, so every run into a fresh scratch path
// is a full rebuild: measured 36s, then 36s again on the next run. With it the
// shared build cache is reused across scratch directories: 36s once, then 5.7s.
//
// The return value is NAMED so the deferred timing assignment lands in the
// value the caller receives. With an unnamed result, `return result` copies the
// struct before the defer runs, and every flavor reported 0.0s.
func Build(ctx context.Context, dest string, flavor Flavor, features []string, floor int) (result Result) {
	tags := append(append([]string{}, flavor.Tags...), tagsIf(flavor.Features, features)...)
	spec := strings.Join(tags, " ")
	result = Result{Name: flavor.Name, Tags: spec, GOOS: flavor.GOOS, Anchor: flavor.Anchor}
	start := time.Now()
	defer func() { result.Seconds = time.Since(start).Seconds() }()

	// No `-o`: with a wildcard pattern `go build` compiles every matched
	// package and discards the results, which is all this gate wants and writes
	// no binaries. `-o <dir>` was the first attempt and is wrong here -- it
	// refuses a pattern that matches no main package at all ("go: no main
	// packages to build"), so a library-only tree would fail for a reason that
	// has nothing to do with what was committed.
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-tags", spec, buildPackages) //nolint:gosec // spec comes from this file's table plus the feature manifest
	cmd.Dir = dest
	cmd.Env = goEnv(flavor)
	out, err := cmd.CombinedOutput()
	result.Output = strings.TrimSpace(string(out))
	if err != nil {
		return result
	}

	// `go build ./...` exits 0 over a pattern that matched nothing buildable,
	// so "no error" is not "it compiled". These facts are what make the exit
	// code mean something.
	count, missing, anchorErr, listErr := flavorPackages(ctx, dest, flavor, spec)
	result.Packages = count
	switch {
	case listErr != nil:
		result.Output = listErr.Error()
	case anchorErr != nil:
		result.Output = anchorUnresolved(flavor, spec, anchorErr)
	case len(missing) != 0:
		result.Output = anchorMissing(flavor, spec, missing)
	case count < floor:
		result.Output = tooFewPackages(count, floor)
	default:
		result.OK = true
	}
	return result
}

// flavorPackages counts the packages this flavor selects, and answers the
// anchor files the flavor's tags FAILED to select.
//
// The file list, not the package, is the guard. `go list` resolves a package
// whenever any one of its files survives the constraints, and cmd/ze/main.go
// carries no constraint at all, so the package resolves under every tag set --
// `-tags ze_bogus` included. Asking for a tag-gated FILE is what ties the
// answer to the flavor's own tags.
//
// The four results are deliberately distinct: a `./...` listing failure (the
// gate cannot judge), an anchor package that does not resolve (the commit lost
// a whole binary), the anchor files the tags failed to select (the tags no
// longer reach the flavor's own code), and the package count. Collapsing the
// middle two sent the reader hunting a build tag when the directory was simply
// absent.
func flavorPackages(ctx context.Context, dest string, flavor Flavor, spec string) (int, []string, error, error) {
	list := exec.CommandContext(ctx, "go", "list", "-tags", spec, buildPackages) //nolint:gosec // spec comes from this file's table plus the feature manifest
	list.Dir = dest
	list.Env = goEnv(flavor)
	var listErr strings.Builder
	list.Stderr = &listErr
	out, err := list.Output()
	if err != nil {
		return 0, nil, nil, fmt.Errorf("go list %s: %w: %s", buildPackages, err, strings.TrimSpace(listErr.String()))
	}

	count := 0
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	selected, anchorErr := anchorGoFiles(ctx, dest, flavor, spec)
	if anchorErr != nil {
		return count, nil, anchorErr, nil
	}
	var missing []string
	for _, want := range flavor.AnchorFiles {
		if !selected[want] {
			missing = append(missing, want)
		}
	}
	return count, missing, nil, nil
}

// anchorGoFiles answers the set of Go files the flavor's tags select in its
// anchor package.
func anchorGoFiles(ctx context.Context, dest string, flavor Flavor, spec string) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-tags", spec, "-f", "{{range .GoFiles}}{{.}}\n{{end}}", flavor.Anchor) //nolint:gosec // spec and Anchor come from this file's table
	cmd.Dir = dest
	cmd.Env = goEnv(flavor)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", flavor.Anchor, err, strings.TrimSpace(stderr.String()))
	}

	files := map[string]bool{}
	for line := range strings.SplitSeq(string(out), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			files[name] = true
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s selects no Go files", flavor.Anchor)
	}
	return files, nil
}

// anchorUnresolved states a flavor whose anchor package is not in the commit at
// all.
func anchorUnresolved(flavor Flavor, spec string, cause error) string {
	var tb textbuf.Buffer
	tb.Str("the flavor's anchor package did not resolve: ").Err(cause)
	tb.Str("\nThe commit does not hold a buildable ").Str(flavor.Anchor).
		Str(" under -tags '").Str(spec).Byte('\'')
	if flavor.GOOS != "" {
		tb.Str(" on GOOS=").Str(flavor.GOOS)
	}
	tb.Str(".\nEither the package is absent from the commit, or nothing in it survives the constraints.")
	return tb.String()
}

// anchorMissing states a flavor whose tags no longer select its own code.
func anchorMissing(flavor Flavor, spec string, missing []string) string {
	var tb textbuf.Buffer
	tb.Str("the flavor compiled the wrong thing: -tags '").Str(spec).Byte('\'')
	if flavor.GOOS != "" {
		tb.Str(" on GOOS=").Str(flavor.GOOS)
	}
	tb.Str(" did not select ").Join(missing, ", ").Str(" in ").Str(flavor.Anchor)
	tb.Str(".\nThose files carry this flavor's own build tags, so the tag set no longer reaches the code it exists to compile.")
	tb.Str("\n`go build ./...` SKIPS every file its constraints exclude and still exits 0, so this would otherwise read as success.")
	return tb.String()
}

// tooFewPackages states a tree that lost most of the module.
func tooFewPackages(count, floor int) string {
	var tb textbuf.Buffer
	return tb.Str("the flavor selected ").Int(int64(count)).Str(" packages, below the floor of ").
		Int(int64(floor)).Str(". The extraction or the tag set lost most of the module.").String()
}

// tagsIf answers the feature tags when the flavor takes them, and nothing
// otherwise.
func tagsIf(want bool, tags []string) []string {
	if !want {
		return nil
	}
	return tags
}
