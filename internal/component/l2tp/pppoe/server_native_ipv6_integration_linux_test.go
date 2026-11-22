//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- kernel session IPv6 carrier and native release
// Related: server.go -- handlePADR and handlePADT, reached through dispatchFrame
// Related: subsystem_reload.go -- newInterfaceServer opens sessions through the prepared access

package pppoe

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	_ "github.com/ze-software/ze/internal/component/l2tp/plugins/pppoe-kernel"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
)

const (
	serverV6Wait    = 10 * time.Second
	serverV6Access  = "zv6ac0"
	serverV6Peer    = "zv6sub0"
	serverV6Renamed = "zv6orig0"
	// RFC 2516 Section 4: the Session stage ETHER_TYPE.
	serverV6SessionEtherType = 0x8864
	serverV6IPv6EtherType    = 0x86dd
	serverV6Magic            = 0x5a650001
	// RFC 4861 Section 6.2.6 answers a solicitation within MIN_DELAY_BETWEEN_RAS
	// plus MAX_RA_DELAY_TIME (3.5 s). The second unsolicited advertisement of the
	// initial burst is 16 s after the first, so an RA inside this bound after the
	// rename was written by the session's carrier after the rename.
	serverV6RAWait = 8 * time.Second
)

var (
	serverV6AccessMAC = [EthALen]byte{2, 0, 0x5e, 6, 0, 1}
	serverV6PeerMAC   = [EthALen]byte{2, 0, 0x5e, 6, 0, 2}
	serverV6PeerID    = [8]byte{0, 0, 0, 0, 0, 0, 0, 2}
	serverV6PeerLL    = netip.MustParseAddr("fe80::2")
)

// TestServerNativeIPv6CarrierFollowsOriginalUnit admits a subscriber through
// the production discovery handlers over the real netlink access, then gives
// the session's pppN name to a new dummy link.
//
// VALIDATES: the kernel session opens its RA and DHCPv6 sockets from the
// original created PPP unit: after the rename, a Router Solicitation on the
// session is answered by an RA on the session, a DHCPv6 Solicit by a Reply on
// the session, and the dummy that took the old name carries neither.
// PREVENTS: a carrier bound or filtered by interface name, which after the
// rename drops the original's packets and writes onto the dummy.
func TestServerNativeIPv6CarrierFollowsOriginalUnit(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newServerV6Fixture(t)
	f.peer.drainIPv6()
	original, err := f.route.LinkByIndex(int(f.identity.NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.route.LinkSetName(original, serverV6Renamed); err != nil {
		t.Fatalf("rename the original %s: %v", f.identity.InterfaceName, err)
	}
	dummy := &netlink.Dummy{Name: f.identity.InterfaceName}
	if err := f.route.LinkAdd(dummy); err != nil {
		t.Fatalf("give the old name %s to a dummy: %v", f.identity.InterfaceName, err)
	}
	replacement, err := f.route.LinkByName(f.identity.InterfaceName)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.route.LinkDel(replacement); err != nil {
			t.Error(err)
		}
	})
	if replacement.Attrs().Index == int(f.identity.NativeIndex) {
		t.Fatal("the dummy reused the original's index; the rename proves nothing")
	}
	if err := f.route.LinkSetUp(replacement); err != nil {
		t.Fatal(err)
	}
	// Linux packet_do_bind reports ENETDOWN on a socket bound to a down
	// device, so the capture binds only once the dummy is up.
	capture := serverV6Capture(t, replacement.Attrs().Index)

	f.peer.sendIPv6(t, serverV6RS())
	deadline := time.Now().Add(serverV6RAWait)
	for {
		packet := f.peer.receiveIPv6(t, deadline, "a Router Advertisement after the rename")
		if !serverV6IsRA(packet) {
			continue
		}
		if packet.source != f.localLL {
			t.Fatalf("RA source=%s, want the session's link-local %s", packet.source, f.localLL)
		}
		if packet.hop != 255 {
			t.Fatalf("RA hop limit=%d, want 255", packet.hop)
		}
		break
	}

	const marker = 0x31
	f.peer.sendIPv6(t, serverV6Solicit(marker))
	deadline = time.Now().Add(serverV6Wait)
	for {
		packet := f.peer.receiveIPv6(t, deadline, "a DHCPv6 Reply after the rename")
		if !serverV6IsDHCPReply(packet, marker) {
			continue
		}
		if packet.source != f.localLL {
			t.Fatalf("DHCPv6 Reply source=%s, want the session's link-local %s", packet.source, f.localLL)
		}
		if packet.destination != serverV6PeerLL {
			t.Fatalf("DHCPv6 Reply destination=%s, want %s", packet.destination, serverV6PeerLL)
		}
		break
	}

	serverV6RequireSilent(t, capture)
}

// TestServerNativeIPv6TeardownReleasesOriginalUnit ends an IPv6 session with a
// PADT through the production discovery handler.
//
// VALIDATES: teardown completes native release of the original unit: the
// admitted attempt reports its protocol run finished and its owner released
// with no error, the driver holds no pending native release, and the original
// unit's interface index no longer exists once release is reported.
// PREVENTS: a teardown that reports release while the original pppN still
// exists, or that never completes native release.
func TestServerNativeIPv6TeardownReleasesOriginalUnit(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newServerV6Fixture(t)
	f.releaseByPADT(t)
	_, err := f.route.LinkByIndex(int(f.identity.NativeIndex))
	if err == nil {
		t.Fatalf("release reported while the original %s (index %d) still exists", f.identity.InterfaceName, f.identity.NativeIndex)
	}
	if _, missing := errors.AsType[netlink.LinkNotFoundError](err); !missing {
		t.Fatalf("read the original unit's index: %v", err)
	}
}

// serverV6Fixture holds one admitted subscriber whose IPv6CP is Opened and
// whose IPv6 service has joined its multicast groups. Its workers belong to
// userns.Enter's private child process and end in the test's cleanup.
type serverV6Fixture struct {
	t        *testing.T
	route    *netlink.Handle
	driver   *ppp.Driver
	sub      *Subsystem
	server   *InterfaceServer
	peer     *serverV6PeerWire
	sid      uint16
	attempt  ppp.SessionAttempt
	identity ppp.SessionIdentity
	localLL  netip.Addr
	// discovery receives the frames the server's discovery handlers send.
	discovery chan []byte
	// framedRoutes is the profile the fixture's auth acceptance stages. Only
	// a credentialed original round carries one (EventAuthRequest.StageFramedRoutes).
	framedRoutes []l2tpevents.FramedRoute
}

// serverV6NoAuth is the access configuration of the tests that need no
// authenticated profile.
const serverV6NoAuth = "pppoe { auth-method none; allow-no-auth true; interface " + serverV6Access + " {} }"

func newServerV6Fixture(t *testing.T) *serverV6Fixture {
	t.Helper()
	return startServerV6Fixture(t, serverV6NoAuth, nil)
}

// startServerV6Fixture admits one subscriber under config, the PPPoE
// configuration text, and accepts its authentication with framedRoutes.
func startServerV6Fixture(t *testing.T, config string, framedRoutes []l2tpevents.FramedRoute) *serverV6Fixture {
	t.Helper()
	serverV6Defaults(t)
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
	})
	route, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(route.Close)
	f := &serverV6Fixture{t: t, route: route, discovery: make(chan []byte, 8), framedRoutes: framedRoutes}
	f.peer = serverV6Veth(t, route)

	p := extractFromConfigText(t, config)
	backend, err := selectedSessionBackend()
	if err != nil {
		t.Fatal(err)
	}
	accesses, err := prepareAccessConfiguration(t.Context(), backend, p.Interfaces, nil)
	if err != nil {
		t.Fatalf("prepare the real netlink access: %v", err)
	}
	t.Cleanup(func() {
		if err := closeAccessConfiguration(accesses.accesses, nil); err != nil {
			t.Error(err)
		}
	})
	f.driver = ppp.NewProductionDriver(slog.Default())
	if err := f.driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.driver.Stop)
	f.sub = NewSubsystem(p)
	f.sub.pppDriver, f.sub.backend, f.sub.bus = f.driver, backend, newRecordBus()
	f.server, err = f.sub.newInterfaceServer(&p.Interfaces[0], &p, backend, accesses.accesses[serverV6Access])
	if err != nil {
		t.Fatal(err)
	}
	// No discovery socket runs here: the frames the handlers send are observed.
	f.server.sendFrameFn = func(frame []byte) {
		select {
		case f.discovery <- bytes.Clone(frame):
		default:
			t.Error("discovery observer is full; a server frame was dropped")
		}
	}
	f.sub.servers = map[int]*InterfaceServer{f.server.ifIndex: f.server}
	ctx, cancel := context.WithCancel(context.Background())
	pumped := make(chan struct{})
	go f.pump(ctx, pumped)
	t.Cleanup(func() {
		cancel()
		<-pumped
	})

	f.admit(t)
	f.peer.sid = f.sid
	served := make(chan struct{})
	go f.peer.serve(ctx, t, served)
	t.Cleanup(func() {
		cancel()
		<-served
	})
	f.requireIPv6Service(t)
	return f
}

// pump is the fixture's PPP event consumer: lifecycle events reach the
// production handlePPPEvent, and the auth and address requests a pool and an
// auth plugin would answer are accepted here. It returns when ctx ends.
func (f *serverV6Fixture) pump(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	// One worker per fixture; it ends with the test's cleanup.
	for {
		select {
		case <-ctx.Done():
			return
		case event, open := <-f.driver.EventsOut():
			if !open {
				return
			}
			f.sub.handlePPPEvent(event)
		case event, open := <-f.driver.AuthEventsOut():
			if !open {
				return
			}
			request, ok := event.(ppp.EventAuthRequest)
			if !ok {
				continue
			}
			// A round that may carry no profile stages nothing and is no error.
			if err := request.StageFramedRoutes(f.framedRoutes); err != nil {
				f.t.Errorf("stage the accepted profile's framed routes: %v", err)
			}
			if err := f.driver.AuthResponse(request, true, "", nil); err != nil {
				f.t.Errorf("accept the no-auth request: %v", err)
			}
		case event, open := <-f.driver.IPEventsOut():
			if !open {
				return
			}
			request, ok := event.(ppp.EventIPRequest)
			if !ok {
				continue
			}
			args := ppp.IPResponseArgs{Accept: true, Family: request.Family}
			if request.Family == ppp.AddressFamilyIPv4 {
				args.Local, args.Peer = netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")
			}
			if err := f.driver.IPResponse(request, args); err != nil {
				f.t.Errorf("answer the %s address request: %v", request.Family, err)
			}
		}
	}
}

// admit runs PADI, PADO, PADR and PADS through dispatchFrame, the function the
// discovery reader calls for every received frame.
func (f *serverV6Fixture) admit(t *testing.T) {
	t.Helper()
	var buffer [EthMaxLen]byte
	f.sub.dispatchFrame(f.server.ifIndex, BuildPADI(buffer[:], serverV6PeerMAC, "", nil))
	pado := f.discovered(t, CodePADO)
	padr := BuildPADR(buffer[:], serverV6PeerMAC, &pado, "", nil)
	f.sub.dispatchFrame(f.server.ifIndex, padr)
	pads := f.discovered(t, CodePADS)
	if pads.SID == 0 {
		t.Fatal("PADS refused the subscriber")
	}
	f.sid = pads.SID
	session := f.server.sessions.Lookup(f.sid)
	if session == nil {
		t.Fatal("admitted session is not in the table")
	}
	admission := f.server.sessions.admissionControl(session)
	if admission.opened == nil {
		t.Fatal("admitted session has no opened data plane")
	}
	f.attempt = admission.attempt
	f.identity = admission.opened.Dataplane().Identity()
	link, err := f.route.LinkByIndex(int(f.identity.NativeIndex))
	if err != nil {
		t.Fatalf("read the session's original unit: %v", err)
	}
	if link.Type() != "ppp" {
		t.Fatalf("session device kind=%q, want ppp", link.Type())
	}
	if link.Attrs().Name != f.identity.InterfaceName {
		t.Fatalf("session identity %s does not name its original unit %s", f.identity.InterfaceName, link.Attrs().Name)
	}
}

func (f *serverV6Fixture) discovered(t *testing.T, code byte) Packet {
	t.Helper()
	select {
	case frame := <-f.discovery:
		packet, err := ParseDiscovery(frame)
		if err != nil {
			t.Fatalf("parse the server's discovery frame: %v", err)
		}
		if packet.Code != code {
			t.Fatalf("server answered discovery code %#x, want %#x", packet.Code, code)
		}
		return packet
	default:
		t.Fatalf("server sent no discovery frame %#x", code)
	}
	return Packet{}
}

// requireIPv6Service waits for IPv6CP to open on both sides, for the session's
// link-local address to leave DAD, and for the carrier's ff02::2 and
// ff02::1:2 memberships on the original unit.
func (f *serverV6Fixture) requireIPv6Service(t *testing.T) {
	t.Helper()
	select {
	case id := <-f.peer.opened:
		var address [16]byte
		address[0], address[1] = 0xfe, 0x80
		copy(address[8:], id[:])
		f.localLL = netip.AddrFrom16(address)
	case <-time.After(serverV6Wait):
		t.Fatal("IPv6CP did not open with the subscriber")
	}
	index := int(f.identity.NativeIndex)
	serverV6Eventually(t, "the session link-local address", func() (bool, error) {
		link, err := f.route.LinkByIndex(index)
		if err != nil {
			return false, err
		}
		addresses, err := f.route.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			return false, err
		}
		for _, address := range addresses {
			if !address.IP.Equal(f.localLL.AsSlice()) {
				continue
			}
			return address.Flags&unix.IFA_F_TENTATIVE == 0, nil
		}
		return false, nil
	})
	serverV6Eventually(t, "the carrier's multicast memberships", func() (bool, error) {
		groups, err := serverV6Groups(index)
		if err != nil {
			return false, err
		}
		if !groups["ff020000000000000000000000000002"] {
			return false, nil
		}
		return groups["ff020000000000000000000000010002"], nil
	})
}

// releaseByPADT sends the subscriber's PADT through the production discovery
// handler, then waits until the admitted attempt reports its protocol run
// finished and its owner released with no error, and the driver holds no
// pending native release.
func (f *serverV6Fixture) releaseByPADT(t *testing.T) {
	t.Helper()
	var buffer [EthMaxLen]byte
	padt := BuildPADT(buffer[:], serverV6PeerMAC, serverV6AccessMAC, f.sid, "")
	if padt == nil {
		t.Fatal("build the subscriber's PADT")
	}
	f.sub.dispatchFrame(f.server.ifIndex, padt)

	serverV6Eventually(t, "the session's native release", func() (bool, error) {
		status, err := f.attempt.CleanupStatus()
		if err != nil {
			return false, err
		}
		if status.Err != nil {
			return false, fmt.Errorf("session cleanup failed: %w", status.Err)
		}
		if !status.Finished {
			return false, nil
		}
		if !status.Released {
			return false, nil
		}
		return !f.driver.HasPendingNativeRelease(), nil
	})
}

// serverV6Defaults sets the private namespace's IPv6 defaults before any
// device exists, so no kernel-generated address or solicitation appears.
func serverV6Defaults(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{"accept_dad": "0", "addr_gen_mode": "1", "router_solicitations": "0", "accept_ra": "0"} {
		if err := os.WriteFile("/proc/sys/net/ipv6/conf/default/"+key, []byte(value), 0); err != nil {
			t.Fatal(err)
		}
	}
}

func serverV6Veth(t *testing.T, route *netlink.Handle) *serverV6PeerWire {
	t.Helper()
	link := &netlink.Veth{
		Name: serverV6Access, HardwareAddr: serverV6AccessMAC[:],
		PeerName: serverV6Peer, PeerHardwareAddr: serverV6PeerMAC[:],
	}
	if err := route.LinkAdd(link); err != nil {
		t.Fatalf("create the access wire: %v", err)
	}
	access, err := route.LinkByName(serverV6Access)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := route.LinkDel(access); err != nil {
			t.Error(err)
		}
	})
	peer, err := route.LinkByName(serverV6Peer)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.LinkSetUp(access); err != nil {
		t.Fatal(err)
	}
	if err := route.LinkSetUp(peer); err != nil {
		t.Fatal(err)
	}
	protocol := serverV6Network(serverV6SessionEtherType)
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, int(protocol))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: protocol, Ifindex: peer.Attrs().Index}); err != nil {
		t.Fatal(err)
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_PACKET, unix.PACKET_IGNORE_OUTGOING, 1); err != nil {
		t.Fatal(err)
	}
	return &serverV6PeerWire{
		t: t, fd: fd, index: peer.Attrs().Index, ipv6: make(chan []byte, 256),
		opened: make(chan [8]byte, 1), v6Closed: make(chan struct{}, 1),
	}
}

// serverV6Capture opens a packet socket on the dummy that took the old name.
// Outgoing frames are kept: a carrier that writes by name would send there.
func serverV6Capture(t *testing.T, index int) int {
	t.Helper()
	protocol := serverV6Network(unix.ETH_P_ALL)
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, int(protocol))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: protocol, Ifindex: index}); err != nil {
		t.Fatal(err)
	}
	return fd
}

// serverV6RequireSilent refuses any Router Advertisement or DHCPv6 server
// message the dummy sent or received: what a carrier bound by name would put
// on it. The dummy's own IPv6 (DAD, MLD, Router Solicitation) is not refused.
func serverV6RequireSilent(t *testing.T, fd int) {
	t.Helper()
	var buffer [2048]byte
	// Bounded by the socket's queue: each read consumes one queued frame.
	for {
		n, _, err := unix.Recvfrom(fd, buffer[:], 0)
		if errors.Is(err, unix.EAGAIN) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if n < 14 {
			continue
		}
		if binary.BigEndian.Uint16(buffer[12:14]) != serverV6IPv6EtherType {
			continue
		}
		if serverV6CarrierFrame(buffer[14:n]) {
			t.Fatalf("the dummy holding the old name carried the session's IPv6 service: %x", buffer[:n])
		}
	}
}

func serverV6Network(protocol uint16) uint16 {
	return binary.NativeEndian.Uint16([]byte{byte(protocol >> 8), byte(protocol)})
}

// serverV6Groups reads the IPv6 multicast memberships of one interface index.
func serverV6Groups(index int) (map[string]bool, error) {
	data, err := os.ReadFile("/proc/net/igmp6")
	if err != nil {
		return nil, err
	}
	groups := make(map[string]bool)
	want := strconv.Itoa(index)
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] != want {
			continue
		}
		groups[fields[2]] = true
	}
	return groups, nil
}

// serverV6Eventually polls ready until it answers true, fails or times out.
func serverV6Eventually(t *testing.T, what string, ready func() (bool, error)) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(serverV6Wait)
	// Bounded by the deadline: each turn reads native state once.
	for {
		done, err := ready()
		if err != nil {
			t.Fatalf("wait for %s: %v", what, err)
		}
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not happen within %s", what, serverV6Wait)
		}
		<-ticker.C
	}
}

// serverV6PeerWire is the subscriber: a scripted PPP peer on the far end of
// the access veth. It answers LCP and IPv6CP, Protocol-Rejects IPCP, and hands
// every IPv6 packet the session carries to the test.
type serverV6PeerWire struct {
	// t receives the trace of every control packet the subscriber sends or
	// receives. Its workers end before the test does.
	t         *testing.T
	fd, index int
	sid       uint16
	ipv6      chan []byte
	opened    chan [8]byte
	// v6Closed reports each IPv6CP Terminate-Ack the server sent.
	v6Closed chan struct{}

	nextID       uint8
	lcpRequested bool
	// papAsked records that the server's LCP Configure-Request named PAP,
	// so the subscriber authenticates once its own request is acknowledged.
	papAsked    bool
	papSent     bool
	v6Requested bool
	v6Acked     bool
	v6Peer      bool
	serverID    [8]byte
}

// serve owns the wire's reads until ctx ends. The caller MUST cancel ctx and
// wait for done before closing the socket.
func (p *serverV6PeerWire) serve(ctx context.Context, t *testing.T, done chan<- struct{}) {
	defer close(done)
	var buffer [2048]byte
	// One worker per session; it ends when the test cancels ctx.
	for ctx.Err() == nil {
		fds := []unix.PollFd{{Fd: int32(p.fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(fds, 50)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Errorf("subscriber wire poll: %v", err)
			return
		}
		if ready == 0 {
			continue
		}
		n, _, err := unix.Recvfrom(p.fd, buffer[:], 0)
		if errors.Is(err, unix.EAGAIN) {
			continue
		}
		if err != nil {
			t.Errorf("subscriber wire read: %v", err)
			return
		}
		proto, info, ok := p.parse(buffer[:n])
		if !ok {
			continue
		}
		if err := p.handle(t, proto, info); err != nil {
			t.Errorf("subscriber PPP peer: %v", err)
			return
		}
	}
}

func (p *serverV6PeerWire) parse(frame []byte) (uint16, []byte, bool) {
	if len(frame) < 22 {
		return 0, nil, false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != serverV6SessionEtherType {
		return 0, nil, false
	}
	if !bytes.Equal(frame[6:12], serverV6AccessMAC[:]) {
		return 0, nil, false
	}
	if frame[14] != 0x11 {
		return 0, nil, false
	}
	if frame[15] != 0 {
		return 0, nil, false
	}
	if binary.BigEndian.Uint16(frame[16:18]) != p.sid {
		return 0, nil, false
	}
	length := int(binary.BigEndian.Uint16(frame[18:20]))
	if length < 2 {
		return 0, nil, false
	}
	if 20+length > len(frame) {
		return 0, nil, false
	}
	return binary.BigEndian.Uint16(frame[20:22]), frame[22 : 20+length], true
}

func (p *serverV6PeerWire) handle(t *testing.T, proto uint16, info []byte) error {
	if proto != ppp.ProtoIPv6 {
		t.Logf("subscriber received %s", serverV6Trace(proto, info))
	}
	switch proto {
	case ppp.ProtoLCP:
		return p.handleLCP(info)
	case ppp.ProtoIPv6CP:
		return p.handleIPv6CP(info)
	case ppp.ProtoPAP:
		return serverV6PAPReply(info)
	case ppp.ProtoIPCP:
		// RFC 1661 Section 5.7: Protocol-Reject carries the rejected protocol
		// and the rejected packet's information field.
		data := make([]byte, 2+len(info))
		binary.BigEndian.PutUint16(data, ppp.ProtoIPCP)
		copy(data[2:], info)
		return p.sendControl(ppp.ProtoLCP, ppp.LCPProtocolReject, p.id(), data)
	case ppp.ProtoIPv6:
		select {
		case p.ipv6 <- bytes.Clone(info):
		default:
			t.Error("subscriber IPv6 queue is full; a session packet was dropped")
		}
	}
	return nil
}

func (p *serverV6PeerWire) handleLCP(info []byte) error {
	packet, err := ppp.ParseLCPPacket(info)
	if err != nil {
		return err
	}
	switch packet.Code {
	case ppp.LCPConfigureRequest:
		asked, err := serverV6AsksPAP(packet.Data)
		if err != nil {
			return err
		}
		if err := p.sendControl(ppp.ProtoLCP, ppp.LCPConfigureAck, packet.Identifier, packet.Data); err != nil {
			return err
		}
		p.papAsked = asked
		if p.lcpRequested {
			return nil
		}
		p.lcpRequested = true
		// RFC 2516 Section 7: a PPPoE peer MUST NOT request an MRU above 1492.
		options := []byte{ppp.LCPOptMRU, 4, 0x05, 0xd4, ppp.LCPOptMagic, 6, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(options[6:], serverV6Magic)
		return p.sendControl(ppp.ProtoLCP, ppp.LCPConfigureRequest, p.id(), options)
	case ppp.LCPConfigureAck:
		// The subscriber acknowledged the server's request before sending its
		// own, so this Ack opens LCP on both sides.
		if !p.papAsked {
			return nil
		}
		if p.papSent {
			return nil
		}
		p.papSent = true
		return p.sendControl(ppp.ProtoPAP, ppp.PAPAuthenticateRequest, p.id(), serverV6PAPCredentials())
	case ppp.LCPConfigureNak, ppp.LCPConfigureReject:
		return fmt.Errorf("server refused the subscriber's LCP options: code %d %x", packet.Code, packet.Data)
	case ppp.LCPEchoRequest:
		if len(packet.Data) < 4 {
			return errors.New("LCP Echo-Request without a magic number")
		}
		reply := bytes.Clone(packet.Data)
		binary.BigEndian.PutUint32(reply, serverV6Magic)
		return p.sendControl(ppp.ProtoLCP, ppp.LCPEchoReply, packet.Identifier, reply)
	case ppp.LCPTerminateRequest:
		return p.sendControl(ppp.ProtoLCP, ppp.LCPTerminateAck, packet.Identifier, nil)
	}
	return nil
}

func (p *serverV6PeerWire) handleIPv6CP(info []byte) error {
	packet, err := ppp.ParseLCPPacket(info)
	if err != nil {
		return err
	}
	switch packet.Code {
	case ppp.LCPConfigureRequest:
		// RFC 5072 Section 4.1: Interface-Identifier, type 1, length 10.
		if len(packet.Data) != 10 {
			return fmt.Errorf("server IPv6CP request options=%x, want one Interface-Identifier", packet.Data)
		}
		if packet.Data[0] != 1 {
			return fmt.Errorf("server IPv6CP option type %d, want Interface-Identifier", packet.Data[0])
		}
		if packet.Data[1] != 10 {
			return fmt.Errorf("server IPv6CP option length %d, want 10", packet.Data[1])
		}
		copy(p.serverID[:], packet.Data[2:])
		if err := p.sendControl(ppp.ProtoIPv6CP, ppp.LCPConfigureAck, packet.Identifier, packet.Data); err != nil {
			return err
		}
		p.v6Peer = true
		if !p.v6Requested {
			p.v6Requested = true
			option := append([]byte{1, 10}, serverV6PeerID[:]...)
			if err := p.sendControl(ppp.ProtoIPv6CP, ppp.LCPConfigureRequest, p.id(), option); err != nil {
				return err
			}
		}
	case ppp.LCPConfigureAck:
		p.v6Acked = true
	case ppp.LCPConfigureNak, ppp.LCPConfigureReject:
		return fmt.Errorf("server refused the subscriber's Interface-Identifier: code %d %x", packet.Code, packet.Data)
	case ppp.LCPTerminateAck:
		select {
		case p.v6Closed <- struct{}{}:
		default:
			// An earlier Terminate-Ack is still unread; one report suffices.
		}
		return nil
	}
	if !p.v6Peer {
		return nil
	}
	if !p.v6Acked {
		return nil
	}
	select {
	case p.opened <- p.serverID:
	default:
		// IPv6CP Opened was already reported; a renegotiation reports nothing.
	}
	return nil
}

func (p *serverV6PeerWire) id() uint8 {
	p.nextID++
	return p.nextID
}

func (p *serverV6PeerWire) sendControl(proto uint16, code, id uint8, data []byte) error {
	var packet [1500]byte
	n := ppp.WriteLCPPacket(packet[:], 0, code, id, data)
	p.t.Logf("subscriber sent %s", serverV6Trace(proto, packet[:n]))
	return p.send(proto, packet[:n])
}

// send writes one PPPoE Session-stage frame to the access concentrator.
// Safe for concurrent use: each call is one sendto.
func (p *serverV6PeerWire) send(proto uint16, info []byte) error {
	frame := make([]byte, 22+len(info))
	copy(frame[0:6], serverV6AccessMAC[:])
	copy(frame[6:12], serverV6PeerMAC[:])
	binary.BigEndian.PutUint16(frame[12:14], serverV6SessionEtherType)
	frame[14] = 0x11
	binary.BigEndian.PutUint16(frame[16:18], p.sid)
	binary.BigEndian.PutUint16(frame[18:20], uint16(2+len(info)))
	binary.BigEndian.PutUint16(frame[20:22], proto)
	copy(frame[22:], info)
	address := &unix.SockaddrLinklayer{Ifindex: p.index, Protocol: serverV6Network(serverV6SessionEtherType), Halen: 6}
	copy(address.Addr[:], serverV6AccessMAC[:])
	return unix.Sendto(p.fd, frame, 0, address)
}

func (p *serverV6PeerWire) sendIPv6(t *testing.T, packet []byte) {
	t.Helper()
	if err := p.send(ppp.ProtoIPv6, packet); err != nil {
		t.Fatal(err)
	}
}

func (p *serverV6PeerWire) drainIPv6() {
	// Bounded by the queue's capacity.
	for {
		select {
		case <-p.ipv6:
		default:
			return
		}
	}
}

type serverV6Observed struct {
	source      netip.Addr
	destination netip.Addr
	next        byte
	hop         byte
	body        []byte
}

func (p *serverV6PeerWire) receiveIPv6(t *testing.T, deadline time.Time, what string) serverV6Observed {
	t.Helper()
	// Bounded by the deadline.
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("the session never carried %s", what)
		}
		var packet []byte
		select {
		case packet = <-p.ipv6:
		case <-time.After(remaining):
			t.Fatalf("the session never carried %s", what)
		}
		if len(packet) < 40 {
			continue
		}
		if packet[0]>>4 != 6 {
			continue
		}
		length := int(binary.BigEndian.Uint16(packet[4:6]))
		if 40+length > len(packet) {
			continue
		}
		return serverV6Observed{
			source:      netip.AddrFrom16([16]byte(packet[8:24])),
			destination: netip.AddrFrom16([16]byte(packet[24:40])),
			next:        packet[6], hop: packet[7], body: packet[40 : 40+length],
		}
	}
}

// serverV6CarrierFrame reports whether an IPv6 packet is a Router
// Advertisement or a DHCPv6 server message (source port 547): the two kinds
// the session's carrier sends. The carrier adds no extension headers.
func serverV6CarrierFrame(packet []byte) bool {
	if len(packet) < 40 {
		return false
	}
	body := packet[40:]
	switch packet[6] {
	case 58:
		return len(body) > 0 && body[0] == 134
	case 17:
		return len(body) >= 2 && binary.BigEndian.Uint16(body[0:2]) == 547
	}
	return false
}

func serverV6IsRA(packet serverV6Observed) bool {
	if packet.next != 58 {
		return false
	}
	if len(packet.body) < 16 {
		return false
	}
	return packet.body[0] == 134
}

// serverV6IsDHCPReply matches the Reply to serverV6Solicit(marker). With no
// prefix pool the session's server answers RFC 3633 NoPrefixAvail (6).
func serverV6IsDHCPReply(packet serverV6Observed, marker byte) bool {
	if packet.next != 17 {
		return false
	}
	if len(packet.body) < 12 {
		return false
	}
	if binary.BigEndian.Uint16(packet.body[0:2]) != 547 {
		return false
	}
	if binary.BigEndian.Uint16(packet.body[2:4]) != 546 {
		return false
	}
	message := packet.body[8:]
	if message[0] != ppp.DHCPv6Reply {
		return false
	}
	if !bytes.Equal(message[1:4], []byte{0x5a, 0x65, marker}) {
		return false
	}
	// Bounded by the message length: each option advances by its own length.
	for options := message[4:]; len(options) >= 4; {
		code := binary.BigEndian.Uint16(options[0:2])
		size := int(binary.BigEndian.Uint16(options[2:4]))
		if 4+size > len(options) {
			return false
		}
		if code == ppp.D6OptStatusCode {
			if size < 2 {
				return false
			}
			return binary.BigEndian.Uint16(options[4:6]) == ppp.D6StatusNoPrefixAvail
		}
		options = options[4+size:]
	}
	return false
}

// serverV6RS is an RFC 4861 Section 4.1 Router Solicitation from the peer.
func serverV6RS() []byte {
	destination := netip.MustParseAddr("ff02::2")
	body := []byte{133, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(body[2:4], serverV6Checksum(serverV6PeerLL, destination, 58, body))
	return serverV6Packet(serverV6PeerLL, destination, 58, 255, body)
}

// serverV6Solicit is an RFC 8415 Solicit asking for a delegated prefix.
func serverV6Solicit(marker byte) []byte {
	destination := netip.MustParseAddr("ff02::1:2")
	message := []byte{1, 0x5a, 0x65, marker}
	// Client Identifier: DUID-LL, Ethernet, the subscriber MAC.
	message = append(message, 0, 1, 0, 10, 0, 3, 0, 1)
	message = append(message, serverV6PeerMAC[:]...)
	// IA_PD: IAID 1, T1 0, T2 0. Elapsed Time 0.
	message = append(message, 0, 25, 0, 12, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8, 0, 2, 0, 0)
	body := make([]byte, 8+len(message))
	binary.BigEndian.PutUint16(body[0:2], 546)
	binary.BigEndian.PutUint16(body[2:4], 547)
	binary.BigEndian.PutUint16(body[4:6], uint16(len(body)))
	copy(body[8:], message)
	checksum := serverV6Checksum(serverV6PeerLL, destination, 17, body)
	if checksum == 0 {
		checksum = 0xffff
	}
	binary.BigEndian.PutUint16(body[6:8], checksum)
	return serverV6Packet(serverV6PeerLL, destination, 17, 1, body)
}

func serverV6Packet(source, destination netip.Addr, next, hop byte, body []byte) []byte {
	packet := make([]byte, 40+len(body))
	packet[0] = 0x60
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	packet[6], packet[7] = next, hop
	src, dst := source.As16(), destination.As16()
	copy(packet[8:24], src[:])
	copy(packet[24:40], dst[:])
	copy(packet[40:], body)
	return packet
}

// serverV6Checksum is the RFC 8200 Section 8.1 upper-layer checksum.
func serverV6Checksum(source, destination netip.Addr, next byte, body []byte) uint16 {
	src, dst := source.As16(), destination.As16()
	sum := uint32(len(body)) + uint32(next)
	for offset := 0; offset < 16; offset += 2 {
		sum += uint32(binary.BigEndian.Uint16(src[offset:]))
		sum += uint32(binary.BigEndian.Uint16(dst[offset:]))
	}
	for offset := 0; offset+1 < len(body); offset += 2 {
		sum += uint32(binary.BigEndian.Uint16(body[offset:]))
	}
	if len(body)%2 != 0 {
		sum += uint32(body[len(body)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
