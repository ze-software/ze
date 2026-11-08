// Design: docs/guide/ipsec.md -- application override control
// Related: application_override.go -- applyApplicationOverride, the step these tests drive
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.1-8

package engine

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// orderedGuardDP is catchAllDP that also notes how many SPD entries were installed
// when each attach ran, so the guard's place against the catch-all is observable.
type orderedGuardDP struct {
	catchAllDP
	attachedAfter []int
}

func (d *orderedGuardDP) AttachOverrideGuard() error {
	if err := d.catchAllDP.AttachOverrideGuard(); err != nil {
		return err
	}
	d.attachedAfter = append(d.attachedAfter, len(d.installed))
	return nil
}

func overrideConfig(override ipsec.ApplicationOverride) *ipsec.IPsecConfig {
	cfg := testIPsecConfig()
	cfg.Unmatched = dataplane.SPActionDiscard
	cfg.ApplicationOverride = override
	return cfg
}

// TestRFC4301OverrideLeafSelectsTheGuard proves the administrator's control reaches
// the dataplane: deny attaches the guard, allow attaches none.
//
// RFC requirement: RFC4301-4.4.1-8 positive -- the configured `application-override` value decides, through applyConfig, whether the setsockopt guard is attached: deny attaches it and allow detaches it.
//
// RFC 4301 Section 4.4.1: "However, the system administrator MUST be able to specify
// whether or not a user or application can override (default) system policies."
//
// VALIDATES: applyConfig calls applyApplicationOverride with cfg.ApplicationOverride.
// PREVENTS: an apply that ignores the leaf, attaching always or never.
func TestRFC4301OverrideLeafSelectsTheGuard(t *testing.T) {
	for _, tc := range []struct {
		override ipsec.ApplicationOverride
		want     []string
	}{
		{ipsec.ApplicationOverrideDeny, []string{"attach"}},
		{ipsec.ApplicationOverrideAllow, []string{"detach"}},
	} {
		t.Run(tc.override.String(), func(t *testing.T) {
			dp := &catchAllDP{}
			loadFailClosedBackend(t, dp)
			state := applyTestState(t)
			if err := state.applyConfig(overrideConfig(tc.override), applyReload); err != nil {
				t.Fatalf("apply: %v", err)
			}
			if !slices.Equal(dp.guardOps, tc.want) {
				t.Fatalf("guard calls = %v, want %v", dp.guardOps, tc.want)
			}
		})
	}
}

// TestApplyAttachesOverrideGuardUnderDeny is the wiring row: a first apply under deny
// attaches the guard after the node catch-all is installed and before the IKE bypass.
//
// VALIDATES: the order in applyConfig (catch-all, guard, IKE bypass).
// PREVENTS: an IKE socket or an SPD entry in force before the guard.
func TestApplyAttachesOverrideGuardUnderDeny(t *testing.T) {
	dp := &orderedGuardDP{}
	loadFailClosedBackend(t, dp)
	state := applyTestState(t)
	if err := state.applyConfig(overrideConfig(ipsec.ApplicationOverrideDeny), applyStartup); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(dp.attachedAfter) != 1 {
		t.Fatalf("attached %d times, want once", len(dp.attachedAfter))
	}
	before := dp.installed[:dp.attachedAfter[0]]
	after := dp.installed[dp.attachedAfter[0]:]
	if !slices.ContainsFunc(before, func(p dataplane.SPParams) bool { return p.Role == dataplane.PolicyRoleCatchAll }) {
		t.Fatalf("guard attached before the catch-all: installed before it %v", before)
	}
	if slices.ContainsFunc(before, func(p dataplane.SPParams) bool { return p.Role == dataplane.PolicyRoleIKEBypass }) {
		t.Fatal("the IKE bypass went in before the guard")
	}
	if !slices.ContainsFunc(after, func(p dataplane.SPParams) bool { return p.Role == dataplane.PolicyRoleIKEBypass }) {
		t.Fatal("no IKE bypass followed the guard")
	}
}

// TestReloadMovesOverrideGuard proves AC-19: deny to allow detaches, allow to deny
// attaches, and leaving IPsec detaches, with no restart.
//
// VALIDATES: the reload arm of applyConfig and releaseSPD.
// PREVENTS: a guard that outlives the configuration that asked for it, or one a
// reload to deny never attaches.
func TestReloadMovesOverrideGuard(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	state := applyTestState(t)
	for _, override := range []ipsec.ApplicationOverride{
		ipsec.ApplicationOverrideDeny,
		ipsec.ApplicationOverrideAllow,
		ipsec.ApplicationOverrideDeny,
	} {
		if err := state.applyConfig(overrideConfig(override), applyReload); err != nil {
			t.Fatalf("apply %v: %v", override, err)
		}
	}
	state.releaseSPD(dataplane.Get(), state.log)
	want := []string{"attach", "detach", "attach", "detach"}
	if !slices.Equal(dp.guardOps, want) {
		t.Fatalf("guard calls = %v, want %v", dp.guardOps, want)
	}
	if appliedOverride.Load() != nil {
		t.Fatal("the applied override outlived releaseSPD")
	}
}

// TestApplyFailsWhenOverrideGuardCannotAttach proves AC-18: under deny a guard the
// backend refuses fails the apply, and the error names the kernel's reason and the
// three fixes; a reload that is refused leaves the running catch-all as it was.
//
// VALIDATES: attachOverrideGuard's refusal and its place before every reload change.
// PREVENTS: a deny that is reported applied while no guard holds.
func TestApplyFailsWhenOverrideGuardCannotAttach(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	state := applyTestState(t)
	if err := state.applyConfig(overrideConfig(ipsec.ApplicationOverrideAllow), applyReload); err != nil {
		t.Fatalf("allow apply: %v", err)
	}
	installed := len(dp.installed)

	dp.guardErr = errors.New("operation not permitted")
	err := state.applyConfig(overrideConfig(ipsec.ApplicationOverrideDeny), applyReload)
	if err == nil {
		t.Fatal("deny applied with a guard the backend refused")
	}
	for _, want := range []string{
		"operation not permitted",
		"CAP_BPF",
		"mount cgroup2 at /sys/fs/cgroup",
		"set `vpn ipsec application-override allow`",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}
	if len(dp.installed) != installed {
		t.Fatalf("the refused reload installed %d SPD entries", len(dp.installed)-installed)
	}

	// A backend with no guard at all is refused under deny too.
	if err := applyApplicationOverride(&mockDP{}, ipsec.ApplicationOverrideDeny); err == nil {
		t.Fatal("deny applied on a backend that cannot refuse per-socket policies")
	}
	if err := applyApplicationOverride(dp, ipsec.ApplicationOverrideUnspecified); !errors.Is(err, errOverrideUnspecified) {
		t.Fatalf("unspecified override: err = %v, want errOverrideUnspecified", err)
	}
}
