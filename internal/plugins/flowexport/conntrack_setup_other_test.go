//go:build !(linux && ze_appliance)

package flowexport

import "testing"

// VALIDATES: off the gokrazy appliance, ensureConntrackTracking is a safe no-op --
// the conntrack worker calls it unconditionally on Start(), so it must not panic
// or touch the kernel when ze was not built with the ze_appliance tag.
// PREVENTS: a non-appliance build (dev, distro) attempting modprobe / nft-rule /
// sysctl setup at flow-export startup, where the operator or firewall owns it.
func TestEnsureConntrackTrackingIsNoOpOffAppliance(t *testing.T) {
	ensureConntrackTracking(nil) // must not panic, even with a nil logger
}
