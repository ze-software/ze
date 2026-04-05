// Design: docs/architecture/core-design.md -- BGP plugin registration with ConfigRoots
//
// Package plugin provides the BGP plugin registration for config-driven loading.
// Separated from the bgp parent package to avoid import cycles:
// bgp/server (test) -> bgp -> bgp/config -> bgp/reactor -> bgp/server.

package plugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/grmarker"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	bgpschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	zePlugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	bgpMu     sync.Mutex
	bgpBus    ze.Bus
	bgpServer *pluginserver.Server
)

func init() {
	reg := registry.Registration{
		Name:        "bgp",
		Description: "BGP routing daemon",
		Features:    "yang",
		YANG:        bgpschema.ZeBGPConfYANG,
		ConfigRoots: []string{"bgp"},
		RunEngine:   runBGPEngine,
		ConfigureEngineLogger: func(loggerName string) {
			_ = loggerName // BGP uses its own lazy loggers
		},
		ConfigureBus: func(bus any) {
			if b, ok := bus.(ze.Bus); ok {
				bgpMu.Lock()
				bgpBus = b
				bgpMu.Unlock()
			}
		},
		ConfigurePluginServer: func(server any) {
			if s, ok := server.(*pluginserver.Server); ok {
				bgpMu.Lock()
				bgpServer = s
				bgpMu.Unlock()
			}
		},
		CLIHandler: func(_ []string) int {
			return 1
		},
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "bgp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// runBGPEngine is the engine-mode entry point for the BGP plugin.
// It retrieves the pre-parsed config from bgpconfig.GetLoadContext(),
// creates the reactor, wires it to the hub-owned plugin server,
// and blocks until shutdown.
func runBGPEngine(conn net.Conn) int {
	log := slogutil.Logger("bgp.plugin")
	log.Debug("bgp plugin starting")

	p := sdk.NewWithConn("bgp", conn)
	defer func() { _ = p.Close() }()

	var bgpReactor *reactor.Reactor

	p.OnConfigure(func(_ []sdk.ConfigSection) error {
		// Config sections come through the SDK, but we use the pre-parsed
		// Tree from the hub (stored via StoreLoadContext) because the reactor
		// needs the full *config.Tree, not JSON fragments.
		loadResult, configPath, storeAny := bgpconfig.GetLoadContext()
		if loadResult == nil {
			return fmt.Errorf("bgp: no config context available")
		}

		store, ok := storeAny.(storage.Storage)
		if !ok {
			return fmt.Errorf("bgp: invalid storage type in load context")
		}

		// Create reactor from the pre-parsed config tree.
		var err error
		bgpReactor, err = bgpconfig.CreateReactor(loadResult, configPath, store)
		if err != nil {
			return fmt.Errorf("bgp: create reactor: %w", err)
		}

		// Wire reactor to hub-owned infrastructure.
		bgpMu.Lock()
		bus := bgpBus
		server := bgpServer
		bgpMu.Unlock()

		if bus != nil {
			bgpReactor.SetBus(bus)
		}
		if server != nil {
			bgpReactor.SetPluginServer(server)
		}

		// Read GR marker from storage.
		if expiry, grOK := grmarker.Read(store); grOK {
			bgpReactor.SetRestartUntil(expiry)
			log.Info("GR restart marker found", "expires", expiry)
		}
		if removeErr := grmarker.Remove(store); removeErr != nil {
			log.Warn("failed to remove GR marker", "error", removeErr)
		}

		// Register reactor with the coordinator so the plugin server's
		// ReactorLifecycle calls delegate to the real reactor.
		if server != nil {
			if coord, coordOK := server.Reactor().(*zePlugin.Coordinator); coordOK {
				coord.SetReactor(bgpReactor.ReactorLifecycleAdapter())
			}
		}

		// Start reactor asynchronously. OnConfigure must return quickly
		// so the plugin server's stage barriers don't block other plugins.
		// The reactor runs in a goroutine; errors are logged, not returned.
		go func() {
			if startErr := bgpReactor.StartWithContext(context.Background()); startErr != nil {
				log.Error("bgp reactor start failed", "error", startErr)
				return
			}
			log.Info("bgp reactor started")
		}()

		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		log.Error("bgp plugin failed", "error", err)
		return 1
	}

	// Wait for reactor shutdown if it was started.
	if bgpReactor != nil {
		_ = bgpReactor.Wait(context.Background())
	}

	return 0
}
