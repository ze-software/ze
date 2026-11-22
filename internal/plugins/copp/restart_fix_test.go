// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- doctor check
// Related: doctor.go -- restoreHint, the restart the copp-input-chain finding names

package copp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: the doctor-copp-missing finding names the restart for this process:
// `sudo systemctl restart ze` when ze runs under ze.service, and the stop-and-start
// steps (ze signal stop, then ze start) for a ze started another way, with no
// systemctl. Neither names `ze signal restart`, which exits without starting a new
// process.
// METHOD: fix the membership through privilege.SetServiceMembershipForTest, stand in
// a kernel holding no ze_copp table, and read the finding copp-input-chain answers.
// PREVENTS: an operator whose ze runs outside a supervisor told to run a command that
// stops ze and leaves CoPP uninstalled, or told to run systemctl for a ze systemd did
// not start.
func TestCoppDoctorNamesTheRestartForThisProcess(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("control-plane-protection").GetOrCreateContainer("bgp").Set("rate", "100/second")
	tree.GetOrCreateContainer("bgp").AddListEntry("peer", "edge", peerTree("192.0.2.1"))

	saved := readKernelTables
	readKernelTables = func() ([]firewall.Table, error) { return nil, nil }
	t.Cleanup(func() { readKernelTables = saved })

	message := func(t *testing.T) string {
		t.Helper()
		diags := checkCoppInputChain(registry.DoctorCheckContext{Tree: tree})
		if len(diags) != 1 {
			t.Fatalf("diagnostics = %+v, want one for the missing table", diags)
		}
		return diags[0].Message
	}

	t.Run("ze.service", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
		got := message(t)
		const want = " Restart Ze so the daemon installs the table again, then rerun `ze doctor`. " +
			"To restart it: sudo systemctl restart ze."
		if !strings.HasSuffix(got, want) {
			t.Fatalf("under ze.service the finding is %q, want it to end %q", got, want)
		}
		if strings.Contains(got, "ze signal restart") {
			t.Errorf("the finding names ze signal restart, which leaves ze stopped: %q", got)
		}
	})
	t.Run("other", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
		got := message(t)
		if strings.Contains(got, "systemctl") {
			t.Errorf("outside ze.service the finding names systemctl: %q", got)
		}
		if strings.Contains(got, "ze signal restart") {
			t.Errorf("the finding names ze signal restart, which leaves ze stopped: %q", got)
		}
		for _, want := range []string{"ze signal stop", "ze start"} {
			if !strings.Contains(got, want) {
				t.Errorf("outside ze.service the finding does not name %q: %q", want, got)
			}
		}
	})
}
