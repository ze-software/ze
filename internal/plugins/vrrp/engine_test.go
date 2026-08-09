// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- instance manager (config diff + lifecycle) tests
//
// VALIDATES: AC-6 (idle with zero groups), AC-7 (instance created on commit),
// AC-13 (instance deleted on config removal), and the reconfigure-in-place rule.
// PREVENTS: a config reload silently restarting a Master (which would blackhole
// traffic for a full master-down interval), and orphaned macvlans/owners.

package vrrp

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
	"github.com/ze-software/ze/internal/test/sim"
)

// fakePlatform records the transport + iface calls the manager makes.
type fakePlatform struct {
	opened     []transport.InstanceSpec
	closed     []transport.InstanceKey
	devices    map[string]string // device name -> owner
	deleted    []string
	applied    []string // "parent/vmac/family" per applyDataplane
	reasserted []string // "parent/vmac/family" per reassertDataplane
	reverted   []string // "parent/vmac/family" per revertDataplane
}

func newFakePlatform() *fakePlatform {
	return &fakePlatform{devices: map[string]string{}}
}

func (p *fakePlatform) platform() enginePlatform {
	return enginePlatform{
		openInstance: func(spec transport.InstanceSpec) (transport.InstanceKey, error) {
			p.opened = append(p.opened, spec)
			return transport.InstanceKey{Interface: spec.Parent, VRID: spec.VRID, Family: spec.Family}, nil
		},
		closeInstance: func(key transport.InstanceKey) error {
			p.closed = append(p.closed, key)
			return nil
		},
		createMacvlan: func(dev, parent, owner string, _ [6]byte) error {
			p.devices[dev] = owner
			return nil
		},
		deleteMacvlan: func(owner, dev string) {
			delete(p.devices, dev)
			p.deleted = append(p.deleted, dev)
		},
		applyDataplane: func(parent, vmac, family string) error {
			p.applied = append(p.applied, parent+"/"+vmac+"/"+family)
			return nil
		},
		reassertDataplane: func(parent, vmac, family string) {
			p.reasserted = append(p.reasserted, parent+"/"+vmac+"/"+family)
		},
		revertDataplane: func(parent, vmac, family string) {
			p.reverted = append(p.reverted, parent+"/"+vmac+"/"+family)
		},
		parentIfindex: func(string) (int, error) { return 2, nil },
	}
}

func newTestEngine(t *testing.T) (*engine, *fakePlatform) {
	t.Helper()
	p := newFakePlatform()
	f := &fakeDeps{}
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), p.platform(), f.deps())
	t.Cleanup(eng.stopAll)
	return eng, p
}

// TestEngineIdleWithoutGroups proves the plugin is inert when interfaces are
// configured but no vrrp group is (umbrella AC-6/A-4): the plugin auto-loads
// with the shared `interface` root, so "loaded" must not mean "doing anything".
func TestEngineIdleWithoutGroups(t *testing.T) {
	eng, p := newTestEngine(t)
	eng.apply(nil)

	if n := len(eng.instances); n != 0 {
		t.Fatalf("instances = %d, want 0 with no groups configured", n)
	}
	if len(p.opened) != 0 {
		t.Errorf("opened %d transport instances with no groups; want none", len(p.opened))
	}
	if len(p.devices) != 0 {
		t.Errorf("created %d macvlans with no groups; want none", len(p.devices))
	}
}

// TestEngineCreatesInstance proves a committed group brings up exactly one
// instance, with its macvlan created BEFORE the transport opens (the tx socket
// binds to that device) -- umbrella AC-7 and R-4.
func TestEngineCreatesInstance(t *testing.T) {
	eng, p := newTestEngine(t)
	eng.apply([]GroupSpec{testSpec()})

	if n := len(eng.instances); n != 1 {
		t.Fatalf("instances = %d, want 1", n)
	}
	if len(p.devices) != 1 {
		t.Fatalf("macvlans = %d, want 1 (device is created at instance create, not at Master)", len(p.devices))
	}
	if len(p.opened) != 1 {
		t.Fatalf("transport instances = %d, want 1", len(p.opened))
	}
	spec := p.opened[0]
	if spec.MacvlanDevice == "" {
		t.Error("transport must bind tx to the macvlan device")
	}
	if _, ok := p.devices[spec.MacvlanDevice]; !ok {
		t.Errorf("macvlan %q must exist before the transport binds to it", spec.MacvlanDevice)
	}
	// RFC 9568 Section 7.3: the virtual MAC is 00-00-5E-00-01-{VRID} for IPv4.
	want := [6]byte{0x00, 0x00, 0x5e, 0x00, 0x01, 10}
	if spec.VirtualMAC != want {
		t.Errorf("virtual MAC = %v, want %v", spec.VirtualMAC, want)
	}
}

// TestEngineReconfiguresInPlace proves a leaf change re-applies to the running
// instance rather than deleting and recreating it.
func TestEngineReconfiguresInPlace(t *testing.T) {
	eng, p := newTestEngine(t)
	eng.apply([]GroupSpec{testSpec()})

	next := testSpec()
	next.Priority = 150
	eng.apply([]GroupSpec{next})

	if len(p.closed) != 0 {
		t.Fatalf("a priority change must not close the transport instance, closed %v", p.closed)
	}
	if len(p.deleted) != 0 {
		t.Fatalf("a priority change must not delete the macvlan, deleted %v", p.deleted)
	}
	if n := len(eng.instances); n != 1 {
		t.Fatalf("instances = %d, want 1", n)
	}
}

// TestEngineDeletesRemovedInstance proves removing a group from config tears the
// instance down completely: transport closed, macvlan deleted (umbrella AC-13).
func TestEngineDeletesRemovedInstance(t *testing.T) {
	eng, p := newTestEngine(t)
	eng.apply([]GroupSpec{testSpec()})
	eng.apply(nil)

	if n := len(eng.instances); n != 0 {
		t.Fatalf("instances = %d, want 0 after removal", n)
	}
	if len(p.closed) != 1 {
		t.Errorf("transport instances closed = %d, want 1", len(p.closed))
	}
	if len(p.deleted) != 1 {
		t.Errorf("macvlans deleted = %d, want 1", len(p.deleted))
	}
	if len(p.devices) != 0 {
		t.Errorf("macvlan leaked: %v", p.devices)
	}
	if len(p.reverted) != 1 {
		t.Errorf("dataplane reverts = %d, want 1 (parent sysctls must be restored on teardown)", len(p.reverted))
	}
}

// TestEngineAppliesDataplaneOnCreate proves the virtual-MAC ARP recipe is applied
// when the instance is created (not at Master), so a host resolving the VIP once
// the instance advertises already gets the virtual MAC (spec-vrrp-6 AC-1/AC-2).
func TestEngineAppliesDataplaneOnCreate(t *testing.T) {
	eng, p := newTestEngine(t)
	eng.apply([]GroupSpec{testSpec()})

	if len(p.applied) != 1 {
		t.Fatalf("dataplane applies = %d, want 1 at instance create", len(p.applied))
	}
	if len(p.reverted) != 0 {
		t.Errorf("dataplane reverted %d times before teardown, want 0", len(p.reverted))
	}
	if len(p.reasserted) != 1 {
		t.Errorf("dataplane reasserts = %d after one apply, want 1", len(p.reasserted))
	}
}

// TestEngineReassertsDataplaneOnEveryApply proves the dataplane recipe is
// re-asserted for a RUNNING (unchanged) instance on a subsequent apply, so an
// iface config re-emit that clobbered the parent's ARP sysctls self-heals
// (spec-vrrp-6 review finding: iface/sysctl coordination).
func TestEngineReassertsDataplaneOnEveryApply(t *testing.T) {
	eng, p := newTestEngine(t)
	spec := testSpec()
	eng.apply([]GroupSpec{spec}) // create + 1 reassert
	eng.apply([]GroupSpec{spec}) // unchanged instance, still reasserts

	if len(p.applied) != 1 {
		t.Errorf("dataplane applies = %d, want 1 (create only, not re-applied)", len(p.applied))
	}
	if len(p.reasserted) != 2 {
		t.Errorf("dataplane reasserts = %d after two applies, want 2 (once per apply)", len(p.reasserted))
	}
}

// TestEngineSeparateVRIDNamespacesPerFamily proves an IPv4 and an IPv6 group
// with the SAME VRID on the same unit are two independent virtual routers with
// distinct devices and MACs.
//
// RFC 9568 Section 1.2: IPv4 and IPv6 virtual routers are separate.
func TestEngineSeparateVRIDNamespacesPerFamily(t *testing.T) {
	eng, p := newTestEngine(t)
	v4 := testSpec()
	v6 := testSpec()
	v6.Family = familyIPv6
	v6.VIPs = []netip.Addr{netip.MustParseAddr("fe80::1")}
	eng.apply([]GroupSpec{v4, v6})

	if n := len(eng.instances); n != 2 {
		t.Fatalf("instances = %d, want 2 (same VRID, different families)", n)
	}
	if len(p.devices) != 2 {
		t.Fatalf("macvlans = %d, want 2 distinct devices", len(p.devices))
	}
	macs := map[[6]byte]bool{}
	for _, s := range p.opened {
		macs[s.VirtualMAC] = true
	}
	if len(macs) != 2 {
		t.Fatalf("virtual MACs = %d, want 2 distinct (IPv4 00:00:5e:00:01:xx, IPv6 00:00:5e:00:02:xx)", len(macs))
	}
	// RFC 9568 Section 7.3: IPv6 uses the 00-00-5E-00-02-{VRID} block.
	wantV6 := [6]byte{0x00, 0x00, 0x5e, 0x00, 0x02, 10}
	if !macs[wantV6] {
		t.Errorf("no IPv6 virtual MAC %v among %v", wantV6, macs)
	}
}
