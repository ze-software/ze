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
			"- "+want[0]+"\n"+
			"- mode: \"eager\" is not an accepted value\n"+
			"  mode accepts only these values:\n"+
			"    active\n"+
			"    passive\n"+
			"- "+want[2]+"\n- "+want[3]+"\n- "+want[4]+", as in 'selector <selector>'\n"+
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

	t.Run("every missing mandatory leaf is named", func(t *testing.T) {
		_, err := ValidateArgs(nil, defs, nil)
		require.Error(t, err)
		faults, several := errors.AsType[*ArgumentFaults](err)
		require.True(t, several, "a line missing two mandatory leaves holds two faults: %v", err)
		got := faults.Faults()
		require.Len(t, got, 2)
		for i, leaf := range []string{"selector", "asn"} {
			missing, ok := errors.AsType[*MissingArgumentError](got[i])
			require.True(t, ok, "fault %d is %v, want *MissingArgumentError", i, got[i])
			assert.Equal(t, leaf, missing.Leaf, "the missing leaves are named in declaration order")
		}
		assert.Equal(t, "this command has 2 faults, so Ze did not run it\n"+
			"Ze runs a command only when every word of it is valid, and each line below is one fault:\n"+
			"- required argument missing: selector, as in 'selector <selector>'\n"+
			"- required argument missing: asn, as in 'asn <asn>'\n"+
			"fix: correct each fault listed above, then type the command again", err.Error())
		for i, fault := range got {
			assert.NotContains(t, fault.Error(), "as in", "fault %d keeps its own text; only the list line names the usage", i)
		}
		first, ok := errors.AsType[*MissingArgumentError](err)
		require.True(t, ok, "errors.As still finds a missing argument inside the list")
		assert.Equal(t, "selector", first.Leaf)
	})

	t.Run("every missing leaf carries the lone binding", func(t *testing.T) {
		three := []ArgDef{
			mustArgDef(NewStringArg("selector", nil, nil, ArgOptions{Mandatory: true})),
			mustArgDef(NewUintArg("asn", 32, nil, ArgOptions{Mandatory: true})),
			mustArgDef(NewUintArg("hold-time", 16, nil, ArgOptions{Mandatory: true})),
		}
		_, err := ValidateArgs([]string{"192.0.2.7"}, three, nil)
		require.Error(t, err)
		faults, several := errors.AsType[*ArgumentFaults](err)
		require.True(t, several, "%v", err)
		got := faults.Faults()
		require.Len(t, got, 2)
		for i, leaf := range []string{"asn", "hold-time"} {
			missing, ok := errors.AsType[*MissingArgumentError](got[i])
			require.True(t, ok, "fault %d is %v, want *MissingArgumentError", i, got[i])
			assert.Equal(t, leaf, missing.Leaf)
			value, found := missing.Positional("selector")
			assert.True(t, found, "missing %s lost the selector binding", leaf)
			assert.Equal(t, "192.0.2.7", value)
		}
	})

	t.Run("a leaf whose keyword is at fault is not also missing", func(t *testing.T) {
		_, err := ValidateArgs([]string{"asn"}, defs, nil)
		require.Error(t, err)
		faults, several := errors.AsType[*ArgumentFaults](err)
		require.True(t, several, "%v", err)
		got := faults.Faults()
		require.Len(t, got, 2, "asn is named once, for its missing value")
		assert.Equal(t, missingValueError("asn").Error(), got[0].Error())
		assert.Equal(t, "required argument missing: selector", got[1].Error())
	})

	t.Run("a missing leaf's usage is the usage line's value form", func(t *testing.T) {
		withEnum := []ArgDef{
			mustArgDef(NewEnumArg("mode", []string{"active", "passive"}, ArgOptions{Mandatory: true})),
			mustArgDef(NewUintArg("asn", 32, nil, ArgOptions{Mandatory: true})),
		}
		_, err := ValidateArgs(nil, withEnum, nil)
		require.Error(t, err)
		faults, several := errors.AsType[*ArgumentFaults](err)
		require.True(t, several, "%v", err)
		missing, ok := errors.AsType[*MissingArgumentError](faults.Faults()[0])
		require.True(t, ok)
		assert.Equal(t, "mode <active|passive>", missing.Usage())
		assert.Contains(t, err.Error(), "\n- required argument missing: mode, as in 'mode <active|passive>'\n")
	})

	t.Run("a restated list names the replacement and its fix", func(t *testing.T) {
		_, err := ValidateArgs(nil, defs, nil)
		faults, several := errors.AsType[*ArgumentFaults](err)
		require.True(t, several, "%v", err)
		replacement := errors.New("the selector line")
		restated := faults.Restated(func(fault error) error {
			missing, ok := errors.AsType[*MissingArgumentError](fault)
			if ok && missing.Leaf == "selector" {
				return replacement
			}
			return nil
		}, "type the whole command")
		assert.Equal(t, "this command has 2 faults, so Ze did not run it\n"+
			"Ze runs a command only when every word of it is valid, and each line below is one fault:\n"+
			"- the selector line\n"+
			"- required argument missing: asn, as in 'asn <asn>'\n"+
			"fix: type the whole command", restated.Error())
		assert.Contains(t, err.Error(), "- required argument missing: selector, as in", "the receiver is not changed")
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
