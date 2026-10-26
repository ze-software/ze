// Design: docs/architecture/api/process-protocol.md -- StructuredEvent, SessionGeneration
// Related: events.go -- getStructuredStateEvent

package server

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestStructuredStateCarriesSessionGeneration proves the structured state
// event a DirectBridge plugin receives names the session a down ended, as the
// text and JSON forms do. Method: the real producer over a down PeerInfo.
func TestStructuredStateCarriesSessionGeneration(t *testing.T) {
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001),
		SessionGeneration: 9}
	se := getStructuredStateEvent(&peer, rpc.SessionStateDown, "tcp-failure", nil)
	defer rpc.PutStructuredEvent(se)
	if se.SessionGeneration != 9 {
		t.Fatalf("SessionGeneration = %d, want 9", se.SessionGeneration)
	}
}
