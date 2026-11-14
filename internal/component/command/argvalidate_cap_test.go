package command

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenPastTheByteCapGetsItsLengthRefusal proves a token longer than the
// byte cap of a leaf that declares a length is refused as a length fault, with
// the leaf's own error-message, and never with "shorten it to <cap> bytes",
// which names a size the declared length still refuses.
//
// GOAL: an operator who sends a value far over a declared length reads the
// same refusal as one who sends a value just over it.
// METHOD: one leaf with a declared length and an error-message; a token one
// character over the length (under the cap), and a token one byte over the cap
// (four bytes a character), each through ValidateArgString. A multi-byte token
// whose character count stays inside the length is accepted at the cap.
// VALIDATES: outsideEveryLength, the byte-cap branch of ValidateArgString.
// PREVENTS: the content of update data key, far over its bound, refused with
// a byte count to shorten it to that the bound then refuses again.
func TestTokenPastTheByteCapGetsItsLengthRefusal(t *testing.T) {
	const message = "blob carries at most 2000 characters; send a larger one another way."
	def := mustArgDef(NewStringArg("blob", []UintRange{{Min: 1, Max: 2000}}, nil,
		ArgOptions{LengthMessage: message}))
	limit := argLengthMax(&def)
	require.Equal(t, 8000, limit, "the cap is four bytes a declared character")

	for name, token := range map[string]string{
		"just over the length": strings.Repeat("a", 2001),
		"over the byte cap":    strings.Repeat("a", limit+1),
	} {
		err := ValidateArgString(token, &def)
		require.Error(t, err, name)
		fault, ok := errors.AsType[*ValueArgumentError](err)
		require.True(t, ok, "%s: a length miss is a value fault: %v", name, err)
		assert.Equal(t, message, fault.Why(), name)
		assert.NotContains(t, err.Error(), "argument too long", name)
		assert.NotContains(t, err.Error(), "shorten it to", name)
	}

	wide := strings.Repeat("é", 2000)
	require.LessOrEqual(t, len(wide), limit, "2000 two-byte characters sit under the cap")
	require.NoError(t, ValidateArgString(wide, &def), "a token inside the declared length is accepted")
}

// TestOutsideEveryLengthCountsTheFewestCharacters pins the arithmetic: a token
// of n bytes holds at least ceil(n/4) characters, and is outside the declared
// lengths only when that is above every declared maximum.
//
// VALIDATES: outsideEveryLength.
// PREVENTS: a token that could still fit a declared length given the length
// refusal, or a leaf with no declared length losing its byte-cap refusal.
func TestOutsideEveryLengthCountsTheFewestCharacters(t *testing.T) {
	def := mustArgDef(NewStringArg("blob", []UintRange{{Min: 1, Max: 10}, {Min: 20, Max: 30}}, nil, ArgOptions{}))
	assert.False(t, outsideEveryLength(120, &def), "120 bytes may be 30 characters")
	assert.True(t, outsideEveryLength(121, &def), "121 bytes are at least 31 characters")

	plain := mustArgDef(NewStringArg("name", nil, nil, ArgOptions{}))
	assert.False(t, outsideEveryLength(1<<20, &plain), "a leaf with no declared length keeps the byte cap")
}
