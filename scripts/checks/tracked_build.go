// Design: docs/architecture/testing/tracked-build-gate.md -- compile what git holds
//
// tracked_build COMPILES the repository as git holds it, which is the one
// population no other check in this repository compiles.
//
// `make ze-build`, `make ze-precommit-verify`, `make ze-lint-changed` and the test targets all
// build and run the WORKING TREE, so they see uncommitted and untracked files.
// A commit that lands a CONSUMER while its PRODUCER stays uncommitted is
// therefore green for the session that wrote it and broken for anybody who
// builds what git contains. On 2026-08-04 four commits broke `make ze-build` at HEAD
// that way in one day (7abe8a07e, 025a74b72, aa1b7a4d4, fa372140b), each with
// every gate green at the moment it was made.
//
// The mechanism: extract the commit with `git archive`, then build the
// extracted tree. `git archive` is preferred over `git worktree add --detach`
// because it writes nothing into `.git/`. A worktree registration under
// `.git/worktrees/` outlives a killed run and then needs `git worktree prune`
// by hand, and several sessions share this checkout. `git archive` also takes
// any commit-ish with no extra bookkeeping, so `--rev=<sha>` (used to reproduce
// a past break) is the same code path as the default.
//
// Usage:   CGO_ENABLED=0 go run scripts/checks/tracked_build.go [--rev=REV] [--repo=DIR]
//                                                 [--keep] [--json] [--matrix]
//                                                 [--selftest] [--package-floor=N] [--deadline=D]
// Called by: make ze-repository-tracked-build-check, and `make ze-precommit-verify` via
//            stagesForMode() in scripts/status/verify_run.go.
//
//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// tagSet is one build flavor of the module. Each is built over `./...`, not
// over its own main package alone: measured on 2026-08-04, `./...` costs about
// 2 seconds more than `./cmd/ze` and type-checks every package the tag set
// selects, including the ones no binary imports.
type tagSet struct {
	Name string
	// Tags are the literal build tags, minus the feature tags. Features says
	// whether the expansion of $(ZE_FEATURES) is appended, as the Makefile does.
	Tags     []string
	Features bool
	// GOOS pins the target operating system when the flavor's own code is
	// OS-gated. Empty means the host. This is load-bearing rather than tidiness:
	// `go build ./...` SKIPS a package whose build constraints exclude every
	// file, so a Linux-only flavor built on macOS compiles nothing and reports
	// success. See Anchor, which is what actually refuses that outcome.
	GOOS string
	// Anchor is the package this flavor exists to compile.
	Anchor string
	// AnchorFiles are files of the Anchor package that this flavor's OWN tags
	// select. Every one MUST appear in the package's GoFiles, or the flavor
	// compiled the wrong thing and the run fails.
	//
	// The package alone is not enough, and that gap is what a review caught:
	// cmd/ze/main.go carries no build constraint, so `go list ./cmd/ze` resolves
	// under ANY tag set, `-tags ze_bogus` included. A mistyped or dropped tag
	// left five of six flavors green while compiling none of the dispatch code
	// their tags exist to select. Naming a tag-gated FILE is what ties the
	// result back to the tags.
	AnchorFiles []string
	Why         string
}

// buildMatrix is a REPRESENTATIVE set, not the full matrix the Makefile can
// build, and the choice is a cost decision: about 45 seconds warm, against a
// 25-minute `make ze-precommit-verify`.
//
// Included: every flavor whose failure stops a person or the test suite from
// working -- the daemon, the functional runner, the appliance image, the two
// setup binaries, and the installer initrd.
//
// Deliberately NOT included: `ze_chaos ze_bgp`, `ze_perf ze_bgp`, `ze_analyze`
// and `ze_core ze_ssh` (bin/ze-stripped). The first three are developer tools
// whose dispatch imports nothing the daemon flavor misses; the fourth differs
// from `distro` only through `!ze_*` negations. A row costs about 3 seconds --
// add one rather than widening an existing row, if this class appears there.
var buildMatrix = []tagSet{
	{
		Name:        "distro",
		Tags:        []string{"ze_core", "ze_distro"},
		Features:    true,
		Anchor:      "./cmd/ze",
		AnchorFiles: []string{"ze_core_dispatch.go", "setup_features_distro.go"},
		Why:         "bin/ze, the daemon `make ze-build` builds. All four 2026-08-04 breaks were here",
	},
	{
		Name:        "test-runner",
		Tags:        []string{"ze_test"},
		Features:    true,
		Anchor:      "./cmd/ze",
		AnchorFiles: []string{"ze_test_register.go"},
		Why:         "bin/ze-test: a break here disables the whole functional suite",
	},
	{
		Name:        "appliance",
		Tags:        []string{"ze_core", "ze_appliance"},
		Features:    true,
		Anchor:      "./cmd/ze",
		AnchorFiles: []string{"ze_core_dispatch.go", "setup_features_appliance.go"},
		Why:         "the binary gokrazy packs into the appliance image",
	},
	{
		Name:        "setup",
		Tags:        []string{"ze_setup"},
		Anchor:      "./cmd/ze",
		AnchorFiles: []string{"setup_dispatch.go", "setup_features_setup.go"},
		Why:         "bin/ze-setup, the Makefile's ze-setup-build target: a disjoint cmd/ze dispatch",
	},
	{
		Name:   "host",
		Tags:   []string{"ze_core", "ze_setup"},
		Anchor: "./cmd/ze",
		// setup_dispatch.go is `ze_setup && !ze_core`, so this flavor takes the
		// core dispatch instead. That difference is the reason the row exists.
		AnchorFiles: []string{"ze_core_dispatch.go", "setup_features_setup.go"},
		Why:         "ze-host, the `ze appliance ...` build driver (mk/build-gokrazy.mk)",
	},
	{
		Name:        "installer",
		Tags:        []string{"ze_installer"},
		GOOS:        "linux",
		Anchor:      "./cmd/ze-installer",
		AnchorFiles: []string{"main.go"},
		Why:         "cmd/ze-installer, the installer initrd's PID 1 (linux-only)",
	},
}

// buildPackages is the pattern every flavor is built over.
const buildPackages = "./..."

// defaultPackageFloor is a SHRINK detector, not a coverage measure. `go build
// ./...` silently skips every package its constraints exclude and still exits 0,
// so a tree that lost most of its content would otherwise pass as "compiles".
// Every flavor selects about 637 packages here (measured 2026-08-04); the floor
// sits far below that so it never needs maintenance, and it is the arithmetic
// that turns "no error" into "something was compiled".
//
// It is a variable, not a constant, only so `--package-floor` can lower it for
// the gate's own fixture trees (ai/rules/evidence.md requires driving a guard
// from its entry point, and a fixture repository holds a handful of packages).
// No make target passes the flag, though `ARGS=--package-floor=N` reaches it:
// that is an operator deliberately lowering a threshold, which is visible in the
// command they typed, not a default that drifts.
var packageFloor = defaultPackageFloor

const defaultPackageFloor = 200

// runDeadline bounds the whole run. A wedged `git archive` or `go build` would
// otherwise stall a `make ze-precommit-verify` stage with no limit at all.
//
// A variable, not a constant, only so `--deadline` can shorten it for the gate's
// own tests: the INCOMPLETE path is unreachable otherwise, and an untested
// classification is how a killed build came to be reported as a broken commit.
// No make target passes the flag.
var runDeadline = defaultRunDeadline

const defaultRunDeadline = 25 * time.Minute

type buildResult struct {
	Name     string  `json:"name"`
	Tags     string  `json:"tags"`
	GOOS     string  `json:"goos,omitempty"`
	Anchor   string  `json:"anchor"`
	Packages int     `json:"packages"`
	OK       bool    `json:"ok"`
	Seconds  float64 `json:"seconds"`
	Output   string  `json:"output,omitempty"`
}

type report struct {
	Rev      string        `json:"rev"`
	Commit   string        `json:"commit"`
	Tree     string        `json:"tree"`
	Features []string      `json:"features"`
	Results  []buildResult `json:"results"`
	// PackageFloor is published so a green run pasted as evidence carries the
	// threshold it was judged against. `--package-floor` can lower it, and a
	// lowered-floor green would otherwise be indistinguishable from a real one.
	PackageFloor int `json:"package-floor"`
	// Incomplete means the run stopped before judging every flavor, so OK is a
	// statement about the flavors listed and about nothing else.
	Incomplete bool `json:"incomplete,omitempty"`
	OK         bool `json:"ok"`
}

func main() {
	repo, rev := ".", "HEAD"
	keep, jsonOut, matrixOut, selftest := false, false, false, false
	for _, a := range os.Args[1:] {
		switch {
		case strings.HasPrefix(a, "--repo="):
			repo = strings.TrimPrefix(a, "--repo=")
		case strings.HasPrefix(a, "--rev="):
			rev = strings.TrimPrefix(a, "--rev=")
		case a == "--keep":
			keep = true
		case a == "--json":
			jsonOut = true
		case a == "--matrix":
			matrixOut = true
		case a == "--selftest":
			selftest = true
		case strings.HasPrefix(a, "--deadline="):
			d, convErr := time.ParseDuration(strings.TrimPrefix(a, "--deadline="))
			if convErr != nil || d <= 0 {
				fmt.Fprintf(os.Stderr, "tracked-build: --deadline needs a positive duration, got %q\n", a)
				os.Exit(2)
			}
			runDeadline = d
		case strings.HasPrefix(a, "--package-floor="):
			n, convErr := strconv.Atoi(strings.TrimPrefix(a, "--package-floor="))
			if convErr != nil || n < 1 {
				fmt.Fprintf(os.Stderr, "tracked-build: --package-floor needs a positive integer, got %q\n", a)
				os.Exit(2)
			}
			packageFloor = n
		default:
			// Never ignore an argument we do not understand: a typo must not
			// quietly change which commit this gate judged.
			fmt.Fprintf(os.Stderr, "tracked-build: unknown argument %q\n", a)
			fmt.Fprintln(os.Stderr, "  usage: tracked_build.go [--repo=DIR] [--rev=REV] [--keep] [--json] [--matrix] [--selftest] [--package-floor=N] [--deadline=D]")
			os.Exit(2)
		}
	}

	if matrixOut {
		if err := printMatrix(); err != nil {
			fmt.Fprintf(os.Stderr, "tracked-build: %v\n", err)
			os.Exit(2)
		}
		return
	}
	if selftest {
		if err := runSelftest(); err != nil {
			fmt.Fprintf(os.Stderr, "tracked-build: SELFTEST FAILED: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, "tracked-build: selftest OK")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), runDeadline)
	defer cancel()
	code, err := run(ctx, repo, rev, keep, jsonOut)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tracked-build: %v\n", err)
		if code == 0 {
			code = 2
		}
	}
	os.Exit(code)
}

// printMatrix emits the build matrix so a test can pin a flavor's tags against
// the Makefile without re-parsing this file's source (ai/rules/evidence.md:
// drive the guard from its entry point, never from a text search over it).
func printMatrix() error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(buildMatrix); err != nil {
		return fmt.Errorf("encode matrix: %w", err)
	}
	return nil
}

func run(ctx context.Context, repo, rev string, keep, jsonOut bool) (int, error) {
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return 2, fmt.Errorf("resolve --repo %s: %w", repo, err)
	}
	commit, err := resolveRev(ctx, absRepo, rev)
	if err != nil {
		// Say WHICH failure this is. An expired deadline makes every child
		// process fail at once, and "does not name a commit" would send the
		// reader after a bad revision that is perfectly fine.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 2, fmt.Errorf("the %s deadline expired before the commit could be resolved, so nothing was judged: %w", runDeadline, ctxErr)
		}
		return 2, err
	}

	dir, err := scratchTree(ctx, absRepo)
	if err != nil {
		return 2, err
	}
	cleanup := func() {
		if keep {
			return
		}
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			fmt.Fprintf(os.Stderr, "tracked-build: could not remove %s: %v\n", dir, rmErr)
		}
	}
	// A killed run must not leave a ~180MB tree behind in a shared checkout.
	// The handler is released BEFORE the cleanup defer runs (defers unwind in
	// reverse order), so a signal arriving during cleanup cannot start a second
	// os.RemoveAll on the same path, nor os.Exit out of a running defer.
	stop := onSignal(cleanup)
	defer cleanup()
	defer stop()

	if err := extract(ctx, absRepo, commit, dir); err != nil {
		return 2, err
	}
	if err := sanityCheck(ctx, absRepo, commit, dir); err != nil {
		return 2, err
	}
	features, err := featureTags(dir)
	if err != nil {
		return 2, err
	}
	if err := setBuildCache(absRepo); err != nil {
		return 2, err
	}

	rep := report{Rev: rev, Commit: commit, Tree: dir, Features: features, PackageFloor: packageFloor, OK: true}
	var expired error
	var expiredFlavor string
	for _, ts := range buildMatrix {
		res := build(ctx, dir, ts, features)
		rep.Results = append(rep.Results, res)
		if !res.OK {
			rep.OK = false
		}
		// A flavor killed by runDeadline compiled nothing and proved nothing.
		// Its red must NOT be read as "the commit does not compile", which would
		// send somebody hunting an uncommitted producer that does not exist.
		// The report is still printed, because a flavor that failed BEFORE the
		// deadline found a real break and that finding must not be swallowed by
		// a later timeout.
		//
		// The check is UNCONDITIONAL, including after the final flavor. `build`
		// runs under exec.CommandContext, so an expired deadline means the
		// flavor just measured was killed part-way. An earlier revision skipped
		// the check on the last iteration, to avoid saying "the rest were not
		// judged" when none remained -- and thereby reported a killed final
		// build as exit 1, "the tree GIT HOLDS does not compile". Failing closed
		// on a hair-thin race is far cheaper than that.
		if ctxErr := ctx.Err(); ctxErr != nil {
			expired, expiredFlavor = ctxErr, ts.Name
			// OK is a claim about EVERY flavor. A run that judged two of six
			// has not earned it, whatever those two said.
			rep.Incomplete, rep.OK = true, false
			break
		}
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(&rep); encErr != nil {
			return 2, fmt.Errorf("encode report: %w", encErr)
		}
	} else {
		printReport(&rep, keep)
	}
	if expired != nil {
		return 2, fmt.Errorf("the %s flavor was still building when the %s deadline expired, "+
			"so the run is INCOMPLETE and its verdict is not a verdict on the commit "+
			"(any FAIL above it is still real): %w", expiredFlavor, runDeadline, expired)
	}
	if !rep.OK {
		return 1, nil
	}
	return 0, nil
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

// scratchTree returns an EMPTY directory to extract into, under this session's
// scratch dir (scripts/dev/session-scratch.sh) so two concurrent sessions never
// share one and `make ze-session-clean BEFORE=<date>` reclaims it with the rest
// of that session. Nothing under tmp/session/ is removed automatically. Never
// /tmp: a hook refuses it, and this repository keeps its scratch inside the
// checkout on purpose.
func scratchTree(ctx context.Context, repo string) (string, error) {
	base := filepath.Join(repo, "tmp")
	out, err := exec.CommandContext(ctx, filepath.Join(repo, "scripts", "dev", "session-scratch.sh")).Output()
	if err == nil {
		if rel := strings.TrimSpace(string(out)); rel != "" {
			base = filepath.Join(repo, rel)
		}
	}
	// The pid keeps two runs in ONE session apart, and the directory is cleared
	// first. `tar -x` overwrites archived paths but never removes extras, so a
	// reused non-empty directory CAN put a file back into the view that the
	// commit under test deleted.
	dir := filepath.Join(base, "tracked-build", strconv.Itoa(os.Getpid()))
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("clear scratch %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create scratch %s: %w", dir, err)
	}
	return dir, nil
}

// onSignal removes the extracted tree when the run is interrupted, then exits.
// The returned function releases the goroutine (ai/rules/goroutine-lifecycle.md:
// one long-lived worker for the process lifetime, not one per event).
func onSignal(cleanup func()) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		if _, ok := <-ch; !ok {
			return
		}
		cleanup()
		os.Exit(130)
	}()
	return func() { signal.Stop(ch); close(ch) }
}

// extract materializes the commit, streaming `git archive` straight into `tar`
// so no second copy of the archive is written.
//
// vendor/ is INCLUDED, unlike commit_helper.py's discovery-index view of HEAD
// which excludes it: this module is vendored, so a tree without vendor/ cannot
// build at all.
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
			fmt.Fprintf(os.Stderr, "tracked-build: kill git archive: %v\n", killErr)
		}
		if waitErr := archive.Wait(); waitErr != nil {
			fmt.Fprintf(os.Stderr, "tracked-build: reap git archive: %v\n", waitErr)
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

// sanityCheck fails CLOSED. A build with nothing to compile would report a clean
// commit for a tree that never existed (ai/rules/evidence.md).
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
	if tracked {
		if _, err := os.Stat(filepath.Join(dest, "vendor", "modules.txt")); err != nil {
			return fmt.Errorf("the commit tracks vendor/modules.txt but the extracted tree has none, so extraction was partial: %w", err)
		}
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

// featureTags reads feature-gates.txt FROM THE EXTRACTED TREE, so the tag set is
// the one that commit declared, expanded exactly as the Makefile expands
// $(ZE_FEATURES): `awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u`.
func featureTags(dest string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(dest, "feature-gates.txt")) //nolint:gosec // fixed in-repo path
	if err != nil {
		return nil, fmt.Errorf("read feature-gates.txt from the extracted tree: %w", err)
	}
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "ze_") {
			continue
		}
		if !seen[fields[0]] {
			seen[fields[0]] = true
			out = append(out, fields[0])
		}
	}
	sort.Strings(out)
	return out, nil
}

// setBuildCache points the extracted tree's builds at the repository's own build
// cache, so a second run of this gate is seconds rather than a minute. The
// Makefile exports it (`export GOCACHE := $(CURDIR)/cache/go-cache`), so a run
// under make inherits the value; a standalone run reconstructs the same path.
func setBuildCache(repo string) error {
	if os.Getenv("GOCACHE") != "" {
		return nil
	}
	if err := os.Setenv("GOCACHE", filepath.Join(repo, "cache", "go-cache")); err != nil {
		return fmt.Errorf("set GOCACHE: %w", err)
	}
	return nil
}

// goEnv returns the explicit CGO-free environment for one flavor's toolchain
// calls. A cross-target flavor also sets GOOS.
func goEnv(ts tagSet) []string {
	env := append(os.Environ(), "CGO_ENABLED=0")
	if ts.GOOS != "" {
		env = append(env, strings.Join([]string{"GOOS", ts.GOOS}, "="))
	}
	return env
}

// build compiles one flavor of the extracted tree, then proves the compile was
// not vacuous.
//
// -trimpath is load-bearing, not hygiene. Without it the compile action id
// carries the absolute source directory, so every run into a fresh scratch path
// is a full rebuild: measured 36s, then 36s again on the next run. With it the
// shared GOCACHE is reused across scratch directories: 36s once, then 5.7s.
// The return value is NAMED so the deferred timing assignment lands in the value
// the caller receives. With an unnamed result, `return res` copies the struct
// before the defer runs, and every flavor reported 0.0s.
func build(ctx context.Context, dest string, ts tagSet, features []string) (res buildResult) {
	tags := append(append([]string{}, ts.Tags...), tagsIf(ts.Features, features)...)
	spec := strings.Join(tags, " ")
	res = buildResult{Name: ts.Name, Tags: spec, GOOS: ts.GOOS, Anchor: ts.Anchor}
	start := time.Now()
	defer func() { res.Seconds = time.Since(start).Seconds() }()

	// No `-o`: with a wildcard pattern `go build` compiles every matched package
	// and discards the results, which is all this gate wants and writes no
	// binaries. `-o <dir>` was the first attempt and is wrong here -- it refuses
	// a pattern that matches no main package at all ("go: no main packages to
	// build"), so a library-only tree would fail for a reason that has nothing
	// to do with what was committed.
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-tags", spec, buildPackages) //nolint:gosec // spec comes from this file's table plus feature-gates.txt
	cmd.Dir = dest
	cmd.Env = goEnv(ts)
	out, err := cmd.CombinedOutput()
	res.Output = strings.TrimSpace(string(out))
	if err != nil {
		return res
	}

	// `go build ./...` exits 0 over a pattern that matched nothing buildable, so
	// "no error" is not "it compiled". These facts are what make the exit code
	// mean something.
	count, missing, anchorErr, listErr := flavorPackages(ctx, dest, ts, spec)
	res.Packages = count
	switch {
	case listErr != nil:
		res.Output = listErr.Error()
	case anchorErr != nil:
		res.Output = anchorUnresolved(ts, spec, anchorErr)
	case len(missing) != 0:
		res.Output = anchorMissing(ts, spec, missing)
	case count < packageFloor:
		res.Output = tooFewPackages(count)
	default:
		res.OK = true
	}
	return res
}

// flavorPackages counts the packages this flavor selects, and returns the
// anchor files the flavor's tags FAILED to select.
//
// The file list, not the package, is the guard. `go list` resolves a package
// whenever any one of its files survives the constraints, and cmd/ze/main.go
// carries no constraint at all, so the package resolves under every tag set --
// `-tags ze_bogus` included. Asking for a tag-gated FILE is what ties the
// answer to the flavor's own tags.
// The four results are deliberately distinct: a `./...` listing failure (the
// gate cannot judge), an anchor package that does not resolve (the commit lost a
// whole binary), the anchor files the tags failed to select (the tags no longer
// reach the flavor's own code), and the package count. Collapsing the middle two
// sent the reader hunting a build tag when the directory was simply absent.
func flavorPackages(ctx context.Context, dest string, ts tagSet, spec string) (int, []string, error, error) {
	list := exec.CommandContext(ctx, "go", "list", "-tags", spec, buildPackages) //nolint:gosec // spec comes from this file's table plus feature-gates.txt
	list.Dir = dest
	list.Env = goEnv(ts)
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

	selected, anchorErr := anchorGoFiles(ctx, dest, ts, spec)
	if anchorErr != nil {
		return count, nil, anchorErr, nil
	}
	var missing []string
	for _, want := range ts.AnchorFiles {
		if !selected[want] {
			missing = append(missing, want)
		}
	}
	return count, missing, nil, nil
}

// anchorGoFiles returns the set of Go files the flavor's tags select in its
// anchor package.
func anchorGoFiles(ctx context.Context, dest string, ts tagSet, spec string) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-tags", spec, "-f", "{{range .GoFiles}}{{.}}\n{{end}}", ts.Anchor) //nolint:gosec // spec and Anchor come from this file's table
	cmd.Dir = dest
	cmd.Env = goEnv(ts)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", ts.Anchor, err, strings.TrimSpace(stderr.String()))
	}
	files := map[string]bool{}
	for line := range strings.SplitSeq(string(out), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			files[name] = true
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s selects no Go files", ts.Anchor)
	}
	return files, nil
}

func anchorUnresolved(ts tagSet, spec string, cause error) string {
	var b strings.Builder
	b.WriteString("the flavor's anchor package did not resolve: ")
	b.WriteString(cause.Error())
	b.WriteString("\nThe commit does not hold a buildable ")
	b.WriteString(ts.Anchor)
	b.WriteString(" under -tags '")
	b.WriteString(spec)
	b.WriteString("'")
	if ts.GOOS != "" {
		b.WriteString(" on GOOS=")
		b.WriteString(ts.GOOS)
	}
	b.WriteString(".\nEither the package is absent from the commit, or nothing in it survives the constraints.")
	return b.String()
}

func anchorMissing(ts tagSet, spec string, missing []string) string {
	var b strings.Builder
	b.WriteString("the flavor compiled the wrong thing: -tags '")
	b.WriteString(spec)
	b.WriteString("'")
	if ts.GOOS != "" {
		b.WriteString(" on GOOS=")
		b.WriteString(ts.GOOS)
	}
	b.WriteString(" did not select ")
	b.WriteString(strings.Join(missing, ", "))
	b.WriteString(" in ")
	b.WriteString(ts.Anchor)
	b.WriteString(".\nThose files carry this flavor's own build tags, so the tag set no longer reaches the code it exists to compile.")
	b.WriteString("\n`go build ./...` SKIPS every file its constraints exclude and still exits 0, so this would otherwise read as success.")
	return b.String()
}

func tooFewPackages(count int) string {
	var b strings.Builder
	b.WriteString("the flavor selected ")
	b.WriteString(strconv.Itoa(count))
	b.WriteString(" packages, below the floor of ")
	b.WriteString(strconv.Itoa(packageFloor))
	b.WriteString(". The extraction or the tag set lost most of the module.")
	return b.String()
}

func tagsIf(want bool, tags []string) []string {
	if !want {
		return nil
	}
	return tags
}

func printReport(rep *report, keep bool) {
	short := rep.Commit
	if len(short) > 12 {
		short = short[:12]
	}
	floor := ""
	if rep.PackageFloor < defaultPackageFloor {
		// Say so loudly: the shrink detector was WEAKENED for this run, and a
		// green line pasted as evidence would otherwise not show it.
		floor = ", package floor LOWERED to " + strconv.Itoa(rep.PackageFloor)
	} else if rep.PackageFloor > defaultPackageFloor {
		floor = ", package floor raised to " + strconv.Itoa(rep.PackageFloor)
	}
	fmt.Fprintf(os.Stdout, "tracked-build: %s (%s), %d feature tags, %s%s\n",
		rep.Rev, short, len(rep.Features), buildPackages, floor)
	for _, r := range rep.Results {
		state := "OK  "
		if !r.OK {
			state = "FAIL"
		}
		fmt.Fprintf(os.Stdout, "  %s %-12s %5.1fs  %4d pkgs  -tags '%s'\n", state, r.Name, r.Seconds, r.Packages, r.Tags)
	}
	if rep.OK {
		fmt.Fprintln(os.Stdout, "tracked-build: OK (every flavor of the committed tree compiles)")
		return
	}
	if rep.Incomplete {
		fmt.Fprintln(os.Stderr, "\ntracked-build: INCOMPLETE. The flavors listed were judged; the rest were not.")
		fmt.Fprintln(os.Stderr, "  A FAIL below is a real break. The absence of one is not a clean commit.")
	} else {
		fmt.Fprintln(os.Stderr, "\ntracked-build: the tree GIT HOLDS does not compile.")
	}
	// Printed for an incomplete run too: a flavor that failed BEFORE the
	// deadline found a real break, and its compiler output names the symbol.
	// An earlier revision returned above this loop and lost exactly that.
	for _, r := range rep.Results {
		if r.OK {
			continue
		}
		fmt.Fprintf(os.Stderr, "\n  --- %s --- %s\n", r.Name, tagWhy(r.Name))
		for _, line := range strings.Split(r.Output, "\n") {
			fmt.Fprintf(os.Stderr, "    %s\n", line)
		}
	}
	if rep.Incomplete {
		return
	}
	fmt.Fprintln(os.Stderr, "\n  Your working tree compiles; the commit does not. The usual cause is a")
	fmt.Fprintln(os.Stderr, "  CONSUMER committed without its PRODUCER: a symbol named above still lives")
	fmt.Fprintln(os.Stderr, "  in a file that is untracked, or modified but not committed.")
	fmt.Fprintln(os.Stderr, "    git status --short          # find the file holding the named symbol")
	fmt.Fprintln(os.Stderr, "    git log -1 --stat           # what the last commit actually took")
	fmt.Fprintln(os.Stderr, "  Commit the producer. Do not revert the consumer.")
	if !keep {
		fmt.Fprintln(os.Stderr, "  To keep the extracted tree for inspection, re-run with --keep.")
	}
}

func tagWhy(name string) string {
	for _, ts := range buildMatrix {
		if ts.Name == name {
			return ts.Why
		}
	}
	return ""
}

// runSelftest proves the vacuity guards can still FAIL, before the gate is
// allowed to judge the live tree.
//
// It drives `build`, the function that CONSUMES the guards, rather than the
// helpers it calls. A selftest over the helpers alone stays green when a case is
// deleted from build's switch, which is exactly the edit that would disarm the
// gate (ai/rules/evidence.md: drive the guard from its entry point).
func runSelftest() error {
	// Under tmp/, never the system temp dir: this repository keeps its scratch
	// inside the checkout so it is visible to the operator and covered by
	// `make ze-scratch-clean`. No session sweep reclaims it; nothing is removed
	// automatically any more.
	if err := os.MkdirAll("tmp", 0o750); err != nil {
		return fmt.Errorf("create tmp/: %w", err)
	}
	dir, err := os.MkdirTemp("tmp", "tracked-build-selftest")
	if err != nil {
		return fmt.Errorf("create selftest dir: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			fmt.Fprintf(os.Stderr, "tracked-build: selftest cleanup: %v\n", rmErr)
		}
	}()

	write := func(rel, body string) error {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return fmt.Errorf("mkdir %s: %w", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
		return nil
	}
	for rel, body := range map[string]string{
		"go.mod":             "module example.invalid/selftest\n\ngo 1.21\n",
		"feature-gates.txt":  "ze_probe\tinternal/probe\n",
		"cmd/probe/main.go":  "package main\n\nfunc main() {}\n",
		"cmd/probe/gated.go": "//go:build ze_probe\n\npackage main\n\nfunc gated() {}\n",
	} {
		if err := write(rel, body); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// The floor is irrelevant to what this proves, and the fixture holds one
	// package; restore it so a selftest can never relax the live run.
	saved := packageFloor
	packageFloor = 1
	defer func() { packageFloor = saved }()

	// 1. The tags select the gated file: build must PASS. Without this the
	//    selftest could not tell a working guard from one that always fails.
	ok := tagSet{Name: "selftest-ok", Tags: []string{"ze_probe"}, Anchor: "./cmd/probe", AnchorFiles: []string{"gated.go"}}
	if res := build(ctx, dir, ok, nil); !res.OK {
		return fmt.Errorf("build refused a coherent flavor: %s", res.Output)
	}

	// 2. Same tree, a tag that selects nothing. `go build ./...` still exits 0
	//    over it, so this is the fail-open the anchor guard exists to close.
	bad := tagSet{Name: "selftest-vacuous", Tags: []string{"ze_absent"}, Anchor: "./cmd/probe", AnchorFiles: []string{"gated.go"}}
	res := build(ctx, dir, bad, nil)
	if res.OK {
		return errors.New("build accepted a flavor whose tags select none of its anchor files")
	}
	if !strings.Contains(res.Output, "gated.go") {
		return fmt.Errorf("the anchor failure does not name the unselected file: %s", res.Output)
	}

	// 3. The package floor must refuse a tree far below it.
	packageFloor = 99
	if res := build(ctx, dir, ok, nil); res.OK {
		return errors.New("build accepted a tree below the package floor")
	}
	packageFloor = 1

	// 4. featureTags must refuse a tree with no manifest rather than return an
	//    empty tag set, which would build every flavor feature-free.
	bare, err := os.MkdirTemp(dir, "bare")
	if err != nil {
		return fmt.Errorf("create selftest subdir: %w", err)
	}
	if _, err := featureTags(bare); err == nil {
		return errors.New("featureTags accepted a tree with no feature-gates.txt")
	}

	// 5. sanityCheck must refuse a tree with no go.mod.
	if err := sanityCheck(ctx, dir, "HEAD", bare); err == nil {
		return errors.New("sanityCheck accepted a tree with no go.mod")
	}

	// 6. An anchor package that does not exist at all. Deleting build's
	//    `case anchorErr != nil` leaves `missing` nil, so the switch falls
	//    through the file check, past the floor (the ./... count is real) and
	//    into `default: res.OK = true`. That is a fail-open, and this is what
	//    refuses it.
	gone := tagSet{Name: "selftest-no-anchor", Tags: []string{"ze_probe"}, Anchor: "./cmd/absent", AnchorFiles: []string{"main.go"}}
	res = build(ctx, dir, gone, nil)
	if res.OK {
		return errors.New("build accepted a flavor whose anchor package does not exist")
	}
	if !strings.Contains(res.Output, "anchor package did not resolve") {
		return fmt.Errorf("a missing anchor package was misdiagnosed: %s", res.Output)
	}

	// 7. The partial-extraction guard. It is the one branch no fixture repository
	//    can reach, because `git archive` always yields what the commit holds, so
	//    a tree missing vendor/ never arises by accident.
	//
	//    The probe repository is created HERE rather than reusing the live
	//    checkout: a selftest that read the caller's cwd, HEAD and vendor/ would
	//    fail in a shallow clone, a non-vendored checkout, or when run from
	//    another directory, none of which says anything about the guard.
	//    Skipping in those cases would be worse still: a `git ls-tree` that
	//    returned empty for a present path would then disarm the guard silently
	//    and the selftest would still report OK.
	probe := filepath.Join(dir, "probe-repo")
	if err := os.MkdirAll(filepath.Join(probe, "vendor"), 0o750); err != nil {
		return fmt.Errorf("create probe repo: %w", err)
	}
	if err := os.WriteFile(filepath.Join(probe, "go.mod"), []byte("module example.invalid/probe\n\ngo 1.21\n"), 0o600); err != nil {
		return fmt.Errorf("write probe go.mod: %w", err)
	}
	if err := os.WriteFile(filepath.Join(probe, "vendor", "modules.txt"), []byte("# probe\n"), 0o600); err != nil {
		return fmt.Errorf("write probe vendor/modules.txt: %w", err)
	}
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "--", "go.mod", "vendor/modules.txt"},
		{"commit", "--quiet", "-m", "probe"},
	} {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", probe}, args...)...) //nolint:gosec // fixed argument list
		// No global or system git config, so the probe inherits no commit
		// signing, which would fail in a throwaway repository.
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
			"GIT_AUTHOR_NAME=selftest", "GIT_AUTHOR_EMAIL=selftest@example.invalid",
			"GIT_COMMITTER_NAME=selftest", "GIT_COMMITTER_EMAIL=selftest@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("probe repo git %v: %w: %s", args, err, strings.TrimSpace(string(out)))
		}
	}

	tracked, err := commitHasPath(ctx, probe, "HEAD", "vendor/modules.txt")
	if err != nil {
		return fmt.Errorf("commitHasPath over the probe repository: %w", err)
	}
	if !tracked {
		return errors.New("commitHasPath did not find vendor/modules.txt in a commit that holds it, " +
			"so the partial-extraction guard is disarmed")
	}
	absent, err := commitHasPath(ctx, probe, "HEAD", "no/such/path.txt")
	if err != nil {
		return fmt.Errorf("commitHasPath over an absent path: %w", err)
	}
	if absent {
		return errors.New("commitHasPath reported a path the commit does not hold")
	}
	// `dir` holds go.mod but no vendor/, which is the partial extraction.
	if err := sanityCheck(ctx, probe, "HEAD", dir); err == nil {
		return errors.New("sanityCheck accepted a tree with no vendor/ against a commit that tracks it")
	}
	return nil
}
