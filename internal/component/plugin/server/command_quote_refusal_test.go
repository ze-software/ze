// Design: docs/architecture/api/commands.md -- operational invocation quoting
// Related: plugin_rpc_test.go -- partial scanner input remains usable by completion

package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestDispatchRefusesUnfinishedQuote exercises the public execution boundary,
// with a registered handler that would accept every scanned token. A complete
// invocation must be refused before that handler runs, and its diagnostic must
// locate the unmatched opening quote in the original line.
// VALIDATES: spec-web-cli-config-one-off-defects AC-7.
// PREVENTS: completion's partial-input allowance becoming executable input.
func TestDispatchRefusesUnfinishedQuote(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		column string
	}{
		{"value", `show quote "unfinished`, "column 12"},
		{"leading spaces", `  show quote "unfinished`, "column 14"},
		{"escaped quote stays open", `show quote "unfinished \" tail`, "column 12"},
		{"later unmatched quote", `show quote "closed" "unfinished`, "column 21"},
		{"empty unfinished value", `show quote "`, "column 12"},
		{"pipe is inside unfinished value", `show quote "unfinished | count`, "column 12"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher, calls, _ := quoteRefusalTarget(t)
			response, err := dispatcher.Dispatch(&CommandContext{}, tc.input)
			assert.Zero(t, *calls, "an unfinished quote must never reach a command handler")
			require.Error(t, err)
			refusal, ok := errors.AsType[*opererr.Error](err)
			require.True(t, ok, "the execution refusal must explain the operator's repair: %v", err)
			assert.Contains(t, refusal.Problem(), tc.column)
			assert.Contains(t, refusal.Problem(), "quote")
			assert.NotEmpty(t, refusal.Fix())
			require.NotNil(t, response)
			assert.Equal(t, plugin.StatusError, response.Status)
			assert.Equal(t, refusal.Error(), response.Error)
		})
	}
}

// TestDispatchQuotedValuesKeepTheirMeaning proves the permitted survivors at
// the same public boundary: an escaped quote, quoted pipe and explicit empty
// value each reach the handler as exactly one decoded argument.
func TestDispatchQuotedValuesKeepTheirMeaning(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		value string
	}{
		{"escaped quote", `show quote "say \"hello\""`, `say "hello"`},
		{"quoted pipe", `show quote "left | right"`, "left | right"},
		{"empty value", `show quote ""`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher, calls, received := quoteRefusalTarget(t)
			response, err := dispatcher.Dispatch(&CommandContext{}, tc.input)
			require.NoError(t, err)
			require.NotNil(t, response)
			assert.Equal(t, plugin.StatusDone, response.Status)
			assert.Equal(t, 1, *calls)
			assert.Equal(t, []string{tc.value}, *received)
		})
	}
}

func quoteRefusalTarget(t *testing.T) (*Dispatcher, *int, *[]string) {
	t.Helper()
	dispatcher := NewDispatcher()
	calls := new(int)
	received := new([]string)
	require.NoError(t, dispatcher.Register("show quote", func(_ *CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		(*calls)++
		*received = args.Tokens()
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"answered": true}}, nil
	}, "Inspect a quoted value"))
	return dispatcher, calls, received
}
