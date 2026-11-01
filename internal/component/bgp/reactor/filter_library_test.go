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

// The library-chain test attachment and filter types: a member that sets
// local-preference 200 and a member that rejects every route.
const (
	libraryTestImport = "reactor-library-test import"
	libraryTestExport = "reactor-library-test export"
	libraryTypeLP200  = "reactor-library-test-lp200"
	libraryTypeReject = "reactor-library-test-reject"
)

func init() {
	lp := routepolicy.FieldLocalPreference
	routepolicy.MustRegisterAttachment("bgp", libraryTestImport, []routepolicy.Field{lp}, []routepolicy.Field{lp})
	routepolicy.MustRegisterAttachment("bgp", libraryTestExport, []routepolicy.Field{lp}, []routepolicy.Field{lp})
	routepolicy.MustRegisterFilterType(libraryTypeLP200, nil, []routepolicy.Field{lp})
	routepolicy.MustRegisterFilterType(libraryTypeReject, nil, nil)
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
}

// libraryPolicy answers a policy over one library member of filterType,
// compiled at attachment, as bgp/config builds it at peer load.
func libraryPolicy(t *testing.T, attachment, filterType string, final routepolicy.Final) filterapi.Policy {
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
	in := r.runIngressPolicyChain(peer, addr, testWireUpdate(received), received)
	if !in.accept || !bytes.Equal(in.modifiedPayload, want) {
		t.Fatalf("import = accept %v body %x, want accept with %x", in.accept, in.modifiedPayload, want)
	}

	out := r.runEgressPolicyChainASN4(libraryPolicy(t, libraryTestExport, libraryTypeLP200, routepolicy.FinalAccept),
		"10.0.0.5", 65002, 65000, false, testWireUpdate(received), true)
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
		"10.0.0.5", 65002, 65000, false, testWireUpdate(received), true)
	if out.accept || out.failed {
		t.Fatalf("export = %+v, want a policy suppression (accept false, failed false)", out)
	}

	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: libraryPolicy(t, libraryTestImport, libraryTypeReject, routepolicy.FinalReject)})
	if in := r.runIngressPolicyChain(peer, addr, testWireUpdate(received), received); in.accept {
		t.Fatalf("import = %+v, want the route dropped", in)
	}
}
