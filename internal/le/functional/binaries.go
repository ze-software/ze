// Design: docs/architecture/testing/ci-format.md -- the binaries a suite runs against
// Detail: suites.go -- what each of those binaries is asked to run
//
// Each invocation builds an isolated throwaway binary set by default.
// The set contains ze, ze-test, and ze-stripped under <scratch>/testbin-<suffix>/bin/.
// A suite runs against this frozen set while the developer continues other builds and edits.
//
// The binaries use canonical names because .ci tests execute `ze` and `ze-stripped` by name.
// The binaries live in a bin/ subdirectory because ze derives its directories from its location.
// Ze recognizes only a parent named bin or sbin (internal/core/paths/paths.go, isBinDir).
// Without that parent, `ze config archive` answers "cannot determine database location".
//
//	ZE_SUFFIX=<name>     selects a stable directory.
//	                     The runner KEEPS the directory on exit.
//	                     Two runs with this name share the session's etc/ze
//	                     directory and corrupt each other's test database.
//	                     Use it for one serial run that you want to keep.
//	                     Use the default for concurrent runs.
//
//	ZE_TEST_CANONICAL=1  runs the session's own ze-test in place.
//	                     Use this mode for release and CI reproducibility.
//
//	ZE_COVER=1           record which Go packages each suite EXECUTES. The DUT
//	                     binaries are built -cover and each suite gets its own
//	                     GOCOVERDIR. THE PATH MUST BE ABSOLUTE: a .ci that
//	                     declares tmpfs= runs with proc.Dir set to the per-test
//	                     directory, so a relative root resolves against THAT
//	                     directory and the emit fails silently.
//
// ze-test itself is deliberately NOT instrumented: it is the harness, not the
// subject, and what it executed is not what the map is about.

package functional

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

var (
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.suffix",
		Type:        envString,
		Default:     "",
		Description: "a stable name for the throwaway binary set, which is then kept on exit",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.test.canonical",
		Type:        envBool,
		Default:     "false",
		Description: "run the session's own ze-test in place instead of an isolated set",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.cover",
		Type:        envBool,
		Default:     "false",
		Description: "record which Go packages each suite executes, one GOCOVERDIR per suite",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.scratch.dir",
		Type:        envString,
		Default:     "",
		Description: "this session's scratch directory, when a caller has already resolved it",
		Private:     true,
	})
)

// BinarySet is where the binaries a run executes live, and whether they are
// throwaway.
type BinarySet struct {
	// Dir is the bin/ directory itself, which is what ze reads its own
	// location from.
	Dir string
	// Remove says this set is this invocation's own and is deleted on exit.
	Remove bool
	// Canonical says that this is the session's binary set, not a copy.
	// Thus, the runner rebuilds the set in place.
	Canonical bool
}

// ZeTestPath is the harness binary a suite is run through.
func (b BinarySet) ZeTestPath() string { return filepath.Join(b.Dir, ZeTest) }

// Environment is the Go environment plus what freezes the runner against this
// set. A canonical run adds nothing.
func (b BinarySet) Environment(tc gotoolchain.Toolchain) []string {
	base := tc.Environment(gotoolchain.EnvOptions{})
	if b.Canonical {
		return base
	}
	// Append instead of replace because os/exec uses the last duplicate key.
	// gotoolchain.Environment uses the same rule for its inherited environment overrides.
	var tb textbuf.Buffer
	base = append(base, "ZE_TEST_NO_BUILD=1",
		tb.Str("ZE_BIN=").Str(filepath.Join(b.Dir, "ze")).String())
	tb.Reset()
	return append(base, tb.Str("ZE_TEST_BIN=").Str(b.ZeTestPath()).String())
}

// ScratchDir answers this session's own directory, or tmp off-session.
//
// LOOKED UP, never recomputed. scripts/dev/session-scratch.sh is the one shell
// implementation of the rule, shared with the hooks, and make and Go each
// implement it for their own callers (mk/helper-session.mk,
// internal/test/sessionpath). Asking it beats writing a fourth copy, and it
// prints <session>/scratch, whose parent is the directory wanted here.
//
// ZE_SCRATCH_DIR in the environment wins, so a make recipe that already
// resolved it hands the answer over rather than paying for it twice.
func ScratchDir(root string) string {
	if named := env.Get("ze.scratch.dir"); named != "" {
		return filepath.Join(root, named)
	}
	printed, err := sessionScratch(root)
	if err != nil || printed == "" {
		return filepath.Join(root, "tmp")
	}
	return filepath.Dir(filepath.Join(root, printed))
}

// canonicalBinDir is where the session's own binaries live: <session>/bin, or
// bin off-session.
func canonicalBinDir(root string) string {
	if env.Get("ze.session.id") != "" {
		return filepath.Join(ScratchDir(root), "bin")
	}
	return filepath.Join(root, "bin")
}

// CoverRoot answers the absolute coverage root, or the empty string when
// ZE_COVER is off.
//
// A .ci with tmpfs= runs its process from the per-test directory.
// Thus, a relative GOCOVERDIR resolves against that directory.
// The emit then fails and prints "coverage meta-data emit failed" on the child's stderr.
// That result loses coverage data and changes stderr that a .ci can assert on.
func CoverRoot(root string) string {
	if !covering() {
		return ""
	}
	return filepath.Join(ScratchDir(root), "scratch", "covdata")
}

// covering reports whether this run records coverage.
func covering() bool { return env.Get("ze.cover") != "" }

// BuildCommands answers the builds one isolated set needs, in order.
//
// The DUT build mirrors runner.TestBuildTags (internal/test/runner/runner.go).
// It includes the zetest plugins, full command surface, and default feature gates.
// It omits version ldflags so `ze show version` prints "ze dev"
// (test/parse/cli-version-show.ci). The ze-stripped tags match the Makefile rule.
// The chaos dashboard is a second cmd/ze build with its own tags.
// It sits beside the ze binary where cmd_web.go expects it.
func BuildCommands(tc gotoolchain.Toolchain, binaries string, chaos bool) [][]string {
	cover := []string{}
	if covering() {
		cover = []string{"-cover"}
	}

	build := func(extra []string, tags, name string) []string {
		argv := []string{"go", "build"}
		argv = append(argv, extra...)
		return append(argv, "-tags", tags, "-o", filepath.Join(binaries, name), "./cmd/ze")
	}

	dutTags := append([]string{"ze_core", "ze_distro", "ze_setup", "zetest"}, tc.Features...)
	commands := [][]string{
		build(cover, tagString(tc, dutTags...), "ze"),
		build(cover, tagString(tc, "ze_core", "ze_ssh"), "ze-stripped"),
		// NOT instrumented: ze-test is the harness, not the subject.
		build(nil, tagString(tc, append([]string{"ze_test"}, tc.Features...)...), ZeTest),
	}
	if chaos {
		commands = append(commands, build(nil, tagString(tc, "ze_chaos", "ze_bgp"), "ze-chaos"))
	}
	return commands
}

// tagString renders one -tags value: the parts asked for, then whatever ZE_TAGS
// added.
//
// gotoolchain.TestTags supplies the tags of a normal build.
// Each of these four builds has a different purpose, so each call names its tags.
// Only the extra tags are shared.
func tagString(tc gotoolchain.Toolchain, parts ...string) string {
	var tb textbuf.Buffer
	for i, tag := range parts {
		if i > 0 {
			tb.Byte(' ')
		}
		tb.Str(tag)
	}
	for _, tag := range tc.ExtraTags {
		tb.Byte(' ').Str(tag)
	}
	return tb.String()
}

// BinaryRoot answers the throwaway root this invocation builds into, and
// whether to remove it.
//
// An explicit ZE_SUFFIX gives the run a stable name and keeps its directory.
// Otherwise, the name contains the PID and run label.
// This prevents concurrent invocations and suites on one command line from deleting each other's binaries.
func BinaryRoot(root, label string) (dir string, remove bool) {
	scratch := ScratchDir(root)
	if suffix := env.Get("ze.suffix"); suffix != "" {
		var tb textbuf.Buffer
		return filepath.Join(scratch, tb.Str("testbin-").Str(suffix).String()), false
	}
	var tb textbuf.Buffer
	name := tb.Str("testbin-pid-").Str(strconv.Itoa(os.Getpid())).Byte('-').Str(label).String()
	return filepath.Join(scratch, name), true
}

// ErrBuildFailed says one of the builds an isolated set needs did not succeed.
var ErrBuildFailed = errors.New("functional: the isolated test binaries could not be built")

// Prepare builds the set this invocation runs against.
//
// The directory name contains the PID and run label.
// Thus, concurrent invocations and suites on one command line cannot delete each other's binaries.
// The directory is inside the session directory.
// A set therefore survives a missed cleanup but leaves with its session.
func Prepare(tc gotoolchain.Toolchain, label string, chaos bool) (BinarySet, error) {
	if env.Get("ze.test.canonical") != "" {
		return BinarySet{Dir: canonicalBinDir(tc.Root), Canonical: true}, nil
	}

	root, remove := BinaryRoot(tc.Root, label)
	binaries := filepath.Join(root, "bin")
	if err := os.MkdirAll(binaries, 0o750); err != nil {
		return BinarySet{}, err
	}

	var tb textbuf.Buffer
	gaterun.Note(tb.Str("Building isolated test binaries in ").Str(binaries).
		Str("/ (ze, ze-test, ze-stripped)...").String())

	environ := tc.Environment(gotoolchain.EnvOptions{})
	for _, argv := range BuildCommands(tc, binaries, chaos) {
		if gaterun.Stream(argv, tc.Root, environ) != 0 {
			if remove {
				removeTree(root)
			}
			return BinarySet{}, ErrBuildFailed
		}
	}
	return BinarySet{Dir: binaries, Remove: remove}, nil
}

// Release removes a throwaway set. A named or canonical one is left where it is.
func Release(set BinarySet) {
	if set.Remove {
		removeTree(filepath.Dir(set.Dir))
	}
}

// removeTree deletes a directory this run created, and says so when it cannot.
//
// A failure is reported because a leftover set makes the next run use a stale binary.
// The tree is inside the session scratch directory.
func removeTree(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		var tb textbuf.Buffer
		gaterun.Note(tb.Str("could not remove ").Str(dir).Str(": ").Err(err).String())
	}
}
