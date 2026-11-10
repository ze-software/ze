package ifacenetlink

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

// VALIDATES: AC-15 and AC-22 -- the IPv6 reverse-path table ze_iface_rpf is
// reserved for the iface backend, which writes it over netlink without ever
// registering it: the firewall engine's registration of the name, which is
// what an operator table `iface_rpf` becomes, is refused naming the owner.
// PREVENTS: the firewall nft backend and ReplaceIPv6RPF sharing one kernel
// table, each deleting the other's chains and sets on its next apply.
func TestRPFTableIsReserved(t *testing.T) {
	err := firewall.RegisterTables("firewall", []firewall.Table{{Name: rpfTableName, Family: firewall.FamilyIP6}})
	t.Cleanup(func() { _ = firewall.RegisterTables("firewall", nil) })
	if err == nil {
		t.Fatalf("table %q registered by the operator's owner, want a refusal", rpfTableName)
	}
	if !strings.Contains(err.Error(), `"`+rpfTableOwner+`"`) {
		t.Errorf("error %q does not name the reserving owner %q", err, rpfTableOwner)
	}
}
