// Design: docs/architecture/wire/nlri-flowspec.md -- FlowSpec protocol and flag name tables
// Overview: vocabulary.go -- the single protocol and flag vocabularies
package flowspec

import (
	"testing"
)

// TestProtocolNameReverseAgrees asserts the printed name of every protocol
// number round-trips to the same number.
//
// VALIDATES: protocolNumberToName is derived from ProtocolNameToNumber.
// PREVENTS: a number the decoder prints under a name the config parser then
// refuses, which is what two hand-maintained tables had already produced.
func TestProtocolNameReverseAgrees(t *testing.T) {
	for number, name := range protocolNumberToName {
		back, known := ProtocolNameToNumber[name]
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
	for bit, name := range TCPFlagValueToNames {
		value, known := TCPFlagNameToValue[name]
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
	for bit, name := range FragmentFlagValueToNames {
		value, known := FragmentFlagNameToValue[name]
		if !known {
			t.Errorf("fragment flag 0x%02x renders as %q, which the parser does not accept", bit, name)
			continue
		}
		if value != bit {
			t.Errorf("fragment flag 0x%02x renders as %q, which parses back to 0x%02x", bit, name, value)
		}
	}
}
