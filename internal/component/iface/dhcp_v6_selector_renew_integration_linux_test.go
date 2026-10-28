// VALIDATES: a DHCPv6 client started through the factory the interface plugin
// calls for a unit with dhcpv6 enabled (iface.SetDHCPClientFactory, installed by
// ifacedhcp) keeps its lease on the kernel device an os-name selector binds to a
// logical name that is not itself a device: one Rapid Commit Solicit, three
// Renews, a Rebind once the server stops answering Renew, then a Release on
// Stop, every exchange on a socket of its own on the selected device. The kernel
// address carries the lifetimes each Reply granted. It also validates two spec
// assumptions: A-4, the kernel runs duplicate address detection on the address
// the client installs (tentative, then clear), and A-5, a socket per exchange
// rebinds port 546 back to back without EADDRINUSE.
// PREVENTS: a renew path that opens its socket on the logical name, or keeps a
// device resolved once at start, so a selector-bound uplink loses its lease at
// T1; and a Reply whose lifetimes never reach the kernel, so the address
// expires under a lease the server keeps extending.
// METHOD: a veth pair. The client end stays in the test's namespace and is
// named only through the resolver mapping a config apply publishes. The server
// end moves to a namespace of its own, so every Reply crosses the veth instead
// of being delivered locally, and a Go server answers on port 547 there.

//go:build linux

package iface_test

import (
	"bytes"
	"errors"
	"net"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	_ "github.com/ze-software/ze/internal/plugins/iface/dhcp" // installs the DHCP client factory the interface plugin calls
	"github.com/ze-software/ze/internal/test/userns"
)

const (
	// v6sbLogical is the unit's logical name. No kernel device carries it, so
	// only the resolver mapping can lead the client to the veth.
	v6sbLogical = "uplinkv6"

	v6sbT1        = 2 * time.Second
	v6sbT2        = 3 * time.Second
	v6sbPreferred = 6 * time.Second
	v6sbValid     = 8 * time.Second
	v6sbRenews    = 3

	// v6sbWait bounds the gap between two client messages: T2, plus the up to
	// one second SOL_MAX_DELAY before the first Solicit, plus slack.
	v6sbWait = 8 * time.Second
	// v6sbKernelWait bounds the wait for the kernel to show an address change.
	v6sbKernelWait = 5 * time.Second
)

var (
	v6sbLeased     = net.ParseIP("2001:db8:6::50")
	v6sbServerDUID = &dhcpv6.DUIDLL{HWType: iana.HWTypeEthernet, LinkLayerAddr: net.HardwareAddr{0x02, 0, 0, 0, 0x06, 0x01}}
	v6sbPeerLL     = net.ParseIP("fe80::1")
	v6sbServers    = &net.UDPAddr{IP: net.ParseIP("ff02::1:2"), Port: dhcpv6.DefaultServerPort}
)

// v6sbLabSeq keeps two labs in one run from naming the same link.
var v6sbLabSeq atomic.Int32

func TestDHCPv6RenewsOnSelectorBoundInterface(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	device, server := v6sbSetupLab(t)
	if _, err := net.InterfaceByName(v6sbLogical); err == nil {
		t.Fatalf("a kernel device is named %s: the test needs a logical name only the selector resolves", v6sbLogical)
	}
	iface.SetResolverMappingForTest(map[string]string{v6sbLogical: device})
	t.Cleanup(func() { iface.SetResolverMappingForTest(nil) })

	factory := iface.DHCPClientFactoryForTest()
	if factory == nil {
		t.Fatal("no DHCP client factory is installed: the ifacedhcp blank import did not run its init")
	}
	bus := &v6sbBus{}
	client, err := factory(v6sbLogical, "0", bus, false, true, "", "", false, 0, "", 0)
	if err != nil {
		t.Fatalf("start the DHCPv6 client on %s: %v", v6sbLogical, err)
	}
	var stopOnce sync.Once
	stop := func() { stopOnce.Do(client.Stop) }
	t.Cleanup(stop)

	solicit := server.await(t, "the Solicit", dhcpv6.MessageTypeSolicit)
	clientID := solicit.msg.Options.ClientID()
	if clientID == nil {
		t.Fatal("the Solicit carries no Client Identifier")
	}
	server.replyLease(t, solicit)
	v6sbRequireDAD(t, device)

	for i := range v6sbRenews {
		label := "Renew " + strconv.Itoa(i+1)
		renew := server.await(t, label, dhcpv6.MessageTypeRenew)
		v6sbRequireLeaseRequest(t, label, renew.msg, clientID, true)
		// The Renew leaves at T1 after the last Reply, so the kernel lifetime
		// left shows whether that Reply refreshed it.
		v6sbRequireLifetimes(t, device, label, v6sbT1)
		server.replyLease(t, renew)
	}

	// The server stops answering Renew. Only a Rebind at T2 keeps the lease.
	rebind := server.await(t, "the Rebind", dhcpv6.MessageTypeRebind, dhcpv6.MessageTypeRenew)
	v6sbRequireLeaseRequest(t, "the Rebind", rebind.msg, clientID, false)
	v6sbRequireLifetimes(t, device, "the Rebind", v6sbT2)
	server.replyLease(t, rebind)
	v6sbRequireRefreshed(t, device)

	stopped := make(chan struct{})
	go func() {
		stop()
		close(stopped)
	}()
	release := server.await(t, "the Release", dhcpv6.MessageTypeRelease)
	v6sbRequireLeaseRequest(t, "the Release", release.msg, clientID, true)
	server.replyRelease(t, release)
	select {
	case <-stopped:
	case <-time.After(v6sbWait):
		t.Fatal("Stop did not return after the Release was answered")
	}
	if a := v6sbFindAddress(t, device); a != nil {
		t.Fatalf("%s still holds %s after Stop", device, v6sbLeased)
	}

	acquired, renewed := bus.counts()
	if acquired != 1 {
		t.Fatalf("dhcp-acquired events naming %s = %d, want 1", v6sbLogical, acquired)
	}
	if renewed < v6sbRenews+1 {
		t.Fatalf("dhcp-renewed events naming %s = %d, want at least %d (three Renews and the Rebind)", v6sbLogical, renewed, v6sbRenews+1)
	}
}

// v6sbSetupLab builds the veth pair and returns the client's kernel device and
// the server listening on the other end. The client end keeps the test's
// namespace, because the client hands its work to goroutines the runtime puts
// on any thread; only the server's socket is opened from a thread inside the
// peer namespace, and a socket stays in the namespace it was created in.
func v6sbSetupLab(t *testing.T) (string, *v6sbServer) {
	t.Helper()
	device, _, _, server := v6sbSetupLabNS(t)
	return device, server
}

// v6sbSetupLabNS is v6sbSetupLab, also answering the peer end's name and the
// namespace it lives in, for a test that configures the peer.
func v6sbSetupLabNS(t *testing.T) (string, string, netns.NsHandle, *v6sbServer) {
	t.Helper()
	if iface.GetBackend() == nil {
		if err := iface.LoadBackend("netlink"); err != nil {
			t.Fatalf("load iface backend: %v", err)
		}
		t.Cleanup(func() { _ = iface.CloseBackend() })
	}

	suffix := (os.Getpid() % 1000) + int(v6sbLabSeq.Add(1))*1000
	device := "zev6sz" + strconv.Itoa(suffix)
	peer := "zev6sp" + strconv.Itoa(suffix)
	veth := &netlink.Veth{Name: device, PeerName: peer}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("add veth %s/%s in the test's namespace: %v", device, peer, err)
	}
	// Deleting one end deletes the pair.
	t.Cleanup(func() { _ = netlink.LinkDel(veth) })

	// A-4 rests on the kernel running DAD on this device, so the lab asks for
	// it rather than inheriting whatever the host's default is.
	dad := "/proc/sys/net/ipv6/conf/" + device + "/accept_dad"
	if err := os.WriteFile(dad, []byte("1"), 0o644); err != nil {
		t.Fatalf("enable DAD on %s: %v", device, err)
	}

	peerNS := v6sbMovePeer(t, peer)
	server := v6sbListen(t, peerNS, peer)

	link, err := netlink.LinkByName(device)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", device, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("LinkSetUp(%s): %v", device, err)
	}
	// The client's messages leave from the device's link-local address, which
	// the kernel does not use while DAD holds it tentative.
	v6sbAwaitLinkLocal(t, link)
	return device, peer, peerNS, server
}

// v6sbMovePeer moves the peer end into a new namespace, brings it up, and gives
// it a link-local address exempt from DAD, so the server answers at once.
func v6sbMovePeer(t *testing.T, peer string) netns.NsHandle {
	t.Helper()
	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("current namespace: %v", err)
	}
	peerNS, err := netns.New()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("new network namespace for the peer end %s: %v", peer, err)
	}
	if err := netns.Set(origin); err != nil {
		// The thread stays locked: a thread left in the wrong namespace dies
		// with the goroutine rather than serve another one.
		t.Fatalf("return to the test namespace: %v", err)
	}
	runtime.UnlockOSThread()
	_ = origin.Close()
	t.Cleanup(func() { _ = peerNS.Close() })

	link, err := netlink.LinkByName(peer)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", peer, err)
	}
	if err := netlink.LinkSetNsFd(link, int(peerNS)); err != nil {
		t.Fatalf("move %s into the peer namespace: %v", peer, err)
	}
	h, err := netlink.NewHandleAt(peerNS)
	if err != nil {
		t.Fatalf("netlink handle in the peer namespace: %v", err)
	}
	defer h.Close()
	link, err = h.LinkByName(peer)
	if err != nil {
		t.Fatalf("LinkByName(%s) in the peer namespace: %v", peer, err)
	}
	addr := &netlink.Addr{
		IPNet: &net.IPNet{IP: v6sbPeerLL, Mask: net.CIDRMask(64, 128)},
		Flags: unix.IFA_F_NODAD,
	}
	if err := h.AddrAdd(link, addr); err != nil {
		t.Fatalf("add %s to %s: %v", v6sbPeerLL, peer, err)
	}
	if err := h.LinkSetUp(link); err != nil {
		t.Fatalf("LinkSetUp(%s) in the peer namespace: %v", peer, err)
	}
	return peerNS
}

// v6sbListen opens the server socket from a thread inside the peer namespace:
// joined to All_DHCP_Relay_Agents_and_Servers on the peer end, port 547.
func v6sbListen(t *testing.T, peerNS netns.NsHandle, peer string) *v6sbServer {
	t.Helper()
	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("current namespace: %v", err)
	}
	if err := netns.Set(peerNS); err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("enter the peer namespace: %v", err)
	}
	ifc, ierr := net.InterfaceByName(peer)
	var conn *net.UDPConn
	lerr := ierr
	if ierr == nil {
		conn, lerr = net.ListenMulticastUDP("udp6", ifc, v6sbServers)
	}
	if err := netns.Set(origin); err != nil {
		t.Fatalf("return to the test namespace: %v", err)
	}
	runtime.UnlockOSThread()
	_ = origin.Close()
	if lerr != nil {
		t.Fatalf("server socket on %s in the peer namespace: %v", peer, lerr)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &v6sbServer{conn: conn, zone: strconv.Itoa(ifc.Index)}
}

// v6sbAwaitLinkLocal waits until the device holds a link-local address DAD
// has cleared.
func v6sbAwaitLinkLocal(t *testing.T, link netlink.Link) {
	t.Helper()
	deadline := time.Now().Add(v6sbKernelWait)
	for time.Now().Before(deadline) {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			t.Fatalf("AddrList(%s): %v", link.Attrs().Name, err)
		}
		for _, a := range addrs {
			if a.IP.IsLinkLocalUnicast() && a.Flags&unix.IFA_F_TENTATIVE == 0 {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s holds no usable link-local address after %v", link.Attrs().Name, v6sbKernelWait)
}

// v6sbServer is the Go DHCPv6 server on the peer end. It answers only what
// the test tells it to, so the test decides when the client must Rebind.
type v6sbServer struct {
	conn *net.UDPConn
	zone string // the peer end's ifindex inside its namespace
}

// v6sbReceived is one client message and where to answer it.
type v6sbReceived struct {
	msg  *dhcpv6.Message
	from *net.UDPAddr
}

// await returns the next client message of type want. A message of a type in
// skip is read and left unanswered; any other type fails the test, because the
// client sent something its lease state does not allow.
func (s *v6sbServer) await(t *testing.T, label string, want dhcpv6.MessageType, skip ...dhcpv6.MessageType) v6sbReceived {
	t.Helper()
	buf := make([]byte, 1500)
	deadline := time.Now().Add(v6sbWait)
	if err := s.conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("set the server read deadline: %v", err)
	}
	var skipped []string
	for {
		n, from, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				t.Fatalf("waiting for %s: no %s within %v (left unanswered: %s)", label, want, v6sbWait, strings.Join(skipped, ", "))
			}
			t.Fatalf("waiting for %s: read: %v", label, err)
		}
		msg, err := dhcpv6.MessageFromBytes(buf[:n])
		if err != nil {
			t.Fatalf("waiting for %s: undecodable client message from %v: %v", label, from, err)
		}
		if msg.MessageType == want {
			return v6sbReceived{msg: msg, from: from}
		}
		if !slices.Contains(skip, msg.MessageType) {
			t.Fatalf("waiting for %s: the client sent %s", label, msg.MessageType)
		}
		skipped = append(skipped, msg.MessageType.String())
	}
}

// replyLease answers with the one address, T1, T2 and the lifetimes the test
// checks the kernel against.
func (s *v6sbServer) replyLease(t *testing.T, req v6sbReceived) {
	t.Helper()
	ia := req.msg.Options.OneIANA()
	if ia == nil {
		t.Fatalf("the client's %s carries no IA_NA", req.msg.MessageType)
	}
	reply, err := dhcpv6.NewReplyFromMessage(req.msg, dhcpv6.WithServerID(v6sbServerDUID))
	if err != nil {
		t.Fatalf("build the Reply to %s: %v", req.msg.MessageType, err)
	}
	na := &dhcpv6.OptIANA{IaId: ia.IaId, T1: v6sbT1, T2: v6sbT2}
	na.Options.Add(&dhcpv6.OptIAAddress{IPv6Addr: v6sbLeased, PreferredLifetime: v6sbPreferred, ValidLifetime: v6sbValid})
	reply.AddOption(na)
	s.send(t, reply, req.from)
}

func (s *v6sbServer) replyRelease(t *testing.T, req v6sbReceived) {
	t.Helper()
	reply, err := dhcpv6.NewReplyFromMessage(req.msg, dhcpv6.WithServerID(v6sbServerDUID))
	if err != nil {
		t.Fatalf("build the Reply to the Release: %v", err)
	}
	s.send(t, reply, req.from)
}

// send answers on the peer end. The zone is the peer's ifindex in its own
// namespace: a zone NAME would be resolved in the test's namespace, where the
// peer end no longer exists.
func (s *v6sbServer) send(t *testing.T, reply *dhcpv6.Message, from *net.UDPAddr) {
	t.Helper()
	to := &net.UDPAddr{IP: from.IP, Port: from.Port, Zone: s.zone}
	if _, err := s.conn.WriteToUDP(reply.ToBytes(), to); err != nil {
		t.Fatalf("send the Reply to %v: %v", to, err)
	}
}

// v6sbRequireLeaseRequest checks that a Renew, Rebind or Release names the
// client, the leased address, and the server only where the exchange has one.
func v6sbRequireLeaseRequest(t *testing.T, label string, msg *dhcpv6.Message, clientID dhcpv6.DUID, wantServer bool) {
	t.Helper()
	if got := msg.Options.ClientID(); got == nil || !bytes.Equal(got.ToBytes(), clientID.ToBytes()) {
		t.Fatalf("%s Client Identifier = %v, want the Solicit's %v", label, got, clientID)
	}
	server := msg.Options.ServerID()
	if wantServer && (server == nil || !server.Equal(v6sbServerDUID)) {
		t.Fatalf("%s Server Identifier = %v, want %v", label, server, v6sbServerDUID)
	}
	if !wantServer && server != nil {
		t.Fatalf("%s carries Server Identifier %v; a Rebind goes to any server", label, server)
	}
	ia := msg.Options.OneIANA()
	if ia == nil {
		t.Fatalf("%s carries no IA_NA", label)
	}
	addr := ia.Options.OneAddress()
	if addr == nil || !addr.IPv6Addr.Equal(v6sbLeased) {
		t.Fatalf("%s IA_NA address = %v, want the leased %s", label, addr, v6sbLeased)
	}
}

// v6sbRequireDAD waits for the leased address and requires the kernel to hold
// it tentative first and then clear it: DAD ran, and it passed.
func v6sbRequireDAD(t *testing.T, device string) {
	t.Helper()
	deadline := time.Now().Add(v6sbKernelWait)
	var first *netlink.Addr
	for first == nil && time.Now().Before(deadline) {
		first = v6sbFindAddress(t, device)
		if first == nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if first == nil {
		t.Fatalf("%s never received %s after the Solicit's Reply", device, v6sbLeased)
	}
	if first.Flags&unix.IFA_F_TENTATIVE == 0 {
		t.Fatalf("%s first showed %s with flags %#x, not tentative: the kernel ran no duplicate address detection", device, v6sbLeased, first.Flags)
	}
	for time.Now().Before(deadline) {
		a := v6sbFindAddress(t, device)
		if a == nil {
			t.Fatalf("%s lost %s during duplicate address detection", device, v6sbLeased)
		}
		if a.Flags&unix.IFA_F_DADFAILED != 0 {
			t.Fatalf("duplicate address detection failed for %s on %s", v6sbLeased, device)
		}
		if a.Flags&unix.IFA_F_TENTATIVE == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s is still tentative on %s after %v", v6sbLeased, device, v6sbKernelWait)
}

// v6sbRequireLifetimes checks the kernel lifetimes left when a client message
// arrives `since` after the last Reply. Without that Reply reaching the kernel,
// the lifetime left would be lower by a whole T1.
func v6sbRequireLifetimes(t *testing.T, device, label string, since time.Duration) {
	t.Helper()
	a := v6sbFindAddress(t, device)
	if a == nil {
		t.Fatalf("at %s, %s no longer holds %s", label, device, v6sbLeased)
	}
	validMin := int((v6sbValid - since - time.Second) / time.Second)
	validMax := int(v6sbValid / time.Second)
	if a.ValidLft < validMin || a.ValidLft > validMax {
		t.Fatalf("at %s, %s valid lifetime left = %ds, want %d..%ds", label, v6sbLeased, a.ValidLft, validMin, validMax)
	}
	if a.PreferedLft > int(v6sbPreferred/time.Second) {
		t.Fatalf("at %s, %s preferred lifetime left = %ds, above the %v granted", label, v6sbLeased, a.PreferedLft, v6sbPreferred)
	}
}

// v6sbRequireRefreshed waits for the Rebind's Reply to restore the full valid
// lifetime on the kernel address.
func v6sbRequireRefreshed(t *testing.T, device string) {
	t.Helper()
	want := int((v6sbValid - time.Second) / time.Second)
	deadline := time.Now().Add(v6sbKernelWait)
	last := -1
	for time.Now().Before(deadline) {
		a := v6sbFindAddress(t, device)
		if a == nil {
			t.Fatalf("%s lost %s after the Rebind's Reply", device, v6sbLeased)
		}
		last = a.ValidLft
		if last >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("after the Rebind's Reply the valid lifetime left on %s is %ds, want at least %ds", device, last, want)
}

// v6sbFindAddress returns the leased address as the kernel holds it on the
// device, or nil when the device does not hold it.
func v6sbFindAddress(t *testing.T, device string) *netlink.Addr {
	t.Helper()
	link, err := netlink.LinkByName(device)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", device, err)
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		t.Fatalf("AddrList(%s): %v", device, err)
	}
	for i := range addrs {
		if addrs[i].IP.Equal(v6sbLeased) {
			return &addrs[i]
		}
	}
	return nil
}

// v6sbBus counts the lease events the client publishes for the logical name.
type v6sbBus struct {
	mu       sync.Mutex
	acquired int
	renewed  int
}

func (b *v6sbBus) Emit(_, eventType string, payload any) (int, error) {
	s, ok := payload.(string)
	if !ok || !strings.Contains(s, `"`+v6sbLogical+`"`) {
		return 0, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch eventType {
	case ifaceevents.EventDHCPAcquired:
		b.acquired++
	case ifaceevents.EventDHCPRenewed:
		b.renewed++
	}
	return 0, nil
}

func (b *v6sbBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

func (b *v6sbBus) counts() (acquired, renewed int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.acquired, b.renewed
}
