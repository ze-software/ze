// Design: docs/architecture/core-design.md -- named policy references retain stage ownership.

package reactor

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/reactor/loopdetect"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLoopPolicyRegisteredStage drives the receive consumer with the real
// registered LoopIngress hook, counting its invocations without replacing its
// decision. Own-AS rejection, allowance and inactive controls must reach the
// receiver correctly without an API server or a second loop invocation.
func TestLoopPolicyRegisteredStage(t *testing.T) {
	for _, tc := range []struct {
		name       string
		own        bool
		allow      uint8
		inactive   bool
		cluster    uint32
		originator uint32
		final      routepolicy.Final
		want       int
	}{
		{name: "clean-automatic"},
		{name: "loop-rejected", own: true},
		{name: "loop-allowed", own: true, allow: 1},
		{name: "loop-inactive", own: true, inactive: true},
		{name: "loop-rejected-explicit-accept", own: true, final: routepolicy.FinalAccept},
		{name: "loop-allowed-explicit-accept", own: true, allow: 1, final: routepolicy.FinalAccept, want: 1},
		{name: "loop-inactive-explicit-accept", own: true, inactive: true, final: routepolicy.FinalAccept, want: 1},
		{name: "cluster-loop-rejected", cluster: 0x09090909},
		{name: "cluster-other-accepted", cluster: 0x08080808, want: 1},
		{name: "originator-loop-rejected", originator: 0xcb007109},
		{name: "originator-other-accepted", originator: 0xcb007108, want: 1},
		{name: "explicit-accept", final: routepolicy.FinalAccept, want: 1},
		{name: "explicit-reject", final: routepolicy.FinalReject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref, bound := loopdetect.BindLoopDetection(routepolicy.Ref{Name: "AUTO", Inactive: tc.inactive}, "loop-detection")
			if !bound {
				t.Fatal("real loop registration did not bind AUTO")
			}
			policy := bindRefs(filterapi.InternalPolicy([]routepolicy.Ref{ref}), ref)
			if tc.final != 0 {
				policy = bindRefs(mustPolicy(t, policy.Filters(), tc.final), ref)
			}
			addr := netip.MustParseAddr("203.0.113.9")
			peerAS := uint32(65001)
			if tc.cluster != 0 {
				peerAS = 65000
			}
			if tc.originator != 0 {
				peerAS = 65000
			}
			source := NewPeer(&PeerSettings{
				Connection: ConnectionBoth, Address: addr,
				LocalAS: 65000, GlobalLocalAS: 65000, PeerAS: peerAS,
				RouterID: 0xcb007109, ImportPolicy: policy,
				LoopDisabled: tc.inactive, LoopAllowOwnAS: tc.allow,
				LoopClusterID: 0x09090909,
			})
			source.state.Store(int32(PeerStateEstablished))
			dispatched, calls, registrations := 0, 0, 0
			steps := buildOrderedIngressSteps()
			for i := range steps {
				if steps[i].name != "loop" {
					continue
				}
				registrations++
				if steps[i].stage != filterapi.FilterStageProtocol {
					t.Fatalf("loop stage = %d", steps[i].stage)
				}
				original := steps[i].inproc
				steps[i].inproc = func(src filterapi.PeerFilterInfo, body []byte, meta map[string]any) (bool, []byte) {
					calls++
					return original(src, body, meta)
				}
			}
			if registrations != 1 {
				t.Fatalf("loop registrations = %d, want one", registrations)
			}
			r := &Reactor{
				clock: source.clock, config: &Config{LocalAS: 65000},
				peers:           map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
				recentUpdates:   newRecentUpdateCache(100),
				attrModHandlers: attrModHandlersWithDefaults(),
				messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, _ bgptypes.RawMessage) {
					dispatched++
				}},
				orderedIngressSteps: steps,
			}
			path := byte(0xe9) // 65001, a clean path.
			if tc.own {
				path = 0xe8 // 65000, the local AS.
			}
			attrs := []byte{
				0x40, 1, 1, 0,
				0x40, 2, 6, 2, 1, 0, 0, 0xfd, path,
				0x40, 3, 4, 203, 0, 113, 9,
			}
			if tc.cluster != 0 {
				attrs = append(attrs, 0x80, 10, 4)
				attrs = binary.BigEndian.AppendUint32(attrs, tc.cluster)
			}
			if tc.originator != 0 {
				attrs = append(attrs, 0x80, 9, 4)
				attrs = binary.BigEndian.AppendUint32(attrs, tc.originator)
			}
			body := buildUpdatePayload(attrs, []byte{24, 192, 0, 2})
			update := wireu.NewWireUpdate(body, bgpctx.APIContextID)
			update.SetSourceID(source.SourceID())
			r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, update, 0, rpc.DirectionReceived,
				BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
			if calls != 1 || dispatched != tc.want {
				t.Fatalf("loop calls=%d delivered=%d, want one call and %d deliveries", calls, dispatched, tc.want)
			}
		})
	}
}

// TestLoopPolicyDynamicOwnership proves substitution and snapshot publication
// retain execution ownership, while reload equality distinguishes an unbound ref.
func TestLoopPolicyDynamicOwnership(t *testing.T) {
	ref, bound := loopdetect.BindLoopDetection(routepolicy.Ref{Name: "loop:AUTO-$remote_as"}, "loop-detection")
	if !bound {
		t.Fatalf("templated loop ref %q did not bind to its registered stage", ref.Name)
	}
	template := bindRefs(filterapi.InternalPolicy([]routepolicy.Ref{ref}), ref)
	resolved := resolveFilterVars(template.Filters(), 65000, 65001, "192.0.2.1")
	policy := template.WithBoundFilters(resolved, template.Bindings())
	if policy.Configured() || policy.Final() != 0 || len(policy.Filters()) != 1 {
		t.Fatalf("automatic policy changed: %+v", policy)
	}
	if policy.Filters()[0].Name != "loop:AUTO-65001" || !policy.InProcess(0) || hasActiveFilter(policy, directionImport) {
		t.Fatalf("dynamic dispatch ownership lost: %+v", policy.Filters())
	}
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	settings.ImportPolicy = policy
	settings.ExportPolicy = policy
	peer := NewPeer(settings)
	peer.refreshForwardFacts()
	if !slices.Equal(peer.ImportPolicy().Filters(), policy.Filters()) || !slices.Equal(peer.forwardFacts().exportPolicy.Filters(), policy.Filters()) {
		t.Fatal("peer or forwarding snapshot lost the bound reference")
	}
	if !slices.Equal(peer.ImportPolicy().Bindings(), policy.Bindings()) || !peer.ImportPolicy().InProcess(0) {
		t.Fatalf("peer import snapshot lost the stage binding: %v", peer.ImportPolicy().Bindings())
	}
	if !slices.Equal(peer.forwardFacts().exportPolicy.Bindings(), policy.Bindings()) || !peer.forwardFacts().exportPolicy.InProcess(0) {
		t.Fatalf("forwarding export snapshot lost the stage binding: %v", peer.forwardFacts().exportPolicy.Bindings())
	}
	unbound := *settings
	unbound.ImportPolicy = policy.WithFilters([]routepolicy.Ref{{Name: policy.Filters()[0].Name}})
	if peerSettingsEqual(settings, &unbound) {
		t.Fatal("reload equality ignored changed execution ownership")
	}
}

// TestLoopPolicyDryRunScope drives the public adapter and its JSON output. An
// unevaluated protocol guard must never be reported as an accepting RPC call.
func TestLoopPolicyDryRunScope(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	ref, _ := loopdetect.BindLoopDetection(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
	settings.ImportPolicy = bindRefs(filterapi.InternalPolicy([]routepolicy.Ref{ref}), ref)
	peer := NewPeer(settings)
	r.peers[settings.PeerKey()] = peer
	adapter := &reactorAPIAdapter{r: r}
	body := policyChainAdvertisingBody()
	result, err := adapter.PolicyDryRun(settings.Address.String(), directionImport, "", body, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Trace) != 1 || result.Trace[0].Canonical != "loop:AUTO" || result.Trace[0].Action != "not-evaluated" || result.Trace[0].TextAfter != "" {
		t.Fatalf("protocol guard presented as executed: %+v", result.Trace)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"action":"not-evaluated"`) {
		t.Fatalf("JSON lost the evaluation boundary: %s", encoded)
	}
	for _, name := range []string{"AUTO", "loop:AUTO"} {
		if _, err := adapter.PolicyDryRun(settings.Address.String(), directionImport, name, body, true); !errors.Is(err, errInProcessDryRun) {
			t.Fatalf("internal override %q returned %v", name, err)
		}
	}
	ref.Inactive = true
	settings.ImportPolicy = bindRefs(settings.ImportPolicy.WithFilters([]routepolicy.Ref{ref}), ref)
	result, err = adapter.PolicyDryRun(settings.Address.String(), directionImport, "", body, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Trace) != 0 {
		t.Fatalf("inactive internal check was evaluated: %+v", result.Trace)
	}
}

// TestLoopPolicyUnknownGuardFailsClosed proves a configured internal ref does
// not make the remaining unresolved external guard optional at the live entry.
func TestLoopPolicyUnknownGuardFailsClosed(t *testing.T) {
	automatic, _ := loopdetect.BindLoopDetection(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
	refs := []routepolicy.Ref{
		automatic,
		{Name: "missing:guard"},
	}
	policy := bindRefs(filterapi.InternalPolicy(refs), automatic)
	addr := netip.MustParseAddr("192.0.2.1")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: policy})
	r := &Reactor{}
	body := policyChainAdvertisingBody()
	result := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(body), body)
	if result.accept {
		t.Fatal("unknown external guard accepted without a filter server")
	}
	r.api = liveFilterServer(t)
	result = r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(body), body)
	if result.accept {
		t.Fatal("unknown external plugin accepted through the real RPC transport")
	}
}

// TestLoopPolicyManualExportLeavesNativeRS drives the actual fast-path consumer,
// requiring invalid manually constructed ownership to reach the rejecting rail.
func TestLoopPolicyManualExportLeavesNativeRS(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	source := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	destination := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	ref, _ := loopdetect.BindLoopDetection(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
	destination.settings.ExportPolicy = bindRefs(filterapi.InternalPolicy([]routepolicy.Ref{ref}), ref)
	destination.refreshForwardFacts()
	update := &ReceivedUpdate{
		WireUpdate:   wireu.NewWireUpdate(policyChainAdvertisingBody(), ctxID),
		SourcePeerIP: source.Settings().Address,
	}
	r := &Reactor{peers: map[netip.AddrPort]*Peer{
		source.Settings().PeerKey():      source,
		destination.Settings().PeerKey(): destination,
	}}
	skipped, dispatched := reactorForwardRS(r, update, 51, source.Settings().Address, source)
	if dispatched != 0 || len(skipped) != 1 || skipped[0] != destination.Settings().PeerKey() {
		t.Fatalf("unsupported export bypassed policy rail: skipped=%v dispatched=%d", skipped, dispatched)
	}
}
