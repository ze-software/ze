// Design: docs/architecture/behavior/peer-lifecycle.md -- reload owns selected inbound receipts.
// Related: incoming_plugin_reload_test.go -- constructor capture and fresh proposal controls.
package reactor

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestIncomingPluginReloadRetiresStaleHandoff reloads after a valid collision
// already placed its exact incoming Session in the next-cycle receipt.
// The old current Session did not use GR, so only a complete receipt census
// can find the stale offer. The same Peer must then accept a current offer.
func TestIncomingPluginReloadRetiresStaleHandoff(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Connection = ConnectionBoth
	r, peer := newSwapTestReactor(t, settings, settings)
	cleanupPendingFixturePeer(t, peer)
	store := &pluginStore{}
	declaration := func(restart byte) plugin.InjectedCapability {
		return plugin.InjectedCapability{Code: 64, Value: []byte{0, restart}, Plugin: "reload-probe"}
	}
	store.set(declaration(120))
	r.pluginCapabilitySeam = store.get
	original := peer.newSession()
	original.SetClock(sim.NewFakeClock(time.Now()))
	peer.mu.Lock()
	peer.session = original
	peer.mu.Unlock()
	outgoing, _ := incomingCorrectionStart(t, original, false)
	processed := incomingCorrectionProcess(t, original, outgoing)
	delayOpenWrite(t, outgoing, delayOpenRemote(0x02020202))
	delayOpenReadType(t, outgoing, 4)
	if err := <-processed; err != nil {
		t.Fatal(err)
	}
	incoming, local := collisionProofTCPPair(t)
	t.Cleanup(func() { closeConnQuietly(incoming) })
	r.acceptOrReject(local, peer, nil)
	delayOpenReadType(t, incoming, 1)
	remote := delayOpenRemote(0x03030303)
	remote.OptionalParams = []byte{2, 4, 64, 2, 0, 90}
	delayOpenWrite(t, incoming, remote)
	var stale *incomingCandidate
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		stale = inboundCandidate(peer.inbound)
		return stale != nil
	})
	incomingCorrectionAwait(t, stale.handoffDone, "old declaration's valid collision handoff")
	if original.Conn() != nil {
		t.Fatal("the old-store collision never retired its original Session")
	}
	store.set(declaration(90))
	if err := (&reactorAPIAdapter{r: r}).Reload(); err != nil {
		t.Fatal(err)
	}
	peer.mu.RLock()
	retired := peer.inbound == nil
	reserved := peer.collisionHandoff
	peer.mu.RUnlock()
	if !retired {
		t.Fatal("reload kept a stale incoming handoff outside its current-Session census")
	}
	if reserved {
		t.Fatal("retiring a stale handoff kept its admission reservation")
	}
	packet := delayOpenReadType(t, incoming, 3)
	if !bytes.Equal(packet[message.HeaderLen:], []byte{6, 5}) {
		t.Fatalf("stale handoff refusal=%x, want 0605", packet[message.HeaderLen:])
	}
	var octet [1]byte
	if n, err := incoming.Read(octet[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("stale handoff remains open: bytes=%d error=%v", n, err)
	}
	// This non-running fixture now starts the next ordinary Session explicitly,
	// through the same factory and TCP accept rail that runOnce uses.
	freshSession := peer.newSession()
	freshSession.SetClock(sim.NewFakeClock(time.Now()))
	t.Cleanup(func() { freshSession.closeConn(); freshSession.stopSessionTimers() })
	peer.mu.Lock()
	peer.session = freshSession
	peer.mu.Unlock()
	if err := freshSession.startDamped(); err != nil {
		t.Fatal(err)
	}
	fresh, freshLocal := collisionProofTCPPair(t)
	t.Cleanup(func() { closeConnQuietly(fresh) })
	r.acceptOrReject(freshLocal, peer, nil)
	freshOpenPacket := delayOpenReadType(t, fresh, 1)
	freshOpen, err := message.UnpackOpen(freshOpenPacket[message.HeaderLen:])
	if err != nil {
		t.Fatal(err)
	}
	freshCaps, err := capability.ParseFromOptionalParams(freshOpen.OptionalParams, freshOpen.ExtendedParams)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cap := range freshCaps {
		if cap.Code() != capability.CodeGracefulRestart {
			continue
		}
		wire := make([]byte, cap.Len())
		cap.WriteTo(wire, 0)
		if !bytes.Equal(wire, []byte{64, 2, 0, 90}) {
			t.Fatalf("fresh TCP OPEN changed current GR declaration: %x", wire)
		}
		found = true
	}
	if !found {
		t.Fatal("fresh TCP OPEN omitted current GR declaration")
	}
	processed = incomingCorrectionProcess(t, freshSession, fresh)
	delayOpenWrite(t, fresh, remote)
	delayOpenReadType(t, fresh, 4)
	if err := <-processed; err != nil {
		t.Fatal(err)
	}
	incomingCorrectionConfirm(t, freshSession, fresh)
	if freshSession.State() != fsm.StateEstablished {
		t.Fatal("a current incoming offer could not establish after stale handoff retirement")
	}
}
