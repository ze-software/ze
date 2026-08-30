// Design: plan/spec-le-is-a-ze-binary.md -- step 10 guest-side evidence ports
// Replaces the former VRRP, PPPoE, and netns Python guest drivers.
package qemu

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
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

const (
	vrrpQS1 = "QS-1"
	vrrpQS2 = "QS-2"
	vrrpQS3 = "QS-3"

	netnsFirewall = "firewall"
	netnsPolicy   = "policy"
	netnsOSPF     = "ospf"
	netnsOSPFv3   = "ospfv3"
	netnsPPPoE    = "pppoe"
)

var (
	vrrpScenarioNames  = []string{vrrpQS1, vrrpQS2, vrrpQS3}
	defaultNetnsSuites = []string{netnsFirewall, netnsPolicy, netnsOSPF, netnsOSPFv3}
	netnsSuiteNames    = []string{netnsFirewall, netnsPolicy, netnsOSPF, netnsOSPFv3, netnsPPPoE}
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
			out.Str("\n=== ").Str(scenario.Name).Str(": ").Str(vrrpDescription(scenario.Name)).Str(" ===\n")
			for _, detail := range scenario.Details {
				out.Str(detail).Byte('\n')
			}
			if scenario.Verdict == VerdictPass {
				out.Str("PASS: ").Str(scenario.Name).Byte(' ').Str(vrrpDescription(scenario.Name)).Byte('\n')
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

func vrrpDescription(name string) string {
	switch name {
	case vrrpQS1:
		return "v3 IPv4 election: ze prio 200 vs keepalived prio 100 (AC-1)"
	case vrrpQS2:
		return "node-death failover and ze preempt return (AC-2)"
	case vrrpQS3:
		return "graceful stop: Priority-0 skew path (AC-3)"
	default:
		return ""
	}
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
			sort.Strings(known)
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
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func runVRRPHere(args leaction.Arguments) (any, int) {
	selected, err := parseClosedCSV(args["scenarios"], "scenario", vrrpScenarioNames, vrrpScenarioNames)
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
	raw := args["suites"]
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
