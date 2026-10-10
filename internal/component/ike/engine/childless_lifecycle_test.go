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
