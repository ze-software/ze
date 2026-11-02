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

// textMixTransport serves two filters: "probe:f", which reads
// local-preference and answers sets, and "text:t", a text-era filter that
// declares no reads and answers a text delta. It records what each was sent.
type textMixTransport struct {
	viewReply rpc.FilterUpdateOutput
	textReply rpc.FilterUpdateOutput
	viewSent  []rpc.FilterUpdateInput
	textSent  []rpc.FilterUpdateInput
}

func (e *textMixTransport) FilterInfo(_, _ string) ([]string, bool) { return nil, false }

func (e *textMixTransport) FilterReads(_, filter string) []string {
	if filter == "f" {
		return []string{"local-preference"}
	}
	return nil
}

// FilterReadvertise answers that the filter does not opt into the stale
// readvertise rail.
func (e *textMixTransport) FilterReadvertise(_, _ string) (readvertise, ok bool) { return false, true }

func (e *textMixTransport) FilterDeclaration(plugin, filter string) ([]string, []string, rpc.OnErrorPolicy, bool) {
	switch plugin + ":" + filter {
	case "probe:f":
		return []string{"local-preference"}, []string{"med"}, rpc.OnErrorReject, true
	case "text:t":
		return nil, nil, rpc.OnErrorReject, true
	}
	return nil, nil, rpc.OnErrorReject, false
}

func (e *textMixTransport) FilterOnError(_, _ string) rpc.OnErrorPolicy { return rpc.OnErrorReject }

func (e *textMixTransport) CallFilterUpdate(_ context.Context, plugin string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	input, err := route.Input()
	if err != nil {
		return nil, err
	}
	input.Fields = append(json.RawMessage(nil), input.Fields...)
	if plugin == "text" {
		e.textSent = append(e.textSent, input)
		reply := e.textReply
		return &reply, nil
	}
	e.viewSent = append(e.viewSent, input)
	reply := e.viewReply
	return &reply, nil
}

// TestLibraryChainServesTextMember proves the flip keeps a text-era plugin
// filter working inside the library chain: in [probe:f text:t], the view
// member's med 50 and the text member's local-preference 300 delta both land
// in the one UPDATE the import step answers, the body the text path's two
// set ops build, and the text member is sent the text update. Before, the
// text member failed under its on-error (errExternalTextUpdate) and the route
// was dropped.
func TestLibraryChainServesTextMember(t *testing.T) {
	transport := &textMixTransport{
		viewReply: rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
			{Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage("50")}}},
		textReply: rpc.FilterUpdateOutput{Action: rpc.FilterModify, Update: "local-preference 300"},
	}
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	received := libraryTestBody(100)
	want := textBody(t, r, received, func(acc *filterapi.ModAccumulator) {
		acc.OpCopy(uint8(attribute.AttrLocalPref), filterapi.AttrModSet, []byte{0, 0, 0x01, 0x2c})
		acc.OpCopy(uint8(attribute.AttrMED), filterapi.AttrModSet, []byte{0, 0, 0, 50})
	})

	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: mixedPolicy(t, "", policychain.Member{Name: "f", Plugin: "probe"}, policychain.Member{Name: "t", Plugin: "text"})})
	in := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(received), received)
	if !in.accept || !bytes.Equal(in.modifiedPayload, want) {
		t.Fatalf("import = accept %v body %x, want accept with %x", in.accept, in.modifiedPayload, want)
	}
	if len(transport.viewSent) != 1 || len(transport.textSent) != 1 {
		t.Fatalf("calls: view %d, text %d; want one each", len(transport.viewSent), len(transport.textSent))
	}
	if transport.textSent[0].Update == "" {
		t.Fatal("the text member was sent no text update")
	}
}

// TestLibraryChainTextOnlyMatchesTextChain proves a chain of one text-era
// filter answers on the library chain what the text chain answers: the same
// verdict and the same rewritten body, for a modify, and the same drop for a
// reject.
func TestLibraryChainTextOnlyMatchesTextChain(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply rpc.FilterUpdateOutput
	}{
		{"modify", rpc.FilterUpdateOutput{Action: rpc.FilterModify, Update: "local-preference 300"}},
		{"reject", rpc.FilterUpdateOutput{Action: rpc.FilterReject}},
		{"accept", rpc.FilterUpdateOutput{Action: rpc.FilterAccept}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &textMixTransport{textReply: tc.reply}
			r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport, api: liveFilterServer(t)}
			received := libraryTestBody(100)
			addr := netip.MustParseAddr("10.0.0.4")
			library := mixedPolicy(t, "", policychain.Member{Name: "t", Plugin: "text"})
			text := mustPolicy(t, []routepolicy.Ref{{Name: "text:t"}}, routepolicy.FinalAccept)

			onText := r.runIngressPolicyChain(NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: text}),
				addr, nil, testWireUpdate(received), received)
			onLibrary := r.runIngressPolicyChain(NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: library}),
				addr, nil, testWireUpdate(received), received)
			if onText.accept != onLibrary.accept || !bytes.Equal(onText.modifiedPayload, onLibrary.modifiedPayload) {
				t.Fatalf("library = accept %v body %x, text = accept %v body %x",
					onLibrary.accept, onLibrary.modifiedPayload, onText.accept, onText.modifiedPayload)
			}
			if tc.name == "modify" && onText.modifiedPayload == nil {
				t.Fatal("the text chain applied no modify: the comparison proves nothing")
			}
		})
	}
}

// TestLibraryChainServesUndeclaredTextFilter proves a plugin that declares
// no filter at Stage 1 (bgp-filter-modify today) is served the text update
// inside the library chain, as the text chain called it, rather than failing
// as an undeclared member.
func TestLibraryChainServesUndeclaredTextFilter(t *testing.T) {
	transport := &textMixTransport{textReply: rpc.FilterUpdateOutput{Action: rpc.FilterModify, Update: "local-preference 300"}}
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	received := libraryTestBody(100)
	want := textBody(t, r, received, func(acc *filterapi.ModAccumulator) {
		acc.OpCopy(uint8(attribute.AttrLocalPref), filterapi.AttrModSet, []byte{0, 0, 0x01, 0x2c})
	})
	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: mixedPolicy(t, "", policychain.Member{Name: "undeclared", Plugin: "text"})})
	in := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(received), received)
	if !in.accept || !bytes.Equal(in.modifiedPayload, want) {
		t.Fatalf("import = accept %v body %x, want accept with %x", in.accept, in.modifiedPayload, want)
	}
}
