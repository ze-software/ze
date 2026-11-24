// Design: docs/architecture/plugin/plugin-system.md -- authenticated reporter to wire fence.
package reactor

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/selfcert"
)

// Drive the real TLS child with a name also held by compiled metadata. The
// actual initialUpdateReporters/replayFenceOwners path must follow its Stage-1
// flags and produce replay-before-live wire order. The inverse opt-out case
// keeps useful wire output despite a compiled fence.
// MUTATION: apply compiled-first precedence only in SignalsSessionReady.
func TestExternalReplayFenceUsesAuthenticatedReadiness(t *testing.T) {
	for _, fencing := range []bool{false, true} {
		t.Run(map[bool]string{false: "compiled-true-runtime-opt-out", true: "compiled-false-runtime-opt-in"}[fencing], func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			t.Cleanup(cancel)
			const name = "compiled-name-external-readiness"
			require.NoError(t, registry.Register(registry.Registration{Name: name,
				SignalsSessionReady: !fencing, FencesLiveForwards: !fencing,
				RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 }}))
			certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1:0", nil, time.Hour)
			require.NoError(t, err)
			certificate, err := tls.X509KeyPair(certPEM, keyPEM)
			require.NoError(t, err)
			listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13})
			require.NoError(t, err)
			acceptor := pluginipc.NewPluginAcceptor(listener, "external-readiness-test", certPEM)
			acceptor.Start()
			t.Cleanup(acceptor.Stop)
			executable, err := os.Executable()
			require.NoError(t, err)
			quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
			mode := "plain"
			if fencing {
				mode = "fencing"
			}
			host, err := pluginserver.NewServer(&pluginserver.ServerConfig{
				Plugins:         []plugin.PluginConfig{{Name: name, Run: "exec " + quoted + " -test.run='^TestOrdinarySelectedExternalFenceChild$' -- ze-ac10-external " + mode}},
				ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{})}, nil)
			require.NoError(t, err)
			host.SetProcessSpawner(&externalReplayFenceSpawner{ctx: ctx, acceptor: acceptor})
			t.Cleanup(host.Stop)
			require.NoError(t, host.StartWithContext(ctx))
			require.NoError(t, host.WaitForStartupComplete(ctx))
			proc := host.ProcessManager().GetProcess(name)
			require.NotNil(t, proc)
			require.True(t, proc.Config().RunsExternalProgram(), "a compiled name must still run the actual TLS child")
			require.False(t, proc.Conn().HasBridge())
			r, source, destination, wire, encoding := newSyncOrderRail(t)
			setTestPeerBus(t, destination, testBusBoth(name, []string{bgpevents.EventState}, []string{bgpevents.SendUpdate}))
			destination.refreshForwardFacts()
			require.Equal(t, []string{name}, destination.initialUpdateReporters(), "the actual reporter caller must use current Stage-1 readiness")
			destination.state.Store(int32(PeerStateConnecting))
			destination.enterEstablished(destination.testInitialSync())
			destination.testInitialSync().sendingInitialRoutes.Store(0)
			destination.mu.Lock()
			owners := destination.replayFenceOwners()
			destination.mu.Unlock()
			adapter := &reactorAPIAdapter{r: r}
			if !fencing {
				require.Empty(t, owners)
				require.False(t, destination.initialUpdateOwed.Load())
				updateID := nextMsgID()
				update := syncOrderPublish(t, r, encoding, updateID, syncOrderAnnounceBody)
				require.NoError(t, adapter.forwardUpdateCore(update, updateID, []*Peer{destination}, replayFenceSource(source, false)))
				require.NoError(t, r.fwdPool.barrierPeer(ctx, destination.Settings().PeerKey()))
				require.Equal(t, []string{"announce"}, wireKinds(parseWireUpdates(t, wire.written())))
				return
			}
			require.Equal(t, []string{name}, owners)
			require.True(t, destination.initialUpdateOwed.Load())
			liveID := nextMsgID()
			live := syncOrderPublish(t, r, encoding, liveID, syncOrderWithdrawBody)
			require.NoError(t, adapter.forwardUpdateCore(live, liveID, []*Peer{destination}, replayFenceSource(source, false)))
			replayID := nextMsgID()
			replay := syncOrderPublish(t, r, encoding, replayID, syncOrderAnnounceBody)
			require.NoError(t, adapter.forwardUpdateCore(replay, replayID, []*Peer{destination}, replayFenceSource(source, true)))
			require.Eventually(t, func() bool { return len(parseWireUpdates(t, wire.written())) > 0 }, 5*time.Second, 5*time.Millisecond,
				"useful initial replay must reach the actual writer while live output is held")
			require.NoError(t, destination.SignalAPIReady(plugin.ProcessSender(name), destination.session.initialReplay.Load()))
			require.NoError(t, r.fwdPool.barrierPeer(ctx, destination.Settings().PeerKey()))
			require.Equal(t, []string{"announce", "withdraw"}, wireKinds(parseWireUpdates(t, wire.written())))
		})
	}
}
