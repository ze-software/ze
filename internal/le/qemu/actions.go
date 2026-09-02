// Design: docs/architecture/core-design.md -- the qemu area, as one command
// Detail: hugepages.go -- the proof this table reaches
// Detail: run.go -- the host harness this table reaches
// Detail: install.go -- the four import-linked installer proofs
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/leaction, which every ported area shares.
//
// THE AREA IS THE GATE-NAME FAMILY, not its former script directory.
// `ze-qemu-vpp-hugepages-test` begins ze-qemu-. leaction removes `ze-<area>-`
// and nothing else to derive each verb. This gate therefore cannot be a row of
// internal/le/integration unless it is typed as its own whole name. The qemu
// and deployment families remain separate because they own separate evidence
// concerns.
//
// Host-side and guest-side evidence actions share this native area and its
// explicit verbs.

package qemu

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "qemu"

// onlyKeyword types the population an in-VM run covers, so the name of that
// population never sits in an untyped positional slot (ai/rules/cli.md).
const onlyKeyword = "only"

var actions = leaction.New(area,
	leaction.Action{Verb: "vpp-hugepages-test", Why: "boot-time hugepage reservation, end to end: build an appliance carrying" +
		" image.hugepages, boot it, then assert `show host kernel` and `show host" +
		" memory` over the Ze CLI. Self-skips when qemu, sshpass, e2fsprogs or go" +
		" are absent; on Linux it needs membership of the kvm group" +
		" (`./le setup check` reports it as kvm-access)",
		Answer: runHugepagesHere},
	leaction.Action{
		Verb: "run",
		Why: "boot an Alpine Linux guest, share this checkout, install the requested" +
			" packages, and run one command over SSH. The host process owns the ISO" +
			" cache, QEMU lifecycle, bounded waits, and cleanup",
		Parameters: []leaction.Parameter{
			{Keyword: "command", Value: "command"},
			{Keyword: "packages", Value: "space-separated-packages"},
			{Keyword: "timeout", Value: "duration"},
			{Keyword: "kernel", Value: "path"},
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
		Why: "run ze VRRP against a real keepalived peer inside the guest across the" +
			" QS-1 election, QS-2 failover/preemption, and QS-3 Priority-0 paths",
		Parameters: []leaction.Parameter{
			{Keyword: "scenarios", Value: "QS-1,QS-2,QS-3"},
		},
		AnswerArgs: runVRRPHere,
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
			{Keyword: "suites", Value: "firewall,policy,ospf,ospfv3,pppoe"},
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
			" option=needs-linux, which is the tight loop for a change to a Linux-only path",
		Parameters: []leaction.Parameter{
			{Keyword: onlyKeyword, Value: linuxOnlySelection},
		},
		AnswerArgs: runAllTestsHere,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le qemu` command.
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
	if selection, named := args[onlyKeyword]; named {
		if selection != linuxOnlySelection {
			var tb textbuf.Buffer
			leaction.ReportError(errors.New(tb.Str("qemu all-tests only takes ").
				Str(linuxOnlySelection).Str(", got ").Quoted(selection).
				Str(" -- needs-linux runs the .ci tests marked option=needs-linux and no others").String()))
			return nil, 2
		}
		run.LinuxOnly = true
	}

	report, code := run.Execute()
	if len(report.Phases) == 0 {
		// The run never started. Its refusal is already on stderr. A report with
		// no phases does not answer "what did the VM prove". `| json` would put
		// an empty document beside a non-zero code.
		return nil, code
	}
	return report, code
}
