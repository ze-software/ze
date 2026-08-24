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
	parents    map[string]string // macvlan device name -> the parent it was created on
	deleted    []string
	applied    []string // "parent/vmac/family" per applyDataplane
	reasserted []string // "parent/vmac/family" per reassertDataplane
	reverted   []string // "parent/vmac/family" per revertDataplane
	// resolveParent answers a logical interface name's hardware selector.
	// Identity by default, so a test that configures no selector reads back the
	// name it wrote; a test that does configure one replaces this.
	resolveParent deviceResolver
	// macvlanErr, when set, fails every createMacvlan. It stands in for the
	// failure a group cannot control: reconcileOwnedDevices fails fast on the
	// first owned-device error in a pass, so an unrelated device times this
	// group's waitDevicePresent out.
	macvlanErr error
	// ifindexErr, when set, fails every parentIfindex. The real one errors
	// whenever iface.Resolve does, which is the whole transient class: no
	// backend loaded, a failed interface listing, a selector that answers
	// nothing this pass. Without it the fake has no error arm at all and any
	// branch keyed on that failure is written blind.
	ifindexErr error
	// ifindex hands each PARENT its own kernel index, because the real one does.
	// A constant would make two different parents compose one macvlan name
	// (deviceName -> ComposeOwnedDeviceName keys on the parent's ifindex), and a
	// move would then look like a device replacing itself.
	ifindex map[string]int
	// live mirrors the transport's OWN instance map (transport.go OpenInstance /
	// CloseInstance): one entry per key, an open over a live key overwrites it
	// WITHOUT shutting the old sockets down, and a close removes whatever the
	// key holds at that moment. Appending to opened/closed cannot show a
	// replacement that closes itself, because both instances carry the same key
	// VALUE and the lists say nothing about which object each call reached.
	live map[transport.InstanceKey]string // open key -> the macvlan it serves
	// overwrote records every open that landed on a live key. The transport
	// leaks the sockets it displaces there, so this is a defect the engine must
	// never produce, not a state a test may assert around.
	overwrote []transport.InstanceKey
}

func newFakePlatform() *fakePlatform {
	return &fakePlatform{
		devices:       map[string]string{},
		parents:       map[string]string{},
		ifindex:       map[string]int{},
		live:          map[transport.InstanceKey]string{},
		resolveParent: identityDevice,
	}
}

func (p *fakePlatform) platform() enginePlatform {
	return enginePlatform{
		openInstance: func(spec transport.InstanceSpec) (transport.InstanceKey, error) {
			p.opened = append(p.opened, spec)
			key := transport.InstanceKey{Interface: spec.Parent, VRID: spec.VRID, Family: spec.Family}
			if _, held := p.live[key]; held {
				p.overwrote = append(p.overwrote, key)
			}
			p.live[key] = spec.MacvlanDevice
			return key, nil
		},
		closeInstance: func(key transport.InstanceKey) error {
			p.closed = append(p.closed, key)
			delete(p.live, key)
			return nil
		},
		createMacvlan: func(dev, parent, owner string, _ [6]byte) error {
			if p.macvlanErr != nil {
				return p.macvlanErr
			}
			p.devices[dev] = owner
			p.parents[dev] = parent
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
		parentIfindex: func(parent string) (int, error) {
			if p.ifindexErr != nil {
				return 0, p.ifindexErr
			}
			if idx, ok := p.ifindex[parent]; ok {
				return idx, nil
			}
			idx := 2 + len(p.ifindex)
			p.ifindex[parent] = idx
			return idx, nil
		},
		resolveDevice: func(name string) (string, error) { return p.resolveParent(name) },
	}
}

// renameParent makes newName the SAME netdev as oldName: one ifindex wearing two
// names, which is exactly what a kernel rename produces.
//
// It exists because the default allocator hands every unseen name its own index,
// so without it two names can NEVER share one and a rename cannot be represented
// at all. A bug reachable only by a rename would then be untestable against this
// fake however carefully the test were written -- the "green that could not have
// been red" shape, where the DOUBLE has removed the distinction under test.
func (p *fakePlatform) renameParent(oldName, newName string) {
	idx, ok := p.ifindex[oldName]
	if !ok {
		panic("renameParent: " + oldName + " has no ifindex yet; apply the config that builds on it first")
	}
	p.ifindex[newName] = idx
}

// replaceNetdev puts a NEW kernel index behind an existing parent name: the
// device that wore the name is gone and another one wears it now.
//
// It is the counterpart of renameParent, and the two together cover both ways a
// name and an ifindex can come apart. A card that re-enumerates, a driver that
// reloads and an iface apply that recreates a VLAN device all produce this one.
func (p *fakePlatform) replaceNetdev(name string) {
	idx, ok := p.ifindex[name]
	if !ok {
		panic("replaceNetdev: " + name + " has no ifindex yet; apply the config that builds on it first")
	}
	p.ifindex[name] = idx + 100
}

// soleDevice returns the one macvlan this platform holds, failing the test when
// there is not exactly one. Ranging over the map would pass vacuously in the
// state the defect produces, which is the device having been removed.
func (p *fakePlatform) soleDevice(t *testing.T) string {
	t.Helper()
	if len(p.devices) != 1 {
		t.Fatalf("macvlans = %v, want exactly 1", p.devices)
	}
	for dev := range p.devices {
		return dev
	}
	return ""
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
