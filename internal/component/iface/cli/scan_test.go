// Design: docs/features/interfaces.md -- interface discovery CLI
//
// VALIDATES: `ze interface scan --managed` keeps only the interface kinds Ze
//   creates and deletes, so an operator adopting what a box already carries is
//   never offered a kind Ze cannot manage.
// PREVENTS: a kind silently joining or leaving the managed set.

package cli

import (
	"testing"

	ifacepkg "github.com/ze-software/ze/internal/component/iface"
)

// scanFixture is a discovered set with one row of each kind the filter keeps
// and one it drops.
func scanFixture() []ifacepkg.DiscoveredInterface {
	return []ifacepkg.DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "02:00:00:00:00:01"},
		{Name: "ze0", Type: ifaceTypeVeth, MAC: "02:00:00:00:00:02"},
	}
}

func TestFilterManagedKeepsOnlyTheKindsZeCreates(t *testing.T) {
	t.Parallel()

	filtered := filterManaged(scanFixture())
	if len(filtered) != 1 || filtered[0].Name != "ze0" {
		t.Fatalf("filterManaged = %#v, want only the veth row", filtered)
	}
}
