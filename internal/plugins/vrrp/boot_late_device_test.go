// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- instance lifecycle
// Related: register.go -- macvlanCreator, the live create and device wait
// Related: engine.go -- build, which opens the transport after the device exists
//
// VALIDATES: at boot the engine sends no gratuitous ARP or Neighbor
// Advertisement before the Virtual Router MAC device exists, when iface's
// owned-device registry creates that device later than the registration.
// PREVENTS: a create that returns at registration time, so the transport
// resolves a device the kernel does not hold yet.
package vrrp

import (
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
	"github.com/ze-software/ze/internal/test/sim"
)

// lateDeviceWorld models the live boot path with the kernel replaced: the
// registry creates the device some time after it is registered, as iface's
// reconcile pass does, and the transport refuses to open an instance for a
// device the kernel does not hold, as its by-name resolve does.
type lateDeviceWorld struct {
	mu      sync.Mutex
	present map[string]bool
	order   []string
}

func (w *lateDeviceWorld) step(name string) {
	w.mu.Lock()
	w.order = append(w.order, name)
	w.mu.Unlock()
}

func (w *lateDeviceWorld) steps() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.order)
}

func (w *lateDeviceWorld) has(dev string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.present[dev]
}

// register records the device and creates it after delay. A zero delay means
// the device never appears.
func (w *lateDeviceWorld) register(delay time.Duration) func(string, iface.MacvlanSpec) error {
	return func(_ string, spec iface.MacvlanSpec) error {
		if delay == 0 {
			return nil
		}
		time.AfterFunc(delay, func() {
			w.mu.Lock()
			w.present[spec.Name] = true
			w.order = append(w.order, "device")
			w.mu.Unlock()
		})
		return nil
	}
}

// engine builds an engine whose platform uses the live macvlanCreator over
// this world, and whose announce and install record their order.
func (w *lateDeviceWorld) engine(delay, timeout time.Duration) *engine {
	platform := newFakePlatform().platform()
	platform.createMacvlan = macvlanCreator{
		register: w.register(delay),
		present:  w.has,
		timeout:  timeout,
	}.create
	open := platform.openInstance
	platform.openInstance = func(spec transport.InstanceSpec) (transport.InstanceKey, error) {
		if !w.has(spec.MacvlanDevice) {
			return transport.InstanceKey{}, errors.New("resolve macvlan " + spec.MacvlanDevice + ": no such network interface")
		}
		return open(spec)
	}
	f := &fakeDeps{}
	deps := f.deps()
	install, announce := deps.installVIPs, deps.announceMaster
	deps.installVIPs = func(dev, owner string, cidrs []string) error {
		w.step("install")
		return install(dev, owner, cidrs)
	}
	deps.announceMaster = func(key transport.InstanceKey, vips []netip.Addr) {
		w.step("announce")
		announce(key, vips)
	}
	return newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), platform, deps)
}

// TestEngineWaitsForALateVirtualMACDevice proves the boot delay against a
// Virtual Router MAC device that appears after its registration, for an IPv4
// and an IPv6 owner, which announce as soon as the worker starts.
//
// Method: the live create (macvlanCreator.create) runs over a registry that
// creates the device 30ms late and a transport that cannot open an instance
// for an absent device. The engine announces only once the device exists, and
// the recorded order is the device, the address install, then the
// announcement. A create that did not wait would hand the transport an absent
// device, so no instance and no announcement would exist. With a device that
// never appears, the create fails at its deadline and nothing is installed or
// announced.
//
// RFC requirement: RFC9568-8.1.2-2 positive -- for an IPv4 owner whose Virtual Router MAC device the registry creates late, the live create waits for the device (macvlanCreator.create register.go), so the device exists before the address install and the gratuitous ARP (the ipv4 case)
// RFC requirement: RFC9568-8.1.2-2 negative -- when the Virtual Router MAC device never appears, the create fails at its deadline, the engine builds no instance, and no address install and no gratuitous ARP happen (the ipv4 case)
// RFC requirement: RFC9568-8.2.2-4 positive -- for an IPv6 owner whose Virtual Router MAC device the registry creates late, the live create waits for the device (macvlanCreator.create register.go), so the device exists before the address install and the unsolicited Neighbor Advertisement (the ipv6 case)
// RFC requirement: RFC9568-8.2.2-4 negative -- when the Virtual Router MAC device never appears, the create fails at its deadline, the engine builds no instance, and no address install and no Neighbor Advertisement happen (the ipv6 case)
// RFC requirement: RFC5798-8.2.2-4 positive -- for an IPv6 owner whose virtual router MAC device the registry creates late, the live create waits for the device (macvlanCreator.create register.go), so the device exists before the address install and the unsolicited Neighbor Advertisement (the ipv6 case)
// RFC requirement: RFC5798-8.2.2-4 negative -- when the virtual router MAC device never appears, the create fails at its deadline, the engine builds no instance, and no address install and no Neighbor Advertisement happen (the ipv6 case).
func TestEngineWaitsForALateVirtualMACDevice(t *testing.T) {
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

			never := &lateDeviceWorld{present: map[string]bool{}}
			failing := never.engine(0, 100*time.Millisecond)
			failing.apply([]GroupSpec{spec})
			if n := len(failing.instances); n != 0 {
				t.Fatalf("instances = %d for a device that never appears, want 0", n)
			}
			failing.stopAll()
			if got := never.steps(); len(got) != 0 {
				t.Fatalf("steps for a device that never appears = %v, want none", got)
			}

			late := &lateDeviceWorld{present: map[string]bool{}}
			eng := late.engine(30*time.Millisecond, 2*time.Second)
			t.Cleanup(eng.stopAll)
			eng.apply([]GroupSpec{spec})
			if n := len(eng.instances); n != 1 {
				t.Fatalf("instances = %d once the late device appears, want 1: steps = %v", n, late.steps())
			}
			deadline := time.Now().Add(2 * time.Second)
			for !slices.Contains(late.steps(), "announce") {
				if time.Now().After(deadline) {
					t.Fatalf("the owner never announced: steps = %v", late.steps())
				}
				time.Sleep(time.Millisecond)
			}
			got := late.steps()
			if len(got) < 3 || got[0] != "device" || got[1] != "install" || got[2] != "announce" {
				t.Fatalf("steps = %v, want the device, then the install, then the announcement", got)
			}
		})
	}
}
