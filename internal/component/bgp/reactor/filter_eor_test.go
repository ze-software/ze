package reactor

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// unavailableError is what the plugin server answers for a filter whose
// plugin's slot answers rpc.ErrBridgeClosed (server.CallFilterUpdate, proven
// by TestCallFilterUpdateUnavailable in plugin/server).
func unavailableError(plugin, filter string) error {
	return fmt.Errorf("filter %s: plugin %q is unavailable (stopped, crashed or respawning): %w",
		filter, plugin, rpc.ErrBridgeClosed)
}

// unavailableTransport is a plugin server whose one filter, "probe:f", has
// a stopped plugin: every call answers the server's "unavailable" error, and
// the member's on-error is onError.
type unavailableTransport struct {
	externalTransport
	onError rpc.OnErrorPolicy
	calls   int
}

func (u *unavailableTransport) FilterDeclaration(plugin, filter string) ([]string, []string, rpc.OnErrorPolicy, bool) {
	reads, sets, _, ok := u.externalTransport.FilterDeclaration(plugin, filter)
	return reads, sets, u.onError, ok
}

func (u *unavailableTransport) FilterOnError(_, _ string) rpc.OnErrorPolicy { return u.onError }

func (u *unavailableTransport) CallFilterUpdate(_ context.Context, plugin string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	u.calls++
	return nil, unavailableError(plugin, route.Filter())
}

// TestIngressPassDeliversEndOfRIBUnjudged proves that the ingress filter pass
// never judges an End-of-RIB marker: the marker carries no route, so no
// import member is called, and a member that would fail closed cannot keep
// the marker from the peer's attached plugins. Method: a received marker,
// IPv4 unicast and multiprotocol, goes through notifyMessageReceiver with an
// import chain whose plugin member is unavailable under on-error reject,
// which drops any route it judges (unavailableTransport, below); the
// consumer receives the marker unchanged and the plugin is never called.
func TestIngressPassDeliversEndOfRIBUnjudged(t *testing.T) {
	markers := []struct {
		name string
		body []byte
	}{
		// RFC 4724 Section 2: the IPv4 unicast marker is the minimum-length
		// UPDATE: withdrawn length 0, path attribute length 0.
		{"ipv4 unicast", []byte{0x00, 0x00, 0x00, 0x00}},
		// RFC 4724 Section 2: any other family's marker carries only
		// MP_UNREACH_NLRI (flags 0x80, type 15, length 3) naming AFI 2,
		// SAFI 1, with no withdrawn routes.
		{"ipv6 unicast", []byte{0x00, 0x00, 0x00, 0x06, 0x80, 0x0f, 0x03, 0x00, 0x02, 0x01}},
	}
	for _, marker := range markers {
		t.Run(marker.name, func(t *testing.T) {
			transport := &unavailableTransport{onError: rpc.OnErrorReject,
				externalTransport: externalTransport{reads: []string{"local-preference"}}}
			addr := netip.MustParseAddr("203.0.113.9")
			source := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
				ImportPolicy: mixedPolicy(t, libraryTypeLP200,
					policychain.Member{Name: "member"}, policychain.Member{Name: "f", Plugin: "probe"})})
			source.state.Store(int32(PeerStateEstablished))
			var delivered [][]byte
			r := &Reactor{clock: source.clock, config: &Config{LocalAS: 65000},
				peers:               map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
				recentUpdates:       newRecentUpdateCache(100),
				attrModHandlers:     attrModHandlersWithDefaults(),
				filterTransportSeam: transport,
				orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
				messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
					delivered = append(delivered, bytes.Clone(msg.RawBytes))
				}}}
			wu := wireu.NewWireUpdate(marker.body, 0)
			wu.SetSourceID(source.SourceID())

			r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, marker.body, wu, 0, rpc.DirectionReceived,
				BufHandle{ID: noPoolBufID, Buf: marker.body}, nil, "", 0)

			if len(delivered) != 1 || !bytes.Equal(delivered[0], marker.body) {
				t.Fatalf("consumer received %x, want the End-of-RIB %x once", delivered, marker.body)
			}
			if transport.calls != 0 {
				t.Fatalf("the plugin member was called %d times for an End-of-RIB, want 0", transport.calls)
			}
		})
	}
}
