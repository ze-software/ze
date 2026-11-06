// Design: docs/architecture/config/transaction-protocol.md -- nothing applied means nothing compensated.
// Related: reload_startup_authority_test.go -- the refused reload that did move the tree.

package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestReloadRefusedBeforeApplyNeverReachesTheReactor is the contract a refused
// verify owes the running daemon.
//
// GOAL: a reload a plugin refused at verify applies nothing, so it undoes nothing.
// METHOD: a mock reactor recording every write; a plugin answering verify with an
// error; the plain ReloadConfig entry point, whose scope rejects itself on return.
// VALIDATES: neither ApplyConfigDiff nor SetConfigTree ran, and no scope stayed
// the owner of the running configuration.
// PREVENTS: a refused reload reconciling the reactor against the prior tree.
func TestReloadRefusedBeforeApplyNeverReachesTheReactor(t *testing.T) {
	running := map[string]any{"bgp": map[string]any{"router-id": "192.0.2.1"}}
	reactor := &mockReloadReactor{tree: running}
	plugins := []pluginDef{{
		name: "rib", roots: []string{"bgp"},
		verifyResp: &rpc.ConfigVerifyOutput{Status: plugin.StatusError, Error: "refused by test"},
	}}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), map[string]any{"bgp": map[string]any{"router-id": "192.0.2.2"}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "refused by test")
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	assert.Nil(t, reactor.applyTree, "a refused reload must not reconcile the reactor")
	assert.Nil(t, reactor.setTree, "a refused reload must not republish the running tree")
	assert.Nil(t, s.txLock.configOwner, "a refused reload must not own the running configuration")
}

// TestReloadRefusalKeepsTheScopesLastAppliedTree holds the second half of the
// same contract, for a scope that already applied one tree.
//
// GOAL: a later refused reload in that scope leaves the record naming the tree
// that really was applied, so rejection undoes that one.
// METHOD: one deferred acceptance over two reloads: the first applies, the second
// fails auto-load on a missing dependency, having moved nothing.
// VALIDATES: the record's after is the applied tree, and rejecting the scope still
// restores the original.
// PREVENTS: an early record overwriting after with a candidate nothing applied,
// which decomposes the undo against roots that never changed.
func TestReloadRefusalKeepsTheScopesLastAppliedTree(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:         "refusal-auto-load-owner",
		Description:  "needs a dependency nothing provides",
		ConfigRoots:  []string{"autopath"},
		Dependencies: []string{"missing-dep"},
		RunEngine:    func(net.Conn) int { return 0 },
		CLIHandler:   func([]string) int { return 0 },
	}))

	s, _ := newLifecycleStartupServer(t)
	reactor := &createdPeersReactor{}
	reactor.tree = map[string]any{"revision": 1}
	s.reactor = reactor

	ctx, finish := s.DeferReloadAcceptance(t.Context())
	defer finish(false) //nolint:errcheck // Cleanup; the completion outcome is asserted below.
	pending, ok := ctx.Value(reloadAcceptanceKey{}).(*reloadAcceptance)
	require.True(t, ok, "the deferred acceptance carries its reload scope")

	applied := map[string]any{"revision": 2}
	require.NoError(t, s.ReloadConfig(ctx, applied))
	require.Error(t, s.ReloadConfig(ctx, map[string]any{"revision": 3, "autopath": map[string]any{"enabled": "true"}}))

	pending.mu.Lock()
	undo := pending.compensation
	pending.mu.Unlock()
	require.NotNil(t, undo, "the first reload applied, so its scope owes a compensation")
	assert.Equal(t, applied, undo.after, "only an applied tree is what a rejection undoes")

	require.NoError(t, finish(false))
	assert.Equal(t, map[string]any{"revision": 1}, reactor.GetConfigTree(), "rejection restores the original")
}

// TestReloadAbandonedHandshakeOfAFatalPluginKeepsTheDaemon drives a reload whose
// context ends during a fatal plugin's handshake, through the real entry point.
//
// GOAL: a reload that outlived its caller is refused, and the daemon stays up.
// METHOD: a plugin speaking the wire itself declares fatal at Stage 1 and never
// answers Stage 2, so only the context can end the handshake. The plugin exits
// when the rollback closes its connection, with no timer in the test.
// VALIDATES: the reload fails naming the cancellation, ze did not stop, the plugin
// is gone from the manager, and the running tree was not written.
// PREVENTS: a client disconnect or a 30-second budget stopping a healthy daemon.
func TestReloadAbandonedHandshakeOfAFatalPluginKeepsTheDaemon(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "fatal-abandoned-handshake"
	configureArrived := make(chan struct{})
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "declares fatal and never answers configure",
		ConfigRoots: []string{"abandonroot"},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			mux := rpc.NewMuxConn(rpc.NewConn(conn, conn))
			defer mux.Close() //nolint:errcheck // The engine already closed its end.
			if _, err := mux.CallRPC(context.Background(), rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				FailurePolicy:   rpc.FailureFatal,
			}); err != nil {
				return 1
			}
			<-mux.Requests() // Stage 2 configure, which is never answered.
			close(configureArrived)
			<-mux.Done()
			return 0
		},
	}))

	s, spawner := newLifecycleStartupServer(t)
	reactor := &mockReloadReactor{tree: map[string]any{}}
	s.reactor = reactor
	_, stopped := watchDaemonStop(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-configureArrived
		cancel()
	}()

	err := s.ReloadConfig(ctx, map[string]any{"abandonroot": map[string]any{"enabled": "true"}})

	require.ErrorIs(t, err, context.Canceled, "the reload names why it ended")
	assert.False(t, stopped(), "a reload its caller abandoned must not stop the daemon")
	assert.Nil(t, spawner.pm.GetProcess(name), "the plugin the abandoned reload started must be stopped")
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	assert.Nil(t, reactor.setTree, "the abandoned reload must not write the running tree")
}

// registerHeldSibling registers a plugin that connects and never declares, so it
// holds its startup tier at Stage 1 until the context ends. Declaring nothing, its
// own end stops nobody.
func registerHeldSibling(t *testing.T, name, root string) {
	t.Helper()
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "connects and holds its tier",
		ConfigRoots: []string{root},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			mux := rpc.NewMuxConn(rpc.NewConn(conn, conn))
			defer mux.Close() //nolint:errcheck // The engine already closed its end.
			<-mux.Done()
			return 0
		},
	}))
}

// TestFatalPluginThatDiesAtTheBarrierStopsTheDaemon drives the failure no caller
// reports: a fatal plugin passes Stage 1 and exits inside the tier barrier, whose
// wait watches no connection, while a sibling holds the tier until the reload ends.
//
// GOAL: ze honors the plugin's fatal declaration although only the context ended
// the wait.
// METHOD: real plugins over the real entry point. The test cancels only after the
// plugin's engine has returned, so the death precedes the end of the wait.
// VALIDATES: the daemon stops, and the reload's error names the plugin's death
// rather than the bystander's abort.
// PREVENTS: a dead fatal plugin read as a barrier abort and excused, which leaves
// the daemon up without the plugin it was told it could not run without.
func TestFatalPluginThatDiesAtTheBarrierStopsTheDaemon(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "fatal-dies-at-barrier"
	declared := make(chan struct{})
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "declares fatal and exits once answered",
		ConfigRoots: []string{"barrierdeadroot"},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			mux := rpc.NewMuxConn(rpc.NewConn(conn, conn))
			defer mux.Close() //nolint:errcheck // The engine already closed its end.
			defer close(declared)
			if _, err := mux.CallRPC(context.Background(), rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				FailurePolicy:   rpc.FailureFatal,
			}); err != nil {
				return 1
			}
			return 0
		},
	}))
	// The name sorts after the plugin's: a phase reports the first diagnosis in
	// name order, and this sibling's own is the cancellation.
	registerHeldSibling(t, "held-sibling-of-the-dead", "barrierdeadsibling")

	s, spawner := newLifecycleStartupServer(t)
	s.reactor = &mockReloadReactor{tree: map[string]any{}}
	_, stopped := watchDaemonStop(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-declared
		if proc := spawner.pm.GetProcess(name); proc != nil {
			<-proc.EngineDone()
		}
		cancel()
	}()

	err := s.ReloadConfig(ctx, map[string]any{
		"barrierdeadroot":    map[string]any{"enabled": "true"},
		"barrierdeadsibling": map[string]any{"enabled": "true"},
	})

	require.ErrorIs(t, err, errStartupPluginEnded, "the reload names the plugin's death over the bystander's abort")
	assert.True(t, stopped(), "a fatal plugin that died at the barrier must stop the daemon")
}

// TestFatalPluginAliveAtTheBarrierKeepsTheDaemonWhenItsCallerEnds is the polarity
// of the test above: the same barrier and sibling, with the plugin still alive.
//
// GOAL: a reload its caller abandoned does not stop the daemon through a healthy
// plugin that was only waiting for its tier.
// METHOD: the plugin declares fatal and stays connected, then the context ends.
// VALIDATES: the reload fails, and ze keeps running.
// PREVENTS: a liveness check that reads every barrier abort as a death.
func TestFatalPluginAliveAtTheBarrierKeepsTheDaemonWhenItsCallerEnds(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	declared := make(chan struct{})
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "fatal-alive-at-barrier",
		Description: "declares fatal and stays connected",
		ConfigRoots: []string{"barrieraliveroot"},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			mux := rpc.NewMuxConn(rpc.NewConn(conn, conn))
			defer mux.Close() //nolint:errcheck // The engine already closed its end.
			if _, err := mux.CallRPC(context.Background(), rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				FailurePolicy:   rpc.FailureFatal,
			}); err != nil {
				return 1
			}
			close(declared)
			<-mux.Done()
			return 0
		},
	}))
	registerHeldSibling(t, "barrier-alive-sibling", "barrieralivesibling")

	s, _ := newLifecycleStartupServer(t)
	s.reactor = &mockReloadReactor{tree: map[string]any{}}
	_, stopped := watchDaemonStop(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-declared
		cancel()
	}()

	err := s.ReloadConfig(ctx, map[string]any{
		"barrieraliveroot":    map[string]any{"enabled": "true"},
		"barrieralivesibling": map[string]any{"enabled": "true"},
	})

	require.Error(t, err)
	assert.NotErrorIs(t, err, errStartupPluginEnded, "the plugin was alive when the wait ended")
	assert.False(t, stopped(), "a healthy plugin at the barrier must not stop the daemon when its caller ends")
}

// TestSiblingRefusingConfigDoesNotStopTheDaemonThroughAFatalPlugin drives the
// operator's typo in a reload under a context that stays live.
//
// GOAL: a plugin that refuses its configuration refuses the reload, and a healthy
// fatal plugin in its tier, aborted at the barrier for it, stops nothing.
// METHOD: two real SDK plugins in one tier, one declaring fatal, one refusing.
// VALIDATES: the reload names the refusal, ze keeps running, and both are gone.
// PREVENTS: the barrier abort standing for the fatal plugin's own failure, which
// let a config typo kill a running daemon.
func TestSiblingRefusingConfigDoesNotStopTheDaemonThroughAFatalPlugin(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const fatalName, refusingName = "fatal-bystander", "config-refuser"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        fatalName,
		Description: "declares fatal and accepts its configuration",
		ConfigRoots: []string{"bystanderroot"},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(fatalName, conn)
			if err := p.Run(context.Background(), &sdk.Registration{FailurePolicy: rpc.FailureFatal}); err != nil {
				return 1
			}
			return 0
		},
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:        refusingName,
		Description: "refuses its configuration",
		ConfigRoots: []string{"refuserroot"},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(refusingName, conn)
			p.OnConfigure(func([]sdk.ConfigSection) error { return errors.New("a typo in this config") })
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
	}))

	s, spawner := newLifecycleStartupServer(t)
	s.reactor = &mockReloadReactor{tree: map[string]any{}}
	_, stopped := watchDaemonStop(s)

	err := s.ReloadConfig(context.Background(), map[string]any{
		"bystanderroot": map[string]any{"enabled": "true"},
		"refuserroot":   map[string]any{"enabled": "true"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "a typo in this config", "the reload names the refusal")
	assert.False(t, stopped(), "a sibling's config refusal must not stop the daemon through a fatal plugin")
	assert.Nil(t, spawner.pm.GetProcess(fatalName), "the failed tier is rolled back")
	assert.Nil(t, spawner.pm.GetProcess(refusingName), "the failed tier is rolled back")
}

// TestReloadSectionFailureStopsItsAutoLoadedPlugin holds the refusal between a
// successful auto-load and the transaction.
//
// GOAL: a reload that cannot build a plugin's config sections leaves running only
// what it found.
// METHOD: a value JSON cannot encode, under a root another plugin wants, beside a
// root whose plugin auto-loads.
// VALIDATES: the reload names the root, and the auto-loaded plugin is gone.
// PREVENTS: a plugin started for a refused reload running on with a config the
// daemon never applied.
func TestReloadSectionFailureStopsItsAutoLoadedPlugin(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "section-failure-auto-loaded"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "auto-loaded and healthy",
		ConfigRoots: []string{"sectionautoroot"},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
	}))

	s, spawner := newLifecycleStartupServer(t)
	s.reactor = &mockReloadReactor{tree: map[string]any{}}
	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.StartWithContext(s.ctx))
	wanting := process.NewProcess(plugin.PluginConfig{Name: "section-failure-wanter"})
	wanting.SetRegistration(&plugin.PluginRegistration{WantsConfigRoots: []string{"sectionbadroot"}})
	pm.AddProcess("section-failure-wanter", wanting)
	s.procManager.Store(pm)
	spawner.pm = pm

	err := s.ReloadConfig(context.Background(), map[string]any{
		"sectionautoroot": map[string]any{"enabled": "true"},
		"sectionbadroot":  map[string]any{"unencodable": make(chan int)},
	})

	require.ErrorContains(t, err, "marshal sectionbadroot config")
	assert.Nil(t, spawner.pm.GetProcess(name), "the plugin this reload auto-loaded must be stopped")
}

// TestStopDaemonOnStartupFailureAsksWhyTheHandshakeEnded pins the classification,
// row by row, over every pairing of context and cause.
//
// GOAL: only a handshake the plugin did not end is excused, and a real failure
// never is, whatever the context did afterwards.
// METHOD: the real decision over a process declaring fatal, under a reload's scope.
// VALIDATES: the exact stop or carry-on for each pairing of context and cause.
// PREVENTS: a fix excusing everything once the context ended, which would forgive
// a plugin that crashed while a sibling held the tier until the budget ran out, or
// excusing a timeout under a live context, which would forgive a hung plugin.
func TestStopDaemonOnStartupFailureAsksWhyTheHandshakeEnded(t *testing.T) {
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	crash := errors.New("connection closed")
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		cause error
		stops bool
	}{
		{"caller canceled", ended, context.Canceled, false},
		{"caller out of budget", ended, context.DeadlineExceeded, false},
		{"barrier wait ended with the caller", ended, errStartupBarrierAborted, false},
		{"crash before the caller ended", ended, crash, true},
		{"crash under a live caller", context.Background(), crash, true},
		{"own timeout under a live caller", context.Background(), context.DeadlineExceeded, true},
		{"sibling's abort under a live caller", context.Background(), errStartupBarrierAborted, false},
		{"death at the barrier as the caller ended", ended, errStartupPluginEnded, true},
		{"death at the barrier under a live caller", context.Background(), errStartupPluginEnded, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newLifecycleStartupServer(t)
			_, stopped := watchDaemonStop(s)
			proc := process.NewProcess(plugin.PluginConfig{Name: "fatal-classified"})
			proc.SetRegistration(&plugin.PluginRegistration{FailurePolicy: rpc.FailureFatal})

			reload := context.WithValue(tc.ctx, reloadAcceptanceKey{}, &reloadAcceptance{server: s})
			s.stopDaemonOnStartupFailure(reload, proc, tc.cause)

			assert.Equal(t, tc.stops, stopped())
		})
	}
}

// TestBootStopsForAFatalPluginWhateverEndedItsHandshake pins the other half of the
// classification: the causes a reload excuses, under a context with no reload scope.
//
// GOAL: at boot no cause excuses a fatal plugin that did not reach running.
// METHOD: the real decision over a process declaring fatal, with no reload scope.
// VALIDATES: the daemon stops for every pairing of context and cause.
// PREVENTS: a reload's excuse reaching boot, which has no prior state to keep.
func TestBootStopsForAFatalPluginWhateverEndedItsHandshake(t *testing.T) {
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		cause error
	}{
		{"sibling's abort under a live context", context.Background(), errStartupBarrierAborted},
		{"abort as the context ended", ended, errStartupBarrierAborted},
		{"context canceled", ended, context.Canceled},
		{"context out of budget", ended, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newLifecycleStartupServer(t)
			signaled, stopped := watchDaemonStop(s)
			proc := process.NewProcess(plugin.PluginConfig{Name: "fatal-at-boot"})
			proc.SetRegistration(&plugin.PluginRegistration{FailurePolicy: rpc.FailureFatal})

			s.stopDaemonOnStartupFailure(tc.ctx, proc, tc.cause)

			assert.True(t, stopped(), "boot must stop for a fatal plugin that did not reach running")
			assert.True(t, signaled(), "the stop must reach the daemon's own shutdown function")
		})
	}
}

// TestBootSiblingRefusalStopsTheDaemonThroughAFatalPlugin is the boot polarity of
// TestSiblingRefusingConfigDoesNotStopTheDaemonThroughAFatalPlugin.
//
// GOAL: a boot phase that continues after a failure never leaves ze running without
// a healthy plugin that declared fatal and was aborted by a sibling.
// METHOD: the same two real SDK plugins through runPluginPhase, the path the
// explicit phase takes. The fatal plugin's name sorts first, so it is walked before
// the sibling whose failure aborted it.
// VALIDATES: the daemon stops, both are rolled back, and the log line that says why
// names the fatal plugin, its abort and the sibling's refusal.
// PREVENTS: the barrier excuse reaching boot, where runPluginStartup logs the phase's
// error and carries on, leaving ze up without the plugin in silence.
func TestBootSiblingRefusalStopsTheDaemonThroughAFatalPlugin(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const fatalName, refusingName = "boot-fatal-bystander", "boot-refuser"
	const refusal = "a typo in this config"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        fatalName,
		Description: "declares fatal and accepts its configuration",
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(fatalName, conn)
			if err := p.Run(context.Background(), &sdk.Registration{FailurePolicy: rpc.FailureFatal}); err != nil {
				return 1
			}
			return 0
		},
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:        refusingName,
		Description: "refuses its configuration",
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(refusingName, conn)
			p.OnConfigure(func([]sdk.ConfigSection) error { return errors.New(refusal) })
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
	}))

	// Restored after the server's own cleanup, which runs first, so nothing logs
	// through a stale logger.
	orig := logger
	t.Cleanup(func() { logger = orig })
	var buf syncBuffer
	captured := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	logger = func() *slog.Logger { return captured }

	s, spawner := newLifecycleStartupServer(t)
	signaled, stopped := watchDaemonStop(s)

	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: fatalName, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: refusingName, Internal: true, Encoder: plugin.EncodingJSON},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), refusal, "the phase names the refusal")
	assert.True(t, stopped(), "a healthy fatal plugin a sibling aborted at boot must stop the daemon")
	assert.True(t, signaled(), "the stop must reach the daemon's own shutdown function")
	assert.Nil(t, spawner.pm.GetProcess(fatalName), "the failed tier is rolled back")
	assert.Nil(t, spawner.pm.GetProcess(refusingName), "the failed tier is rolled back")

	var stopLine string
	for line := range strings.SplitSeq(buf.String(), "\n") {
		if strings.Contains(line, "stopping ze because a plugin failed") {
			stopLine = line
		}
	}
	require.NotEmpty(t, stopLine, "the stop must be logged")
	assert.Contains(t, stopLine, fatalName, "the stop names the fatal plugin")
	assert.Contains(t, stopLine, errStartupBarrierAborted.Error(), "the stop says the plugin was aborted")
	assert.Contains(t, stopLine, refusingName, "the stop names the sibling")
	assert.Contains(t, stopLine, refusal, "the stop names the sibling's failure")
}

// syncBuffer is a bytes.Buffer a logger may write to while a test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
