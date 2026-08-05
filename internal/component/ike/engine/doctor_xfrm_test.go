package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	coreenv "github.com/ze-software/ze/internal/core/env"
)

// withXFRMProbe swaps the kernel XFRM probe for the duration of a test, so both
// answers are reachable on a host whose own kernel never changes.
func withXFRMProbe(t *testing.T, err error) {
	t.Helper()
	original := xfrmProbe
	xfrmProbe = func() error { return err }
	t.Cleanup(func() { xfrmProbe = original })
}

// VALIDATES: AC-9. An ipsec config on a host whose XFRM dataplane does not answer
// produces doctor-ipsec-xfrm-unavailable at warning severity, and the message
// carries the netlink failure.
// PREVENTS: the operator reading "the tunnel is up" from a daemon that cannot
// install a single SA. Every other IPsec surface reports engine belief, so a
// dataplane that answers nothing is invisible until traffic stops.
func TestXFRMUnavailableDiagnostic(t *testing.T) {
	withXFRMProbe(t, errors.New("operation not permitted"))

	diags := checkXFRMReachable(registry.DoctorCheckContext{Tree: ipsecTree("eth0")})
	if len(diags) != 1 {
		t.Fatalf("an unreachable XFRM dataplane produced %d diagnostics, want 1", len(diags))
	}
	if diags[0].Code != "doctor-ipsec-xfrm-unavailable" {
		t.Errorf("code is %q, want doctor-ipsec-xfrm-unavailable", diags[0].Code)
	}
	// Warning, not error: the same probe fails for a host that lacks CAP_NET_ADMIN,
	// where the kernel is fine and the privilege is not.
	if diags[0].Severity != "warning" {
		t.Errorf("severity is %q, want warning", diags[0].Severity)
	}
	// The netlink failure must reach the operator. Without it the message says the
	// dataplane is unavailable and nothing about which action fixes it.
	if !strings.Contains(diags[0].Message, "operation not permitted") {
		t.Errorf("the message does not name the netlink failure: %s", diags[0].Message)
	}
}

// VALIDATES: AC-9. The check is silent when the dataplane answers, and silent for
// every config that holds no vpn ipsec container.
// PREVENTS: a check that warns on every run, which trains an operator to ignore it.
func TestXFRMReachableSilentWhenNothingIsWrong(t *testing.T) {
	t.Run("the dataplane answers", func(t *testing.T) {
		withXFRMProbe(t, nil)
		if diags := checkXFRMReachable(registry.DoctorCheckContext{Tree: ipsecTree("eth0")}); len(diags) != 0 {
			t.Errorf("a reachable dataplane produced %d diagnostics: %+v", len(diags), diags)
		}
	})

	// The expectation comes from the CONFIG TREE. ze doctor runs offline in a
	// process where the engine never ran, so ActiveTable() is nil there and a
	// host with no IPsec configured must say nothing about XFRM.
	withXFRMProbe(t, errors.New("no such device"))
	for _, tc := range []struct {
		name string
		tree any
	}{
		{"no vpn section", config.NewTree()},
		{"nil tree", (*config.Tree)(nil)},
		{"a tree of the wrong type", "not a tree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if diags := checkXFRMReachable(registry.DoctorCheckContext{Tree: tc.tree}); len(diags) != 0 {
				t.Errorf("a config without ipsec produced %d diagnostics: %+v", len(diags), diags)
			}
		})
	}
}

// VALIDATES: AC-9. The check is declared on the ike plugin registration, so
// ze doctor runs it and ze explain resolves its code.
// PREVENTS: the check existing as dead code, which is what an unregistered
// readiness check is (ai/rules/completion.md).
func TestXFRMReachableDoctorCheckRegistered(t *testing.T) {
	for _, check := range registry.PluginDoctorChecks() {
		if check.PluginName != "ike" || check.Name != "ipsec-xfrm" {
			continue
		}
		if check.Check == nil {
			t.Fatal("ike ipsec-xfrm doctor check has a nil Check function")
		}
		found := false
		for _, code := range check.Codes {
			if code == "doctor-ipsec-xfrm-unavailable" {
				found = true
			}
		}
		if !found {
			t.Errorf("declared codes %v do not include doctor-ipsec-xfrm-unavailable", check.Codes)
		}
		return
	}
	t.Fatal("ike plugin declares no ipsec-xfrm doctor check")
}

// VALIDATES: the ze.test.ike.xfrm-fail override drives the probe to failure, which
// is what test/ui/doctor-ipsec-xfrm.ci needs to reach the diagnostic through the
// user entry point on a host whose kernel is healthy.
// PREVENTS: a functional test that passes only where XFRM happens to be absent.
func TestXFRMProbeHonorsTestOverride(t *testing.T) {
	original := coreenv.Get(envKeyIKEXFRMFail)
	t.Cleanup(func() { _ = coreenv.Set(envKeyIKEXFRMFail, original) })
	if err := coreenv.Set(envKeyIKEXFRMFail, "true"); err != nil {
		t.Fatalf("set %s: %v", envKeyIKEXFRMFail, err)
	}
	// errors.Is, not err != nil. A host without CAP_NET_ADMIN fails the real probe
	// with EPERM, so a nil check would pass with the override deleted.
	if err := probeXFRMDataplane(); !errors.Is(err, errXFRMForced) {
		t.Fatalf("the override did not force a probe failure: %v", err)
	}
}
