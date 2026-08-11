// Design: docs/features/interfaces.md — Interface plugin registration
// Overview: iface.go — shared types and topic constants

package iface

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/config"
	ifaceyang "github.com/ze-software/ze/internal/component/iface/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/health"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/observation"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errInterfaceNoBackendConfiguredAndNo   = errors.New("interface: no backend configured and no OS default available")
	errInterfaceConfigApplyNoBackendLoaded = errors.New("interface config apply: no backend loaded")
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
		return fmt.Errorf("interface commit rejected:\n  %s", textbuf.Join(msgs, "\n  "))
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
	health.Register("iface", checkHealth)

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
		Name:                    "interface",
		Description:             "OS network interface monitoring and management",
		Features:                "yang",
		YANG:                    ifaceyang.ZeIfaceConfYANG,
		ConfigRoots:             []string{"interface"},
		Dependencies:            []string{"sysctl"},
		InProcessConfigVerifier: verifyIfaceConfig,
		RunEngine:               runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			bindMetricsRegistry(reg)
			observation.BindMetrics(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			SetEventBus(eb)
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

func verifyIfaceConfig(sections []sdk.ConfigSection) error {
	_, err := parseAndVerifyIfaceSections(sections)
	return err
}

func parseAndVerifyIfaceSections(sections []sdk.ConfigSection) (*ifaceConfig, error) {
	activeBackend, err := parseIfaceBackend(sections)
	if err != nil {
		return nil, fmt.Errorf("interface config: %w", err)
	}
	if err := validateBackendGate(sections, activeBackend); err != nil {
		return nil, err
	}
	cfg, err := parseIfaceSections(sections)
	if err != nil {
		return nil, fmt.Errorf("interface config: %w", err)
	}
	if cfg.Backend == "" {
		return nil, errInterfaceNoBackendConfiguredAndNo
	}
	if err := validateUniqueMatchMAC(cfg); err != nil {
		return nil, fmt.Errorf("interface config: %w", err)
	}
	if cfg.Backend == vppBackendName {
		if err := validateVPPQoSMaps(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func parseIfaceBackend(sections []sdk.ConfigSection) (string, error) {
	backend := defaultBackendName
	for _, s := range sections {
		if s.Root != configRootInterface {
			continue
		}
		var root map[string]any
		if err := json.Unmarshal([]byte(s.Data), &root); err != nil {
			return "", fmt.Errorf("backend: unmarshal: %w", err)
		}
		ifaceMap, ok := root[configRootInterface].(map[string]any)
		if !ok {
			return backend, nil
		}
		if b, ok := ifaceMap["backend"].(string); ok && b != "" {
			backend = b
		}
		return backend, nil
	}
	return backend, nil
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
	unit      string
}

// routerKey identifies an IPv6 router discovered via NDP neighbor events.
type routerKey struct {
	ifaceName string
	routerIP  string // bare link-local address, no zone ID
}

// routerEntry tracks an installed IPv6 default route for a discovered router.
type routerEntry struct {
	metric      int // route-priority at install time
	metricState routeMetricState
}

// nonBlockingNotify sends a coalescing signal on ch without blocking: if a
// signal is already pending (buffer full, no receiver ready), the send is
// dropped since the next worker iteration absorbs it. Shared by the vpp-ready
// and registry-change reconcile triggers below.
func nonBlockingNotify(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default: // reconcile already pending, next worker iteration absorbs this event
	}
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
	operationJournals := make(map[string]*sdk.Journal)
	var operationMu sync.Mutex

	// vppReadyOnce guards the one-shot subscription to vppevents so that a
	// config reload does not double-subscribe. The subscription lives inside
	// OnConfigure because that is where unsubscribers is populated and
	// where we know the EventBus is available.
	var vppReadyOnce sync.Once

	// activeDHCP tracks running DHCP clients keyed by interface+unit.
	// Protected by dhcpMu for concurrent access from event handlers.
	activeDHCP := make(map[dhcpUnitKey]dhcpEntry)
	var dhcpMu sync.Mutex

	// activeRA tracks running Router Advertisement senders keyed by
	// interface+unit. Protected by raMu, which config apply and shutdown both
	// take. See reconcile_ra.go.
	activeRA := make(map[raUnitKey]raEntry)
	var raMu sync.Mutex

	// activePPPoE tracks running PPPoE client sessions keyed by config name.
	activePPPoE := make(map[string]*PPPoEClient)
	var pppoeMu sync.Mutex

	// activeRouters tracks IPv6 routers discovered via NTF_ROUTER neighbor events.
	// Protected by dhcpMu (shared lock, short critical sections).
	activeRouters := make(map[routerKey]routerEntry)

	// suppressedRA tracks interfaces where ze set accept_ra_defrtr=0.
	// Used to restore the sysctl on config change or clean shutdown.
	// Protected by dhcpMu.
	suppressedRA := make(map[string]bool)

	// raRoutePriority holds the route-priority the operator wrote for each
	// interface ze took the IPv6 default routes of. suppressRAForConfig writes
	// it from the config and handleRouterDiscovered reads it, so suppression
	// and installation cannot disagree about who owns those routes.
	// Protected by dhcpMu.
	raRoutePriority := make(map[string]int)

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
	// registryReconcileCh coalesces address_owner.go registry-change
	// notifications into at most one pending work item, the same
	// coalescing shape as vppReconcileCh above. Unlike the vpp trigger
	// (gated to the vpp backend only), this fires for any backend: a
	// plugin's RegisterOwnedAddresses/UnregisterOwnedAddresses call must
	// reach the kernel within the same enable/disable operation regardless
	// of which backend is active (design finding B1). The channel is
	// never closed -- only registryReconcileStop is -- so a registry
	// mutation racing shutdown can still send without panicking on a
	// closed channel; the worker simply may not drain it.
	registryReconcileCh := make(chan struct{}, 1)
	registryReconcileStop := make(chan struct{})
	registryReconcileDone := make(chan struct{})
	go func() {
		defer close(registryReconcileDone)
		for {
			select {
			case <-registryReconcileCh:
				reconcileOnRegistryChange(&activeCfg)
			case <-registryReconcileStop:
				// select does not prefer registryReconcileCh over
				// registryReconcileStop when both are ready, so a signal
				// that raced the stop could otherwise be silently
				// dropped. Drain it once, non-blocking, before exiting.
				select {
				case <-registryReconcileCh:
					reconcileOnRegistryChange(&activeCfg)
				default: // nothing pending to drain
				}
				return
			}
		}
	}()
	setAddressOwnerReconcileTrigger(func() { nonBlockingNotify(registryReconcileCh) })
	// The owned-device registry (device_owner.go) shares the SAME reconcile
	// channel and worker: reconcileOnReadyWithJournal reconciles BOTH
	// registries from snapshots on every pass, so one worker serves both and a
	// second channel would add interleaving without adding freshness.
	setDeviceOwnerReconcileTrigger(func() { nonBlockingNotify(registryReconcileCh) })
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
			return errInterfaceNoBackendConfiguredAndNo
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
		// Publish the logical<->os-name mapping to the shared resolver so
		// consumers (IS-IS, ...) that call iface.Resolve translate a logical
		// name to its kernel device. Done on every apply so a config change
		// re-points the binding.
		setResolverConfig(cfg)
		log.Info("interface config applied")

		eb := GetEventBus()
		if eb == nil {
			log.Warn("interface plugin: no event bus configured, monitor will not start")
			return nil
		}

		// Bind the shared resolver to the event bus so monitor link events
		// invalidate its cache and drive iface.Subscribe consumers. Idempotent.
		bindResolverEvents(eb)

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

		// Reconcile PPPoE clients.
		pppoeMu.Lock()
		reconcilePPPoEClients(cfg, activePPPoE, b, log)
		pppoeMu.Unlock()

		// Start DHCP clients for units that have DHCP enabled, and suppress
		// accept_ra_defrtr on interfaces whose config writes a route-priority,
		// so ze manages their IPv6 default routes instead of the kernel. The
		// suppression runs BEFORE the router-discovered subscription below: it
		// publishes raRoutePriority, which is what tells that handler ze owns
		// those routes.
		dhcpMu.Lock()
		reconcileDHCP(cfg, eb, activeDHCP, log)
		suppressRAForConfig(cfg, suppressedRA, activeRouters, raRoutePriority, eb, log)
		dhcpMu.Unlock()

		// Start Router Advertisement senders for units that advertise.
		raMu.Lock()
		reconcileRA(cfg, activeRA, log)
		raMu.Unlock()

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
					// A registered owned-macvlan parent coming up re-triggers a
					// reconcile pass so a device whose earlier create failed
					// (parent was absent) is retried with no plugin calls.
					if isRegisteredMacvlanParent(ev.Name) {
						nonBlockingNotify(registryReconcileCh)
					}
				}
			})),
			// A registered owned-macvlan parent APPEARING (first RTM_NEWLINK)
			// re-triggers a reconcile pass so the device is created once its
			// parent exists (holo bug 12: fire-and-forget create replaced by an
			// event-driven retry). No I/O in the handler -- it only signals.
			eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventCreated, events.AsString(func(data string) {
				var ev struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.Name != "" && isRegisteredMacvlanParent(ev.Name) {
					nonBlockingNotify(registryReconcileCh)
				}
			})),
			// IPv6 router discovery events from netlink neighbor monitor.
			eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventRouterDiscovered, events.AsString(func(data string) {
				dhcpMu.Lock()
				handleRouterDiscovered(data, activeRouters, raRoutePriority, log)
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
			trigger := func() { nonBlockingNotify(vppReconcileCh) }
			unsubscribers = append(unsubscribers, subscribeReconcileOnReady(eb, trigger)...)
		})

		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseAndVerifyIfaceSections(sections)
		if err != nil {
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

		previousCfg := activeCfg.Load()

		// Detect backend change and switch transactionally.
		previousBackend := ""
		if previousCfg != nil {
			previousBackend = previousCfg.Backend
		}
		if cfg.Backend != previousBackend && cfg.Backend != "" {
			if err := LoadBackend(cfg.Backend); err != nil {
				return fmt.Errorf("interface backend switch to %q: %w", cfg.Backend, err)
			}
			log.Info("interface backend switched", "from", previousBackend, "to", cfg.Backend)
		}

		b := GetBackend()
		if b == nil {
			return errInterfaceConfigApplyNoBackendLoaded
		}
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

		// Reconcile PPPoE clients on reload.
		pppoeMu.Lock()
		reconcilePPPoEClients(cfg, activePPPoE, b, log)
		pppoeMu.Unlock()

		// Reconcile DHCP clients and IPv6 RA suppression after successful reload.
		eb := GetEventBus()
		if eb != nil {
			dhcpMu.Lock()
			reconcileDHCP(cfg, eb, activeDHCP, log)
			suppressRAForConfig(cfg, suppressedRA, activeRouters, raRoutePriority, eb, log)
			dhcpMu.Unlock()
		}

		// Reconcile Router Advertisement senders after a successful reload.
		// This is the send side, independent of the accept_ra suppression
		// above, which is the receive side.
		raMu.Lock()
		reconcileRA(cfg, activeRA, log)
		raMu.Unlock()

		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		pendingCfg = nil
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

	p.OnConfigOperationVerify(func(input sdk.ConfigOperationVerifyInput) error {
		return verifyIfaceOperation(&input.Operation)
	})

	p.OnConfigOperationApply(func(input sdk.ConfigOperationApplyInput) (*sdk.ConfigOperationApplyOutput, error) {
		b := GetBackend()
		if b == nil {
			return nil, errInterfaceConfigApplyNoBackendLoaded
		}
		j, err := applyIfaceOperation(&input.Operation, b)
		if err != nil {
			return nil, err
		}
		operationMu.Lock()
		operationJournals[operationJournalKey(input.TransactionID, input.Operation.ID)] = j
		operationMu.Unlock()
		return &sdk.ConfigOperationApplyOutput{Status: "ok"}, nil
	})

	p.OnConfigOperationRollback(func(input sdk.ConfigOperationRollbackInput) error {
		operationMu.Lock()
		defer operationMu.Unlock()
		for i := range input.Operations {
			op := &input.Operations[i]
			key := operationJournalKey(input.TransactionID, op.ID)
			j := operationJournals[key]
			delete(operationJournals, key)
			if j == nil {
				continue
			}
			if errs := j.Rollback(); len(errs) > 0 {
				return fmt.Errorf("interface operation rollback %s: %d errors", op.ID, len(errs))
			}
		}
		return nil
	})

	p.OnConfigOperationCommit(func(input sdk.ConfigOperationCommitInput) error {
		operationMu.Lock()
		for key, j := range operationJournals {
			if strings.HasPrefix(key, input.TransactionID+"\x00") {
				j.Discard()
				delete(operationJournals, key)
			}
		}
		operationMu.Unlock()
		if pendingCfg != nil {
			activeCfg.Store(pendingCfg)
			pendingCfg = nil
		}
		log.Info("interface config operation journal committed")
		return nil
	})

	tracker := newRateTracker()
	globalTracker.Store(tracker)
	tracker.Start()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:      []string{"interface"},
		ConfigOperations: ifaceConfigOperationDecls(),
		VerifyBudget:     2,
		ApplyBudget:      10,
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

	// Stop the registry reconcile worker. Deliberately does NOT call
	// setAddressOwnerReconcileTrigger(nil) here: addressOwnerTrigger is a
	// single package-level var, and a respawned iface engine instance
	// (ProcessManager.Respawn) can call setAddressOwnerReconcileTrigger
	// with its own trigger before this shutdown sequence reaches this
	// point -- clearing it here would then clobber the NEW instance's
	// trigger with nil, silently disabling registry-triggered
	// reconciliation until an unrelated config commit. Leaving a stale
	// trigger installed after shutdown is harmless: registryReconcileCh is
	// never closed (see its declaration), so a send through the dead
	// trigger is a no-op nobody drains, not a panic; a subsequent engine
	// instance's own setAddressOwnerReconcileTrigger call simply replaces
	// it, which is the only way this reaches a consistent final state.
	close(registryReconcileStop)
	<-registryReconcileDone

	// Stop all PPPoE clients on shutdown.
	pppoeMu.Lock()
	for name, client := range activePPPoE {
		log.Debug("interface: stopping PPPoE client on shutdown", "name", name)
		client.Stop()
	}
	pppoeMu.Unlock()

	// Stop all Router Advertisement senders on shutdown. Each one sends a
	// final advertisement with Router Lifetime 0 as it stops, so hosts drop
	// Ze from their default router list at once (RFC 4861 Section 6.2.5).
	raMu.Lock()
	stopAllRASenders(activeRA, log)
	raMu.Unlock()

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

	tracker.Stop()
	globalTracker.Store(nil)

	if err := CloseBackend(); err != nil {
		log.Warn("interface backend close failed", "error", err)
	}
	log.Info("interface backend closed")

	return 0
}

// permissionRemediation is appended to an interface apply failure that the
// kernel refused for want of privilege, so the returned error carries the
// corrective action and not just the syscall text (ai/rules/cli.md:
// an error must say what failed, why, and what to do next).
//
// Interface work is netlink RTM_NEWLINK/RTM_NEWADDR, which the kernel gates on
// CAP_NET_ADMIN. An unprivileged ze therefore fails EVERY interface operation
// and aborts startup, so the operator needs the capability named at the point
// of failure -- the pre-flight "running without root" warning
// (internal/core/privilege/check_linux.go) scrolls past long before this and
// does not survive into the error at all.
const permissionRemediation = " (interface configuration needs CAP_NET_ADMIN: run ze as root, or grant the binary the capability with `setcap cap_net_admin+ep <path-to-ze>`)"

// lacksPrivilege reports whether any apply error was the kernel refusing the
// operation for want of privilege. netlink returns EPERM, and some paths EACCES;
// syscall.Errno.Is maps both to os.ErrPermission, so this needs no build tag and
// no errno spelling of its own.
func lacksPrivilege(errs []error) bool {
	for _, e := range errs {
		if errors.Is(e, os.ErrPermission) {
			return true
		}
	}
	return false
}

// joinApplyErrors logs each error at Warn level and returns the error the
// plugin's configure/reload callback reports to the engine.
//
// It WRAPS a cause rather than summarizing one away: this error is what the
// operator sees on stderr when startup aborts, and "N errors (see log for
// details)" pointed at a log the caller may not have (the engine's copy of this
// error crosses an RPC boundary as text). One error is wrapped whole; several
// name the count and wrap the first, which keeps the message bounded while
// still carrying evidence. Every error is logged individually above regardless.
func joinApplyErrors(prefix string, errs []error) error {
	log := loggerPtr.Load()
	for _, e := range errs {
		log.Warn(prefix, "err", e)
	}
	hint := ""
	if lacksPrivilege(errs) {
		hint = permissionRemediation
	}
	if len(errs) == 1 {
		return fmt.Errorf("%s: %w%s", prefix, errs[0], hint)
	}
	return fmt.Errorf("%s: %d errors, first: %w%s", prefix, len(errs), errs[0], hint)
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
var dhcpClientFactory func(ifaceName string, unit string, eb ze.EventBus, v4, v6 bool, hostname, clientID string, pdLength int, duid, resolvConfPath string, hasStaticNameServers bool, routeMetric int) (DHCPStopper, error)

// SetDHCPClientFactory registers the factory function used to create
// DHCP clients. Called from ifacedhcp's init().
func SetDHCPClientFactory(f func(string, string, ze.EventBus, bool, bool, string, string, int, string, string, bool, int) (DHCPStopper, error)) {
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
	// routePriority is the kernel metric of the default route the lease
	// installs: defaultLearnedRouteMetric unless the unit sets route-priority.
	routePriority int
	// routePrioritySet says the operator wrote route-priority on the unit.
	// Only that answers whether ze owns this interface's IPv6 default routes;
	// routePriority alone cannot, because its default is now non-zero.
	routePrioritySet bool
}

// dhcpEntry tracks a running DHCP client and the params it was created with.
type dhcpEntry struct {
	client      DHCPStopper
	params      dhcpParams
	gateway     string // last known gateway from DHCP lease (for link failover)
	metricState routeMetricState
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
			var dhcp *dhcpUnitConfig
			var dhcpv6 *dhcpv6UnitConfig
			if u.IPv4 != nil {
				dhcp = u.IPv4.DHCP
			}
			if u.IPv6 != nil {
				dhcpv6 = u.IPv6.DHCPv6
			}
			v4 := dhcp != nil && dhcp.Enabled
			v6 := dhcpv6 != nil && dhcpv6.Enabled
			if !v4 && !v6 {
				continue
			}
			key := dhcpUnitKey{ifaceName: name, unit: u.Label}
			p := dhcpParams{v4: v4, v6: v6, routePriority: u.RoutePriority, routePrioritySet: u.RoutePrioritySet}
			if dhcp != nil {
				p.hostname = dhcp.Hostname
				p.clientID = dhcp.ClientID
			}
			if dhcpv6 != nil {
				p.pdLength = dhcpv6.PDLength
				p.duid = dhcpv6.DUID
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
	for i := range cfg.XFRM {
		collectDHCPUnits(cfg.XFRM[i].Name, cfg.XFRM[i].Units)
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
			key := dhcpUnitKey{ifaceName: name, unit: "default"}
			// dhcp-auto writes no unit, so it takes the same learned-route
			// metric an unset route-priority gives a configured one.
			desired[key] = dhcpParams{v4: true, routePriority: defaultLearnedRouteMetric}
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

// routeMetricState says where a link handler last put an interface-layer
// default route. A handler that does not know moves a route that is not there:
// (*monitor).handleLinkUpdate (internal/plugins/iface/netlink/monitor_linux.go)
// emits up or down on EVERY RTM_NEWLINK for a link it already knows, with no
// comparison against the state it last reported, so an MTU change or a carrier
// bounce reaches these handlers many times with nothing to do. Each repeat cost
// a delete the kernel refused, a replace that reinstalled the route it had just
// refused to delete, and the route-table read that a refused delete triggers
// (reportRemoveRouteMiss, internal/plugins/iface/netlink/manage_linux.go).
//
// routeMetricUnknown is the zero value and says ze does not know which metric
// carries this route. The handler then runs its full remove-and-add, and three
// paths reach it:
//
//   - An entry ze has not moved since the process started. A previous run that
//     deprioritized the route and exited left it at base + deprioritizedMetric,
//     and the first link event after the lease is what clears it.
//   - A lease event (handleDHCPLeaseEvent), because the DHCP client publishes
//     the lease whether or not its own route install succeeded.
//   - A handler whose AddRoute failed after its RemoveRoute had already taken
//     the route away. The route is then at neither metric, and the full run on
//     the next event is the repair.
type routeMetricState uint8

const (
	routeMetricUnknown routeMetricState = iota
	routeMetricBase
	routeMetricDeprioritized
)

// effectiveMetric returns the metric an interface-layer route is installed at:
// the deprioritized one when a link handler moved it there, the base one
// otherwise. A remove that names the other metric matches nothing, and the
// route it meant to remove stays in the kernel.
func effectiveMetric(base int, state routeMetricState) int {
	if state == routeMetricDeprioritized {
		return base + deprioritizedMetric
	}
	return base
}

// handleDHCPLeaseEvent updates the stored gateway for link-state failover.
func handleDHCPLeaseEvent(data string, active map[dhcpUnitKey]dhcpEntry, log *slog.Logger) {
	var payload struct {
		Name   string `json:"name"`
		Unit   string `json:"unit"`
		Router string `json:"router"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil || payload.Router == "" {
		return
	}
	key := dhcpUnitKey{ifaceName: payload.Name, unit: payload.Unit}
	if entry, ok := active[key]; ok {
		entry.gateway = payload.Router
		// A lease says nothing reliable about which metrics carry a route now.
		// (*DHCPClient).handleV4Lease (internal/plugins/iface/dhcp/dhcp_v4_linux.go)
		// installs at the configured metric and then publishes the lease even
		// when that install returned an error, and its install removes no route
		// a link handler put at the deprioritized metric. Recording either
		// metric here would make the next link event a no-op over a kernel that
		// holds something else, so record that ze does not know: the next link
		// event runs its full remove-and-add and converges the two.
		entry.metricState = routeMetricUnknown
		active[key] = entry
		log.Debug("interface: stored DHCP gateway for failover", "iface", payload.Name, "gw", payload.Router)
	}
}

// handleLinkDown is called by the link worker when an interface carrier drops.
// If there's a DHCP client on that interface with a known gateway, remove the
// normal-metric route and add a deprioritized one. A down event for a route
// already at the deprioritized metric moves nothing (routeMetricState).
// Caller MUST hold dhcpMu.
func handleLinkDown(ifaceName string, active map[dhcpUnitKey]dhcpEntry, log *slog.Logger) {
	for key, entry := range active {
		if key.ifaceName != ifaceName || entry.gateway == "" {
			continue
		}
		if entry.metricState == routeMetricDeprioritized {
			// The route is already at the deprioritized metric. A repeated down
			// event has nothing to move, and moving it again would delete a
			// route that is not at the base metric any more.
			return
		}
		baseMetric := entry.params.routePriority
		newMetric := baseMetric + deprioritizedMetric
		log.Info("interface: link down, deprioritizing route", "iface", ifaceName, "gw", entry.gateway, "from", baseMetric, "to", newMetric)
		// Remove the base-metric route first, then add deprioritized.
		// Linux route identity is (dst, gw, link, metric) so RouteReplace
		// with a different metric creates a second entry, not a replacement.
		_ = RemoveRoute(ifaceName, "0.0.0.0/0", entry.gateway, baseMetric, rtproto.Iface)
		if err := AddRoute(ifaceName, "0.0.0.0/0", entry.gateway, newMetric, rtproto.Iface); err != nil {
			// The remove landed and the add did not, so the kernel now holds
			// this route at neither metric. Record that ze does not know where
			// it is BEFORE leaving: an entry still claiming a metric makes the
			// next opposite event a no-op, and the route stays gone until the
			// next lease. routeMetricUnknown is what makes that event run its
			// full remove-and-add and put the route back.
			entry.metricState = routeMetricUnknown
			active[key] = entry
			log.Debug("interface: deprioritize route failed", "iface", ifaceName, "err", err)
			return
		}
		entry.metricState = routeMetricDeprioritized
		active[key] = entry
		return
	}
}

// handleLinkUp is called by the link worker when an interface carrier is
// restored. Removes the deprioritized route and installs normal metric. An up
// event for a route already at the base metric restores nothing
// (routeMetricState).
// Caller MUST hold dhcpMu.
func handleLinkUp(ifaceName string, active map[dhcpUnitKey]dhcpEntry, log *slog.Logger) {
	for key, entry := range active {
		if key.ifaceName != ifaceName || entry.gateway == "" {
			continue
		}
		if entry.metricState == routeMetricBase {
			// The route is already at the base metric. A repeated up event has
			// nothing to restore.
			return
		}
		baseMetric := entry.params.routePriority
		oldMetric := baseMetric + deprioritizedMetric
		log.Info("interface: link up, restoring route priority", "iface", ifaceName, "gw", entry.gateway, "from", oldMetric, "to", baseMetric)
		// Remove the deprioritized route, restore base metric.
		_ = RemoveRoute(ifaceName, "0.0.0.0/0", entry.gateway, oldMetric, rtproto.Iface)
		if err := AddRoute(ifaceName, "0.0.0.0/0", entry.gateway, baseMetric, rtproto.Iface); err != nil {
			// See handleLinkDown: the route is at neither metric now, so the
			// entry MUST NOT keep claiming one. The next down event then runs
			// its full remove-and-add and puts the route back.
			entry.metricState = routeMetricUnknown
			active[key] = entry
			log.Debug("interface: restore route priority failed", "iface", ifaceName, "err", err)
			return
		}
		entry.metricState = routeMetricBase
		active[key] = entry
		return
	}
}

// handleRouterDiscovered processes a router-discovered event from the netlink
// monitor. It installs an IPv6 default route via the discovered router with
// the route-priority metric the operator wrote for that interface.
//
// priorities is the map suppressRAForConfig published, so the interfaces ze
// installs ::/0 on are exactly the interfaces whose kernel RA default route ze
// suppressed (writtenRoutePriorities).
// Caller MUST hold dhcpMu.
func handleRouterDiscovered(data string, routers map[routerKey]routerEntry, priorities map[string]int, log *slog.Logger) {
	var payload RouterEventPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil || payload.RouterIP == "" || payload.Name == "" {
		return
	}
	key := routerKey{ifaceName: payload.Name, routerIP: payload.RouterIP}
	if _, exists := routers[key]; exists {
		return // already tracking this router
	}
	metric := priorities[payload.Name]
	if metric <= 0 {
		return // route-priority not written, kernel keeps its RA default routes
	}
	if err := AddRoute(payload.Name, "::/0", payload.RouterIP, metric, rtproto.Iface); err != nil {
		log.Warn("interface: IPv6 default route install failed", "iface", payload.Name, "router", payload.RouterIP, "metric", metric, "err", err)
		return
	}
	routers[key] = routerEntry{metric: metric, metricState: routeMetricBase}
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
	_ = RemoveRoute(payload.Name, "::/0", payload.RouterIP, effectiveMetric(entry.metric, entry.metricState), rtproto.Iface)
	delete(routers, key)
	log.Info("interface: IPv6 default route removed (router lost)", "iface", payload.Name, "router", payload.RouterIP)
}

// handleLinkDownIPv6 deprioritizes all IPv6 default routes on an interface
// when its carrier drops. Same pattern as IPv4 handleLinkDown.
// Caller MUST hold dhcpMu.
func handleLinkDownIPv6(ifaceName string, routers map[routerKey]routerEntry, log *slog.Logger) {
	for key, entry := range routers {
		if key.ifaceName != ifaceName || entry.metricState == routeMetricDeprioritized {
			continue
		}
		newMetric := entry.metric + deprioritizedMetric
		log.Info("interface: link down, deprioritizing IPv6 route", "iface", ifaceName, "router", key.routerIP, "from", entry.metric, "to", newMetric)
		_ = RemoveRoute(ifaceName, "::/0", key.routerIP, entry.metric, rtproto.Iface)
		if err := AddRoute(ifaceName, "::/0", key.routerIP, newMetric, rtproto.Iface); err != nil {
			// See handleLinkDown: the route is at neither metric now, so the
			// entry MUST NOT keep claiming one. Without this, recovery waits
			// for a full router-lost then router-discovered cycle.
			entry.metricState = routeMetricUnknown
			routers[key] = entry
			log.Debug("interface: IPv6 deprioritize failed", "iface", ifaceName, "err", err)
			continue
		}
		entry.metricState = routeMetricDeprioritized
		routers[key] = entry
	}
}

// handleLinkUpIPv6 restores all IPv6 default routes on an interface
// when its carrier is restored.
// Caller MUST hold dhcpMu.
func handleLinkUpIPv6(ifaceName string, routers map[routerKey]routerEntry, log *slog.Logger) {
	for key, entry := range routers {
		if key.ifaceName != ifaceName || entry.metricState == routeMetricBase {
			continue
		}
		oldMetric := entry.metric + deprioritizedMetric
		log.Info("interface: link up, restoring IPv6 route priority", "iface", ifaceName, "router", key.routerIP, "from", oldMetric, "to", entry.metric)
		_ = RemoveRoute(ifaceName, "::/0", key.routerIP, oldMetric, rtproto.Iface)
		if err := AddRoute(ifaceName, "::/0", key.routerIP, entry.metric, rtproto.Iface); err != nil {
			// See handleLinkDown: the route is at neither metric now, so the
			// entry MUST NOT keep claiming one. The next down event then runs
			// its full remove-and-add and puts the route back.
			entry.metricState = routeMetricUnknown
			routers[key] = entry
			log.Debug("interface: IPv6 restore priority failed", "iface", ifaceName, "err", err)
			continue
		}
		entry.metricState = routeMetricBase
		routers[key] = entry
	}
}

// writtenRoutePriorities returns, per interface, the route-priority the
// operator WROTE above 0 on one of its units. It is the ONE answer to "does ze
// own the IPv6 default routes of this interface": suppressRAForConfig sets
// accept_ra_defrtr=0 on exactly the interfaces in this map, and
// handleRouterDiscovered installs ::/0 at the metric each one carries.
//
// Deriving that answer twice, from two different sources, is the defect this
// replaced. The suppression read the config while the install read the running
// DHCP clients, and reconcileDHCP starts no client for a unit that enables
// neither DHCPv4 nor DHCPv6. A SLAAC-only unit carrying route-priority was
// therefore suppressed and never installed for: ze deleted the kernel's RA
// default route and put none of its own back, leaving the interface with no
// IPv6 default route at all.
//
// The WRITTEN value is the test, not the effective one: route-priority defaults
// to defaultLearnedRouteMetric, so an interface whose units never mention the
// leaf is absent from this map and the kernel keeps its RA default routes.
// RA routes are per-interface, not per-unit, so when several units on one
// interface write the leaf, the first written value above 0 wins.
func writtenRoutePriorities(cfg *ifaceConfig) map[string]int {
	priorities := make(map[string]int)
	collect := func(name string, units []unitEntry) {
		for i := range units {
			if !units[i].RoutePrioritySet || units[i].RoutePriority <= 0 {
				continue
			}
			if _, seen := priorities[name]; !seen {
				priorities[name] = units[i].RoutePriority
			}
		}
	}
	for _, e := range cfg.Ethernet {
		collect(e.Name, e.Units)
	}
	for _, e := range cfg.Dummy {
		collect(e.Name, e.Units)
	}
	for _, e := range cfg.Veth {
		collect(e.Name, e.Units)
	}
	for _, e := range cfg.Bridge {
		collect(e.Name, e.Units)
	}
	for i := range cfg.Tunnel {
		collect(cfg.Tunnel[i].Name, cfg.Tunnel[i].Units)
	}
	for i := range cfg.Wireguard {
		collect(cfg.Wireguard[i].Name, cfg.Wireguard[i].Units)
	}
	for i := range cfg.XFRM {
		collect(cfg.XFRM[i].Name, cfg.XFRM[i].Units)
	}
	return priorities
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

// restoreAcceptRaDefrtr hands the IPv6 default routes of an interface back to
// the kernel: it restores accept_ra_defrtr=1, then removes the ::/0 routes ze
// installed while it owned them.
//
// The order is load-bearing, and so is the failure path. The kernel installs an
// RA default route again only once accept_ra_defrtr is back to 1, so the sysctl
// goes first and ze's own route stays until it lands. An Emit failure then
// leaves the interface as it was, suppressed and carrying ze's ::/0, and it
// stays in suppressed so a later suppressRAForConfig retries it. Removing the
// routes first, or forgetting the interface on a failed Emit, leaves the kernel
// suppressed with nothing installed: no IPv6 default route at all, and nothing
// left that would ever try again.
func restoreAcceptRaDefrtr(ifaceName string, suppressed map[string]bool, routers map[routerKey]routerEntry, eb ze.EventBus, log *slog.Logger) {
	if !suppressed[ifaceName] {
		return
	}
	var tb textbuf.Buffer
	sysctlKey := tb.Str("net.ipv6.conf.").Str(ifaceName).Str(".accept_ra_defrtr").String()
	tb.Reset()
	payload := tb.Str(`{"key":`).Quoted(sysctlKey).Str(`,"value":"1","source":"interface"}`).String()
	if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventSet, payload); err != nil {
		log.Warn("interface: failed to restore accept_ra_defrtr; the interface stays suppressed and keeps the ::/0 ze installed, and a later reconcile retries",
			"iface", ifaceName, "err", err)
		return
	}
	// The kernel owns these routes again, so ze's own go.
	// Collect keys first: we delete from routers during iteration.
	var removeKeys []routerKey
	for key := range routers {
		if key.ifaceName == ifaceName {
			removeKeys = append(removeKeys, key)
		}
	}
	for _, key := range removeKeys {
		entry := routers[key]
		_ = RemoveRoute(ifaceName, "::/0", key.routerIP, effectiveMetric(entry.metric, entry.metricState), rtproto.Iface)
		delete(routers, key)
	}
	delete(suppressed, ifaceName)
	log.Info("interface: restored accept_ra_defrtr", "iface", ifaceName)
}

// suppressRAForConfig suppresses accept_ra_defrtr on every interface whose
// config writes a route-priority above 0, and restores it on interfaces that no
// longer qualify (config removal). DHCPv6 is not part of the test: SLAAC and
// static IPv6 receive the same RAs, and suppressing on an IPv4-only interface
// is harmless because it processes no RA.
//
// priorities is the map handleRouterDiscovered reads. This function is what
// publishes it, so one derivation decides both halves: the interfaces ze ASKS to
// suppress the kernel's RA default route on are the interfaces ze installs its
// own ::/0 on, at the same metric. Suppressing without installing leaves the
// interface with no IPv6 default route at all, which is what two separate
// derivations produced.
//
// The two sets differ only while a sysctl event fails to reach the sysctl
// component, and each of the two failures leaves a default route on the
// interface:
//
//   - The suppression failed. suppressAcceptRaDefrtr records nothing, so the
//     interface is published here and not suppressed, and the kernel keeps its
//     own RA default route beside the one ze installs.
//   - The restore failed. restoreAcceptRaDefrtr emits before it removes
//     anything and keeps the interface in suppressed, so the interface is
//     suppressed, not published, and still carries the ::/0 ze installed. The
//     restore loop below retries it on the next call.
//
// The second state has one gap while it lasts: a router-lost event removes
// that ::/0, and handleRouterDiscovered installs no replacement for an
// interface this map does not publish, so the interface has no IPv6 default
// route until the retry lands.
// Caller MUST hold dhcpMu.
func suppressRAForConfig(cfg *ifaceConfig, suppressed map[string]bool, routers map[routerKey]routerEntry, priorities map[string]int, eb ze.EventBus, log *slog.Logger) {
	if eb == nil {
		// Suppression travels on the sysctl event bus. Without a bus ze cannot
		// stop the kernel installing its RA default route, so it MUST NOT
		// install one of its own: an empty map keeps handleRouterDiscovered off
		// every interface.
		clear(priorities)
		return
	}
	clear(priorities)
	maps.Copy(priorities, writtenRoutePriorities(cfg))

	// Suppress on newly qualifying interfaces: exactly the ones just published.
	for name := range priorities {
		suppressAcceptRaDefrtr(name, suppressed, eb, log)
	}
	// Restore on interfaces that no longer qualify.
	// Collect keys first: restoreAcceptRaDefrtr deletes from suppressed.
	restoreList := make([]string, 0)
	for name := range suppressed {
		if _, ok := priorities[name]; !ok {
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
		_ = RemoveRoute(ifaceName, "::/0", r.Gateway, r.Metric, rtproto.Any)
		log.Info("interface: removed stale kernel IPv6 default route", "iface", ifaceName, "gw", r.Gateway, "metric", r.Metric)
	}
}
