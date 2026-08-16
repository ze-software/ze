//go:build integration && linux

// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- Linux raw IPv6 OSPFv3 multicast
// integration. Validates A-1 (ipv6.PacketConn raw proto 89), A-2 (multicast
// receive), A-3 (control-message dst/ifindex/hoplimit), A-4 (hop limit 1 / loop
// off), A-6 (checksum source binding: peer VerifyPacketChecksum passes), A-5
// (ff02::6 join/leave), A-7 (pending link-local), A-8 (CAP_NET_RAW).

package transport

import (
	"bytes"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register netlink backend so iface.Resolve works
	"github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

// ospfv3SampleHello is a minimal 24-byte OSPFv3 Hello (version 3, type 1,
// Length 24, checksum field zero) for the round-trip; the transport finalizes the
// checksum from the egress source before send.
func ospfv3SampleHello() []byte {
	p := make([]byte, 24)
	p[0] = 3 // Version
	p[1] = 1 // Type Hello
	p[2], p[3] = 0, 24
	p[4], p[5], p[6], p[7] = 192, 0, 2, 1 // Router ID
	return p
}

func TestOSPFv3TransportRawSocketCap(t *testing.T) {
	if !rawSocketAvailable() {
		t.Skip("CAP_NET_RAW unavailable")
	}
}

func TestOSPFv3TransportVethMulticastRoundTrip(t *testing.T) {
	if !rawSocketAvailable() {
		t.Skip("CAP_NET_RAW unavailable")
	}
	loadIfaceBackend(t)
	withVethPeerNamespace(t, func(lab ospfv3VethLab) {
		backend := NewBackend()
		ha := openWithRetry(t, backend, lab.nameA, "")
		defer ha.Close() //nolint:errcheck // best-effort cleanup
		if err := ha.JoinAllSPFRouters(); err != nil {
			t.Fatalf("JoinAllSPFRouters A: %v", err)
		}
		expectHopLimitAndLoop(t, ha)

		var hb InterfaceHandle
		runInNS(t, lab.peerNS, func() {
			hb = openWithRetry(t, backend, lab.nameB, lab.peerNS.String())
			if err := hb.JoinAllSPFRouters(); err != nil {
				t.Fatalf("JoinAllSPFRouters B: %v", err)
			}
		})
		defer hb.Close() //nolint:errcheck // best-effort cleanup

		src := ha.LinkLocalSource()
		payload := ospfv3SampleHello()
		packet.FinalizePacketChecksum(src, AllSPFRouters, payload)
		if err := ha.Send(AllSPFRouters, src, payload); err != nil {
			t.Fatalf("Send multicast: %v\n  source %v on %s; %s", err, src, lab.nameA, describeLinkLocals(lab.nameA))
		}
		expectOSPFv3Packet(t, hb, src, AllSPFRouters, payload)

		// The sender must not receive its own multicast (loopback off, A-4).
		select {
		case got := <-ha.Recv():
			t.Fatalf("sender received looped multicast: %+v", got)
		case <-time.After(200 * time.Millisecond):
		}
	})
}

func TestOSPFv3TransportAllDRoutersReceive(t *testing.T) {
	if !rawSocketAvailable() {
		t.Skip("CAP_NET_RAW unavailable")
	}
	loadIfaceBackend(t)
	withVethPeerNamespace(t, func(lab ospfv3VethLab) {
		backend := NewBackend()
		ha := openWithRetry(t, backend, lab.nameA, "")
		defer ha.Close() //nolint:errcheck // best-effort cleanup
		if err := ha.JoinAllSPFRouters(); err != nil {
			t.Fatalf("JoinAllSPFRouters A: %v", err)
		}

		var hb InterfaceHandle
		runInNS(t, lab.peerNS, func() {
			hb = openWithRetry(t, backend, lab.nameB, lab.peerNS.String())
		})
		defer hb.Close() //nolint:errcheck // best-effort cleanup

		if err := hb.JoinAllDRouters(); err != nil {
			t.Fatalf("JoinAllDRouters: %v", err)
		}
		src := ha.LinkLocalSource()
		payload := ospfv3SampleHello()
		packet.FinalizePacketChecksum(src, AllDRouters, payload)
		if err := ha.Send(AllDRouters, src, payload); err != nil {
			t.Fatalf("Send AllDRouters: %v\n  source %v on %s; %s", err, src, lab.nameA, describeLinkLocals(lab.nameA))
		}
		expectOSPFv3Packet(t, hb, src, AllDRouters, payload)

		if err := hb.LeaveAllDRouters(); err != nil {
			t.Fatalf("LeaveAllDRouters: %v", err)
		}
		if err := ha.Send(AllDRouters, src, payload); err != nil {
			t.Fatalf("Send after AllDRouters leave: %v", err)
		}
		select {
		case got := <-hb.Recv():
			t.Fatalf("received AllDRouters after leave: %+v", got)
		case <-time.After(300 * time.Millisecond):
		}
	})
}

// openWithRetry opens the interface, retrying while the link-local source is still
// tentative (IPv6 DAD, ErrNoLinkLocal) -- validates A-7.
//
// It now waits for DAD to actually COMPLETE, which is what the sentence above
// always claimed. It only ever retried on ErrNoLinkLocal, and that is returned
// solely when the interface has NO link-local at all; interfaceLinkLocal returns
// a tentative address happily. So on a veth created moments earlier the open
// succeeded with an address still in DAD, which the kernel refuses as a packet
// source, and the first Send failed `sendmsg: invalid argument`. Real OSPF waits
// out its hello timers before sending; only this test is fast enough to race DAD.
func openWithRetry(t *testing.T, backend Backend, name, _ string) InterfaceHandle {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		h, err := backend.OpenInterface(name, nil)
		if err == nil {
			if _, tentative, llErr := interfaceLinkLocal(name); llErr == nil && !tentative {
				return h
			}
			// Close and retry rather than hold a handle bound to an unusable
			// source; the next open latches the DAD-complete address.
			h.Close() //nolint:errcheck // discarded handle, retrying
			if time.Now().After(deadline) {
				t.Fatalf("OpenInterface(%s): link-local still tentative after DAD deadline; %s", name, describeLinkLocals(name))
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if !errors.Is(err, ErrNoLinkLocal) || time.Now().After(deadline) {
			t.Fatalf("OpenInterface(%s): %v", name, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func loadIfaceBackend(t *testing.T) {
	t.Helper()
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatalf("load iface backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })
}

func expectHopLimitAndLoop(t *testing.T, h InterfaceHandle) {
	t.Helper()
	li, ok := h.(*linuxInterface)
	if !ok {
		t.Fatalf("handle type = %T, want *linuxInterface", h)
	}
	if hop, err := li.pc.MulticastHopLimit(); err != nil || hop != 1 {
		t.Fatalf("multicast hop limit = %d (err %v), want 1", hop, err)
	}
	if loop, err := li.pc.MulticastLoopback(); err != nil || loop {
		t.Fatalf("multicast loopback = %v (err %v), want false", loop, err)
	}
}

type ospfv3VethLab struct {
	nameA  string
	nameB  string
	peerNS netns.NsHandle
}

func withVethPeerNamespace(t *testing.T, fn func(ospfv3VethLab)) {
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
	nsName := ospfv3NSName(t.Name())
	peerNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close() //nolint:errcheck // best-effort cleanup
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}
	if err := netns.Set(origNS); err != nil {
		peerNS.Close() //nolint:errcheck // best-effort cleanup
		origNS.Close() //nolint:errcheck // best-effort cleanup
		unlock()
		t.Fatalf("restore original namespace after create %s: %v", nsName, err)
	}
	nameA, nameB := ospfv3LinkNames()
	t.Cleanup(func() {
		if rerr := netns.Set(origNS); rerr != nil {
			t.Errorf("restore namespace: %v", rerr)
		}
		if link, lerr := netlink.LinkByName(nameA); lerr == nil {
			_ = netlink.LinkDel(link)
		}
		origNS.Close()            //nolint:errcheck // best-effort cleanup
		peerNS.Close()            //nolint:errcheck // best-effort cleanup
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
	// OSPFv3 needs only the auto-assigned IPv6 link-local; just bring each end up.
	bringUp(t, nameA)
	runInNS(t, peerNS, func() { bringUp(t, nameB) })
	fn(ospfv3VethLab{nameA: nameA, nameB: nameB, peerNS: peerNS})
}

// describeLinkLocals reports every address the resolver sees on name, with its
// tentative flag.
//
// A send that fails EINVAL for a link-local multicast is equally consistent with
// "no source at all" and "the source is still tentative", and the kernel's error
// distinguishes neither. interfaceLinkLocal (backend_linux.go:80-114) prefers a
// DAD-complete address but deliberately FALLS BACK to a tentative one, and a
// freshly created veth is in DAD for about a second -- so which of the two
// happened is the whole diagnosis, and the bare error hides it.
func describeLinkLocals(name string) string {
	addrs, err := iface.Addresses(name)
	if err != nil {
		return fmt.Sprintf("iface.Addresses(%s) failed: %v", name, err)
	}
	var b strings.Builder
	b.WriteString("addresses: ")
	for i, a := range addrs {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s(link-local=%v tentative=%v)", a.Address, a.LinkLocal, a.Tentative)
	}
	if len(addrs) == 0 {
		b.WriteString("(none)")
	}
	return b.String()
}

func runInNS(t *testing.T, target netns.NsHandle, fn func()) {
	t.Helper()
	orig, err := netns.Get()
	if err != nil {
		t.Fatalf("get namespace: %v", err)
	}
	defer orig.Close() //nolint:errcheck // best-effort cleanup
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

func ospfv3NSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zov3_" + name
}

func osPidSuffix() int { return os.Getpid() % 10000 }

// ospfv3LabSeq makes each lab's veth pair unique WITHIN the process. The
// namespace name already varies per test (ospfv3NSName uses t.Name()); the link
// names did not, so every lab in one binary created and deleted "zev3a<pid>".
//
// Reusing the name is not merely untidy, it produces a stale resolve: the iface
// resolver caches name -> Binding (internal/component/iface/resolve.go:82-88)
// and only evicts on a monitor link event (:294). No monitor runs under `go test`,
// so the second lab resolved the FIRST lab's ifindex for a name that now belongs
// to a different device. The symptom is diagnostic: SO_BINDTODEVICE by NAME
// succeeded while SetMulticastInterface by INDEX returned ENODEV --
// "OpenInterface(zev3a460): set multicast interface zev3a460: setsockopt: no
// such device" -- name current, index stale.
//
// The pid keeps names distinct across concurrent test binaries; the counter
// keeps them distinct within one. Both stay inside IFNAMSIZ-1 (15).
var ospfv3LabSeq atomic.Uint32

func ospfv3LinkNames() (nameA, nameB string) {
	n := ospfv3LabSeq.Add(1)
	return fmt.Sprintf("zev3a%d_%d", osPidSuffix(), n), fmt.Sprintf("zev3b%d_%d", osPidSuffix(), n)
}

func expectOSPFv3Packet(t *testing.T, h InterfaceHandle, src, dst netip.Addr, payload []byte) {
	t.Helper()
	select {
	case got := <-h.Recv():
		if got.IfIndex != h.IfIndex() {
			t.Fatalf("received ifindex %d, want %d", got.IfIndex, h.IfIndex())
		}
		if got.Src != src {
			t.Fatalf("received src %v, want %v (checksum source binding, A-6)", got.Src, src)
		}
		if got.Dst != dst {
			t.Fatalf("received dst %v, want %v (control-message dst, A-3)", got.Dst, dst)
		}
		if got.HopLimit != 1 {
			t.Fatalf("received hop limit %d, want 1 (A-4)", got.HopLimit)
		}
		if !bytes.Equal(got.Payload, payload) {
			t.Fatalf("payload mismatch: got % x want % x", got.Payload, payload)
		}
		// A-6: the peer verifies the checksum against the on-wire source/dest.
		if !packet.VerifyPacketChecksum(got.Src, got.Dst, got.Payload) {
			t.Fatalf("peer VerifyPacketChecksum failed (source/checksum mismatch): src %v dst %v", got.Src, got.Dst)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for multicast receive")
	}
}

func bringUp(t *testing.T, name string) {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("LinkSetUp(%s): %v", name, err)
	}
}
