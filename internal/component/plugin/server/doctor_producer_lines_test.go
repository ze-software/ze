package server

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// These tests each drive one producer line of a doctor record through the
// function that holds it, and assert the record the doctor snapshot reads, so
// deleting that line turns its test red (spec-doctor-coverage-12-plugin-infra,
// R-10).

// TestReadyParamsRefusedRecordsDrop runs the engine's own startup handshake
// against a plugin whose ready parameters do not parse.
//
// VALIDATES: the driver hands the parse error to the engine sink
// (runStartupHandshake, sink.readyParamsRefused), the engine sink records a
// dropped subscription naming the cause and the parse error
// (engineStartupSink.readyParamsRefused), and startup still completes.
// PREVENTS: a plugin running subscribed to nothing because its ready
// parameters were malformed, with only a Warn line to say so.
// METHOD: handleProcessStartupRPC over net.Pipe, as TestHandleProcessStartupRPC,
// with a JSON string where the ready object belongs: valid JSON, so it reaches
// the driver, and no ReadyInput, so it does not parse.
func TestReadyParamsRefusedRecordsDrop(t *testing.T) {
	t.Parallel()

	engineConn, pluginMux := newDriverPipe(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: "ready-garbled", Internal: true, Encoder: "json"})
	proc.SetConn(engineConn)
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	s, err := NewServer(&ServerConfig{}, &mockReactor{})
	require.NoError(t, err)
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.coordinator = plugin.NewStartupCoordinator(1)

	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)
		drivePluginHandshakeWith(ctx, pluginMux,
			&rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion}, "not-a-ready-object")
	}()
	s.handleProcessStartupRPC(proc)
	<-pluginDone

	require.Equal(t, plugin.StageRunning, proc.Stage(), "unparsable ready parameters stopped the startup")
	drops := proc.DoctorRecord(time.Now()).SubscriptionDrops
	require.Len(t, drops, 1, "the dropped startup subscriptions were not recorded")
	require.Equal(t, process.SubscriptionDropReadyParamsUnparsable, drops[0].Cause)
	require.Contains(t, drops[0].Detail, "unmarshal", "the drop does not carry the parse error")
}

// TestSubscriptionWithNoDefaultNamespaceRecordsDrop registers a subscribe block
// that names no namespace while no protocol component registered a default.
//
// VALIDATES: resolveSubscriptionNamespace records the NoDefaultNamespace drop
// on the process, reached through registerSubscriptions.
// PREVENTS: subscriptions that can never match reading as healthy.
// METHOD: defaultEventNamespace answers empty for this test only; the test
// binary's composition root always registers "bgp". Not parallel: it swaps a
// package variable.
func TestSubscriptionWithNoDefaultNamespaceRecordsDrop(t *testing.T) {
	registered := defaultEventNamespace
	defaultEventNamespace = func() string { return "" }
	t.Cleanup(func() { defaultEventNamespace = registered })

	proc := process.NewProcess(plugin.PluginConfig{Name: "no-default-namespace"})
	newSubscriptionServer().registerSubscriptions(proc, &rpc.SubscribeEventsInput{Events: []string{"update"}})

	drops := proc.DoctorRecord(time.Now()).SubscriptionDrops
	require.Len(t, drops, 1, "the block with no namespace to resolve to was not recorded")
	require.Equal(t, process.SubscriptionDropNoDefaultNamespace, drops[0].Cause)
	require.Empty(t, drops[0].Namespace, "the block named no namespace")
}

// TestPostStartupNoConnectionRecordsUndelivered sends the post-startup
// callback to a running plugin that has no connection, and to a stopped one.
//
// VALIDATES: sendPostStartupTo records the running plugin undelivered with
// errPostStartupNoConn, and records nothing for the stopped one.
// PREVENTS: a plugin that will never get its callback reading as healthy.
func TestPostStartupNoConnectionRecordsUndelivered(t *testing.T) {
	t.Parallel()

	s := &Server{ctx: context.Background()}
	running := process.NewProcess(plugin.PluginConfig{Name: "no-conn"})
	running.SetRunning(true)
	s.sendPostStartupTo(running)
	record := running.DoctorRecord(time.Now())
	require.Equal(t, process.PostStartupUndelivered, record.PostStartup)
	require.Equal(t, errPostStartupNoConn.Error(), record.PostStartupError)

	stopped := process.NewProcess(plugin.PluginConfig{Name: "stopped"})
	s.sendPostStartupTo(stopped)
	require.Equal(t, process.PostStartupUnspecified, stopped.DoctorRecord(time.Now()).PostStartup,
		"a stopped plugin has nothing to deliver to, so nothing is recorded")
}

// TestPostStartupPendingUntilAnswered sends the post-startup callback to a
// running plugin that has not answered yet, then answers it.
//
// VALIDATES: sendPostStartupTo records Pending before the delivery goroutine
// runs, and the answer moves the record to Delivered.
// PREVENTS: a callback still in flight reading as never sent.
// METHOD: the plugin side holds the request unanswered while the record is
// read, so only the Pending write can have produced it.
func TestPostStartupPendingUntilAnswered(t *testing.T) {
	t.Parallel()

	engineConn, pluginMux := newDriverPipe(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: "pending"})
	proc.SetConn(engineConn)
	proc.SetRunning(true)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := &Server{ctx: ctx}

	s.sendPostStartupTo(proc)
	require.Equal(t, process.PostStartupPending, proc.DoctorRecord(time.Now()).PostStartup,
		"an unanswered callback is not recorded pending")

	select {
	case req := <-pluginMux.Requests():
		require.NoError(t, pluginMux.SendOK(ctx, req.ID))
	case <-time.After(5 * time.Second):
		t.Fatal("the post-startup callback never reached the plugin")
	}
	require.Eventually(t, func() bool {
		return proc.DoctorRecord(time.Now()).PostStartup == process.PostStartupDelivered
	}, 5*time.Second, 10*time.Millisecond, "the answered callback is not recorded delivered")
}

// TestReloadRecordsAndClearsCapabilityRestore runs a refused reload whose
// capability restore fails, then a committed reload.
//
// VALIDATES: runTxCoordinator records the failed restore after the refused
// reload (s.capabilityRestore.failed), and the committed reload clears it
// (s.capabilityRestore.clear), both read where PluginReloadRecords reads them.
// PREVENTS: an OPEN carrying a refused reload's capabilities with nothing in
// doctor, and a restore failure reported after a commit replaced it.
// METHOD: "caps" gives up capability 9 in its verify answer; after the bridge
// installs that, and before the failing apply answers, a plugin outside the
// transaction declares capability 9 for the same peer, as a plugin starting
// mid-reload does. Restoring "caps"' previous declaration then conflicts.
func TestReloadRecordsAndClearsCapabilityRestore(t *testing.T) {
	t.Parallel()

	plugins := []pluginDef{
		{
			name: "caps", roots: []string{"bgp"},
			verifyResp: &rpc.ConfigVerifyOutput{Status: rpc.StatusOK, Capabilities: &rpc.DeclareCapabilitiesInput{}},
		},
		{
			name: "failing", roots: []string{"bgp"},
			applyResp: &rpc.ConfigApplyOutput{Status: rpc.StatusError, Error: "journal full"},
		},
	}
	s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{
		"bgp": map[string]any{"router-id": "1.2.3.4"},
	}}, plugins)
	s.capInjector = plugin.NewCapabilityInjector()
	seedCapabilities(t, s, "caps", roleDeclaration("00"))

	intruder := capabilitiesFromRPC(roleDeclaration("07"))
	intruder.PluginName = "intruder"
	added := make(chan error, 1)
	failing := plugins[1].responder
	failing.mu.Lock()
	failing.beforeApplyRsp = func() {
		addErr := s.capInjector.AddPluginCapabilities(intruder)
		select {
		case added <- addErr:
		default: // the first apply's answer is the one the test reads
		}
	}
	failing.mu.Unlock()

	err := s.ReloadConfig(context.Background(), map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}})
	require.Error(t, err)
	require.ErrorContains(t, err, "journal full")
	select {
	case addErr := <-added:
		require.NoError(t, addErr, "the intruder could not take the capability caps gave up")
	default:
		t.Fatal("the failing apply never ran, so nothing was installed to restore")
	}
	require.ErrorContains(t, err, "restore previous capabilities")
	failures := s.capabilityRestore.read()
	require.Len(t, failures, 1, "the failed restore was not recorded")
	require.Contains(t, failures[0].Err, "restore previous capabilities")

	failing.mu.Lock()
	failing.applyResp = nil
	failing.beforeApplyRsp = nil
	failing.mu.Unlock()
	require.NoError(t, s.ReloadConfig(context.Background(), map[string]any{"bgp": map[string]any{"router-id": "9.9.9.9"}}))
	require.Empty(t, s.capabilityRestore.read(), "a committed reload left the restore failure recorded")
}

// TestCleanupRecordsTheEngineThatMissedItsGrace stops a server whose plugin
// engine never returns.
//
// VALIDATES: Server.cleanup writes each engine ProcessManager.Stop saw late to
// the last-stop record (recordLateEngines(pm.CleanupLate())), naming it.
// PREVENTS: a stop that lost a plugin's cleanup leaving the next local
// `ze doctor` with nothing to report.
// METHOD: a registered engine that blocks after its connection closes. This
// package cannot shorten process.pluginStopGrace, so the test spends the real
// 3s grace. Not parallel: it swaps the state store and the plugin registry.
func TestCleanupRecordsTheEngineThatMissedItsGrace(t *testing.T) {
	useLastStopStore(t)
	const name = "test-server-cleanup-late-engine"
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "test plugin whose engine misses the stop grace",
		RunEngine: func(conn net.Conn) int {
			_, _ = io.Copy(io.Discard, conn) // returns once Stop closes the connection
			<-stuck
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	pm := process.NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	require.NoError(t, pm.Start())
	s := &Server{}
	s.procManager.Store(pm)
	s.cleanup()

	record, found, err := ReadLastStop()
	require.NoError(t, err)
	require.True(t, found, "the late engine wrote no last-stop record")
	require.Len(t, record.Entries, 1)
	require.Equal(t, LastStopCleanupLate, record.Entries[0].Reason)
	require.Equal(t, name, record.Entries[0].Plugin)
}
