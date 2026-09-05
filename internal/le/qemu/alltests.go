// Design: docs/architecture/testing/qemu-integration.md -- what the VM proves
// Related: alltests_report.go -- what one whole run answers
// Related: alltests_tally.go -- how many tests a suite actually executed
// Related: netns.go -- which suites run outside the transport's namespace
// Related: actions.go -- the verb that reaches this run
//
// alltests.go is the whole ze test suite, run INSIDE the QEMU Linux VM.
//
// It runs in the guest, not on the host. The native qemu run action has
// already prepared the guest: it booted Alpine, 9p-mounted the repository at
// /workspace, loaded the ppp, l2tp and nft kernel modules, and installed the Go
// toolchain. The three ze binaries and the le personality were cross-compiled
// on the host and shared in over that mount. The remaining task is to run every
// phase and report which phases failed.
//
// Four phases run. The functional suites use concurrency that an 8-vCPU VM can
// carry, rather than the concurrency that a build host can carry. The unit tests
// include the //go:build linux files that never compile on a macOS dev box. The
// installer tests are behind ze_installer, a personality tag no other phase's
// tag set names, so nothing else in the tree compiles them. The integration
// tests are the VM's whole reason for existing. They exercise netlink, nft, fib
// and procfs code paths that need a real kernel.
//
// THE SUITE LIST IS CHECKED AGAINST THE ONE THE REPOSITORY DECLARES. A run whose
// list has a hole refuses to start. The retired shell helper had 25 hand-written
// lines while the repository declared 29 suites, so runner, flow-export, vpp,
// and web never executed in the VM
// (plan/journal/gate-excludes-part-of-its-population.md, 2026-08-26).
package qemu

import (
	"debug/elf"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/functional"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/population"
)

// The guest uses fixed paths. The repository arrives on a 9p mount at a known
// place. The binary shim is a VM-local directory because some UI tests exec
// `ze-stripped` through PATH. The build and guest command use this one path.
const (
	guestWorkspace = "/workspace"
	guestBinDir    = "/tmp/ze-qemu-bin"
)

// killAfterFlag and killAfterSeconds are how long `timeout` waits after its own
// signal before it kills the process group. A stuck ze or plugin child cannot
// wedge the run past it.
//
// The spelling is the GUEST's. /usr/bin/timeout in the Alpine guest is a symlink
// to BusyBox 1.37.0, whose usage is `timeout [-s SIG] [-k KILL_SECS] SECS PROG
// ARGS`. It answers `timeout: unrecognized option: kill-after=15s` to the GNU
// long form and exits 1, so every suite printed the BusyBox usage under its own
// header and ran no test at all, and a reader who greps the log for a suite name
// finds it (plan/journal/gate-excludes-part-of-its-population.md, 2026-09-05).
// `-k 15` is accepted by BusyBox and by GNU coreutils, so one spelling serves a
// guest with either, and the wrapper no longer depends on `packages coreutils`.
// Both forms were run in the guest before this was written.
const (
	killAfterFlag    = "-k"
	killAfterSeconds = "15"
)

// These defaults belong to the native guest action. The host passes explicit
// values when it needs a different concurrency, timeout, or skip set.
const (
	defaultParallel = "4"
	defaultTimeout  = "900s"
	defaultSkip     = functionalWeb
)

// functionalWeb is the one suite skipped by default: it drives a browser
// through agent-browser, which the guest does not carry.
const functionalWeb = "web"

// The stable names the shim and the capability copies carry. A tool dispatches
// on its basename, so these are what a test execs through PATH and what ZE_BIN
// and ZE_STRIPPED_BIN name.
const (
	zeName         = "ze"
	zeStrippedName = "ze-stripped"
	zeTestName     = "ze-test"
)

// The environment every child is given.
const (
	repoRootKey    = "ZE_REPO_ROOT"
	noBuildKey     = "ZE_TEST_NO_BUILD"
	inVMKey        = "ZE_QEMU"
	zeBinKey       = "ZE_BIN"
	strippedBinKey = "ZE_STRIPPED_BIN"
	pathKey        = "PATH"
)

// The needs-linux selection.
//
// linuxOnlyKey is the variable the .ci runner reads to skip every test that
// carries no `option=needs-linux` marker (parseAndAdd,
// internal/test/runner/record_parse.go). The filter therefore lives in the
// runner and the run only says which population it wants. linuxOnlySelection
// is how a caller asks for it, and how the report records that it was asked
// for.
const (
	linuxOnlyKey       = "ZE_QEMU_LINUX_ONLY"
	linuxOnlyValue     = "1"
	linuxOnlySelection = "needs-linux"
)

// bgpVerb is the ze-test subcommand the four BGP suites run under. It is the
// same word internal/le/functional spells for the same reason.
const bgpVerb = "bgp"

// tagsFlag is the go command's build-tag selector. Three phases pass one, and
// they deliberately pass different sets: the unit and integration phases carry
// the feature manifest's, and the installer phase carries the initrd's own.
const tagsFlag = "-tags"

// allTests is the ze-test flag that selects every .ci of a suite. It is the
// same word internal/le/functional spells for the same reason.
const allTests = "--all"

// The concurrency a suite takes. An empty value is the run's own, and takeNoP
// is the suite whose default lives in its own command.
const (
	scaledConcurrency = ""
	serial            = "1"
	takeNoP           = "-"
)

// vmSuite is one functional suite as the VM runs it. It specifies what to pass
// to ze-test, the concurrency to use, and the network namespace its tests get.
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
	// Namespace is the network namespace this suite's tests run in. Every row
	// states it, and namespaceUnspecified is refused before the run starts.
	Namespace networkNamespace
	// Why records why a suite does not use the run's concurrency, or why it does
	// not run in the guest root namespace. It also records why a suite is in
	// this list when it gates nowhere else.
	Why string
}

// vmSuites is every functional suite, in the ORDER the VM runs them.
//
// The order is behavior in one place. ipsec runs before the IGP suites because
// test/ipsec/ipsec-teardown-leaves-nothing.ci asserts that the XFRM state and
// policy tables are EMPTY after its daemons stop. RFC 4552 tests in test/ospfv3
// program XFRM of their own.
var vmSuites = []vmSuite{
	{Name: "encode", Args: []string{bgpVerb, "encode", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{Name: "plugin", Args: []string{bgpVerb, "plugin", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{Name: "parse", Args: []string{bgpVerb, "parse", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{Name: "decode", Args: []string{bgpVerb, "decode", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{
		Name: "reload", Args: []string{bgpVerb, "reload", allTests}, Concurrency: serial, Namespace: guestRoot,
		Why: "it shares the VM's one routing table with managed, and each asserts on the whole of it",
	},
	{
		Name: "static", Args: []string{"static", allTests}, Concurrency: serial, Namespace: guestRoot,
		Why: "every test here programs kernel routes through netlink, so each needs CAP_NET_ADMIN" +
			" and can only run as root; the tests share the VM's one routing table",
	},
	{Name: "ui", Args: []string{"ui", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{
		Name: "editor", Args: []string{"editor"}, Concurrency: takeNoP, Namespace: guestRoot,
		Why: "the .et editor suite takes the runner's own default",
	},
	{
		Name: "managed", Args: []string{"managed", allTests}, Concurrency: serial, Namespace: guestRoot,
		Why: "it shares the VM's one routing table with reload",
	},
	{Name: "l2tp", Args: []string{"l2tp", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{
		Name: "firewall", Args: []string{"firewall", allTests}, Concurrency: serial, Namespace: perTest,
		Why: "firewall-nat-exclude installs a nat prerouting chain, and in the guest root namespace" +
			" that chain sits under the SSH transport carrying the run: the 2026-09-05 run died there" +
			" with ssh's own exit 255 and every later phase went unaccounted. Serial because the" +
			" launcher has only ever been exercised one test at a time (./le qemu netns-test)",
	},
	{
		Name: "policy", Args: []string{"policy", allTests}, Concurrency: serial, Namespace: perTest,
		Why: "it programs ip rules and a deterministic fwmark (marks.go) that reach every packet in" +
			" the namespace they are installed in, the transport's included. Serial for firewall's reason",
	},
	{
		Name: "ipsec", Args: []string{"ipsec", allTests}, Concurrency: serial, Namespace: guestRoot,
		Why: "the teardown test reads the whole XFRM state and policy table, which is one table" +
			" for the VM; several tests here also share one local address and one IKE port",
	},
	{Name: "install", Args: []string{"install", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{Name: "appliance", Args: []string{"appliance", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{Name: "ldp", Args: []string{"ldp", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{Name: "rsvpte", Args: []string{"rsvpte", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{Name: "isis", Args: []string{"isis", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{
		Name: "ospf", Args: []string{"ospf", allTests}, Concurrency: scaledConcurrency, Namespace: perTest,
		Why: "its tests declare option=netns-link and provision eth0, eth1, nbma0 and ptmp0. The" +
			" runner SKIPS a netns-link test outside this mode (applyNetnsLinkGate), so eight of them" +
			" executed in no VM phase, and creating those names in the guest root namespace is the" +
			" one thing the mode exists to prevent",
	},
	{
		Name: "ospfv3", Args: []string{"ospfv3", allTests}, Concurrency: scaledConcurrency, Namespace: perTest,
		Why: "three netns-link tests, skipped outside this mode for ospf's reason",
	},
	{
		Name: "vrrp", Args: []string{"vrrp", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot,
		Why: "every VRRP test that boots a daemon can only run here: the iface plugin fails its" +
			" Config stage on darwin, so no VRRP runtime surface exists on the dev machine",
	},
	{Name: "l2tp-wire", Args: []string{"l2tp-wire", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{Name: "isis-wire", Args: []string{"isis-wire", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{Name: "ospf-wire", Args: []string{"ospf-wire", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot},
	{
		Name: "traffic", Args: []string{"traffic", allTests}, Concurrency: serial, Namespace: guestRoot,
		Why: "the needs-linux qdisc tests mutate shared kernel qdisc state on eth0, which is the" +
			" transport's own interface: they shape it and none of them drops a packet",
	},
	{
		Name: functionalWeb, Args: []string{functionalWeb, allTests}, Concurrency: scaledConcurrency,
		Namespace: guestRoot,
		Why: "browser-driven, so the default skip list holds it. It is LISTED rather than omitted," +
			" because a suite nobody lists is a suite nobody notices has stopped running",
	},
	{
		Name: "runner", Args: []string{"runner", allTests}, Concurrency: scaledConcurrency, Namespace: guestRoot,
		Why: "a GATING suite the hand-written list left out entirely, so it ran in no VM phase" +
			" (2026-08-26)",
	},
	{
		Name: "flow-export", Args: []string{"flow-export", allTests}, Concurrency: serial, Namespace: guestRoot,
		Why: "sFlow, NetFlow v9 and IPFIX export need the Linux daemon and, for packet sampling," +
			" CAP_NET_ADMIN. Serial, because the tests share one collector port range",
	},
	{
		Name: "vpp", Args: []string{"vpp", allTests}, Concurrency: takeNoP, Namespace: guestRoot,
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
// `./le qemu all-tests` names.
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
	"./internal/plugins/traffic/vpp/...",
	"./internal/plugins/traffic/netlink/...",
	"./internal/plugins/tftpserver/...",
	"./internal/plugins/dhcpserver/...",

	// Added 2026-08-29. Each of these carries `//go:build integration` test
	// files and was named by no runner, so its tests compiled under the lint
	// matrix and executed nowhere. TestEveryIntegrationPackageIsNamed found
	// them by deriving the population from the tree; 19 of the 36 packages
	// holding integration tests were absent.
	//
	// Written without the /... suffix, like ./internal/plugins/isis above: the
	// package named IS the one holding the tests, and a suffix here would
	// subsume siblings that optionalPackages already names separately.
	"./cmd/ze/hub",
	"./internal/chaos/peer",
	"./internal/component/ike/dataplane",
	"./internal/component/ike/engine",
	"./internal/component/l2tp",
	"./internal/component/l2tp/pppoeclient",
	"./internal/component/telemetry/collector",
	"./internal/core/dnsserver",
	"./internal/core/smart",
	"./internal/exabgp/bridge",
	"./internal/plugins/flowexport/conntrack",
	"./internal/plugins/flowexport/sampling",
	"./internal/plugins/iface/dhcp",
	"./internal/plugins/iface/netlink",
	"./internal/plugins/iface/ra",
	"./internal/plugins/ldp",
	"./internal/plugins/ntp",
	"./internal/plugins/ospf",
	"./internal/plugins/static",
	"./internal/plugins/trafficusage",
}

// excludedIntegrationPackages names a package that holds integration tests and
// is deliberately not run, against the reason it is not.
//
// It is EMPTY by design, for the reason excludedSuites is: a package left out
// of the run needs a reason a reader can check, not an omission. The map is
// what TestEveryIntegrationPackageIsNamed accepts in place of membership, so
// excluding one is a decision somebody writes down rather than a silence.
var excludedIntegrationPackages = map[string]string{
	"internal/plugins/as112": "runs as its own verb, `./le integration as112`" +
		" (internal/le/integration/gates.go), so naming it here would run it twice",
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
	// ErrNamespaceUnspecified says a suite does not state which network
	// namespace its tests run in.
	ErrNamespaceUnspecified = errors.New("qemu: a suite does not name the network namespace it runs in")
	// ErrNamespacePreparation says the per-test namespace launcher could not be
	// given the capability copies it needs.
	ErrNamespacePreparation = errors.New("qemu: the per-test network namespace could not be prepared")
)

// allTestsRun is one whole in-VM run.
type allTestsRun struct {
	// Workspace is where the repository is mounted in the guest.
	Workspace string
	// BinDir is the VM-local directory the binary shim is built in.
	BinDir string
	// The three binaries, as the host named them. A relative path is resolved
	// against the workspace, which is how the native host action passes them.
	ZeBin       string
	StrippedBin string
	TestBin     string
	// Skip names the suites this run must not start.
	Skip []string
	// LinuxOnly runs only the .ci tests marked `option=needs-linux`. It is the
	// tight loop for a developer who changed one Linux-only path: the whole
	// suite list still starts, and each suite runs the marked tests alone.
	//
	// It filters the FUNCTIONAL suites and nothing else. The unit, installer
	// and integration phases compile and run whole, so the report says the run
	// was filtered rather than leaving a reader to infer it.
	LinuxOnly bool
	// Parallel is the concurrency a scaled suite takes, and Timeout the
	// wall-clock cap each suite runs under.
	Parallel string
	Timeout  string
	// BuildCache and ModuleCache are passed directly in the child environment.
	BuildCache  string
	ModuleCache string
	// Run runs one child and answers its exit code. The zero value streams it
	// to the terminal. tally, when it is not nil, receives a second copy of the
	// child's stdout so the run can read the count out of it.
	Run func(argv, environ []string, tally io.Writer) int
	// Look answers where a command the run needs lives. The zero value is
	// exec.LookPath.
	Look func(name string) (string, error)
	// Note writes one progress line for a person watching. The zero value
	// writes to stderr.
	Note func(line string)
}

// newAllTests reads the run's knobs from the environment the native host action
// exports, and defaults the rest.
func newAllTests() *allTestsRun {
	return &allTestsRun{
		Workspace:   guestWorkspace,
		BinDir:      guestBinDir,
		ZeBin:       envOr(zeBinKey, "bin/ze"),
		StrippedBin: envOr("ZE_STRIPPED_BIN", "bin/ze-stripped"),
		TestBin:     envOr("ZE_TEST_BIN", "bin/ze-test"),
		Skip:        splitList(envOr("ZE_QEMU_SKIP_SUITES", defaultSkip)),
		LinuxOnly:   os.Getenv(linuxOnlyKey) == linuxOnlyValue,
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
func (a *allTestsRun) note(line string) {
	if a.Note == nil {
		gaterun.Note(line)
		return
	}
	a.Note(line)
}

// child runs one command through the run's runner, defaulted to a streaming run
// in the workspace. tally may be nil, and is nil for every phase that reports
// no test count of its own.
func (a *allTestsRun) child(argv, environ []string, tally io.Writer) int {
	if a.Run == nil {
		return gaterun.StreamTee(argv, a.Workspace, environ, tally)
	}
	return a.Run(argv, environ, tally)
}

// look answers where a command lives, through the run's lookup, defaulted to
// the guest's PATH.
func (a *allTestsRun) look(name string) (string, error) {
	if a.Look == nil {
		return exec.LookPath(name)
	}
	return a.Look(name)
}

// The three phases that are not a functional suite. The names are constants
// because the run states its whole population before the first child starts,
// and a phase whose planned name and reported name disagree would read as one
// that never ran.
const (
	unitPhaseName        = "unit tests (no -race, cacheable)"
	installerPhaseName   = "installer initrd tests (-tags ze_core ze_installer)"
	integrationPhaseName = "integration tests (-tags integration)"
)

// suitePhaseName is the phase name one functional suite reports under.
func suitePhaseName(suite string) string {
	var tb textbuf.Buffer
	return tb.Str("functional/").Str(suite).String()
}

// plannedPhases names every phase this run intends to reach, in order.
//
// It is stated BEFORE the first child, so a run that dies halfway can be told
// apart from one that answered for its whole population. The 2026-09-05 run
// died inside functional/firewall and said nothing at all about the twelve
// suites, the unit pass, the installer phase and the integration phase that
// were still to come.
func plannedPhases() []string {
	planned := make([]string, 0, len(vmSuites)+3)
	for _, suite := range vmSuites {
		planned = append(planned, suitePhaseName(suite.Name))
	}
	return append(planned, unitPhaseName, installerPhaseName, integrationPhaseName)
}

// Execute runs every phase and returns the report and the process exit code.
//
// Execute checks EVERY precondition before the first child starts. If a run
// discovers halfway through that it cannot answer, it has already spent an
// hour of VM time. Its partial result reads like a test failure.
func (a *allTestsRun) Execute() (AllTestsReport, int) {
	if err := a.verify(); err != nil {
		leaction.ReportError(err)
		return AllTestsReport{}, 1
	}
	if err := a.shim(); err != nil {
		leaction.ReportError(err)
		return AllTestsReport{}, 1
	}

	environ := a.environment()
	report := AllTestsReport{Planned: plannedPhases()}
	if a.LinuxOnly {
		report.Selection = linuxOnlySelection
	}
	a.note(plan(report.Planned))

	if err := a.prepareNamespace(environ); err != nil {
		leaction.ReportError(err)
		return report, 1
	}

	for _, suite := range vmSuites {
		report.add(a.suite(suite, environ))
	}
	unit, err := a.unitPhase(environ)
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	report.add(unit)
	report.add(a.installerPhase(environ))

	integration, err := a.integrationPhase(environ)
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	report.add(integration)

	if len(report.Failed) > 0 || len(report.Unreached()) > 0 {
		return report, 1
	}
	return report, 0
}

// verify rejects any run that was unable to answer honestly.
func (a *allTestsRun) verify() error {
	if info, err := os.Stat(a.Workspace); err != nil || !info.IsDir() {
		var tb textbuf.Buffer
		return errors.New(tb.Err(ErrNotMounted).Str(": ").Str(a.Workspace).String())
	}

	for _, bin := range []string{a.ZeBin, a.StrippedBin, a.TestBin} {
		path := a.workspacePath(bin)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			var tb textbuf.Buffer
			return errors.New(tb.Str("qemu: ").Str(bin).
				Str(" is missing or not executable -- cross-compile it on the host first").String())
		}
		if err := runnableInGuest(path); err != nil {
			var tb textbuf.Buffer
			return errors.New(tb.Str("qemu: ").Str(bin).Str(": ").Err(err).String())
		}
	}

	if a.BuildCache == "" || a.ModuleCache == "" {
		return errors.New("qemu: GOCACHE and GOMODCACHE must both be named by the native toolchain; " +
			"an unnamed cache can compile through a host-only symlink")
	}

	if err := a.verifySuiteCoverage(); err != nil {
		return err
	}
	if err := a.verifyNamespaces(); err != nil {
		return err
	}
	_, err := a.integrationArgs()
	return err
}

// loaderNameMax bounds the dynamic loader's name in a refusal. A path is
// shorter than PATH_MAX, and the ELF header declares its own length, so the
// read is bounded here rather than by what the file claims.
const loaderNameMax = 4096

// runnableInGuest refuses a binary the guest cannot exec.
//
// os.Stat answers that a file exists and carries an execute bit. It cannot see
// the one thing that actually stopped a run: a `ze` built on a glibc host names
// /lib64/ld-linux-x86-64.so.2 in its PT_INTERP header, musl Alpine has no such
// loader, and the kernel answers ENOENT for the LOADER while naming the BINARY.
// The 2026-09-04 run reported that as 326 identical per-test failures
// (`start ze: fork/exec ...: no such file or directory`) rather than as one
// broken precondition, and every one of them read like a product defect.
//
// A statically linked Go binary carries no PT_INTERP at all, which is what
// CGO_ENABLED=0 produces and what the guest needs.
func runnableInGuest(path string) error {
	file, err := elf.Open(path)
	if err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("is not a Linux ELF binary (").Err(err).
			Str(") -- cross-compile it with GOOS=linux CGO_ENABLED=0").String())
	}
	defer file.Close() //nolint:errcheck // read-only open, and the caller is about to refuse the run

	for _, prog := range file.Progs {
		if prog.Type != elf.PT_INTERP {
			continue
		}
		// The header declares its own length, so the read is bounded by a path
		// length rather than by what the file claims. The refusal does not
		// depend on the name being complete.
		loader := make([]byte, min(prog.Filesz, loaderNameMax))
		if _, err := prog.ReadAt(loader, 0); err != nil {
			return errors.New("names a dynamic loader that could not be read -- rebuild it with CGO_ENABLED=0")
		}
		var tb textbuf.Buffer
		return errors.New(tb.Str("is dynamically linked against ").
			Str(strings.TrimRight(string(loader), "\x00")).
			Str(", which the musl guest does not have -- rebuild it with CGO_ENABLED=0").String())
	}
	return nil
}

// suiteCoverage accounts for every declared functional suite against the ones
// this run lists.
//
// The population is functional.Suites, which is the repository's own declaration
// and not derived from vmSuites, so the accounting cannot be satisfied by the
// list under test agreeing with itself.
func suiteCoverage() (population.Coverage, error) {
	declared := make(map[string]bool, len(functional.Suites))
	for _, suite := range functional.Suites {
		declared[suite.Name] = true
	}
	listed := make(map[string]bool, len(vmSuites))
	for _, suite := range vmSuites {
		listed[suite.Name] = true
	}
	claim := population.Claim{
		Subject:         "declared functional suite",
		Population:      declared,
		Walked:          listed,
		Excused:         excludedSuites,
		UnexcusedReason: "RUNS IN NO VM PHASE",
	}
	return claim.Assess()
}

// verifySuiteCoverage refuses a run that would leave a declared suite executing
// nowhere. It is the guard the shell's hand-written list does not have.
//
// It also refuses a stale entry in excludedSuites. An exclusion whose suite the
// VM now runs, or whose suite no longer exists, is a statement nobody rechecked,
// and it hides the next suite to take that name.
func (a *allTestsRun) verifySuiteCoverage() error {
	coverage, err := suiteCoverage()
	if err != nil {
		return err
	}
	if coverage.Code == 0 {
		return nil
	}
	var tb textbuf.Buffer
	if len(coverage.Unexcused) != 0 {
		return errors.New(tb.Err(ErrIncompleteRun).Str(": ").Join(coverage.Unexcused, ", ").String())
	}
	return errors.New(tb.Str("excludedSuites names a suite the VM already runs, or one that is not declared: ").
		Join(coverage.Healed, ", ").String())
}

// workspacePath resolves a binary path in the same way that the native host
// action passes it. An absolute path is unchanged. A relative path is relative
// to the workspace.
func (a *allTestsRun) workspacePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(a.Workspace, path)
}

// shim builds the VM-local directory of stable binary names.
//
// Some UI tests exec `ze-stripped` through PATH, and the host cross-compiles to
// arch-suffixed names so that bin/ze stays the host-native binary.
//
// The directory is TRAVERSABLE BY EVERY USER, not only by root. A suite in the
// per-test network namespace runs ze as an ordinary user, ze relays a plugin
// through `ze-test` on this PATH, and a 0750 root-owned directory answers that
// exec with `/bin/sh: ze-test: Permission denied`. The plugin then never
// starts and the test times out on a symptom that names neither the directory
// nor the user: measured on 2026-09-05, 18 test/ospf tests timed out that way.
// The links point into the read-only checkout, so a wider directory exposes
// nothing the guest does not already share.
func (a *allTestsRun) shim() error {
	if err := os.MkdirAll(a.BinDir, 0o755); err != nil { //nolint:gosec // see the comment above: the dropped user must exec through it
		return err
	}
	// MkdirAll leaves an EXISTING directory's mode alone, and a previous run in
	// the same guest made this one 0750.
	if err := os.Chmod(a.BinDir, 0o755); err != nil { //nolint:gosec // same reason
		return err
	}
	for name, target := range map[string]string{
		zeName:         a.workspacePath(a.ZeBin),
		zeStrippedName: a.workspacePath(a.StrippedBin),
		zeTestName:     a.workspacePath(a.TestBin),
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
func (a *allTestsRun) environment() []string {
	environ := os.Environ()
	environ = setEnv(environ, noBuildKey, "1")
	environ = setEnv(environ, inVMKey, "1")
	environ = setEnv(environ, repoRootKey, a.Workspace)
	environ = setEnv(environ, zeBinKey, filepath.Join(a.BinDir, zeName))
	if a.LinuxOnly {
		environ = setEnv(environ, linuxOnlyKey, linuxOnlyValue)
	}
	if a.BuildCache != "" {
		environ = setEnv(environ, "GOCACHE", a.BuildCache)
	}
	if a.ModuleCache != "" {
		environ = setEnv(environ, "GOMODCACHE", a.ModuleCache)
	}

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
//
// A suite that answers 0 without printing a count executed no test, and this
// is where that becomes a failure rather than a phase the summary counts as
// passed.
func (a *allTestsRun) suite(suite vmSuite, environ []string) PhaseResult {
	name := suitePhaseName(suite.Name)
	if slices.Contains(a.Skip, suite.Name) {
		return PhaseResult{Name: name, Skipped: true, Reason: "ZE_QEMU_SKIP_SUITES"}
	}

	argv := a.suiteCommand(suite)
	a.note(banner(name))

	var tally suiteTally
	result := PhaseResult{
		Name:      name,
		Command:   argv,
		Namespace: suite.Namespace.String(),
		Counted:   true,
		Code:      a.child(argv, a.suiteEnvironment(suite, environ), &tally),
	}
	result.Tests = tally.Tests
	if tally.Seen {
		return result
	}

	result.Counted = false
	result.Reason = "the suite printed no `pass N/M` summary line, so it executed no test"
	if result.Code == 0 {
		result.Code = 1
	}
	return result
}

// suiteCommand is the command line one suite runs under.
//
// `timeout` runs the suite in its own process group. On expiry, it kills the
// whole group. Thus a stuck ze or plugin child cannot wedge the run.
func (a *allTestsRun) suiteCommand(suite vmSuite) []string {
	argv := make([]string, 0, len(suite.Args)+7)
	argv = append(argv, "timeout", killAfterFlag, killAfterSeconds, a.Timeout, filepath.Join(a.BinDir, zeTestName))
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

// unitPhase runs the complete Go package population without -race. The Alpine
// guest ships no C compiler, and the race detector needs CGO. Native host runs
// provide race coverage.
func (a *allTestsRun) unitPhase(environ []string) (PhaseResult, error) {
	tags, err := a.integrationTags()
	if err != nil {
		return PhaseResult{}, err
	}
	tags = strings.Replace(tags, " integration", "", 1)
	argv := []string{
		"env", "CGO_ENABLED=0", "go", "test",
		"-timeout", "20m",
		tagsFlag, tags,
		"./...",
	}
	a.note(banner(unitPhaseName))
	return PhaseResult{Name: unitPhaseName, Command: argv, Code: a.child(argv, environ, nil)}, nil
}

// installerPhase runs the installer initrd's own tests, which no other phase
// compiles.
//
// The initrd is built with ze_installer, a personality tag rather than a
// feature the manifest declares, so integrationTags never names it and
// unitPhase's `go test ./...` excludes every file guarded by it without saying
// so. Five test files sit behind that tag, the rescue console's fatal-branch
// policy among them. Off Linux a host can only type-check them
// (`le test-unit installer`), so this VM is where they run.
func (a *allTestsRun) installerPhase(environ []string) PhaseResult {
	argv := []string{
		"env", "CGO_ENABLED=0", "go", "test",
		"-count=1",
		"-timeout", "120s",
		tagsFlag, "ze_core ze_installer",
		"./internal/install/...",
	}
	a.note(banner(installerPhaseName))
	return PhaseResult{Name: installerPhaseName, Command: argv, Code: a.child(argv, environ, nil)}
}

// integrationPhase runs the linux-only, integration-tagged tests.
func (a *allTestsRun) integrationPhase(environ []string) (PhaseResult, error) {
	argv, err := a.integrationArgs()
	if err != nil {
		return PhaseResult{}, err
	}
	a.note(banner(integrationPhaseName))
	return PhaseResult{Name: integrationPhaseName, Command: argv, Code: a.child(argv, environ, nil)}, nil
}

// integrationArgs builds the integration command and refuses a package list
// naming a path that is not there.
//
// A path typo is a bug in THIS file, not a test failure. `go test` reports a
// missing directory as `FAIL <pkg> [setup failed]` among real results. Thus,
// ./cmd/ze/doctor remained broken here when the phase only looked red.
func (a *allTestsRun) integrationArgs() ([]string, error) {
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
			Join(missing, " ").Str(" -- this is a bug in internal/le/qemu, not a test failure").String())
	}

	packages := append([]string(nil), integrationPackages...)
	for _, optional := range optionalPackages {
		if a.hasPackage(optional) {
			packages = append(packages, optional)
		}
	}

	argv := make([]string, 0, len(packages)+9)
	argv = append(argv, "env", "CGO_ENABLED=0", "go", "test", tagsFlag, tags,
		"-count=1", "-timeout", "120s")
	return append(argv, packages...), nil
}

// hasPackage reports whether a package pattern names a directory of the
// workspace.
func (a *allTestsRun) hasPackage(pkg string) bool {
	dir := strings.TrimSuffix(pkg, "/...")
	info, err := os.Stat(filepath.Join(a.Workspace, dir))
	return err == nil && info.IsDir()
}

// integrationTags answers the build tags the integration pass compiles with.
//
// `-tags integration` ADDS the integration files to a package. It does not
// replace the package's ordinary unit tests, which also compile and run.
//
// Without ze_core and the feature set, feature-gated surfaces silently vanish.
// The set is derived from feature-gates.txt, so a missing manifest is an error,
// not a smaller tag set.
func (a *allTestsRun) integrationTags() (string, error) {
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

// plan is the population the run states before its first child.
//
// A person watching, and a reader of a log whose run was killed, can then tell
// which phases the run never reached. A process that loses its transport
// cannot report anything at all, so the population has to be on the terminal
// before the phase that can cut it.
func plan(phases []string) string {
	var tb textbuf.Buffer
	return tb.Str("planned phases (").Int(int64(len(phases))).Str("): ").Join(phases, ", ").String()
}
