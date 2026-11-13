// Design: docs/architecture/api/commands.md -- typed refusals give an exact operational command.

package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	plugin "github.com/ze-software/ze/internal/component/plugin"
)

// TestOperationalEnumFixReplaysLiteralValue takes the actual last-line fix
// through the normal checked pipeline entry and dispatcher, proving the handler
// receives the declared value byte for byte and no handler runs on the refusal.
// Placeholders that happen to resemble accepted enum data cannot replace it.
func TestOperationalEnumFixReplaysLiteralValue(t *testing.T) {
	const path = "show demo literal"
	cases := []struct {
		name   string
		values []string
		bad    string
		want   string
		union  bool
	}{
		{name: "spaced singleton", values: []string{"a b"}, bad: "wrong", want: "a b"},
		{name: "literal pipe", values: []string{"a|b"}, bad: "wrong", want: "a|b"},
		{name: "embedded quote", values: []string{`a"b`}, bad: "wrong", want: `a"b`},
		{name: "escaped quote before pipe", values: []string{`a"|b`}, bad: "wrong", want: `a"|b`},
		{name: "backslash", values: []string{`a\b`}, bad: "wrong", want: `a\b`},
		{name: "line break", values: []string{"a\nb"}, bad: "wrong", want: "a\nb"},
		{name: "carriage return", values: []string{"a\rb"}, bad: "wrong", want: "a\rb"},
		{name: "tab", values: []string{"a\tb"}, bad: "wrong", want: "a\tb"},
		{name: "Unicode whitespace edges", values: []string{"\u00a0a\u00a0"}, bad: "wrong", want: "\u00a0a\u00a0"},
		{name: "leading apostrophe", values: []string{"'tis"}, bad: "wrong", want: "'tis"},
		{name: "literal placeholder", values: []string{"<a|b>"}, bad: "wrong", want: "<a|b>"},
		{name: "suggestion", values: []string{"alpha beta", "other"}, bad: "alpha bet", want: "alpha beta"},
		{name: "union suggestion", values: []string{"alpha beta"}, bad: "alpha bet", want: "alpha beta", union: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode := commandtest.Must(command.NewEnumArg("mode", tc.values, command.ArgOptions{Mandatory: true}))
			if tc.union {
				mode = commandtest.Must(command.NewUnionArg("mode", []command.ArgDef{
					mode, commandtest.Must(command.NewUintArg("mode", 32, nil, command.ArgOptions{})),
				}, command.ArgOptions{Mandatory: true}))
			}
			defs := []command.ArgDef{mode, commandtest.Must(command.NewStringArg("payload", nil, nil, command.ArgOptions{Mandatory: true}))}
			d := NewDispatcher()
			calls := 0
			require.NoError(t, d.RegisterWithOptions(path, func(_ *CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
				calls++
				value, found := args.Value("mode")
				require.True(t, found)
				require.Equal(t, tc.want, value)
				payload, found := args.Value("payload")
				require.True(t, found)
				require.Equal(t, "keep|tail", payload, "an already accepted operand is preserved too")
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "Replay a literal enum correction", RegisterOptions{ArgDefs: defs, ReadOnly: true}))

			// The refused input is independently spelled, so the new writer is
			// not also the test's oracle for the original value or pipe quoting.
			bad := tc.bad
			if strings.ContainsRune(bad, ' ') {
				bad = `"` + bad + `"`
			}
			_, err := d.Dispatch(&CommandContext{}, path+" mode "+bad+` payload "keep|tail"`)
			require.Error(t, err)
			require.Zero(t, calls)
			_, fix, found := strings.Cut(err.Error(), "\nfix: ")
			require.True(t, found, "refusal must end with the actual correction: %v", err)
			require.NotContains(t, fix, "\n", "the exact correction is one command line")

			line, _, message := command.ProcessPipesDefaultFormatChecked(fix, "text")
			require.Empty(t, message)
			require.Equal(t, fix, line, "a literal pipe cannot become a pipeline")
			_, err = d.Dispatch(&CommandContext{}, line)
			require.NoError(t, err, "printed correction %q", fix)
			require.Equal(t, 1, calls)
		})
	}
}

// TestOperationalEnumFixKeepsMissingArgument proves concrete correction
// quoting and the existing missing-argument placeholder coexist in one fix.
func TestOperationalEnumFixKeepsMissingArgument(t *testing.T) {
	const path = "show demo missing"
	defs := []command.ArgDef{
		commandtest.Must(command.NewEnumArg("mode", []string{"a|b"}, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewStringArg("payload", nil, nil, command.ArgOptions{Mandatory: true})),
	}
	d := NewDispatcher()
	calls := 0
	require.NoError(t, d.RegisterWithOptions(path, func(_ *CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		calls++
		mode, found := args.Value("mode")
		require.True(t, found)
		require.Equal(t, "a|b", mode)
		payload, found := args.Value("payload")
		require.True(t, found)
		require.Equal(t, "complete", payload)
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Replay a correction with a missing argument", RegisterOptions{ArgDefs: defs, ReadOnly: true}))
	_, err := d.Dispatch(&CommandContext{}, path+" mode wrong")
	require.Error(t, err)
	require.Zero(t, calls)
	_, fix, found := strings.Cut(err.Error(), "\nfix: ")
	require.True(t, found)
	require.Equal(t, path+` mode "a|b" payload <payload>`, fix)
	completed := strings.Replace(fix, "<payload>", "complete", 1)
	line, _, message := command.ProcessPipesDefaultFormatChecked(completed, "text")
	require.Empty(t, message)
	require.Equal(t, completed, line)
	_, err = d.Dispatch(&CommandContext{}, line)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

// TestOperationalInvalidEscapeCannotReachHandler retains fail-closed lexical
// refusal through both the checked pipeline boundary and the dispatcher.
func TestOperationalInvalidEscapeCannotReachHandler(t *testing.T) {
	const path = "show demo escape"
	d := NewDispatcher()
	calls := 0
	require.NoError(t, d.RegisterWithOptions(path, func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		calls++
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Refuse malformed escapes", RegisterOptions{ReadOnly: true, ArgDefs: []command.ArgDef{
		commandtest.Must(command.NewStringArg("mode", nil, nil, command.ArgOptions{Mandatory: true})),
	}}))
	for _, suffix := range []string{` mode a\b | count`, ` mode "a\q" | count`, ` mode "a\`} {
		t.Run(suffix, func(t *testing.T) {
			line, _, message := command.ProcessPipesDefaultFormatChecked(path+suffix, "text")
			require.Empty(t, message)
			_, err := d.Dispatch(&CommandContext{}, line)
			require.True(t, errors.Is(err, errBackslashInCommand), "lexical refusal = %v", err)
			require.Zero(t, calls)
		})
	}
}
