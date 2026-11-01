package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// libraryTestMixed carries and lets a filter set local-preference and med,
// for chains that hold in-process and external members.
const libraryTestMixed = "reactor-library-test mixed import"

func init() {
	lp, med := routepolicy.FieldLocalPreference, routepolicy.FieldMED
	routepolicy.MustRegisterAttachment("bgp", libraryTestMixed, []routepolicy.Field{lp, med}, []routepolicy.Field{lp, med})
}

// externalTransport is a plugin server with one filter, "probe:f": it
// answers the declaration given and the reply given, and records the
// filter-update input it was sent.
type externalTransport struct {
	reads, sets []string
	reply       rpc.FilterUpdateOutput
	sent        []rpc.FilterUpdateInput
}

func (e *externalTransport) FilterInfo(_, _ string) ([]string, bool) { return nil, false }

func (e *externalTransport) FilterReads(_, _ string) []string { return e.reads }

func (e *externalTransport) FilterDeclaration(plugin, filter string) ([]string, []string, rpc.OnErrorPolicy, bool) {
	if plugin != "probe" || filter != "f" {
		return nil, nil, rpc.OnErrorReject, false
	}
	return e.reads, e.sets, rpc.OnErrorReject, true
}

func (e *externalTransport) FilterOnError(_, _ string) rpc.OnErrorPolicy { return rpc.OnErrorReject }

func (e *externalTransport) CallFilterUpdate(_ context.Context, _ string, input *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	sent := *input
	sent.Fields = append(json.RawMessage(nil), input.Fields...)
	e.sent = append(e.sent, sent)
	reply := e.reply
	return &reply, nil
}

// mixedPolicy compiles members at libraryTestMixed, the in-process member
// "member" of filterType included when filterType is set.
func mixedPolicy(t *testing.T, filterType string, members ...policychain.Member) filterapi.Policy {
	t.Helper()
	var instances []routepolicy.Instance
	if filterType != "" {
		instances = append(instances, routepolicy.Instance{Name: "member", Type: filterType})
	}
	reg, err := routepolicy.NewRegistry(instances...)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(libraryTestMixed, reg, members, routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	refs := make([]routepolicy.Ref, len(members))
	for i, m := range members {
		refs[i] = routepolicy.Ref{Name: m.Name}
		if m.Plugin != "" {
			refs[i].Name = m.Plugin + ":" + m.Name
		}
	}
	return mustPolicy(t, refs, routepolicy.FinalAccept).WithLibrary(chain)
}

// textBody answers the body the text path's set ops build from received.
func textBody(t *testing.T, r *Reactor, received []byte, ops func(*filterapi.ModAccumulator)) []byte {
	t.Helper()
	var acc filterapi.ModAccumulator
	ops(&acc)
	body, _, fail := buildModifiedPayload(received, &acc, r.attrModHandlers, nil, nil)
	if fail.failed() {
		t.Fatal("the text-path body could not be built")
	}
	return body
}

// TestMixedChainExternalReadsInProcessSet proves AC-13 through the import
// step: in one chain, an in-process member sets local-preference 200 and an
// external member that reads local-preference is sent 200, not the received
// 100; the med 50 the external member answers lands in the same UPDATE as the
// in-process set, the body the text path's two set ops build.
func TestMixedChainExternalReadsInProcessSet(t *testing.T) {
	transport := &externalTransport{reads: []string{"local-preference"}, sets: []string{"med"},
		reply: rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
			{Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage("50")}}}}
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	received := libraryTestBody(100)
	want := textBody(t, r, received, func(acc *filterapi.ModAccumulator) {
		acc.OpCopy(uint8(attribute.AttrLocalPref), filterapi.AttrModSet, []byte{0, 0, 0, 200})
		acc.OpCopy(uint8(attribute.AttrMED), filterapi.AttrModSet, []byte{0, 0, 0, 50})
	})

	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: mixedPolicy(t, libraryTypeLP200, policychain.Member{Name: "member"}, policychain.Member{Name: "f", Plugin: "probe"})})
	in := r.runIngressPolicyChain(peer, addr, testWireUpdate(received), received)
	if !in.accept || !bytes.Equal(in.modifiedPayload, want) {
		t.Fatalf("import = accept %v body %x, want accept with %x", in.accept, in.modifiedPayload, want)
	}
	if len(transport.sent) != 1 {
		t.Fatalf("the external member was called %d times, want 1", len(transport.sent))
	}
	sent := transport.sent[0]
	if string(sent.Fields) != `{"local-preference":200}` || sent.Direction != "import" || sent.Peer != "10.0.0.4" || sent.PeerAS != 65001 {
		t.Fatalf("filter-update = %+v fields %s, want local-preference 200 for import from 10.0.0.4 AS 65001", sent, sent.Fields)
	}
	if sent.Update != "" {
		t.Fatalf("a mixed chain sent the text update %q", sent.Update)
	}
}

// TestExternalSetsBytesEqualInProcess proves the external halves of AC-4 and
// AC-25 at UPDATE bytes: each change an external filter answers in sets
// leaves the import step with the body the in-process member asking the same
// change produces, which is the body the text path's op builds.
func TestExternalSetsBytesEqualInProcess(t *testing.T) {
	addr := netip.MustParseAddr("10.0.0.4")
	received := libraryTestBody(100)

	inProcess := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	inPeer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: libraryPolicy(t, libraryTestImport, libraryTypeLP200, routepolicy.FinalAccept)})
	want := inProcess.runIngressPolicyChain(inPeer, addr, testWireUpdate(received), received)
	if !want.accept || want.modifiedPayload == nil {
		t.Fatalf("in-process import = %+v, want accept with a body", want)
	}

	cases := []struct {
		name string
		sets []rpc.FilterSet
		want []byte
	}{
		{"local-preference set 200", []rpc.FilterSet{{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("200")}},
			want.modifiedPayload},
		{"local-preference increment 100", []rpc.FilterSet{{Field: "local-preference", Action: rpc.FilterSetActionIncrement, Value: json.RawMessage("100")}},
			want.modifiedPayload},
		{"local-preference decrement 50", []rpc.FilterSet{{Field: "local-preference", Action: rpc.FilterSetActionDecrement, Value: json.RawMessage("50")}},
			libraryTestBody(50)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &externalTransport{reads: []string{"local-preference"}, sets: []string{"local-preference"},
				reply: rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: tc.sets}}
			r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
			peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
				ImportPolicy: mixedPolicy(t, "", policychain.Member{Name: "f", Plugin: "probe"})})
			got := r.runIngressPolicyChain(peer, addr, testWireUpdate(received), received)
			if !got.accept || !bytes.Equal(got.modifiedPayload, tc.want) {
				t.Fatalf("external import = accept %v body %x, want %x", got.accept, got.modifiedPayload, tc.want)
			}
		})
	}
}

// TestMixedChainNoServerFailsClosed proves the external member of a mixed
// chain fails under its on-error when no plugin server reaches it, rather
// than passing the route unfiltered.
func TestMixedChainNoServerFailsClosed(t *testing.T) {
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	received := libraryTestBody(100)
	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: mixedPolicy(t, libraryTypeLP200, policychain.Member{Name: "member"}, policychain.Member{Name: "f", Plugin: "probe"})})
	if in := r.runIngressPolicyChain(peer, addr, testWireUpdate(received), received); in.accept {
		t.Fatalf("import = %+v, want the route dropped", in)
	}
}
