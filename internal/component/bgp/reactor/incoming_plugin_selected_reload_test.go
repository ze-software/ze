// Design: docs/architecture/behavior/peer-lifecycle.md -- selection rechecks after plugin RPC.
// Related: incoming_open_publication_test.go -- actual listener and callback transports.
package reactor

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestIncomingPluginChangeDuringSelectedValidation models a transaction's
// atomic capability publication before its BGP reconciliation. The selected
// policy RPC was already in flight, so only the post-RPC check can stop the
// retained old offer before peer-up and engine publication.
func TestIncomingPluginChangeDuringSelectedValidation(t *testing.T) {
	for _, bridged := range []bool{false, true} {
		t.Run(map[bool]string{false: "rpc", true: "bridged"}[bridged], func(t *testing.T) {
			entered, proceed := make(chan struct{}), make(chan struct{})
			release := sync.OnceFunc(func() { close(proceed) })
			observed := &incomingOpenObservations{
				onSelected: func() { close(entered); <-proceed },
			}
			store := &pluginStore{}
			declaration := func(restart byte) plugin.InjectedCapability {
				return plugin.InjectedCapability{Code: 64, Value: []byte{0, restart}, Plugin: "reload-probe"}
			}
			store.set(declaration(120))
			r, peer, _, outgoing := incomingOpenListenerPeer(t, true, bridged, observed, func(r *Reactor, _ *Peer) {
				r.pluginCapabilitySeam = store.get
			})
			t.Cleanup(release)
			delayOpenReadType(t, outgoing, 1)
			incoming := incomingListenerDial(t, r)
			delayOpenReadType(t, incoming, 1)
			remote := delayOpenRemote(0x03030303)
			remote.OptionalParams = []byte{2, 4, 64, 2, 0, 90}
			delayOpenWrite(t, incoming, remote)
			delayOpenReadType(t, incoming, 4)
			delayOpenWrite(t, incoming, &message.Keepalive{})
			incomingCorrectionAwait(t, entered, "selected policy callback before return")
			selected := peer.currentSession()
			if selected == nil {
				t.Fatal("selected callback has no owning Session")
			}
			// configTxBridge.installStagedCapabilities installs this complete
			// store before the BGP participant reconciles any Session.
			store.set(declaration(90))
			release()
			delayOpenAwait(t, func() bool { return peer.Stats().ConnectionsEstablished != 0 || selected.Conn() == nil })
			if got := peer.Stats().ConnectionsEstablished; got != 0 {
				t.Fatalf("stale selected offer published peer-up %d times", got)
			}
			packet := delayOpenReadType(t, incoming, 3)
			if !bytes.Equal(packet[message.HeaderLen:], []byte{6, 5}) {
				t.Fatalf("changed selected offer refusal=%x, want 0605", packet[message.HeaderLen:])
			}
			var octet [1]byte
			if n, err := incoming.Read(octet[:]); n != 0 || !errors.Is(err, io.EOF) {
				t.Fatalf("stale selected connection remains open: bytes=%d error=%v", n, err)
			}
			if got := peer.Stats().ConnectionsEstablished; got != 0 {
				t.Fatalf("stale selected offer published peer-up %d times", got)
			}
		})
	}
}
