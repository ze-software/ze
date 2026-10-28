// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: dhcp_v6_selector_renew_integration_linux_test.go -- the veth lab and the Go DHCPv6 server this test reuses
// Related: export_test.go -- BindResolverEventsForTest, the binding a config apply makes
// RFC: rfc/short/rfc9915.md

//go:build linux

package iface_test

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

const (
	// v6lmLong is T1 for the lease of these tests, past their whole run, so
	// every message the server reads after the Reply comes from a link event.
	v6lmLong = 300 * time.Second
	// v6lmQuiet is how long the server listens for a message that must not
	// come: three times CNF_MAX_DELAY, the longest a link-change Rebind waits.
	v6lmQuiet = 3 * time.Second
	// v6lmRebindWait bounds the gap between the up and the Rebind:
	// CNF_MAX_DELAY plus slack.
	v6lmRebindWait = 3 * time.Second
)

var v6lmPrefix = &net.IPNet{IP: net.ParseIP("2001:db8:60::"), Mask: net.CIDRMask(56, 128)}

// TestRFC9915LinkMoveRebindsThroughTheMonitor flaps the carrier of the
// client's veth while the lease holds a delegated prefix.
//
// VALIDATES: AC-28 through the real path: the kernel's RTM_NEWLINK, the netlink
// monitor, the event bus, the resolver remapping the device to the logical
// name, iface.Subscribe, and the v6 worker. Down then up on the device starts a
// Rebind, within CNF_MAX_DELAY plus slack, carrying the leased address and
// the delegated prefix and no Server Identifier.
// PREVENTS: a link-change Rebind that works against injected events but never
// fires on a real carrier loss.
func TestRFC9915LinkMoveRebindsThroughTheMonitor(t *testing.T) {
	// RFC requirement: RFC9915-18.2.12-2 positive -- with a delegated prefix held, a carrier loss then return on the bound device, reported by the kernel through the netlink monitor, starts a Rebind carrying the IA_NA and IA_PD leases.
	if !userns.Enter(t) {
		return
	}
	lab := v6lmLease(t)

	h, err := netlink.NewHandleAt(lab.peerNS)
	if err != nil {
		t.Fatalf("netlink handle in the peer namespace: %v", err)
	}
	defer h.Close()
	peer, err := h.LinkByName(lab.peer)
	if err != nil {
		t.Fatalf("LinkByName(%s) in the peer namespace: %v", lab.peer, err)
	}
	if err := h.LinkSetDown(peer); err != nil {
		t.Fatalf("take %s down: %v", lab.peer, err)
	}
	lab.awaitLink(t, iface.LinkDown)
	if err := h.LinkSetUp(peer); err != nil {
		t.Fatalf("bring %s back up: %v", lab.peer, err)
	}
	lab.awaitLink(t, iface.LinkUp)
	up := time.Now()

	rebind := lab.server.await(t, "the link-change Rebind", dhcpv6.MessageTypeRebind)
	if waited := time.Since(up); waited > v6lmRebindWait {
		t.Errorf("the Rebind came %v after the up, want at most %v", waited, v6lmRebindWait)
	}
	v6sbRequireLeaseRequest(t, "the link-change Rebind", rebind.msg, lab.clientID, false)
	pds := rebind.msg.Options.IAPD()
	if len(pds) != 1 {
		t.Fatalf("the Rebind carries %d IA_PD, want 1", len(pds))
	}
	prefixes := pds[0].Options.Prefixes()
	if len(prefixes) != 1 || prefixes[0].Prefix.String() != v6lmPrefix.String() {
		t.Errorf("the Rebind's IA_PD prefixes %v, want only %v", prefixes, v6lmPrefix)
	}
}

// TestRFC9915LinkChangeWithoutDownSendsNothing changes the MTU of the client's
// veth while the lease holds a delegated prefix. The kernel answers with an
// RTM_NEWLINK, which the monitor reports as an up with no down before it.
//
// VALIDATES: AC-28, the negative half, through the same real path as the test
// above: the up reaches the logical name's subscribers, and the client sends
// nothing.
// PREVENTS: a client that reads every repeated up as a move and Rebinds on
// each link attribute change.
func TestRFC9915LinkChangeWithoutDownSendsNothing(t *testing.T) {
	// RFC requirement: RFC9915-18.2.12-2 negative -- an up the netlink monitor reports for a link change with no disconnection before it starts no Rebind, although a delegated prefix is held.
	if !userns.Enter(t) {
		return
	}
	lab := v6lmLease(t)

	link, err := netlink.LinkByName(lab.device)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", lab.device, err)
	}
	if err := netlink.LinkSetMTU(link, 1400); err != nil {
		t.Fatalf("set the MTU of %s: %v", lab.device, err)
	}
	lab.awaitLink(t, iface.LinkUp)
	lab.server.awaitNothing(t, "after an up with no down before it", v6lmQuiet)
}

// v6lmLab is one leased client whose link events travel the real chain.
type v6lmLab struct {
	device   string
	peer     string
	peerNS   netns.NsHandle
	server   *v6sbServer
	clientID dhcpv6.DUID
	links    <-chan iface.LinkEvent // the test's own subscription to the logical name
}

// v6lmLease builds the veth lab, starts the backend's monitor on a bus the
// resolver is bound to, starts the client under the logical name with a
// delegated prefix requested, and answers its Solicit with an address and a
// prefix whose T1 outlasts the test.
func v6lmLease(t *testing.T) *v6lmLab {
	t.Helper()
	device, peer, peerNS, server := v6sbSetupLabNS(t)
	iface.SetResolverMappingForTest(map[string]string{v6sbLogical: device})
	t.Cleanup(func() { iface.SetResolverMappingForTest(nil) })

	bus := newV6lmBus()
	t.Cleanup(iface.BindResolverEventsForTest(bus))
	backend := iface.GetBackend()
	if err := backend.StartMonitor(bus); err != nil {
		t.Fatalf("start the netlink monitor: %v", err)
	}
	t.Cleanup(backend.StopMonitor)

	links, cancel := iface.Subscribe(v6sbLogical)
	t.Cleanup(cancel)

	factory := iface.DHCPClientFactoryForTest()
	if factory == nil {
		t.Fatal("no DHCP client factory is installed: the ifacedhcp blank import did not run its init")
	}
	client, err := factory(v6sbLogical, "0", &v6sbBus{}, false, true, "", "", true, 56, "", 0)
	if err != nil {
		t.Fatalf("start the DHCPv6 client on %s: %v", v6sbLogical, err)
	}
	var stopOnce sync.Once
	t.Cleanup(func() { stopOnce.Do(client.Stop) })

	solicit := server.await(t, "the Solicit", dhcpv6.MessageTypeSolicit)
	if len(solicit.msg.Options.IAPD()) != 1 {
		t.Fatalf("the Solicit carries %d IA_PD, want 1: the client was started with a prefix requested", len(solicit.msg.Options.IAPD()))
	}
	server.replyLeasePD(t, solicit)
	lab := &v6lmLab{
		device: device, peer: peer, peerNS: peerNS, server: server,
		clientID: solicit.msg.Options.ClientID(), links: links,
	}
	// The lease is held once DAD has run on the address; no message is owed.
	v6sbRequireDAD(t, device)
	lab.drain()
	return lab
}

// replyLeasePD answers with the one address and one delegated prefix, both
// with T1 past the end of the test.
func (s *v6sbServer) replyLeasePD(t *testing.T, req v6sbReceived) {
	t.Helper()
	na := req.msg.Options.OneIANA()
	if na == nil {
		t.Fatalf("the client's %s carries no IA_NA", req.msg.MessageType)
	}
	pd := req.msg.Options.IAPD()[0]
	reply, err := dhcpv6.NewReplyFromMessage(req.msg, dhcpv6.WithServerID(v6sbServerDUID))
	if err != nil {
		t.Fatalf("build the Reply to %s: %v", req.msg.MessageType, err)
	}
	leasedNA := &dhcpv6.OptIANA{IaId: na.IaId, T1: v6lmLong, T2: 2 * v6lmLong}
	leasedNA.Options.Add(&dhcpv6.OptIAAddress{IPv6Addr: v6sbLeased, PreferredLifetime: 3 * v6lmLong, ValidLifetime: 4 * v6lmLong})
	reply.AddOption(leasedNA)
	leasedPD := &dhcpv6.OptIAPD{IaId: pd.IaId, T1: v6lmLong, T2: 2 * v6lmLong}
	leasedPD.Options.Add(&dhcpv6.OptIAPrefix{Prefix: v6lmPrefix, PreferredLifetime: 3 * v6lmLong, ValidLifetime: 4 * v6lmLong})
	reply.AddOption(leasedPD)
	s.send(t, reply, req.from)
}

// awaitNothing fails the test if the client sends any message within quiet.
func (s *v6sbServer) awaitNothing(t *testing.T, label string, quiet time.Duration) {
	t.Helper()
	if err := s.conn.SetReadDeadline(time.Now().Add(quiet)); err != nil {
		t.Fatalf("set the server read deadline: %v", err)
	}
	buf := make([]byte, 1500)
	n, from, err := s.conn.ReadFromUDP(buf)
	if err == nil {
		msg, decodeErr := dhcpv6.MessageFromBytes(buf[:n])
		if decodeErr != nil {
			t.Fatalf("%s: the client sent an undecodable message from %v: %v", label, from, decodeErr)
		}
		t.Fatalf("%s: the client sent %s, want nothing within %v", label, msg.MessageType, quiet)
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("%s: read: %v", label, err)
	}
}

// awaitLink waits for the test's own subscription to see kind for the logical
// name, which proves the event crossed the monitor, the bus and the resolver.
func (l *v6lmLab) awaitLink(t *testing.T, kind iface.LinkEventKind) {
	t.Helper()
	deadline := time.After(v6sbKernelWait)
	// The bound: the deadline, and the kernel sends a few events per change.
	for {
		select {
		case ev, ok := <-l.links:
			if !ok {
				t.Fatalf("the subscription to %s closed while waiting for %s", v6sbLogical, kind)
			}
			if ev.Kind == kind {
				return
			}
		case <-deadline:
			t.Fatalf("no %s event reached the subscribers of %s within %v", kind, v6sbLogical, v6sbKernelWait)
		}
	}
}

// drain empties the test's subscription of the events the lab setup caused.
func (l *v6lmLab) drain() {
	// The bound: the subscription buffer holds eight events.
	for {
		select {
		case <-l.links:
		default:
			return
		}
	}
}

// v6lmBus delivers each event to its subscribers on the emitter's goroutine,
// the contract of the engine's bus that the monitor and the resolver rely on.
// Safe for concurrent use.
type v6lmBus struct {
	mu   sync.Mutex
	next int
	subs map[string]map[int]func(any)
}

func newV6lmBus() *v6lmBus { return &v6lmBus{subs: make(map[string]map[int]func(any))} }

func (b *v6lmBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	handlers := make([]func(any), 0, len(b.subs[namespace+"/"+eventType]))
	for _, h := range b.subs[namespace+"/"+eventType] {
		handlers = append(handlers, h)
	}
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *v6lmBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[key] == nil {
		b.subs[key] = make(map[int]func(any))
	}
	id := b.next
	b.next++
	b.subs[key][id] = handler
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[key], id)
	}
}
