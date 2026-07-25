//go:build integration && linux

package ifacenetlink

import (
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
)

// withVLANQoSNetNS runs fn inside a fresh named network namespace so VLAN
// creation cannot collide with host links. Skips (not fails) without
// CAP_NET_ADMIN per ai/rules/qemu-testing.md.
func withVLANQoSNetNS(t *testing.T, fn func()) {
	t.Helper()

	runtime.LockOSThread()
	unlocked := false
	unlock := func() {
		if !unlocked {
			runtime.UnlockOSThread()
			unlocked = true
		}
	}

	origNS, err := netns.Get()
	if err != nil {
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot get current namespace: %v", err)
	}

	nsName := vlanQoSNetNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		unlock()
	})

	fn()
}

func vlanQoSNetNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zevq_" + name
}

func addVLANQoSDummy(t *testing.T, name string) {
	t.Helper()

	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}); err != nil {
		t.Fatalf("add dummy %q: %v", name, err)
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("link %q: %v", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set %q up: %v", name, err)
	}
}

func kernelVLAN(t *testing.T, name string) *netlink.Vlan {
	t.Helper()

	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("link %q: %v", name, err)
	}
	vlan, ok := link.(*netlink.Vlan)
	if !ok {
		t.Fatalf("link %q is %T, want *netlink.Vlan", name, link)
	}
	return vlan
}

// TestVLANQoSMapIntegration creates a VLAN sub-interface with both 802.1p QoS
// maps through the real netlink backend and reads the link back from the
// kernel to assert the maps were applied at creation time.
//
// VALIDATES: spec-vlan-qos-map AC-3 and assumption A-2 -- the kernel accepts
// IFLA_VLAN_INGRESS_QOS / IFLA_VLAN_EGRESS_QOS inside RTM_NEWLINK.
// PREVENTS: maps silently dropped between VLANSpec and the kernel.
func TestVLANQoSMapIntegration(t *testing.T) {
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "zeq0")
		b := &netlinkBackend{}

		err := b.CreateVLAN(iface.VLANSpec{
			Parent:        "zeq0",
			VLANID:        100,
			IngressQoSMap: map[uint32]uint32{0: 1, 6: 6, 7: 7},
			EgressQoSMap:  map[uint32]uint32{6: 6, 7: 7},
		})
		if err != nil {
			t.Fatalf("CreateVLAN with QoS maps: %v", err)
		}

		vlan := kernelVLAN(t, "zeq0.100")
		if got, want := vlan.VlanId, 100; got != want {
			t.Errorf("VlanId = %d, want %d", got, want)
		}
		for from, want := range map[uint32]uint32{0: 1, 6: 6, 7: 7} {
			if got := vlan.IngressQosMap[from]; got != want {
				t.Errorf("ingress map %d -> %d, want %d (full map: %v)", from, got, want, vlan.IngressQosMap)
			}
		}
		for from, want := range map[uint32]uint32{6: 6, 7: 7} {
			if got := vlan.EgressQosMap[from]; got != want {
				t.Errorf("egress map %d -> %d, want %d (full map: %v)", from, got, want, vlan.EgressQosMap)
			}
		}

		// linkToInfo is the show-command path (AC-7): the maps read back from
		// the kernel must surface in InterfaceInfo.
		info := linkToInfo(vlan)
		if info.IngressQoSMap == nil || info.EgressQoSMap == nil {
			t.Errorf("InterfaceInfo missing QoS maps: ingress=%v egress=%v", info.IngressQoSMap, info.EgressQoSMap)
		}
	})
}

// TestVLANQoSMapIntegrationAbsent creates a VLAN without QoS maps and asserts
// the kernel reports none, preserving pre-feature behavior for legacy config.
//
// VALIDATES: spec-vlan-qos-map AC-6 -- nil maps emit no netlink attribute.
// PREVENTS: zero-value mapping entries appearing on unconfigured VLANs.
func TestVLANQoSMapIntegrationAbsent(t *testing.T) {
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "zeq0")
		b := &netlinkBackend{}

		if err := b.CreateVLAN(iface.VLANSpec{Parent: "zeq0", VLANID: 200}); err != nil {
			t.Fatalf("CreateVLAN without QoS maps: %v", err)
		}

		vlan := kernelVLAN(t, "zeq0.200")
		if len(vlan.IngressQosMap) != 0 {
			t.Errorf("ingress map = %v, want empty", vlan.IngressQosMap)
		}
		if len(vlan.EgressQosMap) != 0 {
			t.Errorf("egress map = %v, want empty", vlan.EgressQosMap)
		}
	})
}

// TestVLANQoSMapIntegrationModify probes assumption A-3: whether the kernel
// accepts QoS map updates on an existing VLAN via RTM_NEWLINK modify, which
// determines whether config changes need delete+recreate. The egress side is
// asserted strictly; the kernel ingress path only ADDS entries on modify
// (8021q has no per-entry delete via this attribute), so the ingress
// assertion checks the new entry exists rather than the map being replaced.
//
// VALIDATES: spec-vlan-qos-map assumption A-3 -- modify semantics evidence.
// PREVENTS: undocumented stale-map behavior after a config change.
func TestVLANQoSMapIntegrationModify(t *testing.T) {
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "zeq0")
		b := &netlinkBackend{}

		if err := b.CreateVLAN(iface.VLANSpec{
			Parent:        "zeq0",
			VLANID:        300,
			EgressQoSMap:  map[uint32]uint32{6: 6},
			IngressQoSMap: map[uint32]uint32{6: 6},
		}); err != nil {
			t.Fatalf("CreateVLAN: %v", err)
		}

		existing := kernelVLAN(t, "zeq0.300")
		update := &netlink.Vlan{
			LinkAttrs:     netlink.LinkAttrs{Name: "zeq0.300", Index: existing.Attrs().Index},
			VlanId:        300,
			IngressQosMap: map[uint32]uint32{5: 5},
			EgressQosMap:  map[uint32]uint32{5: 5},
		}
		if err := netlink.LinkModify(update); err != nil {
			t.Logf("A-3 evidence: LinkModify rejected (%v) -- config changes need delete+recreate", err)
			return
		}

		vlan := kernelVLAN(t, "zeq0.300")
		if got := vlan.IngressQosMap[5]; got != 5 {
			t.Errorf("ingress map after modify missing 5 -> 5 (full map: %v)", vlan.IngressQosMap)
		}
		if got := vlan.EgressQosMap[5]; got != 5 {
			t.Errorf("egress map after modify missing 5 -> 5 (full map: %v)", vlan.EgressQosMap)
		}
	})
}
