package flowspec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBitmaskGrammarConfigAndTextAgree drives the config parser and the update
// text encoder over the same tcp-flags and fragment spellings and compares
// both with the wire octets the spelling means.
//
// VALIDATES: one bitmask grammar on both surfaces, ExaBGP 6.0's: "&" joins
// terms with the AND bit, a space (or a bracketed list) joins them with OR,
// "+" names several bits of ONE operand, and "=", "!" and "!=" set the match
// and NOT bits of the term they lead.
// PREVENTS: `tcp-flags syn&ack` selecting SYN-and-ACK packets from config and
// SYN-or-ACK packets from update text, as it did until 2026-10-21.
//
// RFC 8955 Section 4.2.1.2: "m (Match bit): If set, this is a bitwise match
// operation defined as "(data AND value) == value"; if unset, (data AND value)
// evaluates to TRUE if any of the bits in the value mask are set in the data.".
func TestBitmaskGrammarConfigAndTextAgree(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kw    string
		value string
		wire  []byte
	}{
		// two terms, the second ANDed: SYN set AND ACK set
		{"tcp-flags", "syn&ack", []byte{0x05, 0x09, 0x00, 0x02, 0xc0, 0x10}},
		// one operand of two bits: SYN or ACK set
		{"tcp-flags", "syn+ack", []byte{0x03, 0x09, 0x80, 0x12}},
		// two OR terms
		{"tcp-flags", "[ syn ack ]", []byte{0x05, 0x09, 0x00, 0x02, 0x80, 0x10}},
		{"tcp-flags", "=syn", []byte{0x03, 0x09, 0x81, 0x02}},
		{"tcp-flags", "!syn", []byte{0x03, 0x09, 0x82, 0x02}},
		{"tcp-flags", "!=syn", []byte{0x03, 0x09, 0x83, 0x02}},
		// the operator belongs to the term it leads
		{"tcp-flags", "=syn&!rst", []byte{0x05, 0x09, 0x01, 0x02, 0xc2, 0x04}},
		{"tcp-flags", "=syn+ack", []byte{0x03, 0x09, 0x81, 0x12}},
		{"tcp-flags", "0x12", []byte{0x03, 0x09, 0x80, 0x12}},
		{"fragment", "is-fragment&last-fragment", []byte{0x05, 0x0c, 0x00, 0x02, 0xc0, 0x08}},
		{"fragment", "first-fragment+last-fragment", []byte{0x03, 0x0c, 0x80, 0x0c}},
		{"fragment", "!is-fragment", []byte{0x03, 0x0c, 0x82, 0x02}},
	}
	for _, tt := range tests {
		t.Run(tt.kw+" "+tt.value, func(t *testing.T) {
			t.Parallel()
			fs, dropped := buildFlowSpecComponents(map[string][]string{tt.kw: {tt.value}}, false)
			require.Empty(t, dropped, "config refused %s %s", tt.kw, tt.value)
			require.Equal(t, tt.wire, fs.Bytes(), "config %s %s", tt.kw, tt.value)

			args := append([]string{tt.kw}, strings.Fields(strings.NewReplacer("[", "", "]", "").Replace(tt.value))...)
			text, err := EncodeFlowSpecComponents(IPv4FlowSpec, args)
			require.NoError(t, err)
			require.Equal(t, tt.wire, text, "update text %s %s", tt.kw, tt.value)
		})
	}
}

// TestBitmaskGrammarRefusals checks both surfaces refuse the same malformed
// bitmask terms with a reason, rather than dropping or widening them.
//
// VALIDATES: an "&" with nothing before it or after it, a dangling "+", an empty operand and
// an unknown flag are refused by config and by update text alike.
// PREVENTS: a term vanishing from one surface, which widens the rule.
func TestBitmaskGrammarRefusals(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"&syn", "syn&", "syn+", "=", "bogus", "syn&&ack"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			_, dropped := buildFlowSpecComponents(map[string][]string{"tcp-flags": {value}}, false)
			require.NotEmpty(t, dropped, "config accepted tcp-flags %q", value)

			_, err := EncodeFlowSpecComponents(IPv4FlowSpec, []string{"tcp-flags", value})
			require.Error(t, err, "update text accepted tcp-flags %q", value)
		})
	}
}
