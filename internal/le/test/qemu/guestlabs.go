// Design: docs/architecture/testing/qemu-integration.md -- guest-side evidence ports
// Replaces the former VRRP, PPPoE, and netns Python guest drivers.
package testqemu

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// The four guest-side labs. A lab name is the report's Lab field, the key its
// Text renderer switches on, and the word the operator sees, so the three
// cannot drift apart.
const (
	labVRRPKeepalived = "vrrp-keepalived"
	labPPPoEAccel     = "pppoe-accel"
	labNetns          = "netns"
	labPPPoENetns     = "pppoe-netns"
)

// enumeration: exempt (these are the network-namespace names the QEMU labs create, not plugin names)
const (
	vrrpQS1 = "QS-1"
	vrrpQS2 = "QS-2"
	vrrpQS3 = "QS-3"
	// vrrpTrackedUplink is NAMED rather than numbered
	// (ai/rules/interop-and-goal-validation.md): a number goes stale in two ways
	// a name cannot, because a deleted scenario leaves a hole no reader can tell
	// from a reservation. The three above predate that rule and renaming them is
	// not this scenario's work.
	vrrpTrackedUplink = "tracked-uplink-hands-the-vip-to-keepalived"
	// vrrpOwnerKeepsTheAddress is the QEMU path of the Docker interop
	// scenario vrrp-v2-owner-keepalived: ze, the VRRPv2 address owner,
	// discards keepalived's conflicting priority-255 advertisements.
	vrrpOwnerKeepsTheAddress = "v2-owner-keeps-the-address"

	netnsFirewall = "firewall"
	netnsPolicy   = "policy"
	netnsOSPF     = "ospf"
	netnsOSPFv3   = "ospfv3"
	netnsPPPoE    = "pppoe"
	netnsPlugin   = "plugin"
)

var (
	vrrpScenarioNames  = []string{vrrpQS1, vrrpQS2, vrrpQS3, vrrpTrackedUplink, vrrpOwnerKeepsTheAddress}
	defaultNetnsSuites = []string{netnsFirewall, netnsPolicy, netnsOSPF, netnsOSPFv3}
	netnsSuiteNames    = []string{netnsFirewall, netnsPolicy, netnsOSPF, netnsOSPFv3, netnsPPPoE, netnsPlugin}
	// vrrpScenarioHelp says what each of vrrpScenarioNames proves. The report
	// header and PASS line print it, and so do the scenarios keyword's help and
	// completion, so le refuses to start while a scenario has no sentence here
	// (leroot.requireEveryItemHasText).
	vrrpScenarioHelp = map[string]string{
		vrrpQS1:                  "v3 IPv4 election: ze prio 200 vs keepalived 100 (AC-1)",
		vrrpQS2:                  "node-death failover and ze preempt return (AC-2)",
		vrrpQS3:                  "graceful stop: Priority-0 skew path (AC-3)",
		vrrpTrackedUplink:        "tracked uplink down: keepalived takes the VIP (AC-6, AC-7)",
		vrrpOwnerKeepsTheAddress: "v2 owner keeps the address over keepalived prio 255",
	}
	// netnsSuiteHelp says what each subset of netnsSuiteNames runs, for the
	// suites keyword's help and completion. The tests of each subset are
	// netnsSelections (netns_linux.go).
	netnsSuiteHelp = map[string]string{
		netnsFirewall: "nftables firewall, NAT, CoPP and DDoS withdraw",
		netnsPolicy:   "policy routing: tables, TCP flags and MSS, next hop",
		netnsOSPF:     "OSPFv2: demux, multi-AF, NBMA, point-to-multipoint",
		netnsOSPFv3:   "OSPFv3: virtual link, NBMA, point-to-multipoint",
		netnsPPPoE:    "PPPoE: sessions, L2TP, service name, VLAN, floods",
		netnsPlugin:   "RPKI ASPA validation and Adj-RIB-In replay",
	}
)

// GuestScenario is one scenario or suite attempted by a guest-side proof.
type GuestScenario struct {
	Name      string   `json:"name"`
	Verdict   Verdict  `json:"verdict"`
	Details   []string `json:"details,omitempty"`
	Failure   string   `json:"failure,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
}

// guestLabReport is the structured answer shared by the four guest-side actions.
// A zero Verdict is invalid and always maps to a failing command.
type guestLabReport struct {
	Lab       string          `json:"lab"`
	Verdict   Verdict         `json:"verdict"`
	Selected  []string        `json:"selected,omitempty"`
	Scenarios []GuestScenario `json:"scenarios,omitempty"`
	HostSafe  *bool           `json:"host-safe,omitempty"`
	Failure   string          `json:"failure,omitempty"`
	Artifacts []string        `json:"artifacts,omitempty"`
	Cleanup   []string        `json:"cleanup,omitempty"`
}

// Text preserves each producer's terminal summary while the pipe operators use
// the fields above. Progress and peer output stream to stderr during the run.
func (r guestLabReport) Text() string {
	var out textbuf.Buffer
	switch r.Lab {
	case labVRRPKeepalived:
		for _, scenario := range r.Scenarios {
			out.Str("\n=== ").Str(scenario.Name).Str(": ").Str(vrrpScenarioHelp[scenario.Name]).Str(" ===\n")
			for _, detail := range scenario.Details {
				out.Str(detail).Byte('\n')
			}
			if scenario.Verdict == VerdictPass {
				out.Str("PASS: ").Str(scenario.Name).Byte(' ').Str(vrrpScenarioHelp[scenario.Name]).Byte('\n')
			}
		}
		if r.Verdict == VerdictPass {
			out.Str("\nOK: ze VRRP interoperates with keepalived across ").Int(int64(len(r.Selected))).
				Str(" scenario(s): ").Join(r.Selected, ", ").Byte('\n')
		}
	case labPPPoEAccel:
		if r.Verdict == VerdictPass && len(r.Scenarios) == 1 {
			for _, detail := range r.Scenarios[0].Details {
				out.Str(detail).Byte('\n')
			}
		}
	case labNetns, labPPPoENetns:
		for _, scenario := range r.Scenarios {
			if scenario.Verdict == VerdictFail {
				out.Str("FAIL: ").Str(scenario.Name).Str(" netns subset returned ").Str(scenario.Failure).Byte('\n')
			}
		}
		if r.HostSafe != nil && *r.HostSafe {
			out.Str("HOST-SAFE: host nft tables unchanged\n")
		}
		out.Str("netns-qemu: ")
		if r.Verdict == VerdictPass {
			out.Str("PASS\n")
		} else {
			out.Str("FAIL\n")
		}
	}
	return out.String()
}

func parseClosedCSV(raw, label string, allowed, defaults []string) ([]string, error) {
	if raw == "" {
		return append([]string(nil), defaults...), nil
	}
	parts := strings.Split(raw, ",")
	selected := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	allowedSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = true
	}
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, fmt.Errorf("%s list contains an empty name", label)
		}
		if !allowedSet[name] {
			known := append([]string(nil), allowed...)
			slices.Sort(known)
			return nil, fmt.Errorf("unknown %s %q; known %ss: %s", label, name, label, strings.Join(known, ", "))
		}
		if seen[name] {
			return nil, fmt.Errorf("%s %q was selected more than once", label, name)
		}
		seen[name] = true
		selected = append(selected, name)
	}
	return selected, nil
}

func guestContext() (context.Context, context.CancelFunc) {
	// SIGHUP too: a closed terminal must reach the guest command's teardown
	// and stop the VM, rather than kill le and leave QEMU running.
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
}

func runVRRPHere(args leaction.Arguments) (any, int) {
	selected, err := parseClosedCSV(args.One("scenarios"), "scenario", vrrpScenarioNames, vrrpScenarioNames)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	ctx, cancel := guestContext()
	defer cancel()
	report, err := runVRRPGuest(ctx, root, selected)
	return finishGuestLab(report, err)
}

func runPPPoEAccelHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	ctx, cancel := guestContext()
	defer cancel()
	report, err := runPPPoEAccelGuest(ctx, root)
	return finishGuestLab(report, err)
}

func runNetnsHere(args leaction.Arguments) (any, int) {
	raw := args.One("suites")
	if raw == "" {
		raw = strings.Join(strings.Fields(os.Getenv("ZE_NETNS_QEMU_SUITES")), ",")
	}
	selected, err := parseClosedCSV(raw, "suite", netnsSuiteNames, defaultNetnsSuites)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	ctx, cancel := guestContext()
	defer cancel()
	report, err := runNetnsGuest(ctx, selected)
	return finishGuestLab(report, err)
}

func runPPPoENetnsHere() (any, int) {
	ctx, cancel := guestContext()
	defer cancel()
	report, err := runNetnsGuest(ctx, []string{netnsPPPoE})
	return finishGuestLab(report, err)
}

func finishGuestLab(report guestLabReport, err error) (any, int) {
	if err != nil {
		leaction.ReportError(err)
		if report.Lab == "" {
			return nil, 1
		}
		return report, 1
	}
	if report.Verdict != VerdictPass {
		return report, 1
	}
	return report, 0
}
