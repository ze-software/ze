package reactor

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// externalIntentPolicy compiles the one external member probe:f at the
// intent test attachment, which lets a filter set as-path, prefix and family.
func externalIntentPolicy(t *testing.T) filterapi.Policy {
	t.Helper()
	reg, err := routepolicy.NewRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	members := []policychain.Member{{Name: "f", Plugin: "probe"}}
	chain, err := policychain.Compile(libraryIntentImport, reg, members, routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return mustPolicy(t, []routepolicy.Ref{{Name: "probe:f"}}, routepolicy.FinalAccept).WithLibrary(chain)
}

// contextUpdate wraps body with a registered receive context, which the
// prefix field's JSON writer needs to frame the NLRI for the external filter.
func contextUpdate(t *testing.T, body []byte) *wireu.WireUpdate {
	t.Helper()
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(body, id)
}

// intentSet is one reply set, its value given as JSON text.
func intentSet(field string, action rpc.FilterSetAction, value string) rpc.FilterSet {
	return rpc.FilterSet{Field: field, Action: action, Value: json.RawMessage(value)}
}

// TestExternalIntentsBytesEqualInProcess proves AC-7 and AC-26 for an
// external filter at UPDATE bytes: the named intents it answers in sets (an
// AS-path prepend, remove-private-AS, an NLRI subset dropping one IPv4 and
// one IPv6 prefix, a family strip) leave the import step with the body an
// in-process member asking the same intents through the Sets methods
// produces, MP_REACH_NLRI included.
func TestExternalIntentsBytesEqualInProcess(t *testing.T) {
	family.RegisterTestFamilies()
	twoFamilies := "0000 003a 40010100 40020602010000FDE9 40030401010101" +
		" 800E23 0002 01 10 20010db8000000000000000000000001 00 3020010db80001 3020010db80002" +
		" 180a0001 180a0009"
	withUnreach := "0000 0040 40010100 40020602010000FDE9 40030401010101" +
		" 800E1c 0002 01 10 20010db8000000000000000000000001 00 3020010db80001" +
		" 800F0a 0002 01 3020010db80002" +
		" 180a0001"
	privatePath := "0000 0016 40010100 4002080203 0003 fc00 00ae 40030401010101 180a0001"
	cases := []struct {
		name      string
		body      string
		inProcess func(*policyview.View, *policychain.Sets) error
		reply     []rpc.FilterSet
	}{
		{
			name:      "prefix drop one IPv4 and one IPv6",
			body:      twoFamilies,
			inProcess: dropCIDRs(t, "3020010db80001", "180a0009"),
			reply: []rpc.FilterSet{
				intentSet("prefix", rpc.FilterSetActionRemove, `"2001:db8:1::/48"`),
				intentSet("prefix", rpc.FilterSetActionRemove, `"10.0.9.0/24"`),
			},
		},
		{
			name: "family strip ipv6/unicast",
			body: withUnreach,
			inProcess: func(_ *policyview.View, s *policychain.Sets) error {
				return s.StripFamily(family.IPv6Unicast)
			},
			reply: []rpc.FilterSet{intentSet("family", rpc.FilterSetActionRemove, `"ipv6/unicast"`)},
		},
		{
			name: "remove-private and prepend 2",
			body: privatePath,
			inProcess: func(_ *policyview.View, s *policychain.Sets) error {
				if err := s.RemovePrivateAS(policychain.RemovePrivateStrip); err != nil {
					return err
				}
				return s.PrependASPath(2)
			},
			reply: []rpc.FilterSet{
				intentSet("as-path", rpc.FilterSetActionRemove, `"private"`),
				intentSet("as-path", rpc.FilterSetActionPrepend, "2"),
			},
		},
		{
			name: "every intent at once, peer-as mode",
			body: twoFamilies,
			inProcess: func(v *policyview.View, s *policychain.Sets) error {
				if err := s.RemovePrivateAS(policychain.RemovePrivatePeerAS); err != nil {
					return err
				}
				if err := s.PrependASPath(2); err != nil {
					return err
				}
				return dropCIDRs(t, "3020010db80001", "180a0009")(v, s)
			},
			reply: []rpc.FilterSet{
				intentSet("as-path", rpc.FilterSetActionRemove, `"private-peer-as"`),
				intentSet("as-path", rpc.FilterSetActionPrepend, "2"),
				intentSet("prefix", rpc.FilterSetActionRemove, `"2001:db8:1::/48"`),
				intentSet("prefix", rpc.FilterSetActionRemove, `"10.0.9.0/24"`),
			},
		},
	}
	addr := netip.MustParseAddr("10.0.0.4")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			received := nlriHex(t, tc.body)
			libraryIntentEval = tc.inProcess
			inProcess := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
			inPeer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
				ImportPolicy: libraryPolicy(t, libraryIntentImport, libraryTypeIntents, routepolicy.FinalAccept)})
			want := inProcess.runIngressPolicyChain(inPeer, addr, contextUpdate(t, received), received)
			if !want.accept || want.modifiedPayload == nil {
				t.Fatalf("in-process import = %+v, want accept with a body", want)
			}

			transport := &externalTransport{reads: []string{"prefix"}, sets: []string{"as-path", "prefix", "family"},
				reply: rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: tc.reply}}
			r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
			peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
				ImportPolicy: externalIntentPolicy(t)})
			got := r.runIngressPolicyChain(peer, addr, contextUpdate(t, received), received)
			if !got.accept || !bytes.Equal(got.modifiedPayload, want.modifiedPayload) {
				t.Fatalf("external import = accept %v body %x, want %x", got.accept, got.modifiedPayload, want.modifiedPayload)
			}
			if bytes.Equal(got.modifiedPayload, received) {
				t.Fatalf("the body is unchanged: the intents were not applied")
			}
		})
	}
}
