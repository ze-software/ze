package doctor

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
)

// TestDaemonCapabilitiesCheck drives the daemon-capabilities check through the
// inventories privilege.CheckPrivileges gives, with the probe and the service
// membership stubbed.
//
// VALIDATES: a daemon lacking a capability of the set is one informational row
// under doctor-daemon-capability-missing that names the capability and what it
// serves and says each feature that needs one reports its own impact; its fix is
// privilege.CapabilityRemedy for the capabilities named, which is the unit
// rewrite only under ze.service and never a systemd command for any other
// process; an unreadable set is an informational row saying Ze cannot determine
// the capabilities, with the read error; a daemon with nothing to report answers
// no row; the check runs in the runtime phase only, so it measures the daemon
// and never the operator's ze doctor process.
// PREVENTS: a generic inventory that makes a box read not ready or warns about a
// capability no configured feature uses, and an operator who did not start ze
// from ze.service told to reinstall the systemd unit.
func TestDaemonCapabilitiesCheck(t *testing.T) {
	if daemonCapabilitiesCheck.Phase != diagnostic.DoctorPhaseRuntime {
		t.Fatalf("phase = %q, want %q", daemonCapabilitiesCheck.Phase, diagnostic.DoctorPhaseRuntime)
	}
	bpf := privilege.Capability{Name: "CAP_BPF", Bit: 39, Use: "eBPF: IPsec ESP form fast path, IPsec override guard, traffic-usage"}
	for _, tc := range []struct {
		name       string
		membership privilege.ServiceMembership
		missing    []privilege.Capability
		err        error
		contains   []string
		fix        string
		noSystemd  bool
	}{
		{name: "nothing to report", membership: privilege.MembershipZeService},
		{name: "a capability missing under ze.service", membership: privilege.MembershipZeService, missing: []privilege.Capability{bpf}, contains: []string{
			"runs without CAP_BPF (eBPF: IPsec ESP form fast path", "not a problem by itself", "reports its own impact",
		}, fix: privilege.DaemonCapabilitiesFix},
		{name: "a capability missing outside ze.service", membership: privilege.MembershipOther, missing: []privilege.Capability{bpf}, contains: []string{
			"runs without CAP_BPF", "reports its own impact",
		}, noSystemd: true},
		{name: "a capability missing, membership unknown", membership: privilege.MembershipUnknown, missing: []privilege.Capability{bpf}, contains: []string{
			"runs without CAP_BPF",
		}, noSystemd: true},
		{name: "the set cannot be read", membership: privilege.MembershipOther, err: errors.New("permission denied"), contains: []string{
			"Ze cannot determine the capabilities", "permission denied", "reports its own impact",
		}, noSystemd: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := daemonCapabilityInventory
			daemonCapabilityInventory = func() ([]privilege.Capability, error) { return tc.missing, tc.err }
			t.Cleanup(func() { daemonCapabilityInventory = saved })
			t.Cleanup(privilege.SetServiceMembershipForTest(tc.membership, nil))

			diags := checkDaemonCapabilities(diagnostic.DoctorCheckContext{})
			if tc.missing == nil && tc.err == nil {
				if len(diags) != 0 {
					t.Fatalf("diagnostics = %v, want none", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("diagnostics = %v, want one", diags)
			}
			d := diags[0]
			if d.Code != codeDoctorDaemonCapabilityMissing || d.Severity != diagnostic.SeverityInfo {
				t.Errorf("code %q severity %q, want %q info", d.Code, d.Severity, codeDoctorDaemonCapabilityMissing)
			}
			if !readyOf(diags) {
				t.Errorf("the inventory row made the box not ready")
			}
			for _, text := range tc.contains {
				if !strings.Contains(d.Message, text) {
					t.Errorf("message does not carry %q: %q", text, d.Message)
				}
			}
			if strings.Contains(d.Message, "systemctl") || strings.Contains(d.Message, "ze install systemd") {
				t.Errorf("the message carries a systemd command, which belongs to the fix alone: %q", d.Message)
			}
			if d.Fix == "" {
				t.Errorf("the row names no fix")
			}
			if tc.fix != "" && d.Fix != tc.fix {
				t.Errorf("fix = %q, want %q", d.Fix, tc.fix)
			}
			if tc.noSystemd && (strings.Contains(d.Fix, "systemctl") || strings.Contains(d.Fix, "ze install systemd")) {
				t.Errorf("membership %s is offered a systemd command: %q", tc.membership, d.Fix)
			}
		})
	}
}
