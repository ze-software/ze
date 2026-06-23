// VALIDATES: ensureNetwork keys off install-server reachability, not the mere
// presence of a default route.
// PREVENTS: regression of the dual-homed-target stall where the kernel's
// ip=dhcp configured a corporate NIC, leaving a default route that does not
// reach the install server, and the installer never brought up the install NIC.

package disk

import "testing"

func swapNetProbes(probe func(string) bool, bringUp func() error) func() {
	origProbe, origBringUp := probeServer, bringUpAllNICs
	probeServer, bringUpAllNICs = probe, bringUp
	return func() { probeServer, bringUpAllNICs = origProbe, origBringUp }
}

// When the install server is already reachable (kernel configured the right
// NIC), ensureNetwork must not touch the NICs.
func TestEnsureNetworkSkipsDHCPWhenServerReachable(t *testing.T) {
	restore := swapNetProbes(
		func(string) bool { return true },
		func() error { t.Fatal("bringUpAllNICs called although server was reachable"); return nil },
	)
	defer restore()

	if err := ensureNetwork("198.19.255.1", "80", 5); err != nil {
		t.Fatalf("ensureNetwork: %v", err)
	}
}

// Pins the dual-homed-target regression: a default route exists (kernel grabbed
// a corporate NIC) but the install server is unreachable. ensureNetwork must
// bring up all NICs and re-probe rather than trusting the stray default route.
func TestEnsureNetworkBringsUpNICsWhenServerUnreachable(t *testing.T) {
	probes := 0
	broughtUp := false
	restore := swapNetProbes(
		func(string) bool { probes++; return probes > 1 }, // unreachable first, reachable after bring-up
		func() error { broughtUp = true; return nil },
	)
	defer restore()

	if err := ensureNetwork("198.19.255.1", "80", 5); err != nil {
		t.Fatalf("ensureNetwork: %v", err)
	}
	if !broughtUp {
		t.Fatal("bringUpAllNICs not called although the install server was unreachable")
	}
}

// ze.wait=0 is an explicit opt-out: no probe, no NIC changes.
func TestEnsureNetworkWaitZeroSkipsEverything(t *testing.T) {
	restore := swapNetProbes(
		func(string) bool { t.Fatal("probeServer called with ze.wait=0"); return false },
		func() error { t.Fatal("bringUpAllNICs called with ze.wait=0"); return nil },
	)
	defer restore()

	if err := ensureNetwork("198.19.255.1", "80", 0); err != nil {
		t.Fatalf("ensureNetwork: %v", err)
	}
}
