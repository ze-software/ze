// Design: docs/architecture/core-design.md -- the Go toolchain environment le runs every build and test under
//
// Package gotoolchain provides the environment and argv for every le action
// that starts a Go command.
//
// The package DERIVES these values from the checkout instead of defining them
// here. A literal beside the source value in go.mod or feature-gates.txt would
// create two records of one fact. An unchecked copy can drift. The manifest read
// uses internal/le/repo/featuretags, which already reads this value for daemon builds.
//
// Five variables have behavior that their names do not show:
//
//   - GOCACHE resides inside the checkout, not under TMPDIR. A cache under
//     TMPDIR breaks the Unix-socket tests because socket paths exceed the kernel
//     limit. The cache resides at cache/, which is symlinked out of tree by
//     `./le scratch links-ensure`. Therefore, it survives a scratch wipe. A
//     detached verify worktree gets that link too, from EnsureCache at
//     extraction (internal/le/scratch, and internal/le/verify, sharedCacheLink),
//     so a worktree run shares this cache instead of building a private one from
//     cold. A tree that carries neither link resolves GOCACHE to a REAL
//     directory under itself, which costs a cold build and the disk to hold it.
//
//   - CGO_ENABLED defaults to 0. The race detector requires 1, and only a race
//     run requests it.
//
//   - GOROOT, GOTOOLCHAIN and the head of PATH are NOT set here. New calls
//     gen.Activate, which builds the patched GOROOT from the toolchain
//     go.mod pins and sets GOROOT to it, GOTOOLCHAIN to local and its bin
//     first on PATH in this process (docs/contributing/patching-the-standard-library.md).
//     Environment inherits the three with the rest of os.Environ(), so one
//     declaration decides the toolchain and no override can disagree with it.
//     The pin still matters: golangci-lint is a separate binary linked against
//     one version of go/types and decodes export data the toolchain writes, so
//     an ambient Go newer than the linter's build version fails every package
//     with "export data version N is greater than maximum supported version
//     M" (measured 2026-08-22, ambient 1.27.0 against a linter built with
//     1.26.6). The tree is built from the pinned toolchain, and local runs it.
//
//   - GOMAXPROCS limits each Go process or test run to a QUARTER of the cores.
//     No process or test run gets more.
//
//   - GOTMPDIR, on a test run only, is <home>/.zt (TestTempDir). Go 1.27's
//     t.TempDir() creates beneath GOTMPDIR, and the go command writes its work
//     directory there, so both land on the device holding the user's home
//     rather than under TMPDIR. TMPDIR itself is left alone: the socket helpers
//     that call os.MkdirTemp("") keep the directory they chose for path length.
//
// PORT CORRECTION. The Python implementation returns an empty tag tuple when
// it cannot read feature-gates.txt. It also returns an empty tuple for a
// manifest that declares no ze_ tag. test_tags then renders `ze_core` alone.
// This output is byte-identical to core_tags. The core_tags docstring calls a
// reduced set "a defect everywhere else" because it excludes modules from
// compilation.
//
// All 156 gates build every argv from test_tags. Thus, all gates silently test
// a smaller product than the one that ships. New returns an error for both
// feature-list cases. It also returns an error for an unreadable go.mod.

package gotoolchain

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// DefaultTestTimeout is the per-test-binary timeout. The environment can
// override it.
const DefaultTestTimeout = "20m"

// coreTag is the build tag every flavor carries. The feature gates are added to
// it, never substituted for it.
const coreTag = "ze_core"

// The keys the three run-time knobs are read under. Two retain their established
// environment aliases so existing automation keeps reaching the same value.
const (
	// TagsKey specifies more build tags without a manifest change.
	TagsKey = "ze.tags"
	// TimeoutKey is the per-test-binary timeout.
	TimeoutKey = "ze.go.test.timeout"
	// MemLimitKey is the linter's soft heap ceiling.
	MemLimitKey = "ze.lint.memlimit"
)

// memLimitFloorGiB defines the minimum soft heap ceiling for a lint run. It
// protects systems where total memory cannot be determined. totalMemoryGiB
// returns 0 when neither /proc/meminfo nor sysctl returns a value. The floor
// then sets the ceiling to 4GiB instead of 0GiB. A 0GiB ceiling would make the
// linter thrash on every package.
const memLimitFloorGiB = 4

// memLimitDivisor allows a lint run to use one eighth of the RAM. Ze development
// uses machines with different RAM sizes. A fixed limit can equal one eighth of
// one machine's RAM and half of another machine's RAM.
const memLimitDivisor = 8

// procDivisor allows a Go test run to use one quarter of the cores.
const procDivisor = 4

// typeString is the env registry's word for a free-text value. It is a constant
// because three entries declare it and a typo in one of them would register a
// type nothing reads.
const typeString = "string"

// sysctlTimeout limits the duration of the one request this package sends to a
// foreign binary. One second for `sysctl -n hw.memsize` is already abnormally
// long. The limit prevents a blocked sysctl process from delaying a build
// indefinitely.
const sysctlTimeout = 5 * time.Second

var tagsEntry = env.MustRegister(env.EnvEntry{
	Key:         TagsKey,
	Type:        typeString,
	Default:     "",
	Description: "extra Go build tags every le-driven build and test carries, beyond the manifest",
	// Private keeps the key out of `ze env list`. It is a build-host knob and
	// an operator has nothing to do with it.
	Private: true,
})

var timeoutEntry = env.MustRegister(env.EnvEntry{
	Key:         TimeoutKey,
	Type:        typeString,
	Default:     DefaultTestTimeout,
	Description: "the per-test-binary timeout every le-driven `go test` carries",
	Aliases:     []string{"GO_TEST_TIMEOUT"},
	Private:     true,
})

var memLimitEntry = env.MustRegister(env.EnvEntry{
	Key:         MemLimitKey,
	Type:        typeString,
	Default:     "",
	Description: "the soft heap ceiling a golangci-lint run gets; derived from this machine's RAM when unset",
	Aliases:     []string{"ZE_LINT_MEMLIMIT"},
	Private:     true,
})

// Toolchain is the environment and the command prefixes derived from one
// checkout. Every field is data a test can set, so a test drives the same code
// the command runs rather than a copy of it.
type Toolchain struct {
	// Root is the checkout every derived path is rooted at.
	Root string
	// Features is every ze_ gate tag the manifest declares, sorted.
	Features []string
	// GoToolchain is the pin from go.mod, or "" when go.mod declares none.
	GoToolchain string
	// Procs is the GOMAXPROCS cap a test run gets.
	Procs int
	// Timeout is the per-test-binary timeout.
	Timeout string
	// Version is the release identity every built binary carries.
	Version string
	// BuildDate is when this invocation started, in UTC, to the second.
	BuildDate string
	// LintMemLimit is the soft heap ceiling a golangci-lint run gets.
	LintMemLimit string
	// ExtraTags is what ZE_TAGS added.
	ExtraTags []string
}

// New returns the toolchain for this checkout.
//
// New returns an error instead of a reduced tag set. An empty feature list
// builds a daemon with every feature excluded from compilation. The caller
// cannot distinguish that result from a defect in the subject under test.
func New(root string) (Toolchain, error) {
	features, err := repofeaturetags.DaemonTags(root)
	if err != nil {
		return Toolchain{}, err
	}

	pin, err := gen.Pin(root)
	if err != nil {
		return Toolchain{}, err
	}

	// Every caller is about to start a Go command, so a missing go is refused
	// here, once, rather than surfacing as exit 127 from whichever command runs
	// first.
	if _, err := exec.LookPath(goProgram); err != nil {
		return Toolchain{}, goMissing(pin)
	}

	// Every Go command compiles against the patched GOROOT. Under le the root
	// handler already activated it and this answers the same tree; in a test
	// binary this is the activation. The tree belongs to the checkout this
	// process runs in, which is root itself outside the fixtures of tests.
	// A refusal (a stale review stamp, a patch that no longer applies) is
	// New's error, because no command this toolchain starts could compile.
	checkout, err := lepath.Root()
	if err != nil {
		return Toolchain{}, err
	}
	if _, err := gen.Activate(checkout); err != nil {
		return Toolchain{}, err
	}

	now := time.Now()
	return Toolchain{
		Root:         root,
		Features:     features,
		GoToolchain:  pin,
		Procs:        testProcs(),
		Timeout:      timeoutFromEnv(),
		Version:      now.Format("06.01.02"),
		BuildDate:    now.UTC().Format("2006-01-02T15:04:05Z"),
		LintMemLimit: lintMemLimit(),
		ExtraTags:    extraTags(),
	}, nil
}

// goProgram is the program every argv this package builds starts with.
const goProgram = "go"

// goMissing answers the refusal New gives when no go is on PATH. It names the
// PATH it searched, the version go.mod pins, and the two fixes, because the
// usual cause is a non-interactive shell that never read the profile adding Go
// to PATH, and the exec error alone says none of that.
func goMissing(pin string) error {
	var tb textbuf.Buffer
	tb.Str("le needs the Go toolchain and finds no `go` program on PATH (")
	path := os.Getenv("PATH")
	if path == "" {
		tb.Str("PATH is empty")
	} else {
		tb.Str(path)
	}
	tb.Str("). ")
	if pin == "" {
		tb.Str("go.mod pins no toolchain, so the Go on PATH builds as it is. ")
	} else {
		tb.Str("This checkout builds with ").Str(pin).
			Str(", the version go.mod pins, and any Go 1.21 or newer on PATH downloads it on first use. ")
	}
	tb.Str("Install Go with `./le setup tools` or from https://go.dev/dl/, then put its bin directory on PATH," +
		" for example `export PATH=$PATH:/usr/local/go/bin`. A non-interactive shell such as `ssh host command`" +
		" does not read ~/.bashrc, so set PATH in ~/.profile or on the command line")
	return errors.New(tb.String())
}

// CoresPerJob answers how many cores one heavy job is allowed, which is the
// GOMAXPROCS a test run gets and the -j a lint run gets.
//
// Job admission divides the machine by this number to decide how many jobs run
// at once (internal/le/job, defaultSlots). The two have to agree: a share
// declared here and a slot count chosen there would let the machine be
// oversubscribed by whichever number was larger.
func CoresPerJob() int { return testProcs() }

// testProcs answers a quarter of the cores, and never fewer than one.
func testProcs() int {
	procs := runtime.NumCPU() / procDivisor
	if procs < 1 {
		return 1
	}
	return procs
}

// timeoutFromEnv answers the per-test-binary timeout, honoring the
// GO_TEST_TIMEOUT environment alias.
func timeoutFromEnv() string {
	if named := env.Get(timeoutEntry.Aliases[0]); named != "" {
		return named
	}
	return DefaultTestTimeout
}

// extraTags answers what ZE_TAGS declared, split on whitespace.
func extraTags() []string {
	return strings.Fields(env.Get(tagsEntry.Key))
}

// lintMemLimit answers the soft heap ceiling a golangci-lint run gets: an
// eighth of this machine's RAM, floored.
//
// ZE_LINT_MEMLIMIT in the environment wins, so
// `ZE_LINT_MEMLIMIT=16GiB ./le go lint run` reaches this setting directly.
// The default is derived below when the caller names no value.
func lintMemLimit() string {
	if named := env.Get(memLimitEntry.Aliases[0]); named != "" {
		return named
	}
	share := max(totalMemoryGiB()/memLimitDivisor, memLimitFloorGiB)
	var tb textbuf.Buffer
	return tb.Int(int64(share)).Str("GiB").String()
}

// kibPerGiB and bytesPerGiB are the two divisors the two sources need:
// /proc/meminfo reports KiB and sysctl reports bytes.
const (
	kibPerGiB   = 1048576
	bytesPerGiB = 1073741824
)

// totalMemoryGiB answers this machine's RAM in whole GiB, or 0 when neither
// source answers. Truncating division both times, which is what the Makefile's
// printf and shell arithmetic did.
//
// A 0 here is not a fail-open: lintMemLimit floors it, so an unanswerable
// machine gets the floor rather than a ceiling of zero.
func totalMemoryGiB() int {
	if kib, ok := procMemTotalKiB(); ok {
		return kib / kibPerGiB
	}
	return sysctlMemoryGiB()
}

// procMemTotalKiB reads MemTotal out of /proc/meminfo, in KiB.
func procMemTotalKiB() (int, bool) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if !strings.HasPrefix(line, "MemTotal") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		return atoi(fields[1])
	}
	return 0, false
}

// sysctlMemoryGiB asks Darwin's sysctl for hw.memsize, in whole GiB.
func sysctlMemoryGiB() int {
	ctx, cancel := context.WithTimeout(context.Background(), sysctlTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	size, ok := atoi(strings.TrimSpace(string(out)))
	if !ok {
		return 0
	}
	return size / bytesPerGiB
}

// atoi parses a non-negative decimal and reports whether parsing succeeded. It
// is implemented here because both callers must interpret an unparsable answer
// as "this source did not reply". An error value that contains 0 does not
// communicate this interpretation by itself.
func atoi(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	value := 0
	for i := range len(s) {
		digit := s[i]
		if digit < '0' || digit > '9' {
			return 0, false
		}
		value = value*10 + int(digit-'0')
	}
	return value, true
}

// GoCache answers the Go build cache every le-driven command writes for one
// checkout. It takes the root rather than a Toolchain, so a caller that only
// has to name or empty the cache needs no manifest read (internal/le/scratch,
// CleanCaches).
func GoCache(root string) string {
	return filepath.Join(root, "cache", "go-cache")
}

// LintCache answers the golangci-lint cache every le-driven lint run writes for
// one checkout. It takes the root for the same reason GoCache does: emptying
// the cache needs no manifest read (internal/le/scratch, CleanCaches).
//
// This cache is NOT under cache/, so the scratch relocation leaves it on the
// checkout's own device, and no Go cache clean touches it. It was measured at
// 9.5G on 2026-09-13, larger than both Go caches together
// (plan/journal/full-disk-false-red.md).
func LintCache(root string) string {
	return filepath.Join(root, "tmp", "golangci-lint-cache")
}

// BootstrapCache answers the Go build cache used before, or instead of, the
// checkout cache: the `le` bootstrap that builds bin/ze-le, the deployment
// daemon and VPP evidence builds, and the QEMU guest. Each of those runs with
// no inherited GOCACHE and would otherwise write the machine default.
//
// It is declared here because four writers spelled the same path themselves,
// and the cache cleaner knew about none of them: `./le scratch cache-clean`
// published that it empties every build cache this checkout fills, walked past
// 1.3G of this one, and the operator had to be told about it by hand
// (plan/journal/full-disk-false-red.md).
//
// The `le` shell script holds the one copy that cannot ask this function,
// because it runs before any Go binary exists. `TestBootstrapCacheMatchesTheShellScript`
// compares the two so the pair cannot drift.
func BootstrapCache(root string) string {
	return filepath.Join(root, "tmp", "go-cache")
}

// The Unix socket budget a test temp base must fit. A socket test binds a
// leaf under t.TempDir(), which is the base plus "/", the test name cut to 64
// bytes, up to 10 random digits, and "/001": 79 bytes. sun_path holds 108
// bytes with its NUL, so 107 are usable, and a base of 23 leaves 5 for the
// socket leaf ("/a.sk"). ~/.cache/tmp, 22 bytes, is the default it replaces on
// the Linux VM (plan/journal/full-disk-false-red.md, 2026-10-16).
const (
	sunPathMax       = 107
	tempDirSuffixMax = 79
	socketLeafMax    = 5
	testTempPathMax  = sunPathMax - tempDirSuffixMax - socketLeafMax
)

// testTempLeaf is the directory under the user's home that holds every le-run
// Go test's temporary directories. It is short because the base is part of
// every socket path a test builds under t.TempDir().
const testTempLeaf = ".zt"

// TestTempBase answers the test temp base for one home directory: <home>/.zt.
// It refuses a home that is empty or relative, and a base longer than
// testTempPathMax, rather than answering a path a socket test cannot use. It
// touches no filesystem, so internal/le/scratch can name the directory it
// trims without creating it.
func TestTempBase(home string) (string, error) {
	if home == "" {
		return "", errors.New("HOME is not set; set HOME to your home directory")
	}
	if !filepath.IsAbs(home) {
		return "", errors.New("HOME=" + home + " is not an absolute path; set HOME to your home directory's absolute path")
	}
	base := filepath.Join(home, testTempLeaf)
	if len(base) > testTempPathMax {
		var tb textbuf.Buffer
		tb.Str(base).Str(" is ").Int(int64(len(base))).Str(" bytes, longer than the ").
			Int(testTempPathMax).Str(" a Unix socket under t.TempDir() can afford; run le with HOME set to an ").
			Str("absolute path of at most ").Int(testTempPathMax - int64(len(testTempLeaf)) - 1).Str(" bytes")
		return "", errors.New(tb.String())
	}
	return base, nil
}

// makeTestTempDir creates base (0700) when it is missing and proves a test can
// create a directory in it, the one operation t.TempDir() performs there. Its
// error names the command that repairs the directory: chown and chmod when
// the probe is refused permission (root-owned after a `sudo -E` run), the
// device otherwise, because a full, read-only or failing disk is no
// ownership problem.
func makeTestTempDir(base string) error {
	if err := os.MkdirAll(base, 0o700); err != nil {
		return errors.New(err.Error() + "; make the parent writable, or create it: mkdir -m 700 " + base)
	}
	probe, err := os.MkdirTemp(base, "probe-")
	if errors.Is(err, fs.ErrPermission) {
		return errors.New(err.Error() + "; give it back to yourself: sudo chown -R $USER " + base + " && chmod 700 " + base)
	}
	if err != nil {
		return errors.New(err.Error() + "; check the device that holds it: df -h " + base)
	}
	return os.Remove(probe)
}

// testTempOnce resolves the test temp dir once for each le process, so the
// warning is written once however many commands the process starts.
var testTempOnce = sync.OnceValue(func() string {
	base, err := testTempDirFor(os.UserHomeDir)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("le: Go tests keep their temp dirs under TMPDIR, because the test temp dir is unusable: ").
			Str(err.Error()).Byte('\n')
		_, _ = os.Stderr.WriteString(tb.String()) // A warning that cannot be written has nowhere else to go.
		return ""
	}
	return base
})

// testTempDir is the resolver Overrides calls; a test replaces it so it
// neither touches the real ~/.zt nor reads the answer it asserts from the
// function under test.
var testTempDir = TestTempDir

// SetTestTempDirForTest makes every later test-run Overrides answer
// GOTMPDIR=dir, or no GOTMPDIR when dir is "", and returns the call that
// restores the previous resolver. Test use only: a package in another
// directory that pins a test run's exact environment calls it from TestMain,
// so its answer depends neither on the developer's HOME nor on whether ~/.zt
// is usable, and the run never creates the real ~/.zt. Not safe for
// concurrent use with Overrides.
func SetTestTempDirForTest(dir string) (restore func()) {
	previous := testTempDir
	testTempDir = func() string { return dir }
	return func() { testTempDir = previous }
}

// testTempDirFor resolves, creates and probes the base under the directory
// home answers.
func testTempDirFor(home func() (string, error)) (string, error) {
	dir, err := home()
	if err != nil {
		return "", errors.New(err.Error() + "; set HOME to your home directory")
	}
	base, err := TestTempBase(dir)
	if err != nil {
		return "", err
	}
	if err := makeTestTempDir(base); err != nil {
		return "", err
	}
	return base, nil
}

// TestTempDir answers the GOTMPDIR every le-run Go test gets: <home>/.zt,
// created 0700 on first use, on the device that holds the user's home. It
// answers "" when the directory cannot be used (no home, a base too long for
// a socket path, a directory a test cannot write), after writing the reason
// to stderr once; a caller then sets no GOTMPDIR and the run keeps Go's
// default. Safe for concurrent use. The store trim removes what a killed run
// leaves there (internal/le/scratch, trimTestTemp).
func TestTempDir() string {
	return testTempOnce()
}

// namedLauncherPrefix starts the bin/ directory of every launcher the `le`
// script builds under a name: `./le --name <name>` builds bin/le-<name>/le,
// and a foreign-platform bin/le falls back to bin/le-<uname -s>-<uname -m>/le.
const namedLauncherPrefix = "le-"

// NamedLauncherDir answers bin/le-<name>, the directory `./le --name <name>`
// builds its own le into, rebuilt on every call.
//
// The `le` shell script holds the one copy that cannot ask this function,
// because it runs before any Go binary exists, and so does its check_name
// rule, which NamedLauncherName repeats.
// `TestNamedLauncherMatchesTheShellScript` compares them so they cannot drift.
func NamedLauncherDir(root, name string) string {
	return filepath.Join(root, "bin", namedLauncherPrefix+name)
}

// NamedLauncherName answers the build name of a bin/ entry when the entry is
// the directory of a launcher built under a name, and false otherwise. It
// answers false for the platform fallback the script shares between sessions,
// bin/le-<uname -s>-<uname -m>, and for any name the script's check_name would
// refuse, so a caller that removes what it answers can reach neither.
//
// The platform test reads the name's first character: `uname -s` answers a
// capitalized kernel name (Darwin, Linux, FreeBSD), so a name that starts with
// a capital is never answered. A build somebody named with a capital is
// therefore never reclaimed: that costs disk, and never a peer's binary.
func NamedLauncherName(entry string) (string, bool) {
	name, found := strings.CutPrefix(entry, namedLauncherPrefix)
	if !found {
		return "", false
	}
	if !validLauncherName(name) {
		return "", false
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return "", false
	}
	return name, true
}

// validLauncherName is the `le` script's check_name: letters, digits, dot,
// underscore and hyphen, never empty, and never `..` anywhere in it.
func validLauncherName(name string) bool {
	if name == "" {
		return false
	}
	if strings.Contains(name, "..") {
		return false
	}
	for _, character := range name {
		if !launcherNameCharacter(character) {
			return false
		}
	}
	return true
}

// launcherNameCharacter answers whether check_name accepts one character:
// a letter, a digit, dot, underscore or hyphen.
func launcherNameCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' || character == '.' || character == '_' || character == '-'
}

// LDFlags answers the linker flags every released binary carries.
//
// One string, because that is how `go build -ldflags` takes it. A binary built
// without these reports an empty version, which is indistinguishable from a
// binary somebody built by hand.
func (t Toolchain) LDFlags() string {
	var tb textbuf.Buffer
	return tb.Str("-X main.version=").Str(t.Version).
		Str(" -X main.buildDate=").Str(t.BuildDate).String()
}

// TestTags answers the tag set a normal build and test carries: core, every
// feature the manifest declares, then the extras.
func (t Toolchain) TestTags() string {
	return joinTags(t.Features, t.ExtraTags)
}

// coreTags returns the bare tag set for the compile-out checks.
//
// A reduced set excludes modules from compilation and reduces the surface that
// a gate can see. This reduction is required for those checks and is a defect
// everywhere else.
func (t Toolchain) coreTags() string {
	return joinTags(nil, t.ExtraTags)
}

// joinTags renders a tag list the one way `go build -tags` documents:
// space-separated. Commas work too, and using both spellings is how two builds
// of one tree come to carry different features.
func joinTags(features, extras []string) string {
	var tb textbuf.Buffer
	tb.Str(coreTag)
	for _, tag := range features {
		tb.Byte(' ').Str(tag)
	}
	for _, tag := range extras {
		tb.Byte(' ').Str(tag)
	}
	return tb.String()
}

// EnvOptions says which of the optional overrides a command needs.
type EnvOptions struct {
	// Test removes the launching le's checkout and build identity, and the
	// verify gate's change scope. Go tests discover their source checkout from
	// cwd and own their fixture overrides.
	// Every caller running Go tests MUST set this, including fuzz and benchmarks.
	Test bool
	// CGO turns CGO_ENABLED on, which the race detector cannot run without.
	CGO bool
	// Procs adds the GOMAXPROCS cap, which belongs on a test run rather than
	// on a build.
	Procs bool
	// MemLimit adds the linter's soft heap ceiling, which belongs on a
	// golangci-lint run and nowhere else.
	MemLimit bool
	// GOOS and GOARCH are for a cross build. A host build passes neither and
	// inherits the machine's.
	GOOS   string
	GOARCH string
}

// Overrides returns the variables that this toolchain SETS: GOCACHE and
// GOLANGCI_LINT_CACHE always, each spelled where a link inside the root
// carries it, CGO_ENABLED always, GOTMPDIR on a test run when
// TestTempDir has one, and GOMAXPROCS, GOMEMLIMIT, GOOS and GOARCH when opts
// asks. Like GOCACHE, a GOTMPDIR the caller exported is overridden: the
// test temp dir is the one place the store trim reaps. It returns KEY=VALUE
// entries in a fixed order. The test temp dir is resolved here, on the first
// test run, so a process that starts no test never creates it. It sets no
// GOROOT and no GOTOOLCHAIN: the process environment holds
// both, set by gen.Activate, and a second declaration here could only
// disagree with it.
//
// Overrides is separate from Environment so that a test can verify
// toolchain-specific values. The inherited environment is machine-specific and
// does not describe the port.
func (t Toolchain) Overrides(opts EnvOptions) []string {
	var tb textbuf.Buffer
	over := make([]string, 0, 8)

	// Each cache is named where its link lands (gen.Landing), so two verify
	// worktrees of one commit hand their children the same value.
	over = append(over, tb.Str("GOCACHE=").Str(gen.Landing(t.Root, GoCache(t.Root))).String())
	tb.Reset()
	over = append(over, tb.Str("GOLANGCI_LINT_CACHE=").Str(gen.Landing(t.Root, LintCache(t.Root))).String())

	if opts.CGO {
		over = append(over, "CGO_ENABLED=1")
	} else {
		over = append(over, "CGO_ENABLED=0")
	}
	if opts.Test {
		if testTemp := testTempDir(); testTemp != "" {
			tb.Reset()
			over = append(over, tb.Str("GOTMPDIR=").Str(testTemp).String())
		}
	}
	if opts.Procs {
		tb.Reset()
		over = append(over, tb.Str("GOMAXPROCS=").Int(int64(t.Procs)).String())
	}
	if opts.MemLimit {
		tb.Reset()
		over = append(over, tb.Str("GOMEMLIMIT=").Str(t.LintMemLimit).String())
	}
	if opts.GOOS != "" {
		tb.Reset()
		over = append(over, tb.Str("GOOS=").Str(opts.GOOS).String())
	}
	if opts.GOARCH != "" {
		tb.Reset()
		over = append(over, tb.Str("GOARCH=").Str(opts.GOARCH).String())
	}
	return over
}

// Environment answers the whole environment a Go command runs under. Test
// children do not inherit the launching le's checkout or build identity, or
// the verify gate's change scope.
// Toolchain overrides follow inherited entries, so os/exec uses their values.
// It carries exactly one GOFLAGS entry, the effective value with -trimpath
// (TrimpathGoFlags), unless Go cannot answer what its effective value is.
//
// Environment holds no signal root, so the `go env GOFLAGS` query it may run
// cannot be stopped. A caller that holds one (a verb's root, a build under
// job admission) MUST call EnvironmentContext instead, or a SIGTERM that has
// already ended the verb still starts a go process.
func (t Toolchain) Environment(opts EnvOptions) []string {
	return t.EnvironmentContext(context.Background(), opts)
}

// EnvironmentContext is Environment under ctx: a done ctx starts no
// `go env GOFLAGS` query, and the answer then carries no GOFLAGS entry. That
// environment builds the same result as one with the entry, so it is still a
// correct answer, and no warning is printed, because the caller's own command
// is refused by the same done ctx and nothing would run without -trimpath.
func (t Toolchain) EnvironmentContext(ctx context.Context, opts EnvOptions) []string {
	inherited := os.Environ()
	if opts.Test {
		// The checkout root, the build name and the verify gate's change scope
		// (repochanged.ScopeFileKey, ScopeTagsKey: files under the run's own
		// worktree) describe the launching le, not the test. A test that
		// inherited the scope would also be narrowed by the gate's change set.
		// TestATestChildInheritsNoChangeScope (internal/le/repo/changed)
		// compares the two scope spellings with their declarations.
		inherited = env.Without(inherited, "ze.repo.root", "ze.le.build.name",
			"ze.verify.scope.packages", "ze.verify.scope.tags")
	}
	overrides := t.Overrides(opts)
	full := make([]string, 0, len(inherited)+len(overrides)+1)
	full = append(full, inherited...)
	full = append(full, overrides...)
	flags, err := TrimpathGoFlags(ctx, full, t.Root)
	if err != nil {
		if ctx.Err() != nil {
			// The root ended before Go answered: the caller's command will
			// not start, so there is no loss to report.
			return full
		}
		// The command still runs, and runs correctly: Go reads its own GOFLAGS
		// as it would without le. What it loses is cache reuse across
		// checkouts, so the loss is said and the command is not refused. One
		// le process builds many Go environments, and Go's answer does not
		// change between them, so the loss is said once per process.
		goFlagsWarning.Do(func() {
			var tb textbuf.Buffer
			_, _ = os.Stderr.WriteString(tb.Str("le: ").Err(err).
				Str("; Go commands run without -trimpath, so they reuse no compile from another checkout path. ").
				Str("Run `go env GOFLAGS` to see why Go cannot answer\n").String())
		})
		return full
	}
	return append(withoutKey(full, goFlagsKey), flags)
}

// goFlagsWarning makes Environment say once per process that Go could not
// answer its effective GOFLAGS. A pointer, so a test can start a fresh one.
var goFlagsWarning = new(sync.Once)

// goFlagsKey is the variable the go command reads for its default flags.
const goFlagsKey = "GOFLAGS"

// trimpathFlag is the go flag that drops the source directory from every
// compile action ID.
const trimpathFlag = "-trimpath"

// TrimpathGoFlags answers the GOFLAGS entry ("GOFLAGS=<value>") every
// le-driven Go command runs under: the effective value a go command started
// with environ in dir would read (EffectiveGoFlags), plus -trimpath unless a
// field already is it. Environment applies it, and the Staticcheck gate
// applies it to the environment it assembles for itself; nothing else in le
// adds -trimpath, except the tracked-build gate
// (internal/le/repo/compiles/repositorytrackedbuild.go), which assembles its
// own environment without this toolchain and keeps the flag in its argv.
//
// -trimpath is what makes a fresh checkout reuse the shared Go cache. Go
// hashes the package directory into every compile action ID unless the flag
// is set ((*Builder).buildActionID, cmd/go/internal/work/exec.go), and each
// verify run extracts the tree into a new worktree path, so every run compiled
// the module cold. On 2026-10-16 the Staticcheck all-features row took 495.7s
// cold (2157s of CPU) against 11.3s warm. golangci-lint has no -trimpath flag
// of its own: its `go list -export` load sees only GOFLAGS, which is why the
// flag lives in the environment and not in each argv. The flag changes no
// diagnostic: a compile error names the same tree-relative file. It does
// change runtime.Caller, which answers a module-path-prefixed name, so a test
// MUST NOT find the repository from its own source path.
//
// An error means Go could not say what its effective GOFLAGS are, and the two
// callers answer it differently on purpose. Environment proceeds without the
// flag: the command it starts still reads Go's own GOFLAGS and builds the same
// result, only colder. The Staticcheck gate (judgeEnvironment) refuses: its
// matrix runs under a deadline measured with warm, path-independent compiles,
// so a cold run would time out and read as a red the tree never earned, and a
// verdict the gate cannot stand behind is refused rather than guessed.
//
// ctx bounds the `go env` query: a done ctx starts no go process and answers
// its error.
func TrimpathGoFlags(ctx context.Context, environ []string, dir string) (string, error) {
	flags, err := EffectiveGoFlags(ctx, []string{goProgram}, dir, environ)
	if err != nil {
		return "", err
	}
	if !slices.Contains(strings.Fields(flags), trimpathFlag) {
		flags = strings.TrimSpace(flags + " " + trimpathFlag)
	}
	return goFlagsKey + "=" + flags, nil
}

// EffectiveGoFlags answers the GOFLAGS value a go command started from
// goCommand (the argv prefix before the subcommand) with environ in dir would
// use. A nonempty GOFLAGS variable already decides it: Go reads the variable
// before its persistent configuration, and the last entry is the one a child
// sees. An empty or absent one does not suppress the GOENV file or the
// GOROOT go.env, so the answer then comes from `go env GOFLAGS` itself,
// including its platform-specific default GOENV path. The query runs under
// ctx, the caller's signal root, so a done ctx starts no go process
// (exec.Cmd.Start refuses one), and it is bounded by a minute besides, so a
// wedged go command cannot hold the caller forever.
func EffectiveGoFlags(ctx context.Context, goCommand []string, dir string, environ []string) (string, error) {
	for _, entry := range slices.Backward(environ) {
		if value, found := strings.CutPrefix(entry, goFlagsKey+"="); found {
			if value != "" {
				return value, nil
			}
			break
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	args := make([]string, 0, len(goCommand)+1)
	args = append(args, goCommand[1:]...)
	args = append(args, "env", goFlagsKey)
	cmd := exec.CommandContext(ctx, goCommand[0], args...) //nolint:gosec // the caller's own Go executable
	cmd.Dir = dir
	cmd.Env = environ
	var output, diagnostic strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil {
		var tb textbuf.Buffer
		tb.Str("resolve the effective GOFLAGS: ").Err(err)
		if said := strings.TrimSpace(diagnostic.String()); said != "" {
			tb.Str(": ").Str(said)
		}
		return "", errors.New(tb.String())
	}
	return strings.TrimSpace(output.String()), nil
}

// withoutKey answers environ with every entry for key removed, so the entry
// appended after it is the only one and no reader can take an earlier one.
func withoutKey(environ []string, key string) []string {
	prefix := key + "="
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		if !strings.HasPrefix(entry, prefix) {
			kept = append(kept, entry)
		}
	}
	return kept
}

// goRun returns the argv for `go run` on one script. The argv includes the full
// feature tag set.
//
// Gates that inspect the command surface require the full set. A reduced set
// excludes modules from compilation. The gate would then report on a smaller
// product than the one that ships.
func (t Toolchain) goRun(script string, args ...string) []string {
	argv := make([]string, 0, 5+len(args))
	argv = append(argv, "go", "run", "-tags", t.TestTags(), script)
	return append(argv, args...)
}

// TestOptions says which tag set and which detector a `go test` argv carries.
type TestOptions struct {
	// Core selects the bare tag set, for a compile-out check.
	Core bool
	// Race turns the race detector on. It needs EnvOptions.CGO as well.
	Race bool
	// Tags are build tags this one command needs on top of the set Core or the
	// feature manifest selects. A personality tag lives here rather than in
	// feature-gates.txt: the manifest declares features that register
	// themselves and that the linker can drop, and ze_installer selects the
	// initrd's own PID 1 instead.
	Tags []string
}

// tags answers the tag string these options ask for.
func (t Toolchain) tags(opts TestOptions) string {
	base := t.TestTags()
	if opts.Core {
		base = t.coreTags()
	}
	if len(opts.Tags) == 0 {
		return base
	}
	var tb textbuf.Buffer
	tb.Str(base)
	for _, tag := range opts.Tags {
		tb.Byte(' ').Str(tag)
	}
	return tb.String()
}

// GoTest answers the argv for `go test` with this checkout's tags, timeout and
// flags.
func (t Toolchain) GoTest(opts TestOptions, args ...string) []string {
	argv := make([]string, 0, 7+len(args))
	argv = append(argv, "go", "test", "-timeout", t.Timeout, "-tags", t.tags(opts))
	if opts.Race {
		argv = append(argv, "-race")
	}
	return append(argv, args...)
}

// GoList answers the argv for `go list -e` printing one import path per
// line, over the same tag set and race constraint GoTest builds with, so a
// caller that cuts a test population into pieces lists exactly the packages
// the test command would build. A package whose every file carries a feature
// tag, or the race build constraint, is invisible to a list without it. The -e
// keeps a package that fails to load in the answer, so the test command
// reports it as a normal FAIL rather than the list refusing every package.
func (t Toolchain) GoList(opts TestOptions, args ...string) []string {
	argv := make([]string, 0, 8+len(args))
	argv = append(argv, "go", "list", "-e", "-f", "{{.ImportPath}}", "-tags", t.tags(opts))
	if opts.Race {
		argv = append(argv, "-race")
	}
	return append(argv, args...)
}

// GoVet answers the argv for `go vet` over the same tag set.
//
// Vet type-checks a package's _test.go files without running them, and unlike
// `go test -c` it accepts a package pattern, so a second package under the same
// tree cannot silently drop out of the check.
func (t Toolchain) GoVet(opts TestOptions, args ...string) []string {
	argv := make([]string, 0, 5+len(args))
	argv = append(argv, "go", "vet", "-tags", t.tags(opts))
	return append(argv, args...)
}
