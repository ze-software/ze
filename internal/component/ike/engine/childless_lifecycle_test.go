// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the childless IKE SA
// Related: childless_readers_test.go -- the readers of a session with no Child SA
// Related: create_child_test.go -- the Child SA a childless IKE SA creates later

package engine

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// clfLoopWait bounds every wait on the owner loop in this file. The loop reads its
// lifetimes and liveness on a one-second tick, so a few ticks is the whole budget.
const clfLoopWait = 10 * time.Second

// clfRunLoop runs the owner loop of a childless session on its own goroutine, with the
// lifetimes and liveness state the caller chose. The channel carries the error the loop
// ends with. The cleanup stops a loop the test left running.
func clfRunLoop(t *testing.T, sa *SA, ps *PeerSession, dpd *dpdState, childLT, ikeLT *lifetimeState,
	myTr *transport.UDPTransport, bus *sepBus,
) chan error {
	t.Helper()
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.inbound = make(chan transport.Packet, inboundQueueDepth)
	done := make(chan error, 1)
	exited := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-ps.stopCh:
		default:
			close(ps.stopCh)
		}
		<-exited
	})
	go func() {
		defer close(exited)
		done <- ps.maintainSA(sa, dpd, childLT, ikeLT,
			testIKEGroup(), NewSATable(), nil, myTr, bus, slogutil.DiscardLogger())
	}()
	return done
}

// clfLoopEnd waits for the owner loop to end and returns its error. It fails the test
// when the loop is still running after clfLoopWait.
func clfLoopEnd(t *testing.T, done chan error, what string) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(clfLoopWait):
		t.Fatalf("the owner loop of a childless IKE SA never ended: %s", what)
		return nil
	}
}

// clfMakeChildless takes the Child SA away from an established session, which is the state
// an IKE_AUTH that refused its child leaves behind (RFC 7296 Section 2.21.2).
func clfMakeChildless(t *testing.T, sa *SA, ps *PeerSession) {
	t.Helper()
	ps.setChildSA(nil)
	sa.IKEAuthChildless = true
	if ps.getChildSA() != nil {
		t.Fatal("the session still holds a Child SA")
	}
}

// TestChildlessSALiveness checks that liveness works on an IKE SA with no Child SA, in
// both directions (AC-10).
//
// Goal: RFC 7296 Section 2.4 liveness runs on the IKE SA, so a session with no Child SA
// still probes its peer, still answers the peer's probe, and still ends when the peer
// goes silent. Method: an established PSK pair loses its Child SA and the owner loop
// runs. In "probes and ends", the peer reads the probe, answers nothing, and the loop
// MUST end with the liveness timeout. In "answers", the peer sends an empty
// INFORMATIONAL request and MUST read a response at that Message ID.
//
// MUTATION: make maintainSA skip sendDPD when ps.getChildSA() is nil, and "probes and
// ends" goes red with no probe on the wire.
func TestChildlessSALiveness(t *testing.T) {
	t.Run("probes and ends", func(t *testing.T) {
		ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
		clfMakeChildless(t, ini, ps)
		dpd := &dpdState{
			interval: time.Millisecond,
			timeout:  time.Millisecond,
			lastSent: time.Now().Add(-time.Hour),
		}
		done := clfRunLoop(t, ini, ps, dpd, nil, nil, myTr, nil)

		probe := rtxRecv(t, peerTr)
		if probe == nil {
			t.Fatal("the childless SA sent no liveness probe")
		}
		if hdr := parseMsg(t, probe).Header; hdr.ExchangeType != wire.ExchangeInformational {
			t.Errorf("the probe exchange = %d, want INFORMATIONAL", hdr.ExchangeType)
		}
		if _, err := decryptAndParse(peer, parseMsg(t, probe), probe); err != nil {
			t.Errorf("the peer could not authenticate the probe: %v", err)
		}
		if err := clfLoopEnd(t, done, "a silent peer"); !errors.Is(err, errTimeout) {
			t.Fatalf("a silent peer ended the childless loop with %v, want the liveness timeout", err)
		}
	})

	t.Run("answers", func(t *testing.T) {
		ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
		clfMakeChildless(t, ini, ps)
		msgID := ini.ExpectedMsgID
		done := clfRunLoop(t, ini, ps, nil, nil, nil, myTr, nil)

		ps.inbound <- transport.Packet{Data: lcyRequest(t, peer, msgID, nil)}
		answer := rtxRecv(t, peerTr)
		if answer == nil {
			t.Fatal("the childless SA did not answer the peer's liveness probe")
		}
		hdr := parseMsg(t, answer).Header
		if hdr.Flags&wire.FlagResponse == 0 {
			t.Error("the answer is not a response")
		}
		if hdr.MessageID != msgID {
			t.Errorf("the answer rides Message ID %d, want %d", hdr.MessageID, msgID)
		}
		if _, err := decryptAndParse(peer, parseMsg(t, answer), answer); err != nil {
			t.Errorf("the peer could not authenticate the answer: %v", err)
		}
		close(ps.stopCh)
		if err := clfLoopEnd(t, done, "an operator stop"); err != nil {
			t.Errorf("an operator stop ended the childless loop with %v, want nil", err)
		}
	})
}

// TestChildlessSAHardLifetimeEnds checks that the IKE SA hard lifetime ends an IKE SA
// with no Child SA (AC-10).
//
// Goal: RFC 7296 Section 2.8 forbids using an SA past its lifetime, and a childless SA
// carries no Child lifetime that could end it first. Method: the owner loop of a
// childless session runs with an IKE hard time already past and a soft time far ahead,
// so no rekey starts; the loop MUST end with errTimeout on its next tick. A control run
// with a live lifetime MUST keep running until stopped.
//
// MUTATION: delete the `ikeLT.hardExpired(now)` branch of maintainSA and the expired
// case goes red: the loop never ends.
func TestChildlessSAHardLifetimeEnds(t *testing.T) {
	t.Run("expired", func(t *testing.T) {
		ini, _, ps, _, myTr := dpdProbeLink(t)
		clfMakeChildless(t, ini, ps)
		now := time.Now()
		ikeLT := &lifetimeState{softTime: now.Add(time.Hour), hardTime: now.Add(-time.Second)}
		done := clfRunLoop(t, ini, ps, nil, nil, ikeLT, myTr, nil)
		if err := clfLoopEnd(t, done, "an expired IKE hard lifetime"); !errors.Is(err, errTimeout) {
			t.Fatalf("an expired IKE lifetime ended the childless loop with %v, want errTimeout", err)
		}
	})

	t.Run("live", func(t *testing.T) {
		ini, _, ps, _, myTr := dpdProbeLink(t)
		clfMakeChildless(t, ini, ps)
		done := clfRunLoop(t, ini, ps, nil, nil, newLifetimeState(3600), myTr, nil)
		select {
		case err := <-done:
			t.Fatalf("the childless loop ended (%v) under a live IKE lifetime", err)
		case <-time.After(2500 * time.Millisecond):
			// sleep(timer): two owner-loop ticks of one second each must pass without an exit.
		}
		close(ps.stopCh)
		if err := clfLoopEnd(t, done, "an operator stop"); err != nil {
			t.Errorf("an operator stop ended the childless loop with %v, want nil", err)
		}
	})
}

// TestCreatedChildStartsItsLifetimeOnTheOwnerLoop checks what the owner loop does with a
// Child SA a peer created on a childless IKE SA (AC-7, AC-4 "starts the Child lifetime at
// install").
//
// Goal: the created Child SA is announced with child-up and runs under its own lifetime
// from install, so it rekeys and expires like any other. Method: a childless responder
// session with a one-second esp-group lifetime runs its owner loop with no IKE lifetime
// and no liveness; the peer's CREATE_CHILD_SA new-child request arrives on the inbound
// queue. Only the Child hard lifetime can end that loop, so an exit with errTimeout
// proves the lifetime started. The bus MUST carry child-up, then child-down from the
// exit's cleanup.
//
// MUTATION: delete `childLT = newLifetimeState(ps.espGroup.Lifetime)` from the
// createdChild arm of maintainSA and the loop never ends. Delete the emitChildUp call
// there and the bus carries no child-up.
func TestCreatedChildStartsItsLifetimeOnTheOwnerLoop(t *testing.T) {
	link := errLink(t)
	ps := link.ps
	ps.peerName = "create-child-loop"
	ps.espGroup = testESPGroup()
	ps.espGroup.Lifetime = 1
	ps.setChildSA(nil)
	msgID := link.resp.ExpectedMsgID

	const peerSPI = 0x0c0ffee1
	inner := peerNewChildRequest(t, ps.espGroup, peerSPI, testNonce(62), nil, false, "0.0.0.0/0", "0.0.0.0/0")
	raw, err := buildEncryptedMessageEx(link.ini, inner, msgID, wire.ExchangeCreateChildSA, initiatorFlag(link.ini))
	if err != nil {
		t.Fatalf("build the peer's new Child SA request: %v", err)
	}

	bus := &sepBus{}
	done := clfRunLoop(t, link.resp, ps, nil, nil, nil, link.myTr, bus)
	ps.inbound <- transport.Packet{Data: raw}

	if err := clfLoopEnd(t, done, "the created Child SA's hard lifetime"); !errors.Is(err, errTimeout) {
		t.Fatalf("the loop ended with %v, want errTimeout from the Child hard lifetime", err)
	}
	bus.mu.Lock()
	events := slices.Clone(bus.types)
	bus.mu.Unlock()
	up := slices.Index(events, Namespace+"/child-up")
	if up < 0 {
		t.Fatalf("the bus carries %v, want child-up for the created Child SA", events)
	}
	down := slices.Index(events, Namespace+"/child-down")
	if down < up {
		t.Errorf("the bus carries %v, want child-down after child-up", events)
	}
}

// TestPeerDeleteOfTheLiveChildEndsTheOwnerLoop checks AC-15: a peer Delete of the Child
// SA carrying the session's traffic still ends the owner loop so the session
// re-establishes, now that a childless IKE SA is a normal state.
//
// Goal: the owner decision Q-5 keeps this behavior unchanged. A session that lost its
// Child SA to the peer does not stay up childless; it ends with errTimeout, which
// PeerSession.run reads as "reconnect". Method: an established responder session with
// its Child SA runs the owner loop; the peer sends a Delete naming its inbound ESP SPI
// (the session's outbound SPI). The loop MUST end with errTimeout, and the bus MUST
// carry child-down.
//
// MUTATION: drop `out.reestablish = true` for a downed child in handleInformationalOwned
// (inbound.go) and the loop keeps running.
func TestPeerDeleteOfTheLiveChildEndsTheOwnerLoop(t *testing.T) {
	local, peer, ps, _, myTr := lcyLoopback(t)
	child := ps.getChildSA()
	if child == nil {
		t.Fatal("the session holds no Child SA")
	}
	msgID := local.ExpectedMsgID
	bus := &sepBus{}
	done := clfRunLoop(t, local, ps, nil, newLifetimeState(3600), newLifetimeState(3600), myTr, bus)

	ps.inbound <- transport.Packet{Data: lcyRequest(t, peer, msgID, lcyESPDeleteChain(child.OutboundSPI))}
	if err := clfLoopEnd(t, done, "a peer Delete of the live Child SA"); !errors.Is(err, errTimeout) {
		t.Fatalf("a peer Delete of the live Child SA ended the loop with %v, want errTimeout", err)
	}
	bus.mu.Lock()
	events := slices.Clone(bus.types)
	bus.mu.Unlock()
	if !slices.Contains(events, Namespace+"/child-down") {
		t.Errorf("the bus carries %v, want child-down", events)
	}
}

// TestPeerDeleteOfTheLiveChildDeletesTheIKESA checks that the session the peer's Child SA
// Delete ends tells the peer its IKE SA is gone, before the reconnect builds a new one.
//
// Goal: RFC 7296 Section 2.4: "Closing the IKE SA implicitly closes all associated Child
// SAs. In this case, an IKE endpoint SHOULD send a Delete payload indicating that it has
// closed the IKE SA unless the other endpoint is no longer responding." The peer just
// sent a request, so it is responding. strongSwan 6.x re-creates a reloaded Child SA in
// place with CREATE_CHILD_SA on the same IKE SA; a Ze that dropped the IKE SA in silence
// answered that request with an unprotected INVALID_IKE_SPI the peer cannot
// authenticate, and the tunnel stayed down (child-rekey-narrowing, strongSwan 6.1.0).
//
// Method: the owner loop runs with the session's Child SA; the peer deletes it. Every
// datagram the peer then reads is decrypted under the IKE SA, and one of them MUST be an
// INFORMATIONAL request carrying a Delete payload for the IKE SA.
//
// MUTATION: drop the sayGoodbye call from the owner loop's reestablish exit
// (established.go) and no IKE SA Delete reaches the peer.
func TestPeerDeleteOfTheLiveChildDeletesTheIKESA(t *testing.T) {
	local, peer, ps, peerTr, myTr := lcyLoopback(t)
	child := ps.getChildSA()
	if child == nil {
		t.Fatal("the session holds no Child SA")
	}
	msgID := local.ExpectedMsgID
	bus := &sepBus{}
	done := clfRunLoop(t, local, ps, nil, newLifetimeState(3600), newLifetimeState(3600), myTr, bus)

	ps.inbound <- transport.Packet{Data: lcyRequest(t, peer, msgID, lcyESPDeleteChain(child.OutboundSPI))}
	if err := clfLoopEnd(t, done, "a peer Delete of the live Child SA"); !errors.Is(err, errTimeout) {
		t.Fatalf("a peer Delete of the live Child SA ended the loop with %v, want errTimeout", err)
	}

	sawResponse := false
	for raw := rtxRecv(t, peerTr); raw != nil; raw = rtxRecv(t, peerTr) {
		hdr := parseMsg(t, raw).Header
		if hdr.Flags&wire.FlagResponse != 0 {
			sawResponse = true
			continue
		}
		if hdr.ExchangeType != wire.ExchangeInformational {
			continue
		}
		for _, del := range lcyDeletes(lcyDecrypt(t, peer, raw)) {
			if del.ProtocolID == wire.ProtocolIKE {
				if !sawResponse {
					t.Error("the IKE SA Delete went out before the answer to the peer's Child SA Delete")
				}
				return
			}
		}
	}
	t.Fatal("the session ended its IKE SA and sent the peer no Delete for it")
}

// TestPeerDeleteOfTheIKESAWithItsChildSendsNoGoodbye checks that a request deleting the
// IKE SA and the live Child SA together draws no Delete of ours for that IKE SA.
//
// Goal: RFC 7296 Section 2.4 asks the endpoint CLOSING an IKE SA to "send a Delete
// payload indicating that it has closed the IKE SA". Here the peer closed it, so the IKE
// SA no longer exists at either end and a Delete for it names nothing. The goodbye the
// reestablish exit sends (TestPeerDeleteOfTheLiveChildDeletesTheIKESA) MUST NOT run for
// it. The session still reconnects, by the exit a peer's IKE SA Delete takes.
//
// Method: the owner loop runs with the session's Child SA; the peer sends one request
// carrying a Delete for the IKE SA and a Delete for the live Child SA. The loop MUST end
// with errSADeletedByPeer, and no datagram the peer then reads is a request.
//
// MUTATION: drop the `out.reestablish = false` guard on ikeDeleted in
// handleInformationalOwned (inbound.go) and the loop ends with errTimeout after sending
// an IKE SA Delete.
func TestPeerDeleteOfTheIKESAWithItsChildSendsNoGoodbye(t *testing.T) {
	local, peer, ps, peerTr, myTr := lcyLoopback(t)
	child := ps.getChildSA()
	if child == nil {
		t.Fatal("the session holds no Child SA")
	}
	msgID := local.ExpectedMsgID
	bus := &sepBus{}
	done := clfRunLoop(t, local, ps, nil, newLifetimeState(3600), newLifetimeState(3600), myTr, bus)

	chain := append([]wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}},
		lcyESPDeleteChain(child.OutboundSPI)...)
	ps.inbound <- transport.Packet{Data: lcyRequest(t, peer, msgID, chain)}
	if err := clfLoopEnd(t, done, "a peer Delete of the IKE SA and its Child SA"); !errors.Is(err, errSADeletedByPeer) {
		t.Fatalf("a peer Delete of the IKE SA and its Child SA ended the loop with %v, want errSADeletedByPeer", err)
	}

	for raw := rtxRecv(t, peerTr); raw != nil; raw = rtxRecv(t, peerTr) {
		if parseMsg(t, raw).Header.Flags&wire.FlagResponse == 0 {
			t.Fatal("the session sent a request after the peer deleted its IKE SA")
		}
	}
}

// TestPeerDeleteOfTheOldIKESAWithTheLiveChildSaysGoodbyeUnderTheNewSA checks the request
// that confirms Ze's answer to a peer IKE SA rekey by deleting the old IKE SA, and also
// deletes the live Child SA.
//
// Goal: RFC 7296 Section 2.8: the rekeyed IKE SA "inherits all of the original IKE SA's
// Child SAs, and the new IKE SA is used for all control messages needed to maintain those
// Child SAs", and the old SA's Delete "MUST be the last request sent over the old IKE
// SA". The Child SA Delete still ends the session (owner decision Q-5), so the peer is
// owed a Delete for the IKE SA it still holds, which is the NEW one (Section 2.4). The
// request deletes an IKE SA, so its answer is empty (Section 1.4.1: "The response to a
// request that deletes the IKE SA is an empty INFORMATIONAL response."). Nothing adopts
// the new SA, so it forgets its keys (Section 2.12).
//
// Method: a second established pair stands in for the rekeyed IKE SA in pendingIKESwap.
// The peer sends one request on the old SA deleting the old IKE SA and the live Child
// SA. The loop MUST end with errTimeout, the response MUST carry no Delete payload, the
// only request the peer reads MUST be an IKE SA Delete under the new SA, and the new SA
// MUST have forgotten its nonce.
//
// MUTATION: say goodbye under the loop's own SA in maintainSA's reestablish exit
// (established.go) and the Delete goes out under the old SA's keys, which the peer
// deleted.
func TestPeerDeleteOfTheOldIKESAWithTheLiveChildSaysGoodbyeUnderTheNewSA(t *testing.T) {
	local, peer, ps, peerTr, myTr := lcyLoopback(t)
	newPeer, newSA, _ := establishPSK(t)
	if newSA.LocalNonce == nil {
		t.Fatal("setup: the new IKE SA holds no nonce to forget")
	}
	// A rekeyed IKE SA talks to the same peer, from the same address, as the old one.
	newSA.PeerCfg.LocalAddress = local.PeerCfg.LocalAddress
	newSA.PeerCfg.RemoteAddress = local.PeerCfg.RemoteAddress
	ps.pendingIKESwap = newSA
	child := ps.getChildSA()
	if child == nil {
		t.Fatal("the session holds no Child SA")
	}
	msgID := local.ExpectedMsgID
	done := clfRunLoop(t, local, ps, nil, newLifetimeState(3600), newLifetimeState(3600), myTr, &sepBus{})

	chain := append([]wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}},
		lcyESPDeleteChain(child.OutboundSPI)...)
	ps.inbound <- transport.Packet{Data: lcyRequest(t, peer, msgID, chain)}
	if err := clfLoopEnd(t, done, "a peer Delete of the old IKE SA and the live Child SA"); !errors.Is(err, errTimeout) {
		t.Fatalf("the loop ended with %v, want errTimeout", err)
	}
	if newSA.LocalNonce != nil {
		t.Error("the new IKE SA nobody adopted kept its keying material")
	}

	goodbye := false
	for raw := rtxRecv(t, peerTr); raw != nil; raw = rtxRecv(t, peerTr) {
		m := parseMsg(t, raw)
		if m.Header.Flags&wire.FlagResponse != 0 {
			if dels := lcyDeletes(lcyDecrypt(t, peer, raw)); len(dels) != 0 {
				t.Errorf("the answer to a request deleting the IKE SA carries %d Delete payloads, want none", len(dels))
			}
			continue
		}
		if m.Header.InitiatorSPI != newSA.InitiatorSPI {
			t.Fatal("a request went out under the old IKE SA, which the peer deleted")
		}
		for _, del := range lcyDeletes(lcyDecrypt(t, newPeer, raw)) {
			if del.ProtocolID == wire.ProtocolIKE {
				goodbye = true
			}
		}
	}
	if !goodbye {
		t.Fatal("the session sent no Delete for the new IKE SA the peer still holds")
	}
}

// TestDatagramQueuedBehindThePeerIKESADeleteSendsNoGoodbye checks that a request queued
// behind the peer's IKE SA Delete, which raises the reestablish exit before the next tick
// ends the loop, sends no goodbye for the IKE SA the peer already deleted.
//
// Goal: RFC 7296 Section 2.4 asks the endpoint CLOSING an IKE SA to "send a Delete
// payload indicating that it has closed the IKE SA". The peer closed this one at both
// ends, so a Delete names nothing it holds.
//
// Method: Ze holds the request window for an INFORMATIONAL request the peer will never
// answer, having deleted the SA. Two requests are queued at once: an IKE SA Delete, then
// a Delete of the live Child SA. The loop MUST end with errTimeout (the reestablish exit)
// inside goodbyeWindowWait, and no datagram the peer reads is a request.
//
// MUTATION: drop the first peerDeletedIKE guard in sayGoodbye (established.go) and the
// loop waits goodbyeWindowWait for an answer a peer that deleted the SA never sends.
func TestDatagramQueuedBehindThePeerIKESADeleteSendsNoGoodbye(t *testing.T) {
	local, peer, ps, peerTr, myTr := lcyLoopback(t)
	child := ps.getChildSA()
	if child == nil {
		t.Fatal("the session holds no Child SA")
	}
	if !local.reserveRequestWindow() {
		t.Fatal("setup: the request window is not free")
	}
	local.NextMsgID++
	msgID := local.ExpectedMsgID
	ikeDelete := lcyRequest(t, peer, msgID, []wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}})
	childDelete := lcyRequest(t, peer, msgID+1, lcyESPDeleteChain(child.OutboundSPI))
	done := clfRunLoop(t, local, ps, nil, newLifetimeState(3600), newLifetimeState(3600), myTr, &sepBus{})
	start := time.Now()
	ps.inbound <- transport.Packet{Data: ikeDelete}
	ps.inbound <- transport.Packet{Data: childDelete}
	if err := clfLoopEnd(t, done, "a Child SA Delete behind an IKE SA Delete"); !errors.Is(err, errTimeout) {
		t.Fatalf("the loop ended with %v, want errTimeout from the queued Child SA Delete", err)
	}
	if waited := time.Since(start); waited >= goodbyeWindowWait {
		t.Errorf("the loop waited %s to say goodbye to an IKE SA the peer deleted", waited)
	}
	for raw := rtxRecv(t, peerTr); raw != nil; raw = rtxRecv(t, peerTr) {
		if parseMsg(t, raw).Header.Flags&wire.FlagResponse == 0 {
			t.Fatal("the session sent a request after the peer deleted its IKE SA")
		}
	}
}

// TestPeerIKESADeleteDuringTheGoodbyeWaitSendsNoGoodbye checks the same fact for an IKE
// SA Delete that sayGoodbye itself reads while it waits for our outstanding request.
//
// Goal: as TestDatagramQueuedBehindThePeerIKESADeleteSendsNoGoodbye. The wait reads
// inbound datagrams (waitForRequestWindow), so it can process the peer's Delete and then
// the answer that frees the window.
//
// Method: Ze holds the request window for an INFORMATIONAL request. Three datagrams are
// queued: a Delete of the live Child SA (the reestablish exit), an IKE SA Delete, and the
// peer's answer to our request. No datagram the peer reads is a request.
//
// MUTATION: drop the second peerDeletedIKE guard in sayGoodbye (established.go), after
// the wait, and an IKE SA Delete goes out under the deleted SA.
func TestPeerIKESADeleteDuringTheGoodbyeWaitSendsNoGoodbye(t *testing.T) {
	local, peer, ps, peerTr, myTr := lcyLoopback(t)
	child := ps.getChildSA()
	if child == nil {
		t.Fatal("the session holds no Child SA")
	}
	if !local.reserveRequestWindow() {
		t.Fatal("setup: the request window is not free")
	}
	ours := local.requestMsgID
	local.NextMsgID++
	msgID := local.ExpectedMsgID
	childDelete := lcyRequest(t, peer, msgID, lcyESPDeleteChain(child.OutboundSPI))
	ikeDelete := lcyRequest(t, peer, msgID+1, []wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}})
	answer, err := buildEncryptedMessageEx(peer, nil, ours, wire.ExchangeInformational, initiatorFlag(peer)|wire.FlagResponse)
	if err != nil {
		t.Fatalf("build the peer's answer: %v", err)
	}
	done := clfRunLoop(t, local, ps, nil, newLifetimeState(3600), newLifetimeState(3600), myTr, &sepBus{})
	ps.inbound <- transport.Packet{Data: childDelete}
	ps.inbound <- transport.Packet{Data: ikeDelete}
	ps.inbound <- transport.Packet{Data: answer}
	if err := clfLoopEnd(t, done, "an IKE SA Delete during the goodbye wait"); !errors.Is(err, errTimeout) {
		t.Fatalf("the loop ended with %v, want errTimeout", err)
	}
	for raw := rtxRecv(t, peerTr); raw != nil; raw = rtxRecv(t, peerTr) {
		if parseMsg(t, raw).Header.Flags&wire.FlagResponse == 0 {
			t.Fatal("the session sent a request after the peer deleted its IKE SA")
		}
	}
}
