package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestValidateArgsNamesEveryFault pins that one refusal names every fault a
// command line holds, in the order the line was read.
//
// GOAL: an operator who fixes the fault a refusal named is never refused again
// for a second fault the same line already held.
// METHOD: validate lines that hold one fault of each kind, several stray
// tokens, a single fault, and more faults than the cap, and require the list,
// its rendering, and the errors.As view every route branches on.
//
// VALIDATES: ValidateArgs fault collection, ArgumentFaults, faultList,
// argumentFaultsMax (AC-13 of spec-operator-errors-name-the-fix).
// PREVENTS: ValidateArgs answering only its first fault, so the refusal for a
// line with two faults hid the second one.
func TestValidateArgsNamesEveryFault(t *testing.T) {
	defs := []ArgDef{
		mustArgDef(NewStringArg("selector", nil, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewUintArg("asn", 32, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewStringArg("router-id", nil, nil, ArgOptions{})),
		mustArgDef(NewEnumArg("mode", []string{"active", "passive"}, ArgOptions{})),
	}

	t.Run("one fault of each kind", func(t *testing.T) {
		_, err := ValidateArgs([]string{"asn", "x", "mode", "eager", "mode", "active", "router-id"}, defs, nil)
		require.Error(t, err)
		faults, several := errors.AsType[*ArgumentFaults](err)
		require.True(t, several, "%v", err)

		want := []string{
			ValidateArgString("x", &defs[1]).Error(),
			ValidateArgString("eager", &defs[3]).Error(),
			`duplicate keyword "mode"`,
			missingValueError("router-id").Error(),
			"required argument missing: selector",
		}
		got := make([]string, 0, len(want))
		for _, fault := range faults.Faults() {
			got = append(got, fault.Error())
		}
		assert.Equal(t, want, got)

		assert.Equal(t, "this command has 5 faults, so Ze did not run it\n"+
			"Ze runs a command only when every word of it is valid, and each line below is one fault:\n"+
			"- "+want[0]+"\n- "+want[1]+"\n- "+want[2]+"\n- "+want[3]+"\n- "+want[4]+"\n"+
			"fix: correct each fault listed above, then type the command again", err.Error())

		missing, ok := errors.AsType[*MissingArgumentError](err)
		require.True(t, ok, "the missing argument is reachable inside the list")
		assert.Equal(t, "selector", missing.Leaf)
		refusal, ok := errors.AsType[*opererr.Error](err)
		require.True(t, ok, "the web notice reads the three parts")
		assert.Equal(t, "this command has 5 faults, so Ze did not run it", refusal.Problem())
		assert.Equal(t, "correct each fault listed above, then type the command again", refusal.Fix())
	})

	t.Run("each stray token is named, and its leaf is not also missing", func(t *testing.T) {
		two := []ArgDef{
			mustArgDef(NewUintArg("first", 8, nil, ArgOptions{Mandatory: true})),
			mustArgDef(NewUintArg("second", 8, nil, ArgOptions{Mandatory: true})),
		}
		_, err := ValidateArgs([]string{"x", "y"}, two, nil)
		require.Error(t, err)
		faults, several := errors.AsType[*ArgumentFaults](err)
		require.True(t, several, "%v", err)
		got := faults.Faults()
		require.Len(t, got, 2)
		assert.Equal(t, `unexpected argument "x", valid keywords: first, second`, got[0].Error())
		assert.Equal(t, `unexpected argument "y", valid keywords: first, second`, got[1].Error())
		_, missing := errors.AsType[*MissingArgumentError](err)
		assert.False(t, missing, "a leaf a stray token was typed for is not reported missing too")
	})

	t.Run("one fault is refused as itself", func(t *testing.T) {
		_, err := ValidateArgs([]string{"192.0.2.7", "asn", "65002", "router-id"}, defs, nil)
		require.Error(t, err)
		_, several := errors.AsType[*ArgumentFaults](err)
		assert.False(t, several, "%v", err)
		assert.Equal(t, missingValueError("router-id").Error(), err.Error())
	})

	t.Run("the list is bounded and counts what it leaves out", func(t *testing.T) {
		args := []string{"192.0.2.7", "asn", "65002"}
		const duplicates = argumentFaultsMax + 4
		for range duplicates {
			args = append(args, "asn", "1")
		}
		_, err := ValidateArgs(args, defs, nil)
		require.Error(t, err)
		faults, several := errors.AsType[*ArgumentFaults](err)
		require.True(t, several, "%v", err)
		assert.Len(t, faults.Faults(), argumentFaultsMax)
		assert.Contains(t, err.Error(), "this command has 20 faults, so Ze did not run it\n")
		assert.Contains(t, err.Error(), "\n- and 4 more faults not listed here\nfix: ")
	})
}
