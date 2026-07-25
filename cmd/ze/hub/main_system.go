// Design: docs/architecture/hub-architecture.md -- Host-level system config application
// Related: main.go -- orchestration, main_servers.go -- server setup

package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/archive"
	configstorage "github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve"
	"github.com/ze-software/ze/internal/component/resolve/cymru"
	resolveDNS "github.com/ze-software/ze/internal/component/resolve/dns"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	zestorage "github.com/ze-software/ze/internal/component/storage"
	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	coreenv "github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/identity"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
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
		Timeout:          sc.DNSTimeout,
		ResolvConfPath:   sc.ResolvConfPath,
		CacheSize:        sc.DNSCacheSize,
		CacheTTL:         sc.DNSCacheTTL,
		DNSSECValidation: sc.DNSSECValidation,
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

// cymruOriginAdapter bridges cymru.CymruResolver to command.OriginResolver.
type cymruOriginAdapter struct {
	r *cymru.CymruResolver
}

func (a cymruOriginAdapter) LookupOrigin(ctx context.Context, ip string) (command.OriginResult, error) {
	o, err := a.r.LookupOrigin(ctx, ip)
	return command.OriginResult{ASN: o.ASN, Prefix: o.Prefix, Name: o.Name}, err
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

// applyResolvConf rewrites resolv.conf from the reloaded tree (reload path).
// A session commit that changes system name-server must reach the resolver
// without a daemon restart ("commit = apply + propagate"). Best-effort like
// the other reload-time system effects; no-op on non-Linux.
func applyResolvConf(parsedTree *zeconfig.Tree) {
	if parsedTree == nil {
		return
	}
	sc := system.ExtractSystemConfig(parsedTree)
	if len(sc.NameServers) == 0 {
		return
	}
	if err := system.WriteResolvConf(sc.ResolvConfPath, sc.NameServers); err != nil {
		slogutil.Logger("hub").Warn("resolv.conf write failed (reload)", "path", sc.ResolvConfPath, "err", err)
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

var (
	hubIdentityOnce  sync.Once
	hubIdentityStore identity.Storage
)

// SetIdentityStore sets the storage backend used for machine identity
// resolution. Called once at hub startup before startUpdateChecker.
func SetIdentityStore(s identity.Storage) {
	hubIdentityOnce.Do(func() { hubIdentityStore = s })
}

func startUpdateChecker(sc *system.SystemConfig) system.UpdateBackend {
	cfg := system.UpdateCheckConfig{
		URL:        sc.UpdateCheckURL,
		Interval:   sc.UpdateCheckInterval,
		SelfUpdate: sc.UpdateSelfUpdate,
	}
	return startBackend(cfg, "started")
}

func applyUpdateCheckerFromMap(tree map[string]any) {
	cfg := system.ExtractUpdateCheckFromMap(tree)
	stopBackend()
	startBackend(cfg, "reloaded")
}

var detectPlatform = sync.OnceValues(host.DetectPlatform)

func startBackend(cfg system.UpdateCheckConfig, action string) system.UpdateBackend {
	platform, err := detectPlatform()
	if err != nil {
		slogutil.Logger("update-check").Warn("platform detection failed", "error", err)
		platform = &host.PlatformInfo{Type: host.PlatformUnknown}
	}
	platformType := platform.Type
	backend, err := system.NewBackend(platformType, cfg, system.BackendOptions{
		GokrazySocketPath: coreenv.Get("ze.gokrazy.socket"),
		IdentityStore:     hubIdentityStore,
	})
	if err != nil {
		slogutil.Logger("update-check").Warn("invalid config", "error", err)
		return nil
	}
	if platformType != host.PlatformGokrazy && cfg.URL == "" {
		status := backend.Status()
		if status.StatusText == "" && status.Message == "" {
			system.SetActiveBackend(nil)
			return nil
		}
	}

	backend.Start(context.Background())
	system.SetActiveBackend(backend)

	slogutil.Logger("update-check").Info(action,
		"backend", backend.Name(), "url", cfg.URL, "interval", cfg.Interval)
	return backend
}

func stopBackend() {
	backend := system.ActiveBackend()
	system.SetActiveBackend(nil)
	if backend != nil {
		backend.Stop()
	}
}

// startStandaloneTelemetry starts the Prometheus metrics exporter when
// telemetry{} is configured but bgp{} is absent, via the always-on
// metrics.StartExporter seam. The gated exporter (//go:build ze_telemetry)
// parses the config, builds the registry, starts the HTTP listeners + Netdata
// collectors, and returns the registry; the seam is nil (and the exporter
// dropped from the binary) without ze_telemetry. Mirrors the reactor path in
// loader_create.go but without requiring a reactor.
type standaloneTelemetry struct {
	closer io.Closer
}

func startStandaloneTelemetry(tree *zeconfig.Tree) *standaloneTelemetry {
	if metrics.StartExporter == nil {
		return nil
	}
	reg, closer := metrics.StartExporter(tree.ToMap(), slog.Default())
	if reg == nil {
		return nil
	}
	registry.SetMetricsRegistry(reg)
	return &standaloneTelemetry{closer: closer}
}

var (
	archiveSchedulerMu     sync.Mutex
	archiveSchedulerCancel context.CancelFunc
)

// startArchiveScheduler launches the background scheduler for time-based
// archive triggers (daily/hourly). The scheduler goroutine stops when the
// cancel function is called (at shutdown or config reload).
func startArchiveScheduler(tree *zeconfig.Tree, configPath string, store configstorage.Storage, srv *pluginserver.Server) {
	if tree == nil {
		return
	}

	archiveSchedulerMu.Lock()
	defer archiveSchedulerMu.Unlock()

	if archiveSchedulerCancel != nil {
		archiveSchedulerCancel()
		archiveSchedulerCancel = nil
	}

	configs := archive.ExtractConfigs(tree)
	if len(configs) == 0 {
		return
	}

	var eventFn archive.EventEmitter
	if srv != nil {
		eventFn = func(_, _ string, content []byte) {
			srv.EmitEngineEvent("config", "archive", content) //nolint:errcheck // best-effort
		}
	}

	sched := archive.NewScheduler(configs, configPath, readConfigWithStorage(store, configPath), eventFn)

	parent := context.Background()
	if srv != nil {
		parent = srv.Context()
	}

	ctx, cancel := context.WithCancel(parent)
	archiveSchedulerCancel = cancel

	go sched.Run(ctx)
}

// readConfigWithStorage returns a config reader that tries blob storage
// (active version) before falling back to os.ReadFile. On gokrazy the
// config lives in blob storage; a bare os.ReadFile on the logical config
// name fails.
func readConfigWithStorage(store configstorage.Storage, configPath string) func() ([]byte, *zeconfig.Tree, error) {
	if store == nil {
		return archive.ReadConfigFromPath(configPath)
	}
	return func() ([]byte, *zeconfig.Tree, error) {
		data, err := configstorage.ReadActiveConfig(store, configPath)
		if err != nil {
			data, err = os.ReadFile(configPath) //nolint:gosec // operator-supplied path
		}
		if err != nil {
			return nil, nil, err
		}

		schema, schErr := zeconfig.YANGSchema()
		if schErr != nil {
			return nil, nil, schErr
		}

		parser := zeconfig.NewParser(schema)
		tree, parseErr := parser.Parse(string(data))
		if parseErr != nil {
			return nil, nil, parseErr
		}

		return data, tree, nil
	}
}

// stopArchiveScheduler stops the active archive scheduler.
func stopArchiveScheduler() {
	archiveSchedulerMu.Lock()
	defer archiveSchedulerMu.Unlock()

	if archiveSchedulerCancel != nil {
		archiveSchedulerCancel()
		archiveSchedulerCancel = nil
	}
}

var smartManager *zestorage.Manager

// startSmartManager extracts SMART config from the tree and starts the
// storage health manager if enabled.
func startSmartManager(tree *zeconfig.Tree) {
	cfg := extractSmartConfig(tree)
	if cfg == nil {
		return
	}
	smartManager = zestorage.NewManager(*cfg)
	smartManager.Start()
	zestorage.SetStorageManager(smartManager)
}

func stopSmartManager() {
	if smartManager != nil {
		smartManager.Stop()
	}
}

func reloadSmartManager(tree *zeconfig.Tree) {
	if tree == nil {
		return
	}
	cfg := extractSmartConfig(tree)
	if cfg == nil {
		if smartManager != nil {
			smartManager.Stop()
			smartManager = nil
			zestorage.SetStorageManager(nil)
		}
		return
	}
	if smartManager != nil {
		smartManager.Reconfigure(*cfg)
	} else {
		smartManager = zestorage.NewManager(*cfg)
		smartManager.Start()
		zestorage.SetStorageManager(smartManager)
	}
}

func extractSmartConfig(tree *zeconfig.Tree) *zestorage.Config {
	storageCfg := tree.GetContainer("storage")
	if storageCfg == nil {
		return nil
	}
	smartCfg := storageCfg.GetContainer("smart")
	if smartCfg == nil {
		return nil
	}
	enabled, ok := smartCfg.Get("enabled")
	if !ok || enabled != "true" {
		return nil
	}

	logger := slogutil.Logger("storage")
	cfg := zestorage.DefaultConfig()
	cfg.Enabled = true

	if v, ok := smartCfg.Get("check-interval"); ok {
		if secs, valid := parsePositiveInt(v); valid {
			cfg.CheckInterval = time.Duration(secs) * time.Second
		} else {
			logger.Warn("invalid check-interval, using default", "value", v)
		}
	}

	if tempCfg := smartCfg.GetContainer("temperature"); tempCfg != nil {
		if v, ok := tempCfg.Get("difference"); ok {
			if n, valid := parsePositiveInt(v); valid {
				cfg.Temperature.Difference = n
			} else {
				logger.Warn("invalid temperature difference, using default", "value", v)
			}
		}
		if v, ok := tempCfg.Get("informational"); ok {
			if n, valid := parsePositiveInt(v); valid {
				cfg.Temperature.Informational = n
			} else {
				logger.Warn("invalid temperature informational, using default", "value", v)
			}
		}
		if v, ok := tempCfg.Get("critical"); ok {
			if n, valid := parsePositiveInt(v); valid {
				cfg.Temperature.Critical = n
			} else {
				logger.Warn("invalid temperature critical, using default", "value", v)
			}
		}
	}

	if stCfg := smartCfg.GetContainer("self-test"); stCfg != nil {
		if shortCfg := stCfg.GetContainer("short"); shortCfg != nil {
			if v, ok := shortCfg.Get("interval"); ok {
				if d, valid := parseDuration(v); valid {
					cfg.SelfTest.Short.Interval = d
				} else {
					logger.Warn("invalid short self-test interval, using default", "value", v)
				}
			}
			if v, ok := shortCfg.Get("time"); ok {
				cfg.SelfTest.Short.TimeOfDay = v
			}
		}
		if longCfg := stCfg.GetContainer("long"); longCfg != nil {
			if v, ok := longCfg.Get("interval"); ok {
				if d, valid := parseDuration(v); valid {
					cfg.SelfTest.Long.Interval = d
				} else {
					logger.Warn("invalid long self-test interval, using default", "value", v)
				}
			}
			if v, ok := longCfg.Get("time"); ok {
				cfg.SelfTest.Long.TimeOfDay = v
			}
			if v, ok := longCfg.Get("day"); ok {
				cfg.SelfTest.Long.Day = v
			}
		}
	}

	return &cfg
}

func parsePositiveInt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func parseDuration(s string) (time.Duration, bool) {
	if len(s) < 2 {
		return 0, false
	}
	unit := s[len(s)-1]
	switch unit {
	case 'h', 'd', 'm', 's':
		n, ok := parsePositiveInt(s[:len(s)-1])
		if !ok {
			return 0, false
		}
		switch unit {
		case 'h':
			return time.Duration(n) * time.Hour, true
		case 'd':
			return time.Duration(n) * 24 * time.Hour, true
		case 'm':
			return time.Duration(n) * time.Minute, true
		default:
			return time.Duration(n) * time.Second, true
		}
	default:
		n, ok := parsePositiveInt(s)
		if !ok {
			return 0, false
		}
		return time.Duration(n) * time.Second, true
	}
}
