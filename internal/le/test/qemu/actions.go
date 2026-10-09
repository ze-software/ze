// Design: docs/architecture/core-design.md -- the qemu area, as one command
// Detail: hugepages.go -- the proof this table reaches
// Detail: mplsboot.go -- the appliance MPLS boot proof
// Detail: crashcapture.go -- the two kernel crash-capture proofs
// Detail: run.go -- the host harness this table reaches
// Detail: install.go -- the four import-linked installer proofs
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/le/action, which every ported area shares.
//
// THE AREA IS THE GATE-NAME FAMILY, not its former script directory.
// `ze-qemu-vpp-hugepages-test` begins ze-qemu-. leaction removes `ze-<area>-`
// and nothing else to derive each verb. This gate therefore cannot be a row of
// internal/le/test/integration unless it is typed as its own whole name. The qemu
// and deployment families remain separate because they own separate evidence
// concerns.
//
// Host-side and guest-side evidence actions share this native area and its
// explicit verbs.

package testqemu

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// The run and install actions share these keywords, and the code that reads an
// argument names the same constant the action declares.
const (
	keywordCommand  = "command"
	keywordKernel   = "kernel"
	keywordPackages = "packages"
	keywordTimeout  = "timeout"
)

// valueDuration is the value placeholder every timeout keyword shows in help.
const valueDuration = "duration"

// zeMainPackage is the package path of the ze binary each build here compiles.
const zeMainPackage = "./cmd/ze"

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "test qemu"

// onlyKeyword types the population an in-VM run covers, so the name of that
// population never sits in an untyped positional slot (ai/rules/cli.md).
const onlyKeyword = "only"

// testKeyword names the one .ci an all-tests run narrows to.
const testKeyword = "test"

var actions = leaction.New(area,
	leaction.Action{Verb: "vpp-hugepages-test", Why: "boot-time hugepage reservation, end to end: build an appliance carrying" +
		" image.hugepages, boot it, then assert `show host kernel` and `show host" +
		" memory` over the Ze CLI. Self-skips when qemu, sshpass, e2fsprogs or go" +
		" are absent; on Linux it needs membership of the kvm group" +
		" (`./le setup check` reports it as kvm-access)",
		Answer: runHugepagesHere},
	leaction.Action{Verb: "mpls-boot-test", Why: "the appliance starts with MPLS in use, end to end: build an appliance whose" +
		" seed adds `set fib kernel` and `set ldp`, boot it, then assert `show ldp neighbor`" +
		" answers over the Ze CLI, which a daemon the kernel capability gate refused cannot." +
		" Self-skips like vpp-hugepages-test",
		Answer: runMPLSBootHere},
	leaction.Action{Verb: "crash-capture-panic-harvest", Why: "kernel crash capture survives a panic, end to end:" +
		" build an appliance that reserves a crash region, boot it, inject an NMI the seed turns into a" +
		" kernel panic, and assert the next boot harvested a kernel artifact carrying the panic into" +
		" /perm/ze/crash. amd64 only; self-skips like vpp-hugepages-test",
		Answer: runCrashPanicHarvestHere},
	leaction.Action{Verb: "crash-capture-ota-unaffected", Why: "an OTA reboot still works with crash capture active:" +
		" build an appliance that reserves a crash region, boot it, ask gokrazy's update server for the" +
		" kexec reboot `gok update` ends with, and assert the next boot is armed again with no kernel" +
		" artifact. amd64 only; self-skips like vpp-hugepages-test",
		Answer: runCrashOTAUnaffectedHere},
	leaction.Action{
		Verb: "run",
		Why: "boot an Alpine Linux guest, share this checkout, install the requested" +
			" packages, and run one command over SSH. The host process owns the ISO" +
			" cache, QEMU lifecycle, bounded waits, and cleanup",
		Parameters: []leaction.Parameter{
			// A run needs command or keep-alive, and parseRunArguments is
			// what refuses one carrying neither. The keyword is optional
			// because the switch beside it can answer for it.
			{Keyword: keywordCommand, Value: "command", Requirement: leaction.Optional},
			{Keyword: keywordPackages, Value: "space-separated-packages", Requirement: leaction.Optional},
			{Keyword: keywordTimeout, Value: valueDuration, Requirement: leaction.Optional},
			{Keyword: keywordKernel, Value: "path", Requirement: leaction.Optional},
			{Keyword: "keep-alive"},
		},
		AnswerArgs: runQEMUHere,
	},
	leaction.Action{
		Verb:   "install-test",
		Why:    "build the installer initrd and appliance image, install over HTTP, then prove the installed ZeFS power user over SSH",
		Answer: runInstallHTTPHere,
	},
	leaction.Action{
		Verb:   "install-iso-test",
		Why:    "build and boot the appliance installer ISO, prove its embedded image bytes, target, GPT layout, safe poweroff, and SSH login",
		Answer: runInstallISOHere,
	},
	leaction.Action{
		Verb:   "install-scenarios-test",
		Why:    "prove installer panic recovery, boot-NIC pin and fallback, the three rescue-console policy branches, and the refusal to choose between two fixed disks",
		Answer: runInstallScenariosHere,
	},
	leaction.Action{
		Verb:   "install-ventoy-test",
		Why:    "put the appliance ISO on a whole-disk FAT volume and prove the installer finds it through the Ventoy scan",
		Answer: runInstallVentoyHere,
	},
	leaction.Action{
		Verb: "vrrp-keepalived-test",
		Why: "run ze VRRP against a real keepalived peer inside the guest, across every" +
			" scenario the lab declares",
		Parameters: []leaction.Parameter{
			// The placeholder is derived from vrrpScenarioNames, the same list
			// the run itself selects from. Spelled out here it went stale the
			// day a fourth scenario was added, and the new one was then
			// undiscoverable from `./le test qemu`.
			{Keyword: "scenarios", Value: strings.Join(vrrpScenarioNames, ","), Requirement: leaction.Optional},
		},
		AnswerArgs: runVRRPHere,
	},
	leaction.Action{
		Verb: "ipsec-mobike-test",
		Why: "target-build Ze and the native runner, boot the supplied runtime kernel," +
			" and prove both MOBIKE movement scenarios against Alpine strongSwan without Docker",
		Parameters: []leaction.Parameter{
			{Keyword: keywordKernel, Value: "vmlinuz-path", Requirement: leaction.Required},
			{Keyword: keywordTimeout, Value: valueDuration, Requirement: leaction.Optional},
		},
		AnswerArgs: runIPsecMOBIKEHere,
	},
	leaction.Action{
		Verb:   "pppoe-accel-test",
		Why:    "run ze's PPPoE client against accel-ppp inside paired network namespaces",
		Answer: runPPPoEAccelHere,
	},
	leaction.Action{
		Verb: "netns-test",
		Why: "run the selected functional subsets through the credential-dropped network" +
			" namespace launcher and prove the guest root nftables state is unchanged",
		Parameters: []leaction.Parameter{
			{Keyword: "suites", Value: "firewall,policy,ospf,ospfv3,pppoe,plugin", Requirement: leaction.Optional},
		},
		AnswerArgs: runNetnsHere,
	},
	leaction.Action{
		Verb: "pppoe-test",
		Why: "run the PPPoE functional subset through the same guest network-namespace" +
			" engine, on ze's runtime kernel",
		Answer: runPPPoENetnsHere,
	},
	leaction.Action{
		// This action runs inside the VM. Its caller is the command value passed
		// to the host qemu run action.
		Verb: "all-tests",
		Why: "the whole ze test suite, inside the QEMU Linux VM: every functional suite at a" +
			" VM-appropriate concurrency, the unit pass, and the integration-tagged tests." +
			" Refuses to start outside the guest, or with a suite list that has a hole in it." +
			" `only needs-linux` narrows the functional suites to the .ci tests marked" +
			" option=needs-linux, which is the tight loop for a change to a Linux-only path." +
			" `test <path>` runs that one .ci through the suite that walks its directory, and nothing else",
		Parameters: []leaction.Parameter{
			{Keyword: onlyKeyword, Value: linuxOnlySelection, Requirement: leaction.Optional},
			{Keyword: testKeyword, Value: "test/<dir>/<name>.ci", Requirement: leaction.Optional},
		},
		AnswerArgs: runAllTestsHere,
	},
	stressAction(),
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le test qemu` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runHugepagesHere proves the reservation over the checkout this command was
// run in.
func runHugepagesHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runHugepages(newHugepages(root))
}

// runHugepages answers the proof over one run.
//
// A step that the run cannot perform is an error. It answers 1 with no verdict.
// A SKIP answers 0. This is the self-skip contract that the functional suite and
// CI both rely on. A machine without QEMU has not disproved anything.
func runHugepages(run *Hugepages) (HugepagesReport, int) {
	report, err := run.Run()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if report.Verdict == VerdictFail || report.Verdict == VerdictUnspecified {
		return report, 1
	}
	return report, 0
}

// runMPLSBootHere proves the appliance starts with MPLS in use, over the
// checkout this command was run in.
func runMPLSBootHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := newMPLSBoot(root).Run()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if report.Verdict == VerdictFail || report.Verdict == VerdictUnspecified {
		return report, 1
	}
	return report, 0
}

// runCrashPanicHarvestHere proves a kernel panic is harvested on the next
// boot, over the checkout this command was run in.
func runCrashPanicHarvestHere() (any, int) { return runCrashCaptureHere(CrashLabPanicHarvest) }

// runCrashOTAUnaffectedHere proves an OTA reboot still works with crash capture
// active, over the checkout this command was run in.
func runCrashOTAUnaffectedHere() (any, int) { return runCrashCaptureHere(CrashLabOTAUnaffected) }

// runCrashCaptureHere runs one crash-capture proof over the checkout this
// command was run in.
func runCrashCaptureHere(lab CrashLab) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := newCrashCapture(root, lab).Run()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if report.Verdict == VerdictFail || report.Verdict == VerdictUnspecified {
		return report, 1
	}
	return report, 0
}

// runQEMUHere runs the host harness over this checkout.
func runQEMUHere(args leaction.Arguments) (any, int) {
	options, err := parseRunArguments(args)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	// The guest runs every harness command as `le test <name>` from this
	// build, so the harness it runs is the harness of this checkout.
	if err := buildGuestLe(root, GuestArch()); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 1)
	received := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case caught := <-signals:
			received <- caught
			cancel()
		case <-ctx.Done():
		}
	}()

	report, runErr := NewRun(root, options).Execute(ctx)
	select {
	case caught := <-received:
		return nil, signalExitCode(caught)
	default:
	}
	if runErr != nil {
		leaction.ReportError(runErr)
		return nil, 1
	}
	return &report, runExitCode(&report)
}

func runInstallHTTPHere() (any, int)      { return runInstallerHere(InstallKindHTTP) }
func runInstallISOHere() (any, int)       { return runInstallerHere(InstallKindISO) }
func runInstallScenariosHere() (any, int) { return runInstallerHere(InstallKindScenarios) }
func runInstallVentoyHere() (any, int)    { return runInstallerHere(InstallKindVentoy) }

func runInstallerHere(kind InstallKind) (any, int) {
	options, err := DefaultInstallOptions(kind)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report, err := NewInstaller(root, options).Execute(ctx)
	if err != nil {
		leaction.ReportError(err)
		return &report, 1
	}
	return &report, installExitCode(&report)
}

func installExitCode(report *InstallReport) int {
	if report.Verdict == InstallVerdictPass || report.Verdict == InstallVerdictSkip {
		return 0
	}
	return 1
}

func runExitCode(report *RunReport) int {
	if report.Verdict == RunVerdictPass {
		return 0
	}
	if report.ProofFailure != "" {
		return 1
	}
	if report.GuestExitCode == 0 {
		return 1
	}
	return report.GuestExitCode
}

func signalExitCode(caught os.Signal) int {
	signalNumber, ok := caught.(syscall.Signal)
	if !ok {
		return 1
	}
	return 128 + int(signalNumber)
}

// runAllTestsHere runs every phase inside the guest this command was started in.
//
// It takes no checkout argument, and it does not consult lepath.Root(). The
// guest mounts the repository at one known path. If a run used a checkout from
// another location, it would use the host's tree with the guest's assumptions.
// The mount is the precondition, and the refusal names it.
//
// `only` selects the population the functional suites run over. The one
// population a caller can name is needs-linux, and any other word is refused:
// a value this action cannot honor would otherwise run the whole suite while
// the caller believed it had narrowed it.
func runAllTestsHere(args leaction.Arguments) (any, int) {
	run := newAllTests()
	if args.Has(onlyKeyword) {
		selection := args.One(onlyKeyword)
		if selection != linuxOnlySelection {
			var tb textbuf.Buffer
			leaction.ReportError(errors.New(tb.Str("qemu all-tests only takes ").
				Str(linuxOnlySelection).Str(", got ").Quoted(selection).
				Str(" -- needs-linux runs the .ci tests marked option=needs-linux and no others").String()))
			return nil, 2
		}
		run.LinuxOnly = true
	}

	run.Test = args.One(testKeyword)

	report, code := run.Execute()
	if len(report.Phases) == 0 {
		// The run never started. Its refusal is already on stderr. A report with
		// no phases does not answer "what did the VM prove". `| json` would put
		// an empty document beside a non-zero code.
		return nil, code
	}
	return report, code
}
