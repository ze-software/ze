// Design: docs/architecture/testing/tracked-build-gate.md -- running Staticcheck over the matrix
//
// judge.go is the half of this gate that leaves the process: it hands the
// derived matrix to standalone Staticcheck on stdin and reads its verdict.
//
// Every way the run can end WITHOUT a verdict answers 2, apart from the tree
// failing to type-check, which answers 1. Staticcheck missing, Staticcheck
// exceeding the deadline, Staticcheck matching no package, and Staticcheck
// exiting a code that is not 1 are each "the matrix could not be judged", which
// a caller must be able to tell apart from "the matrix was judged and it
// failed".

package gostaticcheck

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// DeadlineKey is the dot-notation spelling of ZE_STATICCHECK_DEADLINE. It
// replaces the script's --deadline=D flag, which no caller passed: the tree is
// the checkout and the rendering is a pipe operator, so this command takes a
// keyword and no value of its own.
const DeadlineKey = "ze.staticcheck.deadline"

// deadlinePerRow bounds ONE row of the matrix. A run past its bound is a hung
// process rather than a slow one.
//
// The bound is per row because a run's SIZE is a choice: `check` on its own
// judges every row, a verify run judges the rows its change set can move, and a
// CI piece judges the rows dealt to it. One flat bound over all three either
// strangles the widest run or stops detecting a hang in the narrowest. The flat
// 25 minutes this replaces was set on 2026-08-17 against a matrix that reached
// 38 rows on 2026-07-24, and CI run 33450825487 measured 23m36s inside it.
//
// 90s is about two and a half times the widest cost one row has been measured
// at: 23m36s over 38 rows is 37.3s a row in CI, and a cold local run of
// 1142.63s over the same rows is 30.1s a row. That leaves a shared runner room
// to be slow and still calls a stuck Staticcheck stuck.
const deadlinePerRow = 90 * time.Second

// coldMatrixMeasured is the widest cold run measured in CI: the whole 38-row
// matrix from an empty Staticcheck cache, CI run 33450825487.
const coldMatrixMeasured = 23*time.Minute + 36*time.Second

// deadlineCold is the floor a run gets when it starts from an empty
// Staticcheck cache: two and a half times coldMatrixMeasured, the same margin
// deadlinePerRow keeps over its measurement.
//
// deadlinePerRow holds only for a cache that already answers the packages
// every row shares. Each row loads the same dependency closure under other
// tags, so a cold run pays for analysing that closure once, whatever its row
// count: the 38-row measurement amortized it, and a 6-row CI piece pays it
// alone. Verify run 41071626530 started every piece on a runner with an empty
// cache, and each one exceeded its 9m0s or 10m30s bound. A piece judges a
// subset of the matrix, so the whole matrix's cold cost bounds it, and a run
// past that bound is still hung.
const deadlineCold = coldMatrixMeasured * 5 / 2

var deadlineEntry = env.MustRegister(env.EnvEntry{
	Key:  DeadlineKey,
	Type: "duration",
	// No default value: an unset key derives the bound from the number of rows
	// the run judges, which a fixed string cannot state.
	Default:     "",
	Description: "an absolute bound on one Staticcheck run over the feature matrix; unset allows 90s per judged row, and at least 59m from an empty cache, before the matrix is declared unjudgeable",
	// Private keeps the key out of `ze env list`. It bounds a build-host tool
	// and an operator has nothing to do with it.
	Private: true,
})

// Deadline answers the bound one Staticcheck run over rows may take, and the
// reason a declared value could not be used. cold is whether the run starts
// from an empty Staticcheck cache (CacheCold).
func Deadline(rows int, cold bool) (time.Duration, error) {
	return DeadlineFrom(env.Get(deadlineEntry.Key), rows, cold)
}

// DeadlineFrom is the parse, apart from where the value came from.
//
// It is exported for the same reason DeriveScoped is: env.Get caches the whole
// environment on its first call, so a test that sets the variable afterwards
// would be reading a value that is no longer there.
//
// A declared value is ABSOLUTE and bounds the run whatever its size, because a
// person who names a duration is bounding the wall clock they are waiting on.
// An unset value scales with rows, and a cold run is never bounded below
// deadlineCold.
func DeadlineFrom(declared string, rows int, cold bool) (time.Duration, error) {
	if rows < 1 {
		var tb textbuf.Buffer
		return 0, errors.New(tb.Str("a deadline needs at least one row to judge, got ").
			Int(int64(rows)).Str("; matrix could not be judged").String())
	}
	raw := strings.TrimSpace(declared)
	if raw == "" {
		scaled := time.Duration(rows) * deadlinePerRow
		if cold {
			return max(scaled, deadlineCold), nil
		}
		return scaled, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return 0, &badDeadlineError{value: raw}
	}
	return parsed, nil
}

// badDeadlineError is a declared deadline the run cannot use. It is a type
// rather than a formatted string so the caller can answer 2 for it without
// matching on message text.
type badDeadlineError struct{ value string }

// Error states the declared value and what was wanted.
func (e *badDeadlineError) Error() string {
	var tb textbuf.Buffer
	return tb.Str(DeadlineKey).Str(" needs a positive duration, got ").
		Quoted(e.value).Str("; matrix could not be judged").String()
}

// cacheKey is the variable Staticcheck reads for the directory of its cache.
const cacheKey = "STATICCHECK_CACHE"

// CacheDir answers the Staticcheck cache the matrix over tree uses:
// cache/staticcheck-matrix, owned by this gate and never shared with an ad-hoc
// Staticcheck run. A verify worktree's cache/ links to the per-user target, so
// every worktree's matrix shares that one directory.
//
// Staticcheck trims its cache in DiskCache.Close, after the verdict exists, at
// most once a day, and records the trim in trim.txt only when the trim ends.
// The trim is a walk over the whole cache, so its cost grows with the cache.
// Over the shared ~/.cache/staticcheck, which every ad-hoc run on the machine
// fills, one trim was measured at 120s on 2026-09-24: past the deadline, so the
// run was killed after its verdict existed, trim.txt was never written, and
// every later run started the same trim again. The verdict then depended on the
// day and the disk (plan/journal/gate-verdict-depends-on-the-machine.md). A cache
// only this gate fills holds only the matrix's own entries, so the trim walks a
// bounded population and the deadline measures the type check.
//
// Staticcheck's trim keeps five days of entries, so the population is bounded
// only by le: the store trim holds both matrix caches to
// ze.le.store.staticcheck-cache-budget and `le scratch clean` cleans
// them (internal/le/scratch, cleanTargets), each sparing entries younger than
// three hours.
func CacheDir(tree string) string {
	return filepath.Join(tree, "cache", "staticcheck-matrix")
}

// CacheCold answers whether the Staticcheck cache at dir holds no entry, so a
// run over it analyses the whole dependency closure from nothing. An absent
// directory is cold. The cache keeps each entry in a two-character
// subdirectory, as Go's build cache does, and creates all of them when it
// opens, so an empty subdirectory proves nothing and only a file inside one is
// an entry. The walk reads at most one name from each subdirectory.
func CacheCold(dir string) (bool, error) {
	subdirs, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		var tb textbuf.Buffer
		return false, errors.New(tb.Str("could not read the Staticcheck cache: ").Err(err).
			Str("; matrix could not be judged").String())
	}
	for _, subdir := range subdirs {
		if !subdir.IsDir() {
			continue
		}
		held, err := holdsAnEntry(filepath.Join(dir, subdir.Name()))
		if err != nil {
			return false, err
		}
		if held {
			return false, nil
		}
	}
	return true, nil
}

// holdsAnEntry answers whether the cache subdirectory dir holds any name.
func holdsAnEntry(dir string) (bool, error) {
	f, err := os.Open(dir) //nolint:gosec // dir is a subdirectory of the gate's own cache, read by CacheCold
	if err != nil {
		var tb textbuf.Buffer
		return false, errors.New(tb.Str("could not read the Staticcheck cache: ").Err(err).
			Str("; matrix could not be judged").String())
	}
	defer f.Close() //nolint:errcheck // a read-only directory handle

	names, err := f.Readdirnames(1)
	if len(names) > 0 {
		return true, nil
	}
	if errors.Is(err, io.EOF) {
		return false, nil
	}
	var tb textbuf.Buffer
	return false, errors.New(tb.Str("could not read the Staticcheck cache: ").Err(err).
		Str("; matrix could not be judged").String())
}

// Judge runs Staticcheck over the rendered matrix in tree and answers what it
// said. It runs with CacheDir(tree) as its cache, whatever the caller's
// environment names.
//
// The bool is whether the matrix was JUDGED at all. False means the answer is
// exit code 2 and the error says why; it is not a type-check failure.
func Judge(tree string, matrix Matrix, deadline time.Duration) (Verdict, bool, error) {
	rendered, err := matrix.Render()
	if err != nil {
		return Verdict{}, false, err
	}

	cache := CacheDir(tree)
	if err := os.MkdirAll(cache, 0o750); err != nil {
		var tb textbuf.Buffer
		return Verdict{}, false, errors.New(tb.Str("could not create the Staticcheck cache: ").Err(err).
			Str("; matrix could not be judged").String())
	}

	environ, err := judgeEnvironment(os.Environ(), cache, tree)
	if err != nil {
		var tb textbuf.Buffer
		return Verdict{}, false, errors.New(tb.Str("could not set the Staticcheck environment: ").Err(err).
			Str("; matrix could not be judged").String())
	}
	if err := warmBuildCache(tree, matrix, environ); err != nil {
		return Verdict{}, false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "staticcheck", "-checks=-all", "-matrix", "./...")
	cmd.Dir = tree
	cmd.Env = environ
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = killGroup(cmd)
	cmd.Stdin = bytes.NewReader(rendered)

	var toolStdout, toolStderr bytes.Buffer
	cmd.Stdout = &toolStdout
	cmd.Stderr = &toolStderr

	if err := cmd.Start(); err != nil {
		return Verdict{}, false, startFailure(ctx, err, deadline)
	}
	runErr := cmd.Wait()

	verdict := Verdict{
		Rows:        len(matrix),
		Names:       matrix.Names(),
		Diagnostics: splitLines(toolStdout.String()),
		Tool:        toolStderr.String(),
	}

	// Staticcheck reports an empty selection on its own streams and exits 0,
	// which reads as a clean tree and is the emptiest possible false green.
	if strings.Contains(toolStdout.String(), "matched no packages") ||
		strings.Contains(toolStderr.String(), "matched no packages") {
		return verdict, false, errors.New(
			"staticcheck matched no packages; matrix could not be judged; run from a Go module containing the selected packages")
	}

	if runErr == nil {
		verdict.Passed = true
		return verdict, true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 {
		return verdict, true, nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		var tb textbuf.Buffer
		return verdict, false, errors.New(tb.Str("Staticcheck exceeded the ").Str(deadline.String()).
			Str(" deadline; matrix could not be judged, so retry after resolving the timeout").String())
	}
	if errors.As(runErr, &exitErr) {
		var tb textbuf.Buffer
		return verdict, false, errors.New(tb.Str("Staticcheck exited ").Int(int64(exitErr.ExitCode())).
			Str(" without a type-check verdict; verify that standalone Staticcheck supports -checks=-all and -matrix; matrix could not be judged").String())
	}
	var tb textbuf.Buffer
	return verdict, false, errors.New(tb.Str("Staticcheck did not complete: ").Err(runErr).
		Str("; matrix could not be judged").String())
}

// compileDeadline bounds the compile warmBuildCache runs before the Staticcheck
// deadline starts. The all-features row alone measured 495.7s cold on
// 2026-10-16, and 30 minutes is the job registry's default stall window
// (ZE_JOB_STALL_SECONDS, internal/le/job): a compile still running past it is
// hung, and the verify slot holding it can be broken anyway.
const compileDeadline = 30 * time.Minute

// warmBuildCache compiles every package and test variant of the widest row
// Staticcheck is about to load, so the Staticcheck deadline bounds the type
// check and not the compile.
//
// deadlinePerRow was sized against a whole-matrix cold run, whose one compile
// was shared by 38 rows. A verify cuts the matrix into pieces, and a change to a
// package most of the module imports gives every dependent a new action ID, so
// the first piece of the next run paid that whole compile alone: on 2026-10-16
// piece 1 exceeded 10m30s at 9858e07c8e, after f1cfbc2a60 changed
// internal/component/config. A compile is slow, not hung, so it gets its own
// bound.
//
// One row and not each: the rows differ only by feature tags, so the widest
// row compiles nearly every package the others load, and what a narrower row
// adds is its tag-gated files and their dependents, which stay inside the
// deadline as they did before the cut. A warm load still hashes the whole
// module, measured at about 26s a row on 2026-10-16, so warming all seven rows
// of a piece cost three minutes in a fresh tree whose cache was already warm.
//
// The arguments are the ones golang.org/x/tools/go/packages passes for
// Staticcheck's load mode (golistargs in go/packages/golist.go), and environ
// is the one Staticcheck runs under, so each compile here is the action
// Staticcheck would otherwise run, with the same action ID. A load that fails
// is not an answer here: Staticcheck repeats the same load and reports the
// failure as its verdict, so only the bound is an error.
func warmBuildCache(tree string, matrix Matrix, environ []string) error {
	row := widestRow(matrix)

	ctx, cancel := context.WithTimeout(context.Background(), compileDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", warmArguments(row)...) //nolint:gosec // the arguments are the fixed go/packages load flags plus the build tags of a matrix row Derive read from the checkout
	cmd.Dir = tree
	cmd.Env = environ
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = killGroup(cmd)
	cmd.Run() //nolint:errcheck // Staticcheck repeats this load and reports its failure as the verdict
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		var tb textbuf.Buffer
		return errors.New(tb.Str("compiling row ").Str(row.Name).Str(" for Staticcheck exceeded the ").
			Str(compileDeadline.String()).Str(" compile bound; matrix could not be judged").String())
	}
	return nil
}

// widestRow answers the row of matrix carrying the most feature tags, the
// first of them on a tie. Render has already refused an empty matrix.
func widestRow(matrix Matrix) Row {
	widest := matrix[0]
	for _, row := range matrix[1:] {
		if len(row.Tags) > len(widest.Tags) {
			widest = row
		}
	}
	return widest
}

// warmArguments answers the go list arguments Staticcheck's load runs for row.
// The tags are spelled as Render spells them for Staticcheck.
func warmArguments(row Row) []string {
	var tags textbuf.Buffer
	tags.Str("-tags=").Str(strings.Join(row.Tags, ","))
	return []string{"list", "-e", "-compiled=true", "-test=true", "-export=true",
		"-deps=true", "-find=false", "-buildvcs=false", "-pgo=off", tags.String(), "--", "./..."}
}

// killGroup answers the Cancel function that kills cmd's whole process group,
// so a go or Staticcheck child that outlives its deadline leaves no compiler
// behind. The command MUST set Setpgid.
func killGroup(cmd *exec.Cmd) func() error {
	return func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// judgeEnvironment answers the environment Staticcheck runs under: environ
// with the gate's own cache and with the toolchain's GOFLAGS entry, which
// carries -trimpath (gotoolchain.TrimpathGoFlags holds the measurement and
// why the deadline depends on it). The gate does not compose the flag itself:
// one declaration serves every le-driven Go command.
//
// The last entry for a key wins, so an inherited STATICCHECK_CACHE cannot
// point this run back at a shared cache. An error means Go could not say what
// its effective GOFLAGS are, and the matrix is not judged on a guess.
func judgeEnvironment(environ []string, cache, dir string) ([]string, error) {
	// Judge holds no signal root; the query keeps its own one-minute bound.
	flags, err := gotoolchain.TrimpathGoFlags(context.Background(), environ, dir)
	if err != nil {
		return nil, err
	}
	return append(slices.Clip(environ), cacheKey+"="+cache, flags), nil
}

// startFailure states why Staticcheck never ran, in the three forms a reader
// can act on: fix the clock, install the tool, or fix the executable.
func startFailure(ctx context.Context, err error, deadline time.Duration) error {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		var tb textbuf.Buffer
		return errors.New(tb.Str("Staticcheck could not start before the ").Str(deadline.String()).
			Str(" deadline; matrix could not be judged").String())
	case errors.Is(err, exec.ErrNotFound):
		return errors.New(
			"staticcheck was not found on PATH; install standalone Staticcheck, ensure it is on PATH, and retry; matrix could not be judged")
	default:
		var tb textbuf.Buffer
		return errors.New(tb.Str("could not start staticcheck: ").Err(err).
			Str("; verify the executable and PATH, then retry; matrix could not be judged").String())
	}
}

// splitLines answers one entry per non-empty line of a tool's output.
func splitLines(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}
