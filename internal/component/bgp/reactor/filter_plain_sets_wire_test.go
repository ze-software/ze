// Design: docs/plugin-development/protocol.md -- The Sets of a filter-update answer
package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const plainSetsImport = "reactor-plain-sets-proof import"

func init() {
	fields := policychain.SettableFields()
	policy.MustRegisterAttachment("bgp", plainSetsImport, fields, fields)
}

// plainSetsPolicy compiles the declared external member at an attachment with
// the production settable vocabulary, so the proof reaches the normal chain.
func plainSetsPolicy(t *testing.T) filterapi.Policy {
	t.Helper()
	registry, err := policy.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	chain, err := policychain.Compile(plainSetsImport, registry,
		[]policychain.Member{{Plugin: "probe", Name: "f"}}, policy.FinalAccept,
		policychain.Bases{LocalPreference: 100, MED: 20})
	if err != nil {
		t.Fatal(err)
	}
	return mustPolicy(t, []policy.Ref{{Name: "probe:f"}}, policy.FinalAccept).WithLibrary(chain)
}

// plainSetsBody frames literal attribute bytes with both withdrawn and announced
// routes. Expected values never call the set converter or attribute handlers.
func plainSetsBody(t *testing.T, attrs []string) []byte {
	t.Helper()
	var attributes []byte
	for _, attr := range attrs {
		attributes = append(attributes, nlriHex(t, attr)...)
	}
	body := []byte{0, 4, 24, 203, 0, 113}
	body = binary.BigEndian.AppendUint16(body, uint16(len(attributes)))
	body = append(body, attributes...)
	return append(body, 24, 198, 51, 100)
}

// TestPlainSetActionsProduceExactUpdate proves A-2, AC-4 and AC-25 at the
// consumer boundary: every plain settable field and supported action travels
// from a declared filter reply through Chain.Apply and the registered handlers
// to a whole UPDATE body with independently specified flags, lengths and values.
// Existing unrelated attributes and both route sections must remain unchanged.
func TestPlainSetActionsProduceExactUpdate(t *testing.T) {
	attrs := []string{
		"40010100",                     // ORIGIN IGP.
		"40020602010000fde9",           // AS_SEQUENCE 65001, four octets.
		"400304c0000201",               // NEXT_HOP 192.0.2.1.
		"40050400000064",               // LOCAL_PREF 100.
		"80040400000014",               // MED 20.
		"801a0b01000b0000000000000064", // AIGP metric 100.
		"c00808fde80001fde80002",       // COMMUNITY 65000:1, 65000:2.
		"c020180000fde800000001000000020000fde80000000300000004", // Large 65000:1:2, 65000:3:4.
		"c010100002fde8000000010002fde800000002",                 // RT 65000:1, 65000:2.
	}
	cases := []struct {
		field  string
		action rpc.FilterSetAction
		value  string
		index  int
		want   string
	}{
		{"local-preference", rpc.FilterSetActionSet, "200", 3, "400504000000c8"},
		{"local-preference", rpc.FilterSetActionIncrement, "7", 3, "4005040000006b"},
		{"local-preference", rpc.FilterSetActionDecrement, "7", 3, "4005040000005d"},
		{"med", rpc.FilterSetActionSet, "50", 4, "80040400000032"},
		{"med", rpc.FilterSetActionIncrement, "7", 4, "8004040000001b"},
		{"med", rpc.FilterSetActionDecrement, "7", 4, "8004040000000d"},
		{"med", rpc.FilterSetActionRemove, "", 4, ""},
		{"origin", rpc.FilterSetActionSet, `"incomplete"`, 0, "40010102"},
		{"next-hop", rpc.FilterSetActionSet, `"192.0.2.9"`, 2, "400304c0000209"},
		{"aigp", rpc.FilterSetActionSet, "200", 5, "801a0b01000b00000000000000c8"},
		{"aigp", rpc.FilterSetActionIncrement, "7", 5, "801a0b01000b000000000000006b"},
		{"aigp", rpc.FilterSetActionDecrement, "7", 5, "801a0b01000b000000000000005d"},
		// The community handlers preserve their established extended-length
		// output convention even for a short input (handler.go emitCommunity).
		{"community", rpc.FilterSetActionAdd, `"65000:3"`, 6, "d008000cfde80001fde80002fde80003"},
		{"community", rpc.FilterSetActionRemove, `"65000:1"`, 6, "d0080004fde80002"},
		{"large-community", rpc.FilterSetActionAdd, `"65000:5:6"`, 7,
			"d02000240000fde800000001000000020000fde800000003000000040000fde80000000500000006"},
		{"large-community", rpc.FilterSetActionRemove, `"65000:1:2"`, 7, "d020000c0000fde80000000300000004"},
		{"extended-community", rpc.FilterSetActionAdd, `"target:65000:3"`, 8,
			"d01000180002fde8000000010002fde8000000020002fde800000003"},
		{"extended-community", rpc.FilterSetActionRemove, `"target:65000:1"`, 8, "d01000080002fde800000002"},
	}
	covered := make(map[string]bool)
	for _, tc := range cases {
		covered[tc.field] = true
		t.Run(tc.field+"/"+string(tc.action), func(t *testing.T) {
			transport := &externalTransport{reads: []string{rpc.FilterFieldRaw}, sets: []string{tc.field},
				reply: rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{{
					Field: tc.field, Action: tc.action, Value: json.RawMessage(tc.value),
				}}}}
			reactor := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
			addr := netip.MustParseAddr("192.0.2.4")
			peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
				ImportPolicy: plainSetsPolicy(t)})
			received := plainSetsBody(t, attrs)
			expected := append([]string(nil), attrs...)
			expected[tc.index] = tc.want
			want := plainSetsBody(t, expected)
			got := reactor.runIngressPolicyChain(peer, addr, nil, contextUpdate(t, received), received)
			if !got.accept {
				t.Fatal("the declared set was rejected")
			}
			if !bytes.Equal(got.modifiedPayload, want) {
				t.Fatalf("UPDATE = %x, want %x", got.modifiedPayload, want)
			}
			if len(transport.sent) != 1 {
				t.Fatalf("filter calls = %d, want 1", len(transport.sent))
			}
		})
	}
	for _, field := range policychain.SettableFields() {
		if field == policy.FieldASPath {
			// AS-path has named intents, not a plain replacement. The captured
			// UPDATE goldens are TestLibraryASPathIntentsMatchText.
			continue
		}
		if !covered[field.String()] {
			t.Errorf("settable field %s has no exact UPDATE proof", field)
		}
	}
}
