//go:build integration && linux

// Design: plan/learned/957-ospf-3-ip-transport.md -- Linux raw OSPF multicast integration

package transport

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register netlink backend so iface.Resolve works
)

func TestOSPFTransportRawSocketCap(t *testing.T) {
	if !rawSocketAvailable() {
		t.Skip("CAP_NET_RAW unavailable")
	}
}

func TestOSPFTransportVethMulticastRoundTrip(t *testing.T) {
	if !rawSocketAvailable() {
		t.Skip("CAP_NET_RAW unavailable")
	}
	loadIfaceBackend(t)
	withVethPeerNamespace(t, func(lab ospfVethLab) {
		backend := NewBackend()
		ha, err := backend.OpenInterface(lab.nameA, nil)
		if err != nil {
			t.Fatalf("OpenInterface A: %v", err)
		}
		defer ha.Close()
		expectTransmitTTL(t, ha)

		var hb InterfaceHandle
		runInNS(t, lab.peerNS, func() {
			hb, err = backend.OpenInterface(lab.nameB, nil)
		})
		if err != nil {
			t.Fatalf("OpenInterface B: %v", err)
		}
		defer hb.Close()

		payload := []byte{2, 1, 0, 24, 192, 0, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		if err := ha.Send(AllSPFRouters, payload); err != nil {
			t.Fatalf("Send multicast: %v", err)
		}
		expectOSPFPacket(t, hb, netip.MustParseAddr("192.0.2.1"), payload)
		select {
		case got := <-ha.Recv():
			t.Fatalf("sender received looped multicast: %+v", got)
		case <-time.After(200 * time.Millisecond):
		}
	})
}

func TestOSPFTransportAllDRoutersReceive(t *testing.T) {
	if !rawSocketAvailable() {
		t.Skip("CAP_NET_RAW unavailable")
	}
	loadIfaceBackend(t)
	withVethPeerNamespace(t, func(lab ospfVethLab) {
		backend := NewBackend()
		ha, err := backend.OpenInterface(lab.nameA, nil)
		if err != nil {
			t.Fatalf("OpenInterface A: %v", err)
		}
		defer ha.Close()

		var hb InterfaceHandle
		runInNS(t, lab.peerNS, func() {
			hb, err = backend.OpenInterface(lab.nameB, nil)
		})
		if err != nil {
			t.Fatalf("OpenInterface B: %v", err)
		}
		defer hb.Close()

		if err := hb.JoinAllDRouters(); err != nil {
			t.Fatalf("JoinAllDRouters: %v", err)
		}
		payload := []byte{2, 1, 0, 24, 192, 0, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		if err := ha.Send(AllDRouters, payload); err != nil {
			t.Fatalf("Send AllDRouters: %v", err)
		}
		expectOSPFPacket(t, hb, netip.MustParseAddr("192.0.2.1"), payload)

		if err := hb.LeaveAllDRouters(); err != nil {
			t.Fatalf("LeaveAllDRouters: %v", err)
		}
		if err := ha.Send(AllDRouters, payload); err != nil {
			t.Fatalf("Send after AllDRouters leave: %v", err)
		}
		select {
		case got := <-hb.Recv():
			t.Fatalf("received AllDRouters after leave: %+v", got)
		case <-time.After(300 * time.Millisecond):
		}
	})
}

func loadIfaceBackend(t *testing.T) {
	t.Helper()
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatalf("load iface backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })
}

func expectTransmitTTL(t *testing.T, h InterfaceHandle) {
	t.Helper()
	li, ok := h.(*linuxInterface)
	if !ok {
		t.Fatalf("handle type = %T, want *linuxInterface", h)
	}
	ttl, err := unix.GetsockoptInt(li.txFD, unix.IPPROTO_IP, unix.IP_TTL)
	if err != nil {
		t.Fatalf("getsockopt IP_TTL: %v", err)
	}
	if ttl != 1 {
		t.Fatalf("IP_TTL = %d, want 1", ttl)
	}
	mttl, err := unix.GetsockoptInt(li.txFD, unix.IPPROTO_IP, unix.IP_MULTICAST_TTL)
	if err != nil {
		t.Fatalf("getsockopt IP_MULTICAST_TTL: %v", err)
	}
	if mttl != 1 {
		t.Fatalf("IP_MULTICAST_TTL = %d, want 1", mttl)
	}
}

type ospfVethLab struct {
	nameA  string
	nameB  string
	peerNS netns.NsHandle
}

func withVethPeerNamespace(t *testing.T, fn func(ospfVethLab)) {
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
	nsName := ospfNSName(t.Name())
	peerNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}
	if err := netns.Set(origNS); err != nil {
		peerNS.Close()
		origNS.Close()
		unlock()
		t.Fatalf("restore original namespace after create %s: %v", nsName, err)
	}
	nameA := fmt.Sprintf("zeospa%d", osPidSuffix())
	nameB := fmt.Sprintf("zeospb%d", osPidSuffix())
	t.Cleanup(func() {
		if rerr := netns.Set(origNS); rerr != nil {
			t.Errorf("restore namespace: %v", rerr)
		}
		if link, lerr := netlink.LinkByName(nameA); lerr == nil {
			_ = netlink.LinkDel(link)
		}
		origNS.Close()
		peerNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		unlock()
	})
	veth := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: nameA}, PeerName: nameB}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Skipf("add veth (needs CAP_NET_ADMIN): %v", err)
	}
	linkB, err := netlink.LinkByName(nameB)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", nameB, err)
	}
	if err := netlink.LinkSetNsFd(linkB, int(peerNS)); err != nil {
		t.Fatalf("LinkSetNsFd(%s): %v", nameB, err)
	}
	setupVethAddr(t, nameA, "192.0.2.1/24")
	runInNS(t, peerNS, func() {
		setupVethAddr(t, nameB, "192.0.2.2/24")
	})
	fn(ospfVethLab{nameA: nameA, nameB: nameB, peerNS: peerNS})
}

func runInNS(t *testing.T, target netns.NsHandle, fn func()) {
	t.Helper()
	orig, err := netns.Get()
	if err != nil {
		t.Fatalf("get namespace: %v", err)
	}
	defer orig.Close()
	if err := netns.Set(target); err != nil {
		t.Fatalf("set namespace: %v", err)
	}
	defer func() {
		if err := netns.Set(orig); err != nil {
			t.Fatalf("restore namespace: %v", err)
		}
	}()
	fn()
}

func ospfNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zospf_" + name
}

func osPidSuffix() int { return os.Getpid() % 10000 }

func expectOSPFPacket(t *testing.T, h InterfaceHandle, src netip.Addr, payload []byte) {
	t.Helper()
	select {
	case got := <-h.Recv():
		if got.IfIndex != h.IfIndex() || got.Src != src || !bytes.Equal(got.Payload, payload) {
			t.Fatalf("received = ifindex %d src %v payload % x", got.IfIndex, got.Src, got.Payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for multicast receive")
	}
}

func setupVethAddr(t *testing.T, name, cidr string) {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", name, err)
	}
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		t.Fatalf("ParseAddr(%s): %v", cidr, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("AddrAdd(%s): %v", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("LinkSetUp(%s): %v", name, err)
	}
}
