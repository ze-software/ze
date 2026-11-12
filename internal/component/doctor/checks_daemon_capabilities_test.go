package doctor

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
)

// TestDaemonCapabilitiesCheck drives the daemon-capabilities check through the answers
// privilege.CheckPrivileges gives, with the probe stubbed.
//
// VALIDATES: a daemon lacking a capability of the set is one warning under
// doctor-daemon-capability-missing that names the capability and what it serves, says
// how to grant it under systemd and without, and carries the unit rewrite as its fix;
// an unreadable set is the same warning carrying the read error; a daemon with nothing
// to report answers no row; the check runs in the runtime phase only, so it measures
// the daemon and never the operator's ze doctor process.
// PREVENTS: a daemon whose eBPF features never load while doctor reports nothing about
// the capability that would load them.
func TestDaemonCapabilitiesCheck(t *testing.T) {
	if daemonCapabilitiesCheck.Phase != diagnostic.DoctorPhaseRuntime {
		t.Fatalf("phase = %q, want %q", daemonCapabilitiesCheck.Phase, diagnostic.DoctorPhaseRuntime)
	}
	missing := []string{"running without root; missing capabilities:", "  CAP_BPF (eBPF: IPsec ESP form fast path, IPsec override guard, traffic-usage)"}
	for _, tc := range []struct {
		name     string
		warnings []string
		contains []string
	}{
		{name: "nothing to report"},
		{name: "a capability missing", warnings: missing, contains: []string{
			"missing capabilities: CAP_BPF (eBPF: IPsec ESP form fast path", "sudo systemctl stop ze",
			"Without systemd, start ze with the capabilities named",
		}},
		{name: "the set cannot be read", warnings: []string{"running without root; cannot read capabilities: permission denied"}, contains: []string{
			"cannot read capabilities: permission denied",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := daemonPrivilegeWarnings
			daemonPrivilegeWarnings = func() []string { return tc.warnings }
			t.Cleanup(func() { daemonPrivilegeWarnings = saved })

			diags := checkDaemonCapabilities(diagnostic.DoctorCheckContext{})
			if tc.warnings == nil {
				if len(diags) != 0 {
					t.Fatalf("diagnostics = %v, want none", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("diagnostics = %v, want one", diags)
			}
			d := diags[0]
			if d.Code != diagnostic.CodeDoctorDaemonCapabilityMissing || d.Severity != diagnostic.SeverityWarning {
				t.Errorf("code %q severity %q, want %q warning", d.Code, d.Severity, diagnostic.CodeDoctorDaemonCapabilityMissing)
			}
			for _, text := range tc.contains {
				if !strings.Contains(d.Message, text) {
					t.Errorf("message does not carry %q: %q", text, d.Message)
				}
			}
			if d.Fix != privilege.DaemonCapabilitiesFix {
				t.Errorf("fix = %q, want %q", d.Fix, privilege.DaemonCapabilitiesFix)
			}
		})
	}
}
