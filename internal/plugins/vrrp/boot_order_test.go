// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- instance lifecycle
// Related: engine.go -- build, which creates the virtual-MAC device before the instance
// Related: register.go -- createMacvlan and waitDevicePresent, the live device wait
//
// VALIDATES: at boot no gratuitous ARP or Neighbor Advertisement leaves before
// the Virtual Router MAC device exists: the engine builds no instance while
// the device cannot be created, and once it exists the device comes first.
// PREVENTS: an engine that starts the worker, and so announces, before the
// virtual-MAC device is configured.
package vrrp

import (
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestEngineAnnouncesNothingBeforeTheVirtualMACDevice proves the Virtual
// Router MAC half of the boot delay, for an IPv4 and an IPv6 owner.
//
// Method: the engine applies an owner group, which would announce as soon as
// its worker starts. With the virtual-MAC device creation failing (the live
// createMacvlan returns waitDevicePresent's error when the device never
// appears, TestWaitDevicePresentMissing), no instance exists and nothing is
// announced or installed. With creation succeeding, the recorded order is the
// device first, then the address install, then the announcement.
//
// RFC requirement: RFC9568-8.1.2-2 positive -- for an IPv4 group the engine creates the Virtual Router MAC device before it builds the instance, so the gratuitous ARP follows both the device and the address install (build engine.go, execute instance.go)
// RFC requirement: RFC9568-8.1.2-2 negative -- while the Virtual Router MAC device cannot be created the engine builds no instance, so no gratuitous ARP and no address install happen (build engine.go)
// RFC requirement: RFC9568-8.2.2-4 positive -- for an IPv6 group the engine creates the Virtual Router MAC device before it builds the instance, so the unsolicited Neighbor Advertisement follows both the device and the address install (build engine.go, execute instance.go)
// RFC requirement: RFC9568-8.2.2-4 negative -- while the Virtual Router MAC device cannot be created the engine builds no instance, so no Neighbor Advertisement and no address install happen (build engine.go)
// RFC requirement: RFC5798-8.2.2-4 positive -- for an IPv6 group the engine creates the virtual router MAC device before it builds the instance, so the unsolicited Neighbor Advertisement follows both the device and the address install (build engine.go, execute instance.go)
// RFC requirement: RFC5798-8.2.2-4 negative -- while the virtual router MAC device cannot be created the engine builds no instance, so no Neighbor Advertisement and no address install happen (build engine.go)
func TestEngineAnnouncesNothingBeforeTheVirtualMACDevice(t *testing.T) {
	cases := []struct {
		name string
		spec GroupSpec
	}{
		{"ipv4", testSpec()},
		{"ipv6", testSpecV6()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.spec
			spec.IsOwner = true

			var mu sync.Mutex
			var order []string
			record := func(step string) {
				mu.Lock()
				order = append(order, step)
				mu.Unlock()
			}
			snapshot := func() []string {
				mu.Lock()
				defer mu.Unlock()
				return slices.Clone(order)
			}

			p := newFakePlatform()
			platform := p.platform()
			create := platform.createMacvlan
			platform.createMacvlan = func(dev, parent, owner string, mac [6]byte) error {
				if err := create(dev, parent, owner, mac); err != nil {
					return err
				}
				record("device")
				return nil
			}
			f := &fakeDeps{}
			deps := f.deps()
			install, announce := deps.installVIPs, deps.announceMaster
			deps.installVIPs = func(dev, owner string, cidrs []string) error {
				record("install")
				return install(dev, owner, cidrs)
			}
			deps.announceMaster = func(key transport.InstanceKey, vips []netip.Addr) {
				record("announce")
				announce(key, vips)
			}

			p.macvlanErr = errors.New("macvlan did not appear")
			failing := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), platform, deps)
			failing.apply([]GroupSpec{spec})
			if n := len(failing.instances); n != 0 {
				t.Fatalf("instances = %d with no virtual-MAC device, want 0", n)
			}
			failing.stopAll()
			if got := snapshot(); len(got) != 0 {
				t.Fatalf("steps without a virtual-MAC device = %v, want none", got)
			}

			p.macvlanErr = nil
			eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), platform, deps)
			t.Cleanup(eng.stopAll)
			eng.apply([]GroupSpec{spec})
			deadline := time.Now().Add(2 * time.Second)
			for !slices.Contains(snapshot(), "announce") {
				if time.Now().After(deadline) {
					t.Fatalf("the owner never announced: steps = %v", snapshot())
				}
				time.Sleep(time.Millisecond)
			}
			got := snapshot()
			if len(got) < 3 || got[0] != "device" || got[1] != "install" || got[2] != "announce" {
				t.Fatalf("steps = %v, want the device, then the install, then the announcement", got)
			}
		})
	}
}
