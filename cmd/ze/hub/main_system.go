// Design: docs/architecture/hub-architecture.md -- Host-level system config application
// Related: main.go -- orchestration, main_servers.go -- server setup

package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/host"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/cymru"
	resolveDNS "codeberg.org/thomas-mangin/ze/internal/component/resolve/dns"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/irr"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/peeringdb"
	"codeberg.org/thomas-mangin/ze/internal/component/telemetry/collector"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/privilege"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysctlevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// dropPrivileges drops to the user/group from ze.user/ze.group env vars.
// Called after port binding, before accepting connections or spawning plugins.
// No-op if not running as root or if ze.user is not set.
// Warns if running as root without ze.user configured.
func dropPrivileges() error {
	cfg := privilege.DropConfigFromEnv()
	if cfg.User == "" {
		if os.Getuid() == 0 {
			fmt.Fprintln(os.Stderr, "warning: running as root, set ze.user to drop privileges")
		}
		return nil
	}
	return privilege.Drop(cfg)
}

// monitorStdinEOF blocks until stdin is closed (EOF or error), then sends
// SIGTERM to sigCh to trigger reactor shutdown.
func monitorStdinEOF(sigCh chan<- os.Signal) {
	b := make([]byte, 1)
	if _, err := os.Stdin.Read(b); err != nil {
		fmt.Fprintf(os.Stderr, "\nUpstream pipe closed (%v), shutting down...\n", err)
	} else {
		fmt.Fprintln(os.Stderr, "\nUpstream pipe closed, shutting down...")
	}
	select {
	case sigCh <- syscall.SIGTERM:
	default:
	}
}

// newResolvers creates a shared Resolvers struct with a single DNS instance
// and a Cymru resolver wired to it. Called once at hub startup.
func newResolvers(sc *system.SystemConfig) *resolve.Resolvers {
	cfg := resolveDNS.ResolverConfig{
		Timeout:        sc.DNSTimeout,
		ResolvConfPath: sc.ResolvConfPath,
		CacheSize:      sc.DNSCacheSize,
		CacheTTL:       sc.DNSCacheTTL,
	}
	if len(sc.NameServers) > 0 {
		cfg.Server = sc.NameServers[0]
	}

	dnsResolver := resolveDNS.NewResolver(cfg)

	// Wrap DNS ResolveTXT to match Cymru's TXTResolver signature (adds context).
	txtResolver := func(_ context.Context, name string) ([]string, error) {
		return dnsResolver.ResolveTXT(name)
	}

	return &resolve.Resolvers{
		DNS:       dnsResolver,
		Cymru:     cymru.New(txtResolver, nil),
		PeeringDB: peeringdb.NewPeeringDB(sc.PeeringDBURL),
		IRR:       irr.NewIRR(""),
	}
}

// applyConsole configures serial console devices via termios.
// Best-effort: logs warnings on failure or getty conflict, never blocks startup.
func applyConsole(sc *system.SystemConfig) {
	if len(sc.ConsoleDevices) == 0 {
		return
	}
	result := system.ApplyConsole(sc.ConsoleDevices)
	for _, applied := range result.Applied {
		slogutil.Logger("console").Info("serial console configured", "device", applied)
	}
	for _, skip := range result.Skipped {
		slogutil.Logger("console").Warn("serial console skipped", "device", skip.Device, "reason", skip.Reason)
	}
	for _, ce := range result.Errors {
		slogutil.Logger("console").Warn("serial console failed", "device", ce.Device, "error", ce.Err)
	}
}

// applyHostTuning extracts tuning config and applies it. Errors are
// logged as warnings (tuning is best-effort, never blocks startup).
func applyHostTuning(sc *system.SystemConfig) {
	cfg := sc.Tuning.ToHostTuningConfig()
	if cfg.CPUGovernor == "" && len(cfg.IRQAffinity) == 0 && len(cfg.Ethtool) == 0 {
		return
	}
	result := host.ApplyTuning(cfg)
	for _, applied := range result.Applied {
		slogutil.Logger("host").Info("tuning applied", "op", applied)
	}
	for _, te := range result.Errors {
		slogutil.Logger("host").Warn("tuning failed", "op", te.Operation, "subject", te.Subject, "error", te.Err)
	}
}

// applyHostTuningFromMap extracts tuning from a map tree (reload path)
// and applies it.
func applyHostTuningFromMap(tree map[string]any) {
	tc := system.ExtractTuningFromMap(tree)
	cfg := tc.ToHostTuningConfig()
	if cfg.CPUGovernor == "" && len(cfg.IRQAffinity) == 0 && len(cfg.Ethtool) == 0 {
		return
	}
	result := host.ApplyTuning(cfg)
	for _, applied := range result.Applied {
		slogutil.Logger("host").Info("tuning applied (reload)", "op", applied)
	}
	for _, te := range result.Errors {
		slogutil.Logger("host").Warn("tuning failed (reload)", "op", te.Operation, "subject", te.Subject, "error", te.Err)
	}
}

// applyConsoleFromMap extracts console config from a map tree (reload path)
// and applies it.
func applyConsoleFromMap(tree map[string]any) {
	devices := system.ExtractConsoleFromMap(tree)
	if len(devices) == 0 {
		return
	}
	result := system.ApplyConsole(devices)
	for _, applied := range result.Applied {
		slogutil.Logger("console").Info("serial console configured (reload)", "device", applied)
	}
	for _, skip := range result.Skipped {
		slogutil.Logger("console").Warn("serial console skipped (reload)", "device", skip.Device, "reason", skip.Reason)
	}
	for _, ce := range result.Errors {
		slogutil.Logger("console").Warn("serial console failed (reload)", "device", ce.Device, "error", ce.Err)
	}
}

// applyConntrack loads conntrack modules and sends sysctl values to the sysctl
// plugin via EventBus. Best-effort: logs warnings on failure, never blocks startup.
func applyConntrack(sc *system.SystemConfig, eb ze.EventBus) {
	cc := sc.Conntrack
	if !cc.HasConfig() {
		return
	}
	applyConntrackConfig(&cc, eb)
}

// applyConntrackFromMap extracts conntrack config from a map tree (reload path)
// and applies it.
func applyConntrackFromMap(tree map[string]any, eb ze.EventBus) {
	cc := system.ExtractConntrackFromMap(tree)
	if !cc.HasConfig() {
		return
	}
	applyConntrackConfig(&cc, eb)
}

type conntrackSysctlEvent struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

func applyConntrackConfig(cc *system.ConntrackConfig, eb ze.EventBus) {
	log := slogutil.Logger("conntrack")

	if err := cc.ValidateModules(); err != nil {
		log.Warn("conntrack module validation failed", "error", err)
	} else if len(cc.Modules) > 0 {
		loaded, errs := system.LoadConntrackModules(cc.Modules)
		for _, m := range loaded {
			log.Info("conntrack module loaded", "module", m)
		}
		for _, err := range errs {
			log.Warn("conntrack module load failed", "error", err)
		}
	}

	if eb == nil {
		return
	}
	keys := cc.ConntrackSysctlKeys()
	for key, value := range keys {
		payload, _ := json.Marshal(conntrackSysctlEvent{Key: key, Value: value, Source: "system-conntrack"})
		if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload)); err != nil {
			log.Warn("conntrack sysctl emit failed", "key", key, "error", err)
		}
	}
	if len(keys) > 0 {
		log.Info("conntrack sysctl values emitted", "keys", len(keys))
	}
}

// parseASNForDecorator converts an ASN string to uint32 for the Cymru resolver.
// Returns 0 on parse failure (Cymru handles ASN 0 gracefully).
func parseASNForDecorator(asn string) uint32 {
	var n uint64
	for _, c := range asn {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + uint64(c-'0')
		if n > 4294967295 {
			return 0
		}
	}
	return uint32(n)
}

// startStandaloneTelemetry starts the Prometheus metrics server when
// telemetry{} is configured but bgp{} is absent. Mirrors the startup
// logic in loader_create.go but without requiring a reactor.
type standaloneTelemetry struct {
	srv     metrics.Server
	manager *collector.Manager
}

func startStandaloneTelemetry(tree *zeconfig.Tree) *standaloneTelemetry {
	telemetryCfg := metrics.ExtractTelemetryConfig(tree.ToMap())
	if !telemetryCfg.Enabled {
		return nil
	}

	st := &standaloneTelemetry{}
	reg := metrics.NewPrometheusRegistry()
	if err := st.srv.Start(reg, telemetryCfg); err != nil {
		slog.Warn("standalone telemetry: metrics server failed to start", "error", err)
		return nil
	}
	for _, path := range telemetryCfg.DeprecatedAliases {
		slog.Warn("standalone telemetry: deprecated prometheus config; move setting under telemetry.prometheus.netdata", "path", path)
	}
	for _, ep := range telemetryCfg.Endpoints {
		slog.Info("standalone telemetry: prometheus metrics enabled",
			"address", ep.Host, "port", ep.Port, "path", telemetryCfg.Path)
	}

	if telemetryCfg.Netdata.Enabled {
		overrides := make(map[string]collector.CollectorOverride, len(telemetryCfg.Netdata.Collectors))
		for name, cc := range telemetryCfg.Netdata.Collectors {
			overrides[name] = collector.CollectorOverride{
				Enabled:  cc.Enabled,
				Interval: time.Duration(cc.Interval) * time.Second,
			}
		}
		st.manager = collector.StartOSCollectors(reg, telemetryCfg.Netdata.Prefix, time.Duration(telemetryCfg.Netdata.Interval)*time.Second, overrides, slog.Default())
	}
	return st
}
