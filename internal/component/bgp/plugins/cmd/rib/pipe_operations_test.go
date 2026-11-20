// Design: docs/architecture/api/commands.md — command-owned pipe operations
// Related: rib.go — registerPipeFilters, the declarations this file pins

package rib

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// samePipeOperation compares two declared operations by name and argument
// words. Nil and empty argument lists differ on purpose: the partition answers
// nil for an operation with no argument on every branch (argumentWords,
// internal/component/command/pipe_partition.go), and a handler compares one
// value.
func samePipeOperation(a, b command.PipeOperation) bool {
	if a.Name != b.Name {
		return false
	}
	if (a.Args == nil) != (b.Args == nil) {
		return false
	}
	return slices.Equal(a.Args, b.Args)
}

// declaredNames answers the names of the operations a command path declares,
// sorted, as help and the partition resolve them.
func declaredNames(path string) []string {
	filters := command.PipeFiltersForCommand(path)
	names := make([]string, 0, len(filters))
	for _, filter := range filters {
		names = append(names, filter.Name)
	}
	slices.Sort(names)
	return names
}

// TestPeerRibDeclaresItsOperations holds `show bgp peer rib` to the operation
// set of `show bgp rib`, the scope words included, and `show bgp peer` to none.
//
// VALIDATES: AC-34, declaration half. Both paths forward to the same plugin
// command (forwardRibRoutes), so they answer the same operations; the scope
// is a pipe operation now that the YANG scope leaf is gone (D-9).
// PREVENTS: the peer-scoped reader losing its scope with the leaf, and the
// declaration leaking up to `show bgp peer`, whose answer has no routes.
func TestPeerRibDeclaresItsOperations(t *testing.T) {
	peerRib := declaredNames(cmdBgpPeerRib)
	require.NotEmpty(t, peerRib, "%s declares no pipe operation", cmdBgpPeerRib)
	assert.Equal(t, declaredNames(cmdRibShow), peerRib,
		"%s and %s answer the same rows and must declare the same operations", cmdBgpPeerRib, cmdRibShow)
	for _, scope := range []string{"received", "advertised", "sent", "sent-received"} {
		assert.Contains(t, peerRib, scope, "%s does not declare the scope %s", cmdBgpPeerRib, scope)
	}
	assert.Empty(t, declaredNames("show bgp peer"), "show bgp peer inherits the peer RIB's operations")
}

// TestRibDeclaresTheWordsItsReadersAccept holds each RIB reader to declaring
// the words its argument grammar accepted and no declaration named.
//
// VALIDATES: the "Declarations audited" decisions of the spec. The argument
// form is refused now (AC-8), so a word left undeclared would be unreachable:
// `sent` and `sent-received` on `show bgp rib`, `aspath` on it and on
// `show bgp rib best`. `show bgp rib best` declares no scope, no covering and
// no exact, which its reader refuses, and it alone declares reason.
// PREVENTS: a reachable feature dropped with the argument form.
func TestRibDeclaresTheWordsItsReadersAccept(t *testing.T) {
	routes := declaredNames(cmdRibShow)
	for _, name := range []string{"sent", "sent-received", "aspath"} {
		assert.Contains(t, routes, name, "%s does not declare %s", cmdRibShow, name)
	}
	assert.NotContains(t, routes, "reason", "%s declares the best-path operation reason", cmdRibShow)

	best := declaredNames(cmdRibBest)
	assert.Contains(t, best, "aspath", "%s does not declare aspath", cmdRibBest)
	assert.Contains(t, best, "reason", "%s does not declare reason", cmdRibBest)
	for _, name := range []string{"received", "advertised", "sent", "sent-received", "covering", "exact"} {
		assert.NotContains(t, best, name, "%s declares %s, which its reader refuses", cmdRibBest, name)
	}
}

// TestProtocolRibDeclaresItsOwnSet holds `show bgp rib protocol` to the
// operations of `show bgp rib` less the scope words.
//
// VALIDATES: D-3, declaration half. The protocol tables hold received routes
// only, so a scope means nothing there.
// PREVENTS: the protocol reader inheriting the scope words of `show bgp rib`
// by prefix again, which accepted `| advertised` and did nothing with it.
func TestProtocolRibDeclaresItsOwnSet(t *testing.T) {
	protocol := declaredNames(cmdRibProtocol)
	require.NotEmpty(t, protocol, "%s declares no pipe operation", cmdRibProtocol)
	scopes := []string{"received", "advertised", "sent", "sent-received"}
	for _, scope := range scopes {
		assert.NotContains(t, protocol, scope, "%s declares the scope %s", cmdRibProtocol, scope)
	}
	withoutScopes := slices.DeleteFunc(declaredNames(cmdRibShow), func(name string) bool {
		return slices.Contains(scopes, name)
	})
	assert.Equal(t, withoutScopes, protocol)
}

// protocolDispatcher registers `show bgp rib protocol` with the argument
// definitions the model produces and a handler that records what it received.
func protocolDispatcher(t *testing.T) (*pluginserver.Dispatcher, *bool, *[]command.PipeOperation) {
	t.Helper()
	defs := configyang.PathToArgDefs(ribLoader(t))[cmdRibProtocol]
	require.NotEmpty(t, defs, "the model declares no argument for %q", cmdRibProtocol)

	called := false
	var declared []command.PipeOperation
	handler := func(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
		called = true
		declared = validated.PipeOperations()
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"count": 0}}, nil
	}
	d := pluginserver.NewDispatcher()
	if err := d.RegisterWithOptions(cmdRibProtocol, handler, "under test", pluginserver.RegisterOptions{ArgDefs: defs}); err != nil {
		t.Fatal(err)
	}
	return d, &called, &declared
}

// TestProtocolRibRefusesScopeOperation dispatches a scope on
// `show bgp rib protocol <p>` and expects a refusal naming the valid set,
// before the handler runs.
//
// VALIDATES: D-3, dispatch half.
// PREVENTS: a scope accepted on a reader that discards it, answering every
// received route to an operator who asked for advertised ones.
func TestProtocolRibRefusesScopeOperation(t *testing.T) {
	d, called, _ := protocolDispatcher(t)

	_, err := d.Dispatch(&pluginserver.CommandContext{}, cmdRibProtocol+" bmp | advertised")
	require.Error(t, err, "a scope on %s was accepted", cmdRibProtocol)
	assert.Contains(t, err.Error(), "advertised", "the refusal does not name the operation")
	assert.Contains(t, err.Error(), "valid:", "the refusal does not name the valid set")
	assert.Contains(t, err.Error(), "peer", "the valid set does not hold the protocol reader's own operations")
	assert.NotContains(t, err.Error(), "received", "the valid set offers a scope")
	assert.False(t, *called, "the handler ran on a refused scope")
}

// TestProtocolRibHandsItsOperationsToTheHandler is the positive case beside the
// refusal: the protocol reader's own operations reach its handler in the
// declared list, in the order typed, and the protocol stays an argument.
func TestProtocolRibHandsItsOperationsToTheHandler(t *testing.T) {
	d, called, declared := protocolDispatcher(t)

	resp, err := d.Dispatch(&pluginserver.CommandContext{}, cmdRibProtocol+" bmp | peer 192.0.2.1 | count")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, *called, "the handler did not run")
	want := []command.PipeOperation{{Name: "peer", Args: []string{"192.0.2.1"}}, {Name: "count"}}
	assert.True(t, slices.EqualFunc(*declared, want, samePipeOperation),
		"the handler received %v, want %v", *declared, want)
}
