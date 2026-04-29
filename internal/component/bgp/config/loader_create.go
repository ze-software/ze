// Design: docs/architecture/config/syntax.md — reactor creation from config tree
// Overview: loader.go — config loading pipeline
// Related: infra_hook.go -- infrastructure setup hook types and callback

package bgpconfig

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos"
	coreenv "codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/family"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/telemetry/collector"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// ze.test.bgp.port is a runtime-only env var for the test infrastructure.
// It creates a global listener so the ze-test peer can connect to ze.
const envKeyTCPPort = "ze.test.bgp.port"

// initRedistribute parses redistribute import rules from the config tree
// and installs the global evaluator. Called during reactor creation.
// Non-fatal: logs a warning if parsing fails (redistribute is optional).
func initRedistribute(tree *config.Tree) {
	rules, err := config.ExtractRedistributeRules(tree)
	if err != nil {
		slogutil.Logger("config").Warn("redistribute config error", "error", err)
		return
	}
	if len(rules) > 0 {
		redistribute.SetGlobal(redistribute.NewEvaluator(rules))
	}
}

var _ = coreenv.MustRegister(coreenv.EnvEntry{
	Key:         envKeyTCPPort,
	Type:        "int",
	Default:     "",
	Description: "BGP listen port (test infrastructure)",
	Private:     true,
})

// CreateReactorFromTree creates a Reactor directly from a parsed config tree.
func CreateReactorFromTree(tree *config.Tree, configDir, configPath string, plugins []reactor.PluginConfig, store storage.Storage) (*reactor.Reactor, error) {
	// Pruning + env plumbing already happened in the top-level loader
	// (config.ParseTreeWithYANG calls PruneInactive -> ApplyEnvConfig). The
	// BGP reactor consumes the pruned tree directly; no second extraction.
	pruneSchema, err := config.YANGSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	_ = pruneSchema // kept available for listener validation below

	// Extract global BGP settings directly from tree
	var routerID uint32
	var localAS uint32

	if bgpContainer := tree.GetContainer("bgp"); bgpContainer != nil {
		if v, ok := bgpContainer.Get("router-id"); ok {
			if ip, parseErr := netip.ParseAddr(v); parseErr == nil {
				routerID = ipToUint32(ip)
			}
		}
		if localContainer := bgpContainer.GetContainer("local"); localContainer != nil {
			if v, ok := localContainer.Get("as"); ok {
				if n, parseErr := strconv.ParseUint(v, 10, 32); parseErr == nil {
					localAS = uint32(n)
				}
			}
		}
	}

	// Build peers from tree (resolves templates, extracts routes).
	// Incomplete peers (missing required fields) are skipped so the daemon
	// can start for config editing with partial configs. Hard validation
	// errors (unknown family, invalid address) still fail.
	peers, err := PeersFromConfigTree(tree)
	if err != nil {
		if errors.Is(err, reactor.ErrIncompleteConfig) {
			slogutil.Logger("bgp.reactor").Warn("skipping peer with incomplete config", "error", err)
			peers = nil // continue with no peers
		} else {
			return nil, fmt.Errorf("build peers: %w", err)
		}
	}

	// Parse and install redistribution import rules (optional, non-fatal).
	initRedistribute(tree)

	// Validate plugin references
	if err := ValidatePluginReferences(tree, plugins); err != nil {
		return nil, fmt.Errorf("validate plugin references: %w", err)
	}

	// Validate listener port conflicts across all services.
	listeners := config.CollectListeners(tree, pruneSchema)
	if err := config.ValidateListenerConflicts(listeners); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	// Derive ConfiguredFamilies from peer capabilities.
	// Multiprotocol capabilities declare which families each peer supports.
	var configuredFamilies []string
	familySeen := make(map[string]bool)
	for _, ps := range peers {
		for _, cap := range ps.Capabilities {
			if mp, ok := cap.(*capability.Multiprotocol); ok {
				fam := family.Family{AFI: mp.AFI, SAFI: mp.SAFI}
				fs := fam.String()
				if !familySeen[fs] {
					familySeen[fs] = true
					configuredFamilies = append(configuredFamilies, fs)
				}
			}
		}
	}

	// Derive ConfiguredCustomEvents from peer process receive bindings.
	// Custom event types (e.g., "update-rpki") trigger auto-loading of producing plugins.
	var configuredCustomEvents []string
	customEventSeen := make(map[string]bool)
	for _, ps := range peers {
		for _, pb := range ps.ProcessBindings {
			for et := range pb.ReceiveCustom {
				if !customEventSeen[et] {
					customEventSeen[et] = true
					configuredCustomEvents = append(configuredCustomEvents, et)
				}
			}
		}
	}

	// Derive ConfiguredCustomSendTypes from peer process send bindings.
	// Custom send types (e.g., "enhanced-refresh") trigger auto-loading of enabling plugins.
	var configuredCustomSendTypes []string
	customSendSeen := make(map[string]bool)
	for _, ps := range peers {
		for _, pb := range ps.ProcessBindings {
			for st := range pb.SendCustom {
				if !customSendSeen[st] {
					customSendSeen[st] = true
					configuredCustomSendTypes = append(configuredCustomSendTypes, st)
				}
			}
		}
	}

	// Extract hub config for TLS plugin transport.
	hubConfig, hubErr := config.ExtractHubConfig(tree)
	if hubErr != nil {
		return nil, fmt.Errorf("hub config: %w", hubErr)
	}
	// Convert to pointer: nil when not configured (no servers).
	var hubPtr *plugin.HubConfig
	if len(hubConfig.Servers) > 0 {
		hubPtr = &hubConfig
	}

	// Build reactor config
	reactorCfg := &reactor.Config{
		// No global ListenAddr -- Ze derives listeners from per-peer connection > local.
		RouterID:                  routerID,
		LocalAS:                   localAS,
		ConfigDir:                 configDir,
		ConfigTree:                tree.ToMap(),
		ConfiguredFamilies:        configuredFamilies,
		ConfiguredCustomEvents:    configuredCustomEvents,
		ConfiguredCustomSendTypes: configuredCustomSendTypes,
		ConfiguredPaths:           config.CollectContainerPaths(tree),
		Plugins:                   plugins,
		Hub:                       hubPtr,
		RecentUpdateMax:           coreenv.GetInt("ze.bgp.reactor.cache-max", 1000000),
	}

	r := reactor.New(reactorCfg)

	// Start pprof HTTP server from ze.pprof env var (set by ApplyEnvConfig
	// from the YANG `environment/pprof` leaf, or directly by the operator).
	// CLI --pprof flag takes precedence (started earlier in main.go).
	if addr := coreenv.Get("ze.pprof"); addr != "" {
		startPprofServer(addr)
	}

	// Start Prometheus metrics HTTP server from telemetry config block.
	// Creates a shared registry that the reactor (and future components) register metrics into.
	// Every entry in cfg.Endpoints becomes a bound listener on the shared http.Server.
	if telemetryCfg := metrics.ExtractTelemetryConfig(tree.ToMap()); telemetryCfg.Enabled {
		reg := metrics.NewPrometheusRegistry()
		var srv metrics.Server
		if err := srv.Start(reg, telemetryCfg); err != nil {
			configLogger().Warn("metrics server failed to start", "error", err)
		} else {
			for _, path := range telemetryCfg.DeprecatedAliases {
				configLogger().Warn("deprecated prometheus telemetry config; move setting under telemetry.prometheus.netdata", "path", path)
			}
			for _, ep := range telemetryCfg.Endpoints {
				configLogger().Info("prometheus metrics enabled",
					"address", ep.Host, "port", ep.Port, "path", telemetryCfg.Path)
			}
			r.SetMetricsRegistry(reg)
			registry.SetMetricsRegistry(reg)

			if telemetryCfg.Netdata.Enabled {
				overrides := make(map[string]collector.CollectorOverride, len(telemetryCfg.Netdata.Collectors))
				for name, cc := range telemetryCfg.Netdata.Collectors {
					overrides[name] = collector.CollectorOverride{
						Enabled:  cc.Enabled,
						Interval: time.Duration(cc.Interval) * time.Second,
					}
				}
				collector.StartOSCollectors(reg, telemetryCfg.Netdata.Prefix, time.Duration(telemetryCfg.Netdata.Interval)*time.Second, overrides, configLogger())
			}
		}
	}

	// Validate authorization config (AC-8: reject undefined profile references).
	if err := ValidateAuthzConfig(tree); err != nil {
		return nil, fmt.Errorf("authorization config: %w", err)
	}

	// Extract authz profiles from config (independent of SSH).
	authzStore := extractAuthzConfig(tree)

	// Infrastructure setup: SSH server, authz, CLI wiring.
	// Delegated to the hub-provided hook to avoid bgpconfig importing
	// ssh, cli, and web packages.
	sshCfg := ExtractSSHConfig(tree)
	if infraHook != nil {
		infraHook(InfraHookParams{
			Reactor:              r,
			SSHConfig:            sshCfg,
			ConfigTree:           tree,
			AuthzStore:           authzStore,
			ConfigDir:            configDir,
			ConfigPath:           configPath,
			Store:                store,
			CollectLoginWarnings: collectPrefixWarnings,
			FormatResponseData:   formatResponseData,
			APIServer:            r.APIServer,
		})
	}

	// Inject chaos wrappers from config environment block.
	// CLI flags (--chaos-seed) override this via SetClock/SetDialer/SetListenerFactory after load.
	if seed := coreenv.GetInt64("ze.bgp.chaos.seed", 0); seed != 0 {
		resolved := chaos.ResolveSeed(seed)
		rate := chaosRateFromEnv()
		chaosLogger := slogutil.Logger("chaos")
		chaosCfg := chaos.ChaosConfig{Seed: resolved, Rate: rate, Logger: chaosLogger}
		clock, dialer, lf := chaos.NewChaosWrappers(clock.RealClock{}, &network.RealDialer{}, network.RealListenerFactory{}, chaosCfg)
		r.SetClock(clock)
		r.SetDialer(dialer)
		r.SetListenerFactory(lf)
		chaosLogger.Info("chaos self-test mode enabled (config)", "seed", resolved, "rate", rate)
	}

	// Add peers
	for _, ps := range peers {
		if err := r.AddPeer(ps); err != nil {
			return nil, fmt.Errorf("add peer %s: %w", ps.Address, err)
		}
	}

	return r, nil
}

// chaosRateFromEnv returns ze.bgp.chaos.rate as a float64.
// Falls back to 0.1 (YANG default) when unset or malformed.
func chaosRateFromEnv() float64 {
	const defaultRate = 0.1
	raw := coreenv.Get("ze.bgp.chaos.rate")
	if raw == "" {
		return defaultRate
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || v > 1 {
		return defaultRate
	}
	return v
}

// createReloadFunc creates a ReloadFunc that parses config files.
// It returns full PeerSettings to ensure reloaded peers are identical to initial load.
// Uses PeersFromConfigTree which resolves templates and extracts routes directly.
//
// Config-read fallback: mirrors the hub's initial-load path. Try the blob store
// first; if the store is blob-only (gokrazy read-only root, ze-test tmpfs)
// fall back to a direct filesystem read. Without this, SIGHUP-driven reloads
// fail with "read file/active/...: file does not exist" whenever the daemon
// was started with a filesystem path that is not a blob key.
func createReloadFunc(store storage.Storage) reactor.ReloadFunc {
	return func(configPath string) ([]*reactor.PeerSettings, error) {
		data, err := store.ReadFile(configPath)
		if err != nil && storage.IsBlobStorage(store) {
			data, err = os.ReadFile(configPath) //nolint:gosec // daemon operator supplied path
		}
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", configPath, err)
		}

		// Parse the config using YANG-derived schema.
		schema, err := config.YANGSchema()
		if err != nil {
			return nil, fmt.Errorf("YANG schema: %w", err)
		}
		p := config.NewParser(schema)
		tree, err := p.Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}

		// Update redistribute rules on reload.
		initRedistribute(tree)

		return PeersFromConfigTree(tree)
	}
}
