// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the childless IKE SA
// Related: reconcile.go -- reconcilePeers, the reload that keeps a childless session
// Related: childless_lifecycle_test.go -- the owner loop of a childless session

package engine

import (
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rcxReloadedESPGroup shares no proposal with testESPGroup, so a Child SA request built
// from it is one the session's original esp-group refuses with NO_PROPOSAL_CHOSEN.
func rcxReloadedESPGroup() ipsec.ESPGroup {
	group := testESPGroup()
	group.Name = "esp-reloaded"
	group.Proposals = []ipsec.ESPProposal{{
		Number:     1,
		Encryption: ipsec.EncryptionAES128,
		Hash:       ipsec.HashSHA256,
	}}
	return group
}

// rcxConfig is the configuration a reload hands the session: its own peer block, and the
// two groups under the names that block uses.
func rcxConfig(ps *PeerSession, ike ipsec.IKEGroup, esp ipsec.ESPGroup) *ipsec.IPsecConfig {
	return &ipsec.IPsecConfig{
		IKEGroups: map[string]ipsec.IKEGroup{ps.peerCfg.IKEGroup: ike},
		ESPGroups: map[string]ipsec.ESPGroup{ps.peerCfg.ESPGroup: esp},
		Peers:     map[string]ipsec.SiteToSitePeer{ps.peerName: ps.peerCfg},
	}
}

// rcxRunLoop runs the owner loop of an established session the way runEstablished does
// for reconcilePeers: ownedSA names the SA, and done closes when the loop returns, so
// stopPeerSession's Stop can join it. The cleanup stops a loop the test left running.
func rcxRunLoop(t *testing.T, link errPair, bus *sepBus) {
	t.Helper()
	ps := link.ps
	ps.stopCh = make(chan struct{})
	ps.done = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.inbound = make(chan transport.Packet, inboundQueueDepth)
	ps.espReloads = make(chan espReload)
	ps.ownedSA.Store(link.resp)
	t.Cleanup(func() {
		ps.stopOnce.Do(func() { close(ps.stopCh) })
		<-ps.done
	})
	go func() {
		defer close(ps.done)
		err := ps.maintainSA(link.resp, nil, newLifetimeState(3600), newLifetimeState(3600),
			testIKEGroup(), NewSATable(), nil, link.myTr, bus, slogutil.DiscardLogger())
		if err != nil {
			t.Errorf("the owner loop ended with %v", err)
		}
	}()
}

// rcxStopRestarted stops every session reconcilePeers started in place of the test's own.
func rcxStopRestarted(t *testing.T, active map[string]*PeerSession, own *PeerSession) {
	t.Helper()
	t.Cleanup(func() {
		for _, ps := range active {
			if ps != own {
				ps.Stop()
			}
		}
	})
}

// TestReloadKeepsChildlessSessionForESPGroupChange checks the owner decision of
// 2026-10-10: a reload that changes only the esp-group of a session whose IKE SA holds no
// Child SA keeps the IKE SA, and the Child SA created afterwards uses the new esp-group.
//
// Goal: an IKE SA that refused its Child SA in IKE_AUTH because the esp-groups did not
// match recovers when the operator fixes the esp-group, through a CREATE_CHILD_SA on the
// same IKE SA, not through a reconnect. A childless session runs no ESP SA against the old
// policy, so RFC 4301 Section 4.4.2's concern (an SA the new policy no longer allows)
// does not arise. Method: an established responder session loses its Child SA and its
// owner loop runs. reconcilePeers is handed the same peer and ike-group with an esp-group
// that shares no proposal with the old one. The active map MUST still hold the same
// session. The peer then asks for a new Child SA offering only the new esp-group's
// proposal: the session MUST create it, with the new algorithm, and emit child-up.
//
// VALIDATES: a reload changing only the esp-group keeps a childless IKE SA, and the next
// Child SA creation uses the reloaded proposals.
// PREVENTS: a recovery that reads as a CREATE_CHILD_SA but is a reconnect; before this
// change the reload stopped the session, so the active map held a fresh one.
func TestReloadKeepsChildlessSessionForESPGroupChange(t *testing.T) {
	link := errLink(t)
	ps := link.ps
	ps.peerName = "reload-childless"
	clfMakeChildless(t, link.resp, ps)
	active := map[string]*PeerSession{ps.peerName: ps}
	rcxStopRestarted(t, active, ps)
	bus := &sepBus{}
	rcxRunLoop(t, link, bus)

	reloaded := rcxReloadedESPGroup()
	reconcilePeers(rcxConfig(ps, ps.ikeGroup, reloaded), active, NewSATable(), nil, nil, nil, slog.Default())
	if active[ps.peerName] != ps {
		t.Fatal("an esp-group change restarted a childless session, want the IKE SA kept")
	}
	if got := ps.getESPGroup(); !got.Equal(reloaded) {
		t.Fatalf("the session runs esp-group %q, want the reloaded %q", got.Name, reloaded.Name)
	}

	const peerSPI = 0x0e5bead1
	inner := peerNewChildRequest(t, reloaded, peerSPI, testNonce(63), nil, false, "0.0.0.0/0", "0.0.0.0/0")
	raw, err := buildEncryptedMessageEx(link.ini, inner, link.resp.ExpectedMsgID,
		wire.ExchangeCreateChildSA, initiatorFlag(link.ini))
	if err != nil {
		t.Fatalf("build the peer's new Child SA request: %v", err)
	}
	ps.inbound <- transport.Packet{Data: raw}

	deadline := time.Now().Add(clfLoopWait)
	child := ps.getChildSA()
	for child == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		child = ps.getChildSA()
	}
	if child == nil {
		t.Fatal("the peer's request for the reloaded proposal created no Child SA")
	}
	if got := child.ESPGroup.Proposals[0].Encryption; got != ipsec.EncryptionAES128 {
		t.Errorf("the created Child SA runs %v, want the reloaded esp-group's aes128", got)
	}
	bus.mu.Lock()
	events := slices.Clone(bus.types)
	bus.mu.Unlock()
	if !slices.Contains(events, Namespace+"/child-up") {
		t.Errorf("the bus carries %v, want child-up for the created Child SA", events)
	}
}

// TestReloadRestartsSessionOutsideTheChildlessESPChange checks that the decision above is
// as narrow as the owner made it: every other reload of a running session still restarts
// it.
//
// Goal: the kept session is the exception for a childless IKE SA and an esp-group-only
// edit. A session that holds a Child SA runs ESP against the old policy, and an ike-group
// edit changes the IKE SA itself, so both still restart (RFC 7296 Section 2.9.2: an SA
// against the new policy "should have been already deleted after the policy change took
// effect"). A Child SA being negotiated from the old esp-group, by our own creation or by a
// parallel responder handshake, is the same case one exchange early. Method: for each case an established session runs its owner loop, and
// reconcilePeers is handed the edit. The active map MUST hold a fresh session.
//
// VALIDATES: a session with a Child SA, and any ike-group edit, still restart on reload.
// PREVENTS: the childless exception widening to a session that runs ESP or to an IKE edit.
func TestReloadRestartsSessionOutsideTheChildlessESPChange(t *testing.T) {
	tests := []struct {
		name      string
		childless bool
		ike       func(ipsec.IKEGroup) ipsec.IKEGroup
		esp       ipsec.ESPGroup
		// inFlight puts a Child SA negotiation from the old esp-group in progress.
		inFlight func(*PeerSession)
	}{
		{
			name:      "esp-group change with a Child SA",
			childless: false,
			ike:       func(g ipsec.IKEGroup) ipsec.IKEGroup { return g },
			esp:       rcxReloadedESPGroup(),
		},
		{
			name:      "ike-group change on a childless SA",
			childless: true,
			ike: func(g ipsec.IKEGroup) ipsec.IKEGroup {
				g.Lifetime++
				return g
			},
			esp: testESPGroup(),
		},
		{
			name:      "ike-group and esp-group change on a childless SA",
			childless: true,
			ike: func(g ipsec.IKEGroup) ipsec.IKEGroup {
				g.Lifetime++
				return g
			},
			esp: rcxReloadedESPGroup(),
		},
		{
			name:      "esp-group change while our creation is in flight",
			childless: true,
			ike:       func(g ipsec.IKEGroup) ipsec.IKEGroup { return g },
			esp:       rcxReloadedESPGroup(),
			inFlight: func(ps *PeerSession) {
				ps.pendingRekey = &pendingRekey{kind: rekeyCreate, offered: testESPGroup()}
			},
		},
		{
			name:      "esp-group change while a parallel handshake is pending",
			childless: true,
			ike:       func(g ipsec.IKEGroup) ipsec.IKEGroup { return g },
			esp:       rcxReloadedESPGroup(),
			inFlight:  func(ps *PeerSession) { ps.setPendingSA(testSA()) },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			link := errLink(t)
			ps := link.ps
			ps.peerName = "reload-restarts"
			if tc.childless {
				clfMakeChildless(t, link.resp, ps)
			}
			if !tc.childless && ps.getChildSA() == nil {
				t.Fatal("setup: the session holds no Child SA")
			}
			if tc.inFlight != nil {
				tc.inFlight(ps)
			}
			active := map[string]*PeerSession{ps.peerName: ps}
			rcxStopRestarted(t, active, ps)
			rcxRunLoop(t, link, &sepBus{})

			reconcilePeers(rcxConfig(ps, tc.ike(ps.ikeGroup), tc.esp), active, NewSATable(), nil, nil, nil, slog.Default())
			if active[ps.peerName] == ps {
				t.Fatal("the reload kept the running session, want it restarted")
			}
		})
	}
}
