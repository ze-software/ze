// Design: docs/guide/bmp.md -- Loc-RIB monitoring requires its runtime feed.
// Related: bmp.go -- Stage-2 configure and reload verification callbacks.
package bmp

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRuntimeLocRIBStartupRefusesWithoutBus requires a real Stage-2 refusal
// before a receiver root delivered ahead of the sender can open its listener.
func TestRuntimeLocRIBStartupRefusesWithoutBus(t *testing.T) {
	withoutLocRIBBus(t)
	bp, _, err := startBMPRuntimeConfig(t, []sdk.ConfigSection{
		{Root: configRootEnvironment, Data: runtimeReceiverJSON},
		{Root: configRootBGP, Data: senderJSON(t, map[string]any{"loc-rib": yangTrue})},
	})
	if err == nil {
		t.Fatal("runtime configure accepted loc-rib true without its event bus")
	}
	requireLocRIBRuntimeCause(t, err)
	bp.mu.RLock()
	listeners := len(bp.listeners)
	identity := bp.identity
	bp.mu.RUnlock()
	if listeners != 0 || identity != nil {
		t.Fatalf("refused configure changed receiver/identity: listeners=%d identity=%v", listeners, identity)
	}
}

// TestRuntimeLocRIBReloadRefusesWithoutBus checks the runtime transaction
// boundary, rather than the pure JSON parser or the sender's apply helper.
func TestRuntimeLocRIBReloadRefusesWithoutBus(t *testing.T) {
	withoutLocRIBBus(t)
	bp, engine, err := startBMPRuntimeConfig(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = engine.reloadBGP(senderJSON(t, map[string]any{"loc-rib": yangTrue}))
	if err == nil {
		t.Fatal("runtime reload accepted loc-rib true without its event bus")
	}
	requireLocRIBRuntimeCause(t, err)
	bp.mu.RLock()
	identity := bp.identity
	subscribed := bp.locRIBUnsub != nil
	bp.mu.RUnlock()
	if identity != nil || subscribed {
		t.Fatalf("refused reload applied the sender: identity=%v subscribed=%v", identity, subscribed)
	}
}

// TestRuntimeLocRIBWithBusSubscribesAndReplays preserves the useful internal
// path on both runtime config rails: a real replay request reaches the bus.
func TestRuntimeLocRIBWithBusSubscribesAndReplays(t *testing.T) {
	for _, rail := range []string{"startup", "reload"} {
		t.Run(rail, func(t *testing.T) {
			previous := eventBusPtr.Load()
			bus := newLocRIBTestBus()
			setEventBus(bus)
			t.Cleanup(func() { eventBusPtr.Store(previous) })
			var requests atomic.Int32
			unsubscribe := ribevents.ReplayRequest.Subscribe(bus, func(*replay.Request) { requests.Add(1) })
			t.Cleanup(unsubscribe)
			section := sdk.ConfigSection{Root: configRootBGP, Data: senderJSON(t, map[string]any{"loc-rib": yangTrue})}
			var sections []sdk.ConfigSection
			if rail == "startup" {
				sections = []sdk.ConfigSection{section}
			}
			bp, engine, err := startBMPRuntimeConfig(t, sections)
			if err != nil {
				t.Fatal(err)
			}
			if rail == "reload" {
				if err := engine.reloadBGP(section.Data); err != nil {
					t.Fatal(err)
				}
			}
			bp.mu.RLock()
			subscribed := bp.locRIBUnsub != nil
			bp.mu.RUnlock()
			if !subscribed || requests.Load() != 1 {
				t.Fatalf("runtime Loc-RIB subscription=%v replay requests=%d, want true and one", subscribed, requests.Load())
			}
		})
	}
}

// TestRuntimeBMPWithoutLocRIBNeedsNoBus keeps receiver-only and ordinary
// sender configuration available when neither requests the Loc-RIB feed.
func TestRuntimeBMPWithoutLocRIBNeedsNoBus(t *testing.T) {
	for _, mode := range []string{"receiver", "sender"} {
		t.Run(mode, func(t *testing.T) {
			withoutLocRIBBus(t)
			section := sdk.ConfigSection{Root: configRootEnvironment, Data: runtimeReceiverJSON}
			if mode == "sender" {
				section = sdk.ConfigSection{Root: configRootBGP, Data: senderJSON(t, map[string]any{"loc-rib": "false"})}
			}
			bp, _, err := startBMPRuntimeConfig(t, []sdk.ConfigSection{section})
			if err != nil {
				t.Fatal(err)
			}
			bp.mu.RLock()
			listeners := len(bp.listeners)
			subscribed := bp.locRIBUnsub != nil
			bp.mu.RUnlock()
			if subscribed {
				t.Fatal("mode without Loc-RIB installed a bus subscription")
			}
			if mode == "receiver" && listeners != 1 {
				t.Fatalf("receiver-only mode has %d listeners, want one", listeners)
			}
		})
	}
}

// TestOfflineSenderParseDoesNotRequireRuntimeBus preserves offline parsing;
// the daemon's runtime admission owns availability of the live RIB feed.
func TestOfflineSenderParseDoesNotRequireRuntimeBus(t *testing.T) {
	withoutLocRIBBus(t)
	cfg, err := parseSenderConfig(senderJSON(t, map[string]any{"loc-rib": yangTrue}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LocRIB != yangTrue {
		t.Fatalf("offline loc-rib=%q, want true", cfg.LocRIB)
	}
}

const runtimeReceiverJSON = `{"environment":{"bmp":{"enabled":"true","server":{"test":{"ip":"127.0.0.1","port":"0"}}}}}`

// withoutLocRIBBus is serial: callers MUST NOT run in parallel because the
// package-level runtime bus is shared with the other BMP test fixtures.
func withoutLocRIBBus(t *testing.T) {
	t.Helper()
	previous := eventBusPtr.Load()
	eventBusPtr.Store(nil)
	t.Cleanup(func() { eventBusPtr.Store(previous) })
}

func requireLocRIBRuntimeCause(t *testing.T, err error) {
	t.Helper()
	for _, part := range []string{"Loc-RIB", "event bus", "plugin { internal"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("runtime refusal %q does not name %q", err, part)
		}
	}
}

// startBMPRuntimeConfig drives the real SDK startup callbacks and returns a
// Stage-2 refusal to the test. Callers MUST let its registered cleanup stop
// listeners, unsubscribe, and join the plugin before restoring a shared bus.
func startBMPRuntimeConfig(t *testing.T, sections []sdk.ConfigSection) (*BMPPlugin, *reloadEngine, error) {
	t.Helper()
	pluginSide, engineSide := net.Pipe()
	plugin := sdk.NewWithConn("bgp-bmp", pluginSide)
	mux := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
	bp := &BMPPlugin{plugin: plugin, state: newBMPState(), stopCh: make(chan struct{})}
	bp.registerCallbacks()
	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- plugin.Run(ctx, &sdk.Registration{WantsConfig: []string{configRootBGP, configRootEnvironment}})
	}()
	t.Cleanup(func() {
		close(bp.stopCh)
		bp.stopLocRIB()
		bp.stopSenders()
		bp.stopListeners()
		bp.sessions.Wait()
		cancel()
		closeLog(mux, "runtime-engine")
		closeLog(plugin, "runtime-plugin")
		select {
		case <-runResult:
		case <-time.After(5 * time.Second):
			t.Error("runtime plugin did not stop after its transport closed")
		}
	})
	engine := &reloadEngine{ctx: ctx, mux: mux}
	engine.expectRequest(t, rpc.MethodDeclareRegistration)
	_, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", struct {
		Sections []sdk.ConfigSection `json:"sections"`
	}{Sections: sections})
	if err != nil {
		return bp, engine, err
	}
	engine.expectRequest(t, "ze-plugin-engine:declare-capabilities")
	engine.call(t, "ze-plugin-callback:share-registry", struct {
		Commands []sdk.RegistryCommand `json:"commands"`
	}{})
	engine.expectRequest(t, "ze-plugin-engine:ready")
	return bp, engine, nil
}
