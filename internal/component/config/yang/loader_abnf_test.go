package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseABNFBodyReadsTheRepetitionForms parses rule bodies in each RFC
// 5234 form Section 14 uses, "[x]", "*x", "1*x", "4x", a bare "x", "(a / b)"
// and quoted literals, and compares the trees.
//
// VALIDATES: parseABNFBody, the reader of every rule of the grammar.
func TestParseABNFBodyReadsTheRepetitionForms(t *testing.T) {
	cases := []struct {
		body string
		want abnfExpr
	}{
		{"x", abnfRef{name: "x"}},
		{"[x]", abnfRepeat{min: 0, max: 1, item: abnfRef{name: "x"}}},
		{"*x", abnfRepeat{min: 0, max: repeatUnbounded, item: abnfRef{name: "x"}}},
		{"1*x", abnfRepeat{min: 1, max: repeatUnbounded, item: abnfRef{name: "x"}}},
		{"4DIGIT", abnfRepeat{min: 4, max: 4, item: abnfRef{name: "DIGIT"}}},
		{`a ";" / %s"b"`, abnfAlternation{choices: []abnfExpr{
			abnfSequence{items: []abnfExpr{abnfRef{name: "a"}, abnfTerminal{text: `";"`}}},
			abnfTerminal{text: `%s"b"`},
		}}},
		{"*(a / b)", abnfRepeat{min: 0, max: repeatUnbounded, item: abnfAlternation{choices: []abnfExpr{
			abnfRef{name: "a"}, abnfRef{name: "b"},
		}}}},
	}
	for _, tc := range cases {
		got, err := parseABNFBody(tc.body)
		require.NoError(t, err, tc.body)
		assert.Equal(t, tc.want, got, tc.body)
	}
	for _, broken := range []string{"(a", "[a", "a )", `"a`, "*", "a / "} {
		_, err := parseABNFBody(broken)
		assert.Error(t, err, broken)
	}
	assert.Equal(t, "  a = b \n  c = \";\" ", abnfWithoutComments("  a = b ; comment\n  c = \";\" ; more"))
}
