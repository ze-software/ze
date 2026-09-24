// Design: docs/architecture/wire/nlri-bgpls.md -- native exporter lifecycle

package ls_export

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func verifyExporterConfig(sections []rpc.ConfigSection) error {
	_, err := parseExportConfig(sections)
	return err
}

func runTopologyExporter(conn net.Conn) int {
	p := sdk.NewWithConn(exporterName, conn)
	defer func() { _ = p.Close() }()
	bus := registry.GetEventBus()
	if bus == nil {
		exportLogger.Error("native BGP-LS exporter requires the engine event bus; configure it as an internal plugin")
		return 1
	}
	exporter := newTopologyExporter(p)
	defer exporter.stopWorker()
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	// Every subscription lives until the plugin returns, so the one deferred
	// cleanup below releases them all.
	sources := linkstateevents.Sources()
	unsubscribes := make([]func(), 0, len(sources))
	defer func() {
		for _, unsubscribe := range unsubscribes {
			unsubscribe()
		}
	}()
	for _, namespace := range sources {
		handle := events.Register[*linkstateevents.Snapshot](namespace, linkstateevents.EventType)
		unsubscribe := handle.Subscribe(bus, func(snapshot *linkstateevents.Snapshot) {
			if err := exporter.replace(namespace, snapshot); err != nil {
				exportLogger.Error("native BGP-LS snapshot refused", "source", namespace, "error", err)
			}
		})
		unsubscribes = append(unsubscribes, unsubscribe)
	}
	request := func() error { _, err := linkstateevents.Request.Emit(bus); return err }
	var configMu sync.Mutex
	var current, candidate, previous exportConfig
	var staged, applied bool
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		enabled, err := parseExportConfig(sections)
		if err != nil {
			return err
		}
		configMu.Lock()
		candidate, staged, applied = enabled, true, false
		configMu.Unlock()
		return nil
	})
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		enabled, err := parseExportConfig(sections)
		if err != nil {
			return err
		}
		configMu.Lock()
		current = enabled
		configMu.Unlock()
		exporter.resume(ctx, enabled, request)
		return nil
	})
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		configMu.Lock()
		if !staged {
			configMu.Unlock()
			return errors.New("BGP-LS export apply has no verified configuration")
		}
		previous, current, staged, applied = current, candidate, false, true
		enabled := current
		configMu.Unlock()
		exporter.resume(ctx, enabled, request)
		return nil
	})
	p.OnConfigRollback(func(_ string) error {
		configMu.Lock()
		if applied {
			current, applied = previous, false
		}
		staged = false
		enabled := current
		configMu.Unlock()
		exporter.resume(ctx, enabled, request)
		return nil
	})
	p.SetStartupSubscriptions([]string{"state", "refresh"}, []string{"*"}, "json")
	p.OnEvent(func(payload string) error {
		event, err := bgp.ParseEvent([]byte(payload))
		if err != nil {
			return err
		}
		peer := event.GetPeerAddress()
		if _, err := netip.ParseAddr(peer); err != nil {
			return fmt.Errorf("BGP-LS peer state address: %w", err)
		}
		switch event.GetEventType() {
		case rpc.EventKindState:
			exporter.peerState(peer, event.GetPeerState() == "up")
		case rpc.EventKindRefresh:
			if event.AFI == ls.BGPLSFamily.AFI {
				if event.SAFI == ls.BGPLSFamily.SAFI {
					exporter.peerRefresh(peer)
				}
			}
		default:
			// Other event kinds carry no BGP-LS export state.
		}
		return nil
	})
	p.OnAllPluginsReady(func() error { return exporter.start(request) })
	p.OnBye(func(_ string) error {
		// Cancel the worker's current transport operation before cleanup owns
		// its sent maps. A refusal keeps the process and failed withdrawals
		// available for retry; the SDK ends Run only after successful cleanup.
		shutdown, finish := context.WithTimeout(context.Background(), 400*time.Millisecond)
		defer finish()
		return exporter.shutdown(shutdown)
	})
	if err := p.Run(ctx, sdk.Registration{WantsConfig: []string{exporterName}}); err != nil {
		exportLogger.Error("native BGP-LS exporter stopped", "error", err)
		return 1
	}
	return 0
}
