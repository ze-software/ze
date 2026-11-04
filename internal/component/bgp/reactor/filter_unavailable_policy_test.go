// Design: docs/architecture/core-design.md -- Plugin filters join the chain
package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestClosedFilterSlotAppliesDeclaredOnError proves AC-33 at the consumer:
// an unavailable slot rejects under reject and passes the UPDATE unchanged
// under accept, rather than substituting a guessed verdict for on-error.
func TestClosedFilterSlotAppliesDeclaredOnError(t *testing.T) {
	for _, onError := range []rpc.OnErrorPolicy{rpc.OnErrorReject, rpc.OnErrorAccept} {
		t.Run(onError.String(), func(t *testing.T) {
			transport := &unavailableTransport{
				reads:   []string{rpc.FilterFieldRaw},
				onError: onError,
			}
			reactor := &Reactor{filterTransportSeam: transport}
			addr := netip.MustParseAddr("192.0.2.4")
			peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
				ImportPolicy: mixedPolicy(t, "", policychain.Member{Name: "f", Plugin: "probe"})})
			body := libraryTestBody(100)
			got := reactor.runIngressPolicyChain(peer, addr, nil, testWireUpdate(body), body)
			if got.accept != (onError == rpc.OnErrorAccept) {
				t.Fatalf("closed slot accept = %v, declared on-error %s", got.accept, onError)
			}
			if got.modifiedPayload != nil {
				t.Fatalf("unavailable slot changed the UPDATE: %x", got.modifiedPayload)
			}
			if transport.calls != 1 {
				t.Fatalf("closed-slot calls = %d, want 1", transport.calls)
			}
		})
	}
}
