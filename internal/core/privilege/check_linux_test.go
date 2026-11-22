// Design: docs/architecture/system-architecture.md -- daemon privilege checking
// Related: check_linux.go -- CheckPrivileges, under test

//go:build linux

package privilege

import "testing"

// TestCheckPrivilegesInventoriesTheSet reads the test process's own inventory.
//
// VALIDATES: on Linux the effective set is read, so CheckPrivileges answers no
// error, and every capability it answers missing is one of DaemonCapabilities,
// in the set's order, with its bit absent from the effective set.
// PREVENTS: an inventory that names a capability outside the set the unit
// grants, or one the process holds.
// METHOD: compare the answer with effectiveCapabilities and the declared order;
// it holds whether the test runs as root, with some capabilities, or with none.
func TestCheckPrivilegesInventoriesTheSet(t *testing.T) {
	missing, err := CheckPrivileges()
	if err != nil {
		t.Fatalf("CheckPrivileges: %v", err)
	}
	eff, err := effectiveCapabilities()
	if err != nil {
		t.Fatalf("effectiveCapabilities: %v", err)
	}
	next := 0
	for _, c := range missing {
		found := false
		for next < len(daemonCapabilities) {
			declared := daemonCapabilities[next]
			next++
			if declared.Name == c.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is answered missing but is not in the set, or not in its order", c.Name)
		}
		if eff&(1<<c.Bit) != 0 {
			t.Errorf("%s is answered missing but the effective set holds it", c.Name)
		}
	}
}
