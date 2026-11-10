package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestDispatchTerminalNounSelectorNamesTheRealFault holds which guard speaks
// for a terminal-noun selector command whose line carries a second fault.
//
// GOAL: the operator reads the fault the line has, never "requires a selector"
// over a line that typed one.
// METHOD: register a command shaped like `create bgp peer <address> asn <asn>`,
// whose own noun is the last key token so its selector is the lone spare token,
// then dispatch lines that type the selector and one other fault, and require
// the whole refusal. Lines with two and three faults require every fault named,
// each on its own line, the missing selector among them when it is one. A line
// with nothing after the path is the boundary: there the selector IS the fault
// and the guard still speaks.
//
// VALIDATES: AC-13 of spec-operator-errors-name-the-fix (journal
// earlier-guard-hides-the-better-error, 2026-09-06 row).
// PREVENTS: the selector guard answering for a fault command.ValidateArgs had
// already named, which sent the operator to repair a selector that was right.
func TestDispatchTerminalNounSelectorNamesTheRealFault(t *testing.T) {
	defs := []command.ArgDef{
		commandtest.Must(command.NewStringArg("selector", nil, nil, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewUintArg("asn", 32, nil, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewStringArg("router-id", nil, nil, command.ArgOptions{})),
	}
	d := NewDispatcher()
	require.NoError(t, d.RegisterWithOptions("create demo peer", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		t.Fatal("handler must not run on a refused line")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Create a peer", RegisterOptions{RequiresSelector: true, ArgDefs: defs}))

	cases := []struct {
		name string
		line string
		want string
	}{
		{
			"a keyword with no value",
			"create demo peer 192.0.2.7 asn 65002 router-id",
			"router-id requires a value: it is the last word, so nothing follows it to read as its value;" +
				" write the value right after the keyword, as in 'router-id <router-id>'",
		},
		{
			"a keyword typed twice",
			"create demo peer 192.0.2.7 asn 65002 asn 65003",
			`duplicate keyword "asn"`,
		},
		{
			"a bad value and a keyword with no value",
			"create demo peer 192.0.2.7 asn x router-id",
			"this command has 2 faults, so Ze did not run it\n" +
				"Ze runs a command only when every word of it is valid, and each line below is one fault:\n" +
				`- invalid value "x", expected unsigned integer` + "\n" +
				"- router-id requires a value: it is the last word, so nothing follows it to read as its value;" +
				" write the value right after the keyword, as in 'router-id <router-id>'\n" +
				"fix: correct each fault listed above, then type the command again",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := d.Dispatch(&CommandContext{}, tc.line)
			require.Error(t, err)
			assert.Equal(t, tc.want, err.Error())
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Equal(t, tc.want, resp.Error)
		})
	}

	t.Run("a missing selector beside two other faults is named with them", func(t *testing.T) {
		const want = "this command has 3 faults, so Ze did not run it\n" +
			"Ze runs a command only when every word of it is valid, and each line below is one fault:\n" +
			`- invalid value "x", expected unsigned integer` + "\n" +
			"- router-id requires a value: it is the last word, so nothing follows it to read as its value;" +
			" write the value right after the keyword, as in 'router-id <router-id>'\n" +
			"- required argument missing: selector\n" +
			"fix: correct each fault listed above, then type the command again"
		resp, err := d.Dispatch(&CommandContext{}, "create demo peer asn x router-id")
		require.Error(t, err)
		assert.Equal(t, want, err.Error())
		require.NotNil(t, resp)
		assert.Equal(t, want, resp.Error)
		faults, several := errors.AsType[*command.ArgumentFaults](err)
		require.True(t, several, "a line with three faults is refused with *command.ArgumentFaults")
		assert.Len(t, faults.Faults(), 3)
		missing, ok := errors.AsType[*command.MissingArgumentError](err)
		require.True(t, ok, "the missing selector stays reachable through errors.As")
		assert.Equal(t, "selector", missing.Leaf)
	})

	t.Run("nothing after the path is still a missing selector", func(t *testing.T) {
		_, err := d.Dispatch(&CommandContext{}, "create demo peer")
		require.Error(t, err)
		assert.Equal(t, "create demo peer requires a selector: create demo peer <selector>", err.Error())
	})
}
