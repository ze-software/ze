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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

// zeTestPath is the harness binary a suite is run through.
func (b BinarySet) zeTestPath() string { return filepath.Join(b.Dir, ZeTest) }

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
	return append(base, tb.Str("ZE_TEST_BIN=").Str(b.zeTestPath()).String())
}

// scratchDir answers this session's own directory, or tmp when ZE_SCRATCH_DIR
// explicitly names the off-session path.
//
// The native resolver looks up an existing dated directory before naming a new
// one. ZE_SCRATCH_DIR wins when a make recipe already resolved it. A malformed
// named path or an identity resolution failure is returned rather than silently
// routing the run to checkout-wide tmp.
func scratchDir(root string) (string, error) {
	if named := strings.TrimSpace(env.Get("ze.scratch.dir")); named != "" {
		cleaned := filepath.Clean(named)
		if filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." ||
			strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("functional: unsafe ZE_SCRATCH_DIR %q", named)
		}
		return filepath.Join(root, cleaned), nil
	}
	printed, err := sessionScratch(root)
	if err != nil {
		return "", err
	}
	if printed == "" {
		return "", errors.New("functional: session scratch resolver returned an empty path")
	}
	return filepath.Dir(filepath.Join(root, printed)), nil
}

// canonicalBinDir is where the session's own binaries live: <session>/bin, or
// bin off-session.
func canonicalBinDir(root string) (string, error) {
	if env.Get("ze.session.id") == "" && os.Getenv("CLAUDE_CODE_SESSION_ID") == "" {
		return filepath.Join(root, "bin"), nil
	}
	scratch, err := scratchDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(scratch, "bin"), nil
}

// coverRoot answers the absolute coverage root, or the empty string when
// ZE_COVER is off.
//
// A .ci with tmpfs= runs its process from the per-test directory.
// Thus, a relative GOCOVERDIR resolves against that directory.
// The emit then fails and prints "coverage meta-data emit failed" on the child's stderr.
// That result loses coverage data and changes stderr that a .ci can assert on.
func coverRoot(root string) (string, error) {
	if !covering() {
		return "", nil
	}
	scratch, err := scratchDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(scratch, "scratch", "covdata"), nil
}

// covering reports whether this run records coverage.
func covering() bool { return env.Get("ze.cover") != "" }

// buildCommands answers the builds one isolated set needs, in order.
//
// The DUT build mirrors runner.TestBuildTags (internal/test/runner/runner.go).
// It includes the zetest plugins, full command surface, and default feature gates.
// It omits version ldflags so `ze show version` prints "ze dev"
// (test/parse/cli-version-show.ci). The stripped and chaos builds use the native
// tag sets declared here. The chaos dashboard sits beside the ze binary where
// cmd_web.go expects it.
func buildCommands(tc gotoolchain.Toolchain, binaries string, chaos bool) [][]string {
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

// binaryRoot answers the throwaway root this invocation builds into, and
// whether to remove it.
//
// An explicit ZE_SUFFIX gives the run a stable name and keeps its directory.
// Otherwise, the name contains the PID and run label.
// This prevents concurrent invocations and suites on one command line from deleting each other's binaries.
func binaryRoot(root, label string) (dir string, remove bool, err error) {
	scratch, err := scratchDir(root)
	if err != nil {
		return "", false, err
	}
	if suffix := env.Get("ze.suffix"); suffix != "" {
		var tb textbuf.Buffer
		return filepath.Join(scratch, tb.Str("testbin-").Str(suffix).String()), false, nil
	}
	var tb textbuf.Buffer
	name := tb.Str("testbin-pid-").Str(strconv.Itoa(os.Getpid())).Byte('-').Str(label).String()
	return filepath.Join(scratch, name), true, nil
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
		dir, err := canonicalBinDir(tc.Root)
		if err != nil {
			return BinarySet{}, err
		}
		return BinarySet{Dir: dir, Canonical: true}, nil
	}

	root, remove, err := binaryRoot(tc.Root, label)
	if err != nil {
		return BinarySet{}, err
	}
	binaries := filepath.Join(root, "bin")
	if err := os.MkdirAll(binaries, 0o750); err != nil {
		return BinarySet{}, err
	}

	var tb textbuf.Buffer
	gaterun.Note(tb.Str("Building isolated test binaries in ").Str(binaries).
		Str("/ (ze, ze-test, ze-stripped)...").String())

	environ := tc.Environment(gotoolchain.EnvOptions{})
	for _, argv := range buildCommands(tc, binaries, chaos) {
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
