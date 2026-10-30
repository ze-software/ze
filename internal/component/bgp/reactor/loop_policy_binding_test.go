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
			ref, bound := filterapi.BindInProcess(routepolicy.Ref{Name: "AUTO", Inactive: tc.inactive}, "loop-detection")
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
	ref, bound := filterapi.BindInProcess(routepolicy.Ref{Name: "loop:AUTO-$remote_as"}, "loop-detection")
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
	ref, _ := filterapi.BindInProcess(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
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
	automatic, _ := filterapi.BindInProcess(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
	refs := []routepolicy.Ref{
		automatic,
		{Name: "missing:guard"},
	}
	policy := bindRefs(filterapi.InternalPolicy(refs), automatic)
	addr := netip.MustParseAddr("192.0.2.1")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: policy})
	r := &Reactor{}
	body := policyChainAdvertisingBody()
	result := r.runIngressPolicyChain(peer, addr, testWireUpdate(body), body)
	if result.accept {
		t.Fatal("unknown external guard accepted without a filter server")
	}
	r.api = liveFilterServer(t)
	result = r.runIngressPolicyChain(peer, addr, testWireUpdate(body), body)
	if result.accept {
		t.Fatal("unknown external plugin accepted through the real RPC transport")
	}
}

// TestLoopPolicyManualExportRefusal covers callers that bypass config binding's
// direction validation. Neither a direct chain nor either export origin rail
// may interpret ingress ownership as proof of an executed export check.
func TestLoopPolicyManualExportRefusal(t *testing.T) {
	ref, _ := filterapi.BindInProcess(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
	policy := bindRefs(filterapi.InternalPolicy([]routepolicy.Ref{ref}), ref)
	for _, direction := range []string{directionExport, "invalid-direction"} {
		result := PolicyFilterChain(policy, true, direction, "192.0.2.1", 65001, "origin igp", func(string, string, string, string, uint32, string) PolicyResponse {
			t.Fatal("unsupported internal direction attempted an RPC")
			return PolicyResponse{Action: PolicyReject, Failed: true}
		})
		if result.Action != PolicyReject || !result.Failed {
			t.Fatalf("manual %s policy accepted: %+v", direction, result)
		}
		result, trace := tracePolicyFilterChain(policy, true, direction, "192.0.2.1", 65001, "origin igp", nil)
		if result.Action != PolicyReject || !result.Failed || len(trace) != 1 || trace[0].Action != "not-evaluated" {
			t.Fatalf("manual %s dry-run accepted or claimed execution: result=%+v trace=%+v", direction, result, trace)
		}
	}
	if !hasActiveFilter(policy, directionExport) {
		t.Fatal("native export fast path would bypass an unsupported internal ref")
	}
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	settings.ExportPolicy = policy
	peer := NewPeer(settings)
	r := &Reactor{api: liveFilterServer(t)}
	body := policyChainAdvertisingBody()
	if suppress, _ := r.exportFilterForBody(peer, body); !suppress {
		t.Fatal("pre-snapshot origination bypassed the unsupported internal ref")
	}
	peer.refreshForwardFacts()
	if suppress, _ := r.exportFilterForBody(peer, body); !suppress {
		t.Fatal("snapshot origination bypassed the unsupported internal ref")
	}
	result := r.runEgressPolicyChain(policy, settings.Address.String(), 65001, 65000, false, testWireUpdate(body))
	if result.accept || !result.failed {
		t.Fatalf("forwarded export bypassed the unsupported internal ref: %+v", result)
	}
	ref.Inactive = true
	policy = bindRefs(policy.WithFilters([]routepolicy.Ref{ref}), ref)
	if hasActiveFilter(policy, directionExport) {
		t.Fatal("intentionally inactive internal ref needs export execution")
	}
	if result := PolicyFilterChain(policy, true, directionExport, "192.0.2.1", 65001, "origin igp", nil); result.Action != PolicyReject || result.Failed {
		t.Fatalf("inactive export bypassed absent EBGP final: %+v", result)
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
	ref, _ := filterapi.BindInProcess(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
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

// TestLoopPolicyExportOwnershipPreflight requires whole-policy refusal before
// any external callback, including a terminal raw rewrite before the loop ref.
// First ownership tests the existing boundary; middle and last expose early
// terminal escape and side effects before a later unsupported reference.
func TestLoopPolicyExportOwnershipPreflight(t *testing.T) {
	ref, bound := filterapi.BindInProcess(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
	if !bound {
		t.Fatal("real loop registration did not bind AUTO")
	}
	for _, tc := range []struct {
		name string
		refs []routepolicy.Ref
	}{
		{name: "first", refs: []routepolicy.Ref{ref, {Name: "external:raw"}}},
		{name: "middle", refs: []routepolicy.Ref{{Name: "external:raw"}, ref, {Name: "external:later"}}},
		{name: "last", refs: []routepolicy.Ref{{Name: "external:pass"}, {Name: "external:raw"}, ref}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := bindRefs(mustPolicy(t, tc.refs, routepolicy.FinalAccept), ref)
			for _, direction := range []string{directionExport, "invalid-direction"} {
				t.Run(direction, func(t *testing.T) {
					calls := 0
					call := func(_, name, _, _ string, _ uint32, _ string) PolicyResponse {
						calls++
						if name == "pass" {
							return PolicyResponse{Action: PolicyAccept}
						}
						return PolicyResponse{Action: PolicyModify, Raw: policyChainAdvertisingBody()}
					}
					result := PolicyFilterChain(policy, true, direction, "192.0.2.1", 65001, "origin igp", call)
					if result.Action != PolicyReject || !result.Failed || calls != 0 || len(result.Raw) != 0 {
						t.Errorf("direct unsupported policy: result=%+v calls=%d", result, calls)
					}
					calls = 0
					result, trace := tracePolicyFilterChain(policy, true, direction, "192.0.2.1", 65001, "origin igp", call)
					if result.Action != PolicyReject || !result.Failed || calls != 0 || len(result.Raw) != 0 {
						t.Errorf("traced unsupported policy: result=%+v calls=%d", result, calls)
					}
					if len(trace) != 1 || trace[0].Canonical != ref.Name || trace[0].Action != "not-evaluated" || trace[0].TextAfter != "" {
						t.Errorf("refused policy claimed execution: %+v", trace)
					}
				})
			}
		})
	}
}

// TestLoopPolicyExportOwnershipConsumers drives forwarded and originated export
// gates with a valid raw response preceding a real bound internal reference.
func TestLoopPolicyExportOwnershipConsumers(t *testing.T) {
	ref, bound := filterapi.BindInProcess(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
	if !bound {
		t.Fatal("real loop registration did not bind AUTO")
	}
	policy := bindRefs(mustPolicy(t, []routepolicy.Ref{{Name: "external:raw"}, ref}, routepolicy.FinalAccept), ref)
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	settings.ExportPolicy = policy
	peer := NewPeer(settings)
	body := policyChainAdvertisingBody()
	calls := 0
	r := &Reactor{
		api: liveFilterServer(t),
		policyFilterSeam: func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
			calls++
			return PolicyResponse{Action: PolicyModify, Raw: body}
		},
	}
	if suppress, override := r.exportFilterForBody(peer, body); !suppress || override != nil || calls != 0 {
		t.Errorf("pre-snapshot export: suppress=%v override=%x calls=%d", suppress, override, calls)
	}
	peer.refreshForwardFacts()
	calls = 0
	if suppress, override := r.exportFilterForBody(peer, body); !suppress || override != nil || calls != 0 {
		t.Errorf("snapshot export: suppress=%v override=%x calls=%d", suppress, override, calls)
	}
	calls = 0
	result := r.runEgressPolicyChain(policy, settings.Address.String(), 65001, 65000, false, testWireUpdate(body))
	if result.accept || !result.failed || result.wireOverride != nil || calls != 0 {
		t.Errorf("forwarded export: result=%+v calls=%d", result, calls)
	}
}

// TestLoopPolicyOwnershipReplacement preserves valid raw terminal semantics and
// both explicit finals when WithFilters replaces active ownership with inactive
// or external references, then restores the unsupported ownership.
func TestLoopPolicyOwnershipReplacement(t *testing.T) {
	ref, bound := filterapi.BindInProcess(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
	if !bound {
		t.Fatal("real loop registration did not bind AUTO")
	}
	inactive := ref
	inactive.Inactive = true
	for _, final := range []routepolicy.Final{routepolicy.FinalAccept, routepolicy.FinalReject} {
		original := bindRefs(mustPolicy(t, []routepolicy.Ref{ref}, final), ref)
		for _, refs := range [][]routepolicy.Ref{
			{{Name: "external:raw"}, inactive, {Name: "external:later"}},
			{{Name: "external:raw"}, {Name: "external:later"}},
		} {
			policy := bindRefs(original.WithFilters(refs), ref)
			if policy.Final() != final || !policy.Configured() {
				t.Fatalf("replacement changed final: %+v", policy)
			}
			for _, traced := range []bool{false, true} {
				calls := 0
				call := func(_, name, _, _ string, _ uint32, _ string) PolicyResponse {
					calls++
					if name != "raw" {
						t.Errorf("raw rewrite did not stop traversal: %s", name)
					}
					return PolicyResponse{Action: PolicyModify, Raw: policyChainAdvertisingBody()}
				}
				var result PolicyChainResult
				if traced {
					var trace []plugin.PolicyTraceEntry
					result, trace = tracePolicyFilterChain(policy, true, directionExport, "192.0.2.1", 65001, "origin igp", call)
					if len(trace) != 1 || trace[0].Canonical != "external:raw" || trace[0].Action != "modify" {
						t.Errorf("supported raw trace changed: %+v", trace)
					}
				} else {
					result = PolicyFilterChain(policy, true, directionExport, "192.0.2.1", 65001, "origin igp", call)
				}
				want := PolicyModify
				if final == routepolicy.FinalReject {
					want = PolicyReject
				}
				if result.Action != want || result.Failed || calls != 1 {
					t.Errorf("supported raw policy: result=%+v calls=%d want=%v", result, calls, want)
				}
			}
			restored := bindRefs(policy.WithFilters([]routepolicy.Ref{{Name: "external:raw"}, ref}), ref)
			result := PolicyFilterChain(restored, true, directionExport, "192.0.2.1", 65001, "origin igp", func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				t.Error("restored unsupported ownership executed an external filter")
				return PolicyResponse{Action: PolicyModify, Raw: policyChainAdvertisingBody()}
			})
			if result.Action != PolicyReject || !result.Failed || restored.Final() != final {
				t.Errorf("restored ownership was not refused: %+v policy=%+v", result, restored)
			}
		}
	}
}
