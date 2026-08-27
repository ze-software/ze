// Design: docs/architecture/core-design.md -- the deployment area, as one command
// Detail: l2tp.go -- the L2TP peer proof this table reaches
// Detail: vppiface.go -- the VPP interface proof this table reaches
// Detail: vppevidence.go -- the eight real VPP deployment scenarios
// Detail: l2tpppp.go -- the on-host L2TP PPP proof this table reaches
// Detail: l2tpdiag.go -- the two gateless native L2TP diagnostics
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/leaction, which every ported area shares.
//
// The gate names are the family: ze-deployment-l2tp-test, -vpp-test,
// -vpp-iface-test and the rest all begin ze-deployment-, so each verb is that
// name with the area's prefix removed and every new proof is one row here.

package deployment

import (
	"context"

	interopl2tp "github.com/ze-software/ze/internal/le/interoplab/l2tp"
	interoppppoe "github.com/ze-software/ze/internal/le/interoplab/pppoe"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "deployment"

var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-deployment-l2tp-test",
		Why: "the L2TP control session against an external peer." +
			" Needs Docker and a privileged container",
		Answer: runL2TPHere,
	},
	leaction.Action{
		Gate: "ze-deployment-l2tp-ppp-test",
		Why: "the full L2TP PPP/NCP path on the host: LCP, IPCP, a kernel PPP" +
			" interface at each end, a dataplane ping and a clean teardown." +
			" Needs Linux root, xl2tpd, pppd, ping and PPPoL2TP kernel support",
		Answer: runL2TPPPPHere,
	},
	leaction.Action{
		Gate: "ze-deployment-gokrazy-l2tp-ppp-test",
		Why: "the same L2TP PPP/NCP path proven on the gokrazy appliance image" +
			" rather than on a dev host. Needs Linux root, QEMU, dnsmasq," +
			" xl2tpd, pppd and PPPoL2TP kernel support",
		Answer: runGokrazyL2TPHere,
	},
	leaction.Action{
		Gate: "ze-deployment-vpp-iface-test",
		Why: "the VPP interface features against a real daemon: tunnels, mirror," +
			" wireguard, LCP. Needs Docker and a privileged container",
		Answer: runVPPIfaceHere,
	},
	leaction.Action{
		Gate: "ze-deployment-vpp-test",
		Why: "ze driving a real VPP daemon, not a fake channel. Needs Docker and a" +
			" privileged container",
		Answer: runVPPHere,
	},
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
	leaction.Action{
		Gate: "ze-deployment-docker-l2tp-ppp-test",
		Why: "the same L2TP PPP/NCP path in a peer-isolated Docker lab. Needs" +
			" PPPoL2TP support in the Docker host kernel",
		Answer: runDockerL2TPHere,
	},
	leaction.Action{
		Gate: "ze-deployment-docker-pppoe-accel-test",
		Why: "Ze's PPPoE client against a real accel-ppp access concentrator in a" +
			" Docker lab. Needs PPPoE support in the Docker host kernel",
		Answer: runDockerPPPoEHere,
	},
)

func runDockerL2TPHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report := interopl2tp.RunAt(context.Background(), root, "")
	return report, report.Code
}

func runDockerPPPoEHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report := interoppppoe.RunAt(context.Background(), root, interoppppoe.Options{})
	return report, report.Code
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le deployment` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runL2TPHere proves L2TP over the checkout this command was run in.
func runL2TPHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runL2TP(NewL2TP(root))
}

// runL2TP answers the proof over one run. A step that could not be performed is
// an error and answers 1 with no verdict; a session that did not establish is
// the verdict, and it answers 1 with the daemon's last lines behind it.
func runL2TP(run *L2TP) (L2TPReport, int) {
	report, err := run.Run()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if !report.Established {
		return report, 1
	}
	return report, 0
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
	return runGokrazyL2TP(NewGokrazyL2TP(root))
}

// runGokrazyL2TP answers the proof over one run. It uses the same terms as its
// on-host sibling. A step that cannot be performed is an error. A step in the
// path that did not happen is the verdict.
func runGokrazyL2TP(run *GokrazyL2TP) (GokrazyL2TPReport, int) {
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

// runVPPIfaceHere proves the VPP interface features over the checkout this
// command was run in.
func runVPPIfaceHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runVPPIface(NewVPPIface(root))
}

// runVPPIface answers the proof over one run. It uses the same terms as the L2TP
// proof. A step that cannot be performed is an error. A feature that VPP never
// showed is the verdict.
func runVPPIface(run *VPPIface) (VPPIfaceReport, int) {
	report, err := run.Run()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if !report.Passed {
		return report, 1
	}
	return report, 0
}

// runVPPHere proves all eight VPP deployment scenarios over this checkout.
func runVPPHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runVPP(NewVPP(root))
}

// runVPP maps the report and operating error onto the action exit code.
func runVPP(run *VPP) (VPPReport, int) {
	report, err := run.Run()
	if err != nil {
		leaction.ReportError(err)
	}
	return report, vppExitCode(report.Passed, err)
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

func runL2TPPPoXDiagnostic(options l2tpDiagnosticOptions) (L2TPDiagnosticReport, int) {
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

func runL2TPTunnelDiagnostic(options l2tpDiagnosticOptions) (L2TPDiagnosticReport, int) {
	report, err := executeL2TPTunnelDiagnostic(options)
	reportL2TPDiagnosticError(err)
	return report, diagnosticExitCode(report.Verdict, err)
}
