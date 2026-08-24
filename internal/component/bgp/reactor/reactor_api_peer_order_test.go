// Design: docs/architecture/core-design.md — reactor API adapter for plugin integration
// Related: reactor_api.go — reactorAPIAdapter.Peers, the producer under test
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPeersAnswersSortedOrder proves that Peers() answers one deterministic order.
//
// Goal: the row operators the answer shapes publish on `show bgp peer ...`
// (`| first`, `| last`, `| display`) must select the same peer on every run.
// Method: register eight peers in an order that is not sorted, call Peers()
// many times, and require the peer-key order on every call.
//
// VALIDATES: reactorAPIAdapter.Peers() answers in netip.AddrPort peer-key order.
// PREVENTS: Go map iteration randomization reaching the CLI, where `| first 1`
// answers a different peer on each call.
func TestPeersAnswersSortedOrder(t *testing.T) {
	// Insertion order, the lexical order of the address strings, and the netip
	// byte order all disagree here, so an implementation that answers any of the
	// other two fails. "192.0.2.10" sorts before "192.0.2.2" as text and after it
	// as an address. The two peers on 192.0.2.2 differ only by port, which is the
	// tie the address alone cannot break (peer_settings.go, PeerKey).
	inserted := []struct {
		name string
		addr string
		port uint16
	}{
		{"sorted-5", "198.51.100.1", DefaultBGPPort},
		{"sorted-4", "192.0.2.10", DefaultBGPPort},
		{"sorted-7", "2001:db8::1", DefaultBGPPort},
		{"sorted-3", "192.0.2.2", 1790},
		{"sorted-6", "203.0.113.5", DefaultBGPPort},
		{"sorted-2", "192.0.2.2", DefaultBGPPort},
		{"sorted-8", "2001:db8::2", DefaultBGPPort},
		{"sorted-1", "10.0.0.1", DefaultBGPPort},
	}
	// netip.Addr.Compare orders by bit length first, so every IPv4 peer precedes
	// every IPv6 peer, then by address bytes, then netip.AddrPort.Compare breaks
	// the remaining tie on the port.
	expected := []string{
		"sorted-1", "sorted-2", "sorted-3", "sorted-4",
		"sorted-5", "sorted-6", "sorted-7", "sorted-8",
	}

	r := New(&Config{})
	for _, in := range inserted {
		settings := NewPeerSettings(mustParseAddr(in.addr), 65000, 65001, 0x01010101)
		settings.Name = in.name
		settings.Port = in.port
		r.peers[settings.PeerKey()] = NewPeer(settings)
	}
	require.Len(t, r.peers, len(inserted), "every peer key is distinct")

	adapter := &reactorAPIAdapter{r: r}

	// Go randomizes where a map range starts, so with eight peers a single call
	// answers the sorted order by luck with probability at most 1/8. Thirty-two
	// calls put that ceiling at 8^-32, near 5e-29, so a range-order
	// implementation fails this test rather than flaking it.
	const callCount = 32

	for call := range callCount {
		peers := adapter.Peers()
		require.Len(t, peers, len(inserted), "call %d answers every peer", call)

		names := make([]string, 0, len(peers))
		for i := range peers {
			names = append(names, peers[i].Name)
		}
		require.Equal(t, expected, names, "call %d answers peer-key order", call)
	}
}
