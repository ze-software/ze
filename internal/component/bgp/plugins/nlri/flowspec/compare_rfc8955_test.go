package flowspec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC8955-5.1-1 positive -- packet-filter precedence compares component types, prefixes, and received operator bytes (§5.1).
// RFC requirement: RFC8955-5.1-1 negative -- reversing arrival order cannot reverse the precedence of distinct rules (§5.1).
// RFC requirement: RFC8955-5.1-2 positive -- distinct component types, disjoint prefixes, and operator bytes use the specified ordering (§5.1).
// RFC requirement: RFC8955-5.1-2 negative -- numeric value ordering or a canonicalized operand width cannot replace received-byte precedence (§5.1).
// RFC requirement: RFC5575-5.1-1 positive -- overlapping destination prefixes sort most-specific first (§5.1).
// RFC requirement: RFC5575-5.1-1 negative -- a covering prefix cannot precede a more-specific overlapping prefix (§5.1).
func TestFlowSpecPrecedenceFromWire(t *testing.T) {
	pairs := []struct {
		name          string
		first, second []byte
	}{
		{"overlap", []byte{5, 1, 24, 10, 0, 1}, []byte{3, 1, 8, 10}},
		{"disjoint", []byte{3, 1, 8, 10}, []byte{5, 1, 24, 192, 0, 2}},
		{"component-type", []byte{3, 3, 0x81, 17}, []byte{3, 5, 0x81, 80}},
		{"operator-bytes", []byte{3, 5, 0x81, 80}, []byte{3, 5, 0x81, 81}},
		// Numerically 256 exceeds 1, but the width/operator byte decides
		// first. Reencoding the second operand into one byte changes order.
		{"received-width", []byte{4, 5, 0x91, 1, 0}, []byte{6, 5, 0xa1, 0, 0, 0, 1}},
		{"extra-component", []byte{6, 3, 0x81, 6, 5, 0x81, 80}, []byte{3, 3, 0x81, 6}},
	}
	for _, pair := range pairs {
		t.Run(pair.name, func(t *testing.T) {
			a, err := ParseFlowSpec(IPv4FlowSpec, pair.first)
			require.NoError(t, err)
			b, err := ParseFlowSpec(IPv4FlowSpec, pair.second)
			require.NoError(t, err)
			require.Negative(t, Compare(a, b))
			require.Positive(t, Compare(b, a))
			require.Zero(t, Compare(a, a))
		})
	}
}

// RFC requirement: RFC8955-4.2.1.1-3 positive -- reserved numeric bits and a leading AND bit do not change the received numeric predicate (§4.2.1.1).
// RFC requirement: RFC8955-4.2.1.1-3 negative -- retransmission clears reserved bits and never encodes AND on the first operator (§4.2.1.1).
func TestFlowSpecNumericReservedAndEightOctetOperand(t *testing.T) {
	// The high octet distinguishes the operand from a truncated uint32.
	wire := []byte{10, 5, 0xf9, 1, 0, 0, 0, 0, 0, 0, 7}
	fs, err := ParseFlowSpec(IPv4FlowSpec, wire)
	require.NoError(t, err)
	matches := fs.Components()[0].(*numericComponent).Matches()
	require.Equal(t, []FlowMatch{{Op: FlowOpEqual, Value: 1<<56 | 7}}, matches)
	require.Equal(t, []byte{10, 5, 0xb1, 1, 0, 0, 0, 0, 0, 0, 7}, fs.Bytes())
	_, err = ParseFlowSpec(IPv4FlowSpec, []byte{3, 5, 0x01, 80})
	require.Error(t, err, "missing EOL cannot consume a following component as another operand")
}
