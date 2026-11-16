// Design: docs/architecture/api/process-protocol.md -- initialized startup tiers
// Related: startup_driver.go -- Stage 5 joins declared local initialization

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

// The actual SDK activates its bridge before entering OnStarted. A dependent
// tier must nevertheless remain blocked until the normal callback loop begins.
// The socket control proves the same receipt without the bridge optimization.
func TestStartupJoinsDeclaredInitializer(t *testing.T) {
	for _, internal := range []bool{false, true} {
		name := "socket"
		if internal {
			name = "bridge"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				engineEnd, pluginEnd := net.Pipe()
				engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
				defer func() { _ = engineMux.Close() }()
				connection := ipc.NewMuxPluginConn(engineMux)
				var bridge *rpc.DirectBridge
				endpoint := pluginEnd
				if internal {
					bridge = rpc.NewDirectBridge()
					endpoint = rpc.NewBridgedConn(pluginEnd, bridge)
				}
				api := sdk.NewWithConn("widget", endpoint)
				defer func() { _ = api.Close() }()
				entered := make(chan struct{})
				release := make(chan struct{})
				api.OnStarted(func(ctx context.Context) error {
					close(entered)
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				pluginDone := make(chan error, 1)
				go func() { pluginDone <- api.Run(ctx, &sdk.Registration{}) }()
				sink := &initializedStartupSink{pc: connection, bridge: bridge}
				startupDone := make(chan error, 1)
				go func() { startupDone <- runStartupHandshake(ctx, sink) }()
				<-entered
				synctest.Wait()
				if !sink.declared {
					t.Fatal("SDK initializer was not declared at Stage 1")
				}
				if internal {
					if !bridge.Activated() {
						t.Fatal("control did not reach activated bridge")
					}
				}
				select {
				case err := <-startupDone:
					t.Fatalf("startup completed before initialization: %v", err)
				default:
				}
				close(release)
				if err := <-startupDone; err != nil {
					t.Fatal(err)
				}
				cancel()
				<-pluginDone
			})
		})
	}
}

type initializedStartupSink struct {
	recordingSink
	bridge   *rpc.DirectBridge
	declared bool
}

func (s *initializedStartupSink) onRegistration(input *rpc.DeclareRegistrationInput) error {
	s.declared = input.StartedHandler
	return s.recordingSink.onRegistration(input)
}

func (s *initializedStartupSink) publishTransport(connection *ipc.PluginConn, _ *rpc.ReadyInput) {
	if s.bridge == nil {
		return
	}
	// This test starts no metrics publisher and its initializer makes no engine
	// calls. An unexpected operation must refuse rather than fake success.
	s.bridge.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return nil, rpc.ErrBridgeClosed
	})
	connection.SetBridge(s.bridge)
}

// An external initializer can call ordinary engine operations before its
// callback loop starts. The real ownership refusal proves the registry answered
// the request; merely answering every nested request OK would fail this test.
func TestStartupInitializerEngineCallsProgress(t *testing.T) {
	for _, internal := range []bool{false, true} {
		name := "socket"
		if internal {
			name = "bridge"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				engineEnd, pluginEnd := net.Pipe()
				mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
				defer func() { _ = mux.Close() }()
				connection := ipc.NewMuxPluginConn(mux)
				var bridge *rpc.DirectBridge
				endpoint := pluginEnd
				if internal {
					bridge = rpc.NewDirectBridge()
					endpoint = rpc.NewBridgedConn(pluginEnd, bridge)
				}
				api := sdk.NewWithConn("widget", endpoint)
				defer func() { _ = api.Close() }()
				api.OnStarted(func(ctx context.Context) error {
					_, _, err := api.StateGet(ctx, zefs.KeyOSPFAuthBootCount.Key())
					if err == nil {
						return fmt.Errorf("ungranted initializer state read succeeded")
					}
					if !strings.Contains(err.Error(), "holds no grant") {
						return fmt.Errorf("initializer state read did not reach the engine ownership check: %w", err)
					}
					return nil
				})
				pluginDone := make(chan error, 1)
				go func() { defer func() { _ = api.Close() }(); pluginDone <- api.Run(ctx, &sdk.Registration{}) }()
				owner := &engineStartupSink{s: &Server{ctx: ctx}, proc: process.NewProcess(plugin.PluginConfig{Name: "widget"})}
				sink := &initializingEngineSink{pc: connection, bridge: bridge, runtime: owner}
				if err := runStartupHandshake(ctx, sink); err != nil {
					t.Fatal(err)
				}
				cancel()
				if err := <-pluginDone; err != nil {
					if !strings.Contains(err.Error(), "canceled") {
						t.Fatal(err)
					}
				}
			})
		})
	}
}

type initializingEngineSink struct {
	initializedStartupSink
	runtime *engineStartupSink
}

func (s *initializingEngineSink) runStartupWire(ctx context.Context, stage plugin.PluginStage, allowance int, operation func(context.Context) error) error {
	return s.runtime.runStartupWire(ctx, stage, allowance, operation)
}
func (s *initializingEngineSink) serveInitializingRPC(ctx context.Context, connection *ipc.PluginConn, request *rpc.Request) error {
	return s.runtime.serveInitializingRPC(ctx, connection, request)
}
func (s *initializingEngineSink) publishTransport(connection *ipc.PluginConn, _ *rpc.ReadyInput) {
	if s.bridge == nil {
		return
	}
	s.bridge.SetDispatchRPC(func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		return s.runtime.s.dispatchPluginRPCDirect(ctx, s.runtime.proc, method, params)
	})
	connection.SetBridge(s.bridge)
}

// Initialization's accepted allowance and an explicit operator timeout are
// different authorities. Parent cancellation remains effective in both cases.
func TestStartupInitializationUsesAcceptedAllowance(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		parent, cancelParent := context.WithCancel(context.Background())
		ctx, cancel := startupWireContext(parent, time.Second, explicit, plugin.StageRunning, 120)
		deadline, present := ctx.Deadline()
		if !present {
			t.Fatal("initialization has no deadline")
		}
		want := 120 * time.Second
		if explicit {
			want = time.Second
		}
		remaining := time.Until(deadline)
		if remaining > want || remaining < want-time.Second/2 {
			t.Fatalf("explicit=%t allowance=%s want %s", explicit, remaining, want)
		}
		cancelParent()
		if ctx.Err() != context.Canceled {
			t.Fatalf("parent cancellation = %v", ctx.Err())
		}
		cancel()
	}
}

// Tier progress renews the existing inactivity window while a wire operation
// is pending. A hard per-plugin deadline would fail the positive case.
func TestStartupWireWaitTracksTierProgress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		coordinator := plugin.NewStartupCoordinator(2)
		owner := &engineStartupSink{s: &Server{coordinator: coordinator}, proc: process.NewProcess(plugin.PluginConfig{Name: "widget", StageTimeout: time.Second})}
		release := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- owner.runStartupWire(ctx, plugin.StageRegistration, 0, func(ctx context.Context) error {
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}()
		synctest.Wait()
		<-time.After(750 * time.Millisecond)
		coordinator.StageComplete(0, plugin.StageRegistration)
		<-time.After(750 * time.Millisecond)
		close(release)
		if err := <-done; err != nil {
			t.Fatalf("live tier progress was truncated: %v", err)
		}
	})
}

// The initializer's failure must refuse startup on both transports, even
// though ready was acknowledged and the direct bridge may already be active.
func TestStartupInitializerFailureRefusesReceipt(t *testing.T) {
	for _, internal := range []bool{false, true} {
		name := "socket"
		if internal {
			name = "bridge"
		}
		t.Run(name, func(t *testing.T) { testInitializerEnds(t, internal, false) })
	}
}

// Cancellation joins the same initializer lifetime and never credits a queued
// callback as successful initialization.
func TestStartupInitializerCancellationRefusesReceipt(t *testing.T) {
	for _, internal := range []bool{false, true} {
		name := "socket"
		if internal {
			name = "bridge"
		}
		t.Run(name, func(t *testing.T) { testInitializerEnds(t, internal, true) })
	}
}

func testInitializerEnds(t *testing.T, internal, canceled bool) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		engineEnd, pluginEnd := net.Pipe()
		mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
		defer func() { _ = mux.Close() }()
		connection := ipc.NewMuxPluginConn(mux)
		var bridge *rpc.DirectBridge
		endpoint := pluginEnd
		if internal {
			bridge = rpc.NewDirectBridge()
			endpoint = rpc.NewBridgedConn(pluginEnd, bridge)
		}
		api := sdk.NewWithConn("widget", endpoint)
		defer func() { _ = api.Close() }()
		entered, release := make(chan struct{}), make(chan struct{})
		failure := errors.New("initializer refused its runtime resources")
		api.OnStarted(func(ctx context.Context) error {
			close(entered)
			select {
			case <-release:
				return failure
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		pluginDone := make(chan error, 1)
		go func() { defer func() { _ = api.Close() }(); pluginDone <- api.Run(ctx, &sdk.Registration{}) }()
		sink := &initializedStartupSink{pc: connection, bridge: bridge}
		startupDone := make(chan error, 1)
		go func() { startupDone <- runStartupHandshake(ctx, sink) }()
		<-entered
		synctest.Wait()
		select {
		case err := <-startupDone:
			t.Fatalf("startup completed before initialization ended: %v", err)
		default:
		}
		if canceled {
			cancel()
		} else {
			close(release)
		}
		if err := <-startupDone; err == nil {
			t.Fatal("failed initialization produced a startup receipt")
		}
		want := failure
		if canceled {
			want = context.Canceled
		}
		if err := <-pluginDone; !errors.Is(err, want) {
			t.Fatalf("initializer result = %v, want %v", err, want)
		}
	})
}

// A tier with no progress remains bounded even when it has only one plugin,
// so removing the hub's fixed deadline cannot create an endless wire wait.
func TestStartupWireWaitRejectsStalledTier(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		coordinator := plugin.NewStartupCoordinator(1)
		owner := &engineStartupSink{s: &Server{coordinator: coordinator}, proc: process.NewProcess(plugin.PluginConfig{Name: "widget", StageTimeout: time.Second})}
		err := owner.runStartupWire(context.Background(), plugin.StageRegistration, 0, func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
		if err == nil {
			t.Fatal("stalled single-plugin tier had no failure")
		}
		if !strings.Contains(err.Error(), "no plugin progress") {
			t.Fatalf("stalled tier error = %v", err)
		}
	})
}
