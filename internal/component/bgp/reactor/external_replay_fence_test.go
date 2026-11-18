// Design: docs/architecture/plugin/plugin-system.md -- genuine external replay fencing.
// Related: forward_initial_sync_order_test.go -- production forwarding and wire recorder.
package reactor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Only the real child invocation carries this positional marker after --.
// No test environment variable or compiled plugin registration is used.
func TestOrdinarySelectedExternalFenceChild(t *testing.T) {
	if !slices.Contains(os.Args, "ze-ac10-external") {
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	p, err := sdk.NewFromEnv(env.Get("ze.plugin.name"))
	require.NoError(t, err)
	defer p.Close() //nolint:errcheck // The parent owns transport shutdown.
	body := `{"signals-session-ready":true}`
	if slices.Contains(os.Args, "fencing") {
		body = `{"signals-session-ready":true,"fences-live-forwards":true}`
	}
	var declaration sdk.Registration
	require.NoError(t, json.Unmarshal([]byte(body), &declaration))
	// The parent verifies successful startup and the actual process record.
	// Closing its TLS stream is the child's normal stop, not a second verdict.
	_ = p.Run(ctx, &declaration)
}

type externalReplayFenceSpawner struct {
	ctx      context.Context
	acceptor *pluginipc.PluginAcceptor
	mu       sync.Mutex
	manager  *process.ProcessManager
	sealed   bool
}

func (s *externalReplayFenceSpawner) SpawnMore(configs []plugin.PluginConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed {
		return process.ErrDaemonStopping
	}
	if s.manager != nil {
		return s.manager.StartMore(configs)
	}
	s.manager = process.NewProcessManager(configs)
	s.manager.SetAcceptor(s.acceptor)
	return s.manager.StartWithContext(s.ctx)
}
func (s *externalReplayFenceSpawner) GetProcessManager() any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.manager
}
func (s *externalReplayFenceSpawner) SealProcessAdmission() func() {
	s.mu.Lock()
	s.sealed = true
	s.mu.Unlock()
	return func() {}
}

// A truly forked, TLS-authenticated SDK program absent from the compiled
// registry opts in. Initial replay then precedes a live withdrawal on the BGP
// wire. The opt-out control still writes useful live output before any report.
// This tests the existing forward rail's fence, not cross-origin election.
// MUTATION: ignore runtimeReplayFence in registry.FencesLiveForwards; the
// external process then has no compiled fallback and wire order is lost.
func TestOrdinarySelectedExternalFenceDeclaration(t *testing.T) {
	for _, fencing := range []bool{false, true} {
		t.Run(map[bool]string{false: "external-opt-out", true: "external-opt-in"}[fencing], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			t.Cleanup(cancel)
			const name = "unregistered-external-advertiser"
			require.False(t, registry.Has(name), "a forked built-in alias would not prove this boundary")
			certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1:0", nil, time.Hour)
			require.NoError(t, err)
			certificate, err := tls.X509KeyPair(certPEM, keyPEM)
			require.NoError(t, err)
			listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13})
			require.NoError(t, err)
			acceptor := pluginipc.NewPluginAcceptor(listener, "external-fence-test", certPEM)
			acceptor.Start()
			t.Cleanup(acceptor.Stop)
			executable, err := os.Executable()
			require.NoError(t, err)
			quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
			mode := "plain"
			if fencing {
				mode = "fencing"
			}
			host, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: []plugin.PluginConfig{{Name: name, Run: "exec " + quoted + " -test.run='^TestOrdinarySelectedExternalFenceChild$' -- ze-ac10-external " + mode}}, ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{})}, nil)
			require.NoError(t, err)
			host.SetProcessSpawner(&externalReplayFenceSpawner{ctx: ctx, acceptor: acceptor})
			t.Cleanup(host.Stop)
			require.NoError(t, host.StartWithContext(ctx))
			require.NoError(t, host.WaitForStartupComplete(ctx))
			proc := host.ProcessManager().GetProcess(name)
			require.NotNil(t, proc)
			require.True(t, proc.Config().RunsExternalProgram())
			require.False(t, proc.Conn().HasBridge(), "the actual child must use framed TLS, never a registered runner")
			require.True(t, registry.SignalsSessionReady(name))

			r, source, destination, wire, encoding := newSyncOrderRail(t)
			destination.mu.Lock()
			destination.settings.ProcessBindings = []ProcessBinding{{PluginName: name, Send: map[string]bool{bgpevents.SendUpdate: true}, Receive: map[string]events.Direction{bgpevents.EventState: events.DirReceived}}}
			destination.mu.Unlock()
			destination.refreshForwardFacts()
			destination.state.Store(int32(PeerStateConnecting))
			destination.enterEstablished(destination.testInitialSync())
			destination.testInitialSync().sendingInitialRoutes.Store(0)
			adapter := &reactorAPIAdapter{r: r}
			if !fencing {
				require.False(t, destination.initialUpdateOwed.Load())
				updateID := nextMsgID()
				update := syncOrderPublish(t, r, encoding, updateID, syncOrderAnnounceBody)
				require.NoError(t, adapter.forwardUpdateCore(update, updateID, []*Peer{destination}, replayFenceSource(source, false)))
				require.NoError(t, r.fwdPool.barrierPeer(ctx, destination.Settings().PeerKey()))
				require.Equal(t, []string{"announce"}, wireKinds(parseWireUpdates(t, wire.written())), "non-fencing external output must remain useful without a report")
				return
			}
			liveID := nextMsgID()
			live := syncOrderPublish(t, r, encoding, liveID, syncOrderWithdrawBody)
			require.NoError(t, adapter.forwardUpdateCore(live, liveID, []*Peer{destination}, replayFenceSource(source, false)))
			replayID := nextMsgID()
			replay := syncOrderPublish(t, r, encoding, replayID, syncOrderAnnounceBody)
			require.NoError(t, adapter.forwardUpdateCore(replay, replayID, []*Peer{destination}, replayFenceSource(source, true)))
			require.Eventually(t, func() bool { return len(parseWireUpdates(t, wire.written())) > 0 }, 5*time.Second, 5*time.Millisecond, "the initial replay must reach the wire while live output is held")
			require.NoError(t, destination.SignalAPIReady(plugin.ProcessSender(name), destination.session.initialReplay.Load()))
			require.NoError(t, r.fwdPool.barrierPeer(ctx, destination.Settings().PeerKey()))
			require.Equal(t, []string{"announce", "withdraw"}, wireKinds(parseWireUpdates(t, wire.written())), "replay must precede the live withdrawal, leaving no stale route")
		})
	}
}
