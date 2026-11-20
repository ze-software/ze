// Design: docs/architecture/api/commands.md — the pipe chain: declared operations and the residual chain
package server

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// resetPipeOperationRegistries empties the pipe-filter and alias registries for
// one test and puts back what they held when it ends. The test binary links
// packages that declare both from init(): the external test files of this
// package import the composition root (plugin/all), and a later test reads
// those declarations (peer_rib_scope_test.go reads the RIB's). Clearing them
// for good would leave that test reading none. A test that calls it MUST NOT
// be parallel.
func resetPipeOperationRegistries(t *testing.T) {
	t.Helper()
	restoreAliases := command.EmptyAliasesForTest()
	restorePipeFilters := command.EmptyPipeFiltersForTest()
	t.Cleanup(func() {
		restorePipeFilters()
		restoreAliases()
	})
}

// rowsDocument is the answer of the rows target: three peers, one of them up.
var rowsDocument = plugin.Map{"peers": []any{
	map[string]any{"name": "a", "state": "up"},
	map[string]any{"name": "b", "state": "down"},
	map[string]any{"name": "c", "state": "idle"},
}}

// rowsTarget builds a dispatcher with one builtin, "test rows", answering
// rowsDocument, and counts the handler's calls.
func rowsTarget(t *testing.T) (*Dispatcher, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	d := NewDispatcher()
	require.NoError(t, d.Register("test rows", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		calls.Add(1)
		return &plugin.Response{Status: plugin.StatusDone, Data: rowsDocument}, nil
	}, "test rows"))
	return d, calls
}

// surfaceAnswer is what an operator surface prints for line over document:
// the formatter the SSH exec channel and the web build
// (command.ProcessPipesDefaultFormatChecked), the bytes `ze cli -c` prints.
func surfaceAnswer(t *testing.T, line string, document any) string {
	t.Helper()
	_, format, problem := command.ProcessPipesDefaultFormatChecked(line, "")
	require.Empty(t, problem, "the surface accepts %q", line)
	encoded, err := json.Marshal(document)
	require.NoError(t, err)
	return format(string(encoded))
}

// dataOf answers the JSON a data caller receives for resp.
func dataOf(t *testing.T, resp *plugin.Response) string {
	t.Helper()
	output, err := plugin.ResponseJSON(resp, nil)
	require.NoError(t, err)
	return output
}

// TestDispatchHandsDeclaredOperationsToHandler proves a builtin's declared
// operations reach its handler as the structured list, in the order typed,
// through CommandContext and through ValidatedArgs, and never as arguments.
//
// Method: "test rib" declares received and count; the handler records what it
// was handed for `test rib | received | count`.
//
// VALIDATES: AC-1 dispatcher half, TestDispatchHandsDeclaredOperationsToHandler of
// spec-pipe-chain-honored-on-every-command-surface.
// PREVENTS: a declared operation folded into the handler's arguments.
func TestDispatchHandsDeclaredOperationsToHandler(t *testing.T) {
	resetPipeOperationRegistries(t)
	command.RegisterPipeFilters([]string{"test rib"},
		command.PipeFilter{Name: "received", Description: "received routes", Argument: command.ArgNone},
		command.PipeFilter{Name: "count", Description: "count the routes", Argument: command.ArgNone},
	)
	d := NewDispatcher()
	var fromContext, fromArgs []command.PipeOperation
	var args []string
	require.NoError(t, d.Register("test rib", func(ctx *CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
		fromContext = ctx.PipeOperations()
		fromArgs = validated.PipeOperations()
		args = validated.Tokens()
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"count": 2}}, nil
	}, "test rib"))

	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, "test rib | received | count")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	want := []command.PipeOperation{{Name: "received"}, {Name: "count"}}
	assert.Equal(t, want, fromContext, "CommandContext carries the declared list in chain order")
	assert.Equal(t, want, fromArgs, "ValidatedArgs carries the same list")
	assert.Empty(t, args, "no chain word reaches the handler's arguments")

	var answer map[string]any
	require.NoError(t, json.Unmarshal([]byte(dataOf(t, resp)), &answer))
	assert.Equal(t, float64(2), answer["count"], "the handler's count is the data caller's answer")
}

// TestDispatchAppliesResidualChainForDataCaller proves Dispatch applies a
// chain the command does not declare for a caller that does not render it,
// and that the data equals what an operator surface prints for the same line
// with `| json` (AC-4), and that a format operation answers the rendered text
// as a JSON string, byte for byte the surface's output (AC-5, T-1).
//
// VALIDATES: AC-4, AC-5 dispatcher half.
// PREVENTS: a piped line answered as if its chain were absent.
func TestDispatchAppliesResidualChainForDataCaller(t *testing.T) {
	t.Parallel()

	t.Run("generic chain", func(t *testing.T) {
		t.Parallel()
		d, calls := rowsTarget(t)
		resp, err := d.Dispatch(&CommandContext{Username: "alice"}, "test rows | match up | count")
		require.NoError(t, err)
		assert.Equal(t, int32(1), calls.Load())
		var answer map[string]any
		require.NoError(t, json.Unmarshal([]byte(dataOf(t, resp)), &answer))
		assert.Equal(t, float64(1), answer["count"], "one row of three holds up")
		assert.JSONEq(t, surfaceAnswer(t, "test rows | match up | count | json", rowsDocument), dataOf(t, resp),
			"the data caller's answer is the surface's `| json` answer")
	})

	for _, format := range []string{"text", "table", "yaml"} {
		t.Run("format "+format, func(t *testing.T) {
			t.Parallel()
			d, _ := rowsTarget(t)
			line := "test rows | " + format
			resp, err := d.Dispatch(&CommandContext{Username: "alice"}, line)
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusDone, resp.Status)
			var text string
			require.NoError(t, json.Unmarshal([]byte(dataOf(t, resp)), &text), "a format answers a JSON string")
			assert.Equal(t, surfaceAnswer(t, line, rowsDocument), text, "the text the surface prints, byte for byte")
		})
	}

	t.Run("no chain answers as today", func(t *testing.T) {
		t.Parallel()
		d, _ := rowsTarget(t)
		resp, err := d.Dispatch(&CommandContext{Username: "alice"}, "test rows")
		require.NoError(t, err)
		assert.Equal(t, rowsDocument, resp.Data, "the handler's own data, untouched (AC-10)")
	})
}

// TestDispatchLeavesResidualToRenderingCaller proves a caller stating it
// renders the chain receives the handler's data with the residual chain left
// to it.
//
// VALIDATES: TestDispatchLeavesResidualToRenderingCaller of the spec (T-2 A).
// PREVENTS: a surface's chain applied twice.
func TestDispatchLeavesResidualToRenderingCaller(t *testing.T) {
	t.Parallel()
	d, calls := rowsTarget(t)
	resp, err := d.Dispatch(&CommandContext{Username: "alice", RendersResidualChain: true}, "test rows | match up | count")
	require.NoError(t, err)
	assert.Equal(t, int32(1), calls.Load())
	assert.Equal(t, rowsDocument, resp.Data, "the rendering caller applies the chain itself")
}

// TestDispatchAppliesResidualChainToStreamedRows proves the residual chain
// applies to an answer the handler streams as a row generator.
//
// VALIDATES: A-5.
// PREVENTS: a streamed answer reaching a data caller with its chain dropped.
func TestDispatchAppliesResidualChainToStreamedRows(t *testing.T) {
	t.Parallel()
	d := NewDispatcher()
	require.NoError(t, d.Register("test stream", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		var rows iter.Seq[rpc.RowRecord] = func(yield func(rpc.RowRecord) bool) {
			for i := range 3 {
				if !yield(rpc.RowRecord{Item: rpc.RawRow(`{"name":"r` + strconv.Itoa(i) + `"}`)}) {
					return
				}
			}
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Records{Key: "routes", Rows: rows}}, nil
	}, "test stream"))

	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, "test stream | count")
	require.NoError(t, err)
	var answer map[string]any
	require.NoError(t, json.Unmarshal([]byte(dataOf(t, resp)), &answer))
	assert.Equal(t, float64(3), answer["count"], "the three streamed rows are counted")
}

// TestGenericRowOperatorsOnUndeclaredCommand proves each generic row operation
// answers over the rows of a command that declares none, as the surface does.
//
// VALIDATES: AC-16 dispatcher half.
func TestGenericRowOperatorsOnUndeclaredCommand(t *testing.T) {
	t.Parallel()
	for _, chain := range []string{"count", "first 2", "last 1", "match down"} {
		t.Run(chain, func(t *testing.T) {
			t.Parallel()
			d, _ := rowsTarget(t)
			line := "test rows | " + chain
			resp, err := d.Dispatch(&CommandContext{Username: "alice"}, line)
			require.NoError(t, err)
			assert.JSONEq(t, surfaceAnswer(t, line+" | json", rowsDocument), dataOf(t, resp))
		})
	}
}

// TestDispatchRefusesResidualChainItCannotApply proves a chain the engine
// cannot apply for a data caller is refused before the handler runs: `| save`
// (the daemon never writes a caller's file), an unknown operation on a command
// that declares a set (AC-13), and a declared operation typed after a residual
// one (AC-25).
func TestDispatchRefusesResidualChainItCannotApply(t *testing.T) {
	resetPipeOperationRegistries(t)
	command.RegisterPipeFilters([]string{"test rows"},
		command.PipeFilter{Name: "received", Description: "received rows", Argument: command.ArgNone},
	)
	for _, line := range []string{"test rows | save out.txt", "test rows | bogus", "test rows | table | received"} {
		t.Run(line, func(t *testing.T) {
			d, calls := rowsTarget(t)
			resp, err := d.Dispatch(&CommandContext{Username: "alice"}, line)
			require.Error(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Equal(t, int32(0), calls.Load(), "no handler runs for a refused chain")
		})
	}
}

// TestDispatchRefusesPipeOperationWordAsArgument proves the dispatcher refuses
// a declared operation typed as an argument, naming the pipe form.
//
// VALIDATES: AC-8 dispatcher half.
func TestDispatchRefusesPipeOperationWordAsArgument(t *testing.T) {
	resetPipeOperationRegistries(t)
	command.RegisterPipeFilters([]string{"test rows"},
		command.PipeFilter{Name: "received", Description: "received rows", Argument: command.ArgNone},
	)
	d, calls := rowsTarget(t)
	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, "test rows received")
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, err.Error(), "test rows | received", "the fix is the pipe form")
	assert.Equal(t, int32(0), calls.Load())
}

// TestDispatchRefusesOverrideAnswerOfWrongShape proves a builtin's override of
// count is held to count's answer shape on the dispatcher's route.
//
// VALIDATES: AC-17 on Dispatch.
func TestDispatchRefusesOverrideAnswerOfWrongShape(t *testing.T) {
	resetPipeOperationRegistries(t)
	command.RegisterPipeFilters([]string{"test rows"},
		command.PipeFilter{Name: "count", Description: "count rows", Argument: command.ArgNone},
	)
	d, _ := rowsTarget(t)
	_, err := d.Dispatch(&CommandContext{Username: "alice"}, "test rows | count")
	_, isOperatorError := errors.AsType[*opererr.Error](err)
	assert.True(t, isOperatorError, "rows answered for a count override are refused by name, got %v", err)
}

// TestDispatchCarriesPipeOperationsToThePluginRequest proves a plugin command
// whose line declares an operation reaches the plugin process with that
// operation in the request's pipe list, and the plain line with none.
//
// Method: an engine-declared operation that takes no argument on a command a
// plugin serves over the process connection; the plugin side reads both
// requests off the wire, so the list is the one JSON carried.
//
// VALIDATES: AC-14 carrier half: the execute-command request carries the
// declared operations; a line without one carries no list.
// PREVENTS: a declared operation dropped on the way to the plugin, or a list
// sent for a line that holds none.
func TestDispatchCarriesPipeOperationsToThePluginRequest(t *testing.T) {
	resetPipeOperationRegistries(t)
	d := NewDispatcher()
	var requested [2]rpc.ExecuteCommandInput
	done := registerExecuteCommandTarget(t, d, "request target echo", 2, func(call int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
		requested[call] = *input
		return &rpc.ExecuteCommandOutput{Status: plugin.StatusDone}, nil
	})
	command.RegisterPipeFilters([]string{"request target echo"},
		command.PipeFilter{Name: "received", Description: "received rows", Argument: command.ArgNone},
	)

	_, err := d.Dispatch(&CommandContext{Username: "alice"}, "request target echo | received")
	require.NoError(t, err)
	_, err = d.Dispatch(&CommandContext{Username: "alice"}, "request target echo")
	require.NoError(t, err)
	require.NoError(t, <-done)

	assert.Equal(t, "request target echo", requested[0].Command)
	assert.Empty(t, requested[0].Args, "no operation word reaches the arguments")
	assert.Equal(t, []rpc.PipeOperation{{Name: "received"}}, requested[0].Pipe)
	assert.Equal(t, "request target echo", requested[1].Command)
	assert.Nil(t, requested[1].Pipe, "the plain line carries no pipe list")
}

// TestDispatchCommandAppliesPipeChain proves a plugin's dispatch-command of a
// piped line, through the DirectBridge slot and through the answer slot,
// receives the chain applied.
//
// VALIDATES: AC-2, AC-4 on the dispatch-command route (dispatcher half).
// PREVENTS: a plugin's piped line refused, or answered unfiltered.
func TestDispatchCommandAppliesPipeChain(t *testing.T) {
	t.Parallel()
	const line = "test rows | match up | count"

	t.Run("dispatch-command", func(t *testing.T) {
		t.Parallel()
		d, calls := rowsTarget(t)
		s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
		s.ctx, s.cancel = context.WithCancel(context.Background())
		defer s.cancel()
		proc := process.NewProcess(plugin.PluginConfig{Name: "pipe-chain-bridge"})
		out, err := pipeChainBridge(s, proc).DispatchCommand(t.Context(), line)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, plugin.StatusDone, out.Status)
		assert.Equal(t, int32(1), calls.Load())
		var answer map[string]any
		require.NoError(t, json.Unmarshal(out.Data, &answer))
		assert.Equal(t, float64(1), answer["count"])
	})

	t.Run("socket", func(t *testing.T) {
		t.Parallel()
		d, calls := rowsTarget(t)
		pluginSide, engineSide := net.Pipe()
		t.Cleanup(func() { _ = pluginSide.Close() })
		t.Cleanup(func() { _ = engineSide.Close() })
		proc := process.NewProcess(plugin.PluginConfig{Name: "pipe-chain-socket"})
		proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
		s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
		s.ctx, s.cancel = context.WithCancel(context.Background())
		defer s.cancel()
		handlerDone := make(chan struct{})
		go func() {
			defer close(handlerDone)
			s.handleSingleProcessCommandsRPC(proc)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		status, data, err := dispatchAnswerOverSocket(ctx, pluginSide, &rpc.DispatchCommandInput{Command: line})
		require.NoError(t, err)
		assert.Equal(t, rpc.StatusDone, status)
		assert.Equal(t, int32(1), calls.Load(), "one handler call")
		var answer map[string]any
		require.NoError(t, json.Unmarshal(data, &answer))
		assert.Equal(t, float64(1), answer["count"])

		require.NoError(t, pluginSide.Close())
		select {
		case <-handlerDone:
		case <-time.After(2 * time.Second):
			t.Fatal("handler did not exit")
		}
	})

	t.Run("dispatch-command answer", func(t *testing.T) {
		t.Parallel()
		d, _ := rowsTarget(t)
		s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
		s.ctx, s.cancel = context.WithCancel(context.Background())
		defer s.cancel()
		proc := process.NewProcess(plugin.PluginConfig{Name: "pipe-chain-answer"})
		answer, err := pipeChainBridge(s, proc).DispatchCommandAnswer(t.Context(), line)
		require.NoError(t, err)
		require.NotNil(t, answer)
		var rows []string
		for record := range answer.Records {
			rows = append(rows, string(record.Item))
		}
		require.Len(t, rows, 1, "the count is one document")
		assert.JSONEq(t, surfaceAnswer(t, line+" | json", rowsDocument), rows[0])
	})
}

// TestDispatchKeepsQuotedPipeArgument proves a quoted pipe character, alone or
// inside a quoted value, still reaches the handler as an argument value and
// opens no chain. Moved unchanged from command_pipe_refusal_test.go, whose
// other tests asserted the refusal this design removes.
//
// VALIDATES: AC-2, AC-4 of spec-plugin-dispatch-pipe-chain-reaches-the-handler;
// AC-6 command-part half of spec-pipe-chain-honored-on-every-command-surface.
// PREVENTS: a quoted pipe read as an operator (R-1).
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

// stringAuthorizerCapture is a string-only authorizer recording the line.
type stringAuthorizerCapture struct {
	allow bool
	lines []string
}

func (a *stringAuthorizerCapture) Authorize(_, _, line string, _ bool) bool {
	a.lines = append(a.lines, line)
	return a.allow
}

// TestTypedAuthorizerReceivesChainTokens proves, under T-6 A, a typed
// authorizer receives the command, its arguments, then the chain as tokens,
// with read-only judged on the command path, while the handler's arguments
// hold no chain token; and a string authorizer receives the typed line.
//
// VALIDATES: AC-27, AC-28 dispatcher half.
func TestTypedAuthorizerReceivesChainTokens(t *testing.T) {
	t.Parallel()

	t.Run("typed", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher()
		var handled []string
		require.NoError(t, d.RegisterWithOptions("test echo", func(_ *CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
			handled = slices.Clone(validated.Tokens())
			return &plugin.Response{Status: plugin.StatusDone, Data: rowsDocument}, nil
		}, "test echo", RegisterOptions{ReadOnly: true}))
		authorizer := &captureCommandArgsAuthorizer{allow: true}
		d.SetAuthorizer(authorizer)

		_, err := d.Dispatch(&CommandContext{Username: "alice"}, `test echo alpha | match "a b" | count`)
		require.NoError(t, err)
		assert.True(t, authorizer.structuredCalled)
		assert.Equal(t, "test", authorizer.command)
		assert.Equal(t, []string{"echo", "alpha", "|", "match", "a b", "|", "count"}, authorizer.args)
		assert.True(t, authorizer.readOnly, "read-only is the command path's flag")
		assert.Equal(t, []string{"alpha"}, handled, "the handler's arguments hold no chain token")
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		d, _ := rowsTarget(t)
		authorizer := &stringAuthorizerCapture{allow: true}
		d.SetAuthorizer(authorizer)
		const line = `test rows | match "a b" | count`
		_, err := d.Dispatch(&CommandContext{Username: "alice"}, line)
		require.NoError(t, err)
		assert.Equal(t, []string{line}, authorizer.lines, "a string authorizer receives the typed line")
	})
}

// TestPipedLineUnauthorizedRefused proves a piped line the policy denies runs
// no handler and answers the permission-denied refusal.
//
// VALIDATES: AC-27 dispatcher half.
func TestPipedLineUnauthorizedRefused(t *testing.T) {
	t.Parallel()
	d, calls := rowsTarget(t)
	d.SetAuthorizer(&captureCommandArgsAuthorizer{allow: false})
	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, "test rows | count")
	require.ErrorIs(t, err, ErrUnauthorized)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, int32(0), calls.Load())
}

// TestAuthorizerTokensKeepChainBackslash proves a backslash outside quotes in
// the chain reaches a typed authorizer intact and an allowed line runs, while
// one in the command part stays refused.
//
// VALIDATES: AC-38, R-13.
// PREVENTS: `| match a\b` denied because a second scan refused the backslash.
func TestAuthorizerTokensKeepChainBackslash(t *testing.T) {
	t.Parallel()
	d, calls := rowsTarget(t)
	authorizer := &captureCommandArgsAuthorizer{allow: true}
	d.SetAuthorizer(authorizer)

	_, err := d.Dispatch(&CommandContext{Username: "alice"}, `test rows | match a\b`)
	require.NoError(t, err)
	assert.Equal(t, []string{"rows", "|", "match", `a\b`}, authorizer.args)
	assert.Equal(t, int32(1), calls.Load(), "the allowed line runs")

	_, err = d.Dispatch(&CommandContext{Username: "alice"}, `test ro\ws | count`)
	require.ErrorIs(t, err, errBackslashInCommand, "a backslash in the command part stays refused")
	assert.Equal(t, int32(1), calls.Load())
}

// TestAccountantTokensKeepChainBackslash proves the typed accountant receives
// the typed line's tokens, the chain's backslash intact, not the line as one
// token.
//
// VALIDATES: AC-38.
func TestAccountantTokensKeepChainBackslash(t *testing.T) {
	t.Parallel()
	d, _ := rowsTarget(t)
	accountant := &typedAccountingCapture{}
	d.SetAccountingHook(accountant)
	_, err := d.Dispatch(&CommandContext{Username: "alice"}, `test rows | match a\b`)
	require.NoError(t, err)
	want := [][]string{{"test", "rows", "|", "match", `a\b`}}
	assert.Equal(t, want, accountant.startTokens)
	assert.Equal(t, want, accountant.stopTokens)
}

// TestDispatchAccountsTypedLineWithMaskedSecret proves the accounting record
// of a piped line holds the typed line, its secret masked, its chain intact
// after the masked part.
//
// VALIDATES: AC-29 (RFC 8907 Section 8.3: the command entered).
func TestDispatchAccountsTypedLineWithMaskedSecret(t *testing.T) {
	t.Parallel()
	const secret = "hunter2"
	d, _ := pipeChainTarget(t)
	accountant := &typedAccountingCapture{}
	d.SetAccountingHook(accountant)
	_, _ = d.Dispatch(&CommandContext{Username: "alice"}, "test command password "+secret+" | match up | count")
	want := [][]string{{"test", "command", "password", redact.Placeholder, "|", "match", "up", "|", "count"}}
	assert.Equal(t, want, accountant.startTokens, "the masked line, chain intact")
	assert.Equal(t, want, accountant.stopTokens)
}

// echoTarget builds a dispatcher with one builtin, "test echo", answering
// rowsDocument, and records the arguments its handler was handed.
func echoTarget(t *testing.T) (*Dispatcher, *atomic.Int32, *[]string) {
	t.Helper()
	calls := &atomic.Int32{}
	handled := &[]string{}
	d := NewDispatcher()
	require.NoError(t, d.Register("test echo", func(_ *CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
		calls.Add(1)
		*handled = slices.Clone(validated.Tokens())
		return &plugin.Response{Status: plugin.StatusDone, Data: rowsDocument}, nil
	}, "test echo"))
	return d, calls, handled
}

// chainCountOf answers the count a data caller receives for resp.
func chainCountOf(t *testing.T, resp *plugin.Response) float64 {
	t.Helper()
	var answer map[string]any
	require.NoError(t, json.Unmarshal([]byte(dataOf(t, resp)), &answer))
	count, ok := answer["count"].(float64)
	require.True(t, ok, "the answer is a count document, got %s", dataOf(t, resp))
	return count
}

// TestDispatchReadsJoinedPipeAsChain proves a pipe joined to the word before
// it, or to the operation after it, opens the chain as a spaced one does.
//
// Method: `test rows|count` and `test rows |count` over the three rows; the
// data caller receives the count. Replaces the joined rows of the removed
// TestDispatchRefusesUnquotedPipeChain.
//
// VALIDATES: AC-6 (one grammar for the chain), AC-4 dispatcher half.
// PREVENTS: a joined pipe read as part of a command word.
func TestDispatchReadsJoinedPipeAsChain(t *testing.T) {
	t.Parallel()
	for _, line := range []string{"test rows|count", "test rows |count"} {
		t.Run(line, func(t *testing.T) {
			t.Parallel()
			d, calls := rowsTarget(t)
			resp, err := d.Dispatch(&CommandContext{Username: "alice"}, line)
			require.NoError(t, err)
			assert.Equal(t, int32(1), calls.Load(), "the command part runs once")
			assert.Equal(t, float64(3), chainCountOf(t, resp), "the three rows are counted")
		})
	}
}

// TestDispatchRefusesChainWithNoCommand proves a line holding only a chain is
// refused as an empty command and runs no handler.
//
// Method: `| count` against a dispatcher with one builtin. Replaces the "no
// command before the pipe" row of the removed TestDispatchRefusesUnquotedPipeChain.
//
// PREVENTS: a chain with no command answered as if a command had run.
func TestDispatchRefusesChainWithNoCommand(t *testing.T) {
	t.Parallel()
	d, calls := rowsTarget(t)
	_, err := d.Dispatch(&CommandContext{Username: "alice"}, "| count")
	require.ErrorIs(t, err, ErrEmptyCommand)
	assert.Equal(t, int32(0), calls.Load(), "no handler runs")
}

// TestDispatchTrailingPipeAnswersTheCommand proves a trailing pipe opens an
// empty chain: the command runs with no argument and its answer is unchanged.
//
// Method: `test echo |`; the handler's arguments and the answer are read.
// Replaces the "trailing pipe" row of the removed
// TestDispatchRefusesUnquotedPipeChain.
//
// VALIDATES: AC-10 (a line with no operation answers the handler's data).
// PREVENTS: the trailing pipe reaching the handler as an argument.
func TestDispatchTrailingPipeAnswersTheCommand(t *testing.T) {
	t.Parallel()
	d, calls, handled := echoTarget(t)
	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, "test echo |")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(1), calls.Load())
	assert.Empty(t, *handled, "no pipe reaches the handler's arguments")
	assert.Equal(t, rowsDocument, resp.Data, "the handler's own data, untouched")
}

// TestDispatchQuotedPipeArgumentWithChain proves a quoted pipe before the chain
// stays the handler's argument while the unquoted pipe after it opens the
// chain, which runs over the answer.
//
// Method: `test echo "a|b" | count`. Replaces the quoted-pipe row of the
// removed TestDispatchPipeRefusalNamesFoldedForm.
//
// VALIDATES: AC-6 command-part half; R-1.
// PREVENTS: the chain opened at the quoted pipe.
func TestDispatchQuotedPipeArgumentWithChain(t *testing.T) {
	t.Parallel()
	d, _, handled := echoTarget(t)
	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, `test echo "a|b" | count`)
	require.NoError(t, err)
	assert.Equal(t, []string{"a|b"}, *handled, "the quoted pipe is the handler's argument")
	assert.Equal(t, float64(3), chainCountOf(t, resp), "the chain after the quoted value runs")
}

// TestDispatchPluginCommandReceivesCommandPartOnly proves a plugin command's
// request carries the command part alone, never a chain word, and that the
// residual chain runs over the plugin's answer.
//
// Method: a plugin target answers two peers for `request target echo | count`;
// the request it read and the count the caller receives are checked. Replaces
// the removed TestDispatchPipeRefusalReachesNoPluginProcess.
//
// VALIDATES: AC-3, AC-4 on the plugin route.
// PREVENTS: a chain handed to a plugin process as arguments.
func TestDispatchPluginCommandReceivesCommandPartOnly(t *testing.T) {
	t.Parallel()
	d := NewDispatcher()
	var requested rpc.ExecuteCommandInput
	done := registerExecuteCommandTarget(t, d, "request target echo", 1, func(_ int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
		requested = *input
		return &rpc.ExecuteCommandOutput{Status: plugin.StatusDone, Data: json.RawMessage(`{"peers":[{"name":"a"},{"name":"b"}]}`)}, nil
	})

	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, "request target echo | count")
	require.NoError(t, err)
	require.NoError(t, <-done)
	assert.Equal(t, "request target echo", requested.Command, "the request names the command part")
	assert.Empty(t, requested.Args, "no chain word reaches the plugin's arguments")
	assert.Equal(t, float64(2), chainCountOf(t, resp), "the chain counts the plugin's two rows")
}

// TestDispatchAccountsRefusedChain proves a line whose chain the engine
// refuses is still accounted, START and STOP, with the tokens typed.
//
// Method: `test rows | save out.txt`, refused because the daemon never writes
// a caller's file; the typed accounting capture records each call. Replaces
// the removed TestDispatchPipeRefusalIsAccounted.
//
// VALIDATES: AC-29 (RFC 8907 Section 8.3: every command entered is accounted).
// PREVENTS: a refusal skipping the accounting record.
func TestDispatchAccountsRefusedChain(t *testing.T) {
	t.Parallel()
	d, calls := rowsTarget(t)
	accountant := &typedAccountingCapture{}
	d.SetAccountingHook(accountant)
	_, err := d.Dispatch(&CommandContext{Username: "alice"}, "test rows | save out.txt")
	require.Error(t, err, "the daemon refuses | save")
	assert.Equal(t, int32(0), calls.Load(), "no handler runs")
	want := [][]string{{"test", "rows", "|", "save", "out.txt"}}
	assert.Equal(t, want, accountant.startTokens, "START carries the typed tokens")
	assert.Equal(t, want, accountant.stopTokens, "STOP carries the typed tokens")
}

// TestPipeRefusalsMaskSecrets proves a refusal that repeats the operator's line
// masks a secret on it: the pipe form of a declared operation typed as an
// argument (AC-8), and the reordered line of a declared operation typed after a
// residual one (AC-25).
//
// Method: "test rows" declares received; each line carries `password <value>`.
// Replaces the removed TestDispatchPipeRefusalRedactsSecrets.
//
// VALIDATES: AC-8, AC-25 fix lines; R-5.
// PREVENTS: a refusal publishing the credential the caller sent.
func TestPipeRefusalsMaskSecrets(t *testing.T) {
	// Not parallel: it writes the process-wide pipe-filter registry.
	resetPipeOperationRegistries(t)
	command.RegisterPipeFilters([]string{"test rows"},
		command.PipeFilter{Name: "received", Description: "received rows", Argument: command.ArgNone},
	)
	const secret = "hunter2"
	for _, line := range []string{
		"test rows password " + secret + " received",
		"test rows password " + secret + " | table | received",
	} {
		t.Run(line, func(t *testing.T) {
			d, calls := rowsTarget(t)
			resp, err := d.Dispatch(&CommandContext{Username: "alice"}, line)
			require.Error(t, err)
			assert.Equal(t, int32(0), calls.Load(), "no handler runs")
			assert.NotContains(t, err.Error(), secret, "the refusal masks the secret")
			assert.Contains(t, err.Error(), "password "+redact.Placeholder, "the line is repeated with its secret masked")
			if resp != nil {
				assert.NotContains(t, resp.Error, secret, "the error answer masks the secret")
			}
		})
	}
}

// TestDispatchCommandSocketRefusesPipeOperationWord proves a forked plugin's
// dispatch-command typing a declared operation as an argument is refused on
// the socket before the handler runs, with the pipe form on the refusal's last
// line, and raised under the doctor code for a refused plugin request.
//
// Method: "test rows" declares received; the real engine-op loop
// (handleSingleProcessCommandsRPC) answers `test rows received`, read the way
// a forked plugin reads it. Replaces the socket arm of the removed
// TestDispatchCommandRefusesPipeChain.
//
// VALIDATES: AC-8 on the dispatch-command socket route.
// PREVENTS: a plugin's argument-form line run, or refused with no doctor event.
func TestDispatchCommandSocketRefusesPipeOperationWord(t *testing.T) {
	// Not parallel: it reads the process-wide report bus and writes the
	// pipe-filter registry.
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	resetPipeOperationRegistries(t)
	command.RegisterPipeFilters([]string{"test rows"},
		command.PipeFilter{Name: "received", Description: "received rows", Argument: command.ArgNone},
	)

	d, calls := rowsTarget(t)
	pluginSide, engineSide := net.Pipe()
	t.Cleanup(func() { _ = pluginSide.Close() })
	t.Cleanup(func() { _ = engineSide.Close() })
	proc := process.NewProcess(plugin.PluginConfig{Name: "pipe-word-socket"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	status, data, err := dispatchAnswerOverSocket(ctx, pluginSide, &rpc.DispatchCommandInput{Command: "test rows received"})
	require.Error(t, err, "the plugin must receive an error answer, got status=%q data=%s", status, data)
	assert.Equal(t, int32(0), calls.Load(), "no handler runs for a refused line")
	lines := strings.Split(err.Error(), "\n")
	assert.Equal(t, "fix: test rows | received", lines[len(lines)-1], "the pipe form is alone on the last line")

	raised := false
	for _, issue := range report.Errors(0) {
		if issue.Source != serverEventSource {
			continue
		}
		if issue.Code != diagnostic.CodeDoctorPluginRequestRefused {
			continue
		}
		if issue.Subject != proc.Name() {
			continue
		}
		raised = true
		assert.Contains(t, issue.Message, "received is a pipe operation of test rows", "the raised event carries the refusal")
	}
	assert.True(t, raised, "the refusal must be raised under %s for the plugin: %v",
		diagnostic.CodeDoctorPluginRequestRefused, report.Errors(0))

	require.NoError(t, pluginSide.Close())
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit")
	}
}

// TestDispatchCommandAnswerRefusesPipeOperationWord proves the DirectBridge
// answer slot (the one an in-process plugin's DispatchCommandResult walks)
// refuses a declared operation typed as an argument with the operator error,
// before the handler runs, and carries no answer.
//
// Method: the typed slots are wired from engineOps as wireBridgeDispatch does.
// Replaces the removed TestDispatchCommandAnswerRefusesPipeChain.
//
// VALIDATES: AC-8 on the dispatch-command answer route.
// PREVENTS: an in-process plugin reading an answer for an argument-form line.
func TestDispatchCommandAnswerRefusesPipeOperationWord(t *testing.T) {
	// Not parallel: it writes the process-wide pipe-filter registry.
	resetPipeOperationRegistries(t)
	command.RegisterPipeFilters([]string{"test rows"},
		command.PipeFilter{Name: "received", Description: "received rows", Argument: command.ArgNone},
	)

	d, calls := rowsTarget(t)
	s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	proc := process.NewProcess(plugin.PluginConfig{Name: "pipe-word-answer"})
	answer, err := pipeChainBridge(s, proc).DispatchCommandAnswer(t.Context(), "test rows received")
	if answer != nil {
		// Walk the answer so the bridge releases its dispatch admission, as the
		// DispatchCommandAnswer contract requires of every caller.
		rows := 0
		for range answer.Records {
			rows++
		}
		t.Errorf("a refused line carries no answer, got %d rows ending %q", rows, answer.Verdict())
	}
	refusal, isOperatorError := errors.AsType[*opererr.Error](err)
	require.True(t, isOperatorError, "the refusal is an operator error, got %T: %v", err, err)
	assert.Equal(t, "test rows | received", refusal.Fix(), "the fix is the pipe form")
	assert.Equal(t, int32(0), calls.Load(), "no handler runs for a refused line")
}
