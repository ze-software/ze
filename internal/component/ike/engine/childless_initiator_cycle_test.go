// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the childless IKE SA
// Related: fsm.go -- runInitiator emits sa-up and hands the SA to runEstablished
// Related: established.go -- runEstablished keeps a childless IKE SA in maintainSA

package engine

import (
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestInitiatorCycleKeepsChildlessIKESA checks AC-2 through the whole initiator cycle,
// not handleAuthResponse alone.
//
// Goal: an initiator whose peer authenticates and refuses the Child SA runs the IKE SA:
// sa-up is emitted, the session stays in its owner loop with no Child SA, and nothing
// ends the cycle (the deleted deleteChildlessIKESA ended it with a Delete and sa-down).
// Method: runInitiator drives a real PSK handshake against a far end whose esp-group
// shares no proposal with ze's, so the far end's IKE_AUTH response authenticates and
// carries NO_PROPOSAL_CHOSEN. Once the owner loop holds the SA, the bus MUST carry
// exactly sa-up and no child-up or sa-down, the session MUST hold no Child SA, and the
// cycle MUST still be running after a settle period.
//
// PREVENTS: an IKE SA that establishes and is then torn down because its Child SA was
// refused.
func TestInitiatorCycleKeepsChildlessIKESA(t *testing.T) {
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "childless-cycle-psk")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"

	ps := &PeerSession{
		peerName:  "ze",
		peerCfg:   iniPeer,
		ikeGroup:  ikeGroup,
		espGroup:  espGroup,
		stopCh:    make(chan struct{}),
		inbound:   make(chan transport.Packet, inboundQueueDepth),
		supersede: make(chan struct{}, 1),
	}
	// The far end's esp-group shares no proposal with ze's, so it refuses the Child SA.
	refusing := rcxReloadedESPGroup()
	respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: refusing}
	table := NewSATable()
	far := &icyFarEnd{}
	bus := &sepBus{}

	old := afterFunc
	t.Cleanup(func() { afterFunc = old })
	afterFunc = func(_ time.Duration) <-chan time.Time {
		if cur := ps.getSA(); cur != nil {
			far.advance(cur, respPS, table, log)
		}
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}

	done := make(chan struct{})
	go func() {
		_ = ps.runInitiator(iniPeer, ikeGroup, table, nil, bus, log)
		close(done)
	}()
	t.Cleanup(func() {
		close(ps.stopCh)
		<-done
	})
	icyWaitFor(t, "the owner loop adopting the established SA", func() bool {
		return ps.ownedSA.Load() != nil
	})
	if _, err := far.get(); err != nil {
		t.Fatalf("the driven handshake failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("the initiator cycle ended after a refused Child SA, want the IKE SA kept")
	default:
	}
	if !ps.ownedSA.Load().IKEAuthChildless {
		t.Error("the established SA is not marked childless")
	}
	if ps.getChildSA() != nil {
		t.Error("the session holds a Child SA the far end refused")
	}
	bus.mu.Lock()
	events := slices.Clone(bus.types)
	bus.mu.Unlock()
	want := []string{Namespace + "/sa-up"}
	if !slices.Equal(events, want) {
		t.Errorf("the bus carries %v, want exactly %v for a kept childless IKE SA", events, want)
	}
}
