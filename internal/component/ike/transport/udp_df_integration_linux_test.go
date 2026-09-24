//go:build integration && linux

// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- the DF send and the error queue proven against the kernel
//
// VALIDATES: A-1 of spec-ike-padded-path-probe (DF toggled around one send on
// the shared socket takes effect for that datagram and for no other) and A-4
// (the kernel queues a router's Fragmentation Needed on the IKE socket's
// error queue, and the entry names the peer the datagram was sent to), each
// read off an AF_PACKET capture on the router or off the socket's own error
// queue rather than from Ze's report.
// PREVENTS: a DF send whose bit never reaches the wire, a retransmission that
// is not the same UDP payload, a plain Send that leaves under a probe's
// option, and a refusal the engine cannot match to an SA.
//
// Topology, three namespaces joined by two veth pairs, the same clamped path
// internal/core/probe proves the error queue on:
//
//	sender ----sr0/rs0---- router ----rf0/fr0---- far
//	10.99.1.1     1500    10.99.1.2  1400      10.99.2.2
//
// The router forwards and its far-side link is clamped to 1400, so a
// 1500-octet DF datagram from the sender is refused there with Fragmentation
// Needed reporting 1400. Every test skips, never fails, when the namespaces
// or the raw socket are out of reach: the QEMU runner (./le qemu all-tests)
// is where they run for real.

package transport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/probe"
)

// The addresses and sizes of the clamped path.
const (
	pathLinkMTU  = 1500
	pathClampMTU = 1400
	// pathFillPayload makes an IPv4 UDP datagram exactly pathLinkMTU long
	// (20 IP + 8 UDP + payload), so it fits the sender's link and exceeds the
	// clamp.
	pathFillPayload = pathLinkMTU - 20 - 8
	// pathFitPayload makes a datagram under the clamp, so both copies of it
	// cross the router and the capture sees each one.
	pathFitPayload = 1200
	// pathReadWait bounds every wait on the kernel in these tests.
	pathReadWait = 3 * time.Second
	// pathReadsMax bounds the frames one wait reads past.
	pathReadsMax = 64
	// plainSendRounds is how many DF-off sends the leak test interleaves with
	// plain sends. Without the lock a leak is a matter of scheduling, so the
	// count is what gives one a chance to show.
	plainSendRounds = 200
	// plainSendsPerRound is how many plain sends race each DF-off send.
	plainSendsPerRound = 4
	// captureBufferBytes is the capture socket's receive buffer, sized so the
	// leak test's every frame is kept: a dropped frame could be the leaked
	// one. The kernel charges about 2 KiB of truesize per frame.
	captureBufferBytes = 8 << 20
)

var (
	senderAddr4   = netip.MustParseAddr("10.99.1.1")
	routerNear4   = netip.MustParseAddr("10.99.1.2")
	routerFarAddr = netip.MustParseAddr("10.99.2.1")
	farAddr4      = netip.MustParseAddr("10.99.2.2")
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

	configureLink(t, senderHandle, "sr0", pathLinkMTU, senderAddr4)
	configureLink(t, p.routerHandle, "rs0", pathLinkMTU, routerNear4)
	configureLink(t, p.routerHandle, "rf0", pathClampMTU, routerFarAddr)
	configureLink(t, p.farHandle, "fr0", pathClampMTU, farAddr4)

	addRoute(t, senderHandle, "sr0", netip.MustParsePrefix("10.99.2.0/24"), routerNear4)
	addRoute(t, p.farHandle, "fr0", netip.MustParsePrefix("10.99.1.0/24"), routerFarAddr)

	enableForwarding(t, p.router)

	if setErr := netns.Set(p.sender); setErr != nil {
		t.Fatalf("enter sender namespace: %v", setErr)
	}
	fn(p)
}

// nsBaseName derives a short, valid namespace name prefix from a test name.
func nsBaseName(testName string) string {
	name := strings.NewReplacer("/", "", "_", "", "Test", "", "DF", "").Replace(testName)
	name = strings.ToLower(name)
	if len(name) > 10 {
		name = name[:10]
	}
	return "zeike" + name
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

// configureLink sets the MTU, adds the address and brings the link up.
func configureLink(t *testing.T, h *netlink.Handle, name string, mtu int, addr netip.Addr) {
	t.Helper()
	link, err := h.LinkByName(name)
	if err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	if err := h.LinkSetMTU(link, mtu); err != nil {
		t.Fatalf("set %s mtu %d: %v", name, mtu, err)
	}
	a := &netlink.Addr{IPNet: &net.IPNet{IP: addr.AsSlice(), Mask: net.CIDRMask(24, 32)}}
	if err := h.AddrAdd(link, a); err != nil {
		t.Fatalf("add %s to %s: %v", a.IPNet, name, err)
	}
	if err := h.LinkSetUp(link); err != nil {
		t.Fatalf("up %s: %v", name, err)
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
		Dst:       &net.IPNet{IP: dst.Addr().AsSlice(), Mask: net.CIDRMask(dst.Bits(), 32)},
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
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0o644); err != nil {
		t.Fatalf("enable forwarding: %v", err)
	}
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
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUFFORCE, captureBufferBytes); err != nil {
		t.Fatalf("set capture buffer: %v", err)
	}
	return fd
}

func htons(v uint16) uint16 { return v<<8 | v>>8 }

// capturedDatagram is one UDP datagram the router saw: its IPv4 flags byte
// and its UDP payload.
type capturedDatagram struct {
	flags   byte
	payload []byte
}

// df reports the Don't Fragment bit of the captured datagram.
func (d capturedDatagram) df() bool { return d.flags&0x40 != 0 }

// captureUDP reads frames off the capture until an unfragmented IPv4 UDP
// datagram to port passes, and returns it. A fragment (offset or more
// fragments set) is skipped, so a DF-off datagram larger than the link is
// never mistaken for a whole one.
func captureUDP(t *testing.T, fd int, port uint16) (capturedDatagram, bool) {
	t.Helper()
	frame := make([]byte, 2048)
	for range pathReadsMax {
		n, _, err := unix.Recvfrom(fd, frame, 0)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) {
				return capturedDatagram{}, false
			}
			t.Fatalf("capture read: %v", err)
		}
		const ethLen = 14
		if n < ethLen+20+8 {
			continue
		}
		ip := frame[ethLen:n]
		if ip[0]>>4 != 4 {
			continue
		}
		ihl := int(ip[0]&0x0f) * 4
		if ip[9] != syscall.IPPROTO_UDP {
			continue
		}
		if binary.BigEndian.Uint16(ip[6:8])&0x3fff != 0 {
			continue
		}
		if len(ip) < ihl+8 {
			continue
		}
		udp := ip[ihl:]
		if binary.BigEndian.Uint16(udp[2:4]) != port {
			continue
		}
		payload := make([]byte, len(udp)-8)
		copy(payload, udp[8:])
		return capturedDatagram{flags: ip[6], payload: payload}, true
	}
	return capturedDatagram{}, false
}

// openTransport opens the IKE transport on the sender's address in the
// sender namespace.
func openTransport(t *testing.T) *UDPTransport {
	t.Helper()
	tr, err := NewUDPTransport(net.JoinHostPort(senderAddr4.String(), "0"), slog.Default())
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	return tr
}

// farIKE is the peer endpoint every test sends to.
func farIKE() *net.UDPAddr {
	return &net.UDPAddr{IP: farAddr4.AsSlice(), Port: IKEPort}
}

// listenFar opens a UDP listener on the peer endpoint in the far namespace
// so a datagram that crosses the router is consumed there. Without it the
// far kernel answers every datagram with Port Unreachable, which the sender
// socket, carrying IP_RECVERR, then reports to its next send; the capture
// tests read the wire, not that path, so they take it out of the way.
func listenFar(t *testing.T, p *clampedPath) {
	t.Helper()
	if setErr := netns.Set(p.far); setErr != nil {
		t.Fatalf("enter far namespace: %v", setErr)
	}
	defer func() {
		if setErr := netns.Set(p.sender); setErr != nil {
			t.Fatalf("return to sender namespace: %v", setErr)
		}
	}()
	conn, err := net.ListenUDP("udp4", farIKE())
	if err != nil {
		t.Fatalf("listen on the far endpoint: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
}

// payloadTagged is a payload of size octets whose first four octets are
// tag, so the capture can tell one sender's datagrams from another's.
func payloadTagged(tag string, size int) []byte {
	payload := make([]byte, size)
	copy(payload, tag)
	for i := 4; i < size; i++ {
		payload[i] = byte(i)
	}
	return payload
}

// TestDFSendThenPlainCopyOnTheWire VALIDATES A-1: one payload sent twice
// through SendDF, first honor-cache then off, appears on the router's near
// link as a datagram with DF set and then the identical UDP payload with DF
// clear. The payload fits the clamp, so the capture sees both copies
// forwarded, and the second copy is what RFC 7296 Section 2.1 calls a
// retransmission: the same bytes from the IKE header on, only the IP header
// differs.
func TestDFSendThenPlainCopyOnTheWire(t *testing.T) {
	withClampedPath(t, func(p *clampedPath) {
		listenFar(t, p)
		capture := openCapture(t, p, "rs0")
		tr := openTransport(t)
		payload := payloadTagged("IKE1", pathFitPayload)

		if err := tr.SendDF(payload, nil, farIKE(), probe.DFHonorCache); err != nil {
			t.Fatalf("SendDF honor-cache: %v", err)
		}
		first, ok := captureUDP(t, capture, IKEPort)
		if !ok {
			t.Fatal("the capture on the router never saw the DF copy")
		}
		if !first.df() {
			t.Errorf("the first copy left with DF clear (flags byte %#x)", first.flags)
		}
		if !bytes.Equal(first.payload, payload) {
			t.Errorf("the first copy's UDP payload is not the payload sent (%d octets, want %d)", len(first.payload), len(payload))
		}

		if err := tr.SendDF(payload, nil, farIKE(), probe.DFOff); err != nil {
			t.Fatalf("SendDF off: %v", err)
		}
		second, ok := captureUDP(t, capture, IKEPort)
		if !ok {
			t.Fatal("the capture on the router never saw the DF-clear copy")
		}
		if second.df() {
			t.Errorf("the second copy left with DF set (flags byte %#x)", second.flags)
		}
		if !bytes.Equal(second.payload, payload) {
			t.Errorf("the second copy's UDP payload differs from the first: retransmission is not bitwise identical")
		}
	})
}

// TestPlainSendNeverLeavesUnderTheProbeOption VALIDATES A-1's other half
// and R-2: a plain Send racing SendDF on the same socket keeps the kernel
// default. Under that default (IP_PMTUDISC_WANT) a datagram that fits the
// path carries DF, so a plain datagram captured with DF clear is one that
// left while a DF-off send held the option, which the lock forbids. The
// DF-off sends are repeated so a leak has every chance to show.
func TestPlainSendNeverLeavesUnderTheProbeOption(t *testing.T) {
	withClampedPath(t, func(p *clampedPath) {
		listenFar(t, p)
		capture := openCapture(t, p, "rs0")
		tr := openTransport(t)
		plain := payloadTagged("PLAI", 64)
		probePayload := payloadTagged("PROB", 64)

		const plainSends = plainSendRounds * plainSendsPerRound
		var wg sync.WaitGroup
		wg.Go(func() {
			for range plainSends {
				if err := tr.Send(plain, farIKE()); err != nil {
					t.Errorf("Send: %v", err)
					return
				}
			}
		})
		for range plainSendRounds {
			if err := tr.SendDF(probePayload, nil, farIKE(), probe.DFOff); err != nil {
				t.Fatalf("SendDF off: %v", err)
			}
		}
		wg.Wait()

		// Every frame is read, and every frame is counted, so a leaked one
		// cannot hide behind a dropped one: the capture buffer holds them all.
		plainSeen, probeSeen := 0, 0
		for range plainSends + plainSendRounds {
			d, ok := captureUDP(t, capture, IKEPort)
			if !ok {
				break
			}
			switch string(d.payload[:4]) {
			case "PLAI":
				plainSeen++
				if !d.df() {
					t.Fatalf("a plain Send left with DF clear: it interleaved with a DF-off send (flags byte %#x)", d.flags)
				}
			case "PROB":
				probeSeen++
				if d.df() {
					t.Fatalf("a DF-off send left with DF set (flags byte %#x)", d.flags)
				}
			}
		}
		if plainSeen != plainSends {
			t.Errorf("the capture saw %d plain datagrams, want %d", plainSeen, plainSends)
		}
		if probeSeen != plainSendRounds {
			t.Errorf("the capture saw %d DF-off datagrams, want %d", probeSeen, plainSendRounds)
		}
	})
}

// TestOversizedDFSendQueuesRefusalForThePeer VALIDATES A-4: a DF datagram
// larger than the clamp draws the router's Fragmentation Needed, the kernel
// queues it on the IKE socket, and Run delivers it on Refusals naming the
// peer the datagram was sent to, the router as the offender and the
// reported 1400. The second honor-cache send of the same size is then
// refused by this host's own kernel against the cache the first answer
// filled: SendDF fails, and the LOCAL entry reaches Refusals too, with
// Local set, no offender, and the peer's port 0, which is what the kernel
// knows of an unconnected socket's destination port (__ip_append_data
// hands ip_local_error inet_dport).
func TestOversizedDFSendQueuesRefusalForThePeer(t *testing.T) {
	withClampedPath(t, func(p *clampedPath) {
		tr := openTransport(t)
		go tr.Run()
		payload := payloadTagged("BIG1", pathFillPayload)
		peer := netip.AddrPortFrom(farAddr4, IKEPort)

		if err := tr.SendDF(payload, nil, farIKE(), probe.DFHonorCache); err != nil {
			t.Fatalf("SendDF honor-cache: %v", err)
		}
		var routerRefusal SizeRefusal
		select {
		case routerRefusal = <-tr.Refusals():
		case <-time.After(pathReadWait):
			t.Fatal("the router's Fragmentation Needed never reached Refusals")
		}
		want := SizeRefusal{Peer: peer, Outcome: probe.ErrQueueMTUReported, MTU: pathClampMTU, Offender: routerNear4}
		if routerRefusal != want {
			t.Errorf("router refusal = %+v, want %+v", routerRefusal, want)
		}

		err := tr.SendDF(payload, nil, farIKE(), probe.DFHonorCache)
		if !errors.Is(err, unix.EMSGSIZE) {
			t.Fatalf("the second honor-cache send answered %v, want EMSGSIZE from the cache", err)
		}
		var localRefusal SizeRefusal
		select {
		case localRefusal = <-tr.Refusals():
		case <-time.After(pathReadWait):
			t.Fatal("the local refusal never reached Refusals")
		}
		wantLocal := SizeRefusal{Peer: netip.AddrPortFrom(farAddr4, 0), Outcome: probe.ErrQueueMTUReported, MTU: pathClampMTU, Local: true}
		if localRefusal != wantLocal {
			t.Errorf("local refusal = %+v, want %+v", localRefusal, wantLocal)
		}
	})
}
