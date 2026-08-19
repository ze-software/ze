package server

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/bgp/transaction"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/selector"
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

	d.Register("alpha show", handler, "Show alpha")

	resp, err := d.Dispatch(nil, "alpha show extensive")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, []string{"extensive"}, receivedArgs)
}

// TestDispatchRejectsFlagShapedArgs is the fix for the silent-swallow class.
//
// matchCommandTokens walks only the KEY's tokens and never checks that the
// input is exhausted; it returns the unmatched tail as args and reports a
// successful match. For a command whose node has no leaves there are no
// ArgDefs, so the validation guarded by `len(matchedCmd.ArgDefs) > 0` is
// skipped entirely and the tail reaches a handler that may ignore it. The real
// case: `show l2tp --user alice tunnels` matched `show l2tp`, whose handler
// takes `_ []string`, so the operator got the SUMMARY for the DEFAULT user,
// exit 0, with no hint that `--user alice` and `tunnels` were both discarded.
//
// A flag-shaped token can never be legitimate here: ArgKind has no signed
// numeric kind (internal/component/command/node.go:17-20), and the pipe folding
// that rewrites `| peer X` into trailing args (internal/component/command/
// pipe.go:181) only ever emits bare filter names and their values.
//
// VALIDATES: a flag-shaped leftover is rejected with a non-nil error and the
// handler never runs.
// PREVENTS: acting as, or reporting on, something the operator did not ask for
// while returning success.
func TestDispatchRejectsFlagShapedArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "long flag before subcommand", input: "alpha show --user alice detail", want: "--user"},
		{name: "short flag", input: "alpha show -u alice", want: "-u"},
		{name: "single-dash long form", input: "alpha show -user alice", want: "-user"},
		{name: "flag after positional", input: "alpha show detail --user alice", want: "--user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDispatcher()
			called := false
			d.Register("alpha show", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
				called = true
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "Show alpha")

			resp, err := d.Dispatch(nil, tt.input)
			require.Error(t, err, "flag-shaped leftover must not dispatch successfully")
			assert.Contains(t, err.Error(), tt.want)
			if resp != nil {
				assert.Equal(t, plugin.StatusError, resp.Status)
			}
			assert.False(t, called, "handler ran despite the rejected flag")
		})
	}
}

// TestDispatchAllowsNonFlagArgs pins the behavior the fix must NOT break.
//
// Zero ArgDefs does not mean "takes no arguments": extractArgDefs reads YANG
// LEAF children only (internal/component/config/yang/command.go), so the ~60%
// of command nodes whose children are containers get nil ArgDefs while their
// handlers still read positional args (all of OSPF, the L2TP diag verbs,
// `clear l2tp tunnel id 42`). Folded pipe filters arrive the same way:
// `show bgp rib | peer X | family ipv4` is sent as `show bgp rib peer X family
// ipv4`.
//
// VALIDATES: ordinary positional args, folded pipe-filter args, a bare "-",
// the "--" end-of-options marker, and negative-looking values all still reach
// the handler untouched.
// PREVENTS: a flag check that overreaches into "reject any leftover", which
// would disable RIB filters, OSPF, and L2TP teardown.
func TestDispatchAllowsNonFlagArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "plain positional", input: "alpha show extensive", want: []string{"extensive"}},
		{name: "folded pipe filters", input: "alpha show peer 10.0.0.1 family ipv4 count", want: []string{"peer", "10.0.0.1", "family", "ipv4", "count"}},
		{name: "bare dash", input: "alpha show -", want: []string{"-"}},
		{name: "end of options marker", input: "alpha show --", want: []string{"--"}},
		{name: "negative-looking value", input: "alpha show -5", want: []string{"-5"}},
		{name: "kebab filter name", input: "alpha show prefix-summary", want: []string{"prefix-summary"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDispatcher()
			var got []string
			d.Register("alpha show", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
				got = args
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "Show alpha")

			resp, err := d.Dispatch(nil, tt.input)
			require.NoError(t, err)
			assert.Equal(t, "done", resp.Status)
			assert.Equal(t, tt.want, got)
		})
	}
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
// PREVENTS: "alpha show" matching when "alpha show extensive" is meant.
func TestDispatcherLongestMatch(t *testing.T) {
	d := NewDispatcher()

	var matched string
	d.Register("alpha", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		matched = "alpha"
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")
	d.Register("alpha show", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		matched = "alpha show"
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")
	d.Register("alpha show extensive", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		matched = "alpha show extensive"
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")

	// "alpha show extensive" should match the most specific
	_, err := d.Dispatch(nil, "alpha show extensive")
	require.NoError(t, err)
	assert.Equal(t, "alpha show extensive", matched)

	// "alpha show summary" should match "alpha show" with arg "summary"
	_, err = d.Dispatch(nil, "alpha show summary")
	require.NoError(t, err)
	assert.Equal(t, "alpha show", matched)

	// "alpha list" should match "alpha" with arg "list"
	_, err = d.Dispatch(nil, "alpha list")
	require.NoError(t, err)
	assert.Equal(t, "alpha", matched)
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
		{"alpha show", []string{"alpha", "show"}},
		{"alpha  show", []string{"alpha", "show"}},
		{"  alpha show  ", []string{"alpha", "show"}},
		{"alpha\tshow", []string{"alpha", "show"}},
		{"update text nlri ipv4/unicast add 10.0.0.0/24", []string{"update", "text", "nlri", "ipv4/unicast", "add", "10.0.0.0/24"}},
		// Quoted strings
		{`myapp check "hello world"`, []string{"myapp", "check", "hello world"}},
		{`register command "myapp status" description "Show status"`, []string{"register", "command", "myapp status", "description", "Show status"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.tokens, tokens)
		})
	}
}

// TestTokenizeRejectsBackslash verifies backslash is rejected in commands.
//
// VALIDATES: Commands with backslash return an error.
// PREVENTS: Escape sequences in command input.
func TestTokenizeRejectsBackslash(t *testing.T) {
	inputs := []string{
		`set path C:\Users`,
		`myapp path "C:\Users\test"`,
		`myapp set "value with \"quotes\""`,
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := tokenize(input)
			require.ErrorIs(t, err, errBackslashInCommand)
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
	d.Register("alpha show", nil, "Show alpha")
	d.Register("show bgp rib received", nil, "Show Adj-RIB-In")

	cmds := d.Commands()
	assert.Len(t, cmds, 3)

	// Check all are present
	names := make(map[string]bool)
	for _, cmd := range cmds {
		names[cmd.Name] = true
	}
	assert.True(t, names["daemon shutdown"])
	assert.True(t, names["alpha show"])
	assert.True(t, names["show bgp rib received"])
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
	subs := newSubscriptionManager()

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

// VALIDATES: AC-1 -- CommandContext.Context uses request -> server -> background fallback.
// PREVENTS: request cancellation being dropped before dispatch reaches handlers.
func TestCommandContextContextFallback(t *testing.T) {
	serverCtx, serverCancel := context.WithCancel(t.Context())
	defer serverCancel()

	requestCtx, requestCancel := context.WithCancel(t.Context())
	defer requestCancel()

	srv := &Server{ctx: serverCtx}

	assert.Same(t, requestCtx, (&CommandContext{Server: srv, RequestContext: requestCtx}).Context())
	assert.Same(t, serverCtx, (&CommandContext{Server: srv}).Context())

	backgroundCtx := (&CommandContext{}).Context()
	require.NotNil(t, backgroundCtx)
	assert.NoError(t, backgroundCtx.Err())
}

// VALIDATES: AC-2 -- subsystem dispatch derives child work from the caller context.
// PREVENTS: canceled callers still reaching subsystem RPC handlers through a new root context.
func TestDispatchSubsystemUsesCommandContextContext(t *testing.T) {
	t.Parallel()

	engineSide, subsystemSide := net.Pipe()
	t.Cleanup(func() { _ = engineSide.Close() })
	t.Cleanup(func() { _ = subsystemSide.Close() })

	proc := process.NewProcess(plugin.PluginConfig{Name: "test-subsystem"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	markProcessRunning(t, proc)

	handler := &SubsystemHandler{proc: proc}
	subsystemConn := ipc.NewPluginConn(subsystemSide, subsystemSide)

	readCtx, readCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer readCancel()

	gotRequest := make(chan struct{}, 1)
	readErrCh := make(chan error, 1)
	go func() {
		_, err := subsystemConn.ReadRequest(readCtx)
		if err != nil {
			readErrCh <- err
			return
		}
		gotRequest <- struct{}{}
	}()

	parentCtx, cancelParent := context.WithCancel(context.Background())
	cancelParent()

	resp, err := NewDispatcher().dispatchSubsystem(&CommandContext{RequestContext: parentCtx}, handler, "show version")
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	if !strings.Contains(resp.Error, context.Canceled.Error()) {
		t.Fatalf("expected canceled error in response, got %q", resp.Error)
	}

	select {
	case <-gotRequest:
		t.Fatal("subsystem received a request despite canceled parent context")
	case readErr := <-readErrCh:
		assert.ErrorIs(t, readErr, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("subsystem read did not complete")
	}
}

// VALIDATES: AC-2 -- plugin RPC routing derives timeout contexts from the caller context.
// PREVENTS: plugin execute-command requests surviving caller cancellation because they start from Background.
func TestRouteToProcessUsesParentContextTimeout(t *testing.T) {
	t.Parallel()

	engineSide, pluginSide := net.Pipe()
	t.Cleanup(func() { _ = engineSide.Close() })
	t.Cleanup(func() { _ = pluginSide.Close() })

	proc := process.NewProcess(plugin.PluginConfig{Name: "test-plugin"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	markProcessRunning(t, proc)

	cmd := &RegisteredCommand{
		Name:      "plugin command",
		LowerName: "plugin command",
		Timeout:   time.Second,
		Process:   proc,
	}

	pluginConn := ipc.NewPluginConn(pluginSide, pluginSide)
	readCtx, readCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer readCancel()

	gotRequest := make(chan struct{}, 1)
	readErrCh := make(chan error, 1)
	go func() {
		_, err := pluginConn.ReadRequest(readCtx)
		if err != nil {
			readErrCh <- err
			return
		}
		gotRequest <- struct{}{}
	}()

	parentCtx, cancelParent := context.WithCancel(context.Background())
	cancelParent()

	resp, err := NewDispatcher().routeToProcess(&CommandContext{RequestContext: parentCtx}, cmd, nil, "*")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	if !strings.Contains(resp.Error, context.Canceled.Error()) {
		t.Fatalf("expected canceled error in response, got %q", resp.Error)
	}

	select {
	case <-gotRequest:
		t.Fatal("plugin received a request despite canceled parent context")
	case readErr := <-readErrCh:
		assert.ErrorIs(t, readErr, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("plugin read did not complete")
	}
}

// TestDispatcherPluginMatch verifies plugin command dispatch via the registry.
//
// VALIDATES: Plugin commands are matched by the dispatcher's plugin registry path.
// PREVENTS: Plugin commands unreachable through normal dispatch.
func TestDispatcherPluginMatch(t *testing.T) {
	d := NewDispatcher()

	// Register plugin command with full prefix — plugins that handle
	// commands arriving via update-route RPC must include the verb and domain
	// prefix (e.g., "request bgp watchdog announce", not bare "announce").
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-watchdog"})
	d.Registry().Register(proc, []CommandDef{
		{Name: "request bgp watchdog announce", Description: "Announce watchdog group"},
	})

	// Prefixed command matches (process not running → error, but not ErrUnknownCommand)
	_, err := d.Dispatch(nil, "request bgp watchdog announce dnsr")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrUnknownCommand),
		"plugin command should match, got: %v", err)
}

// TestDispatcherCaseInsensitive verifies case handling.
//
// VALIDATES: Commands are matched case-insensitively.
//
// PREVENTS: Users typing "Alpha Show" failing when "alpha show" works.
func TestDispatcherCaseInsensitive(t *testing.T) {
	d := NewDispatcher()

	called := false
	d.Register("alpha show", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")

	// Should match regardless of case
	_, err := d.Dispatch(nil, "ALPHA SHOW")
	require.NoError(t, err)
	assert.True(t, called)

	called = false
	_, err = d.Dispatch(nil, "Alpha Show")
	require.NoError(t, err)
	assert.True(t, called)
}

// TestDispatchTypedSelectorMissingValue verifies keyworded selectors fail cleanly
// when the selector value is omitted.
func TestDispatchTypedSelectorMissingValue(t *testing.T) {
	d := NewDispatcher()

	d.RegisterWithOptions("show demo name detail", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		t.Fatal("handler should not be called without selector value")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Show demo detail", RegisterOptions{
		ArgDefs: []command.ArgDef{{Name: "name", Kind: command.ArgString, Mandatory: true}},
	})

	ctx := &CommandContext{}
	_, err := d.Dispatch(ctx, "show demo name detail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required argument missing: name")
}

// TestDispatchTypedSelectorExtractsValue verifies generic typed selectors populate
// CommandContext.Selector and still allow trailing handler args.
func TestDispatchTypedSelectorExtractsValue(t *testing.T) {
	d := NewDispatcher()

	var calledName string
	var calledArgs []string
	d.RegisterWithOptions("show demo name detail", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		calledName = ctx.Selector("name")
		calledArgs = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Show demo detail", RegisterOptions{
		ArgDefs: []command.ArgDef{{Name: "name", Kind: command.ArgString, Mandatory: true}},
	})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "show demo name node-1 detail counters")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "node-1", calledName)
	assert.Equal(t, []string{"counters"}, calledArgs)
}

// TestDispatchImplicitSelectorExtractsValue verifies the generic dispatcher can
// carry one positional selector between a resource token and a later action token.
func TestDispatchImplicitSelectorExtractsValue(t *testing.T) {
	d := NewDispatcher()

	var calledSelector string
	var calledArgs []string
	d.RegisterWithOptions("show demo entry detail", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		calledSelector = ctx.Selector("selector")
		calledArgs = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Show entry detail", RegisterOptions{
		RequiresSelector: true,
		ArgDefs:          []command.ArgDef{{Name: "selector", Kind: command.ArgString, Mandatory: true}},
	})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "show demo entry node-1 detail counters")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "node-1", calledSelector)
	assert.Equal(t, []string{"counters"}, calledArgs)
}

// TestDispatchImplicitSelectorMissingValue verifies missing positional selectors
// fail cleanly for generic commands that require them.
func TestDispatchImplicitSelectorMissingValue(t *testing.T) {
	d := NewDispatcher()

	d.RegisterWithOptions("show demo entry detail", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		t.Fatal("handler should not be called without selector")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Show entry detail", RegisterOptions{
		RequiresSelector: true,
		ArgDefs:          []command.ArgDef{{Name: "selector", Kind: command.ArgString, Mandatory: true}},
	})

	ctx := &CommandContext{}
	_, err := d.Dispatch(ctx, "show demo entry detail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a selector")
}

// TestDispatchNoSelectorNeeded verifies commands without selectors dispatch normally.
func TestDispatchNoSelectorNeeded(t *testing.T) {
	d := NewDispatcher()

	d.RegisterWithOptions("show demo brief", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		assert.Empty(t, ctx.Selector("name"))
		assert.Empty(t, ctx.Selector("selector"))
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Show demo brief", RegisterOptions{})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "show demo brief")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
}

// TestForwardToPluginNotRegistered verifies ForwardToPlugin returns error
// when the plugin command is not registered (plugin not running).
//
// VALIDATES: ForwardToPlugin returns wrapped ErrUnknownCommand for missing commands.
// PREVENTS: Silent failures when proxy handlers call ForwardToPlugin before plugin starts.
func TestForwardToPluginNotRegistered(t *testing.T) {
	d := NewDispatcher()

	resp, err := d.ForwardToPlugin(nil, "show bgp rib status", nil, "*")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, ErrUnknownCommand),
		"expected ErrUnknownCommand, got: %v", err)
	assert.Contains(t, err.Error(), "show bgp rib status")
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
		{Name: "show bgp rib status", Description: "RIB summary"},
	})

	// ForwardToPlugin should find the command but fail because process isn't running
	resp, err := d.ForwardToPlugin(nil, "show bgp rib status", nil, "*")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrUnknownCommand),
		"command should be found in registry, got: %v", err)
	// routeToProcess returns ErrPluginProcessNotRunning
	assert.True(t, errors.Is(err, ErrPluginProcessNotRunning),
		"expected ErrPluginProcessNotRunning, got: %v", err)
	assert.Nil(t, resp)
}

// VALIDATES: builtin proxy handlers preserve caller cancellation when forwarding to plugins.
// PREVENTS: ForwardToPlugin re-rooting proxy commands at context.Background().
func TestForwardToPluginUsesParentContext(t *testing.T) {
	t.Parallel()

	engineSide, pluginSide := net.Pipe()
	t.Cleanup(func() { _ = engineSide.Close() })
	t.Cleanup(func() { _ = pluginSide.Close() })

	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-rib"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	markProcessRunning(t, proc)

	d := NewDispatcher()
	d.Registry().Register(proc, []CommandDef{
		{Name: "show bgp rib status", Description: "RIB summary"},
	})

	pluginConn := ipc.NewPluginConn(pluginSide, pluginSide)
	readCtx, readCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer readCancel()

	gotRequest := make(chan struct{}, 1)
	readErrCh := make(chan error, 1)
	go func() {
		_, err := pluginConn.ReadRequest(readCtx)
		if err != nil {
			readErrCh <- err
			return
		}
		gotRequest <- struct{}{}
	}()

	parentCtx, cancelParent := context.WithCancel(context.Background())
	cancelParent()

	resp, err := d.ForwardToPlugin(&CommandContext{RequestContext: parentCtx}, "show bgp rib status", nil, "*")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	if !strings.Contains(resp.Error, context.Canceled.Error()) {
		t.Fatalf("expected canceled error in response, got %q", resp.Error)
	}

	select {
	case <-gotRequest:
		t.Fatal("plugin received a request despite canceled parent context")
	case readErr := <-readErrCh:
		assert.ErrorIs(t, readErr, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("plugin read did not complete")
	}
}

// TestForwardToPluginBuiltinConflict verifies that registering a builtin
// with "show bgp rib status" conflicts with a plugin command "show bgp rib status".
//
// VALIDATES: Builtin proxy "show bgp rib status" blocks plugin registration of same name.
// PREVENTS: Duplicate command name confusion in dispatch.
func TestForwardToPluginBuiltinConflict(t *testing.T) {
	d := NewDispatcher()

	// Register builtin "show bgp rib status" (the proxy handler)
	d.Register("show bgp rib status", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "RIB summary")

	// Plugin tries to register same name -- should be rejected
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-rib"})
	results := d.Registry().Register(proc, []CommandDef{
		{Name: "show bgp rib status", Description: "RIB summary"},
	})
	assert.False(t, results[0].OK, "plugin 'rib status' should conflict with builtin 'rib status'")
	assert.Contains(t, results[0].Error, "conflicts with builtin")
}

// mockAuthorizer implements aaa.Authorizer for testing.
type mockAuthorizer struct {
	allow bool
}

func (m *mockAuthorizer) Authorize(_, _, _ string, _ bool) bool {
	return m.allow
}

// TestDispatcherAuthorizationAllow verifies authorized commands execute.
//
// VALIDATES: AC-1 — Dispatcher permits command when authorizer returns Allow.
// PREVENTS: Authorization blocking all commands.
func TestDispatcherAuthorizationAllow(t *testing.T) {
	d := NewDispatcher()
	d.SetAuthorizer(&mockAuthorizer{allow: true})

	called := false
	d.Register("alpha show", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")

	ctx := &CommandContext{Username: "noc-user"}
	resp, err := d.Dispatch(ctx, "alpha show")
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
	d.SetAuthorizer(&mockAuthorizer{allow: false})

	d.Register("restart", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		t.Fatal("handler should not be called when denied")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")

	ctx := &CommandContext{Username: "noc-user"}
	resp, err := d.Dispatch(ctx, "restart")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, plugin.UnauthorizedMessage)
	assert.Contains(t, resp.Error, "restart", "the denial must name the command it refused")
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

	d.RegisterWithOptions("alpha show", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "", RegisterOptions{ReadOnly: true})

	d.RegisterWithOptions("config set", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "", RegisterOptions{ReadOnly: false})

	ctx := &CommandContext{Username: "user1"}

	// ReadOnly command
	resp, err := d.Dispatch(ctx, "alpha show")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, capturedReadOnly, "alpha show should be ReadOnly=true")

	// Write command
	resp, err = d.Dispatch(ctx, "config set")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.False(t, capturedReadOnly, "config set should be ReadOnly=false")
}

// VALIDATES: AC-8 -- daemon lifecycle commands are write commands for API read-only enforcement.
// PREVENTS: no-auth API callers running daemon reload/shutdown because the daemon prefix was treated as read-only.
func TestIsReadOnlyPathDaemonLifecycle(t *testing.T) {
	assert.True(t, IsReadOnlyPath("show status"), "show status is read-only")

	for _, path := range []string{"request reload", "request shutdown", "request reboot", "request halt"} {
		t.Run(path, func(t *testing.T) {
			assert.False(t, IsReadOnlyPath(path), "%s mutates daemon state", path)
		})
	}
}

// VALIDATES: AC-11 -- Daemon reload through dispatcher emits an audit record with actor, surface, and action.
// PREVENTS: Lifecycle operations bypassing the unified audit trail.
func TestDispatcherDaemonReloadAuditRecord(t *testing.T) {
	d := NewDispatcher()
	d.RegisterWithOptions("request reload", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"result": "ok"}}, nil
	}, "reload", RegisterOptions{})
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	d.SetAuditRecorder(recorder)

	resp, err := d.Dispatch(&CommandContext{
		Username:   "alice",
		RemoteAddr: "192.0.2.10:2222",
		Surface:    audit.SSH,
	}, "request reload")

	require.NoError(t, err)
	require.NotNil(t, resp)
	entries := recorder.Query(audit.Filter{Action: audit.ActionDaemonReload})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "192.0.2.10:2222", entries[0].RemoteAddr)
	assert.Equal(t, audit.SSH, entries[0].Surface)
	assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
}

// readOnlyCapture captures the readOnly argument passed to Authorize.
type readOnlyCapture struct {
	captured *bool
}

func (r *readOnlyCapture) Authorize(_, _, _ string, readOnly bool) bool {
	*r.captured = readOnly
	return true
}

// TestDispatcherAuthorizationUsesUsername verifies Username from context is passed.
//
// VALIDATES: AC-12 — CommandContext.Username passed to authorizer.
// PREVENTS: Authorization using wrong or empty username.
func TestDispatcherAuthorizationUsesUsername(t *testing.T) {
	d := NewDispatcher()

	var capturedUsername string
	d.SetAuthorizer(&usernameCapture{captured: &capturedUsername})

	d.Register("alpha show", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "")

	ctx := &CommandContext{Username: "admin-user"}
	resp, err := d.Dispatch(ctx, "alpha show")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "admin-user", capturedUsername)
}

// usernameCapture captures the username argument passed to Authorize.
type usernameCapture struct {
	captured *string
}

func (u *usernameCapture) Authorize(username, _, _ string, _ bool) bool {
	*u.captured = username
	return true
}

// TestDispatcherWithAuthzStore verifies the authz.Store integrates with the dispatcher.
// This is the wiring test: authz.Store satisfies server.Authorizer and controls dispatch.
//
// VALIDATES: AC-3 — authz.Store plugs into Dispatcher as Authorizer.
// PREVENTS: Type mismatch or interface incompatibility at integration boundary.
func TestDispatcherWithAuthzStore(t *testing.T) {
	store := authz.NewStore()

	// Create a restrictive profile: allow "alpha show", deny everything else
	store.AddProfile(authz.Profile{
		Name: "noc",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "alpha show"},
			},
		},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AssignProfiles("operator", []string{"noc"})

	d := NewDispatcher()
	d.SetAuthorizer(authz.StoreAuthorizer{Store: store})

	showCalled := false
	d.RegisterWithOptions("alpha show", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
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
	resp, err := d.Dispatch(ctx, "alpha show")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, showCalled)

	// Denied command
	resp, err = d.Dispatch(ctx, "restart")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.False(t, restartCalled)

	// Empty username denied when user assignments exist (fail closed)
	noAuthCtx := &CommandContext{Username: ""}
	_, err = d.Dispatch(noAuthCtx, "restart")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))

	// Unassigned user denied when user assignments exist (fail closed)
	unknownCtx := &CommandContext{Username: "unknown-user"}
	_, err = d.Dispatch(unknownCtx, "restart")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
}

// TestDispatcherAuthorizationAppliesToPluginCommands verifies that authorization
// is checked for commands that reach the plugin path rather than a builtin
// handler. The command is registered as a plugin command (not a builtin), so a
// denial here proves the check happens before the dispatcher routes to the
// plugin process, not that the command was simply unrecognized.
//
// VALIDATES: AC-4 — Authorization applies to all command paths, not just builtins.
// PREVENTS: Authorization bypass by sending plugin/subsystem commands, which skip
//
//	the builtin authorization check in Dispatch.
func TestDispatcherAuthorizationAppliesToPluginCommands(t *testing.T) {
	d := NewDispatcher()
	d.SetAuthorizer(&mockAuthorizer{allow: false})

	// Register as a PLUGIN command, so it resolves on the plugin path and is not
	// a builtin. Authorization must still deny it before it routes to the process.
	const pluginCmd = "show custom-probe"
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})
	results := d.registry.Register(proc, []CommandDef{
		{Name: pluginCmd, Description: "Plugin-provided command"},
	})
	require.Len(t, results, 1)
	require.True(t, results[0].OK, "plugin command registration must succeed: %s", results[0].Error)

	ctx := &CommandContext{Username: "noc-user"}
	resp, err := d.Dispatch(ctx, pluginCmd)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, plugin.UnauthorizedMessage)
}

// TestDispatcherUnknownCommandNotReportedAsUnauthorized verifies that a command
// registered nowhere reports ErrUnknownCommand even when the authorizer denies
// everything. Nothing can execute, so there is no authorization to bypass, and
// reporting a denial would send an operator to debug their RBAC profile for what
// is really a typo.
//
// VALIDATES: A denial message means "your profile blocked this", not "no such command".
// PREVENTS: Every typo coming back as "restricted by access control" for read-only
//
//	profiles, whose Edit section denies this path by default.
func TestDispatcherUnknownCommandNotReportedAsUnauthorized(t *testing.T) {
	d := NewDispatcher()
	d.SetAuthorizer(&mockAuthorizer{allow: false})

	ctx := &CommandContext{Username: "noc-user"}
	_, err := d.Dispatch(ctx, "no such command anywhere")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownCommand), "want ErrUnknownCommand, got %v", err)
	assert.False(t, errors.Is(err, ErrUnauthorized), "a command that exists nowhere is not an authorization failure")
}

// fakeAccountant records accounting calls for testing.
type fakeAccountant struct {
	starts       []string // commands that triggered START
	stops        []string // commands that triggered STOP
	startUsers   []string
	startRemotes []string
}

func (f *fakeAccountant) CommandStart(username, remoteAddr, command string) string {
	f.starts = append(f.starts, command)
	f.startUsers = append(f.startUsers, username)
	f.startRemotes = append(f.startRemotes, remoteAddr)
	return "task-1"
}

func (f *fakeAccountant) CommandStop(_, _, _, command string) {
	f.stops = append(f.stops, command)
}

// TestDispatcherAccountingHook verifies that the accounting hook is called on command dispatch.
//
// VALIDATES: AC-8 -- accounting START/STOP records sent around command execution.
// PREVENTS: accounting hook never firing after being wired.
func TestDispatcherAccountingHook(t *testing.T) {
	d := NewDispatcher()
	acct := &fakeAccountant{}
	d.SetAccountingHook(acct)

	d.Register("show version", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"version": "v1.0"}}, nil
	}, "Show version")

	ctx := &CommandContext{Username: "admin", RemoteAddr: "10.0.0.1:12345"}
	resp, err := d.Dispatch(ctx, "show version")

	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, []string{"show version"}, acct.starts, "START should fire before handler")
	assert.Equal(t, []string{"show version"}, acct.stops, "STOP should fire after handler")
}

// TestDispatcherAccountingSkipsNoUsername verifies accounting is skipped for unauthenticated contexts.
//
// VALIDATES: AC-8 -- accounting only fires for authenticated users.
// PREVENTS: sending accounting records with empty username.
func TestDispatcherAccountingSkipsNoUsername(t *testing.T) {
	d := NewDispatcher()
	acct := &fakeAccountant{}
	d.SetAccountingHook(acct)

	d.Register("show version", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Show version")

	// No username -- accounting should be skipped.
	ctx := &CommandContext{}
	_, err := d.Dispatch(ctx, "show version")

	require.NoError(t, err)
	assert.Empty(t, acct.starts, "no START for unauthenticated user")
	assert.Empty(t, acct.stops, "no STOP for unauthenticated user")
}

// TestDispatcherAccountingNilHook verifies commands work without an accounting hook.
//
// VALIDATES: AC-5 -- no accounting hook = no crash, normal operation.
// PREVENTS: nil pointer dereference when accounting is disabled.
func TestDispatcherAccountingNilHook(t *testing.T) {
	d := NewDispatcher()
	// No accounting hook set.

	d.Register("show version", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Show version")

	ctx := &CommandContext{Username: "admin"}
	resp, err := d.Dispatch(ctx, "show version")

	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// TestDispatcherBeginAccounting verifies non-Dispatch command paths can share
// the same accounting hook as normal command dispatch.
//
// VALIDATES: streaming handlers can emit AAA START/STOP records.
// PREVENTS: SSH streaming commands bypassing TACACS+ accounting.
func TestDispatcherBeginAccounting(t *testing.T) {
	d := NewDispatcher()
	acct := &fakeAccountant{}
	d.SetAccountingHook(acct)

	ctx := &CommandContext{Username: "admin", RemoteAddr: "10.0.0.1:12345"}
	stop := d.BeginAccounting(ctx, "monitor event")
	assert.Equal(t, []string{"monitor event"}, acct.starts)
	assert.Equal(t, []string{"admin"}, acct.startUsers)
	assert.Equal(t, []string{"10.0.0.1:12345"}, acct.startRemotes)
	assert.Empty(t, acct.stops)

	stop()
	assert.Equal(t, []string{"monitor event"}, acct.stops)
}

// TestDispatcherArgValidation verifies that valid args pass and invalid args are rejected
// when ArgDefs are configured on a command.
//
// VALIDATES: AC-7, AC-8 -- valid args pass, invalid enum rejected before handler.
func TestDispatcherArgValidation(t *testing.T) {
	d := NewDispatcher()

	var called bool
	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.RegisterWithOptions("show system goroutines", handler, "Show goroutines", RegisterOptions{
		ReadOnly: true,
		ArgDefs: []command.ArgDef{
			{Name: "mode", Kind: command.ArgEnum, EnumValues: []string{"blocked", "full", "summary"}},
		},
	})

	// Valid enum arg.
	called = false
	resp, err := d.Dispatch(nil, "show system goroutines summary")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, called)

	// Invalid enum arg.
	called = false
	resp, err = d.Dispatch(nil, "show system goroutines invalid")
	require.Error(t, err)
	assert.False(t, called)
	assert.Contains(t, resp.Error, "invalid")
}

// TestDispatcherKeywordExtraction verifies keyword-value pair matching.
//
// VALIDATES: AC-13, AC-14 -- keyword-value pairs matched and validated.
func TestDispatcherKeywordExtraction(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.RegisterWithOptions("show ping", handler, "Ping", RegisterOptions{
		ReadOnly: true,
		ArgDefs: []command.ArgDef{
			{Name: "count", Kind: command.ArgUint, UintBits: 32},
			{Name: "dest", Kind: command.ArgString},
		},
	})

	// Valid keyword-value pair.
	resp, err := d.Dispatch(nil, "show ping count 5")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, []string{"count", "5"}, receivedArgs)

	// Missing value after keyword.
	_, err = d.Dispatch(nil, "show ping count")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a value")
}

// TestDispatcherPositionalMatching verifies positional arg validation.
//
// VALIDATES: AC-15 -- unmatched positional args validated against enum types.
func TestDispatcherPositionalMatching(t *testing.T) {
	d := NewDispatcher()

	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.RegisterWithOptions("show system goroutines", handler, "Goroutines", RegisterOptions{
		ReadOnly: true,
		ArgDefs: []command.ArgDef{
			{Name: "mode", Kind: command.ArgEnum, EnumValues: []string{"blocked", "full", "summary"}},
		},
	})

	// Valid positional.
	resp, err := d.Dispatch(nil, "show system goroutines blocked")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// Invalid positional.
	resp, err = d.Dispatch(nil, "show system goroutines invalid")
	require.Error(t, err)
	assert.Contains(t, resp.Error, "invalid")
}

// TestDispatcherMixedArgs verifies mixed positional + keyword args.
//
// VALIDATES: AC-16 -- mixed positional and keyword args parsed correctly.
func TestDispatcherMixedArgs(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.RegisterWithOptions("show ping", handler, "Ping", RegisterOptions{
		ReadOnly: true,
		ArgDefs: []command.ArgDef{
			{Name: "count", Kind: command.ArgUint, UintBits: 32},
			{Name: "dest", Kind: command.ArgString},
			{Name: "timeout", Kind: command.ArgString},
		},
	})

	resp, err := d.Dispatch(nil, "show ping 192.168.1.1 count 5 timeout 3s")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, []string{"192.168.1.1", "count", "5", "timeout", "3s"}, receivedArgs)
}

// TestDispatcherMandatoryMissing verifies that missing mandatory args are rejected.
//
// VALIDATES: AC-17 -- required argument missing rejected.
func TestDispatcherMandatoryMissing(t *testing.T) {
	d := NewDispatcher()

	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.RegisterWithOptions("show tcp-check", handler, "TCP check", RegisterOptions{
		ReadOnly: true,
		ArgDefs: []command.ArgDef{
			{Name: "host", Kind: command.ArgString, Mandatory: true},
			{Name: "port", Kind: command.ArgUint, UintBits: 16, Mandatory: true},
		},
	})

	// Missing mandatory args.
	_, err := d.Dispatch(nil, "show tcp-check")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required argument missing")
}

// TestDispatcherNoArgDefsPassthrough verifies commands without ArgDefs skip validation.
//
// VALIDATES: Commands without ArgDefs skip validation entirely.
func TestDispatcherNoArgDefsPassthrough(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.Register("show version", handler, "Show version")

	resp, err := d.Dispatch(nil, "show version anything goes here")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, []string{"anything", "goes", "here"}, receivedArgs)
}

// TestDispatcherArgValidationUnion verifies union type validation in the dispatcher.
//
// VALIDATES: AC-11, AC-12 -- union accepts enum and uint members.
func TestDispatcherArgValidationUnion(t *testing.T) {
	d := NewDispatcher()

	var called bool
	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.RegisterWithOptions("set system file-descriptors", handler, "Set FD limit", RegisterOptions{
		ArgDefs: []command.ArgDef{
			{
				Name: "limit",
				Kind: command.ArgUnion,
				UnionDefs: []command.ArgDef{
					{Kind: command.ArgUint, UintBits: 64},
					{Kind: command.ArgEnum, EnumValues: []string{"max"}},
				},
				EnumValues: []string{"max"},
			},
		},
	})

	// Enum member.
	called = false
	resp, err := d.Dispatch(nil, "set system file-descriptors max")
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "done", resp.Status)

	// Uint member.
	called = false
	resp, err = d.Dispatch(nil, "set system file-descriptors 1024")
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "done", resp.Status)

	// Invalid.
	called = false
	resp, err = d.Dispatch(nil, "set system file-descriptors invalid")
	require.Error(t, err)
	assert.False(t, called)
	assert.Contains(t, resp.Error, "invalid")
}

// TestDispatcherDuplicateKeywordRejected verifies that duplicate keywords are rejected.
//
// VALIDATES: Review fix -- duplicate keywords like "count 5 count 10" are rejected.
func TestDispatcherDuplicateKeywordRejected(t *testing.T) {
	d := NewDispatcher()

	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.RegisterWithOptions("show ping", handler, "Ping", RegisterOptions{
		ReadOnly: true,
		ArgDefs: []command.ArgDef{
			{Name: "count", Kind: command.ArgUint, UintBits: 32},
			{Name: "dest", Kind: command.ArgString},
		},
	})

	_, err := d.Dispatch(nil, "show ping count 5 count 10")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate keyword")
}

// TestDispatcherPositionalErrorMessage verifies that unmatched positional args
// produce a helpful error listing valid keywords when no enum/union exists.
//
// VALIDATES: Review fix -- positionalError lists keyword names instead of empty enum.
func TestDispatcherPositionalErrorMessage(t *testing.T) {
	d := NewDispatcher()

	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.RegisterWithOptions("show ping", handler, "Ping", RegisterOptions{
		ReadOnly: true,
		ArgDefs: []command.ArgDef{
			{Name: "count", Kind: command.ArgUint, UintBits: 32},
			{Name: "timeout", Kind: command.ArgUint, UintBits: 32},
		},
	})

	resp, err := d.Dispatch(nil, "show ping hello")
	require.Error(t, err)
	assert.Contains(t, resp.Error, "unexpected argument")
	assert.Contains(t, resp.Error, "count")
}

// TestTakesInlineSelector proves the predicate MCP relies on agrees with what
// Dispatch actually accepts, for both polarities.
//
// VALIDATES: TakesInlineSelector is true exactly when matchCommandTokens would
// consume an interior selector token, and Dispatch resolves the spliced form.
// PREVENTS: a command-string BUILDER (MCP) advertising or placing a peer
// selector on a command the dispatcher reads none for -- the defect that made
// every peer-scoped MCP tool emit `peer <sel> show bgp peer detail`.
func TestTakesInlineSelector(t *testing.T) {
	selectorDef := []command.ArgDef{{Name: "selector", Kind: command.ArgString, Mandatory: true}}

	tests := []struct {
		name string
		cmd  *Command
		want bool
	}{
		{"interior selector slot", &Command{Name: "show bgp peer detail", ArgDefs: selectorDef}, true},
		{"two-token key with selector", &Command{Name: "peer update", ArgDefs: selectorDef}, true},
		{"no mandatory string arg", &Command{Name: "show bgp summary"}, false},
		{"single token has no interior slot", &Command{Name: "announce", ArgDefs: selectorDef}, false},
		{"selector name is itself a key token", &Command{Name: "show vpn ipsec peer selector", ArgDefs: selectorDef}, false},
		{"ambiguous: two mandatory string args", &Command{
			Name: "request cache forward",
			ArgDefs: []command.ArgDef{
				{Name: "id", Kind: command.ArgString, Mandatory: true},
				{Name: "selector", Kind: command.ArgString, Mandatory: true},
			},
		}, false},
		{"nil command", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cmd.TakesInlineSelector())
		})
	}

	// The predicate is only useful if the spliced form really dispatches, so
	// drive it through Dispatch rather than trusting the boolean alone.
	d := NewDispatcher()
	var gotSelector string
	d.RegisterWithOptions("show bgp peer detail", func(ctx *CommandContext, _ []string) (*plugin.Response, error) {
		gotSelector = ctx.PeerSelector()
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Peer detail", RegisterOptions{ArgDefs: selectorDef})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "show bgp peer 10.0.0.1 detail")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "10.0.0.1", gotSelector)
}

// TestResolveSinglePeer proves the selector resolution the peer-scoped commands
// share: a configured NAME resolves, an address still resolves, the wildcard and
// every exclusion form are refused by name, and an unmatched or ambiguous
// selector fails with the value quoted.
//
// The two-peer fixture is load-bearing for the exclusion rows. With exactly two
// peers "!edge1" matches exactly one peer, so the ambiguity branch would NOT
// catch it: without the explicit refusal it resolves cleanly and hands the
// caller the WRONG peer. That is the shape the refusal exists to stop, and a
// three-peer fixture would hide it behind an ambiguity error.
//
// VALIDATES: ResolveSinglePeer accepts every selector form the YANG leaf
// advertises ("Peer selector"), returns exactly one peer or a named error, and
// refuses set-valued selectors (`*`, `!<sel>`) rather than resolving them.
// PREVENTS: two regressions. (1) the one this replaced --
// raw/pause/resume/clear-soft/remove calling netip.ParseAddr on the selector, so
// `peer peer1 raw ...` was rejected with "invalid peer address" even though
// peer1 is the configured peer's name. (2) the INVERSION that shipped with it:
// selectorMatchesPeer dropped the exclude flag on the name and ASN arms, so
// `delete bgp peer !edge1` resolved to edge1 -- deleting exactly the peer the
// operator asked to spare.
func TestResolveSinglePeer(t *testing.T) {
	edge1 := netip.MustParseAddr("10.0.0.1")
	edge2 := netip.MustParseAddr("10.0.0.2")
	srv := &Server{reactor: &mockReactor{peers: []plugin.PeerInfo{
		{Address: edge1, Name: "edge1", PeerAS: 65001},
		{Address: edge2, Name: "edge2", PeerAS: 65002},
	}}}

	tests := []struct {
		name     string
		selector string
		want     netip.Addr
		wantErr  error
		errHas   string
	}{
		{"configured peer name", "edge1", edge1, nil, ""},
		{"other peer name", "edge2", edge2, nil, ""},
		{"ip literal still works", "10.0.0.1", edge1, nil, ""},
		{"asn selector", "as65002", edge2, nil, ""},
		{"wildcard refused", "*", netip.Addr{}, errNoSpecificPeer, `"*"`},
		{"empty refused", "", netip.Addr{}, errNoSpecificPeer, `"*"`},
		{"unknown name", "nope", netip.Addr{}, errNoPeerMatchesSelector, `"nope"`},
		// An address that matches no current peer passes through: the handler
		// downstream owns "no such peer". Only a non-address selector must
		// resolve here, since nothing downstream could interpret one.
		{"unknown address passes through", "192.0.2.9", netip.MustParseAddr("192.0.2.9"), nil, ""},
		{"ambiguous glob", "10.0.0.*", netip.Addr{}, errAmbiguousPeerSelector, `"10.0.0.*"`},
		// Exclusion selectors name a SET. Each of these would otherwise resolve
		// to exactly one peer in this two-peer topology -- the wrong one for the
		// name/ASN arms, the complement for the address/glob arms -- and would
		// change meaning the day a third peer is configured.
		{"exclude by name refused", "!edge1", netip.Addr{}, errExcludePeerSelector, `"!edge1"`},
		{"exclude by asn refused", "!as65001", netip.Addr{}, errExcludePeerSelector, `"!as65001"`},
		{"exclude by address refused", "!10.0.0.1", netip.Addr{}, errExcludePeerSelector, `"!10.0.0.1"`},
		{"exclude by glob refused", "!10.0.0.2*", netip.Addr{}, errExcludePeerSelector, `"!10.0.0.2*"`},
		// Parse rejects "!*" outright, so ParseDefault falls back to
		// PeerName("!*") and IsExclude() is false. The raw-string half of the
		// guard is what refuses it with usable advice instead of sending the
		// operator to look for a peer named "!*".
		{"exclude-all refused", "!*", netip.Addr{}, errExcludePeerSelector, `"!*"`},
		{"bare bang refused", "!", netip.Addr{}, errExcludePeerSelector, `"!"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &CommandContext{Server: srv, Peer: tt.selector}
			got, resp, err := ResolveSinglePeer(ctx, "raw")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.NotNil(t, resp)
				assert.Equal(t, plugin.StatusError, resp.Status)
				assert.Contains(t, resp.Error, tt.errHas, "error must quote the offending selector")
				assert.Contains(t, resp.Error, "raw", "error must name the action")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSelectorMatchesPeerPolarity pins the exclude flag on every selector kind
// the helper handles, INCLUDING the two arms Selector.Matches cannot answer.
//
// Why this exists separately from TestResolveSinglePeer: that test now refuses
// exclusion selectors before the helper ever runs, so it can no longer detect an
// inverted arm. The helper still claims, in its own doc comment, to mirror
// cmd/peer's filterPeersBySelectorValue -- and a claim of parity that nothing
// checks is how the inversion got in. Driving the helper directly keeps the
// claim honest for the next caller, who may well want the set semantics.
//
// VALIDATES: selectorMatchesPeer negates the KindName and KindASN comparisons
// when the selector carries "!", and defers to Selector.Matches (which negates
// internally) for the address-shaped kinds.
// PREVENTS: the shipped inversion -- `return p.Name == sel.NameValue()` and
// `return p.PeerAS == sel.ASNValue()` discarding the exclude flag, so "!edge1"
// matched edge1 and "!as65001" matched the peer inside AS 65001.
func TestSelectorMatchesPeerPolarity(t *testing.T) {
	edge1 := plugin.PeerInfo{Address: netip.MustParseAddr("10.0.0.1"), Name: "edge1", PeerAS: 65001}
	edge2 := plugin.PeerInfo{Address: netip.MustParseAddr("10.0.0.2"), Name: "edge2", PeerAS: 65002}

	tests := []struct {
		name      string
		selector  string
		wantEdge1 bool
		wantEdge2 bool
	}{
		{"name includes only its own peer", "edge1", true, false},
		{"excluded name spares its own peer", "!edge1", false, true},
		{"asn includes only its own peer", "as65001", true, false},
		{"excluded asn spares its own peer", "!as65001", false, true},
		{"address includes only its own peer", "10.0.0.1", true, false},
		{"excluded address spares its own peer", "!10.0.0.1", false, true},
		{"glob includes matching peers", "10.0.0.*", true, true},
		{"excluded glob spares matching peers", "!10.0.0.*", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sel := selector.ParseDefault(tt.selector)
			assert.Equal(t, tt.wantEdge1, selectorMatchesPeer(sel, &edge1), "edge1 (as65001)")
			assert.Equal(t, tt.wantEdge2, selectorMatchesPeer(sel, &edge2), "edge2 (as65002)")
		})
	}
}

// TestDispatcherPositionalTypedLeaf covers the FIRST shape the positional
// matcher could not fill: a mandatory leaf whose YANG type is not a string.
//
// VALIDATES: a positional token is offered to every ArgKind, so `show tcp-check
// <host> <port> timeout <t>` fills the uint16 `port` leaf and dispatches.
// PREVENTS: the shipped Phase 2 loop, which tested only ArgEnum/ArgUnion/
// ArgString. The numeric token skipped `port` (uint), landed on the next
// STRING leaf (`source`), and Phase 3 then rejected the fully-formed command
// with "required argument missing: port".
func TestDispatcherPositionalTypedLeaf(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(_ *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	// Same ArgDef shape the YANG produces for `show tcp-check`
	// (internal/plugins/diag/yang/ze-diag-cmd.yang): two mandatory leaves, the
	// second of them a uint16, followed by two optional string leaves.
	d.RegisterWithOptions("show tcp-check", handler, "TCP check", RegisterOptions{
		ReadOnly: true,
		ArgDefs: []command.ArgDef{
			{Name: "host", Kind: command.ArgString, Mandatory: true},
			{Name: "port", Kind: command.ArgUint, UintBits: 16, Mandatory: true},
			{Name: "source", Kind: command.ArgString},
			{Name: "timeout", Kind: command.ArgString},
		},
	})

	resp, err := d.Dispatch(nil, "show tcp-check 127.0.0.1 1 timeout 2s")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, []string{"127.0.0.1", "1", "timeout", "2s"}, receivedArgs,
		"the handler still owns the raw tail; matching must not rewrite it")

	// The mandatory typed leaf is still REQUIRED: filling it positionally must
	// not turn Phase 3 into a rubber stamp.
	receivedArgs = nil
	_, err = d.Dispatch(nil, "show tcp-check 127.0.0.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required argument missing: port")
	assert.Nil(t, receivedArgs)

	// Out-of-range values are still rejected by the leaf's own type.
	_, err = d.Dispatch(nil, "show tcp-check 127.0.0.1 65536")
	require.Error(t, err)
}

// TestDispatcherPositionalPrefersMandatory pins the order the positional
// matcher offers a token in: a required leaf before an optional one.
//
// VALIDATES: an optional leaf that merely accepts the same lexical shape cannot
// starve a mandatory leaf of its value.
// PREVENTS: first-fit-in-declaration-order, which lets the optional `source`
// string swallow a token the mandatory `port` needed and then reports the
// command as incomplete.
func TestDispatcherPositionalPrefersMandatory(t *testing.T) {
	d := NewDispatcher()

	handler := func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	// `source` is declared BEFORE the mandatory `port` and accepts any string,
	// so declaration order alone would bind the numeric token to it.
	d.RegisterWithOptions("show demo probe", handler, "Probe", RegisterOptions{
		ReadOnly: true,
		ArgDefs: []command.ArgDef{
			{Name: "source", Kind: command.ArgString},
			{Name: "port", Kind: command.ArgUint, UintBits: 16, Mandatory: true},
		},
	})

	resp, err := d.Dispatch(nil, "show demo probe 179")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchTerminalNounSelector covers the SECOND shape: a command whose own
// noun is the LAST key token, so its selector can only arrive as a trailing
// positional.
//
// VALIDATES: `delete bgp peer <selector>` reaches its handler with the selector
// applied to the command context.
// PREVENTS: the shipped guard. matchCommandTokens fills only INTERIOR selector
// slots (it needs a later key token to stop at), so a terminal noun yielded no
// selector at all and RequiresSelector rejected the documented form outright
// with "delete bgp peer requires a selector" -- the root cause of
// test/plugin/api-peer-remove.ci.
func TestDispatchTerminalNounSelector(t *testing.T) {
	d := NewDispatcher()

	var gotPeer string
	var gotArgs []string
	handler := func(ctx *CommandContext, args []string) (*plugin.Response, error) {
		gotPeer = ctx.PeerSelector()
		gotArgs = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	d.RegisterWithOptions("delete bgp peer", handler, "Remove a peer", RegisterOptions{
		RequiresSelector: true,
		ArgDefs:          []command.ArgDef{{Name: "selector", Kind: command.ArgString, Mandatory: true}},
	})

	ctx := &CommandContext{}
	resp, err := d.Dispatch(ctx, "delete bgp peer 127.0.0.1")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "127.0.0.1", gotPeer, "the trailing positional must reach the handler as the peer selector")
	assert.Equal(t, "127.0.0.1", ctx.Selector("selector"))
	assert.Equal(t, []string{"127.0.0.1"}, gotArgs,
		"the tail is still handed to the handler unchanged")

	// A configured peer NAME is a selector too, not only an address.
	ctx = &CommandContext{}
	_, err = d.Dispatch(ctx, "delete bgp peer edge1")
	require.NoError(t, err)
	assert.Equal(t, "edge1", gotPeer)
}

// TestDispatchTerminalNounSelectorBoundaries fences the trailing-positional
// selector so it cannot swallow a payload the handler owns.
//
// VALIDATES: the selector is adopted ONLY from a lone trailing token, only for
// a command that requires a selector, and never over one supplied out of band.
// PREVENTS: the greedy reading of the same fix. `announce` is a single-token
// command carrying `leaf selector mandatory`
// (internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang:13)
// whose real arguments are a multi-token route; binding its FIRST positional
// would silently announce to a peer named "unicast".
func TestDispatchTerminalNounSelectorBoundaries(t *testing.T) {
	selectorDefs := []command.ArgDef{{Name: "selector", Kind: command.ArgString, Mandatory: true}}

	t.Run("multi-token payload is not a selector", func(t *testing.T) {
		d := NewDispatcher()
		d.RegisterWithOptions("announce", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
			t.Fatal("handler must not run without a selector")
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}, "Announce", RegisterOptions{RequiresSelector: true, ArgDefs: selectorDefs})

		_, err := d.Dispatch(&CommandContext{}, "announce unicast 10.0.0.0/24 next-hop 192.0.2.1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a selector")
	})

	t.Run("no trailing token still reports the selector as missing", func(t *testing.T) {
		d := NewDispatcher()
		d.RegisterWithOptions("delete bgp peer", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
			t.Fatal("handler must not run without a selector")
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}, "Remove a peer", RegisterOptions{RequiresSelector: true, ArgDefs: selectorDefs})

		_, err := d.Dispatch(&CommandContext{}, "delete bgp peer")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a selector")
	})

	t.Run("an out-of-band selector wins over the trailing token", func(t *testing.T) {
		d := NewDispatcher()
		var gotPeer string
		var gotArgs []string
		d.RegisterWithOptions("peer raw", func(ctx *CommandContext, args []string) (*plugin.Response, error) {
			gotPeer = ctx.PeerSelector()
			gotArgs = args
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}, "Raw bytes", RegisterOptions{RequiresSelector: true, ArgDefs: selectorDefs})

		ctx := &CommandContext{Peer: "10.0.0.9"}
		_, err := d.Dispatch(ctx, "peer raw deadbeef")
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.9", gotPeer, "the caller-supplied selector must not be replaced by the payload")
		assert.Equal(t, []string{"deadbeef"}, gotArgs)
	})

	t.Run("a command that needs no selector is untouched", func(t *testing.T) {
		d := NewDispatcher()
		var gotPeer string
		d.RegisterWithOptions("show demo record", func(ctx *CommandContext, _ []string) (*plugin.Response, error) {
			gotPeer = ctx.PeerSelector()
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}, "Record", RegisterOptions{ReadOnly: true, ArgDefs: selectorDefs})

		ctx := &CommandContext{}
		_, err := d.Dispatch(ctx, "show demo record whatever")
		require.NoError(t, err)
		assert.Equal(t, "*", gotPeer, "no RequiresSelector means no trailing-positional adoption")
	})
}

// noopHandler answers every dispatch-guard test that only cares about routing.
func noopHandler(_ *CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusDone}, nil
}

// TestMatchBuiltinRefusesWhenLongerPathMatches verifies the longer-path rule
// that matchBuiltinTokens applies before it serves a match.
//
// VALIDATES: longerCommandPath sees a registered path past the tokens the
// parent claimed, and stops seeing one once the parent claimed them all.
//
// PREVENTS: a command registered at a short path swallowing a child another
// owner registered, which is how `show bgp rpki status` would reach
// handleBgpSummary and be rejected as an address family.
//
// A builtin child always sorts ahead of its parent, because its key is the
// parent key plus a suffix and updateSortedKeys orders by length. The rule is
// therefore exercised here at the guard, and end to end against plugin names in
// TestShowBgpDoesNotSwallowPluginSubcommands.
func TestMatchBuiltinRefusesWhenLongerPathMatches(t *testing.T) {
	d := NewDispatcher()
	d.Register("show demo", noopHandler, "Demo")
	d.Register("show demo detail", noopHandler, "Demo detail")

	tokens := []string{"show", "demo", "detail"}
	assert.True(t, d.longerCommandPath(tokens, 2), "the child path must be visible past the parent match")
	assert.False(t, d.longerCommandPath(tokens, 3), "an exhausted input leaves no longer path to find")

	upper := []string{"SHOW", "DEMO", "DETAIL"}
	assert.True(t, d.longerCommandPath(upper, 2), "registration keys are lowercase, so the walk must lower its tokens")
}

// TestMatchBuiltinServesWhenNoLongerPathMatches verifies the guard refuses
// nothing when the tokens reach no registered command past the match.
//
// VALIDATES: an ordinary command still resolves, with and without a sibling
// registered elsewhere in the tree.
//
// PREVENTS: R-1, a guard eager enough to break every command on the dispatch
// path it sits on.
func TestMatchBuiltinServesWhenNoLongerPathMatches(t *testing.T) {
	d := NewDispatcher()
	d.Register("show demo", noopHandler, "Demo")
	d.Register("show other detail", noopHandler, "Other detail")

	cmd, args, _, ok := d.matchBuiltinTokens([]string{"show", "demo"})
	require.True(t, ok, "an exact match must be served")
	require.NotNil(t, cmd)
	assert.Equal(t, "show demo", cmd.Name)
	assert.Empty(t, args)

	cmd, args, _, ok = d.matchBuiltinTokens([]string{"show", "other", "detail"})
	require.True(t, ok, "a longer key that matches must be served by the walk itself")
	require.NotNil(t, cmd)
	assert.Equal(t, "show other detail", cmd.Name)
	assert.Empty(t, args)
}

// TestMatchBuiltinKeepsArgumentsForLeftoverValues verifies AC-4.
//
// VALIDATES: leftover tokens that name no registered command still reach the
// handler as arguments.
//
// PREVENTS: the naive rule the spec rejects, refusing any match that leaves
// tokens over. Leftovers are how every argument-taking command works, so that
// rule would break `show bgp summary ipv4`.
func TestMatchBuiltinKeepsArgumentsForLeftoverValues(t *testing.T) {
	d := NewDispatcher()
	d.Register("show demo summary", noopHandler, "Demo summary")

	cmd, args, _, ok := d.matchBuiltinTokens([]string{"show", "demo", "summary", "ipv4"})
	require.True(t, ok, "a value argument must not be read as a command path")
	require.NotNil(t, cmd)
	assert.Equal(t, "show demo summary", cmd.Name)
	assert.Equal(t, []string{"ipv4"}, args)

	cmd, args, _, ok = d.matchBuiltinTokens([]string{"show", "demo", "summary", "ipv4", "unicast"})
	require.True(t, ok, "two value arguments must not be read as a command path")
	require.NotNil(t, cmd)
	assert.Equal(t, []string{"ipv4", "unicast"}, args)
}

// TestShowBgpDoesNotSwallowPluginSubcommands verifies AC-2 and assumption A-1.
//
// VALIDATES: the guard sees PLUGIN-registered names, so a builtin `show bgp`
// refuses the match for each of the four plugin subtrees and the plugin
// fallback in Dispatch answers instead.
//
// PREVENTS: R-2. A guard reading d.commands alone passes every builtin test and
// still sends `show bgp rpki status` to handleBgpSummary, which rejects "rpki"
// as an address family.
func TestShowBgpDoesNotSwallowPluginSubcommands(t *testing.T) {
	d := NewDispatcher()
	d.Register("show bgp", noopHandler, "BGP overview")
	d.Register("show bgp summary", noopHandler, "BGP summary")

	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-subtrees"})
	results := d.Registry().Register(proc, []CommandDef{
		{Name: "show bgp rpki status"},
		{Name: "show bgp rpki roa"},
		{Name: "show bgp rs status"},
		{Name: "show bgp rs peers"},
		{Name: "show bgp adj-rib-in"},
		{Name: "show bgp adj-rib-in status"},
		{Name: "show bgp healthcheck"},
	})
	for _, result := range results {
		require.True(t, result.OK, "%s: %s", result.Name, result.Error)
	}

	plugins := []string{
		"show bgp rpki status",
		"show bgp rs peers",
		"show bgp adj-rib-in",
		"show bgp healthcheck",
		"show bgp rpki roa 10.0.0.0/24",
	}
	for _, input := range plugins {
		t.Run(input, func(t *testing.T) {
			_, _, _, ok := d.matchBuiltinTokens(strings.Fields(input))
			assert.False(t, ok, "the builtin parent must refuse a path the plugin registry owns")

			_, err := d.Dispatch(nil, input)
			require.Error(t, err, "the plugin process is not running in this test")
			assert.ErrorIs(t, err, ErrPluginProcessNotRunning,
				"the command must reach the plugin route, not the builtin handler")
		})
	}

	cmd, args, _, ok := d.matchBuiltinTokens([]string{"show", "bgp"})
	require.True(t, ok, "the bare parent must still resolve")
	assert.Equal(t, "show bgp", cmd.Name)
	assert.Empty(t, args)

	cmd, args, _, ok = d.matchBuiltinTokens([]string{"show", "bgp", "summary", "ipv4"})
	require.True(t, ok, "a family argument must still reach the summary handler")
	assert.Equal(t, "show bgp summary", cmd.Name)
	assert.Equal(t, []string{"ipv4"}, args)
}

// TestGuardSeesSubsystemCommands verifies the third registry the guard reads.
//
// VALIDATES: a subsystem command below a builtin parent refuses the parent
// match, and a subsystem command that only shares a prefix does not.
//
// PREVENTS: a forked subsystem's command being swallowed by a parent container
// the way the plugin subtrees were.
func TestGuardSeesSubsystemCommands(t *testing.T) {
	d := NewDispatcher()
	d.Register("show demo", noopHandler, "Demo")

	manager := NewSubsystemManager()
	manager.handlers["forked"] = &SubsystemHandler{commands: []string{"show demo trace"}}

	assert.False(t, d.longerCommandPath([]string{"show", "demo", "trace"}, 2),
		"an empty subsystem manager declares no command below the parent")

	d.setSubsystems(manager)
	assert.True(t, d.longerCommandPath([]string{"show", "demo", "trace"}, 2),
		"a subsystem command below the parent must refuse the parent match")
	assert.False(t, d.longerCommandPath([]string{"show", "demo", "tracer"}, 2),
		"the subsystem comparison is exact, never a prefix")
}

// TestShowBgpSummaryStillResolvesToItsOwnHandler verifies AC-1, AC-3, AC-4 and
// AC-5 against the real YANG command tree.
//
// VALIDATES: assumption A-3. `container bgp` now carries a ze:command and still
// has children, the shape `show ospf` already had, and every path under it
// resolves to the command registered at that path.
//
// PREVENTS: the parent command stealing `show bgp summary`, and the guard
// refusing a parent whose leftover token is a value.
func TestShowBgpSummaryStillResolvesToItsOwnHandler(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load YANG")

	wireToPaths := yang.WireMethodToPaths(loader)
	assert.Contains(t, wireToPaths["ze-bgp:overview"], "show bgp", "the container command must produce the bare path")
	assert.Contains(t, wireToPaths["ze-bgp:summary"], "show bgp summary")

	d := NewDispatcher()
	loadBuiltinsWithAliases(d, wireToPaths, yang.PathToDescription(loader),
		yang.PathToArgDefs(loader), yang.BuildCommandTree(loader))

	cases := []struct {
		input string
		key   string
		args  []string
	}{
		{input: "show bgp", key: "show bgp"},
		{input: "show bgp summary", key: "show bgp summary"},
		{input: "show bgp summary ipv4", key: "show bgp summary", args: []string{"ipv4"}},
		{input: "show bgp peer list", key: "show bgp peer list"},
		{input: "show ospf", key: "show ospf"},
		{input: "show ospf instance", key: "show ospf instance"},
		{input: "show system ntp", key: "show system ntp"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			cmd, args, _, ok := d.matchBuiltinTokens(strings.Fields(tc.input))
			require.True(t, ok, "must resolve to a builtin")
			require.NotNil(t, cmd)
			assert.Equal(t, tc.key, cmd.Name, "must resolve to the command registered at its own path")
			if len(tc.args) == 0 {
				assert.Empty(t, args)
				return
			}
			assert.Equal(t, tc.args, args)
		})
	}
}

// unguardedMatch repeats the longest-prefix walk matchBuiltinTokens performs,
// with the longer-path guard left out. It is the control the AC-7 sweep
// compares against. A guard that changed an answer it must not change is then
// visible as a difference rather than as an absence.
func unguardedMatch(d *Dispatcher, tokens []string) (*Command, []string) {
	for _, key := range d.sortedKeys {
		cmd := d.commands[key]
		if cmd == nil {
			continue
		}
		args, _, ok := matchCommandTokens(tokens, key, cmd.ArgDefs)
		if ok {
			return cmd, args
		}
	}
	return nil, nil
}

// TestNoArgTakingKeyIsAPrefixOfAnotherPath verifies AC-7 and assumption A-2 over
// every dispatcher key the real YANG tree produces.
//
// Nine argument-taking keys ARE strict prefixes of a longer path, `show route`
// and `show route lookup` among them. The collision is handled rather than
// absent. The guard tests for a registered PATH. So a value typed where the
// longer path expects a keyword resolves exactly as it did before the guard.
//
// VALIDATES: for every such key, the guarded match and the unguarded match agree
// once the leftover token names no command.
//
// PREVENTS: R-3, a parent elsewhere in the tree that already relies on keeping a
// value the guard CAN mistake for a child command.
func TestNoArgTakingKeyIsAPrefixOfAnotherPath(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load YANG")

	pathToArgDefs := yang.PathToArgDefs(loader)
	d := NewDispatcher()
	loadBuiltinsWithAliases(d, yang.WireMethodToPaths(loader), yang.PathToDescription(loader),
		pathToArgDefs, yang.BuildCommandTree(loader))

	keys := make([]string, 0, len(d.commands))
	for key := range d.commands {
		keys = append(keys, key)
	}
	require.Greater(t, len(keys), 100, "the builtin surface must be loaded, or this test proves nothing")

	// A token no YANG container can be named, standing for the value an operator
	// types where an argument-taking key meets a longer registered path.
	const value = "zz-not-a-command"

	collisions := 0
	for _, key := range keys {
		if len(pathToArgDefs[key]) == 0 {
			continue
		}
		for _, other := range keys {
			if !strings.HasPrefix(other, key+" ") {
				continue
			}
			collisions++
			tokens := append(strings.Fields(key), value) //nolint:gocritic // a fresh slice per case is the point
			assert.False(t, d.longerCommandPath(tokens, len(strings.Fields(key))),
				"%q: a value must not be read as a step toward %q", key, other)

			wantCmd, wantArgs := unguardedMatch(d, tokens)
			gotCmd, gotArgs, _, ok := d.matchBuiltinTokens(tokens)
			if wantCmd == nil {
				assert.False(t, ok, "%q: the guard must not invent a match", key)
				continue
			}
			require.True(t, ok, "%q: the guard refused a match the walk alone serves", key)
			assert.Same(t, wantCmd, gotCmd, "%q: the guard must not change which command answers", key)
			assert.Equal(t, wantArgs, gotArgs, "%q: the guard must not change the arguments", key)
		}
	}
	assert.Positive(t, collisions, "the sweep found no collision, so it proved nothing")
}

// BenchmarkMatchBuiltinTokens measures assumption A-4, the guard's cost on the
// dispatch path, over the real builtin key set.
//
// The exact-match case pays nothing: the guard returns before its first
// comparison. The leftover-token case walks one path per spare token.
func BenchmarkMatchBuiltinTokens(b *testing.B) {
	loader, err := yang.DefaultLoader()
	require.NoError(b, err, "load YANG")

	d := NewDispatcher()
	loadBuiltinsWithAliases(d, yang.WireMethodToPaths(loader), yang.PathToDescription(loader),
		yang.PathToArgDefs(loader), yang.BuildCommandTree(loader))

	inputs := map[string][]string{
		"exact":    {"show", "bgp", "summary"},
		"one-arg":  {"show", "bgp", "summary", "ipv4"},
		"two-args": {"show", "bgp", "summary", "ipv4", "unicast"},
	}
	for name, tokens := range inputs {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, _, _, ok := d.matchBuiltinTokens(tokens); !ok {
					b.Fatal("no match")
				}
			}
		})
	}

	// The guard alone, which is the cost this change adds. The whole-match
	// numbers above cannot isolate it. sortedKeys ties resolve in map order, so
	// how many keys the walk tries before its match varies between processes.
	for name, tokens := range inputs {
		b.Run("guard/"+name, func(b *testing.B) {
			consumed := 3
			b.ReportAllocs()
			for range b.N {
				if d.longerCommandPath(tokens, consumed) {
					b.Fatal("no longer path exists for these tokens")
				}
			}
		})
	}
}
