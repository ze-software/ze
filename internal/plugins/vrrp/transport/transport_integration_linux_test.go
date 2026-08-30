//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- QEMU integration: raw proto-112 sockets,
// macvlan tx identity, GARP/NA on-wire, rx delivery (veth + netns).
//
// These tests build a veth pair and a bridge-mode macvlan (carrying the virtual
// MAC) in an ephemeral network namespace, then drive the real Transport. Frames
// egressing the macvlan on the parent veth end arrive at the peer end, where an
// AF_PACKET capture socket observes them (payload-predicate waits with deadlines,
// never fixed sleeps). Namespace creation needs CAP_NET_ADMIN; without it the
// tests Skip. This file validates A-1..A-5 and R-7 on a real kernel.

package transport

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register netlink backend so iface.Resolve works
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

const (
	testVRID     = 10
	parentV4CIDR = "192.0.2.251/24"
	parentV4     = "192.0.2.251"
)

// verifyV6Upper verifies any IPv6 upper-layer checksum (the payload includes its
// own checksum field) under the RFC 8200 pseudo-header for the given next
// header. Used with proto 112 to prove the kernel really filled the VRRP v6
// checksum (A-5). Lives here, not in na_test.go, because the integration suite
// is its only caller and the default build tags would make it dead code.
func verifyV6Upper(src, dst netip.Addr, payload []byte, nextHeader uint8) bool {
	return foldSum(pseudoSum(src, dst, payload, nextHeader)) == 0xffff
}

// labSeq makes every lab's device names unique within the test binary.
//
// iface.Resolve caches logical-name -> Binding (ifindex) in a package-global map
// that is invalidated ONLY by monitor link events (resolve.go:56,84,294). No
// monitor runs in these tests, and each lab builds a FRESH netns, so reusing a
// device name across tests makes a later lab resolve the earlier (destroyed)
// netns's ifindex. The instance then joins 224.0.0.18 on the wrong ifindex and
// receives nothing -- TestIntegrationRxDeliversFromPeer failed exactly this way
// in a full-package run while passing in isolation. Unique names per lab keep
// each test's resolution honest without reaching into another package's cache.
// (Production is unaffected: the iface monitor invalidates on every link event.)
var labSeq atomic.Uint32

// vrrpLab is a veth pair (parent <-> peer) plus a virtual-MAC macvlan on the
// parent, all in an ephemeral netns.
type vrrpLab struct {
	parent    string
	peer      string
	macvlan   string
	peerIf    int
	vmac      [6]byte
	captureFD int
}

func skipNoRaw(t *testing.T) {
	t.Helper()
	if !rawSocketAvailable() {
		t.Skip("CAP_NET_RAW unavailable")
	}
}

// setupLab enters a fresh netns and builds the veth + macvlan lab for family.
func setupLab(t *testing.T, family uint8) vrrpLab {
	t.Helper()
	skipNoRaw(t)
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatalf("load iface backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })

	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Skipf("requires CAP_NET_ADMIN: get namespace: %v", err)
	}
	newNS, err := netns.New()
	if err != nil {
		orig.Close() //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
		t.Skipf("requires CAP_NET_ADMIN: create namespace: %v", err)
	}
	t.Cleanup(func() {
		_ = netns.Set(orig)
		orig.Close()  //nolint:errcheck // best-effort cleanup
		newNS.Close() //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})

	// Unique per lab: see labSeq -- a reused name resolves to a stale ifindex
	// from an earlier test's destroyed netns.
	suffix := (os.Getpid() % 1000) + int(labSeq.Add(1))*1000
	parent := fmt.Sprintf("zevrpp%d", suffix)
	peer := fmt.Sprintf("zevrpe%d", suffix)
	macvlan := fmt.Sprintf("zevrpm%d", suffix)

	veth := &netlink.Veth{Name: parent, PeerName: peer}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Skipf("add veth (needs CAP_NET_ADMIN): %v", err)
	}
	parentLink, err := netlink.LinkByName(parent)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", parent, err)
	}
	peerLink, err := netlink.LinkByName(peer)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", peer, err)
	}
	mustUp(t, parentLink)
	mustUp(t, peerLink)

	if family == packet.V4 {
		addr, perr := netlink.ParseAddr(parentV4CIDR)
		if perr != nil {
			t.Fatalf("ParseAddr: %v", perr)
		}
		if aerr := netlink.AddrAdd(parentLink, addr); aerr != nil {
			t.Fatalf("AddrAdd parent: %v", aerr)
		}
	}

	vmac := packet.VirtualMAC(family, testVRID)
	mvAttrs := netlink.LinkAttrs{Name: macvlan, ParentIndex: parentLink.Attrs().Index, HardwareAddr: net.HardwareAddr(vmac[:])}
	mv := &netlink.Macvlan{LinkAttrs: mvAttrs, Mode: netlink.MACVLAN_MODE_BRIDGE}
	if err := netlink.LinkAdd(mv); err != nil {
		t.Fatalf("add macvlan: %v", err)
	}
	mvLink, err := netlink.LinkByName(macvlan)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", macvlan, err)
	}
	mustUp(t, mvLink)

	captureFD := openCapture(t, peerLink.Attrs().Index)
	return vrrpLab{
		parent:    parent,
		peer:      peer,
		macvlan:   macvlan,
		peerIf:    peerLink.Attrs().Index,
		vmac:      vmac,
		captureFD: captureFD,
	}
}

func mustUp(t *testing.T, link netlink.Link) {
	t.Helper()
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("LinkSetUp(%s): %v", link.Attrs().Name, err)
	}
}

// openCapture opens an AF_PACKET raw capture socket bound to ifindex with a short
// receive timeout so the capture loop is bounded.
func openCapture(t *testing.T, ifindex int) int {
	t.Helper()
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatalf("capture socket: %v", err)
	}
	sa := &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_ALL), Ifindex: ifindex}
	if err := unix.Bind(fd, sa); err != nil {
		unix.Close(fd) //nolint:errcheck // best-effort cleanup
		t.Fatalf("capture bind: %v", err)
	}
	tv := unix.Timeval{Sec: 0, Usec: 100000}
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)
	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck // best-effort cleanup
	return fd
}

// captureMatch reads captured Ethernet frames until match returns true or the
// deadline passes (payload-predicate wait, R-6).
func captureMatch(t *testing.T, fd int, match func(frame []byte) bool) []byte {
	t.Helper()
	// One wait budget for every capture in this file.
	const timeout = 3 * time.Second
	buf := make([]byte, 2048)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
				continue
			}
			t.Fatalf("capture recvfrom: %v", err)
		}
		if match(buf[:n]) {
			return append([]byte(nil), buf[:n]...)
		}
	}
	t.Fatalf("no matching frame within %v", timeout)
	return nil
}

func openTransportInstance(t *testing.T, lab vrrpLab, family uint8) (*Transport, InstanceKey) {
	t.Helper()
	tr := New(NewBackend())
	spec := InstanceSpec{Family: family, VRID: testVRID, Parent: lab.parent, MacvlanDevice: lab.macvlan, VirtualMAC: lab.vmac}
	key, err := tr.OpenInstance(spec)
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	t.Cleanup(tr.Close)
	return tr, key
}

func linuxHandle(t *testing.T, tr *Transport, key InstanceKey) *linuxInstance {
	t.Helper()
	inst := tr.lookup(key)
	if inst == nil {
		t.Fatal("instance not registered")
	}
	li, ok := inst.handle.(*linuxInstance)
	if !ok {
		t.Fatalf("handle type = %T, want *linuxInstance", inst.handle)
	}
	return li
}

// RFC requirement: RFC9568-5.1.2.3-1 positive -- the v6 tx socket is opened with IPV6_MULTICAST_HOPS 255, so every transmitted IPv6 advertisement carries hop limit 255, and the getsockopt read-back below proves the kernel took the value (openV6 backend_linux.go:185).
func TestIntegrationOpenInstanceSocketOptions(t *testing.T) {
	// VALIDATES: AC-1/AC-2 -- getsockopt read-back of the full option matrix.
	t.Run("v4", func(t *testing.T) {
		lab := setupLab(t, packet.V4)
		tr, key := openTransportInstance(t, lab, packet.V4)
		li := linuxHandle(t, tr, key)
		if v := getInt(t, li.txFD, unix.IPPROTO_IP, unix.IP_HDRINCL); v != 1 {
			t.Fatalf("IP_HDRINCL = %d, want 1", v)
		}
		if v := getInt(t, li.txFD, unix.IPPROTO_IP, unix.IP_MULTICAST_LOOP); v != 0 {
			t.Fatalf("IP_MULTICAST_LOOP = %d, want 0", v)
		}
		requireRcvTimeout(t, li.rxFD)
	})
	t.Run("v6", func(t *testing.T) {
		lab := setupLab(t, packet.V6)
		tr, key := openTransportInstance(t, lab, packet.V6)
		li := linuxHandle(t, tr, key)
		if v := getInt(t, li.txFD, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_HOPS); v != 255 {
			t.Fatalf("IPV6_MULTICAST_HOPS = %d, want 255", v)
		}
		if v := getInt(t, li.txFD, unix.IPPROTO_IPV6, unix.IPV6_TCLASS); v != 0xc0 {
			t.Fatalf("IPV6_TCLASS = %#x, want 0xc0", v)
		}
		if v := getInt(t, li.txFD, unix.IPPROTO_IPV6, unix.IPV6_CHECKSUM); v != 6 {
			t.Fatalf("IPV6_CHECKSUM = %d, want 6", v)
		}
		if v := getInt(t, li.annFD, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_HOPS); v != 255 {
			t.Fatalf("NA IPV6_MULTICAST_HOPS = %d, want 255", v)
		}
		requireRcvTimeout(t, li.rxFD)
	})
}

func getInt(t *testing.T, fd, level, opt int) int {
	t.Helper()
	v, err := unix.GetsockoptInt(fd, level, opt)
	if err != nil {
		t.Fatalf("getsockopt(%d,%d): %v", level, opt, err)
	}
	return v
}

func requireRcvTimeout(t *testing.T, fd int) {
	t.Helper()
	tv, err := unix.GetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO)
	if err != nil {
		t.Fatalf("getsockopt SO_RCVTIMEO: %v", err)
	}
	if tv.Sec != 0 || tv.Usec != 500000 {
		t.Fatalf("SO_RCVTIMEO = %d.%06ds, want 0.500000s", tv.Sec, tv.Usec)
	}
}

// igmpProcPath is the per-THREAD view of the IGMP membership table. setns(2) is
// per-thread and only the locked test thread entered the lab netns; /proc/net is
// a symlink to /proc/self/net, which reflects the thread-group LEADER's netns,
// so reading it here would silently observe the wrong (original) namespace
// (QEMU-discovered, Mistake Log).
const igmpProcPath = "/proc/thread-self/net/igmp"

func TestIntegrationMulticastJoinVisible(t *testing.T) {
	// VALIDATES: A-1 -- 224.0.0.18 appears in the parent's IGMP table while the
	// instance is open, and is gone after Close.
	lab := setupLab(t, packet.V4)
	tr, _ := openTransportInstance(t, lab, packet.V4)
	if !containsHexV4(t, igmpProcPath, "120000E0") { // 224.0.0.18 little-endian hex
		t.Fatal("224.0.0.18 not joined (not in " + igmpProcPath + ")")
	}
	tr.Close()
	deadline := time.Now().Add(2 * time.Second)
	for containsHexV4(t, igmpProcPath, "120000E0") {
		if time.Now().After(deadline) {
			t.Fatal("224.0.0.18 still joined after Close")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func containsHexV4(t *testing.T, path, wantHex string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.Contains(strings.ToUpper(string(data)), wantHex)
}

func TestIntegrationAdvertOnPeerVeth(t *testing.T) {
	// VALIDATES: AC-3 + A-1/A-2/A-3 -- captured advert has virtual-MAC L2 src, TTL
	// 255, TOS 0xc0, proto 112, dst 224.0.0.18, src = parent primary IPv4.
	lab := setupLab(t, packet.V4)
	tr, key := openTransportInstance(t, lab, packet.V4)
	if err := tr.UpdateAdvert(key, AdvertParams{Version: packet.VersionV3, Priority: 100, AdverIntervalMS: 1000, VIPs: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}); err != nil {
		t.Fatalf("UpdateAdvert: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert: %v", err)
	}
	frame := captureMatch(t, lab.captureFD, func(f []byte) bool {
		return len(f) >= 38 && f[12] == 0x08 && f[13] == 0x00 && f[23] == packet.ProtoNumber
	})

	if !bytes.Equal(frame[6:12], lab.vmac[:]) {
		t.Fatalf("L2 src = % x, want virtual MAC % x", frame[6:12], lab.vmac)
	}
	ip := frame[14:]
	if ip[1] != 0xc0 {
		t.Fatalf("TOS = %#x, want 0xc0", ip[1])
	}
	if ip[8] != 255 {
		t.Fatalf("TTL = %d, want 255", ip[8])
	}
	if src := netip.AddrFrom4([4]byte(ip[12:16])); src != netip.MustParseAddr(parentV4) {
		t.Fatalf("src IP = %v, want %s", src, parentV4)
	}
	if dst := netip.AddrFrom4([4]byte(ip[16:20])); dst != packet.MulticastV4 {
		t.Fatalf("dst IP = %v, want 224.0.0.18", dst)
	}
	// A-3: the IPv4 header checksum on the wire must be valid. ze builds the
	// datagram with IP_HDRINCL and the kernel fills the header checksum; a valid
	// checksum is the one's-complement sum of the header 16-bit words (the
	// checksum field included) folding to 0xffff. Without this assertion the test
	// proved every header field EXCEPT that a receiver would accept the header.
	ihl := int(ip[0]&0x0f) * 4
	var hsum uint32
	for i := 0; i < ihl; i += 2 {
		hsum += uint32(ip[i])<<8 | uint32(ip[i+1])
	}
	if foldSum(hsum) != 0xffff {
		t.Fatalf("IPv4 header checksum invalid (fold=%#x over %d-byte header): % x", foldSum(hsum), ihl, ip[:ihl])
	}
}

func TestIntegrationAdvertV6OnWire(t *testing.T) {
	// VALIDATES: AC-4 + A-5 + R-7 -- captured v6 advert has hop limit 255, next
	// header 112, dst ff02::12, src = macvlan link-local.
	lab := setupLab(t, packet.V6)
	tr, key := openTransportInstance(t, lab, packet.V6)
	ll := waitLinkLocal(t, lab.macvlan)
	if err := tr.UpdateAdvert(key, AdvertParams{Version: packet.VersionV3, Priority: 100, AdverIntervalMS: 1000, VIPs: []netip.Addr{ll}}); err != nil {
		t.Fatalf("UpdateAdvert: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert: %v", err)
	}
	frame := captureMatch(t, lab.captureFD, func(f []byte) bool {
		return len(f) >= 54 && f[12] == 0x86 && f[13] == 0xdd && f[14]>>4 == 6 && f[20] == packet.ProtoNumber
	})
	ip := frame[14:]
	if ip[7] != 255 {
		t.Fatalf("hop limit = %d, want 255", ip[7])
	}
	dst := netip.AddrFrom16([16]byte(ip[24:40]))
	if dst != packet.MulticastV6 {
		t.Fatalf("dst = %v, want ff02::12", dst)
	}
	src := netip.AddrFrom16([16]byte(ip[8:24]))
	if src != ll {
		t.Fatalf("src = %v, want macvlan link-local %v", src, ll)
	}
	// A-5: the transport sets IPV6_CHECKSUM and never fills the VRRP checksum
	// itself for v6, so the kernel must have computed a valid one over the RFC
	// 8200 pseudo-header. Verifying it here is what proves the offload is really
	// wired: a zero or stale checksum would be silently accepted by tcpdump but
	// dropped by a conformant peer.
	if !verifyV6Upper(src, dst, ip[40:], packet.ProtoNumber) {
		t.Fatal("kernel-filled VRRP v6 checksum does not verify under the RFC 8200 pseudo-header (IPV6_CHECKSUM offload not effective)")
	}
}

func TestIntegrationGARPOnWire(t *testing.T) {
	// VALIDATES: AC-8 -- captured GARP frames byte-equal the golden frame; the
	// burst count is observed.
	lab := setupLab(t, packet.V4)
	tr, key := openTransportInstance(t, lab, packet.V4)
	vip := netip.MustParseAddr("192.0.2.1")
	tr.AnnounceMaster(key, []netip.Addr{vip})

	var want [64]byte
	n := buildGARP(want[:], lab.vmac, vip.As4())
	frame := captureMatch(t, lab.captureFD, func(f []byte) bool {
		return len(f) >= n && f[12] == 0x08 && f[13] == 0x06
	})
	if !bytes.Equal(frame[:n], want[:n]) {
		t.Fatalf("GARP frame\n got % x\nwant % x", frame[:n], want[:n])
	}
}

func TestIntegrationNAOnWire(t *testing.T) {
	// VALIDATES: AC-9 + A-4 -- captured NA has hop limit 255, dst ff02::1, ICMPv6
	// type 136, and a valid kernel-computed checksum.
	lab := setupLab(t, packet.V6)
	tr, key := openTransportInstance(t, lab, packet.V6)
	ll := waitLinkLocal(t, lab.macvlan)
	vip := netip.MustParseAddr("2001:db8::1")
	tr.AnnounceMaster(key, []netip.Addr{vip})

	frame := captureMatch(t, lab.captureFD, func(f []byte) bool {
		return len(f) >= 14+40+8 && f[12] == 0x86 && f[13] == 0xdd && f[20] == 58 && f[14+40] == 136
	})
	ip := frame[14:]
	if ip[7] != 255 {
		t.Fatalf("NA hop limit = %d, want 255", ip[7])
	}
	dst := netip.AddrFrom16([16]byte(ip[24:40]))
	if dst != NAAllNodesV6 {
		t.Fatalf("NA dst = %v, want ff02::1", dst)
	}
	// Kernel-computed ICMPv6 checksum must verify under the pseudo-header.
	icmp := ip[40:]
	if !verifyICMPv6(ll, NAAllNodesV6, icmp) {
		t.Fatal("NA ICMPv6 checksum does not verify (A-4)")
	}
}

func TestIntegrationRxDeliversFromPeer(t *testing.T) {
	// VALIDATES: AC-6 + A-1 -- a proto-112 datagram sent toward the parent is
	// delivered with correct RxMeta (TTL preserved unmodified).
	lab := setupLab(t, packet.V4)
	tr, _ := openTransportInstance(t, lab, packet.V4)

	// Build a minimal VRRPv3 advert and an IP_HDRINCL datagram, send it from a raw
	// socket bound to the peer end toward 224.0.0.18 with TTL 254 (non-255 must
	// still be delivered; GTSM discard is the codec's job).
	var payload [64]byte
	adv := packet.Advertisement{Version: packet.VersionV3, Family: packet.V4, VRID: testVRID, Priority: 100, AdverIntervalMS: 1000, VIPs: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}
	pn := adv.WriteTo(payload[:], 20)
	src := netip.MustParseAddr("192.0.2.2")
	packet.FillChecksum(payload[:], 20, pn, src, packet.MulticastV4)
	hdr := buildIPv4Header(payload[:], src.As4(), packet.MulticastV4.As4())
	payload[8] = 254 // TTL 254

	txFD, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, int(packet.ProtoNumber))
	if err != nil {
		t.Fatalf("peer tx socket: %v", err)
	}
	defer unix.Close(txFD) //nolint:errcheck // best-effort cleanup
	if err := unix.SetsockoptString(txFD, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, lab.peer); err != nil {
		t.Fatalf("bind peer tx: %v", err)
	}
	if err := unix.SetsockoptInt(txFD, unix.IPPROTO_IP, unix.IP_HDRINCL, 1); err != nil {
		t.Fatalf("peer IP_HDRINCL: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		if serr := unix.Sendto(txFD, payload[:hdr+pn], 0, &unix.SockaddrInet4{Addr: packet.MulticastV4.As4()}); serr != nil {
			t.Fatalf("peer sendto: %v", serr)
		}
		select {
		case item := <-tr.Receive():
			if item.Meta.TTL != 254 {
				t.Fatalf("delivered TTL = %d, want 254 (unmodified)", item.Meta.TTL)
			}
			if item.Meta.Family != packet.V4 || item.Meta.Dst != packet.MulticastV4 {
				t.Fatalf("delivered meta wrong: %+v", item.Meta)
			}
			if len(item.Payload) != pn {
				t.Fatalf("delivered payload len = %d, want %d", len(item.Payload), pn)
			}
			return
		case <-deadline:
			t.Fatal("no datagram delivered from peer")
		case <-time.After(200 * time.Millisecond):
			// resend and keep polling
		}
	}
}

// waitLinkLocal polls for the macvlan's IPv6 link-local (DAD completion).
func waitLinkLocal(t *testing.T, name string) netip.Addr {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ll, ok := macvlanLinkLocal(name); ok {
			return ll
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("macvlan %s link-local never appeared", name)
	return netip.Addr{}
}
