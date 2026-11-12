package server

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// resetPipeRegistries clears the pipe-filter and alias registries before a
// test and again after it. The server package links no package that registers
// either from init(), so an empty registry is the state every other test of
// the package runs in. A test that calls it MUST NOT be parallel.
func resetPipeRegistries(t *testing.T) {
	t.Helper()
	reset := func() {
		command.ResetAliasesForTest()
		command.ResetPipeFiltersForTest()
	}
	reset()
	t.Cleanup(reset)
}

// requirePipeRefusal dispatches line and returns the operator error it was
// refused with, after checking the shape every refusal has: Status error, the
// rendered operator error as Response.Error, and no handler call.
func requirePipeRefusal(t *testing.T, d *Dispatcher, calls *atomic.Int32, line string) *opererr.Error {
	t.Helper()
	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, line)
	require.Error(t, err, "a pipe chain outside quotes must be refused: %q", line)
	refusal, ok := errors.AsType[*opererr.Error](err)
	require.True(t, ok, "the refusal is an operator error, got %T: %v", err, err)
	require.NotNil(t, resp, "the refusal carries an error response")
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, refusal.Error(), resp.Error, "Response.Error is the rendered operator error")
	if calls != nil {
		assert.Equal(t, int32(0), calls.Load(), "no handler may run for a refused chain")
	}
	return refusal
}

// TestDispatchRefusesUnquotedPipeChain proves Dispatch refuses every line that
// carries a pipe outside quotes before the matched builtin's handler runs, with
// a problem naming the command part and the chain from the first pipe, a why
// naming data and rendering, and, with no filter registered to fold into, the
// fix that names the command part and the `ze cli -c` form of the line.
//
// Method: one counting builtin ("test command") and a chain of each shape:
// a folding operator, a format, several operators, and a pipe joined to a word
// on one or both sides.
//
// VALIDATES: AC-1, AC-1b, AC-8 of spec-plugin-dispatch-pipe-chain-reaches-the-handler.
// PREVENTS: a piped line answered by the handler as if the chain were absent.
func TestDispatchRefusesUnquotedPipeChain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		line  string
		chain string
	}{
		{name: "count", line: "test command | count", chain: "| count"},
		{name: "format", line: "test command | json", chain: "| json"},
		{name: "several operators", line: "test command | match x | count", chain: "| match x | count"},
		{name: "joined both sides", line: "test command|count", chain: "| count"},
		{name: "joined before operator", line: "test command |count", chain: "| count"},
		{name: "trailing pipe", line: "test command |", chain: "| "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d, calls := pipeChainTarget(t)
			refusal := requirePipeRefusal(t, d, calls, tt.line)

			assert.Contains(t, refusal.Problem(), `"test command"`, "the problem names the command part")
			assert.Contains(t, refusal.Problem(), tt.chain, "the problem names the chain from the first pipe")
			assert.Contains(t, refusal.Why(), "data")
			assert.Contains(t, refusal.Why(), "render")
			if tt.name == "trailing pipe" {
				// An empty chain folds to the command alone, which is the
				// line to send.
				assert.Equal(t, "test command", refusal.Fix())
				return
			}
			assert.Contains(t, refusal.Fix(), `dispatch "test command"`, "the fix names the command part to dispatch")
			assert.Contains(t, refusal.Fix(), "ze cli -c '"+tt.line+"'", "the fix names the operator form of the line")
			lines := strings.Split(refusal.Error(), "\n")
			assert.Equal(t, "fix: "+refusal.Fix(), lines[len(lines)-1], "the fix is alone on the last line")
		})
	}

	t.Run("no command before the pipe", func(t *testing.T) {
		t.Parallel()
		d, calls := pipeChainTarget(t)
		refusal := requirePipeRefusal(t, d, calls, "| count")
		assert.Contains(t, refusal.Problem(), "names no command")
		assert.Equal(t, "ze cli -c '<command> | count'", refusal.Fix())
	})
}

// TestDispatchPipeRefusalNamesFoldedForm proves the refusal's fix is the folded
// dispatch line, exactly, when every operator of the chain is a filter the
// command owns, and the generic fix when one operator stays in the chain.
//
// Method: the pipe-filter registry is cleared and "test command" registers
// count and a named peer filter. A quoted pipe before the chain checks that
// the fold reads the command part the quote-aware scan separated (R-4).
//
// VALIDATES: AC-1 fix line, R-4 of spec-plugin-dispatch-pipe-chain-reaches-the-handler.
// PREVENTS: a suggested line that drops or misplaces part of the chain.
func TestDispatchPipeRefusalNamesFoldedForm(t *testing.T) {
	// Not parallel: it writes the process-wide pipe-filter registry.
	resetPipeRegistries(t)
	command.RegisterPipeFilters([]string{"test command"},
		command.PipeFilter{Name: "count", Description: "count rows"},
		command.PipeFilter{Name: "peer", Description: "rows of one peer", TakesArg: true},
	)

	tests := []struct {
		name    string
		line    string
		wantFix string
	}{
		{name: "operator filter", line: "test command | count", wantFix: "test command count"},
		{name: "named filter then operator", line: "test command | peer 10.0.0.1 | count", wantFix: "test command peer 10.0.0.1 count"},
		{name: "joined pipe", line: "test command|count", wantFix: "test command count"},
		{name: "quoted pipe before the chain", line: `test command "a|b" | count`, wantFix: `test command "a|b" count`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, calls := pipeChainTarget(t)
			refusal := requirePipeRefusal(t, d, calls, tt.line)
			assert.Equal(t, tt.wantFix, refusal.Fix(), "the fix is the folded line alone")
		})
	}

	t.Run("a format stays in the chain", func(t *testing.T) {
		d, calls := pipeChainTarget(t)
		refusal := requirePipeRefusal(t, d, calls, "test command | count | json")
		assert.Contains(t, refusal.Fix(), "ze cli -c 'test command | count | json'", "a partial fold is never offered")
	})

	t.Run("the folded line dispatches", func(t *testing.T) {
		// Positive control: the line the fix names is accepted and answered.
		d, calls := pipeChainTarget(t)
		resp, err := d.Dispatch(&CommandContext{Username: "alice"}, "test command count")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		assert.Equal(t, int32(1), calls.Load())
	})
}

// TestDispatchKeepsQuotedPipeArgument proves a quoted pipe character, alone or
// inside a quoted value, still reaches the handler as an argument value.
//
// Method: a builtin records the tokens it was handed; each line quotes a pipe.
//
// VALIDATES: AC-2, AC-4 of spec-plugin-dispatch-pipe-chain-reaches-the-handler.
// PREVENTS: a refusal keyed on the unquoted token refusing data (R-1).
func TestDispatchKeepsQuotedPipeArgument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want []string
	}{
		{name: "quoted pipe alone", line: `test echo "|"`, want: []string{"|"}},
		{name: "quoted value holding a pipe", line: `test echo "a | b"`, want: []string{"a | b"}},
		{name: "quoted joined pipe", line: `test echo "a|b" c`, want: []string{"a|b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := NewDispatcher()
			var handled []string
			require.NoError(t, d.Register("test echo", func(_ *CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
				handled = slices.Clone(validated.Tokens())
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "test echo"))

			resp, err := d.Dispatch(&CommandContext{Username: "alice"}, tt.line)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusDone, resp.Status)
			assert.Equal(t, tt.want, handled, "the quoted pipe is the handler's argument")
		})
	}
}

// TestDispatchPipeRefusalReachesNoPluginProcess proves a piped line naming a
// plugin's command is refused before an execute-command request leaves for the
// plugin process, for a direct caller of Dispatch (the API surfaces'
// serverDispatcher shape) and for another plugin's dispatch-command.
//
// Method: a target plugin serves one execute-command request and counts what it
// receives. Both piped lines must leave the count at zero; the plain line, the
// positive control, then reaches the target once.
//
// VALIDATES: AC-1, AC-3 of spec-plugin-dispatch-pipe-chain-reaches-the-handler.
// PREVENTS: a chain handed to a plugin process as arguments.
func TestDispatchPipeRefusalReachesNoPluginProcess(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	var received atomic.Int32
	done := registerExecuteCommandTarget(t, d, "request target echo", 1, func(_ int, _ *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
		received.Add(1)
		return &rpc.ExecuteCommandOutput{Status: plugin.StatusDone}, nil
	})

	refusal := requirePipeRefusal(t, d, nil, "request target echo | count")
	assert.Contains(t, refusal.Problem(), `"request target echo"`)

	s := &Server{dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	caller := process.NewProcess(plugin.PluginConfig{Name: "caller-plugin"})

	_, err := s.dispatchCommand(t.Context(), caller, "request target echo | json")
	_, isOperatorError := errors.AsType[*opererr.Error](err)
	assert.True(t, isOperatorError, "a plugin's piped dispatch-command is refused with the operator error, got %v", err)
	assert.Equal(t, int32(0), received.Load(), "no execute-command may reach the plugin for a refused chain")

	out, err := s.dispatchCommand(t.Context(), caller, "request target echo")
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NoError(t, <-done)
	assert.Equal(t, int32(1), received.Load(), "the plain line reaches the plugin")
}

// TestDispatchPipeRefusalIsAccounted proves a refused piped line is still
// accounted, START and STOP, with the tokens tokenize produces for it.
//
// Method: the typed accounting capture records the tokens of each call.
//
// VALIDATES: AC-5 of spec-plugin-dispatch-pipe-chain-reaches-the-handler (RFC 8907 Section 8.3).
// PREVENTS: the refusal skipping the accounting record, or changing its tokens (R-2).
func TestDispatchPipeRefusalIsAccounted(t *testing.T) {
	t.Parallel()

	const line = "test command | count"
	d, calls := pipeChainTarget(t)
	accountant := &typedAccountingCapture{}
	d.SetAccountingHook(accountant)

	// The refusal's text is judged elsewhere; this test reads only the record.
	_ = requirePipeRefusal(t, d, calls, line)

	tokens, err := tokenize(line)
	require.NoError(t, err)
	want := [][]string{tokens}
	assert.Equal(t, [][]string{{"test", "command", "|", "count"}}, want, "tokenize keeps the pipe as a token")
	if !reflect.DeepEqual(accountant.startTokens, want) || !reflect.DeepEqual(accountant.stopTokens, want) {
		t.Fatalf("refused line accounting = START %q / STOP %q, want %q", accountant.startTokens, accountant.stopTokens, want)
	}
}

// TestDispatchPipeRefusalRedactsSecrets proves a secret value on a refused line
// is masked in every part of the refusal: the echoed command part, the
// `ze cli -c` form of the line, and the folded line.
//
// Method: the line carries `password <value>`, which config.DisplayCommand
// masks; the folded case registers a count filter for the command.
//
// VALIDATES: AC-7 of spec-plugin-dispatch-pipe-chain-reaches-the-handler.
// PREVENTS: a refusal publishing the credential the caller sent (R-5).
func TestDispatchPipeRefusalRedactsSecrets(t *testing.T) {
	// Not parallel: the folded case writes the process-wide pipe-filter registry.
	resetPipeRegistries(t)

	const secret = "hunter2"
	const line = "test command password " + secret + " | count"

	d, calls := pipeChainTarget(t)
	refusal := requirePipeRefusal(t, d, calls, line)
	assert.NotContains(t, refusal.Error(), secret, "the generic refusal masks the secret")
	assert.Contains(t, refusal.Problem(), redact.Placeholder)
	assert.Contains(t, refusal.Fix(), "ze cli -c 'test command password "+redact.Placeholder+" | count'")

	command.RegisterPipeFilters([]string{"test command"}, command.PipeFilter{Name: "count", Description: "count rows"})
	refusal = requirePipeRefusal(t, d, calls, line)
	assert.NotContains(t, refusal.Error(), secret, "the folded refusal masks the secret")
	assert.Equal(t, "test command password "+redact.Placeholder+" count", refusal.Fix())
}
