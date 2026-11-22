//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// GOAL: every dataplane site that reports a missing capability, and every remedy that
// runs through the ze unit, offers the systemd command only when ze runs under
// ze.service, names the capabilities the operation needs, and otherwise tells the
// process how to get them without the unit; a membership Ze cannot read is said so.
// METHOD: fix the membership through privilege.SetServiceMembershipForTest, build each
// refusal, and read its fix.
//
// MUTATION: in espFormSocketRefusal, ask CapabilityRemedy for CAP_NET_ADMIN: the raw
// socket's remedy no longer names CAP_NET_RAW.
// MUTATION: in espFormFileLimitFix, answer privilege.DaemonCapabilitiesFix
// unconditionally: the other-process remedy carries systemctl.
// MUTATION: in privilege.ServiceRemedy, drop the unknown sentence: the unknown remedy does not
// say Ze could not tell.
func TestESPFormRemediesFollowServiceMembership(t *testing.T) {
	sites := []struct {
		name  string
		build func() error
		caps  []string
	}{
		{"raw socket EPERM", func() error { return espFormSocketRefusal(unix.EPERM) }, []string{"CAP_NET_RAW"}},
		{"policy exemption EPERM", func() error { return espFormBypassRefusal(unix.EPERM) }, []string{"CAP_NET_ADMIN"}},
		{"TUNSETIFF EPERM", func() error { return espFormTUNSetIffRefusal(unix.EPERM) }, []string{"CAP_NET_ADMIN"}},
		{"device step EPERM", func() error { return espFormTUNStepRefusal("LinkSetUp", "bring it up", unix.EPERM) }, []string{"CAP_NET_ADMIN"}},
		{"forward default EPERM", func() error { return xfrmForwardDefaultRefusal(unix.EPERM, "Ze cannot set the forward default") }, []string{"CAP_NET_ADMIN"}},
	}
	fixOf := func(t *testing.T, err error) string {
		t.Helper()
		oe, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Fatalf("the refusal %v is not an operator error", err)
		}
		return oe.Fix()
	}
	for _, site := range sites {
		t.Run(site.name+" under ze.service", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
			if got := fixOf(t, site.build()); got != privilege.DaemonCapabilitiesFix {
				t.Errorf("fix = %q, want the unit repair %q", got, privilege.DaemonCapabilitiesFix)
			}
		})
		t.Run(site.name+" outside ze.service", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
			got := fixOf(t, site.build())
			if strings.Contains(got, "systemctl") || strings.Contains(got, "install systemd") {
				t.Errorf("fix = %q offers the systemd repair to a ze the unit did not start", got)
			}
			for _, name := range site.caps {
				if !strings.Contains(got, name) {
					t.Errorf("fix = %q does not name %s, which the operation needs", got, name)
				}
			}
		})
	}

	t.Run("file limit under ze.service", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
		if got := fixOf(t, espFormSocketRefusal(unix.EMFILE)); got != privilege.DaemonCapabilitiesFix {
			t.Errorf("fix = %q, want the unit rewrite %q", got, privilege.DaemonCapabilitiesFix)
		}
	})
	t.Run("file limit outside ze.service", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
		if got := fixOf(t, espFormSocketRefusal(unix.EMFILE)); got != espFormFixFileLimitOther {
			t.Errorf("fix = %q, want %q", got, espFormFixFileLimitOther)
		}
	})
	t.Run("file limit with unknown membership", func(t *testing.T) {
		cause := errors.New("no cgroup v2 line")
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipUnknown, cause))
		got := fixOf(t, espFormSocketRefusal(unix.EMFILE))
		want := "Ze could not tell whether it runs under " + privilege.ServiceUnit + " (no cgroup v2 line); " + espFormFixFileLimitOther
		if got != want {
			t.Errorf("fix = %q, want %q", got, want)
		}
	})
}

// GOAL: the fast path's fix follows the fault the kernel's answer was classified as, and
// the capability fault's remedy follows the membership.
// METHOD: classify wrapped EPERM, ebpf.ErrNotSupported and EINVAL, and read
// ESPFormFastPathFix for each under both memberships.
//
// MUTATION: in espFormFastPathFaultOf, drop the EPERM test: EPERM classifies Unknown.
// MUTATION: in ESPFormFastPathFix, answer RuntimeKernelBuildFix for a capability: the
// capability fix no longer names CAP_BPF.
func TestESPFormFastPathFaultDecidesTheFix(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want ESPFormFastPathFault
	}{
		{"EPERM", fmt.Errorf("load the rewrite program: %w", unix.EPERM), ESPFormFastPathFaultCapability},
		{"no TCX", fmt.Errorf("this kernel does not attach TCX programs: %w", ebpf.ErrNotSupported), ESPFormFastPathFaultKernel},
		{"verifier EINVAL", fmt.Errorf("load the rewrite program: %w", unix.EINVAL), ESPFormFastPathFaultUnknown},
		{"verifier EACCES", fmt.Errorf("load the rewrite program: %w", unix.EACCES), ESPFormFastPathFaultUnknown},
	} {
		if got := espFormFastPathFaultOf(tc.err); got != tc.want {
			t.Errorf("%s: fault = %d, want %d", tc.name, got, tc.want)
		}
	}

	restore := privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil)
	if got := ESPFormFastPathFix(ESPFormFastPathFaultCapability); got != privilege.DaemonCapabilitiesFix {
		t.Errorf("capability fix under ze.service = %q, want %q", got, privilege.DaemonCapabilitiesFix)
	}
	restore()
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
	got := ESPFormFastPathFix(ESPFormFastPathFaultCapability)
	if strings.Contains(got, "systemctl") || !strings.Contains(got, "CAP_BPF") || !strings.Contains(got, "CAP_NET_ADMIN") {
		t.Errorf("capability fix outside ze.service = %q, want CAP_BPF and CAP_NET_ADMIN with no systemd command", got)
	}
	if got := ESPFormFastPathFix(ESPFormFastPathFaultKernel); got != RuntimeKernelBuildFix() {
		t.Errorf("kernel fix = %q, want %q", got, RuntimeKernelBuildFix())
	}
	if got := ESPFormFastPathFix(ESPFormFastPathFaultUnknown); got != ESPFormFixLog {
		t.Errorf("unknown fix = %q, want %q", got, ESPFormFixLog)
	}
}
