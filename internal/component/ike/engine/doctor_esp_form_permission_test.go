// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- firewall readings and their operator fixes.
// Related: doctor_esp_form_fast_path.go -- the unreadable-ruleset warning.
package engine

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/privilege"
)

// TestDoctorESPFormFirewallPermissionCauseRetainsUncertainty checks the warning
// for a failed namespace entry. The cause identifies the refused operation, but
// does not establish that CAP_NET_ADMIN is absent. The operator must still get
// the declared-capability guidance, restart command and diagnostic log command.
func TestDoctorESPFormFirewallPermissionCauseRetainsUncertainty(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	cause := "watch the nftables ruleset for commits: open an nftables connection to watch the ruleset: setns: operation not permitted"
	finding := espFormFirewallUnread(cause)
	if finding.Code != dataplane.CodeDoctorIPsecESPFormFirewall {
		t.Errorf("finding code = %q, want %q", finding.Code, dataplane.CodeDoctorIPsecESPFormFirewall)
	}
	if finding.Severity != "warning" {
		t.Errorf("finding severity = %q, want warning", finding.Severity)
	}
	if finding.Fix != "sudo systemctl restart ze" {
		t.Errorf("finding fix = %q, want the exact restart command", finding.Fix)
	}
	for _, want := range []string{
		cause,
		"off for safety",
		"Restart ze with the command below",
		"daemon-capabilities",
		"CAP_NET_ADMIN",
		"ze show log recent level warn component ike.dataplane",
	} {
		if !strings.Contains(finding.Message, want) {
			t.Errorf("finding message lacks %q: %s", want, finding.Message)
		}
	}
	if strings.Contains(finding.Message, "lacks CAP_NET_ADMIN") {
		t.Errorf("a refused setns operation does not establish a missing CAP_NET_ADMIN: %s", finding.Message)
	}

	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
	other := espFormFirewallUnread(cause)
	if strings.Contains(other.Fix, "systemctl") {
		t.Errorf("outside ze.service, finding fix carries systemctl: %q", other.Fix)
	}
	if !strings.Contains(other.Fix, "ze signal stop") {
		t.Errorf("outside ze.service, finding fix = %q, want stop-and-start instructions", other.Fix)
	}
}
