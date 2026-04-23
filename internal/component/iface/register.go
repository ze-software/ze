// Design: docs/features/interfaces.md — Interface plugin registration
// Overview: iface.go — shared types and topic constants

package iface

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	ifaceevents "codeberg.org/thomas-mangin/ze/internal/component/iface/events"
	ifaceschema "codeberg.org/thomas-mangin/ze/internal/component/iface/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	vppevents "codeberg.org/thomas-mangin/ze/internal/component/vpp/events"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysctlevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// configRootInterface is the top-level YANG config root that the iface
// plugin owns. Used to select the right ConfigSection and to name the
// subtree walked by the backend feature gate.
const configRootInterface = "interface"

// backendLeafPath is the YANG path shown to the user in backend-gate
// error text so they know where to change the backend leaf.
const backendLeafPath = "/interface/backend"

// backendGateSchema caches the config schema used by validateBackendGate.
// Built lazily on first commit/verify to avoid paying YANG load cost at
// daemon startup. Schema is immutable after build -- safe for concurrent
// reads from any goroutine.
var (
	backendGateSchemaOnce sync.Once
	backendGateSchema     *config.Schema
	backendGateSchemaErr  error
)

// validateBackendGate runs the ze:backend commit-time feature check.
// sections holds the raw config sections delivered by the SDK; activeBackend
// is the already-parsed backend leaf value. On any mismatch or on schema
// load failure, it returns a single joined error suitable for propagation
// back to the SDK as the commit rejection.
//
// Runs cheaply on the happy path (no annotations trigger -> nil). The
// schema is built once per daemon lifetime.
func validateBackendGate(sections []sdk.ConfigSection, activeBackend string) error {
	backendGateSchemaOnce.Do(func() {
		backendGateSchema, backendGateSchemaErr = config.YANGSchema()
	})
	if backendGateSchemaErr != nil {
		return fmt.Errorf("interface backend gate: schema load: %w", backendGateSchemaErr)
	}
	for _, s := range sections {
		if s.Root != configRootInterface {
			continue
		}
		errs := config.ValidateBackendFeaturesJSON(
			s.Data, backendGateSchema,
			configRootInterface, activeBackend, backendLeafPath,
		)
		if len(errs) == 0 {
			return nil
		}
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return fmt.Errorf("interface commit rejected:\n  %s", strings.Join(msgs, "\n  "))
	}
	return nil
}

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
	_ = events.RegisterNamespace(ifaceevents.Namespace,
		ifaceevents.EventCreated, ifaceevents.EventUp, ifaceevents.EventDown,
		ifaceevents.EventAddrAdded, ifaceevents.EventAddrRemoved,
		ifaceevents.EventDHCPAcquired, ifaceevents.EventDHCPRenewed,
		ifaceevents.EventDHCPExpired, ifaceevents.EventRollback,
		ifaceevents.EventRouterDiscovered, ifaceevents.EventRouterLost,
	)

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

// routerKey identifies an IPv6 router discovered via NDP neighbor events.
type routerKey struct {
	ifaceName string
	routerIP  string // bare link-local address, no zone ID
}

// routerEntry tracks an installed IPv6 default route for a discovered router.
type routerEntry struct {
	metric int // route-priority at install time
}

// subscribeReconcileOnReady registers the vpp lifecycle handlers that trigger
// iface reconciliation once the vpp backend finishes its handshake. Subscribes
// to EventConnected (first handshake after daemon start) and EventReconnected
// (handshake after a vpp crash).
//
// trigger is invoked synchronously inside the EventBus Emit goroutine. Per
// pkg/ze/eventbus.go the handler MUST NOT block on I/O, so the production
// caller passes a non-blocking enqueue that hands the actual reconcile off
// to a worker goroutine. Tests may pass a synchronous reconcile for
// deterministic assertions.
//
// The returned unsubscribe funcs are appended by the caller to its shutdown
// cleanup list.
func subscribeReconcileOnReady(bus ze.EventBus, trigger func()) []func() {
	handler := events.AsString(func(_ string) {
		trigger()
	})
	return []func(){
		bus.Subscribe(vppevents.Namespace, vppevents.EventConnected, handler),
		bus.Subscribe(vppevents.Namespace, vppevents.EventReconnected, handler),
	}
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

	// activeCfg tracks the last successfully applied config for rollback
	// and for reconciliation triggered by vpp lifecycle events. Stored as
	// atomic.Pointer because the vppevents.EventConnected handler reads it
	// concurrently with the SDK's OnConfigApply writer.
	// Initialized from OnConfigure so the first reload rollback restores startup state.
	var activeCfg atomic.Pointer[ifaceConfig]
	var activeJournal *sdk.Journal

	// vppReadyOnce guards the one-shot subscription to vppevents so that a
	// config reload does not double-subscribe. The subscription lives inside
	// OnConfigure because that is where unsubscribers is populated and
	// where we know the EventBus is available.
	var vppReadyOnce sync.Once

	// activeDHCP tracks running DHCP clients keyed by interface+unit.
	// Protected by dhcpMu for concurrent access from event handlers.
	activeDHCP := make(map[dhcpUnitKey]dhcpEntry)
	var dhcpMu sync.Mutex

	// activeRouters tracks IPv6 routers discovered via NTF_ROUTER neighbor events.
	// Protected by dhcpMu (shared lock, short critical sections).
	activeRouters := make(map[routerKey]routerEntry)

	// suppressedRA tracks interfaces where ze set accept_ra_defrtr=0.
	// Used to restore the sysctl on config change or clean shutdown.
	// Protected by dhcpMu.
	suppressedRA := make(map[string]bool)

	// linkEventCh is a buffered channel for link failover work items.
	// Event bus handlers enqueue here (non-blocking, no I/O) and the
	// linkWorker goroutine processes them with netlink calls.
	type linkEvent struct {
		name string
		up   bool
	}
	linkEventCh := make(chan linkEvent, 16)
	linkWorkerDone := make(chan struct{})
	// vppReconcileCh coalesces vpp-lifecycle reconcile requests into at most
	// one pending work item. The vppReadyWorker goroutine drains it and
	// calls reconcileOnVPPReady so the actual GoVPP I/O runs outside the
	// EventBus Emit caller (the VPPManager goroutine). Honors the
	// pkg/ze/eventbus.go "handler MUST NOT block on I/O" contract.
	vppReconcileCh := make(chan struct{}, 1)
	vppReconcileDone := make(chan struct{})
	go func() {
		defer close(vppReconcileDone)
		for range vppReconcileCh {
			reconcileOnVPPReady(&activeCfg)
		}
	}()
	go func() {
		defer close(linkWorkerDone)
		for ev := range linkEventCh {
			dhcpMu.Lock()
			if ev.up {
				handleLinkUp(ev.name, activeDHCP, log)
				handleLinkUpIPv6(ev.name, activeRouters, log)
			} else {
				handleLinkDown(ev.name, activeDHCP, log)
				handleLinkDownIPv6(ev.name, activeRouters, log)
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

		if err := validateBackendGate(sections, cfg.Backend); err != nil {
			return err
		}

		if err := LoadBackend(cfg.Backend); err != nil {
			return fmt.Errorf("interface backend %q: %w", cfg.Backend, err)
		}
		log.Info("interface backend loaded", "backend", cfg.Backend)

		b := GetBackend()

		if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
			return joinApplyErrors("interface config", errs)
		}
		activeCfg.Store(cfg)
		log.Info("interface config applied")

		eb := GetEventBus()
		if eb == nil {
			log.Warn("interface plugin: no event bus configured, monitor will not start")
			return nil
		}

		if err := b.StartMonitor(eb); err != nil {
			if errors.Is(err, ErrBackendNotReady) {
				log.Debug("iface monitor deferred, backend not ready")
				// The vppevents.EventConnected handler retries StartMonitor.
			} else {
				return fmt.Errorf("interface monitor start: %w", err)
			}
		} else {
			log.Info("interface monitor started")
		}

		// Start DHCP clients for units that have DHCP enabled.
		dhcpMu.Lock()
		reconcileDHCP(cfg, eb, activeDHCP, log)
		dhcpMu.Unlock()

		// Subscribe to DHCP lease events to track gateways for link failover.
		// Handlers only update the map (no I/O), so mutex is sufficient.
		unsubscribers = append(unsubscribers,
			eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventDHCPAcquired, events.AsString(func(data string) {
				dhcpMu.Lock()
				handleDHCPLeaseEvent(data, activeDHCP, log)
				dhcpMu.Unlock()
			})),
			eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventDHCPRenewed, events.AsString(func(data string) {
				dhcpMu.Lock()
				handleDHCPLeaseEvent(data, activeDHCP, log)
				dhcpMu.Unlock()
			})),
			// Link events enqueue to worker channel (no I/O in handler).
			eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventDown, events.AsString(func(data string) {
				var ev struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.Name != "" {
					select {
					case linkEventCh <- linkEvent{name: ev.Name, up: false}:
					default: // non-blocking: drop if buffer full (transient overload)
					}
				}
			})),
			eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventUp, events.AsString(func(data string) {
				var ev struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.Name != "" {
					select {
					case linkEventCh <- linkEvent{name: ev.Name, up: true}:
					default: // non-blocking: drop if buffer full (transient overload)
					}
				}
			})),
			// IPv6 router discovery events from netlink neighbor monitor.
			eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventRouterDiscovered, events.AsString(func(data string) {
				dhcpMu.Lock()
				handleRouterDiscovered(data, activeRouters, activeDHCP, log)
				dhcpMu.Unlock()
			})),
			eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventRouterLost, events.AsString(func(data string) {
				dhcpMu.Lock()
				handleRouterLost(data, activeRouters, log)
				dhcpMu.Unlock()
			})),
		)

		// Subscribe once to vpp lifecycle events so reconciliation that was
		// deferred during initial apply (vpp handshake still in flight) runs
		// as soon as GoVPP is connected. The same handler fires on
		// EventReconnected so post-crash recovery also re-reconciles.
		// The handler itself does not touch the VPP backend -- it signals
		// vppReconcileCh and vppReadyWorker does the GoVPP RPCs outside the
		// Emit goroutine.
		vppReadyOnce.Do(func() {
			trigger := func() {
				select {
				case vppReconcileCh <- struct{}{}:
				default: // non-blocking: reconcile already pending, next worker iteration absorbs this event
				}
			}
			unsubscribers = append(unsubscribers, subscribeReconcileOnReady(eb, trigger)...)
		})

		// Suppress accept_ra_defrtr on interfaces with route-priority > 0,
		// so ze manages IPv6 default routes instead of the kernel.
		suppressRAForConfig(cfg, suppressedRA, activeRouters, eb, log)

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
		if err := validateBackendGate(sections, cfg.Backend); err != nil {
			return err
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

		previousCfg := activeCfg.Load()
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
					if _, emitErr := eb.Emit(ifaceevents.Namespace, ifaceevents.EventRollback, ""); emitErr != nil {
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

		activeCfg.Store(cfg)
		activeJournal = j
		log.Info("interface config reloaded via transaction")

		// Reconcile DHCP clients and IPv6 RA suppression after successful reload.
		eb := GetEventBus()
		if eb != nil {
			dhcpMu.Lock()
			reconcileDHCP(cfg, eb, activeDHCP, log)
			suppressRAForConfig(cfg, suppressedRA, activeRouters, eb, log)
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

	ctx, cancel := sdk.SignalContext()
	defer cancel()
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

	// Stop vpp-ready reconcile worker. Must happen after the vpp-events
	// unsubscribers above have run so no further sends race the close.
	close(vppReconcileCh)
	<-vppReconcileDone

	// Stop all DHCP clients on shutdown.
	dhcpMu.Lock()
	for key, entry := range activeDHCP {
		log.Debug("interface: stopping DHCP client on shutdown", "iface", key.ifaceName, "unit", key.unit)
		entry.client.Stop()
	}

	// Restore accept_ra_defrtr on all suppressed interfaces.
	// Collect keys first: restoreAcceptRaDefrtr deletes from suppressedRA.
	eb := GetEventBus()
	if eb != nil {
		suppNames := make([]string, 0, len(suppressedRA))
		for name := range suppressedRA {
			suppNames = append(suppNames, name)
		}
		for _, name := range suppNames {
			restoreAcceptRaDefrtr(name, suppressedRA, activeRouters, eb, log)
		}
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
var dhcpClientFactory func(ifaceName string, unit int, eb ze.EventBus, v4, v6 bool, hostname, clientID string, pdLength int, duid, resolvConfPath string, hasStaticNameServers bool, routeMetric int) (DHCPStopper, error)

// SetDHCPClientFactory registers the factory function used to create
// DHCP clients. Called from ifacedhcp's init().
func SetDHCPClientFactory(f func(string, int, ze.EventBus, bool, bool, string, string, int, string, string, bool, int) (DHCPStopper, error)) {
	dhcpClientFactory = f
}

// dhcpSystemConfig holds system-level DNS settings passed from the hub
// to the interface plugin for DHCP client creation. Atomic because the
// hub goroutine writes and the iface engine goroutine reads.
var dhcpSystemResolvConfPath atomic.Value // string
var dhcpSystemHasStaticNameServers atomic.Bool

// SetDHCPSystemConfig configures system-level DNS settings used by DHCP
// clients. Called from hub startup after extracting system config.
func SetDHCPSystemConfig(resolvConfPath string, hasStaticNameServers bool) {
	dhcpSystemResolvConfPath.Store(resolvConfPath)
	dhcpSystemHasStaticNameServers.Store(hasStaticNameServers)
}

// dhcpParams holds the config parameters for a DHCP client so reconcile
// can detect changes and restart clients when config changes.
type dhcpParams struct {
	v4, v6             bool
	hostname, clientID string
	pdLength           int
	duid               string
	routePriority      int
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
		for i := range units {
			u := &units[i]
			v4 := u.DHCP != nil && u.DHCP.Enabled
			v6 := u.DHCPv6 != nil && u.DHCPv6.Enabled
			if !v4 && !v6 {
				continue
			}
			key := dhcpUnitKey{ifaceName: name, unit: u.ID}
			p := dhcpParams{v4: v4, v6: v6, routePriority: u.RoutePriority}
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
		resolvPath, _ := dhcpSystemResolvConfPath.Load().(string)
		client, err := dhcpClientFactory(key.ifaceName, key.unit, eb, p.v4, p.v6, p.hostname, p.clientID, p.pdLength, p.duid, resolvPath, dhcpSystemHasStaticNameServers.Load(), p.routePriority)
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
		baseMetric := entry.params.routePriority
		newMetric := baseMetric + deprioritizedMetric
		log.Info("interface: link down, deprioritizing route", "iface", ifaceName, "gw", entry.gateway, "from", baseMetric, "to", newMetric)
		// Remove the base-metric route first, then add deprioritized.
		// Linux route identity is (dst, gw, link, metric) so RouteReplace
		// with a different metric creates a second entry, not a replacement.
		_ = RemoveRoute(ifaceName, "0.0.0.0/0", entry.gateway, baseMetric)
		if err := AddRoute(ifaceName, "0.0.0.0/0", entry.gateway, newMetric); err != nil {
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
		baseMetric := entry.params.routePriority
		oldMetric := baseMetric + deprioritizedMetric
		log.Info("interface: link up, restoring route priority", "iface", ifaceName, "gw", entry.gateway, "from", oldMetric, "to", baseMetric)
		// Remove the deprioritized route, restore base metric.
		_ = RemoveRoute(ifaceName, "0.0.0.0/0", entry.gateway, oldMetric)
		if err := AddRoute(ifaceName, "0.0.0.0/0", entry.gateway, baseMetric); err != nil {
			log.Debug("interface: restore route priority failed", "iface", ifaceName, "err", err)
		}
		return
	}
}

// handleRouterDiscovered processes a router-discovered event from the netlink
// monitor. It installs an IPv6 default route via the discovered router with
// the configured route-priority metric.
// Caller MUST hold dhcpMu.
func handleRouterDiscovered(data string, routers map[routerKey]routerEntry, active map[dhcpUnitKey]dhcpEntry, log *slog.Logger) {
	var payload RouterEventPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil || payload.RouterIP == "" || payload.Name == "" {
		return
	}
	key := routerKey{ifaceName: payload.Name, routerIP: payload.RouterIP}
	if _, exists := routers[key]; exists {
		return // already tracking this router
	}
	metric := routePriorityForInterface(payload.Name, active)
	if metric <= 0 {
		return // route-priority not configured, kernel handles RA routes
	}
	if err := AddRoute(payload.Name, "::/0", payload.RouterIP, metric); err != nil {
		log.Warn("interface: IPv6 default route install failed", "iface", payload.Name, "router", payload.RouterIP, "metric", metric, "err", err)
		return
	}
	routers[key] = routerEntry{metric: metric}
	log.Info("interface: IPv6 default route installed", "iface", payload.Name, "router", payload.RouterIP, "metric", metric)
}

// handleRouterLost processes a router-lost event. Removes the IPv6 default
// route that was installed for this router.
// Caller MUST hold dhcpMu.
func handleRouterLost(data string, routers map[routerKey]routerEntry, log *slog.Logger) {
	var payload RouterEventPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil || payload.RouterIP == "" || payload.Name == "" {
		return
	}
	key := routerKey{ifaceName: payload.Name, routerIP: payload.RouterIP}
	entry, exists := routers[key]
	if !exists {
		return // not tracking this router
	}
	_ = RemoveRoute(payload.Name, "::/0", payload.RouterIP, entry.metric)
	delete(routers, key)
	log.Info("interface: IPv6 default route removed (router lost)", "iface", payload.Name, "router", payload.RouterIP)
}

// handleLinkDownIPv6 deprioritizes all IPv6 default routes on an interface
// when its carrier drops. Same pattern as IPv4 handleLinkDown.
// Caller MUST hold dhcpMu.
func handleLinkDownIPv6(ifaceName string, routers map[routerKey]routerEntry, log *slog.Logger) {
	for key, entry := range routers {
		if key.ifaceName != ifaceName {
			continue
		}
		newMetric := entry.metric + deprioritizedMetric
		log.Info("interface: link down, deprioritizing IPv6 route", "iface", ifaceName, "router", key.routerIP, "from", entry.metric, "to", newMetric)
		_ = RemoveRoute(ifaceName, "::/0", key.routerIP, entry.metric)
		if err := AddRoute(ifaceName, "::/0", key.routerIP, newMetric); err != nil {
			log.Debug("interface: IPv6 deprioritize failed", "iface", ifaceName, "err", err)
		}
	}
}

// handleLinkUpIPv6 restores all IPv6 default routes on an interface
// when its carrier is restored.
// Caller MUST hold dhcpMu.
func handleLinkUpIPv6(ifaceName string, routers map[routerKey]routerEntry, log *slog.Logger) {
	for key, entry := range routers {
		if key.ifaceName != ifaceName {
			continue
		}
		oldMetric := entry.metric + deprioritizedMetric
		log.Info("interface: link up, restoring IPv6 route priority", "iface", ifaceName, "router", key.routerIP, "from", oldMetric, "to", entry.metric)
		_ = RemoveRoute(ifaceName, "::/0", key.routerIP, oldMetric)
		if err := AddRoute(ifaceName, "::/0", key.routerIP, entry.metric); err != nil {
			log.Debug("interface: IPv6 restore priority failed", "iface", ifaceName, "err", err)
		}
	}
}

// routePriorityForInterface returns the route-priority configured for the
// given interface from the active DHCP entries. Returns 0 if not configured.
// IPv6 RA routes are per-interface (not per-unit), so if multiple units have
// different route-priority values, the first non-zero value is used.
func routePriorityForInterface(ifaceName string, active map[dhcpUnitKey]dhcpEntry) int {
	result := 0
	for key, entry := range active {
		if key.ifaceName != ifaceName {
			continue
		}
		rp := entry.params.routePriority
		if rp > 0 && result == 0 {
			result = rp
		}
	}
	return result
}

// suppressAcceptRaDefrtr sets accept_ra_defrtr=0 on the given interface via
// the sysctl event bus, and cleans up any stale kernel-installed ::/0 routes.
// Records the interface in suppressedRA for restore on shutdown/config change.
func suppressAcceptRaDefrtr(ifaceName string, suppressed map[string]bool, eb ze.EventBus, log *slog.Logger) {
	if suppressed[ifaceName] {
		return // already suppressed
	}
	sysctlKey := "net.ipv6.conf." + ifaceName + ".accept_ra_defrtr"
	payload := fmt.Sprintf(`{"key":%q,"value":"0","source":"interface"}`, sysctlKey)
	if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventSet, payload); err != nil {
		log.Warn("interface: failed to suppress accept_ra_defrtr", "iface", ifaceName, "err", err)
		return
	}
	suppressed[ifaceName] = true
	log.Info("interface: suppressed accept_ra_defrtr", "iface", ifaceName)
	cleanupStaleIPv6DefaultRoutes(ifaceName, log)
}

// restoreAcceptRaDefrtr restores accept_ra_defrtr=1 on the given interface
// and removes all ze-managed ::/0 routes for it.
func restoreAcceptRaDefrtr(ifaceName string, suppressed map[string]bool, routers map[routerKey]routerEntry, eb ze.EventBus, log *slog.Logger) {
	if !suppressed[ifaceName] {
		return
	}
	// Remove all ze-managed IPv6 default routes on this interface.
	// Collect keys first: we delete from routers during iteration.
	var removeKeys []routerKey
	for key := range routers {
		if key.ifaceName == ifaceName {
			removeKeys = append(removeKeys, key)
		}
	}
	for _, key := range removeKeys {
		_ = RemoveRoute(ifaceName, "::/0", key.routerIP, routers[key].metric)
		delete(routers, key)
	}
	sysctlKey := "net.ipv6.conf." + ifaceName + ".accept_ra_defrtr"
	payload := fmt.Sprintf(`{"key":%q,"value":"1","source":"interface"}`, sysctlKey)
	if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventSet, payload); err != nil {
		log.Warn("interface: failed to restore accept_ra_defrtr", "iface", ifaceName, "err", err)
	}
	delete(suppressed, ifaceName)
	log.Info("interface: restored accept_ra_defrtr", "iface", ifaceName)
}

// suppressRAForConfig iterates the config and suppresses accept_ra_defrtr on
// interfaces that have route-priority > 0. Also restores accept_ra_defrtr on
// interfaces that no longer qualify (config removal).
func suppressRAForConfig(cfg *ifaceConfig, suppressed map[string]bool, routers map[routerKey]routerEntry, eb ze.EventBus, log *slog.Logger) {
	if eb == nil {
		return
	}
	// Build the set of interfaces that need suppression.
	desired := make(map[string]bool)
	// Suppress accept_ra_defrtr whenever route-priority > 0, regardless of
	// whether DHCPv6 is enabled. SLAAC and static IPv6 also receive RAs that
	// install kernel default routes. Suppressing on IPv4-only interfaces is
	// harmless (no RAs to process).
	collectSuppression := func(name string, units []unitEntry) {
		for i := range units {
			if units[i].RoutePriority > 0 {
				desired[name] = true
			}
		}
	}
	for _, e := range cfg.Ethernet {
		collectSuppression(e.Name, e.Units)
	}
	for _, e := range cfg.Dummy {
		collectSuppression(e.Name, e.Units)
	}
	for _, e := range cfg.Veth {
		collectSuppression(e.Name, e.Units)
	}
	for _, e := range cfg.Bridge {
		collectSuppression(e.Name, e.Units)
	}
	for i := range cfg.Tunnel {
		collectSuppression(cfg.Tunnel[i].Name, cfg.Tunnel[i].Units)
	}
	for i := range cfg.Wireguard {
		collectSuppression(cfg.Wireguard[i].Name, cfg.Wireguard[i].Units)
	}

	// Suppress on newly qualifying interfaces.
	for name := range desired {
		suppressAcceptRaDefrtr(name, suppressed, eb, log)
	}
	// Restore on interfaces that no longer qualify.
	// Collect keys first: restoreAcceptRaDefrtr deletes from suppressed.
	restoreList := make([]string, 0)
	for name := range suppressed {
		if !desired[name] {
			restoreList = append(restoreList, name)
		}
	}
	for _, name := range restoreList {
		restoreAcceptRaDefrtr(name, suppressed, routers, eb, log)
	}
}

// cleanupStaleIPv6DefaultRoutes removes any pre-existing ::/0 routes on the
// interface that were installed by the kernel before ze suppressed
// accept_ra_defrtr. Prevents duplicate default routes with different metrics.
//
// Safe to remove all ::/0 routes because this only runs on first suppression
// (suppressAcceptRaDefrtr returns early if already suppressed), before ze has
// installed any routes via handleRouterDiscovered.
func cleanupStaleIPv6DefaultRoutes(ifaceName string, log *slog.Logger) {
	routes, err := ListRoutes(ifaceName, "::/0")
	if err != nil {
		log.Debug("interface: failed to list routes for stale cleanup", "iface", ifaceName, "err", err)
		return
	}
	for _, r := range routes {
		_ = RemoveRoute(ifaceName, "::/0", r.Gateway, r.Metric)
		log.Info("interface: removed stale kernel IPv6 default route", "iface", ifaceName, "gw", r.Gateway, "metric", r.Metric)
	}
}
