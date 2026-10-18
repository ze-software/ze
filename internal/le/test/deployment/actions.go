// Design: docs/architecture/core-design.md -- the deployment area, as one command
// Detail: l2tp.go -- the L2TP peer proof this table reaches
// Detail: vppiface.go -- the VPP interface proof this table reaches
// Detail: vppevidence.go -- the eight real VPP deployment scenarios
// Detail: l2tpppp.go -- the on-host L2TP PPP proof this table reaches
// Detail: l2tpdiag.go -- the two gateless native L2TP diagnostics
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/le/action, which every ported area shares.
//
// The gate names are the family: ze-deployment-l2tp-test, -vpp-test,
// -vpp-iface-test and the rest all begin ze-deployment-, so each verb is that
// name with the area's prefix removed and every new proof is one row here.

package testdeployment

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	interopl2tp "github.com/ze-software/ze/internal/le/interoplab/l2tp"
	interoppppoe "github.com/ze-software/ze/internal/le/interoplab/pppoe"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "test deployment"

var actions = leaction.New(area,
	leaction.Action{Verb: "l2tp-test", Why: "the L2TP control session against an external peer." +
		" Needs Docker and the host's /dev/ppp",
		Answer: signalRooted(runL2TPAt)},
	leaction.Action{Verb: "l2tp-ppp-test", Why: "the full L2TP PPP/NCP path on the host: LCP, IPCP, a kernel PPP" +
		" interface at each end, a dataplane ping and a clean teardown." +
		" Needs Linux root, xl2tpd, pppd, ping and PPPoL2TP kernel support",
		Answer: runL2TPPPPHere},
	leaction.Action{Verb: L2TPScaleAction, Why: "the native loopback L2TP scale registry: 2,000 sessions," +
		" clean teardown, pool exhaustion, and delayed RADIUS." +
		" L2TP_SCALE_SCENARIO=<name> runs one scenario",
		Answer: runL2TPScaleHere},
	leaction.Action{Verb: "gokrazy-l2tp-ppp-test", Why: "the same L2TP PPP/NCP path proven on the gokrazy appliance image" +
		" rather than on a dev host. Needs Linux root, QEMU, dnsmasq," +
		" xl2tpd, pppd and PPPoL2TP kernel support",
		Answer: runGokrazyL2TPHere},
	leaction.Action{Verb: "vpp-iface-test", Why: "the VPP interface features against a real daemon: tunnels, mirror," +
		" wireguard, LCP. Needs Docker and the host's /dev/net/tun and /dev/vhost-net",
		Answer: signalRooted(runVPPIfaceAt)},
	leaction.Action{Verb: "vpp-test", Why: "ze driving a real VPP daemon, not a fake channel. Needs Docker and the" +
		" host's /dev/net/tun and /dev/vhost-net",
		Answer: signalRooted(runVPPAt)},
	leaction.Action{
		Verb:       l2tpPPPoXDiagnosticName,
		Why:        "the native Linux netlink, PPPoX and PPP ioctl path. Leaves created L2TP objects for inspection",
		Parameters: pppoxDiagnosticParameters(),
		AnswerArgs: runL2TPPPoXDiagnosticHere,
	},
	leaction.Action{
		Verb:       l2tpTunnelDiagnosticName,
		Why:        "the native Linux protocol-v3 L2TP tunnel create and kernel dump. Leaves the tunnel for inspection",
		Parameters: tunnelDiagnosticParameters(),
		AnswerArgs: runL2TPTunnelDiagnosticHere,
	},
	leaction.Action{Verb: "docker-l2tp-ppp-test", Why: "the same L2TP PPP/NCP path in a peer-isolated Docker lab. Needs" +
		" PPPoL2TP support in the Docker host kernel",
		Answer: signalRooted(runDockerL2TPAt)},
	leaction.Action{Verb: "docker-pppoe-accel-test", Why: "Ze's PPPoE client against a real accel-ppp access concentrator in a" +
		" Docker lab. Needs PPPoE support in the Docker host kernel",
		Answer: signalRooted(runDockerPPPoEAt)},
	leaction.Action{
		Verb: "ipsec-mobike-test",
		Why: "prove both MOBIKE initiator and responder address movement against packaged strongSwan" +
			" in private Linux namespaces, retaining IKE/Child identities and encrypted traffic",
		Parameters: []leaction.Parameter{
			{Keyword: "daemon", Value: "prebuilt-ze-path", Requirement: leaction.Required},
			{Keyword: "charon", Value: "path", Requirement: leaction.Optional},
			{Keyword: "swanctl", Value: "path", Requirement: leaction.Optional},
		},
		AnswerArgs: runIPsecMOBIKEHere,
	},
)

// signalRooted is the entry root of every verb that starts a container: the
// checkout this command was run in, and a context SIGINT or SIGTERM cancels,
// so the verb's waits end and its deferred removals run before it exits.
func signalRooted(run func(context.Context, string) (any, int)) func() (any, int) {
	return func() (any, int) {
		root, err := lepath.Root()
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return run(ctx, root)
	}
}

func runDockerL2TPAt(ctx context.Context, root string) (any, int) {
	report := interopl2tp.RunAt(ctx, root, "")
	return report, report.Code
}

func runDockerPPPoEAt(ctx context.Context, root string) (any, int) {
	report := interoppppoe.RunAt(ctx, root, interoppppoe.Options{})
	return report, report.Code
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le test deployment` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runL2TPAt proves L2TP over the checkout at root.
func runL2TPAt(ctx context.Context, root string) (any, int) {
	return runL2TP(ctx, NewL2TP(root))
}

// runL2TP answers the proof over one run. A step that could not be performed is
// an error and answers 1 with no verdict; a session that did not establish is
// the verdict, and it answers 1 with the daemon's last lines behind it. A
// container the run could not remove answers 1 whatever the verdict.
func runL2TP(ctx context.Context, run *L2TP) (L2TPReport, int) {
	report, err := run.Run(ctx)
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	return report, l2tpExitCode(report)
}

// l2tpExitCode is the verdict of a run that reached one.
func l2tpExitCode(report L2TPReport) int {
	if !report.Established {
		return 1
	}
	if report.ContainerRemoval != "" {
		return 1
	}
	return 0
}

func runL2TPScaleHere() (any, int) {
	report, code := runL2TPScale(context.Background(), l2tpScaleOptions())
	return report, code
}

// runL2TPPPPHere proves the whole L2TP PPP path over the checkout this command
// was run in.
func runL2TPPPPHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runL2TPPPP(NewL2TPPPP(root))
}

// runL2TPPPP answers the proof over one run. It uses the same terms as the
// container L2TP proof. A step that cannot be performed is an error. A step in
// the path that did not happen is the verdict.
func runL2TPPPP(run *L2TPPPP) (L2TPPPPReport, int) {
	report, err := run.Run()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if !report.Proven {
		return report, 1
	}
	return report, 0
}

// runGokrazyL2TPHere proves the L2TP PPP path against the appliance image built
// from the checkout this command was run in.
func runGokrazyL2TPHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runGokrazyL2TP(newGokrazyL2TP(root))
}

// runGokrazyL2TP answers the proof over one run. It uses the same terms as its
// on-host sibling. A step that cannot be performed is an error. A step in the
// path that did not happen is the verdict.
func runGokrazyL2TP(run *gokrazyL2TP) (GokrazyL2TPReport, int) {
	report, err := run.Run()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if !report.Proven {
		return report, 1
	}
	return report, 0
}

// runVPPIfaceAt proves the VPP interface features over the checkout at root.
func runVPPIfaceAt(ctx context.Context, root string) (any, int) {
	return runVPPIface(ctx, newVPPIface(root))
}

// runVPPIface answers the proof over one run. It uses the same terms as the L2TP
// proof. A step that cannot be performed is an error. A feature that VPP never
// showed is the verdict.
func runVPPIface(ctx context.Context, run *vppIface) (VPPIfaceReport, int) {
	report, err := run.Run(ctx)
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if !report.Passed {
		return report, 1
	}
	if report.ContainerRemoval != "" {
		return report, 1
	}
	return report, 0
}

// runVPPAt proves all eight VPP deployment scenarios over the checkout at root.
func runVPPAt(ctx context.Context, root string) (any, int) {
	return runVPP(ctx, newVPP(root))
}

// runVPP maps the report and operating error onto the action exit code.
func runVPP(ctx context.Context, run *VPP) (VPPReport, int) {
	report, err := run.Run(ctx)
	if err != nil {
		leaction.ReportError(err)
	}
	return report, vppExitCode(report.Passed && report.ContainerRemoval == "", err)
}

func vppExitCode(passed bool, err error) int {
	if err != nil {
		return 1
	}
	if !passed {
		return 1
	}
	return 0
}

// runL2TPPPoXDiagnosticHere validates every selector before the Linux path can
// create a socket, load a module, or send a netlink request.
func runL2TPPPoXDiagnosticHere(args leaction.Arguments) (any, int) {
	options, err := parsePPPoXDiagnosticArguments(args)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runL2TPPPoXDiagnostic(options)
}

func runL2TPPPoXDiagnostic(options l2tpDiagnosticOptions) (l2tpDiagnosticReport, int) {
	report, err := executeL2TPPPoXDiagnostic(options)
	reportL2TPDiagnosticError(err)
	return report, diagnosticExitCode(report.Verdict, err)
}

// runL2TPTunnelDiagnosticHere validates the six protocol-v3 selectors before
// the Linux path resolves the generic netlink family.
func runL2TPTunnelDiagnosticHere(args leaction.Arguments) (any, int) {
	options, err := parseTunnelDiagnosticArguments(args)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runL2TPTunnelDiagnostic(options)
}

func runL2TPTunnelDiagnostic(options l2tpDiagnosticOptions) (l2tpDiagnosticReport, int) {
	report, err := executeL2TPTunnelDiagnostic(options)
	reportL2TPDiagnosticError(err)
	return report, diagnosticExitCode(report.Verdict, err)
}
