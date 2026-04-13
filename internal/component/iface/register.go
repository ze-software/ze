// Design: docs/features/interfaces.md — Interface plugin registration
// Overview: iface.go — shared types and topic constants

package iface

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"

	ifaceschema "codeberg.org/thomas-mangin/ze/internal/component/iface/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// loggerPtr is the package-level logger, disabled by default.
// Stored as atomic.Pointer to avoid data races when tests start
// multiple in-process plugin instances concurrently.
var loggerPtr atomic.Pointer[slog.Logger]

// eventBusMu guards eventBusRef. An interface cannot be stored in
// atomic.Pointer directly, so a mutex is used instead.
var (
	eventBusMu  sync.Mutex
	eventBusRef ze.EventBus
)

// SetEventBus sets the package-level EventBus reference used by the monitor.
// MUST be called before RunEngine starts the monitor. The engine calls this
// during plugin startup to inject the EventBus dependency.
func SetEventBus(eb ze.EventBus) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	eventBusRef = eb
}

// GetEventBus returns the package-level EventBus reference, or nil if not set.
func GetEventBus() ze.EventBus {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	return eventBusRef
}

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)

	reg := registry.Registration{
		Name:         "interface",
		Description:  "OS network interface monitoring and management",
		Features:     "yang",
		YANG:         ifaceschema.ZeIfaceConfYANG,
		ConfigRoots:  []string{"interface"},
		Dependencies: []string{"sysctl"},
		RunEngine:    runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				SetEventBus(e)
			}
		},
	}
	reg.CLIHandler = func(_ []string) int {
		return 1
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "interface: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// setLogger sets the package-level logger.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// dhcpUnitKey uniquely identifies a DHCP client by interface + unit.
type dhcpUnitKey struct {
	ifaceName string
	unit      int
}

// runEngine is the engine-mode entry point for the interface plugin.
// It uses the SDK 5-stage protocol to receive configuration, starts
// the netlink interface monitor, and blocks until shutdown.
func runEngine(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("interface plugin starting")

	p := sdk.NewWithConn("interface", conn)
	defer func() { _ = p.Close() }()

	// pendingCfg holds the validated config between verify and apply phases.
	var pendingCfg *ifaceConfig

	// activeCfg tracks the last successfully applied config for rollback.
	// Initialized from OnConfigure so the first reload rollback restores startup state.
	var activeCfg *ifaceConfig
	var activeJournal *sdk.Journal

	// activeDHCP tracks running DHCP clients keyed by interface+unit.
	// Protected by dhcpMu for concurrent access from event handlers.
	activeDHCP := make(map[dhcpUnitKey]dhcpEntry)
	var dhcpMu sync.Mutex

	// linkEventCh is a buffered channel for link failover work items.
	// Event bus handlers enqueue here (non-blocking, no I/O) and the
	// linkWorker goroutine processes them with netlink calls.
	type linkEvent struct {
		name string
		up   bool
	}
	linkEventCh := make(chan linkEvent, 16)
	linkWorkerDone := make(chan struct{})
	go func() {
		defer close(linkWorkerDone)
		for ev := range linkEventCh {
			dhcpMu.Lock()
			if ev.up {
				handleLinkUp(ev.name, activeDHCP, log)
			} else {
				handleLinkDown(ev.name, activeDHCP, log)
			}
			dhcpMu.Unlock()
		}
	}()

	// unsubscribers tracks event bus subscriptions for cleanup.
	var unsubscribers []func()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseIfaceSections(sections)
		if err != nil {
			return fmt.Errorf("interface config: %w", err)
		}

		if cfg.Backend == "" {
			return fmt.Errorf("interface: no backend configured and no OS default available")
		}

		if err := LoadBackend(cfg.Backend); err != nil {
			return fmt.Errorf("interface backend %q: %w", cfg.Backend, err)
		}
		log.Info("interface backend loaded", "backend", cfg.Backend)

		b := GetBackend()

		if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
			return joinApplyErrors("interface config", errs)
		}
		activeCfg = cfg
		log.Info("interface config applied")

		eb := GetEventBus()
		if eb == nil {
			log.Warn("interface plugin: no event bus configured, monitor will not start")
			return nil
		}

		if err := b.StartMonitor(eb); err != nil {
			return fmt.Errorf("interface monitor start: %w", err)
		}
		log.Info("interface monitor started")

		// Start DHCP clients for units that have DHCP enabled.
		dhcpMu.Lock()
		reconcileDHCP(cfg, eb, activeDHCP, log)
		dhcpMu.Unlock()

		// Subscribe to DHCP lease events to track gateways for link failover.
		// Handlers only update the map (no I/O), so mutex is sufficient.
		unsubscribers = append(unsubscribers,
			eb.Subscribe(plugin.NamespaceInterface, plugin.EventInterfaceDHCPAcquired, func(data string) {
				dhcpMu.Lock()
				handleDHCPLeaseEvent(data, activeDHCP, log)
				dhcpMu.Unlock()
			}),
			eb.Subscribe(plugin.NamespaceInterface, plugin.EventInterfaceDHCPRenewed, func(data string) {
				dhcpMu.Lock()
				handleDHCPLeaseEvent(data, activeDHCP, log)
				dhcpMu.Unlock()
			}),
			// Link events enqueue to worker channel (no I/O in handler).
			eb.Subscribe(plugin.NamespaceInterface, plugin.EventInterfaceDown, func(data string) {
				var ev struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.Name != "" {
					select {
					case linkEventCh <- linkEvent{name: ev.Name, up: false}:
					default: // non-blocking: drop if buffer full (transient overload)
					}
				}
			}),
			eb.Subscribe(plugin.NamespaceInterface, plugin.EventInterfaceUp, func(data string) {
				var ev struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.Name != "" {
					select {
					case linkEventCh <- linkEvent{name: ev.Name, up: true}:
					default: // non-blocking: drop if buffer full (transient overload)
					}
				}
			}),
		)

		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseIfaceSections(sections)
		if err != nil {
			return fmt.Errorf("interface config: %w", err)
		}
		if cfg.Backend == "" {
			return fmt.Errorf("interface: no backend configured and no OS default available")
		}
		pendingCfg = cfg
		log.Debug("interface config verified", "backend", cfg.Backend)
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			log.Warn("interface config apply: no pending config (verify not called?)")
			return nil
		}

		b := GetBackend()
		if b == nil {
			return fmt.Errorf("interface config apply: no backend loaded")
		}

		previousCfg := activeCfg
		j := sdk.NewJournal()
		err := j.Record(
			func() error {
				if errs := applyConfig(cfg, previousCfg, b); len(errs) > 0 {
					return joinApplyErrors("interface reload", errs)
				}
				return nil
			},
			func() error {
				// Rollback: re-apply previous config. If no previous config,
				// apply an empty config to undo all interface changes. The
				// "previous" passed to applyConfig here is cfg (the failed
				// reload's state) so any tunnels we created get rebuilt
				// with the previous spec, not skipped as unchanged.
				rollbackCfg := previousCfg
				if rollbackCfg == nil {
					rollbackCfg = &ifaceConfig{Backend: defaultBackendName}
				}
				if errs := applyConfig(rollbackCfg, cfg, b); len(errs) > 0 {
					return joinApplyErrors("interface rollback", errs)
				}
				// Emit rollback event so downstream plugins react.
				eb := GetEventBus()
				if eb != nil {
					if _, emitErr := eb.Emit(plugin.NamespaceInterface, plugin.EventInterfaceRollback, ""); emitErr != nil {
						log.Debug("interface rollback emit failed", "error", emitErr)
					}
				}
				return nil
			},
		)
		if err != nil {
			j.Rollback()
			return err
		}

		activeCfg = cfg
		activeJournal = j
		log.Info("interface config reloaded via transaction")

		// Reconcile DHCP clients after successful reload.
		eb := GetEventBus()
		if eb != nil {
			dhcpMu.Lock()
			reconcileDHCP(cfg, eb, activeDHCP, log)
			dhcpMu.Unlock()
		}

		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("interface rollback: %d errors", len(errs))
		}
		log.Info("interface config rolled back")
		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"interface"},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("interface plugin failed", "error", err)
		return 1
	}

	// Unsubscribe event handlers.
	for _, unsub := range unsubscribers {
		unsub()
	}

	// Stop link event worker.
	close(linkEventCh)
	<-linkWorkerDone

	// Stop all DHCP clients on shutdown.
	dhcpMu.Lock()
	for key, entry := range activeDHCP {
		log.Debug("interface: stopping DHCP client on shutdown", "iface", key.ifaceName, "unit", key.unit)
		entry.client.Stop()
	}
	dhcpMu.Unlock()

	if err := CloseBackend(); err != nil {
		log.Warn("interface backend close failed", "error", err)
	}
	log.Info("interface backend closed")

	return 0
}

// joinApplyErrors logs each error at Warn level and returns a short summary
// for the status line. Detailed errors are visible via log output.
func joinApplyErrors(prefix string, errs []error) error {
	log := loggerPtr.Load()
	for _, e := range errs {
		log.Warn(prefix, "err", e)
	}
	if len(errs) == 1 {
		return fmt.Errorf("%s: %w", prefix, errs[0])
	}
	return fmt.Errorf("%s: %d errors (see log for details)", prefix, len(errs))
}

// DHCPStopper is the subset of ifacedhcp.DHCPClient needed by the
// interface plugin to stop running clients. Defined as an interface
// so the iface package does not import ifacedhcp directly.
type DHCPStopper interface {
	Stop()
}

// dhcpClientFactory is set by the ifacedhcp package at init time via
// SetDHCPClientFactory. It returns a started DHCP client or an error.
// The interface plugin calls this to create clients without importing
// the ifacedhcp package.
var dhcpClientFactory func(ifaceName string, unit int, eb ze.EventBus, v4, v6 bool, hostname, clientID string, pdLength int, duid, resolvConfPath string) (DHCPStopper, error)

// SetDHCPClientFactory registers the factory function used to create
// DHCP clients. Called from ifacedhcp's init().
func SetDHCPClientFactory(f func(string, int, ze.EventBus, bool, bool, string, string, int, string, string) (DHCPStopper, error)) {
	dhcpClientFactory = f
}

// dhcpParams holds the config parameters for a DHCP client so reconcile
// can detect changes and restart clients when config changes.
type dhcpParams struct {
	v4, v6             bool
	hostname, clientID string
	pdLength           int
	duid               string
}

// dhcpEntry tracks a running DHCP client and the params it was created with.
type dhcpEntry struct {
	client  DHCPStopper
	params  dhcpParams
	gateway string // last known gateway from DHCP lease (for link failover)
}

// reconcileDHCP starts DHCP clients for newly enabled units, stops clients
// for units that are no longer DHCP-enabled, and restarts clients whose
// config parameters changed. Called from OnConfigure and OnConfigApply.
func reconcileDHCP(cfg *ifaceConfig, eb ze.EventBus, active map[dhcpUnitKey]dhcpEntry, log *slog.Logger) {
	if dhcpClientFactory == nil {
		return
	}

	// Build the desired set from all interface types that have units.
	desired := make(map[dhcpUnitKey]dhcpParams)

	// Collect from all interface types. Veth and bridge embed ifaceEntry;
	// tunnel and wireguard embed ifaceEntry; loopback has only units.
	collectDHCPUnits := func(name string, units []unitEntry) {
		for _, u := range units {
			v4 := u.DHCP != nil && u.DHCP.Enabled
			v6 := u.DHCPv6 != nil && u.DHCPv6.Enabled
			if !v4 && !v6 {
				continue
			}
			key := dhcpUnitKey{ifaceName: name, unit: u.ID}
			p := dhcpParams{v4: v4, v6: v6}
			if u.DHCP != nil {
				p.hostname = u.DHCP.Hostname
				p.clientID = u.DHCP.ClientID
			}
			if u.DHCPv6 != nil {
				p.pdLength = u.DHCPv6.PDLength
				p.duid = u.DHCPv6.DUID
			}
			desired[key] = p
		}
	}

	for _, e := range cfg.Ethernet {
		collectDHCPUnits(e.Name, e.Units)
	}
	for _, e := range cfg.Dummy {
		collectDHCPUnits(e.Name, e.Units)
	}
	for _, e := range cfg.Veth {
		collectDHCPUnits(e.Name, e.Units)
	}
	for _, e := range cfg.Bridge {
		collectDHCPUnits(e.Name, e.Units)
	}
	for i := range cfg.Tunnel {
		collectDHCPUnits(cfg.Tunnel[i].Name, cfg.Tunnel[i].Units)
	}
	for i := range cfg.Wireguard {
		collectDHCPUnits(cfg.Wireguard[i].Name, cfg.Wireguard[i].Units)
	}
	if cfg.Loopback != nil {
		collectDHCPUnits("lo", cfg.Loopback.Units)
	}

	// Auto-discovery: if dhcp-auto is true and no explicit DHCP is configured,
	// find the first ethernet interface and run DHCPv4 on it.
	if cfg.DHCPAuto && len(desired) == 0 {
		if name := discoverPrimaryEthernet(log); name != "" {
			// Bring the interface administratively UP before DHCP.
			// Without this, the kernel cannot send DHCP packets.
			if b := GetBackend(); b != nil {
				if err := b.SetAdminUp(name); err != nil {
					log.Warn("interface: dhcp-auto: failed to bring up", "iface", name, "err", err)
				}
			}
			key := dhcpUnitKey{ifaceName: name, unit: 0}
			desired[key] = dhcpParams{v4: true}
			log.Info("interface: dhcp-auto discovered primary ethernet", "iface", name)
		}
	}

	// Stop clients that are no longer desired or whose params changed.
	for key, entry := range active {
		newParams, stillDesired := desired[key]
		if !stillDesired || newParams != entry.params {
			if !stillDesired {
				log.Info("interface: stopping DHCP client", "iface", key.ifaceName, "unit", key.unit)
			} else {
				log.Info("interface: restarting DHCP client (config changed)", "iface", key.ifaceName, "unit", key.unit)
			}
			entry.client.Stop()
			delete(active, key)
		}
	}

	// Start clients that are newly desired (or restarted after param change).
	for key, p := range desired {
		if _, running := active[key]; running {
			continue
		}
		client, err := dhcpClientFactory(key.ifaceName, key.unit, eb, p.v4, p.v6, p.hostname, p.clientID, p.pdLength, p.duid, cfg.ResolvConfPath)
		if err != nil {
			log.Warn("interface: DHCP client creation failed",
				"iface", key.ifaceName, "unit", key.unit, "err", err)
			continue
		}
		active[key] = dhcpEntry{client: client, params: p}
		log.Info("interface: DHCP client started", "iface", key.ifaceName, "unit", key.unit, "v4", p.v4, "v6", p.v6)
	}
}

// discoverPrimaryEthernet finds the first ethernet interface on the system.
// Used by dhcp-auto mode to avoid hardcoding interface names. Returns ""
// if no suitable interface is found (e.g., backend not loaded, no ethernet).
func discoverPrimaryEthernet(log *slog.Logger) string {
	ifaces, err := DiscoverInterfaces()
	if err != nil {
		log.Debug("interface: dhcp-auto discovery failed", "err", err)
		return ""
	}
	for _, iface := range ifaces {
		if iface.Type == zeTypeEthernet {
			return iface.Name
		}
	}
	log.Debug("interface: dhcp-auto found no ethernet interface")
	return ""
}

// deprioritizedMetric is the route metric applied when a link goes down.
// Matches gokrazy's behavior (priority 1024 for downed links).
const deprioritizedMetric = 1024

// handleDHCPLeaseEvent updates the stored gateway for link-state failover.
func handleDHCPLeaseEvent(data string, active map[dhcpUnitKey]dhcpEntry, log *slog.Logger) {
	var payload struct {
		Name   string `json:"name"`
		Unit   int    `json:"unit"`
		Router string `json:"router"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil || payload.Router == "" {
		return
	}
	key := dhcpUnitKey{ifaceName: payload.Name, unit: payload.Unit}
	if entry, ok := active[key]; ok {
		entry.gateway = payload.Router
		active[key] = entry
		log.Debug("interface: stored DHCP gateway for failover", "iface", payload.Name, "gw", payload.Router)
	}
}

// handleLinkDown is called by the link worker when an interface carrier drops.
// If there's a DHCP client on that interface with a known gateway, remove the
// normal-metric route and add a deprioritized one.
// Caller MUST hold dhcpMu.
func handleLinkDown(ifaceName string, active map[dhcpUnitKey]dhcpEntry, log *slog.Logger) {
	for key, entry := range active {
		if key.ifaceName != ifaceName || entry.gateway == "" {
			continue
		}
		log.Info("interface: link down, deprioritizing route", "iface", ifaceName, "gw", entry.gateway, "metric", deprioritizedMetric)
		// Remove the metric-0 route first, then add metric-1024.
		// Linux route identity is (dst, gw, link, metric) so RouteReplace
		// with a different metric creates a second entry, not a replacement.
		_ = RemoveRoute(ifaceName, "0.0.0.0/0", entry.gateway, 0)
		if err := AddRoute(ifaceName, "0.0.0.0/0", entry.gateway, deprioritizedMetric); err != nil {
			log.Debug("interface: deprioritize route failed", "iface", ifaceName, "err", err)
		}
		return
	}
}

// handleLinkUp is called by the link worker when an interface carrier is
// restored. Removes the deprioritized route and installs normal metric.
// Caller MUST hold dhcpMu.
func handleLinkUp(ifaceName string, active map[dhcpUnitKey]dhcpEntry, log *slog.Logger) {
	for key, entry := range active {
		if key.ifaceName != ifaceName || entry.gateway == "" {
			continue
		}
		log.Info("interface: link up, restoring route priority", "iface", ifaceName, "gw", entry.gateway)
		// Remove the deprioritized route, add normal metric.
		_ = RemoveRoute(ifaceName, "0.0.0.0/0", entry.gateway, deprioritizedMetric)
		if err := AddRoute(ifaceName, "0.0.0.0/0", entry.gateway, 0); err != nil {
			log.Debug("interface: restore route priority failed", "iface", ifaceName, "err", err)
		}
		return
	}
}
