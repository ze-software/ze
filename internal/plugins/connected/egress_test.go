// Design: docs/architecture/core-design.md -- connected covering-path devices.
// VALIDATES: address events retain their OS devices in one connected path,
// including shared prefixes, duplicate addresses and surviving-device changes.
// PREVENTS: recursively resolved static gateways reaching the FIB without a device.

package connected

import (
	"encoding/json"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	connectedevents "github.com/ze-software/ze/internal/plugins/connected/events"
)

// egressSink captures the forked operation and applies it to an engine-side RIB.
// It performs the ordinary route-install upsert, without deriving any devices.
type egressSink struct {
	recordingSink
	rib *locrib.RIB
}

func (s *egressSink) InsertForward(fam family.Family, prefix netip.Prefix, path locrib.Path) {
	s.recordingSink.InsertForward(fam, prefix, path)
	s.rib.InsertForward(fam, prefix, path, nil)
}

func (s *egressSink) Remove(fam family.Family, prefix netip.Prefix, source redistevents.ProtocolID, instance uint32) {
	s.recordingSink.Remove(fam, prefix, source, instance)
	s.rib.Remove(fam, prefix, source, instance)
}

func egressObserver(t *testing.T, remote bool) (*routeObserver, *locrib.RIB, *egressSink, *recordingBus) {
	t.Helper()
	rib := locrib.NewRIB()
	bus := &recordingBus{}
	obs := newRouteObserver(bus)
	sink := &egressSink{rib: rib}
	if remote {
		obs.setLocRIB(nil, sink)
		return obs, rib, sink, bus
	}
	obs.setLocRIB(rib, nil)
	return obs, rib, sink, bus
}

func egressAddress(t *testing.T, name string, unit int, address string) []byte {
	t.Helper()
	payload, err := json.Marshal(addrPayload{
		Name: name, Unit: unit, Address: address, PrefixLength: 24, Family: "ipv4",
	})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func egressPath(t *testing.T, rib *locrib.RIB) locrib.Path {
	t.Helper()
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	path, matched, found := rib.LPM(family.IPv4Unicast, netip.MustParseAddr("192.0.2.42"))
	if !found {
		t.Fatal("address event published no selectable covering route")
	}
	if matched != prefix {
		t.Fatalf("cover = %s, want %s", matched, prefix)
	}
	if path.Source != connectedevents.ProtocolID {
		t.Fatalf("cover source = %d, want connected %d", path.Source, connectedevents.ProtocolID)
	}
	if path.Instance != 0 {
		t.Fatalf("connected instance = %d, want the single prefix instance 0", path.Instance)
	}
	if path.NextHop.IsValid() {
		t.Fatalf("connected gateway = %s, want no gateway so recursion terminates", path.NextHop)
	}
	group, held := rib.Lookup(family.IPv4Unicast, prefix)
	if !held {
		t.Fatal("selected cover is missing from the authority")
	}
	if len(group.Paths) != 1 {
		t.Fatalf("connected candidate count = %d, want one prefix path", len(group.Paths))
	}
	return path
}

func TestConnectedEgressDeviceFromAddressEvent(t *testing.T) {
	for _, mode := range []string{"local", "remote"} {
		for _, device := range []struct {
			name string
			unit int
			want string
		}{
			{name: "eth7", want: "eth7"},
			{name: "eth7", unit: 234, want: "eth7.234"},
		} {
			t.Run(mode+"/"+device.want, func(t *testing.T) {
				obs, rib, sink, _ := egressObserver(t, mode == "remote")
				obs.handleAddrAdded(egressAddress(t, device.name, device.unit, "192.0.2.1"))
				path := egressPath(t, rib)
				if path.Interface != device.want {
					t.Fatalf("cover device = %q, want %q", path.Interface, device.want)
				}
				if len(path.ECMP) != 0 {
					t.Fatalf("single-device cover has extra targets: %+v", path.ECMP)
				}
				if mode == "remote" {
					if sink.flushes != 1 {
						t.Fatalf("remote flush count = %d, want one delivered insert", sink.flushes)
					}
					if sink.inserted[0].Interface != device.want {
						t.Fatalf("remote operation device = %q, want %q", sink.inserted[0].Interface, device.want)
					}
				}
			})
		}
	}
}

func TestConnectedEgressDuplicatesKeepOneDevicePublication(t *testing.T) {
	obs, rib, sink, bus := egressObserver(t, true)
	first := egressAddress(t, "eth7", 234, "192.0.2.1")
	second := egressAddress(t, "eth7", 234, "192.0.2.2")
	obs.handleAddrAdded(first)
	obs.handleAddrAdded(first)
	obs.handleAddrAdded(second)
	obs.handleAddrRemoved(first)
	path := egressPath(t, rib)
	if path.Interface != "eth7.234" {
		t.Fatalf("surviving address device = %q, want eth7.234", path.Interface)
	}
	if len(sink.inserted) != 1 {
		t.Fatalf("same-device publication count = %d, want one", len(sink.inserted))
	}
	if len(sink.removed) != 0 {
		t.Fatalf("removed cover with a surviving address: %v", sink.removed)
	}
	if len(bus.events()) != 1 {
		t.Fatalf("redistribution count = %d, want one prefix add", len(bus.events()))
	}
}

func TestConnectedEgressSharedPrefixCanonicalDevices(t *testing.T) {
	for _, mode := range []string{"local", "remote"} {
		for _, order := range []struct {
			name    string
			devices []string
		}{
			{name: "reverse", devices: []string{"eth9", "eth7", "eth8"}},
			{name: "forward", devices: []string{"eth7", "eth8", "eth9"}},
		} {
			t.Run(mode+"/"+order.name, func(t *testing.T) {
				obs, rib, _, bus := egressObserver(t, mode == "remote")
				for _, device := range order.devices {
					obs.handleAddrAdded(egressAddress(t, device, 0, "192.0.2.1"))
				}
				path := egressPath(t, rib)
				if path.Interface != "eth7" {
					t.Fatalf("primary device = %q, want canonical eth7", path.Interface)
				}
				want := []locrib.NextHop{{Interface: "eth8"}, {Interface: "eth9"}}
				if !slices.Equal(path.ECMP, want) {
					t.Fatalf("other connected targets = %+v, want %+v", path.ECMP, want)
				}
				if len(bus.events()) != 1 {
					t.Fatalf("redistribution count = %d, want one prefix add", len(bus.events()))
				}
			})
		}
	}
}

func TestConnectedEgressDeviceRemovalPublishesSurvivor(t *testing.T) {
	for _, mode := range []string{"local", "remote"} {
		t.Run(mode, func(t *testing.T) {
			obs, rib, sink, bus := egressObserver(t, mode == "remote")
			selected := egressAddress(t, "eth7", 234, "192.0.2.1")
			survivor := egressAddress(t, "eth9", 0, "192.0.2.1")
			obs.handleAddrAdded(selected)
			obs.handleAddrAdded(survivor)
			obs.handleAddrRemoved(egressAddress(t, "eth8", 0, "192.0.2.1"))
			obs.handleAddrRemoved(selected)
			path := egressPath(t, rib)
			if path.Interface != "eth9" {
				t.Fatalf("surviving cover device = %q, want eth9", path.Interface)
			}
			if len(path.ECMP) != 0 {
				t.Fatalf("removed device retained in cover: %+v", path.ECMP)
			}
			if len(bus.events()) != 1 {
				t.Fatalf("survivor emitted %d redistribution operations, want one add", len(bus.events()))
			}
			if mode == "remote" {
				if len(sink.inserted) != 3 {
					t.Fatalf("remote device-set inserts = %d, want initial/shared/survivor", len(sink.inserted))
				}
			}
			obs.handleAddrRemoved(survivor)
			obs.handleAddrRemoved(survivor)
			if _, found := rib.Best(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24")); found {
				t.Fatal("last address removal retained the connected cover")
			}
			events := bus.events()
			if len(events) != 2 {
				t.Fatalf("redistribution count = %d, want first add and final remove", len(events))
			}
			batch, ok := events[1].payload.(*redistevents.RouteChangeBatch)
			if !ok {
				t.Fatalf("withdrawal payload = %T, want route-change batch", events[1].payload)
			}
			if batch.Entries[0].Action != redistevents.ActionRemove {
				t.Fatalf("final redistribution action = %v, want remove", batch.Entries[0].Action)
			}
			if mode == "remote" {
				if len(sink.removed) != 1 {
					t.Fatalf("remote prefix removals = %d, want one", len(sink.removed))
				}
				if sink.flushes != 4 {
					t.Fatalf("remote flushes = %d, want each of three inserts and one remove", sink.flushes)
				}
			}
		})
	}
}

func TestConnectedEgressRetirementFencesDeviceChanges(t *testing.T) {
	obs, rib, sink, _ := egressObserver(t, true)
	obs.handleAddrAdded(egressAddress(t, "eth7", 0, "192.0.2.1"))
	if path := egressPath(t, rib); path.Interface != "eth7" {
		t.Fatalf("live connected device = %q, want eth7", path.Interface)
	}
	if err := obs.retire(); err != nil {
		t.Fatal(err)
	}
	obs.handleAddrAdded(egressAddress(t, "eth8", 234, "192.0.2.1"))
	obs.handleAddrRemoved(egressAddress(t, "eth7", 0, "192.0.2.1"))
	if len(sink.inserted) != 1 {
		t.Fatalf("retired observer published %d inserts, want only its live insert", len(sink.inserted))
	}
	if len(sink.removed) != 0 {
		t.Fatalf("forked observer took engine disconnect cleanup ownership: %v", sink.removed)
	}
}

func TestConnectedEgressIPv6LogicalUnit(t *testing.T) {
	for _, mode := range []string{"local", "remote"} {
		t.Run(mode, func(t *testing.T) {
			obs, rib, _, _ := egressObserver(t, mode == "remote")
			payload, err := json.Marshal(addrPayload{
				Name: "eth7", Unit: 234, Address: "2001:db8:7::1", PrefixLength: 64, Family: "ipv6",
			})
			if err != nil {
				t.Fatal(err)
			}
			obs.handleAddrAdded(payload)
			path, matched, found := rib.LPM(family.IPv6Unicast, netip.MustParseAddr("2001:db8:7::42"))
			if !found {
				t.Fatal("IPv6 address event published no selectable covering route")
			}
			if matched != netip.MustParsePrefix("2001:db8:7::/64") {
				t.Fatalf("IPv6 cover = %s, want 2001:db8:7::/64", matched)
			}
			if path.Source != connectedevents.ProtocolID {
				t.Fatalf("IPv6 cover source = %d, want connected", path.Source)
			}
			if path.NextHop.IsValid() {
				t.Fatalf("IPv6 cover has gateway %s, want direct termination", path.NextHop)
			}
			if path.Interface != "eth7.234" {
				t.Fatalf("IPv6 cover device = %q, want eth7.234", path.Interface)
			}
		})
	}
}

func TestConnectedEgressLocalWithdrawalPreservesOtherSource(t *testing.T) {
	obs, rib, _, bus := egressObserver(t, false)
	obs.handleAddrAdded(egressAddress(t, "eth7", 234, "192.0.2.1"))
	if path := egressPath(t, rib); path.Interface != "eth7.234" {
		t.Fatalf("live connected device = %q, want eth7.234", path.Interface)
	}
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	survivor := redistevents.RegisterProtocol("connected-egress-retirement-survivor")
	rib.InsertForward(family.IPv4Unicast, prefix, locrib.Path{
		Source: survivor, Instance: 47, Interface: "eth9",
	}, nil)
	if err := obs.retire(); err != nil {
		t.Fatal(err)
	}
	if path, found := rib.Best(family.IPv4Unicast, prefix); !found || path.Source != connectedevents.ProtocolID {
		t.Fatalf("retire released the local path before the caller's post-join cleanup: path=%+v found=%v", path, found)
	}
	// No SDK worker exists in this controlled test. The synchronous calls above
	// have all returned, providing the no-in-flight-delivery precondition that
	// the production caller obtains by waiting for SDK Run to join.
	obs.withdrawLocalPaths()
	obs.handleAddrAdded(egressAddress(t, "eth8", 0, "192.0.2.1"))
	path, found := rib.Best(family.IPv4Unicast, prefix)
	if !found {
		t.Fatal("connected cleanup removed the other source's permitted survivor")
	}
	if path.Source != survivor {
		t.Fatalf("post-cleanup source = %d, want survivor %d", path.Source, survivor)
	}
	if path.Instance != 47 {
		t.Fatalf("survivor instance = %d, want 47", path.Instance)
	}
	if path.Interface != "eth9" {
		t.Fatalf("survivor device = %q, want eth9", path.Interface)
	}
	group, held := rib.Lookup(family.IPv4Unicast, prefix)
	if !held {
		t.Fatal("surviving source is missing from the authority")
	}
	if len(group.Paths) != 1 {
		t.Fatalf("post-cleanup candidates = %d, want the other source alone", len(group.Paths))
	}
	if len(bus.events()) != 2 {
		t.Fatalf("redistribution count = %d, want one live add and one retirement remove", len(bus.events()))
	}
}
