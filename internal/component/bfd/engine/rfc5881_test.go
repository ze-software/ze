// VALIDATES: RFC 5881 single-hop demultiplexing and transmit-destination
// rules. A first packet (Your Discriminator = 0) is bound to the session by
// remote address, ingress interface, and protocol; every subsequent packet is
// demuxed solely by Your Discriminator; a changed peer source address never
// becomes the transmit destination; and a separate session exists per protocol.
// PREVENTS: a first packet being accepted from the wrong source/interface, an
// established session being matched by source instead of discriminator, the
// transmit destination drifting to a spoofed source address, and two protocols
// collapsing onto one session.
package engine

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/session"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
)

// peerMyDiscr is the discriminator the synthetic peer stamps as its
// MyDiscriminator. Any nonzero value works; ParseControl rejects zero.
const peerMyDiscr uint32 = 0x2222

// captureTransport records the last Outbound handed to Send so a test can
// assert the transmit destination without binding a real socket. RX returns a
// nil channel: these tests never Start the loop, so the express-loop goroutine
// never runs and nothing drains it.
type captureTransport struct {
	last transport.Outbound
	sent bool
}

func (*captureTransport) Start() error { return nil }
func (*captureTransport) Stop() error  { return nil }
func (c *captureTransport) Send(o transport.Outbound) error {
	c.last = o
	c.sent = true
	return nil
}
func (*captureTransport) RX() <-chan transport.Inbound { return nil }

// inboundControl builds a wire-encoded single-hop Control packet wrapped in a
// transport.Inbound with TTL 255 (so passesTTLGate accepts it). yd is the
// Your Discriminator the packet carries.
func inboundControl(from, local netip.Addr, iface string, yd uint32) transport.Inbound {
	c := packet.Control{
		Version:               packet.Version,
		State:                 packet.StateDown,
		DetectMult:            3,
		Length:                packet.MandatoryLen,
		MyDiscriminator:       peerMyDiscr,
		YourDiscriminator:     yd,
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
	}
	buf := make([]byte, packet.MandatoryLen)
	c.WriteTo(buf, 0)
	return transport.Inbound{
		From:      from,
		Local:     local,
		Interface: iface,
		Mode:      api.SingleHop,
		TTL:       255,
		Bytes:     buf,
	}
}

// machineFor returns the session.Machine registered for key, failing the test
// if none exists.
func machineFor(t *testing.T, l *Loop, key api.Key) *session.Machine {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.sessions[key]
	if e == nil {
		t.Fatalf("no session registered for key %+v", key)
	}
	return e.machine
}

// newSingleHopLoop creates an unstarted Loop with one pinned single-hop
// session to addrB from addrA and returns the loop, its capture transport, and
// the session key.
func newSingleHopLoop(t *testing.T) (*Loop, *captureTransport, api.Key) {
	t.Helper()
	ct := &captureTransport{}
	l := NewLoop(ct, clock.RealClock{})
	req := reqFor(addrB, addrA)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	return l, ct, req.Key()
}

// RFC requirement: RFC5881-3-2 positive -- a received packet with Your
// Discriminator = 0 MUST be associated with the session bound to the remote
// address, ingress interface, and protocol. handleInbound
// (internal/component/bfd/engine/loop.go:96-102) builds firstPacketKey{peer:
// in.From, local, vrf, iface, mode} and finds the session via byKey, so a
// first packet whose tuple matches is delivered (RemoteDiscr becomes the
// peer's MyDiscriminator).
// RFC requirement: RFC5880-6.8.6-18 positive -- the same producer performs the
// zero-discriminator selection RFC 5880 Section 6.8.6 mandates. Source
// addressing information (peer, local) and the ingress interface are the
// "combination of other fields" the session is selected on. A packet carrying
// Your Discriminator = 0 therefore reaches the session bound to that tuple.
func TestRFC5881FirstPacketMatchesByTuple(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	in := inboundControl(key.Peer, key.Local, key.Interface, 0)
	l.handleInbound(in)

	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("first packet not associated with the session: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
}

// RFC requirement: RFC5881-3-2 negative -- the first-packet association is
// scoped to the bound tuple, not blanket-accept. A packet with Your
// Discriminator = 0 arriving from a DIFFERENT source address misses the byKey
// lookup (loop.go:104, entry == nil) and is dropped, so RemoteDiscr stays zero.
// Without this the positive test could pass on code that accepts any first
// packet regardless of source.
// RFC requirement: RFC5880-6.8.6-18 negative -- selection is ON the combination
// of fields, not despite it. Changing only the source address moves the packet
// off the tuple, so no session is selected. Without this, the positive passes
// on code that hands every zero-discriminator packet to the one session it
// holds, which selects on nothing.
func TestRFC5881FirstPacketWrongSourceDropped(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	wrong := netip.MustParseAddr("198.51.100.9")
	in := inboundControl(wrong, key.Local, key.Interface, 0)
	l.handleInbound(in)

	if got := m.RemoteDiscriminator(); got != 0 {
		t.Fatalf("first packet from wrong source was associated (RemoteDiscriminator = %d); it must be dropped", got)
	}
}

// RFC requirement: RFC5881-4-4 positive -- ultimately RFC 5880 mechanisms
// (Your Discriminator) demux incoming packets to the proper session.
// RFC requirement: RFC5881-6-5 positive -- once a discriminator is learned,
// subsequent packets are demuxed SOLELY by Your Discriminator. handleInbound
// (internal/component/bfd/engine/loop.go:82-83) looks the session up in byDiscr
// by c.YourDiscriminator, so a packet carrying the local discriminator is
// delivered even when its source differs from the configured peer.
func TestRFC5881DiscriminatorDemuxDelivers(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)
	localDiscr := m.LocalDiscriminator()

	// Source deliberately unequal to the configured peer to prove the
	// discriminator (not the source) is what demuxes an established session.
	other := netip.MustParseAddr("198.51.100.7")
	in := inboundControl(other, key.Local, key.Interface, localDiscr)
	l.handleInbound(in)

	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("packet with matching Your Discriminator was not delivered: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
}

// RFC requirement: RFC5881-4-4 negative -- a packet whose Your Discriminator
// matches no session is not associated with one.
// RFC requirement: RFC5881-6-5 negative -- demux is by discriminator alone, so
// a nonzero Your Discriminator that is unallocated (loop.go:83, byDiscr miss)
// is dropped even though its source equals the configured peer. Without this
// the positive could pass on code that fell back to source matching.
func TestRFC5881DiscriminatorDemuxUnknownDropped(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	const unallocated uint32 = 0x7777 // only discriminator 1 is allocated
	in := inboundControl(key.Peer, key.Local, key.Interface, unallocated)
	l.handleInbound(in)

	if got := m.RemoteDiscriminator(); got != 0 {
		t.Fatalf("packet with unknown Your Discriminator was associated (RemoteDiscriminator = %d); it must be dropped", got)
	}
}

// RFC requirement: RFC5881-6-6 positive -- when a received source address
// changes on a point-to-point link, the local system MUST continue using the
// destination configured at session creation. sendLocked
// (internal/component/bfd/engine/loop.go:232) always sends To:
// entry.machine.PeerAddr(), which returns the immutable configReq.Peer
// (internal/component/bfd/session/session.go:302), so after receiving from a
// changed source the transmit destination is still the configured peer.
// RFC requirement: RFC5881-6-2 positive -- the transmitted Control packet's
// destination is the operator-configured single-hop peer (on-subnet by config),
// the same sendLocked To: PeerAddr producer.
func TestRFC5881TransmitDestinationStable(t *testing.T) {
	l, ct, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)
	localDiscr := m.LocalDiscriminator()

	changed := netip.MustParseAddr("198.51.100.7")
	l.handleInbound(inboundControl(changed, key.Local, key.Interface, localDiscr))

	if m.PeerAddr() != key.Peer {
		t.Fatalf("PeerAddr drifted after source change: got %v, want configured peer %v", m.PeerAddr(), key.Peer)
	}

	l.mu.Lock()
	e := l.sessions[key]
	l.sendLocked(e, e.machine.Build())
	l.mu.Unlock()

	if !ct.sent {
		t.Fatal("sendLocked did not transmit")
	}
	if ct.last.To != key.Peer {
		t.Fatalf("transmit destination = %v, want configured peer %v", ct.last.To, key.Peer)
	}
}

// RFC requirement: RFC5881-6-6 negative -- the transmit destination is the
// configured peer and MUST NOT be the changed received source. The same
// sendLocked producer (loop.go:232) never copies in.From into the outbound To,
// so a spoofed source that reached the session by discriminator does not
// redirect transmission. Without this the positive could pass on code that
// happened to leave PeerAddr equal to the source by coincidence.
func TestRFC5881TransmitDestinationIgnoresChangedSource(t *testing.T) {
	l, ct, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)
	localDiscr := m.LocalDiscriminator()

	changed := netip.MustParseAddr("198.51.100.7")
	l.handleInbound(inboundControl(changed, key.Local, key.Interface, localDiscr))

	l.mu.Lock()
	e := l.sessions[key]
	l.sendLocked(e, e.machine.Build())
	l.mu.Unlock()

	if ct.last.To == changed {
		t.Fatalf("transmit destination adopted the changed received source %v; RFC 5881 sec 6 forbids it", changed)
	}
}

// RFC requirement: RFC5881-2-2 positive -- a separate BFD session MUST be
// established for each protocol (IPv4 and IPv6) over a link. The session key
// (internal/component/bfd/api/events.go:166-174) carries the peer/local
// netip.Addr, whose family differs between IPv4 and IPv6, so an IPv4 peer and
// an IPv6 peer form two distinct sessions with distinct discriminators.
func TestRFC5881PerProtocolSessions(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})

	v4 := reqFor(addrB, addrA)
	v6 := api.SessionRequest{
		Peer:                  netip.MustParseAddr("2001:db8::2"),
		Local:                 netip.MustParseAddr("2001:db8::1"),
		Interface:             "loop",
		Mode:                  api.SingleHop,
		DesiredMinTxInterval:  10_000,
		RequiredMinRxInterval: 10_000,
		DetectMult:            3,
	}
	if _, err := l.EnsureSession(v4); err != nil {
		t.Fatalf("EnsureSession v4: %v", err)
	}
	if _, err := l.EnsureSession(v6); err != nil {
		t.Fatalf("EnsureSession v6: %v", err)
	}

	l.mu.Lock()
	n := len(l.sessions)
	d4 := l.sessions[v4.Key()].machine.LocalDiscriminator()
	d6 := l.sessions[v6.Key()].machine.LocalDiscriminator()
	l.mu.Unlock()

	if n != 2 {
		t.Fatalf("IPv4 and IPv6 peers produced %d sessions, want 2 (one per protocol)", n)
	}
	if v4.Key() == v6.Key() {
		t.Fatal("IPv4 and IPv6 peers collapsed onto one session key")
	}
	if d4 == d6 {
		t.Fatalf("IPv4 and IPv6 sessions share discriminator %d; they must be independent", d4)
	}
}

// RFC requirement: RFC5881-2-2 negative -- session separation is keyed by the
// address (which bears the protocol), not created unconditionally. Two requests
// for the SAME peer address coalesce into ONE session via refcounting
// (EnsureSession, internal/component/bfd/engine/engine.go:349-351), proving the
// key is what separates protocols rather than every EnsureSession minting a new
// session.
func TestRFC5881SamePeerCoalesces(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	req := reqFor(addrB, addrA)

	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("first EnsureSession: %v", err)
	}
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("second EnsureSession: %v", err)
	}

	l.mu.Lock()
	n := len(l.sessions)
	rc := l.sessions[req.Key()].machine.Refcount()
	l.mu.Unlock()

	if n != 1 {
		t.Fatalf("same peer produced %d sessions, want 1 (refcount share)", n)
	}
	if rc != 2 {
		t.Fatalf("refcount after two EnsureSession = %d, want 2", rc)
	}
}
