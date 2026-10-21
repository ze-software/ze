package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// notifyShapedDefs answers the leaves of a command shaped like request peer
// notify: two mandatory octets, an optional hex word, and a rest-of-line text
// declared last.
func notifyShapedDefs() []ArgDef {
	return []ArgDef{
		mustArgDef(NewUintArg("code", 8, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewUintArg("subcode", 8, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewStringArg("hex", nil, nil, ArgOptions{})),
		mustArgDef(NewStringArg("text", nil, nil, ArgOptions{Rest: true})),
	}
}

// TestValidateArgsRestOfLine pins how a ze:rest-of-line leaf takes its words.
//
// GOAL: a free text reaches the handler whole, whatever words it holds, in
// both the positional form an operator types and the keyword form
// WriteInvocation writes.
// METHOD: judge each line against notify-shaped leaves and compare what
// Keyed and PositionalTokens answer.
//
// VALIDATES: restStart, the rest branch of ValidateArgs, Keyed and
// PositionalTokens.
// PREVENTS: a text word that names a leaf read as that leaf's keyword (`the
// code is wrong` refused as an invalid code), or the keyword form refused.
func TestValidateArgsRestOfLine(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		text       string // "" with hasText false: no text
		hasText    bool
		hex        string
		hasHex     bool
		positional []string
	}{
		{"positional text", []string{"6", "2", "going", "down"}, "going down", true, "", false, []string{"6", "2"}},
		{"text naming every leaf", []string{"1", "2", "the", "code", "subcode", "hex"}, "the code subcode hex", true, "", false, []string{"1", "2"}},
		{"text after its keyword", []string{"1", "2", "text", "hex", "ff"}, "hex ff", true, "", false, []string{"1", "2"}},
		{"keyword form", []string{"code", "6", "subcode", "2", "text", "going down"}, "going down", true, "", false, []string{}},
		{"keyword form, hex", []string{"code", "3", "subcode", "1", "hex", "ff"}, "", false, "ff", true, []string{}},
		{"positional hex", []string{"3", "1", "hex", "ff"}, "", false, "ff", true, []string{"3", "1"}},
		{"hex then a word", []string{"3", "1", "hex", "ff", "extra"}, "extra", true, "ff", true, []string{"3", "1"}},
		{"no text", []string{"6", "2"}, "", false, "", false, []string{"6", "2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validated, err := ValidateArgs(tc.args, notifyShapedDefs(), nil)
			require.NoError(t, err)
			text, hasText := validated.Keyed("text")
			assert.Equal(t, tc.hasText, hasText)
			assert.Equal(t, tc.text, text)
			hex, hasHex := validated.Keyed("hex")
			assert.Equal(t, tc.hasHex, hasHex)
			assert.Equal(t, tc.hex, hex)
			assert.Equal(t, tc.positional, validated.PositionalTokens())
			assert.Equal(t, tc.args, validated.Tokens(), "Tokens keeps every token typed")
		})
	}
}

// TestValidateArgsRestOfLineRefusals pins the refusals around a rest leaf.
//
// VALIDATES: a keyword typed last names the fix; a rest leaf on a non-string
// kind, and two rest leaves on one command, are Ze defects refused before any
// token is read.
// PREVENTS: a refusal with no fix, or a model shape whose binding is ambiguous
// accepted in silence.
func TestValidateArgsRestOfLineRefusals(t *testing.T) {
	_, err := ValidateArgs([]string{"6", "2", "hex"}, notifyShapedDefs(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hex requires a value")
	assert.Contains(t, err.Error(), "as in 'hex <hex>'")

	_, err = ValidateArgs([]string{"6", "2", "text"}, notifyShapedDefs(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "as in 'text <text>'")

	notString := []ArgDef{mustArgDef(NewUintArg("count", 8, nil, ArgOptions{Rest: true}))}
	_, err = ValidateArgs([]string{"1"}, notString, nil)
	assert.True(t, errors.Is(err, ErrArgDef), "%v", err)

	two := []ArgDef{
		mustArgDef(NewStringArg("a", nil, nil, ArgOptions{Rest: true})),
		mustArgDef(NewStringArg("b", nil, nil, ArgOptions{Rest: true})),
	}
	_, err = ValidateArgs([]string{"x"}, two, nil)
	assert.True(t, errors.Is(err, ErrArgDef), "%v", err)
}

// TestArgLengthFollowsDeclaredLength pins that a declared length longer than
// the default cap lifts it, and that a long refused value is quoted short.
//
// VALIDATES: argLengthMax and echoValue.
// PREVENTS: a leaf whose model allows 8150 characters refused at 1024 bytes,
// or a refusal that prints thousands of characters.
func TestArgLengthFollowsDeclaredLength(t *testing.T) {
	long := mustArgDef(NewStringArg("hex", []UintRange{{Min: 0, Max: 8150}}, nil, ArgOptions{}))
	value := make([]byte, 8150)
	for i := range value {
		value[i] = 'a'
	}
	require.NoError(t, ValidateArgString(string(value), &long))
	err := ValidateArgString(string(value)+"aa", &long)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "length 8152 out of range 0..8150: give hex a value of 0..8150 characters")
	assert.Less(t, len(err.Error()), 200, "the refusal quotes a short prefix of the value")

	plain := mustArgDef(NewStringArg("name", nil, nil, ArgOptions{}))
	err = ValidateArgString(string(value[:maxArgLength+1]), &plain)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "argument too long (max 1024 bytes)")
	assert.Contains(t, err.Error(), "shorten it to 1024 bytes or fewer")
}
