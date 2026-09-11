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

// newMultiHopLoop is the RFC 5883 sibling of newSingleHopLoop. The request
// carries no interface, which is what api.SessionRequest.Canonical produces for
// a multi-hop client: the session is routed, so no link is part of its
// identity. Each call uses a fresh peer address, so two loops in one test hold
// two distinct sessions.
func newMultiHopLoop(t *testing.T) (*Loop, api.Key) {
	t.Helper()
	multiHopPeers++
	ct := &captureTransport{}
	l := NewLoop(ct, clock.RealClock{})
	req := reqFor(addrB, addrA)
	req.Peer = netip.AddrFrom4([4]byte{203, 0, 113, byte(multiHopPeers)})
	req.Interface = ""
	req.Mode = api.MultiHop
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	return l, req.Key()
}

// multiHopPeers hands each newMultiHopLoop call its own peer address.
var multiHopPeers int

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

// TestFirstPacketMatchesWhatTheTransportSurfaces is the round-7 finding. The
// tests above hand handleInbound the session key's OWN Local and Interface, so
// they pass on any code that indexes on those two fields, including code the
// transport can never feed. This one supplies what a received packet actually
// carries and asserts both directions.
//
// RFC 5880 Section 6.8.6: "If the Your Discriminator field is zero, the session
// MUST be selected based on some combination of other fields." The fields have
// to be observable at the receiver. (*UDP).readLoop takes the destination
// address and the ingress interface from IP_PKTINFO; until it did, it stamped
// u.Bind.Addr(), the WILDCARD the socket binds, onto every Inbound, so no
// session carrying a real local address could ever be selected. The wildcard
// case below is that bug, and it fails against the old readLoop.
//
// RFC requirement: RFC5880-6.8.6-18 positive -- the selection runs on fields the
// receiver can OBSERVE. A packet carrying this session's real destination
// address selects it, and one carrying the wildcard the socket binds does not,
// so the "combination of other fields" is read off the packet rather than
// copied from the session being looked for.
func TestFirstPacketMatchesWhatTheTransportSurfaces(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	if !key.Local.IsValid() {
		t.Fatalf("setup: the session key carries no local address, so this test proves nothing")
	}

	// What IP_PKTINFO reports: the address the peer sent TO, and the interface
	// the packet arrived on. Same values, arrived at from the wire rather than
	// copied from the session.
	m := machineFor(t, l, key)
	l.handleInbound(inboundControl(key.Peer, key.Local, key.Interface, 0))
	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("a first packet carrying the real destination address was not selected: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}

	// The interface, varied independently of the key. A packet that arrived on
	// another link is another session's, or nobody's, and MUST NOT select this
	// one. Passing key.Interface through, which is what this test did until
	// round 9, cannot tell an index that reads the field from one that ignores
	// it.
	elsewhere, _, onEth1 := newSingleHopLoop(t)
	em := machineFor(t, elsewhere, onEth1)
	elsewhere.handleInbound(inboundControl(onEth1.Peer, onEth1.Local, onEth1.Interface+"-other", 0))
	if got := em.RemoteDiscriminator(); got != 0 {
		t.Fatalf("a first packet from another interface selected a session bound to %q: RemoteDiscriminator = %d, want 0", onEth1.Interface, got)
	}

	// What the socket's bind address reports, which is what readLoop used to
	// stamp: the wildcard. It is not this session's local address and MUST NOT
	// select it, or a second session on another address would be selected by
	// the same packet.
	second, _, other := newSingleHopLoop(t)
	sm := machineFor(t, second, other)
	wildcard := netip.AddrFrom4([4]byte{})
	second.handleInbound(inboundControl(other.Peer, wildcard, other.Interface, 0))
	if got := sm.RemoteDiscriminator(); got != 0 {
		t.Fatalf("a first packet carrying the wildcard bind address selected a session keyed on %s: RemoteDiscriminator = %d, want 0", other.Local, got)
	}
}

// TestFirstPacketMultiHopIgnoresTheIngressInterface is the round-9 blocker. A
// multi-hop session is routed, so api.SessionRequest.Canonical clears its
// interface and the key carries none. The RFC 5883 socket must therefore not
// stamp one on what it receives, or the five-field key misses on every packet
// whose Your Discriminator is zero and the session is never selected.
//
// Before round 8 this held by accident, because readLoop set no interface at
// all. Round 8 started setting the real one for every mode and broke it. The
// arm below is the accident made into an assertion.
//
// RFC 5880 Section 6.8.6: "If the Your Discriminator field is zero, the session
// MUST be selected based on some combination of other fields." For a multi-hop
// session those fields are the addresses and the VRF; the ingress interface is
// not one of them and must not be compared.
func TestFirstPacketMultiHopIgnoresTheIngressInterface(t *testing.T) {
	l, key := newMultiHopLoop(t)
	if key.Interface != "" {
		t.Fatalf("setup: a multi-hop key carries interface %q, want none", key.Interface)
	}
	m := machineFor(t, l, key)

	// What the multi-hop socket surfaces: no interface.
	in := inboundControl(key.Peer, key.Local, "", 0)
	in.Mode = api.MultiHop
	l.handleInbound(in)
	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("a multi-hop first packet was not selected: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}

	// What round 8 made it surface: the real ingress interface, which no
	// multi-hop key can carry. Since the relaxation walk landed the engine no longer depends on
	// the transport getting this right -- a packet carrying an interface still
	// finds a session that named none, by the same second lookup the single-hop
	// blocker needed -- so this arm asserts SELECTION rather than a drop.
	//
	// That means this test cannot red if the transport starts stamping
	// multi-hop packets again: the discriminating proof of that rule is the
	// multi arm of TestIngressInterfaceIsSingleHopOnly, in the transport
	// package, where the rule lives. What this arm holds is the property that
	// matters to an operator: a multi-hop session is findable either way.
	second, other := newMultiHopLoop(t)
	sm := machineFor(t, second, other)
	stamped := inboundControl(other.Peer, other.Local, "eth0", 0)
	stamped.Mode = api.MultiHop
	second.handleInbound(stamped)
	if got := sm.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("a multi-hop packet carrying an ingress interface did not reach the session keyed without one: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
}

// TestFirstPacketSelectsASessionKeyedWithoutAnInterface is the round-11
// blocker, and the single-hop half of the same mismatch rounds 7, 8 and 10 each
// fixed on one side.
//
// api.SessionRequest.Canonical refuses to derive an interface in four
// configurations it cannot resolve (a link-local peer, two links on one subnet,
// an off-link peer, no interface backend loaded), so the session's key carries
// none. The transport stamps the real ingress interface on every single-hop
// packet, and byKey is an exact struct match, so every packet whose Your
// Discriminator is zero missed and the session never left Down.
//
// RFC 5880 Section 6.8.6: "If the Your Discriminator field is zero, the session
// MUST be selected based on some combination of other fields." A session whose
// client could not name its link states, by that silence, that the link is not
// part of its identity, so the combination for it is the tuple without one.
func TestFirstPacketSelectsASessionKeyedWithoutAnInterface(t *testing.T) {
	ct := &captureTransport{}
	l := NewLoop(ct, clock.RealClock{})
	req := reqFor(addrB, addrA)
	req.Interface = "" // what Canonical leaves behind when it refuses to derive
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := req.Key()
	m := machineFor(t, l, key)

	// The packet arrives on a real link, because every single-hop packet does.
	l.handleInbound(inboundControl(key.Peer, key.Local, "eth0", 0))
	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("a session keyed without an interface was not selected for a packet that arrived on one: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
}

// TestFirstPacketRelaxedMatchCosts drives the RULE rather than two instances of
// it: a key field the session left unset does not participate in the match
// (engine.go, keyRelaxation). Each case leaves a different field unset, and
// they are written as a table so a fifth optional dimension is covered by a row
// rather than by a new test.
//
// The two costs are pinned here, not just the two conveniences. A session that
// named nothing takes a packet from any link, INCLUDING one a more specific
// session would have wanted if it existed; and where a more specific session
// does exist it wins only for the value it names.
func TestFirstPacketRelaxedMatchCosts(t *testing.T) {
	// The packet is the same in every case, and it carries a real value in
	// every field, because that is what a received packet always does: the
	// kernel reports the destination address and the ingress interface whether
	// or not any session asked about them. What varies is the SESSION.
	// packetIface is what the kernel reports for the arriving packet. Where the
	// session NAMED an interface the packet has to be on it, because a session
	// that named a link is not matched by a packet from another one; where the
	// session named none, the packet is on a link it never heard of.
	for name, tc := range map[string]struct {
		unset       func(*api.SessionRequest)
		packetIface string
	}{
		"the interface the client could not name": {
			unset:       func(r *api.SessionRequest) { r.Interface = "" },
			packetIface: "eth7",
		},
		"the local address the client never wrote": {
			unset:       func(r *api.SessionRequest) { r.Local = netip.Addr{} },
			packetIface: "loop",
		},
		"both at once, which is what Canonical leaves on a refusal": {
			unset: func(r *api.SessionRequest) {
				r.Interface = ""
				r.Local = netip.Addr{}
			},
			packetIface: "eth7",
		},
	} {
		ct := &captureTransport{}
		l := NewLoop(ct, clock.RealClock{})
		req := reqFor(addrB, addrA)
		tc.unset(&req)
		if _, err := l.EnsureSession(req); err != nil {
			t.Fatalf("%s: EnsureSession: %v", name, err)
		}
		m := machineFor(t, l, req.Key())

		l.handleInbound(inboundControl(req.Key().Peer, netip.MustParseAddr(addrA), tc.packetIface, 0))
		if got := m.RemoteDiscriminator(); got != peerMyDiscr {
			t.Errorf("%s: the session was not selected for a packet carrying a value it never named: RemoteDiscriminator = %d, want %d", name, got, peerMyDiscr)
		}
	}
}

// TestFirstPacketPrefersTheSessionThatNamedTheLink states the cost of the
// choice above. Where two sessions share (peer, local, vrf, mode) and only one
// names a link, the exact match wins for a packet on that link, and the one
// that named no link takes what is left.
func TestFirstPacketPrefersTheSessionThatNamedTheLink(t *testing.T) {
	ct := &captureTransport{}
	l := NewLoop(ct, clock.RealClock{})
	named := reqFor(addrB, addrA)
	named.Interface = "eth0"
	if _, err := l.EnsureSession(named); err != nil {
		t.Fatalf("EnsureSession named: %v", err)
	}
	anyLink := reqFor(addrB, addrA)
	anyLink.Interface = ""
	if _, err := l.EnsureSession(anyLink); err != nil {
		t.Fatalf("EnsureSession unnamed: %v", err)
	}
	onEth0 := machineFor(t, l, named.Key())
	onAny := machineFor(t, l, anyLink.Key())

	l.handleInbound(inboundControl(named.Key().Peer, named.Key().Local, "eth0", 0))
	if got := onEth0.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("a packet on eth0 did not reach the session that named eth0: RemoteDiscriminator = %d", got)
	}
	if got := onAny.RemoteDiscriminator(); got != 0 {
		t.Fatalf("the packet on eth0 also reached the session that named no link: RemoteDiscriminator = %d, want 0", got)
	}

	// The other half of the same choice, and the arm that makes this test
	// about the COST rather than only the ordering: a packet from a link
	// nobody named is not dropped, it reaches the session that named none.
	// A fix that made the relaxation conditional on there being no
	// interface-named sibling would leave the assertions above green and this
	// one red.
	l.handleInbound(inboundControl(named.Key().Peer, named.Key().Local, "eth1", 0))
	if got := onAny.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("a packet on eth1 did not reach the session that named no link: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
	if got := onEth0.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("the eth0 session lost its association: RemoteDiscriminator = %d", got)
	}
}

// TestFirstPacketTieBreakPrefersTheLink pins the order INSIDE a popcount tier,
// which the comment above keyRelaxations states as a specification and which
// nothing else asserts.
//
// Two sessions to one peer, each naming exactly one of the two optional fields:
// one names the link and not the local address, the other the local address and
// not the link. A packet carrying both is one relaxation away from each, so the
// declaration order of relaxLocal and relaxIface decides which it reaches.
// relaxLocal is declared first, so the packet is offered to the session that
// named the LINK. Swap the two constants and this test reds; nothing else in
// the package does, which is what made the comment the only guard.
func TestFirstPacketTieBreakPrefersTheLink(t *testing.T) {
	ct := &captureTransport{}
	l := NewLoop(ct, clock.RealClock{})

	linkOnly := reqFor(addrB, addrA)
	linkOnly.Local = netip.Addr{}
	linkOnly.Interface = "eth0"
	if _, err := l.EnsureSession(linkOnly); err != nil {
		t.Fatalf("EnsureSession link-only: %v", err)
	}
	localOnly := reqFor(addrB, addrA)
	localOnly.Interface = ""
	if _, err := l.EnsureSession(localOnly); err != nil {
		t.Fatalf("EnsureSession local-only: %v", err)
	}
	onLink := machineFor(t, l, linkOnly.Key())
	onLocal := machineFor(t, l, localOnly.Key())

	// The packet names both, so neither session matches exactly.
	l.handleInbound(inboundControl(linkOnly.Key().Peer, netip.MustParseAddr(addrA), "eth0", 0))

	if got := onLink.RemoteDiscriminator(); got != peerMyDiscr {
		t.Errorf("the session that named the LINK was not selected: RemoteDiscriminator = %d, want %d.\n"+
			"The declaration order of relaxLocal and relaxIface decides this tie, and the comment above "+
			"keyRelaxations says the link wins.", got, peerMyDiscr)
	}
	if got := onLocal.RemoteDiscriminator(); got != 0 {
		t.Errorf("the session that named only the local address also took the packet: RemoteDiscriminator = %d, want 0", got)
	}
}
