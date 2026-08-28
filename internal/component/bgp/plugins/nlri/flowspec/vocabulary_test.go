// Design: docs/architecture/config/syntax.md -- FlowSpec config route parsing
// Overview: plugin_encode_text.go -- the single protocol and flag vocabularies
package flowspec

import "testing"

// TestConfigProtocolNamesResolve drives the CONFIG parser over every protocol
// name this package accepts and asserts each one produces a match.
//
// VALIDATES: parseFlowProtocolMatches reads protocolNameToNumber, so the config
// path knows every name the text encoder and the JSON decoder know.
// PREVENTS: the divergence that shipped before this test existed. The config
// parser held a private table carrying esp and ah but NOT icmpv6, ospf or sctp.
// An unmatched name is neither an error nor a log in that parser, it is a
// `continue`, so `protocol =sctp` produced a FlowSpec rule with NO protocol
// component at all and matched every protocol -- a firewall rule strictly wider
// than the operator wrote.
func TestConfigProtocolNamesResolve(t *testing.T) {
	for name, number := range protocolNameToNumber {
		matches := parseFlowProtocolMatches("=" + name)
		if len(matches) != 1 {
			t.Errorf("protocol %q: got %d matches, want 1 -- the component is dropped and the rule matches every protocol", name, len(matches))
			continue
		}
		if matches[0].Value != uint64(number) {
			t.Errorf("protocol %q: got value %d, want %d", name, matches[0].Value, number)
		}
	}
}

// TestProtocolNameReverseAgrees asserts the printed name of every protocol
// number round-trips to the same number.
//
// VALIDATES: protocolNumberToName is derived from protocolNameToNumber.
// PREVENTS: a number the decoder prints under a name the config parser then
// refuses, which is what two hand-maintained tables had already produced.
func TestProtocolNameReverseAgrees(t *testing.T) {
	for number, name := range protocolNumberToName {
		back, known := protocolNameToNumber[name]
		if !known {
			t.Errorf("protocol %d prints as %q, which the forward table does not accept", number, name)
			continue
		}
		if back != number {
			t.Errorf("protocol %d prints as %q, which resolves back to %d", number, name, back)
		}
	}
}

// TestFlagNamesRoundTrip asserts every rendered flag name is a name the
// parsers accept, for both bitmask vocabularies.
//
// VALIDATES: tcpFlagsToString and fragmentFlagsToString render through the same
// canonical tables the config parser and the text encoder read.
// PREVENTS: bit 0x08 rendering as "psh" from one function and "push" from the
// other, which is what the two open-coded renderers did.
func TestFlagNamesRoundTrip(t *testing.T) {
	for bit, name := range tcpFlagValueToNames {
		value, known := tcpFlagNameToValue[name]
		if !known {
			t.Errorf("TCP flag 0x%02x renders as %q, which the parser does not accept", bit, name)
			continue
		}
		if value != bit {
			t.Errorf("TCP flag 0x%02x renders as %q, which parses back to 0x%02x", bit, name, value)
		}
		if got := tcpFlagsToString(bit); got != name {
			t.Errorf("tcpFlagsToString(0x%02x) = %q, want %q -- the two renderers disagree", bit, got, name)
		}
	}
	for bit, name := range fragmentFlagValueToNames {
		value, known := fragmentFlagNameToValue[name]
		if !known {
			t.Errorf("fragment flag 0x%02x renders as %q, which the parser does not accept", bit, name)
			continue
		}
		if value != bit {
			t.Errorf("fragment flag 0x%02x renders as %q, which parses back to 0x%02x", bit, name, value)
		}
	}
}

// TestConfigTCPFlagAliasesResolve drives the config parser over every TCP flag
// spelling, aliases included.
//
// VALIDATES: parseFlowTCPFlagMatches reads tcpFlagNameToValue.
// PREVENTS: the config parser accepting `reset` and `urgent` while the text
// encoder rejected them, which is how the two tables differed.
func TestConfigTCPFlagAliasesResolve(t *testing.T) {
	for name, value := range tcpFlagNameToValue {
		matches := parseFlowTCPFlagMatches(name)
		if len(matches) != 1 {
			t.Errorf("tcp flag %q: got %d matches, want 1", name, len(matches))
			continue
		}
		if matches[0].Value != uint64(value) {
			t.Errorf("tcp flag %q: got value 0x%02x, want 0x%02x", name, matches[0].Value, value)
		}
	}
}

// TestConfigFragmentFlagsResolve drives the config parser over every fragment
// flag spelling, the short aliases included.
//
// VALIDATES: parseFlowFragment reads fragmentFlagNameToValue, so the four
// short aliases the text encoder accepted are no longer refused by config.
// PREVENTS: `fragment df` parsing to nothing, which drops the component and
// widens the rule in silence.
func TestConfigFragmentFlagsResolve(t *testing.T) {
	for name, value := range fragmentFlagNameToValue {
		flags := parseFlowFragment(name)
		if len(flags) != 1 {
			t.Errorf("fragment flag %q: got %d flags, want 1", name, len(flags))
			continue
		}
		if uint8(flags[0]) != value {
			t.Errorf("fragment flag %q: got 0x%02x, want 0x%02x", name, uint8(flags[0]), value)
		}
	}
}
