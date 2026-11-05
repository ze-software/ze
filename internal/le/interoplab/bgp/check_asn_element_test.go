// Design: docs/architecture/testing/interop.md -- FRR route evidence retains typed ASN values.
package bgp

import (
	"encoding/json"
	"testing"
)

// TestFRRASNElementContract checks the numeric and textual forms FRR's
// segments[].list can carry. Numeric JSON stays an unsigned integer; a decimal
// point is asdot notation only inside a string, never in a JSON number.
// MUTATION: parse numeric JSON through the textual ASN reader and 1.2 becomes
// AS 65538; decode null into a uint32 without a presence check and it becomes 0.
func TestFRRASNElementContract(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  uint32
	}{
		{name: "numeric zero", input: "0", want: 0},
		{name: "numeric two octets", input: "65000", want: 65000},
		{name: "numeric maximum", input: "4294967295", want: 4294967295},
		{name: "text asplain", input: `"65000"`, want: 65000},
		{name: "text asdot", input: `"64086.59904"`, want: 4200000000},
		{name: "text asdot maximum", input: `"65535.65535"`, want: 4294967295},
		{name: "text asdot zero", input: `"0.0"`, want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := frrASPathElement(json.RawMessage(test.input))
			if err != nil {
				t.Fatalf("element %s refused: %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("element %s = %d, want %d", test.input, got, test.want)
			}
		})
	}
	for _, test := range []struct {
		name  string
		input string
	}{
		{name: "null", input: "null"},
		{name: "boolean", input: "true"},
		{name: "array", input: "[]"},
		{name: "object", input: "{}"},
		{name: "negative", input: "-1"},
		{name: "overflow", input: "4294967296"},
		{name: "fraction is not asdot", input: "1.2"},
		{name: "integral float", input: "1.0"},
		{name: "exponent", input: "1e2"},
		{name: "empty text", input: `""`},
		{name: "text prefix", input: `"AS65000"`},
		{name: "text overflow", input: `"4294967296"`},
		{name: "text high overflow", input: `"65536.0"`},
		{name: "text low overflow", input: `"0.65536"`},
		{name: "text missing half", input: `"1."`},
		{name: "text extra half", input: `"1.2.3"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := frrASPathElement(json.RawMessage(test.input)); err == nil {
				t.Fatalf("invalid element %s accepted as AS %d", test.input, got)
			}
		})
	}
}
