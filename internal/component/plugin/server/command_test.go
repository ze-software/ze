package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/transaction"
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
		return &plugin.Response{Status: plugin.StatusDone}, nil
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
		return &plugin.Response{Status: plugin.StatusDone}, nil
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
		return &plugin.Response{Status: plugin.StatusDone}, nil
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
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")
	d.Register("peer show", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		matched = "peer show"
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")
	d.Register("peer show extensive", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		matched = "peer show extensive"
		return &plugin.Response{Status: plugin.StatusDone}, nil
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
	cm := transaction.NewCommitManager()
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
	// (e.g., "watchdog announce" not "watchdog announce").
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-watchdog"})
	d.Registry().Register(proc, []CommandDef{
		{Name: "watchdog announce", Description: "Announce watchdog group"},
	})

	// Prefixed command matches (process not running → error, but not ErrUnknownCommand)
	_, err := d.Dispatch(nil, "watchdog announce dnsr")
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
		return &plugin.Response{Status: plugin.StatusDone}, nil
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
// VALIDATES: spec-editor-3 AC-1: "peer eorr ipv4/unicast" → error.
// PREVENTS: Destructive commands silently operating on all peers.
func TestDispatchRejectsNoSelector(t *testing.T) {
	d := NewDispatcher()

	d.RegisterWithOptions("peer eorr", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		t.Fatal("handler should not be called without selector")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Send EoRR", RegisterOptions{RequiresSelector: true})

	ctx := &CommandContext{}
	_, err := d.Dispatch(ctx, "peer eorr ipv4/unicast")
	require.Error(t, err, "mutating command without selector must be rejected")
	assert.Contains(t, err.Error(), "requires a peer selector")
}

// TestDispatchWithSelector verifies that mutating peer commands work with a selector.
//
// VALIDATES: spec-editor-3 AC-2: "peer 1.1.1.1 eorr ipv4/unicast" → works.
// PREVENTS: Selector-requiring commands broken when selector is provided.
func TestDispatchWithSelector(t *testing.T) {
	d := NewDispatcher()

	var calledWithPeer string
	d.RegisterWithOptions("peer eorr", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		calledWithPeer = ctx.PeerSelector()
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Send EoRR", RegisterOptions{RequiresSelector: true})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "peer 1.1.1.1 eorr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "1.1.1.1", calledWithPeer)
}

// TestDispatchReadOnlyNoSelector verifies that read-only peer commands
// default to all peers when no selector is provided.
//
// VALIDATES: spec-editor-3 AC-5: "peer list" → works (defaults to *).
// PREVENTS: Read-only commands broken by selector enforcement.
func TestDispatchReadOnlyNoSelector(t *testing.T) {
	d := NewDispatcher()

	called := false
	d.RegisterWithOptions("peer list", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "List peers", RegisterOptions{RequiresSelector: false})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "peer list")
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchTeardownNoSelector verifies teardown without selector is rejected.
//
// VALIDATES: spec-editor-3 AC-4: "peer teardown 3" → error.
// PREVENTS: Teardown operating on all peers silently.
func TestDispatchTeardownNoSelector(t *testing.T) {
	d := NewDispatcher()

	d.RegisterWithOptions("peer teardown", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		t.Fatal("handler should not be called without selector")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Teardown peer", RegisterOptions{RequiresSelector: true})

	ctx := &CommandContext{}
	_, err := d.Dispatch(ctx, "peer teardown 3")
	require.Error(t, err, "teardown without selector must be rejected")
	assert.Contains(t, err.Error(), "requires a peer selector")
}

// TestDispatchWildcardSelector verifies that "*" counts as a valid selector.
//
// VALIDATES: spec-editor-3 AC-3: "peer * eorr ipv4/unicast" → works.
// PREVENTS: Explicit wildcard rejected when it should be allowed.
func TestDispatchWildcardSelector(t *testing.T) {
	d := NewDispatcher()

	var calledWithPeer string
	d.RegisterWithOptions("peer eorr", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		calledWithPeer = ctx.PeerSelector()
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Send EoRR", RegisterOptions{RequiresSelector: true})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "peer * eorr ipv4/unicast")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "*", calledWithPeer)
}

// TestForwardToPluginNotRegistered verifies ForwardToPlugin returns error
// when the plugin command is not registered (plugin not running).
//
// VALIDATES: ForwardToPlugin returns wrapped ErrUnknownCommand for missing commands.
// PREVENTS: Silent failures when proxy handlers call ForwardToPlugin before plugin starts.
func TestForwardToPluginNotRegistered(t *testing.T) {
	d := NewDispatcher()

	resp, err := d.ForwardToPlugin("rib status", nil, "*")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, ErrUnknownCommand),
		"expected ErrUnknownCommand, got: %v", err)
	assert.Contains(t, err.Error(), "rib status")
}

// TestForwardToPluginRegistered verifies ForwardToPlugin finds registered commands.
// The process is not running, so routeToProcess fails — but the lookup succeeds.
//
// VALIDATES: ForwardToPlugin looks up commands by exact name in the registry.
// PREVENTS: Proxy handlers unable to reach plugin commands after registration.
func TestForwardToPluginRegistered(t *testing.T) {
	d := NewDispatcher()

	// Register a plugin command (process not running)
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-rib"})
	d.Registry().Register(proc, []CommandDef{
		{Name: "rib status", Description: "RIB summary"},
	})

	// ForwardToPlugin should find the command but fail because process isn't running
	resp, err := d.ForwardToPlugin("rib status", nil, "*")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrUnknownCommand),
		"command should be found in registry, got: %v", err)
	// routeToProcess returns ErrPluginProcessNotRunning
	assert.True(t, errors.Is(err, ErrPluginProcessNotRunning),
		"expected ErrPluginProcessNotRunning, got: %v", err)
	assert.Nil(t, resp)
}

// TestForwardToPluginNoBuiltinConflict verifies that registering a builtin
// with "rib status" does not conflict with a plugin command "rib status".
//
// VALIDATES: Builtin proxy "rib status" blocks plugin registration of same name.
// PREVENTS: Duplicate command name confusion in dispatch.
func TestForwardToPluginBuiltinConflict(t *testing.T) {
	d := NewDispatcher()

	// Register builtin "rib status" (the proxy handler)
	d.Register("rib status", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "RIB summary")

	// Plugin tries to register same name -- should be rejected
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-rib"})
	results := d.Registry().Register(proc, []CommandDef{
		{Name: "rib status", Description: "RIB summary"},
	})
	assert.False(t, results[0].OK, "plugin 'rib status' should conflict with builtin 'rib status'")
	assert.Contains(t, results[0].Error, "conflicts with builtin")
}

// mockAuthorizer implements Authorizer for testing.
type mockAuthorizer struct {
	decision authz.Action
}

func (m *mockAuthorizer) Authorize(_, _ string, _ bool) authz.Action {
	return m.decision
}

// TestDispatcherAuthorizationAllow verifies authorized commands execute.
//
// VALIDATES: AC-1 — Dispatcher permits command when authorizer returns Allow.
// PREVENTS: Authorization blocking all commands.
func TestDispatcherAuthorizationAllow(t *testing.T) {
	d := NewDispatcher()
	d.SetAuthorizer(&mockAuthorizer{decision: authz.Allow})

	called := false
	d.Register("peer show", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")

	ctx := &CommandContext{Username: "noc-user"}
	resp, err := d.Dispatch(ctx, "peer show")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, called)
}

// TestDispatcherAuthorizationDeny verifies denied commands return error.
//
// VALIDATES: AC-2 — Dispatcher blocks command when authorizer returns Deny.
// PREVENTS: Authorization bypass allowing all commands.
func TestDispatcherAuthorizationDeny(t *testing.T) {
	d := NewDispatcher()
	d.SetAuthorizer(&mockAuthorizer{decision: authz.Deny})

	d.Register("restart", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		t.Fatal("handler should not be called when denied")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")

	ctx := &CommandContext{Username: "noc-user"}
	resp, err := d.Dispatch(ctx, "restart")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "authorization denied")
}

// TestDispatcherNoAuthorizerAllowsAll verifies nil authorizer permits everything.
//
// VALIDATES: AC-5 — No authorizer set = all commands allowed.
// PREVENTS: Nil authorizer causing panics or denials.
func TestDispatcherNoAuthorizerAllowsAll(t *testing.T) {
	d := NewDispatcher()
	// No SetAuthorizer call

	called := false
	d.Register("restart", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")

	resp, err := d.Dispatch(nil, "restart")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, called)
}

// TestDispatcherAuthorizationUsesReadOnly verifies ReadOnly flag is passed to authorizer.
//
// VALIDATES: ReadOnly flag from Command is used for section selection.
// PREVENTS: All commands evaluated against wrong section.
func TestDispatcherAuthorizationUsesReadOnly(t *testing.T) {
	d := NewDispatcher()

	var capturedReadOnly bool
	d.SetAuthorizer(&readOnlyCapture{captured: &capturedReadOnly})

	d.RegisterWithOptions("peer show", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "", RegisterOptions{ReadOnly: true})

	d.RegisterWithOptions("config set", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "", RegisterOptions{ReadOnly: false})

	ctx := &CommandContext{Username: "user1"}

	// ReadOnly command
	resp, err := d.Dispatch(ctx, "peer show")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, capturedReadOnly, "peer show should be ReadOnly=true")

	// Write command
	resp, err = d.Dispatch(ctx, "config set")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.False(t, capturedReadOnly, "config set should be ReadOnly=false")
}

// readOnlyCapture captures the readOnly argument passed to Authorize.
type readOnlyCapture struct {
	captured *bool
}

func (r *readOnlyCapture) Authorize(_, _ string, readOnly bool) authz.Action {
	*r.captured = readOnly
	return authz.Allow
}

// TestDispatcherAuthorizationUsesUsername verifies Username from context is passed.
//
// VALIDATES: AC-12 — CommandContext.Username passed to authorizer.
// PREVENTS: Authorization using wrong or empty username.
func TestDispatcherAuthorizationUsesUsername(t *testing.T) {
	d := NewDispatcher()

	var capturedUsername string
	d.SetAuthorizer(&usernameCapture{captured: &capturedUsername})

	d.Register("peer show", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")

	ctx := &CommandContext{Username: "admin-user"}
	resp, err := d.Dispatch(ctx, "peer show")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "admin-user", capturedUsername)
}

// usernameCapture captures the username argument passed to Authorize.
type usernameCapture struct {
	captured *string
}

func (u *usernameCapture) Authorize(username, _ string, _ bool) authz.Action {
	*u.captured = username
	return authz.Allow
}

// TestDispatcherWithAuthzStore verifies the authz.Store integrates with the dispatcher.
// This is the wiring test: authz.Store satisfies server.Authorizer and controls dispatch.
//
// VALIDATES: AC-3 — authz.Store plugs into Dispatcher as Authorizer.
// PREVENTS: Type mismatch or interface incompatibility at integration boundary.
func TestDispatcherWithAuthzStore(t *testing.T) {
	store := authz.NewStore()

	// Create a restrictive profile: allow "peer show", deny everything else
	store.AddProfile(authz.Profile{
		Name: "noc",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "peer show"},
			},
		},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AssignProfiles("operator", []string{"noc"})

	d := NewDispatcher()
	d.SetAuthorizer(store)

	showCalled := false
	d.RegisterWithOptions("peer show", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		showCalled = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "", RegisterOptions{ReadOnly: true})

	restartCalled := false
	d.RegisterWithOptions("restart", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		restartCalled = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "", RegisterOptions{ReadOnly: true})

	ctx := &CommandContext{Username: "operator"}

	// Allowed command
	resp, err := d.Dispatch(ctx, "peer show")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, showCalled)

	// Denied command
	resp, err = d.Dispatch(ctx, "restart")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.False(t, restartCalled)

	// No-auth user (empty username) should be allowed everything
	noAuthCtx := &CommandContext{Username: ""}
	resp, err = d.Dispatch(noAuthCtx, "restart")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	// User with no profile assignment gets admin (allow all)
	unknownCtx := &CommandContext{Username: "unknown-user"}
	resp, err = d.Dispatch(unknownCtx, "restart")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// TestDispatcherAuthorizationAppliesToUnknownCommands verifies that authorization
// is checked even for commands that don't match any builtin handler (subsystem/plugin path).
//
// VALIDATES: AC-4 — Authorization applies to all command paths, not just builtins.
// PREVENTS: Authorization bypass by sending unregistered commands to plugin/subsystem dispatch.
func TestDispatcherAuthorizationAppliesToUnknownCommands(t *testing.T) {
	d := NewDispatcher()
	d.SetAuthorizer(&mockAuthorizer{decision: authz.Deny})

	// Don't register "custom plugin cmd" as a builtin — it falls to plugin dispatch.
	ctx := &CommandContext{Username: "noc-user"}
	resp, err := d.Dispatch(ctx, "custom plugin cmd")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "authorization denied")
}
