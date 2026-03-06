package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/commit"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
)

// TestDispatcherRegister verifies command registration.
//
// VALIDATES: Commands are registered and retrievable.
//
// PREVENTS: Missing command handlers causing silent failures.
func TestDispatcherRegister(t *testing.T) {
	d := NewDispatcher()

	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		return &plugin.Response{Status: "done"}, nil
	}

	d.Register("test command", handler, "Test command help")

	cmd := d.Lookup("test command")
	require.NotNil(t, cmd, "registered command must be found")
	assert.Equal(t, "test command", cmd.Name)
	assert.Equal(t, "Test command help", cmd.Help)

	// Verify handler is set
	require.NotNil(t, cmd.Handler)
}

// TestDispatcherDispatch verifies command routing.
//
// VALIDATES: Commands are routed to correct handler with args.
//
// PREVENTS: Command misdirection or lost arguments.
func TestDispatcherDispatch(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: "done"}, nil
	}

	d.Register("peer show", handler, "Show peers")

	resp, err := d.Dispatch(nil, "peer show extensive")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, []string{"extensive"}, receivedArgs)
}

// TestDispatcherDispatchNoArgs verifies dispatch with no extra args.
//
// VALIDATES: Commands without args receive empty slice.
//
// PREVENTS: Nil slice causing panics in handlers.
func TestDispatcherDispatchNoArgs(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: "done"}, nil
	}

	d.Register("daemon shutdown", handler, "Shutdown daemon")

	resp, err := d.Dispatch(nil, "daemon shutdown")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Empty(t, receivedArgs)
}

// TestDispatcherUnknownCommand verifies error for unknown commands.
//
// VALIDATES: Unknown commands return ErrUnknownCommand.
//
// PREVENTS: Silent failures on typos or unsupported commands.
func TestDispatcherUnknownCommand(t *testing.T) {
	d := NewDispatcher()

	resp, err := d.Dispatch(nil, "unknown command")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownCommand))
	assert.Nil(t, resp)
}

// TestDispatcherEmptyCommand verifies error for empty input.
//
// VALIDATES: Empty commands are rejected.
//
// PREVENTS: Panics or undefined behavior on empty input.
func TestDispatcherEmptyCommand(t *testing.T) {
	d := NewDispatcher()

	resp, err := d.Dispatch(nil, "")
	require.Error(t, err)
	assert.Nil(t, resp)

	resp, err = d.Dispatch(nil, "   ")
	require.Error(t, err)
	assert.Nil(t, resp)
}

// TestDispatcherLongestMatch verifies longest prefix matching.
//
// VALIDATES: More specific commands take precedence.
//
// PREVENTS: "peer show" matching when "peer show extensive" is meant.
func TestDispatcherLongestMatch(t *testing.T) {
	d := NewDispatcher()

	var matched string
	d.Register("peer", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		matched = "peer"
		return &plugin.Response{Status: "done"}, nil
	}, "")
	d.Register("peer show", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		matched = "peer show"
		return &plugin.Response{Status: "done"}, nil
	}, "")
	d.Register("peer show extensive", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		matched = "peer show extensive"
		return &plugin.Response{Status: "done"}, nil
	}, "")

	// "peer show extensive" should match the most specific
	_, err := d.Dispatch(nil, "peer show extensive")
	require.NoError(t, err)
	assert.Equal(t, "peer show extensive", matched)

	// "peer show summary" should match "peer show" with arg "summary"
	_, err = d.Dispatch(nil, "peer show summary")
	require.NoError(t, err)
	assert.Equal(t, "peer show", matched)

	// "peer list" should match "peer" with arg "list"
	_, err = d.Dispatch(nil, "peer list")
	require.NoError(t, err)
	assert.Equal(t, "peer", matched)
}

// TestDispatcherTokenize verifies command tokenization.
//
// VALIDATES: Commands are split correctly on whitespace.
//
// PREVENTS: Argument parsing errors from extra whitespace.
func TestDispatcherTokenize(t *testing.T) {
	tests := []struct {
		input  string
		tokens []string
	}{
		{"peer show", []string{"peer", "show"}},
		{"peer  show", []string{"peer", "show"}},
		{"  peer show  ", []string{"peer", "show"}},
		{"peer\tshow", []string{"peer", "show"}},
		{"update text nlri ipv4/unicast add 10.0.0.0/24", []string{"update", "text", "nlri", "ipv4/unicast", "add", "10.0.0.0/24"}},
		// Quoted strings
		{`myapp check "hello world"`, []string{"myapp", "check", "hello world"}},
		{`register command "myapp status" description "Show status"`, []string{"register", "command", "myapp status", "description", "Show status"}},
		// Escaped quotes
		{`myapp set "value with \"quotes\""`, []string{"myapp", "set", `value with "quotes"`}},
		// Escaped backslash
		{`myapp path "C:\\Users\\test"`, []string{"myapp", "path", `C:\Users\test`}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := tokenize(tt.input)
			assert.Equal(t, tt.tokens, tokens)
		})
	}
}

// TestDispatcherHandlerError verifies handler error propagation.
//
// VALIDATES: Handler errors are returned to caller.
//
// PREVENTS: Swallowed errors hiding failures.
func TestDispatcherHandlerError(t *testing.T) {
	d := NewDispatcher()

	handlerErr := errors.New("handler failed")
	d.Register("fail", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		return nil, handlerErr
	}, "")

	resp, err := d.Dispatch(nil, "fail")
	require.Error(t, err)
	assert.True(t, errors.Is(err, handlerErr))
	assert.Nil(t, resp)
}

// TestDispatcherListCommands verifies command listing.
//
// VALIDATES: All registered commands are returned.
//
// PREVENTS: Help command missing available commands.
func TestDispatcherListCommands(t *testing.T) {
	d := NewDispatcher()

	d.Register("daemon shutdown", nil, "Shutdown the daemon")
	d.Register("peer show", nil, "Show peers")
	d.Register("rib show in", nil, "Show Adj-RIB-In")

	cmds := d.Commands()
	assert.Len(t, cmds, 3)

	// Check all are present
	names := make(map[string]bool)
	for _, cmd := range cmds {
		names[cmd.Name] = true
	}
	assert.True(t, names["daemon shutdown"])
	assert.True(t, names["peer show"])
	assert.True(t, names["rib show in"])
}

// TestCommandContextNilServer verifies accessor methods return nil safely when Server is nil.
//
// VALIDATES: Nil-safe accessors return nil/zero when Server is not set.
// PREVENTS: Nil pointer panics in tests or handlers that don't need a full Server.
func TestCommandContextNilServer(t *testing.T) {
	ctx := &CommandContext{}

	assert.Nil(t, ctx.Reactor(), "Reactor() should return nil when Server is nil")
	assert.Nil(t, ctx.Dispatcher(), "Dispatcher() should return nil when Server is nil")
	assert.Nil(t, ctx.CommitManager(), "CommitManager() should return nil when Server is nil")
	assert.Nil(t, ctx.Subscriptions(), "Subscriptions() should return nil when Server is nil")
}

// TestCommandContextAccessors verifies accessor methods delegate to Server fields correctly.
//
// VALIDATES: Accessor methods on CommandContext return the corresponding Server fields.
// PREVENTS: Accessor methods returning wrong or stale values.
func TestCommandContextAccessors(t *testing.T) {
	reactor := &mockReactor{}
	dispatcher := NewDispatcher()
	cm := commit.NewCommitManager()
	subs := NewSubscriptionManager()

	srv := &Server{
		reactor:       reactor,
		dispatcher:    dispatcher,
		commitManager: cm,
		subscriptions: subs,
	}

	ctx := &CommandContext{Server: srv}

	assert.Equal(t, reactor, ctx.Reactor(), "Reactor() should return server's reactor")
	assert.Equal(t, dispatcher, ctx.Dispatcher(), "Dispatcher() should return server's dispatcher")
	assert.Equal(t, cm, ctx.CommitManager(), "CommitManager() should return server's commitManager")
	assert.Equal(t, subs, ctx.Subscriptions(), "Subscriptions() should return server's subscriptions")
}

// TestDispatcherPluginMatch verifies plugin command dispatch via the registry.
//
// VALIDATES: Plugin commands are matched by the dispatcher's plugin registry path.
// PREVENTS: Plugin commands unreachable through normal dispatch.
func TestDispatcherPluginMatch(t *testing.T) {
	d := NewDispatcher()

	// Register plugin command with full prefix — plugins that handle
	// commands arriving via update-route RPC must include the domain prefix
	// (e.g., "bgp watchdog announce" not "watchdog announce").
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-watchdog"})
	d.Registry().Register(proc, []CommandDef{
		{Name: "bgp watchdog announce", Description: "Announce watchdog group"},
	})

	// Prefixed command matches (process not running → error, but not ErrUnknownCommand)
	_, err := d.Dispatch(nil, "bgp watchdog announce dnsr")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrUnknownCommand),
		"plugin command should match, got: %v", err)
}

// TestDispatcherCaseInsensitive verifies case handling.
//
// VALIDATES: Commands are matched case-insensitively.
//
// PREVENTS: Users typing "Peer Show" failing when "peer show" works.
func TestDispatcherCaseInsensitive(t *testing.T) {
	d := NewDispatcher()

	called := false
	d.Register("peer show", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: "done"}, nil
	}, "")

	// Should match regardless of case
	_, err := d.Dispatch(nil, "PEER SHOW")
	require.NoError(t, err)
	assert.True(t, called)

	called = false
	_, err = d.Dispatch(nil, "Peer Show")
	require.NoError(t, err)
	assert.True(t, called)
}

// TestDispatchRejectsNoSelector verifies that mutating peer commands
// without a peer selector are rejected at the dispatcher level.
//
// VALIDATES: spec-editor-3 AC-1: "bgp peer eorr ipv4/unicast" → error.
// PREVENTS: Destructive commands silently operating on all peers.
func TestDispatchRejectsNoSelector(t *testing.T) {
	d := NewDispatcher()

	d.RegisterWithOptions("bgp peer eorr", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		t.Fatal("handler should not be called without selector")
		return &plugin.Response{Status: "done"}, nil
	}, "Send EoRR", RegisterOptions{RequiresSelector: true})

	ctx := &CommandContext{}
	_, err := d.Dispatch(ctx, "bgp peer eorr ipv4/unicast")
	require.Error(t, err, "mutating command without selector must be rejected")
	assert.Contains(t, err.Error(), "requires a peer selector")
}

// TestDispatchWithSelector verifies that mutating peer commands work with a selector.
//
// VALIDATES: spec-editor-3 AC-2: "bgp peer 1.1.1.1 eorr ipv4/unicast" → works.
// PREVENTS: Selector-requiring commands broken when selector is provided.
func TestDispatchWithSelector(t *testing.T) {
	d := NewDispatcher()

	var calledWithPeer string
	d.RegisterWithOptions("bgp peer eorr", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		calledWithPeer = ctx.PeerSelector()
		return &plugin.Response{Status: "done"}, nil
	}, "Send EoRR", RegisterOptions{RequiresSelector: true})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "bgp peer 1.1.1.1 eorr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "1.1.1.1", calledWithPeer)
}

// TestDispatchReadOnlyNoSelector verifies that read-only peer commands
// default to all peers when no selector is provided.
//
// VALIDATES: spec-editor-3 AC-5: "bgp peer list" → works (defaults to *).
// PREVENTS: Read-only commands broken by selector enforcement.
func TestDispatchReadOnlyNoSelector(t *testing.T) {
	d := NewDispatcher()

	called := false
	d.RegisterWithOptions("bgp peer list", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: "done"}, nil
	}, "List peers", RegisterOptions{RequiresSelector: false})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "bgp peer list")
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchTeardownNoSelector verifies teardown without selector is rejected.
//
// VALIDATES: spec-editor-3 AC-4: "bgp peer teardown 3" → error.
// PREVENTS: Teardown operating on all peers silently.
func TestDispatchTeardownNoSelector(t *testing.T) {
	d := NewDispatcher()

	d.RegisterWithOptions("bgp peer teardown", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		t.Fatal("handler should not be called without selector")
		return &plugin.Response{Status: "done"}, nil
	}, "Teardown peer", RegisterOptions{RequiresSelector: true})

	ctx := &CommandContext{}
	_, err := d.Dispatch(ctx, "bgp peer teardown 3")
	require.Error(t, err, "teardown without selector must be rejected")
	assert.Contains(t, err.Error(), "requires a peer selector")
}

// TestDispatchWildcardSelector verifies that "*" counts as a valid selector.
//
// VALIDATES: spec-editor-3 AC-3: "bgp peer * eorr ipv4/unicast" → works.
// PREVENTS: Explicit wildcard rejected when it should be allowed.
func TestDispatchWildcardSelector(t *testing.T) {
	d := NewDispatcher()

	var calledWithPeer string
	d.RegisterWithOptions("bgp peer eorr", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		calledWithPeer = ctx.PeerSelector()
		return &plugin.Response{Status: "done"}, nil
	}, "Send EoRR", RegisterOptions{RequiresSelector: true})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "bgp peer * eorr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "*", calledWithPeer)
}
