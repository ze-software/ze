//go:build integration && linux

package peer

import (
	"net"
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/chaos/engine"
)

// withChaosNetNS runs fn inside a fresh named network namespace so interface
// faults never touch the host. Mirrors internal/plugins/traffic/netlink's
// withTrafficNetNS. Skips (never fails) when CAP_NET_ADMIN is unavailable --
// the QEMU/root run is where it executes.
func withChaosNetNS(t *testing.T, fn func()) {
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

	nsName := chaosNetNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("restore namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		unlock()
	})

	fn()
}

func chaosNetNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zecf_" + name
}

// addChaosVeth creates an up veth pair and assigns cidr to the primary end.
func addChaosVeth(t *testing.T, name, peer, cidr string) netlink.Link {
	t.Helper()
	if err := netlink.LinkAdd(&netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: name}, PeerName: peer}); err != nil {
		t.Fatalf("add veth %q/%q: %v", name, peer, err)
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("link %q: %v", name, err)
	}
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		t.Fatalf("parse addr %q: %v", cidr, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("addr add %q: %v", cidr, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set %q up: %v", name, err)
	}
	if peerLink, err := netlink.LinkByName(peer); err == nil {
		netlink.LinkSetUp(peerLink) //nolint:errcheck // peer up is best-effort
	}
	return link
}

func linkIsUp(t *testing.T, name string) bool {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("link %q: %v", name, err)
	}
	return link.Attrs().Flags&net.FlagUp != 0
}

func hasAddr(t *testing.T, name, cidr string) bool {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("link %q: %v", name, err)
	}
	want, err := netlink.ParseAddr(cidr)
	if err != nil {
		t.Fatalf("parse addr %q: %v", cidr, err)
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("addr list: %v", err)
	}
	for _, a := range addrs {
		if a.IPNet != nil && a.IPNet.IP.Equal(want.IPNet.IP) {
			return true
		}
	}
	return false
}

// TestChaosIfaceLinkFlap drives the link-flap action against a real veth in a
// private netns and asserts the interface is flapped (down->up) and the action
// reports the session as torn so the simulator's reconnect machinery recovers it.
//
// VALIDATES: AC-6 -- iface-link-flap performs the netlink down/up and signals
// Disconnected so BGP-session recovery is triggered.
// PREVENTS: a link-flap that leaves the interface down, or one that fails to
// signal the transport teardown that drives reconnection.
func TestChaosIfaceLinkFlap(t *testing.T) {
	withChaosNetNS(t, func() {
		const iface = "ze_cf0"
		addChaosVeth(t, iface, "ze_cf1", "10.242.0.1/24")

		var events []Event
		emit := func(e Event) { events = append(events, e) }

		res := executeIfaceLinkFlap(engine.ChaosAction{
			Type:   engine.ActionIfaceLinkFlap,
			Params: map[string]string{engine.ParamIface: iface, engine.ParamCycles: "2"},
		}, emit)

		if len(events) != 0 {
			t.Fatalf("unexpected error events: %+v", events)
		}
		if !res.Disconnected {
			t.Fatal("link-flap must report Disconnected=true to drive session recovery")
		}
		if !linkIsUp(t, iface) {
			t.Fatal("interface must be back UP after the flap (recovered)")
		}
	})
}

// TestChaosIfaceAddrRemove drives the addr-remove action against a real veth in
// a private netns and asserts the address is removed then restored.
//
// VALIDATES: AC-6 -- iface-addr-remove performs the netlink addr del/add cycle.
// PREVENTS: an addr-remove that leaves the interface without its address.
func TestChaosIfaceAddrRemove(t *testing.T) {
	withChaosNetNS(t, func() {
		const iface = "ze_cf0"
		const cidr = "10.242.0.1/24"
		addChaosVeth(t, iface, "ze_cf1", cidr)

		var events []Event
		emit := func(e Event) { events = append(events, e) }

		res := executeIfaceAddrRemove(engine.ChaosAction{
			Type:   engine.ActionIfaceAddrRemove,
			Params: map[string]string{engine.ParamIface: iface, engine.ParamAddr: cidr},
		}, emit)

		if len(events) != 0 {
			t.Fatalf("unexpected error events: %+v", events)
		}
		if res.Disconnected {
			t.Fatal("addr-remove restores the address; it must not report a permanent disconnect")
		}
		if !hasAddr(t, iface, cidr) {
			t.Fatal("address must be restored after the addr-remove cycle")
		}
	})
}
