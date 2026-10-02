package ls_export

import (
	"context"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/epe"
)

// TestRFC9086EveryBGPNLRICarriesLocalRouterIDAndASN proves that both BGP-LS
// NLRIs Ze distributes for protocol BGP, the Node NLRI and the Link NLRI,
// carry the local BGP Router-ID (TLV 516) and the local ASN (TLV 512) in their
// Local Node Descriptors.
//
// Method: native EPE is configured for peer 198.51.100.2 and the session comes
// up with local router-id 192.0.2.1 and local AS 65000. Each of the two
// emitted NLRIs is read from the wire: NLRI type (1 Node, 2 Link), Protocol-ID
// 7 (BGP), then TLV 256 holding TLV 516 = c0 00 02 01 and TLV 512 =
// 00 00 fd e8, each exactly once.
//
// RFC requirement: RFC9086-4.2-1 positive -- the Node NLRI (type 1) and the
// Link NLRI (type 2), Protocol-ID 7, each carry Local Node Descriptors with
// BGP Router-ID c0 00 02 01 (TLV 516) and ASN 00 00 fd e8 (TLV 512).
func TestRFC9086EveryBGPNLRICarriesLocalRouterIDAndASN(t *testing.T) {
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
	source := epe.NewSource(bus)
	peer := netip.MustParseAddr("198.51.100.2")
	require.NoError(t, source.Configure(epe.Config{Base: 16000, Size: 100, Peers: map[netip.Addr]epe.PeerConfig{peer: {Index: 7}}}))
	require.NoError(t, source.State(epeProofEvent(t, "up")))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 2)

	for i, nlriType := range []uint16{1, 2} {
		nlri := exportCommandBytes(t, capture.commands[i], "nlri")
		require.Equal(t, nlriType, binary.BigEndian.Uint16(nlri), "command %d NLRI type", i)
		require.Equal(t, byte(7), nlri[4], "NLRI type %d Protocol-ID is BGP", nlriType)
		local := exportTLVValues(t, nlri[13:], 256)
		require.Len(t, local, 1, "NLRI type %d has one Local Node Descriptors TLV", nlriType)
		require.Equal(t, [][]byte{{192, 0, 2, 1}}, exportTLVValues(t, local[0], 516),
			"NLRI type %d local BGP Router-ID", nlriType)
		require.Equal(t, [][]byte{{0, 0, 253, 232}}, exportTLVValues(t, local[0], 512),
			"NLRI type %d local ASN", nlriType)
	}
}

// TestRFC9086NativePeerRequiresLocalRouterID is the counterpart for TLV 516:
// a session whose local BGP Router-ID is absent or 0.0.0.0 has no valid BGP
// Identifier to put in TLV 516, so native origination is refused before a
// label is installed and nothing reaches the collector.
//
// RFC requirement: RFC9086-4.2-1 negative -- with the local router-id 0.0.0.0
// or absent, State returns an error, no label is installed and no BGP-LS NLRI
// is emitted.
func TestRFC9086NativePeerRequiresLocalRouterID(t *testing.T) {
	for _, identity := range []struct{ name, from, to string }{
		{"zero local router-id", `"router-id":"192.0.2.1"`, `"router-id":"0.0.0.0"`},
		{"absent local router-id", `"router-id":"192.0.2.1",`, ``},
	} {
		t.Run(identity.name, func(t *testing.T) {
			exporter, capture := exportFixture(t)
			bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
			source := epe.NewSource(bus)
			peer := netip.MustParseAddr("198.51.100.2")
			require.NoError(t, source.Configure(epe.Config{Base: 16000, Size: 100, Peers: map[netip.Addr]epe.PeerConfig{peer: {Index: 7}}}))
			event := epeProofEvent(t, "up")
			event.Peer = []byte(strings.Replace(string(event.Peer), identity.from, identity.to, 1))
			require.NotContains(t, string(event.Peer), "192.0.2.1", "the fixture lost the local router-id")
			require.Error(t, source.State(event))
			require.Empty(t, bus.labels)
			require.NoError(t, exporter.reconcile(context.Background()))
			require.Empty(t, capture.commands)
		})
	}
}
