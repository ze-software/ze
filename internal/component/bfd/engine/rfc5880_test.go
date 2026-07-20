// VALIDATES: RFC 5880 base-protocol engine behavior -- discriminator
// allocation, Your-Discriminator demultiplexing, the authentication step of
// the reception procedure, the immediate Final reply to a Poll, periodic
// transmission and its jitter bounds, the echo scheduler's Up gate, echo
// demultiplexing, and the single-hop receive TTL check.
// PREVENTS: a zero or duplicate local discriminator, a packet delivered to
// the wrong session, an unauthenticated packet reaching the FSM of a
// protected session, a Poll that goes unanswered, jitter outside the RFC
// band, and echo packets leaving a session that is not Up.
package engine

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/component/bfd/auth"
	"codeberg.org/thomas-mangin/ze/internal/component/bfd/packet"
	"codeberg.org/thomas-mangin/ze/internal/component/bfd/transport"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
)

// rfc5880Secret is the shared authentication key used by the engine-level
// authentication tests.
var rfc5880Secret = []byte("rfc5880-engine-key")

// rfc5880Inbound builds a wire-encoded single-hop Control packet wrapped in a
// transport.Inbound. mut lets a test set the flags or intervals it cares
// about before encoding.
func rfc5880Inbound(from, local netip.Addr, iface string, yd uint32, mut func(*packet.Control)) transport.Inbound {
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
	if mut != nil {
		mut(&c)
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

// ---------------------------------------------------------------------
// Section 6.3 / 6.8.1 -- discriminator allocation
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.3-1 positive -- My Discriminator is a nonzero
// value unique across all BFD sessions on the system. EnsureSession
// (internal/component/bfd/engine/engine.go:354) draws it from
// allocateDiscriminatorLocked (engine.go:445-464), which skips zero and any
// value already present in the byDiscr index.
// RFC requirement: RFC5880-6.8.1-3 positive -- bfd.LocalDiscr is that same
// allocation, so it is unique across sessions and nonzero.
func TestRFC5880DiscriminatorsAreNonZeroAndUnique(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	seen := map[uint32]bool{}
	for i := range 32 {
		peer := netip.AddrFrom4([4]byte{203, 0, 113, byte(10 + i)})
		req := reqFor(peer.String(), addrA)
		if _, err := l.EnsureSession(req); err != nil {
			t.Fatalf("EnsureSession %v: %v", peer, err)
		}
		m := machineFor(t, l, req.Key())
		d := m.LocalDiscriminator()
		if d == 0 {
			t.Fatalf("session %v got discriminator 0, which RFC 5880 reserves", peer)
		}
		if seen[d] {
			t.Fatalf("discriminator %d handed out twice", d)
		}
		seen[d] = true
	}
}

// RFC requirement: RFC5880-6.3-1 negative -- the allocator is not a naive
// counter that could emit the reserved zero or a live value: with the counter
// parked on zero and the next candidate already taken,
// allocateDiscriminatorLocked (engine.go:449-462) skips both and returns a
// fresh nonzero value.
// RFC requirement: RFC5880-6.8.1-3 negative -- the same skip logic is what
// keeps bfd.LocalDiscr unique when the counter wraps into occupied space.
func TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})

	l.mu.Lock()
	// Park the counter on the reserved value and occupy the two slots
	// immediately after it.
	l.nextDiscr = 0
	l.byDiscr[1] = &sessionEntry{}
	l.byDiscr[2] = &sessionEntry{}
	got, err := l.allocateDiscriminatorLocked()
	l.mu.Unlock()

	if err != nil {
		t.Fatalf("allocateDiscriminatorLocked: %v", err)
	}
	if got == 0 {
		t.Fatal("allocator handed out the reserved discriminator 0")
	}
	if got == 1 || got == 2 {
		t.Fatalf("allocator handed out the already-taken discriminator %d", got)
	}
}

// ---------------------------------------------------------------------
// Section 6.8.6 -- Your Discriminator demultiplexing
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.8.6-7 positive -- when Your Discriminator is
// nonzero it selects the session. handleInbound
// (internal/component/bfd/engine/loop.go:82-83) looks the value up in the
// byDiscr index and drives that session's FSM, so the packet is delivered and
// bfd.RemoteDiscr becomes the peer's My Discriminator.
func TestRFC5880YourDiscriminatorSelectsSession(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), nil))

	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("packet selected by Your Discriminator was not delivered: RemoteDiscr = %d, want %d", got, peerMyDiscr)
	}
}

// RFC requirement: RFC5880-6.8.6-7 negative -- if no session is found for a
// nonzero Your Discriminator the packet is discarded: handleInbound
// (loop.go:96-98) returns on a byDiscr miss without falling back to a
// source-address match, so an unallocated discriminator touches no state.
func TestRFC5880UnknownYourDiscriminatorDiscarded(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	const unallocated uint32 = 0xABCDEF
	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, unallocated, nil))

	if got := m.RemoteDiscriminator(); got != 0 {
		t.Fatalf("packet with an unallocated Your Discriminator was delivered: RemoteDiscr = %d", got)
	}
}

// ---------------------------------------------------------------------
// Section 9 -- single-hop TTL check on receipt
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-9-2 negative -- for a single-hop session a received
// packet whose TTL is not the maximum MUST be discarded. passesTTLGate
// (internal/component/bfd/engine/loop.go:158-170) returns in.TTL == 255 for
// api.SingleHop, and handleInbound (loop.go:104-111) returns before touching
// the FSM when the gate fails, so an off-link packet cannot install itself as
// the remote end.
func TestRFC5880SingleHopReceiveTTLNotMaxDiscarded(t *testing.T) {
	for _, ttl := range []uint8{0, 1, 64, 254} {
		l, _, key := newSingleHopLoop(t)
		m := machineFor(t, l, key)

		in := rfc5880Inbound(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), nil)
		in.TTL = ttl
		l.handleInbound(in)

		if got := m.RemoteDiscriminator(); got != 0 {
			t.Fatalf("TTL %d accepted: RemoteDiscr = %d, want the packet discarded", ttl, got)
		}
	}
}

// ---------------------------------------------------------------------
// Section 6.8.6 -- authentication step of the reception procedure
// ---------------------------------------------------------------------

// rfc5880AuthLoop builds an unstarted Loop with one authenticated single-hop
// session and returns the loop, its capture transport, the session key, and a
// signer configured with the same key so a test can forge valid packets.
func rfc5880AuthLoop(t *testing.T, authType uint8) (*Loop, *captureTransport, api.Key, auth.Signer) {
	t.Helper()
	ct := &captureTransport{}
	l := NewLoop(ct, clock.RealClock{})
	req := reqFor(addrB, addrA)
	req.Auth = &api.AuthSettings{
		Type:       authType,
		KeyID:      5,
		Secret:     rfc5880Secret,
		Meticulous: authType == packet.AuthTypeMeticulousKeyedSHA1 || authType == packet.AuthTypeMeticulousKeyedMD5,
	}
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	signer, err := auth.NewSigner(auth.Settings{
		Type:       authType,
		KeyID:      5,
		Secret:     rfc5880Secret,
		Meticulous: req.Auth.Meticulous,
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return l, ct, req.Key(), signer
}

// rfc5880SignedInbound builds an authenticated single-hop Control packet for
// the given signer and sequence number.
func rfc5880SignedInbound(key api.Key, signer auth.Signer, yd, seq uint32) transport.Inbound {
	c := packet.Control{
		Version:               packet.Version,
		State:                 packet.StateDown,
		Auth:                  true,
		DetectMult:            3,
		Length:                uint8(packet.MandatoryLen + signer.BodyLen()),
		MyDiscriminator:       peerMyDiscr,
		YourDiscriminator:     yd,
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
	}
	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c.WriteTo(buf, 0)
	signer.Sign(buf, packet.MandatoryLen, seq)
	return transport.Inbound{
		From:      key.Peer,
		Local:     key.Local,
		Interface: key.Interface,
		Mode:      api.SingleHop,
		TTL:       255,
		Bytes:     buf,
	}
}

// RFC requirement: RFC5880-6.8.6-11 positive -- when the A bit is set the
// packet is authenticated per Section 6.7 before the reception procedure
// continues. handleInbound (internal/component/bfd/engine/loop.go:123-133)
// calls Machine.Verify (internal/component/bfd/session/auth.go:101-106) and
// only reaches Receive when the digest and sequence check pass, so a correctly
// signed packet is delivered.
func TestRFC5880AuthenticatedPacketVerifiedAndDelivered(t *testing.T) {
	l, _, key, signer := rfc5880AuthLoop(t, packet.AuthTypeKeyedSHA1)
	m := machineFor(t, l, key)

	l.handleInbound(rfc5880SignedInbound(key, signer, m.LocalDiscriminator(), 1000))

	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("correctly signed packet was dropped: RemoteDiscr = %d, want %d", got, peerMyDiscr)
	}
}

// RFC requirement: RFC5880-6.8.6-11 negative -- a packet that fails
// authentication is discarded: handleInbound (loop.go:124-132) returns on the
// Verify error, so neither a tampered digest nor a wrong-key signature reaches
// the FSM.
func TestRFC5880UnauthenticPacketDiscarded(t *testing.T) {
	l, _, key, signer := rfc5880AuthLoop(t, packet.AuthTypeKeyedSHA1)
	m := machineFor(t, l, key)

	tampered := rfc5880SignedInbound(key, signer, m.LocalDiscriminator(), 1000)
	tampered.Bytes[len(tampered.Bytes)-1] ^= 0xFF
	l.handleInbound(tampered)
	if got := m.RemoteDiscriminator(); got != 0 {
		t.Fatalf("packet with a tampered digest was delivered: RemoteDiscr = %d", got)
	}

	wrongKey, err := auth.NewSigner(auth.Settings{
		Type:   packet.AuthTypeKeyedSHA1,
		KeyID:  5,
		Secret: []byte("not-the-shared-key"),
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	l.handleInbound(rfc5880SignedInbound(key, wrongKey, m.LocalDiscriminator(), 1001))
	if got := m.RemoteDiscriminator(); got != 0 {
		t.Fatalf("packet signed with the wrong key was delivered: RemoteDiscr = %d", got)
	}
}

// RFC requirement: RFC5880-6.7.3-4 positive -- for Meticulous Keyed MD5,
// bfd.XmitAuthSeq is incremented for each packet transmitted. sendLocked
// (internal/component/bfd/engine/loop.go:225-228) calls Machine.Sign and then
// Machine.AdvanceAuthSeq (internal/component/bfd/session/auth.go:86-94) on
// every transmission, so consecutive packets carry consecutive sequence
// numbers.
// RFC requirement: RFC5880-6.7.4-4 positive -- Meticulous Keyed SHA1 uses the
// same producer, so its transmitted sequence increments per packet too.
func TestRFC5880MeticulousSequenceIncrementsPerPacket(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeMeticulousKeyedMD5, packet.AuthTypeMeticulousKeyedSHA1} {
		l, ct, key, _ := rfc5880AuthLoop(t, at)

		var seqs []uint32
		for range 3 {
			l.mu.Lock()
			e := l.sessions[key]
			l.sendLocked(e, e.machine.Build())
			l.mu.Unlock()
			if !ct.sent {
				t.Fatalf("type %d: sendLocked did not transmit", at)
			}
			seqs = append(seqs, binary.BigEndian.Uint32(ct.last.Bytes[packet.MandatoryLen+4:]))
		}
		for i := 1; i < len(seqs); i++ {
			if seqs[i] != seqs[i-1]+1 {
				t.Fatalf("type %d: transmitted sequences %v are not incremented per packet", at, seqs)
			}
		}
	}
}

// ---------------------------------------------------------------------
// Section 6.8.6 / 6.8.7 -- Final reply and periodic transmission
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.8.6-16 positive -- when a received packet has the
// Poll bit set, a packet with the Final bit set is transmitted immediately,
// independent of the transmit timer. handleInbound
// (internal/component/bfd/engine/loop.go:140-142) calls sendLocked with
// Machine.BuildFinal (internal/component/bfd/session/fsm.go:236-241) as soon
// as Receive returns.
func TestRFC5880PollAnsweredWithImmediateFinal(t *testing.T) {
	l, ct, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	ct.sent = false
	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), func(c *packet.Control) {
		c.Poll = true
	}))

	if !ct.sent {
		t.Fatal("no packet transmitted in response to a Poll")
	}
	out, _, err := packet.ParseControl(ct.last.Bytes)
	if err != nil {
		t.Fatalf("ParseControl of the reply: %v", err)
	}
	if !out.Final {
		t.Fatal("the immediate reply to a Poll does not set F")
	}
	if out.Poll {
		t.Fatal("the Final reply also set P")
	}
}

// RFC requirement: RFC5880-6.8.6-16 negative -- the immediate transmission is
// driven by the P bit alone: a packet with P=0 produces no out-of-schedule
// reply, because the branch at loop.go:140 is not taken. Without this the
// positive could pass on code that answered every received packet.
func TestRFC5880NonPollProducesNoImmediateReply(t *testing.T) {
	l, ct, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	ct.sent = false
	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), nil))

	if ct.sent {
		t.Fatal("a packet with P=0 triggered an immediate reply")
	}
}

// RFC requirement: RFC5880-6.8.6-15 positive -- when Demand mode is not active
// on the remote system, periodic BFD Control packets are transmitted. tick
// (internal/component/bfd/engine/loop.go:186-201) walks every session and
// calls sendLocked once the periodic deadline has passed.
// RFC requirement: RFC5880-6.8.14-1 positive -- a session whose peer stops
// asserting the D bit keeps receiving periodic Control packets from ze: the
// same tick is unconditional on the remote demand state, so the periodic train
// is running whenever remote Demand mode is inactive.
func TestRFC5880PeriodicTransmitWhenRemoteDemandInactive(t *testing.T) {
	l, ct, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	// Peer asserts Demand, then withdraws it.
	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, 0, func(c *packet.Control) {
		c.Demand = true
	}))
	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), func(c *packet.Control) {
		c.State = packet.StateInit
		c.Demand = false
	}))

	ct.sent = false
	l.tick()
	if !ct.sent {
		t.Fatal("no periodic Control packet transmitted while remote Demand mode is inactive")
	}
	out, _, err := packet.ParseControl(ct.last.Bytes)
	if err != nil {
		t.Fatalf("ParseControl of the periodic packet: %v", err)
	}
	if out.MyDiscriminator != m.LocalDiscriminator() {
		t.Fatalf("periodic packet carries My Discriminator %d, want %d", out.MyDiscriminator, m.LocalDiscriminator())
	}
}

// RFC requirement: RFC5880-6.8.6-15 negative -- periodic transmission is
// conditional, not unconditional: tick (loop.go:189-191) skips a session in
// AdminDown, and (loop.go:192-195) a session with no armed deadline. Without
// this the positive could pass on code that transmitted on every tick
// regardless of session state.
func TestRFC5880NoPeriodicTransmitWhileAdminDown(t *testing.T) {
	l, ct, key := newSingleHopLoop(t)

	l.mu.Lock()
	l.sessions[key].machine.AdminDown(packet.DiagAdminDown)
	l.mu.Unlock()

	ct.sent = false
	l.tick()
	if ct.sent {
		t.Fatal("a session in AdminDown transmitted a periodic Control packet")
	}
}

// RFC requirement: RFC5880-6.8.7-2 positive -- the periodic transmit interval
// is jittered on a per-packet basis. applyJitter
// (internal/component/bfd/engine/engine.go:249-263) draws a fresh reduction
// for every transmission, so a run of draws is neither constant nor zero and
// its mean sits near the middle of the band.
func TestRFC5880JitterIsAppliedPerPacket(t *testing.T) {
	l := NewLoop(nil, nil)
	const base = 300 * time.Millisecond

	var sum time.Duration
	distinct := map[time.Duration]bool{}
	const draws = 2000
	for range draws {
		d := l.applyJitter(base, 3)
		sum += d
		distinct[d] = true
	}
	if len(distinct) < 100 {
		t.Fatalf("only %d distinct jitter values across %d draws; the reduction is not per-packet", len(distinct), draws)
	}
	mean := sum / draws
	lo := time.Duration(float64(base) * 0.10)
	hi := time.Duration(float64(base) * 0.15)
	if mean < lo || mean > hi {
		t.Fatalf("mean jitter %v outside the expected ~12.5%% of %v", mean, base)
	}
}

// RFC requirement: RFC5880-6.8.7-2 negative -- the jitter never leaves the
// 0-25% band: applyJitter (engine.go:259-261) scales a [0,1) draw by
// JitterMaxFraction, so no reduction is negative and none reaches 25% of the
// base. A reduction outside the band would let the receiver time out or make
// the sender transmit faster than the negotiated rate.
func TestRFC5880JitterStaysWithinBand(t *testing.T) {
	l := NewLoop(nil, nil)
	const base = 300 * time.Millisecond
	upper := time.Duration(float64(base) * JitterMaxFraction)
	for i := range 20_000 {
		d := l.applyJitter(base, 3)
		if d < 0 {
			t.Fatalf("draw %d: negative reduction %v", i, d)
		}
		if d >= upper {
			t.Fatalf("draw %d: reduction %v reaches or exceeds 25%% of %v", i, d, base)
		}
	}
}

// RFC requirement: RFC5880-6.8.7-3 positive -- when bfd.DetectMult is 1 the
// transmitted interval is between 75% and 90% of the negotiated interval.
// applyJitter (engine.go:254-258) draws the reduction from
// [JitterMinFractionDetectMultOne, JitterMaxFraction) = [10%, 25%), so the
// resulting interval (base minus reduction) lands in [75%, 90%].
func TestRFC5880JitterDetectMultOneWindow(t *testing.T) {
	l := NewLoop(nil, nil)
	const base = 1000 * time.Millisecond
	lo := time.Duration(float64(base) * 0.75)
	hi := time.Duration(float64(base) * 0.90)
	for i := range 20_000 {
		interval := base - l.applyJitter(base, 1)
		if interval < lo || interval > hi {
			t.Fatalf("draw %d: transmitted interval %v outside [75%%, 90%%] of %v", i, interval, base)
		}
	}
}

// RFC requirement: RFC5880-6.8.7-3 negative -- the 10% floor is specific to
// bfd.DetectMult == 1 rather than applied to every session: with DetectMult 3
// the else branch (engine.go:260) draws from [0, 25%), so reductions below 10%
// do occur and the interval can exceed 90% of the base. Without this the
// positive could pass on code that always applied the tighter window.
func TestRFC5880JitterFloorOnlyForDetectMultOne(t *testing.T) {
	l := NewLoop(nil, nil)
	const base = 1000 * time.Millisecond
	floor := time.Duration(float64(base) * JitterMinFractionDetectMultOne)
	var belowFloor int
	for range 20_000 {
		if l.applyJitter(base, 3) < floor {
			belowFloor++
		}
	}
	if belowFloor == 0 {
		t.Fatal("no DetectMult=3 draw fell below the 10% floor; the DetectMult==1 window is being applied to every session")
	}
}

// ---------------------------------------------------------------------
// Section 6.8.8 / 6.8.9 -- echo scheduling and demultiplexing
// ---------------------------------------------------------------------

// rfc5880EchoLoop builds an unstarted Loop with a capture transport on both
// the Control and the Echo path plus one session that has echo configured. It
// drives the session to Up with the peer advertising a nonzero Required Min
// Echo RX Interval so echo is negotiated.
func rfc5880EchoLoop(t *testing.T) (*Loop, *captureTransport, api.Key) {
	t.Helper()
	ct := &captureTransport{}
	echoCT := &captureTransport{}
	l := NewLoopWithEcho(ct, echoCT, clock.RealClock{})
	req := echoReqFor(addrB, addrA)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := req.Key()
	m := machineFor(t, l, key)

	withEcho := func(c *packet.Control) { c.RequiredMinEchoRxInterval = 50_000 }
	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, 0, withEcho))
	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), func(c *packet.Control) {
		c.State = packet.StateInit
		withEcho(c)
	}))
	if m.State() != packet.StateUp {
		t.Fatalf("precondition: session state = %v, want Up", m.State())
	}
	if !m.EchoEnabled() {
		t.Fatal("precondition: echo must be negotiated")
	}
	return l, echoCT, key
}

// RFC requirement: RFC5880-6.8.9-1 positive -- echo packets are transmitted
// only while bfd.SessionState is Up. echoTickLocked
// (internal/component/bfd/engine/echo.go:47-71) requires m.State() == Up
// before priming the schedule and calling sendEchoLocked, so an Up session
// with echo negotiated does emit an echo.
func TestRFC5880EchoTransmittedWhileUp(t *testing.T) {
	l, echoCT, _ := rfc5880EchoLoop(t)

	echoCT.sent = false
	l.mu.Lock()
	l.echoTickLocked(time.Now())
	l.mu.Unlock()

	if !echoCT.sent {
		t.Fatal("no echo packet transmitted by an Up session with echo negotiated")
	}
	if _, err := packet.ParseEcho(echoCT.last.Bytes); err != nil {
		t.Fatalf("transmitted echo is not a valid envelope: %v", err)
	}
}

// RFC requirement: RFC5880-6.8.9-1 negative -- echo packets MUST NOT be
// transmitted when bfd.SessionState is not Up. echoTickLocked
// (echo.go:49-52) clears the echo schedule and skips the session for any state
// other than Up, so a session driven back to Down emits nothing.
func TestRFC5880NoEchoTransmittedWhenNotUp(t *testing.T) {
	l, echoCT, key := rfc5880EchoLoop(t)

	l.mu.Lock()
	l.sessions[key].machine.AdminDown(packet.DiagAdminDown)
	l.mu.Unlock()

	echoCT.sent = false
	l.mu.Lock()
	l.echoTickLocked(time.Now())
	l.mu.Unlock()

	if echoCT.sent {
		t.Fatal("an echo packet was transmitted by a session that is not Up")
	}
	m := machineFor(t, l, key)
	if !m.NextEchoTxDeadline().IsZero() {
		t.Fatalf("echo schedule left armed at %v on a session that is not Up", m.NextEchoTxDeadline())
	}
}

// RFC requirement: RFC5880-6.8.8-1 positive -- a returning echo packet is
// demultiplexed to the appropriate session. handleEchoInbound
// (internal/component/bfd/engine/echo.go:136-141) looks the envelope's Local
// Discriminator up in byDiscr and requires the source address to equal that
// session's configured peer before recording the round-trip time on it.
func TestRFC5880EchoDemultiplexedToItsSession(t *testing.T) {
	l, _, key := rfc5880EchoLoop(t)
	m := machineFor(t, l, key)

	// Transmit one echo so the outstanding ring holds its sequence number.
	l.mu.Lock()
	l.echoTickLocked(time.Now())
	l.mu.Unlock()

	hook := newEchoHook()
	l.SetMetricsHook(hook)

	buf := make([]byte, packet.EchoLen)
	packet.WriteEcho(buf, 0, packet.Echo{
		LocalDiscriminator: m.LocalDiscriminator(),
		Sequence:           1,
		TimestampMs:        uint32(time.Now().UnixMilli()),
	})
	l.handleEchoInbound(transport.Inbound{
		From:      key.Peer,
		Local:     key.Local,
		Interface: key.Interface,
		Mode:      api.SingleHop,
		TTL:       255,
		Bytes:     buf,
	})

	if hook.rxs.Load() != 1 {
		t.Fatalf("returning echo not delivered to its session: OnEchoRx fired %d times", hook.rxs.Load())
	}
}

// RFC requirement: RFC5880-6.8.8-1 negative -- an echo that belongs to no
// session is dropped rather than delivered to an arbitrary one:
// handleEchoInbound (echo.go:136-151) requires either a discriminator hit
// whose peer address matches, or a source address pinned to a live session,
// and otherwise logs and drops. This is also the amplification guard.
func TestRFC5880UnknownEchoDropped(t *testing.T) {
	l, _, key := rfc5880EchoLoop(t)

	hook := newEchoHook()
	l.SetMetricsHook(hook)

	buf := make([]byte, packet.EchoLen)
	packet.WriteEcho(buf, 0, packet.Echo{
		LocalDiscriminator: 0x99999999, // no such session
		Sequence:           1,
		TimestampMs:        uint32(time.Now().UnixMilli()),
	})
	l.handleEchoInbound(transport.Inbound{
		From:      netip.MustParseAddr("198.51.100.44"), // no session pinned here
		Local:     key.Local,
		Interface: key.Interface,
		Mode:      api.SingleHop,
		TTL:       255,
		Bytes:     buf,
	})

	if hook.rxs.Load() != 0 {
		t.Fatalf("an echo matching no session was delivered: OnEchoRx fired %d times", hook.rxs.Load())
	}
}
