//go:build ze_web

// Design: docs/guide/l2tp.md -- RADIUS retirement follows whole-reload acceptance
package hub

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	l2tpauthradius "github.com/ze-software/ze/internal/component/l2tp/plugins/authradius"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/component/web"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const radiusReloadProbeName = "zzz-radius-reload-probe"
const radiusReloadProbeRoot = "radius-reload-probe"

type radiusReloadProbeGate struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

var (
	radiusReloadProbeOnce    sync.Once
	radiusReloadProbeError   error
	radiusReloadProbeFailure atomic.Pointer[radiusReloadProbeGate]
)

// The unrelated participant uses the same registered SDK path as RADIUS. Its
// gate permits an older outer reload to finish while a newer apply is tentative.
func registerRadiusReloadProbe(t *testing.T) {
	t.Helper()
	radiusReloadProbeOnce.Do(func() {
		radiusReloadProbeError = registry.Register(registry.Registration{
			Name:        radiusReloadProbeName,
			Description: "RADIUS reload acceptance test participant",
			ConfigRoots: []string{radiusReloadProbeRoot},
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(radiusReloadProbeName, conn)
				defer func() { _ = p.Close() }()
				ctx, cancel := sdk.SignalContext()
				defer cancel()
				p.OnConfigVerify(func([]sdk.ConfigSection) error { return nil })
				p.OnConfigRollback(func(string) error { return nil })
				p.OnConfigApply(func([]sdk.ConfigDiffSection) error {
					gate := radiusReloadProbeFailure.Load()
					if gate == nil {
						return nil
					}
					gate.once.Do(func() { close(gate.entered) })
					select {
					case <-gate.release:
						return errors.New("test unrelated participant rejects apply")
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				if err := p.Run(ctx, sdk.Registration{
					WantsConfig: []string{radiusReloadProbeRoot}, VerifyBudget: 1, ApplyBudget: 10,
				}); err != nil {
					return 1
				}
				return 0
			},
		})
	})
	require.NoError(t, radiusReloadProbeError)
	t.Cleanup(func() { radiusReloadProbeFailure.Store(nil) })
}

// This receiver acknowledges real Accounting-Requests and retains their wire
// identity and cause. No transaction event is fabricated by this fixture.
func reloadRadiusReceiver(t *testing.T) (string, <-chan *radius.Packet) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	packets := make(chan *radius.Packet, 32)
	done := make(chan struct{})
	t.Cleanup(func() { _ = conn.Close(); <-done })
	go func() {
		defer close(done)
		wire := make([]byte, radius.MaxPacketLen)
		for {
			n, peer, readErr := conn.ReadFromUDP(wire)
			if readErr != nil {
				return
			}
			packet, decodeErr := radius.Decode(append([]byte(nil), wire[:n]...))
			if decodeErr != nil || packet.Code != radius.CodeAccountingReq {
				continue
			}
			select {
			case packets <- packet:
			default:
				t.Error("accounting capture overflow")
				return
			}
			response := make([]byte, radius.HeaderLen)
			response[0], response[1] = radius.CodeAccountingResp, packet.Identifier
			binary.BigEndian.PutUint16(response[2:4], radius.HeaderLen)
			auth := radius.ResponseAuthenticator(radius.CodeAccountingResp, packet.Identifier, radius.HeaderLen, packet.Authenticator, nil, []byte("reload-key"))
			copy(response[4:], auth[:])
			_, _ = conn.WriteToUDP(response, peer)
		}
	}()
	return conn.LocalAddr().String(), packets
}

func nextReloadAccounting(t *testing.T, packets <-chan *radius.Packet, status uint32) *radius.Packet {
	t.Helper()
	select {
	case packet := <-packets:
		value := packet.FindAttr(radius.AttrAcctStatusType)
		require.Len(t, value, 4)
		require.Equal(t, status, binary.BigEndian.Uint32(value), "unexpected accounting transition")
		return packet
	case <-time.After(5 * time.Second):
		t.Fatal("accounting record did not arrive")
		return nil
	}
}

func TestDoReloadRadiusRemovalLateBindFailurePreservesAccounting(t *testing.T) {
	resetAAABundleForTest(t)
	registerRadiusReloadProbe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	originalAuth := l2tp.GetAuthHandler()
	originalBus := registry.GetEventBus()
	t.Cleanup(func() {
		l2tp.RegisterAuthHandler(originalAuth)
		registry.SetEventBus(originalBus)
		registry.Lookup(l2tpauthradius.Name).ConfigureEventBus(originalBus)
	})

	address, packets := reloadRadiusReceiver(t)
	_, radiusPort, err := net.SplitHostPort(address)
	require.NoError(t, err)
	_, certB64, keyB64 := caSignedB64(t, "radius-reload")
	certDER, err := base64.StdEncoding.DecodeString(certB64)
	require.NoError(t, err)
	keyDER, err := base64.StdEncoding.DecodeString(keyB64)
	require.NoError(t, err)
	listener, err := web.NewWebServer(web.WebConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		CertPEM:     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
		KeyPEM:      pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	})
	require.NoError(t, err)
	webDone := make(chan error, 1)
	go func() { webDone <- listener.ListenAndServe(ctx) }()
	t.Cleanup(func() {
		stopCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		require.NoError(t, listener.Shutdown(stopCtx))
		require.NoError(t, <-webDone)
	})
	require.NoError(t, listener.WaitReady(ctx))
	originalAddresses := listener.Addresses()
	_, webPort, err := net.SplitHostPort(originalAddresses[0])
	require.NoError(t, err)
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = occupied.Close() })
	_, occupiedPort, err := net.SplitHostPort(occupied.Addr().String())
	require.NoError(t, err)

	// Keep L2TP configured while removing its RADIUS provider. Stopping the
	// L2TP subsystem itself would genuinely terminate its subscriber sessions.
	running := map[string]any{
		radiusReloadProbeRoot: map[string]any{"revision": "0"},
		"l2tp": map[string]any{
			"enabled": "true",
			"auth": map[string]any{"radius": map[string]any{
				"timeout": "1",
				"server": map[string]any{"test": map[string]any{
					"address": "127.0.0.1", "port": radiusPort, "shared-key": "reload-key",
				}},
			}},
		},
	}
	provider := zeconfig.NewProvider()
	applyLoadedTreeToProvider(provider, running)
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{
		Plugins: []plugin.PluginConfig{
			{Name: l2tpauthradius.Name, Internal: true},
			{Name: radiusReloadProbeName, Internal: true},
		},
	}, &reloadTestReactor{tree: running})
	require.NoError(t, err)
	manager := pluginmgr.NewManager()
	require.NoError(t, manager.StartAll(ctx, server, provider))
	server.SetProcessSpawner(manager)
	registry.SetEventBus(server)
	t.Cleanup(func() { server.Stop(); require.NoError(t, manager.StopAll(context.Background())) })
	require.NoError(t, server.StartWithContext(ctx))
	require.NoError(t, server.WaitForStartupComplete(ctx))

	up := func(t *testing.T, id uint16) *radius.Packet {
		t.Helper()
		_, emitErr := server.Emit(l2tpevents.Namespace, l2tpevents.SessionIPAssignedEvent, &l2tpevents.SessionIPAssignedPayload{
			TunnelID: 81, SessionID: id, Username: "alice", PeerAddr: "192.0.2.81",
		})
		require.NoError(t, emitErr)
		return nextReloadAccounting(t, packets, radius.AcctStatusStart)
	}
	first, second := up(t, 1), up(t, 2)
	loadRemoval := func(port string) func() (map[string]any, *zeconfig.Tree, error) {
		return func() (map[string]any, *zeconfig.Tree, error) {
			tree := zeconfig.NewTree()
			environment := zeconfig.NewTree()
			environment.SetContainer("web", listenerServiceTree(port))
			tree.SetContainer("environment", environment)
			l2tpTree := zeconfig.NewTree()
			l2tpTree.Set("enabled", "true")
			tree.SetContainer("l2tp", l2tpTree)
			probeTree := zeconfig.NewTree()
			probeTree.Set("revision", "0")
			tree.SetContainer(radiusReloadProbeRoot, probeTree)
			return tree.ToPluginMap(), tree, nil
		}
	}
	migrator := &listenerMigrator{web: listener}
	migrator.markUnauthenticated(svcWeb)
	err = doReloadContext(ctx, server, nil, provider, nil, "", loadRemoval(occupiedPort), migrator)
	require.ErrorIs(t, err, syscall.EADDRINUSE, "must reach the genuine late web bind failure")
	require.Equal(t, originalAddresses, listener.Addresses())

	// A failed candidate must leave this session able to produce its eventual
	// User-Request Stop with the same Acct-Session-Id as its original Start.
	_, err = server.Emit(l2tpevents.Namespace, l2tpevents.SessionDownEvent, &l2tpevents.SessionDownPayload{
		TunnelID: 81, SessionID: 1, Cause: l2tpevents.TerminateCauseUserRequest,
	})
	require.NoError(t, err)
	stop := nextReloadAccounting(t, packets, radius.AcctStatusStop)
	require.Equal(t, first.FindAttr(radius.AttrAcctSessionID), stop.FindAttr(radius.AttrAcctSessionID))
	require.Equal(t, []byte{0, 0, 0, byte(l2tpevents.TerminateCauseUserRequest)}, stop.FindAttr(radius.AttrAcctTerminateCause))
	third := up(t, 3) // Compensation also resumes new accounting admissions.

	require.NoError(t, doReloadContext(ctx, server, nil, provider, nil, "", loadRemoval(webPort), migrator))
	remaining := map[string]bool{
		string(second.FindAttr(radius.AttrAcctSessionID)): true,
		string(third.FindAttr(radius.AttrAcctSessionID)):  true,
	}
	for range 2 {
		stop = nextReloadAccounting(t, packets, radius.AcctStatusStop)
		id := string(stop.FindAttr(radius.AttrAcctSessionID))
		require.True(t, remaining[id], "accepted removal stopped an unknown or already closed session")
		delete(remaining, id)
		require.Equal(t, []byte{0, 0, 0, byte(l2tpevents.TerminateCauseAdminReboot)}, stop.FindAttr(radius.AttrAcctTerminateCause))
	}

	// A standalone server transaction has no later listener/publication steps.
	// Its successful return must still retire accounting, including after a
	// previous whole-reload retirement has installed a fresh client.
	require.NoError(t, server.ReloadConfig(ctx, running))
	fourth := up(t, 4)
	removed, _, err := loadRemoval(webPort)()
	require.NoError(t, err)
	require.NoError(t, server.ReloadConfig(ctx, removed))
	stop = nextReloadAccounting(t, packets, radius.AcctStatusStop)
	require.Equal(t, fourth.FindAttr(radius.AttrAcctSessionID), stop.FindAttr(radius.AttrAcctSessionID))
	require.Equal(t, []byte{0, 0, 0, byte(l2tpevents.TerminateCauseAdminReboot)}, stop.FindAttr(radius.AttrAcctTerminateCause))

	// Rejecting an outer scope restores accounting before a later removal
	// retry. Sessions must still end normally between rejection and retry.
	require.NoError(t, server.ReloadConfig(ctx, running))
	retryFirst, retryRemaining := up(t, 5), up(t, 6)
	pendingCtx, finishPending := server.DeferReloadAcceptance(ctx)
	defer finishPending(false)
	require.NoError(t, server.ReloadConfig(pendingCtx, removed))
	finishPending(false)
	_, err = server.Emit(l2tpevents.Namespace, l2tpevents.SessionDownEvent, &l2tpevents.SessionDownPayload{
		TunnelID: 81, SessionID: 5, Cause: l2tpevents.TerminateCauseUserRequest,
	})
	require.NoError(t, err)
	stop = nextReloadAccounting(t, packets, radius.AcctStatusStop)
	require.Equal(t, retryFirst.FindAttr(radius.AttrAcctSessionID), stop.FindAttr(radius.AttrAcctSessionID))
	require.Equal(t, []byte{0, 0, 0, byte(l2tpevents.TerminateCauseUserRequest)}, stop.FindAttr(radius.AttrAcctTerminateCause))
	require.NoError(t, server.ReloadConfig(ctx, removed))
	stop = nextReloadAccounting(t, packets, radius.AcctStatusStop)
	require.Equal(t, retryRemaining.FindAttr(radius.AttrAcctSessionID), stop.FindAttr(radius.AttrAcctSessionID))
	require.Equal(t, []byte{0, 0, 0, byte(l2tpevents.TerminateCauseAdminReboot)}, stop.FindAttr(radius.AttrAcctTerminateCause))

	t.Run("unrelated-participant-accepts-pending-removal", func(t *testing.T) {
		require.NoError(t, server.ReloadConfig(ctx, running))
		start := up(t, 7)
		abandonedCtx, abandon := server.DeferReloadAcceptance(ctx)
		defer abandon(false)
		require.NoError(t, server.ReloadConfig(abandonedCtx, removed))
		abandon(false)

		// Retry the removal alongside an unrelated root change. Rejection
		// restored RADIUS, so this accepted tree must retire its live records.
		unrelated := cloneStringAnyMap(removed)
		unrelated[radiusReloadProbeRoot] = map[string]any{"revision": "1"}
		require.NoError(t, server.ReloadConfig(ctx, unrelated))
		stop := nextReloadAccounting(t, packets, radius.AcctStatusStop)
		require.Equal(t, start.FindAttr(radius.AttrAcctSessionID), stop.FindAttr(radius.AttrAcctSessionID))
		require.Equal(t, []byte{0, 0, 0, byte(l2tpevents.TerminateCauseAdminReboot)}, stop.FindAttr(radius.AttrAcctTerminateCause))
	})

	t.Run("accepted-previous-state-survives-newer-rollback", func(t *testing.T) {
		require.NoError(t, server.ReloadConfig(ctx, running))
		start := up(t, 8)
		olderCtx, finishOlder := server.DeferReloadAcceptance(ctx)
		defer finishOlder(false)
		require.NoError(t, server.ReloadConfig(olderCtx, removed))

		gate := &radiusReloadProbeGate{entered: make(chan struct{}), release: make(chan struct{})}
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(gate.release) }) }
		t.Cleanup(release)
		t.Cleanup(func() { radiusReloadProbeFailure.Store(nil) })
		radiusReloadProbeFailure.Store(gate)
		newer := cloneStringAnyMap(running)
		newer[radiusReloadProbeRoot] = map[string]any{"revision": "1"}
		newerDone := make(chan error, 1)
		go func() { newerDone <- server.ReloadConfig(ctx, newer) }()
		newerJoined := false
		t.Cleanup(func() {
			release()
			if !newerJoined {
				select {
				case <-newerDone:
				case <-time.After(5 * time.Second):
					t.Error("rejecting reload did not unwind during cleanup")
				}
			}
		})
		select {
		case <-gate.entered:
		case <-ctx.Done():
			t.Fatal("newer reload did not reach its late rejecting participant")
		}

		// The probe applies after RADIUS. Accepting A must preserve B's
		// tentative client: a new subscriber can still start accounting.
		finishOlder(true)
		tentative := up(t, 9)
		radiusReloadProbeFailure.Store(nil)
		release()
		select {
		case err := <-newerDone:
			newerJoined = true
			require.ErrorContains(t, err, "test unrelated participant rejects apply")
		case <-ctx.Done():
			t.Fatal("newer rejected reload did not finish rollback")
		}
		// Rollback restores A after A's completion was already consumed.
		// Both retained records must retire without another acceptance event.
		retained := map[string]bool{
			string(start.FindAttr(radius.AttrAcctSessionID)):     true,
			string(tentative.FindAttr(radius.AttrAcctSessionID)): true,
		}
		for range 2 {
			stop := nextReloadAccounting(t, packets, radius.AcctStatusStop)
			id := string(stop.FindAttr(radius.AttrAcctSessionID))
			require.True(t, retained[id], "rollback retired an unexpected accounting lifetime")
			delete(retained, id)
			require.Equal(t, []byte{0, 0, 0, byte(l2tpevents.TerminateCauseAdminReboot)}, stop.FindAttr(radius.AttrAcctTerminateCause))
		}
	})
}
