package rib

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRouteReadTakesScopeAtAnyPosition proves a scope operation of
// `show bgp rib` is read wherever it sits before the terminal
// (AC-26 of spec-pipe-chain-honored-on-every-command-surface).
//
// METHOD: the same received scope typed first, after a filter, and just
// before the terminal; each must answer the received scope with the filter
// and the terminal kept in order.
func TestRouteReadTakesScopeAtAnyPosition(t *testing.T) {
	for _, words := range [][]string{
		{"received", "family", "ipv4/unicast", "count"},
		{"family", "ipv4/unicast", "received", "count"},
	} {
		scope, _, _, stages, problem := parsePipelineOperations(ribPipe(words...))
		require.Empty(t, problem, "%v", words)
		assert.Equal(t, scopeReceived, scope, "%v", words)
		require.Len(t, stages, 2, "%v", words)
		assert.Equal(t, rowKeyFamily, stages[0].kind, "%v", words)
		assert.Equal(t, terminalCount, stages[1].kind, "%v", words)
	}
}

// TestRouteReadRefusesTwoScopes proves two scopes are refused naming both,
// wherever the second one sits (AC-26).
func TestRouteReadRefusesTwoScopes(t *testing.T) {
	_, _, _, _, problem := parsePipelineOperations(ribPipe("received", "family", "ipv4/unicast", "advertised"))
	assert.Equal(t, "multiple route direction filters: received and advertised", problem)
}

// TestRouteReadRefusesArgumentForm proves the route reads take no selection
// as an argument: a word left on the path is refused with the pipe form as
// the fix, and no route is read (AC-8, the plugin's half; the engine's
// argument validator refuses a declared word before the plugin is reached).
func TestRouteReadRefusesArgumentForm(t *testing.T) {
	r := newTestRIBManager(t)
	seedShowRows(t, r, 2)
	for _, command := range []string{"show bgp rib", "show bgp rib best"} {
		status, data, err := r.handleCommand(command, "*", []string{"received"})
		require.Error(t, err, command)
		assert.Equal(t, statusError, status, command)
		assert.Empty(t, data, command)
		assert.Contains(t, err.Error(), "takes its selection after a pipe, and was given the argument received", command)
		assert.Contains(t, err.Error(), command+" | count", command)
	}
}

// TestPlainCommandRefusesAPipeOperation proves a command that declares no pipe
// operation refuses one rather than answer as if it was not typed.
func TestPlainCommandRefusesAPipeOperation(t *testing.T) {
	r := newTestRIBManager(t)
	status, _, err := r.handleCommand("show bgp rib status", "*", nil, sdk.PipeOperation{Name: "count"})
	require.Error(t, err)
	assert.Equal(t, statusError, status)
	assert.Contains(t, err.Error(), "show bgp rib status declares no pipe operation, and was handed count")

	status, _, err = r.handleCommand("show bgp rib status", "*", nil)
	require.NoError(t, err, "with no operation the command answers as before")
	assert.Equal(t, statusDone, status)
}

// TestProtocolRefusesScopeOperation proves a scope on `show bgp rib protocol`
// is refused naming it, on an empty protocol RIB too (D-3): a protocol RIB
// holds received routes only, so the scope could only be ignored.
func TestProtocolRefusesScopeOperation(t *testing.T) {
	r := newTestRIBManager(t)
	status, _, err := r.showProtocolPipeline("bmp", ribPipe("advertised", "count"))
	require.Error(t, err)
	assert.Equal(t, statusError, status)
	assert.Contains(t, err.Error(), "show bgp rib protocol takes no scope, and was given advertised")

	status, _, err = r.showProtocolPipeline("bmp", ribPipe("count"))
	require.NoError(t, err, "the same line without the scope is answered")
	assert.Equal(t, statusDone, status)
}

// TestDeclaredFilterValueWords proves how a filter reads its argument words:
// `match` reads them as one text, as the generic `| match` it overrides does,
// and a one-value filter refuses a second word naming the quoted form.
func TestDeclaredFilterValueWords(t *testing.T) {
	_, _, _, stages, problem := parsePipelineOperations([]sdk.PipeOperation{
		{Name: filterMatch, Args: []string{"link", "down"}},
	})
	require.Empty(t, problem)
	require.Len(t, stages, 1)
	assert.Equal(t, "link down", stages[0].arg)

	_, _, _, _, problem = parsePipelineOperations([]sdk.PipeOperation{
		{Name: rowKeyPeer, Args: []string{"192.0.2.1", "192.0.2.2"}},
	})
	assert.Equal(t, `peer takes one value: quote a value that holds a space, as in | peer "192.0.2.1 192.0.2.2"`, problem)

	_, _, problem = parseBestPipelineOperations([]sdk.PipeOperation{{Name: terminalCount, Args: []string{"5"}}})
	assert.Equal(t, "count takes no value, and was given 5", problem)
}

// TestForkedRIBReceivesTheDeclaredList proves the declared list reaches the
// route read over the plugin's wire transport, the route a forked bgp-rib is
// reached by (AC-9, its unit half; dispatch-pipe-chain-applied.ci holds the
// daemon half).
//
// METHOD: the plugin runs on its RPC connection with no DirectBridge, so the
// request crosses as JSON. `show bgp rib` with the list [count] must answer
// the count document; the same request with no list answers the route rows.
// A transport that dropped the list would answer rows for both.
func TestForkedRIBReceivesTheDeclaredList(t *testing.T) {
	// The helper's cleanup waits for the plugin to exit on this context, so it
	// MUST outlive the test body: a deferred cancel ends it before any cleanup
	// runs, and that wait then fails at once. selectedDispatchContext cancels
	// it in a cleanup registered first, which runs last.
	ctx := selectedDispatchContext(t)
	peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")}
	// The RIB holds no route, so nothing is selected; a selection would be a
	// fault of this test, answered as one rather than as a nil call.
	refuseSelection := func(context.Context, rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return nil, errors.New("no route selection is expected from an empty RIB")
	}
	manager, _ := startQuiesceRIB(t, ctx, false, peers, refuseSelection)
	connection := manager.GetProcess("bgp-rib").Conn()

	counted, err := connection.SendExecuteCommand(ctx, &rpc.ExecuteCommandInput{
		Serial: "declared-count", Command: "show bgp rib", Pipe: []rpc.PipeOperation{{Name: terminalCount}},
	})
	require.NoError(t, err)
	require.Equal(t, statusDone, counted.Status)
	var count map[string]any
	require.NoError(t, json.Unmarshal(counted.Data, &count))
	assert.Equal(t, map[string]any{jsonKeyCount: float64(0)}, count)

	bare, err := connection.SendExecuteCommand(ctx, &rpc.ExecuteCommandInput{Serial: "bare", Command: "show bgp rib"})
	require.NoError(t, err)
	require.Equal(t, statusDone, bare.Status)
	var rows map[string]any
	require.NoError(t, json.Unmarshal(bare.Data, &rows))
	assert.Contains(t, rows, showRowsEnvelopeKey, "with no list the command answers its rows")
}
