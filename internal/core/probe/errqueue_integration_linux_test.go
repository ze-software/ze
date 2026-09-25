//go:build integration && linux

// Design: docs/architecture/diagnostics/active-probes.md -- the error queue proven against the kernel
//
// VALIDATES: A-1 (the kernel hands the reported next-hop MTU to the socket
// error queue), A-2 (IP_PMTUDISC_PROBE bypasses the cached path MTU), A-3
// (what the kernel matches a queued error to), and the DF bit on the wire,
// each against a real Linux router rather than Ze's own report. This is
// the spec's interop scenario probe-df-clamped-path: the Linux router in
// the middle namespace is the other implementation.
// PREVENTS: a design that reads ee_info the kernel never fills, a bypass
// mode that reports the cache as a measurement, a reader that believes an
// error for another flow, and a DF mode that never sets the bit.
//
// Topology, three namespaces joined by two veth pairs:
//
//	sender ----sr0/rs0---- router ----rf0/fr0---- far
//	10.99.1.1     1500    10.99.1.2  1400      10.99.2.2
//	fd99:1::1             fd99:1::2            fd99:2::2
//
// The router forwards and its far-side link is clamped to 1400, so a
// 1500-octet DF datagram from the sender is refused there with
// Fragmentation Needed (or Packet Too Big) reporting 1400. Every test
// skips, never fails, when the namespaces or the raw socket are out of
// reach: the QEMU runner (./le test qemu all-tests) is where they run for real.

package probe

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// The addresses and sizes of the clamped path.
const (
	pathLinkMTU  = 1500
	pathClampMTU = 1400
	// pathFillPayload makes an IPv4 echo datagram exactly pathLinkMTU long
	// (20 IP + 8 ICMP + payload), so it fits the sender's link and exceeds
	// the clamp.
	pathFillPayload = pathLinkMTU - 20 - 8
	// pathFillPayload6 is the IPv6 twin (40 IPv6 + 8 ICMPv6 + payload).
	pathFillPayload6 = pathLinkMTU - 40 - 8
	// pathMidPayload makes a 1478-octet datagram: over the clamp, under the
	// link, which is the size that tells a poisoned cache from the wire.
	pathMidPayload = 1450
	// pathReadWait bounds every wait on the kernel in these tests.
	pathReadWait = 3 * time.Second
	// pathReadsMax bounds the datagrams one wait reads past: the raw socket
	// also receives the ICMP error itself, and anything else on the link.
	pathReadsMax = 16
)

var (
	senderAddr4   = netip.MustParseAddr("10.99.1.1")
	routerNear4   = netip.MustParseAddr("10.99.1.2")
	routerFarAddr = netip.MustParseAddr("10.99.2.1")
	farAddr4      = netip.MustParseAddr("10.99.2.2")
	farAddr4Other = netip.MustParseAddr("10.99.2.3")
	senderAddr6   = netip.MustParseAddr("fd99:1::1")
	routerNear6   = netip.MustParseAddr("fd99:1::2")
	routerFar6    = netip.MustParseAddr("fd99:2::1")
	farAddr6      = netip.MustParseAddr("fd99:2::2")
)

// clampedPath is the three-namespace topology and the handles to drive it.
type clampedPath struct {
	orig, sender, router, far netns.NsHandle
	routerHandle, farHandle   *netlink.Handle
}

// withClampedPath builds the topology, leaves the calling goroutine locked
// to its thread and inside the sender namespace, runs fn, and tears
// everything down. It skips, never fails, when a namespace or a link cannot
// be created.
func withClampedPath(t *testing.T, fn func(p *clampedPath)) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires root: network namespaces, veth links and raw sockets")
	}
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)

	orig, err := netns.Get()
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: current namespace: %v", err)
	}
	t.Cleanup(func() {
		if setErr := netns.Set(orig); setErr != nil {
			t.Errorf("restore namespace: %v", setErr)
		}
		orig.Close() //nolint:errcheck // best-effort cleanup
	})
	p := &clampedPath{orig: orig}
	base := nsBaseName(t.Name())
	p.sender = newNamespace(t, orig, base+"s")
	p.router = newNamespace(t, orig, base+"r")
	p.far = newNamespace(t, orig, base+"f")

	senderHandle := handleAt(t, p.sender)
	p.routerHandle = handleAt(t, p.router)
	p.farHandle = handleAt(t, p.far)

	addVeth(t, senderHandle, "sr0", "rs0", p.router)
	addVeth(t, p.routerHandle, "rf0", "fr0", p.far)

	configureLink(t, senderHandle, "sr0", pathLinkMTU, senderAddr4, senderAddr6)
	configureLink(t, p.routerHandle, "rs0", pathLinkMTU, routerNear4, routerNear6)
	configureLink(t, p.routerHandle, "rf0", pathClampMTU, routerFarAddr, routerFar6)
	configureLink(t, p.farHandle, "fr0", pathClampMTU, farAddr4, farAddr6)
	addAddr(t, p.farHandle, "fr0", farAddr4Other, 24)

	addRoute(t, senderHandle, "sr0", netip.MustParsePrefix("10.99.2.0/24"), routerNear4)
	addRoute(t, senderHandle, "sr0", netip.MustParsePrefix("fd99:2::/64"), routerNear6)
	addRoute(t, p.farHandle, "fr0", netip.MustParsePrefix("10.99.1.0/24"), routerFarAddr)
	addRoute(t, p.farHandle, "fr0", netip.MustParsePrefix("fd99:1::/64"), routerFar6)

	enableForwarding(t, p.router)

	if setErr := netns.Set(p.sender); setErr != nil {
		t.Fatalf("enter sender namespace: %v", setErr)
	}
	fn(p)
}

// nsBaseName derives a short, valid namespace name prefix from a test name.
func nsBaseName(testName string) string {
	name := strings.NewReplacer("/", "", "_", "", "Test", "", "Probe", "").Replace(testName)
	name = strings.ToLower(name)
	if len(name) > 10 {
		name = name[:10]
	}
	return "zedf" + name
}

func newNamespace(t *testing.T, orig netns.NsHandle, name string) netns.NsHandle {
	t.Helper()
	ns, err := netns.NewNamed(name)
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: create namespace %s: %v", name, err)
	}
	// NewNamed moves the thread into the new namespace; go back at once.
	if setErr := netns.Set(orig); setErr != nil {
		t.Fatalf("return to original namespace: %v", setErr)
	}
	t.Cleanup(func() {
		ns.Close()              //nolint:errcheck // best-effort cleanup
		netns.DeleteNamed(name) //nolint:errcheck // best-effort cleanup
	})
	return ns
}

func handleAt(t *testing.T, ns netns.NsHandle) *netlink.Handle {
	t.Helper()
	h, err := netlink.NewHandleAt(ns)
	if err != nil {
		t.Fatalf("netlink handle: %v", err)
	}
	t.Cleanup(h.Close)
	return h
}

// addVeth creates a veth pair with name in h's namespace and peer moved
// into peerNS.
func addVeth(t *testing.T, h *netlink.Handle, name, peer string, peerNS netns.NsHandle) {
	t.Helper()
	veth := &netlink.Veth{PeerName: peer, PeerNamespace: netlink.NsFd(peerNS)}
	veth.Name = name
	veth.MTU = pathLinkMTU
	if err := h.LinkAdd(veth); err != nil {
		t.Skipf("add veth %s/%s (needs CAP_NET_ADMIN): %v", name, peer, err)
	}
}

// configureLink sets the MTU, adds one address of each family and brings
// the link up.
func configureLink(t *testing.T, h *netlink.Handle, name string, mtu int, addr4, addr6 netip.Addr) {
	t.Helper()
	link, err := h.LinkByName(name)
	if err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	if err := h.LinkSetMTU(link, mtu); err != nil {
		t.Fatalf("set %s mtu %d: %v", name, mtu, err)
	}
	addAddr(t, h, name, addr4, 24)
	addAddr(t, h, name, addr6, 64)
	if err := h.LinkSetUp(link); err != nil {
		t.Fatalf("up %s: %v", name, err)
	}
}

// addAddr adds addr/bits to the named link. An IPv6 address skips DAD so
// it is usable at once.
func addAddr(t *testing.T, h *netlink.Handle, name string, addr netip.Addr, bits int) {
	t.Helper()
	link, err := h.LinkByName(name)
	if err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	a := &netlink.Addr{IPNet: &net.IPNet{IP: addr.AsSlice(), Mask: net.CIDRMask(bits, addr.BitLen())}}
	if addr.Is6() {
		a.Flags = unix.IFA_F_NODAD
	}
	if err := h.AddrAdd(link, a); err != nil {
		t.Fatalf("add %s to %s: %v", a.IPNet, name, err)
	}
}

func addRoute(t *testing.T, h *netlink.Handle, name string, dst netip.Prefix, via netip.Addr) {
	t.Helper()
	link, err := h.LinkByName(name)
	if err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: dst.Addr().AsSlice(), Mask: net.CIDRMask(dst.Bits(), dst.Addr().BitLen())},
		Gw:        via.AsSlice(),
	}
	if err := h.RouteAdd(route); err != nil {
		t.Fatalf("add route %s via %s: %v", dst, via, err)
	}
}

// enableForwarding turns the router namespace into a router. /proc/sys is
// the calling thread's namespace, so the thread steps in and back out.
func enableForwarding(t *testing.T, router netns.NsHandle) {
	t.Helper()
	orig, err := netns.Get()
	if err != nil {
		t.Fatalf("current namespace: %v", err)
	}
	defer orig.Close() //nolint:errcheck // best-effort cleanup
	if setErr := netns.Set(router); setErr != nil {
		t.Fatalf("enter router namespace: %v", setErr)
	}
	defer func() {
		if setErr := netns.Set(orig); setErr != nil {
			t.Fatalf("leave router namespace: %v", setErr)
		}
	}()
	for _, path := range []string{"/proc/sys/net/ipv4/ip_forward", "/proc/sys/net/ipv6/conf/all/forwarding"} {
		if err := os.WriteFile(path, []byte("1\n"), 0o644); err != nil {
			t.Fatalf("enable forwarding %s: %v", path, err)
		}
	}
}

// setFarLinkMTU moves the clamp: both ends of the far link take mtu.
func (p *clampedPath) setFarLinkMTU(t *testing.T, mtu int) {
	t.Helper()
	for _, end := range []struct {
		h    *netlink.Handle
		name string
	}{{p.routerHandle, "rf0"}, {p.farHandle, "fr0"}} {
		link, err := end.h.LinkByName(end.name)
		if err != nil {
			t.Fatalf("link %s: %v", end.name, err)
		}
		if err := end.h.LinkSetMTU(link, mtu); err != nil {
			t.Fatalf("set %s mtu %d: %v", end.name, mtu, err)
		}
	}
}

// openProbe opens a probe socket in the sender namespace, skipping when the
// raw socket is refused.
func openProbe(t *testing.T, family Family, df DFMode) *Socket {
	t.Helper()
	conn, err := OpenICMP(context.Background(), family, netip.Addr{}, df)
	if err != nil {
		if errors.Is(err, unix.EPERM) {
			t.Skipf("requires CAP_NET_RAW: %v", err)
		}
		t.Fatalf("OpenICMP(%v, %v): %v", family, df, err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // best-effort cleanup
	return conn
}

func echoType(family Family) byte {
	if family == FamilyIPv6 {
		return 128
	}
	return 8
}

func sendEcho(t *testing.T, conn *Socket, family Family, dest netip.Addr, id, seq uint16, payloadLen int) error {
	t.Helper()
	pkt := BuildICMPEcho(echoType(family), id, seq, make([]byte, payloadLen))
	_, err := conn.WriteTo(pkt, &net.IPAddr{IP: dest.AsSlice()})
	return err
}

// readOutcome is what one bounded wait on the probe socket observed.
type readOutcome struct {
	// readErr is the first error the ordinary read returned, if any: the
	// wake a queued refusal produces under IP_RECVERR.
	readErr error
	// reply is true when an echo reply for (id, seq) arrived.
	reply bool
}

// awaitReadEvent reads the probe socket until the ordinary read fails, an
// echo reply for (id, seq) arrives, or the wait ends. It observes rather
// than assumes how a queued refusal reaches a reader.
func awaitReadEvent(t *testing.T, conn *Socket, family Family, id, seq uint16) readOutcome {
	t.Helper()
	if err := conn.SetDeadline(time.Now().Add(pathReadWait)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 2048)
	replyType := echoType(family) + 1
	if family == FamilyIPv4 {
		replyType = 0
	}
	for range pathReadsMax {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return readOutcome{}
			}
			return readOutcome{readErr: err}
		}
		if n < 8 {
			continue
		}
		if buf[0] != replyType {
			continue
		}
		if binary.BigEndian.Uint16(buf[4:6]) != id {
			continue
		}
		if binary.BigEndian.Uint16(buf[6:8]) != seq {
			continue
		}
		return readOutcome{reply: true}
	}
	return readOutcome{}
}

func drainAll(t *testing.T, conn *Socket, _ Family) []QueuedError {
	t.Helper()
	var entries []QueuedError
	if err := conn.DrainErrors(func(q QueuedError) { entries = append(entries, q) }); err != nil {
		t.Fatalf("drainErrorQueue: %v", err)
	}
	return entries
}

// TestProbeErrorQueueReportsNextHopMTU is the wiring test for the error
// queue and VALIDATES A-1: an oversized DF probe on the clamped path yields
// an error-queue entry whose reported MTU is the clamp, from the router,
// quoting the probe. It also observes the wake: under IP_RECVERR the
// ordinary read returns EMSGSIZE once, which is what the ping receiver
// relies on to ask for a drain.
func TestProbeErrorQueueReportsNextHopMTU(t *testing.T) {
	withClampedPath(t, func(_ *clampedPath) {
		for _, df := range []DFMode{DFHonorCache, DFBypassCache} {
			conn := openProbe(t, FamilyIPv4, df)
			const id, seq = 0x5a01, 3
			if err := sendEcho(t, conn, FamilyIPv4, farAddr4, id, seq, pathFillPayload); err != nil {
				t.Fatalf("%v: send: %v", df, err)
			}
			got := awaitReadEvent(t, conn, FamilyIPv4, id, seq)
			if got.reply {
				t.Fatalf("%v: a %d-octet DF datagram crossed a %d clamp: the router fragmented it", df, pathLinkMTU, pathClampMTU)
			}
			if !errors.Is(got.readErr, unix.EMSGSIZE) {
				t.Fatalf("%v: ordinary read returned %v, want EMSGSIZE: the receiver's wake is not what the design assumed", df, got.readErr)
			}
			entries := drainAll(t, conn, FamilyIPv4)
			if len(entries) != 1 {
				t.Fatalf("%v: %d queued entries, want 1: %+v", df, len(entries), entries)
			}
			e := entries[0]
			if e.Outcome != ErrQueueMTUReported || e.MTU != pathClampMTU {
				t.Errorf("%v: outcome %v mtu %d, want %v %d", df, e.Outcome, e.MTU, ErrQueueMTUReported, pathClampMTU)
			}
			if e.Local {
				t.Errorf("%v: the router's refusal read as local", df)
			}
			if e.Offender != routerNear4 {
				t.Errorf("%v: offender %v, want the router %v", df, e.Offender, routerNear4)
			}
			want := QuotedEcho{Present: true, ID: id, Seq: seq}
			if e.Echo != want {
				t.Errorf("%v: quoted echo %+v, want %+v", df, e.Echo, want)
			}
			// Nothing else may be queued behind it.
			if left := drainAll(t, conn, FamilyIPv4); len(left) != 0 {
				t.Errorf("%v: %d entries left after the drain", df, len(left))
			}
		}
	})
}

// TestProbeErrorQueueReportsNextHopMTUIPv6 is A-1 for IPv6: a Packet Too
// Big from the router reaches the queue with the clamp.
func TestProbeErrorQueueReportsNextHopMTUIPv6(t *testing.T) {
	withClampedPath(t, func(_ *clampedPath) {
		conn := openProbe(t, FamilyIPv6, DFBypassCache)
		const id, seq = 0x5a06, 4
		if err := sendEcho(t, conn, FamilyIPv6, farAddr6, id, seq, pathFillPayload6); err != nil {
			t.Fatalf("send: %v", err)
		}
		got := awaitReadEvent(t, conn, FamilyIPv6, id, seq)
		if got.reply {
			t.Fatalf("a %d-octet IPv6 datagram crossed a %d clamp", pathLinkMTU, pathClampMTU)
		}
		if !errors.Is(got.readErr, unix.EMSGSIZE) {
			t.Fatalf("ordinary read returned %v, want EMSGSIZE", got.readErr)
		}
		entries := drainAll(t, conn, FamilyIPv6)
		if len(entries) != 1 {
			t.Fatalf("%d queued entries, want 1: %+v", len(entries), entries)
		}
		e := entries[0]
		if e.Outcome != ErrQueueMTUReported || e.MTU != pathClampMTU {
			t.Errorf("outcome %v mtu %d, want %v %d", e.Outcome, e.MTU, ErrQueueMTUReported, pathClampMTU)
		}
		if e.Offender != routerNear6 {
			t.Errorf("offender %v, want the router %v", e.Offender, routerNear6)
		}
		want := QuotedEcho{Present: true, ID: id, Seq: seq}
		if e.Echo != want {
			t.Errorf("quoted echo %+v, want %+v", e.Echo, want)
		}
	})
}

// TestProbeBypassCacheDisagreesWithPoisonedCache VALIDATES A-2 and AC-5,
// and AC-6 for the kernel estimate. The cache is poisoned by letting an
// honor-cache probe learn the 1400 clamp, then the clamp is lifted to 1500.
// A 1478-octet honor-cache probe is then refused at send with EMSGSIZE and
// IP_MTU still reads 1400, while the same probe in bypass mode crosses the
// path and is answered: the disagreement is the proof the cache was
// bypassed.
func TestProbeBypassCacheDisagreesWithPoisonedCache(t *testing.T) {
	withClampedPath(t, func(p *clampedPath) {
		honor := openProbe(t, FamilyIPv4, DFHonorCache)
		const id = 0x5a02
		if err := sendEcho(t, honor, FamilyIPv4, farAddr4, id, 1, pathFillPayload); err != nil {
			t.Fatalf("poisoning send: %v", err)
		}
		got := awaitReadEvent(t, honor, FamilyIPv4, id, 1)
		if !errors.Is(got.readErr, unix.EMSGSIZE) {
			t.Fatalf("poisoning probe: read returned %v, want EMSGSIZE", got.readErr)
		}
		if entries := drainAll(t, honor, FamilyIPv4); len(entries) != 1 || entries[0].MTU != pathClampMTU {
			t.Fatalf("poisoning probe queued %+v, want one entry reporting %d", entries, pathClampMTU)
		}

		p.setFarLinkMTU(t, pathLinkMTU)

		estimate, err := KernelPathMTU(context.Background(), farAddr4)
		if err != nil {
			t.Fatalf("KernelPathMTU: %v", err)
		}
		if estimate != pathClampMTU {
			t.Fatalf("kernel estimate %d after the clamp was lifted, want the cached %d", estimate, pathClampMTU)
		}

		sendErr := sendEcho(t, honor, FamilyIPv4, farAddr4, id, 2, pathMidPayload)
		if !errors.Is(sendErr, unix.EMSGSIZE) {
			t.Fatalf("honor-cache send of %d octets against a %d cache: err %v, want EMSGSIZE", pathMidPayload+28, pathClampMTU, sendErr)
		}
		local := drainAll(t, honor, FamilyIPv4)
		if len(local) != 1 {
			t.Fatalf("refused send queued %d entries, want 1: %+v", len(local), local)
		}
		if !local[0].Local || local[0].MTU != pathClampMTU {
			t.Errorf("refused send queued %+v, want a local entry reporting %d", local[0], pathClampMTU)
		}

		bypass := openProbe(t, FamilyIPv4, DFBypassCache)
		if err := sendEcho(t, bypass, FamilyIPv4, farAddr4, id, 3, pathMidPayload); err != nil {
			t.Fatalf("bypass send: %v", err)
		}
		got = awaitReadEvent(t, bypass, FamilyIPv4, id, 3)
		if !got.reply {
			t.Fatalf("bypass probe of %d octets got no reply (read err %v): the cache was honored", pathMidPayload+28, got.readErr)
		}
	})
}

// TestProbeErrorQueueAndAnotherFlow VALIDATES A-3: what the kernel matches
// a queued error to. Two probe sockets in the sender namespace each provoke
// a refusal toward a different far address. Whatever the kernel queues on
// the first socket must quote the probe it is about, so an entry about the
// other flow is never attributed to this socket's own probe. The count of
// foreign entries is logged: zero means the kernel matched per flow, more
// means it matched per protocol and Ze's quoted-echo match is the defense.
func TestProbeErrorQueueAndAnotherFlow(t *testing.T) {
	withClampedPath(t, func(_ *clampedPath) {
		first := openProbe(t, FamilyIPv4, DFBypassCache)
		second := openProbe(t, FamilyIPv4, DFBypassCache)
		const firstID, secondID = 0x5a03, 0x5a04

		if err := sendEcho(t, second, FamilyIPv4, farAddr4Other, secondID, 1, pathFillPayload); err != nil {
			t.Fatalf("second socket send: %v", err)
		}
		got := awaitReadEvent(t, second, FamilyIPv4, secondID, 1)
		if !errors.Is(got.readErr, unix.EMSGSIZE) {
			t.Fatalf("second socket: read returned %v, want EMSGSIZE", got.readErr)
		}
		own := drainAll(t, second, FamilyIPv4)
		if len(own) != 1 || own[0].Echo.ID != secondID {
			t.Fatalf("second socket queued %+v, want its own refusal", own)
		}

		foreign := drainAll(t, first, FamilyIPv4)
		for _, e := range foreign {
			if !e.Echo.Present {
				t.Errorf("first socket holds an entry quoting nothing: %+v", e)
				continue
			}
			if e.Echo.ID == firstID {
				t.Errorf("first socket holds an entry attributed to a probe it never sent: %+v", e)
			}
			if e.Echo.ID != secondID {
				t.Errorf("first socket holds an entry quoting an unknown probe: %+v", e)
			}
		}
		t.Logf("A-3: the first socket held %d entries about the other flow", len(foreign))

		// The first socket's own probe still gets its own refusal, with the
		// foreign traffic already drained.
		if err := sendEcho(t, first, FamilyIPv4, farAddr4, firstID, 1, pathFillPayload); err != nil {
			t.Fatalf("first socket send: %v", err)
		}
		got = awaitReadEvent(t, first, FamilyIPv4, firstID, 1)
		if !errors.Is(got.readErr, unix.EMSGSIZE) {
			t.Fatalf("first socket: read returned %v, want EMSGSIZE", got.readErr)
		}
		mine := drainAll(t, first, FamilyIPv4)
		if len(mine) != 1 || mine[0].Echo.ID != firstID || mine[0].MTU != pathClampMTU {
			t.Fatalf("first socket queued %+v, want its own refusal reporting %d", mine, pathClampMTU)
		}
	})
}

// TestProbeDFBitOnTheWire proves the DF flag as the router sees it, from an
// AF_PACKET capture on the router's near link rather than from Ze's report:
// set for a DF mode, clear for DFOff. The probe is small enough to cross
// the clamp so the capture sees a forwarded datagram, not a refusal.
func TestProbeDFBitOnTheWire(t *testing.T) {
	withClampedPath(t, func(p *clampedPath) {
		capture := openCapture(t, p, "rs0")
		cases := []struct {
			df   DFMode
			want bool
		}{{DFBypassCache, true}, {DFHonorCache, true}, {DFOff, false}}
		for i, c := range cases {
			conn := openProbe(t, FamilyIPv4, c.df)
			id := uint16(0x5a10 + i)
			if err := sendEcho(t, conn, FamilyIPv4, farAddr4, id, 1, 100); err != nil {
				t.Fatalf("%v: send: %v", c.df, err)
			}
			flags, ok := captureEchoFlags(t, capture, id)
			if !ok {
				t.Fatalf("%v: the capture on the router never saw the probe", c.df)
			}
			if got := flags&0x40 != 0; got != c.want {
				t.Errorf("%v: DF bit on the wire = %v, want %v (flags byte %#x)", c.df, got, c.want, flags)
			}
		}
	})
}

// openCapture opens an AF_PACKET socket in the router namespace bound to
// the named link, with a bounded receive timeout.
func openCapture(t *testing.T, p *clampedPath, name string) int {
	t.Helper()
	if setErr := netns.Set(p.router); setErr != nil {
		t.Fatalf("enter router namespace: %v", setErr)
	}
	defer func() {
		if setErr := netns.Set(p.sender); setErr != nil {
			t.Fatalf("return to sender namespace: %v", setErr)
		}
	}()
	link, err := p.routerHandle.LinkByName(name)
	if err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	proto := int(htons(unix.ETH_P_IP))
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, proto)
	if err != nil {
		t.Skipf("requires CAP_NET_RAW: AF_PACKET socket: %v", err)
	}
	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck // best-effort cleanup
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_IP), Ifindex: link.Attrs().Index}); err != nil {
		t.Fatalf("bind capture to %s: %v", name, err)
	}
	tv := unix.NsecToTimeval(pathReadWait.Nanoseconds())
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		t.Fatalf("set capture timeout: %v", err)
	}
	return fd
}

func htons(v uint16) uint16 { return v<<8 | v>>8 }

// captureEchoFlags reads frames off the capture until an IPv4 ICMP echo
// request carrying id passes, and returns its IPv4 flags byte.
func captureEchoFlags(t *testing.T, fd int, id uint16) (byte, bool) {
	t.Helper()
	frame := make([]byte, 2048)
	for range pathReadsMax * 4 {
		n, _, err := unix.Recvfrom(fd, frame, 0)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) {
				return 0, false
			}
			t.Fatalf("capture read: %v", err)
		}
		const ethLen = 14
		if n < ethLen+20+8 {
			continue
		}
		ip := frame[ethLen:n]
		ihl := int(ip[0]&0x0f) * 4
		if ip[0]>>4 != 4 {
			continue
		}
		if ip[9] != syscall.IPPROTO_ICMP {
			continue
		}
		if len(ip) < ihl+8 {
			continue
		}
		icmp := ip[ihl:]
		if icmp[0] != 8 {
			continue
		}
		if binary.BigEndian.Uint16(icmp[4:6]) != id {
			continue
		}
		return ip[6], true
	}
	return 0, false
}
