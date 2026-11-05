// Design: docs/architecture/core-design.md -- Policy withdrawal continuations.
package reactor

import (
	"encoding/json"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// followWithdrawalRoutes queries the running Adj-RIB-In through its actual
// command callback, not a model of its route-removal behavior.
func followWithdrawalRoutes(t *testing.T, conn *plugipc.PluginConn) []rpc.StoredRoute {
	t.Helper()
	out, err := conn.SendExecuteCommand(t.Context(), &rpc.ExecuteCommandInput{
		Command: ribevents.ReplayRoutesCommand, Args: []string{"192.0.2.99", "0"}, Peer: "*",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != rpc.StatusDone {
		t.Fatalf("Adj-RIB-In route query status=%q data=%s", out.Status, out.Data)
	}
	var page ribevents.ReplayPage
	if err := json.Unmarshal(out.Data, &page); err != nil {
		t.Fatal(err)
	}
	if page.More {
		t.Fatal("three-route fixture unexpectedly needs more than one replay page")
	}
	return page.Routes
}

// TestPolicyWithdrawalFollowUsesOriginalDecision seeds three routes in the
// actual running Adj-RIB-In. Once enabled, the filter withdraws an UPDATE with
// local-preference and rejects one without it. A two-family withdraw verdict
// must remove both named routes without judging its attribute-free follow-up
// again, while an unrelated route survives. The dry run must predict the same
// two withdrawals without changing stored state. Both real transports run.
func TestPolicyWithdrawalFollowUsesOriginalDecision(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "forked"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			var enabled atomic.Bool
			var calls atomic.Int32
			judge := func(input rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
				calls.Add(1)
				if !enabled.Load() {
					return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
				}
				var fields struct {
					LocalPreference *uint32 `json:"local-preference"`
				}
				if err := json.Unmarshal(input.Fields, &fields); err != nil {
					return nil, err
				}
				if fields.LocalPreference != nil {
					return &rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}, nil
				}
				return &rpc.FilterUpdateOutput{Action: rpc.FilterReject}, nil
			}
			observer := &answerTransport{reads: []string{"local-preference"}}
			transport := &errorVerdictTransport{rawBoundaryTransport: &rawBoundaryTransport{answerTransport: observer, onError: rpc.OnErrorReject},
				conn: errorVerdictConnection(t, typed, judge)}
			r := New(&Config{LocalAS: 65000, RecentUpdateMax: 32})
			r.filterTransportSeam = transport
			r.orderedIngressSteps = []orderedIngressStep{{name: policyChainStepName, policyChain: true}}
			address := netip.MustParseAddr("192.0.2.4")
			settings := NewPeerSettings(address, 65000, 65001, 0x01010101)
			settings.ImportPolicy = answerPolicy(t, routepolicy.FinalAccept, "conditional-withdraw")
			peer := NewPeer(settings)
			peer.state.Store(int32(PeerStateEstablished))
			r.peers[settings.PeerKey()] = peer
			server := policyFinalPluginServer(t, r, "bgp-rib", "bgp-adj-rib-in")
			store := server.ProcessManager().GetProcess("bgp-adj-rib-in")
			if store == nil || store.Bridge() == nil || store.Conn() == nil {
				t.Fatal("real Adj-RIB-In process is not connected")
			}
			lowEventually(t, store.Bridge().HasExecuteCommand, "Adj-RIB-In command callback installed")
			r.messageReceiver = &testDeliveryReceiver{consumerCount: 1, onReceived: func(info plugin.PeerInfo, msg bgptypes.RawMessage) {
				if err := store.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
					PeerAddress: info.Address.String(), PeerAS: info.PeerAS, EventType: rpc.EventKindUpdate,
					Direction: rpc.DirectionReceived, MessageID: msg.MessageID, RawMessage: &msg,
					SourceID: msg.SourceID, Meta: msg.Meta,
				}}); err != nil {
					t.Error(err)
				}
			}}
			ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			if err != nil {
				t.Fatal(err)
			}
			receive := func(body []byte) {
				update := wireu.NewWireUpdate(body, ctxID)
				update.SetSourceID(peer.SourceID())
				r.notifyMessageReceiver(address, msgtype.TypeUPDATE, body, update, ctxID, rpc.DirectionReceived,
					BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
			}
			for _, seed := range []struct {
				family family.Family
				nlri   []byte
			}{
				{family.IPv6Unicast, []byte{32, 0x20, 1, 0x0d, 0xb8}},
				{family.IPv4Multicast, []byte{24, 198, 51, 100}},
				{family.IPv4Unicast, []byte{24, 203, 0, 113}},
			} {
				receive(ownershipRailBody(seed.family, false, seed.nlri, 0))
			}
			if routes := followWithdrawalRoutes(t, store.Conn()); len(routes) != 3 {
				t.Fatalf("precondition: Adj-RIB-In holds %+v, want three seeded routes", routes)
			}
			enabled.Store(true)
			body, _ := twoFamilyBody()
			dry, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(address.String(), directionImport, "", body, true)
			if err != nil {
				t.Fatal(err)
			}
			if dry.Action != dryRunActionWithdraw || !strings.Contains(dry.TextAfter, "2001:db8::/32") || !strings.Contains(dry.TextAfter, "198.51.100.0/24") {
				t.Fatalf("dry run=%+v, want both family withdrawals", dry)
			}
			if routes := followWithdrawalRoutes(t, store.Conn()); len(routes) != 3 {
				t.Fatalf("dry run changed Adj-RIB-In: %+v", routes)
			}
			calls.Store(0)
			receive(body)
			routes := followWithdrawalRoutes(t, store.Conn())
			if len(routes) != 1 || routes[0].Family != family.IPv4Unicast.String() || routes[0].NLRIHex != "18cb0071" {
				t.Errorf("Adj-RIB-In after withdraw=%+v, want only unrelated 203.0.113.0/24", routes)
			}
			if got := calls.Load(); got != 1 {
				t.Errorf("policy called %d times, want one decision shared by both family withdrawals", got)
			}
			chain, ok := settings.ImportPolicy.Library().(*policychain.Chain)
			if !ok || chain.Refused() != 0 {
				t.Error("a valid withdraw verdict was counted as a member failure")
			}
		})
	}
}
