package vrrp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

// VALIDATES: AC-15 and AC-22 -- every VRRP table name is reserved by the
// owner that registers it: another owner's registration of the name is
// refused, naming the VRRP owner, whether or not VRRP has registered anything.
// PREVENTS: an operator table `vrrp_owner_nd` merging into the VRRP owner
// table (plan/journal/two-owners-share-one-name.md, 2026-09-28).
func TestOwnerTablesAreReserved(t *testing.T) {
	for _, tc := range []struct {
		table string
		owner string
	}{
		{acceptFilterTableName, acceptFilterOwner},
		{ownerFilterARPTableName, ownerFilterOwner},
		{ownerFilterNDTableName, ownerFilterOwner},
		{backupFilterTableName, backupFilterOwner},
	} {
		t.Run(tc.table, func(t *testing.T) {
			err := firewall.RegisterTables("firewall", []firewall.Table{{Name: tc.table, Family: firewall.FamilyARP}})
			t.Cleanup(func() { _ = firewall.RegisterTables("firewall", nil) })
			if err == nil {
				t.Fatalf("table %q registered by the operator's owner, want a refusal", tc.table)
			}
			if !strings.Contains(err.Error(), `"`+tc.owner+`"`) {
				t.Errorf("error %q does not name the reserving owner %q", err, tc.owner)
			}
		})
	}
}
