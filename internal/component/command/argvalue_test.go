package command

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// encodingHelp is the ze:help of the encoding values these tests refuse.
var encodingHelp = map[string]string{"hex": "Hexadecimal digits", "b64": "Base64 text"}

// rawDefs are the definitions of a raw message command: an encoding word and
// hex data, both mandatory, so a bad encoding is the line's one value fault.
func rawDefs() []ArgDef {
	return []ArgDef{
		mustArgDef(NewEnumArg("encoding", []string{"hex", "b64"}, ArgOptions{Mandatory: true, EnumHelp: encodingHelp})),
		mustArgDef(NewStringArg("data", nil, []*regexp.Regexp{regexp.MustCompile(`^[0-9A-Fa-f]+$`)}, ArgOptions{Mandatory: true})),
	}
}

// requireValueFault answers the value fault err is, failing the test when it
// is not one.
func requireValueFault(t *testing.T, err error) *ValueArgumentError {
	t.Helper()
	require.Error(t, err)
	fault, ok := errors.AsType[*ValueArgumentError](err)
	require.True(t, ok, "want a *ValueArgumentError, got %T: %v", err, err)
	return fault
}

// TestValidateArgEnumRefusal proves a word outside an enumeration or a union is
// refused as the config value refusal reads.
//
// GOAL: an operator who mistypes an enum argument reads the argument, the
// value, every accepted value with its help in the model's order, and the
// value to type instead.
// METHOD: refuse a far value and a close value of an enum argument with help,
// and a value no member of a union with an enum member accepts.
// VALIDATES: AC-12 (validateEnum, validateUnion, enumFault, unionFault).
// PREVENTS: `invalid value "gzip", expected one of: hex, b64`, which named no
// meaning for any value and no command to type.
func TestValidateArgEnumRefusal(t *testing.T) {
	encoding := rawDefs()[0]

	t.Run("far value", func(t *testing.T) {
		fault := requireValueFault(t, ValidateArgString("gzip", &encoding))
		assert.Equal(t, `encoding: "gzip" is not an accepted value`, fault.Problem())
		assert.Equal(t, "encoding accepts only these values:\n  hex  Hexadecimal digits\n  b64  Base64 text", fault.Why())
		assert.Equal(t, "<hex|b64>", fault.Correction())
		assert.NotContains(t, fault.Error(), "expected one of")
	})

	t.Run("close value", func(t *testing.T) {
		fault := requireValueFault(t, ValidateArgString("hx", &encoding))
		assert.Equal(t, `encoding: "hx" is not an accepted value (did you mean hex?)`, fault.Problem())
		assert.Equal(t, "hex", fault.Correction())
	})

	t.Run("union with an enum member", func(t *testing.T) {
		limit := mustArgDef(NewUnionArg("limit", []ArgDef{
			mustArgDef(NewUintArg("limit", 32, nil, ArgOptions{})),
			mustArgDef(NewEnumArg("limit", []string{"unlimited"}, ArgOptions{EnumHelp: map[string]string{"unlimited": "No limit"}})),
		}, ArgOptions{}))
		fault := requireValueFault(t, ValidateArgString("lots", &limit))
		assert.Equal(t, `limit: "lots" is not an accepted value`, fault.Problem())
		assert.Equal(t, "limit accepts one of these values:\n  unlimited  No limit\nor a value of type uint32", fault.Why())
		assert.Equal(t, "<unlimited|uint32>", fault.Correction())
		assert.NotContains(t, fault.Error(), "or one of")
	})
}

// TestValidateArgRangeLengthPatternRefusal proves a range, length or pattern
// miss is refused with its kind's problem, bounds or format, and placeholder.
//
// GOAL: the same refusal kinds as a config value, on a command argument.
// METHOD: refuse a value of each kind, with and without a declared
// error-message, and a pattern with and without format words.
// VALIDATES: AC-38 (rangeFault, lengthFault, patternFault).
// PREVENTS: `value 0 out of range`, `length 5 out of range` and `does not match
// expected pattern`, which named no fix and, for a pattern, no form.
func TestValidateArgRangeLengthPatternRefusal(t *testing.T) {
	t.Run("range", func(t *testing.T) {
		spi := mustArgDef(NewUintArg("spi", 32, []UintRange{{Min: 256, Max: 4294967295}}, ArgOptions{}))
		fault := requireValueFault(t, ValidateArgString("0", &spi))
		assert.Equal(t, `spi: "0" is outside the accepted range`, fault.Problem())
		assert.Equal(t, "spi accepts a value in 256..4294967295", fault.Why())
		assert.Equal(t, "<256..4294967295>", fault.Correction())
	})

	t.Run("range with a declared message", func(t *testing.T) {
		vlan := mustArgDef(NewUintArg("vlan", 16, []UintRange{{Min: 1, Max: 4094}}, ArgOptions{RangeMessage: "a VLAN id is 1 to 4094."}))
		fault := requireValueFault(t, ValidateArgString("4095", &vlan))
		assert.Equal(t, "a VLAN id is 1 to 4094.", fault.Why())
		assert.Equal(t, "<1..4094>", fault.Correction())
	})

	t.Run("not a number keeps the type refusal", func(t *testing.T) {
		spi := mustArgDef(NewUintArg("spi", 32, []UintRange{{Min: 256, Max: 4294967295}}, ArgOptions{}))
		err := ValidateArgString("x", &spi)
		require.Error(t, err)
		_, value := errors.AsType[*ValueArgumentError](err)
		assert.False(t, value, "a type refusal is not a value fault")
		assert.Equal(t, `invalid value "x", expected unsigned integer`, err.Error())
	})

	t.Run("length", func(t *testing.T) {
		name := mustArgDef(NewStringArg("name", []UintRange{{Min: 1, Max: 4}}, nil, ArgOptions{}))
		fault := requireValueFault(t, ValidateArgString("abcde", &name))
		assert.Equal(t, `name: "abcde" is outside the accepted length (5 characters)`, fault.Problem())
		assert.Equal(t, "name accepts 1..4 characters", fault.Why())
		assert.Equal(t, "<1..4 characters>", fault.Correction())
	})

	t.Run("pattern", func(t *testing.T) {
		lower := regexp.MustCompile(`^[a-z]+$`)
		cases := []struct {
			name        string
			opts        ArgOptions
			why         string
			placeholder string
		}{
			{"format words", ArgOptions{FormatText: "a host name", FormatType: "host-name"}, "host accepts a host name", "<host-name>"},
			{"typedef only", ArgOptions{FormatType: "host-name"}, "host accepts a value of type host-name", "<host-name>"},
			{"inline", ArgOptions{}, "host does not accept this value", "<value>"},
			{"declared message", ArgOptions{PatternMessages: []string{"lower case letters only."}}, "lower case letters only.", "<value>"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				host := mustArgDef(NewStringArg("host", nil, []*regexp.Regexp{lower}, tc.opts))
				fault := requireValueFault(t, ValidateArgString("Z9", &host))
				assert.Equal(t, `host: "Z9" is not in the accepted format`, fault.Problem())
				assert.Equal(t, tc.why, fault.Why())
				assert.Equal(t, tc.placeholder, fault.Correction())
				assert.NotContains(t, fault.Error(), "[a-z]", "the regular expression is never shown")
			})
		}
	})
}

// TestOneValueFaultIsRestated proves a line whose one fault is a refused value
// is a refusal of one fault that every route restates with the whole command.
//
// GOAL: `send bgp 192.0.2.1 raw gzip DEADBEEF` is told the command to type,
// with the placeholder where gzip was, and no count of faults.
// METHOD: refuse the line with ValidateArgs, then restate it as a route does,
// and read the refusal before and after.
// VALIDATES: AC-12 one-fault case (faultList.verdict, newArgumentFaults,
// RestatedForLine, wholeCommand).
// PREVENTS: a lone value fault returned bare, which no route restated, so it
// carried no fix at all.
func TestOneValueFaultIsRestated(t *testing.T) {
	args := []string{"gzip", "DEADBEEF"}
	_, err := ValidateArgs(args, rawDefs(), nil)
	require.Error(t, err)
	faults, listed := errors.AsType[*ArgumentFaults](err)
	require.True(t, listed, "a lone value fault is an *ArgumentFaults of one: %v", err)
	require.Len(t, faults.Faults(), 1)
	_ = requireValueFault(t, err)

	const values = "encoding accepts only these values:\n  hex  Hexadecimal digits\n  b64  Base64 text\n"
	assert.Equal(t, `encoding: "gzip" is not an accepted value`+"\n"+values+
		`fix: type the command again with <hex|b64> in place of "gzip"`, err.Error())

	restated := faults.RestatedForLine(nil, "send bgp raw", "send bgp 192.0.2.1 raw", args, "", "")
	assert.Equal(t, `send bgp raw encoding: "gzip" is not an accepted value`+"\n"+values+
		"fix: send bgp 192.0.2.1 raw <hex|b64> DEADBEEF", restated.Error())
	assert.NotContains(t, restated.Error(), "faults, so Ze did not run it", "one fault carries no count")

	refusal, ok := errors.AsType[*opererr.Error](restated)
	require.True(t, ok, "the web notice reads the three parts")
	assert.Equal(t, `send bgp raw encoding: "gzip" is not an accepted value`, refusal.Problem())
	assert.Equal(t, "send bgp 192.0.2.1 raw <hex|b64> DEADBEEF", refusal.Fix())

	t.Run("a close value is the correction", func(t *testing.T) {
		_, err := ValidateArgs([]string{"hx", "DEADBEEF"}, rawDefs(), nil)
		faults, listed := errors.AsType[*ArgumentFaults](err)
		require.True(t, listed, "%v", err)
		restated := faults.RestatedForLine(nil, "send bgp raw", "send bgp 192.0.2.1 raw", []string{"hx", "DEADBEEF"}, "", "")
		assert.True(t, strings.HasSuffix(restated.Error(), "\nfix: send bgp 192.0.2.1 raw hex DEADBEEF"), restated.Error())
	})

	t.Run("a rest-of-line value is replaced whole", func(t *testing.T) {
		text := mustArgDef(NewStringArg("text", []UintRange{{Min: 1, Max: 8}}, nil, ArgOptions{Rest: true}))
		line := []string{"text", "far", "too", "long"}
		_, err := ValidateArgs(line, []ArgDef{text}, nil)
		faults, listed := errors.AsType[*ArgumentFaults](err)
		require.True(t, listed, "%v", err)
		restated := faults.RestatedForLine(nil, "show note", "show note", line, "", "")
		assert.True(t, strings.HasSuffix(restated.Error(), "\nfix: show note text <1..8 characters>"), restated.Error())
	})
}

// TestFaultEntryHoldsNoFixLine proves a value fault beside another fault reads
// in the list as its problem and its why, and the refusal's one fix is its
// last line.
//
// GOAL: a line with a bad enum and a missing argument is one refusal naming
// both, with the whole command, placeholder and usage written, as its fix.
// METHOD: refuse such a line, restate it, and read every line of the refusal.
// VALIDATES: AC-12 two-fault case (faultLine, ValueArgumentError.listEntry).
// PREVENTS: a fault entry carrying a fix line of its own, which put a `fix:`
// before the refusal's last line.
func TestFaultEntryHoldsNoFixLine(t *testing.T) {
	defs := append(rawDefs(), mustArgDef(NewUintArg("count", 8, nil, ArgOptions{Mandatory: true})))
	args := []string{"encoding", "gzip", "DEADBEEF"}
	_, err := ValidateArgs(args, defs, nil)
	faults, listed := errors.AsType[*ArgumentFaults](err)
	require.True(t, listed, "%v", err)
	require.Len(t, faults.Faults(), 2)

	restated := faults.RestatedForLine(nil, "send bgp raw", "send bgp 192.0.2.1 raw", args, "", "")
	assert.Equal(t, "this command has 2 faults, so Ze did not run it\n"+
		"Ze runs a command only when every word of it is valid, and each line below is one fault:\n"+
		`- send bgp raw encoding: "gzip" is not an accepted value`+"\n"+
		"  encoding accepts only these values:\n"+
		"    hex  Hexadecimal digits\n"+
		"    b64  Base64 text\n"+
		"- required argument missing: count, as in 'count <count>'\n"+
		"fix: send bgp 192.0.2.1 raw encoding <hex|b64> DEADBEEF count <count>", restated.Error())
	assert.Equal(t, 1, strings.Count(restated.Error(), "fix:"), "the list holds one fix line, the last")
}

// TestMistypedEarlierTokenIsTheOneRefused proves the token blamed is the one
// typed where the refusing leaf goes, when a later, looser leaf could take it.
//
// GOAL: `raw gzip DEADBEEF`, whose data leaf is a free string as the shipped
// model declares it, refuses gzip as the encoding and keeps DEADBEEF as the
// data, as the usage line `raw <encoding> <data>` reads.
// METHOD: refuse the line with ValidateArgs and restate it as a route does.
// VALIDATES: reattributeUnplaced (AC-12, the fix replaces the word typed).
// PREVENTS: data taking gzip, DEADBEEF blamed as the encoding, and a fix that
// kept gzip: `fix: send bgp 192.0.2.1 raw gzip <hex|b64>`.
func TestMistypedEarlierTokenIsTheOneRefused(t *testing.T) {
	defs := []ArgDef{
		rawDefs()[0],
		mustArgDef(NewStringArg("data", nil, nil, ArgOptions{Mandatory: true})),
	}
	args := []string{"gzip", "DEADBEEF"}
	_, err := ValidateArgs(args, defs, nil)
	faults, listed := errors.AsType[*ArgumentFaults](err)
	require.True(t, listed, "%v", err)
	fault := requireValueFault(t, err)
	assert.Equal(t, "gzip", fault.Value())
	restated := faults.RestatedForLine(nil, "send bgp raw", "send bgp 192.0.2.1 raw", args, "", "")
	assert.True(t, strings.HasPrefix(restated.Error(), `send bgp raw encoding: "gzip" is not an accepted value`+"\n"), restated.Error())
	assert.True(t, strings.HasSuffix(restated.Error(), "\nfix: send bgp 192.0.2.1 raw <hex|b64> DEADBEEF"), restated.Error())

	_, err = ValidateArgs([]string{"hex", "DEADBEEF"}, defs, nil)
	require.NoError(t, err, "a correct line still binds each token to its leaf")
}
