// Design: docs/research/l2tpv2-ze-integration.md -- subsystem lifecycle
// Detail: config.go -- ExtractParameters / Parameters struct consumed by NewSubsystem
// Related: register.go -- blank import wiring for schema/ package
// Related: listener.go -- UDP transport owned by the subsystem
// Related: reactor.go -- dispatch goroutine owned by the subsystem

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/callsink"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

var errL2tpSubsystemAlreadyStarted = errors.New("l2tp: subsystem already started")

// ifaceBackendFn returns the active iface backend wrapped in the small
// interface ppp.Driver consumes. Production wires iface.GetBackend();
// if no backend is loaded the subsystem skips PPP driver construction.
// Package-level var so a future test can swap it when a test-only
// fake iface backend is introduced; no injector exists today.
var ifaceBackendFn = defaultIfaceBackend

// defaultIfaceBackend returns iface.GetBackend() typed as ppp.IfaceBackend
// when one is loaded; nil when none. The PPP driver is only constructed
// when this returns non-nil so MTU-set on pppN is always reachable.
func defaultIfaceBackend() ppp.IfaceBackend {
	b := iface.GetBackend()
	if b == nil {
		return nil
	}
	return b
}

// Compile-time interface check: Subsystem must satisfy ze.Subsystem.
var _ ze.Subsystem = (*Subsystem)(nil)

// SubsystemName is the canonical identifier for the L2TP subsystem.
const SubsystemName = "l2tp"

// probeKernelModulesFn is the kernel module probe invoked at Start().
// Production uses probeKernelModules (Linux modprobe; no-op on other OS).
// Tests override this via export_test.go to run without root privileges.
var probeKernelModulesFn = probeKernelModules

// newSubsystemKernelWorkerFn constructs the per-listener kernel worker.
// Production uses newSubsystemKernelWorker (Linux genl; returns nil on other OS).
// Indirected as a var so a test can assert WHETHER construction was attempted,
// which is the only host-independent way to pin the
// ze.l2tp.disable-kernel-dataplane knob: the real constructor legitimately
// returns nil on non-Linux and on a genl resolve failure, so a nil result proves
// nothing on its own.
var newSubsystemKernelWorkerFn = newSubsystemKernelWorker

// Subsystem is the ze.Subsystem implementation for L2TPv2.
//
// Phase 3 scope: UDP listener + reactor skeleton are wired. Tunnel state
// machines, timer goroutine, and full FSM transitions land in later
// phases. Start with Parameters whose Enabled=false is a no-op.
//
// Caller MUST call Stop when done if Start returned nil.
type Subsystem struct {
	params Parameters
	logger *slog.Logger

	mu            sync.Mutex
	started       bool
	listeners     []*UDPListener
	reactors      []*L2TPReactor
	timers        []*tunnelTimer
	kernelWorkers []*kernelWorker
	pppDrivers    []*ppp.Driver
	drainDones    []<-chan struct{} // auth + pool drain goroutine completion
	// routeObserver tracks subscriber routes (session IP-up / down).
	// One instance per subsystem; installed into every reactor at
	// Start. See spec-l2tp-7 "Redistribute" and route_observer.go.
	routeObserver *subscriberRouteObserver
	// replayUnsub unsubscribes the redistribute ReplayRequest handler
	// installed at Start; nil when no bus was provided. Cleared on Stop/unwind.
	replayUnsub func()
	// observer tracks per-session events and per-login CQM samples.
	// Created at Start when CQMEnabled; nil otherwise.
	observer *Observer
	// statsPoller periodically reads pppN interface counters for Prometheus.
	// Created at Start when metrics registry is available; nil otherwise.
	statsPoller *l2tpStatsPoller
	// subBridge translates L2TP session events into subscriber namespace
	// events and maintains the subscriber registry.
	subBridge *subscriberBridge
}

// NewSubsystem constructs an L2TP subsystem from parsed Parameters. The returned
// value is inert until Start is called.
func NewSubsystem(p Parameters) *Subsystem {
	return &Subsystem{
		params: p,
		logger: slogutil.Logger(SubsystemName),
	}
}

// Name implements ze.Subsystem.
func (s *Subsystem) Name() string { return SubsystemName }

// cqmEchoInterval returns the PPP echo interval override for CQM sampling.
// Returns 0 (no override) when CQM is disabled.
func (s *Subsystem) cqmEchoInterval() time.Duration {
	if !s.params.CQMEnabled {
		return 0
	}
	d := 1 * time.Second
	if raw := env.Get("ze.l2tp.cqm.echo-interval"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			d = parsed
		} else {
			s.logger.Warn("l2tp: invalid ze.l2tp.cqm.echo-interval; falling back to 1s",
				"value", raw)
		}
	}
	return d
}

// Start implements ze.Subsystem. It is a no-op when Enabled=false or when
// no listener addresses are configured. Phase 3 logs the intent; phase 2
// of the tunnel work wires the actual UDP listener.
//
// MUST be called before Stop/Reload.
func (s *Subsystem) Start(ctx context.Context, bus ze.EventBus, _ ze.ConfigProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errL2tpSubsystemAlreadyStarted
	}

	if !s.params.Enabled {
		s.logger.Info("L2TP subsystem disabled in config, skipping start")
		s.started = true
		return nil
	}
	if len(s.params.ListenAddrs) == 0 {
		s.logger.Warn("L2TP subsystem enabled but no listener configured, skipping start")
		s.started = true
		return nil
	}

	if s.params.ReauthInterval > 0 {
		s.logger.Info("l2tp: periodic PPP re-auth enabled", "interval", s.params.ReauthInterval)
	}

	// spec-l2tp-7 Phase 6: register the L2TP redistribute source
	// (`redistribute l2tp` becomes a valid config knob) and stand up
	// the per-subsystem route observer. The observer is installed
	// into every reactor below so session IP-up / session-down
	// transitions drive subscriber route inject / withdraw.
	RegisterL2TPSources()
	s.routeObserver = newSubscriberRouteObserver(s.logger, bus)

	// Redistribute late-join replay: on a ReplayRequest re-emit the current
	// subscriber-route set tagged with the echoed ReplayID so a peer that
	// established after injection receives them (spec-redistribute-late-join-replay).
	if bus != nil {
		obs := s.routeObserver
		s.replayUnsub = redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) {
			obs.reemitAll(r.ReplayID)
		})
	}

	// spec-l2tp-9: create the CQM observer when enabled.
	if s.params.CQMEnabled {
		bucketCount := max(s.params.SampleRetentionSeconds/100, 1)
		maxSess := int(s.params.MaxSessions)
		if maxSess == 0 {
			maxSess = 1000
		}
		s.observer = NewObserver(ObserverConfig{
			MaxSessions:   maxSess,
			EventRingSize: s.params.EventRingSizePerSession,
			MaxLogins:     s.params.MaxLogins,
			BucketCount:   bucketCount,
			EchoInterval:  s.cqmEchoInterval(),
		})
		s.wireObserverSubscriptions(bus)
		s.logger.Info("l2tp: CQM observer started",
			"max-logins", s.params.MaxLogins,
			"event-ring-size", s.params.EventRingSizePerSession,
			"bucket-count", bucketCount)
	}

	// Phase 5: probe kernel modules before binding listeners.
	// AC-1/AC-2: on Linux, modprobe l2tp_ppp or pppol2tp must succeed.
	// On non-Linux, probeKernelModules() is a no-op (returns nil).
	// RFC 2661 Section 24.23: fail startup if module probe fails.
	//
	// spec-l2tp-7 Phase 8: ze.l2tp.skip-kernel-probe bypasses the probe
	// so `.ci` tests that only exercise the CLI surface (no data plane)
	// can boot ze without CAP_NET_ADMIN.
	if env.GetBool("ze.l2tp.skip-kernel-probe", false) {
		s.logger.Warn("l2tp: skipping kernel module probe (ze.l2tp.skip-kernel-probe=true)")
	} else if err := probeKernelModulesFn(); err != nil {
		return fmt.Errorf("l2tp: %w", err)
	}

	// Read once, outside the per-listener loop below: the value cannot change
	// mid-Start, and re-reading it per listener would let a concurrent Setenv in
	// a test produce a subsystem whose listeners disagree about whether the data
	// plane exists.
	disableDataplane := env.GetBool("ze.l2tp.disable-kernel-dataplane", false)

	// Bind every configured listen endpoint and launch a reactor + timer
	// + kernel worker for each. On any bind failure, unwind the partial
	// state so a retry is safe.
	for _, addr := range s.params.ListenAddrs {
		ln := NewUDPListener(addr, s.logger)
		if err := ln.Start(ctx); err != nil {
			s.unwindLocked()
			return fmt.Errorf("l2tp: bind %s: %w", addr, err)
		}
		reactor := NewL2TPReactor(ln, s.logger, ReactorParams{
			MaxTunnels:      s.params.MaxTunnels,
			MaxSessions:     s.params.MaxSessions,
			AuthMethod:      s.params.AuthMethod,
			AuthRequired:    !s.params.AllowNoAuth,
			AuthTimeout:     s.params.AuthTimeout,
			ReauthInterval:  s.params.ReauthInterval,
			HelloInterval:   s.params.HelloInterval,
			HelloRetries:    s.params.HelloRetries,
			EnableIPCP:      s.params.EnableIPCP,
			EnableIPv6CP:    s.params.EnableIPv6CP,
			NCPTimeout:      s.params.NCPTimeout,
			CQMEchoInterval: s.cqmEchoInterval(),
			Defaults: TunnelDefaults{
				// HostName left empty; reactor applies "ze" default.
				// Phase 7 will wire a YANG leaf for operator-controlled hostname.
				FramingCapabilities: 0x00000003, // sync + async per RFC 2661 S4.4.3
				BearerCapabilities:  0,
				RecvWindow:          16,
				SharedSecret:        s.params.SharedSecret,
			},
		})
		// spec-l2tp-7 Phase 6: install the subsystem's route observer
		// into every reactor so EventSessionIPAssigned and
		// EventSessionDown drive inject / withdraw.
		reactor.SetRouteObserver(s.routeObserver)
		// spec-l2tp-8a: install EventBus so reactor can emit
		// (l2tp, session-down) for pool release.
		reactor.SetEventBus(bus)

		// Phase 5: wire the kernel worker BEFORE starting the reactor so
		// SetKernelWorker's writes happen-before the reactor goroutine
		// first reads kernelErrCh. Worker may be nil on non-Linux or when
		// genl resolve fails -- the reactor handles that gracefully.
		//
		// errCh and successCh each have a single sender (the worker) and a
		// single reader (the reactor's run loop). They are never closed: GC
		// reclaims them when both goroutines exit during Stop. Closing
		// would race with the worker's report selects.
		errCh := make(chan kernelSetupFailed, 16)
		successCh := make(chan kernelSetupSucceeded, 16)
		// ze.l2tp.disable-kernel-dataplane leaves the worker nil, which the
		// reactor already treats as "no kernel setup to do" -- the same state it
		// is in on non-Linux (kernel_other.go newSubsystemKernelWorker returns
		// nil). Sessions still establish on the control plane, so the CLI surface
		// (show / teardown) is exercisable without CAP_NET_ADMIN.
		//
		// DISTINCT from ze.l2tp.skip-kernel-probe, which bypasses only the
		// modprobe above and leaves the data plane fully wired. That distinction
		// is load-bearing, not tidiness: test/l2tp/session-stopccn-cascade.ci
		// sets skip-kernel-probe AND needs the data plane (it carries
		// option=needs-linux:caps=net-admin and documents the reason inline), so
		// widening that knob to cover this would break it with a symptom -- no
		// session -- a long way from the cause.
		var worker *kernelWorker
		if disableDataplane {
			s.logger.Warn("l2tp: kernel data plane disabled (ze.l2tp.disable-kernel-dataplane=true); sessions will not be programmed into the kernel")
		} else {
			worker = newSubsystemKernelWorkerFn(errCh, successCh, s.logger)
		}
		reactor.SetKernelWorker(worker, errCh, successCh)

		// Phase 6a: construct a PPP driver if an iface backend is loaded.
		// The driver owns per-session goroutines that drive LCP and (in
		// later specs) auth + NCPs. The iface plugin delivers config
		// asynchronously, so the backend may not be loaded yet. As a
		// fallback, load the OS default backend directly; this is safe
		// because LoadBackend is idempotent when the same backend is
		// already active.
		var pppDriver *ppp.Driver
		if backend := ifaceBackendFn(); backend == nil {
			if name := iface.DefaultBackendName(); name != "" {
				if err := iface.LoadBackend(name); err != nil {
					s.logger.Warn("l2tp: fallback iface backend load failed", "error", err.Error())
				}
			}
		}
		if backend := ifaceBackendFn(); backend != nil {
			pppDriver = ppp.NewProductionDriver(s.logger.With("component", "ppp"), backend)
			reactor.SetPPPDriver(pppDriver)
		}

		// Start ordering: PPP driver before the kernel worker so any
		// success event the worker emits has a consumer ready, and both
		// before the reactor so its select arms have live channels.
		if pppDriver != nil {
			if err := pppDriver.Start(); err != nil {
				startErr := fmt.Errorf("l2tp: start PPP driver for %s: %w", addr, err)
				if stopErr := ln.Stop(); stopErr != nil {
					startErr = errors.Join(startErr, fmt.Errorf("l2tp: close listener %s: %w", addr, stopErr))
				}
				s.unwindLocked()
				return startErr
			}
			// spec-l2tp-8a: spawn auth and pool drain goroutines
			// after pppDriver.Start() so the channels are live.
			// Driver.Stop() closes the channels, causing the drains
			// to exit; we wait for them in unwindLocked/Stop.
			authH := GetAuthHandler()
			poolH := GetPoolHandler()
			if authH == nil {
				s.logger.Warn("l2tp: no auth handler registered; all sessions will be accepted")
			}
			if poolH == nil {
				s.logger.Error("l2tp: no pool handler registered; all IP requests will be rejected")
			}
			s.drainDones = append(s.drainDones,
				startAuthDrain(s.logger, pppDriver, authH, bus),
				startPoolDrain(s.logger, pppDriver, poolH),
			)
		}
		if worker != nil {
			worker.Start()
		}

		if err := reactor.Start(); err != nil {
			if worker != nil {
				worker.Stop()
			}
			if pppDriver != nil {
				pppDriver.Stop()
			}
			reactorErr := fmt.Errorf("l2tp: start reactor for %s: %w", addr, err)
			if stopErr := ln.Stop(); stopErr != nil {
				reactorErr = errors.Join(reactorErr, fmt.Errorf("l2tp: close listener %s: %w", addr, stopErr))
			}
			s.unwindLocked()
			return reactorErr
		}
		timer := newTunnelTimer(reactor.tickCh, reactor.updateCh)
		if err := timer.Start(); err != nil {
			reactor.Stop()
			if worker != nil {
				worker.Stop()
			}
			if pppDriver != nil {
				pppDriver.Stop()
			}
			timerErr := fmt.Errorf("l2tp: start timer for %s: %w", addr, err)
			if stopErr := ln.Stop(); stopErr != nil {
				timerErr = errors.Join(timerErr, fmt.Errorf("l2tp: close listener %s: %w", addr, stopErr))
			}
			s.unwindLocked()
			return timerErr
		}
		s.listeners = append(s.listeners, ln)
		s.reactors = append(s.reactors, reactor)
		s.timers = append(s.timers, timer)
		s.kernelWorkers = append(s.kernelWorkers, worker)
		s.pppDrivers = append(s.pppDrivers, pppDriver)
		s.logger.Info("L2TP listener bound", "address", ln.Addr().String())
	}
	// spec-l2tp-10: bind Prometheus metrics and start the stats poller.
	if reg := registry.GetMetricsRegistry(); reg != nil {
		bindL2TPMetrics(reg)
		subscriber.BindMetrics(reg)
		pollInterval := parsePollInterval()
		poller := newL2TPStatsPoller(s.reactors, pollInterval)
		poller.start()
		s.statsPoller = poller
		s.logger.Info("l2tp: metrics bound and stats poller started",
			"poll-interval", pollInterval)
	}

	if bus != nil {
		s.subBridge = newSubscriberBridge(subscriber.DefaultRegistry, bus, s.logger)
	}

	s.started = true
	// Publish the Service so CLI handlers (internal/component/l2tp/cmd/)
	// can reach the subsystem without importing it directly. Cleared in
	// Stop below so late callers observe nil rather than racing teardown.
	PublishService(s)
	// Register the PPPoE->L2TP relay sink (AC-3). The pppoe server consults
	// it at PADS completion; Relay returns false for services with no relay
	// binding, so registering unconditionally is safe and reload-ready.
	callsink.Register(&relaySink{s: s})
	return nil
}

// stopKernelWorkersLocked releases every kernel worker's resources and reaps its
// goroutine. Caller MUST hold s.mu.
//
// This MUST run before pppDriver.Stop, and that is a correctness requirement,
// not a preference. A PPP session's frame reader sits in a BLOCKING read(2) on
// its /dev/ppp channel descriptor (ppp.NewFDFile keeps the fd out of the Go
// poller on purpose -- frame_linux.go), and pppSession.run waits for that reader
// before returning, which Driver.Stop in turn waits for. Closing the channel
// descriptor does NOT end that read: on Linux close(2) drops one reference to an
// open file description that the blocked read still holds, so ppp_read is never
// woken and no error is delivered. The old comment here ("blocking reads return
// EBADF, per-session goroutines exit") described a shutdown that cannot happen.
//
// What DOES wake the reader is closing the PPPoX socket, which the kernel worker
// owns: pppol2tp_release -> ppp_unregister_channel marks the channel dead and
// wakes every waiter, so the read returns and the session goroutine unwinds.
// That is exactly how a RUNTIME session teardown already works -- the reactor
// never calls Driver.StopSession for L2TP, it enqueues a kernelTeardownEvent and
// the worker's teardownSessionFDsLocked closes the PPPoX socket. Shutdown had
// the two halves inverted, so Driver.Stop waited for a reader that only the
// still-pending kernel teardown could release.
//
// Observed 2026-07-28 on a Linux host with CAP_NET_ADMIN and l2tp_netlink
// loaded: after one session reached the kernel data plane, `ze` logged "L2TP
// subsystem stopping" and never exited (one thread parked in ppp_read, every
// other thread in futex_wait), so test/plugin/show-l2tp-{sessions,history,
// session-detail}.ci timed out instead of asserting.
//
// SignalStop comes first for all workers so an in-flight setupSession is broken
// out of its successCh/errCh send select BEFORE TeardownAll acquires w.mu;
// otherwise a blocked report holds w.mu and TeardownAll deadlocks.
func (s *Subsystem) stopKernelWorkersLocked() {
	for _, kw := range s.kernelWorkers {
		if kw != nil {
			kw.SignalStop()
		}
	}
	for _, kw := range s.kernelWorkers {
		if kw != nil {
			kw.TeardownAll()
			kw.Stop()
		}
	}
}

// unwindLocked stops any partially-started reactors and listeners. Must be
// called with s.mu held. Errors are joined so the caller can surface them
// all without suppressing any.
//
// Order matters. Stop timers and reactors BEFORE anything else so no new
// ppp.StartSession writes land on pppDriver.SessionsIn() mid-teardown. Then the
// kernel workers, then the PPP drivers (see stopKernelWorkersLocked for why that
// pair is in that order). The listener is closed last because the kernel data
// plane (programmed via the worker's socketFD) holds a kernel-side reference
// until tunnel delete completes.
func (s *Subsystem) unwindLocked() {
	var errs []error
	// Timers first: they send on reactor channels, so stop them before
	// the reactors close those channels.
	for _, t := range s.timers {
		t.Stop()
	}
	// Reactors next: after this returns, no new packets are dispatched,
	// no new kernelSetupEvents are enqueued, and no new ppp.StartSession
	// writes land on pppDriver.SessionsIn().
	for _, r := range s.reactors {
		r.Stop()
	}
	// Kernel workers BEFORE the PPP drivers: see stopKernelWorkersLocked.
	s.stopKernelWorkersLocked()
	// PPP drivers: close every active session's chan fd and wait for the
	// per-session goroutines. Driver.Stop closes AuthEventsOut/IPEventsOut,
	// causing drain goroutines to exit.
	for _, d := range s.pppDrivers {
		if d != nil {
			d.Stop()
		}
	}
	// Wait for drain goroutines to finish after channels are closed.
	for _, done := range s.drainDones {
		<-done
	}
	s.drainDones = nil
	// Listeners last: kernel tunnel/session delete commands carry a
	// reference to the UDP socket; close after the worker drains.
	for _, l := range s.listeners {
		if err := l.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.replayUnsub != nil {
		s.replayUnsub()
		s.replayUnsub = nil
	}
	if s.statsPoller != nil {
		s.statsPoller.stop()
		s.statsPoller = nil
	}
	if s.subBridge != nil {
		s.subBridge.stop()
		s.subBridge = nil
	}
	s.pppDrivers = nil
	s.kernelWorkers = nil
	s.timers = nil
	s.reactors = nil
	s.listeners = nil
	if len(errs) > 0 {
		s.logger.Warn("L2TP partial-start unwind encountered errors", "error", errors.Join(errs...).Error())
	}
}

// Stop implements ze.Subsystem. Idempotent. Reactors are stopped first so
// no more dispatch occurs, then listeners are closed to free the UDP
// sockets.
func (s *Subsystem) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}
	// Clear the Service publication BEFORE shutting down the reactors so
	// concurrent CLI handlers observe nil instead of calling into a
	// half-stopped subsystem. LookupService returns nil thereafter.
	PublishService(nil)
	// Clear the relay sink so a stopped subsystem stops accepting PPPoE
	// relay hand-offs (pppoe then terminates locally).
	callsink.Unregister()
	s.logger.Info("L2TP subsystem stopping")

	var errs []error
	// Same order as unwindLocked. Reactors stop before the kernel workers and
	// the PPP drivers so no new kernelSetupEvents / ppp.StartSession
	// dispatches land after TeardownAll, satisfying AC-14: every kernel
	// resource is torn down before Stop() returns.
	for _, t := range s.timers {
		t.Stop()
	}
	for _, r := range s.reactors {
		r.Stop()
	}
	// Kernel workers BEFORE the PPP drivers: see stopKernelWorkersLocked.
	s.stopKernelWorkersLocked()
	for _, d := range s.pppDrivers {
		if d != nil {
			d.Stop()
		}
	}
	for _, done := range s.drainDones {
		<-done
	}
	s.drainDones = nil
	for _, l := range s.listeners {
		if err := l.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.replayUnsub != nil {
		s.replayUnsub()
		s.replayUnsub = nil
	}
	if s.statsPoller != nil {
		s.statsPoller.stop()
		s.statsPoller = nil
	}
	if s.observer != nil {
		s.observer.Stop()
		s.observer = nil
	}
	if s.subBridge != nil {
		s.subBridge.stop()
		s.subBridge = nil
	}
	s.pppDrivers = nil
	s.kernelWorkers = nil
	s.timers = nil
	s.reactors = nil
	s.listeners = nil
	s.started = false
	return errors.Join(errs...)
}

// Reload is implemented in subsystem_reload.go.
