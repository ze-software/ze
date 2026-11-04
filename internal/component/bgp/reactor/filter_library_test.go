// Design: docs/architecture/core-design.md -- the compiled per-peer policy chain.
// Related: docs/functional-tests.md -- allocation-ceiling benchmark contracts.

package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The library-chain test attachments and filter types exercise field sets,
// read-only decisions and the AS-path rewrite intents without a plugin transport.
const (
	libraryTestImport       = "reactor-library-test import"
	libraryTestExport       = "reactor-library-test export"
	libraryTypeLP200        = "reactor-library-test-lp200"
	libraryTypeReject       = "reactor-library-test-reject"
	libraryTypeLP100        = "reactor-library-test-lp100"
	libraryTypeModifyEgress = "reactor-library-test-modify-egress"
)

func init() {
	lp := routepolicy.FieldLocalPreference
	routepolicy.MustRegisterAttachment("bgp", libraryTestImport, []routepolicy.Field{lp}, []routepolicy.Field{lp})
	routepolicy.MustRegisterAttachment("bgp", libraryTestExport, []routepolicy.Field{lp},
		[]routepolicy.Field{lp, routepolicy.FieldASPath})
	routepolicy.MustRegisterFilterType(libraryTypeLP200, nil, []routepolicy.Field{lp})
	routepolicy.MustRegisterFilterType(libraryTypeReject, nil, nil)
	routepolicy.MustRegisterFilterType(libraryTypeLP100, []routepolicy.Field{lp}, nil)
	routepolicy.MustRegisterFilterType(libraryTypeModifyEgress, nil, []routepolicy.Field{lp, routepolicy.FieldASPath})
	policychain.MustRegisterEvaluator(libraryTypeLP200, rpc.OnErrorReject, func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
		return func(_ *policyview.View, sets *policychain.Sets) (policychain.Decision, error) {
			return policychain.DecisionPass, sets.LocalPreference(policychain.NumericActionSet, 200)
		}, nil
	})
	policychain.MustRegisterEvaluator(libraryTypeReject, rpc.OnErrorReject, func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
		return func(*policyview.View, *policychain.Sets) (policychain.Decision, error) {
			return policychain.DecisionReject, nil
		}, nil
	})
	policychain.MustRegisterEvaluator(libraryTypeLP100, rpc.OnErrorReject, func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
		reader, err := policyview.ReaderFor(lp)
		if err != nil {
			return nil, err
		}
		return func(view *policyview.View, _ *policychain.Sets) (policychain.Decision, error) {
			value, err := view.Read(reader)
			if err != nil {
				return policychain.DecisionUnspecified, err
			}
			number, numbered := value.Number()
			if value.Present() && numbered && number == 100 {
				return policychain.DecisionPass, nil
			}
			return policychain.DecisionReject, nil
		}, nil
	})
	policychain.MustRegisterEvaluator(libraryTypeModifyEgress, rpc.OnErrorReject, func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
		return func(_ *policyview.View, sets *policychain.Sets) (policychain.Decision, error) {
			if err := sets.LocalPreference(policychain.NumericActionSet, 200); err != nil {
				return policychain.DecisionUnspecified, err
			}
			if err := sets.RemovePrivateAS(policychain.RemovePrivateStrip); err != nil {
				return policychain.DecisionUnspecified, err
			}
			return policychain.DecisionPass, sets.PrependASPath(2)
		}, nil
	})
}

// libraryPolicy answers a policy over one library member of filterType,
// compiled at attachment, as bgp/config builds it at peer load.
func libraryPolicy(t testing.TB, attachment, filterType string, final routepolicy.Final) filterapi.Policy {
	t.Helper()
	reg, err := routepolicy.NewRegistry(routepolicy.Instance{Name: "member", Type: filterType})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(attachment, reg, []policychain.Member{{Name: "member"}}, final, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return mustPolicy(t, []routepolicy.Ref{{Name: "member"}}, final).WithLibrary(chain)
}

// libraryTestBody is an UPDATE body with ORIGIN, AS_PATH, NEXT_HOP and the
// given LOCAL_PREF, advertising 10.0.0.0/24.
func libraryTestBody(localPref byte) []byte {
	attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), []byte{2, 1, 0, 0, 0xfd, 0xe9})...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 1})...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrLocalPref), []byte{0, 0, 0, localPref})...)
	return buildModTestPayload(attrs, modTestNLRI)
}

// TestLibraryChainSetsLocalPreferenceBytes proves AC-4's in-process half at
// UPDATE bytes, on both policy steps: a library member that sets
// local-preference 200 makes the import step answer, and the export step
// send, the received body with LOCAL_PREF 200 and nothing else changed, the
// body the text path's LOCAL_PREF set op builds through buildModifiedPayload.
// The chain runs with no API server: it is in process, so the transport guard
// does not apply to it.
func TestLibraryChainSetsLocalPreferenceBytes(t *testing.T) {
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	received := libraryTestBody(100)
	want := libraryTestBody(200)

	var textOps filterapi.ModAccumulator
	textOps.OpCopy(uint8(attribute.AttrLocalPref), filterapi.AttrModSet, []byte{0, 0, 0, 200})
	textBody, _, textFail := buildModifiedPayload(received, &textOps, r.attrModHandlers, nil, nil)
	if textFail.failed() || !bytes.Equal(textBody, want) {
		t.Fatalf("text-path body = %x (failed %v), want %x", textBody, textFail.failed(), want)
	}

	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: libraryPolicy(t, libraryTestImport, libraryTypeLP200, routepolicy.FinalAccept)})
	in := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(received), received)
	if !in.accept || !bytes.Equal(in.modifiedPayload, want) {
		t.Fatalf("import = accept %v body %x, want accept with %x", in.accept, in.modifiedPayload, want)
	}

	out := r.runEgressPolicyChainASN4(libraryPolicy(t, libraryTestExport, libraryTypeLP200, routepolicy.FinalAccept),
		"10.0.0.5", 65002, 65000, false, policyview.Session{EBGP: true}, testWireUpdate(received), true)
	if !out.accept || out.wireOverride == nil || !bytes.Equal(out.wireOverride.Payload(), want) {
		t.Fatalf("export = %+v, want accept with %x", out, want)
	}
}

// TestLibraryChainRejects proves a library member's reject is a policy
// outcome on both steps: export suppresses the route for the peer without
// marking the step failed, and import under final reject drops the route.
func TestLibraryChainRejects(t *testing.T) {
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	received := libraryTestBody(100)

	out := r.runEgressPolicyChainASN4(libraryPolicy(t, libraryTestExport, libraryTypeReject, routepolicy.FinalAccept),
		"10.0.0.5", 65002, 65000, false, policyview.Session{EBGP: true}, testWireUpdate(received), true)
	if out.accept || out.failed {
		t.Fatalf("export = %+v, want a policy suppression (accept false, failed false)", out)
	}

	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: libraryPolicy(t, libraryTestImport, libraryTypeReject, routepolicy.FinalReject)})
	if in := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(received), received); in.accept {
		t.Fatalf("import = %+v, want the route dropped", in)
	}
}

// BenchmarkFilterDispatch_ZeroAlloc exercises the compiled export chain's
// read-only dispatch: a member reads LOCAL_PREF and passes the received UPDATE
// unchanged. A negative control proves the member runs, and every measured
// operation must accept the original bytes, so a fast rejection cannot win.
// Registration, compilation and wire construction are outside the timed loop.
func BenchmarkFilterDispatch_ZeroAlloc(b *testing.B) {
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	policy := libraryPolicy(b, libraryTestExport, libraryTypeLP100, routepolicy.FinalAccept)
	received := libraryTestBody(100)
	original := bytes.Clone(received)
	update := testWireUpdate(received)
	session := policyview.Session{EBGP: true}
	rejected := r.runEgressPolicyChainASN4(policy, "10.0.0.5", 65002, 65000, false,
		session, testWireUpdate(libraryTestBody(99)), true)
	if rejected.accept || rejected.failed || rejected.withdraw {
		b.Fatalf("negative control = %+v, want a policy rejection", rejected)
	}

	b.ReportAllocs()
	for b.Loop() {
		out := r.runEgressPolicyChainASN4(policy, "10.0.0.5", 65002, 65000, false, session, update, true)
		if !out.accept || out.failed || out.withdraw || out.wireOverride != nil {
			b.Fatalf("dispatch = %+v, want an accepted unmodified UPDATE", out)
		}
		if !bytes.Equal(update.Payload(), original) {
			b.Fatalf("dispatch changed the received UPDATE: %x, want %x", update.Payload(), original)
		}
	}
}

// BenchmarkFilterModifyEgress runs the historical modify workload's LOCAL_PREF
// set, private-AS strip and two prepends through the compiled export chain and
// wire rebuild, once per destination. Every operation must accept the complete
// expected body and preserve the received bytes. Rejection, skipped intents,
// flattened AS_SET segments and in-place edits therefore fail the benchmark.
func BenchmarkFilterModifyEgress(b *testing.B) {
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	policy := libraryPolicy(b, libraryTestExport, libraryTypeModifyEgress, routepolicy.FinalAccept)
	received := libraryModifyBody(100, []byte{
		byte(attribute.ASSequence), 2, 0, 0, 0xfb, 0xf0, 0, 0, 0xfc, 0,
		byte(attribute.ASSet), 2, 0, 0, 0xfb, 0xf1, 0, 0, 0xff, 0xfe,
	})
	original := bytes.Clone(received)
	want := libraryModifyBody(200, []byte{
		byte(attribute.ASSequence), 3, 0, 0, 0xfd, 0xe8, 0, 0, 0xfd, 0xe8, 0, 0, 0xfb, 0xf0,
		byte(attribute.ASSet), 1, 0, 0, 0xfb, 0xf1,
	})
	update := testWireUpdate(received)
	session := policyview.Session{EBGP: true}

	b.ReportAllocs()
	for b.Loop() {
		out := r.runEgressPolicyChainASN4(policy, "10.0.0.5", 65002, 65000, false, session, update, true)
		if !out.accept || out.failed || out.withdraw || out.wireOverride == nil {
			b.Fatalf("modify = %+v, want an accepted modified UPDATE", out)
		}
		if !bytes.Equal(out.wireOverride.Payload(), want) {
			b.Fatalf("modified UPDATE = %x, want %x", out.wireOverride.Payload(), want)
		}
		if !bytes.Equal(received, original) {
			b.Fatalf("modify changed the received UPDATE: %x, want %x", received, original)
		}
	}
}

// libraryModifyBody builds the modify workload with the given LOCAL_PREF and
// AS_PATH; both communities and the advertised prefix must survive the rewrite.
func libraryModifyBody(localPref byte, path []byte) []byte {
	attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), path)...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 1})...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrLocalPref), []byte{0, 0, 0, localPref})...)
	attrs = append(attrs, makeAttr(0xc0, byte(attribute.AttrCommunity), []byte{0xfd, 0xe9, 0, 100, 0xfd, 0xe9, 0, 200})...)
	return buildModTestPayload(attrs, modTestNLRI)
}
