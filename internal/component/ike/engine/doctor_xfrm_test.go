//go:build linux

package engine

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	coreenv "github.com/ze-software/ze/internal/core/env"
)

const envKeyXFRMState = "ze.test.kernelcap.xfrm"

// withXFRMState forces the shared kernel XFRM probe's answer for the duration of
// a test, so every verdict is reachable on a host whose own kernel never changes.
func withXFRMState(t *testing.T, state string) {
	t.Helper()
	original := coreenv.Get(envKeyXFRMState)
	t.Cleanup(func() { _ = coreenv.Set(envKeyXFRMState, original) })
	if err := coreenv.Set(envKeyXFRMState, state); err != nil {
		t.Fatalf("set %s: %v", envKeyXFRMState, err)
	}
}

// ipsecPeerTree builds a config that installs one site-to-site Child SA, which
// is what makes IPsec in use. An empty vpn ipsec block is not (AC-11).
func ipsecPeerTree() *config.Tree {
	peer := config.NewTree()
	peer.Set("remote-address", "203.0.113.7")
	peers := config.NewTree()
	peers.AddListEntry("peer", "branch", peer)
	ipsecRoot := config.NewTree()
	ipsecRoot.SetContainer("site-to-site", peers)
	vpnRoot := config.NewTree()
	vpnRoot.SetContainer("ipsec", ipsecRoot)
	root := config.NewTree()
	root.SetContainer("vpn", vpnRoot)
	return root
}

// ipsecCapabilityCheck returns the doctor check the ike enrolment registered.
func ipsecCapabilityCheck(t *testing.T) diagnostic.DoctorCheck {
	t.Helper()
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name == "kernel-capability-ipsec" {
			return check
		}
	}
	t.Fatal("the ike engine registered no kernel-capability-ipsec doctor check")
	return diagnostic.DoctorCheck{}
}

// VALIDATES: AC-2 and AC-12. An IPsec config on a host whose kernel holds no XFRM
// dataplane produces doctor-ipsec-xfrm-unavailable at ERROR severity, and the
// message names the subsystem, the kernel feature and the config that asked.
// PREVENTS: the operator reading "the tunnel is up" from a daemon that cannot
// install a single SA. Every other IPsec surface reports engine belief, so a
// dataplane that answers nothing is invisible until traffic stops.
func TestXFRMAbsentIsAStartupError(t *testing.T) {
	withXFRMState(t, "absent")

	check := ipsecCapabilityCheck(t)
	diags := check.Check(diagnostic.DoctorCheckContext{Tree: ipsecPeerTree()})
	if len(diags) != 1 {
		t.Fatalf("an absent XFRM dataplane produced %d diagnostics, want 1", len(diags))
	}
	if diags[0].Code != diagnosticIPsecXFRMUnavailable {
		t.Errorf("code is %q, want %q", diags[0].Code, diagnosticIPsecXFRMUnavailable)
	}
	// Error, not warning: this severity is what refuses the start, and the probe
	// used here reports absence only for the errno that means the kernel carries
	// no XFRM. A denied probe is a separate verdict with its own code.
	if diags[0].Severity != diagnostic.SeverityError {
		t.Errorf("severity is %q, want error", diags[0].Severity)
	}
	for _, want := range []string{"ipsec", "CONFIG_XFRM_USER", "vpn ipsec"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("the message does not name %q: %s", want, diags[0].Message)
		}
	}
}

// VALIDATES: AC-6. A probe that cannot answer warns and does not refuse.
// PREVENTS: a working deployment turned into a dead one by an unreadable probe
// (R-1). An unprivileged reader must never be told the kernel lacks XFRM.
func TestXFRMUnknownWarnsRatherThanRefusing(t *testing.T) {
	withXFRMState(t, "unknown")

	check := ipsecCapabilityCheck(t)
	diags := check.Check(diagnostic.DoctorCheckContext{Tree: ipsecPeerTree()})
	if len(diags) != 1 {
		t.Fatalf("an undetermined XFRM dataplane produced %d diagnostics, want 1", len(diags))
	}
	if diags[0].Code != diagnosticIPsecXFRMUnknown {
		t.Errorf("code is %q, want %q", diags[0].Code, diagnosticIPsecXFRMUnknown)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("severity is %q, want warning", diags[0].Severity)
	}
	if err := kernelcap.Refuse(ipsecPeerTree()); err != nil {
		t.Errorf("cannot-determine refused a start: %v", err)
	}
}

// VALIDATES: AC-1, AC-5 and AC-11. The check is silent when the dataplane answers,
// silent for a config that holds no vpn ipsec container, and silent for an empty
// vpn ipsec block that installs no Security Association.
// PREVENTS: a check that fires on every run, which trains an operator to ignore
// it, and a refusal for a configuration that would have carried no packet.
func TestXFRMCapabilitySilentWhenNothingIsWrong(t *testing.T) {
	check := ipsecCapabilityCheck(t)

	t.Run("the dataplane answers", func(t *testing.T) {
		withXFRMState(t, "present")
		if diags := check.Check(diagnostic.DoctorCheckContext{Tree: ipsecPeerTree()}); len(diags) != 0 {
			t.Errorf("a present dataplane produced %d diagnostics: %+v", len(diags), diags)
		}
	})

	withXFRMState(t, "absent")
	for _, tc := range []struct {
		name string
		tree any
	}{
		{"no vpn section", config.NewTree()},
		{"an empty vpn ipsec block", ipsecTree("eth0")},
		{"nil tree", (*config.Tree)(nil)},
		{"a tree of the wrong type", "not a tree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if diags := check.Check(diagnostic.DoctorCheckContext{Tree: tc.tree}); len(diags) != 0 {
				t.Errorf("a config that installs no SA produced %d diagnostics: %+v", len(diags), diags)
			}
		})
	}
}

// VALIDATES: the enrolment is declared by the ike engine, so ze doctor runs it,
// the startup gate reads it and ze explain resolves both its codes.
// PREVENTS: the capability existing as dead code, which is what an unregistered
// readiness check is (ai/rules/completion.md).
func TestIPsecCapabilityEnrolled(t *testing.T) {
	enrolled := false
	for _, subsystem := range kernelcap.Enrolled() {
		if subsystem == "ipsec" {
			enrolled = true
		}
	}
	if !enrolled {
		t.Fatalf("ipsec is not enrolled; enrolled subsystems are %v", kernelcap.Enrolled())
	}

	check := ipsecCapabilityCheck(t)
	if check.Check == nil {
		t.Fatal("kernel-capability-ipsec has a nil Check function")
	}
	wanted := map[string]bool{diagnosticIPsecXFRMUnavailable: false, diagnosticIPsecXFRMUnknown: false}
	for _, code := range check.Codes {
		if _, ok := wanted[code]; ok {
			wanted[code] = true
		}
	}
	for code, found := range wanted {
		if !found {
			t.Errorf("declared codes %v do not include %s", check.Codes, code)
		}
	}
}
