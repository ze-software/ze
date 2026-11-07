package firewallvpp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

// TestVPPRefusesCtHelper proves the VPP backend refuses conntrack helper
// tables at verify, naming the backend and the config that asked for them.
//
// VALIDATES: AC-8, a table with a ct helper object and a ct helper assignment
// is refused, and the error names backend vpp and system conntrack module.
// PREVENTS: a helper the operator listed that VPP silently never assigns.
func TestVPPRefusesCtHelper(t *testing.T) {
	tables, err := firewall.ConntrackHelperTables([]string{"ftp"})
	if err != nil {
		t.Fatalf("ConntrackHelperTables: %v", err)
	}
	err = Verify(tables)
	if err == nil {
		t.Fatal("backend vpp accepted a conntrack helper table")
	}
	for _, want := range []string{"backend vpp", "system conntrack module", "ct helper assignment"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}
