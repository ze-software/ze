// VALIDATES: the OSPF Graceful Restart install-suppression gate (RFC 3623 sec 2/2.1) makes
// Installer.Apply and Installer.RemoveAll no-ops while suppressed, so the restarting router
// neither churns routes in-restart nor withdraws them on the graceful stop, retaining the FIB
// (AC-8). When suppression clears, install resumes.
// PREVENTS: a route flap (or full FIB withdrawal) during a graceful restart -- the exact black
// hole GR must avoid.
package spf

import (
	"net/netip"
	"testing"
)

func TestInstallerGracefulRestartSuppress(t *testing.T) {
	in := NewInstaller(nil) // nil Loc-RIB: install/remove track the computed set without kernel writes
	route := RouteEntry{
		Prefix:   netip.MustParsePrefix("10.1.0.0/24"),
		Metric:   10,
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}},
	}
	suppress := true
	in.setSuppress(func() bool { return suppress })

	// While suppressed, Apply installs nothing (SPF still ran to produce `route`).
	in.Apply([]RouteEntry{route})
	if n := len(in.Installed()); n != 0 {
		t.Fatalf("Apply must be a no-op while suppressed; installed %d routes", n)
	}

	// Clearing suppression resumes install (GR exit).
	suppress = false
	in.Apply([]RouteEntry{route})
	if n := len(in.Installed()); n != 1 {
		t.Fatalf("Apply must install after suppression clears; installed %d routes", n)
	}

	// Re-suppress (a graceful stop): RemoveAll must NOT withdraw the retained routes.
	suppress = true
	in.RemoveAll()
	if n := len(in.Installed()); n != 1 {
		t.Fatalf("RemoveAll must be a no-op while suppressed (FIB retained); installed %d routes", n)
	}
}
