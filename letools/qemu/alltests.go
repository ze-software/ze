// Design: docs/architecture/testing/qemu-integration.md -- what the VM proves
// Related: alltests_report.go -- what one whole run answers
// Related: actions.go -- the verb that reaches this run
//
// alltests.go is the whole ze test suite, run INSIDE the QEMU Linux VM.
//
// It runs in the guest, not on the host. scripts/evidence/qemu-run.py has
// already prepared the guest. It booted Alpine and 9p-mounted the repository at
// /workspace. It also loaded the ppp, l2tp and nft kernel modules and installed
// the Go toolchain. The three ze binaries were cross-compiled on the host and
// shared in over that mount. The remaining task is to run every phase and
// report which phases failed.
//
// Three phases run. The functional suites use concurrency that an 8-vCPU VM can
// carry, rather than the concurrency that a build host can carry. The unit tests
// include the //go:build linux files that never compile on a macOS dev box. The
// integration tests are the VM's whole reason for existing. They exercise
// netlink, nft, fib and procfs code paths that need a real kernel.
//
// THE SUITE LIST IS CHECKED AGAINST THE ONE THE REPOSITORY DECLARES. A run whose
// list has a hole refuses to start. The shell half has 25 hand-written lines,
// but the repository declares 29 suites. Thus runner, flow-export, vpp and web
// never execute in the VM. Four comments in the shell half explain that a suite
// left off the list "would execute NOWHERE"
// (plan/journal/gate-excludes-part-of-its-population.md, 2026-08-26).
package qemu

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/functional"
	"github.com/ze-software/ze/letools/gaterun"
	"github.com/ze-software/ze/letools/leaction"
)

// The guest uses fixed paths. The repository arrives on a 9p mount at a known
// place. The binary shim is a VM-local directory because some UI tests exec
// `ze-stripped` through PATH. mk/test-integration.mk mirrors this path for the
// same reason. The two path definitions must move together.
const (
	guestWorkspace = "/workspace"
	guestBinDir    = "/tmp/ze-qemu-bin"
)

// killAfterFlag is how long `timeout` waits after its own signal before it kills
// the process group, spelled as the flag it is passed as. A stuck ze or plugin
// child cannot wedge the run past it. killAfter names the same duration alone,
// for a reader and for the test that rebuilds the expected command line.
const (
	killAfter     = "15s"
	killAfterFlag = "--kill-after=15s"
)

// The defaults the make targets and this file must agree on. Both spellings of
// each are one decision, and the one here wins whenever the command is invoked
// directly (mk/test-integration.mk).
const (
	defaultParallel = "4"
	defaultTimeout  = "900s"
	defaultSkip     = functionalWeb
)

// functionalWeb is the one suite skipped by default: it drives a browser
// through agent-browser, which the guest does not carry.
const functionalWeb = "web"

// The environment every child is given.
const (
	repoRootKey = "ZE_REPO_ROOT"
	noBuildKey  = "ZE_TEST_NO_BUILD"
	inVMKey     = "ZE_QEMU"
	zeBinKey    = "ZE_BIN"
	pathKey     = "PATH"
)

// The two make variables the unit phase passes on the command line, spelled
// with their assignment so a value is appended rather than concatenated.
const (
	buildCacheAssignment  = "GOCACHE="
	moduleCacheAssignment = "GOMODCACHE="
)

// bgpVerb is the ze-test subcommand the four BGP suites run under. It is the
// same word letools/functional spells for the same reason.
const bgpVerb = "bgp"

// allTests is the ze-test flag that selects every .ci of a suite. It is the
// same word letools/functional spells for the same reason.
const allTests = "--all"

// The concurrency a suite takes. An empty value is the run's own, and takeNoP
// is the suite whose default lives in its own command.
const (
	scaledConcurrency = ""
	serial            = "1"
	takeNoP           = "-"
)

// vmSuite is one functional suite as the VM runs it. It specifies what to pass
// to ze-test and the concurrency to use.
//
// The ARGUMENTS are stated here rather than derived from functional.Suites.
// The two intentionally disagree. This VM runs several suites serially that a
// build host runs in parallel, and all suites share one kernel. The COMPLETENESS
// of this list IS derived from that table. This is the property that a
// hand-written list cannot have (verifySuiteCoverage).
type vmSuite struct {
	Name        string
	Args        []string
	Concurrency string
	// Why records why a suite does not use the run's concurrency. It also records
	// why a suite is in this list when it gates nowhere else.
	Why string
}

// vmSuites is every functional suite, in the ORDER the VM runs them.
//
// The order is behavior in one place. ipsec runs before the IGP suites because
// test/ipsec/ipsec-teardown-leaves-nothing.ci asserts that the XFRM state and
// policy tables are EMPTY after its daemons stop. RFC 4552 tests in test/ospfv3
// program XFRM of their own.
var vmSuites = []vmSuite{
	{Name: "encode", Args: []string{bgpVerb, "encode", allTests}, Concurrency: scaledConcurrency},
	{Name: "plugin", Args: []string{bgpVerb, "plugin", allTests}, Concurrency: scaledConcurrency},
	{Name: "parse", Args: []string{bgpVerb, "parse", allTests}, Concurrency: scaledConcurrency},
	{Name: "decode", Args: []string{bgpVerb, "decode", allTests}, Concurrency: scaledConcurrency},
	{
		Name: "reload", Args: []string{bgpVerb, "reload", allTests}, Concurrency: serial,
		Why: "it shares the VM's one routing table with managed, and each asserts on the whole of it",
	},
	{
		Name: "static", Args: []string{"static", allTests}, Concurrency: serial,
		Why: "every test here programs kernel routes through netlink, so each needs CAP_NET_ADMIN" +
			" and can only run as root; the tests share the VM's one routing table",
	},
	{Name: "ui", Args: []string{"ui", allTests}, Concurrency: scaledConcurrency},
	{
		Name: "editor", Args: []string{"editor"}, Concurrency: takeNoP,
		Why: "the .et editor suite takes the runner's own default",
	},
	{
		Name: "managed", Args: []string{"managed", allTests}, Concurrency: serial,
		Why: "it shares the VM's one routing table with reload",
	},
	{Name: "l2tp", Args: []string{"l2tp", allTests}, Concurrency: scaledConcurrency},
	{
		Name: "firewall", Args: []string{"firewall", allTests}, Concurrency: serial,
		Why: "the VM has no per-test network namespace, so concurrent tests collide on the single" +
			" `inet ze_pr` nft table, the global ip-rule table and the global nft input hook",
	},
	{
		Name: "policy", Args: []string{"policy", allTests}, Concurrency: serial,
		Why: "the same one root namespace as firewall: marks.go makes the fwmark deterministic," +
			" so two tests build the identical rule and the loser gets EEXIST",
	},
	{
		Name: "ipsec", Args: []string{"ipsec", allTests}, Concurrency: serial,
		Why: "the teardown test reads the whole XFRM state and policy table, which is one table" +
			" for the VM; several tests here also share one local address and one IKE port",
	},
	{Name: "install", Args: []string{"install", allTests}, Concurrency: scaledConcurrency},
	{Name: "appliance", Args: []string{"appliance", allTests}, Concurrency: scaledConcurrency},
	{Name: "ldp", Args: []string{"ldp", allTests}, Concurrency: scaledConcurrency},
	{Name: "rsvpte", Args: []string{"rsvpte", allTests}, Concurrency: scaledConcurrency},
	{Name: "isis", Args: []string{"isis", allTests}, Concurrency: scaledConcurrency},
	{Name: "ospf", Args: []string{"ospf", allTests}, Concurrency: scaledConcurrency},
	{Name: "ospfv3", Args: []string{"ospfv3", allTests}, Concurrency: scaledConcurrency},
	{
		Name: "vrrp", Args: []string{"vrrp", allTests}, Concurrency: scaledConcurrency,
		Why: "every VRRP test that boots a daemon can only run here: the iface plugin fails its" +
			" Config stage on darwin, so no VRRP runtime surface exists on the dev machine",
	},
	{Name: "l2tp-wire", Args: []string{"l2tp-wire", allTests}, Concurrency: scaledConcurrency},
	{Name: "isis-wire", Args: []string{"isis-wire", allTests}, Concurrency: scaledConcurrency},
	{Name: "ospf-wire", Args: []string{"ospf-wire", allTests}, Concurrency: scaledConcurrency},
	{
		Name: "traffic", Args: []string{"traffic", allTests}, Concurrency: serial,
		Why: "the needs-linux qdisc tests mutate shared kernel qdisc state on eth0",
	},
	{
		Name: functionalWeb, Args: []string{functionalWeb, allTests}, Concurrency: scaledConcurrency,
		Why: "browser-driven, so the default skip list holds it. It is LISTED rather than omitted," +
			" because a suite nobody lists is a suite nobody notices has stopped running",
	},
	{
		Name: "runner", Args: []string{"runner", allTests}, Concurrency: scaledConcurrency,
		Why: "a GATING suite the hand-written list left out entirely, so it ran in no VM phase" +
			" (2026-08-26)",
	},
	{
		Name: "flow-export", Args: []string{"flow-export", allTests}, Concurrency: serial,
		Why: "sFlow, NetFlow v9 and IPFIX export need the Linux daemon and, for packet sampling," +
			" CAP_NET_ADMIN. Serial, because the tests share one collector port range",
	},
	{
		Name: "vpp", Args: []string{"vpp", allTests}, Concurrency: takeNoP,
		Why: "the VPP stub. It carries no -p because its serial default lives in the command itself",
	},
}

// excludedSuites names any declared suite the VM must NOT run, and why.
//
// It is EMPTY by design. A suite left out of the run needs a reason that a
// reader can check, not an omission. `web` is not here. It is listed above and
// held by the default skip list. An operator who empties that list gets it.
var excludedSuites = map[string]string{}

// integrationPackages are the linux-only test packages, the same set
// `make ze-qemu-integration-test` names.
var integrationPackages = []string{
	// The doctor's integration test is internal/component/doctor
	// (checks_integration_linux_test.go). This entry once read ./cmd/ze/doctor,
	// a directory that does not exist, and every run reported
	// `FAIL ./cmd/ze/doctor [setup failed]` while the check never executed.
	"./internal/component/doctor/...",
	"./internal/component/host/...",
	"./internal/component/iface/...",
	"./internal/component/config/system/...",
	"./internal/core/routewatch/...",
	"./internal/core/network/...",
	"./internal/component/bgp/reactor/...",
	"./internal/plugins/fib/kernel/...",
	"./internal/plugins/firewall/nft/...",
	"./internal/plugins/firewall/vpp/...",
	"./internal/plugins/traffic/netlink/...",
	"./internal/plugins/tftpserver/...",
	"./internal/plugins/dhcpserver/...",
}

// optionalPackages are added when the directory is there. Each is a transport
// whose integration tests need a real kernel and a capability: raw sockets,
// multicast joins, a veth pair.
var optionalPackages = []string{
	"./internal/plugins/isis/transport/...",
	"./internal/plugins/ospf/transport/...",
	"./internal/plugins/ospf/v3/transport/...",
	// VRRP: raw IP proto 112 sockets, the 224.0.0.18 and ff02::12 multicast
	// joins, and the GTSM TTL=255 checks.
	"./internal/plugins/vrrp/transport/...",
	// The root isis package carries the two-engine adjacency test over a real
	// veth pair, so it is named without the /... suffix.
	"./internal/plugins/isis",
}

// The errors a run refuses to start with.
var (
	// ErrNotMounted says the repository is not where the guest expects it.
	ErrNotMounted = errors.New("qemu: the repository is not mounted")
	// ErrIncompleteRun says this run would leave a declared suite executing
	// nowhere.
	ErrIncompleteRun = errors.New("qemu: a declared functional suite is neither run nor excluded")
)

// AllTests is one whole in-VM run.
type AllTests struct {
	// Workspace is where the repository is mounted in the guest.
	Workspace string
	// BinDir is the VM-local directory the binary shim is built in.
	BinDir string
	// The three binaries, as the host named them. A relative path is resolved
	// against the workspace, which is how the make targets pass them.
	ZeBin       string
	StrippedBin string
	TestBin     string
	// Skip names the suites this run must not start.
	Skip []string
	// Parallel is the concurrency a scaled suite takes, and Timeout the
	// wall-clock cap each suite runs under.
	Parallel string
	Timeout  string
	// BuildCache and ModuleCache are handed to make as COMMAND-LINE variables.
	// The Makefile assigns GOCACHE itself, and a makefile assignment beats an
	// environment variable, so an exported value is silently discarded.
	BuildCache  string
	ModuleCache string
	// Run runs one child and answers its exit code. The zero value streams it
	// to the terminal.
	Run func(argv, environ []string) int
	// Note writes one progress line for a person watching. The zero value
	// writes to stderr.
	Note func(line string)
}

// NewAllTests reads the run's knobs from the environment the make target
// exported, and defaults the rest.
func NewAllTests() *AllTests {
	return &AllTests{
		Workspace:   guestWorkspace,
		BinDir:      guestBinDir,
		ZeBin:       envOr(zeBinKey, "bin/ze"),
		StrippedBin: envOr("ZE_STRIPPED_BIN", "bin/ze-stripped"),
		TestBin:     envOr("ZE_TEST_BIN", "bin/ze-test"),
		Skip:        splitList(envOr("ZE_QEMU_SKIP_SUITES", defaultSkip)),
		Parallel:    envOr("ZE_QEMU_PARALLEL", defaultParallel),
		Timeout:     envOr("ZE_QEMU_SUITE_TIMEOUT", defaultTimeout),
		BuildCache:  os.Getenv("GOCACHE"),
		ModuleCache: os.Getenv("GOMODCACHE"),
	}
}

// envOr answers an environment variable, or a fallback when it is unset or
// empty.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// splitList reads a comma-separated knob into its non-empty members.
func splitList(value string) []string {
	var out []string
	for item := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// note writes one progress line through the run's writer, defaulted to stderr.
func (a *AllTests) note(line string) {
	if a.Note == nil {
		gaterun.Note(line)
		return
	}
	a.Note(line)
}

// child runs one command through the run's runner, defaulted to a streaming run
// in the workspace.
func (a *AllTests) child(argv, environ []string) int {
	if a.Run == nil {
		return gaterun.Stream(argv, a.Workspace, environ)
	}
	return a.Run(argv, environ)
}

// Execute runs every phase and returns the report and the process exit code.
//
// Execute checks EVERY precondition before the first child starts. If a run
// discovers halfway through that it cannot answer, it has already spent an
// hour of VM time. Its partial result reads like a test failure.
func (a *AllTests) Execute() (AllTestsReport, int) {
	if err := a.verify(); err != nil {
		leaction.ReportError(err)
		return AllTestsReport{}, 1
	}
	if err := a.shim(); err != nil {
		leaction.ReportError(err)
		return AllTestsReport{}, 1
	}

	environ := a.environment()
	var report AllTestsReport

	for _, suite := range vmSuites {
		report.add(a.suite(suite, environ))
	}
	report.add(a.unitPhase(environ))

	integration, err := a.integrationPhase(environ)
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	report.add(integration)

	if len(report.Failed) > 0 {
		return report, 1
	}
	return report, 0
}

// verify rejects any run that was unable to answer honestly.
func (a *AllTests) verify() error {
	if info, err := os.Stat(a.Workspace); err != nil || !info.IsDir() {
		var tb textbuf.Buffer
		return errors.New(tb.Err(ErrNotMounted).Str(": ").Str(a.Workspace).String())
	}

	for _, bin := range []string{a.ZeBin, a.StrippedBin, a.TestBin} {
		info, err := os.Stat(a.workspacePath(bin))
		if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			var tb textbuf.Buffer
			return errors.New(tb.Str("qemu: ").Str(bin).
				Str(" is missing or not executable -- cross-compile it on the host first").String())
		}
	}

	if a.BuildCache == "" || a.ModuleCache == "" {
		return errors.New("qemu: GOCACHE and GOMODCACHE must both be named: the Makefile assigns" +
			" GOCACHE itself, so an unnamed cache compiles through a host-only symlink")
	}

	if err := a.verifySuiteCoverage(); err != nil {
		return err
	}
	_, err := a.integrationArgs()
	return err
}

// verifySuiteCoverage refuses a run that would leave a declared suite executing
// nowhere. It is the guard the shell's hand-written list does not have.
func (a *AllTests) verifySuiteCoverage() error {
	listed := make(map[string]bool, len(vmSuites))
	for _, suite := range vmSuites {
		listed[suite.Name] = true
	}

	var missing []string
	for _, suite := range functional.Suites {
		if !listed[suite.Name] && excludedSuites[suite.Name] == "" {
			missing = append(missing, suite.Name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	var tb textbuf.Buffer
	return errors.New(tb.Err(ErrIncompleteRun).Str(": ").Join(missing, ", ").String())
}

// workspacePath resolves a binary path in the same way that the make targets
// pass it. An absolute path is unchanged. A relative path is relative to the
// workspace.
func (a *AllTests) workspacePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(a.Workspace, path)
}

// shim builds the VM-local directory of stable binary names.
//
// Some UI tests exec `ze-stripped` through PATH, and the host cross-compiles to
// arch-suffixed names so that bin/ze stays the host-native binary.
func (a *AllTests) shim() error {
	if err := os.MkdirAll(a.BinDir, 0o750); err != nil {
		return err
	}
	for name, target := range map[string]string{
		"ze":          a.workspacePath(a.ZeBin),
		"ze-stripped": a.workspacePath(a.StrippedBin),
		"ze-test":     a.workspacePath(a.TestBin),
	} {
		link := filepath.Join(a.BinDir, name)
		if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Symlink(target, link); err != nil {
			return err
		}
	}
	return nil
}

// environment is what every child of this run is given.
//
// ZE_REPO_ROOT is STATED rather than left to be derived. The repo-invariant
// tests derive the tree as the parent of the directory that holds `ze`. This is
// correct for a plain checkout and wrong here. The PATH shim puts `ze` in a
// scratch directory whose parent is not the repository. Those tests then grep
// paths that do not exist. They find nothing, read no source, and fail.
func (a *AllTests) environment() []string {
	environ := os.Environ()
	environ = setEnv(environ, noBuildKey, "1")
	environ = setEnv(environ, inVMKey, "1")
	environ = setEnv(environ, repoRootKey, a.Workspace)
	environ = setEnv(environ, zeBinKey, filepath.Join(a.BinDir, "ze"))

	var tb textbuf.Buffer
	return setEnv(environ, pathKey, tb.Str(a.BinDir).Byte(':').Str(os.Getenv(pathKey)).String())
}

// setEnv replaces or appends one variable of an environment block.
//
// Two buffers are necessary. String() does not leave the first buffer with the
// text that it rendered. If the code appends the value to the buffer that
// supplied the prefix, the resulting entry contains only the value. Every
// variable that this function sets then reaches the child without a name.
func setEnv(environ []string, key, value string) []string {
	var prefixBuf textbuf.Buffer
	prefix := prefixBuf.Str(key).Byte('=').String()

	var entryBuf textbuf.Buffer
	entry := entryBuf.Str(prefix).Str(value).String()

	for i, existing := range environ {
		if strings.HasPrefix(existing, prefix) {
			environ[i] = entry
			return environ
		}
	}
	return append(environ, entry)
}

// suite runs one functional suite, or reports it skipped.
func (a *AllTests) suite(suite vmSuite, environ []string) PhaseResult {
	var tb textbuf.Buffer
	name := tb.Str("functional/").Str(suite.Name).String()

	if slices.Contains(a.Skip, suite.Name) {
		return PhaseResult{Name: name, Skipped: true, Reason: "ZE_QEMU_SKIP_SUITES"}
	}

	argv := a.suiteCommand(suite)
	a.note(banner(name))
	return PhaseResult{Name: name, Command: argv, Code: a.child(argv, environ)}
}

// suiteCommand is the command line one suite runs under.
//
// `timeout` runs the suite in its own process group. On expiry, it kills the
// whole group. Thus a stuck ze or plugin child cannot wedge the run.
func (a *AllTests) suiteCommand(suite vmSuite) []string {
	argv := make([]string, 0, len(suite.Args)+6)
	argv = append(argv, "timeout", killAfterFlag, a.Timeout, filepath.Join(a.BinDir, "ze-test"))
	argv = append(argv, suite.Args...)

	switch suite.Concurrency {
	case takeNoP:
		return argv
	case scaledConcurrency:
		return append(argv, "-p", a.Parallel)
	default:
		return append(argv, "-p", suite.Concurrency)
	}
}

// unitPhase runs the full unit pass without -race. The Alpine image ships no C
// compiler, and the race detector needs CGO. Native runs provide race coverage.
// This phase adds the //go:build linux test files that never compile on a macOS
// dev box.
//
// unitPhase calls the -impl body DIRECTLY. It bypasses the job-admission wrapper
// that the public target carries. That wrapper must not run here.
//
// The wrapper registry is under the repository's tmp/, which the guest shares
// over 9p. The host and guest have DISJOINT pid namespaces. If a guest reads a
// host holder's entry, it finds no such pid and reaps the slot as dead. The VM
// is single-tenant, and its whole job is this run. There is nothing to admit.
//
// ze-scratch-links-ensure is named explicitly. It is a prerequisite of the
// PUBLIC target, but not of the -impl body. A direct call to an -impl skips the
// prerequisites that the public half declared.
func (a *AllTests) unitPhase(environ []string) PhaseResult {
	var cache textbuf.Buffer
	var module textbuf.Buffer
	argv := []string{
		"make", "--no-print-directory",
		cache.Str(buildCacheAssignment).Str(a.BuildCache).String(),
		module.Str(moduleCacheAssignment).Str(a.ModuleCache).String(),
		// ./scripts/... is host developer tooling with no linux-only surface,
		// and in the guest it fails on Alpine having neither brew nor apt.
		"ZE_PACKAGES_EXCLUDE=/scripts/",
		"ze-scratch-links-ensure", "_ze-unit-test-cached-impl",
	}
	const name = "unit tests (no -race, cacheable)"
	a.note(banner(name))
	return PhaseResult{Name: name, Command: argv, Code: a.child(argv, environ)}
}

// integrationPhase runs the linux-only, integration-tagged tests.
func (a *AllTests) integrationPhase(environ []string) (PhaseResult, error) {
	argv, err := a.integrationArgs()
	if err != nil {
		return PhaseResult{}, err
	}
	const name = "integration tests (-tags integration)"
	a.note(banner(name))
	return PhaseResult{Name: name, Command: argv, Code: a.child(argv, environ)}, nil
}

// integrationArgs builds the integration command and refuses a package list
// naming a path that is not there.
//
// A path typo is a bug in THIS file, not a test failure. `go test` reports a
// missing directory as `FAIL <pkg> [setup failed]` among real results. Thus,
// ./cmd/ze/doctor remained broken here when the phase only looked red.
func (a *AllTests) integrationArgs() ([]string, error) {
	tags, err := a.integrationTags()
	if err != nil {
		return nil, err
	}

	var missing []string
	for _, pkg := range integrationPackages {
		if !a.hasPackage(pkg) {
			missing = append(missing, pkg)
		}
	}
	if len(missing) > 0 {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("qemu: the integration package list names path(s) that do not exist: ").
			Join(missing, " ").Str(" -- this is a bug in letools/qemu, not a test failure").String())
	}

	packages := append([]string(nil), integrationPackages...)
	for _, optional := range optionalPackages {
		if a.hasPackage(optional) {
			packages = append(packages, optional)
		}
	}

	argv := make([]string, 0, len(packages)+9)
	argv = append(argv, "env", "CGO_ENABLED=0", "go", "test", "-tags", tags,
		"-count=1", "-timeout", "120s")
	return append(argv, packages...), nil
}

// hasPackage reports whether a package pattern names a directory of the
// workspace.
func (a *AllTests) hasPackage(pkg string) bool {
	dir := strings.TrimSuffix(pkg, "/...")
	info, err := os.Stat(filepath.Join(a.Workspace, dir))
	return err == nil && info.IsDir()
}

// integrationTags answers the build tags the integration pass compiles with.
//
// `-tags integration` ADDS the integration files to a package. It does not
// replace the package's ordinary unit tests, which also compile and run.
//
// Without ze_core and the feature set that they usually get, every
// feature-gated surface silently vanishes. The tests that assert on these
// surfaces then fail. Four doctor listener tests once failed because ze_ssh
// alone was absent. The set is DERIVED from feature-gates.txt, so it cannot
// drift from the Makefile's own set. A manifest that is not there is an ERROR,
// not a smaller tag set.
func (a *AllTests) integrationTags() (string, error) {
	body, err := os.ReadFile(filepath.Join(a.Workspace, "feature-gates.txt")) //nolint:gosec // a fixed path of the checkout
	if err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("qemu: feature-gates.txt could not be read, and without it every").
			Str(" feature-gated surface vanishes from the integration build: ").Err(err).String())
	}

	seen := make(map[string]bool)
	var gates []string
	for line := range strings.SplitSeq(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "ze_") || seen[fields[0]] {
			continue
		}
		seen[fields[0]] = true
		gates = append(gates, fields[0])
	}
	sort.Strings(gates)

	var tb textbuf.Buffer
	tb.Str("ze_core integration")
	for _, gate := range gates {
		tb.Byte(' ').Str(gate)
	}
	return tb.String(), nil
}

// banner is the heading one phase prints before it runs.
func banner(name string) string {
	var tb textbuf.Buffer
	return tb.Str("========================= ").Str(name).Str(" =========================").String()
}
