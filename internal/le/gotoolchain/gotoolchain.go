// Design: docs/architecture/core-design.md -- the Go toolchain environment le runs every build and test under
//
// Package gotoolchain provides the environment and argv for every le action
// that starts a Go command.
//
// The package DERIVES these values from the checkout instead of defining them
// here. A literal beside the source value in go.mod or feature-gates.txt would
// create two records of one fact. An unchecked copy can drift. The manifest read
// uses internal/le/featuretags, which already reads this value for daemon builds.
//
// Four variables have behavior that their names do not show:
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
//   - GOTOOLCHAIN comes from the toolchain line of go.mod. golangci-lint is a
//     separate binary linked against one version of go/types. It decodes export
//     data that the ambient toolchain writes. If ambient Go is newer than the
//     linter's build version, every package fails to typecheck with
//     "export data version N is greater than maximum supported version M".
//
//     At the same time, the linter prints "0 issues" and exits non-zero. The
//     measurement on 2026-08-22 compared ambient Go 1.27.0 with golangci-lint
//     built with 1.26.6. A warm GOCACHE hides the fault. Therefore, the fault
//     appears only with a cold cache.
//
//   - GOMAXPROCS limits each Go process or test run to a QUARTER of the cores.
//     No process or test run gets more.
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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/featuretags"
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
	features, err := featuretags.DaemonTags(root)
	if err != nil {
		return Toolchain{}, err
	}

	pin, err := goToolchainPin(root)
	if err != nil {
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

// goToolchainPin returns the toolchain go.mod pins, preferring an explicit
// toolchain line and falling back to the go directive.
//
// If go.mod is unreadable, goToolchainPin returns an ERROR. The Python
// implementation returned an empty string in this case. This retained the
// ambient toolchain and was indistinguishable from a checkout that never
// declared a pin. As a result, a lost or unreadable go.mod caused the export-data
// failure that the pin prevents, without a diagnostic.
//
// The go directive is read because a module stating its Go version and no
// separate toolchain line still pins one: `go 1.27.0` means go1.27.0, and
// GOTOOLCHAIN accepts that spelling. Reading only the toolchain line left this
// repository with no pin at all once the toolchain line was dropped, so every le
// action ran under the ambient toolchain and the export-data protection this
// package exists to provide was silently off. An empty answer now means go.mod
// declares neither a toolchain line nor a patch-qualified go directive, and
// no pin is available.
func goToolchainPin(root string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // a build tool reads the checkout it was pointed at
	if err != nil {
		return "", err
	}
	version := ""
	for line := range strings.SplitSeq(string(raw), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		if parts[0] == "toolchain" {
			return parts[1], nil
		}
		if parts[0] == "go" && version == "" {
			version = parts[1]
		}
	}
	// Only a patch-qualified directive names a toolchain. Go refuses a language
	// version: `GOTOOLCHAIN=go1.26` answers "go1.26 is a language version but
	// not a toolchain version (go1.26.x)", which would break every command this
	// package launches. A two-component directive therefore yields no pin, and
	// the export-data protection is unavailable until go.mod states a patch or
	// a toolchain line.
	if strings.Count(version, ".") != 2 {
		return "", nil
	}
	return "go" + version, nil
}

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
// `ZE_LINT_MEMLIMIT=16GiB ./le verify lint run` reaches this setting directly.
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

// Overrides returns the variables that this toolchain SETS. It returns
// KEY=VALUE entries in a fixed order.
//
// Overrides is separate from Environment so that a test can verify
// toolchain-specific values. The inherited environment is machine-specific and
// does not describe the port.
func (t Toolchain) Overrides(opts EnvOptions) []string {
	var tb textbuf.Buffer
	over := make([]string, 0, 8)

	over = append(over, tb.Str("GOCACHE=").Str(GoCache(t.Root)).String())
	tb.Reset()
	over = append(over, tb.Str("GOLANGCI_LINT_CACHE=").
		Str(filepath.Join(t.Root, "tmp", "golangci-lint-cache")).String())

	if opts.CGO {
		over = append(over, "CGO_ENABLED=1")
	} else {
		over = append(over, "CGO_ENABLED=0")
	}
	if t.GoToolchain != "" {
		tb.Reset()
		over = append(over, tb.Str("GOTOOLCHAIN=").Str(t.GoToolchain).String())
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

// Environment answers the whole environment a Go command runs under: this
// process's, with the overrides appended. Later entries win in os/exec, so an
// inherited GOCACHE does not survive.
func (t Toolchain) Environment(opts EnvOptions) []string {
	inherited := os.Environ()
	overrides := t.Overrides(opts)
	full := make([]string, 0, len(inherited)+len(overrides))
	full = append(full, inherited...)
	return append(full, overrides...)
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
