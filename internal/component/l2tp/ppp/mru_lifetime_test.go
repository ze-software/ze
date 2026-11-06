// Design: docs/guide/l2tp.md -- a new LCP negotiation starts from the configured MRU.

package ppp

import (
	"errors"
	"testing"
	"time"
)

// assertLocalMRUOffer reads the receive MRU a Configure-Request offers.
func assertLocalMRUOffer(t *testing.T, request LCPPacket, want uint16) {
	t.Helper()
	opts, err := ParseLCPOptions(request.Data)
	if err != nil {
		t.Fatal(err)
	}
	mru, present := lookupMRUOption(opts)
	if !present || mru != want {
		t.Fatalf("offered local MRU = %d, present=%t; want %d", mru, present, want)
	}
}

// TestNewLCPLifetimeRestoresConfiguredMRU proves docs/guide/l2tp.md: "A later
// LCP negotiation can restore IPv6 when the new directional MRUs support 1280."
// The peer Naks Ze's MRU to 1000 in the first lifetime, which refuses IPv6, then
// renegotiates with no limit of its own. The second lifetime MUST offer the
// configured 1500 and start IPv6CP. Method: the real Driver, observed on the wire.
func TestNewLCPLifetimeRestoresConfiguredMRU(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 29301)
	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := makeTestDriver(backend, ops)
	go autoAcceptIP(d)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeConn(pair.peerEnd)
		d.Stop()
	})
	d.SessionsIn() <- StartSession{
		TunnelID: 29, SessionID: 4, ChanFD: 29301, UnitFD: 29302, UnitNum: 42,
		MaxMRU: 1500, AuthTimeout: 5 * time.Second, IPTimeout: 5 * time.Second,
	}
	td := &ncpTestDriver{t: t, driver: d, backend: backend, peer: pair.peerEnd}

	// First lifetime: the Nak lowers Ze's offer and refuses IPv6 service.
	request := readLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureRequest)
	assertLocalMRUOffer(t, request, 1500)
	writeLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureNak, request.Identifier, optStream(mruOpt(1000)))
	retry := readLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureRequest)
	assertLocalMRUOffer(t, retry, 1000)
	writeLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureAck, retry.Identifier, retry.Data)
	writeLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureRequest, 0x31, nil)
	readLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureAck)
	if _, ok := td.waitForEvent(t, 5*time.Second).(EventLCPUp); !ok {
		t.Fatal("first lifetime did not publish LCPUp")
	}
	td.completeIPCP(t)
	if _, ok := td.waitForEvent(t, 5*time.Second).(EventSessionIPAssigned); !ok {
		t.Fatal("first lifetime did not assign IPv4")
	}
	if _, ok := td.waitForEvent(t, 5*time.Second).(EventSessionUp); !ok {
		t.Fatal("first lifetime did not become usable")
	}
	writeLifecyclePacket(t, pair.peerEnd, ProtoIPv6CP, LCPConfigureRequest, 11,
		[]byte{1, 10, 2, 0, 0, 0, 0, 0, 0, 1})
	refusal := readLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPProtocolReject)
	if rejected := rejectedProtocol(t, refusal); rejected != ProtoIPv6CP {
		t.Fatalf("first lifetime rejected protocol %#x, want IPv6CP", rejected)
	}

	// Second lifetime: nothing limits either direction any more.
	writeLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureRequest, 0x41, nil)
	down, ok := td.waitForEvent(t, 5*time.Second).(EventLCPDown)
	if !ok || !down.NetworkPhase {
		t.Fatal("restart did not withdraw the first network lifetime")
	}
	down.Acknowledge()
	replacement := readLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureRequest)
	assertLocalMRUOffer(t, replacement, 1500)
	readLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureAck)
	writeLifecyclePacket(t, pair.peerEnd, ProtoLCP, LCPConfigureAck, replacement.Identifier, replacement.Data)
	if _, ok := td.waitForEvent(t, 5*time.Second).(EventLCPUp); !ok {
		t.Fatal("second lifetime did not publish LCPUp")
	}
	if cr := td.readPeerNCPPacket(t, ProtoIPCP); cr.Code != LCPConfigureRequest {
		t.Fatalf("second lifetime IPCP code = %d, want Configure-Request", cr.Code)
	}
	if cr := td.readPeerNCPPacket(t, ProtoIPv6CP); cr.Code != LCPConfigureRequest {
		t.Fatalf("second lifetime IPv6CP code = %d, want Configure-Request", cr.Code)
	}
}

// TestNewLCPLifetimeRestoresRejectedOptions gives a rejected MRU and Magic-Number
// the same answer as a Nakked MRU: a Configure-Reject binds the negotiation it
// answered, and a new lifetime offers both again. A Nak first lowers the local MRU,
// so the restored offer proves both MRU fields reset. Every way into a new lifetime
// that carries a packet is covered: a Configure-Request received in Opened or in
// Stopped, and a Terminate-Ack received in Opened (the RTA edge is tld,scr).
func TestNewLCPLifetimeRestoresRejectedOptions(t *testing.T) {
	for _, tc := range []struct {
		from LCPState
		code uint8
	}{
		{LCPStateOpened, LCPConfigureRequest},
		{LCPStateStopped, LCPConfigureRequest},
		{LCPStateOpened, LCPTerminateAck},
	} {
		t.Run(tc.from.String()+"/"+LCPCodeName(tc.code), func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateReqSent)
			if !s.sendConfigureRequest() {
				t.Fatal("first Configure-Request was not sent")
			}
			request := lastLCPConfigureRequest(t, rec)
			s.handleLCPPacket(LCPPacket{Code: LCPConfigureNak, Identifier: request.Identifier, Data: optStream(mruOpt(1000))})
			request = lastLCPConfigureRequest(t, rec)
			assertLocalMRUOffer(t, request, 1000)
			s.handleLCPPacket(LCPPacket{Code: LCPConfigureReject, Identifier: request.Identifier, Data: request.Data})
			opts, err := ParseLCPOptions(lastLCPConfigureRequest(t, rec).Data)
			if err != nil {
				t.Fatal(err)
			}
			if _, present := lookupMRUOption(opts); present {
				t.Fatal("the same negotiation offered a rejected MRU again")
			}
			if _, present := lookupMagicNumber(opts); present {
				t.Fatal("the same negotiation offered a rejected Magic-Number again")
			}

			s.mu.Lock()
			s.state = tc.from
			s.mu.Unlock()
			s.handleLCPPacket(LCPPacket{Code: tc.code, Identifier: 9})
			if got := s.currentState(); got == tc.from {
				t.Fatalf("state stayed %s, so the packet started no new lifetime", got)
			}
			replacement := lastLCPConfigureRequest(t, rec)
			assertLocalMRUOffer(t, replacement, MaxFrameLen)
			opts, err = ParseLCPOptions(replacement.Data)
			if err != nil {
				t.Fatal(err)
			}
			if magic, present := lookupMagicNumber(opts); !present || magic != s.magic {
				t.Fatalf("new lifetime Magic-Number = %#x, present=%t; want %#x", magic, present, s.magic)
			}
		})
	}
}

// TestNakedMRUHoldsWithinOneLifetime is the negative of the two tests above: a
// reset placed in the send path would pass them and forget the Nak on the next
// retransmission.
func TestNakedMRUHoldsWithinOneLifetime(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	if !s.sendConfigureRequest() {
		t.Fatal("first Configure-Request was not sent")
	}
	request := lastLCPConfigureRequest(t, rec)
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureNak, Identifier: request.Identifier, Data: optStream(mruOpt(1000))})
	assertLocalMRUOffer(t, lastLCPConfigureRequest(t, rec), 1000)
	if !s.sendConfigureRequest() {
		t.Fatal("retransmission was not sent")
	}
	assertLocalMRUOffer(t, lastLCPConfigureRequest(t, rec), 1000)
}

// mtuOrderBackend reports each SetMTU to the test before recording it.
type mtuOrderBackend struct {
	*fakeBackend
	onSetMTU func()
}

func (b *mtuOrderBackend) SetMTU(name string, mtu int) error {
	b.onSetMTU()
	return b.fakeBackend.SetMTU(name, mtu)
}

// TestInterfaceMTUInstalledBeforeChannelConnect fixes the order afterLCPOpen
// uses. A reopened lifetime keeps the previous interface MTU until SetMTU runs,
// and the unit transmits from the moment the channel connects, so a smaller peer
// MRU has to reach the interface first. Method: direct call, recording both steps.
func TestInterfaceMTUInstalledBeforeChannelConnect(t *testing.T) {
	s, _, _ := newRFC1661Session(LCPStateOpened)
	s.negotiatedMRU = 1400
	s.authRespCh <- authResponseMsg{accept: true}
	var steps []string
	s.backend = &mtuOrderBackend{
		fakeBackend: &fakeBackend{},
		onSetMTU:    func() { steps = append(steps, "mtu") },
	}
	s.ops.connect = func(_, _ int) error {
		steps = append(steps, "connect")
		return nil
	}
	if !s.afterLCPOpen() {
		t.Fatal("afterLCPOpen failed")
	}
	if len(steps) != 2 || steps[0] != "mtu" || steps[1] != "connect" {
		t.Fatalf("steps = %v, want [mtu connect]", steps)
	}
}

// TestFailedMTUDoesNotConnectChannel is the negative of the order above: an MTU
// the interface refused leaves no channel connected with the old MTU in place.
func TestFailedMTUDoesNotConnectChannel(t *testing.T) {
	s, _, events := newRFC1661Session(LCPStateOpened)
	s.authRespCh <- authResponseMsg{accept: true}
	s.backend = &fakeBackend{mtuErr: errors.New("mtu refused")}
	connects := 0
	s.ops.connect = func(_, _ int) error {
		connects++
		return nil
	}
	if s.afterLCPOpen() {
		t.Fatal("afterLCPOpen succeeded with a refused MTU")
	}
	if connects != 0 || s.channelConnected {
		t.Fatalf("connects = %d, connected = %t; want neither", connects, s.channelConnected)
	}
	if _, ok := (<-events).(EventSessionDown); !ok {
		t.Fatal("refused MTU did not report the session down")
	}
}
