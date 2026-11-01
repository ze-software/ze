// Design: docs/plugin-development/protocol.md -- the Sets a filter answers
// Related: external_attributes.go -- applyNumericSet, which reads an answered numeric set

package policychain

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestExternalNumericSetsResolveAgainstTheRoute proves the arithmetic a
// filter's increment or decrement answer gets from the engine, on the wire
// bytes Apply writes: the route's own LOCAL_PREF or MULTI_EXIT_DISC is the
// operand when the route carries one, even where the chain's base differs;
// an absent attribute starts from the chain's base (bgp defaults attribute);
// and the result clamps at 0 and at 4294967295 rather than wrapping. These
// are the numbers bgp-filter-modify computed itself before it answered sets.
func TestExternalNumericSetsResolveAgainstTheRoute(t *testing.T) {
	for _, tt := range []struct {
		name   string
		bases  Bases
		answer rpc.FilterSet
		attrs  [][]byte
		code   attribute.AttributeCode
		want   uint32
	}{
		{"med from the route, plus", Bases{MED: 50}, set("med", rpc.FilterSetActionIncrement, "50"),
			[][]byte{med(100)}, attribute.AttrMED, 150},
		{"med from the route, less", Bases{MED: 50}, set("med", rpc.FilterSetActionDecrement, "30"),
			[][]byte{med(100)}, attribute.AttrMED, 70},
		{"local-preference from the route, not the base", Bases{LocalPreference: 80},
			set("local-preference", rpc.FilterSetActionIncrement, "30"),
			[][]byte{localPref(200)}, attribute.AttrLocalPref, 230},
		{"absent local-preference from the default base", Bases{LocalPreference: 100},
			set("local-preference", rpc.FilterSetActionIncrement, "50"), nil, attribute.AttrLocalPref, 150},
		{"absent local-preference from a configured base", Bases{LocalPreference: 80},
			set("local-preference", rpc.FilterSetActionIncrement, "50"), nil, attribute.AttrLocalPref, 130},
		{"absent med from the default base 0", Bases{},
			set("med", rpc.FilterSetActionIncrement, "50"), nil, attribute.AttrMED, 50},
		{"absent med decrement floors at 0", Bases{},
			set("med", rpc.FilterSetActionDecrement, "30"), nil, attribute.AttrMED, 0},
		{"absent med from a configured base, less", Bases{MED: 50},
			set("med", rpc.FilterSetActionDecrement, "30"), nil, attribute.AttrMED, 20},
		{"a configured base, less more than it holds", Bases{MED: 50},
			set("med", rpc.FilterSetActionDecrement, "80"), nil, attribute.AttrMED, 0},
		{"a base at the top saturates", Bases{MED: 4294967295},
			set("med", rpc.FilterSetActionIncrement, "1"), nil, attribute.AttrMED, 4294967295},
		{"a base at the top, less one", Bases{MED: 4294967295},
			set("med", rpc.FilterSetActionDecrement, "1"), nil, attribute.AttrMED, 4294967294},
		{"a base of 0, plus one", Bases{MED: 0},
			set("med", rpc.FilterSetActionIncrement, "1"), nil, attribute.AttrMED, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			caller := &fakeCaller{filters: map[string]fakeFilter{"ext:num": {
				decl: ExternalDeclaration{Reads: []string{"local-preference"},
					Sets: []string{"local-preference", "med"}, OnError: rpc.OnErrorReject},
				answer: modify(tt.answer),
			}}}
			reg, err := policy.NewRegistry()
			if err != nil {
				t.Fatal(err)
			}
			chain, err := Compile(testAttachment, reg, []Member{{Name: "num", Plugin: "ext"}}, policy.FinalAccept, tt.bases)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			attrs := append([][]byte{originIGP()}, tt.attrs...)
			outcome, ops := runApplied(t, chain, caller, "import", attrs...)
			if outcome.Action != OutcomeAccept {
				t.Fatalf("outcome = %+v, want accept", outcome)
			}
			var want [4]byte
			want[0], want[1], want[2], want[3] = byte(tt.want>>24), byte(tt.want>>16), byte(tt.want>>8), byte(tt.want)
			if len(ops) != 1 || ops[0].Code != uint8(tt.code) || ops[0].Action != filterapi.AttrModSet ||
				!bytes.Equal(ops[0].Buf, want[:]) {
				t.Fatalf("ops = %+v, want one set of attribute %d to %d", ops, tt.code, tt.want)
			}
		})
	}
}
