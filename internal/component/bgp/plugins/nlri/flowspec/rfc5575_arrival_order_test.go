package flowspec

import (
	"bytes"
	"slices"
	"testing"
)

// TestRFC5575PrecedenceIndependentOfArrival parses and installs the same rules
// in opposite arrival orders, then applies the production ordering function.
// RFC 5575 Section 5.1: "This ordering function must be such that it must not
// depend on the arrival order of the flow specification's rules and must be
// constant in the network."
// RFC requirement: RFC5575-5.1-1 positive -- a /24 precedes its covering /8 and a lower component type precedes a higher one after sorting received rules.
// RFC requirement: RFC5575-5.1-1 negative -- reversing parsing and insertion order cannot reverse either pair's resulting precedence.
func TestRFC5575PrecedenceIndependentOfArrival(t *testing.T) {
	for _, pair := range [][2][]byte{
		{{5, 1, 24, 10, 0, 1}, {3, 1, 8, 10}},
		{{3, 3, 0x81, 17}, {3, 5, 0x81, 80}},
	} {
		for _, order := range [][2]int{{0, 1}, {1, 0}} {
			var received []*FlowSpec
			for _, index := range order {
				rule, err := ParseFlowSpec(IPv4FlowSpec, bytes.Clone(pair[index]))
				if err != nil {
					t.Fatal(err)
				}
				received = append(received, rule)
			}
			slices.SortFunc(received, Compare)
			for index, rule := range received {
				if !bytes.Equal(rule.Bytes(), pair[index]) {
					t.Fatalf("arrival %v: precedence %d is %x, want %x", order, index, rule.Bytes(), pair[index])
				}
			}
		}
	}
}
