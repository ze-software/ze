// Design: docs/architecture/api/process-protocol.md -- socket-to-writer settlement.
package reactor

import (
	"bufio"
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Hiding BridgedConn's optional bridge is how the real SDK negotiates the
// public socket transport even though this fixture's process is in-process.
type selectedRefreshSocketConn struct{ net.Conn }

type selectedRefreshSpawner struct {
	admissionMu sync.Mutex
	sealed      bool
	ctx         context.Context
	manager     *process.ProcessManager
}

func (spawner *selectedRefreshSpawner) SpawnMore(configs []plugin.PluginConfig) error {
	spawner.admissionMu.Lock()
	if spawner.sealed {
		spawner.admissionMu.Unlock()
		return process.ErrDaemonStopping
	}
	first := spawner.manager == nil
	if first {
		spawner.manager = process.NewProcessManager(configs)
	}
	pm := spawner.manager
	spawner.admissionMu.Unlock()
	if first {
		return pm.StartWithContext(spawner.ctx)
	}
	return pm.StartMore(configs)
}

func (spawner *selectedRefreshSpawner) GetProcessManager() any {
	spawner.admissionMu.Lock()
	defer spawner.admissionMu.Unlock()
	return spawner.manager
}

func (spawner *selectedRefreshSpawner) SealProcessAdmission() func() {
	spawner.admissionMu.Lock()
	defer spawner.admissionMu.Unlock()
	spawner.sealed = true
	return func() {}
}

// The real net.Pipe write returns on its request-bound deadline, then the test
// holds that final writer before it can report completion to the evaluator.
type selectedRefreshHeldConn struct {
	net.Conn
	writing     chan struct{}
	interrupted chan struct{}
	release     <-chan struct{}
}

func (conn *selectedRefreshHeldConn) Write(data []byte) (int, error) {
	close(conn.writing)
	n, err := conn.Conn.Write(data)
	close(conn.interrupted)
	<-conn.release
	return n, err
}

// This test crosses five-stage declaration, public SDK RPC admission,
// cancel-request/EOF/process-stop and the actual reactor's net.Pipe writer.
// A held A must neither block useful B nor allow acknowledged cancellation or
// process cleanup to claim completion before A's writer actually returns.
func TestSelectedRefreshSocketCancellationJoinsRealWriter(t *testing.T) {
	for _, ending := range []string{"request", "EOF", "process"} {
		t.Run(ending, func(t *testing.T) {
			fixture, input := selectedRefreshFixture(t)
			work, stop := context.WithTimeout(t.Context(), 15*time.Second)
			defer stop()
			declaration := []rpc.DisseminationDecl{{Namespace: "bgp", Role: SelectedDisseminationRole,
				SourceReceive:      []string{"update-received", "state"},
				DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"}}}
			ready := make(chan *sdk.Plugin, 1)
			runnerDone := make(chan struct{})
			const implementation = "selected-refresh-socket-runner"
			require.NoError(t, registry.Register(registry.Registration{Name: implementation,
				Sends: testAllBGPSendDecls(), Receives: testAllBGPReceiveDecls(),
				Dissemination: declaration, CLIHandler: func([]string) int { return 0 },
				RunEngine: func(conn net.Conn) int {
					defer close(runnerDone)
					client := sdk.NewWithConn(selectedWriterProcess, selectedRefreshSocketConn{conn})
					defer func() { _ = client.Close() }()
					client.OnConfigure(func([]sdk.ConfigSection) error { return nil })
					client.OnStarted(func(context.Context) error { ready <- client; return nil })
					_ = client.Run(work, &sdk.Registration{Sends: testAllBGPSendDecls(), Receives: testAllBGPReceiveDecls(), Dissemination: declaration})
					return 0
				}}))
			host, err := pluginserver.NewServer(&pluginserver.ServerConfig{
				ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{}),
				Plugins: []plugin.PluginConfig{{Name: selectedWriterProcess, Run: implementation,
					Internal: true, Encoder: plugin.EncodingJSON}}}, fixture.api)
			require.NoError(t, err)
			spawner := &selectedRefreshSpawner{ctx: work}
			host.SetProcessSpawner(spawner)
			destBus := testBusSelectedDest(selectedWriterProcess)
			host.UpdateDeliveryGraph("bgp", []pluginserver.DeliveryPeer{
				testDeliveryPeer("192.0.2.79", "", testBusSourceLease(selectedWriterProcess)),
				testDeliveryPeer(input.Destination, "", destBus),
				testDeliveryPeer("192.0.2.81", "", destBus),
			})
			fixture.api.r.api = host
			require.NoError(t, host.StartWithContext(work))
			t.Cleanup(func() {
				host.Stop()
				join, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
				defer cancel()
				require.NoError(t, host.Wait(join))
				select {
				case <-runnerDone:
				case <-join.Done():
					t.Error("SDK runner did not join")
				}
			})
			require.NoError(t, host.WaitForStartupComplete(work))
			var client *sdk.Plugin
			select {
			case client = <-ready:
			case <-work.Done():
				t.Fatal("socket SDK never became ready")
			}
			proc := host.ProcessManager().GetProcess(selectedWriterProcess)
			require.NotNil(t, proc)
			require.False(t, proc.Conn().HasBridge(), "this proof must cross public socket RPC")
			fixture.peer.refreshForwardFacts()
			result, err := client.SelectRefresh(work, input)
			require.NoError(t, err)
			require.Equal(t, rpc.SelectedRefreshWritten, result.Outcome)
			require.Equal(t, []string{"borr"}, refreshWireLabels(t, fixture.conn.written()))
			// Finish this useful recording connection's replay before the
			// fixture installs a different socket epoch on the same Session.
			// The following held request still exercises a fresh actual Begin.
			completed := input
			completed.Action = rpc.SelectedRefreshEnd
			result, err = client.SelectRefresh(work, completed)
			require.NoError(t, err)
			require.Equal(t, rpc.SelectedRefreshWritten, result.Outcome)
			require.Equal(t, []string{"borr", "eorr"}, refreshWireLabels(t, fixture.conn.written()))

			sibling, siblingWire := newGroupUpdatesPeer(t, "192.0.2.81", "absent")
			sibling.settings.ExportPolicy = fixture.peer.settings.ExportPolicy
			sibling.settings.Bus = fixture.peer.settings.Bus.Clone()
			sibling.reactor = fixture.api.r
			sibling.session.initialReplay.Store(72)
			sibling.negotiated.Load().EnhancedRouteRefresh = true
			sibling.refreshForwardFacts()
			fixture.api.r.peers[sibling.Settings().PeerKey()] = sibling
			siblingInput := input
			siblingInput.Destination, siblingInput.PeerOwner, siblingInput.DestinationSession = "192.0.2.81", sibling.sourceOwner, 72

			left, right := net.Pipe()
			release := make(chan struct{})
			released := false
			held := &selectedRefreshHeldConn{Conn: left, writing: make(chan struct{}),
				interrupted: make(chan struct{}), release: release}
			session := fixture.peer.session
			session.conn = held
			writer := &observedBGPWriter{writer: held, session: session}
			writer.context.Store(uint32(session.sendCtxID))
			session.wireWriter, session.bufWriter = writer, bufio.NewWriter(writer)
			ctx, cancel := context.WithCancel(work)
			defer cancel()
			finished := make(chan error, 1)
			joined := false
			t.Cleanup(func() {
				cancel()
				_ = left.Close()
				_ = right.Close()
				if !released {
					close(release)
				}
				if !joined {
					select {
					case <-finished: //nolint:errcheck // test cleanup joins the canceled socket goroutine
					case <-time.After(5 * time.Second):
						t.Error("socket marker caller did not join")
					}
				}
			})
			go func() { _, err := client.SelectRefresh(ctx, input); finished <- err }()
			select {
			case <-held.writing:
			case err := <-finished:
				joined = true
				t.Fatalf("marker never reached its real writer: %v", err)
			case <-work.Done():
				t.Fatal("marker never entered its real socket")
			}
			result, err = client.SelectRefresh(work, siblingInput)
			require.NoError(t, err, "healthy B must complete while A is blocked")
			require.Equal(t, rpc.SelectedRefreshWritten, result.Outcome)
			require.Equal(t, []string{"borr"}, refreshWireLabels(t, siblingWire.written()))
			var cleanupDone chan error
			if ending != "request" {
				cleanupDone = make(chan error, 1)
				go func() { cleanupDone <- proc.WaitRuntimeCleanupContext(work) }()
			}
			switch ending {
			case "request":
				cancel()
			case "EOF":
				require.NoError(t, client.Close())
			case "process":
				proc.Stop()
			}
			select {
			case <-held.interrupted:
			case <-work.Done():
				t.Fatal("socket end never reached the real final writer")
			}
			if ending == "request" {
				select {
				case err := <-finished:
					joined = true
					t.Fatalf("unjoined writer returned acknowledged cancellation: %v", err)
				default:
				}
			} else {
				select {
				case err := <-cleanupDone:
					t.Fatalf("unjoined writer completed process cleanup: %v", err)
				default:
				}
			}
			close(release)
			released = true
			select {
			case err := <-finished:
				joined = true
				require.Error(t, err)
				if ending == "request" {
					require.ErrorIs(t, err, context.Canceled)
					var unsettled *rpc.CancellationUnsettledError
					require.False(t, errors.As(err, &unsettled), "cancel-request must acknowledge the joined writer")
				}
			case <-work.Done():
				t.Fatal("socket marker caller did not join")
			}
			if cleanupDone != nil {
				select {
				case err := <-cleanupDone:
					require.NoError(t, err)
				case <-work.Done():
					t.Fatal("ended process cleanup did not join")
				}
			}
			require.True(t, session.writeMu.TryLock(), "final writer lock must be released before settlement")
			session.writeMu.Unlock()
			require.NoError(t, host.Context().Err(), "one connection must not stop the daemon")
			require.Equal(t, []string{"borr"}, refreshWireLabels(t, siblingWire.written()))
			if ending == "request" {
				require.NoError(t, proc.RequirePeerDeclarations())
				result, err = client.SelectRefresh(work, siblingInput)
				require.NoError(t, err, "the selecting process and healthy peer remain useful")
				require.Equal(t, rpc.SelectedRefreshWritten, result.Outcome)
				require.Equal(t, []string{"borr", "borr"}, refreshWireLabels(t, siblingWire.written()))
			}
		})
	}
}
