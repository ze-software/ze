// Design: docs/guide/ipsec.md -- application override control
// Related: application_override.go -- applyApplicationOverride, the step these tests drive
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.1-8

package engine

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/privilege"
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
// attaches the guard once, after the node catch-all is installed, and the apply
// writes no IKE bypass into the SPD: Ze's own IKE is exempt on its sockets
// (dataplane.BypassIKESocket), which open after the apply, so the guard is in force first.
//
// VALIDATES: the order in applyConfig (catch-all, then guard), and that no node-wide
// IKE bypass entry is installed.
// PREVENTS: an SPD entry in force before the guard, and a node-wide UDP 500/4500
// bypass that would exempt a protected host's transit datagram (RFC 4301 Section 5.2).
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
	if !slices.ContainsFunc(before, func(p dataplane.SPParams) bool { return p.Role == dataplane.PolicyRoleCatchAll }) {
		t.Fatalf("guard attached before the catch-all: installed before it %v", before)
	}
	if slices.ContainsFunc(dp.installed, func(p dataplane.SPParams) bool { return p.Role == dataplane.PolicyRoleIKEBypass }) {
		t.Fatalf("the apply installed an IKE bypass SPD entry; Ze's IKE is exempt on its sockets only: %v", dp.installed)
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
// backend refuses fails the apply, and the error names the kernel's answer, the cause
// Ze reads from it, that cause's fix and no other cause's, and the allow statement as
// the way to accept the risk instead; a reload that is refused leaves the running
// catch-all as it was.
//
// VALIDATES: attachOverrideGuard's refusal, one row per overrideGuardFault class
// (EPERM, dataplane.ErrNoCgroup2, an answer Ze does not classify), and its place
// before every reload change.
// PREVENTS: a deny that is reported applied while no guard holds, and a refusal that
// lists every cause whatever the kernel answered or names the cgroup2 mount twice.
func TestApplyFailsWhenOverrideGuardCannotAttach(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	state := applyTestState(t)
	if err := state.applyConfig(overrideConfig(ipsec.ApplicationOverrideAllow), applyReload); err != nil {
		t.Fatalf("allow apply: %v", err)
	}
	installed := len(dp.installed)

	cases := []struct {
		name     string
		guardErr error
		want     []string
		refuse   []string
		// mounts is how many times the refusal names the cgroup2 mount command: once
		// as the fix where it is the fix, and never stated twice.
		mounts int
	}{
		{
			name:     "capability",
			guardErr: fmt.Errorf("load the setsockopt guard: %w", syscall.EPERM),
			want: []string{
				"operation not permitted", "lacks CAP_BPF or CAP_NET_ADMIN",
				"To fix it: " + privilege.CapabilityRemedy("CAP_BPF", "CAP_NET_ADMIN") +
					", then commit again, or set `vpn ipsec application-override allow` to accept that risk",
			},
			refuse: []string{"Linux 6.10", "cgroup2 none"},
		},
		{
			name:     "no cgroup2",
			guardErr: dataplane.ErrNoCgroup2,
			want: []string{
				"no cgroup2 file system is mounted",
				"To fix it: " + overrideGuardFixMount +
					", then commit again, or set `vpn ipsec application-override allow` to accept that risk",
			},
			refuse: []string{"CAP_BPF", "Linux 6.10"},
			mounts: 1,
		},
		{
			name:     "unknown",
			guardErr: errors.New("operation not permitted"),
			want: []string{
				"operation not permitted", "Ze cannot tell the cause", "Linux 6.10", "security module",
				"To fix it: set `vpn ipsec application-override allow` to accept that risk, or keep deny on Linux 6.10 or later",
			},
			refuse: []string{"CAP_BPF", "cgroup2 none"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dp.guardErr = tc.guardErr
			err := state.applyConfig(overrideConfig(ipsec.ApplicationOverrideDeny), applyReload)
			if err == nil {
				t.Fatal("deny applied with a guard the backend refused")
			}
			if !errors.Is(err, tc.guardErr) {
				t.Errorf("refusal %q does not wrap the backend's answer", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
			for _, other := range tc.refuse {
				if strings.Contains(err.Error(), other) {
					t.Errorf("refusal %q names %q, the fix of another cause", err, other)
				}
			}
			if got := strings.Count(err.Error(), "mount -t cgroup2 none /sys/fs/cgroup"); got != tc.mounts {
				t.Errorf("refusal %q names the cgroup2 mount %d times, want %d", err, got, tc.mounts)
			}
			if len(dp.installed) != installed {
				t.Fatalf("the refused reload installed %d SPD entries", len(dp.installed)-installed)
			}
		})
	}
	dp.guardErr = nil

	// A backend with no guard at all is refused under deny too.
	if err := applyApplicationOverride(&mockDP{}, ipsec.ApplicationOverrideDeny); err == nil {
		t.Fatal("deny applied on a backend that cannot refuse per-socket policies")
	}
	if err := applyApplicationOverride(dp, ipsec.ApplicationOverrideUnspecified); !errors.Is(err, errOverrideUnspecified) {
		t.Fatalf("unspecified override: err = %v, want errOverrideUnspecified", err)
	}
}
